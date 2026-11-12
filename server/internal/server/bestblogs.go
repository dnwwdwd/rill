package server

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
)

const (
	bestBlogsCatalogHost         = "wechat2rss.bestblogs.dev"
	bestBlogsXHost               = "api.xgo.ing"
	bestBlogsPodcastHost         = "rsshub.bestblogs.dev"
	bestBlogsYouTubeHost         = "www.youtube.com"
	bestBlogsCatalogBase         = "https://cdn.jsdelivr.net/gh/ginobefun/BestBlogs@main/"
	bestBlogsCatalogFallbackBase = "https://raw.githubusercontent.com/ginobefun/BestBlogs/main/"
	bestBlogsCatalogTTL          = 6 * time.Hour
	bestBlogsPartialCatalogRetry = 10 * time.Minute
	bestBlogsMaxResults          = 10
	bestBlogsWeChatNote          = "BestBlogs 托管公众号 · 仅近期文章"
	bestBlogsXNote               = "BestBlogs X 目录 · 第三方 RSS（备用）"
	bestBlogsPodcastNote         = "BestBlogs 小宇宙目录 · RSSHub（官方 Feed 优先）"
	bestBlogsYouTubeNote         = "YouTube 官方 RSS（BestBlogs 目录可搜索）"
	bestBlogsCatalogFileWeChat   = "opml/bestblogs_wechat2rss_opml_all.opml"
	bestBlogsCatalogFileX        = "BestBlogs_RSS_Twitters.opml"
	bestBlogsCatalogFileCast     = "BestBlogs_RSS_Podcasts.opml"
	bestBlogsCatalogFileVideo    = "BestBlogs_RSS_Videos.opml"
)

var (
	bestBlogsWeChatFeedPattern  = regexp.MustCompile(`^/feed/[0-9a-f]{40}\.xml$`)
	bestBlogsXFeedPattern       = regexp.MustCompile(`^/rss/user/[0-9a-f]{32}$`)
	bestBlogsPodcastFeedPattern = regexp.MustCompile(`^/xiaoyuzhou/podcast/[0-9a-f]{24}$`)
	bestBlogsYouTubeIDPattern   = regexp.MustCompile(`^UC[A-Za-z0-9_-]{22}$`)
)

// bestBlogsCatalog describes one platform directory published by BestBlogs.
// The inventory is intentionally platform-specific: the upstream hosts differ.
type bestBlogsCatalog struct {
	Platform     string
	Label        string
	Note         string
	Priority     int
	CatalogURL   string
	FallbackURL  string
	ParseFeedURL func(string) (string, bool)
}

type bestBlogsSource struct {
	Name     string
	FeedURL  string
	Platform string
	Note     string
	Priority int
}

// loadBestBlogsCatalog returns the combined cached directory, refreshing it at
// most once per TTL. A failed platform keeps its stale entries; a cold failure
// for every platform is returned so callers can fall back to MP search.
func (a *App) loadBestBlogsCatalog(ctx context.Context) ([]bestBlogsSource, error) {
	a.bestblogsMu.Lock()
	if len(a.bestblogsSources) > 0 && a.now().Sub(a.bestblogsFetchedAt) < bestBlogsCatalogTTL {
		sources := append([]bestBlogsSource(nil), a.bestblogsSources...)
		a.bestblogsMu.Unlock()
		return sources, nil
	}
	a.bestblogsMu.Unlock()

	value, err, _ := a.bestblogsGroup.Do("catalog", func() (any, error) {
		a.bestblogsMu.Lock()
		if len(a.bestblogsSources) > 0 && a.now().Sub(a.bestblogsFetchedAt) < bestBlogsCatalogTTL {
			sources := append([]bestBlogsSource(nil), a.bestblogsSources...)
			a.bestblogsMu.Unlock()
			return sources, nil
		}
		a.bestblogsMu.Unlock()
		return a.refreshBestBlogsCatalog(ctx)
	})
	if err != nil {
		a.bestblogsMu.Lock()
		stale := append([]bestBlogsSource(nil), a.bestblogsSources...)
		a.bestblogsMu.Unlock()
		if len(stale) > 0 {
			a.logger.Warn("bestblogs catalog refresh failed; using stale cache", "err", err)
			return stale, nil
		}
		return nil, err
	}
	sources, ok := value.([]bestBlogsSource)
	if !ok {
		return nil, fmt.Errorf("BestBlogs 目录缓存类型异常")
	}
	return sources, nil
}

