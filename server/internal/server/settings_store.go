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
	return Settings{
		Reader: internal.Reader,
		AI: AISettings{
			Enabled:      internal.AI.Enabled,
			BaseURL:      internal.AI.BaseURL,
			Model:        internal.AI.Model,
			APIKeySet:    apiKeySet,
			SummaryStyle: internal.AI.SummaryStyle,
			TargetLang:   internal.AI.TargetLang,
			DailyLimit:   internal.AI.DailyLimit,
			UsedToday:    usage.Calls,
			TokensMonth:  usage.TokensMonth,
			Consent:      internal.AI.Consent,
		},
		Wechat: wechat,
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
}

type wechatPatch struct {
	Enabled *bool `json:"enabled"`
}

type backupPatch struct {
	Daily *bool `json:"daily"`
	Keep  *int  `json:"keep"`
}

type settingsPatch struct {
	Reader *readerPatch `json:"reader"`
	AI     *aiPatch     `json:"ai"`
	Wechat *wechatPatch `json:"wechat"`
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
	return a.apiSettings(ctx, sessionExpiresAt)
}

func (a *App) today() string {
	return a.now().UTC().Format("2006-01-02")
}

func (a *App) thisMonth() string {
	return a.now().UTC().Format("2006-01")
}
