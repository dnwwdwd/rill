package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"math/rand/v2"

	"rill/internal/fetch"
)

func (a *App) sourceFetchLock(id int64) func() {
	value, _ := a.fetchLocks.LoadOrStore(id, &sync.Mutex{})
	lock := value.(*sync.Mutex)
	lock.Lock()
	return lock.Unlock
}

func (a *App) fetchSource(ctx context.Context, id int64, manual bool, overrideURL string) (Source, error) {
	unlock := a.sourceFetchLock(id)
	defer unlock()
	source, err := a.sourceInternal(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Source{}, notFound("source_not_found", "订阅源不存在")
	}
	if err != nil {
		return Source{}, internalError(err)
	}
	if manual && !source.Enabled {
		return Source{}, conflict("source_paused", "该源已暂停，请先启用")
	}
	if !manual && !source.Enabled {
		return source.Source, nil
	}
	fetchURL := source.FeedURL
	state := fetch.FetchState{ETag: source.ETag, LastModified: source.LastModified}
	if strings.TrimSpace(overrideURL) != "" {
		fetchURL = overrideURL
		state = fetch.FetchState{}
	}
	started := a.now()
	startReal := time.Now()
	var result *fetch.FeedResult
	var fetchErr error
	emailHandled := false
	fallbackURL := ""
	if source.FallbackURL != nil {
		fallbackURL = strings.TrimSpace(*source.FallbackURL)
	}
	// X sources degrade automatically to the FxEmbed public mirror; no manual
	// standby source is required.
	if fallbackURL == "" && source.Kind == "x" {
		fallbackURL = autoXFallbackURL(source.FeedURL)
	}
	var fallbackErr error
	if isBuiltinWechatSource(source) {
		result, fetchErr = a.fetchWechatSource(ctx, source)
	} else if strings.HasPrefix(source.FeedURL, "xiaoyuzhou://podcast/") {
		result, fetchErr = a.fetchXiaoyuzhou(ctx, source)
	} else if strings.HasPrefix(source.FeedURL, "imap://") {
		// REQ-041：邮箱同步源。syncEmailMailbox 内部完成抓取与入库，
		// 返回 nil 表示无需外部再走 applyFeedResult。
		fetchErr = a.syncEmailMailbox(ctx, source)
		emailHandled = fetchErr == nil
	} else if strings.HasPrefix(source.FeedURL, "weibo://user/") {
		// 微博渠道已移除（2026-11-08 用户决策）；存量源由启动迁移统一停用，
		// 这里兜底给出明确原因，避免报成普通抓取失败。
		result, fetchErr = nil, fmt.Errorf("微博渠道已移除，该订阅源已停用，可手动删除")
	} else {
		client, readyErr := a.clientForSource(ctx, source)
		if readyErr != nil {
			message := readyErr.Error()
			var apiErr *apiError
			if errors.As(readyErr, &apiErr) && apiErr.Message != "" {
				message = apiErr.Message
			}
			a.recordFetchFailure(ctx, source, message, nil, a.now(), 0)
			return Source{}, readyErr
		}
		result, fetchErr = client.FetchFeed(ctx, fetchURL, state, source.FeedToken)
		if fetchErr != nil {
			// Degradation policy (PDD §9): a scheduled fetch retries the primary
			// URL after a short pause — transient DNS/network failures must not
			// flip the source into an error state immediately. X gets two retries
			// 10s apart before the fixupx fallback; manual refresh does not wait.
			if !manual && ctx.Err() == nil {
				retryDelay := 2 * time.Second
				retryCount := 1
				if source.Kind == "x" {
					retryDelay = a.xFetchRetryDelay
					retryCount = a.xFetchRetryCount
				}
				for attempt := 0; attempt < retryCount && fetchErr != nil; attempt++ {
					select {
					case <-time.After(retryDelay):
					case <-ctx.Done():
					}
					if ctx.Err() != nil {
						break
					}
					startReal = time.Now()
					result, fetchErr = client.FetchFeed(ctx, fetchURL, state, source.FeedToken)
				}
			}
			// A configured fallback takes over before the failure is recorded,
			// matching the manual /fallback behaviour.
			if fetchErr != nil && overrideURL == "" && fallbackURL != "" {
				startReal = time.Now()
				var fallbackResult *fetch.FeedResult
				fallbackResult, fallbackErr = client.FetchFeed(ctx, fallbackURL, fetch.FetchState{}, source.FeedToken)
				if fallbackErr == nil {
					result, fetchErr, overrideURL = fallbackResult, nil, fallbackURL
				}
			}
		}
	}
	// 小宇宙分发 Feed 上游 IP 白名单（2026-11-21 实测 denied by IP ACL）：
	// 恢复节目 pid 并【永久替换】订阅方式为页面抓取源。必须在失败处理之前完成，
	// 否则替换成功 + 页面抓取成功仍会被当成失败返回（0.1.10 实测缺陷）。
	if fetchErr != nil && isXiaoyuzhouFeedURL(source.FeedURL) {
		if replaced, replacedPid, replaceErr := a.replaceXiaoyuzhouFeedSource(ctx, source); replaceErr == nil {
			source = replaced
			if replacedResult, replacedFetchErr := a.fetchXiaoyuzhou(ctx, source); replacedFetchErr == nil {
				result, fetchErr = replacedResult, nil
				overrideURL = "xiaoyuzhou://podcast/" + replacedPid
			} else {
				fetchErr = replacedFetchErr
			}
		}
	}
	duration := time.Since(startReal).Milliseconds()
	if fetchErr != nil {
		type statusCoder interface{ StatusCode() int }
		var coded statusCoder
		httpStatus := 0
		if errors.As(fetchErr, &coded) {
			httpStatus = coded.StatusCode()
		}
		message := readableFetchError(fetchErr, source.Kind)
		if isXiaoyuzhouFeedURL(source.FeedURL) {
			message = "小宇宙分发 Feed 被上游 IP 白名单拒绝（denied by IP ACL），且无法自动恢复小宇宙节目 ID"
			if source.SiteURL != nil && strings.Contains(*source.SiteURL, "podcasts.apple.com") {
				message += "；可用小宇宙节目链接重新订阅以走页面抓取"
			}
		} else if strings.HasPrefix(source.FeedURL, "xiaoyuzhou://podcast/") {
			message = "订阅已自动改用小宇宙页面抓取，但节目页暂时无法访问：" + message
		}
		if fallbackErr != nil {
			message = fmt.Sprintf(
				"主源 %s 失败：%s；降级源 %s 失败：%s",
				sourceHostLabel(fetchURL), message,
				sourceHostLabel(fallbackURL), readableFetchError(fallbackErr, source.Kind),
			)
		}
		a.recordFetchFailure(ctx, source, message, &httpStatus, started, duration)
		return Source{}, &apiError{
			Status:  http.StatusBadGateway,
			Code:    "fetch_failed",
			Message: message,
			Details: map[string]any{"source_id": id, "http_status": httpStatus},
		}
	}
	var applyErr error
	if fetchErr == nil && emailHandled {
		applyErr = a.applyEmailSourceOK(ctx, source, started, duration)
	} else if result.NotModified {
		applyErr = a.applyNotModified(ctx, source, started, duration, overrideURL)
	} else {
		a.enrichFullText(ctx, source, result.Items)
		_, _, applyErr = a.applyFeedResult(ctx, source, result, started, duration, overrideURL, source.InitialFetched && source.WatchEnabled)
	}
	if applyErr != nil {
		return Source{}, internalError(applyErr)
	}
	// BUG-004：抓取成功说明出口可用，清除图标失败标记，让下一轮
	// 图标请求立即用新候选链重试（旧失败标记会拦截最长 1 小时）。
	a.clearIconFailure(id)
	// X 源头像候选链在 2026-11-21 更新过；此前成功缓存的多是通用站点图标。
	// 图标默认按 7 天 TTL 不重抓，这里对 X 源按 6 小时失效，让新头像尽早替换。
	if isXFeedURL(source.FeedURL) {
		a.invalidateOldIcon(id, 6*time.Hour)
	}
	updated, err := a.getSource(ctx, id)
	if err != nil {
		return Source{}, internalError(err)
	}
	return updated, nil
}

