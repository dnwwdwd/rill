package server

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"rill/internal/fetch"
)

const (
	settingEmailPassword    = "email_password_enc"
	settingEmailSyncState   = "email_sync_state"
	legacyEmailAccountID    = "mail-legacy"
	emailPasswordKeyPrefix  = "email_password_enc:"
	emailSyncStateKeyPrefix = "email_sync_state:"
)

const (
	emailDefaultPort      = 993
	emailDefaultInterval  = 15
	emailMinInterval      = 5
	emailMaxInterval      = 1440
	emailDefaultSyncLimit = 60
	emailMinSyncLimit     = 10
	emailMaxSyncLimit     = 500
	emailAggregateNote    = "邮箱自动同步（所有文件夹）"
	emailLegacySendNote   = "旧版按发件人分源，已改为邮箱自动同步，可手动删除"
)

var emailAccountIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{8,64}$`)

func newEmailAccountID() (string, error) {
	buffer := make([]byte, 8)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return "mail-" + hex.EncodeToString(buffer), nil
}

func emailPasswordSettingKey(accountID string) string {
	return emailPasswordKeyPrefix + accountID
}

func emailSyncStateSettingKey(accountID string) string {
	return emailSyncStateKeyPrefix + accountID
}

func normalizeTrustedEmailSenders(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	clean := make([]string, 0, len(values))
	for _, value := range values {
		address := strings.ToLower(strings.TrimSpace(value))
		if address == "" || len(address) > 320 || !strings.Contains(address, "@") {
			continue
		}
		if _, exists := seen[address]; exists {
			continue
		}
		seen[address] = struct{}{}
		clean = append(clean, address)
	}
	return clean
}

func blockRemoteEmailImages(settings internalEmailSettings) bool {
	if settings.BlockRemoteImages == nil {
		return true
	}
	return *settings.BlockRemoteImages
}

func findEmailAccount(settings internalEmailSettings, username string) (internalEmailAccount, bool) {
	username = strings.ToLower(strings.TrimSpace(username))
	for _, account := range settings.Accounts {
		if strings.ToLower(strings.TrimSpace(account.Username)) == username {
			return account, true
		}
	}
	return internalEmailAccount{}, false
}

// parseEmailFeedURL splits an `imap://{username}` or legacy
// `imap://{username}/{sender}` feed URL. Deliberately string-based (not
// url.Parse): the username itself contains '@', which url.Parse would treat as
// a userinfo separator.
func parseEmailFeedURL(raw string) (username, sender string, ok bool) {
	rest, found := strings.CutPrefix(strings.TrimSpace(raw), "imap://")
	if !found {
		return "", "", false
	}
	rest = strings.TrimSuffix(strings.TrimSuffix(rest, "/"), "?")
	if idx := strings.IndexAny(rest, "?#"); idx >= 0 {
		rest = rest[:idx]
	}
	username, sender, hasSender := strings.Cut(rest, "/")
	username = strings.ToLower(strings.TrimSpace(username))
	sender = strings.ToLower(strings.TrimSpace(sender))
	if !hasSender {
		sender = ""
	}
	if username == "" {
		return "", "", false
	}
	return username, sender, true
}

func emailFeedURL(username string, sender string) string {
	if strings.TrimSpace(sender) == "" {
		return "imap://" + strings.ToLower(strings.TrimSpace(username))
	}
	return "imap://" + strings.ToLower(strings.TrimSpace(username)) + "/" + strings.ToLower(strings.TrimSpace(sender))
}

func (a *App) emailAccountPassword(ctx context.Context, accountID string) (string, error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return "", nil
	}
	value, err := a.readSetting(ctx, emailPasswordSettingKey(accountID))
	if errors.Is(err, sql.ErrNoRows) && accountID == legacyEmailAccountID {
		// 旧版单账号授权码的键名没有后缀，首次升级时继续可用。
		value, err = a.readSetting(ctx, settingEmailPassword)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil || value == "" {
		return "", err
	}
	return a.secret.Decrypt(value)
}

func (a *App) loadEmailSyncState(ctx context.Context, accountID string) fetch.EmailSyncState {
	var state fetch.EmailSyncState
	raw, err := a.readSetting(ctx, emailSyncStateSettingKey(accountID))
	if errors.Is(err, sql.ErrNoRows) && accountID == legacyEmailAccountID {
		raw, err = a.readSetting(ctx, settingEmailSyncState)
	}
	if errors.Is(err, sql.ErrNoRows) || raw == "" {
		return state
	}
	if err != nil {
		a.logger.Warn("read email sync state failed", "err", err)
		return state
	}
	if err := json.Unmarshal([]byte(raw), &state); err != nil {
		a.logger.Warn("decode email sync state failed", "err", err)
		return fetch.EmailSyncState{}
	}
	if state.Folders == nil {
		state.Folders = make(map[string]fetch.EmailFolderState)
	}
	return state
}

