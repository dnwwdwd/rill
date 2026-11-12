package server

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"testing"
)

func TestItemMarkdownEndpointExportsOriginalBody(t *testing.T) {
	app, client := newTestApp(t, true)
	client.login()
	source := createTestSource(t, client, "Markdown Source", "https://example.com/feed.xml")
	now := app.now().Format(timeRFC3339)
	result, err := app.db.ExecContext(context.Background(), `
		INSERT INTO items (source_id, guid, title, url, author, content_html, content_text,
			published_at, fetched_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		source.ID, "md-1", "一篇 / 可导出的文章", "https://example.com/posts/md-1", "作者",
		`<article><h2>小节</h2><p>正文 <strong>重点</strong>。</p><p><a href="/more">继续阅读</a></p></article>`,
		"小节 正文 重点。继续阅读", now, now, now)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := result.LastInsertId()

	response, data := client.raw(http.MethodGet, "/api/v1/items/"+strconv.FormatInt(id, 10)+"/markdown", nil, "", "")
	if response.StatusCode != http.StatusOK {
		t.Fatalf("markdown status %d: %s", response.StatusCode, data)
	}
	if !strings.HasPrefix(response.Header.Get("Content-Type"), "text/markdown") {
		t.Fatalf("content type: %s", response.Header.Get("Content-Type"))
	}
	body := string(data)
	for _, want := range []string{
		"# 一篇 / 可导出的文章",
		"- 来源：Markdown Source",
		"- 作者：作者",
		"- 原文链接：https://example.com/posts/md-1",
		"## 小节",
		"**重点**",
		"[继续阅读](https://example.com/more)",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("markdown missing %q:\n%s", want, body)
		}
	}
	if !strings.Contains(response.Header.Get("Content-Disposition"), ".md") {
		t.Fatalf("content disposition: %s", response.Header.Get("Content-Disposition"))
	}

	// 管理 API 仍要求登录。
	request, _ := http.NewRequest(http.MethodGet, client.base+"/api/v1/items/"+strconv.FormatInt(id, 10)+"/markdown", nil)
	response2, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response2.Body.Close()
	if response2.StatusCode != http.StatusUnauthorized {
		t.Fatalf("anonymous markdown status %d", response2.StatusCode)
	}
}
