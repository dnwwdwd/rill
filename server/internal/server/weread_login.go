package server

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"sort"
	"strings"
	"time"

	qrcode "github.com/skip2/go-qrcode"
)

const (
	wereadLoginTTL         = 5 * time.Minute
	wereadStatusTimeout    = 8 * time.Second
	wereadPollBlockTimeout = 2500 * time.Millisecond
	wereadVerifyTimeout    = 8 * time.Second
	wereadCookieMaxBytes   = 32 << 10
)

type wereadLoginSession struct {
	UID         string
	Fingerprint string
	Client      *http.Client
	Jar         *cookiejar.Jar
	ExpiresAt   time.Time
}

type wereadLoginInfo struct {
	Succeed      flexBool
	LogicCode    flexString
	WebLoginVid  flexString
	Vid          flexString
	UserVid      flexString
	AccessToken  flexString
	RefreshToken flexString
}

// flexString accepts JSON strings or numbers. WeRead returns webLoginVid as a
// number on real accounts even though other fields are strings.
type flexString string

func (s *flexString) UnmarshalJSON(data []byte) error {
	value := strings.TrimSpace(string(data))
	if value == "" || value == "null" {
		*s = ""
		return nil
	}
	if value[0] == '"' {
		var parsed string
		if err := json.Unmarshal(data, &parsed); err != nil {
			return err
		}
		*s = flexString(parsed)
		return nil
	}
	*s = flexString(strings.Trim(value, `"`))
	return nil
}

// flexBool accepts true/false, 1/0 or their string forms.
type flexBool bool

func (b *flexBool) UnmarshalJSON(data []byte) error {
	value := strings.Trim(strings.TrimSpace(string(data)), `"`)
	switch strings.ToLower(value) {
	case "true", "1":
		*b = true
	case "false", "0", "null", "":
		*b = false
	default:
		*b = false
	}
	return nil
}

// decodeWereadLoginInfo parses getLoginInfo tolerantly: real responses put the
// fields either at the top level or inside one or more `data` wrappers, and
// field types differ between accounts (webLoginVid may be numeric).
func decodeWereadLoginInfo(data []byte) (wereadLoginInfo, error) {
	var info wereadLoginInfo
	layers := make([]map[string]json.RawMessage, 0, 3)
	current := data
	for depth := 0; depth < 3; depth++ {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(current, &fields); err != nil {
			if depth == 0 {
				return info, err
			}
			break
		}
		layers = append(layers, fields)
		next, ok := fields["data"]
		if !ok {
			break
		}
		trimmed := bytes.TrimSpace(next)
		if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
			break
		}
		current = trimmed
	}
	get := func(name string) (json.RawMessage, bool) {
		for _, layer := range layers {
			value, ok := layer[name]
			if !ok {
				continue
			}
			trimmed := bytes.TrimSpace(value)
			if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
				continue
			}
			return trimmed, true
		}
		return nil, false
	}
	if value, ok := get("succeed"); ok {
		_ = json.Unmarshal(value, &info.Succeed)
	}
	readString := func(name string) flexString {
		value, ok := get(name)
		if !ok {
			return ""
		}
		var parsed flexString
		if err := json.Unmarshal(value, &parsed); err == nil {
			return parsed
		}
		return ""
	}
	info.LogicCode = readString("logicCode")
	info.WebLoginVid = readString("webLoginVid")
	info.Vid = readString("vid")
	info.UserVid = readString("userVid")
	info.AccessToken = readString("accessToken")
	info.RefreshToken = readString("refreshToken")
	return info, nil
}

// wereadJSONKeys returns top-level keys only, so poll diagnostics never log
// token values.
func wereadJSONKeys(data []byte) string {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return "unparseable"
	}
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	return strings.Join(keys, ",")
}

type wereadScanResponse struct {
	State   string `json:"state"`
	Message string `json:"message,omitempty"`
	Qrcode  string `json:"qrcode,omitempty"`
	ErrCode *int   `json:"errCode,omitempty"`
}

