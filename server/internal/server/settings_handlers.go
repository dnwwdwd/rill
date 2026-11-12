package server

import (
	"database/sql"
	"io"
	"net/http"
	"time"
)

func (a *App) handleGetSettings(w http.ResponseWriter, r *http.Request) error {
	session, _ := sessionFromContext(r.Context())
	var expires *string
	if session != nil {
		expires = formatTimePtr(session.ExpiresAt)
	}
	settings, err := a.apiSettings(r.Context(), expires)
	if err != nil {
		return internalError(err)
	}
	writeJSON(w, http.StatusOK, settings)
	return nil
}

func (a *App) handlePatchSettings(w http.ResponseWriter, r *http.Request) error {
	body, err := io.ReadAll(io.LimitReader(r.Body, (1<<20)+1))
	if err != nil {
		return badRequest("invalid_json", "读取设置内容失败")
	}
	if len(body) > 1<<20 {
		return badRequest("invalid_json", "设置内容过大")
	}
	session, _ := sessionFromContext(r.Context())
	var expires *string
	if session != nil {
		expires = formatTimePtr(session.ExpiresAt)
	}
	settings, err := a.updateSettings(r.Context(), body, expires)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, settings)
	return nil
}

func (a *App) handleDetailedHealth(w http.ResponseWriter, r *http.Request) error {
	var dbWritable bool
	if err := a.db.QueryRowContext(r.Context(), `SELECT 1`).Scan(new(int)); err == nil {
		dbWritable = true
	}
	var migrationVersion int
	_ = a.db.QueryRowContext(r.Context(), `PRAGMA user_version`).Scan(&migrationVersion)
	var dueCount, feedCount, errorCount int
	_ = a.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM sources WHERE enabled = 1 AND fail_count < 5 AND next_fetch_at <= ?`,
		a.now().Format(timeRFC3339)).Scan(&dueCount)
	_ = a.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM sources`).Scan(&feedCount)
	_ = a.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM sources WHERE last_status = 'error'`).Scan(&errorCount)
	var lastSuccess sql.NullString
	_ = a.db.QueryRowContext(r.Context(), `SELECT MAX(started_at) FROM fetch_logs WHERE result IN ('ok', 'not_modified')`).Scan(&lastSuccess)
	a.schedMu.RLock()
	lastTick := a.schedLastTick
	a.schedMu.RUnlock()
	payload := map[string]any{
		"ok":                     dbWritable,
		"db_writable":            dbWritable,
		"migration_version":      migrationVersion,
		"due_count":              dueCount,
		"feed_count":             feedCount,
		"error_feed_count":       errorCount,
		"last_fetch_success_at":  nullStringPtr(lastSuccess),
		"scheduler_last_tick_at": nil,
	}
	if !lastTick.IsZero() {
		payload["scheduler_last_tick_at"] = lastTick.UTC().Format(time.RFC3339)
	}
	writeJSON(w, http.StatusOK, payload)
	return nil
}
