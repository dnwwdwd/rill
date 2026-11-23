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

const sourceSelectColumns = `
	s.id, s.kind, s.name, s.feed_url, s.site_url, s.folder_id, s.enabled,
	s.refresh_seconds, s.initial_limit, s.fallback_url, s.last_fetch_at,
	s.last_new_item_at, s.next_fetch_at, s.last_status, s.last_error, s.fail_count,
	(SELECT COUNT(*) FROM items i WHERE i.source_id = s.id),
	(SELECT COUNT(*) FROM items i WHERE i.source_id = s.id AND i.read_at IS NULL AND i.hidden = 0),
	s.retention_days, s.retention_max_items, s.icon_url, s.note, s.backfill_mode, s.config_json`

type rowScanner interface {
	Scan(dest ...any) error
}

type dbSource struct {
	Source
	IconSourceURL  string
	ETag           string
	LastModified   string
	InitialFetched bool
	FeedToken      string
	ConfigJSON     string
}

func scanSource(scanner rowScanner) (dbSource, error) {
	var result dbSource
	var siteURL, fallbackURL, lastFetch, lastNew, lastStatus, lastError, iconURL, note, backfillMode sql.NullString
	var folderID sql.NullInt64
	var enabled, failCount, itemCount, unreadCount int
	err := scanner.Scan(
		&result.ID, &result.Kind, &result.Name, &result.FeedURL, &siteURL, &folderID, &enabled,
		&result.RefreshSeconds, &result.InitialLimit, &fallbackURL, &lastFetch,
		&lastNew, &result.NextFetchAt, &lastStatus, &lastError, &failCount,
		&itemCount, &unreadCount, &result.RetentionDays, &result.RetentionMaxItems, &iconURL, &note, &backfillMode,
		&result.ConfigJSON,
	)
	if err != nil {
		return result, err
	}
	result.SiteURL = nullStringPtr(siteURL)
	if folderID.Valid {
		value := folderID.Int64
		result.FolderID = &value
	}
	result.Enabled = enabled != 0
	result.FallbackURL = nullStringPtr(fallbackURL)
	result.LastFetchAt = nullStringPtr(lastFetch)
	result.LastNewItemAt = nullStringPtr(lastNew)
	result.LastStatus = nullStringPtr(lastStatus)
	result.LastError = nullStringPtr(lastError)
	result.FailCount = failCount
	result.ItemCount = itemCount
	result.UnreadCount = unreadCount
	result.IconSourceURL = iconURL.String
	iconPath := sourceIconPath(result.ID)
	result.IconURL = &iconPath
	result.Note = nullStringPtr(note)
	result.BackfillMode = backfillMode.String
	if result.BackfillMode == "" {
		result.BackfillMode = "window"
	}
	if strings.TrimSpace(result.ConfigJSON) != "" {
		var config struct {
			FullText        bool    `json:"full_text"`
			WatchEnabled    bool    `json:"watch_enabled"`
			WatchMode       string  `json:"watch_mode"`
			WatchAI         bool    `json:"watch_ai"`
			WatchChannelIDs []int64 `json:"watch_channel_ids"`
		}
		if json.Unmarshal([]byte(result.ConfigJSON), &config) == nil {
			result.FullText = config.FullText
			result.WatchEnabled = config.WatchEnabled
			result.WatchMode = config.WatchMode
			result.WatchAI = config.WatchAI
			result.WatchChannelIDs = config.WatchChannelIDs
		}
	}
	if result.WatchMode != "each" && result.WatchMode != "batch" {
		result.WatchMode = "batch"
	}
	if result.WatchChannelIDs == nil {
		result.WatchChannelIDs = []int64{}
	}
	return result, nil
}

func (a *App) getSourceRow(ctx context.Context, id int64) (dbSource, error) {
	row := a.db.QueryRowContext(ctx, `SELECT `+sourceSelectColumns+` FROM sources s WHERE s.id = ?`, id)
	return scanSource(row)
}

func (a *App) getSource(ctx context.Context, id int64) (Source, error) {
	row, err := a.getSourceRow(ctx, id)
	return row.Source, err
}

