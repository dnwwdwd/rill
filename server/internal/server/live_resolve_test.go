//go:build live

package server

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"rill/internal/fetch"
)

func transientNetworkError(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	for _, marker := range []string{"eof", "timeout", "tls", "connection refused", "connection reset", "请求失败"} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

// Run with:
//
//	go test -tags live ./internal/server -run TestLivePlatformResolvers -v
//
// This test intentionally uses the public internet; it is excluded from the
// default test suite and its results are recorded in docs/RSS-SOURCES.md.
func TestLivePlatformResolvers(t *testing.T) {
	app := &App{
		fetcher:                      fetch.NewClient("Rill/0.1 (live verification)"),
		appleAPIBase:                 "https://itunes.apple.com",
		xiaoyuzhouBase:               "https://www.xiaoyuzhoufm.com",
		weiboBase:                    "https://m.weibo.cn",
		weiboVisitorBase:             "https://visitor.passport.weibo.cn",
		bestblogsCatalogBase:         bestBlogsCatalogBase,
		bestblogsCatalogFallbackBase: bestBlogsCatalogFallbackBase,
		nowFn:                        time.Now,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	t.Run("apple", func(t *testing.T) {
		candidates, err := app.resolveAppleByID(ctx, "1651572616")
		if err != nil {
			t.Fatalf("resolveAppleByID: %v", err)
		}
		if len(candidates) != 1 || candidates[0].FeedURL == "" {
			t.Fatalf("unexpected candidates: %+v", candidates)
		}
		title, link, _, err := app.fetchFeedMeta(ctx, candidates[0].FeedURL)
		if err != nil {
			if transientNetworkError(err) {
				t.Skipf("apple feed transient network error: %v", err)
			}
			t.Fatalf("fetchFeedMeta(%s): %v", candidates[0].FeedURL, err)
		}
		t.Logf("apple: %s | %s | %s", candidates[0].Name, candidates[0].FeedURL, title)
		if link == "" {
			t.Fatal("feed link is empty")
		}
	})

	t.Run("xiaoyuzhou_feed", func(t *testing.T) {
		candidates, err := app.resolveXiaoyuzhouRef(ctx, "", "", "j7d9mlny9ea6")
		if err != nil {
			t.Fatalf("resolveXiaoyuzhouRef feed: %v", err)
		}
		if len(candidates) != 1 || candidates[0].Kind != "rss" {
			t.Fatalf("unexpected candidates: %+v", candidates)
		}
		t.Logf("xiaoyuzhou feed: %s | %s", candidates[0].Name, candidates[0].FeedURL)
	})

	t.Run("xiaoyuzhou_page", func(t *testing.T) {
		data, err := app.fetchXiaoyuzhouPage(ctx, "podcast", "5ed33dcc418a84a046f579ec")
		if err != nil {
			t.Fatalf("fetchXiaoyuzhouPage: %v", err)
		}
		if data.Props.PageProps.Podcast == nil {
			t.Fatal("podcast is nil")
		}
		t.Logf("xiaoyuzhou page: %s | episodes=%d", data.Props.PageProps.Podcast.Title, len(data.Props.PageProps.Podcast.Episodes))
	})

	t.Run("xiaoyuzhou_discovery", func(t *testing.T) {
		candidates, err := app.resolveXiaoyuzhouRef(ctx, "634270332a992d56e91e5892", "", "")
		if err != nil {
			t.Fatalf("resolveXiaoyuzhouRef discovery: %v", err)
		}
		if len(candidates) != 1 {
			t.Fatalf("unexpected candidates: %+v", candidates)
		}
		t.Logf("xiaoyuzhou discovery: %s | %s | %s", candidates[0].Name, candidates[0].Kind, candidates[0].FeedURL)
	})

	t.Run("bare_ids", func(t *testing.T) {
		weibo, recognized, err := app.resolveSourceInput(ctx, "2803301701")
		if err != nil || !recognized || len(weibo) != 1 || weibo[0].Platform != "weibo" {
			t.Fatalf("bare weibo id: recognized=%v candidates=%+v err=%v", recognized, weibo, err)
		}
		apple, recognized, err := app.resolveSourceInput(ctx, "1651572616")
		if err != nil || !recognized || len(apple) != 1 || apple[0].Platform != "apple" {
			t.Fatalf("bare apple id: recognized=%v candidates=%+v err=%v", recognized, apple, err)
		}
		t.Logf("bare ids: weibo=%s apple=%s", weibo[0].FeedURL, apple[0].FeedURL)
	})

	t.Run("weibo_name", func(t *testing.T) {
		candidates, _, err := app.resolveWeiboRef(ctx, weiboRef{Name: "人民日报"})
		if err != nil {
			t.Fatalf("resolveWeiboRef name: %v", err)
		}
		if len(candidates) != 1 || candidates[0].Name != "人民日报" {
			t.Fatalf("unexpected candidates: %+v", candidates)
		}
		t.Logf("weibo name: %s | %s", candidates[0].Name, candidates[0].FeedURL)
	})

	t.Run("weibo_post", func(t *testing.T) {
		candidates, _, err := app.resolveWeiboRef(ctx, weiboRef{PostID: "RpwSofKOl"})
		if err != nil {
			t.Fatalf("resolveWeiboRef post: %v", err)
		}
		if len(candidates) != 1 || candidates[0].FeedURL != "weibo://user/2803301701" {
			t.Fatalf("unexpected candidates: %+v", candidates)
		}
		t.Logf("weibo post: %s | %s", candidates[0].Name, candidates[0].FeedURL)
	})

	t.Run("weibo", func(t *testing.T) {
		candidates, err := app.resolveWeiboProfile(ctx, "2803301701")
		if err != nil {
			t.Fatalf("resolveWeiboProfile: %v", err)
		}
		if len(candidates) != 1 {
			t.Fatalf("unexpected candidates: %+v", candidates)
		}
		result, err := app.fetchWeibo(ctx, dbSource{
			Source:    Source{Name: candidates[0].Name, FeedURL: candidates[0].FeedURL},
			FeedToken: "live",
		})
		if err != nil {
			t.Fatalf("fetchWeibo: %v", err)
		}
		t.Logf("weibo: %s | items=%d", candidates[0].Name, len(result.Items))
	})

	t.Run("medium", func(t *testing.T) {
		feedURL, name, ok := mediumFeedURL("https://medium.com/@zhgchgli")
		if !ok {
			t.Fatal("mediumFeedURL did not match")
		}
		title, _, _, err := app.fetchFeedMeta(ctx, feedURL)
		if err != nil {
			if transientNetworkError(err) {
				t.Skipf("medium feed transient network error: %v", err)
			}
			t.Fatalf("fetchFeedMeta: %v", err)
		}
		t.Logf("medium: %s | %s | %s", name, feedURL, title)
	})

	t.Run("mastodon", func(t *testing.T) {
		candidate, ok := app.resolveMastodon(ctx, "https://mastodon.social/@Mastodon")
		if !ok {
			title, link, _, err := app.fetchFeedMeta(ctx, "https://mastodon.social/@Mastodon.rss")
			if transientNetworkError(err) {
				t.Skipf("mastodon feed transient network error: %v", err)
			}
			t.Fatalf("resolveMastodon did not match; feedMeta title=%q link=%q err=%v", title, link, err)
		}
		t.Logf("mastodon: %s | %s", candidate.Name, candidate.FeedURL)
	})

	t.Run("bestblogs_catalog", func(t *testing.T) {
		cases := []struct {
			name     string
			platform string
		}{
			{"数字生命卡兹克", "bestblogs"},
			{"OpenAI", "bestblogs-x"},
			{"科技早知道", "bestblogs-podcast"},
			{"Andrej Karpathy", "bestblogs-youtube"},
		}
		for _, test := range cases {
			t.Run(test.platform, func(t *testing.T) {
				candidates, _, err := app.resolveBestBlogsByName(ctx, test.name)
				if err != nil {
					t.Fatalf("resolveBestBlogsByName(%q): %v", test.name, err)
				}
				var selected *ResolveCandidate
				for i := range candidates {
					if candidates[i].Platform == test.platform {
						selected = &candidates[i]
						break
					}
				}
				if selected == nil {
					t.Fatalf("%q: no %s candidate in %+v", test.name, test.platform, candidates)
				}
				title, _, _, err := app.fetchFeedMeta(ctx, selected.FeedURL)
				if err != nil {
					t.Logf("%s: feed metadata unavailable: %v", selected.Name, err)
				}
				t.Logf("%s: name=%s feed=%s title=%q", test.platform, selected.Name, selected.FeedURL, title)
			})
		}
	})

	t.Run("bestblogs_end_to_end", func(t *testing.T) {
		_, client := newTestApp(t, false)
		client = client.login()
		cases := []struct {
			label    string
			input    string
			platform string
		}{
			{"wechat", "bestblogs:数字生命卡兹克", "bestblogs"},
			{"x", "bestblogs:OpenAI", "bestblogs-x"},
		}
		for _, test := range cases {
			t.Run(test.label, func(t *testing.T) {
				var resolved struct {
					Candidates []ResolveCandidate `json:"candidates"`
				}
				client.decode(http.MethodPost, "/api/v1/sources/resolve", map[string]any{"input": test.input}, true, http.StatusOK, &resolved)
				if len(resolved.Candidates) == 0 {
					t.Fatalf("no resolve candidates for %s", test.input)
				}
				var selected *ResolveCandidate
				for i := range resolved.Candidates {
					if resolved.Candidates[i].Platform == test.platform {
						selected = &resolved.Candidates[i]
						break
					}
				}
				if selected == nil {
					t.Fatalf("%s: no %s candidate in %+v", test.input, test.platform, resolved.Candidates)
				}
				var source Source
				client.decode(http.MethodPost, "/api/v1/sources", map[string]any{
					"kind":     selected.Kind,
					"name":     selected.Name,
					"feed_url": selected.FeedURL,
				}, true, http.StatusCreated, &source)
				client.decode(http.MethodPost, "/api/v1/sources/"+itoa(source.ID)+"/fetch", nil, true, http.StatusOK, nil)
				var items ItemPage
				client.decode(http.MethodGet, "/api/v1/items?source_id="+itoa(source.ID)+"&limit=5", nil, false, http.StatusOK, &items)
				if len(items.Items) == 0 {
					t.Fatalf("%s: no items after fetch", test.label)
				}
				t.Logf("%s: resolved=%s feed=%s items=%d first=%s", test.label, selected.Platform, selected.FeedURL, len(items.Items), items.Items[0].Title)
			})
		}
	})

	t.Run("end_to_end", func(t *testing.T) {
		_, client := newTestApp(t, false)
		client = client.login()
		cases := []struct {
			label   string
			input   string
			kind    string
			name    string
			feedURL string
		}{
			{"apple", "https://podcasts.apple.com/cn/podcast/id1651572616", "rss", "这集我看过", "https://feed.xyzfm.space/j7d9mlny9ea6"},
			{"xiaoyuzhou", "https://www.xiaoyuzhoufm.com/podcast/5ed33dcc418a84a046f579ec", "rss", "Echo.js", "xiaoyuzhou://podcast/5ed33dcc418a84a046f579ec"},
			{"weibo", "https://weibo.com/u/2803301701", "rss", "人民日报", "weibo://user/2803301701"},
		}
		for _, test := range cases {
			t.Run(test.label, func(t *testing.T) {
				var resolved struct {
					Candidates []ResolveCandidate `json:"candidates"`
				}
				client.decode(http.MethodPost, "/api/v1/sources/resolve", map[string]any{"input": test.input}, true, http.StatusOK, &resolved)
				if len(resolved.Candidates) == 0 {
					t.Fatalf("no resolve candidates for %s", test.label)
				}
				candidate := resolved.Candidates[0]
				var source Source
				client.decode(http.MethodPost, "/api/v1/sources", map[string]any{
					"kind":     test.kind,
					"name":     test.name,
					"feed_url": test.feedURL,
				}, true, http.StatusCreated, &source)
				client.decode(http.MethodPost, "/api/v1/sources/"+itoa(source.ID)+"/fetch", nil, true, http.StatusOK, nil)
				var items ItemPage
				client.decode(http.MethodGet, "/api/v1/items?source_id="+itoa(source.ID)+"&limit=5", nil, false, http.StatusOK, &items)
				if len(items.Items) == 0 {
					t.Fatalf("%s: no items after fetch", test.label)
				}
				t.Logf("%s: candidate=%s/%s items=%d first=%s", test.label, candidate.Platform, candidate.Kind, len(items.Items), items.Items[0].Title)
			})
		}
	})
}
