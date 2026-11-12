package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"

	"rill/internal/fetch"
)

type resolveRequest struct {
	Input string `json:"input"`
}

type resolveResponse struct {
	Candidates []ResolveCandidate `json:"candidates"`
}

var (
	appleShowIDPattern  = regexp.MustCompile(`(?i)(?:^|[/?])id(\d{5,})`)
	bareDigitsPattern   = regexp.MustCompile(`^\d{5,12}$`)
	xiaoyuzhouIDPattern = regexp.MustCompile(`(?i)^[0-9a-f]{24}$`)
	weiboHostPattern    = regexp.MustCompile(`(?i)^(?:https?://)?(?:www\.|m\.)?(?:weibo\.com|weibo\.cn|t\.cn)(?:/|$)`)
	shareURLPattern     = regexp.MustCompile(`https?://[^\s<>"']+`)
	trailingPunct       = "，。！？、；：）】》」』”’.,!?;:)]}>"
)

// weiboRemovedError 统一提示：微博渠道已按用户决策移除（2026-11-08）。
func weiboRemovedError() error {
	return fmt.Errorf("微博渠道已移除；如需微博内容，请使用自建 RSSHub 等外部方案作为普通 RSS 添加")
}

func (a *App) handleResolveSource(w http.ResponseWriter, r *http.Request) error {
	var body resolveRequest
	if err := decodeJSON(r, &body); err != nil {
		return err
	}
	raw := strings.TrimSpace(body.Input)
	if raw == "" {
		return badRequest("input_required", "请输入链接或账号 ID")
	}
	candidates, recognized, err := a.resolveSourceInput(r.Context(), raw)
	if err != nil {
		if recognized {
			return &apiError{Status: http.StatusBadGateway, Code: "resolve_failed", Message: err.Error()}
		}
		return badRequest("invalid_input", err.Error())
	}
	if !recognized {
		writeJSON(w, http.StatusOK, resolveResponse{Candidates: []ResolveCandidate{}})
		return nil
	}
	if len(candidates) == 0 {
		return notFound("resolve_not_found", "没有找到可订阅的账号或播客")
	}
	iconURLs := make([]string, 0, len(candidates))
	for index := range candidates {
		if proxy := candidateIconProxyURL(candidates[index].IconURL); proxy != "" {
			candidates[index].IconProxyURL = proxy
			iconURLs = append(iconURLs, candidates[index].IconURL)
		}
	}
	if err := a.prepareCandidateIcons(r.Context(), iconURLs); err != nil {
		return internalError(err)
	}
	writeJSON(w, http.StatusOK, resolveResponse{Candidates: candidates})
	return nil
}

