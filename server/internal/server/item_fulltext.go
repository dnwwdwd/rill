package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"rill/internal/fetch"
)

// REQ-039（2026-11-12）：当前文章的按需全文抓取。
// 与源级 full_text 自动抓取不同，这里由用户在正文工具栏显式触发一次。
const (
	itemFullTextTimeout  = 15 * time.Second
	itemFullTextMinRunes = 60
)

func (a *App) handleFetchItemFullText(w http.ResponseWriter, r *http.Request) error {
	id, err := apiRouteID(r)
	if err != nil {
		return err
	}
	detail, err := a.getItemDetail(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		return notFound("item_not_found", "条目不存在")
	}
	if err != nil {
		return internalError(err)
	}
	itemURL := stringValue(detail.URL)
	if !isHTTPItemURL(itemURL) {
		return badRequest("fulltext_not_supported", "该条目没有可抓取的原文地址")
	}
	source, err := a.getSourceRow(r.Context(), detail.SourceID)
	if errors.Is(err, sql.ErrNoRows) {
		return notFound("source_not_found", "订阅源不存在")
	}
	if err != nil {
		return internalError(err)
	}
	if source.Kind != "rss" {
		return badRequest("fulltext_not_supported", "只有 RSS 源支持全文抓取")
	}

	ctx, cancel := context.WithTimeout(r.Context(), itemFullTextTimeout)
	defer cancel()
	headers := http.Header{}
	headers.Set("Accept", "text/html,application/xhtml+xml;q=0.9,*/*;q=0.5")
	response, err := a.fetcher.GetBytes(ctx, itemURL, fullTextMaxBody, headers)
	if err != nil {
		return &apiError{
			Status:  http.StatusBadGateway,
			Code:    "fulltext_failed",
			Message: "抓取原文失败：" + readableFetchError(err, source.Kind),
			Err:     err,
		}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return &apiError{
			Status:  http.StatusBadGateway,
			Code:    "fulltext_failed",
			Message: "原文页面返回 HTTP " + http.StatusText(response.StatusCode),
		}
	}
	if response.ContentType != "" && !strings.Contains(strings.ToLower(response.ContentType), "html") {
		return badRequest("fulltext_failed", "原文地址不是 HTML 页面")
	}
	extracted := extractArticleHTML(response.Body)
	if strings.TrimSpace(extracted) == "" {
		return badRequest("fulltext_empty", "未能从原文页面提取正文，已保留 Feed 摘要")
	}
	base, err := url.Parse(response.FinalURL)
	if err != nil || base.Host == "" {
		base, _ = url.Parse(itemURL)
	}
	if base == nil {
		return badRequest("fulltext_failed", "原文地址无效")
	}
	html, text, mediaURLs := fetch.SanitizeContent(extracted, itemURL, base, source.FeedToken)
	text = strings.Join(strings.Fields(text), " ")
	if utf8.RuneCountInString(text) < itemFullTextMinRunes {
		return badRequest("fulltext_empty", "提取到的正文过短，已保留 Feed 摘要")
	}

	var mediaRaw sql.NullString
	if err := a.db.QueryRowContext(r.Context(), `SELECT COALESCE(media_json, '') FROM items WHERE id = ?`, id).Scan(&mediaRaw); err != nil {
		return internalError(err)
	}
	payload := decodeMediaPayload(mediaRaw.String)
	payload.Images = uniqueMediaURLs(append(payload.Images, mediaURLs...))
	mediaJSON, err := json.Marshal(payload)
	if err != nil {
		return internalError(err)
	}
	if _, err := a.db.ExecContext(r.Context(), `
		UPDATE items SET content_html = ?, content_text = ?, media_json = ?, updated_at = ? WHERE id = ?`,
		html, text, string(mediaJSON), a.now().Format(timeRFC3339), id); err != nil {
		return internalError(err)
	}
	for _, mediaURL := range mediaURLs {
		if _, err := a.db.ExecContext(r.Context(), `
			INSERT INTO media_cache (url_hash, source_url, status, fail_count, last_access_at)
			VALUES (?, ?, 'pending', 0, ?)
			ON CONFLICT(url_hash) DO NOTHING`,
			fetch.URLHash(mediaURL), mediaURL, a.now().Format(timeRFC3339)); err != nil {
			return internalError(err)
		}
	}
	updated, err := a.getItemDetail(r.Context(), id)
	if err != nil {
		return internalError(err)
	}
	writeJSON(w, http.StatusOK, updated)
	return nil
}

func uniqueMediaURLs(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
