package server

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
)

const sampleRSS = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0"><channel>
<title>Local Feed</title><link>%s/</link><description>Local test feed</description><image><url>/image.png</url><title>Local Feed</title><link>%s/</link></image>
<item>
  <guid>post-1</guid><title>First post</title><link>/post/1</link>
  <pubDate>Mon, 27 Oct 2026 10:00:00 +0000</pubDate>
  <description><![CDATA[<p>Hello <strong>world</strong>.<script>alert(1)</script></p><p><img src="/image.png" onerror="steal()" alt="pic"></p><p><a href="javascript:alert(2)">bad link</a></p>]]></description>
</item>
<item>
  <guid>post-2</guid><title>Second post</title><link>/post/2</link>
  <pubDate>Mon, 26 Oct 2026 10:00:00 +0000</pubDate>
  <description><![CDATA[<p>Second body.</p>]]></description>
</item>
</channel></rss>`

const podcastRSS = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:itunes="http://www.itunes.com/dtds/podcast-1.0.dtd"><channel>
<title>Local Podcast</title><link>%s/</link><description>Local podcast feed</description>
<item>
  <guid>episode-1</guid><title>Episode One</title><link>/podcast/1</link>
  <pubDate>Mon, 27 Oct 2026 10:00:00 +0000</pubDate>
  <description><![CDATA[<p>Show notes for episode one.</p>]]></description>
  <enclosure url="%s/audio/episode-1.mp3" type="audio/mpeg" length="123456"/>
</item>
</channel></rss>`

