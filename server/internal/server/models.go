package server

import (
	"encoding/json"
	"fmt"
	"time"
)

type Folder struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Sort        int    `json:"sort"`
	ParentID    *int64 `json:"parent_id"`
	SourceCount int    `json:"source_count"`
}

type sourceSummary struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind"`
}

type Source struct {
	ID                int64   `json:"id"`
	Kind              string  `json:"kind"`
	Name              string  `json:"name"`
	FeedURL           string  `json:"feed_url"`
	SiteURL           *string `json:"site_url"`
	FolderID          *int64  `json:"folder_id"`
	Enabled           bool    `json:"enabled"`
	RefreshSeconds    int     `json:"refresh_seconds"`
	InitialLimit      int     `json:"initial_limit"`
	FallbackURL       *string `json:"fallback_url"`
	LastFetchAt       *string `json:"last_fetch_at"`
	LastNewItemAt     *string `json:"last_new_item_at"`
	NextFetchAt       string  `json:"next_fetch_at"`
	LastStatus        *string `json:"last_status"`
	LastError         *string `json:"last_error"`
	FailCount         int     `json:"fail_count"`
	ItemCount         int     `json:"item_count"`
	UnreadCount       int     `json:"unread_count"`
	RetentionDays     int     `json:"retention_days"`
	RetentionMaxItems int     `json:"retention_max_items"`
	IconURL           *string `json:"icon_url"`
	Note              *string `json:"note,omitempty"`
	BackfillMode      string  `json:"backfill_mode"`
	FullText          bool    `json:"full_text"`
	WatchEnabled      bool    `json:"watch_enabled"`
	WatchMode         string  `json:"watch_mode"`
	WatchAI           bool    `json:"watch_ai"`
	WatchChannelIDs   []int64 `json:"watch_channel_ids"`
}

type FetchLog struct {
	ID         int64   `json:"id"`
	StartedAt  string  `json:"started_at"`
	DurationMS *int64  `json:"duration_ms"`
	HTTPStatus *int    `json:"http_status"`
	Result     string  `json:"result"`
	NewCount   int     `json:"new_count"`
	Error      *string `json:"error"`
}

type FeedPaths struct {
	RSS  string `json:"rss"`
	Atom string `json:"atom"`
	JSON string `json:"json"`
}

type SourceHealth struct {
	Source          Source     `json:"source"`
	Logs            []FetchLog `json:"logs"`
	Trend           []int      `json:"trend"`
	FeedTokenMasked string     `json:"feed_token_masked"`
	FeedPaths       FeedPaths  `json:"feed_paths"`
}

type ItemSummary struct {
	ID                 int64         `json:"id"`
	SourceID           int64         `json:"source_id"`
	Source             sourceSummary `json:"source"`
	Title              string        `json:"title"`
	URL                *string       `json:"url"`
	Author             *string       `json:"author"`
	PublishedAt        *string       `json:"published_at"`
	PublishedEstimated bool          `json:"published_estimated"`
	Excerpt            string        `json:"excerpt"`
	CoverURL           *string       `json:"cover_url"`
	ReadAt             *string       `json:"read_at"`
	StarredAt          *string       `json:"starred_at"`
	LaterAt            *string       `json:"later_at"`
	FetchedAt          string        `json:"fetched_at"`
}