func (a *App) wereadCookie(ctx context.Context) (string, error) {
	encrypted, err := a.readSetting(ctx, settingWereadCookie)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(encrypted) == "" {
		return "", nil
	}
	plain, err := a.secret.Decrypt(strings.TrimSpace(encrypted))
	if err != nil {
		return "", fmt.Errorf("解密微信读书 Cookie 失败：%w", err)
	}
	return plain, nil
}

func wereadLoginClient() (*http.Client, *cookiejar.Jar, *url.URL, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, nil, nil, err
	}
	base, err := url.Parse("https://weread.qq.com/")
	if err != nil {
		return nil, nil, nil, err
	}
	return &http.Client{
		Timeout:   wereadStatusTimeout,
		Jar:       jar,
		Transport: http.DefaultTransport,
	}, jar, base, nil
}

func wereadSetInitialCookie(jar *cookiejar.Jar, fingerprint string) error {
	base, err := url.Parse("https://weread.qq.com/")
	if err != nil {
		return err
	}
	jar.SetCookies(base, []*http.Cookie{{Name: "wr_fp", Value: fingerprint, Path: "/"}})
	return nil
}

func wereadRandomFingerprint() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return "rill" + hex.EncodeToString(raw), nil
}

func wereadLoginHeaders(referer string) http.Header {
	headers := http.Header{}
	headers.Set("Accept", "application/json, text/plain, */*")
	headers.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	headers.Set("Origin", "https://weread.qq.com")
	headers.Set("Referer", referer)
	headers.Set("X-SSR-Request-Id", fmt.Sprintf("rill-%d", time.Now().UnixNano()))
	return headers
}

func decodeWereadEnvelope(data []byte, out any) error {
	var envelope struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return err
	}
	if envelope.Code != 0 {
		return fmt.Errorf("%s", envelope.Message)
	}
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		// Some WeRead endpoints return the payload directly instead of the
		// app envelope; accept both shapes.
		return json.Unmarshal(data, out)
	}
	return json.Unmarshal(envelope.Data, out)
}

// startWereadScan opens the current WeRead QR-login protocol and returns a PNG
// data URL. The UID is intentionally not returned to the client. Requesting a
// new QR while one is pending replaces it, so 「刷新二维码」 always works.
func (a *App) startWereadScan(ctx context.Context) (wereadScanResponse, error) {
	a.wereadLoginMu.Lock()
	defer a.wereadLoginMu.Unlock()

	fingerprint, err := wereadRandomFingerprint()
	if err != nil {
		return wereadScanResponse{}, internalError(err)
	}
	client, jar, _, err := wereadLoginClient()
	if err != nil {
		return wereadScanResponse{}, internalError(err)
	}
	if err := wereadSetInitialCookie(jar, fingerprint); err != nil {
		return wereadScanResponse{}, internalError(err)
	}

	pageRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, wereadSkillsURL, nil)
	if err != nil {
		return wereadScanResponse{}, internalError(err)
	}
	pageRequest.Header = wereadLoginHeaders("https://weread.qq.com/")
	pageResponse, err := client.Do(pageRequest)
	if err != nil {
		return wereadScanResponse{}, unavailable("weread_scan_unavailable", "无法打开微信读书登录页："+err.Error())
	}
	_, _ = io.Copy(io.Discard, pageResponse.Body)
	pageResponse.Body.Close()
	if pageResponse.StatusCode < 200 || pageResponse.StatusCode >= 300 {
		return wereadScanResponse{}, unavailable("weread_scan_unavailable", fmt.Sprintf("微信读书登录页返回 HTTP %d", pageResponse.StatusCode))
	}

	uidRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, wereadLoginUIDURL, nil)
	if err != nil {
		return wereadScanResponse{}, internalError(err)
	}
	uidRequest.Header = wereadLoginHeaders(wereadSkillsURL)
	uidResponse, err := client.Do(uidRequest)
	if err != nil {
		return wereadScanResponse{}, unavailable("weread_scan_unavailable", "获取微信读书登录 UID 失败："+err.Error())
	}
	defer uidResponse.Body.Close()
	data, err := io.ReadAll(io.LimitReader(uidResponse.Body, 1<<20))
	if err != nil {
		return wereadScanResponse{}, internalError(err)
	}
	if uidResponse.StatusCode < 200 || uidResponse.StatusCode >= 300 {
		return wereadScanResponse{}, unavailable("weread_scan_unavailable", fmt.Sprintf("获取微信读书登录 UID 失败：HTTP %d", uidResponse.StatusCode))
	}
	var payload struct {
		UID string `json:"uid"`
	}
	if err := decodeWereadEnvelope(data, &payload); err != nil {
		return wereadScanResponse{}, badRequest("weread_scan_invalid", "微信读书登录响应无法解析")
	}
	if strings.TrimSpace(payload.UID) == "" {
		return wereadScanResponse{}, unavailable("weread_scan_unavailable", "微信读书未返回有效登录标识")
	}

	confirmURL := "https://weread.qq.com/web/confirm?uid=" + url.QueryEscape(payload.UID)
	png, err := qrcode.Encode(confirmURL, qrcode.Medium, 320)
	if err != nil {
		return wereadScanResponse{}, internalError(err)
	}
	a.wereadLogin = &wereadLoginSession{
		UID:         payload.UID,
		Fingerprint: fingerprint,
		Client:      client,
		Jar:         jar,
		ExpiresAt:   a.now().Add(wereadLoginTTL),
	}
	return wereadScanResponse{
		State:  "waiting",
		Qrcode: "data:image/png;base64," + base64.StdEncoding.EncodeToString(png),
	}, nil
}

