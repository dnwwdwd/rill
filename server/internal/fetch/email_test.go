package fetch

import (
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
)

const sampleMultipartEmail = `Message-Id: <20261121.abc123@substack.com>
From: The Pragmatic Engineer <news@substack.com>
To: reader@example.com
Subject: =?UTF-8?B?%s?=
Date: Fri, 21 Nov 2026 08:30:00 +0800
Archived-At: <https://newsletter.example.com/archive/123>
Content-Type: multipart/alternative; boundary="BOUND"

--BOUND
Content-Type: text/plain; charset=utf-8

这是纯文本版本。

--BOUND
Content-Type: multipart/related; boundary="BOUND2"

--BOUND2
Content-Type: text/html; charset=utf-8

<html><body>
<p>在<a href="https://newsletter.example.com/archive/123">浏览器中阅读</a></p>
<img src="cid:header.png@01D">
<img src="https://cdn.example.com/inline.png">
<script>alert(1)</script>
</body></html>

--BOUND2
Content-Type: image/png; name="header.png"
Content-Transfer-Encoding: base64
Content-ID: <header.png@01D>

iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==

--BOUND2--
--BOUND--
`

func sampleEmailWithSubject(subject string) []byte {
	encoded := base64.StdEncoding.EncodeToString([]byte(subject))
	return []byte(fmt.Sprintf(sampleMultipartEmail, encoded))
}

func TestParseEmailMessage(t *testing.T) {
	message, err := ParseEmailMessage(sampleEmailWithSubject("标题：欢迎订阅本栏目"))
	if err != nil {
		t.Fatalf("ParseEmailMessage: %v", err)
	}
	if message.MessageID != "20261121.abc123@substack.com" {
		t.Errorf("MessageID = %q", message.MessageID)
	}
	if message.FromAddr != "news@substack.com" {
		t.Errorf("FromAddr = %q", message.FromAddr)
	}
	if message.FromName != "The Pragmatic Engineer" {
		t.Errorf("FromName = %q", message.FromName)
	}
	if message.Subject != "标题：欢迎订阅本栏目" {
		t.Errorf("Subject = %q", message.Subject)
	}
	if message.Date.IsZero() {
		t.Errorf("Date should be parsed")
	}
	if message.BrowserLink != "https://newsletter.example.com/archive/123" {
		t.Errorf("BrowserLink = %q", message.BrowserLink)
	}
	if !strings.Contains(message.HTML, "浏览器中阅读") {
		t.Errorf("HTML should come from text/html part: %q", message.HTML)
	}
	// 原始解析保留 HTML 原文（脚本由 SanitizeContent 移除，见下一测试）。
	if !strings.Contains(message.HTML, "<script>") {
		t.Errorf("raw parse should keep the html verbatim")
	}
	if len(message.Inline) != 1 {
		t.Fatalf("Inline count = %d, want 1", len(message.Inline))
	}
	if message.Inline[0].CID != "header.png@01D" {
		t.Errorf("Inline CID = %q", message.Inline[0].CID)
	}
	if !strings.HasPrefix(message.Inline[0].ContentType, "image/png") {
		t.Errorf("Inline ContentType = %q", message.Inline[0].ContentType)
	}
	if len(message.Inline[0].Data) == 0 {
		t.Errorf("Inline data should be base64-decoded")
	}
}

func TestBuildEmailFeedItemKeepsRichEmailMarkup(t *testing.T) {
	raw := `Message-Id: <rich@example.com>
From: Bank <notice@example.com>
Subject: Rich email
Date: Fri, 21 Nov 2026 08:30:00 +0800
Content-Type: text/html; charset=utf-8

<html><body>
<table background="https://cdn.example.com/bg.png" style="background-image:url('https://cdn.example.com/bg2.png')"><tr><td>
<center><font color="#336699">标题</font></center>
<img src="data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9Wl2n4sAAAAASUVORK5CYII=">
<img srcset="https://cdn.example.com/one.png 1x, https://cdn.example.com/two.png 2x">
<img src="//cdn.example.com/three.png">
</td></tr></table>
</body></html>`
	message, err := ParseEmailMessage([]byte(raw))
	if err != nil {
		t.Fatalf("ParseEmailMessage: %v", err)
	}
	item := BuildEmailFeedItem(message, "token123")
	html := item.ContentHTML
	for _, want := range []string{
		`background="https://cdn.example.com/bg.png"`,
		`background-image:url`,
		`<center>`,
		`<font color="#336699">`,
		`src="data:image/png;base64,`,
		`/media/token123/`,
		`data-remote-src="https://cdn.example.com/one.png"`,
		`data-remote-src="https://cdn.example.com/three.png"`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("email HTML missing %q: %s", want, html)
		}
	}
	if strings.Contains(html, "srcset=") {
		t.Errorf("srcset should be replaced by the first candidate: %s", html)
	}
}

