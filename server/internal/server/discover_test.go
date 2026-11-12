package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoverHTMLAndDirectFeed(t *testing.T) {
	_, client := newTestApp(t, true)
	client.login()
	site := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(`<!doctype html><html><head><title>Example Blog</title>
				<link rel="alternate" type="application/atom+xml" title="Atom" href="/atom.xml">
				<link rel="alternate" type="application/feed+json" href="feed.json">
				</head><body>blog</body></html>`))
		case "/direct.xml":
			w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
			_, _ = w.Write([]byte(`<rss version="2.0"><channel><title>Direct Feed</title><link>` + "http://" + r.Host + `/</link></channel></rss>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer site.Close()

	var candidates []FeedCandidate
	client.decode(http.MethodPost, "/api/v1/sources/discover", map[string]any{"url": site.URL}, true, http.StatusOK, &candidates)
	if len(candidates) != 2 {
		t.Fatalf("candidates: %+v", candidates)
	}
	if candidates[0].Title != "Atom" || !strings.HasSuffix(candidates[0].URL, "/atom.xml") || candidates[0].Format != "Atom 1.0" {
		t.Fatalf("atom candidate: %+v", candidates[0])
	}
	if candidates[1].Format != "JSON Feed" || !strings.HasSuffix(candidates[1].URL, "/feed.json") {
		t.Fatalf("json candidate: %+v", candidates[1])
	}

	var direct []FeedCandidate
	client.decode(http.MethodPost, "/api/v1/sources/discover", map[string]any{"url": site.URL + "/direct.xml"}, true, http.StatusOK, &direct)
	if len(direct) != 1 || direct[0].Title != "Direct Feed" || direct[0].Format != "RSS 2.0" {
		t.Fatalf("direct feed: %+v", direct)
	}
}

func TestFallbackSourceFetch(t *testing.T) {
	_, client := newTestApp(t, true)
	client.login()
	feedServer, _ := feedTestServer(t)
	var source Source
	client.decode(http.MethodPost, "/api/v1/sources", map[string]any{
		"kind":         "rss",
		"name":         "Fallback",
		"feed_url":     "https://example.com/primary.xml",
		"fallback_url": feedServer.URL + "/feed.xml",
	}, true, http.StatusCreated, &source)
	var fetched Source
	client.decode(http.MethodPost, "/api/v1/sources/"+itoa(source.ID)+"/fallback", nil, true, http.StatusOK, &fetched)
	if fetched.ItemCount != 2 || fetched.LastStatus == nil || *fetched.LastStatus != "ok" {
		t.Fatalf("fallback fetch: %+v", fetched)
	}
	if fetched.Note == nil || !strings.Contains(*fetched.Note, "自动降级") {
		t.Fatalf("fallback note: %+v", fetched.Note)
	}
}

func TestStaticSPAFallbackAndReservedPaths(t *testing.T) {
	staticDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(staticDir, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staticDir, "index.html"), []byte("<html>SPA</html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staticDir, "assets", "app-0123456789abcdef.js"), []byte("console.log('ok')"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, client := newTestAppWithStatic(t, true, staticDir)

	response, body := client.raw(http.MethodGet, "/sources/42", nil, "", "")
	if response.StatusCode != http.StatusOK || !strings.Contains(string(body), "SPA") {
		t.Fatalf("SPA fallback: %d %s", response.StatusCode, body)
	}
	if response.Header.Get("Cache-Control") != "no-cache" {
		t.Fatalf("index cache: %q", response.Header.Get("Cache-Control"))
	}
	response, body = client.raw(http.MethodGet, "/assets/app-0123456789abcdef.js", nil, "", "")
	if response.StatusCode != http.StatusOK || !strings.Contains(response.Header.Get("Cache-Control"), "immutable") {
		t.Fatalf("asset cache: %d %q", response.StatusCode, response.Header.Get("Cache-Control"))
	}
	response, body = client.raw(http.MethodGet, "/feed/not-a-feed", nil, "", "")
	if response.StatusCode != http.StatusNotFound || strings.Contains(string(body), "SPA") {
		t.Fatalf("feed fallback swallowed: %d %s", response.StatusCode, body)
	}
	response, body = client.raw(http.MethodGet, "/api/v1/nope", nil, "", "")
	if response.StatusCode != http.StatusUnauthorized || strings.Contains(string(body), "SPA") {
		t.Fatalf("API fallback swallowed: %d %s", response.StatusCode, body)
	}
	client.login()
	response, body = client.raw(http.MethodGet, "/api/v1/nope", nil, "", "")
	if response.StatusCode != http.StatusNotFound || strings.Contains(string(body), "SPA") {
		t.Fatalf("API 404: %d %s", response.StatusCode, body)
	}
}
