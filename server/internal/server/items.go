package server

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"
)

type itemFilter struct {
	folderID *int64
	sourceID *int64
	unread   bool
	read     bool
	starred  bool
	later    bool
	hidden   bool // true = 只看被过滤隐藏的条目；false = 默认列表（排除隐藏）
	query    string
	sort     string
	limit    int
	cursor   string
}

type itemCursor struct {
	SortKey   string `json:"k"`
	ID        int64  `json:"id"`
	Estimated int    `json:"e"`
}

func parseItemFilter(r *http.Request) (itemFilter, error) {
	filter := itemFilter{limit: 20, sort: r.URL.Query().Get("sort")}
	if filter.sort == "" {
		filter.sort = "published"
	}
	if filter.sort != "published" && filter.sort != "fetched" {
		return filter, badRequest("invalid_sort", "排序只能是 published 或 fetched")
	}
	if raw := r.URL.Query().Get("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value <= 0 {
			return filter, badRequest("invalid_limit", "limit 无效")
		}
		filter.limit = value
	}
	if filter.limit > 50 {
		filter.limit = 50
	}
	for name, target := range map[string]**int64{"folder_id": &filter.folderID, "source_id": &filter.sourceID} {
		if raw := strings.TrimSpace(r.URL.Query().Get(name)); raw != "" {
			value, err := strconv.ParseInt(raw, 10, 64)
			if err != nil || value <= 0 {
				return filter, badRequest("invalid_filter", name+" 无效")
			}
			*target = &value
		}
	}
	var err error
	if filter.hidden, err = queryBool(r, "hidden"); err != nil {
		return filter, err
	}
	if filter.unread, err = queryBool(r, "unread"); err != nil {
		return filter, err
	}
	if filter.read, err = queryBool(r, "read"); err != nil {
		return filter, err
	}
	if filter.starred, err = queryBool(r, "starred"); err != nil {
		return filter, err
	}
	if filter.later, err = queryBool(r, "later"); err != nil {
		return filter, err
	}
	if filter.unread && filter.read {
		return filter, badRequest("invalid_filter", "不能同时筛选已读和未读")
	}
	filter.query = strings.TrimSpace(r.URL.Query().Get("q"))
	if len([]rune(filter.query)) > 300 {
		return filter, badRequest("invalid_query", "搜索关键词过长")
	}
	filter.cursor = strings.TrimSpace(r.URL.Query().Get("cursor"))
	return filter, nil
}

func queryBool(r *http.Request, key string) (bool, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return false, nil
	}
	switch strings.ToLower(raw) {
	case "1", "true", "yes":
		return true, nil
	case "0", "false", "no":
		return false, nil
	default:
		return false, badRequest("invalid_filter", key+" 必须是布尔值")
	}
}

func (a *App) handleListItems(w http.ResponseWriter, r *http.Request) error {
	filter, err := parseItemFilter(r)
	if err != nil {
		return err
	}
	page, err := a.queryItems(r.Context(), filter)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, page)
	return nil
}

func (a *App) handleSearch(w http.ResponseWriter, r *http.Request) error {
	filter, err := parseItemFilter(r)
	if err != nil {
		return err
	}
	if filter.query == "" {
		writeJSON(w, http.StatusOK, ItemPage{Items: []ItemSummary{}, NextCursor: nil})
		return nil
	}
	page, err := a.queryItems(r.Context(), filter)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, page)
	return nil
}

func (a *App) queryItems(ctx context.Context, filter itemFilter) (ItemPage, error) {
	if filter.query != "" && len([]rune(filter.query)) >= 3 {
		page, err := a.queryItemsWithMode(ctx, filter, true)
		if err == nil {
			return page, nil
		}
		a.logger.Warn("FTS search failed, falling back to LIKE", "err", err)
	}
	return a.queryItemsWithMode(ctx, filter, false)
}

