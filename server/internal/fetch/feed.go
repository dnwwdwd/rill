package fetch

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/PuerkitoBio/goquery"
	"github.com/microcosm-cc/bluemonday"
	"github.com/mmcdole/gofeed"
	"golang.org/x/net/html/charset"
)

// FeedItem is the normalized, storage-ready representation of one entry.
type FeedItem struct {
	GUID               string
	URL                string
	Title              string
	Author             string
	PublishedAt        time.Time
	PublishedRaw       string
	PublishedEstimated bool
	ContentHTML        string
	ContentText        string
	ContentHash        string
	MediaURLs          []string
	AudioURL           string
	AudioType          string
}

type FeedResult struct {
	Title        string
	Link         string
	IconURL      string
	Items        []FeedItem
	ETag         string
	LastModified string
	NotModified  bool
	HTTPStatus   int
}

type FetchState struct {
	ETag         string
	LastModified string
}

// FetchFeed performs a conditional GET and parses RSS/Atom/JSON Feed with
// gofeed. The body is read before parsing so size and timeout rules apply.
func (c *Client) FetchFeed(ctx context.Context, rawURL string, state FetchState, sourceToken string) (*FeedResult, error) {
	headers := http.Header{}
	headers.Set("Accept", "application/rss+xml, application/atom+xml, application/feed+json, application/json;q=0.9, application/xml;q=0.8, text/xml;q=0.7, */*;q=0.5")
	if state.ETag != "" {
		headers.Set("If-None-Match", state.ETag)
	}
	if state.LastModified != "" {
		headers.Set("If-Modified-Since", state.LastModified)
	}
	response, err := c.GetBytes(ctx, rawURL, DefaultMaxBody, headers)
	if err != nil {
		return nil, err
	}
	if response.StatusCode == http.StatusNotModified {
		return &FeedResult{NotModified: true, HTTPStatus: response.StatusCode, ETag: state.ETag, LastModified: state.LastModified}, nil
	}
	if !isFeedContentType(response.ContentType) {
		return nil, &ResponseError{Status: response.StatusCode, Message: fmt.Sprintf("响应 Content-Type 不是 Feed：%s", shortContentType(response.ContentType))}
	}
	body := response.Body
	decoded, err := decodeCharset(body, response.ContentType)
	if err != nil {
		return nil, &ResponseError{Status: response.StatusCode, Message: fmt.Sprintf("字符集转换失败：%v", err)}
	}
	parser := gofeed.NewParser()
	parsed, err := parser.ParseString(string(decoded))
	if err != nil {
		return nil, &ResponseError{Status: response.StatusCode, Message: fmt.Sprintf("Feed 解析失败：%s", sanitizeParserError(err))}
	}
	if parsed == nil {
		return nil, &ResponseError{Status: response.StatusCode, Message: "Feed 解析结果为空"}
	}
	feedLink := strings.TrimSpace(parsed.Link)
	if feedLink == "" {
		feedLink = rawURL
	}
	base := baseURL(feedLink, response.FinalURL)
	iconURL := ""
	if parsed.Image != nil {
		iconURL = resolveURL(base, strings.TrimSpace(parsed.Image.URL))
	}
	result := &FeedResult{
		Title:        strings.TrimSpace(parsed.Title),
		Link:         feedLink,
		IconURL:      iconURL,
		ETag:         response.ETag,
		LastModified: response.LastModified,
		HTTPStatus:   response.StatusCode,
	}
	for _, entry := range parsed.Items {
		if entry == nil {
			continue
		}
		item, err := normalizeItem(entry, base, sourceToken)
		if err != nil {
			// A single malformed item must not abort the feed.
			continue
		}
		result.Items = append(result.Items, item)
	}
	// The feed order is not promised to be newest-first. Keep the newest
	// initial_limit entries when the caller applies its first-fetch policy.
	sort.SliceStable(result.Items, func(i, j int) bool {
		return result.Items[i].PublishedAt.After(result.Items[j].PublishedAt)
	})
	return result, nil
}