func (a *App) sourceInternal(ctx context.Context, id int64) (dbSource, error) {
	result, err := a.getSourceRow(ctx, id)
	if err != nil {
		return result, err
	}
	err = a.db.QueryRowContext(ctx, `
		SELECT COALESCE(etag, ''), COALESCE(last_modified, ''), initial_fetched, feed_token, config_json
		FROM sources WHERE id = ?`, id).Scan(
		&result.ETag, &result.LastModified, &result.InitialFetched, &result.FeedToken, &result.ConfigJSON)
	return result, err
}

func (a *App) listSourceRows(ctx context.Context, folderID *int64) ([]dbSource, error) {
	query := `SELECT ` + sourceSelectColumns + ` FROM sources s`
	args := make([]any, 0, 1)
	if folderID != nil {
		query += ` WHERE s.folder_id = ?`
		args = append(args, *folderID)
	}
	query += ` ORDER BY s.name COLLATE NOCASE ASC, s.id ASC`
	rows, err := a.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]dbSource, 0)
	for rows.Next() {
		source, err := scanSource(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, source)
	}
	return result, rows.Err()
}

func (a *App) listFetchLogs(ctx context.Context, sourceID int64, limit int) ([]FetchLog, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	rows, err := a.db.QueryContext(ctx, `
		SELECT id, started_at, duration_ms, http_status, result, new_count, error
		FROM fetch_logs WHERE source_id = ?
		ORDER BY started_at DESC, id DESC LIMIT ?`, sourceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]FetchLog, 0)
	for rows.Next() {
		var log FetchLog
		var duration sql.NullInt64
		var status sql.NullInt64
		var errorText sql.NullString
		if err := rows.Scan(&log.ID, &log.StartedAt, &duration, &status, &log.Result, &log.NewCount, &errorText); err != nil {
			return nil, err
		}
		if duration.Valid {
			value := duration.Int64
			log.DurationMS = &value
		}
		if status.Valid {
			value := int(status.Int64)
			log.HTTPStatus = &value
		}
		log.Error = nullStringPtr(errorText)
		result = append(result, log)
	}
	return result, rows.Err()
}