func (a *App) wereadJarCookieMap(session *wereadLoginSession) map[string]string {
	result := make(map[string]string)
	base, err := url.Parse("https://weread.qq.com/")
	if err != nil {
		return result
	}
	// 参考实现把所有 Set-Cookie 平铺进一个 Cookie 头；Go 的 jar 会按路径过滤，
	// 这里查多个常见路径并去重，避免丢 /web 或 /api 作用域的登录 Cookie。
	targets := []*url.URL{base}
	for _, path := range []string{"/web", "/web/mp", "/api"} {
		target := *base
		target.Path = path
		targets = append(targets, &target)
	}
	for _, target := range targets {
		for _, cookie := range session.Jar.Cookies(target) {
			if cookie.Name == "" || cookie.Value == "" {
				continue
			}
			if _, exists := result[cookie.Name]; !exists {
				result[cookie.Name] = cookie.Value
			}
		}
	}
	return result
}

func (a *App) saveWereadCookie(ctx context.Context, cookie string) error {
	cookie = strings.TrimSpace(cookie)
	if cookie == "" {
		return badRequest("invalid_weread_cookie", "微信读书 Cookie 为空")
	}
	if len(cookie) > wereadCookieMaxBytes {
		return badRequest("invalid_weread_cookie", "微信读书 Cookie 过大")
	}
	encrypted, err := a.secret.Encrypt(cookie)
	if err != nil {
		return internalError(err)
	}
	return a.writeSetting(ctx, settingWereadCookie, encrypted)
}

func (a *App) clearWereadLogin() {
	a.wereadLoginMu.Lock()
	a.wereadLogin = nil
	a.wereadLoginMu.Unlock()
}