func (a *App) saveEmailSyncState(ctx context.Context, accountID string, state fetch.EmailSyncState) error {
	if state.Folders == nil {
		state.Folders = make(map[string]fetch.EmailFolderState)
	}
	raw, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return a.writeSetting(ctx, emailSyncStateSettingKey(accountID), string(raw))
}

// syncEmailMailbox mirrors one configured IMAP account into its
// `imap://{username}` source. It walks every syncable folder, advances UID
// cursors, and never changes \Seen in the user's mailbox.
// Errors are plain (not apiError) so their text lands in last_error verbatim.
func (a *App) syncEmailMailbox(ctx context.Context, trigger dbSource) error {
	triggerUsername, _, ok := parseEmailFeedURL(trigger.FeedURL)
	if !ok {
		return errors.New("邮箱同步源地址无效")
	}
	internal, err := a.loadInternalSettings(ctx)
	if err != nil {
		return internalError(err)
	}
	email := internal.Email
	if !email.Enabled {
		return errors.New("邮箱同步未启用：请在设置中配置 IMAP 邮箱")
	}
	account, found := findEmailAccount(email, triggerUsername)
	if !found {
		return errors.New("邮箱账号已在设置中移除，请删除该源或重新添加账号")
	}
	password, err := a.emailAccountPassword(ctx, account.ID)
	if err != nil {
		return internalError(err)
	}
	if password == "" {
		return errors.New("未配置邮箱授权码：请在设置中填写客户端授权码")
	}
	cfg := fetch.IMAPConfig{
		Host:     account.Host,
		Port:     account.Port,
		Username: account.Username,
		Password: password,
	}
	previous := a.loadEmailSyncState(ctx, account.ID)
	result, err := fetch.FetchNewEmails(ctx, cfg, previous, account.SyncLimit)
	if err != nil {
		return err
	}
	for _, warning := range result.Warnings {
		a.logger.Warn("email folder skipped", "err", warning)
	}
	if result.Folders == 0 && len(result.Warnings) > 0 {
		return fmt.Errorf("所有邮箱文件夹同步失败：%v", result.Warnings[0])
	}
	if len(result.Emails) == 0 {
		if err := a.saveEmailSyncState(ctx, account.ID, result.State); err != nil {
			return internalError(err)
		}
		return nil
	}
	target, err := a.emailTargetSource(ctx, account)
	if err != nil {
		return err
	}
	started := a.now()
	failedFolders := make(map[string]bool)
	for _, rawEmail := range result.Emails {
		message, parseErr := fetch.ParseEmailMessage(rawEmail.Raw)
		if parseErr != nil {
			a.logger.Warn("email parse failed", "mailbox", rawEmail.Mailbox, "uid", rawEmail.UID, "err", parseErr)
			failedFolders[rawEmail.Mailbox] = true
			continue
		}
		if err := a.applyEmailMessage(ctx, target, message, started); err != nil {
			a.logger.Warn("email item apply failed", "mailbox", rawEmail.Mailbox, "uid", rawEmail.UID, "err", err)
			failedFolders[rawEmail.Mailbox] = true
		}
	}
	// Failed folders keep their old cursor so the next poll retries them;
	// already-applied messages are deduplicated by Message-ID.
	for folder := range failedFolders {
		if old, exists := previous.Folders[folder]; exists {
			result.State.Folders[folder] = old
		} else {
			delete(result.State.Folders, folder)
		}
	}
	if err := a.saveEmailSyncState(ctx, account.ID, result.State); err != nil {
		return internalError(err)
	}
	return nil
}