func (a *App) sourceTrend(ctx context.Context, sourceID int64) ([]int, error) {
	now := a.now()
	start := now.AddDate(0, 0, -13).Format("2006-01-02")
	rows, err := a.db.QueryContext(ctx, `
		SELECT substr(started_at, 1, 10), COALESCE(SUM(new_count), 0)
		FROM fetch_logs
		WHERE source_id = ? AND substr(started_at, 1, 10) >= ?
		GROUP BY substr(started_at, 1, 10)`, sourceID, start)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byDay := make(map[string]int)
	for rows.Next() {
		var day string
		var count int
		if err := rows.Scan(&day, &count); err != nil {
			return nil, err
		}
		byDay[day] = count
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	result := make([]int, 14)
	for i := 0; i < 14; i++ {
		day := now.AddDate(0, 0, i-13).Format("2006-01-02")
		result[i] = byDay[day]
	}
	return result, nil
}

func (a *App) createSourceRow(ctx context.Context, input sourceCreateInput) (Source, error) {
	kind := strings.ToLower(strings.TrimSpace(input.Kind))
	if kind == "" {
		kind = "rss"
	}
	if kind != "rss" && kind != "x" && kind != "wechat" {
		return Source{}, badRequest("invalid_source", "源类型只能是 rss、x 或 wechat")
	}
	feedURL := strings.TrimSpace(input.FeedURL)
	if feedURL == "" {
		feedURL = strings.TrimSpace(input.URL)
	}
	feedURL = normalizeRSSHubURL(feedURL)
	name := strings.TrimSpace(input.Name)
	if kind == "x" && strings.TrimSpace(input.Handle) != "" {
		handle := strings.TrimPrefix(strings.TrimSpace(input.Handle), "@")
		if !validXHandle(handle) {
			return Source{}, badRequest("invalid_x_handle", "X 账号只能包含字母、数字与下划线")
		}
		feedURL = fmt.Sprintf("https://fxtwitter.com/%s/feed.xml?count=20", handle)
		if name == "" {
			name = "@" + handle
		}
		if input.SiteURL == nil {
			site := "https://x.com/" + handle
			input.SiteURL = &site
		}
	}
	if feedURL == "" {
		return Source{}, badRequest("url_required", "请输入订阅地址")
	}
	if err := a.validateSourceURL(ctx, kind, feedURL); err != nil {
		return Source{}, err
	}
	if len([]rune(name)) > 200 {
		return Source{}, badRequest("invalid_source", "源名称不能超过 200 字")
	}
	if name == "" {
		name = sourceNameFromURL(feedURL)
	}
	if input.SiteURL != nil && strings.TrimSpace(*input.SiteURL) != "" {
		if _, err := parseHTTPURL(*input.SiteURL); err != nil {
			return Source{}, badRequest("invalid_source", "站点地址无效："+err.Error())
		}
	}
	if input.FolderID != nil {
		if err := a.ensureFolderExists(ctx, *input.FolderID); err != nil {
			return Source{}, err
		}
	}
	refresh := 1800
	if input.RefreshSeconds != nil {
		refresh = *input.RefreshSeconds
	}
	if kind == "wechat" && refresh < 1800 {
		// WeRead and the MP back office both need aggressive rate limiting.
		refresh = 1800
	}
	if refresh < 60 || refresh > 604800 {
		return Source{}, badRequest("invalid_source", "刷新间隔需在 60 秒到 7 天之间")
	}
	// 默认首次回溯 100 条：源刚添加时「全部」流才有足够历史可看；
	// 上限仍为 100，可用编辑源调整（REQ-022 窗口语义不变）。
	initialLimit := 100
	if input.InitialLimit != nil {
		initialLimit = *input.InitialLimit
	}
	if initialLimit < 1 || initialLimit > 100 {
		return Source{}, badRequest("invalid_source", "首次回溯上限需在 1–100 之间")
	}
	retentionDays := 0
	if input.RetentionDays != nil {
		retentionDays = *input.RetentionDays
	}
	retentionMaxItems := 0
	if input.RetentionMaxItems != nil {
		retentionMaxItems = *input.RetentionMaxItems
	}
	if err := validateRetention(retentionDays, retentionMaxItems); err != nil {
		return Source{}, err
	}
	backfillMode := "window"
	if input.BackfillMode != nil && strings.TrimSpace(*input.BackfillMode) != "" {
		backfillMode = strings.TrimSpace(*input.BackfillMode)
	}
	if backfillMode != "window" && backfillMode != "full" {
		return Source{}, badRequest("invalid_source", "抓取范围只能是 window（最近窗口）或 full（全部历史）")
	}
	if backfillMode == "full" && !sourceSupportsFullBackfill(kind, feedURL) {
		return Source{}, badRequest("invalid_source", "该源暂不支持全量历史回补（当前支持 X 与 Bluesky）")
	}
	fallback := ""
	if input.FallbackURL != nil {
		fallback = strings.TrimSpace(*input.FallbackURL)
		if fallback != "" {
			if err := a.validateSourceURL(ctx, kind, fallback); err != nil {
				var apiErr *apiError
				if errors.As(err, &apiErr) && apiErr.Message != "" {
					return Source{}, badRequest("invalid_fallback", "备用源地址无效："+apiErr.Message)
				}
				return Source{}, badRequest("invalid_fallback", "备用源地址无效")
			}
		}
	}
	var duplicate int
	err := a.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sources WHERE lower(feed_url) = lower(?)`, feedURL).Scan(&duplicate)
	if err != nil {
		return Source{}, internalError(err)
	}
	if duplicate > 0 {
		return Source{}, conflict("source_exists", "该源已在订阅列表中")
	}
	token, err := newFeedToken()
	if err != nil {
		return Source{}, internalError(err)
	}
	now := a.now()
	nextFetch := now.Add(30 * time.Second)
	note := "等待首次抓取"
	if input.Note != nil && strings.TrimSpace(*input.Note) != "" {
		note = strings.TrimSpace(*input.Note)
	}
	config := map[string]any{}
	if input.FullText != nil && *input.FullText {
		config["full_text"] = true
	}
	if input.WatchEnabled != nil {
		config["watch_enabled"] = *input.WatchEnabled
	}
	if input.WatchMode != nil {
		if *input.WatchMode != "each" && *input.WatchMode != "batch" {
			return Source{}, badRequest("invalid_watch_mode", "关注提醒方式只能是 each 或 batch")
		}
		config["watch_mode"] = *input.WatchMode
	}
	if input.WatchAI != nil {
		config["watch_ai"] = *input.WatchAI
	}
	if input.WatchChannelIDs != nil {
		ids, apiErr := normalizeWatchChannelIDs(input.WatchChannelIDs)
		if apiErr != nil {
			return Source{}, apiErr
		}
		config["watch_channel_ids"] = ids
	}
	configData, err := json.Marshal(config)
	if err != nil {
		return Source{}, internalError(err)
	}
	configJSON := string(configData)
	result, err := a.db.ExecContext(ctx, `
		INSERT INTO sources (
			kind, name, feed_url, site_url, folder_id, config_json, enabled, refresh_seconds,
			initial_limit, fallback_url, icon_url, next_fetch_at, note, feed_token, created_at, updated_at,
			retention_days, retention_max_items, backfill_mode
		) VALUES (?, ?, ?, ?, ?, ?, 1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		kind, name, feedURL, nullableString(input.SiteURL), input.FolderID, configJSON, refresh, initialLimit,
		nullableStringValue(fallback), nullableString(input.IconURL), nextFetch.Format(timeRFC3339), note, token,
		now.Format(timeRFC3339), now.Format(timeRFC3339), retentionDays, retentionMaxItems, backfillMode)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return Source{}, conflict("source_exists", "该源已在订阅列表中")
		}
		return Source{}, internalError(err)
	}
	id, _ := result.LastInsertId()
	source, err := a.getSource(ctx, id)
	if err != nil {
		return Source{}, internalError(err)
	}
	return source, nil
}