func feedTestServer(t *testing.T) (*httptest.Server, *int) {
	t.Helper()
	var mu sync.Mutex
	conditionalRequests := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/feed.xml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
		fmt.Fprintf(w, sampleRSS, "http://"+r.Host, "http://"+r.Host)
	})
	mux.HandleFunc("/podcast.xml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
		fmt.Fprintf(w, podcastRSS, "http://"+r.Host, "http://"+r.Host)
	})
	mux.HandleFunc("/conditional.xml", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		conditionalRequests++
		count := conditionalRequests
		mu.Unlock()
		if r.Header.Get("If-None-Match") == `"local-v1"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
		w.Header().Set("ETag", `"local-v1"`)
		fmt.Fprintf(w, sampleRSS, "http://"+r.Host, "http://"+r.Host)
		_ = count
	})
	png, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9Wl2n4sAAAAASUVORK5CYII=")
	mux.HandleFunc("/image.png", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(png)
	})
	mux.HandleFunc("/favicon.ico", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(png)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server, &conditionalRequests
}

func createTestSource(t *testing.T, client *testClient, name, feedURL string) Source {
	t.Helper()
	var source Source
	client.decode(http.MethodPost, "/api/v1/sources", map[string]any{
		"kind":     "rss",
		"name":     name,
		"feed_url": feedURL,
	}, true, http.StatusCreated, &source)
	return source
}

func TestCreateSourceFetchesImmediately(t *testing.T) {
	_, client := newTestAppWithInitialFetch(t, true)
	client.login()
	feedServer, _ := feedTestServer(t)

	source := createTestSource(t, client, "Auto sync", feedServer.URL+"/feed.xml")
	if source.ItemCount != 2 {
		t.Fatalf("initial fetch item count=%d, want 2", source.ItemCount)
	}
	if source.LastStatus == nil || *source.LastStatus != "ok" {
		t.Fatalf("initial fetch status=%v, want ok", source.LastStatus)
	}
}

func TestRSSFetchDedupeSanitizeAndMedia(t *testing.T) {
	_, client := newTestApp(t, true)
	client.login()
	feedServer, _ := feedTestServer(t)
	source := createTestSource(t, client, "Local", feedServer.URL+"/feed.xml")

	var fetched Source
	client.decode(http.MethodPost, "/api/v1/sources/"+itoa(source.ID)+"/fetch", nil, true, http.StatusOK, &fetched)
	if fetched.LastStatus == nil || *fetched.LastStatus != "ok" || fetched.ItemCount != 2 {
		t.Fatalf("first fetch source: %+v", fetched)
	}
	client.decode(http.MethodPost, "/api/v1/sources/"+itoa(source.ID)+"/fetch", nil, true, http.StatusOK, &fetched)
	if fetched.ItemCount != 2 {
		t.Fatalf("dedupe failed, item_count=%d", fetched.ItemCount)
	}

	var stats ReaderStats
	client.decode(http.MethodGet, "/api/v1/items/stats", nil, false, http.StatusOK, &stats)
	if stats.Total != 2 || stats.Unread != 2 || stats.Read != 0 {
		t.Fatalf("stats: %+v", stats)
	}

	var firstPage ItemPage
	client.decode(http.MethodGet, "/api/v1/items?limit=1", nil, false, http.StatusOK, &firstPage)
	if len(firstPage.Items) != 1 || firstPage.NextCursor == nil {
		t.Fatalf("first page: %+v", firstPage)
	}
	if firstPage.Items[0].CoverURL == nil || !strings.HasPrefix(*firstPage.Items[0].CoverURL, "/media/") {
		t.Fatalf("first page cover: %+v", firstPage.Items[0].CoverURL)
	}
	var secondPage ItemPage
	client.decode(http.MethodGet, "/api/v1/items?limit=1&cursor="+url.QueryEscape(*firstPage.NextCursor), nil, false, http.StatusOK, &secondPage)
	if len(secondPage.Items) != 1 || secondPage.Items[0].ID == firstPage.Items[0].ID {
		t.Fatalf("second page: %+v", secondPage)
	}
	if secondPage.NextCursor != nil {
		t.Fatalf("unexpected third cursor: %v", *secondPage.NextCursor)
	}

	var detail ItemDetail
	client.decode(http.MethodGet, "/api/v1/items/"+itoa(firstPage.Items[0].ID), nil, false, http.StatusOK, &detail)
	if !strings.Contains(detail.ContentHTML, "/media/") {
		t.Fatalf("content not rewritten: %s", detail.ContentHTML)
	}
	if !strings.Contains(detail.ContentHTML, `referrerpolicy="no-referrer"`) {
		t.Fatalf("article image missing no-referrer: %s", detail.ContentHTML)
	}
	if strings.Contains(strings.ToLower(detail.ContentHTML), "<script") ||
		strings.Contains(strings.ToLower(detail.ContentHTML), "onerror") ||
		strings.Contains(strings.ToLower(detail.ContentHTML), "javascript:") {
		t.Fatalf("unsafe HTML survived: %s", detail.ContentHTML)
	}
	if !strings.Contains(detail.ContentText, "Hello world") {
		t.Fatalf("content text: %q", detail.ContentText)
	}
	if detail.CoverURL == nil || !strings.HasPrefix(*detail.CoverURL, "/media/") {
		t.Fatalf("detail cover: %+v", detail.CoverURL)
	}
	mediaPattern := regexp.MustCompile(`src="(/media/[^"]+)"`)
	match := mediaPattern.FindStringSubmatch(detail.ContentHTML)
	if len(match) != 2 {
		t.Fatalf("media URL missing: %s", detail.ContentHTML)
	}
	response, image := client.raw(http.MethodGet, match[1], nil, "", "")
	if response.StatusCode != http.StatusOK || !strings.HasPrefix(response.Header.Get("Content-Type"), "image/png") || len(image) == 0 {
		t.Fatalf("media proxy: %d %s", response.StatusCode, response.Header.Get("Content-Type"))
	}
	response, image = client.raw(http.MethodGet, match[1], nil, "", "")
	if response.StatusCode != http.StatusOK || len(image) == 0 {
		t.Fatalf("cached media: %d", response.StatusCode)
	}

	var health SourceHealth
	client.decode(http.MethodGet, "/api/v1/sources/"+itoa(source.ID), nil, false, http.StatusOK, &health)
	if health.FeedTokenMasked == "" || strings.Contains(health.FeedPaths.RSS, health.FeedTokenMasked) {
		t.Fatalf("token masking: %+v", health)
	}
	parts := strings.Split(health.FeedPaths.RSS, "/")
	token := strings.TrimSuffix(parts[len(parts)-1], ".xml")
	if len(token) < 20 {
		t.Fatalf("feed token too short: %q", token)
	}
	for _, path := range []string{health.FeedPaths.RSS, health.FeedPaths.Atom, health.FeedPaths.JSON} {
		response, body := client.raw(http.MethodGet, path, nil, "", "")
		if response.StatusCode != http.StatusOK {
			t.Fatalf("feed %s status %d: %s", path, response.StatusCode, body)
		}
		if !bytes.Contains(body, []byte("rill:")) && !bytes.Contains(body, []byte("post-1")) {
			t.Fatalf("feed %s missing stable guid: %s", path, body)
		}
	}
	response, _ = client.raw(http.MethodGet, health.FeedPaths.RSS, nil, "", "")
	etag := response.Header.Get("ETag")
	if etag == "" {
		t.Fatal("feed ETag missing")
	}
	request, _ := http.NewRequest(http.MethodGet, client.base+health.FeedPaths.RSS, nil)
	request.Header.Set("If-None-Match", etag)
	conditional, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = conditional.Body.Close()
	if conditional.StatusCode != http.StatusNotModified {
		t.Fatalf("conditional feed status %d", conditional.StatusCode)
	}

	var reset TokenResetResponse
	client.decode(http.MethodPost, "/api/v1/sources/"+itoa(source.ID)+"/token/reset", nil, true, http.StatusOK, &reset)
	oldPath := health.FeedPaths.RSS
	response, _ = client.raw(http.MethodGet, oldPath, nil, "", "")
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("old feed token should be invalid: %d", response.StatusCode)
	}
	client.decode(http.MethodGet, "/api/v1/sources/"+itoa(source.ID), nil, false, http.StatusOK, &health)
	response, _ = client.raw(http.MethodGet, health.FeedPaths.RSS, nil, "", "")
	if response.StatusCode != http.StatusOK {
		t.Fatalf("new feed token: %d", response.StatusCode)
	}
}

