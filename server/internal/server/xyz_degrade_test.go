package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"rill/internal/fetch"
)

// 2026-11-21 实测：feed.xyzfm.space 对非白名单出口返回 302 → 403 HTML
// （X-Tengine-Error: denied by IP ACL），官方分发 Feed 不可用。
func TestIsXiaoyuzhouFeedURLAndPidRecovery(t *testing.T) {
	if !isXiaoyuzhouFeedURL("https://feed.xyzfm.space/j7d9mlny9ea6") {
		t.Fatalf("xyzfm feed URL should be detected")
	}
	if isXiaoyuzhouFeedURL("https://example.com/feed.xml") {
		t.Fatalf("non-xyzfm feed URL must not be detected")
	}
	page := "https://www.xiaoyuzhoufm.com/podcast/5ed33dcc418a84a046f579ec"
	if pid := xiaoyuzhouPidFromSiteURL(&page); pid != "5ed33dcc418a84a046f579ec" {
		t.Fatalf("pid recovery = %q", pid)
	}
	if pid := xiaoyuzhouPidFromSiteURL(nil); pid != "" {
		t.Fatalf("nil site url must recover nothing")
	}
	apple := "https://podcasts.apple.com/cn/podcast/foo/id1234567890"
	if pid := xiaoyuzhouPidFromSiteURL(&apple); pid != "" {
		t.Fatalf("apple site url must recover nothing, got %q", pid)
	}
}

func xiaoyuzhouPageTestServer(t *testing.T, payload map[string]any, path string) *httptest.Server {
	t.Helper()
	encoded, _ := json.Marshal(payload)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != path {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(fmt.Sprintf(`<html><script id="__NEXT_DATA__" type="application/json">%s</script></html>`, encoded)))
	}))
	t.Cleanup(server.Close)
	return server
}

// Apple 解析创建的源 site_url 是 Apple 页面（无 pid），但分发 Feed 可用期间
// 入库的条目 URL 是小宇宙单集页 → 从单集页恢复 pid 后【永久替换】feed_url。
func TestXiaoyuzhouFeedAutoReplaceViaEpisodeItems(t *testing.T) {
	app, client := newTestApp(t, false)
	client.login()
	source := createTestSource(t, client, "忽左忽右", "https://feed.xyzfm.space/j7d9mlny9ea6")
	apple := "https://podcasts.apple.com/cn/podcast/foo/id1234567890"
	pid := "5ed33dcc418a84a046f579ec"
	eid := "6571efa208f81e5d30bf086f"
	if _, err := app.db.Exec(`UPDATE sources SET site_url = ? WHERE id = ?`, apple, source.ID); err != nil {
		t.Fatal(err)
	}
	now := app.now().Format(timeRFC3339)
	if _, err := app.db.Exec(`
		INSERT INTO items (source_id, guid, title, url, published_at, published_estimated,
			content_html, content_text, content_hash, fetched_at, updated_at)
		VALUES (?, ?, ?, ?, ?, 0, '', '正文', 'hash-1', ?, ?)`,
		source.ID, "https://x.com/ep/1", "Episode 9",
		"https://www.xiaoyuzhoufm.com/episode/"+eid,
		now, now, now); err != nil {
		t.Fatal(err)
	}

	payload := map[string]any{
		"props": map[string]any{
			"pageProps": map[string]any{
				"episode": map[string]any{"pid": pid},
			},
		},
	}
	server := xiaoyuzhouPageTestServer(t, payload, "/episode/"+eid)
	previousBase := app.xiaoyuzhouBase
	app.xiaoyuzhouBase = server.URL
	t.Cleanup(func() { app.xiaoyuzhouBase = previousBase })

	dbSource, err := app.sourceInternal(context.Background(), source.ID)
	if err != nil {
		t.Fatal(err)
	}
	replaced, recoveredPid, replaceErr := app.replaceXiaoyuzhouFeedSource(context.Background(), dbSource)
	if replaceErr != nil {
		t.Fatalf("replaceXiaoyuzhouFeedSource: %v", replaceErr)
	}
	if recoveredPid != pid {
		t.Fatalf("pid = %q", recoveredPid)
	}
	if replaced.FeedURL != "xiaoyuzhou://podcast/"+pid {
		t.Fatalf("feed_url should be rewritten, got %q", replaced.FeedURL)
	}
	if replaced.Note == nil || !strings.Contains(*replaced.Note, "已自动改用小宇宙页面抓取") {
		t.Fatalf("note = %v", replaced.Note)
	}
	var storedURL string
	if err := app.db.QueryRow(`SELECT feed_url FROM sources WHERE id = ?`, source.ID).Scan(&storedURL); err != nil {
		t.Fatal(err)
	}
	if storedURL != "xiaoyuzhou://podcast/"+pid {
		t.Fatalf("replacement must be persisted, got %q", storedURL)
	}
}