type sourceCreateInput struct {
	Kind              string  `json:"kind"`
	Name              string  `json:"name"`
	FeedURL           string  `json:"feed_url"`
	URL               string  `json:"url"`
	Handle            string  `json:"handle"`
	SiteURL           *string `json:"site_url"`
	FolderID          *int64  `json:"folder_id"`
	RefreshSeconds    *int    `json:"refresh_seconds"`
	InitialLimit      *int    `json:"initial_limit"`
	FallbackURL       *string `json:"fallback_url"`
	RetentionDays     *int    `json:"retention_days"`
	RetentionMaxItems *int    `json:"retention_max_items"`
	BackfillMode      *string `json:"backfill_mode"`
	IconURL           *string `json:"icon_url"`
	Note              *string `json:"note"`
	FullText          *bool   `json:"full_text"`
	WatchEnabled      *bool   `json:"watch_enabled"`
	WatchMode         *string `json:"watch_mode"`
	WatchAI           *bool   `json:"watch_ai"`
	WatchChannelIDs   []int64 `json:"watch_channel_ids"`
}

// normalizeRSSHubURL turns rsshub:// share links into fetchable https URLs.
// The standard share format is `rsshub://<route/path>` which maps to the
// public instance `https://rsshub.app/<route/path>`; a dotted first segment is
// treated as a custom instance host (`rsshub://rsshub.example.com/route`).
// Non-rsshub URLs are returned unchanged.
func normalizeRSSHubURL(raw string) string {
	raw = strings.TrimSpace(raw)
	parsed, err := url.Parse(raw)
	if err != nil || !strings.EqualFold(parsed.Scheme, "rsshub") {
		return raw
	}
	host := parsed.Hostname()
	route := strings.Trim(parsed.Path, "/")
	if host == "" {
		return raw
	}
	base := "https://rsshub.app"
	if strings.Contains(host, ".") {
		base = "https://" + host
	} else {
		route = host + "/" + route
	}
	route = strings.Trim(route, "/")
	if route == "" {
		return raw
	}
	result := base + "/" + route
	if parsed.RawQuery != "" {
		result += "?" + parsed.RawQuery
	}
	return result
}