func (a *App) pollWereadScan(ctx context.Context) (wereadScanResponse, error) {
	a.wereadLoginMu.Lock()
	defer a.wereadLoginMu.Unlock()
	if a.wereadLogin == nil {
		return wereadScanResponse{}, conflict("weread_scan_missing", "请先获取微信读书二维码")
	}
	if a.now().After(a.wereadLogin.ExpiresAt) {
		a.wereadLogin = nil
		return wereadScanResponse{State: "expired", Message: "二维码已过期，请重新获取"}, nil
	}
	session := a.wereadLogin
	base, _ := url.Parse("https://weread.qq.com/")
	loginURL := wereadLoginInfoURL + "?uid=" + url.QueryEscape(session.UID) + "&otp="
	// getLoginInfo is a long poll (websites hold it up to ~70s). Bound each
	// backend poll so 前端 3 秒轮询不会在锁上排队；确认事件由下一次短轮询接住。
	pollCtx, cancelPoll := context.WithTimeout(ctx, wereadPollBlockTimeout)
	defer cancelPoll()
	request, err := http.NewRequestWithContext(pollCtx, http.MethodGet, loginURL, nil)
	if err != nil {
		return wereadScanResponse{}, internalError(err)
	}
	request.Header = wereadLoginHeaders(wereadSkillsURL)
	response, err := session.Client.Do(request)
	if err != nil {
		// Transient network problems must not kill the pending QR login; the
		// TTL below is the real deadline.
		a.logger.Debug("weread scan poll request failed", "err", err)
		return wereadScanResponse{State: "waiting", Message: "正在等待扫码确认"}, nil
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		a.logger.Debug("weread scan poll read failed", "err", err)
		return wereadScanResponse{State: "waiting", Message: "正在等待扫码确认"}, nil
	}
	session.Jar.SetCookies(base, response.Cookies())
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		// WeRead occasionally answers the poll with 3xx/5xx while the login is
		// still pending. Keep waiting until the QR session expires instead of
		// stranding the user on an error.
		a.logger.Debug("weread scan poll http status", "status", response.StatusCode)
		return wereadScanResponse{State: "waiting", Message: "正在等待扫码确认"}, nil
	}
	info, decodeErr := decodeWereadLoginInfo(data)
	if decodeErr != nil {
		// Unrecognised payloads while not yet confirmed are treated the same
		// way: keep polling until the TTL ends the session.
		a.logger.Debug("weread scan poll payload not parseable yet", "err", decodeErr, "keys", wereadJSONKeys(data))
		return wereadScanResponse{State: "waiting", Message: "正在等待扫码确认"}, nil
	}
	if !bool(info.Succeed) {
		switch strings.ToUpper(string(info.LogicCode)) {
		case "", "PENDING", "0":
			return wereadScanResponse{State: "waiting", Message: "等待微信扫码"}, nil
		case "1", "SCANNED":
			return wereadScanResponse{State: "waiting", Message: "已扫码，请在手机上确认"}, nil
		case "LOGIN_TIMEOUT":
			// we-mp-rss / papers3-weread 实测：等待阶段的长轮询经常直接返回
			// LOGIN_TIMEOUT，这是正常等待状态，不能当作二维码过期；
			// 是否过期只由本地 TTL（wereadLoginTTL）决定。
			return wereadScanResponse{State: "waiting", Message: "正在等待扫码确认"}, nil
		case "NEED_OTP":
			a.wereadLogin = nil
			return wereadScanResponse{State: "error", Message: "微信读书要求手机验证码，当前不支持"}, nil
		case "OTP_EXPIRED", "OTP_NOT_MATCH":
			a.wereadLogin = nil
			return wereadScanResponse{State: "error", Message: "微信读书验证失败，请重新扫码"}, nil
		default:
			return wereadScanResponse{State: "waiting", Message: "微信读书登录确认中"}, nil
		}
	}

	vid := firstNonEmpty(string(info.WebLoginVid), string(info.Vid), string(info.UserVid))
	accessToken := strings.TrimSpace(string(info.AccessToken))
	refreshToken := strings.TrimSpace(string(info.RefreshToken))
	if vid == "" || (accessToken == "" && refreshToken == "") {
		a.wereadLogin = nil
		return wereadScanResponse{State: "error", Message: "微信读书登录响应缺少凭据"}, nil
	}

	// 保留 Set-Cookie 下发的原始 Cookie（wr_skey 短值也是有效令牌），
	// refreshToken 只作为候选兜底；每个候选都先过真实接口校验再落库。
	baseCookies := a.wereadJarCookieMap(session)
	baseCookies["wr_vid"] = vid
	cookie := a.verifyWereadCookieCandidates(ctx, baseCookies, accessToken, refreshToken)
	a.wereadLogin = nil
	if cookie == "" {
		return wereadScanResponse{State: "error", Message: "微信读书登录成功但 Cookie 验证失败，请重新扫码"}, nil
	}
	// wr_skey 是短时效票据；官方网页靠 wr_rt 定期调 /web/login/renewal 续期。
	// 无论最终选中哪个候选，都必须把 wr_rt 合并进落库 Cookie，否则后续无法续期。
	cookie = mergeWereadRefreshToken(cookie, refreshToken)
	if err := a.saveWereadCookie(ctx, cookie); err != nil {
		return wereadScanResponse{}, err
	}
	// 新登录已通过真实接口校验：清除验证码冷却，避免验证后仍显示「冷却中」。
	a.clearWereadCooldown()
	// 扫码成功即清掉公众号源上残留的「登录态已失效/需重新扫码」错误状态，
	// 并把刷新时间提前到现在，让调度器尽快用新登录态重抓；否则源列表会一直
	// 显示「需重新扫码」，直到下一轮 30 分钟调度成功为止。
	a.recoverWechatSources(ctx)
	// 新 Cookie 的 wr_rt 是新鲜的：从现在起每 30 分钟主动续期一次。
	a.markWereadKeepAlive()
	return wereadScanResponse{State: "done", Message: "微信读书登录成功"}, nil
}

