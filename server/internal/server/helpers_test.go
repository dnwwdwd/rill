package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"rill/internal/config"
)

type testClient struct {
	t    *testing.T
	app  *App
	base string
	http *http.Client
	csrf string
}

func newTestApp(t *testing.T, _ bool) (*App, *testClient) {
	return newTestAppWithOptions(t, false, "", false)
}

func newTestAppWithStatic(t *testing.T, _ bool, staticDir string) (*App, *testClient) {
	return newTestAppWithOptions(t, false, staticDir, false)
}

func newTestAppWithInitialFetch(t *testing.T, _ bool) (*App, *testClient) {
	return newTestAppWithOptions(t, false, "", true)
}

func newTestAppWithOptions(t *testing.T, _ bool, staticDir string, initialFetch bool) (*App, *testClient) {
	t.Helper()
	dir := t.TempDir()
	cfg := config.Config{
		Listen:            "127.0.0.1:0",
		DataDir:           dir + "/data",
		CacheDir:          dir + "/cache",
		CookieSecure:      false,
		SessionTTL:        time.Hour,
		LogLevel:          "error",
		MediaMaxBytes:     2 << 30,
		OutboundUserAgent: "Rill/test",
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	fixedNow := time.Now().UTC().Truncate(time.Second)
	app, err := NewWithOptions(context.Background(), cfg, Options{
		DisableScheduler:    true,
		DisableBackfill:     true,
		DisableInitialFetch: !initialFetch,
		Now:                 func() time.Time { return fixedNow },
		Logger:              logger,
		StaticDir:           staticDir,
	})
	if err != nil {
		t.Fatalf("NewWithOptions: %v", err)
	}
	// Unit tests use local catalog servers; never let a failing local server
	// silently fall through to the public GitHub mirror.
	app.bestblogsCatalogFallbackBase = ""
	t.Cleanup(app.Close)
	server := httptest.NewServer(app.Handler())
	t.Cleanup(server.Close)
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	return app, &testClient{t: t, app: app, base: server.URL, http: &http.Client{Jar: jar}}
}

func (c *testClient) raw(method, path string, body []byte, contentType string, csrf string) (*http.Response, []byte) {
	c.t.Helper()
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	request, err := http.NewRequest(method, c.base+path, reader)
	if err != nil {
		c.t.Fatalf("new request: %v", err)
	}
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	if csrf != "" {
		request.Header.Set("X-CSRF-Token", csrf)
	}
	response, err := c.http.Do(request)
	if err != nil {
		c.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		c.t.Fatalf("read response: %v", err)
	}
	return response, data
}

func (c *testClient) request(method, path string, body any, csrf bool) (*http.Response, []byte) {
	c.t.Helper()
	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			c.t.Fatalf("marshal body: %v", err)
		}
	}
	token := ""
	if csrf {
		token = c.csrf
	}
	return c.raw(method, path, payload, "application/json", token)
}

// login seeds a session directly (DEC-017: the password login endpoint no
// longer exists; real logins go through the OIDC redirect flow).
func (c *testClient) login() *testClient {
	c.t.Helper()
	token, session, err := c.app.newSession(context.Background())
	if err != nil {
		c.t.Fatalf("new session: %v", err)
	}
	target, err := url.Parse(c.base)
	if err != nil {
		c.t.Fatalf("parse base url: %v", err)
	}
	c.http.Jar.SetCookies(target, []*http.Cookie{{
		Name:   sessionCookieName,
		Value:  token,
		Path:   "/",
		Secure: false,
	}})
	c.csrf = session.CSRF
	return c
}

func (c *testClient) decode(method, path string, body any, csrf bool, status int, destination any) {
	c.t.Helper()
	response, data := c.request(method, path, body, csrf)
	if response.StatusCode != status {
		c.t.Fatalf("%s %s status %d, want %d: %s", method, path, response.StatusCode, status, data)
	}
	if destination != nil {
		if err := json.Unmarshal(data, destination); err != nil {
			c.t.Fatalf("decode %s: %s (%v)", path, data, err)
		}
	}
}

func errorCode(t *testing.T, data []byte) string {
	t.Helper()
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatalf("decode error body %s: %v", data, err)
	}
	return body.Error.Code
}
