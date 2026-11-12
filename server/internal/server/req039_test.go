package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"rill/internal/fetch"
)

func seedItem(t *testing.T, app *App, folderName string) (folderID, sourceID, itemID int64) {
	t.Helper()
	ctx := context.Background()
	now := app.now().Format(timeRFC3339)
	res, err := app.db.ExecContext(ctx,
		`INSERT INTO folders (name, sort, parent_id, created_at) VALUES (?, 1, NULL, ?)`, folderName, now)
	if err != nil {
		t.Fatal(err)
	}
	folderID, _ = res.LastInsertId()
	res, err = app.db.ExecContext(ctx, `
		INSERT INTO sources (kind, name, feed_url, folder_id, config_json, enabled, refresh_seconds,
			initial_limit, next_fetch_at, feed_token, created_at, updated_at)
		VALUES ('rss', ?, ?, ?, '{}', 1, 1800, 20, ?, ?, ?, ?)`,
		"测试源-"+folderName, "https://example.com/"+strings.ReplaceAll(folderName, " ", "-")+".xml",
		folderID, now, "token-"+folderName, now, now)
	if err != nil {
		t.Fatal(err)
	}
	sourceID, _ = res.LastInsertId()
	res, err = app.db.ExecContext(ctx, `
		INSERT INTO items (source_id, guid, title, url, content_text, content_html, fetched_at, updated_at)
		VALUES (?, ?, '测试文章', 'https://example.com/post', '正文内容', '<p>正文内容</p>', ?, ?)`,
		sourceID, "guid-"+folderName, now, now)
	if err != nil {
		t.Fatal(err)
	}
	itemID, _ = res.LastInsertId()
	return folderID, sourceID, itemID
}

func TestItemLaterAndNotes(t *testing.T) {
	_, client := newTestApp(t, false)
	client.login()
	_, _, itemID := seedItem(t, client.app, "分组A")
	id := itoa(itemID)

	var summary ItemSummary
	client.decode(http.MethodPatch, "/api/v1/items/"+id+"/state", map[string]any{"later": true}, true, http.StatusOK, &summary)
	if summary.LaterAt == nil {
		t.Fatal("later_at not set")
	}

	var page ItemPage
	client.decode(http.MethodGet, "/api/v1/items?later=1", nil, false, http.StatusOK, &page)
	if len(page.Items) != 1 || page.Items[0].LaterAt == nil {
		t.Fatalf("later filter: %+v", page.Items)
	}

	var stats ReaderStats
	client.decode(http.MethodGet, "/api/v1/items/stats", nil, false, http.StatusOK, &stats)
	if stats.Later != 1 {
		t.Fatalf("later stats: %+v", stats)
	}

	var note ItemNote
	client.decode(http.MethodPost, "/api/v1/items/"+id+"/notes",
		map[string]any{"quote": "正文", "note": "想法", "color": "green"}, true, http.StatusCreated, &note)
	if note.Quote != "正文" || note.Color != "green" {
		t.Fatalf("note: %+v", note)
	}

	var notes []ItemNote
	client.decode(http.MethodGet, "/api/v1/items/"+id+"/notes", nil, false, http.StatusOK, &notes)
	if len(notes) != 1 {
		t.Fatalf("notes: %+v", notes)
	}

	var detail ItemDetail
	client.decode(http.MethodGet, "/api/v1/items/"+id, nil, false, http.StatusOK, &detail)
	if len(detail.Notes) != 1 {
		t.Fatalf("detail notes: %+v", detail.Notes)
	}

	client.decode(http.MethodDelete, "/api/v1/items/"+id+"/notes/"+itoa(note.ID), nil, true, http.StatusNoContent, nil)
	client.decode(http.MethodGet, "/api/v1/items/"+id+"/notes", nil, false, http.StatusOK, &notes)
	if len(notes) != 0 {
		t.Fatalf("notes after delete: %+v", notes)
	}
}