func TestConditionalFetchAndLogs(t *testing.T) {
	_, client := newTestApp(t, true)
	client.login()
	feedServer, requests := feedTestServer(t)
	source := createTestSource(t, client, "Conditional", feedServer.URL+"/conditional.xml")

	var fetched Source
	client.decode(http.MethodPost, "/api/v1/sources/"+itoa(source.ID)+"/fetch", nil, true, http.StatusOK, &fetched)
	if fetched.LastStatus == nil || *fetched.LastStatus != "ok" || fetched.ItemCount != 2 {
		t.Fatalf("first conditional fetch: %+v", fetched)
	}
	client.decode(http.MethodPost, "/api/v1/sources/"+itoa(source.ID)+"/fetch", nil, true, http.StatusOK, &fetched)
	if fetched.LastStatus == nil || *fetched.LastStatus != "not_modified" || fetched.ItemCount != 2 {
		t.Fatalf("second conditional fetch: %+v", fetched)
	}
	if *requests < 2 {
		t.Fatalf("expected conditional requests, got %d", *requests)
	}
	var logs []FetchLog
	client.decode(http.MethodGet, "/api/v1/sources/"+itoa(source.ID)+"/logs?limit=10", nil, false, http.StatusOK, &logs)
	if len(logs) != 2 || logs[0].Result != "not_modified" || logs[0].HTTPStatus == nil || *logs[0].HTTPStatus != http.StatusNotModified {
		t.Fatalf("fetch logs: %+v", logs)
	}
}