type ItemNote struct {
	ID        int64  `json:"id"`
	ItemID    int64  `json:"item_id"`
	Quote     string `json:"quote"`
	Note      string `json:"note"`
	Color     string `json:"color"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type AISummary struct {
	Model       string   `json:"model"`
	GeneratedAt string   `json:"generated_at"`
	Style       string   `json:"style"`
	Points      []string `json:"points"`
}

type Translation struct {
	Title            string `json:"title"`
	TargetLang       string `json:"target_lang"`
	OriginalLang     string `json:"original_lang"`
	Model            string `json:"model"`
	GeneratedAt      string `json:"generated_at"`
	NeedsRetranslate bool   `json:"needs_retranslate"`
	ContentHTML      string `json:"content_html"`
}

type ItemDetail struct {
	ItemSummary
	OriginalTitle          *string      `json:"original_title,omitempty"`
	SourceFeedURL          string       `json:"source_feed_url,omitempty"`
	SenderEmail            string       `json:"sender_email,omitempty"`
	EmailImagesAllowed     bool         `json:"email_images_allowed,omitempty"`
	ContentHTML            string       `json:"content_html"`
	ContentText            string       `json:"content_text"`
	AudioURL               string       `json:"audio_url,omitempty"`
	AudioType              string       `json:"audio_type,omitempty"`
	PodcastPositionSeconds float64      `json:"podcast_position_seconds,omitempty"`
	PodcastDurationSeconds float64      `json:"podcast_duration_seconds,omitempty"`
	Summary                *AISummary   `json:"summary,omitempty"`
	Translation            *Translation `json:"translation,omitempty"`
	Notes                  []ItemNote   `json:"notes,omitempty"`
}

type ItemPage struct {
	Items      []ItemSummary `json:"items"`
	NextCursor *string       `json:"next_cursor"`
}

type ReaderStats struct {
	Total   int `json:"total"`
	Unread  int `json:"unread"`
	Starred int `json:"starred"`
	Read    int `json:"read"`
	Hidden  int `json:"hidden"`
	Later   int `json:"later"`
}

type FeedCandidate struct {
	Title  string `json:"title"`
	URL    string `json:"url"`
	Format string `json:"format"`
}

type ResolveCandidate struct {
	Platform     string `json:"platform"`
	Kind         string `json:"kind"`
	Name         string `json:"name"`
	FeedURL      string `json:"feed_url"`
	SiteURL      string `json:"site_url,omitempty"`
	IconURL      string `json:"icon_url,omitempty"`
	IconProxyURL string `json:"icon_proxy_url,omitempty"`
	Note         string `json:"note,omitempty"`
	Partial      bool   `json:"partial,omitempty"`
}

type CreatorSearchCandidate struct {
	Platform         string `json:"platform"`
	Kind             string `json:"kind"`
	Name             string `json:"name"`
	Handle           string `json:"handle,omitempty"`
	FeedURL          string `json:"feed_url"`
	SiteURL          string `json:"site_url,omitempty"`
	IconURL          string `json:"icon_url,omitempty"`
	IconProxyURL     string `json:"icon_proxy_url,omitempty"`
	Note             string `json:"note,omitempty"`
	Match            string `json:"match,omitempty"`
	Source           string `json:"source,omitempty"`
	Partial          bool   `json:"partial,omitempty"`
	ExistingSourceID int64  `json:"existing_source_id,omitempty"`
}

type CreatorSearchPlatformStatus struct {
	Platform string `json:"platform"`
	Label    string `json:"label"`
	Status   string `json:"status"`
	Count    int    `json:"count"`
	Error    string `json:"error,omitempty"`
}

type CreatorSearchResponse struct {
	Query      string                        `json:"query"`
	Candidates []CreatorSearchCandidate      `json:"candidates"`
	Platforms  []CreatorSearchPlatformStatus `json:"platforms"`
}

type SourceBatchError struct {
	Index            int    `json:"index"`
	Name             string `json:"name,omitempty"`
	FeedURL          string `json:"feed_url,omitempty"`
	Code             string `json:"code"`
	Message          string `json:"message"`
	ExistingSourceID int64  `json:"existing_source_id,omitempty"`
}

type SourceBatchCreateResponse struct {
	Created []Source           `json:"created"`
	Errors  []SourceBatchError `json:"errors"`
}

type BatchStateResponse struct {
	Updated int     `json:"updated"`
	IDs     []int64 `json:"ids"`
}

type TokenResetResponse struct {
	TokenMasked string `json:"token_masked"`
}

type AITestResponse struct {
	OK        bool `json:"ok"`
	LatencyMS int  `json:"latency_ms"`
}

type ReaderSettings struct {
	FontSize         int         `json:"font_size"`
	LineWidth        int         `json:"line_width"`
	LineHeight       float64     `json:"line_height"`
	Theme            string      `json:"theme"`
	Accent           string      `json:"accent"`
	AutoRead         bool        `json:"auto_read"`
	RememberPosition bool        `json:"remember_position"`
	RailWidth        int         `json:"rail_width"`
	ListWidth        int         `json:"list_width"`
	Views            []SavedView `json:"views"`
	PinnedSources    []int64     `json:"pinned_sources"`
}

type SavedView struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	SourceID   *int64 `json:"source_id,omitempty"`
	FolderID   *int64 `json:"folder_id,omitempty"`
	Query      string `json:"query,omitempty"`
	ReadFilter string `json:"read_filter,omitempty"`
}

// AIPromptSettings 是用户可编辑的任务指令。空字符串表示使用系统默认；
// system 角色、任务声明、不可信内容边界仍由服务端固定（REQ-042）。
type AIPromptSettings struct {
	SummaryBullets string `json:"summary_bullets"`
	SummaryOne     string `json:"summary_one"`
	DigestBatch    string `json:"digest_batch"`
	DigestMerge    string `json:"digest_merge"`
}

type AISettings struct {
	Enabled        bool             `json:"enabled"`
	BaseURL        string           `json:"base_url"`
	Model          string           `json:"model"`
	APIKeySet      bool             `json:"api_key_set"`
	SummaryStyle   string           `json:"summary_style"`
	TargetLang     string           `json:"target_lang"`
	DailyLimit     int              `json:"daily_limit"`
	UsedToday      int              `json:"used_today"`
	TokensMonth    int64            `json:"tokens_month"`
	Consent        bool             `json:"consent"`
	Prompts        AIPromptSettings `json:"prompts"`
	PromptDefaults AIPromptSettings `json:"prompt_defaults"`
}

type WechatSettings struct {
	Enabled bool `json:"enabled"`

	WereadConnected bool `json:"weread_connected"`
	// WereadCanRenew 表示落库 Cookie 是否带 wr_rt 刷新令牌。旧版扫码的 Cookie
	// 没有它，运行期无法自动续期，只能重新扫码一次；UI 必须把这种情况说清楚。
	WereadCanRenew bool `json:"weread_can_renew"`
}

// EmailAccount 是单个 IMAP 邮箱的对外视图；授权码永不回传，只给 PasswordSet。
type EmailAccount struct {
	ID                  string `json:"id"`
	Host                string `json:"host"`
	Port                int    `json:"port"`
	Username            string `json:"username"`
	PollIntervalMinutes int    `json:"poll_interval_minutes"`
	SyncLimit           int    `json:"sync_limit"`
	FolderID            *int64 `json:"folder_id"`
	PasswordSet         bool   `json:"password_set"`
}

// EmailSettings 是「邮箱同步」的对外视图。多个邮箱各自对应一个订阅源；
// 远程图片默认拦截，TrustedSenders 中的发件人可长期放行。
type EmailSettings struct {
	Enabled           bool           `json:"enabled"`
	BlockRemoteImages bool           `json:"block_remote_images"`
	TrustedSenders    []string       `json:"trusted_senders"`
	Accounts          []EmailAccount `json:"accounts"`
}

type BackupSettings struct {
	Daily        bool    `json:"daily"`
	Keep         int     `json:"keep"`
	LastBackupAt *string `json:"last_backup_at"`
}

type FeedSettings struct {
	TokenMasked string `json:"token_masked"`
}

type SecuritySettings struct {
	LastLoginAt      *string `json:"last_login_at"`
	SessionExpiresAt *string `json:"session_expires_at"`
	Account          *string `json:"account,omitempty"`
}

// REQ-038（DEC-029）：MCP 通过 Bearer Token 暴露给外部 Agent。
// 设置只返回开关与固定端点；令牌列表走独立接口，完整令牌只在创建时返回一次。
type MCPSettings struct {
	Enabled  bool   `json:"enabled"`
	Endpoint string `json:"endpoint"`
}

type MCPToken struct {
	ID         int64   `json:"id"`
	Name       string  `json:"name"`
	Masked     string  `json:"masked"`
	Enabled    bool    `json:"enabled"`
	CreatedAt  string  `json:"created_at"`
	LastUsedAt *string `json:"last_used_at"`
}

type MCPTokenCreated struct {
	MCPToken
	Token string `json:"token"`
}

type Settings struct {
	Reader   ReaderSettings   `json:"reader"`
	AI       AISettings       `json:"ai"`
	Wechat   WechatSettings   `json:"wechat"`
	Email    EmailSettings    `json:"email"`
	Backup   BackupSettings   `json:"backup"`
	Feed     FeedSettings     `json:"feed"`
	Security SecuritySettings `json:"security"`
	MCP      MCPSettings      `json:"mcp"`
}

// internalAISettings is persisted as JSON; the API key is stored separately
// so it can never be returned by a generic settings marshal.
type internalAISettings struct {
	Enabled      bool             `json:"enabled"`
	BaseURL      string           `json:"base_url"`
	Model        string           `json:"model"`
	SummaryStyle string           `json:"summary_style"`
	TargetLang   string           `json:"target_lang"`
	DailyLimit   int              `json:"daily_limit"`
	Consent      bool             `json:"consent"`
	Prompts      AIPromptSettings `json:"prompts"`
}

type internalSettings struct {
	Reader ReaderSettings        `json:"reader"`
	AI     internalAISettings    `json:"ai"`
	Wechat WechatSettings        `json:"wechat"`
	Email  internalEmailSettings `json:"email"`
	Backup BackupSettings        `json:"backup"`
	MCP    internalMCPSettings   `json:"mcp"`
}

// internalEmailSettings 持久化为 app_settings JSON 的 email 节；授权码按账号
// 分别加密存 `email_password_enc:{id}`，与 ai_api_key_enc 模式一致，避免随
// 通用设置序列化泄漏。旧版单账号字段保留只用于读取迁移。
type internalEmailAccount struct {
	ID                  string `json:"id"`
	Host                string `json:"host"`
	Port                int    `json:"port"`
	Username            string `json:"username"`
	PollIntervalMinutes int    `json:"poll_interval_minutes"`
	SyncLimit           int    `json:"sync_limit"`
	FolderID            *int64 `json:"folder_id"`
}

type internalEmailSettings struct {
	Enabled           bool                   `json:"enabled"`
	BlockRemoteImages *bool                  `json:"block_remote_images,omitempty"`
	TrustedSenders    []string               `json:"trusted_senders,omitempty"`
	Accounts          []internalEmailAccount `json:"accounts,omitempty"`

	// Legacy single-account fields（REQ-041 初版），读取后迁移到 Accounts。
	Host                string `json:"host,omitempty"`
	Port                int    `json:"port,omitempty"`
	Username            string `json:"username,omitempty"`
	PollIntervalMinutes int    `json:"poll_interval_minutes,omitempty"`
	FolderID            *int64 `json:"folder_id,omitempty"`
}

type internalMCPSettings struct {
	Enabled bool `json:"enabled"`
}

type aiUsage struct {
	Date        string `json:"date"`
	Calls       int    `json:"calls"`
	TokensMonth int64  `json:"tokens_month"`
	Month       string `json:"month"`
}

func defaultInternalSettings() internalSettings {
	return internalSettings{
		Reader: ReaderSettings{
			FontSize:         18,
			LineWidth:        660,
			LineHeight:       1.85,
			Theme:            "system",
			Accent:           "blue",
			AutoRead:         true,
			RememberPosition: true,
			RailWidth:        224,
			ListWidth:        384,
			Views:            []SavedView{},
			PinnedSources:    []int64{},
		},
		AI: internalAISettings{
			Enabled:      false,
			BaseURL:      "https://api.openai.com/v1",
			Model:        "gpt-4o-mini",
			SummaryStyle: "bullets",
			TargetLang:   "zh-CN",
			DailyLimit:   200,
			Consent:      false,
		},
		Wechat: WechatSettings{Enabled: false},
		Email: internalEmailSettings{
			Enabled: false,
		},
		Backup: BackupSettings{Daily: true, Keep: 7},
	}
}

func (s Settings) JSON() ([]byte, error) { return json.Marshal(s) }

func ptrString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func formatTimePtr(t time.Time) *string {
	if t.IsZero() {
		return nil
	}
	value := t.UTC().Format(time.RFC3339)
	return &value
}

func mustJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		panic(fmt.Sprintf("marshal constant JSON: %v", err))
	}
	return string(data)
}