// site_url 直接是节目页（token 建源时 Feed 内容写入过）→ 无需网络即可替换。
func TestXiaoyuzhouFeedAutoReplaceViaSiteURL(t *testing.T) {
	app, client := newTestApp(t, false)
	client.login()
	source := createTestSource(t, client, "Echo.js", "https://feed.xyzfm.space/mcnun9bubfhb")
	page := "https://www.xiaoyuzhoufm.com/podcast/5ed33dcc418a84a046f579ec"
	if _, err := app.db.Exec(`UPDATE sources SET site_url = ? WHERE id = ?`, page, source.ID); err != nil {
		t.Fatal(err)
	}
	dbSource, err := app.sourceInternal(context.Background(), source.ID)
	if err != nil {
		t.Fatal(err)
	}
	replaced, pid, replaceErr := app.replaceXiaoyuzhouFeedSource(context.Background(), dbSource)
	if replaceErr != nil {
		t.Fatalf("replaceXiaoyuzhouFeedSource: %v", replaceErr)
	}
	if pid != "5ed33dcc418a84a046f579ec" || replaced.FeedURL != "xiaoyuzhou://podcast/5ed33dcc418a84a046f579ec" {
		t.Fatalf("unexpected replace: pid=%q url=%q", pid, replaced.FeedURL)
	}
}

func TestXiaoyuzhouFeedReplaceRejectsDuplicate(t *testing.T) {
	app, client := newTestApp(t, false)
	client.login()
	first := createTestSource(t, client, "Echo.js", "xiaoyuzhou://podcast/5ed33dcc418a84a046f579ec")
	second := createTestSource(t, client, "Echo.js Feed", "https://feed.xyzfm.space/mcnun9bubfhb")
	page := "https://www.xiaoyuzhoufm.com/podcast/5ed33dcc418a84a046f579ec"
	if _, err := app.db.Exec(`UPDATE sources SET site_url = ? WHERE id = ?`, page, second.ID); err != nil {
		t.Fatal(err)
	}
	dbSource, err := app.sourceInternal(context.Background(), second.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := app.replaceXiaoyuzhouFeedSource(context.Background(), dbSource); err == nil {
		t.Fatalf("duplicate page-fetch source must block replacement")
	}
	_ = first
}

func TestFetchFeedRedirectToHTMLHint(t *testing.T) {
	// 模拟小宇宙 ACL 行为：Feed 域名 302 到另一台机器上的 200 HTML 页面。
	denial := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<html><body>403 Forbidden</body></html>"))
	}))
	defer denial.Close()
	feeder := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, denial.URL+"/denied.html", http.StatusFound)
	}))
	defer feeder.Close()

	client := fetch.NewClient("Rill/test")
	_, err := client.FetchFeed(context.Background(), feeder.URL+"/feed.xml", fetch.FetchState{}, "tok")
	if err == nil {
		t.Fatalf("redirect to html must fail")
	}
	denialHost, _ := url.Parse(denial.URL)
	if !strings.Contains(err.Error(), denialHost.Hostname()) {
		t.Fatalf("error should hint the redirect target host, got: %s", err.Error())
	}
}

func TestSourceIconUnavatarFallback(t *testing.T) {
	app, _ := newTestApp(t, false)
	png := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0, 0, 0, 13, 0x49, 0x48, 0x44, 0x52,
	}
	avatar := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/x/HiTw93" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write(append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, png...))
	}))
	defer avatar.Close()
	previousBase := xUnavatarBase
	xUnavatarBase = avatar.URL + "/x/"
	t.Cleanup(func() { xUnavatarBase = previousBase })

	// icon_url 指向一个立刻拒绝连接的地址（模拟被墙的 pbs.twimg.com），
	// 降级候选 unavatar 应成功缓存。
	blocked := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Second)
	}))
	blocked.Close()

	row := iconSourceRow{
		IconSourceURL: blocked.URL + "/avatar.jpg",
		FeedURL:       "https://fxtwitter.com/HiTw93/feed.xml?count=20",
		SiteURL:       "https://x.com/HiTw93",
	}
	if err := app.refreshSourceIcon(context.Background(), 42, row); err != nil {
		t.Fatalf("refreshSourceIcon: %v", err)
	}
	info, err := os.Stat(app.iconPath(42))
	if err != nil {
		t.Fatalf("icon cache file missing: %v", err)
	}
	if info.Size() == 0 {
		t.Fatalf("icon cache file empty")
	}
	if app.iconFailureRecent(42) {
		t.Fatalf("failure marker must be cleared")
	}
}

