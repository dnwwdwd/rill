package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFoldersAndSourcesCRUD(t *testing.T) {
	_, client := newTestApp(t, true)
	client.login()

	var folder Folder
	client.decode(http.MethodPost, "/api/v1/folders", map[string]any{"name": "Tech"}, true, http.StatusCreated, &folder)

	var source Source
	client.decode(http.MethodPost, "/api/v1/sources", map[string]any{
		"kind":      "rss",
		"name":      "Example",
		"feed_url":  "https://example.com/feed.xml",
		"site_url":  "https://example.com",
		"folder_id": folder.ID,
	}, true, http.StatusCreated, &source)
	if source.ID == 0 || source.FolderID == nil || *source.FolderID != folder.ID {
		t.Fatalf("created source: %+v", source)
	}
	if !source.Enabled || source.RefreshSeconds != 1800 || source.InitialLimit != 100 {
		t.Fatalf("source defaults: %+v", source)
	}

	var folders []Folder
	client.decode(http.MethodGet, "/api/v1/folders", nil, false, http.StatusOK, &folders)
	if len(folders) != 1 || folders[0].SourceCount != 1 {
		t.Fatalf("folders: %+v", folders)
	}

	var sources []Source
	client.decode(http.MethodGet, "/api/v1/sources", nil, false, http.StatusOK, &sources)
	if len(sources) != 1 || sources[0].ID != source.ID {
		t.Fatalf("sources: %+v", sources)
	}

	var health SourceHealth
	client.decode(http.MethodGet, "/api/v1/sources/"+itoa(source.ID), nil, false, http.StatusOK, &health)
	if health.Source.ID != source.ID || health.FeedTokenMasked == "" || !strings.Contains(health.FeedPaths.RSS, ".xml") {
		t.Fatalf("health: %+v", health)
	}
	if health.Logs == nil || len(health.Logs) != 0 {
		t.Fatalf("expected empty logs, got %+v", health.Logs)
	}

	var updated Source
	client.decode(http.MethodPatch, "/api/v1/sources/"+itoa(source.ID), map[string]any{
		"name":            "Renamed",
		"enabled":         false,
		"refresh_seconds": 3600,
	}, true, http.StatusOK, &updated)
	if updated.Name != "Renamed" || updated.Enabled || updated.RefreshSeconds != 3600 {
		t.Fatalf("updated source: %+v", updated)
	}

	response, _ := client.request(http.MethodDelete, "/api/v1/folders/"+itoa(folder.ID), nil, true)
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("delete folder status %d", response.StatusCode)
	}
	client.decode(http.MethodGet, "/api/v1/sources/"+itoa(source.ID), nil, false, http.StatusOK, &health)
	if health.Source.FolderID != nil {
		t.Fatalf("source should fall back to ungrouped: %+v", health.Source)
	}

	response, _ = client.request(http.MethodDelete, "/api/v1/sources/"+itoa(source.ID), nil, true)
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("delete source status %d", response.StatusCode)
	}
	response, data := client.request(http.MethodGet, "/api/v1/sources/"+itoa(source.ID), nil, false)
	if response.StatusCode != http.StatusNotFound || errorCode(t, data) != "source_not_found" {
		t.Fatalf("deleted source: %d %s", response.StatusCode, data)
	}
}

func TestSourceValidationAndDuplicate(t *testing.T) {
	_, client := newTestApp(t, true)
	client.login()
	var xSource Source
	client.decode(http.MethodPost, "/api/v1/sources", map[string]any{
		"kind": "x", "handle": "example_user",
	}, true, http.StatusCreated, &xSource)
	if xSource.Kind != "x" || xSource.Name != "@example_user" ||
		!strings.Contains(xSource.FeedURL, "fxtwitter.com/example_user/feed.xml") {
		t.Fatalf("x source: %+v", xSource)
	}
	body := map[string]any{"kind": "rss", "name": "A", "feed_url": "https://example.com/feed.xml"}
	var source Source
	client.decode(http.MethodPost, "/api/v1/sources", body, true, http.StatusCreated, &source)
	response, data := client.request(http.MethodPost, "/api/v1/sources", body, true)
	if response.StatusCode != http.StatusConflict || errorCode(t, data) != "source_exists" {
		t.Fatalf("duplicate source: %d %s", response.StatusCode, data)
	}
	response, data = client.request(http.MethodPost, "/api/v1/sources", map[string]any{
		"kind": "rss", "name": "bad", "feed_url": "ftp://example.com/feed.xml",
	}, true)
	if response.StatusCode != http.StatusBadRequest || errorCode(t, data) != "url_blocked" {
		t.Fatalf("bad scheme: %d %s", response.StatusCode, data)
	}
}

