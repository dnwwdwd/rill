package server

import (
	"context"
	"net/http"
	"net/url"
	"testing"
)

func TestItemSearchUsesFTSAndShortQueryFallback(t *testing.T) {
	app, client := newTestApp(t, true)
	client.login()
	source := createTestSource(t, client, "Search Source", "https://example.com/feed.xml")
	now := app.now()
	insert := func(guid, title, content, author string) {
		t.Helper()
		if _, err := app.db.ExecContext(context.Background(), `
			INSERT INTO items (source_id, guid, title, content_text, author, published_at, fetched_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			source.ID, guid, title, content, author, now.Format(timeRFC3339),
			now.Format(timeRFC3339), now.Format(timeRFC3339)); err != nil {
			t.Fatal(err)
		}
	}
	insert("one", "中文网页排版与 RSS 阅读", "行高、字距与标点挤压", "张三")
	insert("two", "普通文章", "无关内容", "李四")

	search := func(query string) ItemPage {
		t.Helper()
		var page ItemPage
		client.decode(http.MethodGet, "/api/v1/items?q="+url.QueryEscape(query), nil, false, http.StatusOK, &page)
		return page
	}
	for _, query := range []string{"网页排版", "网页", "张三"} {
		page := search(query)
		if len(page.Items) != 1 || page.Items[0].Title == "" {
			t.Fatalf("query %q returned %+v", query, page.Items)
		}
	}
	if page := search("Search Source"); len(page.Items) != 2 {
		t.Fatalf("source name search returned %d items", len(page.Items))
	}
}
