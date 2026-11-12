package wechat

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newTestClient(server *httptest.Server) *Client {
	client := NewClient()
	client.BaseURL = server.URL
	client.HTTPClient = server.Client()
	return client
}

func TestFetchArticleExtraction(t *testing.T) {
	var sawHeaders bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/s/test-article" {
			http.NotFound(w, r)
			return
		}
		sawHeaders = strings.Contains(r.Header.Get("User-Agent"), "Chrome") &&
			r.Header.Get("Accept-Language") != "" &&
			r.Header.Get("Referer") == "http://"+r.Host+r.URL.RequestURI()
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		io.WriteString(w, `<!doctype html>
<html><body>
<script>var biz = "MzA5TESTBIZ";</script>
<h1 id="activity-name">  A  Test   Article </h1>
<a id="js_name">Test Account</a>
<span id="js_author_name">Test Author</span>
<em id="publish_time">2024-01-02 03:04</em>
<div id="js_content">
  <script>alert("x")</script>
  <style>.x{}</style>
  <p onclick="evil()" data-x="1">Hello <a href="javascript:alert(1)">bad</a></p>
  <img data-src="/images/a.png" src="data:image/gif;base64,AAAA" alt="a">
  <img data-src="//cdn.example.com/b.jpg" alt="b">
  <img src="https://cdn.example.com/c.jpg" alt="c">
  <img src="https://cdn.example.com/c.jpg" alt="dup">
  <iframe src="https://example.com/embed"></iframe>
  <svg><script>alert(2)</script></svg>
</div>
</body></html>`)
	}))
	defer server.Close()

	client := newTestClient(server)
	articleURL := server.URL + "/s/test-article"

	article, err := client.FetchArticle(context.Background(), articleURL)
	if err != nil {
		t.Fatalf("FetchArticle: %v", err)
	}
	if !sawHeaders {
		t.Error("article request did not carry the expected browser headers")
	}
	if article.Link != articleURL {
		t.Errorf("Link = %q, want %q", article.Link, articleURL)
	}
	if article.Title != "A Test Article" {
		t.Errorf("Title = %q", article.Title)
	}
	if article.AccountName != "Test Account" {
		t.Errorf("AccountName = %q", article.AccountName)
	}
	if article.Biz != "MzA5TESTBIZ" {
		t.Errorf("Biz = %q", article.Biz)
	}
	if article.Author != "Test Author" {
		t.Errorf("Author = %q", article.Author)
	}
	if article.PublishedText != "2024-01-02 03:04" || !article.HasPublished {
		t.Errorf("published = %q (has=%v)", article.PublishedText, article.HasPublished)
	}
	if year, month, day := article.PublishedAt.Date(); year != 2024 || month != time.January || day != 2 {
		t.Errorf("PublishedAt date = %v", article.PublishedAt)
	}
	if hour, minute := article.PublishedAt.Hour(), article.PublishedAt.Minute(); hour != 3 || minute != 4 {
		t.Errorf("PublishedAt time = %v", article.PublishedAt)
	}

	firstImage := server.URL + "/images/a.png"
	secondImage := "http://cdn.example.com/b.jpg" // resolved with the article's http scheme
	for _, want := range []string{firstImage, secondImage, "https://cdn.example.com/c.jpg"} {
		if !strings.Contains(article.HTML, want) {
			t.Errorf("HTML does not contain %q: %s", want, article.HTML)
		}
	}
	for _, unwanted := range []string{"<script", "<style", "<iframe", "<svg", "onclick", "javascript:", "data-src"} {
		if strings.Contains(strings.ToLower(article.HTML), strings.ToLower(unwanted)) {
			t.Errorf("HTML still contains %q: %s", unwanted, article.HTML)
		}
	}
	wantMedia := []string{firstImage, secondImage, "https://cdn.example.com/c.jpg"}
	if len(article.MediaURLs) != len(wantMedia) {
		t.Fatalf("MediaURLs = %v, want %v", article.MediaURLs, wantMedia)
	}
	for i := range wantMedia {
		if article.MediaURLs[i] != wantMedia[i] {
			t.Errorf("MediaURLs[%d] = %q, want %q", i, article.MediaURLs[i], wantMedia[i])
		}
	}
}

func TestFetchArticleFallbacks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		switch r.URL.Path {
		case "/fallback":
			io.WriteString(w, `<html><body>
<em id="publish_time" class="rich_media_meta_text">2024年2月3日</em>
<span class="rich_media_meta_text">Fallback Author</span>
<div class="rich_media_content"><p>Fallback body</p><img src="/rel.jpg"></div>
</body></html>`)
		case "/bad-date":
			io.WriteString(w, `<html><body>
<em id="publish_time">昨天</em>
<div id="js_content"><p>Body</p></div>
</body></html>`)
		case "/blocked":
			io.WriteString(w, `<html><body><p>当前环境异常，完成验证后即可继续访问</p></body></html>`)
		default:
			io.WriteString(w, `<html><body><p>no article body</p></body></html>`)
		}
	}))
	defer server.Close()

	client := newTestClient(server)

	article, err := client.FetchArticle(context.Background(), server.URL+"/fallback")
	if err != nil {
		t.Fatalf("FetchArticle fallback: %v", err)
	}
	if article.Author != "Fallback Author" {
		t.Errorf("Author = %q, want Fallback Author", article.Author)
	}
	if !strings.Contains(article.HTML, "Fallback body") {
		t.Errorf("HTML = %q, want fallback body", article.HTML)
	}
	if len(article.MediaURLs) != 1 || article.MediaURLs[0] != server.URL+"/rel.jpg" {
		t.Errorf("MediaURLs = %v", article.MediaURLs)
	}
	if !article.HasPublished || article.PublishedAt.Year() != 2024 || article.PublishedAt.Month() != time.February || article.PublishedAt.Day() != 3 {
		t.Errorf("PublishedAt = %v", article.PublishedAt)
	}

	article, err = client.FetchArticle(context.Background(), server.URL+"/bad-date")
	if err != nil {
		t.Fatalf("FetchArticle bad date: %v", err)
	}
	if article.PublishedText != "昨天" {
		t.Errorf("PublishedText = %q, want 昨天", article.PublishedText)
	}
	if article.HasPublished || !article.PublishedAt.IsZero() {
		t.Errorf("PublishedAt = %v (has=%v), want zero/false", article.PublishedAt, article.HasPublished)
	}

	for _, path := range []string{"/blocked", "/missing"} {
		_, err := client.FetchArticle(context.Background(), server.URL+path)
		if !errors.Is(err, ErrArticleUnavailable) {
			t.Errorf("%s error = %v, want ErrArticleUnavailable", path, err)
		}
	}
}

func TestFetchArticleBizFromQuery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		io.WriteString(w, `<html><body><a id="js_name">Query Account</a><div id="js_content"><p>x</p></div></body></html>`)
	}))
	defer server.Close()
	client := newTestClient(server)
	article, err := client.FetchArticle(context.Background(), server.URL+"/s/x?__biz=MzA5URLBIZ&mid=1")
	if err != nil {
		t.Fatalf("FetchArticle: %v", err)
	}
	if article.Biz != "MzA5URLBIZ" {
		t.Fatalf("Biz = %q", article.Biz)
	}
}
