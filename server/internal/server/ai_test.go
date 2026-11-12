package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func aiTestServer(t *testing.T) (*httptest.Server, *int) {
	t.Helper()
	var mu sync.Mutex
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer sk-FAKE-AI-KEY" {
			http.Error(w, "bad auth", http.StatusUnauthorized)
			return
		}
		body, _ := io.ReadAll(r.Body)
		var request struct {
			Model    string `json:"model"`
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.Unmarshal(body, &request); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		mu.Lock()
		calls++
		mu.Unlock()
		all := ""
		for _, message := range request.Messages {
			all += message.Content + "\n"
		}
		content := "OK"
		switch {
		case strings.Contains(all, "paragraphs"):
			title, segments := extractTestTranslation(all)
			translated := make([]string, len(segments))
			for index := range segments {
				translated[index] = fmt.Sprintf("译文%d", index+1)
			}
			result := map[string]any{"paragraphs": translated}
			if title != "" {
				result["title"] = "译文标题"
			}
			data, _ := json.Marshal(result)
			content = string(data)
		case strings.Contains(all, "要点"):
			content = "- 要点一\n- 要点二\n- 要点三"
		}
		response := map[string]any{
			"choices": []map[string]any{{"message": map[string]any{"content": content}}},
			"usage":   map[string]any{"total_tokens": 42},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	t.Cleanup(server.Close)
	return server, &calls
}

func extractTestTranslation(value string) (string, []string) {
	start := strings.Index(value, "<<<BEGIN_UNTRUSTED_CONTENT>>>")
	end := strings.LastIndex(value, "<<<END_UNTRUSTED_CONTENT>>>")
	if start < 0 || end <= start {
		return "", []string{"译文"}
	}
	payload := strings.TrimSpace(value[start+len("<<<BEGIN_UNTRUSTED_CONTENT>>>") : end])
	var input struct {
		Title      string   `json:"title"`
		Paragraphs []string `json:"paragraphs"`
	}
	if err := json.Unmarshal([]byte(payload), &input); err == nil && input.Paragraphs != nil {
		return input.Title, input.Paragraphs
	}
	return "", extractTestSegments(value)
}

func extractTestSegments(value string) []string {
	start := strings.Index(value, "<<<BEGIN_UNTRUSTED_CONTENT>>>")
	end := strings.LastIndex(value, "<<<END_UNTRUSTED_CONTENT>>>")
	if start < 0 || end <= start {
		return []string{"译文"}
	}
	payload := value[start+len("<<<BEGIN_UNTRUSTED_CONTENT>>>") : end]
	first := strings.Index(payload, "[")
	last := strings.LastIndex(payload, "]")
	if first < 0 || last <= first {
		return []string{"译文"}
	}
	var segments []string
	if err := json.Unmarshal([]byte(payload[first:last+1]), &segments); err != nil {
		return []string{"译文"}
	}
	return segments
}

func TestAIErrorsSummaryTranslationAndCaching(t *testing.T) {
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

	response, data := client.request(http.MethodPost, "/api/v1/items/"+itoa(itemID)+"/summary", nil, true)
	if response.StatusCode != http.StatusConflict || errorCode(t, data) != "ai_disabled" {
		t.Fatalf("AI disabled: %d %s", response.StatusCode, data)
	}

	aiServer, calls := aiTestServer(t)
	client.decode(http.MethodPatch, "/api/v1/settings", map[string]any{
		"ai": map[string]any{
			"enabled":       true,
			"base_url":      aiServer.URL,
			"model":         "fake-model",
			"api_key":       "sk-FAKE-AI-KEY",
			"summary_style": "bullets",
			"target_lang":   "zh-CN",
			"daily_limit":   20,
			"consent":       true,
		},
	}, true, http.StatusOK, nil)

	var testResponse AITestResponse
	client.decode(http.MethodPost, "/api/v1/settings/ai/test", nil, true, http.StatusOK, &testResponse)
	if !testResponse.OK {
		t.Fatalf("AI test: %+v", testResponse)
	}
	beforeSummaryCalls := *calls

	var summary AISummary
	client.decode(http.MethodPost, "/api/v1/items/"+itoa(itemID)+"/summary", nil, true, http.StatusOK, &summary)
	if summary.Model != "fake-model" || len(summary.Points) != 3 || summary.Style != "bullets" {
		t.Fatalf("summary: %+v", summary)
	}
	if *calls != beforeSummaryCalls+1 {
		t.Fatalf("summary calls: before=%d after=%d", beforeSummaryCalls, *calls)
	}
	client.decode(http.MethodPost, "/api/v1/items/"+itoa(itemID)+"/summary", nil, true, http.StatusOK, &summary)
	if *calls != beforeSummaryCalls+1 {
		t.Fatalf("cached summary triggered another call: %d", *calls)
	}

	var detail ItemDetail
	client.decode(http.MethodGet, "/api/v1/items/"+itoa(itemID), nil, false, http.StatusOK, &detail)
	if detail.Summary == nil || len(detail.Summary.Points) != 3 {
		t.Fatalf("detail summary: %+v", detail.Summary)
	}

	beforeTranslationCalls := *calls
	var translation Translation
	client.decode(http.MethodPost, "/api/v1/items/"+itoa(itemID)+"/translation", nil, true, http.StatusOK, &translation)
	if translation.TargetLang != "zh-CN" || translation.NeedsRetranslate || translation.Title != "译文标题" || !strings.Contains(translation.ContentHTML, "译文") {
		t.Fatalf("translation: %+v", translation)
	}
	if *calls != beforeTranslationCalls+1 {
		t.Fatalf("translation calls: before=%d after=%d", beforeTranslationCalls, *calls)
	}
	client.decode(http.MethodPost, "/api/v1/items/"+itoa(itemID)+"/translation", nil, true, http.StatusOK, &translation)
	if *calls != beforeTranslationCalls+1 {
		t.Fatalf("cached translation triggered another call: %d", *calls)
	}
	forcedResponse, forcedData := client.request(http.MethodPost, "/api/v1/items/"+itoa(itemID)+"/translation?force=1", nil, true)
	if forcedResponse.StatusCode != http.StatusOK || *calls != beforeTranslationCalls+2 {
		t.Fatalf("forced translation did not bypass cache: status=%d calls=%d data=%s", forcedResponse.StatusCode, *calls, forcedData)
	}

	if _, err := app.db.Exec(`UPDATE items SET content_html = '<p>Updated source text.</p>', content_text = 'Updated source text.', content_hash = 'changed-hash' WHERE id = ?`, itemID); err != nil {
		t.Fatal(err)
	}
	client.decode(http.MethodGet, "/api/v1/items/"+itoa(itemID), nil, false, http.StatusOK, &detail)
	if detail.Translation == nil || !detail.Translation.NeedsRetranslate {
		t.Fatalf("stale translation flag: %+v", detail.Translation)
	}
	beforeRetranslate := *calls
	client.decode(http.MethodPost, "/api/v1/items/"+itoa(itemID)+"/translation", nil, true, http.StatusOK, &translation)
	if *calls != beforeRetranslate+1 || translation.NeedsRetranslate {
		t.Fatalf("retranslation: calls=%d translation=%+v", *calls, translation)
	}

	var settings Settings
	client.decode(http.MethodGet, "/api/v1/settings", nil, false, http.StatusOK, &settings)
	if settings.AI.UsedToday < 3 {
		t.Fatalf("AI usage not recorded: %+v", settings.AI)
	}
}
