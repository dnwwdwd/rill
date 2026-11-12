package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestAuthConfigSessionAndLogout(t *testing.T) {
	_, client := newTestApp(t, true)

	response, data := client.raw(http.MethodGet, "/api/v1/auth/config", nil, "", "")
	if response.StatusCode != http.StatusOK || !strings.Contains(string(data), `"mode":"oidc"`) {
		t.Fatalf("auth config: %d %s", response.StatusCode, data)
	}
	if !strings.Contains(string(data), `/auth/oidc/login`) {
		t.Fatalf("auth config missing login url: %s", data)
	}

	response, data = client.request(http.MethodGet, "/api/v1/folders", nil, false)
	if response.StatusCode != http.StatusUnauthorized || errorCode(t, data) != "unauthorized" {
		t.Fatalf("unauthenticated folders: %d %s", response.StatusCode, data)
	}

	client.login()
	response, data = client.request(http.MethodGet, "/api/v1/auth/csrf", nil, false)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("csrf status %d: %s", response.StatusCode, data)
	}
	var csrfBody struct {
		CSRF string `json:"csrf_token"`
	}
	if err := json.Unmarshal(data, &csrfBody); err != nil || csrfBody.CSRF != client.csrf {
		t.Fatalf("csrf body %s: %v", data, err)
	}

	response, data = client.request(http.MethodPost, "/api/v1/folders", map[string]any{"name": "no csrf"}, false)
	if response.StatusCode != http.StatusForbidden || errorCode(t, data) != "csrf_invalid" {
		t.Fatalf("missing csrf: %d %s", response.StatusCode, data)
	}

	var folder Folder
	client.decode(http.MethodPost, "/api/v1/folders", map[string]any{"name": "Tech"}, true, http.StatusCreated, &folder)
	if folder.ID == 0 {
		t.Fatal("folder id is zero")
	}

	response, _ = client.request(http.MethodPost, "/api/v1/auth/logout", nil, true)
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("logout status %d", response.StatusCode)
	}
	response, data = client.request(http.MethodGet, "/api/v1/folders", nil, false)
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("folders after logout: %d %s", response.StatusCode, data)
	}
}

// DEC-017: password login endpoints are gone. Unauthenticated requests are
// rejected by the session middleware (401), authenticated ones hit the API
// 404 fallback.
func TestPasswordEndpointsRemoved(t *testing.T) {
	_, client := newTestApp(t, true)

	response, data := client.request(http.MethodPost, "/api/v1/auth/login", map[string]string{"password": "whatever"}, false)
	if response.StatusCode != http.StatusUnauthorized || errorCode(t, data) != "unauthorized" {
		t.Fatalf("login endpoint unauthenticated: %d %s", response.StatusCode, data)
	}

	client.login()
	response, data = client.request(http.MethodPost, "/api/v1/auth/login", map[string]string{"password": "whatever"}, true)
	if response.StatusCode != http.StatusNotFound || errorCode(t, data) != "not_found" {
		t.Fatalf("login endpoint authenticated: %d %s", response.StatusCode, data)
	}
	response, data = client.request(http.MethodPost, "/api/v1/auth/password", map[string]string{
		"current_password": "a",
		"new_password":     "b",
	}, true)
	if response.StatusCode != http.StatusNotFound || errorCode(t, data) != "not_found" {
		t.Fatalf("password endpoint authenticated: %d %s", response.StatusCode, data)
	}
}
