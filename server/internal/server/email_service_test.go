package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"rill/internal/fetch"
)

func TestParseEmailFeedURL(t *testing.T) {
	username, sender, ok := parseEmailFeedURL("imap://user@163.com")
	if !ok || username != "user@163.com" || sender != "" {
		t.Errorf("aggregate parse: %q %q %v", username, sender, ok)
	}
	username, sender, ok = parseEmailFeedURL("imap://user@163.com/news@substack.com")
	if !ok || username != "user@163.com" || sender != "news@substack.com" {
		t.Errorf("per-sender parse: %q %q %v", username, sender, ok)
	}
	if _, _, ok = parseEmailFeedURL("https://example.com/feed.xml"); ok {
		t.Errorf("non-imap URLs must not parse")
	}
	if _, _, ok = parseEmailFeedURL("imap://"); ok {
		t.Errorf("empty username must not parse")
	}
}

func TestEmailFeedURLRoundTrip(t *testing.T) {
	if got := emailFeedURL("User@163.com", ""); got != "imap://user@163.com" {
		t.Errorf("emailFeedURL aggregate = %q", got)
	}
	if got := emailFeedURL("User@163.com", "News@Substack.com"); got != "imap://user@163.com/news@substack.com" {
		t.Errorf("emailFeedURL per-sender = %q", got)
	}
}