func (a *App) queryItemsWithMode(ctx context.Context, filter itemFilter, useFTS bool) (ItemPage, error) {
	where := make([]string, 0, 8)
	args := make([]any, 0, 12)
	if filter.folderID != nil {
		where = append(where, "s.folder_id = ?")
		args = append(args, *filter.folderID)
	}
	if filter.sourceID != nil {
		where = append(where, "i.source_id = ?")
		args = append(args, *filter.sourceID)
	}
	if filter.hidden {
		where = append(where, "i.hidden = 1")
	} else {
		where = append(where, "i.hidden = 0")
	}
	if filter.unread {
		where = append(where, "i.read_at IS NULL")
	}
	if filter.read {
		where = append(where, "i.read_at IS NOT NULL")
	}
	if filter.starred {
		where = append(where, "i.starred_at IS NOT NULL")
	}
	if filter.later {
		where = append(where, "i.later_at IS NOT NULL")
	}
	if filter.query != "" {
		if useFTS {
			where = append(where, `(i.id IN (SELECT rowid FROM items_fts WHERE items_fts MATCH ?)
				OR instr(lower(s.name), lower(?)) > 0)`)
			args = append(args, ftsPhrase(filter.query), filter.query)
		} else {
			where = append(where, `(instr(lower(i.title), lower(?)) > 0
				OR instr(lower(COALESCE(i.content_text, '')), lower(?)) > 0
				OR instr(lower(COALESCE(i.author, '')), lower(?)) > 0
				OR instr(lower(s.name), lower(?)) > 0)`)
			args = append(args, filter.query, filter.query, filter.query, filter.query)
		}
	}
	var cursor *itemCursor
	if filter.cursor != "" {
		parsed, err := decodeItemCursor(filter.cursor)
		if err != nil {
			return ItemPage{}, badRequest("invalid_cursor", "分页游标无效")
		}
		cursor = parsed
	}
	orderBy := "i.published_estimated ASC, COALESCE(i.published_at, i.fetched_at) DESC, i.id DESC"
	if filter.sort == "fetched" {
		orderBy = "i.fetched_at DESC, i.id DESC"
	}
	if cursor != nil {
		if filter.sort == "fetched" {
			where = append(where, "(i.fetched_at < ? OR (i.fetched_at = ? AND i.id < ?))")
			args = append(args, cursor.SortKey, cursor.SortKey, cursor.ID)
		} else if cursor.Estimated == 0 {
			where = append(where, `(i.published_estimated = 1
				OR COALESCE(i.published_at, i.fetched_at) < ?
				OR (COALESCE(i.published_at, i.fetched_at) = ? AND i.id < ?))`)
			args = append(args, cursor.SortKey, cursor.SortKey, cursor.ID)
		} else {
			where = append(where, `i.published_estimated = 1 AND
				(COALESCE(i.published_at, i.fetched_at) < ?
				OR (COALESCE(i.published_at, i.fetched_at) = ? AND i.id < ?))`)
			args = append(args, cursor.SortKey, cursor.SortKey, cursor.ID)
		}
	}
	query := `
		SELECT i.id, i.source_id, s.name, s.kind, i.title, i.url, i.author,
		       i.published_at, i.published_estimated, i.content_text, substr(i.content_html, 1, 16384),
		       i.read_at, i.starred_at, i.later_at, i.fetched_at,
		       COALESCE(i.published_at, i.fetched_at) AS sort_key
		FROM items i JOIN sources s ON s.id = i.source_id`
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY " + orderBy + " LIMIT ?"
	args = append(args, filter.limit+1)
	rows, err := a.db.QueryContext(ctx, query, args...)
	if err != nil {
		return ItemPage{}, internalError(err)
	}
	defer rows.Close()
	items := make([]ItemSummary, 0, filter.limit)
	cursors := make([]itemCursor, 0, filter.limit)
	for rows.Next() {
		summary, sortKey, estimated, err := scanItemSummary(rows)
		if err != nil {
			return ItemPage{}, internalError(err)
		}
		items = append(items, summary)
		cursors = append(cursors, itemCursor{SortKey: sortKey, ID: summary.ID, Estimated: estimated})
	}
	if err := rows.Err(); err != nil {
		return ItemPage{}, internalError(err)
	}
	var next *string
	if len(items) > filter.limit {
		items = items[:filter.limit]
		cursorValue := encodeItemCursor(cursors[filter.limit-1])
		next = &cursorValue
	}
	return ItemPage{Items: items, NextCursor: next}, nil
}

