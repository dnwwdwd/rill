package wechat

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const (
	desktopUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	acceptLanguage   = "zh-CN,zh;q=0.9,en;q=0.8"
	acceptHTML       = "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8"

	maxArticleBody = 16 << 20
)

// MPBaseURL is the WeChat official-account site origin. Tests may replace it
// with the URL of an httptest server.
var MPBaseURL = "https://mp.weixin.qq.com"

func ensureContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func isSuccessStatus(status int) bool {
	return status >= 200 && status < 300
}

func (c *Client) baseURL() (string, error) {
	base := c.BaseURL
	if strings.TrimSpace(base) == "" {
		base = MPBaseURL
	}
	return normalizeBaseURL(base)
}

func (c *Client) httpClient() *http.Client {
	if c.HTTPClient == nil {
		return &http.Client{Timeout: DefaultTimeout}
	}
	if c.HTTPClient.Timeout > 0 {
		return c.HTTPClient
	}
	clone := *c.HTTPClient
	clone.Timeout = DefaultTimeout
	return &clone
}

func normalizeBaseURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = MPBaseURL
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("wechat: invalid base URL: %w", err)
	}
	if !strings.EqualFold(parsed.Scheme, "http") && !strings.EqualFold(parsed.Scheme, "https") {
		return "", fmt.Errorf("wechat: unsupported base URL scheme %q", parsed.Scheme)
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("wechat: base URL is missing a host")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return strings.TrimRight(parsed.String(), "/"), nil
}

func setBrowserHeaders(req *http.Request, referer string) {
	req.Header.Set("User-Agent", desktopUserAgent)
	req.Header.Set("Accept", acceptHTML)
	req.Header.Set("Accept-Language", acceptLanguage)
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
}

func readLimited(reader io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("wechat: response exceeds %d bytes", limit)
	}
	return data, nil
}