func (a *App) clientForSource(ctx context.Context, source dbSource) (*fetch.Client, error) {
	if source.Kind != "wechat" {
		return a.fetcher, nil
	}
	settings, err := a.loadInternalSettings(ctx)
	if err != nil {
		return nil, internalError(err)
	}
	if !settings.Wechat.Enabled {
		return nil, conflict("wechat_disabled", "公众号采集已关闭")
	}
	// Built-in `wechat://` sources never reach this path (fetchWechatSource
	// handles them). Legacy sidecar feed URLs (DEC-024 removed the sidecar)
	// degrade to ordinary RSS fetching against their own host.
	return a.fetcher, nil
}

func readableFetchError(err error, kind string) string {
	message := strings.TrimSpace(err.Error())
	var statusErr *fetch.HTTPStatusError
	if errors.As(err, &statusErr) && statusErr.Status == http.StatusUnauthorized && kind == "wechat" {
		return "微信读书登录态已失效，请重新扫码"
	}
	if kind == "wechat" && errors.Is(err, errWereadNotConfigured) {
		return "微信读书未连接：请在设置中扫码登录微信读书"
	}
	if errors.As(err, &statusErr) {
		switch statusErr.Status {
		case http.StatusTooManyRequests:
			return "上游限流（HTTP 429），请稍后重试或配置备用源"
		case http.StatusForbidden:
			return "上游拒绝访问（HTTP 403），可能被 Cloudflare / 反爬拦截"
		case http.StatusNotFound:
			return "上游返回 404，账号或地址可能不存在"
		}
	}
	if message == "" {
		message = "抓取失败"
	}
	if len([]rune(message)) > 300 {
		message = string([]rune(message)[:300]) + "…"
	}
	return message
}

