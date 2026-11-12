package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"rill/internal/wechat"
)

// REQ-034 / DEC-024: the WeRead QR login is the only channel left for
// official-account sources. Article-link resolution reads the public page
// without any login; creating a source requires a connected WeRead login.

func TestWechatResolveArticlePublic(t *testing.T) {
	mp := httptestArticleServer(t)
	oldMP := wechat.MPBaseURL
	oldHost := wechatArticleHost
	wechat.MPBaseURL = mp.URL
	wechatArticleHost = "127.0.0.1"
	t.Cleanup(func() {
		wechat.MPBaseURL = oldMP
		wechatArticleHost = oldHost
	})

	app, client := newTestApp(t, false)
	client = client.login()
	client.decode(http.MethodPatch, "/api/v1/settings", map[string]any{
		"wechat": map[string]any{"enabled": true},
	}, true, http.StatusOK, new(Settings))

	var payload struct {
		Account wechat.Account `json:"account"`
	}
	client.decode(http.MethodPost, "/api/v1/wechat/resolve", map[string]any{
		"url": mp.URL + "/s/test?__biz=MzUxNzAzMTU4OQ==",
	}, true, http.StatusOK, &payload)
	if payload.Account.FakeID != "MzUxNzAzMTU4OQ==" || payload.Account.Name != "二哥狗腿子" {
		t.Fatalf("resolved account: %+v", payload.Account)
	}

	// 无需后台登录即可解析：不保存任何微信后台会话。
	if _, err := app.readSetting(context.Background(), settingWereadCookie); err == nil {
		t.Fatal("resolve should not persist any weread state")
	}

	// 非公众号链接拒绝。
	response, data := client.request(http.MethodPost, "/api/v1/wechat/resolve", map[string]any{
		"url": "https://example.com/s/x",
	}, true)
	if response.StatusCode != http.StatusBadRequest || errorCode(t, data) != "invalid_wechat_url" {
		t.Fatalf("non-wechat url: %d %s", response.StatusCode, data)
	}
}

func TestWechatCreateSourceRequiresWeread(t *testing.T) {
	mp := httptestArticleServer(t)
	oldMP := wechat.MPBaseURL
	wechat.MPBaseURL = mp.URL
	t.Cleanup(func() { wechat.MPBaseURL = oldMP })

	_, client := newTestApp(t, false)
	client = client.login()
	client.decode(http.MethodPatch, "/api/v1/settings", map[string]any{
		"wechat": map[string]any{"enabled": true},
	}, true, http.StatusOK, new(Settings))

	response, data := client.request(http.MethodPost, "/api/v1/wechat/sources", map[string]any{
		"account": "MzUxNzAzMTU4OQ==",
		"name":    "二哥狗腿子",
	}, true)
	if response.StatusCode != http.StatusUnauthorized || errorCode(t, data) != "wechat_auth_required" {
		t.Fatalf("create without weread: %d %s", response.StatusCode, data)
	}

	if err := client.app.saveWereadCookie(context.Background(), "wr_vid=test; wr_skey=test"); err != nil {
		t.Fatalf("save weread cookie: %v", err)
	}
	var source Source
	client.decode(http.MethodPost, "/api/v1/wechat/sources", map[string]any{
		"account": "MzUxNzAzMTU4OQ==",
		"name":    "二哥狗腿子",
	}, true, http.StatusCreated, &source)
	if source.Kind != "wechat" || !strings.HasPrefix(source.FeedURL, "wechat://MzUxNzAzMTU4OQ==") {
		t.Fatalf("created source: %+v", source)
	}
}

func httptestArticleServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html>
<html><body>
<script>var biz = "MzUxNzAzMTU4OQ==", __M = {}; </script>
<script>var round_head_img = "https://mmbiz.qpic.test/avatar";</script>
<h1 id="activity-name">测试文章</h1>
<a id="js_name">二哥狗腿子</a>
<div id="js_content"><p>正文</p></div>
</body></html>`))
	}))
}
