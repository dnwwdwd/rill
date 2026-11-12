package fetch

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"strings"
	"time"
)

// Full-history backfill adapters. They normalize pages into the same
// FeedItem shape as the RSS pipeline, reusing SanitizeContent so media
// rewriting and XSS cleaning stay identical. Dedupe keys match the RSS
// window adapters (X: post permalink, Bluesky: at:// URI) so a backfill
// and the regular window fetch interleave without duplicates.

const (
	xFetchAPIBase        = "https://api.fxtwitter.com"
	xPageSize            = 100
	xMaxPages            = 200 // 实测每页最多回 20 条；200 页覆盖 ~3200 条时间线上限并留余量
	xPagePause           = 300 * time.Millisecond
	bskyAppViewBase      = "https://public.api.bsky.app"
	bskyPageSize         = 100
	bskyMaxPages         = 300 // 30k posts; AppView pages until cursor is empty
	bskyPagePause        = 200 * time.Millisecond
	backfillMaxBodyBytes = 8 << 20
)

// FetchXTimeline pages api.fxtwitter.com's JSON timeline backwards until the
// cursor is exhausted (roughly the account's last ~3200 posts, a Twitter
// platform limit). baseURLOverride exists for tests; onPage reports the
// accumulated item count after each page.
func (c *Client) FetchXTimeline(ctx context.Context, handle, baseURLOverride string, onPage func(total int)) ([]FeedItem, error) {
	base := strings.TrimSuffix(xFetchAPIBase, "/")
	if baseURLOverride != "" {
		base = strings.TrimSuffix(baseURLOverride, "/")
	}
	handle = strings.TrimPrefix(strings.TrimSpace(handle), "@")
	if handle == "" {
		return nil, fmt.Errorf("X handle 为空")
	}
	all := make([]FeedItem, 0, 256)
	cursor := ""
	for page := 0; page < xMaxPages; page++ {
		url := fmt.Sprintf("%s/2/profile/%s/statuses?count=%d", base, handle, xPageSize)
		if cursor != "" {
			url += "&cursor=" + cursor
		}
		var payload xTimelineResponse
		if err := c.getJSON(ctx, url, &payload); err != nil {
			return all, err
		}
		tweets := payload.all()
		if len(tweets) == 0 {
			break
		}
		for i := range tweets {
			item, err := xTweetToItem(&tweets[i], handle)
			if err != nil {
				continue
			}
			all = append(all, item)
		}
		if payload.Cursor == nil || strings.TrimSpace(payload.Cursor.Bottom) == "" {
			break
		}
		cursor = payload.Cursor.Bottom
		if onPage != nil {
			onPage(len(all))
		}
		select {
		case <-time.After(xPagePause):
		case <-ctx.Done():
			return all, ctx.Err()
		}
	}
	return all, nil
}

type xTimelineResponse struct {
	Code    int      `json:"code"`
	Results []xTweet `json:"results"`
	Tweets  []xTweet `json:"tweets"` // 旧字段名，兼容保留
	Cursor  *struct {
		Bottom string `json:"bottom"`
	} `json:"cursor"`
}

// all returns whichever of results/tweets the API version provided.
func (r *xTimelineResponse) all() []xTweet {
	if len(r.Results) > 0 {
		return r.Results
	}
	return r.Tweets
}

type xTweet struct {
	URL       string        `json:"url"`
	Text      string        `json:"text"`
	CreatedAt string        `json:"created_at"`
	Author    *xTweetAuthor `json:"author"`
	Media     *xTweetMedia  `json:"media"`
}

type xTweetAuthor struct {
	Name       string `json:"name"`
	ScreenName string `json:"screen_name"`
}

type xTweetMedia struct {
	Photos []xTweetPhoto `json:"photos"`
	All    []struct {
		Type string `json:"type"`
		URL  string `json:"url"`
	} `json:"all"`
}

type xTweetPhoto struct {
	URL     string `json:"url"`
	AltText string `json:"alt_text"`
}

