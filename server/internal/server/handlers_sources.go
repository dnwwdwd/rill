package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type sourcePatch struct {
	Name              *string         `json:"name"`
	FeedURL           *string         `json:"feed_url"`
	FolderID          json.RawMessage `json:"folder_id"`
	RefreshSeconds    *int            `json:"refresh_seconds"`
	FallbackURL       json.RawMessage `json:"fallback_url"`
	Enabled           *bool           `json:"enabled"`
	Note              *string         `json:"note"`
	RetentionDays     *int            `json:"retention_days"`
	RetentionMaxItems *int            `json:"retention_max_items"`
	BackfillMode      *string         `json:"backfill_mode"`
	FullText          *bool           `json:"full_text"`
	WatchEnabled      *bool           `json:"watch_enabled"`
	WatchMode         *string         `json:"watch_mode"`
	WatchAI           *bool           `json:"watch_ai"`
	WatchChannelIDs   *[]int64        `json:"watch_channel_ids"`
}

func (a *App) handleListSources(w http.ResponseWriter, r *http.Request) error {
	var folderID *int64
	if raw := strings.TrimSpace(r.URL.Query().Get("folder_id")); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || value <= 0 {
			return badRequest("invalid_folder", "folder_id 无效")
		}
		folderID = &value
	}
	rows, err := a.listSourceRows(r.Context(), folderID)
	if err != nil {
		return internalError(err)
	}
	sources := make([]Source, 0, len(rows))
	for _, row := range rows {
		sources = append(sources, row.Source)
	}
	writeJSON(w, http.StatusOK, sources)
	return nil
}

func (a *App) handleCreateSource(w http.ResponseWriter, r *http.Request) error {
	var input sourceCreateInput
	if err := decodeJSON(r, &input); err != nil {
		return err
	}
	source, err := a.createSourceRow(r.Context(), input)
	if err != nil {
		return err
	}
	// Fetch immediately so a newly added source has articles without requiring
	// the user to press refresh. Failure is recorded on the source but must not
	// turn a successful creation into a 5xx.
	if !a.disableInitialFetch {
		fetchCtx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		if _, fetchErr := a.fetchSource(fetchCtx, source.ID, false, ""); fetchErr != nil {
			a.logger.Warn("initial source fetch failed", "source_id", source.ID, "err", fetchErr)
		}
		cancel()
		if refreshed, refreshErr := a.getSource(r.Context(), source.ID); refreshErr == nil {
			source = refreshed
		}
	}
	// 全量模式：窗口抓取完成后立即在后台回补全部历史。
	if source.BackfillMode == "full" && !a.disableBackfill {
		a.startBackfill(source.ID)
	}
	writeJSON(w, http.StatusCreated, source)
	return nil
}

func (a *App) handleGetSource(w http.ResponseWriter, r *http.Request) error {
	id, err := apiRouteID(r)
	if err != nil {
		return err
	}
	sourceRow, err := a.getSourceRow(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		return notFound("source_not_found", "订阅源不存在")
	}
	if err != nil {
		return internalError(err)
	}
	logs, err := a.listFetchLogs(r.Context(), id, 50)
	if err != nil {
		return internalError(err)
	}
	trend, err := a.sourceTrend(r.Context(), id)
	if err != nil {
		return internalError(err)
	}
	token, err := a.sourceFeedToken(r.Context(), id)
	if err != nil {
		return internalError(err)
	}
	health := SourceHealth{
		Source:          sourceRow.Source,
		Logs:            logs,
		Trend:           trend,
		FeedTokenMasked: maskToken(token),
		FeedPaths: FeedPaths{
			RSS:  sourceFeedPath(id, token, "xml"),
			Atom: sourceFeedPath(id, token, "atom"),
			JSON: sourceFeedPath(id, token, "json"),
		},
	}
	writeJSON(w, http.StatusOK, health)
	return nil
}

