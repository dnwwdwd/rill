package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"rill/internal/fetch"
)

func TestAppleIDFromInput(t *testing.T) {
	tests := []struct {
		input string
		id    string
		ok    bool
	}{
		{"https://podcasts.apple.com/us/podcast/darknet-diaries/id1296350485", "1296350485", true},
		{"https://podcasts.apple.com/cn/podcast/foo/id1234567890?i=1000123", "1234567890", true},
		{"id1296350485", "1296350485", true},
		{"1296350485", "1296350485", true},
		{"https://example.com/id1296350485", "", false},
		{"not-an-id", "", false},
	}
	for _, test := range tests {
		id, ok := appleIDFromInput(test.input)
		if id != test.id || ok != test.ok {
			t.Fatalf("appleIDFromInput(%q) = (%q, %v), want (%q, %v)", test.input, id, ok, test.id, test.ok)
		}
	}
}

func TestXiaoyuzhouRefFromInput(t *testing.T) {
	tests := []struct {
		input string
		pid   string
		eid   string
		token string
		ok    bool
	}{
		{"https://www.xiaoyuzhoufm.com/podcast/5ed33dcc418a84a046f579ec", "5ed33dcc418a84a046f579ec", "", "", true},
		{"https://www.xiaoyuzhoufm.com/episode/6571efa208f81e5d30bf086f", "", "6571efa208f81e5d30bf086f", "", true},
		{"https://feed.xyzfm.space/j7d9mlny9ea6", "", "", "j7d9mlny9ea6", true},
		{"5ed33dcc418a84a046f579ec", "5ed33dcc418a84a046f579ec", "", "", true},
		{"xiaoyuzhou://podcast/5ed33dcc418a84a046f579ec", "5ed33dcc418a84a046f579ec", "", "", true},
		{"https://example.com/podcast/5ed33dcc418a84a046f579ec", "", "", "", false},
	}
	for _, test := range tests {
		pid, eid, token, ok := xiaoyuzhouRefFromInput(test.input)
		if pid != test.pid || eid != test.eid || token != test.token || ok != test.ok {
			t.Fatalf("xiaoyuzhouRefFromInput(%q) = (%q, %q, %q, %v), want (%q, %q, %q, %v)",
				test.input, pid, eid, token, ok, test.pid, test.eid, test.token, test.ok)
		}
	}
}

// 微博渠道移除后，微博链接/UID 前缀必须返回明确错误而不是被当成普通网站。
func TestWeiboInputRejected(t *testing.T) {
	app := &App{fetcher: fetch.NewClient("Rill/test")}
	for _, input := range []string{
		"weibo://user/2803301701",
		"weibo:2803301701",
		"https://weibo.com/u/2803301701",
		"https://m.weibo.cn/u/2803301701",
	} {
		_, recognized, err := app.resolveSourceInput(context.Background(), input)
		if !recognized || err == nil || !strings.Contains(err.Error(), "微博渠道已移除") {
			t.Fatalf("resolveSourceInput(%q) = (%v, %v), want weibo removed error", input, recognized, err)
		}
	}
}

