package server

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"rill/internal/fetch"
)

const candidateIconPNG = "\x89PNG\r\n\x1a\n0000"

// 候选头像代理：resolve/搜索登记的图标 URL 必须能通过 /api/v1/media/candidate
// 取回，且未登记的 hash 一律 404（不允许用该端点抓任意 URL）。
func TestCandidateIconProxyRoundTrip(t *testing.T) {
	imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/avatar.jpg" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte(candidateIconPNG))
	}))
	defer imageServer.Close()

	app, client := newTestApp(t, false)
	client = client.login()
	iconURL := imageServer.URL + "/avatar.jpg"
	if err := app.prepareCandidateIcons(context.Background(), []string{iconURL}); err != nil {
		t.Fatalf("prepareCandidateIcons: %v", err)
	}
	hash := fetch.URLHash(iconURL)

	response, data := client.raw(http.MethodGet, "/api/v1/media/candidate/"+hash, nil, "", "")
	if response.StatusCode != http.StatusOK {
		t.Fatalf("proxy icon status %d: %s", response.StatusCode, data)
	}
	if contentType := response.Header.Get("Content-Type"); contentType != "image/png" {
		t.Fatalf("content type %q", contentType)
	}
	if len(data) != len(candidateIconPNG) {
		t.Fatalf("body size %d, want %d", len(data), len(candidateIconPNG))
	}

	response, _ = client.raw(http.MethodGet, "/api/v1/media/candidate/"+strings.Repeat("ab", 32), nil, "", "")
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("unregistered hash status %d, want 404", response.StatusCode)
	}

	response, _ = client.raw(http.MethodGet, "/api/v1/media/candidate/not-a-hash", nil, "", "")
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("invalid hash status %d, want 404", response.StatusCode)
	}
}

// Apple 候选头像：icon_url 保留原始 URL（用于创建源），icon_proxy_url 指向代理。
func TestResolveAppleCandidateIconProxy(t *testing.T) {
	imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/cover.jpg" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte(candidateIconPNG))
	}))
	defer imageServer.Close()

	apple := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/lookup" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"resultCount":1,"results":[{"collectionName":"Darknet Diaries","feedUrl":"https://podcast.darknetdiaries.example","collectionViewUrl":"https://podcasts.apple.com/podcast/id1296350485","artworkUrl600":"` + imageServer.URL + `/cover.jpg"}]}`))
	}))
	defer apple.Close()

	app, client := newTestApp(t, false)
	app.appleAPIBase = apple.URL
	client = client.login()

	var resolved struct {
		Candidates []ResolveCandidate `json:"candidates"`
	}
	client.decode(http.MethodPost, "/api/v1/sources/resolve", map[string]any{"input": "apple:1296350485"}, true, http.StatusOK, &resolved)
	if len(resolved.Candidates) != 1 {
		t.Fatalf("candidates: %+v", resolved.Candidates)
	}
	candidate := resolved.Candidates[0]
	if candidate.IconURL != imageServer.URL+"/cover.jpg" {
		t.Fatalf("icon_url: %q", candidate.IconURL)
	}
	wantProxy := "/api/v1/media/candidate/" + fetch.URLHash(candidate.IconURL)
	if candidate.IconProxyURL != wantProxy {
		t.Fatalf("icon_proxy_url %q, want %q", candidate.IconProxyURL, wantProxy)
	}

	// 代理端点实际可取回已登记的头像。
	response, data := client.raw(http.MethodGet, wantProxy, nil, "", "")
	if response.StatusCode != http.StatusOK || len(data) != len(candidateIconPNG) {
		t.Fatalf("proxy fetch status %d size %d", response.StatusCode, len(data))
	}
}

// mergeWereadRefreshToken 保证落库 Cookie 始终带 wr_rt。
func TestMergeWereadRefreshToken(t *testing.T) {
	merged := mergeWereadRefreshToken("wr_skey=old; wr_vid=test", "abc/def")
	if !strings.Contains(merged, "wr_rt=abc%2Fdef") {
		t.Fatalf("merged cookie missing encoded wr_rt: %q", merged)
	}
	if got := mergeWereadRefreshToken("wr_skey=old", ""); got != "wr_skey=old" {
		t.Fatalf("empty refresh token must not change cookie: %q", got)
	}
}