func TestFetchFailureLogsAndBackoff(t *testing.T) {
	_, client := newTestApp(t, true)
	client.login()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "upstream exploded", http.StatusInternalServerError)
	}))
	defer upstream.Close()
	source := createTestSource(t, client, "Broken", upstream.URL+"/feed.xml")
	response, data := client.request(http.MethodPost, "/api/v1/sources/"+itoa(source.ID)+"/fetch", nil, true)
	if response.StatusCode != http.StatusBadGateway || errorCode(t, data) != "fetch_failed" {
		t.Fatalf("failed fetch response: %d %s", response.StatusCode, data)
	}
	var health SourceHealth
	client.decode(http.MethodGet, "/api/v1/sources/"+itoa(source.ID), nil, false, http.StatusOK, &health)
	if health.Source.FailCount != 1 || health.Source.LastStatus == nil || *health.Source.LastStatus != "error" {
		t.Fatalf("source failure state: %+v", health.Source)
	}
	if len(health.Logs) != 1 || health.Logs[0].Result != "error" ||
		health.Logs[0].HTTPStatus == nil || *health.Logs[0].HTTPStatus != http.StatusInternalServerError {
		t.Fatalf("failure logs: %+v", health.Logs)
	}
}

func TestItemsBatchReadMarkAllAndStats(t *testing.T) {
	_, client := newTestApp(t, true)
	client.login()
	feedServer, _ := feedTestServer(t)
	source := createTestSource(t, client, "Local", feedServer.URL+"/feed.xml")
	client.decode(http.MethodPost, "/api/v1/sources/"+itoa(source.ID)+"/fetch", nil, true, http.StatusOK, nil)
	var page ItemPage
	client.decode(http.MethodGet, "/api/v1/items", nil, false, http.StatusOK, &page)
	if len(page.Items) != 2 {
		t.Fatalf("items: %+v", page)
	}
	ids := []int64{page.Items[0].ID, page.Items[1].ID}
	var batch BatchStateResponse
	client.decode(http.MethodPatch, "/api/v1/items/state", map[string]any{"ids": ids, "read": true}, true, http.StatusOK, &batch)
	if batch.Updated != 2 || len(batch.IDs) != 2 {
		t.Fatalf("batch: %+v", batch)
	}
	var stats ReaderStats
	client.decode(http.MethodGet, "/api/v1/items/stats", nil, false, http.StatusOK, &stats)
	if stats.Read != 2 || stats.Unread != 0 {
		t.Fatalf("stats after batch: %+v", stats)
	}
	client.decode(http.MethodPatch, "/api/v1/items/state", map[string]any{"ids": ids, "read": false}, true, http.StatusOK, &batch)
	client.decode(http.MethodGet, "/api/v1/items/stats", nil, false, http.StatusOK, &stats)
	if stats.Unread != 2 {
		t.Fatalf("stats after undo: %+v", stats)
	}
	client.decode(http.MethodPost, "/api/v1/items/mark-all-read", map[string]any{"source_id": source.ID}, true, http.StatusOK, &batch)
	if batch.Updated != 2 {
		t.Fatalf("mark all: %+v", batch)
	}
	client.decode(http.MethodGet, "/api/v1/items/stats", nil, false, http.StatusOK, &stats)
	if stats.Unread != 0 || stats.Read != 2 {
		t.Fatalf("stats after mark-all: %+v", stats)
	}
}