func TestMarkAllReadIncludesFolderSubtree(t *testing.T) {
	_, client := newTestApp(t, false)
	client.login()
	parent, _, _ := seedItem(t, client.app, "父分组")
	child, _, _ := seedItem(t, client.app, "子分组")
	if _, err := client.app.db.ExecContext(context.Background(),
		`UPDATE folders SET parent_id = ? WHERE id = ?`, parent, child); err != nil {
		t.Fatal(err)
	}
	var result BatchStateResponse
	client.decode(http.MethodPost, "/api/v1/items/mark-all-read",
		map[string]any{"folder_id": parent}, true, http.StatusOK, &result)
	if result.Updated != 2 {
		t.Fatalf("updated = %d, want 2", result.Updated)
	}
}

func TestFetchItemFullTextOnDemand(t *testing.T) {
	article := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<html><body><nav>导航</nav><article><h1>标题</h1><p>这是一段由原网站提供的完整正文，长度足够通过全文抓取的最小阈值，用户点击工具栏后应当看到这段内容。</p><p>第二段继续补充细节，确保这段测试正文的长度超过按需全文抓取的最小阈值，而不是只有导航或摘要。</p></article></body></html>`))
	}))
	defer article.Close()
	_, client := newTestApp(t, false)
	client.login()
	_, _, itemID := seedItem(t, client.app, "全文分组")
	if _, err := client.app.db.ExecContext(context.Background(),
		`UPDATE items SET url = ?, content_text = 'Feed 摘要', content_html = '<p>Feed 摘要</p>' WHERE id = ?`,
		article.URL, itemID); err != nil {
		t.Fatal(err)
	}
	var detail ItemDetail
	client.decode(http.MethodPost, "/api/v1/items/"+itoa(itemID)+"/fulltext", nil, true, http.StatusOK, &detail)
	if !strings.Contains(detail.ContentText, "完整正文") || !strings.Contains(detail.ContentHTML, "完整正文") {
		t.Fatalf("full text not stored: %+v", detail)
	}
}

func TestEnrichFullTextTracksImages(t *testing.T) {
	article := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<html><body><article><h1>标题</h1><p>这是一段由原网站提供的完整正文，长度足够通过全文抓取的最小阈值，用户启用源级全文后应当看到这段内容。</p><p>第二段继续补充细节，确保这段测试正文的长度超过全文抓取的最小阈值。这里再补充一些内容，让提取结果稳定超过一百六十个字符，从而真正走到正文替换和图片登记这一步，验证源级全文抓取的完整链路。正文还要继续写下去，因为测试断言依赖最小字数门槛被跨过，否则这段用例会静默跳过图片处理逻辑。</p><img src="/cover.png" alt="封面"></article></body></html>`))
	}))
	defer article.Close()
	_, client := newTestApp(t, false)
	client.login()
	_, sourceID, _ := seedItem(t, client.app, "全文图片")
	source, err := client.app.getSourceRow(context.Background(), sourceID)
	if err != nil {
		t.Fatal(err)
	}
	source.FullText = true
	source.FeedToken = "token-images"
	items := []fetch.FeedItem{{
		GUID:        "fulltext-image",
		URL:         article.URL + "/post",
		ContentText: "Feed 摘要",
	}}
	client.app.enrichFullText(context.Background(), source, items)
	if !strings.Contains(items[0].ContentHTML, "/media/token-images/") {
		t.Fatalf("image was not rewritten to the media proxy: %s", items[0].ContentHTML)
	}
	if len(items[0].MediaURLs) != 1 || !strings.HasSuffix(items[0].MediaURLs[0], "/cover.png") {
		t.Fatalf("full-text media URL not tracked: %+v", items[0].MediaURLs)
	}
	if _, _, err := client.app.applyFeedResult(context.Background(), source,
		&fetch.FeedResult{Items: items, HTTPStatus: http.StatusOK}, time.Now(), 0, ""); err != nil {
		t.Fatal(err)
	}
	var mediaJSON string
	if err := client.app.db.QueryRowContext(context.Background(),
		`SELECT COALESCE(media_json, '') FROM items WHERE source_id = ? AND guid = ?`,
		sourceID, "fulltext-image").Scan(&mediaJSON); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(mediaJSON, "/cover.png") {
		t.Fatalf("media_json missing full-text image: %s", mediaJSON)
	}
	var count int
	if err := client.app.db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM media_cache WHERE source_url LIKE '%/cover.png'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("media_cache rows = %d, want 1", count)
	}
}

