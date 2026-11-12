package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestCreatorSearchAppleAndBluesky(t *testing.T) {
	appleServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"resultCount": 2,
			"results": []map[string]any{
				{
					"collectionName":    "Jay's Podcast",
					"artistName":        "Jay",
					"feedUrl":           "https://example.com/jay.xml",
					"collectionViewUrl": "https://podcasts.apple.com/podcast/id1",
					"artworkUrl600":     "https://example.com/jay.jpg",
				},
				{
					"collectionName": "Another Jay",
					"feedUrl":        "https://example.com/another.xml",
				},
			},
		})
	}))
	defer appleServer.Close()

	blueskyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/xrpc/app.bsky.actor.searchActors" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"actors": []map[string]any{
				{"handle": "jay.bsky.social", "displayName": "Jay", "avatar": "https://example.com/jay.png"},
				{"handle": "jayden.bsky.social", "displayName": "Jayden"},
			},
		})
	}))
	defer blueskyServer.Close()

	app, client := newTestApp(t, false)
	app.appleAPIBase = appleServer.URL
	app.bskyAppViewBase = blueskyServer.URL
	client.login()

	var response CreatorSearchResponse
	client.decode(http.MethodPost, "/api/v1/sources/search", map[string]any{
		"query":     "jay",
		"platforms": []string{"apple", "bluesky"},
	}, true, http.StatusOK, &response)

	if response.Query != "jay" || len(response.Candidates) != 4 {
		t.Fatalf("unexpected response: %+v", response)
	}
	if len(response.Platforms) != 2 || response.Platforms[0].Platform != "apple" || response.Platforms[1].Platform != "bluesky" {
		t.Fatalf("unexpected platform statuses: %+v", response.Platforms)
	}
	apple := response.Platforms[0]
	if apple.Status != creatorSearchStatusOK || apple.Count != 2 {
		t.Fatalf("apple status=%+v", apple)
	}
	bluesky := response.Platforms[1]
	if bluesky.Status != creatorSearchStatusOK || bluesky.Count != 2 {
		t.Fatalf("bluesky status=%+v", bluesky)
	}
	var appleCandidate, bskyCandidate *CreatorSearchCandidate
	for index := range response.Candidates {
		switch response.Candidates[index].Platform {
		case "apple":
			if appleCandidate == nil {
				appleCandidate = &response.Candidates[index]
			}
		case "bluesky":
			if bskyCandidate == nil || response.Candidates[index].Handle == "jay.bsky.social" {
				bskyCandidate = &response.Candidates[index]
			}
		}
	}
	if appleCandidate == nil || appleCandidate.Match != "prefix" {
		t.Fatalf("apple candidate=%+v", appleCandidate)
	}
	if bskyCandidate == nil || bskyCandidate.Match != "exact" || bskyCandidate.Handle != "jay.bsky.social" || !strings.Contains(bskyCandidate.FeedURL, "bsky.app/profile/") {
		t.Fatalf("bluesky candidate=%+v", bskyCandidate)
	}
}

func TestCreatorSearchKeepsPartialResults(t *testing.T) {
	appleServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer appleServer.Close()
	blueskyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"actors":[{"handle":"alice.bsky.social","displayName":"Alice"}]}`))
	}))
	defer blueskyServer.Close()

	app, client := newTestApp(t, false)
	app.appleAPIBase = appleServer.URL
	app.bskyAppViewBase = blueskyServer.URL
	client.login()

	var response CreatorSearchResponse
	client.decode(http.MethodPost, "/api/v1/sources/search", map[string]any{
		"query":     "alice",
		"platforms": []string{"apple", "bluesky"},
	}, true, http.StatusOK, &response)
	if len(response.Candidates) != 1 || response.Candidates[0].Platform != "bluesky" {
		t.Fatalf("candidates=%+v", response.Candidates)
	}
	if response.Platforms[0].Status != creatorSearchStatusError || response.Platforms[1].Status != creatorSearchStatusOK {
		t.Fatalf("platforms=%+v", response.Platforms)
	}
	if !strings.Contains(response.Platforms[0].Error, "503") {
		t.Fatalf("apple error=%q", response.Platforms[0].Error)
	}
}

func TestCreatorSearchMarksExistingAndRefreshesCache(t *testing.T) {
	var appleRequests atomic.Int32
	appleServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		appleRequests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"resultCount":1,"results":[{"collectionName":"Alice","feedUrl":"https://example.com/alice.xml"}]}`))
	}))
	defer appleServer.Close()

	app, client := newTestApp(t, false)
	app.appleAPIBase = appleServer.URL
	client.login()

	search := func() CreatorSearchResponse {
		t.Helper()
		var response CreatorSearchResponse
		client.decode(http.MethodPost, "/api/v1/sources/search", map[string]any{
			"query":     "alice",
			"platforms": []string{"apple"},
		}, true, http.StatusOK, &response)
		return response
	}

	first := search()
	if len(first.Candidates) != 1 || first.Candidates[0].ExistingSourceID != 0 {
		t.Fatalf("first search=%+v", first)
	}
	var source Source
	client.decode(http.MethodPost, "/api/v1/sources", map[string]any{
		"kind":     "rss",
		"name":     "Alice",
		"feed_url": "https://example.com/alice.xml",
	}, true, http.StatusCreated, &source)

	second := search()
	if len(second.Candidates) != 1 || second.Candidates[0].ExistingSourceID != source.ID {
		t.Fatalf("second search=%+v, source=%+v", second, source)
	}
	var casing CreatorSearchResponse
	client.decode(http.MethodPost, "/api/v1/sources/search", map[string]any{
		"query":     "ALICE",
		"platforms": []string{"apple"},
	}, true, http.StatusOK, &casing)
	if casing.Query != "ALICE" || len(casing.Candidates) != 1 || casing.Candidates[0].ExistingSourceID != source.ID {
		t.Fatalf("case-insensitive cache search=%+v", casing)
	}
	if appleRequests.Load() != 1 {
		t.Fatalf("apple requests=%d, want cache hit on second search", appleRequests.Load())
	}
}