func TestSourceIconProxyAndCache(t *testing.T) {
	_, client := newTestApp(t, true)
	client.login()
	feedServer, _ := feedTestServer(t)
	source := createTestSource(t, client, "Icon test", feedServer.URL+"/feed.xml")
	client.request(http.MethodPatch, "/api/v1/sources/"+strconv.FormatInt(source.ID, 10), map[string]any{
		"site_url": feedServer.URL,
	}, true)
	client.request(http.MethodPost, "/api/v1/sources/"+strconv.FormatInt(source.ID, 10)+"/fetch", nil, true)
	if source.IconURL == nil || *source.IconURL != "/api/v1/sources/"+strconv.FormatInt(source.ID, 10)+"/icon" {
		t.Fatalf("icon path: %v", source.IconURL)
	}
	response, body := client.raw(http.MethodGet, *source.IconURL, nil, "", "")
	if response.StatusCode != http.StatusOK || response.Header.Get("Content-Type") != "image/png" || len(body) == 0 {
		t.Fatalf("source icon: %d %q %d", response.StatusCode, response.Header.Get("Content-Type"), len(body))
	}
	// The second request must be served from the local cache.
	response, body = client.raw(http.MethodGet, *source.IconURL, nil, "", "")
	if response.StatusCode != http.StatusOK || len(body) == 0 {
		t.Fatalf("cached source icon: %d %d", response.StatusCode, len(body))
	}
}

func TestPodcastEnclosureAndFeedOutput(t *testing.T) {
	_, client := newTestApp(t, true)
	client.login()
	feedServer, _ := feedTestServer(t)
	source := createTestSource(t, client, "Podcast test", feedServer.URL+"/podcast.xml")
	client.decode(http.MethodPost, "/api/v1/sources/"+itoa(source.ID)+"/fetch", nil, true, http.StatusOK, nil)

	var page ItemPage
	client.decode(http.MethodGet, "/api/v1/items?limit=1", nil, false, http.StatusOK, &page)
	if len(page.Items) != 1 {
		t.Fatalf("podcast items: %+v", page)
	}
	var detail ItemDetail
	client.decode(http.MethodGet, "/api/v1/items/"+itoa(page.Items[0].ID), nil, false, http.StatusOK, &detail)
	if detail.AudioURL != feedServer.URL+"/audio/episode-1.mp3" || detail.AudioType != "audio/mpeg" {
		t.Fatalf("podcast audio: url=%q type=%q", detail.AudioURL, detail.AudioType)
	}

	progressResponse, progressBody := client.request(http.MethodPut, "/api/v1/items/"+itoa(page.Items[0].ID)+"/podcast-progress", map[string]any{
		"position_seconds": 42.5,
		"duration_seconds": 120.5,
	}, true)
	if progressResponse.StatusCode != http.StatusNoContent {
		t.Fatalf("save podcast progress: %d %s", progressResponse.StatusCode, progressBody)
	}
	client.decode(http.MethodGet, "/api/v1/items/"+itoa(page.Items[0].ID), nil, false, http.StatusOK, &detail)
	if detail.PodcastPositionSeconds != 42.5 || detail.PodcastDurationSeconds != 120.5 {
		t.Fatalf("podcast progress: position=%v duration=%v", detail.PodcastPositionSeconds, detail.PodcastDurationSeconds)
	}
	progressResponse, _ = client.request(http.MethodPut, "/api/v1/items/"+itoa(page.Items[0].ID)+"/podcast-progress", map[string]any{
		"position_seconds": -1,
	}, true)
	if progressResponse.StatusCode != http.StatusBadRequest {
		t.Fatalf("negative podcast progress: %d", progressResponse.StatusCode)
	}

	var health SourceHealth
	client.decode(http.MethodGet, "/api/v1/sources/"+itoa(source.ID), nil, false, http.StatusOK, &health)
	response, body := client.raw(http.MethodGet, health.FeedPaths.RSS, nil, "", "")
	if response.StatusCode != http.StatusOK || !bytes.Contains(body, []byte(`<enclosure url="`+feedServer.URL+`/audio/episode-1.mp3" type="audio/mpeg" length="0"/>`)) {
		t.Fatalf("RSS podcast enclosure: %d %s", response.StatusCode, body)
	}
	response, body = client.raw(http.MethodGet, health.FeedPaths.JSON, nil, "", "")
	if response.StatusCode != http.StatusOK || !bytes.Contains(body, []byte(`"mime_type": "audio/mpeg"`)) {
		t.Fatalf("JSON podcast attachment: %d %s", response.StatusCode, body)
	}
}