func isFeedContentType(contentType string) bool {
	if strings.TrimSpace(contentType) == "" {
		return true // some feeds omit Content-Type; parsing still decides.
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		mediaType = strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	}
	mediaType = strings.ToLower(mediaType)
	if mediaType == "text/html" || mediaType == "application/xhtml+xml" {
		return false
	}
	if strings.Contains(mediaType, "rss") || strings.Contains(mediaType, "atom") || strings.Contains(mediaType, "feed") {
		return true
	}
	switch mediaType {
	case "application/xml", "text/xml", "application/json":
		return true
	}
	return strings.HasSuffix(mediaType, "+xml") || strings.HasSuffix(mediaType, "+json")
}

func shortContentType(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "（空）"
	}
	if len(value) > 80 {
		return value[:80]
	}
	return value
}

var xmlEncodingPattern = regexp.MustCompile(`(?i)<\?xml[^>]*encoding\s*=\s*["']([^"']+)`)

func decodeCharset(body []byte, contentType string) ([]byte, error) {
	if utf8.Valid(body) {
		return body, nil
	}
	label := ""
	if match := xmlEncodingPattern.FindSubmatch(body); len(match) == 2 {
		label = string(match[1])
	}
	if label == "" {
		_, name, _ := charset.DetermineEncoding(body, contentType)
		label = name
	}
	if label == "" {
		label = "utf-8"
	}
	reader, err := charset.NewReaderLabel(label, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	return io.ReadAll(io.LimitReader(reader, DefaultMaxBody+1))
}

func sanitizeParserError(err error) string {
	text := strings.TrimSpace(err.Error())
	// Encoding/xml errors may quote a chunk of the body. Keep diagnostics short.
	if idx := strings.Index(text, "\n"); idx >= 0 {
		text = text[:idx]
	}
	if len(text) > 240 {
		text = text[:240] + "…"
	}
	return text
}

func baseURL(values ...string) *url.URL {
	for _, value := range values {
		if value == "" {
			continue
		}
		parsed, err := url.Parse(value)
		if err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") {
			return parsed
		}
	}
	return &url.URL{}
}

