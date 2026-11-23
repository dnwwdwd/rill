package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"rill/internal/fetch"
)

var nextDataPattern = regexp.MustCompile(`(?s)<script id="__NEXT_DATA__" type="application/json">(.*?)</script>`)

type xyzImage struct {
	PicURL string `json:"picUrl"`
}

type xyzPodcast struct {
	Type         string       `json:"type"`
	PID          string       `json:"pid"`
	Title        string       `json:"title"`
	Author       string       `json:"author"`
	Brief        string       `json:"brief"`
	Description  string       `json:"description"`
	Image        xyzImage     `json:"image"`
	EpisodeCount int          `json:"episodeCount"`
	Episodes     []xyzEpisode `json:"episodes"`
}

type xyzEpisode struct {
	Type        string   `json:"type"`
	EID         string   `json:"eid"`
	PID         string   `json:"pid"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Duration    int      `json:"duration"`
	Image       xyzImage `json:"image"`
	Enclosure   struct {
		URL string `json:"url"`
	} `json:"enclosure"`
	PubDate string      `json:"pubDate"`
	Podcast *xyzPodcast `json:"podcast"`
}

type xyzNextData struct {
	Props struct {
		PageProps struct {
			Podcast *xyzPodcast `json:"podcast"`
			Episode *xyzEpisode `json:"episode"`
		} `json:"pageProps"`
	} `json:"props"`
}

func xiaoyuzhouRefFromInput(raw string) (pid, eid, token string, ok bool) {
	value := strings.TrimSpace(raw)
	lower := strings.ToLower(value)
	if strings.HasPrefix(lower, "xiaoyuzhou://podcast/") {
		pid = strings.TrimSpace(value[len("xiaoyuzhou://podcast/"):])
		if xiaoyuzhouIDPattern.MatchString(pid) {
			return pid, "", "", true
		}
		return "", "", "", false
	}
	if xiaoyuzhouIDPattern.MatchString(value) {
		return value, "", "", true
	}
	parsed, err := url.Parse(normalizeInputURL(value))
	if err != nil {
		return "", "", "", false
	}
	host := strings.ToLower(parsed.Hostname())
	path := strings.Trim(parsed.Path, "/")
	switch host {
	case "feed.xyzfm.space":
		if path == "" {
			return "", "", "", false
		}
		return "", "", strings.SplitN(path, "/", 2)[0], true
	case "xiaoyuzhoufm.com", "www.xiaoyuzhoufm.com":
		parts := strings.Split(path, "/")
		if len(parts) >= 2 && parts[0] == "podcast" && xiaoyuzhouIDPattern.MatchString(parts[1]) {
			return parts[1], "", "", true
		}
		if len(parts) >= 2 && parts[0] == "episode" && xiaoyuzhouIDPattern.MatchString(parts[1]) {
			return "", parts[1], "", true
		}
	}
	return "", "", "", false
}

func (a *App) resolveXiaoyuzhouReference(ctx context.Context, raw string) ([]ResolveCandidate, bool, error) {
	value := strings.TrimSpace(raw)
	if xiaoyuzhouIDPattern.MatchString(value) {
		candidates, err := a.resolveXiaoyuzhouRef(ctx, value, "", "")
		return candidates, true, err
	}
	if pid, eid, token, ok := xiaoyuzhouRefFromInput(value); ok {
		candidates, err := a.resolveXiaoyuzhouRef(ctx, pid, eid, token)
		return candidates, true, err
	}
	return nil, true, fmt.Errorf("无法识别小宇宙链接或 ID")
}

func (a *App) resolveXiaoyuzhouRef(ctx context.Context, pid, eid, token string) ([]ResolveCandidate, error) {
	if token != "" {
		title, link, icon, err := a.fetchFeedMeta(ctx, "https://feed.xyzfm.space/"+url.PathEscape(token))
		if err != nil {
			return nil, fmt.Errorf("小宇宙分发 Feed 无法访问：%v", err)
		}
		if strings.TrimSpace(title) == "" {
			title = "小宇宙播客 " + token
		}
		return []ResolveCandidate{{
			Platform: "xiaoyuzhou",
			Kind:     "rss",
			Name:     title,
			FeedURL:  "https://feed.xyzfm.space/" + url.PathEscape(token),
			SiteURL:  link,
			IconURL:  icon,
			Note:     "小宇宙官方分发 Feed",
		}}, nil
	}
	if pid == "" && eid != "" {
		data, err := a.fetchXiaoyuzhouPage(ctx, "episode", eid)
		if err != nil {
			return nil, err
		}
		if data.Props.PageProps.Episode != nil {
			pid = strings.TrimSpace(data.Props.PageProps.Episode.PID)
			if pid == "" && data.Props.PageProps.Episode.Podcast != nil {
				pid = strings.TrimSpace(data.Props.PageProps.Episode.Podcast.PID)
			}
		}
		if pid == "" {
			return nil, fmt.Errorf("无法从这一集找到所属播客")
		}
	}
	if pid == "" {
		return nil, fmt.Errorf("小宇宙节目 ID 为空")
	}
	data, err := a.fetchXiaoyuzhouPage(ctx, "podcast", pid)
	if err != nil {
		return nil, err
	}
	podcast := data.Props.PageProps.Podcast
	if podcast == nil || strings.TrimSpace(podcast.PID) == "" {
		return nil, fmt.Errorf("小宇宙页面没有返回节目信息")
	}
	pageURL := a.xiaoyuzhouBase + "/podcast/" + url.PathEscape(pid)
	icon := strings.TrimSpace(podcast.Image.PicURL)
	if feedURL, feedTitle, feedLink, feedIcon := a.findXiaoyuzhouFeed(ctx, pid, podcast.Title); feedURL != "" {
		if strings.TrimSpace(feedTitle) == "" {
			feedTitle = strings.TrimSpace(podcast.Title)
		}
		if strings.TrimSpace(feedLink) == "" {
			feedLink = pageURL
		}
		if strings.TrimSpace(feedIcon) != "" {
			icon = feedIcon
		}
		return []ResolveCandidate{{
			Platform: "xiaoyuzhou",
			Kind:     "rss",
			Name:     feedTitle,
			FeedURL:  feedURL,
			SiteURL:  feedLink,
			IconURL:  icon,
			Note:     "小宇宙官方分发 Feed",
		}}, nil
	}
	return []ResolveCandidate{{
		Platform: "xiaoyuzhou",
		Kind:     "rss",
		Name:     strings.TrimSpace(podcast.Title),
		FeedURL:  "xiaoyuzhou://podcast/" + url.PathEscape(pid),
		SiteURL:  pageURL,
		IconURL:  icon,
		Note:     "小宇宙页面抓取，最近 15 条（非全量）",
		Partial:  true,
	}}, nil
}

// findXiaoyuzhouFeed uses the Apple directory as a discovery index for
// feed.xyzfm.space. A candidate is accepted only when the feed's channel link
// points back to the same Xiaoyuzhou podcast ID.
func (a *App) findXiaoyuzhouFeed(ctx context.Context, pid, title string) (feedURL, feedTitle, feedLink, feedIcon string) {
	title = strings.TrimSpace(title)
	if title == "" {
		return "", "", "", ""
	}
	endpoint := a.appleAPIBase + "/search?term=" + url.QueryEscape(title) + "&entity=podcast&country=cn&limit=10"
	headers := http.Header{}
	headers.Set("Accept", "application/json")
	response, err := a.fetcher.GetBytes(ctx, endpoint, 2<<20, headers)
	if err != nil {
		return "", "", "", ""
	}
	var payload appleLookupResponse
	if err := json.Unmarshal(response.Body, &payload); err != nil {
		return "", "", "", ""
	}
	for _, result := range payload.Results {
		candidate := strings.TrimSpace(result.FeedURL)
		parsed, err := url.Parse(candidate)
		if err != nil || !strings.EqualFold(parsed.Hostname(), "feed.xyzfm.space") {
			continue
		}
		feedTitle, feedLink, feedIcon, err = a.fetchFeedMeta(ctx, candidate)
		if err != nil {
			continue
		}
		if strings.Contains(feedLink, "/podcast/"+pid) {
			return candidate, feedTitle, feedLink, feedIcon
		}
	}
	return "", "", "", ""
}

func (a *App) fetchXiaoyuzhouPage(ctx context.Context, kind, id string) (xyzNextData, error) {
	endpoint := a.xiaoyuzhouBase + "/" + kind + "/" + url.PathEscape(id)
	headers := http.Header{}
	headers.Set("Accept", "text/html,application/xhtml+xml")
	headers.Set("Referer", a.xiaoyuzhouBase+"/")
	response, err := a.fetcher.GetBytes(ctx, endpoint, 5<<20, headers)
	if err != nil {
		var statusErr *fetch.HTTPStatusError
		if errors.As(err, &statusErr) || ctx.Err() != nil {
			return xyzNextData{}, fmt.Errorf("小宇宙页面无法访问：%v", err)
		}
		time.Sleep(250 * time.Millisecond)
		response, err = a.fetcher.GetBytes(ctx, endpoint, 5<<20, headers)
		if err != nil {
			return xyzNextData{}, fmt.Errorf("小宇宙页面无法访问：%v", err)
		}
	}
	return parseXiaoyuzhouPage(response.Body)
}

func parseXiaoyuzhouPage(body []byte) (xyzNextData, error) {
	match := nextDataPattern.FindSubmatch(body)
	if len(match) != 2 {
		return xyzNextData{}, fmt.Errorf("小宇宙页面结构已变化，找不到节目数据")
	}
	var data xyzNextData
	if err := json.Unmarshal(match[1], &data); err != nil {
		return xyzNextData{}, fmt.Errorf("小宇宙页面数据无法解析")
	}
	return data, nil
}

// isXiaoyuzhouFeedURL reports whether the source fetches the Xiaoyuzhou
// official distribution feed (feed.xyzfm.space). Since 2026-11 that host
// enforces an upstream IP whitelist and denies non-whitelisted exits.
func isXiaoyuzhouFeedURL(feedURL string) bool {
	return strings.Contains(strings.ToLower(feedURL), "feed.xyzfm.space/")
}

// xiaoyuzhouPidFromSiteURL recovers the podcast id from a stored site URL
// (www.xiaoyuzhoufm.com/podcast/{pid}) so a denied distribution feed can
// degrade to the page fetcher.
func xiaoyuzhouPidFromSiteURL(siteURL *string) string {
	if siteURL == nil {
		return ""
	}
	if pid, _, _, ok := xiaoyuzhouRefFromInput(*siteURL); ok && pid != "" {
		return pid
	}
	return ""
}

func (a *App) fetchXiaoyuzhou(ctx context.Context, source dbSource) (*fetch.FeedResult, error) {
	pid := strings.TrimSpace(strings.TrimPrefix(source.FeedURL, "xiaoyuzhou://podcast/"))
	if pid == "" {
		return nil, fmt.Errorf("小宇宙源地址无效")
	}
	data, err := a.fetchXiaoyuzhouPage(ctx, "podcast", pid)
	if err != nil {
		return nil, err
	}
	podcast := data.Props.PageProps.Podcast
	if podcast == nil {
		return nil, fmt.Errorf("小宇宙页面没有返回节目信息")
	}
	pageURL := a.xiaoyuzhouBase + "/podcast/" + url.PathEscape(pid)
	base, _ := url.Parse(a.xiaoyuzhouBase + "/")
	result := &fetch.FeedResult{
		Title:      strings.TrimSpace(podcast.Title),
		Link:       pageURL,
		IconURL:    strings.TrimSpace(podcast.Image.PicURL),
		HTTPStatus: http.StatusOK,
	}
	now := a.now()
	for _, episode := range podcast.Episodes {
		episodeURL := a.xiaoyuzhouBase + "/episode/" + url.PathEscape(episode.EID)
		title := strings.TrimSpace(episode.Title)
		if title == "" {
			title = "(无标题)"
		}
		published := now
		if parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(episode.PubDate)); err == nil {
			published = parsed.UTC()
		}
		var rawHTML strings.Builder
		if description := strings.TrimSpace(episode.Description); description != "" {
			for _, paragraph := range strings.Split(description, "\n") {
				paragraph = strings.TrimSpace(paragraph)
				if paragraph == "" {
					continue
				}
				rawHTML.WriteString("<p>")
				rawHTML.WriteString(escapeHTML(paragraph))
				rawHTML.WriteString("</p>")
			}
		}
		if image := strings.TrimSpace(episode.Image.PicURL); image != "" {
			rawHTML.WriteString(`<p><img src="`)
			rawHTML.WriteString(escapeHTML(image))
			rawHTML.WriteString(`" alt=""></p>`)
		}
		contentHTML, contentText, mediaURLs := fetch.SanitizeContent(rawHTML.String(), episodeURL, base, source.FeedToken)
		audioURL := strings.TrimSpace(episode.Enclosure.URL)
		item := fetch.FeedItem{
			GUID:         strings.TrimSpace(episode.EID),
			URL:          episodeURL,
			Title:        title,
			Author:       strings.TrimSpace(podcast.Author),
			PublishedAt:  published,
			PublishedRaw: strings.TrimSpace(episode.PubDate),
			ContentHTML:  contentHTML,
			ContentText:  contentText,
			MediaURLs:    mediaURLs,
			AudioURL:     audioURL,
			AudioType:    audioTypeForPodcastURL(audioURL),
		}
		if item.GUID == "" {
			item.GUID = episodeURL
		}
		item.ContentHash = feedItemHash(item)
		result.Items = append(result.Items, item)
	}
	return result, nil
}

func audioTypeForPodcastURL(raw string) string {
	lower := strings.ToLower(strings.TrimSpace(raw))
	if lower == "" {
		return ""
	}
	switch {
	case strings.HasSuffix(lower, ".m4a"), strings.HasSuffix(lower, ".mp4"):
		return "audio/mp4"
	case strings.HasSuffix(lower, ".aac"):
		return "audio/aac"
	case strings.HasSuffix(lower, ".ogg"), strings.HasSuffix(lower, ".opus"):
		return "audio/ogg"
	case strings.HasSuffix(lower, ".wav"):
		return "audio/wav"
	default:
		return "audio/mpeg"
	}
}

func feedItemHash(item fetch.FeedItem) string {
	payload := strings.Join([]string{
		item.Title,
		item.PublishedRaw,
		item.ContentText,
		strings.Join(item.MediaURLs, "\n"),
		item.AudioURL,
	}, "\n")
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}

func escapeHTML(value string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&#39;",
	)
	return replacer.Replace(value)
}
