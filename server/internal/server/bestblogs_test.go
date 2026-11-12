package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const (
	testBestBlogsWeChatFeed  = "https://wechat2rss.bestblogs.dev/feed/ff621c3e98d6ae6fceb3397e57441ffc6ea3c17f.xml"
	testBestBlogsXFeed       = "https://api.xgo.ing/rss/user/0c0856a69f9f49cf961018c32a0b0049"
	testBestBlogsPodcastFeed = "https://rsshub.bestblogs.dev/xiaoyuzhou/podcast/5e74b52c418a84a046ecaceb"
	testBestBlogsYouTubeFeed = "https://www.youtube.com/feeds/videos.xml?channel_id=UCrDwWp7EBBv4NwvScIpBDOA"
)

func TestParseBestBlogsFeedURLs(t *testing.T) {
	tests := []struct {
		name   string
		raw    string
		parser func(string) (string, bool)
		want   string
		ok     bool
	}{
		{"wechat", testBestBlogsWeChatFeed, parseBestBlogsWeChatFeedURL, testBestBlogsWeChatFeed, true},
		{"x", testBestBlogsXFeed, parseBestBlogsXFeedURL, testBestBlogsXFeed, true},
		{"podcast", testBestBlogsPodcastFeed, parseBestBlogsPodcastFeedURL, testBestBlogsPodcastFeed, true},
		{"youtube", testBestBlogsYouTubeFeed, parseBestBlogsYouTubeFeedURL, testBestBlogsYouTubeFeed, true},
		{"wechat http", "http://wechat2rss.bestblogs.dev/feed/ff621c3e98d6ae6fceb3397e57441ffc6ea3c17f.xml", parseBestBlogsWeChatFeedURL, "", false},
		{"x wrong host", "https://example.com/rss/user/0c0856a69f9f49cf961018c32a0b0049", parseBestBlogsXFeedURL, "", false},
		{"podcast short id", "https://rsshub.bestblogs.dev/xiaoyuzhou/podcast/abc", parseBestBlogsPodcastFeedURL, "", false},
		{"youtube wrong channel", "https://www.youtube.com/feeds/videos.xml?channel_id=bad", parseBestBlogsYouTubeFeedURL, "", false},
		{"wechat query", testBestBlogsWeChatFeed + "?x=1", parseBestBlogsWeChatFeedURL, "", false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := test.parser(test.raw)
			if ok != test.ok || got != test.want {
				t.Fatalf("parser(%q) = (%q, %v), want (%q, %v)", test.raw, got, ok, test.want, test.ok)
			}
		})
	}
}