// verifyWereadCookieCandidates tries the cookie shapes seen in the wild and
// returns the first one that passes a real WeRead business request. The order
// matches the reference implementation: raw Set-Cookie jar first, then token
// variants, then a renewal-refreshed cookie.
func (a *App) verifyWereadCookieCandidates(ctx context.Context, base map[string]string, accessToken, refreshToken string) string {
	candidates := wereadCookieCandidates(base, accessToken, refreshToken)
	for _, candidate := range candidates {
		if a.wereadCookieValid(ctx, candidate) {
			return candidate
		}
	}
	for _, candidate := range candidates {
		renewed, _ := a.renewWereadCookie(ctx, candidate)
		if renewed == "" {
			continue
		}
		if a.wereadCookieValid(ctx, renewed) {
			return renewed
		}
	}
	return ""
}

func wereadCookieCandidates(base map[string]string, accessToken, refreshToken string) []string {
	build := func(overrides map[string]string) string {
		cookies := make(map[string]string, len(base)+len(overrides))
		for key, value := range base {
			cookies[key] = value
		}
		for key, value := range overrides {
			if value != "" {
				cookies[key] = value
			}
		}
		return wereadCookieString(cookies)
	}
	raw := map[string]string{}
	if strings.TrimSpace(base["wr_skey"]) == "" && accessToken != "" {
		// 没有 Set-Cookie wr_skey 时用短令牌补齐。
		raw["wr_skey"] = accessToken
	}
	candidates := []string{build(raw)}
	if accessToken != "" {
		candidates = append(candidates, build(map[string]string{"wr_skey": accessToken}))
	}
	if refreshToken != "" {
		encodedRT := url.QueryEscape(refreshToken)
		candidates = append(candidates,
			build(map[string]string{"wr_skey": refreshToken}),
			build(map[string]string{"wr_skey": refreshToken, "wr_rt": encodedRT}),
			build(map[string]string{"wr_rt": encodedRT}),
		)
	}
	seen := make(map[string]bool)
	result := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate == "" || seen[candidate] {
			continue
		}
		seen[candidate] = true
		result = append(result, candidate)
	}
	return result
}