// sourceSupportsFullBackfill reports whether a source can page through its
// complete history. X uses the FxTwitter cursor API (~3200 posts ceiling);
// Bluesky uses the public AppView getAuthorFeed (no ceiling). Everything else
// only offers a recent window.
func sourceSupportsFullBackfill(kind, feedURL string) bool {
	switch kind {
	case "x":
		return true
	case "rss":
		return strings.Contains(strings.ToLower(feedURL), "bsky.app/profile/")
	}
	return false
}

func validXHandle(value string) bool {
	if len(value) == 0 || len(value) > 30 {
		return false
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			continue
		}
		return false
	}
	return true
}

func sourceNameFromURL(raw string) string {
	parts := strings.SplitN(raw, "://", 2)
	if len(parts) == 2 {
		host := strings.SplitN(parts[1], "/", 2)[0]
		if host != "" {
			return host
		}
	}
	if len(raw) > 80 {
		return raw[:80]
	}
	return raw
}

func nullableString(value *string) any {
	if value == nil {
		return nil
	}
	if strings.TrimSpace(*value) == "" {
		return nil
	}
	return strings.TrimSpace(*value)
}

func nullableStringValue(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return strings.TrimSpace(value)
}

func nullStringPtr(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	result := value.String
	return &result
}

func (a *App) validateSourceURL(ctx context.Context, kind, raw string) error {
	if kind == "wechat" {
		return a.validateWechatFeedURL(ctx, raw)
	}
	// REQ-041：邮箱同步源的 feed_url 是 imap://{username}[/{sender}]，
	// 不是 HTTP 目标，不走 fetcher 校验。
	if strings.HasPrefix(raw, "imap://") {
		if _, _, ok := parseEmailFeedURL(raw); !ok {
			return badRequest("invalid_email_url", "邮箱同步源地址无效")
		}
		return nil
	}
	if strings.HasPrefix(raw, "xiaoyuzhou://podcast/") {
		pid := strings.TrimSpace(strings.TrimPrefix(raw, "xiaoyuzhou://podcast/"))
		if !xiaoyuzhouIDPattern.MatchString(pid) {
			return badRequest("invalid_xiaoyuzhou_url", "小宇宙源地址无效")
		}
		return nil
	}
	if strings.HasPrefix(raw, "weibo://user/") || strings.HasPrefix(raw, "weibo:") {
		return badRequest("weibo_removed", "微博渠道已移除，请使用自建 RSSHub 等外部方案作为普通 RSS 添加")
	}
	if err := a.fetcher.ValidateURL(ctx, raw); err != nil {
		return badRequest("url_blocked", err.Error())
	}
	return nil
}

func (a *App) validateWechatFeedURL(ctx context.Context, raw string) error {
	settings, err := a.loadInternalSettings(ctx)
	if err != nil {
		return internalError(err)
	}
	if !settings.Wechat.Enabled {
		return conflict("wechat_disabled", "请先在设置中启用公众号采集")
	}
	// 内置公众号源（REQ-034）：wechat://<fakeid>，由微信读书通道抓取。
	if strings.HasPrefix(raw, "wechat://") {
		if strings.TrimSpace(strings.TrimPrefix(raw, "wechat://")) == "" {
			return badRequest("invalid_wechat_url", "公众号标识为空")
		}
		return nil
	}
	// DEC-024 移除 sidecar 后，历史 sidecar Feed 地址按普通 RSS 校验，
	// 存量源继续按标准 Feed 抓取。
	if _, err := parseHTTPURL(raw); err != nil {
		return badRequest("invalid_url", err.Error())
	}
	return nil
}

func parseHTTPURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed == nil {
		return nil, errors.New("地址格式无效")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, errors.New("只允许 http/https 地址")
	}
	if parsed.Hostname() == "" {
		return nil, errors.New("地址缺少主机名")
	}
	if parsed.User != nil {
		return nil, errors.New("地址不能包含用户凭据")
	}
	return parsed, nil
}

func validateRetention(days, maxItems int) error {
	if days < 0 || days > 3650 {
		return badRequest("invalid_retention", "保留天数需在 0–3650 之间，0 表示不限")
	}
	if maxItems < 0 || maxItems > 100000 {
		return badRequest("invalid_retention", "最多保留条数需在 0–100000 之间，0 表示不限")
	}
	return nil
}
