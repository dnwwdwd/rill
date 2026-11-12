package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestSchemeValidationAndPublicHealth(t *testing.T) {
	_, client := newTestApp(t, false)
	client.login()
	// Only the scheme is rejected; loopback and private addresses are valid
	// targets in a self-hosted deployment (feeds may live on the LAN or on
	// the same microserver).
	response, data := client.request(http.MethodPost, "/api/v1/sources", map[string]any{
		"kind": "rss", "name": "blocked", "feed_url": "file:///etc/passwd",
	}, true)
	if response.StatusCode != http.StatusBadRequest || errorCode(t, data) != "url_blocked" {
		t.Fatalf("file scheme: %d %s", response.StatusCode, data)
	}
	var source Source
	client.decode(http.MethodPost, "/api/v1/sources", map[string]any{
		"kind": "rss", "name": "loopback", "feed_url": "http://127.0.0.1:8080/feed.xml",
	}, true, http.StatusCreated, &source)
	response, data = client.request(http.MethodPost, "/api/v1/sources/discover", map[string]any{
		"url": "http://127.0.0.1:1/",
	}, true)
	if response.StatusCode == http.StatusBadRequest && errorCode(t, data) == "url_blocked" {
		t.Fatalf("discover should not block loopback: %d %s", response.StatusCode, data)
	}

	response, data = client.raw(http.MethodGet, "/healthz", nil, "", "")
	if response.StatusCode != http.StatusOK {
		t.Fatalf("public health: %d", response.StatusCode)
	}
	var health map[string]any
	if err := json.Unmarshal(data, &health); err != nil {
		t.Fatal(err)
	}
	if len(health) != 1 || health["ok"] != true {
		t.Fatalf("public health leaks details: %s", data)
	}
}

func TestSettingsRedactionEncryptionAndWechatBoundary(t *testing.T) {
	app, client := newTestApp(t, true)
	client.login()

	var settings Settings
	client.decode(http.MethodGet, "/api/v1/settings", nil, false, http.StatusOK, &settings)
	if settings.AI.Enabled || settings.AI.APIKeySet || settings.AI.Consent {
		t.Fatalf("unsafe AI defaults: %+v", settings.AI)
	}

	const fakeKey = "sk-FAKE-RILL-TEST-NOT-A-REAL-KEY"
	response, data := client.request(http.MethodPatch, "/api/v1/settings", map[string]any{
		"reader": map[string]any{"font_size": 20, "theme": "dark", "accent": "blue", "rail_width": 240, "list_width": 420},
		"ai": map[string]any{
			"enabled":       true,
			"base_url":      "https://api.example.invalid/v1",
			"model":         "fake-model",
			"api_key":       fakeKey,
			"summary_style": "one",
			"target_lang":   "zh-CN",
			"daily_limit":   3,
			"consent":       true,
		},
	}, true)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("patch settings: %d %s", response.StatusCode, data)
	}
	if strings.Contains(string(data), fakeKey) {
		t.Fatalf("settings response leaked key: %s", data)
	}
	client.decode(http.MethodGet, "/api/v1/settings", nil, false, http.StatusOK, &settings)
	if !settings.AI.Enabled || !settings.AI.APIKeySet || !settings.AI.Consent || settings.Reader.FontSize != 20 || settings.Reader.Accent != "blue" ||
		settings.Reader.RailWidth != 240 || settings.Reader.ListWidth != 420 {
		t.Fatalf("settings after patch: %+v", settings)
	}
	var encrypted string
	if err := app.db.QueryRow(`SELECT value FROM settings WHERE key = 'ai_api_key_enc'`).Scan(&encrypted); err != nil {
		t.Fatal(err)
	}
	if encrypted == fakeKey || strings.Contains(encrypted, fakeKey) {
		t.Fatal("API key stored in plaintext")
	}
	decrypted, err := app.secret.Decrypt(encrypted)
	if err != nil || decrypted != fakeKey {
		t.Fatalf("decrypt key: %q %v", decrypted, err)
	}
	rows, err := app.db.Query(`SELECT value FROM settings`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(value, fakeKey) {
			t.Fatal("plaintext API key found in settings table")
		}
	}
	_ = rows.Close()

	response, data = client.request(http.MethodPatch, "/api/v1/settings", map[string]any{
		"reader": map[string]any{"font_size": 99},
	}, true)
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid settings: %d %s", response.StatusCode, data)
	}
	response, data = client.request(http.MethodPatch, "/api/v1/settings", map[string]any{
		"reader": map[string]any{"accent": "magenta"},
	}, true)
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid accent: %d %s", response.StatusCode, data)
	}
	response, data = client.request(http.MethodPatch, "/api/v1/settings", map[string]any{
		"reader": map[string]any{"rail_width": 120},
	}, true)
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid rail width: %d %s", response.StatusCode, data)
	}
	response, data = client.request(http.MethodPatch, "/api/v1/settings", map[string]any{
		"reader": map[string]any{"list_width": 900},
	}, true)
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid list width: %d %s", response.StatusCode, data)
	}

	// REQ-034 / DEC-024: the MP back-office and sidecar channel is removed;
	// wechat settings only carry enabled + weread_connected.
	client.decode(http.MethodPatch, "/api/v1/settings", map[string]any{
		"wechat": map[string]any{"enabled": true},
	}, true, http.StatusOK, &settings)
	if !settings.Wechat.Enabled || settings.Wechat.WereadConnected {
		t.Fatalf("wechat settings after patch: %+v", settings.Wechat)
	}
	for _, route := range []struct{ method, path string }{
		{http.MethodPost, "/api/v1/wechat/scan"},
		{http.MethodGet, "/api/v1/wechat/scan/status"},
		{http.MethodGet, "/api/v1/wechat/search?q=x"},
		{http.MethodGet, "/api/v1/wechat/accounts"},
		{http.MethodPost, "/api/v1/wechat/logout"},
	} {
		response, data = client.request(route.method, route.path, nil, true)
		if response.StatusCode != http.StatusNotFound {
			t.Fatalf("removed endpoint %s %s: %d %s", route.method, route.path, response.StatusCode, data)
		}
	}
}