// wereadCookieValid performs one real business request. /web/shelf/sync with an
// empty userVid is the verification endpoint used by the reference
// implementation; it is much less likely to trigger the -2041 captcha than
// /web/mp/articles.
func (a *App) wereadCookieValid(ctx context.Context, cookie string) bool {
	verifyCtx, cancel := context.WithTimeout(ctx, wereadVerifyTimeout)
	defer cancel()
	_, data, err := a.wereadDo(verifyCtx, http.MethodGet, wereadBaseURL+"/web/shelf/sync?userVid=&synckey=0", cookie, nil, "")
	if err != nil {
		return false
	}
	var payload struct {
		ErrCode   int             `json:"errCode"`
		ErrCode2  int             `json:"errcode"`
		Books     json.RawMessage `json:"books"`
		BookCount json.RawMessage `json:"bookCount"`
		Synckey   json.RawMessage `json:"synckey"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return false
	}
	if payload.ErrCode != 0 || payload.ErrCode2 != 0 {
		return false
	}
	return len(payload.Books) > 0 || len(payload.BookCount) > 0 || len(payload.Synckey) > 0
}

// renewWereadCookie exchanges wr_rt for a fresh wr_skey. The returned detail
// string carries the HTTP status / body errCode / Set-Cookie count for logs;
// never log cookie values themselves.
//
// 2026-11-09 真机实测：当前 renewal 接口对 ql=true 返回 200 +
// {"errCode":-2013,"errMsg":"鉴权失败"} 并下发 4 个空值 Set-Cookie（清场），
// 即使 wr_rt 完全有效；ql=false（或省略 ql）才返回 {"succ":1} 和新
// wr_vid/wr_skey/wr_pf/wr_rt。官方网页与 weread.koplugin 都使用 ql=false。
func (a *App) renewWereadCookie(ctx context.Context, cookie string) (string, string) {
	renewCtx, cancel := context.WithTimeout(ctx, wereadVerifyTimeout)
	defer cancel()
	payload := strings.NewReader(`{"rq":"%2Fweb%2Fbook%2Fread","ql":false}`)
	response, data, err := a.wereadDo(renewCtx, http.MethodPost, wereadRenewalURL, cookie, payload, "application/json;charset=UTF-8")
	if err != nil || response == nil {
		if err == errWereadAuthRequired {
			return "", "renewal HTTP 401"
		}
		if err != nil {
			return "", fmt.Sprintf("renewal request error: %v", err)
		}
		return "", "renewal response missing"
	}
	detail := fmt.Sprintf("renewal HTTP %d", response.StatusCode)
	if len(data) > 0 && len(data) < 4096 {
		// body 形如 {"errCode":-2013,"errMsg":"鉴权失败"}，不含凭据，可安全记录。
		detail += " body=" + strings.TrimSpace(string(data))
	}
	cookies := response.Cookies()
	detail += fmt.Sprintf(" set-cookie=%d", len(cookies))
	merged := wereadCookieMap(cookie)
	for _, item := range cookies {
		// 无效凭据时服务端会下发「空值 Cookie」清场（实测 -2013），
		// 必须跳过空值，避免把存量凭据抹掉。
		if item.Name != "" && item.Value != "" {
			merged[item.Name] = item.Value
		}
	}
	return wereadCookieString(merged), detail
}

// mergeWereadRefreshToken makes sure the persisted cookie carries wr_rt so the
// runtime renewal path can exchange it for a fresh wr_skey later.
func mergeWereadRefreshToken(cookie, refreshToken string) string {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" || cookie == "" {
		return cookie
	}
	cookies := wereadCookieMap(cookie)
	cookies["wr_rt"] = url.QueryEscape(refreshToken)
	return wereadCookieString(cookies)
}

// renewWereadSession exchanges the stored wr_rt for a fresh wr_skey and
// persists it. It returns false when the stored cookie has no wr_rt (older
// scans before this fix), the renewal endpoint refuses, or the renewed cookie
// fails validation — in those cases the caller surfaces 「请重新扫码连接」.
func (a *App) renewWereadSession(ctx context.Context) bool {
	cookie, err := a.wereadCookie(ctx)
	if err != nil || strings.TrimSpace(cookie) == "" {
		a.logger.Warn("weread renewal skipped: no stored cookie", "err", err)
		return false
	}
	if !strings.Contains(wereadCookiesHeader(cookie), "wr_rt=") {
		a.logger.Warn("weread renewal skipped: stored cookie has no wr_rt; rescan required on the new build")
		return false
	}
	renewCtx, cancel := context.WithTimeout(ctx, wereadVerifyTimeout)
	defer cancel()
	renewed, detail := a.renewWereadCookie(renewCtx, cookie)
	if renewed == "" || renewed == cookie {
		// 实测：wr_rt 被服务端作废时返回 200 + {"errCode":-2013 鉴权失败} +
		// 全空值 Set-Cookie（合并后等于原 Cookie）。此时只能重新扫码。
		a.logger.Warn("weread renewal failed: no usable new cookies", "detail", detail)
		return false
	}
	if !a.wereadCookieValid(renewCtx, renewed) {
		a.logger.Warn("weread renewal failed: renewed cookie failed validation", "detail", detail)
		return false
	}
	if err := a.saveWereadCookie(ctx, renewed); err != nil {
		a.logger.Warn("weread renewed cookie save failed", "err", err)
		return false
	}
	// 续期成功说明登录态已恢复：清掉 -2041 风控冷却，避免误报「冷却中」。
	a.clearWereadCooldown()
	a.markWereadKeepAlive()
	a.logger.Info("weread session renewed via wr_rt", "detail", detail)
	return true
}

// markWereadKeepAlive 记录一次成功的续期/新登录，重置主动续期计时。
func (a *App) markWereadKeepAlive() {
	a.wereadKeepAliveMu.Lock()
	a.wereadLastKeepAlive = a.now()
	a.wereadKeepAliveMu.Unlock()
}

const wereadKeepAliveInterval = 30 * time.Minute

// maybeWereadKeepAlive 由调度器每 30 秒调用一次；距上次续期超过
// wereadKeepAliveInterval 才真正发请求。wr_rt 长时间不用会被微信读书
// 服务端作废（-2013），所以必须像官方网页一样持续换票，而不是等失败补救。
func (a *App) maybeWereadKeepAlive(ctx context.Context) {
	a.wereadKeepAliveMu.Lock()
	last := a.wereadLastKeepAlive
	a.wereadKeepAliveMu.Unlock()
	if !last.IsZero() && a.now().Sub(last) < wereadKeepAliveInterval {
		return
	}
	cookie, err := a.wereadCookie(ctx)
	if err != nil || strings.TrimSpace(cookie) == "" {
		// 未连接微信读书：不计时、不告警，保持静默。
		if err == nil {
			a.markWereadKeepAlive()
		}
		return
	}
	a.markWereadKeepAlive()
	a.renewWereadSession(ctx)
}

func wereadCookieMap(cookie string) map[string]string {
	result := make(map[string]string)
	for _, part := range strings.Split(cookie, ";") {
		name, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok || name == "" || value == "" {
			continue
		}
		result[name] = value
	}
	return result
}

func wereadCookieString(cookies map[string]string) string {
	names := make([]string, 0, len(cookies))
	for name, value := range cookies {
		if name == "" || value == "" {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	parts := make([]string, 0, len(names))
	for _, name := range names {
		parts = append(parts, name+"="+cookies[name])
	}
	return strings.Join(parts, "; ")
}

func (a *App) wereadCookieConfigured(ctx context.Context) bool {
	cookie, err := a.wereadCookie(ctx)
	return err == nil && strings.TrimSpace(cookie) != ""
}

// wereadCookieHasRefreshToken reports whether the stored cookie carries wr_rt;
// without it the runtime renewal path cannot work and the user must re-scan.
func (a *App) wereadCookieHasRefreshToken(ctx context.Context) bool {
	cookie, err := a.wereadCookie(ctx)
	if err != nil {
		return false
	}
	return strings.Contains(wereadCookiesHeader(cookie), "wr_rt=")
}

func (a *App) wereadCooldownRemaining() time.Duration {
	a.wereadCooldownLock.Lock()
	defer a.wereadCooldownLock.Unlock()
	if a.wereadCooldownAt.IsZero() {
		return 0
	}
	remaining := a.wereadCooldownAt.Sub(a.now())
	if remaining <= 0 {
		a.wereadCooldownAt = time.Time{}
		return 0
	}
	return remaining
}

func (a *App) clearWereadCooldown() {
	a.wereadCooldownLock.Lock()
	a.wereadCooldownAt = time.Time{}
	a.wereadCooldownLock.Unlock()
}

func (a *App) startWereadCooldown() {
	a.wereadCooldownLock.Lock()
	defer a.wereadCooldownLock.Unlock()
	until := a.now().Add(wereadCooldownDuration)
	if until.After(a.wereadCooldownAt) {
		a.wereadCooldownAt = until
	}
}

func (a *App) handleWereadScan(w http.ResponseWriter, r *http.Request) error {
	response, err := a.startWereadScan(r.Context())
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, response)
	return nil
}

func (a *App) handleWereadScanStatus(w http.ResponseWriter, r *http.Request) error {
	response, err := a.pollWereadScan(r.Context())
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, response)
	return nil
}