func TestBuildEmailFeedItemRewritesCID(t *testing.T) {
	message, err := ParseEmailMessage(sampleEmailWithSubject("标题：欢迎订阅本栏目"))
	if err != nil {
		t.Fatalf("ParseEmailMessage: %v", err)
	}
	item := BuildEmailFeedItem(message, "token123")
	if item.GUID != message.MessageID {
		t.Errorf("GUID = %q, want Message-ID", item.GUID)
	}
	if item.Author != "The Pragmatic Engineer" {
		t.Errorf("Author = %q", item.Author)
	}
	if !strings.Contains(item.ContentHTML, "/media/token123/") {
		t.Errorf("cid image should be rewritten to /media path: %s", item.ContentHTML)
	}
	if strings.Contains(item.ContentHTML, "cid:") {
		t.Errorf("cid reference should be gone: %s", item.ContentHTML)
	}
	// 邮件正文里的远程图片同样走媒体代理，原始地址只保存在 data 属性里，
	// 供阅读器在用户放行后直接加载。
	if !strings.Contains(item.ContentHTML, "/media/") ||
		!strings.Contains(item.ContentHTML, `data-remote-src="https://cdn.example.com/inline.png"`) ||
		strings.Contains(item.ContentHTML, ` src="https://cdn.example.com/inline.png"`) {
		t.Errorf("remote image should be proxied: %s", item.ContentHTML)
	}
	if !strings.Contains(item.ContentHTML, `data-inline-image="1"`) {
		t.Errorf("cid image should be marked inline: %s", item.ContentHTML)
	}
	if strings.Contains(item.ContentHTML, "<script") {
		t.Errorf("scripts must be stripped: %s", item.ContentHTML)
	}
	if !strings.Contains(item.ContentHTML, "浏览器中阅读") {
		t.Errorf("content should keep the browser link: %s", item.ContentHTML)
	}
	foundPseudo := false
	for _, media := range item.MediaURLs {
		if strings.Contains(media, "rill-email-inline.invalid") {
			foundPseudo = true
		}
	}
	if !foundPseudo {
		t.Errorf("MediaURLs should include the inline pseudo URL: %v", item.MediaURLs)
	}
}

func TestParseEmailPlainTextOnly(t *testing.T) {
	raw := "From: weekly@example.com\r\nSubject: 周刊 #1\r\nDate: Fri, 20 Nov 2026 10:00:00 +0000\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n第一段内容。\r\n第二段内容。"
	message, err := ParseEmailMessage([]byte(raw))
	if err != nil {
		t.Fatalf("ParseEmailMessage: %v", err)
	}
	if message.HTML != "" {
		t.Errorf("no html part expected")
	}
	if !strings.Contains(message.Text, "第一段内容") {
		t.Errorf("Text = %q", message.Text)
	}
	item := BuildEmailFeedItem(message, "tok")
	if !strings.Contains(item.ContentHTML, "<p>第一段内容。</p>") {
		t.Errorf("plain text should convert to paragraphs: %s", item.ContentHTML)
	}
}

func TestParseEmailQuotedPrintableAndCharset(t *testing.T) {
	raw := "From: a@b.c\r\nSubject: test\r\nContent-Type: text/plain; charset=gbk\r\nContent-Transfer-Encoding: quoted-printable\r\n\r\n=C4=E3=BA=C3"
	message, err := ParseEmailMessage([]byte(raw))
	if err != nil {
		t.Fatalf("ParseEmailMessage: %v", err)
	}
	if message.Text != "你好" {
		t.Errorf("Text = %q, want 你好", message.Text)
	}
}