func sourceHostLabel(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err == nil && parsed.Host != "" {
		return parsed.Host
	}
	return "地址"
}

// autoXFallbackURL maps the primary FxTwitter RSS feed to FxEmbed's public
// mirror. Both serve the same feed; only the host changes.
func autoXFallbackURL(feedURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(feedURL))
	if err != nil || !strings.EqualFold(parsed.Hostname(), "fxtwitter.com") {
		return ""
	}
	parsed.Host = "fixupx.com"
	return parsed.String()
}

func (a *App) recordFetchFailure(ctx context.Context, source dbSource, message string, httpStatus *int, started time.Time, duration int64) {
	failCount := source.FailCount + 1
	next := started.Add(backoffFor(failCount))
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		a.logger.Error("write fetch failure", "source_id", source.ID, "err", err)
		return
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE sources SET last_fetch_at = ?, last_status = 'error', last_error = ?,
			fail_count = ?, next_fetch_at = ?, updated_at = ?
		WHERE id = ?`,
		started.UTC().Format(timeRFC3339), message, failCount, next.UTC().Format(timeRFC3339),
		a.now().Format(timeRFC3339), source.ID)
	if err == nil {
		var status any
		if httpStatus != nil && *httpStatus > 0 {
			status = *httpStatus
		}
		var durationValue any
		if duration > 0 {
			durationValue = duration
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO fetch_logs (source_id, started_at, duration_ms, http_status, result, item_count, new_count, error)
			VALUES (?, ?, ?, ?, 'error', 0, 0, ?)`,
			source.ID, started.UTC().Format(timeRFC3339), durationValue, status, message)
	}
	if err == nil {
		_, err = tx.ExecContext(ctx, `
			DELETE FROM fetch_logs WHERE source_id = ? AND id NOT IN (
				SELECT id FROM fetch_logs WHERE source_id = ? ORDER BY started_at DESC, id DESC LIMIT 50
			)`, source.ID, source.ID)
	}
	if err != nil {
		_ = tx.Rollback()
		a.logger.Error("write fetch failure", "source_id", source.ID, "err", err)
		return
	}
	if err := tx.Commit(); err != nil {
		a.logger.Error("commit fetch failure", "source_id", source.ID, "err", err)
	}
}

func backoffFor(failCount int) time.Duration {
	switch {
	case failCount <= 1:
		return time.Minute
	case failCount == 2:
		return 2 * time.Minute
	case failCount == 3:
		return 5 * time.Minute
	case failCount == 4:
		return 15 * time.Minute
	default:
		return time.Hour
	}
}

