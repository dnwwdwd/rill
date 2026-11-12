package server

import (
	"net/http"
	"testing"
)

func TestXHandleAndBskyHandleFromFeedURL(t *testing.T) {
	cases := []struct{ feedURL, xHandle, bskyHandle string }{
		{"https://fxtwitter.com/example_user/feed.xml?count=20", "example_user", ""},
		{"https://fixupx.com/alice/feed.xml", "alice", ""},
		{"https://bsky.app/profile/alice.bsky.social/rss", "", "alice.bsky.social"},
		{"https://bsky.app/profile/alice.bsky.social", "", "alice.bsky.social"},
		{"https://example.com/feed.xml", "", ""},
	}
	for _, tc := range cases {
		if got := xHandleFromFeedURL(tc.feedURL); got != tc.xHandle {
			t.Fatalf("xHandle(%q)=%q want %q", tc.feedURL, got, tc.xHandle)
		}
		if got := bskyHandleFromFeedURL(tc.feedURL); got != tc.bskyHandle {
			t.Fatalf("bskyHandle(%q)=%q want %q", tc.feedURL, got, tc.bskyHandle)
		}
	}
}

func TestSourceSupportsFullBackfill(t *testing.T) {
	if !sourceSupportsFullBackfill("x", "https://fxtwitter.com/a/feed.xml") {
		t.Fatal("x should support backfill")
	}
	if !sourceSupportsFullBackfill("rss", "https://bsky.app/profile/a.bsky.social/rss") {
		t.Fatal("bsky rss should support backfill")
	}
	if sourceSupportsFullBackfill("rss", "https://example.com/feed.xml") {
		t.Fatal("plain rss should not support backfill")
	}
	if sourceSupportsFullBackfill("wechat", "http://sidecar/feed") {
		t.Fatal("wechat should not support backfill")
	}
}

func TestUpdateSourceBackfillMode(t *testing.T) {
	_, client := newTestApp(t, true)
	client.login()
	// 普通 RSS 不允许全量。
	var source Source
	client.decode(http.MethodPost, "/api/v1/sources", map[string]any{
		"kind": "rss", "name": "plain", "feed_url": "https://example.com/feed.xml",
	}, true, http.StatusCreated, &source)
	response, data := client.request(http.MethodPatch, "/api/v1/sources/"+itoa(source.ID), map[string]any{
		"backfill_mode": "full",
	}, true)
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("plain rss full should be rejected: %d %s", response.StatusCode, data)
	}

	// X 源允许；切换到 full 时字段落库。
	var xSource Source
	client.decode(http.MethodPost, "/api/v1/sources", map[string]any{
		"kind": "x", "handle": "alice",
	}, true, http.StatusCreated, &xSource)
	response, data = client.request(http.MethodPatch, "/api/v1/sources/"+itoa(xSource.ID), map[string]any{
		"backfill_mode": "full",
	}, true)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("x full rejected: %d %s", response.StatusCode, data)
	}
	var health SourceHealth
	client.decode(http.MethodGet, "/api/v1/sources/"+itoa(xSource.ID), nil, false, http.StatusOK, &health)
	if health.Source.BackfillMode != "full" {
		t.Fatalf("backfill_mode=%q want full", health.Source.BackfillMode)
	}
	// 非法值拒绝。
	response, _ = client.request(http.MethodPatch, "/api/v1/sources/"+itoa(xSource.ID), map[string]any{
		"backfill_mode": "everything",
	}, true)
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid mode accepted: %d", response.StatusCode)
	}
}