func TestApplyFeedResultUpdatesChangedItemURL(t *testing.T) {
	_, client := newTestApp(t, false)
	client.login()
	_, sourceID, _ := seedItem(t, client.app, "URL同步")
	source, err := client.app.getSourceRow(context.Background(), sourceID)
	if err != nil {
		t.Fatal(err)
	}
	item := fetch.FeedItem{
		GUID:        "same-guid",
		URL:         "https://example.com/old",
		Title:       "标题",
		ContentText: "正文",
		ContentHTML: "<p>正文</p>",
		ContentHash: "same-hash",
		PublishedAt: time.Now().UTC(),
	}
	apply := func(item fetch.FeedItem) {
		t.Helper()
		if _, _, err := client.app.applyFeedResult(context.Background(), source,
			&fetch.FeedResult{Items: []fetch.FeedItem{item}, HTTPStatus: http.StatusOK},
			time.Now(), 0, ""); err != nil {
			t.Fatal(err)
		}
	}
	apply(item)
	item.URL = "https://example.com/new"
	apply(item)
	var got string
	if err := client.app.db.QueryRowContext(context.Background(),
		`SELECT url FROM items WHERE source_id = ? AND guid = ?`, sourceID, "same-guid").Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != "https://example.com/new" {
		t.Fatalf("url = %q, want updated url", got)
	}
}

func TestOPMLFolderScopedExportAndImport(t *testing.T) {
	_, client := newTestApp(t, false)
	client.login()
	folderA, _, _ := seedItem(t, client.app, "分组A")
	folderB, _, _ := seedItem(t, client.app, "分组B")

	var exported string
	response, data := client.request(http.MethodGet, "/api/v1/opml/export?folder_id="+itoa(folderA), nil, false)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("export status %d: %s", response.StatusCode, data)
	}
	exported = string(data)
	if !strings.Contains(exported, "分组A") || strings.Contains(exported, "分组B") {
		t.Fatalf("folder export not scoped: %s", exported)
	}

	opml := `<?xml version="1.0" encoding="UTF-8"?><opml version="2.0"><head><title>x</title></head><body><outline text="导入源" title="导入源" type="rss" xmlUrl="https://example.com/imported.xml"/></body></opml>`
	response, data = client.raw(http.MethodPost, "/api/v1/opml/import?folder_id="+itoa(folderB), []byte(opml), "text/xml", client.csrf)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("import status %d: %s", response.StatusCode, data)
	}
	var sources []Source
	client.decode(http.MethodGet, "/api/v1/sources", nil, false, http.StatusOK, &sources)
	found := false
	for _, source := range sources {
		if source.Name == "导入源" && source.FolderID != nil && *source.FolderID == folderB {
			found = true
		}
	}
	if !found {
		t.Fatalf("imported source not in folder B: %+v", sources)
	}
}

func TestExtractArticleHTML(t *testing.T) {
	html := []byte(`<html><body><nav>导航</nav><header>页头</header><article><h1>标题</h1><p>这是一段足够长的正文内容，用来验证提取器会选择 article 而不是整页。</p><p>第二段正文。</p></article><footer>页脚</footer></body></html>`)
	got := extractArticleHTML(html)
	if !strings.Contains(got, "这是一段足够长的正文内容") || strings.Contains(got, "导航") || strings.Contains(got, "页脚") {
		t.Fatalf("extracted: %s", got)
	}
}

func TestMCPExposesOrganizationTools(t *testing.T) {
	app, _ := newTestApp(t, false)
	names := map[string]bool{}
	for _, tool := range app.mcpTools() {
		names[tool.name] = true
	}
	for _, expected := range []string{"move_source", "move_sources", "reorder_folders"} {
		if !names[expected] {
			t.Fatalf("missing MCP tool %s", expected)
		}
	}
}