func (a *App) handleUpdateSource(w http.ResponseWriter, r *http.Request) error {
	id, err := apiRouteID(r)
	if err != nil {
		return err
	}
	current, err := a.getSourceRow(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		return notFound("source_not_found", "订阅源不存在")
	}
	if err != nil {
		return internalError(err)
	}
	var body sourcePatch
	if err := decodeJSON(r, &body); err != nil {
		return err
	}
	updates := make([]string, 0, 8)
	args := make([]any, 0, 10)
	feedChanged := false
	if body.FeedURL != nil {
		feedURL := normalizeRSSHubURL(strings.TrimSpace(*body.FeedURL))
		if feedURL == "" {
			return badRequest("url_required", "请输入订阅地址")
		}
		if feedURL != current.FeedURL {
			if strings.HasPrefix(current.FeedURL, "imap://") {
				return badRequest("invalid_source", "邮箱同步源的订阅地址由「邮箱同步」设置管理")
			}
			if err := a.validateSourceURL(r.Context(), current.Kind, feedURL); err != nil {
				var apiErr *apiError
				if errors.As(err, &apiErr) && apiErr.Message != "" {
					return badRequest("invalid_url", "订阅地址无效："+apiErr.Message)
				}
				return badRequest("invalid_url", "订阅地址无效")
			}
			var duplicate int
			if err := a.db.QueryRowContext(r.Context(),
				`SELECT COUNT(*) FROM sources WHERE lower(feed_url) = lower(?) AND id <> ?`,
				feedURL, id).Scan(&duplicate); err != nil {
				return internalError(err)
			}
			if duplicate > 0 {
				return conflict("source_exists", "该源已在订阅列表中")
			}
			feedChanged = true
			updates = append(updates,
				"feed_url = ?", "etag = NULL", "last_modified = NULL", "icon_url = NULL",
				"last_status = NULL", "last_error = NULL", "fail_count = 0",
				"initial_fetched = 0", "next_fetch_at = ?", "note = ?")
			args = append(args, feedURL, a.now().Format(timeRFC3339), "订阅地址已更新，正在抓取新内容…")
		}
	}
	renameRequested := false
	if body.Name != nil {
		name := strings.TrimSpace(*body.Name)
		if name == "" || len([]rune(name)) > 200 {
			return badRequest("invalid_source", "源名称不能为空且不能超过 200 字")
		}
		renameRequested = name != current.Name
		updates = append(updates, "name = ?")
		args = append(args, name)
	}
	if len(body.FolderID) > 0 {
		if string(body.FolderID) == "null" {
			updates = append(updates, "folder_id = NULL")
		} else {
			var folderID int64
			if err := json.Unmarshal(body.FolderID, &folderID); err != nil || folderID <= 0 {
				return badRequest("invalid_folder", "folder_id 无效")
			}
			if err := a.ensureFolderExists(r.Context(), folderID); err != nil {
				return err
			}
			updates = append(updates, "folder_id = ?")
			args = append(args, folderID)
		}
	}
	if body.RefreshSeconds != nil {
		if *body.RefreshSeconds < 60 || *body.RefreshSeconds > 604800 {
			return badRequest("invalid_source", "刷新间隔需在 60 秒到 7 天之间")
		}
		updates = append(updates, "refresh_seconds = ?")
		args = append(args, *body.RefreshSeconds)
	}
	if body.RetentionDays != nil || body.RetentionMaxItems != nil {
		days := current.RetentionDays
		maxItems := current.RetentionMaxItems
		if body.RetentionDays != nil {
			days = *body.RetentionDays
		}
		if body.RetentionMaxItems != nil {
			maxItems = *body.RetentionMaxItems
		}
		if err := validateRetention(days, maxItems); err != nil {
			return err
		}
		updates = append(updates, "retention_days = ?", "retention_max_items = ?")
		args = append(args, days, maxItems)
	}
	startBackfill := false
	if body.BackfillMode != nil {
		mode := strings.TrimSpace(*body.BackfillMode)
		if mode != "window" && mode != "full" {
			return badRequest("invalid_source", "抓取范围只能是 window（最近窗口）或 full（全部历史）")
		}
		if mode == "full" && !sourceSupportsFullBackfill(current.Kind, current.FeedURL) {
			return badRequest("invalid_source", "该源暂不支持全量历史回补（当前支持 X 与 Bluesky）")
		}
		if mode == "full" && current.BackfillMode != "full" {
			startBackfill = true
			updates = append(updates, "note = ?")
			args = append(args, "正在回补全部历史…")
		}
		updates = append(updates, "backfill_mode = ?")
		args = append(args, mode)
	}
	if len(body.FallbackURL) > 0 {
		if string(body.FallbackURL) == "null" {
			updates = append(updates, "fallback_url = NULL")
		} else {
			var fallback string
			if err := json.Unmarshal(body.FallbackURL, &fallback); err != nil {
				return badRequest("invalid_fallback", "备用源地址无效")
			}
			fallback = strings.TrimSpace(fallback)
			if fallback == "" {
				updates = append(updates, "fallback_url = NULL")
			} else {
				if err := a.validateSourceURL(r.Context(), current.Kind, fallback); err != nil {
					var apiErr *apiError
					if errors.As(err, &apiErr) && apiErr.Message != "" {
						return badRequest("invalid_fallback", "备用源地址无效："+apiErr.Message)
					}
					return badRequest("invalid_fallback", "备用源地址无效")
				}
				updates = append(updates, "fallback_url = ?")
				args = append(args, fallback)
			}
		}
	}
	if body.Enabled != nil {
		updates = append(updates, "enabled = ?")
		args = append(args, boolInt(*body.Enabled))
		if *body.Enabled && !current.Enabled {
			updates = append(updates, "fail_count = 0", "last_status = NULL", "last_error = NULL", "note = ?", "next_fetch_at = ?")
			args = append(args, "已重新启用", a.now().Add(30*time.Second).Format(timeRFC3339))
		} else if !*body.Enabled {
			updates = append(updates, "note = ?")
			args = append(args, "已暂停")
		}
	}
	if body.Note != nil {
		note := strings.TrimSpace(*body.Note)
		if note == "" {
			updates = append(updates, "note = NULL")
		} else {
			updates = append(updates, "note = ?")
			args = append(args, note)
		}
	}
	if body.FullText != nil || body.WatchEnabled != nil || body.WatchMode != nil || body.WatchAI != nil || body.WatchChannelIDs != nil || feedChanged {
		config := map[string]any{}
		if strings.TrimSpace(current.ConfigJSON) != "" {
			_ = json.Unmarshal([]byte(current.ConfigJSON), &config)
		}
		if feedChanged {
			// ApplyFeedResult uses this marker to adopt the new feed's title,
			// site URL and icon on the next successful fetch. An explicit rename
			// in the same request wins over the feed title.
			config["refresh_metadata"] = true
			if !renameRequested {
				config["refresh_metadata_name"] = true
			}
		}
		if body.FullText != nil {
			config["full_text"] = *body.FullText
		}
		if body.WatchEnabled != nil {
			config["watch_enabled"] = *body.WatchEnabled
		}
		if body.WatchMode != nil {
			if *body.WatchMode != "each" && *body.WatchMode != "batch" {
				return badRequest("invalid_watch_mode", "关注提醒方式只能是 each 或 batch")
			}
			config["watch_mode"] = *body.WatchMode
		}
		if body.WatchAI != nil {
			config["watch_ai"] = *body.WatchAI
		}
		if body.WatchChannelIDs != nil {
			ids, apiErr := normalizeWatchChannelIDs(*body.WatchChannelIDs)
			if apiErr != nil {
				return apiErr
			}
			config["watch_channel_ids"] = ids
		}
		data, err := json.Marshal(config)
		if err != nil {
			return internalError(err)
		}
		updates = append(updates, "config_json = ?")
		args = append(args, string(data))
	}
	if len(updates) == 0 {
		writeJSON(w, http.StatusOK, current.Source)
		return nil
	}
	updates = append(updates, "updated_at = ?")
	args = append(args, a.now().Format(timeRFC3339), id)
	if _, err := a.db.ExecContext(r.Context(), "UPDATE sources SET "+strings.Join(updates, ", ")+" WHERE id = ?", args...); err != nil {
		return internalError(err)
	}
	if feedChanged {
		a.clearSourceIcon(id)
		// Match source creation: fetch right away so the new address, metadata
		// and articles are visible without waiting for the next scheduler tick.
		if !a.disableInitialFetch {
			fetchCtx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
			if _, fetchErr := a.fetchSource(fetchCtx, id, true, ""); fetchErr != nil {
				a.logger.Warn("source URL change fetch failed", "source_id", id, "err", fetchErr)
			}
			cancel()
		}
	}
	if startBackfill && !a.disableBackfill {
		a.startBackfill(id)
	}
	source, err := a.getSource(r.Context(), id)
	if err != nil {
		return internalError(err)
	}
	writeJSON(w, http.StatusOK, source)
	return nil
}

