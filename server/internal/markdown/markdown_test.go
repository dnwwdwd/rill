package markdown

import (
	"strings"
	"testing"
)

func TestFromHTMLRendersArticleMarkdown(t *testing.T) {
	html := `<article>
		<h2>标题</h2>
		<p>第一段 <strong>重点</strong> 和 <em>强调</em>。</p>
		<ul><li>条目一</li><li>条目二</li></ul>
		<blockquote><p>引用</p></blockquote>
		<pre><code>fmt.Println("hi")</code></pre>
		<p><a href="/relative">相对链接</a> <img src="/pic.png" alt="图"></p>
	</article>`
	got := FromHTML(html, "https://example.com/post/1")
	for _, want := range []string{
		"## 标题",
		"**重点**",
		"*强调*",
		"- 条目一",
		"> 引用",
		"```\nfmt.Println(\"hi\")\n```",
		"[相对链接](https://example.com/relative)",
		"![图](https://example.com/pic.png)",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("markdown missing %q:\n%s", want, got)
		}
	}
}

func TestFromHTMLFallsBackToPlainText(t *testing.T) {
	if got := FromHTML("", ""); got != "" {
		t.Fatalf("empty input = %q", got)
	}
}

func TestFromHTMLWithRootKeepsMediaLinksAbsolute(t *testing.T) {
	html := `<p><img src="/media/token/hash" alt="封面"><a href="/post">站内</a></p>`
	got := FromHTMLWithRoot(html, "https://source.example/article", "https://rill.example")
	for _, want := range []string{
		"![封面](https://rill.example/media/token/hash)",
		"[站内](https://source.example/post)",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("markdown missing %q:\n%s", want, got)
		}
	}
}