// resolveSourceInput turns a pasted link or ID into source candidates. A false
// recognized result means "not a platform input"; the caller should fall back
// to ordinary HTML feed discovery.
func (a *App) resolveSourceInput(ctx context.Context, raw string) ([]ResolveCandidate, bool, error) {
	value := extractFirstURL(strings.TrimSpace(raw))
	if value == "" {
		value = strings.TrimSpace(raw)
	}
	lower := strings.ToLower(value)
	if strings.HasPrefix(lower, "rsshub://") {
		return nil, false, nil
	}

	// BestBlogs is the preferred WeChat source and an optional discovery index
	// for its podcast/X/YouTube catalogs. The prefix searches all four; bare
	// names use the platform-specific priority below.
	if strings.HasPrefix(lower, "bestblogs:") {
		name := strings.TrimSpace(value[len("bestblogs:"):])
		if name == "" {
			return nil, true, fmt.Errorf("请输入 BestBlogs 目录名称")
		}
		candidates, _, err := a.resolveBestBlogsByName(ctx, name)
		if err != nil {
			return nil, true, err
		}
		if len(candidates) == 0 {
			return nil, true, fmt.Errorf("BestBlogs 目录中没有找到「%s」", name)
		}
		return candidates, true, nil
	}
	if candidates, ok := a.resolveBestBlogsFeed(ctx, value); ok {
		return candidates, true, nil
	}
	if a.shouldTryBestBlogsName(value) {
		candidates, matched, err := a.resolveBestBlogsByName(ctx, value)
		if err != nil {
			a.logger.Warn("bestblogs name lookup failed; falling back", "name", value, "err", err)
		} else if matched {
			return candidates, true, nil
		}
	}

	// Explicit platform prefixes remove ambiguity for numeric IDs.
	switch {
	case strings.HasPrefix(lower, "apple:"):
		id := strings.TrimSpace(value[len("apple:"):])
		if !bareDigitsPattern.MatchString(id) {
			return nil, true, fmt.Errorf("Apple Podcasts ID 应为 5–12 位数字")
		}
		candidates, err := a.resolveAppleByID(ctx, id)
		return candidates, true, err
	case strings.HasPrefix(lower, "weibo://user/"), strings.HasPrefix(lower, "weibo:"):
		return nil, true, weiboRemovedError()
	case strings.HasPrefix(lower, "xyz:"):
		return a.resolveXiaoyuzhouReference(ctx, strings.TrimSpace(value[len("xyz:"):]))
	}

	// Bare numbers are Apple Podcasts show IDs（微博渠道已移除，不再兜底解析）。
	if bareDigitsPattern.MatchString(value) {
		candidates, err := a.resolveAppleByID(ctx, value)
		if err == nil && len(candidates) > 0 {
			return candidates, true, nil
		}
		return nil, true, fmt.Errorf("没有找到对应的 Apple Podcasts 节目；可加 apple: 前缀指定平台")
	}

	if id, ok := appleIDFromInput(value); ok {
		candidates, err := a.resolveAppleByID(ctx, id)
		return candidates, true, err
	}
	if pid, eid, token, ok := xiaoyuzhouRefFromInput(value); ok {
		candidates, err := a.resolveXiaoyuzhouRef(ctx, pid, eid, token)
		return candidates, true, err
	}
	if weiboHostPattern.MatchString(value) {
		return nil, true, weiboRemovedError()
	}
	if feedURL, name, ok := mediumFeedURL(value); ok {
		return []ResolveCandidate{{
			Platform: "medium",
			Kind:     "rss",
			Name:     name,
			FeedURL:  feedURL,
			SiteURL:  value,
			Note:     "Medium RSS",
		}}, true, nil
	}
	if candidate, ok := a.resolveMastodon(ctx, value); ok {
		return []ResolveCandidate{candidate}, true, nil
	}
	return nil, false, nil
}

func extractFirstURL(raw string) string {
	match := shareURLPattern.FindString(raw)
	if match == "" {
		return ""
	}
	return strings.TrimRight(match, trailingPunct)
}

func normalizeInputURL(raw string) string {
	value := strings.TrimSpace(raw)
	if strings.Contains(value, "://") {
		return value
	}
	return "https://" + value
}

