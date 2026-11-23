package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const (
	settingAppSettings = "app_settings"
	settingGlobalToken = "global_feed_token"
	settingAIKey       = "ai_api_key_enc"
	settingAIUsage     = "ai_usage"
	settingLastLoginAt = "last_login_at"
	settingAccount     = "account_name"
)

func (a *App) bootstrapSettings(ctx context.Context) error {
	if _, err := a.readSetting(ctx, settingAppSettings); errors.Is(err, sql.ErrNoRows) {
		if err := a.writeSetting(ctx, settingAppSettings, mustJSON(defaultInternalSettings())); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	if _, err := a.readSetting(ctx, settingGlobalToken); errors.Is(err, sql.ErrNoRows) {
		token, err := newFeedToken()
		if err != nil {
			return err
		}
		if err := a.writeSetting(ctx, settingGlobalToken, token); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	if _, err := a.readSetting(ctx, settingAIUsage); errors.Is(err, sql.ErrNoRows) {
		if err := a.saveAIUsage(ctx, aiUsage{Date: a.today(), Month: a.thisMonth()}); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	return nil
}

func (a *App) readSetting(ctx context.Context, key string) (string, error) {
	var value string
	err := a.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
	return value, err
}

func (a *App) writeSetting(ctx context.Context, key, value string) error {
	_, err := a.db.ExecContext(ctx, `
		INSERT INTO settings (key, value, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		key, value, a.now().UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("write setting %s: %w", key, err)
	}
	return nil
}

func (a *App) deleteSetting(ctx context.Context, key string) error {
	_, err := a.db.ExecContext(ctx, `DELETE FROM settings WHERE key = ?`, key)
	return err
}

func (a *App) loadInternalSettings(ctx context.Context) (internalSettings, error) {
	var result internalSettings
	raw, err := a.readSetting(ctx, settingAppSettings)
	if err != nil {
		return result, err
	}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return result, fmt.Errorf("decode app settings: %w", err)
	}
	if result.Reader.Accent == "" {
		result.Reader.Accent = "green"
	}
	if result.Reader.RailWidth < 180 || result.Reader.RailWidth > 320 {
		result.Reader.RailWidth = 224
	}
	if result.Reader.ListWidth < 300 || result.Reader.ListWidth > 520 {
		result.Reader.ListWidth = 384
	}
	if result.Reader.PinnedSources == nil {
		result.Reader.PinnedSources = []int64{}
	}
	if result.Email.BlockRemoteImages == nil {
		defaultBlock := true
		result.Email.BlockRemoteImages = &defaultBlock
	}
	result.Email.TrustedSenders = normalizeTrustedEmailSenders(result.Email.TrustedSenders)
	if len(result.Email.Accounts) == 0 && strings.TrimSpace(result.Email.Username) != "" {
		// REQ-041 初版只支持一个邮箱：把旧字段迁移成第一个账号，授权码
		// 继续从旧 `email_password_enc` 读取，用户无需重新输入。
		port := result.Email.Port
		if port <= 0 {
			port = emailDefaultPort
		}
		poll := result.Email.PollIntervalMinutes
		if poll < emailMinInterval || poll > emailMaxInterval {
			poll = emailDefaultInterval
		}
		result.Email.Accounts = []internalEmailAccount{{
			ID:                  legacyEmailAccountID,
			Host:                strings.TrimSpace(result.Email.Host),
			Port:                port,
			Username:            strings.TrimSpace(result.Email.Username),
			PollIntervalMinutes: poll,
			SyncLimit:           emailDefaultSyncLimit,
			FolderID:            result.Email.FolderID,
		}}
		result.Email.Host = ""
		result.Email.Port = 0
		result.Email.Username = ""
		result.Email.PollIntervalMinutes = 0
		result.Email.FolderID = nil
	}
	for i := range result.Email.Accounts {
		account := &result.Email.Accounts[i]
		account.Host = strings.TrimSpace(account.Host)
		account.Username = strings.TrimSpace(account.Username)
		if account.Port <= 0 {
			account.Port = emailDefaultPort
		}
		if account.PollIntervalMinutes < emailMinInterval || account.PollIntervalMinutes > emailMaxInterval {
			account.PollIntervalMinutes = emailDefaultInterval
		}
		if account.SyncLimit <= 0 {
			account.SyncLimit = emailDefaultSyncLimit
		}
	}
	return result, nil
}

func (a *App) saveInternalSettings(ctx context.Context, value internalSettings) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return a.writeSetting(ctx, settingAppSettings, string(data))
}

func (a *App) currentAIUsage(ctx context.Context) (aiUsage, error) {
	var usage aiUsage
	raw, err := a.readSetting(ctx, settingAIUsage)
	if errors.Is(err, sql.ErrNoRows) {
		return aiUsage{Date: a.today(), Month: a.thisMonth()}, nil
	}
	if err != nil {
		return usage, err
	}
	if err := json.Unmarshal([]byte(raw), &usage); err != nil {
		return usage, fmt.Errorf("decode AI usage: %w", err)
	}
	changed := false
	if usage.Date != a.today() {
		usage.Date = a.today()
		usage.Calls = 0
		changed = true
	}
	if usage.Month != a.thisMonth() {
		usage.Month = a.thisMonth()
		usage.TokensMonth = 0
		changed = true
	}
	if changed {
		if err := a.saveAIUsage(ctx, usage); err != nil {
			return usage, err
		}
	}
	return usage, nil
}

func (a *App) saveAIUsage(ctx context.Context, usage aiUsage) error {
	data, err := json.Marshal(usage)
	if err != nil {
		return err
	}
	return a.writeSetting(ctx, settingAIUsage, string(data))
}

func (a *App) recordAIUsage(ctx context.Context, tokens int64) error {
	usage, err := a.currentAIUsage(ctx)
	if err != nil {
		return err
	}
	usage.Calls++
	if tokens > 0 {
		usage.TokensMonth += tokens
	}
	return a.saveAIUsage(ctx, usage)
}

func (a *App) apiKeyEncrypted(ctx context.Context) (string, error) {
	value, err := a.readSetting(ctx, settingAIKey)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return value, err
}

func (a *App) apiKey(ctx context.Context) (string, error) {
	encrypted, err := a.apiKeyEncrypted(ctx)
	if err != nil || encrypted == "" {
		return "", err
	}
	return a.secret.Decrypt(encrypted)
}

func (a *App) globalFeedToken(ctx context.Context) (string, error) {
	return a.readSetting(ctx, settingGlobalToken)
}

func (a *App) rotateGlobalFeedToken(ctx context.Context) (string, error) {
	token, err := newFeedToken()
	if err != nil {
		return "", err
	}
	if err := a.writeSetting(ctx, settingGlobalToken, token); err != nil {
		return "", err
	}
	return token, nil
}

func (a *App) apiSettings(ctx context.Context, sessionExpiresAt *string) (Settings, error) {
	internal, err := a.loadInternalSettings(ctx)
	if err != nil {
		return Settings{}, err
	}
	usage, err := a.currentAIUsage(ctx)
	if err != nil {
		return Settings{}, err
	}
	encrypted, err := a.apiKeyEncrypted(ctx)
	if err != nil {
		return Settings{}, err
	}
	globalToken, err := a.globalFeedToken(ctx)
	if err != nil {
		return Settings{}, err
	}
	var lastLogin *string
	var account *string
	if value, err := a.readSetting(ctx, settingAccount); err == nil && strings.TrimSpace(value) != "" {
		name := value
		account = &name
	}
	if value, err := a.readSetting(ctx, settingLastLoginAt); err == nil && strings.TrimSpace(value) != "" {
		lastLogin = ptrString(value)
	} else if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return Settings{}, err
	}
	apiKeySet := false
	if encrypted != "" {
		apiKeySet = true
	}
	wechat := internal.Wechat
	wechat.WereadConnected = a.wereadCookieConfigured(ctx)
	wechat.WereadCanRenew = wechat.WereadConnected && a.wereadCookieHasRefreshToken(ctx)
	emailAccounts := make([]EmailAccount, 0, len(internal.Email.Accounts))
	for _, account := range internal.Email.Accounts {
		password, err := a.emailAccountPassword(ctx, account.ID)
		if err != nil {
			return Settings{}, err
		}
		emailAccounts = append(emailAccounts, EmailAccount{
			ID:                  account.ID,
			Host:                account.Host,
			Port:                account.Port,
			Username:            account.Username,
			PollIntervalMinutes: account.PollIntervalMinutes,
			SyncLimit:           account.SyncLimit,
			FolderID:            account.FolderID,
			PasswordSet:         password != "",
		})
	}
	blockRemoteImages := true
	if internal.Email.BlockRemoteImages != nil {
		blockRemoteImages = *internal.Email.BlockRemoteImages
	}
	email := EmailSettings{
		Enabled:           internal.Email.Enabled,
		BlockRemoteImages: blockRemoteImages,
		TrustedSenders:    internal.Email.TrustedSenders,
		Accounts:          emailAccounts,
	}
	return Settings{
		Reader: internal.Reader,
		AI: AISettings{
			Enabled:        internal.AI.Enabled,
			BaseURL:        internal.AI.BaseURL,
			Model:          internal.AI.Model,
			APIKeySet:      apiKeySet,
			SummaryStyle:   internal.AI.SummaryStyle,
			TargetLang:     internal.AI.TargetLang,
			DailyLimit:     internal.AI.DailyLimit,
			UsedToday:      usage.Calls,
			TokensMonth:    usage.TokensMonth,
			Consent:        internal.AI.Consent,
			Prompts:        internal.AI.Prompts,
			PromptDefaults: defaultAIPromptSettings(),
		},
		Wechat: wechat,
		Email:  email,
		Backup: internal.Backup,
		Feed:   FeedSettings{TokenMasked: maskToken(globalToken)},
		Security: SecuritySettings{
			LastLoginAt:      lastLogin,
			SessionExpiresAt: sessionExpiresAt,
			Account:          account,
		},
		MCP: MCPSettings{Enabled: internal.MCP.Enabled, Endpoint: "/mcp"},
	}, nil
}

type readerPatch struct {
	FontSize         *int         `json:"font_size"`
	LineWidth        *int         `json:"line_width"`
	LineHeight       *float64     `json:"line_height"`
	Theme            *string      `json:"theme"`
	Accent           *string      `json:"accent"`
	AutoRead         *bool        `json:"auto_read"`
	RememberPosition *bool        `json:"remember_position"`
	RailWidth        *int         `json:"rail_width"`
	ListWidth        *int         `json:"list_width"`
	Views            *[]SavedView `json:"views"`
	PinnedSources    *[]int64     `json:"pinned_sources"`
}

type aiPromptPatch struct {
	SummaryBullets *string `json:"summary_bullets"`
	SummaryOne     *string `json:"summary_one"`
	DigestBatch    *string `json:"digest_batch"`
	DigestMerge    *string `json:"digest_merge"`
}

type aiPatch struct {
	Enabled      *bool           `json:"enabled"`
	BaseURL      *string         `json:"base_url"`
	Model        *string         `json:"model"`
	APIKey       json.RawMessage `json:"api_key"`
	SummaryStyle *string         `json:"summary_style"`
	TargetLang   *string         `json:"target_lang"`
	DailyLimit   *int            `json:"daily_limit"`
	Consent      *bool           `json:"consent"`
	Prompts      *aiPromptPatch  `json:"prompts"`
}

type wechatPatch struct {
	Enabled *bool `json:"enabled"`
}

type emailAccountPatch struct {
	ID                  string  `json:"id"`
	Host                *string `json:"host"`
	Port                *int    `json:"port"`
	Username            *string `json:"username"`
	PollIntervalMinutes *int    `json:"poll_interval_minutes"`
	SyncLimit           *int    `json:"sync_limit"`
	FolderID            *int64  `json:"folder_id"`
}

type emailPatch struct {
	Enabled           *bool                `json:"enabled"`
	BlockRemoteImages *bool                `json:"block_remote_images"`
	TrustedSenders    *[]string            `json:"trusted_senders"`
	Accounts          *[]emailAccountPatch `json:"accounts"`
	// 单独保存/清除某个账号的授权码，避免把授权码混进账号数组。
	AccountID *string         `json:"account_id"`
	Password  json.RawMessage `json:"password"`
}

type backupPatch struct {
	Daily *bool `json:"daily"`
	Keep  *int  `json:"keep"`
}

type settingsPatch struct {
	Reader *readerPatch `json:"reader"`
	AI     *aiPatch     `json:"ai"`
	Wechat *wechatPatch `json:"wechat"`
	Email  *emailPatch  `json:"email"`
	Backup *backupPatch `json:"backup"`
	MCP    *mcpPatch    `json:"mcp"`
}

type mcpPatch struct {
	Enabled *bool `json:"enabled"`
}

func (a *App) updateSettings(ctx context.Context, body []byte, sessionExpiresAt *string) (Settings, error) {
	var patch settingsPatch
	if err := json.Unmarshal(body, &patch); err != nil {
		return Settings{}, badRequest("invalid_json", "设置内容不是合法 JSON")
	}
	internal, err := a.loadInternalSettings(ctx)
	if err != nil {
		return Settings{}, internalError(err)
	}
	if patch.Reader != nil {
		if patch.Reader.FontSize != nil {
			if *patch.Reader.FontSize < 12 || *patch.Reader.FontSize > 32 {
				return Settings{}, badRequest("invalid_settings", "字号需在 12–32 之间")
			}
			internal.Reader.FontSize = *patch.Reader.FontSize
		}
		if patch.Reader.LineWidth != nil {
			if *patch.Reader.LineWidth < 400 || *patch.Reader.LineWidth > 1200 {
				return Settings{}, badRequest("invalid_settings", "行宽需在 400–1200 之间")
			}
			internal.Reader.LineWidth = *patch.Reader.LineWidth
		}
		if patch.Reader.LineHeight != nil {
			if *patch.Reader.LineHeight < 1.2 || *patch.Reader.LineHeight > 2.5 {
				return Settings{}, badRequest("invalid_settings", "行高需在 1.2–2.5 之间")
			}
			internal.Reader.LineHeight = *patch.Reader.LineHeight
		}
		if patch.Reader.Theme != nil {
			if *patch.Reader.Theme != "light" && *patch.Reader.Theme != "dark" && *patch.Reader.Theme != "system" {
				return Settings{}, badRequest("invalid_settings", "主题只能是 light、dark 或 system")
			}
			internal.Reader.Theme = *patch.Reader.Theme
		}
		if patch.Reader.Accent != nil {
			accent := *patch.Reader.Accent
			switch accent {
			case "green", "blue", "cyan", "teal", "indigo", "purple", "pink", "rose", "red", "orange", "amber", "slate":
				internal.Reader.Accent = accent
			default:
				return Settings{}, badRequest("invalid_settings", "不支持的主题色")
			}
		}
		if patch.Reader.AutoRead != nil {
			internal.Reader.AutoRead = *patch.Reader.AutoRead
		}
		if patch.Reader.RememberPosition != nil {
			internal.Reader.RememberPosition = *patch.Reader.RememberPosition
		}
		if patch.Reader.RailWidth != nil {
			if *patch.Reader.RailWidth < 180 || *patch.Reader.RailWidth > 320 {
				return Settings{}, badRequest("invalid_settings", "左侧栏宽度需在 180–320 之间")
			}
			internal.Reader.RailWidth = *patch.Reader.RailWidth
		}
		if patch.Reader.ListWidth != nil {
			if *patch.Reader.ListWidth < 300 || *patch.Reader.ListWidth > 520 {
				return Settings{}, badRequest("invalid_settings", "文章列表宽度需在 300–520 之间")
			}
			internal.Reader.ListWidth = *patch.Reader.ListWidth
		}
		if patch.Reader.Views != nil {
			views := *patch.Reader.Views
			if len(views) > 30 {
				return Settings{}, badRequest("invalid_settings", "保存视图最多 30 个")
			}
			seen := make(map[string]struct{}, len(views))
			clean := make([]SavedView, 0, len(views))
			for _, view := range views {
				view.ID = strings.TrimSpace(view.ID)
				view.Name = strings.TrimSpace(view.Name)
				view.Query = strings.TrimSpace(view.Query)
				if view.ID == "" || len([]rune(view.ID)) > 80 || view.Name == "" || len([]rune(view.Name)) > 60 {
					return Settings{}, badRequest("invalid_settings", "保存视图缺少 ID 或名称")
				}
				if _, exists := seen[view.ID]; exists {
					return Settings{}, badRequest("invalid_settings", "保存视图 ID 重复")
				}
				seen[view.ID] = struct{}{}
				switch view.Type {
				case "inbox", "unread", "read", "starred", "later", "source", "folder":
				default:
					return Settings{}, badRequest("invalid_settings", "保存视图类型无效")
				}
				if len([]rune(view.Query)) > 300 {
					return Settings{}, badRequest("invalid_settings", "保存视图搜索词过长")
				}
				if view.ReadFilter == "" {
					view.ReadFilter = "all"
				}
				if view.ReadFilter != "all" && view.ReadFilter != "unread" && view.ReadFilter != "read" {
					return Settings{}, badRequest("invalid_settings", "保存视图已读筛选无效")
				}
				if view.Type == "source" && (view.SourceID == nil || *view.SourceID <= 0) {
					return Settings{}, badRequest("invalid_settings", "保存视图缺少源 ID")
				}
				if view.Type == "folder" && (view.FolderID == nil || *view.FolderID <= 0) {
					return Settings{}, badRequest("invalid_settings", "保存视图缺少分组 ID")
				}
				clean = append(clean, view)
			}
			internal.Reader.Views = clean
		}
		if patch.Reader.PinnedSources != nil {
			pinned := *patch.Reader.PinnedSources
			if len(pinned) > 200 {
				return Settings{}, badRequest("invalid_settings", "置顶订阅源最多 200 个")
			}
			seen := make(map[int64]struct{}, len(pinned))
			clean := make([]int64, 0, len(pinned))
			for _, id := range pinned {
				if id <= 0 {
					return Settings{}, badRequest("invalid_settings", "置顶订阅源 ID 无效")
				}
				if _, exists := seen[id]; exists {
					continue
				}
				seen[id] = struct{}{}
				clean = append(clean, id)
			}
			internal.Reader.PinnedSources = clean
		}
	}
	var encryptedKey string
	if patch.AI != nil {
		if patch.AI.BaseURL != nil {
			value := strings.TrimSpace(*patch.AI.BaseURL)
			if value != "" {
				parsed, err := url.Parse(value)
				if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
					return Settings{}, badRequest("invalid_settings", "AI 接口地址必须是 http/https URL")
				}
			}
			internal.AI.BaseURL = value
		}
		if patch.AI.Model != nil {
			model := strings.TrimSpace(*patch.AI.Model)
			if len(model) > 200 {
				return Settings{}, badRequest("invalid_settings", "模型名过长")
			}
			internal.AI.Model = model
		}
		if len(patch.AI.APIKey) > 0 {
			if string(patch.AI.APIKey) == "null" {
				if err := a.deleteSetting(ctx, settingAIKey); err != nil {
					return Settings{}, internalError(err)
				}
			} else {
				var key string
				if err := json.Unmarshal(patch.AI.APIKey, &key); err != nil {
					return Settings{}, badRequest("invalid_settings", "API Key 必须是字符串")
				}
				key = strings.TrimSpace(key)
				if key != "" {
					if len(key) > 4096 {
						return Settings{}, badRequest("invalid_settings", "API Key 过长")
					}
					encrypted, err := a.secret.Encrypt(key)
					if err != nil {
						return Settings{}, internalError(err)
					}
					encryptedKey = encrypted
				}
			}
		}
		if patch.AI.Enabled != nil {
			internal.AI.Enabled = *patch.AI.Enabled
		}
		if patch.AI.SummaryStyle != nil {
			if *patch.AI.SummaryStyle != "bullets" && *patch.AI.SummaryStyle != "one" {
				return Settings{}, badRequest("invalid_settings", "摘要样式只能是 bullets 或 one")
			}
			internal.AI.SummaryStyle = *patch.AI.SummaryStyle
		}
		if patch.AI.TargetLang != nil {
			lang := strings.TrimSpace(*patch.AI.TargetLang)
			if lang == "" || len(lang) > 32 {
				return Settings{}, badRequest("invalid_settings", "目标语言不能为空")
			}
			internal.AI.TargetLang = lang
		}
		if patch.AI.Prompts != nil {
			fields := []struct {
				value  *string
				target *string
				name   string
			}{
				{patch.AI.Prompts.SummaryBullets, &internal.AI.Prompts.SummaryBullets, "要点列表提示词"},
				{patch.AI.Prompts.SummaryOne, &internal.AI.Prompts.SummaryOne, "一句话提示词"},
				{patch.AI.Prompts.DigestBatch, &internal.AI.Prompts.DigestBatch, "报告分批提示词"},
				{patch.AI.Prompts.DigestMerge, &internal.AI.Prompts.DigestMerge, "报告简报提示词"},
			}
			for _, field := range fields {
				if field.value == nil {
					continue
				}
				normalized, apiErr := normalizeAIPrompt(*field.value)
				if apiErr != nil {
					return Settings{}, badRequest("invalid_settings", field.name+"不能超过 4000 个字符")
				}
				*field.target = normalized
			}
		}
		if patch.AI.DailyLimit != nil {
			if *patch.AI.DailyLimit < 1 || *patch.AI.DailyLimit > 10000 {
				return Settings{}, badRequest("invalid_settings", "每日 AI 调用上限需在 1–10000 之间")
			}
			internal.AI.DailyLimit = *patch.AI.DailyLimit
		}
		if patch.AI.Consent != nil {
			internal.AI.Consent = *patch.AI.Consent
		}
	}
	if patch.Wechat != nil {
		if patch.Wechat.Enabled != nil {
			internal.Wechat.Enabled = *patch.Wechat.Enabled
		}
	}
	emailPasswordWrites := make(map[string]string)
	var emailPasswordDeletes []string
	if patch.Email != nil {
		emailPatch := patch.Email
		if emailPatch.Enabled != nil {
			internal.Email.Enabled = *emailPatch.Enabled
		}
		if emailPatch.BlockRemoteImages != nil {
			value := *emailPatch.BlockRemoteImages
			internal.Email.BlockRemoteImages = &value
		}
		if emailPatch.TrustedSenders != nil {
			senders := normalizeTrustedEmailSenders(*emailPatch.TrustedSenders)
			if len(senders) > 200 {
				return Settings{}, badRequest("invalid_settings", "放行发件人最多 200 个")
			}
			internal.Email.TrustedSenders = senders
		}
		if emailPatch.Accounts != nil {
			accounts := *emailPatch.Accounts
			if len(accounts) > 10 {
				return Settings{}, badRequest("invalid_settings", "邮箱账号最多 10 个")
			}
			existing := make(map[string]internalEmailAccount, len(internal.Email.Accounts))
			for _, account := range internal.Email.Accounts {
				existing[account.ID] = account
			}
			seenIDs := make(map[string]struct{}, len(accounts))
			seenUsers := make(map[string]struct{}, len(accounts))
			clean := make([]internalEmailAccount, 0, len(accounts))
			for _, input := range accounts {
				id := strings.TrimSpace(input.ID)
				if id == "" {
					generated, err := newEmailAccountID()
					if err != nil {
						return Settings{}, internalError(err)
					}
					id = generated
				}
				if !emailAccountIDPattern.MatchString(id) {
					return Settings{}, badRequest("invalid_settings", "邮箱账号 ID 无效")
				}
				if _, exists := seenIDs[id]; exists {
					return Settings{}, badRequest("invalid_settings", "邮箱账号 ID 重复")
				}
				seenIDs[id] = struct{}{}
				account, exists := existing[id]
				if !exists {
					account = internalEmailAccount{ID: id, Port: emailDefaultPort, PollIntervalMinutes: emailDefaultInterval, SyncLimit: emailDefaultSyncLimit}
				}
				account.ID = id
				if input.Host != nil {
					account.Host = strings.TrimSpace(*input.Host)
				}
				if input.Port != nil {
					if *input.Port < 1 || *input.Port > 65535 {
						return Settings{}, badRequest("invalid_settings", "端口需在 1–65535 之间")
					}
					account.Port = *input.Port
				}
				if input.Username != nil {
					account.Username = strings.TrimSpace(*input.Username)
				}
				if input.PollIntervalMinutes != nil {
					if *input.PollIntervalMinutes < emailMinInterval || *input.PollIntervalMinutes > emailMaxInterval {
						return Settings{}, badRequest("invalid_settings", fmt.Sprintf("轮询间隔需在 %d–%d 分钟之间", emailMinInterval, emailMaxInterval))
					}
					account.PollIntervalMinutes = *input.PollIntervalMinutes
				}
				if input.SyncLimit != nil {
					if *input.SyncLimit < emailMinSyncLimit || *input.SyncLimit > emailMaxSyncLimit {
						return Settings{}, badRequest("invalid_settings", fmt.Sprintf("同步数量需在 %d–%d 封之间", emailMinSyncLimit, emailMaxSyncLimit))
					}
					account.SyncLimit = *input.SyncLimit
				}
				// accounts 是整体替换：folder_id 缺省/null 都表示不分组。
				account.FolderID = nil
				if input.FolderID != nil && *input.FolderID > 0 {
					if err := a.ensureFolderExists(ctx, *input.FolderID); err != nil {
						return Settings{}, err
					}
					folderID := *input.FolderID
					account.FolderID = &folderID
				} else if input.FolderID != nil && *input.FolderID < 0 {
					return Settings{}, badRequest("invalid_settings", "分组 ID 无效")
				}
				if account.Username != "" {
					key := strings.ToLower(account.Username)
					if _, duplicate := seenUsers[key]; duplicate {
						return Settings{}, badRequest("invalid_settings", "邮箱账号重复")
					}
					seenUsers[key] = struct{}{}
				}
				if account.Port <= 0 {
					account.Port = emailDefaultPort
				}
				if account.PollIntervalMinutes < emailMinInterval || account.PollIntervalMinutes > emailMaxInterval {
					account.PollIntervalMinutes = emailDefaultInterval
				}
				if account.SyncLimit == 0 {
					account.SyncLimit = emailDefaultSyncLimit
				}
				clean = append(clean, account)
			}
			for id := range existing {
				if _, kept := seenIDs[id]; !kept {
					emailPasswordDeletes = append(emailPasswordDeletes, id)
				}
			}
			internal.Email.Accounts = clean
		}
		if len(emailPatch.Password) > 0 {
			target := ""
			if emailPatch.AccountID != nil {
				target = strings.TrimSpace(*emailPatch.AccountID)
			}
			if target == "" && len(internal.Email.Accounts) == 1 {
				target = internal.Email.Accounts[0].ID
			}
			if target == "" {
				return Settings{}, badRequest("invalid_settings", "请指定要保存授权码的邮箱账号")
			}
			if !emailAccountIDPattern.MatchString(target) {
				return Settings{}, badRequest("invalid_settings", "邮箱账号 ID 无效")
			}
			found := false
			for _, account := range internal.Email.Accounts {
				if account.ID == target {
					found = true
					break
				}
			}
			if !found {
				return Settings{}, badRequest("invalid_settings", "邮箱账号不存在")
			}
			if string(emailPatch.Password) == "null" {
				emailPasswordDeletes = append(emailPasswordDeletes, target)
			} else {
				var password string
				if err := json.Unmarshal(emailPatch.Password, &password); err != nil {
					return Settings{}, badRequest("invalid_settings", "授权码必须是字符串")
				}
				password = strings.TrimSpace(password)
				if password != "" {
					if len(password) > 1024 {
						return Settings{}, badRequest("invalid_settings", "授权码过长")
					}
					encrypted, err := a.secret.Encrypt(password)
					if err != nil {
						return Settings{}, internalError(err)
					}
					emailPasswordWrites[target] = encrypted
				}
			}
		}
	}
	if patch.Backup != nil {
		if patch.Backup.Daily != nil {
			internal.Backup.Daily = *patch.Backup.Daily
		}
		if patch.Backup.Keep != nil {
			if *patch.Backup.Keep < 1 || *patch.Backup.Keep > 100 {
				return Settings{}, badRequest("invalid_settings", "备份保留份数需在 1–100 之间")
			}
			internal.Backup.Keep = *patch.Backup.Keep
		}
	}
	if patch.MCP != nil && patch.MCP.Enabled != nil {
		internal.MCP.Enabled = *patch.MCP.Enabled
	}
	if err := a.saveInternalSettings(ctx, internal); err != nil {
		return Settings{}, internalError(err)
	}
	if encryptedKey != "" {
		if err := a.writeSetting(ctx, settingAIKey, encryptedKey); err != nil {
			return Settings{}, internalError(err)
		}
	}
	for accountID, encrypted := range emailPasswordWrites {
		if err := a.writeSetting(ctx, emailPasswordSettingKey(accountID), encrypted); err != nil {
			return Settings{}, internalError(err)
		}
	}
	for _, accountID := range emailPasswordDeletes {
		if err := a.deleteSetting(ctx, emailPasswordSettingKey(accountID)); err != nil {
			return Settings{}, internalError(err)
		}
		if accountID == legacyEmailAccountID {
			_ = a.deleteSetting(ctx, settingEmailPassword)
		}
	}
	if patch.Email != nil {
		// 邮箱配置变化后同步源的轮询间隔 / 新建收集箱 / 重排下次抓取时间。
		if err := a.ensureEmailSources(ctx); err != nil {
			return Settings{}, internalError(err)
		}
	}
	return a.apiSettings(ctx, sessionExpiresAt)
}

func (a *App) today() string {
	return a.now().UTC().Format("2006-01-02")
}

func (a *App) thisMonth() string {
	return a.now().UTC().Format("2006-01")
}
