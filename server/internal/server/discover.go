package server

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/mmcdole/gofeed"
)

type discoverRequest struct {
	URL string `json:"url"`
}

func (a *App) handleDiscoverFeed(w http.ResponseWriter, r *http.Request) error {
	var body discoverRequest
	if err := decodeJSON(r, &body); err != nil {
		return err
	}
	raw := strings.TrimSpace(body.URL)
	if raw == "" {
		return badRequest("url_required", "请输入站点地址或订阅地址")
	}
	raw = normalizeRSSHubURL(raw)
	if !strings.HasPrefix(strings.ToLower(raw), "http://") && !strings.HasPrefix(strings.ToLower(raw), "https://") {
		raw = "https://" + raw
	}
	if err := a.fetcher.ValidateURL(r.Context(), raw); err != nil {
		return badRequest("url_blocked", err.Error())
	}
	headers := http.Header{}
	headers.Set("Accept", "text/html, application/xhtml+xml, application/rss+xml, application/atom+xml, application/feed+json, application/json;q=0.8, */*;q=0.5")
	response, err := a.fetcher.GetBytes(r.Context(), raw, 5<<20, headers)
	if err != nil {
		return &apiError{Status: http.StatusBadGateway, Code: "discover_failed", Message: "无法访问该地址：" + err.Error()}
	}
	finalURL := response.FinalURL
	if finalURL == "" {
		finalURL = raw
	}
	contentType := strings.ToLower(response.ContentType)
	if detected := detectFeedFormat(response.Body, contentType); detected != "" {
		return writeDiscoveredFeed(w, response.Body, finalURL, detected)
	}
	if !strings.Contains(contentType, "html") && !looksLikeHTML(response.Body) {
		return badRequest("discover_failed", "该地址不是网页或 Feed")
	}
	document, err := goquery.NewDocumentFromReader(strings.NewReader(string(response.Body)))
	if err != nil {
		return badRequest("discover_failed", "网页解析失败")
	}
	pageTitle := strings.TrimSpace(document.Find("title").First().Text())
	base := finalURL
	if href, ok := document.Find("base[href]").First().Attr("href"); ok {
		if parsed, err := url.Parse(strings.TrimSpace(href)); err == nil {
			if resolved, err := url.Parse(base); err == nil {
				base = resolved.ResolveReference(parsed).String()
			}
		}
	}
	candidates := make([]FeedCandidate, 0)
	seen := map[string]struct{}{}
	document.Find(`link[rel~="alternate"]`).Each(func(_ int, selection *goquery.Selection) {
		href, ok := selection.Attr("href")
		if !ok || strings.TrimSpace(href) == "" {
			return
		}
		mimeType, _ := selection.Attr("type")
		format := feedFormatFromType(mimeType)
		if format == "" {
			return
		}
		absolute := resolveCandidateURL(base, href)
		if absolute == "" {
			return
		}
		if _, exists := seen[absolute]; exists {
			return
		}
		seen[absolute] = struct{}{}
		title, _ := selection.Attr("title")
		title = strings.TrimSpace(title)
		if title == "" {
			title = pageTitle
		}
		if title == "" {
			title = hostOf(absolute)
		}
		candidates = append(candidates, FeedCandidate{Title: title, URL: absolute, Format: format})
	})
	if len(candidates) == 0 {
		// A homepage often has conventional feed paths even when it omits
		// <link rel=alternate>. Return no candidates rather than inventing URLs.
		writeJSON(w, http.StatusOK, []FeedCandidate{})
		return nil
	}
	writeJSON(w, http.StatusOK, candidates)
	return nil
}

func writeDiscoveredFeed(w http.ResponseWriter, body []byte, rawURL, format string) error {
	title := hostOf(rawURL)
	parser := gofeed.NewParser()
	if parsed, err := parser.ParseString(string(body)); err == nil && strings.TrimSpace(parsed.Title) != "" {
		title = strings.TrimSpace(parsed.Title)
	}
	writeJSON(w, http.StatusOK, []FeedCandidate{{Title: title, URL: rawURL, Format: format}})
	return nil
}

func detectFeedFormat(body []byte, contentType string) string {
	if format := feedFormatFromType(contentType); format != "" {
		return format
	}
	trimmed := strings.TrimSpace(string(body))
	if strings.HasPrefix(trimmed, "{") {
		var doc struct {
			Version string `json:"version"`
		}
		if json.Unmarshal([]byte(trimmed), &doc) == nil && strings.Contains(doc.Version, "jsonfeed.org") {
			return "JSON Feed"
		}
	}
	lower := strings.ToLower(trimmed)
	if strings.Contains(lower, "<rss") || strings.Contains(lower, "<rdf:rdf") {
		return "RSS 2.0"
	}
	if strings.Contains(lower, "<feed") && strings.Contains(lower, "www.w3.org/2005/atom") {
		return "Atom 1.0"
	}
	return ""
}

func feedFormatFromType(contentType string) string {
	value := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	switch {
	case strings.Contains(value, "rss"), strings.Contains(value, "rdf"):
		return "RSS 2.0"
	case strings.Contains(value, "atom"):
		return "Atom 1.0"
	case strings.Contains(value, "feed+json"):
		return "JSON Feed"
	default:
		return ""
	}
}

func resolveCandidateURL(base, href string) string {
	parsedBase, err := url.Parse(base)
	if err != nil {
		return ""
	}
	parsedHref, err := url.Parse(strings.TrimSpace(href))
	if err != nil {
		return ""
	}
	resolved := parsedBase.ResolveReference(parsedHref)
	if resolved.Scheme != "http" && resolved.Scheme != "https" {
		return ""
	}
	return resolved.String()
}

func hostOf(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return raw
	}
	return parsed.Host
}

func looksLikeHTML(body []byte) bool {
	value := strings.ToLower(strings.TrimSpace(string(body)))
	return strings.HasPrefix(value, "<!doctype html") || strings.HasPrefix(value, "<html") || strings.Contains(value, "<head")
}