type roundTripRewriteHost struct{ host string }

func (r roundTripRewriteHost) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Hostname() != "fxtwitter.com" {
		return http.DefaultTransport.RoundTrip(req)
	}
	clone := req.Clone(req.Context())
	clone.URL.Scheme = "http"
	clone.URL.Host = r.host
	clone.URL.Path = "/feed.xml"
	clone.URL.RawQuery = ""
	return http.DefaultTransport.RoundTrip(clone)
}

// 手动刷新必须同时刷新头像：服务端清缓存 + 前端按 last_fetch_at 重载图片。
func TestManualRefreshClearsIconCache(t *testing.T) {
	app, client := newTestApp(t, false)
	client.login()
	feedServer, _ := feedTestServer(t)
	feedURL, _ := url.Parse(feedServer.URL)
	app.fetcher.HTTP.Transport = roundTripRewriteHost{host: feedURL.Host}

	avatar := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10})
	}))
	defer avatar.Close()
	previousBase := xUnavatarBase
	xUnavatarBase = avatar.URL + "/x/"
	t.Cleanup(func() { xUnavatarBase = previousBase })

	source := createTestSource(t, client, "@C1ownhjj", "https://fxtwitter.com/C1ownhjj/feed.xml?count=20")
	if err := os.MkdirAll(app.iconDir(), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(app.iconPath(source.ID), []byte("old-bird"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(app.iconFailurePath(source.ID), []byte("x"), 0o640); err != nil {
		t.Fatal(err)
	}

	var refreshed Source
	client.decode(http.MethodPost, "/api/v1/sources/"+strconv.FormatInt(source.ID, 10)+"/fetch", nil, true, http.StatusOK, &refreshed)
	if _, err := os.Stat(app.iconPath(source.ID)); !os.IsNotExist(err) {
		t.Fatalf("manual refresh must clear cached icon, stat err=%v", err)
	}
	if app.iconFailureRecent(source.ID) {
		t.Fatalf("manual refresh must clear icon failure marker")
	}
	if refreshed.LastFetchAt == nil {
		t.Fatalf("refresh response should include last_fetch_at for icon cache busting")
	}

	// 下一次图标请求应重新走候选链（unavatar 测试端点）并缓存成功。
	iconPath := "/api/v1/sources/" + strconv.FormatInt(source.ID, 10) + "/icon"
	response, body := client.raw(http.MethodGet, iconPath, nil, "", "")
	if response.StatusCode != http.StatusOK || len(body) == 0 {
		t.Fatalf("icon should be re-fetched: %d (%d bytes)", response.StatusCode, len(body))
	}
}

// BUG-004 第四轮：X 源头像候选链必须排除站点 favicon——旧链最终缓存的是
// fxtwitter.com 的蓝鸟图标，用户会一直看到「错误头像」且不会刷新。
func TestXIconCandidatesExcludeSiteFavicons(t *testing.T) {
	app := &App{}
	candidates := app.iconCandidates(context.Background(), iconSourceRow{
		IconSourceURL: "https://pbs.twimg.com/profile_images/1/avatar.jpg",
		FeedURL:       "https://fxtwitter.com/C1ownhjj/feed.xml?count=20",
		SiteURL:       "https://x.com/C1ownhjj",
	})
	if len(candidates) != 2 {
		t.Fatalf("unexpected candidate count: %v", candidates)
	}
	if candidates[0] != "https://unavatar.io/x/C1ownhjj" {
		t.Fatalf("first candidate = %q", candidates[0])
	}
	if candidates[1] != "https://pbs.twimg.com/profile_images/1/avatar.jpg" {
		t.Fatalf("feed image should be the fallback, got %q", candidates[1])
	}
	for _, candidate := range candidates {
		if strings.Contains(candidate, "favicon.ico") || strings.Contains(candidate, "x.com/") {
			t.Fatalf("X icon candidates must not include site favicons: %v", candidates)
		}
	}
	// 非 X 源保留原有 favicon 链（SiteURL 留空避免真实网络请求）。
	regular := app.iconCandidates(context.Background(), iconSourceRow{
		FeedURL: "https://blog.example.com/feed.xml",
	})
	if len(regular) == 0 || !strings.Contains(strings.Join(regular, ","), "favicon.ico") {
		t.Fatalf("non-X candidates should keep favicon fallback: %v", regular)
	}
}

// BUG-007：FxTwitter 频道 <image> 可能长期停留在旧头像（@poteto 实测仍是
// 2026-10 的照片）。unavatar 返回当前头像时必须优先缓存它，而不是旧 Feed 图。
func TestXIconPrefersCurrentAvatarOverStaleFeedImage(t *testing.T) {
	app, _ := newTestApp(t, false)
	current := []byte{0xFF, 0xD8, 0xFF, 0xE0, 'c', 'u', 'r', 'r', 'e', 'n', 't'}
	stale := []byte{0xFF, 0xD8, 0xFF, 0xE0, 's', 't', 'a', 'l', 'e'}

	avatar := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/x/poteto" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write(current)
	}))
	defer avatar.Close()
	previousBase := xUnavatarBase
	xUnavatarBase = avatar.URL + "/x/"
	t.Cleanup(func() { xUnavatarBase = previousBase })

	feedImage := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write(stale)
	}))
	defer feedImage.Close()

	row := iconSourceRow{
		IconSourceURL: feedImage.URL + "/old-avatar.jpg",
		FeedURL:       "https://fxtwitter.com/poteto/feed.xml?count=20",
		SiteURL:       "https://x.com/poteto",
	}
	if err := app.refreshSourceIcon(context.Background(), 77, row); err != nil {
		t.Fatalf("refreshSourceIcon: %v", err)
	}
	cached, err := os.ReadFile(app.iconPath(77))
	if err != nil {
		t.Fatalf("read cached icon: %v", err)
	}
	if string(cached) != string(current) {
		t.Fatalf("cached icon should be the current avatar, got %q", cached)
	}
}