func (a *App) applyNotModified(ctx context.Context, source dbSource, started time.Time, duration int64, override string) error {
	next := nextFetchTime(a.now(), source.RefreshSeconds)
	var note any
	if override != "" {
		note = degradedSourceNote(override)
	}
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `
		UPDATE sources SET last_fetch_at = ?, last_status = 'not_modified', last_error = NULL,
			fail_count = 0, next_fetch_at = ?, note = ?, updated_at = ? WHERE id = ?`,
		started.UTC().Format(timeRFC3339), next.UTC().Format(timeRFC3339), note,
		a.now().Format(timeRFC3339), source.ID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO fetch_logs (source_id, started_at, duration_ms, http_status, result, item_count, new_count, error)
		VALUES (?, ?, ?, ?, 'not_modified', 0, 0, NULL)`,
		source.ID, started.UTC().Format(timeRFC3339), duration, http.StatusNotModified); err != nil {
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

func (a *App) applyFeedResult(ctx context.Context, source dbSource, result *fetch.FeedResult, started time.Time, duration int64, override string, notifyWatch bool) (int, int, error) {
	items := result.Items
	if !source.InitialFetched && len(items) > source.InitialLimit {
		items = items[:source.InitialLimit]
	}
	refreshMeta := false
	refreshMetaName := false
	config := map[string]any{}
	if strings.TrimSpace(source.ConfigJSON) != "" {
		if err := json.Unmarshal([]byte(source.ConfigJSON), &config); err == nil {
			refreshMeta, _ = config["refresh_metadata"].(bool)
			if nameFlag, ok := config["refresh_metadata_name"].(bool); ok {
				refreshMetaName = nameFlag
			}
		}
	}
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, err
	}
	defer func() {
		_ = tx.Rollback()
	}()
	existing := make(map[string]struct{})
	emailImagesAllowed := make(map[string]bool)
	rows, err := tx.QueryContext(ctx, `SELECT guid, COALESCE(media_json, '') FROM items WHERE source_id = ?`, source.ID)
	if err != nil {
		return 0, 0, err
	}
	isEmailSource := strings.HasPrefix(source.FeedURL, "imap://")
	for rows.Next() {
		var guid, mediaRaw string
		if err := rows.Scan(&guid, &mediaRaw); err != nil {
			_ = rows.Close()
			return 0, 0, err
		}
		existing[guid] = struct{}{}
		if isEmailSource && decodeMediaPayload(mediaRaw).EmailImagesAllowed {
			emailImagesAllowed[guid] = true
		}
	}
	if err := rows.Close(); err != nil {
		return 0, 0, err
	}
	newCount := 0
	var newIDs []int64
	newTitles := make(map[int64]string)
	newContents := make(map[int64]string)
	now := a.now()
	for _, item := range items {
		isNew := false
		if _, existed := existing[item.GUID]; !existed {
			newCount++
			isNew = true
		}
		existing[item.GUID] = struct{}{}
		mediaJSON, _ := json.Marshal(mediaPayload{
			Images:             item.MediaURLs,
			AudioURL:           item.AudioURL,
			AudioType:          item.AudioType,
			EmailSender:        item.EmailSender,
			EmailImagesAllowed: emailImagesAllowed[item.GUID],
		})
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO items (
				source_id, guid, url, title, author, published_at, published_raw,
				published_estimated, content_html, content_text, media_json, content_hash,
				fetched_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(source_id, guid) DO UPDATE SET
				url = excluded.url,
				title = excluded.title,
				author = excluded.author,
				content_html = excluded.content_html,
				content_text = excluded.content_text,
				media_json = excluded.media_json,
				content_hash = excluded.content_hash,
				updated_at = excluded.updated_at
			WHERE items.content_hash IS NULL
				OR items.content_hash <> excluded.content_hash
				OR COALESCE(items.url, '') <> COALESCE(excluded.url, '')`,
			source.ID, item.GUID, nullableStringValue(item.URL), item.Title, nullableStringValue(item.Author),
			item.PublishedAt.UTC().Format(timeRFC3339), nullableStringValue(item.PublishedRaw),
			boolInt(item.PublishedEstimated), item.ContentHTML, item.ContentText, string(mediaJSON),
			item.ContentHash, now.Format(timeRFC3339), now.Format(timeRFC3339)); err != nil {
			return 0, 0, err
		}
		if isNew {
			// REQ-037：新条目入库后按过滤规则处理（提交后执行）。
			// upsert 后按 (source_id, guid) 查回 id，LastInsertId 对 ON CONFLICT 不可靠。
			var itemID int64
			if qerr := tx.QueryRowContext(ctx,
				`SELECT id FROM items WHERE source_id = ? AND guid = ?`, source.ID, item.GUID).Scan(&itemID); qerr == nil && itemID > 0 {
				newIDs = append(newIDs, itemID)
				newTitles[itemID] = item.Title
				newContents[itemID] = item.ContentText
			}
		}
		for _, mediaURL := range item.MediaURLs {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO media_cache (url_hash, source_url, status, fail_count, last_access_at)
				VALUES (?, ?, 'pending', 0, ?)
				ON CONFLICT(url_hash) DO NOTHING`,
				fetch.URLHash(mediaURL), mediaURL, now.Format(timeRFC3339)); err != nil {
				return 0, 0, err
			}
		}
	}
	next := nextFetchTime(now, source.RefreshSeconds)
	var lastNew any
	if newCount > 0 {
		lastNew = now.Format(timeRFC3339)
	}
	httpStatus := result.HTTPStatus
	errorText := any(nil)
	etag := nullableStringValue(result.ETag)
	lastModified := nullableStringValue(result.LastModified)
	note := any(nil)
	if override != "" {
		note = degradedSourceNote(override)
		etag = source.nullableETag()
		lastModified = source.nullableLastModified()
	}
	if note == nil {
		if strings.HasPrefix(source.FeedURL, "xiaoyuzhou://podcast/") {
			note = "最近 15 条（小宇宙页面降级，非全量）"
		}
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE sources SET
			last_fetch_at = ?, last_status = 'ok', last_error = NULL, fail_count = 0,
			last_new_item_at = COALESCE(?, last_new_item_at), next_fetch_at = ?,
			initial_fetched = 1, etag = ?, last_modified = ?, note = ?,
			icon_url = COALESCE(?, icon_url), updated_at = ?
		WHERE id = ?`,
		started.UTC().Format(timeRFC3339), lastNew, next.UTC().Format(timeRFC3339),
		etag, lastModified, note, nullableStringValue(result.IconURL), now.Format(timeRFC3339), source.ID); err != nil {
		return 0, 0, err
	}
	if refreshMeta {
		metadataUpdates := make([]string, 0, 3)
		metadataArgs := make([]any, 0, 4)
		if title := strings.TrimSpace(result.Title); refreshMetaName && title != "" && len([]rune(title)) <= 200 && title != source.Name {
			metadataUpdates = append(metadataUpdates, "name = ?")
			metadataArgs = append(metadataArgs, title)
		}
		if link := strings.TrimSpace(result.Link); link != "" {
			if _, err := parseHTTPURL(link); err == nil {
				metadataUpdates = append(metadataUpdates, "site_url = ?")
				metadataArgs = append(metadataArgs, link)
			}
		}
		delete(config, "refresh_metadata")
		delete(config, "refresh_metadata_name")
		configData, err := json.Marshal(config)
		if err != nil {
			return 0, 0, err
		}
		metadataUpdates = append(metadataUpdates, "config_json = ?")
		metadataArgs = append(metadataArgs, string(configData), source.ID)
		if _, err := tx.ExecContext(ctx, "UPDATE sources SET "+strings.Join(metadataUpdates, ", ")+" WHERE id = ?", metadataArgs...); err != nil {
			return 0, 0, err
		}
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO fetch_logs (source_id, started_at, duration_ms, http_status, result, item_count, new_count, error)
		VALUES (?, ?, ?, ?, 'ok', ?, ?, ?)`,
		source.ID, started.UTC().Format(timeRFC3339), duration, httpStatus, len(items), newCount, errorText); err != nil {
		return 0, 0, err
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM fetch_logs WHERE source_id = ? AND id NOT IN (
			SELECT id FROM fetch_logs WHERE source_id = ? ORDER BY started_at DESC, id DESC LIMIT 50
		)`, source.ID, source.ID); err != nil {
		return 0, 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, 0, err
	}
	// REQ-037：提交成功后按过滤规则处理新条目（失败只记日志，不影响抓取结果）。
	if len(newIDs) > 0 {
		var folderNull sql.NullInt64
		if source.FolderID != nil {
			folderNull = sql.NullInt64{Int64: *source.FolderID, Valid: true}
		}
		a.applyFiltersToItems(ctx, source.ID, folderNull, newIDs, newTitles, newContents)
	}
	if notifyWatch && len(newIDs) > 0 {
		// 推送/AI 总结可能耗时数十秒，不能阻塞抓取请求和调度工作池。
		// 使用 App 根上下文而不是请求上下文，避免手动刷新断开后丢掉提醒。
		ids := append([]int64(nil), newIDs...)
		a.wg.Add(1)
		go func() {
			defer a.wg.Done()
			alertCtx := a.ctx
			if alertCtx == nil {
				alertCtx = context.Background()
			}
			a.notifyWatchedSource(alertCtx, source.ID, ids)
		}()
	}
	return newCount, len(items), nil
}