func TestUpdateSourceFeedURLRefreshesMetadataAndFetches(t *testing.T) {
	_, client := newTestAppWithInitialFetch(t, true)
	client.login()

	writeFeed := func(title, itemTitle, guid string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
			fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0"><channel>
<title>%s</title><link>http://%s/site</link><description>test</description>
<item><guid>%s</guid><title>%s</title><link>http://%s/post</link></item>
</channel></rss>`, title, r.Host, guid, itemTitle, r.Host)
		}
	}
	oldServer := httptest.NewServer(writeFeed("Old Feed", "Old item", "old-1"))
	t.Cleanup(oldServer.Close)
	newServer := httptest.NewServer(writeFeed("Updated Feed", "Fresh item", "new-1"))
	t.Cleanup(newServer.Close)

	source := createTestSource(t, client, "Custom name", oldServer.URL+"/feed.xml")
	var updated Source
	client.decode(http.MethodPatch, "/api/v1/sources/"+itoa(source.ID), map[string]any{
		"feed_url": newServer.URL + "/feed.xml",
	}, true, http.StatusOK, &updated)
	if updated.FeedURL != newServer.URL+"/feed.xml" {
		t.Fatalf("feed_url = %q", updated.FeedURL)
	}
	if updated.Name != "Updated Feed" {
		t.Fatalf("name = %q, want feed title", updated.Name)
	}
	if updated.SiteURL == nil || *updated.SiteURL != newServer.URL+"/site" {
		t.Fatalf("site_url = %v, want new feed link", updated.SiteURL)
	}
	if updated.LastStatus == nil || *updated.LastStatus != "ok" {
		t.Fatalf("fetch status = %v, want ok", updated.LastStatus)
	}

	var page ItemPage
	client.decode(http.MethodGet, "/api/v1/items?source_id="+itoa(source.ID), nil, false, http.StatusOK, &page)
	found := false
	for _, item := range page.Items {
		if item.Title == "Fresh item" {
			found = true
		}
	}
	if !found {
		t.Fatalf("new feed item not fetched: %+v", page.Items)
	}
}

func TestUpdateSourceFeedURLValidation(t *testing.T) {
	_, client := newTestApp(t, true)
	client.login()

	first := createTestSource(t, client, "First", "https://example.com/one.xml")
	second := createTestSource(t, client, "Second", "https://example.com/two.xml")

	response, data := client.request(http.MethodPatch, "/api/v1/sources/"+itoa(second.ID), map[string]any{
		"feed_url": first.FeedURL,
	}, true)
	if response.StatusCode != http.StatusConflict || errorCode(t, data) != "source_exists" {
		t.Fatalf("duplicate URL: %d %s", response.StatusCode, data)
	}

	response, data = client.request(http.MethodPatch, "/api/v1/sources/"+itoa(second.ID), map[string]any{
		"feed_url": "ftp://example.com/two.xml",
	}, true)
	if response.StatusCode != http.StatusBadRequest || errorCode(t, data) != "invalid_url" {
		t.Fatalf("invalid URL: %d %s", response.StatusCode, data)
	}

	email := createTestSource(t, client, "Mailbox", "imap://user@example.com")
	response, data = client.request(http.MethodPatch, "/api/v1/sources/"+itoa(email.ID), map[string]any{
		"feed_url": "https://example.com/three.xml",
	}, true)
	if response.StatusCode != http.StatusBadRequest || errorCode(t, data) != "invalid_source" {
		t.Fatalf("email URL edit: %d %s", response.StatusCode, data)
	}
}

func itoa(value int64) string {
	if value == 0 {
		return "0"
	}
	var buf [20]byte
	index := len(buf)
	negative := value < 0
	if negative {
		value = -value
	}
	for value > 0 {
		index--
		buf[index] = byte('0' + value%10)
		value /= 10
	}
	if negative {
		index--
		buf[index] = '-'
	}
	return string(buf[index:])
}