func normalizeItem(entry *gofeed.Item, base *url.URL, sourceToken string) (FeedItem, error) {
	now := time.Now().UTC()
	published := now
	estimated := true
	rawPublished := strings.TrimSpace(entry.Published)
	if entry.PublishedParsed != nil {
		published = entry.PublishedParsed.UTC()
		estimated = false
	} else if entry.UpdatedParsed != nil {
		published = entry.UpdatedParsed.UTC()
		estimated = false
	} else if rawPublished != "" {
		if parsed, ok := parseFeedDate(rawPublished); ok {
			published = parsed.UTC()
			estimated = false
		}
	}
	itemURL := strings.TrimSpace(entry.Link)
	if itemURL == "" && strings.HasPrefix(strings.TrimSpace(entry.GUID), "http") {
		itemURL = strings.TrimSpace(entry.GUID)
	}
	resolvedLink := resolveURL(base, itemURL)
	rawContent := strings.TrimSpace(entry.Content)
	if rawContent == "" {
		rawContent = strings.TrimSpace(entry.Description)
	}
	contentHTML, contentText, mediaURLs := SanitizeContent(rawContent, resolvedLink, base, sourceToken)
	if contentText == "" {
		contentText = strings.TrimSpace(stripHTMLFallback(entry.Description))
	}
	title := strings.TrimSpace(entry.Title)
	if title == "" {
		title = firstLine(contentText)
	}
	if title == "" {
		title = "(无标题)"
	}
	if len(title) > 500 {
		title = truncateRunes(title, 500)
	}
	author := ""
	if entry.Author != nil {
		author = strings.TrimSpace(entry.Author.Name)
	}
	if author == "" && len(entry.Authors) > 0 && entry.Authors[0] != nil {
		author = strings.TrimSpace(entry.Authors[0].Name)
	}
	// Enclosures and feed thumbnails are kept for the media cache even when
	// they are not repeated in the article body. Podcast audio is kept
	// separately for the reader player.
	audioURL := ""
	audioType := ""
	for _, enclosure := range entry.Enclosures {
		if enclosure == nil {
			continue
		}
		mediaURL := resolveURL(base, strings.TrimSpace(enclosure.URL))
		if !isHTTPURL(mediaURL) {
			continue
		}
		enclosureType := strings.ToLower(strings.TrimSpace(strings.Split(enclosure.Type, ";")[0]))
		if strings.HasPrefix(enclosureType, "audio/") || looksLikeAudioURL(mediaURL) {
			if audioURL == "" {
				audioURL = mediaURL
				audioType = enclosureType
				if !strings.HasPrefix(audioType, "audio/") {
					audioType = audioTypeForURL(mediaURL)
				}
			}
			continue
		}
		if strings.HasPrefix(enclosureType, "image/") || looksLikeImageURL(mediaURL) {
			mediaURLs = append(mediaURLs, mediaURL)
		}
	}
	if entry.Image != nil {
		mediaURL := resolveURL(base, strings.TrimSpace(entry.Image.URL))
		if isHTTPURL(mediaURL) {
			mediaURLs = append(mediaURLs, mediaURL)
		}
	}
	if len(mediaURLs) > 0 && !strings.Contains(contentHTML, "/media/") && contentHTML == "" {
		first := mediaURLs[0]
		contentHTML = fmt.Sprintf(`<p><img src="/media/%s/%s" alt=""></p>`, sourceToken, URLHash(first))
	}
	mediaURLs = uniqueStrings(mediaURLs)
	guid := strings.TrimSpace(entry.GUID)
	if guid == "" {
		guid = resolvedLink
	}
	if guid == "" {
		guid = hashString(title + "\n" + rawPublished + "\n" + contentText + "\n" + strings.Join(mediaURLs, "\n") + "\n" + audioURL)
	}
	hash := hashString(title + "\n" + rawPublished + "\n" + contentText + "\n" + strings.Join(mediaURLs, "\n") + "\n" + audioURL)
	if contentText == "" && contentHTML != "" {
		hash = hashString(title + "\n" + rawPublished + "\n" + contentHTML + "\n" + strings.Join(mediaURLs, "\n") + "\n" + audioURL)
	}
	return FeedItem{
		GUID:               guid,
		URL:                resolvedLink,
		Title:              title,
		Author:             author,
		PublishedAt:        published,
		PublishedRaw:       rawPublished,
		PublishedEstimated: estimated,
		ContentHTML:        contentHTML,
		ContentText:        contentText,
		ContentHash:        hash,
		MediaURLs:          mediaURLs,
		AudioURL:           audioURL,
		AudioType:          audioType,
	}, nil
}

