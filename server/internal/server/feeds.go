package server

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const feedItemLimit = 50

type feedItemData struct {
	ID          int64
	SourceID    int64
	SourceName  string
	GUID        string
	Title       string
	URL         string
	Author      string
	ContentHTML string
	AudioURL    string
	AudioType   string
	PublishedAt time.Time
	FetchedAt   time.Time
}

func (a *App) handleSourceFeed(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		http.NotFound(w, r)
		return nil
	}
	format, token, ok := splitFeedFile(r.PathValue("file"))
	if !ok {
		http.NotFound(w, r)
		return nil
	}
	source, err := a.getSourceRow(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return nil
	}
	if err != nil {
		http.Error(w, "feed unavailable", http.StatusInternalServerError)
		return nil
	}
	currentToken, err := a.sourceFeedToken(r.Context(), id)
	if err != nil || token == "" || !constantTimeEqual(token, currentToken) {
		http.NotFound(w, r)
		return nil
	}
	items, err := a.feedItems(r.Context(), id)
	if err != nil {
		http.Error(w, "feed unavailable", http.StatusInternalServerError)
		return nil
	}
	origin := requestOrigin(r)
	meta := feedMeta{
		Title:       source.Name,
		Link:        firstNonEmpty(deref(source.SiteURL), source.FeedURL),
		SelfURL:     origin + sourceFeedPath(id, token, format),
		Description: "Rill subscription output for " + source.Name,
		Updated:     latestItemTime(items),
	}
	data, contentType, err := renderFeed(format, meta, items, origin)
	if err != nil {
		http.Error(w, "feed unavailable", http.StatusInternalServerError)
		return nil
	}
	a.writeFeedResponse(w, r, data, contentType, meta.Updated)
	return nil
}

func (a *App) handleAggregateFeed(w http.ResponseWriter, r *http.Request) error {
	file := r.PathValue("file")
	format, token, ok := splitFeedFile(file)
	if !ok || format != "xml" {
		http.NotFound(w, r)
		return nil
	}
	globalToken, err := a.globalFeedToken(r.Context())
	if err != nil || token == "" || !constantTimeEqual(token, globalToken) {
		http.NotFound(w, r)
		return nil
	}
	items, err := a.feedItems(r.Context(), 0)
	if err != nil {
		http.Error(w, "feed unavailable", http.StatusInternalServerError)
		return nil
	}
	origin := requestOrigin(r)
	meta := feedMeta{
		Title:       "Rill · 全部订阅",
		Link:        origin,
		SelfURL:     origin + "/feed/all/" + token + ".xml",
		Description: "All Rill subscriptions",
		Updated:     latestItemTime(items),
	}
	data, contentType, err := renderFeed("xml", meta, items, origin)
	if err != nil {
		http.Error(w, "feed unavailable", http.StatusInternalServerError)
		return nil
	}
	a.writeFeedResponse(w, r, data, contentType, meta.Updated)
	return nil
}

func splitFeedFile(file string) (format, token string, ok bool) {
	for _, candidate := range []struct {
		suffix string
		format string
	}{{".xml", "xml"}, {".atom", "atom"}, {".json", "json"}} {
		if strings.HasSuffix(file, candidate.suffix) {
			token = strings.TrimSuffix(file, candidate.suffix)
			if token == "" || strings.Contains(token, "/") {
				return "", "", false
			}
			return candidate.format, token, true
		}
	}
	return "", "", false
}

