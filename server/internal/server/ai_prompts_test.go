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
	"time"
)

func TestAIPromptSettingsDefaultsAndValidation(t *testing.T) {
	_, client := newTestApp(t, true)
	client.login()

	var settings Settings
	client.decode(http.MethodGet, "/api/v1/settings", nil, false, http.StatusOK, &settings)
	if settings.AI.Prompts != (AIPromptSettings{}) {
		t.Fatalf("fresh prompts should be empty: %+v", settings.AI.Prompts)
	}
	defaults := defaultAIPromptSettings()
	if settings.AI.PromptDefaults != defaults {
		t.Fatalf("prompt defaults: %+v", settings.AI.PromptDefaults)
	}

	const customSummary = "只输出两条中文要点。"
	const customMerge = "用一句话总结，不超过 80 字。"
	client.decode(http.MethodPatch, "/api/v1/settings", map[string]any{
		"ai": map[string]any{
			"prompts": map[string]any{
				"summary_bullets": customSummary,
				"digest_merge":    customMerge,
			},
		},
	}, true, http.StatusOK, &settings)
	if settings.AI.Prompts.SummaryBullets != customSummary || settings.AI.Prompts.DigestMerge != customMerge {
		t.Fatalf("saved prompts: %+v", settings.AI.Prompts)
	}
	if settings.AI.Prompts.SummaryOne != "" || settings.AI.Prompts.DigestBatch != "" {
		t.Fatalf("partial patch should leave other prompts empty: %+v", settings.AI.Prompts)
	}

	client.decode(http.MethodGet, "/api/v1/settings", nil, false, http.StatusOK, &settings)
	if settings.AI.Prompts.SummaryBullets != customSummary {
		t.Fatalf("prompt did not persist: %+v", settings.AI.Prompts)
	}

	response, data := client.request(http.MethodPatch, "/api/v1/settings", map[string]any{
		"ai": map[string]any{
			"prompts": map[string]any{"summary_bullets": strings.Repeat("长", aiPromptMaxRunes+1)},
		},
	}, true)
	if response.StatusCode != http.StatusBadRequest || errorCode(t, data) != "invalid_settings" {
		t.Fatalf("oversized prompt: %d %s", response.StatusCode, data)
	}

	client.decode(http.MethodPatch, "/api/v1/settings", map[string]any{
		"ai": map[string]any{
			"prompts": map[string]any{"summary_bullets": "  "},
		},
	}, true, http.StatusOK, &settings)
	if settings.AI.Prompts.SummaryBullets != "" {
		t.Fatalf("empty prompt should reset to default: %+v", settings.AI.Prompts)
	}
}

func TestSummaryUsesCustomPromptWithinFixedBoundary(t *testing.T) {
	app, client := newTestApp(t, true)
	client.login()

	var mu sync.Mutex
	var userPrompts []string
	aiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var request struct {
			Messages []chatMessage `json:"messages"`
		}
		if err := json.Unmarshal(body, &request); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		mu.Lock()
		for _, message := range request.Messages {
			if message.Role == "user" {
				userPrompts = append(userPrompts, message.Content)
			}
		}
		mu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]any{"content": "- 要点一\n- 要点二"}}},
			"usage":   map[string]any{"total_tokens": 10},
		})
	}))
	t.Cleanup(aiServer.Close)

	const customInstruction = "只输出两条中文要点，保持克制。"
	client.decode(http.MethodPatch, "/api/v1/settings", map[string]any{
		"ai": map[string]any{
			"enabled":       true,
			"base_url":      aiServer.URL,
			"model":         "fake-model",
			"api_key":       "sk-FAKE-AI-KEY",
			"summary_style": "bullets",
			"prompts":       map[string]any{"summary_bullets": customInstruction},
		},
	}, true, http.StatusOK, nil)

	source := createTestSource(t, client, "Prompt Source", "https://example.com/feed.xml")
	itemID := insertTestItem(t, app, source.ID, "p1", "提示词测试", "正文内容", app.now(), 0)
	var summary AISummary
	client.decode(http.MethodPost, "/api/v1/items/"+itoa(itemID)+"/summary", nil, true, http.StatusOK, &summary)

	mu.Lock()
	captured := append([]string(nil), userPrompts...)
	mu.Unlock()
	if len(captured) != 1 {
		t.Fatalf("AI calls: %d", len(captured))
	}
	prompt := captured[0]
	if !strings.Contains(prompt, customInstruction) {
		t.Fatalf("custom instruction missing: %s", prompt)
	}
	if !strings.Contains(prompt, "<<<BEGIN_UNTRUSTED_CONTENT>>>") || !strings.Contains(prompt, "<<<END_UNTRUSTED_CONTENT>>>") {
		t.Fatalf("untrusted boundary missing: %s", prompt)
	}
	if strings.Contains(prompt, defaultSummaryBulletsInstruction) {
		t.Fatalf("default instruction should be replaced: %s", prompt)
	}
	if len(summary.Points) != 2 {
		t.Fatalf("summary points: %+v", summary)
	}
}

func TestDigestRunUsesCustomPrompts(t *testing.T) {
	app, client := newTestApp(t, true)
	client.login()

	var mu sync.Mutex
	var userPrompts []string
	aiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var request struct {
			Messages []chatMessage `json:"messages"`
		}
		if err := json.Unmarshal(body, &request); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		mu.Lock()
		for _, message := range request.Messages {
			if message.Role == "user" {
				userPrompts = append(userPrompts, message.Content)
			}
		}
		mu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]any{"content": "- 1. 要点\n- 2. 要点"}}},
			"usage":   map[string]any{"total_tokens": 10},
		})
	}))
	t.Cleanup(aiServer.Close)

	const batchInstruction = "每条只写一个词。"
	const mergeInstruction = "只输出一段总述，不要分节。"
	client.decode(http.MethodPatch, "/api/v1/settings", map[string]any{
		"ai": map[string]any{
			"enabled":  true,
			"base_url": aiServer.URL,
			"model":    "fake-model",
			"api_key":  "sk-FAKE-AI-KEY",
			"prompts": map[string]any{
				"digest_batch": batchInstruction,
				"digest_merge": mergeInstruction,
			},
		},
	}, true, http.StatusOK, nil)

	source := createTestSource(t, client, "Digest Prompt", "https://example.com/feed.xml")
	insertTestItem(t, app, source.ID, "dp1", "第一篇", "正文一", app.now().Add(-time.Hour), 0)
	var rule struct {
		ID int64 `json:"id"`
	}
	client.decode(http.MethodPost, "/api/v1/digest/rules", map[string]any{
		"name": "自定义提示词日报", "scope": []any{float64(source.ID)}, "period": "daily", "run_at_local": "09:00",
	}, true, http.StatusCreated, &rule)
	var report DigestReport
	client.decode(http.MethodPost, fmt.Sprintf("/api/v1/digest/rules/%d/run", rule.ID), nil, true, http.StatusOK, &report)

	mu.Lock()
	captured := append([]string(nil), userPrompts...)
	mu.Unlock()
	joined := strings.Join(captured, "\n---\n")
	if !strings.Contains(joined, batchInstruction) {
		t.Fatalf("digest batch instruction missing: %s", joined)
	}
	if !strings.Contains(joined, mergeInstruction) {
		t.Fatalf("digest merge instruction missing: %s", joined)
	}
	if report.Status != "ok" {
		t.Fatalf("report status: %s", report.Status)
	}
}