func (a *App) handleDeleteSource(w http.ResponseWriter, r *http.Request) error {
	id, err := apiRouteID(r)
	if err != nil {
		return err
	}
	result, err := a.db.ExecContext(r.Context(), `DELETE FROM sources WHERE id = ?`, id)
	if err != nil {
		return internalError(err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return notFound("source_not_found", "订阅源不存在")
	}
	writeNoContent(w)
	return nil
}

func (a *App) handleFetchSource(w http.ResponseWriter, r *http.Request) error {
	id, err := apiRouteID(r)
	if err != nil {
		return err
	}
	source, err := a.getSourceRow(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		return notFound("source_not_found", "订阅源不存在")
	}
	if err != nil {
		return internalError(err)
	}
	if !source.Enabled {
		return conflict("source_paused", "该源已暂停，请先启用")
	}
	updated, fetchErr := a.fetchSource(r.Context(), id, true, "")
	if fetchErr != nil {
		return fetchErr
	}
	// 手动刷新 = 头像也刷新：清掉图标缓存（含旧失败标记），下次图标请求
	// 重新走候选链；前端根据 last_fetch_at 变化重新加载图片。
	a.clearSourceIcon(id)
	writeJSON(w, http.StatusOK, updated)
	return nil
}

func (a *App) handleFallbackSource(w http.ResponseWriter, r *http.Request) error {
	id, err := apiRouteID(r)
	if err != nil {
		return err
	}
	source, err := a.getSourceRow(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		return notFound("source_not_found", "订阅源不存在")
	}
	if err != nil {
		return internalError(err)
	}
	if source.FallbackURL == nil || strings.TrimSpace(*source.FallbackURL) == "" {
		return conflict("fallback_missing", "这个源还没有配置备用地址")
	}
	if !source.Enabled {
		return conflict("source_paused", "该源已暂停，请先启用")
	}
	updated, fetchErr := a.fetchSource(r.Context(), id, true, *source.FallbackURL)
	if fetchErr != nil {
		return fetchErr
	}
	writeJSON(w, http.StatusOK, updated)
	return nil
}

func (a *App) handleResetSourceToken(w http.ResponseWriter, r *http.Request) error {
	id, err := apiRouteID(r)
	if err != nil {
		return err
	}
	oldToken, err := a.sourceFeedToken(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		return notFound("source_not_found", "订阅源不存在")
	}
	if err != nil {
		return internalError(err)
	}
	token, err := newFeedToken()
	if err != nil {
		return internalError(err)
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		return internalError(err)
	}
	if _, err := tx.ExecContext(r.Context(), `UPDATE sources SET feed_token = ?, updated_at = ? WHERE id = ?`,
		token, a.now().Format(timeRFC3339), id); err != nil {
		_ = tx.Rollback()
		return internalError(err)
	}
	if oldToken != "" {
		if _, err := tx.ExecContext(r.Context(), `
			UPDATE items SET content_html = replace(content_html, ?, ?)
			WHERE source_id = ? AND content_html LIKE ?`,
			"/media/"+oldToken+"/", "/media/"+token+"/", id, "%/media/"+oldToken+"/%"); err != nil {
			_ = tx.Rollback()
			return internalError(err)
		}
	}
	if err := tx.Commit(); err != nil {
		return internalError(err)
	}
	writeJSON(w, http.StatusOK, TokenResetResponse{TokenMasked: maskToken(token)})
	return nil
}

func (a *App) handleResetGlobalToken(w http.ResponseWriter, r *http.Request) error {
	token, err := a.rotateGlobalFeedToken(r.Context())
	if err != nil {
		return internalError(err)
	}
	writeJSON(w, http.StatusOK, TokenResetResponse{TokenMasked: maskToken(token)})
	return nil
}

func (a *App) handleRevealGlobalToken(w http.ResponseWriter, r *http.Request) error {
	token, err := a.globalFeedToken(r.Context())
	if err != nil {
		return internalError(err)
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	writeJSON(w, http.StatusOK, map[string]string{"token": token})
	return nil
}

func (a *App) handleSourceLogs(w http.ResponseWriter, r *http.Request) error {
	id, err := apiRouteID(r)
	if err != nil {
		return err
	}
	if _, err := a.getSourceRow(r.Context(), id); errors.Is(err, sql.ErrNoRows) {
		return notFound("source_not_found", "订阅源不存在")
	} else if err != nil {
		return internalError(err)
	}
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			return badRequest("invalid_limit", "limit 无效")
		}
		limit = parsed
	}
	logs, err := a.listFetchLogs(r.Context(), id, limit)
	if err != nil {
		return internalError(err)
	}
	writeJSON(w, http.StatusOK, logs)
	return nil
}

func (a *App) sourceFeedToken(ctx context.Context, id int64) (string, error) {
	var token string
	err := a.db.QueryRowContext(ctx, `SELECT feed_token FROM sources WHERE id = ?`, id).Scan(&token)
	return token, err
}

func sourceFeedPath(id int64, token, extension string) string {
	return "/feed/" + strconv.FormatInt(id, 10) + "/" + token + "." + extension
}

func normalizeWatchChannelIDs(ids []int64) ([]int64, *apiError) {
	seen := make(map[int64]struct{}, len(ids))
	result := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return nil, badRequest("invalid_channel_ids", "推送渠道 ID 必须是正整数")
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result, nil
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