func joinBestBlogsCatalogURL(base, file string) string {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if base == "" {
		return ""
	}
	return base + "/" + file
}

func (a *App) refreshBestBlogsCatalog(ctx context.Context) ([]bestBlogsSource, error) {
	base := strings.TrimRight(strings.TrimSpace(a.bestblogsCatalogBase), "/")
	if base == "" {
		base = strings.TrimRight(bestBlogsCatalogBase, "/")
	}
	fallbackBase := strings.TrimRight(strings.TrimSpace(a.bestblogsCatalogFallbackBase), "/")
	catalogs := []bestBlogsCatalog{
		{
			Platform:     "bestblogs",
			Label:        "公众号",
			Note:         bestBlogsWeChatNote,
			Priority:     0,
			CatalogURL:   firstNonEmpty(strings.TrimSpace(a.bestblogsCatalogURL), joinBestBlogsCatalogURL(base, bestBlogsCatalogFileWeChat)),
			FallbackURL:  joinBestBlogsCatalogURL(fallbackBase, bestBlogsCatalogFileWeChat),
			ParseFeedURL: parseBestBlogsWeChatFeedURL,
		},
		{
			Platform:     "bestblogs-x",
			Label:        "X",
			Note:         bestBlogsXNote,
			Priority:     1,
			CatalogURL:   joinBestBlogsCatalogURL(base, bestBlogsCatalogFileX),
			FallbackURL:  joinBestBlogsCatalogURL(fallbackBase, bestBlogsCatalogFileX),
			ParseFeedURL: parseBestBlogsXFeedURL,
		},
		{
			Platform:     "bestblogs-podcast",
			Label:        "小宇宙",
			Note:         bestBlogsPodcastNote,
			Priority:     2,
			CatalogURL:   joinBestBlogsCatalogURL(base, bestBlogsCatalogFileCast),
			FallbackURL:  joinBestBlogsCatalogURL(fallbackBase, bestBlogsCatalogFileCast),
			ParseFeedURL: parseBestBlogsPodcastFeedURL,
		},
		{
			Platform:     "bestblogs-youtube",
			Label:        "YouTube",
			Note:         bestBlogsYouTubeNote,
			Priority:     3,
			CatalogURL:   joinBestBlogsCatalogURL(base, bestBlogsCatalogFileVideo),
			FallbackURL:  joinBestBlogsCatalogURL(fallbackBase, bestBlogsCatalogFileVideo),
			ParseFeedURL: parseBestBlogsYouTubeFeedURL,
		},
	}

	stale := a.bestBlogsSourcesByPlatform()
	all := make([]bestBlogsSource, 0)
	failures := make([]string, 0)
	for _, catalog := range catalogs {
		sources, err := a.fetchBestBlogsCatalog(ctx, catalog)
		if err != nil {
			failures = append(failures, catalog.Label+": "+err.Error())
			sources = stale[catalog.Platform]
		}
		all = append(all, sources...)
	}
	if len(all) == 0 {
		return nil, fmt.Errorf("BestBlogs 目录不可用：%s", strings.Join(failures, "；"))
	}
	if len(failures) > 0 {
		a.logger.Warn("bestblogs partial catalog failure", "failures", strings.Join(failures, "；"))
	}
	all = dedupeBestBlogsSources(all)
	sort.SliceStable(all, func(i, j int) bool {
		if all[i].Priority != all[j].Priority {
			return all[i].Priority < all[j].Priority
		}
		return normalizeBestBlogsText(all[i].Name) < normalizeBestBlogsText(all[j].Name)
	})

	fetchedAt := a.now()
	if len(failures) > 0 {
		// Do not let one platform's transient failure suppress retries for the
		// whole TTL. Keep the good/stale platforms cached, then retry all
		// catalogs after a short interval.
		fetchedAt = fetchedAt.Add(-(bestBlogsCatalogTTL - bestBlogsPartialCatalogRetry))
	}
	a.bestblogsMu.Lock()
	a.bestblogsSources = append([]bestBlogsSource(nil), all...)
	a.bestblogsFetchedAt = fetchedAt
	a.bestblogsMu.Unlock()
	return all, nil
}

func (a *App) bestBlogsSourcesByPlatform() map[string][]bestBlogsSource {
	a.bestblogsMu.Lock()
	defer a.bestblogsMu.Unlock()
	result := make(map[string][]bestBlogsSource)
	for _, source := range a.bestblogsSources {
		result[source.Platform] = append(result[source.Platform], source)
	}
	return result
}