func (a *App) writeFeedResponse(w http.ResponseWriter, r *http.Request, data []byte, contentType string, updated time.Time) {
	sum := sha256.Sum256(data)
	etag := `"` + hex.EncodeToString(sum[:]) + `"`
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=60")
	w.Header().Set("ETag", etag)
	if !updated.IsZero() {
		w.Header().Set("Last-Modified", updated.UTC().Format(http.TimeFormat))
	}
	if match := r.Header.Get("If-None-Match"); match != "" {
		if strings.Contains(match, etag) {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	} else if modified := r.Header.Get("If-Modified-Since"); modified != "" && !updated.IsZero() {
		if since, err := http.ParseTime(modified); err == nil && !updated.Truncate(time.Second).After(since) {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_, _ = w.Write(data)
	}
}

func (a *App) feedItems(ctx context.Context, sourceID int64) ([]feedItemData, error) {
	query := `
		SELECT i.id, i.source_id, s.name, i.guid, COALESCE(i.title, ''), COALESCE(i.url, ''),
		       COALESCE(i.author, ''), COALESCE(i.content_html, ''),
		       COALESCE(i.published_at, i.fetched_at), i.fetched_at, COALESCE(i.media_json, '')
		FROM items i JOIN sources s ON s.id = i.source_id`
	args := make([]any, 0, 2)
	if sourceID > 0 {
		query += ` WHERE i.source_id = ?`
		args = append(args, sourceID)
	}
	query += ` ORDER BY i.published_estimated ASC, COALESCE(i.published_at, i.fetched_at) DESC, i.id DESC LIMIT ?`
	args = append(args, feedItemLimit)
	rows, err := a.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]feedItemData, 0)
	for rows.Next() {
		var item feedItemData
		var published, fetched, mediaRaw string
		if err := rows.Scan(&item.ID, &item.SourceID, &item.SourceName, &item.GUID, &item.Title,
			&item.URL, &item.Author, &item.ContentHTML, &published, &fetched, &mediaRaw); err != nil {
			return nil, err
		}
		media := decodeMediaPayload(mediaRaw)
		item.AudioURL = media.AudioURL
		item.AudioType = media.AudioType
		item.PublishedAt, _ = time.Parse(time.RFC3339, published)
		item.FetchedAt, _ = time.Parse(time.RFC3339, fetched)
		if item.PublishedAt.IsZero() {
			item.PublishedAt = item.FetchedAt
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

type feedMeta struct {
	Title       string
	Link        string
	SelfURL     string
	Description string
	Updated     time.Time
}

func latestItemTime(items []feedItemData) time.Time {
	var latest time.Time
	for _, item := range items {
		if item.PublishedAt.After(latest) {
			latest = item.PublishedAt
		}
	}
	if latest.IsZero() {
		latest = time.Now().UTC()
	}
	return latest
}

func renderFeed(format string, meta feedMeta, items []feedItemData, origin string) ([]byte, string, error) {
	switch format {
	case "xml":
		return []byte(renderRSS(meta, items, origin)), "application/rss+xml; charset=utf-8", nil
	case "atom":
		return []byte(renderAtom(meta, items, origin)), "application/atom+xml; charset=utf-8", nil
	case "json":
		data, err := renderJSONFeed(meta, items, origin)
		return data, "application/feed+json; charset=utf-8", err
	default:
		return nil, "", errors.New("unknown feed format")
	}
}

func renderRSS(meta feedMeta, items []feedItemData, origin string) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<rss version="2.0" xmlns:atom="http://www.w3.org/2005/Atom" xmlns:content="http://purl.org/rss/1.0/modules/content/" xmlns:dc="http://purl.org/dc/elements/1.1/">` + "\n<channel>\n")
	writeXMLText(&b, "title", meta.Title)
	writeXMLText(&b, "link", meta.Link)
	writeXMLText(&b, "description", meta.Description)
	b.WriteString(`<atom:link href="` + xmlEscape(meta.SelfURL) + `" rel="self" type="application/rss+xml"/>` + "\n")
	b.WriteString("<lastBuildDate>" + meta.Updated.UTC().Format(time.RFC1123Z) + "</lastBuildDate>\n")
	for _, item := range items {
		b.WriteString("<item>\n")
		writeXMLText(&b, "title", firstNonEmpty(item.Title, "(无标题)"))
		if item.URL != "" {
			writeXMLText(&b, "link", item.URL)
		}
		guid := "rill:" + strconv.FormatInt(item.SourceID, 10) + ":" + item.GUID
		b.WriteString(`<guid isPermaLink="false">` + xmlEscape(guid) + `</guid>` + "\n")
		if !item.PublishedAt.IsZero() {
			b.WriteString("<pubDate>" + item.PublishedAt.UTC().Format(time.RFC1123Z) + "</pubDate>\n")
		}
		if item.Author != "" {
			writeXMLText(&b, "dc:creator", item.Author)
		}
		if item.AudioURL != "" {
			b.WriteString(`<enclosure url="` + xmlEscape(item.AudioURL) + `" type="` + xmlEscape(firstNonEmpty(item.AudioType, "audio/mpeg")) + `" length="0"/>` + "\n")
		}
		content := absolutizeMedia(item.ContentHTML, origin)
		b.WriteString("<description><![CDATA[" + cdataSafe(content) + "]]></description>\n")
		b.WriteString("<content:encoded><![CDATA[" + cdataSafe(content) + "]]></content:encoded>\n")
		b.WriteString("</item>\n")
	}
	b.WriteString("</channel>\n</rss>\n")
	return b.String()
}

func renderAtom(meta feedMeta, items []feedItemData, origin string) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<feed xmlns="http://www.w3.org/2005/Atom">` + "\n")
	b.WriteString("<id>urn:rill:feed:" + xmlEscape(meta.SelfURL) + "</id>\n")
	writeXMLText(&b, "title", meta.Title)
	if meta.Link != "" {
		b.WriteString(`<link rel="alternate" href="` + xmlEscape(meta.Link) + `"/>` + "\n")
	}
	b.WriteString(`<link rel="self" href="` + xmlEscape(meta.SelfURL) + `" type="application/atom+xml"/>` + "\n")
	b.WriteString("<updated>" + meta.Updated.UTC().Format(time.RFC3339) + "</updated>\n")
	for _, item := range items {
		b.WriteString("<entry>\n")
		guid := "rill:" + strconv.FormatInt(item.SourceID, 10) + ":" + item.GUID
		b.WriteString("<id>" + xmlEscape(guid) + "</id>\n")
		writeXMLText(&b, "title", firstNonEmpty(item.Title, "(无标题)"))
		if item.URL != "" {
			b.WriteString(`<link rel="alternate" href="` + xmlEscape(item.URL) + `"/>` + "\n")
		}
		b.WriteString("<published>" + item.PublishedAt.UTC().Format(time.RFC3339) + "</published>\n")
		b.WriteString("<updated>" + item.PublishedAt.UTC().Format(time.RFC3339) + "</updated>\n")
		if item.Author != "" {
			b.WriteString("<author><name>" + xmlEscape(item.Author) + "</name></author>\n")
		}
		if item.AudioURL != "" {
			b.WriteString(`<link rel="enclosure" href="` + xmlEscape(item.AudioURL) + `" type="` + xmlEscape(firstNonEmpty(item.AudioType, "audio/mpeg")) + `"/>` + "\n")
		}
		b.WriteString(`<content type="html">` + xmlEscape(absolutizeMedia(item.ContentHTML, origin)) + "</content>\n")
		b.WriteString("</entry>\n")
	}
	b.WriteString("</feed>\n")
	return b.String()
}

type jsonFeedDocument struct {
	Version     string         `json:"version"`
	Title       string         `json:"title"`
	HomePageURL string         `json:"home_page_url,omitempty"`
	FeedURL     string         `json:"feed_url,omitempty"`
	Description string         `json:"description,omitempty"`
	Items       []jsonFeedItem `json:"items"`
}

type jsonFeedItem struct {
	ID            string           `json:"id"`
	URL           string           `json:"url,omitempty"`
	Title         string           `json:"title,omitempty"`
	ContentHTML   string           `json:"content_html,omitempty"`
	DatePublished string           `json:"date_published,omitempty"`
	Authors       []jsonAuthor     `json:"authors,omitempty"`
	Attachments   []jsonAttachment `json:"attachments,omitempty"`
}

type jsonAttachment struct {
	URL         string `json:"url"`
	MimeType    string `json:"mime_type"`
	SizeInBytes int64  `json:"size_in_bytes,omitempty"`
}

type jsonAuthor struct {
	Name string `json:"name"`
}

func renderJSONFeed(meta feedMeta, items []feedItemData, origin string) ([]byte, error) {
	document := jsonFeedDocument{
		Version:     "https://jsonfeed.org/version/1.1",
		Title:       meta.Title,
		HomePageURL: meta.Link,
		FeedURL:     meta.SelfURL,
		Description: meta.Description,
		Items:       make([]jsonFeedItem, 0, len(items)),
	}
	for _, item := range items {
		entry := jsonFeedItem{
			ID:            "rill:" + strconv.FormatInt(item.SourceID, 10) + ":" + item.GUID,
			URL:           item.URL,
			Title:         item.Title,
			ContentHTML:   absolutizeMedia(item.ContentHTML, origin),
			DatePublished: item.PublishedAt.UTC().Format(time.RFC3339),
		}
		if item.Author != "" {
			entry.Authors = []jsonAuthor{{Name: item.Author}}
		}
		if item.AudioURL != "" {
			entry.Attachments = []jsonAttachment{{
				URL:      item.AudioURL,
				MimeType: firstNonEmpty(item.AudioType, "audio/mpeg"),
			}}
		}
		document.Items = append(document.Items, entry)
	}
	return json.MarshalIndent(document, "", "  ")
}

func writeXMLText(b *strings.Builder, tag, value string) {
	b.WriteString("<" + tag + ">" + xmlEscape(value) + "</" + tag + ">\n")
}

func xmlEscape(value string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&apos;",
	)
	return replacer.Replace(value)
}

func cdataSafe(value string) string {
	return strings.ReplaceAll(value, "]]>", "]]]]><![CDATA[>")
}

func absolutizeMedia(content, origin string) string {
	if origin == "" {
		return content
	}
	content = strings.ReplaceAll(content, `src="/media/`, `src="`+origin+`/media/`)
	content = strings.ReplaceAll(content, `href="/media/`, `href="`+origin+`/media/`)
	return content
}

func requestOrigin(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if forwarded := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Proto"), ",")[0]); forwarded == "http" || forwarded == "https" {
		scheme = forwarded
	}
	return scheme + "://" + r.Host
}

func deref(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
