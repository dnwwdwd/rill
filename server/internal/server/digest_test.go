package server

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestDigestRuleRunAndPush(t *testing.T) {
	app, client := newTestApp(t, true)
	client.login()

	// 接收 webhook 的本地服务。
	var mu sync.Mutex
	var payloads []map[string]any
	hook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode webhook body: %v", err)
		}
		mu.Lock()
		payloads = append(payloads, body)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(hook.Close)

	source := createTestSource(t, client, "Digest Source", "https://example.com/feed.xml")
	now := app.now()
	insertTestItem(t, app, source.ID, "d1", "Rill 报告测试", "正文一", now.Add(-time.Hour), 0)
	insertTestItem(t, app, source.ID, "d2", "另一篇", "正文二", now.Add(-2*time.Hour), 0)
	insertTestItem(t, app, source.ID, "d3", "被隐藏的", "正文三", now.Add(-3*time.Hour), 1)

	// 渠道：generic_webhook 指向本地服务。
	var channel struct {
		ID int64 `json:"id"`
	}
	client.decode("POST", "/api/v1/channels", map[string]any{
		"name": "测试渠道", "kind": "generic_webhook", "url": hook.URL,
	}, true, http.StatusCreated, &channel)

	// 渠道列表 URL 打码。
	var channels []PushChannel
	client.decode("GET", "/api/v1/channels", nil, false, http.StatusOK, &channels)
	if len(channels) != 1 || channels[0].URLMasked == hook.URL || channels[0].URLMasked == "" {
		t.Fatalf("channel url not masked: %+v", channels)
	}

	// 规则：指定源范围 + daily。
	var rule struct {
		ID int64 `json:"id"`
	}
	client.decode("POST", "/api/v1/digest/rules", map[string]any{
		"name": "日报", "scope": []any{float64(source.ID)}, "period": "daily", "run_at_local": "09:00",
		"channel_ids": []any{float64(channel.ID)},
	}, true, http.StatusCreated, &rule)

	// 手动运行：AI 未配置 → 列表降级 status=error，仍应推送。
	var report DigestReport
	client.decode("POST", fmt.Sprintf("/api/v1/digest/rules/%d/run", rule.ID), nil, true, http.StatusOK, &report)
	if report.Status != "error" {
		t.Fatalf("report status %q, want error (AI 未配置)", report.Status)
	}
	if report.ItemCount != 2 {
		t.Fatalf("item_count %d, want 2（隐藏条目不计入）", report.ItemCount)
	}
	if report.PushStatus != "ok" {
		t.Fatalf("push_status %q, want ok", report.PushStatus)
	}
	mu.Lock()
	got := len(payloads)
	last := map[string]any(nil)
	if got > 0 {
		last = payloads[got-1]
	}
	mu.Unlock()
	if got != 1 || last == nil {
		t.Fatalf("webhook payloads %d, want 1", got)
	}
	if last["app"] != "rill" {
		t.Fatalf("webhook payload wrong: %v", last)
	}

	// 规则 last_run_at / next_run_at 落库。
	var lastRun, nextRun *string
	if err := app.db.QueryRowContext(context.Background(),
		`SELECT last_run_at, next_run_at FROM digest_rules WHERE id = ?`, rule.ID).Scan(&lastRun, &nextRun); err != nil {
		t.Fatal(err)
	}
	if lastRun == nil || nextRun == nil {
		t.Fatal("last_run_at / next_run_at 未落库")
	}

	// 清空窗口：last_run_at 已推进，再跑应为 empty 且不推送。
	mu.Lock()
	payloads = payloads[:0]
	mu.Unlock()
	client.decode("POST", fmt.Sprintf("/api/v1/digest/rules/%d/run", rule.ID), nil, true, http.StatusOK, &report)
	if report.Status != "empty" {
		t.Fatalf("second run status %q, want empty", report.Status)
	}
	if report.PushStatus != "none" {
		t.Fatalf("empty report push_status %q, want none", report.PushStatus)
	}
	mu.Lock()
	got = len(payloads)
	mu.Unlock()
	if got != 0 {
		t.Fatalf("empty report pushed %d times, want 0", got)
	}

	// 报告列表。
	var reports []DigestReport
	client.decode("GET", fmt.Sprintf("/api/v1/digest/reports?rule_id=%d", rule.ID), nil, false, http.StatusOK, &reports)
	if len(reports) != 2 {
		t.Fatalf("reports %d, want 2", len(reports))
	}
}

