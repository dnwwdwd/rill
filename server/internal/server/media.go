package server

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"rill/internal/fetch"
)

type mediaRow struct {
	SourceURL   string
	LocalPath   sql.NullString
	ContentType sql.NullString
	Status      string
	FailCount   int
	FetchedAt   sql.NullString
	LastAccess  string
}

func (a *App) handleMedia(w http.ResponseWriter, r *http.Request) error {
	token := r.PathValue("token")
	hash := strings.ToLower(strings.TrimSpace(r.PathValue("hash")))
	if token == "" || len(hash) != 64 || !isHex(hash) {
		http.NotFound(w, r)
		return nil
	}
	if err := a.validateMediaToken(r.Context(), token); err != nil {
		http.NotFound(w, r)
		return nil
	}
	return a.serveMediaByHash(w, r, hash)
}

// handleCandidateIcon serves candidate avatars (creator search / resolve
// results) through the same media cache. Only URLs that resolve/search
// registered beforehand can be downloaded: an unknown hash is a 404, never a
// fetch. This keeps sinaimg.cn and other hotlink-blocked CDNs off the browser.
func (a *App) handleCandidateIcon(w http.ResponseWriter, r *http.Request) error {
	hash := strings.ToLower(strings.TrimSpace(r.PathValue("hash")))
	if len(hash) != 64 || !isHex(hash) {
		http.NotFound(w, r)
		return nil
	}
	if _, err := a.mediaCacheRow(r.Context(), hash); err != nil {
		http.NotFound(w, r)
		return nil
	}
	return a.serveMediaByHash(w, r, hash)
}

func (a *App) serveMediaByHash(w http.ResponseWriter, r *http.Request, hash string) error {
	row, err := a.mediaCacheRow(r.Context(), hash)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return nil
	}
	if err != nil {
		http.Error(w, "media unavailable", http.StatusInternalServerError)
		return nil
	}
	if row.Status == "ok" && row.LocalPath.Valid {
		if file, ok := a.openCachedMedia(row.LocalPath.String); ok {
			defer file.Close()
			a.serveMediaFile(w, r, file, row.ContentType.String, hash)
			_, _ = a.db.ExecContext(r.Context(), `UPDATE media_cache SET last_access_at = ? WHERE url_hash = ?`,
				a.now().Format(timeRFC3339), hash)
			return nil
		}
	}
	if row.Status == "failed" && row.FailCount >= 3 {
		if last, err := time.Parse(time.RFC3339, row.LastAccess); err == nil && a.now().Sub(last) < 10*time.Minute {
			http.NotFound(w, r)
			return nil
		}
	}
	_, err, _ = a.mediaGroup.Do(hash, func() (any, error) {
		return nil, a.downloadMedia(r.Context(), hash, "")
	})
	if err != nil {
		http.NotFound(w, r)
		return nil
	}
	row, err = a.mediaCacheRow(r.Context(), hash)
	if err != nil || row.Status != "ok" || !row.LocalPath.Valid {
		http.NotFound(w, r)
		return nil
	}
	file, ok := a.openCachedMedia(row.LocalPath.String)
	if !ok {
		http.NotFound(w, r)
		return nil
	}
	defer file.Close()
	a.serveMediaFile(w, r, file, row.ContentType.String, hash)
	_, _ = a.db.ExecContext(r.Context(), `UPDATE media_cache SET last_access_at = ? WHERE url_hash = ?`,
		a.now().Format(timeRFC3339), hash)
	return nil
}