func scanItemSummary(scanner rowScanner) (ItemSummary, string, int, error) {
	var summary ItemSummary
	var url, author, published, contentText, contentHTML, readAt, starredAt, laterAt sql.NullString
	var estimated int
	var sortKey string
	err := scanner.Scan(
		&summary.ID, &summary.SourceID, &summary.Source.Name, &summary.Source.Kind,
		&summary.Title, &url, &author, &published, &estimated, &contentText, &contentHTML,
		&readAt, &starredAt, &laterAt, &summary.FetchedAt, &sortKey,
	)
	if err != nil {
		return summary, "", 0, err
	}
	summary.Source.ID = summary.SourceID
	summary.URL = nullStringPtr(url)
	summary.Author = nullStringPtr(author)
	summary.PublishedAt = nullStringPtr(published)
	summary.PublishedEstimated = estimated != 0
	summary.CoverURL = firstProxiedImage(contentHTML.String)
	summary.ReadAt = nullStringPtr(readAt)
	summary.StarredAt = nullStringPtr(starredAt)
	summary.LaterAt = nullStringPtr(laterAt)
	summary.Excerpt = makeExcerpt(contentText.String, 220)
	return summary, sortKey, estimated, nil
}

func makeExcerpt(value string, maxRunes int) string {
	value = strings.Join(strings.Fields(value), " ")
	if value == "" {
		return ""
	}
	runes := []rune(value)
	if len(runes) > maxRunes {
		value = string(runes[:maxRunes]) + "…"
	}
	return value
}

// firstProxiedImage returns the first media-proxy image in sanitized article
// HTML. The list uses it as an optional cover without storing new columns.
func firstProxiedImage(contentHTML string) *string {
	const marker = `src="/media/`
	index := strings.Index(contentHTML, marker)
	if index < 0 {
		return nil
	}
	start := index + len(`src="`)
	end := strings.IndexByte(contentHTML[start:], '"')
	if end <= 0 {
		return nil
	}
	value := contentHTML[start : start+end]
	if !strings.HasPrefix(value, "/media/") {
		return nil
	}
	return &value
}

func encodeItemCursor(cursor itemCursor) string {
	data, _ := json.Marshal(cursor)
	return base64.RawURLEncoding.EncodeToString(data)
}

func decodeItemCursor(value string) (*itemCursor, error) {
	data, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return nil, err
	}
	var cursor itemCursor
	if err := json.Unmarshal(data, &cursor); err != nil {
		return nil, err
	}
	if cursor.ID <= 0 || cursor.SortKey == "" {
		return nil, errors.New("empty cursor")
	}
	return &cursor, nil
}

func (a *App) handleGetItem(w http.ResponseWriter, r *http.Request) error {
	id, err := apiRouteID(r)
	if err != nil {
		return err
	}
	detail, err := a.getItemDetail(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		return notFound("item_not_found", "条目不存在")
	}
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, detail)
	return nil
}