func TestEmailSettingsPatchValidationAndRedaction(t *testing.T) {
	_, client := newTestApp(t, false)
	client.login()

	var settings Settings
	client.decode(http.MethodGet, "/api/v1/settings", nil, false, http.StatusOK, &settings)
	if settings.Email.Enabled || len(settings.Email.Accounts) != 0 {
		t.Fatalf("email defaults must be off: %+v", settings.Email)
	}
	if !settings.Email.BlockRemoteImages {
		t.Fatalf("remote email images should be blocked by default")
	}

	// 启用时不再强制 host/username 已填；配置完整性由测试连接与抓取报错兜底。
	client.decode(http.MethodPatch, "/api/v1/settings", map[string]any{
		"email": map[string]any{
			"enabled": true,
			"accounts": []map[string]any{{
				"id": "mail-legacy", "host": " ", "username": "user@163.com",
			}},
		},
	}, true, http.StatusOK, &settings)
	if !settings.Email.Enabled || len(settings.Email.Accounts) != 1 {
		t.Fatalf("enabling with incomplete host must not fail: %+v", settings.Email)
	}
	if settings.Email.Accounts[0].Port != 993 || settings.Email.Accounts[0].SyncLimit != 60 {
		t.Fatalf("account defaults not normalized: %+v", settings.Email.Accounts[0])
	}

	// 正确配置：密码只存不回显；每个账号各建一个源。
	client.decode(http.MethodPatch, "/api/v1/settings", map[string]any{
		"email": map[string]any{
			"enabled": true,
			"accounts": []map[string]any{
				{
					"id": "mail-legacy", "host": "imap.163.com", "port": 993,
					"username": "user@163.com", "sync_limit": 120,
				},
				{
					"id": "mail-second", "host": "imap.qq.com", "port": 993,
					"username": "other@qq.com", "sync_limit": 30,
				},
			},
		},
	}, true, http.StatusOK, &settings)
	if len(settings.Email.Accounts) != 2 {
		t.Fatalf("two accounts should be saved: %+v", settings.Email)
	}
	client.decode(http.MethodPatch, "/api/v1/settings", map[string]any{
		"email": map[string]any{"account_id": "mail-legacy", "password": "FAKE-TEST-CODE"},
	}, true, http.StatusOK, &settings)
	client.decode(http.MethodPatch, "/api/v1/settings", map[string]any{
		"email": map[string]any{"account_id": "mail-second", "password": "FAKE-SECOND-CODE"},
	}, true, http.StatusOK, &settings)
	if !settings.Email.Accounts[0].PasswordSet || !settings.Email.Accounts[1].PasswordSet {
		t.Fatalf("email passwords not saved: %+v", settings.Email.Accounts)
	}
	raw, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToLower(string(raw)), "fake-test-code") || strings.Contains(strings.ToLower(string(raw)), "fake-second-code") {
		t.Fatalf("password leaked in settings response")
	}
	var sources []Source
	client.decode(http.MethodGet, "/api/v1/sources", nil, false, http.StatusOK, &sources)
	if len(sources) != 2 {
		t.Fatalf("each mailbox should create one source: %d", len(sources))
	}
	byURL := make(map[string]Source, len(sources))
	for _, source := range sources {
		byURL[source.FeedURL] = source
	}
	first, ok := byURL["imap://user@163.com"]
	if !ok || first.Name != "user@163.com" {
		t.Fatalf("first mailbox source = %+v", first)
	}
	second, ok := byURL["imap://other@qq.com"]
	if !ok || second.Name != "other@qq.com" {
		t.Fatalf("second mailbox source = %+v", second)
	}
	if first.Kind != "rss" || second.Kind != "rss" {
		t.Fatalf("email sources reuse kind=rss")
	}

	// 轮询间隔与分组按账号生效。
	var folder Folder
	client.decode(http.MethodPost, "/api/v1/folders", map[string]any{"name": "资讯"}, true, http.StatusCreated, &folder)
	client.decode(http.MethodPatch, "/api/v1/settings", map[string]any{
		"email": map[string]any{
			"accounts": []map[string]any{
				{
					"id": "mail-legacy", "host": "imap.163.com", "port": 993,
					"username": "user@163.com", "sync_limit": 120,
					"poll_interval_minutes": 10, "folder_id": folder.ID,
				},
				{
					"id": "mail-second", "host": "imap.qq.com", "port": 993,
					"username": "other@qq.com", "sync_limit": 30,
					"poll_interval_minutes": 30,
				},
			},
		},
	}, true, http.StatusOK, &settings)
	client.decode(http.MethodGet, "/api/v1/sources", nil, false, http.StatusOK, &sources)
	for _, source := range sources {
		switch source.FeedURL {
		case "imap://user@163.com":
			if source.RefreshSeconds != 600 || source.FolderID == nil || *source.FolderID != folder.ID {
				t.Fatalf("first mailbox should follow its own interval/group: %+v", source)
			}
		case "imap://other@qq.com":
			if source.RefreshSeconds != 1800 || source.FolderID != nil {
				t.Fatalf("second mailbox should keep its own interval/group: %+v", source)
			}
		}
	}

	// 旧版按发件人源仍允许手动创建，但保存邮箱设置后会被停用。
	var perSender Source
	client.decode(http.MethodPost, "/api/v1/sources", map[string]any{
		"kind": "rss", "name": "Newsletter", "feed_url": "imap://user@163.com/news@substack.com",
	}, true, http.StatusCreated, &perSender)
	client.decode(http.MethodPatch, "/api/v1/settings", map[string]any{
		"email": map[string]any{"accounts": settings.Email.Accounts},
	}, true, http.StatusOK, &settings)
	client.decode(http.MethodGet, "/api/v1/sources", nil, false, http.StatusOK, &sources)
	for _, source := range sources {
		if source.ID == perSender.ID && source.Enabled {
			t.Fatalf("legacy per-sender source should be disabled: %+v", source)
		}
	}

	// 关闭邮箱同步时停用所有 IMAP 源；重新启用后恢复邮箱源。
	client.decode(http.MethodPatch, "/api/v1/settings", map[string]any{
		"email": map[string]any{"enabled": false},
	}, true, http.StatusOK, &settings)
	client.decode(http.MethodGet, "/api/v1/sources", nil, false, http.StatusOK, &sources)
	for _, source := range sources {
		if source.Enabled && strings.HasPrefix(source.FeedURL, "imap://") {
			t.Fatalf("disabling email sync should pause IMAP sources: %+v", source)
		}
	}
	client.decode(http.MethodPatch, "/api/v1/settings", map[string]any{
		"email": map[string]any{"enabled": true},
	}, true, http.StatusOK, &settings)
	client.decode(http.MethodGet, "/api/v1/sources", nil, false, http.StatusOK, &sources)
	for _, source := range sources {
		if strings.HasPrefix(source.FeedURL, "imap://") && source.FeedURL != "imap://user@163.com/news@substack.com" && !source.Enabled {
			t.Fatalf("mailbox source should be enabled again: %+v", source)
		}
	}

	// 清除某个账号的授权码不影响另一个账号。
	client.decode(http.MethodPatch, "/api/v1/settings", map[string]any{
		"email": map[string]any{"account_id": "mail-legacy", "password": nil},
	}, true, http.StatusOK, &settings)
	if settings.Email.Accounts[0].PasswordSet || !settings.Email.Accounts[1].PasswordSet {
		t.Fatalf("clearing one password must not affect the other: %+v", settings.Email.Accounts)
	}

	// 信任发件人列表按小写去重保存。
	client.decode(http.MethodPatch, "/api/v1/settings", map[string]any{
		"email": map[string]any{"trusted_senders": []string{"News@Example.com", "news@example.com", "bad"}},
	}, true, http.StatusOK, &settings)
	if len(settings.Email.TrustedSenders) != 1 || settings.Email.TrustedSenders[0] != "news@example.com" {
		t.Fatalf("trusted senders not normalized: %+v", settings.Email.TrustedSenders)
	}
}