// emailTargetSource resolves one mailbox source. Legacy per-sender sources
// are no longer created; they are disabled by ensureEmailSources.
func (a *App) emailTargetSource(ctx context.Context, account internalEmailAccount) (dbSource, error) {
	username := strings.ToLower(strings.TrimSpace(account.Username))
	feedURL := emailFeedURL(username, "")
	row := a.db.QueryRowContext(ctx, `SELECT `+sourceSelectColumns+` FROM sources s WHERE lower(s.feed_url) = lower(?)`, feedURL)
	source, err := scanSource(row)
	if errors.Is(err, sql.ErrNoRows) {
		created, createErr := a.createSourceRow(ctx, sourceCreateInput{
			Kind:           "rss",
			Name:           aggregateNameForEmail(account),
			FeedURL:        feedURL,
			FolderID:       account.FolderID,
			RefreshSeconds: intPtr(account.PollIntervalMinutes * 60),
			Note:           ptrString(emailAggregateNote),
		})
		if createErr != nil {
			return dbSource{}, createErr
		}
		return a.sourceInternal(ctx, created.ID)
	}
	if err != nil {
		return dbSource{}, err
	}
	if source.FolderID == nil && account.FolderID != nil {
		if _, err := a.db.ExecContext(ctx, `UPDATE sources SET folder_id = ?, updated_at = ? WHERE id = ?`,
			account.FolderID, a.now().Format(timeRFC3339), source.ID); err != nil {
			return dbSource{}, err
		}
		source.FolderID = account.FolderID
	}
	// scanSource does not load feed_token/ETag/initial_fetched; email ingestion
	// needs the token to build /media/{token}/{hash} URLs. Without this, every
	// existing mailbox sync wrote /media//{hash} and its images 404'd.
	return a.sourceInternal(ctx, source.ID)
}

func aggregateNameForEmail(account internalEmailAccount) string {
	if username := strings.TrimSpace(account.Username); username != "" {
		return username
	}
	return "邮箱"
}

// applyEmailMessage registers inline images, sanitizes/inserts the item and
// lets applyFeedResult handle filters, watch notifications and fetch logs.
func (a *App) applyEmailMessage(ctx context.Context, target dbSource, message fetch.EmailMessage, started time.Time) error {
	if err := a.registerEmailInlineImages(ctx, message.MessageID, message.Inline); err != nil {
		a.logger.Warn("email inline image registration failed", "err", err)
	}
	item := fetch.BuildEmailFeedItem(message, target.FeedToken)
	result := &fetch.FeedResult{
		Title:      target.Name,
		Link:       item.URL,
		HTTPStatus: 0,
		Items:      []fetch.FeedItem{item},
	}
	duration := time.Since(started).Milliseconds()
	_, _, err := a.applyFeedResult(ctx, target, result, started, duration, "", target.InitialFetched && target.WatchEnabled)
	return err
}

// registerEmailInlineImages writes cid: attachment bytes into the media cache
// with status='ok' so the /media/{token}/{hash} endpoint serves them locally
// without a network fetch.
func (a *App) registerEmailInlineImages(ctx context.Context, messageID string, images []fetch.EmailInlineImage) error {
	if len(images) == 0 {
		return nil
	}
	mediaDir := filepath.Join(a.cfg.CacheDir, "media")
	if err := os.MkdirAll(mediaDir, 0o750); err != nil {
		return err
	}
	now := a.now().Format(timeRFC3339)
	for _, image := range images {
		hash := fetch.URLHash(fetch.EmailInlinePseudoURL(messageID, image.CID))
		contentType := strings.ToLower(strings.TrimSpace(image.ContentType))
		if !strings.HasPrefix(contentType, "image/") || strings.Contains(contentType, "svg") {
			contentType = http.DetectContentType(image.Data)
		}
		if !strings.HasPrefix(contentType, "image/") {
			continue
		}
		finalPath := filepath.Join(mediaDir, hash)
		if _, err := os.Stat(finalPath); err != nil {
			temp, err := os.CreateTemp(mediaDir, hash+".tmp-*")
			if err != nil {
				return err
			}
			if _, err := temp.Write(image.Data); err != nil {
				_ = temp.Close()
				_ = os.Remove(temp.Name())
				return err
			}
			if err := temp.Close(); err != nil {
				_ = os.Remove(temp.Name())
				return err
			}
			if err := os.Chmod(temp.Name(), 0o640); err != nil {
				_ = os.Remove(temp.Name())
				return err
			}
			if err := os.Rename(temp.Name(), finalPath); err != nil {
				_ = os.Remove(temp.Name())
				return err
			}
		}
		if _, err := a.db.ExecContext(ctx, `
			INSERT INTO media_cache (url_hash, source_url, local_path, content_type, size_bytes, status, fail_count, fetched_at, last_access_at)
			VALUES (?, ?, ?, ?, ?, 'ok', 0, ?, ?)
			ON CONFLICT(url_hash) DO UPDATE SET
				local_path = excluded.local_path,
				content_type = excluded.content_type,
				size_bytes = excluded.size_bytes,
				status = 'ok',
				fail_count = 0,
				fetched_at = excluded.fetched_at,
				last_access_at = excluded.last_access_at`,
			hash, fetch.EmailInlinePseudoURL(messageID, image.CID), finalPath, contentType, len(image.Data), now, now); err != nil {
			return err
		}
	}
	if err := a.enforceMediaQuota(ctx); err != nil {
		a.logger.Warn("media quota cleanup failed", "err", err)
	}
	return nil
}