func (a *App) getItemDetail(ctx context.Context, id int64) (ItemDetail, error) {
	var detail ItemDetail
	var url, author, published, contentText, readAt, starredAt, laterAt, originalTitle, contentHTML, mediaRaw sql.NullString
	var estimated int
	err := a.db.QueryRowContext(ctx, `
		SELECT i.id, i.source_id, s.name, s.kind, i.title, i.url, i.author,
		       i.published_at, i.published_estimated, i.content_text,
		       i.read_at, i.starred_at, i.later_at, i.fetched_at,
		       i.content_html, i.original_title, COALESCE(i.media_json, '')
		FROM items i JOIN sources s ON s.id = i.source_id
		WHERE i.id = ?`, id).Scan(
		&detail.ID, &detail.SourceID, &detail.Source.Name, &detail.Source.Kind,
		&detail.Title, &url, &author, &published, &estimated, &contentText,
		&readAt, &starredAt, &laterAt, &detail.FetchedAt, &contentHTML, &originalTitle, &mediaRaw,
	)
	if err != nil {
		return detail, err
	}
	detail.Source.ID = detail.SourceID
	detail.URL = nullStringPtr(url)
	detail.Author = nullStringPtr(author)
	detail.PublishedAt = nullStringPtr(published)
	detail.PublishedEstimated = estimated != 0
	detail.ReadAt = nullStringPtr(readAt)
	detail.StarredAt = nullStringPtr(starredAt)
	detail.LaterAt = nullStringPtr(laterAt)
	detail.Excerpt = makeExcerpt(contentText.String, 220)
	detail.CoverURL = firstProxiedImage(contentHTML.String)
	detail.ContentHTML = contentHTML.String
	detail.ContentText = contentText.String
	detail.OriginalTitle = nullStringPtr(originalTitle)
	media := decodeMediaPayload(mediaRaw.String)
	detail.AudioURL = media.AudioURL
	detail.AudioType = media.AudioType
	if err := a.db.QueryRowContext(ctx, `
		SELECT position_seconds, duration_seconds FROM podcast_progress WHERE item_id = ?`, id).
		Scan(&detail.PodcastPositionSeconds, &detail.PodcastDurationSeconds); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return detail, err
	}
	summary, translation, err := a.loadAIResults(ctx, id)
	if err != nil {
		return detail, err
	}
	detail.Summary = summary
	detail.Translation = translation
	notes, err := a.listItemNotes(ctx, id)
	if err != nil {
		return detail, err
	}
	detail.Notes = notes
	return detail, nil
}

type podcastProgressPatch struct {
	PositionSeconds float64 `json:"position_seconds"`
	DurationSeconds float64 `json:"duration_seconds"`
}

