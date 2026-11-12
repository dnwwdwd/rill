package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func wereadScanServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/r/weread-skills":
			http.SetCookie(w, &http.Cookie{Name: "wr_gid", Value: "test-gid", Path: "/"})
			_, _ = w.Write([]byte(`<!doctype html><html><body>skills</body></html>`))
		case r.URL.Path == "/api/auth/getLoginUid":
			_, _ = w.Write([]byte(`{"uid":"test-uid"}`))
		case r.URL.Path == "/api/auth/getLoginInfo":
			http.SetCookie(w, &http.Cookie{Name: "wr_skey", Value: "raw-short-key", Path: "/"})
			_, _ = w.Write([]byte(`{"succeed":true,"webLoginVid":"test-vid","accessToken":"test-access","refreshToken":"test-refresh"}`))
		case r.URL.Path == "/web/shelf/sync":
			_, _ = w.Write([]byte(`{"errCode":0,"synckey":0,"bookCount":1}`))
		case r.URL.Path == "/web/login/renewal":
			http.SetCookie(w, &http.Cookie{Name: "wr_rt", Value: "renewed", Path: "/"})
			_, _ = w.Write([]byte(`{"succ":1}`))
		default:
			http.NotFound(w, r)
		}
	}))
}

func TestWereadScanLoginStoresEncryptedCookie(t *testing.T) {
	server := wereadScanServer(t)
	defer server.Close()
	oldBase := wereadBaseURL
	oldSkills := wereadSkillsURL
	oldUID := wereadLoginUIDURL
	oldInfo := wereadLoginInfoURL
	oldRenewal := wereadRenewalURL
	wereadBaseURL = server.URL
	wereadSkillsURL = server.URL + "/r/weread-skills"
	wereadLoginUIDURL = server.URL + "/api/auth/getLoginUid"
	wereadLoginInfoURL = server.URL + "/api/auth/getLoginInfo"
	wereadRenewalURL = server.URL + "/web/login/renewal"
	t.Cleanup(func() {
		wereadBaseURL = oldBase
		wereadSkillsURL = oldSkills
		wereadLoginUIDURL = oldUID
		wereadLoginInfoURL = oldInfo
		wereadRenewalURL = oldRenewal
	})

	app, client := newTestApp(t, false)
	client = client.login()
	// 模拟验证码冷却中；重新扫码成功应清除冷却。
	app.startWereadCooldown()
	var scan wereadScanResponse
	client.decode(http.MethodPost, "/api/v1/wechat/weread/scan", nil, true, http.StatusOK, &scan)
	if scan.State != "waiting" || !strings.HasPrefix(scan.Qrcode, "data:image/png;base64,") {
		t.Fatalf("scan response: %+v", scan)
	}

	var status wereadScanResponse
	client.decode(http.MethodGet, "/api/v1/wechat/weread/scan/status", nil, false, http.StatusOK, &status)
	if status.State != "done" {
		t.Fatalf("scan status: %+v", status)
	}

	// REQ-034：扫码成功后设置必须立即反映 weread_connected，前端才不会停在扫码界面。
	var settings Settings
	client.decode(http.MethodGet, "/api/v1/settings", nil, false, http.StatusOK, &settings)
	if !settings.Wechat.WereadConnected {
		t.Fatalf("weread_connected not reflected after scan: %+v", settings.Wechat)
	}

	saved, err := app.readSetting(context.Background(), settingWereadCookie)
	if err != nil {
		t.Fatalf("read cookie setting: %v", err)
	}
	if strings.Contains(saved, "test-access") || strings.Contains(saved, "wr_skey") {
		t.Fatal("WeRead cookie stored in plaintext")
	}
	cookie, err := app.wereadCookie(context.Background())
	if err != nil {
		t.Fatalf("read weread cookie: %v", err)
	}
	// 校验通过后才落库；原始 Set-Cookie 的短 wr_skey 优先保留。
	if !strings.Contains(cookie, "wr_skey=raw-short-key") || !strings.Contains(cookie, "wr_vid=test-vid") {
		t.Fatalf("verified cookie = %q", cookie)
	}
	if !app.wereadCookieConfigured(context.Background()) {
		t.Fatal("WeRead cookie should be configured")
	}
	if app.wereadCooldownRemaining() > 0 {
		t.Fatal("successful re-login should clear the captcha cooldown")
	}
}