// 启动清理：旧版本缓存的 X 源头像（蓝鸟）必须在启动时删除并清掉失败标记；
// 其他源的图标不受影响。
func TestResetLegacyXSourceIcons(t *testing.T) {
	app, client := newTestApp(t, true)
	client.login()
	xSource := createTestSource(t, client, "@C1ownhjj", "https://fxtwitter.com/C1ownhjj/feed.xml?count=20")
	other := createTestSource(t, client, "Blog", "https://blog.example.com/feed.xml")
	if err := os.MkdirAll(app.iconDir(), 0o750); err != nil {
		t.Fatal(err)
	}
	icon := []byte{0xFF, 0xD8, 0xFF, 0xE0}
	for _, id := range []int64{xSource.ID, other.ID} {
		if err := os.WriteFile(app.iconPath(id), icon, 0o640); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(app.iconFailurePath(xSource.ID), []byte("x"), 0o640); err != nil {
		t.Fatal(err)
	}
	app.resetLegacyXSourceIcons(context.Background())
	if _, err := os.Stat(app.iconPath(xSource.ID)); !os.IsNotExist(err) {
		t.Fatalf("legacy X icon should be removed, stat err=%v", err)
	}
	if app.iconFailureRecent(xSource.ID) {
		t.Fatalf("X icon failure marker should be cleared")
	}
	if _, err := os.Stat(app.iconPath(other.ID)); err != nil {
		t.Fatalf("non-X icon must be kept: %v", err)
	}
}

// BUG-004 复核：旧失败标记会拦截图标重试最长 1 小时；抓取成功后必须清除，
// 用户「刷新源」才能让新头像候选链立即生效。
func TestFetchSourceClearsIconFailureMarker(t *testing.T) {
	app, client := newTestApp(t, false)
	client.login()
	feedServer, _ := feedTestServer(t)
	source := createTestSource(t, client, "Local feed", feedServer.URL+"/feed.xml")
	if err := os.MkdirAll(app.iconDir(), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(app.iconFailurePath(source.ID), []byte(app.now().Format(timeRFC3339)), 0o640); err != nil {
		t.Fatal(err)
	}
	if !app.iconFailureRecent(source.ID) {
		t.Fatalf("failure marker should be present")
	}
	if _, err := app.fetchSource(context.Background(), source.ID, false, ""); err != nil {
		t.Fatalf("fetchSource: %v", err)
	}
	if app.iconFailureRecent(source.ID) {
		t.Fatalf("successful fetch must clear the icon failure marker")
	}
}

// X 源旧图标（可能是升级前缓存的通用站点图标）需在抓取成功后按 6 小时失效，
// 否则 7 天 TTL 内永远不会重试新头像候选链。
func TestFetchSourceInvalidatesOldXIcon(t *testing.T) {
	app, client := newTestApp(t, false)
	client.login()
	feedServer, _ := feedTestServer(t)
	// 把 fxtwitter 域名的请求改写到本地测试 Feed，避免真实网络。
	feedURL, _ := url.Parse(feedServer.URL)
	app.fetcher.HTTP.Transport = roundTripRewriteHost{host: feedURL.Host}
	source := createTestSource(t, client, "@C1ownhjj", "https://fxtwitter.com/C1ownhjj/feed.xml?count=20")
	if err := os.MkdirAll(app.iconDir(), 0o750); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-24 * time.Hour)
	if err := os.WriteFile(app.iconPath(source.ID), []byte{0xFF, 0xD8, 0xFF}, 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(app.iconPath(source.ID), old, old); err != nil {
		t.Fatal(err)
	}
	if _, err := app.fetchSource(context.Background(), source.ID, false, ""); err != nil {
		t.Fatalf("fetchSource: %v", err)
	}
	if _, err := os.Stat(app.iconPath(source.ID)); !os.IsNotExist(err) {
		t.Fatalf("old X icon should be invalidated, stat err=%v", err)
	}
}

// 回归测试（0.1.10 实测缺陷）：替换成功后仍在失败分支里继续记录失败并返回
// 错误。这里走完整 fetchSource：Feed 抓取失败（指向本机关闭端口，快速失败）
// → 替换为页面源 → 页面抓取成功 → 必须返回成功并入库。
func TestFetchSourceXiaoyuzhouReplacesAndSucceeds(t *testing.T) {
	app, client := newTestApp(t, false)
	client.login()
	source := createTestSource(t, client, "Echo.js", "http://127.0.0.1:1/feed.xyzfm.space/tok")
	pid := "5ed33dcc418a84a046f579ec"
	page := "https://www.xiaoyuzhoufm.com/podcast/" + pid
	_, _ = app.db.Exec(`UPDATE sources SET site_url = ? WHERE id = ?`, page, source.ID)

	payload := map[string]any{
		"props": map[string]any{
			"pageProps": map[string]any{
				"podcast": map[string]any{
					"pid":   pid,
					"title": "Echo.js",
					"image": map[string]any{"picUrl": "https://img.example/pod.jpg"},
					"episodes": []map[string]any{{
						"eid":         "6571efa208f81e5d30bf086f",
						"title":       "Episode 9",
						"description": "第一段\n第二段",
						"pubDate":     "2023-12-07T16:00:00.000Z",
						"enclosure":   map[string]any{"url": "https://media.example/ep.mp3"},
						"image":       map[string]any{"picUrl": "https://img.example/ep.jpg"},
					}},
				},
			},
		},
	}
	server := xiaoyuzhouPageTestServer(t, payload, "/podcast/"+pid)
	previousBase := app.xiaoyuzhouBase
	app.xiaoyuzhouBase = server.URL
	t.Cleanup(func() { app.xiaoyuzhouBase = previousBase })

	fetched, err := app.fetchSource(context.Background(), source.ID, true, "")
	if err != nil {
		dbSource, _ := app.sourceInternal(context.Background(), source.ID)
		t.Fatalf("fetchSource after replacement should succeed: %v (feed_url=%q last_error=%v)", err, dbSource.FeedURL, dbSource.LastError)
	}
	if fetched.FeedURL != "xiaoyuzhou://podcast/"+pid {
		t.Fatalf("feed_url = %q", fetched.FeedURL)
	}
	if fetched.LastStatus == nil || *fetched.LastStatus != "ok" {
		t.Fatalf("status = %v, want ok", fetched.LastStatus)
	}
	if fetched.ItemCount == 0 {
		t.Fatalf("items should be ingested after replacement")
	}
}