func (a *App) handlePodcastProgress(w http.ResponseWriter, r *http.Request) error {
	id, err := apiRouteID(r)
	if err != nil {
		return err
	}
	var patch podcastProgressPatch
	if err := decodeJSON(r, &patch); err != nil {
		return err
	}
	if math.IsNaN(patch.PositionSeconds) || math.IsInf(patch.PositionSeconds, 0) || patch.PositionSeconds < 0 {
		return badRequest("invalid_position", "播放位置必须是非负数")
	}
	if math.IsNaN(patch.DurationSeconds) || math.IsInf(patch.DurationSeconds, 0) || patch.DurationSeconds < 0 {
		patch.DurationSeconds = 0
	}
	if patch.DurationSeconds > 0 && patch.PositionSeconds > patch.DurationSeconds {
		patch.PositionSeconds = patch.DurationSeconds
	}
	var exists int
	if err := a.db.QueryRowContext(r.Context(), `SELECT 1 FROM items WHERE id = ?`, id).Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return notFound("item_not_found", "条目不存在")
		}
		return internalError(err)
	}
	if _, err := a.db.ExecContext(r.Context(), `
		INSERT INTO podcast_progress (item_id, position_seconds, duration_seconds, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(item_id) DO UPDATE SET
			position_seconds = excluded.position_seconds,
			duration_seconds = excluded.duration_seconds,
			updated_at = excluded.updated_at`,
		id, patch.PositionSeconds, patch.DurationSeconds, a.now().Format(timeRFC3339)); err != nil {
		return internalError(err)
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

type itemStatePatch struct {
	Read    *bool `json:"read"`
	Starred *bool `json:"starred"`
	Later   *bool `json:"later"`
	Hidden  *bool `json:"hidden"`
}

func (a *App) handlePatchItemState(w http.ResponseWriter, r *http.Request) error {
	id, err := apiRouteID(r)
	if err != nil {
		return err
	}
	var patch itemStatePatch
	if err := decodeJSON(r, &patch); err != nil {
		return err
	}
	if patch.Read == nil && patch.Starred == nil && patch.Later == nil && patch.Hidden == nil {
		return badRequest("invalid_state", "至少提供 read、starred、later 或 hidden")
	}
	updates := make([]string, 0, 4)
	args := make([]any, 0, 5)
	if patch.Hidden != nil {
		updates = append(updates, "hidden = ?")
		if *patch.Hidden {
			args = append(args, 1)
		} else {
			args = append(args, 0)
		}
	}
	if patch.Read != nil {
		updates = append(updates, "read_at = ?")
		if *patch.Read {
			args = append(args, a.now().Format(timeRFC3339))
		} else {
			args = append(args, nil)
		}
	}
	if patch.Starred != nil {
		updates = append(updates, "starred_at = ?")
		if *patch.Starred {
			args = append(args, a.now().Format(timeRFC3339))
		} else {
			args = append(args, nil)
		}
	}
	if patch.Later != nil {
		updates = append(updates, "later_at = ?")
		if *patch.Later {
			args = append(args, a.now().Format(timeRFC3339))
		} else {
			args = append(args, nil)
		}
	}
	args = append(args, id)
	result, err := a.db.ExecContext(r.Context(), "UPDATE items SET "+strings.Join(updates, ", ")+" WHERE id = ?", args...)
	if err != nil {
		return internalError(err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return notFound("item_not_found", "条目不存在")
	}
	summary, err := a.getItemSummary(r.Context(), id)
	if err != nil {
		return internalError(err)
	}
	writeJSON(w, http.StatusOK, summary)
	return nil
}

func (a *App) getItemSummary(ctx context.Context, id int64) (ItemSummary, error) {
	row := a.db.QueryRowContext(ctx, `
		SELECT i.id, i.source_id, s.name, s.kind, i.title, i.url, i.author,
		       i.published_at, i.published_estimated, i.content_text, substr(i.content_html, 1, 16384),
		       i.read_at, i.starred_at, i.later_at, i.fetched_at,
		       COALESCE(i.published_at, i.fetched_at)
		FROM items i JOIN sources s ON s.id = i.source_id WHERE i.id = ?`, id)
	summary, _, _, err := scanItemSummary(row)
	return summary, err
}

type batchStateRequest struct {
	IDs     []int64 `json:"ids"`
	Read    *bool   `json:"read"`
	Starred *bool   `json:"starred"`
	Later   *bool   `json:"later"`
	Hidden  *bool   `json:"hidden"`
}

func (a *App) handleBatchItemState(w http.ResponseWriter, r *http.Request) error {
	var body batchStateRequest
	if err := decodeJSON(r, &body); err != nil {
		return err
	}
	if len(body.IDs) == 0 {
		return badRequest("invalid_state", "ids 不能为空")
	}
	if len(body.IDs) > 1000 {
		return badRequest("invalid_state", "单次最多更新 1000 条")
	}
	if body.Read == nil && body.Starred == nil && body.Later == nil && body.Hidden == nil {
		return badRequest("invalid_state", "至少提供 read、starred、later 或 hidden")
	}
	ids := uniqueIDs(body.IDs)
	updates := make([]string, 0, 3)
	baseArgs := make([]any, 0, 3)
	if body.Read != nil {
		updates = append(updates, "read_at = ?")
		if *body.Read {
			baseArgs = append(baseArgs, a.now().Format(timeRFC3339))
		} else {
			baseArgs = append(baseArgs, nil)
		}
	}
	if body.Starred != nil {
		updates = append(updates, "starred_at = ?")
		if *body.Starred {
			baseArgs = append(baseArgs, a.now().Format(timeRFC3339))
		} else {
			baseArgs = append(baseArgs, nil)
		}
	}
	if body.Later != nil {
		updates = append(updates, "later_at = ?")
		if *body.Later {
			baseArgs = append(baseArgs, a.now().Format(timeRFC3339))
		} else {
			baseArgs = append(baseArgs, nil)
		}
	}
	if body.Hidden != nil {
		updates = append(updates, "hidden = ?")
		baseArgs = append(baseArgs, boolInt(*body.Hidden))
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		return internalError(err)
	}
	existing, err := queryExistingIDs(r.Context(), tx, ids)
	if err != nil {
		_ = tx.Rollback()
		return internalError(err)
	}
	placeholders := make([]string, len(existing))
	args := append([]any{}, baseArgs...)
	for index, id := range existing {
		placeholders[index] = "?"
		args = append(args, id)
	}
	if len(existing) > 0 {
		if _, err := tx.ExecContext(r.Context(), "UPDATE items SET "+strings.Join(updates, ", ")+" WHERE id IN ("+strings.Join(placeholders, ",")+")", args...); err != nil {
			_ = tx.Rollback()
			return internalError(err)
		}
	}
	if err := tx.Commit(); err != nil {
		return internalError(err)
	}
	writeJSON(w, http.StatusOK, BatchStateResponse{Updated: len(existing), IDs: existing})
	return nil
}

func queryExistingIDs(ctx context.Context, tx *sql.Tx, ids []int64) ([]int64, error) {
	if len(ids) == 0 {
		return []int64{}, nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, len(ids))
	for index, id := range ids {
		args[index] = id
	}
	rows, err := tx.QueryContext(ctx, `SELECT id FROM items WHERE id IN (`+placeholders+`) ORDER BY id ASC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]int64, 0, len(ids))
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		result = append(result, id)
	}
	return result, rows.Err()
}

func uniqueIDs(ids []int64) []int64 {
	seen := make(map[int64]struct{}, len(ids))
	result := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

type markAllReadRequest struct {
	SourceID *int64 `json:"source_id"`
	FolderID *int64 `json:"folder_id"`
}

func (a *App) handleMarkAllRead(w http.ResponseWriter, r *http.Request) error {
	var body markAllReadRequest
	if err := decodeJSON(r, &body); err != nil {
		return err
	}
	where := "i.read_at IS NULL"
	args := make([]any, 0, 2)
	if body.SourceID != nil {
		if *body.SourceID <= 0 {
			return badRequest("invalid_filter", "source_id 无效")
		}
		where += " AND i.source_id = ?"
		args = append(args, *body.SourceID)
	} else if body.FolderID != nil {
		if *body.FolderID <= 0 {
			return badRequest("invalid_filter", "folder_id 无效")
		}
		where += ` AND i.source_id IN (
			WITH RECURSIVE tree(id) AS (
				SELECT id FROM folders WHERE id = ?
				UNION ALL
				SELECT f.id FROM folders f JOIN tree t ON f.parent_id = t.id
			)
			SELECT id FROM sources WHERE folder_id IN (SELECT id FROM tree)
		)`
		args = append(args, *body.FolderID)
	}
	rows, err := a.db.QueryContext(r.Context(), `SELECT i.id FROM items i WHERE `+where+` ORDER BY i.id ASC`, args...)
	if err != nil {
		return internalError(err)
	}
	ids := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return internalError(err)
		}
		ids = append(ids, id)
	}
	if err := rows.Close(); err != nil {
		return internalError(err)
	}
	if len(ids) > 0 {
		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
		updateArgs := make([]any, 0, len(ids)+1)
		updateArgs = append(updateArgs, a.now().Format(timeRFC3339))
		for _, id := range ids {
			updateArgs = append(updateArgs, id)
		}
		if _, err := a.db.ExecContext(r.Context(), `UPDATE items SET read_at = ? WHERE id IN (`+placeholders+`)`, updateArgs...); err != nil {
			return internalError(err)
		}
	}
	if ids == nil {
		ids = []int64{}
	}
	writeJSON(w, http.StatusOK, BatchStateResponse{Updated: len(ids), IDs: ids})
	return nil
}

func (a *App) handleItemStats(w http.ResponseWriter, r *http.Request) error {
	var stats ReaderStats
	err := a.db.QueryRowContext(r.Context(), `
		SELECT COUNT(*),
		       COALESCE(SUM(CASE WHEN read_at IS NULL THEN 1 ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN starred_at IS NOT NULL THEN 1 ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN read_at IS NOT NULL THEN 1 ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN later_at IS NOT NULL THEN 1 ELSE 0 END), 0)
		FROM items WHERE hidden = 0`).Scan(&stats.Total, &stats.Unread, &stats.Starred, &stats.Read, &stats.Later)
	if err != nil {
		return internalError(err)
	}
	if err := a.db.QueryRowContext(r.Context(),
		`SELECT COUNT(*) FROM items WHERE hidden = 1`).Scan(&stats.Hidden); err != nil {
		return internalError(err)
	}
	writeJSON(w, http.StatusOK, stats)
	return nil
}

func ftsPhrase(query string) string {
	return `"` + strings.ReplaceAll(query, `"`, `""`) + `"`
}