// degradedSourceNote explains an automatic degradation in the source note.
func degradedSourceNote(override string) string {
	if strings.HasPrefix(override, "xiaoyuzhou://podcast/") {
		return "已自动改用小宇宙页面抓取（最近 15 条；原分发 Feed 被上游 IP 白名单拒绝）"
	}
	return "已自动降级到 " + sourceHostLabel(override)
}

// xiaoyuzhouPidFromItems recovers the podcast id from stored episode links
// (https://www.xiaoyuzhoufm.com/episode/{eid}) written by the distribution
// feed while it was still reachable. The episode page reveals the pid.
func (a *App) xiaoyuzhouPidFromItems(ctx context.Context, sourceID int64) string {
	rows, err := a.db.QueryContext(ctx, `
		SELECT COALESCE(url, '') FROM items
		WHERE source_id = ? AND url LIKE '%xiaoyuzhoufm.com/episode/%'
		ORDER BY published_at DESC LIMIT 3`, sourceID)
	if err != nil {
		return ""
	}
	eids := make([]string, 0, 3)
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			continue
		}
		if _, eid, _, ok := xiaoyuzhouRefFromInput(raw); ok && eid != "" {
			eids = append(eids, eid)
		}
	}
	_ = rows.Close()
	for _, eid := range eids {
		data, err := a.fetchXiaoyuzhouPage(ctx, "episode", eid)
		if err != nil {
			continue
		}
		if episode := data.Props.PageProps.Episode; episode != nil {
			pid := strings.TrimSpace(episode.PID)
			if pid == "" && episode.Podcast != nil {
				pid = strings.TrimSpace(episode.Podcast.PID)
			}
			if pid != "" {
				return pid
			}
		}
	}
	return ""
}

