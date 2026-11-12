package server

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"rill/internal/config"
)

type oidcProviderState struct {
	mu               sync.Mutex
	challenge        string
	expectedRedirect string
}

func (s *oidcProviderState) setChallenge(value, redirect string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.challenge = value
	s.expectedRedirect = redirect
}

func (s *oidcProviderState) check(form url.Values) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if form.Get("redirect_uri") != s.expectedRedirect {
		return "redirect_uri mismatch"
	}
	if form.Get("client_id") != "rill-client" || form.Get("client_secret") != "rill-secret" {
		return "client credentials mismatch"
	}
	sum := sha256.Sum256([]byte(form.Get("code_verifier")))
	if base64.RawURLEncoding.EncodeToString(sum[:]) != s.challenge {
		return "PKCE verifier mismatch"
	}
	return ""
}

func newOIDCTestApp(t *testing.T) (*App, *httptest.Server, *oidcProviderState) {
	t.Helper()
	providerState := &oidcProviderState{}
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			if err := r.ParseForm(); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if err := providerState.check(r.Form); err != "" {
				http.Error(w, err, http.StatusBadRequest)
				return
			}
			if r.Form.Get("grant_type") != "authorization_code" || r.Form.Get("code") != "test-code" {
				http.Error(w, "unexpected grant", http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "test-access-token",
				"token_type":   "Bearer",
			})
		case "/userinfo":
			if r.Header.Get("Authorization") != "Bearer test-access-token" {
				http.Error(w, "missing bearer token", http.StatusUnauthorized)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"sub":                "lazycat-user",
				"preferred_username": "example_user",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(provider.Close)

	dir := t.TempDir()
	cfg := config.Config{
		Listen:        "127.0.0.1:0",
		DataDir:       dir + "/data",
		CacheDir:      dir + "/cache",
		SessionTTL:    time.Hour,
		LogLevel:      "error",
		MediaMaxBytes: 2 << 30,
		OIDC: config.OIDCConfig{
			ClientID:     "rill-client",
			ClientSecret: "rill-secret",
			AuthURI:      provider.URL + "/authorize",
			TokenURI:     provider.URL + "/token",
			UserinfoURI:  provider.URL + "/userinfo",
			Scopes:       "openid profile email",
		},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	app, err := NewWithOptions(context.Background(), cfg, Options{
		DisableScheduler: true,
		Now:              func() time.Time { return time.Now().UTC() },
		Logger:           logger,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(app.Close)
	server := httptest.NewServer(app.Handler())
	t.Cleanup(server.Close)
	return app, server, providerState
}

func newOIDCTestHTTPClient(t *testing.T) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	return &http.Client{
		Jar: jar,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func oidcDo(t *testing.T, client *http.Client, method, target string, body io.Reader) (*http.Response, []byte) {
	t.Helper()
	return oidcDoWithHeaders(t, client, method, target, body, nil)
}

func oidcDoWithHeaders(t *testing.T, client *http.Client, method, target string, body io.Reader, headers map[string]string) (*http.Response, []byte) {
	t.Helper()
	request, err := http.NewRequest(method, target, body)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return response, data
}

func TestOIDCLoginFlowAndPasswordDisabled(t *testing.T) {
	_, server, providerState := newOIDCTestApp(t)
	client := newOIDCTestHTTPClient(t)

	response, data := oidcDo(t, client, http.MethodGet, server.URL+"/api/v1/auth/config", nil)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("auth config: %d %s", response.StatusCode, data)
	}
	var authConfig struct {
		Mode     string `json:"mode"`
		LoginURL string `json:"login_url"`
	}
	if err := json.Unmarshal(data, &authConfig); err != nil {
		t.Fatal(err)
	}
	if authConfig.Mode != "oidc" || authConfig.LoginURL != "/auth/oidc/login" {
		t.Fatalf("unexpected auth config: %s", data)
	}

	// DEC-017: the password login endpoint is removed entirely.
	response, data = oidcDo(t, client, http.MethodPost, server.URL+"/api/v1/auth/login", strings.NewReader(`{"password":"anything"}`))
	if response.StatusCode != http.StatusUnauthorized || errorCode(t, data) != "unauthorized" {
		t.Fatalf("password login endpoint: %d %s", response.StatusCode, data)
	}

	response, data = oidcDo(t, client, http.MethodGet, server.URL+"/auth/oidc/login?return_to=%2Fitems%2F42", nil)
	if response.StatusCode != http.StatusFound {
		t.Fatalf("oidc login: %d %s", response.StatusCode, data)
	}
	location, err := url.Parse(response.Header.Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	query := location.Query()
	if query.Get("response_type") != "code" ||
		query.Get("client_id") != "rill-client" ||
		query.Get("scope") != "openid profile email" ||
		query.Get("code_challenge_method") != "S256" ||
		query.Get("code_challenge") == "" ||
		query.Get("state") == "" {
		t.Fatalf("unexpected authorization URL: %s", location)
	}
	expectedRedirect := server.URL + "/auth/oidc/callback"
	if query.Get("redirect_uri") != expectedRedirect {
		t.Fatalf("redirect_uri=%q, want %q", query.Get("redirect_uri"), expectedRedirect)
	}
	providerState.setChallenge(query.Get("code_challenge"), expectedRedirect)

	callback := server.URL + "/auth/oidc/callback?code=test-code&state=" + url.QueryEscape(query.Get("state"))
	response, data = oidcDo(t, client, http.MethodGet, callback, nil)
	if response.StatusCode != http.StatusSeeOther {
		t.Fatalf("callback: %d %s", response.StatusCode, data)
	}
	if location := response.Header.Get("Location"); location != "/items/42" {
		t.Fatalf("callback redirect=%q", location)
	}
	var sessionCookie bool
	for _, cookie := range response.Cookies() {
		if cookie.Name == sessionCookieName && cookie.Value != "" {
			sessionCookie = true
		}
	}
	if !sessionCookie {
		t.Fatal("callback did not set a session cookie")
	}

	response, data = oidcDo(t, client, http.MethodGet, server.URL+"/api/v1/auth/csrf", nil)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("csrf after OIDC login: %d %s", response.StatusCode, data)
	}
	var csrf struct {
		CSRF string `json:"csrf_token"`
	}
	if err := json.Unmarshal(data, &csrf); err != nil || csrf.CSRF == "" {
		t.Fatalf("csrf response: %s (%v)", data, err)
	}

	// DEC-017: the password change endpoint is removed entirely (authenticated
	// requests fall through to the API 404 fallback).
	response, data = oidcDoWithHeaders(t, client, http.MethodPost, server.URL+"/api/v1/auth/password", strings.NewReader(`{"current_password":"x","new_password":"a-long-enough-password"}`), map[string]string{"X-CSRF-Token": csrf.CSRF})
	if response.StatusCode != http.StatusNotFound || errorCode(t, data) != "not_found" {
		t.Fatalf("password change endpoint: %d %s", response.StatusCode, data)
	}
}

func TestOIDCRejectsStateMismatchAndProviderErrors(t *testing.T) {
	_, server, _ := newOIDCTestApp(t)
	client := newOIDCTestHTTPClient(t)

	response, _ := oidcDo(t, client, http.MethodGet, server.URL+"/auth/oidc/login", nil)
	if response.StatusCode != http.StatusFound {
		t.Fatalf("oidc login status=%d", response.StatusCode)
	}

	response, data := oidcDo(t, client, http.MethodGet, server.URL+"/auth/oidc/callback?code=test-code&state=wrong", nil)
	if response.StatusCode != http.StatusSeeOther {
		t.Fatalf("bad state callback: %d %s", response.StatusCode, data)
	}
	if location := response.Header.Get("Location"); !strings.Contains(location, "oidc_error=oidc_state_invalid") {
		t.Fatalf("bad state redirect=%q", location)
	}

	response, _ = oidcDo(t, client, http.MethodGet, server.URL+"/auth/oidc/login", nil)
	location, err := url.Parse(response.Header.Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	state := location.Query().Get("state")
	response, data = oidcDo(t, client, http.MethodGet, server.URL+"/auth/oidc/callback?error=access_denied&state="+url.QueryEscape(state), nil)
	if response.StatusCode != http.StatusSeeOther {
		t.Fatalf("provider error callback: %d %s", response.StatusCode, data)
	}
	if location := response.Header.Get("Location"); !strings.Contains(location, "oidc_error=oidc_access_denied") {
		t.Fatalf("provider error redirect=%q", location)
	}
}

func TestSafeReturnTo(t *testing.T) {
	cases := map[string]string{
		"":                       "/",
		"/":                      "/",
		"/items/1?filter=unread": "/items/1?filter=unread",
		"//evil.example/":        "/",
		"https://evil.example/":  "/",
		"javascript:alert(1)":    "/",
		"\\\\evil.example\\":     "/",
	}
	for input, want := range cases {
		if got := safeReturnTo(input); got != want {
			t.Errorf("safeReturnTo(%q)=%q, want %q", input, got, want)
		}
	}
}
