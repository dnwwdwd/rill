package server

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestApplyRetentionProtectsUnreadAndStarred(t *testing.T) {
	app, client := newTestApp(t, true)
	client.login()
	source := createTestSource(t, client, "Retention", "https://example.com/feed.xml")
	if _, err := app.db.ExecContext(context.Background(), `
		UPDATE sources SET retention_days = 30, retention_max_items = 2 WHERE id = ?`, source.ID); err != nil {
		t.Fatal(err)
	}
	now := app.now()
	insert := func(guid string, published time.Time, read bool, starred bool) int64 {
		t.Helper()
		readAt := any(nil)
		if read {
			readAt = now.Add(-time.Hour).Format(timeRFC3339)
		}
		starredAt := any(nil)
		if starred {
			starredAt = now.Add(-2 * time.Hour).Format(timeRFC3339)
		}
		result, err := app.db.ExecContext(context.Background(), `
			INSERT INTO items (source_id, guid, title, published_at, content_text, read_at, starred_at, fetched_at, updated_at)
			VALUES (?, ?, ?, ?, 'body', ?, ?, ?, ?)`,
			source.ID, guid, guid, published.Format(timeRFC3339), readAt, starredAt,
			now.Format(timeRFC3339), now.Format(timeRFC3339))
		if err != nil {
			t.Fatal(err)
		}
		id, _ := result.LastInsertId()
		return id
	}
	oldRead := insert("old-read", now.AddDate(0, 0, -40), true, false)
	recentRead := insert("recent-read", now.AddDate(0, 0, -1), true, false)
	olderRead := insert("older-read", now.AddDate(0, 0, -2), true, false)
	overflowRead := insert("overflow-read", now.AddDate(0, 0, -3), true, false)
	oldUnread := insert("old-unread", now.AddDate(0, 0, -40), false, false)
	oldStarred := insert("old-starred", now.AddDate(0, 0, -40), true, true)
	deleted, err := app.applyRetention(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 2 {
		t.Fatalf("deleted=%d want=2", deleted)
	}
	for _, id := range []int64{recentRead, olderRead, oldUnread, oldStarred} {
		var count int
		if err := app.db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM items WHERE id = ?`, id).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("item %d should remain", id)
		}
	}
	for _, id := range []int64{oldRead, overflowRead} {
		var count int
		if err := app.db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM items WHERE id = ?`, id).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("item %d should be removed", id)
		}
	}
}

func TestRetentionSettingsRoundTrip(t *testing.T) {
	_, client := newTestApp(t, true)
	client.login()
	var source Source
	client.decode(http.MethodPost, "/api/v1/sources", map[string]any{
		"kind":                "rss",
		"name":                "Retention settings",
		"feed_url":            "https://example.com/retention.xml",
		"retention_days":      30,
		"retention_max_items": 500,
	}, true, http.StatusCreated, &source)
	if source.RetentionDays != 30 || source.RetentionMaxItems != 500 {
		t.Fatalf("created retention: %d/%d", source.RetentionDays, source.RetentionMaxItems)
	}
	var updated Source
	client.decode(http.MethodPatch, "/api/v1/sources/"+itoa(source.ID), map[string]any{
		"retention_days": 0, "retention_max_items": 100,
	}, true, http.StatusOK, &updated)
	if updated.RetentionDays != 0 || updated.RetentionMaxItems != 100 {
		t.Fatalf("updated retention: %d/%d", updated.RetentionDays, updated.RetentionMaxItems)
	}
}