func TestEmailInlineImagesRegisteredIntoMediaCache(t *testing.T) {
	app, _ := newTestApp(t, false)
	ctx := context.Background()
	messageID := "<test@example.com>"
	png := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	images := []fetch.EmailInlineImage{{CID: "img@x", ContentType: "image/png", Data: png}}
	if err := app.registerEmailInlineImages(ctx, messageID, images); err != nil {
		t.Fatalf("registerEmailInlineImages: %v", err)
	}
	hash := fetch.URLHash(fetch.EmailInlinePseudoURL(messageID, "img@x"))
	row, err := app.mediaCacheRow(ctx, hash)
	if err != nil {
		t.Fatalf("media cache row: %v", err)
	}
	if row.Status != "ok" || !row.LocalPath.Valid {
		t.Fatalf("inline image should be ready: %+v", row)
	}
}

func TestEmailItemRoutingWithoutIMAPServer(t *testing.T) {
	// 不经过 IMAP：直接用 applyEmailMessage 验证入库、去重与源更新路径。
	app, client := newTestApp(t, false)
	client.login()
	ctx := context.Background()

	// 预置授权码，聚合箱配置。
	if err := app.writeSetting(ctx, emailPasswordSettingKey("mail-legacy"), mustEncryptForTest(app, "FAKE-TEST-CODE")); err != nil {
		t.Fatal(err)
	}
	var settings Settings
	client.decode(http.MethodPatch, "/api/v1/settings", map[string]any{
		"email": map[string]any{
			"enabled": true,
			"accounts": []map[string]any{{
				"id": "mail-legacy", "host": "imap.example.com", "username": "user@163.com",
			}},
		},
	}, true, http.StatusOK, &settings)

	var sources []Source
	client.decode(http.MethodGet, "/api/v1/sources", nil, false, http.StatusOK, &sources)
	if len(sources) != 1 {
		t.Fatalf("aggregate source missing: %d", len(sources))
	}
	target, err := app.sourceInternal(ctx, sources[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	started := app.now()
	message := fetch.EmailMessage{
		MessageID: "<weekly-1@substack.com>",
		FromName:  "Weekly",
		FromAddr:  "news@substack.com",
		Subject:   "第 1 期",
		HTML:      "<p>你好，这是第一期。</p>",
	}
	if err := app.applyEmailMessage(ctx, target, message, started); err != nil {
		t.Fatalf("applyEmailMessage: %v", err)
	}
	page, err := app.queryItems(ctx, itemFilter{sourceID: &sources[0].ID, limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("item should be inserted: %d", len(page.Items))
	}
	if page.Items[0].Title != "第 1 期" {
		t.Fatalf("title = %q", page.Items[0].Title)
	}
	// 同一封邮件再次投递不产生重复条目（guid 去重）。
	target, err = app.sourceInternal(ctx, sources[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := app.applyEmailMessage(ctx, target, message, started); err != nil {
		t.Fatalf("applyEmailMessage again: %v", err)
	}
	page, err = app.queryItems(ctx, itemFilter{sourceID: &sources[0].ID, limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("duplicate email must not create a second item: %d", len(page.Items))
	}

	// REQ-044：单封放行后，同一封邮件正文更新也不能丢掉图片放行状态。
	itemID := page.Items[0].ID
	var detail ItemDetail
	client.decode(http.MethodGet, "/api/v1/items/"+itoa(itemID), nil, false, http.StatusOK, &detail)
	if detail.EmailImagesAllowed {
		t.Fatalf("images should start blocked: %+v", detail)
	}
	client.decode(http.MethodPatch, "/api/v1/items/"+itoa(itemID)+"/email-images", map[string]any{
		"allowed": true,
	}, true, http.StatusOK, &detail)
	message.HTML = "<p>你好，这是更新后的第一期。</p>"
	target, err = app.sourceInternal(ctx, sources[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := app.applyEmailMessage(ctx, target, message, started); err != nil {
		t.Fatalf("applyEmailMessage after allow: %v", err)
	}
	client.decode(http.MethodGet, "/api/v1/items/"+itoa(itemID), nil, false, http.StatusOK, &detail)
	if !detail.EmailImagesAllowed {
		t.Fatalf("image allow must survive re-fetch: %+v", detail)
	}
}

// 连接测试成功后必须清掉邮箱源的旧错误并立即重试，否则源会一直挂在
// 「需要关注」列表里（用户 2026-11-21 反馈）。
func TestEmailTestSuccessResetsStaleSourceErrors(t *testing.T) {
	app, client := newTestApp(t, false)
	client.login()
	ctx := context.Background()
	if err := app.writeSetting(ctx, emailPasswordSettingKey("mail-legacy"), mustEncryptForTest(app, "FAKE-TEST-CODE")); err != nil {
		t.Fatal(err)
	}
	var settings Settings
	client.decode(http.MethodPatch, "/api/v1/settings", map[string]any{
		"email": map[string]any{
			"enabled": true,
			"accounts": []map[string]any{{
				"id": "mail-legacy", "host": "imap.example.com", "username": "user@163.com",
			}},
		},
	}, true, http.StatusOK, &settings)
	var sources []Source
	client.decode(http.MethodGet, "/api/v1/sources", nil, false, http.StatusOK, &sources)
	if len(sources) != 1 {
		t.Fatalf("aggregate source missing: %d", len(sources))
	}
	farFuture := app.now().Add(24 * time.Hour).Format(timeRFC3339)
	if _, err := app.db.Exec(`
		UPDATE sources SET last_status = 'error', last_error = '连接 IMAP 服务器失败：timeout',
			fail_count = 5, next_fetch_at = ? WHERE id = ?`, farFuture, sources[0].ID); err != nil {
		t.Fatal(err)
	}

	app.resetEmailSourceHealth(ctx, "User@163.com")

	var status, lastErr sql.NullString
	var failCount int
	var nextFetch string
	if err := app.db.QueryRow(`
		SELECT last_status, last_error, fail_count, next_fetch_at FROM sources WHERE id = ?`,
		sources[0].ID).Scan(&status, &lastErr, &failCount, &nextFetch); err != nil {
		t.Fatal(err)
	}
	if status.Valid || lastErr.Valid || failCount != 0 {
		t.Fatalf("stale error state not cleared: %v %v %d", status, lastErr, failCount)
	}
	if nextFetch > app.now().Add(time.Minute).Format(timeRFC3339) {
		t.Fatalf("next_fetch_at should be immediate, got %s", nextFetch)
	}
}

func TestNormalizeEmailSourcesRepairsLegacySources(t *testing.T) {
	app, client := newTestApp(t, false)
	client.login()
	ctx := context.Background()
	if err := app.writeSetting(ctx, emailPasswordSettingKey("mail-legacy"), mustEncryptForTest(app, "FAKE-TEST-CODE")); err != nil {
		t.Fatal(err)
	}
	var settings Settings
	client.decode(http.MethodPatch, "/api/v1/settings", map[string]any{
		"email": map[string]any{
			"enabled": true,
			"accounts": []map[string]any{{
				"id": "mail-legacy", "host": "imap.163.com", "username": "user@163.com",
			}},
		},
	}, true, http.StatusOK, &settings)
	var sources []Source
	client.decode(http.MethodGet, "/api/v1/sources", nil, false, http.StatusOK, &sources)
	if len(sources) != 1 {
		t.Fatalf("mailbox source missing: %d", len(sources))
	}
	// 模拟旧版本：聚合源不存在，只有一个按发件人源。
	if _, err := app.db.Exec(`DELETE FROM sources WHERE lower(feed_url) = lower(?)`, "imap://user@163.com"); err != nil {
		t.Fatal(err)
	}
	if _, err := app.createSourceRow(ctx, sourceCreateInput{
		Kind: "rss", Name: "Weekly", FeedURL: "imap://user@163.com/news@substack.com",
		Note: ptrString("邮件订阅（按发件人自动创建）"),
	}); err != nil {
		t.Fatal(err)
	}
	app.normalizeEmailSources(ctx)
	client.decode(http.MethodGet, "/api/v1/sources", nil, false, http.StatusOK, &sources)
	var aggregate, legacyRow *Source
	for i := range sources {
		switch sources[i].FeedURL {
		case "imap://user@163.com":
			aggregate = &sources[i]
		case "imap://user@163.com/news@substack.com":
			legacyRow = &sources[i]
		}
	}
	if aggregate == nil || aggregate.Name != "user@163.com" {
		t.Fatalf("legacy mailbox source name not repaired: %+v", aggregate)
	}
	if legacyRow == nil || legacyRow.Enabled {
		t.Fatalf("legacy per-sender source should be disabled: %+v", legacyRow)
	}
	if legacyRow.Note == nil || *legacyRow.Note != emailLegacySendNote {
		t.Fatalf("legacy note = %v", legacyRow.Note)
	}
}

func TestEmailSyncStateRoundTrip(t *testing.T) {
	app, _ := newTestApp(t, false)
	ctx := context.Background()
	state := fetch.EmailSyncState{
		Username: "user@163.com",
		Folders: map[string]fetch.EmailFolderState{
			"INBOX": {UIDValidity: 7, LastUID: 42},
		},
	}
	if err := app.saveEmailSyncState(ctx, "mail-legacy", state); err != nil {
		t.Fatal(err)
	}
	loaded := app.loadEmailSyncState(ctx, "mail-legacy")
	if loaded.Username != state.Username || loaded.Folders["INBOX"] != state.Folders["INBOX"] {
		t.Fatalf("state did not round-trip: %+v", loaded)
	}
}

func mustEncryptForTest(app *App, value string) string {
	encrypted, err := app.secret.Encrypt(value)
	if err != nil {
		panic(err)
	}
	return encrypted
}

func TestDecorateEmailImageAttrsBackfillsLegacyContent(t *testing.T) {
	remote := "https://cdn.example.com/logo.png"
	inline := fetch.EmailInlinePseudoURL("<m@example.com>", "img@x")
	content := `<p><img src="/media/token/` + fetch.URLHash(remote) + `" alt="logo"><img src="/media/token/` + fetch.URLHash(inline) + `" alt="inline"></p>`
	decorated := decorateEmailImageAttrs(content, []string{remote, inline}, "token")
	if !strings.Contains(decorated, `data-remote-src="`+remote+`"`) {
		t.Fatalf("remote image attr missing: %s", decorated)
	}
	if !strings.Contains(decorated, `data-inline-image="1"`) {
		t.Fatalf("inline marker missing: %s", decorated)
	}
}