func parseFeedDate(value string) (time.Time, bool) {
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		time.RFC1123Z,
		time.RFC1123,
		time.RFC822Z,
		time.RFC822,
		"Mon, 2 Jan 2006 15:04:05 -0700",
		"Mon, 2 Jan 2006 15:04:05 MST",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

// SanitizeContent removes scripts/event handlers and rewrites image URLs to
// the local media proxy. Relative links are resolved against the entry/page.
func SanitizeContent(raw, itemLink string, base *url.URL, sourceToken string) (string, string, []string) {
	if strings.TrimSpace(raw) == "" {
		return "", "", nil
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(raw))
	if err != nil {
		return "", stripHTMLFallback(raw), nil
	}
	doc.Find("script,style,noscript,iframe,object,embed,form,base,meta,link").Remove()
	media := make([]string, 0, 4)
	doc.Find("img").Each(func(_ int, selection *goquery.Selection) {
		src, ok := selection.Attr("src")
		if !ok || strings.TrimSpace(src) == "" {
			for _, attr := range []string{"data-src", "data-original", "data-lazy-src", "data-url"} {
				if value, exists := selection.Attr(attr); exists && strings.TrimSpace(value) != "" {
					src = value
					break
				}
			}
		}
		src = strings.TrimSpace(src)
		src = resolveURL(baseForItem(itemLink, base), src)
		if !isHTTPURL(src) {
			selection.Remove()
			return
		}
		if _, exists := selection.Attr("alt"); !exists {
			selection.SetAttr("alt", "")
		}
		selection.RemoveAttr("srcset")
		selection.RemoveAttr("data-src")
		selection.RemoveAttr("data-original")
		selection.RemoveAttr("data-lazy-src")
		selection.RemoveAttr("data-url")
		hash := URLHash(src)
		selection.SetAttr("src", fmt.Sprintf("/media/%s/%s", sourceToken, hash))
		media = append(media, src)
	})
	doc.Find("source,picture").Remove()
	doc.Find("a").Each(func(_ int, selection *goquery.Selection) {
		href, ok := selection.Attr("href")
		if !ok {
			return
		}
		href = strings.TrimSpace(href)
		if strings.HasPrefix(strings.ToLower(href), "javascript:") {
			selection.RemoveAttr("href")
			return
		}
		resolved := resolveURL(baseForItem(itemLink, base), href)
		if resolved != "" {
			selection.SetAttr("href", resolved)
		}
		selection.SetAttr("rel", "noopener noreferrer")
	})
	doc.Find("img").Each(func(_ int, selection *goquery.Selection) {
		selection.SetAttr("loading", "lazy")
		selection.SetAttr("decoding", "async")
	})
	htmlBody, err := doc.Find("body").Html()
	if err != nil {
		return "", stripHTMLFallback(raw), nil
	}
	policy := bluemonday.UGCPolicy()
	policy.AllowAttrs("loading", "decoding", "alt").OnElements("img")
	policy.AllowElements("figure", "figcaption")
	cleaned := strings.TrimSpace(policy.Sanitize(htmlBody))
	text := strings.TrimSpace(doc.Find("body").Text())
	text = collapseWhitespace(text)
	media = uniqueStrings(media)
	return cleaned, text, media
}

func baseForItem(itemLink string, base *url.URL) *url.URL {
	if parsed, err := url.Parse(itemLink); err == nil && parsed.IsAbs() {
		return parsed
	}
	return base
}

func resolveURL(base *url.URL, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return ""
	}
	if parsed.Scheme != "" && parsed.Scheme != "http" && parsed.Scheme != "https" {
		return ""
	}
	if base != nil && parsed.Scheme == "" {
		parsed = base.ResolveReference(parsed)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return ""
	}
	return parsed.String()
}

func isHTTPURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}

func looksLikeImageURL(value string) bool {
	parsed, err := url.Parse(value)
	if err != nil {
		return false
	}
	switch strings.ToLower(path.Ext(parsed.Path)) {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".avif", ".svg":
		return true
	}
	return false
}

func looksLikeAudioURL(value string) bool {
	parsed, err := url.Parse(value)
	if err != nil {
		return false
	}
	switch strings.ToLower(path.Ext(parsed.Path)) {
	case ".mp3", ".m4a", ".aac", ".ogg", ".oga", ".opus", ".wav", ".flac":
		return true
	}
	return false
}

func audioTypeForURL(value string) string {
	parsed, err := url.Parse(value)
	if err != nil {
		return "audio/mpeg"
	}
	switch strings.ToLower(path.Ext(parsed.Path)) {
	case ".m4a", ".aac":
		return "audio/mp4"
	case ".ogg", ".oga", ".opus":
		return "audio/ogg"
	case ".wav":
		return "audio/wav"
	case ".flac":
		return "audio/flac"
	default:
		return "audio/mpeg"
	}
}

func URLHash(rawURL string) string {
	sum := sha256.Sum256([]byte(rawURL))
	return hex.EncodeToString(sum[:])
}

func hashString(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func stripHTMLFallback(raw string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(raw))
	if err != nil {
		return collapseWhitespace(raw)
	}
	return collapseWhitespace(doc.Text())
}

func collapseWhitespace(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func firstLine(value string) string {
	value = strings.TrimSpace(value)
	if idx := strings.IndexAny(value, "\n\r"); idx >= 0 {
		value = value[:idx]
	}
	if len([]rune(value)) > 160 {
		value = truncateRunes(value, 160)
	}
	return strings.TrimSpace(value)
}

func truncateRunes(value string, max int) string {
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max]) + "…"
}