// applyEmailSourceOK refreshes the triggering source's health after a clean
// mailbox sync, even when the messages were routed to other sources.
func (a *App) applyEmailSourceOK(ctx context.Context, source dbSource, started time.Time, duration int64) error {
	next := nextFetchTime(a.now(), source.RefreshSeconds)
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `
		UPDATE sources SET last_fetch_at = ?, last_status = 'ok', last_error = NULL,
			fail_count = 0, next_fetch_at = ?, updated_at = ? WHERE id = ?`,
		started.UTC().Format(timeRFC3339), next.UTC().Format(timeRFC3339),
		a.now().Format(timeRFC3339), source.ID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO fetch_logs (source_id, started_at, duration_ms, http_status, result, item_count, new_count, error)
		VALUES (?, ?, ?, NULL, 'ok', 0, 0, NULL)`,
		source.ID, started.UTC().Format(timeRFC3339), duration); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM fetch_logs WHERE source_id = ? AND id NOT IN (
			SELECT id FROM fetch_logs WHERE source_id = ? ORDER BY started_at DESC, id DESC LIMIT 50
		)`, source.ID, source.ID); err != nil {
		return err
	}
	return tx.Commit()
}

// ensureEmailSources keeps one mailbox source per configured account in sync
// with settings. Removed/disabled accounts and legacy per-sender sources are
// paused but keep their content for manual cleanup.
func (a *App) ensureEmailSources(ctx context.Context) error {
	internal, err := a.loadInternalSettings(ctx)
	if err != nil {
		return err
	}
	email := internal.Email
	now := a.now().Format(timeRFC3339)
	active := make(map[string]struct{}, len(email.Accounts))
	if email.Enabled {
		for _, account := range email.Accounts {
			username := strings.ToLower(strings.TrimSpace(account.Username))
			if username == "" {
				continue
			}
			feedURL := emailFeedURL(username, "")
			active[feedURL] = struct{}{}
			password, err := a.emailAccountPassword(ctx, account.ID)
			if err != nil {
				return err
			}
			if password == "" || strings.TrimSpace(account.Host) == "" {
				// 未配置完整时不建源，避免制造必然失败的源；测试连接会提示。
				if _, err := a.db.ExecContext(ctx,
					`UPDATE sources SET enabled = 0, updated_at = ? WHERE lower(feed_url) = lower(?)`,
					now, feedURL); err != nil {
					return err
				}
				continue
			}
			refresh := account.PollIntervalMinutes * 60
			if refresh < emailMinInterval*60 {
				refresh = emailMinInterval * 60
			}
			var existingID int64
			var existingName string
			err = a.db.QueryRowContext(ctx, `SELECT id, name FROM sources WHERE lower(feed_url) = lower(?)`, feedURL).
				Scan(&existingID, &existingName)
			switch {
			case errors.Is(err, sql.ErrNoRows):
				if _, err := a.createSourceRow(ctx, sourceCreateInput{
					Kind:           "rss",
					Name:           aggregateNameForEmail(account),
					FeedURL:        feedURL,
					FolderID:       account.FolderID,
					RefreshSeconds: &refresh,
					Note:           ptrString(emailAggregateNote),
				}); err != nil {
					var apiErr *apiError
					if !errors.As(err, &apiErr) || apiErr.Code != "source_exists" {
						return err
					}
				}
			case err != nil:
				return err
			default:
				updates := []string{"enabled = 1", "folder_id = ?", "refresh_seconds = ?", "updated_at = ?"}
				args := []any{account.FolderID, refresh, now}
				if strings.TrimSpace(existingName) == "" || existingName == "邮件收集箱" {
					updates = append(updates, "name = ?")
					args = append(args, aggregateNameForEmail(account))
				}
				args = append(args, existingID)
				if _, err := a.db.ExecContext(ctx, "UPDATE sources SET "+strings.Join(updates, ", ")+" WHERE id = ?", args...); err != nil {
					return err
				}
			}
		}
	}
	// 停用所有不在当前账号列表里的 IMAP 源：被移除的账号、旧版按发件人源
	// 都保留已抓内容，但不再调度。关闭邮箱同步时则停用全部。
	rows, err := a.db.QueryContext(ctx, `SELECT id, feed_url FROM sources WHERE lower(feed_url) LIKE 'imap://%'`)
	if err != nil {
		return err
	}
	type inactiveSource struct {
		id      int64
		feedURL string
	}
	var inactive []inactiveSource
	for rows.Next() {
		var id int64
		var feedURL string
		if err := rows.Scan(&id, &feedURL); err != nil {
			_ = rows.Close()
			return err
		}
		if _, ok := active[strings.ToLower(feedURL)]; !ok {
			inactive = append(inactive, inactiveSource{id: id, feedURL: feedURL})
		}
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, source := range inactive {
		if _, err := a.db.ExecContext(ctx, `
			UPDATE sources SET enabled = 0,
				note = CASE WHEN note = ? THEN ? ELSE note END,
				updated_at = ?
			WHERE id = ?`,
			"邮件订阅（按发件人自动创建）", emailLegacySendNote, now, source.id); err != nil {
			return err
		}
	}
	return nil
}

// resetEmailSourceHealth clears stale error state after a successful
// connection test (or settings save) and schedules an immediate retry, so a
// recovered mailbox disappears from the "needs attention" list instead of
// keeping the last network error until the next scheduled poll.
func (a *App) resetEmailSourceHealth(ctx context.Context, username string) {
	username = strings.ToLower(strings.TrimSpace(username))
	if username == "" {
		return
	}
	feedURL := emailFeedURL(username, "")
	now := a.now().Format(timeRFC3339)
	if _, err := a.db.ExecContext(ctx, `
		UPDATE sources SET last_status = NULL, last_error = NULL, fail_count = 0,
			next_fetch_at = ?, updated_at = ?
		WHERE enabled = 1 AND (lower(feed_url) = lower(?) OR lower(feed_url) LIKE ?)`,
		now, now, feedURL, feedURL+"/%"); err != nil {
		a.logger.Warn("reset email source health failed", "err", err)
	}
}

// normalizeEmailSources repairs sources created by older builds on startup.
// ensureEmailSources is idempotent and also migrates the legacy single account.
func (a *App) normalizeEmailSources(ctx context.Context) {
	if err := a.ensureEmailSources(ctx); err != nil {
		a.logger.Warn("normalize email sources failed", "err", err)
	}
}

func (a *App) handleTestEmail(w http.ResponseWriter, r *http.Request) error {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<16))
	if err != nil {
		return badRequest("invalid_json", "读取请求内容失败")
	}
	var payload struct {
		AccountID string `json:"account_id"`
		Host      string `json:"host"`
		Port      int    `json:"port"`
		Username  string `json:"username"`
		Password  string `json:"password"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return badRequest("invalid_json", "请求内容不是合法 JSON")
	}
	accountID := strings.TrimSpace(payload.AccountID)
	cfg := fetch.IMAPConfig{
		Host:     strings.TrimSpace(payload.Host),
		Port:     payload.Port,
		Username: strings.TrimSpace(payload.Username),
		Password: payload.Password,
	}
	if accountID != "" {
		// 只改其它字段或使用已保存授权码时，用账号设置补齐缺失项。
		internal, err := a.loadInternalSettings(r.Context())
		if err != nil {
			return internalError(err)
		}
		for _, account := range internal.Email.Accounts {
			if account.ID != accountID {
				continue
			}
			if cfg.Host == "" {
				cfg.Host = account.Host
			}
			if cfg.Port <= 0 {
				cfg.Port = account.Port
			}
			if cfg.Username == "" {
				cfg.Username = account.Username
			}
			break
		}
	}
	if cfg.Password == "" && accountID != "" {
		stored, err := a.emailAccountPassword(r.Context(), accountID)
		if err == nil {
			cfg.Password = stored
		}
	}
	if cfg.Port <= 0 {
		cfg.Port = emailDefaultPort
	}
	mailboxes, err := fetch.ListMailboxes(r.Context(), cfg)
	if err != nil {
		return badRequest("email_test_failed", readableEmailError(err))
	}
	// 测试成功说明账号与网络恢复：清掉邮箱源的旧错误并立即重试，避免
	// 「连接测试成功但源仍挂在需关注列表」。
	a.resetEmailSourceHealth(r.Context(), cfg.Username)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "mailboxes": mailboxes})
	return nil
}

func readableEmailError(err error) string {
	message := strings.TrimSpace(err.Error())
	if message == "" {
		return "连接失败"
	}
	if len([]rune(message)) > 300 {
		message = string([]rune(message)[:300]) + "…"
	}
	return message
}

func intPtr(value int) *int { return &value }