// photos merges the legacy photos[] array with the current media.all[] list.
func (m *xTweetMedia) photos() []xTweetPhoto {
	if m == nil {
		return nil
	}
	result := make([]xTweetPhoto, 0, len(m.Photos)+len(m.All))
	result = append(result, m.Photos...)
	for _, entry := range m.All {
		if entry.URL == "" {
			continue
		}
		if entry.Type != "" && entry.Type != "photo" && entry.Type != "gif" {
			continue
		}
		result = append(result, xTweetPhoto{URL: entry.URL})
	}
	return result
}

func xTweetToItem(tweet *xTweet, handle string) (FeedItem, error) {
	published := time.Now().UTC()
	if parsed, err := time.Parse("Mon Jan 2 15:04:05 -0700 2006", tweet.CreatedAt); err == nil {
		published = parsed.UTC()
	}
	text := strings.TrimSpace(tweet.Text)
	if text == "" && tweet.URL == "" {
		return FeedItem{}, fmt.Errorf("empty tweet")
	}
	var raw strings.Builder
	if text != "" {
		raw.WriteString("<p>" + html.EscapeString(text) + "</p>")
	}
	var authorName string
	if tweet.Author != nil {
		authorName = tweet.Author.Name
		if authorName == "" {
			authorName = tweet.Author.ScreenName
		}
	}
	if tweet.Media != nil {
		for _, photo := range tweet.Media.photos() {
			if photo.URL == "" {
				continue
			}
			raw.WriteString(fmt.Sprintf(`<p><img src="%s" alt="%s"></p>`, html.EscapeString(photo.URL), html.EscapeString(photo.AltText)))
		}
	}
	item := FeedItem{
		GUID:         tweet.URL,
		URL:          tweet.URL,
		Title:        firstLine(text),
		Author:       authorName,
		PublishedAt:  published,
		PublishedRaw: tweet.CreatedAt,
	}
	item.ContentHTML, item.ContentText, item.MediaURLs = SanitizeContent(raw.String(), tweet.URL, baseURL(tweet.URL), "")
	return item, nil
}

// FetchBskyTimeline pages the public AppView getAuthorFeed for a handle until
// the cursor is exhausted. Public profiles need no credentials; onPage
// reports the accumulated item count after each page.
func (c *Client) FetchBskyTimeline(ctx context.Context, handle, baseURLOverride string, onPage func(total int)) ([]FeedItem, error) {
	base := strings.TrimSuffix(bskyAppViewBase, "/")
	if baseURLOverride != "" {
		base = strings.TrimSuffix(baseURLOverride, "/")
	}
	handle = strings.TrimPrefix(strings.TrimSpace(handle), "@")
	if handle == "" {
		return nil, fmt.Errorf("Bluesky handle 为空")
	}
	all := make([]FeedItem, 0, 256)
	cursor := ""
	for page := 0; page < bskyMaxPages; page++ {
		url := fmt.Sprintf("%s/xrpc/app.bsky.feed.getAuthorFeed?actor=%s&limit=%d&filter=posts_no_replies",
			base, handle, bskyPageSize)
		if cursor != "" {
			url += "&cursor=" + cursor
		}
		var payload struct {
			Feed []struct {
				Post *struct {
					URI    string `json:"uri"`
					Author struct {
						Handle      string `json:"handle"`
						DisplayName string `json:"displayName"`
					} `json:"author"`
					Record struct {
						Text      string `json:"text"`
						CreatedAt string `json:"createdAt"`
					} `json:"record"`
					Embed     *bskyEmbed `json:"embed"`
					IndexedAt string     `json:"indexedAt"`
				} `json:"post"`
			} `json:"feed"`
			Cursor string `json:"cursor"`
		}
		if err := c.getJSON(ctx, url, &payload); err != nil {
			return all, err
		}
		if len(payload.Feed) == 0 {
			break
		}
		for _, entry := range payload.Feed {
			if entry.Post == nil {
				continue
			}
			item := bskyPostToItem(entry.Post.URI, entry.Post.Author.Handle, entry.Post.Author.DisplayName,
				entry.Post.Record.Text, entry.Post.Record.CreatedAt, entry.Post.Embed, entry.Post.IndexedAt)
			all = append(all, item)
		}
		if strings.TrimSpace(payload.Cursor) == "" {
			break
		}
		cursor = payload.Cursor
		if onPage != nil {
			onPage(len(all))
		}
		select {
		case <-time.After(bskyPagePause):
		case <-ctx.Done():
			return all, ctx.Err()
		}
	}
	return all, nil
}