// replaceXiaoyuzhouFeedSource permanently rewrites a distribution-feed source
// into a Xiaoyuzhou page-fetch source once the pid is recovered from the site
// URL or any stored episode link. This is a one-time subscription-method
// replacement: future fetches go straight to the page adapter and the denied
// feed is never requested again.
func (a *App) replaceXiaoyuzhouFeedSource(ctx context.Context, source dbSource) (dbSource, string, error) {
	pid := xiaoyuzhouPidFromSiteURL(source.SiteURL)
	if pid == "" {
		pid = a.xiaoyuzhouPidFromItems(ctx, source.ID)
	}
	if pid == "" {
		return dbSource{}, "", fmt.Errorf("无法从源信息恢复小宇宙节目 ID")
	}
	feedURL := "xiaoyuzhou://podcast/" + pid
	base := strings.TrimSpace(a.xiaoyuzhouBase)
	if base == "" {
		base = "https://www.xiaoyuzhoufm.com"
	}
	pageURL := base + "/podcast/" + url.PathEscape(pid)
	var duplicate int
	if err := a.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sources WHERE lower(feed_url) = lower(?) AND id <> ?`,
		feedURL, source.ID).Scan(&duplicate); err != nil {
		return dbSource{}, "", err
	}
	if duplicate > 0 {
		return dbSource{}, "", fmt.Errorf("已存在同一节目的页面抓取源，请删除其一")
	}
	note := "已自动改用小宇宙页面抓取（最近 15 条；原分发 Feed 被上游 IP 白名单拒绝）"
	if _, err := a.db.ExecContext(ctx, `
		UPDATE sources SET feed_url = ?, site_url = ?, note = ?, updated_at = ? WHERE id = ?`,
		feedURL, pageURL, note, a.now().Format(timeRFC3339), source.ID); err != nil {
		return dbSource{}, "", err
	}
	replaced := source
	replaced.FeedURL = feedURL
	replaced.SiteURL = &pageURL
	replaced.Note = ptrString(note)
	return replaced, pid, nil
}

func (source dbSource) nullableETag() any {
	if strings.TrimSpace(source.ETag) == "" {
		return nil
	}
	return source.ETag
}

func (source dbSource) nullableLastModified() any {
	if strings.TrimSpace(source.LastModified) == "" {
		return nil
	}
	return source.LastModified
}

func nextFetchTime(now time.Time, refreshSeconds int) time.Time {
	if refreshSeconds < 60 {
		refreshSeconds = 60
	}
	jitterRange := refreshSeconds / 10
	if jitterRange < 1 {
		jitterRange = 1
	}
	jitter := rand.IntN(jitterRange*2+1) - jitterRange
	return now.Add(time.Duration(refreshSeconds+jitter) * time.Second)
}
