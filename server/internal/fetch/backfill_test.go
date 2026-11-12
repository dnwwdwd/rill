package fetch

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFetchXTimelinePagination(t *testing.T) {
	var pages []string
	mux := http.NewServeMux()
	mux.HandleFunc("/2/profile/alice/statuses", func(w http.ResponseWriter, r *http.Request) {
		cursor := r.URL.Query().Get("cursor")
		pages = append(pages, cursor)
		w.Header().Set("Content-Type", "application/json")
		switch cursor {
		case "":
			// 第一页：2 条 + 游标
			payload := map[string]any{
				"code": 200,
				"tweets": []map[string]any{
					{
						"url":        "https://x.com/alice/status/2",
						"text":       "second post",
						"created_at": "Sat Oct 31 05:13:48 +0000 2026",
						"author":     map[string]any{"name": "Alice", "screen_name": "alice"},
					},
					{
						"url":        "https://x.com/alice/status/1",
						"text":       "first post with photo",
						"created_at": "Fri Oct 30 05:13:48 +0000 2026",
						"author":     map[string]any{"screen_name": "alice"},
						"media":      map[string]any{"photos": []map[string]any{{"url": "https://pbs.twimg.com/media/1.jpg", "alt_text": "a photo"}}},
					},
				},
				"cursor": map[string]any{"bottom": "page2"},
			}
			_ = json.NewEncoder(w).Encode(payload)
		default:
			payload := map[string]any{
				"code": 200,
				"tweets": []map[string]any{
					{"url": "https://x.com/alice/status/0", "text": "oldest", "created_at": "Thu Oct 29 05:13:48 +0000 2026"},
				},
				// 没有下一页游标 → 停止
			}
			_ = json.NewEncoder(w).Encode(payload)
		}
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := NewClient("Rill-test/0.1")
	items, err := client.FetchXTimeline(context.Background(), "@alice", server.URL, nil)
	if err != nil {
		t.Fatalf("FetchXTimeline: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("items=%d want 3", len(items))
	}
	// guid=推文链接、作者回退、图片进正文与媒体列表
	if items[0].GUID != "https://x.com/alice/status/2" || items[0].Title != "second post" {
		t.Fatalf("item0: %+v", items[0])
	}
	if items[0].Author != "Alice" {
		t.Fatalf("author=%q", items[0].Author)
	}
	if items[1].Author != "alice" {
		t.Fatalf("fallback author=%q", items[1].Author)
	}
	if !strings.Contains(items[1].ContentHTML, "<img") {
		t.Fatalf("photo missing in html: %s", items[1].ContentHTML)
	}
	if len(items[1].MediaURLs) == 0 || !strings.Contains(items[1].MediaURLs[0], "pbs.twimg.com/media/1.jpg") {
		t.Fatalf("media urls: %v", items[1].MediaURLs)
	}
	if items[0].PublishedAt.Format(time.RFC3339) != "2026-10-31T05:13:48Z" {
		t.Fatalf("published=%v", items[0].PublishedAt)
	}
	// 分页走了两页：第二页带游标 page2
	if len(pages) != 2 || pages[1] != "page2" {
		t.Fatalf("pages=%v", pages)
	}
}

func TestFetchBskyTimelinePagination(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/xrpc/app.bsky.feed.getAuthorFeed", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("actor") != "alice.bsky.social" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		cursor := r.URL.Query().Get("cursor")
		w.Header().Set("Content-Type", "application/json")
		if cursor == "" {
			payload := map[string]any{
				"feed": []map[string]any{
					{"post": map[string]any{
						"uri":    "at://did:plc:abc/app.bsky.feed.post/rkey2",
						"author": map[string]any{"handle": "alice.bsky.social", "displayName": "Alice"},
						"record": map[string]any{"text": "newest post", "createdAt": "2026-10-31T05:00:00.000Z"},
						"embed": map[string]any{
							"$type":  "app.bsky.embed.images",
							"images": []map[string]any{{"thumb": "https://cdn.bsky.app/t.jpg", "fullsize": "https://cdn.bsky.app/f.jpg", "alt": "pic"}},
						},
					}},
				},
				"cursor": "page2",
			}
			_ = json.NewEncoder(w).Encode(payload)
		} else {
			payload := map[string]any{
				"feed": []map[string]any{
					{"post": map[string]any{
						"uri":    "at://did:plc:abc/app.bsky.feed.post/rkey1",
						"author": map[string]any{"handle": "alice.bsky.social"},
						"record": map[string]any{"text": "older post", "createdAt": "2026-10-30T05:00:00.000Z"},
					}},
				},
			}
			_ = json.NewEncoder(w).Encode(payload)
		}
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := NewClient("Rill-test/0.1")
	items, err := client.FetchBskyTimeline(context.Background(), "alice.bsky.social", server.URL, nil)
	if err != nil {
		t.Fatalf("FetchBskyTimeline: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("items=%d want 2", len(items))
	}
	// guid=at:// URI（与官方 RSS 去重键一致）；链接由 handle+rkey 拼出
	if items[0].GUID != "at://did:plc:abc/app.bsky.feed.post/rkey2" {
		t.Fatalf("guid=%q", items[0].GUID)
	}
	if items[0].URL != "https://bsky.app/profile/alice.bsky.social/post/rkey2" {
		t.Fatalf("url=%q", items[0].URL)
	}
	// 图片 fullsize 进媒体列表，正文重写为 /media/ 代理地址
	if !strings.Contains(items[0].ContentHTML, "<img") || !strings.Contains(items[0].ContentHTML, "/media/") {
		t.Fatalf("image missing: %s", items[0].ContentHTML)
	}
	if len(items[0].MediaURLs) == 0 || !strings.Contains(items[0].MediaURLs[0], "https://cdn.bsky.app/f.jpg") {
		t.Fatalf("media urls: %v", items[0].MediaURLs)
	}
	if items[0].PublishedAt.Format(time.RFC3339) != "2026-10-31T05:00:00Z" {
		t.Fatalf("published=%v", items[0].PublishedAt)
	}
}

func TestFetchXTimelineStopsOnEmptyPage(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"tweets":[],"cursor":{"bottom":"should-not-be-used"}}`))
	}))
	defer server.Close()
	client := NewClient("Rill-test/0.1")
	items, err := client.FetchXTimeline(context.Background(), "bob", server.URL, nil)
	if err != nil || len(items) != 0 {
		t.Fatalf("items=%d err=%v", len(items), err)
	}
	if calls != 1 {
		t.Fatalf("calls=%d want 1 (empty page must stop)", calls)
	}
}