// 真实 getLoginInfo 响应（2026-11-03 真机）：webLoginVid 是数字、字段可能在
// data 内层、业务 Cookie 的 wr_skey 必须用长令牌 refreshToken。
func TestWereadScanLoginRealisticPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/r/weread-skills":
			_, _ = w.Write([]byte(`<!doctype html><html><body>skills</body></html>`))
		case r.URL.Path == "/api/auth/getLoginUid":
			_, _ = w.Write([]byte(`{"uid":"real-uid"}`))
		case r.URL.Path == "/api/auth/getLoginInfo":
			// 数字 webLoginVid + data 信封 + 顶层无 succeed。
			_, _ = w.Write([]byte(`{"code":0,"data":{"succeed":true,"webLoginVid":123456789,"accessToken":"abc12345","refreshToken":"web@long-refresh-token"}}`))
		case r.URL.Path == "/web/shelf/sync":
			// 真实场景：短令牌被服务端以 -2012 拒绝，只有长令牌能通过校验。
			if strings.Contains(r.Header.Get("Cookie"), "wr_skey=web@long-refresh-token") {
				_, _ = w.Write([]byte(`{"errCode":0,"synckey":0,"bookCount":1}`))
				return
			}
			_, _ = w.Write([]byte(`{"errCode":-2012,"errMsg":"login timeout"}`))
		case r.URL.Path == "/web/login/renewal":
			_, _ = w.Write([]byte(`{"succ":1}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	oldBase := wereadBaseURL
	oldSkills := wereadSkillsURL
	oldUID := wereadLoginUIDURL
	oldInfo := wereadLoginInfoURL
	oldRenewal := wereadRenewalURL
	wereadBaseURL = server.URL
	wereadSkillsURL = server.URL + "/r/weread-skills"
	wereadLoginUIDURL = server.URL + "/api/auth/getLoginUid"
	wereadLoginInfoURL = server.URL + "/api/auth/getLoginInfo"
	wereadRenewalURL = server.URL + "/web/login/renewal"
	t.Cleanup(func() {
		wereadBaseURL = oldBase
		wereadSkillsURL = oldSkills
		wereadLoginUIDURL = oldUID
		wereadLoginInfoURL = oldInfo
		wereadRenewalURL = oldRenewal
	})

	app, client := newTestApp(t, false)
	client = client.login()
	var scan wereadScanResponse
	client.decode(http.MethodPost, "/api/v1/wechat/weread/scan", nil, true, http.StatusOK, &scan)
	var status wereadScanResponse
	client.decode(http.MethodGet, "/api/v1/wechat/weread/scan/status", nil, false, http.StatusOK, &status)
	if status.State != "done" {
		t.Fatalf("realistic scan status: %+v", status)
	}
	cookie, err := app.wereadCookie(context.Background())
	if err != nil {
		t.Fatalf("read weread cookie: %v", err)
	}
	if !strings.Contains(cookie, "wr_vid=123456789") {
		t.Fatalf("cookie missing numeric vid: %q", cookie)
	}
	// 短令牌被 -2012 拒绝后，应回退到 refreshToken 并通过校验。
	if !strings.Contains(cookie, "wr_skey=web@long-refresh-token") {
		t.Fatalf("wr_skey must fall back to the long refresh token: %q", cookie)
	}
}

func TestWereadPollLoginTimeoutKeepsWaiting(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/r/weread-skills":
			_, _ = w.Write([]byte(`<!doctype html><html><body>skills</body></html>`))
		case r.URL.Path == "/api/auth/getLoginUid":
			_, _ = w.Write([]byte(`{"uid":"timeout-uid"}`))
		case r.URL.Path == "/api/auth/getLoginInfo":
			// 等待阶段的长轮询正常返回 LOGIN_TIMEOUT，不能当二维码过期。
			_, _ = w.Write([]byte(`{"succeed":false,"logicCode":"LOGIN_TIMEOUT"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	oldBase := wereadBaseURL
	oldSkills := wereadSkillsURL
	oldUID := wereadLoginUIDURL
	oldInfo := wereadLoginInfoURL
	wereadBaseURL = server.URL
	wereadSkillsURL = server.URL + "/r/weread-skills"
	wereadLoginUIDURL = server.URL + "/api/auth/getLoginUid"
	wereadLoginInfoURL = server.URL + "/api/auth/getLoginInfo"
	t.Cleanup(func() {
		wereadBaseURL = oldBase
		wereadSkillsURL = oldSkills
		wereadLoginUIDURL = oldUID
		wereadLoginInfoURL = oldInfo
	})

	_, client := newTestApp(t, false)
	client = client.login()
	var scan wereadScanResponse
	client.decode(http.MethodPost, "/api/v1/wechat/weread/scan", nil, true, http.StatusOK, &scan)
	var status wereadScanResponse
	client.decode(http.MethodGet, "/api/v1/wechat/weread/scan/status", nil, false, http.StatusOK, &status)
	if status.State != "waiting" {
		t.Fatalf("LOGIN_TIMEOUT should keep waiting, got %+v", status)
	}
}

func TestWereadLoginInfoShapes(t *testing.T) {
	cases := []struct {
		name      string
		payload   string
		succeed   bool
		logicCode string
		vid       string
	}{
		{
			name:    "nested numeric vid",
			payload: `{"code":0,"data":{"succeed":true,"webLoginVid":987654321,"refreshToken":"web@rt"}}`,
			succeed: true,
			vid:     "987654321",
		},
		{
			name:    "top-level succeed with nested data",
			payload: `{"succeed":true,"webLoginVid":"10086","data":{"accessToken":"short123"}}`,
			succeed: true,
			vid:     "10086",
		},
		{
			name:      "pending",
			payload:   `{"succeed":false,"logicCode":"PENDING"}`,
			logicCode: "PENDING",
		},
		{
			name:      "scanned",
			payload:   `{"succeed":false,"logicCode":"SCANNED"}`,
			logicCode: "SCANNED",
		},
		{
			name:    "numeric succeed",
			payload: `{"succeed":1,"vid":"42","accessToken":"abc"}`,
			succeed: true,
			vid:     "42",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			info, err := decodeWereadLoginInfo([]byte(tc.payload))
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			if bool(info.Succeed) != tc.succeed {
				t.Fatalf("succeed = %v, want %v", bool(info.Succeed), tc.succeed)
			}
			if tc.logicCode != "" && string(info.LogicCode) != tc.logicCode {
				t.Fatalf("logicCode = %q, want %q", info.LogicCode, tc.logicCode)
			}
			vid := firstNonEmpty(string(info.WebLoginVid), string(info.Vid), string(info.UserVid))
			if tc.vid != "" && vid != tc.vid {
				t.Fatalf("vid = %q, want %q", vid, tc.vid)
			}
		})
	}
}

func TestWereadBookID(t *testing.T) {
	bookID, err := wereadBookID("MzUxNzAzMTU4OQ==")
	if err != nil {
		t.Fatalf("wereadBookID: %v", err)
	}
	if bookID != "MP_WXS_3517031589" {
		t.Fatalf("bookID = %q", bookID)
	}
	if _, err := wereadBookID("not-base64!!"); err == nil {
		t.Fatal("invalid fakeid accepted")
	}
}

func TestWechatSourceFetchesViaWeread(t *testing.T) {
	weread := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/mp/shelf/addToShelf":
			_, _ = w.Write([]byte(`{"errCode":0}`))
		case r.URL.Path == "/web/mp/articles":
			_, _ = w.Write([]byte(`{"errCode":0,"reviews":[{"createTime":1700000000,"subReviews":[{"review":{"reviewId":"MP_WXS_3517031589_test","createTime":1700000000,"mpInfo":{"title":"二哥狗腿子测试文章","originalId":""}}}]}]}`))
		case r.URL.Path == "/web/mp/content":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(`<!doctype html><meta property="og:url" content="https://mp.weixin.qq.com/s/test"><div id="js_content"><p>微信读书正文</p></div>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer weread.Close()

	oldBase := wereadBaseURL
	oldShelf := wereadShelfURL
	oldArticles := wereadArticlesURL
	oldContentFmt := wereadContentURLFmt
	wereadBaseURL = weread.URL
	wereadShelfURL = weread.URL + "/mp/shelf/addToShelf"
	wereadArticlesURL = weread.URL + "/web/mp/articles"
	wereadContentURLFmt = weread.URL + "/web/mp/content?reviewId=%s"
	t.Cleanup(func() {
		wereadBaseURL = oldBase
		wereadShelfURL = oldShelf
		wereadArticlesURL = oldArticles
		wereadContentURLFmt = oldContentFmt
	})

	app, client := newTestApp(t, false)
	client = client.login()
	client.decode(http.MethodPatch, "/api/v1/settings", map[string]any{
		"wechat": map[string]any{"enabled": true},
	}, true, http.StatusOK, new(Settings))
	if err := app.saveWereadCookie(context.Background(), "wr_vid=test; wr_skey=test; wr_gid=test"); err != nil {
		t.Fatalf("save weread cookie: %v", err)
	}
	source, err := app.createSourceRow(context.Background(), sourceCreateInput{
		Kind:    "wechat",
		Name:    "二哥狗腿子",
		FeedURL: "wechat://MzUxNzAzMTU4OQ==",
	})
	if err != nil {
		t.Fatalf("create source: %v", err)
	}
	if _, err := app.fetchSource(context.Background(), source.ID, false, ""); err != nil {
		t.Fatalf("fetch source: %v", err)
	}
	updated, err := app.getSource(context.Background(), source.ID)
	if err != nil {
		t.Fatalf("get source: %v", err)
	}
	if updated.LastStatus == nil || *updated.LastStatus != "ok" || updated.ItemCount != 1 {
		t.Fatalf("updated source: %+v", updated)
	}
}

func TestWereadListBlockedFallsBackToCover(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/mp/shelf/addToShelf":
			_, _ = w.Write([]byte(`{"errCode":0}`))
		case "/web/mp/articles":
			_, _ = w.Write([]byte(`{"errCode":-2041,"errMsg":"risk control"}`))
		case "/api/mp/cover":
			_, _ = w.Write([]byte(`{"errCode":0,"reviewId":"MP_WXS_3517031589_cover1","title":"最新封面文章","time":1700000100}`))
		case "/web/mp/content":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(`<!doctype html><meta property="og:url" content="https://mp.weixin.qq.com/s/cover1"><div id="js_content"><p>封面正文</p></div>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	oldBase := wereadBaseURL
	oldShelf := wereadShelfURL
	oldArticles := wereadArticlesURL
	oldCover := wereadCoverURL
	oldContentFmt := wereadContentURLFmt
	wereadBaseURL = server.URL
	wereadShelfURL = server.URL + "/mp/shelf/addToShelf"
	wereadArticlesURL = server.URL + "/web/mp/articles"
	wereadCoverURL = server.URL + "/api/mp/cover"
	wereadContentURLFmt = server.URL + "/web/mp/content?reviewId=%s"
	t.Cleanup(func() {
		wereadBaseURL = oldBase
		wereadShelfURL = oldShelf
		wereadArticlesURL = oldArticles
		wereadCoverURL = oldCover
		wereadContentURLFmt = oldContentFmt
	})

	app, _ := newTestApp(t, false)
	if err := app.saveWereadCookie(context.Background(), "wr_vid=test; wr_skey=test"); err != nil {
		t.Fatalf("save cookie: %v", err)
	}
	source := dbSource{Source: Source{ID: 1, Name: "外部号", FeedURL: "wechat://MzUxNzAzMTU4OQ=="}}
	result, err := app.fetchWeread(context.Background(), source)
	if err != nil {
		t.Fatalf("fetchWeread should fall back to cover: %v", err)
	}
	if len(result.Items) != 1 || result.Items[0].Title != "最新封面文章" {
		t.Fatalf("cover fallback items: %+v", result.Items)
	}
	if app.wereadCooldownRemaining() > 0 {
		t.Fatal("cover fallback should not start the channel cooldown")
	}
}

func TestWereadCaptchaStartsCooldown(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/mp/shelf/addToShelf" {
			_, _ = w.Write([]byte(`{"errCode":0}`))
			return
		}
		_, _ = w.Write([]byte(`{"errCode":-2041,"errMsg":"captcha"}`))
	}))
	defer server.Close()
	oldBase := wereadBaseURL
	oldShelf := wereadShelfURL
	oldArticles := wereadArticlesURL
	oldCover := wereadCoverURL
	wereadBaseURL = server.URL
	wereadShelfURL = server.URL + "/mp/shelf/addToShelf"
	wereadArticlesURL = server.URL + "/web/mp/articles"
	wereadCoverURL = server.URL + "/api/mp/cover"
	t.Cleanup(func() {
		wereadBaseURL = oldBase
		wereadShelfURL = oldShelf
		wereadArticlesURL = oldArticles
		wereadCoverURL = oldCover
	})

	app, _ := newTestApp(t, false)
	if err := app.saveWereadCookie(context.Background(), "wr_vid=test; wr_skey=test"); err != nil {
		t.Fatalf("save cookie: %v", err)
	}
	source := dbSource{Source: Source{ID: 1, Name: "外部号", FeedURL: "wechat://MzUxNzAzMTU4OQ=="}}
	_, err := app.fetchWeread(context.Background(), source)
	if !errors.Is(err, errWereadCaptcha) {
		t.Fatalf("fetchWeread error = %v, want captcha", err)
	}
	if app.wereadCooldownRemaining() <= 0 {
		t.Fatal("captcha did not start cooldown")
	}
	if _, err := app.fetchWeread(context.Background(), source); !errors.Is(err, errWereadCooldown) {
		t.Fatalf("second fetch error = %v, want cooldown", err)
	}
	// 列表 -2041 后会再探测一次 cover（同样 -2041），共 shelf/articles/cover 3 次；
	// 冷却期内第二次抓取不再发请求。
	if requests.Load() != 3 {
		t.Fatalf("cooldown made extra requests: %d", requests.Load())
	}
}