func TestParseXiaoyuzhouPage(t *testing.T) {
	payload := map[string]any{
		"props": map[string]any{
			"pageProps": map[string]any{
				"podcast": map[string]any{
					"pid":          "5ed33dcc418a84a046f579ec",
					"title":        "Echo.js",
					"author":       "Astrian & 白羊",
					"episodeCount": 11,
					"image":        map[string]any{"picUrl": "https://img.example/cover.jpg"},
					"episodes": []any{map[string]any{
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
	encoded, _ := json.Marshal(payload)
	body := []byte(fmt.Sprintf(`<html><script id="__NEXT_DATA__" type="application/json">%s</script></html>`, encoded))
	data, err := parseXiaoyuzhouPage(body)
	if err != nil {
		t.Fatalf("parseXiaoyuzhouPage: %v", err)
	}
	if data.Props.PageProps.Podcast == nil || data.Props.PageProps.Podcast.Title != "Echo.js" {
		t.Fatalf("unexpected podcast: %+v", data.Props.PageProps.Podcast)
	}
	if len(data.Props.PageProps.Podcast.Episodes) != 1 {
		t.Fatalf("unexpected episodes: %d", len(data.Props.PageProps.Podcast.Episodes))
	}
}

func TestFetchXiaoyuzhouAdapter(t *testing.T) {
	payload := map[string]any{
		"props": map[string]any{
			"pageProps": map[string]any{
				"podcast": map[string]any{
					"pid":    "5ed33dcc418a84a046f579ec",
					"title":  "Echo.js",
					"author": "Astrian & 白羊",
					"image":  map[string]any{"picUrl": "https://img.example/cover.jpg"},
					"episodes": []any{map[string]any{
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
	encoded, _ := json.Marshal(payload)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/podcast/5ed33dcc418a84a046f579ec" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(fmt.Sprintf(`<html><script id="__NEXT_DATA__" type="application/json">%s</script></html>`, encoded)))
	}))
	defer server.Close()

	app := &App{
		fetcher:        fetch.NewClient("Rill/test"),
		xiaoyuzhouBase: server.URL,
		nowFn:          time.Now,
	}
	result, err := app.fetchXiaoyuzhou(context.Background(), dbSource{
		Source:    Source{Name: "Echo.js", FeedURL: "xiaoyuzhou://podcast/5ed33dcc418a84a046f579ec"},
		FeedToken: "tok",
	})
	if err != nil {
		t.Fatalf("fetchXiaoyuzhou: %v", err)
	}
	if result.Title != "Echo.js" || len(result.Items) != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
	item := result.Items[0]
	if item.GUID != "6571efa208f81e5d30bf086f" || item.AudioURL != "https://media.example/ep.mp3" || item.AudioType != "audio/mpeg" {
		t.Fatalf("unexpected item: %+v", item)
	}
	if !strings.Contains(item.ContentHTML, "/media/tok/") || !strings.Contains(item.ContentText, "第一段") {
		t.Fatalf("unexpected content: %q / %q", item.ContentHTML, item.ContentText)
	}
}

func TestResolveAppleByID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/lookup" || r.URL.Query().Get("id") != "1296350485" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"resultCount":1,"results":[{"collectionName":"Darknet Diaries","feedUrl":"https://podcast.darknetdiaries.com","collectionViewUrl":"https://podcasts.apple.com/us/podcast/id1296350485","artworkUrl600":"https://img.example/cover.jpg"}]}`))
	}))
	defer server.Close()

	app := &App{fetcher: fetch.NewClient("Rill/test"), appleAPIBase: server.URL}
	candidates, err := app.resolveAppleByID(context.Background(), "1296350485")
	if err != nil {
		t.Fatalf("resolveAppleByID: %v", err)
	}
	if len(candidates) != 1 || candidates[0].FeedURL != "https://podcast.darknetdiaries.com" || candidates[0].Kind != "rss" {
		t.Fatalf("unexpected candidates: %+v", candidates)
	}
}

func TestResolveSourceInputLeavesGenericURL(t *testing.T) {
	app := &App{fetcher: fetch.NewClient("Rill/test")}
	candidates, recognized, err := app.resolveSourceInput(context.Background(), "https://example.com/feed.xml")
	if err != nil {
		t.Fatalf("resolveSourceInput: %v", err)
	}
	if recognized || len(candidates) != 0 {
		t.Fatalf("generic URL should fall through: recognized=%v candidates=%+v", recognized, candidates)
	}
}

func TestWeiboSchemeSourceRejected(t *testing.T) {
	app, _ := newTestApp(t, false)
	_, err := app.createSourceRow(context.Background(), sourceCreateInput{
		Kind:    "rss",
		Name:    "人民日报",
		FeedURL: "weibo://user/2803301701",
	})
	var apiErr *apiError
	if !errors.As(err, &apiErr) || apiErr.Code != "weibo_removed" {
		t.Fatalf("expected weibo_removed, got %v", err)
	}
}

func TestResolveXiaoyuzhouFallback(t *testing.T) {
	payload := map[string]any{
		"props": map[string]any{"pageProps": map[string]any{"podcast": map[string]any{
			"pid":   "5ed33dcc418a84a046f579ec",
			"title": "Echo.js",
			"image": map[string]any{"picUrl": "https://img.example/cover.jpg"},
		}}},
	}
	encoded, _ := json.Marshal(payload)
	page := []byte(fmt.Sprintf(`<html><script id="__NEXT_DATA__" type="application/json">%s</script></html>`, encoded))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/podcast/5ed33dcc418a84a046f579ec":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write(page)
		case "/search":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"resultCount":0,"results":[]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	app := &App{fetcher: fetch.NewClient("Rill/test"), xiaoyuzhouBase: server.URL, appleAPIBase: server.URL}
	candidates, err := app.resolveXiaoyuzhouRef(context.Background(), "5ed33dcc418a84a046f579ec", "", "")
	if err != nil {
		t.Fatalf("resolveXiaoyuzhouRef: %v", err)
	}
	if len(candidates) != 1 || candidates[0].Kind != "rss" || !candidates[0].Partial {
		t.Fatalf("unexpected candidates: %+v", candidates)
	}
	if candidates[0].FeedURL != "xiaoyuzhou://podcast/5ed33dcc418a84a046f579ec" {
		t.Fatalf("unexpected feed url: %s", candidates[0].FeedURL)
	}
}