func TestCreatorSearchBestBlogsDirectory(t *testing.T) {
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

	app, client := newTestApp(t, false)
	app.bestblogsCatalogURL = ""
	app.bestblogsCatalogBase = server.URL + "/"
	client.login()

	var response CreatorSearchResponse
	client.decode(http.MethodPost, "/api/v1/sources/search", map[string]any{
		"query":     "数字生命卡兹克",
		"platforms": []string{"bestblogs"},
	}, true, http.StatusOK, &response)
	if len(response.Candidates) != 4 {
		t.Fatalf("candidates=%+v", response.Candidates)
	}
	if response.Platforms[0].Status != creatorSearchStatusOK || response.Platforms[0].Count != 4 {
		t.Fatalf("platforms=%+v", response.Platforms)
	}
	for _, candidate := range response.Candidates {
		if candidate.Source != "bestblogs" || candidate.Kind != "rss" || !strings.HasPrefix(candidate.Platform, "bestblogs") {
			t.Fatalf("candidate=%+v", candidate)
		}
	}
}

func TestCreatorSearchValidation(t *testing.T) {
	_, client := newTestApp(t, false)
	client.login()

	response, data := client.request(http.MethodPost, "/api/v1/sources/search", map[string]any{"query": "  "}, true)
	if response.StatusCode != http.StatusBadRequest || errorCode(t, data) != "query_required" {
		t.Fatalf("status=%d body=%s", response.StatusCode, data)
	}
	response, data = client.request(http.MethodPost, "/api/v1/sources/search", map[string]any{
		"query":     "alice",
		"platforms": []string{"myspace"},
	}, true)
	if response.StatusCode != http.StatusBadRequest || errorCode(t, data) != "invalid_platform" {
		t.Fatalf("status=%d body=%s", response.StatusCode, data)
	}
}

func TestBatchCreateSources(t *testing.T) {
	_, client := newTestApp(t, false)
	client.login()

	var result SourceBatchCreateResponse
	client.decode(http.MethodPost, "/api/v1/sources/batch", map[string]any{
		"sources": []map[string]any{
			{"kind": "rss", "name": "Alice", "feed_url": "https://example.com/alice.xml"},
			{"kind": "rss", "name": "Alice duplicate", "feed_url": "https://example.com/alice.xml"},
			{"kind": "rss", "name": "Bob", "feed_url": "https://example.com/bob.xml"},
		},
	}, true, http.StatusOK, &result)
	if len(result.Created) != 2 || len(result.Errors) != 1 {
		t.Fatalf("batch result=%+v", result)
	}
	if result.Errors[0].Index != 1 || result.Errors[0].Code != "duplicate_source" {
		t.Fatalf("duplicate error=%+v", result.Errors[0])
	}

	var again SourceBatchCreateResponse
	client.decode(http.MethodPost, "/api/v1/sources/batch", map[string]any{
		"sources": []map[string]any{
			{"kind": "rss", "name": "Alice again", "feed_url": "https://example.com/alice.xml"},
		},
	}, true, http.StatusOK, &again)
	if len(again.Created) != 0 || len(again.Errors) != 1 || again.Errors[0].Code != "source_exists" {
		t.Fatalf("second batch=%+v", again)
	}
	if again.Errors[0].ExistingSourceID != result.Created[0].ID {
		t.Fatalf("existing_source_id=%d, want %d", again.Errors[0].ExistingSourceID, result.Created[0].ID)
	}

	var xBatch SourceBatchCreateResponse
	client.decode(http.MethodPost, "/api/v1/sources/batch", map[string]any{
		"sources": []map[string]any{
			{"kind": "x", "handle": "alice_x"},
			{"kind": "x", "handle": "alice_x"},
		},
	}, true, http.StatusOK, &xBatch)
	if len(xBatch.Created) != 1 || len(xBatch.Errors) != 1 || xBatch.Errors[0].Code != "duplicate_source" {
		t.Fatalf("x batch=%+v", xBatch)
	}
}
