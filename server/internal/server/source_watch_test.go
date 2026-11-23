package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestSourceWatchConfigPersists(t *testing.T) {
	_, client := newTestApp(t, false)
	client.login()
	_, sourceID, _ := seedItem(t, client.app, "关注配置")
	id := itoa(sourceID)

	var updated Source
	client.decode(http.MethodPatch, "/api/v1/sources/"+id, map[string]any{
		"watch_enabled":     true,
		"watch_mode":        "each",
		"watch_ai":          true,
		"watch_channel_ids": []int64{3, 3, 5},
	}, true, http.StatusOK, &updated)
	if !updated.WatchEnabled || updated.WatchMode != "each" || !updated.WatchAI {
		t.Fatalf("watch config not returned: %+v", updated)
	}
	if len(updated.WatchChannelIDs) != 2 || updated.WatchChannelIDs[0] != 3 || updated.WatchChannelIDs[1] != 5 {
		t.Fatalf("watch channel ids not normalized: %+v", updated.WatchChannelIDs)
	}

	var loaded SourceHealth
	client.decode(http.MethodGet, "/api/v1/sources/"+id, nil, false, http.StatusOK, &loaded)
	if !loaded.Source.WatchEnabled || loaded.Source.WatchMode != "each" || !loaded.Source.WatchAI {
		t.Fatalf("watch config not persisted: %+v", loaded.Source)
	}

	client.decode(http.MethodPatch, "/api/v1/sources/"+id, map[string]any{
		"watch_mode": "later",
	}, true, http.StatusBadRequest, nil)

	var settings Settings
	client.decode(http.MethodPatch, "/api/v1/settings", map[string]any{
		"reader": map[string]any{"pinned_sources": []int64{sourceID, sourceID}},
	}, true, http.StatusOK, &settings)
	if len(settings.Reader.PinnedSources) != 1 || settings.Reader.PinnedSources[0] != sourceID {
		t.Fatalf("pinned sources not normalized: %+v", settings.Reader.PinnedSources)
	}
}

func TestNotifyWatchedSourceBatch(t *testing.T) {
	var (
		mu       sync.Mutex
		payloads []map[string]any
	)
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload map[string]any
		_ = json.Unmarshal(body, &payload)
		mu.Lock()
		payloads = append(payloads, payload)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	app, _ := newTestApp(t, false)
	ctx := context.Background()
	_, sourceID, itemID := seedItem(t, app, "关注提醒")
	channelID := seedWatchChannel(t, app, target.URL)
	updateWatchConfig(t, app, sourceID, map[string]any{
		"watch_enabled":     true,
		"watch_mode":        "batch",
		"watch_ai":          false,
		"watch_channel_ids": []int64{channelID},
	})

	app.notifyWatchedSource(ctx, sourceID, []int64{itemID})

	mu.Lock()
	defer mu.Unlock()
	if len(payloads) != 1 {
		t.Fatalf("payloads = %d, want 1", len(payloads))
	}
	content, _ := payloads[0]["content"].(string)
	if !strings.Contains(content, "1 条新内容") || !strings.Contains(content, "测试文章") {
		t.Fatalf("unexpected batch content: %q", content)
	}
	if payloads[0]["title"] != "测试源-关注提醒" {
		t.Fatalf("unexpected title: %v", payloads[0]["title"])
	}

	// 已读条目不再提醒。
	if _, err := app.db.ExecContext(ctx, `UPDATE items SET read_at = ? WHERE id = ?`, app.now().Format(timeRFC3339), itemID); err != nil {
		t.Fatal(err)
	}
	app.notifyWatchedSource(ctx, sourceID, []int64{itemID})
	if len(payloads) != 1 {
		t.Fatalf("read item should not notify, payloads = %d", len(payloads))
	}
}

func TestNotifyWatchedSourceEachMode(t *testing.T) {
	var (
		mu    sync.Mutex
		count int
	)
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		count++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	app, _ := newTestApp(t, false)
	ctx := context.Background()
	_, sourceID, itemID := seedItem(t, app, "逐篇提醒")
	now := app.now().Format(timeRFC3339)
	res, err := app.db.ExecContext(ctx, `
		INSERT INTO items (source_id, guid, title, url, content_text, content_html, fetched_at, updated_at)
		VALUES (?, 'second', '第二篇', 'https://example.com/second', '正文', '<p>正文</p>', ?, ?)`,
		sourceID, now, now)
	if err != nil {
		t.Fatal(err)
	}
	secondID, _ := res.LastInsertId()
	channelID := seedWatchChannel(t, app, target.URL)
	updateWatchConfig(t, app, sourceID, map[string]any{
		"watch_enabled":     true,
		"watch_mode":        "each",
		"watch_ai":          false,
		"watch_channel_ids": []int64{channelID},
	})

	app.notifyWatchedSource(ctx, sourceID, []int64{itemID, secondID})

	mu.Lock()
	defer mu.Unlock()
	if count != 2 {
		t.Fatalf("each mode notifications = %d, want 2", count)
	}
}

func seedWatchChannel(t *testing.T, app *App, url string) int64 {
	t.Helper()
	encrypted, err := app.secret.Encrypt(url)
	if err != nil {
		t.Fatal(err)
	}
	now := app.now().Format(timeRFC3339)
	res, err := app.db.ExecContext(context.Background(), `
		INSERT INTO push_channels (name, kind, url_enc, enabled, created_at, updated_at)
		VALUES ('测试渠道', 'generic_webhook', ?, 1, ?, ?)`, encrypted, now, now)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := res.LastInsertId()
	return id
}

func updateWatchConfig(t *testing.T, app *App, sourceID int64, config map[string]any) {
	t.Helper()
	raw, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.db.ExecContext(context.Background(),
		`UPDATE sources SET config_json = ? WHERE id = ?`, string(raw), sourceID); err != nil {
		t.Fatal(err)
	}
}