type bskyEmbed struct {
	Type   string `json:"$type"`
	Images []struct {
		Thumb    string `json:"thumb"`
		Fullsize string `json:"fullsize"`
		Alt      string `json:"alt"`
	} `json:"images"`
	External *struct {
		URI         string `json:"uri"`
		Title       string `json:"title"`
		Description string `json:"description"`
		Thumb       string `json:"thumb"`
	} `json:"external"`
	Video *struct {
		Playlist  string `json:"playlist"`
		Thumbnail string `json:"thumbnail"`
		Alt       string `json:"alt"`
	} `json:"video"`
	Media *bskyEmbed `json:"media"`
}

func bskyPostToItem(uri, authorHandle, authorName, text, createdAt string, embed *bskyEmbed, indexedAt string) FeedItem {
	published := time.Now().UTC()
	if parsed, err := time.Parse(time.RFC3339, createdAt); err == nil {
		published = parsed.UTC()
	} else if parsed, err := time.Parse(time.RFC3339, indexedAt); err == nil {
		published = parsed.UTC()
	}
	text = strings.TrimSpace(text)
	link := uri
	// at://did:plc:.../app.bsky.feed.post/<rkey> → https://bsky.app/profile/<handle>/post/<rkey>
	if idx := strings.LastIndex(uri, "/"); idx >= 0 && idx < len(uri)-1 {
		rkey := uri[idx+1:]
		handle := authorHandle
		if handle == "" {
			handle = "handle.invalid"
		}
		link = fmt.Sprintf("https://bsky.app/profile/%s/post/%s", handle, rkey)
	}
	var raw strings.Builder
	if text != "" {
		raw.WriteString("<p>" + html.EscapeString(text) + "</p>")
	}
	for _, embedPart := range embedParts(embed) {
		for _, image := range embedPart.Images {
			mediaURL := image.Fullsize
			if mediaURL == "" {
				mediaURL = image.Thumb
			}
			if mediaURL == "" {
				continue
			}
			raw.WriteString(fmt.Sprintf(`<p><img src="%s" alt="%s"></p>`, html.EscapeString(mediaURL), html.EscapeString(image.Alt)))
		}
		if embedPart.Video != nil && embedPart.Video.Thumbnail != "" {
			raw.WriteString(fmt.Sprintf(`<p><img src="%s" alt="video poster"></p>`, html.EscapeString(embedPart.Video.Thumbnail)))
		}
	}
	name := authorName
	if name == "" {
		name = authorHandle
	}
	item := FeedItem{
		GUID:         uri,
		URL:          link,
		Title:        firstLine(text),
		Author:       name,
		PublishedAt:  published,
		PublishedRaw: createdAt,
	}
	item.ContentHTML, item.ContentText, item.MediaURLs = SanitizeContent(raw.String(), link, baseURL(link), "")
	return item
}

// embedParts flattens recordWithMedia containers so images/videos in either
// slot are picked up.
func embedParts(embed *bskyEmbed) []*bskyEmbed {
	if embed == nil {
		return nil
	}
	result := []*bskyEmbed{embed}
	if embed.Media != nil {
		result = append(result, embed.Media)
	}
	return result
}

// getJSON fetches and decodes a JSON envelope with the shared client rules.
func (c *Client) getJSON(ctx context.Context, url string, out any) error {
	headers := http.Header{}
	headers.Set("Accept", "application/json")
	response, err := c.GetBytes(ctx, url, backfillMaxBodyBytes, headers)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(response.Body, out); err != nil {
		return fmt.Errorf("响应 JSON 解析失败：%v", err)
	}
	return nil
}