func (a *App) fetchBestBlogsCatalog(ctx context.Context, catalog bestBlogsCatalog) ([]bestBlogsSource, error) {
	urls := []string{catalog.CatalogURL}
	if catalog.FallbackURL != "" && catalog.FallbackURL != catalog.CatalogURL {
		urls = append(urls, catalog.FallbackURL)
	}
	failures := make([]string, 0, len(urls))
	for _, catalogURL := range urls {
		sources, err := a.fetchBestBlogsCatalogURL(ctx, catalog, catalogURL)
		if err == nil {
			return sources, nil
		}
		failures = append(failures, err.Error())
	}
	return nil, fmt.Errorf("%s", strings.Join(failures, "；"))
}

func (a *App) fetchBestBlogsCatalogURL(ctx context.Context, catalog bestBlogsCatalog, catalogURL string) ([]bestBlogsSource, error) {
	headers := http.Header{}
	headers.Set("Accept", "text/x-opml, application/xml;q=0.9, text/xml;q=0.8, */*;q=0.2")
	response, err := a.fetcher.GetBytes(ctx, catalogURL, 2<<20, headers)
	if err != nil {
		return nil, fmt.Errorf("目录拉取失败：%w", err)
	}
	var document opmlDocument
	if err := xml.Unmarshal(response.Body, &document); err != nil {
		return nil, fmt.Errorf("目录解析失败：%w", err)
	}
	if document.XMLName.Local != "opml" {
		return nil, fmt.Errorf("目录不是 OPML")
	}

	seen := make(map[string]struct{})
	sources := make([]bestBlogsSource, 0)
	var walk func([]opmlOutline)
	walk = func(outlines []opmlOutline) {
		for _, outline := range outlines {
			if feedURL, ok := catalog.ParseFeedURL(outline.XMLURL); ok {
				if _, exists := seen[feedURL]; !exists {
					seen[feedURL] = struct{}{}
					name := strings.TrimSpace(firstNonEmpty(outline.Title, outline.Text))
					if name == "" {
						name = catalog.Label
					}
					sources = append(sources, bestBlogsSource{
						Name:     name,
						FeedURL:  feedURL,
						Platform: catalog.Platform,
						Note:     catalog.Note,
						Priority: catalog.Priority,
					})
				}
			}
			walk(outline.Outlines)
		}
	}
	walk(document.Body.Outlines)
	if len(sources) == 0 {
		return nil, fmt.Errorf("没有可用的 Feed")
	}
	return sources, nil
}

func dedupeBestBlogsSources(sources []bestBlogsSource) []bestBlogsSource {
	seen := make(map[string]struct{}, len(sources))
	result := make([]bestBlogsSource, 0, len(sources))
	for _, source := range sources {
		if _, exists := seen[source.FeedURL]; exists {
			continue
		}
		seen[source.FeedURL] = struct{}{}
		result = append(result, source)
	}
	return result
}

func (a *App) searchBestBlogs(ctx context.Context, query string) ([]ResolveCandidate, error) {
	normalized := normalizeBestBlogsText(query)
	if normalized == "" {
		return nil, nil
	}
	sources, err := a.loadBestBlogsCatalog(ctx)
	if err != nil {
		return nil, err
	}

	type scored struct {
		source bestBlogsSource
		score  int
	}
	matches := make([]scored, 0)
	for _, source := range sources {
		name := normalizeBestBlogsText(source.Name)
		score := 0
		switch {
		case name == normalized:
			score = 100
		case strings.HasPrefix(name, normalized):
			score = 80
		case strings.Contains(name, normalized):
			score = 60
		case strings.Contains(normalized, name):
			score = 40
		}
		if score > 0 {
			matches = append(matches, scored{source: source, score: score})
		}
	}
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].score != matches[j].score {
			return matches[i].score > matches[j].score
		}
		if matches[i].source.Priority != matches[j].source.Priority {
			return matches[i].source.Priority < matches[j].source.Priority
		}
		return normalizeBestBlogsText(matches[i].source.Name) < normalizeBestBlogsText(matches[j].source.Name)
	})
	if len(matches) > bestBlogsMaxResults {
		matches = matches[:bestBlogsMaxResults]
	}
	candidates := make([]ResolveCandidate, 0, len(matches))
	for _, match := range matches {
		candidates = append(candidates, ResolveCandidate{
			Platform: match.source.Platform,
			Kind:     "rss",
			Name:     match.source.Name,
			FeedURL:  match.source.FeedURL,
			Note:     match.source.Note,
		})
	}
	return candidates, nil
}

