package server

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestRevealGlobalFeedToken(t *testing.T) {
	app, client := newTestApp(t, true)
	client.login()
	token, err := app.globalFeedToken(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	response, body := client.request(http.MethodPost, "/api/v1/settings/feed-token/reveal", nil, true)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("reveal status %d: %s", response.StatusCode, body)
	}
	if response.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("cache control: %q", response.Header.Get("Cache-Control"))
	}
	var payload struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Token != token || payload.Token == maskToken(token) {
		t.Fatalf("revealed token mismatch")
	}
}
