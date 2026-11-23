package server

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"
)

func insertTestItem(t *testing.T, app *App, sourceID int64, guid, title, content string, published time.Time, hidden int) int64 {
	t.Helper()
	now := app.now().Format(timeRFC3339)
	result, err := app.db.ExecContext(context.Background(), `
		INSERT INTO items (source_id, guid, title, content_text, published_at, hidden, fetched_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		sourceID, guid, title, content, published.Format(timeRFC3339), hidden, now, now)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := result.LastInsertId()
	return id
}

func TestFilterRulesMarkReadAndHide(t *testing.T) {
	app, client := newTestApp(t, true)
	client.login()
	source := createTestSource(t, client, "Filter Source", "https://example.com/feed.xml")
	now := app.now()
	a := insertTestItem(t, app, source.ID, "a", "Rill 发布新版", "内容 A", now.Add(-time.Hour), 0)
	b := insertTestItem(t, app, source.ID, "b", "别的新闻", "这是广告内容", now.Add(-2*time.Hour), 0)

	// 非法正则被拒绝。
	response, data := client.request("POST", "/api/v1/filters/rules", map[string]any{
		"name": "bad", "pattern": "([", "is_regex": true, "match_field": "any", "action": "mark_read",
	}, true)
	if response.StatusCode != 400 {
		t.Fatalf("invalid regex status %d: %s", response.StatusCode, data)
	}

	// mark_read 规则。
	var created struct {
		ID int64 `json:"id"`
	}
	client.decode("POST", "/api/v1/filters/rules", map[string]any{
		"name": "已读关键词", "pattern": "rill", "is_regex": false, "match_field": "title",
		"action": "mark_read", "scope": []any{float64(source.ID)},
	}, true, http.StatusCreated, &created)

	// hide 规则（全部源）。
	var hideRule struct {
		ID int64 `json:"id"`
	}
	client.decode("POST", "/api/v1/filters/rules", map[string]any{
		"name": "隐藏广告", "pattern": "广告", "is_regex": false, "match_field": "content",
		"action": "hide", "scope": []any{},
	}, true, http.StatusCreated, &hideRule)

	var applyResult struct {
		Affected int64 `json:"affected"`
	}
	client.decode("POST", fmt.Sprintf("/api/v1/filters/rules/%d/apply", created.ID), nil, true, http.StatusOK, &applyResult)
	if applyResult.Affected != 1 {
		t.Fatalf("mark_read affected %d, want 1", applyResult.Affected)
	}
	client.decode("POST", fmt.Sprintf("/api/v1/filters/rules/%d/apply", hideRule.ID), nil, true, http.StatusOK, &applyResult)
	if applyResult.Affected != 1 {
		t.Fatalf("hide affected %d, want 1", applyResult.Affected)
	}

	var readAt, hiddenAt map[string]any
	row := app.db.QueryRowContext(context.Background(), `SELECT read_at, hidden FROM items WHERE id = ?`, a)
	var r any
	var h int
	if err := row.Scan(&r, &h); err != nil {
		t.Fatal(err)
	}
	if r == nil {
		t.Fatal("mark_read 未生效")
	}
	row = app.db.QueryRowContext(context.Background(), `SELECT read_at, hidden FROM items WHERE id = ?`, b)
	if err := row.Scan(&r, &h); err != nil {
		t.Fatal(err)
	}
	if h != 1 {
		t.Fatal("hide 未生效")
	}
	_ = readAt
	_ = hiddenAt

	// 默认列表排除隐藏条目，hidden=1 只看隐藏。
	var page ItemPage
	client.decode("GET", "/api/v1/items", nil, false, http.StatusOK, &page)
	if len(page.Items) != 1 {
		t.Fatalf("default list count %d, want 1", len(page.Items))
	}
	client.decode("GET", "/api/v1/items?hidden=1", nil, false, http.StatusOK, &page)
	if len(page.Items) != 1 || page.Items[0].ID != b {
		t.Fatalf("hidden view wrong: %+v", page.Items)
	}

	// 统计：hidden 计入 hidden，不计入默认总数。
	var stats ReaderStats
	client.decode("GET", "/api/v1/items/stats", nil, false, http.StatusOK, &stats)
	if stats.Hidden != 1 || stats.Total != 1 {
		t.Fatalf("stats wrong: %+v", stats)
	}

	// 源列表未读数必须与默认列表一致：被隐藏的未读条目不能继续点亮关注源 dot。
	var sources []Source
	client.decode("GET", "/api/v1/sources", nil, false, http.StatusOK, &sources)
	for _, entry := range sources {
		if entry.ID == source.ID && entry.UnreadCount != 0 {
			t.Fatalf("source unread_count = %d, want 0 after hide/read", entry.UnreadCount)
		}
	}
}

func TestFilterRuleFolderScope(t *testing.T) {
	app, client := newTestApp(t, true)
	client.login()
	var folder struct {
		ID int64 `json:"id"`
	}
	client.decode("POST", "/api/v1/folders", map[string]any{"name": "分组"}, true, http.StatusCreated, &folder)
	source := createTestSource(t, client, "Folder Source", "https://example.com/feed.xml")
	if _, err := app.db.ExecContext(context.Background(),
		`UPDATE sources SET folder_id = ? WHERE id = ?`, folder.ID, source.ID); err != nil {
		t.Fatal(err)
	}
	now := app.now()
	item := insertTestItem(t, app, source.ID, "f1", "标题X", "正文", now.Add(-time.Hour), 0)

	var rule struct {
		ID int64 `json:"id"`
	}
	client.decode("POST", "/api/v1/filters/rules", map[string]any{
		"name": "分组隐藏", "pattern": "x", "is_regex": false, "match_field": "any",
		"action": "hide", "scope": []any{fmt.Sprintf("folder:%d", folder.ID)},
	}, true, http.StatusCreated, &rule)
	var applyResult struct {
		Affected int64 `json:"affected"`
	}
	client.decode("POST", fmt.Sprintf("/api/v1/filters/rules/%d/apply", rule.ID), nil, true, http.StatusOK, &applyResult)
	if applyResult.Affected != 1 {
		t.Fatalf("folder scope affected %d, want 1", applyResult.Affected)
	}
	var hidden int
	if err := app.db.QueryRowContext(context.Background(),
		`SELECT hidden FROM items WHERE id = ?`, item).Scan(&hidden); err != nil {
		t.Fatal(err)
	}
	if hidden != 1 {
		t.Fatal("folder scope filter did not hit")
	}
}