func (a *App) validateMediaToken(ctx context.Context, token string) error {
	var stored string
	err := a.db.QueryRowContext(ctx, `SELECT feed_token FROM sources WHERE feed_token = ?`, token).Scan(&stored)
	if err == nil && constantTimeEqual(stored, token) {
		return nil
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	globalToken, globalErr := a.globalFeedToken(ctx)
	if globalErr == nil && token != "" && constantTimeEqual(token, globalToken) {
		return nil
	}
	return sql.ErrNoRows
}

func (a *App) mediaCacheRow(ctx context.Context, hash string) (mediaRow, error) {
	var row mediaRow
	err := a.db.QueryRowContext(ctx, `
		SELECT source_url, local_path, content_type, status, fail_count, fetched_at, last_access_at
		FROM media_cache WHERE url_hash = ?`, hash).Scan(
		&row.SourceURL, &row.LocalPath, &row.ContentType, &row.Status, &row.FailCount, &row.FetchedAt, &row.LastAccess)
	return row, err
}

func (a *App) downloadMedia(ctx context.Context, hash, token string) error {
	row, err := a.mediaCacheRow(ctx, hash)
	if err != nil {
		return err
	}
	if err := a.fetcher.ValidateURL(ctx, row.SourceURL); err != nil {
		a.markMediaFailed(ctx, hash)
		return err
	}
	response, err := a.fetcher.GetBytes(ctx, row.SourceURL, 20<<20, nil)
	if err != nil {
		a.markMediaFailed(ctx, hash)
		return err
	}
	contentType := strings.ToLower(strings.TrimSpace(strings.Split(response.ContentType, ";")[0]))
	if contentType == "" {
		contentType = http.DetectContentType(response.Body)
	}
	if !strings.HasPrefix(contentType, "image/") || strings.Contains(contentType, "svg") {
		a.markMediaFailed(ctx, hash)
		return errors.New("media is not a supported image")
	}
	if len(response.Body) > 20<<20 {
		a.markMediaFailed(ctx, hash)
		return errors.New("media too large")
	}
	mediaDir := filepath.Join(a.cfg.CacheDir, "media")
	if !pathWithin(a.cfg.CacheDir, mediaDir) {
		return errors.New("cache path escapes cache directory")
	}
	if err := os.MkdirAll(mediaDir, 0o750); err != nil {
		return err
	}
	finalPath := filepath.Join(mediaDir, hash)
	temp, err := os.CreateTemp(mediaDir, hash+".tmp-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	if _, err := temp.Write(response.Body); err != nil {
		_ = temp.Close()
		_ = os.Remove(tempPath)
		return err
	}
	if err := temp.Close(); err != nil {
		_ = os.Remove(tempPath)
		return err
	}
	if err := os.Chmod(tempPath, 0o640); err != nil {
		_ = os.Remove(tempPath)
		return err
	}
	if err := os.Rename(tempPath, finalPath); err != nil {
		_ = os.Remove(tempPath)
		return err
	}
	_, err = a.db.ExecContext(ctx, `
		UPDATE media_cache SET local_path = ?, content_type = ?, size_bytes = ?, status = 'ok',
			fail_count = 0, fetched_at = ?, last_access_at = ?
		WHERE url_hash = ?`,
		finalPath, contentType, len(response.Body), a.now().Format(timeRFC3339),
		a.now().Format(timeRFC3339), hash)
	if err != nil {
		return err
	}
	if err := a.enforceMediaQuota(ctx); err != nil {
		a.logger.Warn("media quota cleanup failed", "err", err)
	}
	return nil
}

func (a *App) enforceMediaQuota(ctx context.Context) error {
	limit := a.cfg.MediaMaxBytes
	if limit <= 0 {
		return nil
	}
	for {
		var total int64
		if err := a.db.QueryRowContext(ctx, `
			SELECT COALESCE(SUM(size_bytes), 0) FROM media_cache WHERE status = 'ok'`).Scan(&total); err != nil {
			return err
		}
		if total <= limit {
			return nil
		}
		rows, err := a.db.QueryContext(ctx, `
			SELECT url_hash, local_path, COALESCE(size_bytes, 0)
			FROM media_cache
			WHERE status = 'ok' AND local_path IS NOT NULL
			ORDER BY last_access_at ASC, id ASC
			LIMIT 100`)
		if err != nil {
			return err
		}
		type candidate struct {
			hash string
			path string
			size int64
		}
		candidates := make([]candidate, 0, 100)
		for rows.Next() {
			var row candidate
			if err := rows.Scan(&row.hash, &row.path, &row.size); err != nil {
				_ = rows.Close()
				return err
			}
			candidates = append(candidates, row)
		}
		if err := rows.Close(); err != nil {
			return err
		}
		if len(candidates) == 0 {
			return nil
		}
		for _, row := range candidates {
			if pathWithin(a.cfg.CacheDir, row.path) {
				_ = os.Remove(row.path)
			}
			if _, err := a.db.ExecContext(ctx, `
				UPDATE media_cache
				SET status = 'pending', local_path = NULL, content_type = NULL,
				    size_bytes = NULL, fail_count = 0, fetched_at = NULL, last_access_at = ?
				WHERE url_hash = ?`, a.now().Format(timeRFC3339), row.hash); err != nil {
				return err
			}
			total -= row.size
			if total <= limit {
				return nil
			}
		}
	}
}

func (a *App) markMediaFailed(ctx context.Context, hash string) {
	_, _ = a.db.ExecContext(ctx, `
		UPDATE media_cache SET status = 'failed', fail_count = fail_count + 1, last_access_at = ?
		WHERE url_hash = ?`, a.now().Format(timeRFC3339), hash)
}

func (a *App) openCachedMedia(path string) (*os.File, bool) {
	if !pathWithin(a.cfg.CacheDir, path) {
		return nil, false
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	return file, true
}

func (a *App) serveMediaFile(w http.ResponseWriter, r *http.Request, file *os.File, contentType, hash string) {
	info, err := file.Stat()
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("ETag", `"`+hash+`"`)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, "", info.ModTime(), file)
}

func isHex(value string) bool {
	for _, r := range value {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') {
			continue
		}
		return false
	}
	return true
}

// prepareCandidateIcons registers candidate avatars (creator search / resolve
// results) in the media cache and fills IconProxyURL. Browsers then load
// avatars same-origin through /api/v1/sources/icon-proxy/{hash} instead of
// hitting hotlink-protected CDNs directly (sinaimg.cn returns 403 for foreign
// Referers). icon_url keeps the original remote URL and remains the value
// persisted when the source is created.
func (a *App) prepareCandidateIcons(ctx context.Context, iconURLs []string) error {
	now := a.now().Format(timeRFC3339)
	for _, raw := range iconURLs {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		parsed, err := url.Parse(raw)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			continue
		}
		if _, err := a.db.ExecContext(ctx, `
			INSERT INTO media_cache (url_hash, source_url, status, fail_count, last_access_at)
			VALUES (?, ?, 'pending', 0, ?)
			ON CONFLICT(url_hash) DO NOTHING`,
			fetch.URLHash(raw), raw, now); err != nil {
			return err
		}
	}
	return nil
}

func candidateIconProxyURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return ""
	}
	return sourceIconProxyPath(fetch.URLHash(raw))
}

func sourceIconProxyPath(hash string) string {
	return "/api/v1/media/candidate/" + strings.ToLower(hash)
}