func appleIDFromInput(raw string) (string, bool) {
	value := strings.TrimSpace(raw)
	lower := strings.ToLower(value)
	if strings.HasPrefix(lower, "id") && bareDigitsPattern.MatchString(value[2:]) {
		return value[2:], true
	}
	if bareDigitsPattern.MatchString(value) {
		return value, true
	}
	parsed, err := url.Parse(normalizeInputURL(value))
	if err != nil || parsed.Hostname() == "" {
		return "", false
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "podcasts.apple.com" && host != "itunes.apple.com" {
		return "", false
	}
	if match := appleShowIDPattern.FindStringSubmatch(parsed.Path); len(match) == 2 {
		return match[1], true
	}
	if queryID := strings.TrimSpace(parsed.Query().Get("id")); bareDigitsPattern.MatchString(queryID) {
		return queryID, true
	}
	return "", false
}

type appleLookupResponse struct {
	ResultCount int `json:"resultCount"`
	Results     []struct {
		CollectionName    string `json:"collectionName"`
		FeedURL           string `json:"feedUrl"`
		CollectionViewURL string `json:"collectionViewUrl"`
		ArtworkURL600     string `json:"artworkUrl600"`
		ArtworkURL100     string `json:"artworkUrl100"`
	} `json:"results"`
}

func (a *App) resolveAppleByID(ctx context.Context, id string) ([]ResolveCandidate, error) {
	endpoint := a.appleAPIBase + "/lookup?id=" + url.QueryEscape(id) + "&entity=podcast"
	headers := http.Header{}
	headers.Set("Accept", "application/json")
	response, err := a.fetcher.GetBytes(ctx, endpoint, 2<<20, headers)
	if err != nil {
		return nil, fmt.Errorf("Apple Podcasts 目录查询失败：%v", err)
	}
	var payload appleLookupResponse
	if err := json.Unmarshal(response.Body, &payload); err != nil {
		return nil, fmt.Errorf("Apple Podcasts 返回的数据无法解析")
	}
	for _, result := range payload.Results {
		feedURL := strings.TrimSpace(result.FeedURL)
		if feedURL == "" {
			continue
		}
		if _, err := parseHTTPURL(feedURL); err != nil {
			continue
		}
		name := strings.TrimSpace(result.CollectionName)
		if name == "" {
			name = "Apple Podcasts " + id
		}
		icon := strings.TrimSpace(result.ArtworkURL600)
		if icon == "" {
			icon = strings.TrimSpace(result.ArtworkURL100)
		}
		return []ResolveCandidate{{
			Platform: "apple",
			Kind:     "rss",
			Name:     name,
			FeedURL:  feedURL,
			SiteURL:  strings.TrimSpace(result.CollectionViewURL),
			IconURL:  icon,
			Note:     "Apple Podcasts 目录解析",
		}}, nil
	}
	return nil, fmt.Errorf("这个 Apple Podcasts 节目没有返回 RSS 地址，请直接粘贴节目 RSS")
}

func mediumFeedURL(raw string) (string, string, bool) {
	parsed, err := url.Parse(normalizeInputURL(raw))
	if err != nil {
		return "", "", false
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "medium.com" && host != "www.medium.com" {
		return "", "", false
	}
	path := strings.Trim(parsed.Path, "/")
	if !strings.HasPrefix(path, "@") {
		return "", "", false
	}
	user := strings.SplitN(path, "/", 2)[0]
	if len(user) < 2 {
		return "", "", false
	}
	return "https://medium.com/feed/" + user, user, true
}

func (a *App) resolveMastodon(ctx context.Context, raw string) (ResolveCandidate, bool) {
	parsed, err := url.Parse(normalizeInputURL(raw))
	if err != nil || parsed.Hostname() == "" {
		return ResolveCandidate{}, false
	}
	path := strings.Trim(parsed.Path, "/")
	if !strings.HasPrefix(path, "@") || strings.Contains(path, "/") {
		return ResolveCandidate{}, false
	}
	siteURL := parsed.String()
	parsed.RawQuery = ""
	parsed.Fragment = ""
	feedURL := strings.TrimRight(parsed.String(), "/") + ".rss"
	title, link, icon, err := a.fetchFeedMeta(ctx, feedURL)
	if err != nil {
		return ResolveCandidate{}, false
	}
	if strings.TrimSpace(title) == "" {
		title = path + "@" + parsed.Hostname()
	}
	if strings.TrimSpace(link) == "" {
		link = siteURL
	}
	return ResolveCandidate{
		Platform: "mastodon",
		Kind:     "rss",
		Name:     title,
		FeedURL:  feedURL,
		SiteURL:  link,
		IconURL:  icon,
		Note:     "Mastodon RSS",
	}, true
}

func (a *App) fetchFeedMeta(ctx context.Context, feedURL string) (string, string, string, error) {
	headers := http.Header{}
	headers.Set("Accept", "application/rss+xml, application/atom+xml, application/feed+json, application/xml;q=0.9, text/xml;q=0.8, */*;q=0.5")
	var response *fetch.Response
	var err error
	for attempt := 0; attempt < 2; attempt++ {
		response, err = a.fetcher.GetBytes(ctx, feedURL, 5<<20, headers)
		if err == nil {
			break
		}
		var statusErr *fetch.HTTPStatusError
		if errors.As(err, &statusErr) || ctx.Err() != nil {
			break
		}
		time.Sleep(250 * time.Millisecond)
	}
	if err != nil {
		return "", "", "", err
	}
	parsed, err := gofeed.NewParser().ParseString(string(response.Body))
	if err != nil || parsed == nil {
		return "", "", "", fmt.Errorf("Feed 解析失败")
	}
	icon := ""
	if parsed.Image != nil {
		icon = strings.TrimSpace(parsed.Image.URL)
	}
	return strings.TrimSpace(parsed.Title), strings.TrimSpace(parsed.Link), icon, nil
}