func TestDigestRuleWithoutChannelsStoresReportWithoutPush(t *testing.T) {
	app, client := newTestApp(t, true)
	client.login()

	source := createTestSource(t, client, "No Channel Source", "https://example.com/no-channel.xml")
	insertTestItem(t, app, source.ID, "nc1", "无渠道报告", "正文", app.now().Add(-time.Hour), 0)

	var rule struct {
		ID int64 `json:"id"`
	}
	client.decode("POST", "/api/v1/digest/rules", map[string]any{
		"name": "无渠道", "scope": []any{float64(source.ID)}, "period": "daily", "run_at_local": "09:00",
	}, true, http.StatusCreated, &rule)

	var report DigestReport
	client.decode("POST", fmt.Sprintf("/api/v1/digest/rules/%d/run", rule.ID), nil, true, http.StatusOK, &report)
	if report.PushStatus != "none" {
		t.Fatalf("push_status %q, want none (未选择渠道)", report.PushStatus)
	}
	if report.PushError == nil || *report.PushError == "" {
		t.Fatal("push_error should explain why nothing was pushed")
	}
	// 报告本身仍可查看，不依赖推送渠道。
	if report.ContentMD == "" {
		t.Fatal("report content should still be stored")
	}
}

func TestDigestFeishuPayload(t *testing.T) {
	_, client := newTestApp(t, true)
	client.login()

	received := make(chan []byte, 1)
	hook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 1<<16)
		n, _ := r.Body.Read(buf)
		received <- buf[:n]
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(hook.Close)

	var channel struct {
		ID int64 `json:"id"`
	}
	client.decode("POST", "/api/v1/channels", map[string]any{
		"name": "飞书", "kind": "feishu", "url": hook.URL,
	}, true, http.StatusCreated, &channel)
	client.decode("POST", fmt.Sprintf("/api/v1/channels/%d/test", channel.ID), nil, true, http.StatusOK, nil)

	select {
	case body := <-received:
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("feishu payload: %v (%s)", err, body)
		}
		if payload["msg_type"] != "interactive" {
			t.Fatalf("msg_type %v", payload["msg_type"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("feishu test push not received")
	}
}

func TestFeishuSignSecret(t *testing.T) {
	_, client := newTestApp(t, true)
	client.login()

	type captured struct {
		Timestamp string `json:"timestamp"`
		Sign      string `json:"sign"`
	}
	received := make(chan captured, 1)
	hook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		received <- captured{
			Timestamp: body["timestamp"].(string),
			Sign:      body["sign"].(string),
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(hook.Close)

	var channel struct {
		ID int64 `json:"id"`
	}
	client.decode("POST", "/api/v1/channels", map[string]any{
		"name": "飞书签名", "kind": "feishu", "url": hook.URL, "secret": "my-secret",
	}, true, http.StatusCreated, &channel)

	var channels []PushChannel
	client.decode("GET", "/api/v1/channels", nil, false, http.StatusOK, &channels)
	if !channels[0].HasSecret {
		t.Fatal("has_secret should be true")
	}
	client.decode("POST", fmt.Sprintf("/api/v1/channels/%d/test", channel.ID), nil, true, http.StatusOK, nil)

	select {
	case got := <-received:
		// 复算：key = timestamp+"\n"+secret，HMAC-SHA256，空消息。
		mac := hmac.New(sha256.New, []byte(got.Timestamp+"\nmy-secret"))
		want := base64.StdEncoding.EncodeToString(mac.Sum(nil))
		if got.Sign != want {
			t.Fatalf("sign mismatch: got %q want %q", got.Sign, want)
		}
		if got.Timestamp == "" {
			t.Fatal("timestamp missing")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("signed push not received")
	}

	// 更新时 secret 传 "" 清除。
	client.decode("PUT", fmt.Sprintf("/api/v1/channels/%d", channel.ID), map[string]any{
		"name": "飞书签名", "secret": "",
	}, true, http.StatusOK, nil)
	client.decode("GET", "/api/v1/channels", nil, false, http.StatusOK, &channels)
	if channels[0].HasSecret {
		t.Fatal("secret should be cleared")
	}
}

func TestNextDigestRunUsesConfiguredTimezone(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Skipf("timezone data unavailable: %v", err)
	}
	// 2026-11-10 03:40 UTC = 11:40 Asia/Shanghai。
	now := time.Date(2026, 11, 10, 3, 40, 0, 0, time.UTC)
	next := nextDigestRun(now, "daily", "11:52", 1, loc)
	want := time.Date(2026, 11, 10, 11, 52, 0, 0, loc).UTC()
	if !next.Equal(want) {
		t.Fatalf("next run = %s, want %s", next.Format(time.RFC3339), want.Format(time.RFC3339))
	}
	if name, _ := next.Zone(); name != "UTC" {
		t.Fatalf("next run zone = %q, want UTC for stable string comparison", name)
	}

	// 已过本地时间点则顺延到次日同一墙钟时间。
	now = time.Date(2026, 11, 10, 4, 30, 0, 0, time.UTC) // 12:30 local
	next = nextDigestRun(now, "daily", "11:52", 1, loc)
	want = time.Date(2026, 11, 11, 11, 52, 0, 0, loc).UTC()
	if !next.Equal(want) {
		t.Fatalf("next run (past time) = %s, want %s", next.Format(time.RFC3339), want.Format(time.RFC3339))
	}
}

func TestRepairDigestNextRunsFixesLegacyUTCOffset(t *testing.T) {
	app, _ := newTestApp(t, true)
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Skipf("timezone data unavailable: %v", err)
	}
	app.digestLocation = loc
	fixedNow := time.Date(2026, 11, 10, 4, 0, 0, 0, time.UTC) // 12:00 local
	app.nowFn = func() time.Time { return fixedNow }
	if _, err := app.db.ExecContext(context.Background(), `
		INSERT INTO digest_rules (name, scope_json, period, run_at_local, weekday,
			only_unread, push_when_empty, enabled, channel_ids_json, next_run_at, created_at, updated_at)
		VALUES ('旧规则', '[]', 'daily', '11:52', 1, 0, 0, 1, '[]', ?, ?, ?)`,
		"2026-11-10T19:52:00Z", fixedNow.Format(timeRFC3339), fixedNow.Format(timeRFC3339)); err != nil {
		t.Fatal(err)
	}
	app.repairDigestNextRuns(context.Background())
	var raw string
	if err := app.db.QueryRowContext(context.Background(),
		`SELECT next_run_at FROM digest_rules WHERE name = '旧规则'`).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	got, err := time.Parse(timeRFC3339, raw)
	if err != nil {
		t.Fatal(err)
	}
	// 目标时间今天已过且规则从未运行：立即补跑。
	if !got.Equal(fixedNow) {
		t.Fatalf("repaired next run = %s, want %s", raw, fixedNow.Format(timeRFC3339))
	}
}

func TestDigestPushBodyOmitsArticleListForAIReports(t *testing.T) {
	report := "# 日报\n\n## 本期重点\n\n- 要点\n\n## 本期文章\n\n- [标题](https://example.com) — 来源 · 时间\n"
	got := digestPushBody(report, "ok")
	if strings.Contains(got, "## 本期文章") || strings.Contains(got, "https://example.com") {
		t.Fatalf("push body still contains article list: %s", got)
	}
	if !strings.Contains(got, "完整文章列表与链接请在 Rill 中查看") {
		t.Fatalf("push body missing Rill pointer: %s", got)
	}
	// AI 不可用时列表就是报告本体，不能删。
	if fallback := digestPushBody(report, "error"); !strings.Contains(fallback, "## 本期文章") {
		t.Fatalf("fallback push body should keep article list: %s", fallback)
	}
}

func TestFeishuPushSurfacesBusinessError(t *testing.T) {
	_, client := newTestApp(t, true)
	client.login()

	hook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":19021,"msg":"sign match fail"}`))
	}))
	t.Cleanup(hook.Close)

	var channel struct {
		ID int64 `json:"id"`
	}
	client.decode("POST", "/api/v1/channels", map[string]any{
		"name": "飞书错误", "kind": "feishu", "url": hook.URL,
	}, true, http.StatusCreated, &channel)

	response, data := client.request("POST", fmt.Sprintf("/api/v1/channels/%d/test", channel.ID), nil, true)
	if response.StatusCode != http.StatusBadGateway {
		t.Fatalf("status %d, want 502: %s", response.StatusCode, data)
	}
	if !strings.Contains(string(data), "19021") {
		t.Fatalf("business error not surfaced: %s", data)
	}
}

