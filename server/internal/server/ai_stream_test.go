package server

import (
	"net/http"
	"strings"
	"testing"
)

func TestAIStreamingEndpoints(t *testing.T) {
	app, client := newTestApp(t, true)
	client.login()
	feedServer, _ := feedTestServer(t)
	source := createTestSource(t, client, "Local", feedServer.URL+"/feed.xml")
	client.decode(http.MethodPost, "/api/v1/sources/"+itoa(source.ID)+"/fetch", nil, true, http.StatusOK, nil)
	var page ItemPage
	client.decode(http.MethodGet, "/api/v1/items?limit=1", nil, false, http.StatusOK, &page)
	if len(page.Items) != 1 {
		t.Fatalf("items: %+v", page)
	}
	itemID := page.Items[0].ID

	aiServer, calls := aiTestServer(t)
	client.decode(http.MethodPatch, "/api/v1/settings", map[string]any{
		"ai": map[string]any{
			"enabled":       true,
			"base_url":      aiServer.URL,
			"model":         "fake-model",
			"api_key":       "sk-FAKE-AI-KEY",
			"summary_style": "bullets",
			"target_lang":   "zh-CN",
		},
	}, true, http.StatusOK, nil)

	// Summary stream: the fake provider returns JSON, which the stream helper
	// forwards as one delta and then a done event with the saved summary.
	response, data := client.request(http.MethodPost, "/api/v1/items/"+itoa(itemID)+"/summary/stream", nil, true)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("summary stream: %d %s", response.StatusCode, data)
	}
	if !strings.Contains(response.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("summary stream content type: %s", response.Header.Get("Content-Type"))
	}
	body := string(data)
	if !strings.Contains(body, `"type":"delta"`) || !strings.Contains(body, `"type":"done"`) || !strings.Contains(body, "要点一") {
		t.Fatalf("summary stream body: %s", body)
	}
	before := *calls
	var detail ItemDetail
	client.decode(http.MethodGet, "/api/v1/items/"+itoa(itemID), nil, false, http.StatusOK, &detail)
	if detail.Summary == nil || len(detail.Summary.Points) != 3 {
		t.Fatalf("summary not persisted: %+v", detail.Summary)
	}
	_, data = client.request(http.MethodPost, "/api/v1/items/"+itoa(itemID)+"/summary/stream", nil, true)
	if *calls != before || !strings.Contains(string(data), `"type":"done"`) {
		t.Fatalf("cached summary stream called AI again: calls=%d body=%s", *calls, data)
	}

	// Translation stream: clear the cache and verify segment + done events and
	// that the rebuilt translation is persisted.
	if _, err := app.db.Exec(`DELETE FROM ai_results WHERE item_id = ? AND kind LIKE 'translation:%'`, itemID); err != nil {
		t.Fatal(err)
	}
	response, data = client.request(http.MethodPost, "/api/v1/items/"+itoa(itemID)+"/translation/stream", nil, true)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("translation stream: %d %s", response.StatusCode, data)
	}
	body = string(data)
	if !strings.Contains(body, `"type":"title"`) || !strings.Contains(body, `"type":"segment"`) || !strings.Contains(body, `"type":"done"`) || !strings.Contains(body, "译文1") {
		t.Fatalf("translation stream body: %s", body)
	}
	client.decode(http.MethodGet, "/api/v1/items/"+itoa(itemID), nil, false, http.StatusOK, &detail)
	if detail.Translation == nil || detail.Translation.ContentHTML == "" || detail.Translation.Title != "译文标题" {
		t.Fatalf("translation not persisted: %+v", detail.Translation)
	}
	beforeForced := *calls
	response, data = client.request(http.MethodPost, "/api/v1/items/"+itoa(itemID)+"/translation/stream?force=1", nil, true)
	if response.StatusCode != http.StatusOK || *calls != beforeForced+1 {
		t.Fatalf("forced translation stream: status=%d calls=%d body=%s", response.StatusCode, *calls, data)
	}
}