func (a *App) resolveBestBlogsByName(ctx context.Context, name string) ([]ResolveCandidate, bool, error) {
	candidates, err := a.searchBestBlogs(ctx, name)
	if err != nil {
		return nil, false, err
	}
	if len(candidates) == 0 {
		return nil, false, nil
	}
	return candidates, true, nil
}

func (a *App) resolveBestBlogsFeed(ctx context.Context, raw string) ([]ResolveCandidate, bool) {
	catalogs := []bestBlogsCatalog{
		{Platform: "bestblogs", Label: "公众号", Note: bestBlogsWeChatNote, ParseFeedURL: parseBestBlogsWeChatFeedURL},
		{Platform: "bestblogs-x", Label: "X", Note: bestBlogsXNote, ParseFeedURL: parseBestBlogsXFeedURL},
		{Platform: "bestblogs-podcast", Label: "小宇宙", Note: bestBlogsPodcastNote, ParseFeedURL: parseBestBlogsPodcastFeedURL},
		{Platform: "bestblogs-youtube", Label: "YouTube", Note: bestBlogsYouTubeNote, ParseFeedURL: parseBestBlogsYouTubeFeedURL},
	}
	for _, catalog := range catalogs {
		feedURL, ok := catalog.ParseFeedURL(raw)
		if !ok {
			continue
		}
		name := "BestBlogs " + catalog.Label
		if title, _, _, err := a.fetchFeedMeta(ctx, feedURL); err == nil && strings.TrimSpace(title) != "" {
			name = strings.TrimSpace(title)
		}
		return []ResolveCandidate{{
			Platform: catalog.Platform,
			Kind:     "rss",
			Name:     name,
			FeedURL:  feedURL,
			Note:     catalog.Note,
		}}, true
	}
	return nil, false
}

func parseBestBlogsWeChatFeedURL(raw string) (string, bool) {
	return parseBestBlogsFixedURL(raw, bestBlogsCatalogHost, bestBlogsWeChatFeedPattern)
}

func parseBestBlogsXFeedURL(raw string) (string, bool) {
	return parseBestBlogsFixedURL(raw, bestBlogsXHost, bestBlogsXFeedPattern)
}

func parseBestBlogsPodcastFeedURL(raw string) (string, bool) {
	return parseBestBlogsFixedURL(raw, bestBlogsPodcastHost, bestBlogsPodcastFeedPattern)
}

func parseBestBlogsYouTubeFeedURL(raw string) (string, bool) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.User != nil {
		return "", false
	}
	if !strings.EqualFold(parsed.Scheme, "https") {
		return "", false
	}
	host := strings.ToLower(parsed.Hostname())
	if host != bestBlogsYouTubeHost && host != "youtube.com" {
		return "", false
	}
	if parsed.Path != "/feeds/videos.xml" || parsed.Fragment != "" {
		return "", false
	}
	query := parsed.Query()
	if len(query) != 1 || !bestBlogsYouTubeIDPattern.MatchString(query.Get("channel_id")) {
		return "", false
	}
	return "https://" + bestBlogsYouTubeHost + "/feeds/videos.xml?channel_id=" + url.QueryEscape(query.Get("channel_id")), true
}

func parseBestBlogsFixedURL(raw, host string, pattern *regexp.Regexp) (string, bool) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.User != nil {
		return "", false
	}
	if !strings.EqualFold(parsed.Scheme, "https") || !strings.EqualFold(parsed.Hostname(), host) {
		return "", false
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" || !pattern.MatchString(parsed.EscapedPath()) {
		return "", false
	}
	return "https://" + host + parsed.EscapedPath(), true
}

func normalizeBestBlogsText(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || strings.ContainsRune("·-—_()（）[]【】:：,，.。#", r) {
			return -1
		}
		return unicode.ToLower(r)
	}, strings.TrimSpace(value))
}

func isBareBestBlogsName(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || strings.ContainsAny(value, "/:@.") {
		return false
	}
	for _, r := range value {
		if unicode.IsSpace(r) {
			return false
		}
	}
	return true
}

func containsCJK(value string) bool {
	for _, r := range value {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

// shouldTryBestBlogsName keeps BestBlogs automatic lookup limited to Chinese
// names. ASCII handles/names belong to the platform-specific paths (FxTwitter
// for X, official feeds for podcasts/YouTube); those users can opt in with the
// explicit bestblogs: prefix.
func (a *App) shouldTryBestBlogsName(value string) bool {
	return isBareBestBlogsName(value) && containsCJK(value)
}