func TestDigestRuleValidation(t *testing.T) {
	_, client := newTestApp(t, true)
	client.login()
	response, data := client.request("POST", "/api/v1/digest/rules", map[string]any{
		"name": "坏规则", "period": "hourly", "run_at_local": "25:00",
	}, true)
	if response.StatusCode != 400 {
		t.Fatalf("invalid rule status %d: %s", response.StatusCode, data)
	}
	response, data = client.request("POST", "/api/v1/channels", map[string]any{
		"name": "坏渠道", "kind": "telegram", "url": "https://example.com",
	}, true)
	if response.StatusCode != 400 {
		t.Fatalf("invalid channel kind status %d: %s", response.StatusCode, data)
	}
}

func TestChannelUpdateEnabled(t *testing.T) {
	_, client := newTestApp(t, true)
	client.login()
	var channel struct {
		ID int64 `json:"id"`
	}
	client.decode("POST", "/api/v1/channels", map[string]any{
		"name": "渠道", "kind": "feishu", "url": "https://open.feishu.cn/open-apis/bot/v2/hook/fake",
	}, true, http.StatusCreated, &channel)
	var channels []PushChannel
	client.decode("GET", "/api/v1/channels", nil, false, http.StatusOK, &channels)
	if !channels[0].Enabled {
		t.Fatal("default enabled should be true")
	}
	disabled := false
	client.decode("PUT", fmt.Sprintf("/api/v1/channels/%d", channel.ID), map[string]any{
		"name": "渠道", "enabled": disabled,
	}, true, http.StatusOK, nil)
	client.decode("GET", "/api/v1/channels", nil, false, http.StatusOK, &channels)
	if channels[0].Enabled {
		t.Fatal("enabled should be false after update")
	}
}