func bestBlogsOPML(name, feedURL string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<opml version="2.0"><head><title>BestBlogs</title></head><body>
  <outline text="AI">
    <outline type="rss" text="` + name + `" title="` + name + `" xmlUrl="` + feedURL + `"/>
  </outline>
</body></opml>`
}

func TestResolveBestBlogsByNameAcrossPlatforms(t *testing.T) {
	files := map[string]string{
		"/opml/bestblogs_wechat2rss_opml_all.opml": bestBlogsOPML("数字生命卡兹克", testBestBlogsWeChatFeed),
		"/BestBlogs_RSS_Twitters.opml":             bestBlogsOPML("数字生命卡兹克", testBestBlogsXFeed),
		"/BestBlogs_RSS_Podcasts.opml":             bestBlogsOPML("数字生命卡兹克", testBestBlogsPodcastFeed),
		"/BestBlogs_RSS_Videos.opml":               bestBlogsOPML("数字生命卡兹克", testBestBlogsYouTubeFeed),
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := files[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/x-opml")
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	app, _ := newTestApp(t, false)
	app.bestblogsCatalogURL = ""
	app.bestblogsCatalogBase = server.URL + "/"
	candidates, recognized, err := app.resolveSourceInput(context.Background(), "数字生命卡兹克")
	if err != nil {
		t.Fatalf("resolveSourceInput: %v", err)
	}
	if !recognized || len(candidates) != 4 {
		t.Fatalf("recognized=%v candidates=%+v", recognized, candidates)
	}
	wantPlatforms := []string{"bestblogs", "bestblogs-x", "bestblogs-podcast", "bestblogs-youtube"}
	for index, want := range wantPlatforms {
		if candidates[index].Platform != want || candidates[index].Kind != "rss" {
			t.Fatalf("candidate[%d]=%+v, want platform=%s kind=rss", index, candidates[index], want)
		}
	}
	if candidates[0].FeedURL != testBestBlogsWeChatFeed || !strings.Contains(candidates[0].Note, "BestBlogs") {
		t.Fatalf("wechat candidate=%+v", candidates[0])
	}
	if app.shouldTryBestBlogsName("OpenAI") {
		t.Fatal("ASCII X handle must not auto-search BestBlogs after the catalog is warm")
	}
}

func TestResolveBestBlogsPartialCatalogFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/opml/bestblogs_wechat2rss_opml_all.opml" {
			w.Header().Set("Content-Type", "text/x-opml")
			_, _ = w.Write([]byte(bestBlogsOPML("数字生命卡兹克", testBestBlogsWeChatFeed)))
			return
		}
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	app, _ := newTestApp(t, false)
	app.bestblogsCatalogURL = ""
	app.bestblogsCatalogBase = server.URL + "/"
	candidates, recognized, err := app.resolveSourceInput(context.Background(), "数字生命卡兹克")
	if err != nil {
		t.Fatalf("resolveSourceInput: %v", err)
	}
	if !recognized || len(candidates) != 1 || candidates[0].Platform != "bestblogs" {
		t.Fatalf("recognized=%v candidates=%+v", recognized, candidates)
	}
}

func TestBestBlogsCatalogUsesFallbackBase(t *testing.T) {
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "primary unavailable", http.StatusServiceUnavailable)
	}))
	defer primary.Close()
	var fallbackHits atomic.Int32
	fallback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fallbackHits.Add(1)
		if r.URL.Path == "/opml/bestblogs_wechat2rss_opml_all.opml" {
			w.Header().Set("Content-Type", "text/x-opml")
			_, _ = w.Write([]byte(bestBlogsOPML("数字生命卡兹克", testBestBlogsWeChatFeed)))
			return
		}
		http.Error(w, "fallback unavailable", http.StatusServiceUnavailable)
	}))
	defer fallback.Close()

	app, _ := newTestApp(t, false)
	app.bestblogsCatalogURL = ""
	app.bestblogsCatalogBase = primary.URL + "/"
	app.bestblogsCatalogFallbackBase = fallback.URL + "/"
	candidates, recognized, err := app.resolveSourceInput(context.Background(), "数字生命卡兹克")
	if err != nil {
		t.Fatalf("resolveSourceInput: %v", err)
	}
	if !recognized || len(candidates) != 1 || candidates[0].Platform != "bestblogs" {
		t.Fatalf("recognized=%v candidates=%+v", recognized, candidates)
	}
	if fallbackHits.Load() == 0 {
		t.Fatal("fallback catalog was never requested")
	}
}

func TestResolveBestBlogsPartialFailureRetriesSooner(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.URL.Path == "/opml/bestblogs_wechat2rss_opml_all.opml" {
			w.Header().Set("Content-Type", "text/x-opml")
			_, _ = w.Write([]byte(bestBlogsOPML("数字生命卡兹克", testBestBlogsWeChatFeed)))
			return
		}
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	app, _ := newTestApp(t, false)
	app.bestblogsCatalogURL = ""
	app.bestblogsCatalogBase = server.URL + "/"
	now := time.Now()
	app.nowFn = func() time.Time { return now }

	resolve := func() {
		t.Helper()
		if _, _, err := app.resolveSourceInput(context.Background(), "数字生命卡兹克"); err != nil {
			t.Fatalf("resolveSourceInput: %v", err)
		}
	}
	resolve()
	first := requests.Load()
	if first != 4 {
		t.Fatalf("first refresh made %d requests, want 4", first)
	}
	resolve()
	if got := requests.Load(); got != first {
		t.Fatalf("fresh cache made extra requests: %d -> %d", first, got)
	}

	now = now.Add(bestBlogsPartialCatalogRetry + time.Minute)
	resolve()
	if got := requests.Load(); got <= first {
		t.Fatalf("partial failure did not retry after short interval: %d -> %d", first, got)
	}
}

func TestResolveBestBlogsCatalogFailureFallsThrough(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	app, _ := newTestApp(t, false)
	app.bestblogsCatalogURL = ""
	app.bestblogsCatalogBase = server.URL + "/"
	candidates, recognized, err := app.resolveSourceInput(context.Background(), "数字生命卡兹克")
	if err != nil {
		t.Fatalf("resolveSourceInput: %v", err)
	}
	if recognized || len(candidates) != 0 {
		t.Fatalf("recognized=%v candidates=%+v, want fall-through", recognized, candidates)
	}
}
