package fetch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	DefaultMaxBody = 5 << 20
	MediaMaxBody   = 20 << 20
)

// HTTPStatusError preserves the upstream status for fetch logs while keeping a
// human-readable message for the UI.
type HTTPStatusError struct {
	Status  int
	Message string
}

func (e *HTTPStatusError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("upstream returned HTTP %d", e.Status)
	}
	return fmt.Sprintf("上游返回 HTTP %d：%s", e.Status, e.Message)
}

func (e *HTTPStatusError) StatusCode() int { return e.Status }

// ResponseError reports a 2xx response whose body/type cannot be used as a
// feed. It keeps the HTTP status for logs without presenting success as failure.
type ResponseError struct {
	Status  int
	Message string
}

func (e *ResponseError) Error() string   { return e.Message }
func (e *ResponseError) StatusCode() int { return e.Status }

// Client performs all outbound feed/site/media requests. Rill is a
// single-user, self-hosted reader running on the owner's own server: feeds
// may legitimately live on the LAN or on the same microserver, so private
// addresses are fetched like any other. Basic scheme and credential checks
// still apply.
type Client struct {
	HTTP          *http.Client
	UserAgent     string
	MaxBodyBytes  int64
	MaxRedirects  int
	RequestTimout time.Duration
}

func NewClient(userAgent string) *Client {
	if strings.TrimSpace(userAgent) == "" {
		userAgent = "Rill/0.1"
	}
	client := &Client{
		UserAgent:     userAgent,
		MaxBodyBytes:  MediaMaxBody,
		MaxRedirects:  3,
		RequestTimout: 15 * time.Second,
	}
	client.HTTP = client.newHTTPClient()
	return client
}

func (c *Client) newHTTPClient() *http.Client {
	transport := &http.Transport{
		Proxy:                 nil,
		DialContext:           dialIPv4First,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          32,
		MaxIdleConnsPerHost:   2,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: time.Second,
	}
	return &http.Client{
		Transport: transport,
		Timeout:   c.RequestTimout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) > c.MaxRedirects {
				return errors.New("too many redirects")
			}
			if err := c.ValidateURL(req.Context(), req.URL.String()); err != nil {
				return err
			}
			return nil
		},
	}
}

// dialIPv4First resolves the host and tries IPv4 addresses before IPv6.
// Some microserver/LAN DNS setups return a non-routable IPv6 address that
// accepts TCP then drops TLS; Go's happy-eyeballs dialer can stall on it even
// when IPv4 works. Trying IPv4 first keeps feeds reachable without disabling
// IPv6 for hosts that only have AAAA records.
func dialIPv4First(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil || net.ParseIP(host) != nil {
		dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
		return dialer.DialContext(ctx, network, address)
	}
	resolved, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	ordered := make([]net.IPAddr, 0, len(resolved))
	for _, addr := range resolved {
		if addr.IP.To4() != nil {
			ordered = append(ordered, addr)
		}
	}
	for _, addr := range resolved {
		if addr.IP.To4() == nil {
			ordered = append(ordered, addr)
		}
	}
	if len(ordered) == 0 {
		return nil, fmt.Errorf("no addresses for %s", host)
	}
	var lastErr error
	for _, addr := range ordered {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
		conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(addr.IP.String(), port))
		if err == nil {
			return conn, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

// ValidateURL rejects non-http schemes and hosts without a hostname before a
// request is made.
func (c *Client) ValidateURL(_ context.Context, raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return fmt.Errorf("地址格式无效")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("只允许 http/https 地址")
	}
	if parsed.Host == "" || parsed.Hostname() == "" {
		return fmt.Errorf("地址缺少主机名")
	}
	if parsed.User != nil {
		return fmt.Errorf("地址不能包含用户凭据")
	}
	return nil
}

type Response struct {
	Body         []byte
	StatusCode   int
	ContentType  string
	ETag         string
	LastModified string
	FinalURL     string
	Header       http.Header
}

// GetBytes fetches a bounded response. It is used for feed, HTML discovery and
// media downloads so that timeout, redirect and SSRF rules stay consistent.
func (c *Client) GetBytes(ctx context.Context, rawURL string, maxBytes int64, headers http.Header) (*Response, error) {
	if err := c.ValidateURL(ctx, rawURL); err != nil {
		return nil, err
	}
	if maxBytes <= 0 || maxBytes > c.MaxBodyBytes {
		maxBytes = c.MaxBodyBytes
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("构造请求失败")
	}
	if headers != nil {
		req.Header = headers.Clone()
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		var statusErr *HTTPStatusError
		if errors.As(err, &statusErr) {
			return nil, err
		}
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return nil, errors.New("请求超时或已取消")
		}
		msg := err.Error()
		var urlErr *url.Error
		if errors.As(err, &urlErr) && urlErr.Err != nil {
			// Never persist the request URL (it may carry feed/media tokens).
			msg = urlErr.Err.Error()
		}
		if strings.Contains(msg, "too many redirects") {
			return nil, errors.New("重定向超过 3 次")
		}
		return nil, fmt.Errorf("请求失败：%s", safeErrorText(msg))
	}
	defer resp.Body.Close()
	result := &Response{
		StatusCode:   resp.StatusCode,
		ContentType:  resp.Header.Get("Content-Type"),
		ETag:         resp.Header.Get("ETag"),
		LastModified: resp.Header.Get("Last-Modified"),
		FinalURL:     resp.Request.URL.String(),
		Header:       resp.Header.Clone(),
	}
	if resp.StatusCode == http.StatusNotModified {
		return result, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &HTTPStatusError{Status: resp.StatusCode, Message: http.StatusText(resp.StatusCode)}
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("读取响应失败")
	}
	if int64(len(body)) > maxBytes {
		return nil, &ResponseError{Status: resp.StatusCode, Message: fmt.Sprintf("响应体超过 %d 字节上限", maxBytes)}
	}
	if len(body) == 0 {
		return nil, &ResponseError{Status: resp.StatusCode, Message: "上游返回空响应"}
	}
	result.Body = body
	return result, nil
}

func safeErrorText(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 240 {
		value = value[:240] + "…"
	}
	// Avoid leaking credentials accidentally embedded in proxy error strings.
	value = strings.ReplaceAll(value, "Authorization", "authorization")
	return value
}