// wr_skey 失效（-2010）时自动用 wr_rt 续期并重试，而不是要求重新扫码。
func TestWereadAuthExpiredRenewsAndRetries(t *testing.T) {
	var renewed atomic.Bool
	weread := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/web/login/renewal":
			renewed.Store(true)
			http.SetCookie(w, &http.Cookie{Name: "wr_skey", Value: "fresh", Path: "/"})
			_, _ = w.Write([]byte(`{"errCode":0}`))
		case r.URL.Path == "/web/shelf/sync":
			_, _ = w.Write([]byte(`{"errCode":0,"books":[{"bookId":"MP_WXS_1"}]}`))
		case r.URL.Path == "/mp/shelf/addToShelf":
			_, _ = w.Write([]byte(`{"errCode":0}`))
		case r.URL.Path == "/web/mp/articles":
			if renewed.Load() {
				_, _ = w.Write([]byte(`{"errCode":0,"reviews":[{"createTime":1700000000,"subReviews":[{"review":{"reviewId":"MP_WXS_3517031589_renewed","createTime":1700000000,"mpInfo":{"title":"续期后的文章","originalId":""}}}]}]}`))
			} else {
				_, _ = w.Write([]byte(`{"errCode":-2010,"errMsg":"login expired"}`))
			}
		case r.URL.Path == "/web/mp/content":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(`<!doctype html><meta property="og:url" content="https://mp.weixin.qq.com/s/renewed"><div id="js_content"><p>续期正文</p></div>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer weread.Close()

	oldBase := wereadBaseURL
	oldShelf := wereadShelfURL
	oldArticles := wereadArticlesURL
	oldContentFmt := wereadContentURLFmt
	oldRenewal := wereadRenewalURL
	wereadBaseURL = weread.URL
	wereadShelfURL = weread.URL + "/mp/shelf/addToShelf"
	wereadArticlesURL = weread.URL + "/web/mp/articles"
	wereadContentURLFmt = weread.URL + "/web/mp/content?reviewId=%s"
	wereadRenewalURL = weread.URL + "/web/login/renewal"
	t.Cleanup(func() {
		wereadBaseURL = oldBase
		wereadShelfURL = oldShelf
		wereadArticlesURL = oldArticles
		wereadContentURLFmt = oldContentFmt
		wereadRenewalURL = oldRenewal
	})

	app, _ := newTestApp(t, false)
	// 启用公众号采集（与设置页 PATCH 等效）。
	internal, err := app.loadInternalSettings(context.Background())
	if err != nil {
		t.Fatalf("load internal settings: %v", err)
	}
	internal.Wechat.Enabled = true
	if err := app.saveInternalSettings(context.Background(), internal); err != nil {
		t.Fatalf("save internal settings: %v", err)
	}
	stored := "wr_vid=test; wr_skey=expired; wr_rt=rttoken"
	if err := app.saveWereadCookie(context.Background(), stored); err != nil {
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
	current, err := app.wereadCookie(context.Background())
	if err != nil {
		t.Fatalf("read weread cookie: %v", err)
	}
	if !strings.Contains(current, "wr_skey=fresh") {
		t.Fatalf("renewed cookie not persisted: %q", current)
	}
	if !strings.Contains(current, "wr_rt=rttoken") {
		t.Fatalf("renewed cookie lost wr_rt: %q", current)
	}
}

// 没有 wr_rt 的存量 Cookie 无法续期：保持报错语义，不假装成功。
func TestWereadRenewalWithoutRefreshTokenFails(t *testing.T) {
	app, _ := newTestApp(t, false)
	if err := app.saveWereadCookie(context.Background(), "wr_vid=test; wr_skey=expired"); err != nil {
		t.Fatalf("save weread cookie: %v", err)
	}
	if app.renewWereadSession(context.Background()) {
		t.Fatal("renewal must fail without wr_rt")
	}
}

// 扫码登录成功后，公众号源上残留的「需重新扫码」错误状态必须被清除，
// 且刷新时间提前到现在，让调度器立刻用新登录态重抓。
func TestWereadScanRecoverWechatSources(t *testing.T) {
	app, _ := newTestApp(t, false)
	internal, err := app.loadInternalSettings(context.Background())
	if err != nil {
		t.Fatalf("load internal settings: %v", err)
	}
	internal.Wechat.Enabled = true
	if err := app.saveInternalSettings(context.Background(), internal); err != nil {
		t.Fatalf("save internal settings: %v", err)
	}
	source, err := app.createSourceRow(context.Background(), sourceCreateInput{
		Kind:    "wechat",
		Name:    "二哥狗腿子",
		FeedURL: "wechat://MzUxNzAzMTU4OQ==",
	})
	if err != nil {
		t.Fatalf("create source: %v", err)
	}
	stale := app.now().Add(-time.Hour).Format(timeRFC3339)
	if _, err := app.db.ExecContext(context.Background(), `
		UPDATE sources SET last_status = 'error', last_error = ?, fail_count = 5, next_fetch_at = ?
		WHERE id = ?`,
		"微信读书登录态已失效，请重新扫码连接", stale, source.ID); err != nil {
		t.Fatalf("seed stale error: %v", err)
	}

	app.recoverWechatSources(context.Background())

	updated, err := app.getSource(context.Background(), source.ID)
	if err != nil {
		t.Fatalf("get source: %v", err)
	}
	if updated.LastStatus == nil || *updated.LastStatus != "ok" || updated.LastError != nil || updated.FailCount != 0 {
		t.Fatalf("stale auth error not cleared: %+v", updated)
	}
	if updated.NextFetchAt == stale {
		t.Fatalf("next fetch not pulled forward: %+v", updated)
	}
}

// 主动续期（keep-alive）：距上次续期超过间隔才发请求；成功后回写新 Cookie；
// 未连接时静默跳过。
func TestWereadKeepAliveRenewal(t *testing.T) {
	var renewals atomic.Int32
	weread := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/web/login/renewal":
			renewals.Add(1)
			http.SetCookie(w, &http.Cookie{Name: "wr_skey", Value: "kept-alive", Path: "/"})
			_, _ = w.Write([]byte(`{"errCode":0,"succeed":1}`))
		case "/web/shelf/sync":
			_, _ = w.Write([]byte(`{"errCode":0,"books":[{"bookId":"MP_WXS_1"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer weread.Close()

	oldBase := wereadBaseURL
	oldRenewal := wereadRenewalURL
	wereadBaseURL = weread.URL
	wereadRenewalURL = weread.URL + "/web/login/renewal"
	t.Cleanup(func() {
		wereadBaseURL = oldBase
		wereadRenewalURL = oldRenewal
	})

	app, _ := newTestApp(t, false)
	if err := app.saveWereadCookie(context.Background(), "wr_vid=test; wr_skey=stale; wr_rt=rttoken"); err != nil {
		t.Fatalf("save weread cookie: %v", err)
	}

	// 第一次：从未续期过 → 触发续期。
	app.maybeWereadKeepAlive(context.Background())
	if renewals.Load() != 1 {
		t.Fatalf("renewal calls = %d, want 1", renewals.Load())
	}
	current, err := app.wereadCookie(context.Background())
	if err != nil || !strings.Contains(current, "wr_skey=kept-alive") {
		t.Fatalf("renewed cookie not persisted: %q (%v)", current, err)
	}

	// 第二次：30 分钟内 → 限流，不再发请求。
	app.maybeWereadKeepAlive(context.Background())
	if renewals.Load() != 1 {
		t.Fatalf("rate limit failed: renewal calls = %d, want 1", renewals.Load())
	}

	// 推进时钟越过间隔 → 再次续期。
	app.wereadKeepAliveMu.Lock()
	app.wereadLastKeepAlive = app.now().Add(-wereadKeepAliveInterval - time.Minute)
	app.wereadKeepAliveMu.Unlock()
	app.maybeWereadKeepAlive(context.Background())
	if renewals.Load() != 2 {
		t.Fatalf("renewal after interval: calls = %d, want 2", renewals.Load())
	}
}

func TestWereadKeepAliveSilentWhenNotConfigured(t *testing.T) {
	app, _ := newTestApp(t, false)
	app.maybeWereadKeepAlive(context.Background())
	// 未连接：保持静默，重复调用无副作用。
	app.maybeWereadKeepAlive(context.Background())
}

// 2026-11-09 真机实测：renewal 只认 ql=false；ql=true 返回 -2013 并用空值
// Set-Cookie 清场。这里锁定请求体，并确认新下发的 wr_rt/wr_pf 一并落库，
// 下一次 keep-alive 才能继续换票。
func TestWereadRenewalUsesQlFalseAndPersistsAllCookies(t *testing.T) {
	var requestBody atomic.Value
	weread := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/web/login/renewal":
			data, _ := io.ReadAll(r.Body)
			requestBody.Store(string(data))
			if !strings.Contains(string(data), `"ql":false`) {
				// 对齐真实服务端：ql=true 被拒并清场。
				http.SetCookie(w, &http.Cookie{Name: "wr_skey", Value: "", Path: "/"})
				http.SetCookie(w, &http.Cookie{Name: "wr_rt", Value: "", Path: "/"})
				_, _ = w.Write([]byte(`{"errCode":-2013,"errMsg":"鉴权失败"}`))
				return
			}
			http.SetCookie(w, &http.Cookie{Name: "wr_vid", Value: "537400866", Path: "/"})
			http.SetCookie(w, &http.Cookie{Name: "wr_skey", Value: "fresh8ch", Path: "/"})
			http.SetCookie(w, &http.Cookie{Name: "wr_pf", Value: "1", Path: "/"})
			http.SetCookie(w, &http.Cookie{Name: "wr_rt", Value: "web%40fresh", Path: "/"})
			_, _ = w.Write([]byte(`{"succ":1}`))
		case "/web/shelf/sync":
			_, _ = w.Write([]byte(`{"errCode":0,"books":[{"bookId":"MP_WXS_1"}],"bookCount":1}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer weread.Close()

	oldBase := wereadBaseURL
	oldRenewal := wereadRenewalURL
	wereadBaseURL = weread.URL
	wereadRenewalURL = weread.URL + "/web/login/renewal"
	t.Cleanup(func() {
		wereadBaseURL = oldBase
		wereadRenewalURL = oldRenewal
	})

	app, _ := newTestApp(t, false)
	stored := "wr_rt=web%40old; wr_skey=stale8; wr_vid=537400866"
	if err := app.saveWereadCookie(context.Background(), stored); err != nil {
		t.Fatalf("save weread cookie: %v", err)
	}
	if !app.renewWereadSession(context.Background()) {
		t.Fatal("renewal must succeed with ql=false")
	}
	if body, _ := requestBody.Load().(string); !strings.Contains(body, `"ql":false`) {
		t.Fatalf("renewal body = %q, want ql=false", body)
	}
	current, err := app.wereadCookie(context.Background())
	if err != nil {
		t.Fatalf("read weread cookie: %v", err)
	}
	for _, want := range []string{"wr_skey=fresh8ch", "wr_rt=web%40fresh", "wr_pf=1"} {
		if !strings.Contains(current, want) {
			t.Fatalf("renewed cookie missing %q: %q", want, current)
		}
	}
}

// renewal 失败时的清场响应不能覆盖现有凭据；下一次抓取仍应保留原 wr_rt，
// 以便网络抖动后还能继续尝试续期。
func TestWereadRenewalFailureKeepsStoredCookie(t *testing.T) {
	weread := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/web/login/renewal":
			http.SetCookie(w, &http.Cookie{Name: "wr_skey", Value: "", Path: "/"})
			http.SetCookie(w, &http.Cookie{Name: "wr_rt", Value: "", Path: "/"})
			_, _ = w.Write([]byte(`{"errCode":-2013,"errMsg":"鉴权失败"}`))
		case "/web/shelf/sync":
			_, _ = w.Write([]byte(`{"errCode":-2012,"errMsg":"登录超时"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer weread.Close()

	oldBase := wereadBaseURL
	oldRenewal := wereadRenewalURL
	wereadBaseURL = weread.URL
	wereadRenewalURL = weread.URL + "/web/login/renewal"
	t.Cleanup(func() {
		wereadBaseURL = oldBase
		wereadRenewalURL = oldRenewal
	})

	app, _ := newTestApp(t, false)
	stored := "wr_rt=web%40keep; wr_skey=stale8; wr_vid=537400866"
	if err := app.saveWereadCookie(context.Background(), stored); err != nil {
		t.Fatalf("save weread cookie: %v", err)
	}
	if app.renewWereadSession(context.Background()) {
		t.Fatal("renewal must fail when the endpoint clears credentials")
	}
	current, err := app.wereadCookie(context.Background())
	if err != nil {
		t.Fatalf("read weread cookie: %v", err)
	}
	if current != stored {
		t.Fatalf("failed renewal must not overwrite cookie: got %q want %q", current, stored)
	}
}
