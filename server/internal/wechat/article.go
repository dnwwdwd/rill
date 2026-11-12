package wechat

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/microcosm-cc/bluemonday"
)

var (
	articleBizPattern    = regexp.MustCompile(`var\s+biz\s*=\s*["']([^"']+)["']`)
	articleAvatarPattern = regexp.MustCompile(`var\s+round_head_img\s*=\s*["']([^"']+)["']`)
)

// extractArticleBiz resolves the official account's fakeid from an article
// URL (?__biz=...) or the page's `var biz = "..."` script.
func extractArticleBiz(body []byte, articleURL *url.URL) string {
	if articleURL != nil {
		if biz := strings.TrimSpace(articleURL.Query().Get("__biz")); biz != "" {
			return biz
		}
	}
	if match := articleBizPattern.FindAllSubmatch(body, -1); len(match) > 0 {
		for _, candidate := range match {
			if biz := strings.TrimSpace(string(candidate[1])); biz != "" {
				return biz
			}
		}
	}
	return ""
}

func extractArticleAvatar(body []byte) string {
	if match := articleAvatarPattern.FindAllSubmatch(body, -1); len(match) > 0 {
		for _, candidate := range match {
			if avatar := strings.TrimSpace(string(candidate[1])); avatar != "" {
				if strings.HasPrefix(avatar, "//") {
					return "https:" + avatar
				}
				return avatar
			}
		}
	}
	return ""
}

// FetchArticle downloads an article page and extracts its metadata and body.
// It returns ErrArticleUnavailable when #js_content (or the legacy
// .rich_media_content) is absent; an empty body is never treated as success.
func (c *Client) FetchArticle(ctx context.Context, rawURL string) (*Article, error) {
	ctx = ensureContext(ctx)
	rawURL = strings.TrimSpace(rawURL)
	target, err := url.Parse(rawURL)
	if err != nil || target.Host == "" ||
		(!strings.EqualFold(target.Scheme, "http") && !strings.EqualFold(target.Scheme, "https")) {
		return nil, fmt.Errorf("wechat: invalid article URL %q", rawURL)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("wechat: article request: %w", err)
	}
	setBrowserHeaders(req, target.String())

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("wechat: fetch article: %w", err)
	}
	defer resp.Body.Close()
	if !isSuccessStatus(resp.StatusCode) {
		return nil, fmt.Errorf("wechat: fetch article: unexpected HTTP status %s", resp.Status)
	}
	body, err := readLimited(resp.Body, maxArticleBody)
	if err != nil {
		return nil, fmt.Errorf("wechat: fetch article: %w", err)
	}
	return parseArticle(body, target)
}

func parseArticle(body []byte, articleURL *url.URL) (*Article, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("wechat: parse article: %w", err)
	}

	article := &Article{
		Link:          articleURL.String(),
		Biz:           extractArticleBiz(body, articleURL),
		AccountAvatar: extractArticleAvatar(body),
		MediaURLs:     make([]string, 0),
	}
	article.Title = cleanText(doc.Find("#activity-name").First().Text())
	if article.Title == "" {
		article.Title = cleanText(doc.Find(".rich_media_title").First().Text())
	}

	article.AccountName = cleanText(doc.Find("#js_name").First().Text())
	if article.AccountName == "" {
		article.AccountName = cleanText(doc.Find(".rich_media_meta_nickname").First().Text())
	}

	article.Author = cleanText(doc.Find("#js_author_name").First().Text())
	if article.Author == "" {
		doc.Find(".rich_media_meta_text").EachWithBreak(func(_ int, selection *goquery.Selection) bool {
			id, _ := selection.Attr("id")
			if id == "publish_time" || id == "js_name" {
				return true
			}
			if candidate := cleanText(selection.Text()); candidate != "" {
				article.Author = candidate
				return false
			}
			return true
		})
	}

	if published := cleanText(doc.Find("#publish_time").First().Text()); published != "" {
		article.PublishedText = published
		if parsed, ok := parsePublishTime(published); ok {
			article.PublishedAt = parsed
			article.HasPublished = true
		}
	}

	content := doc.Find("#js_content").First()
	if content.Length() == 0 {
		content = doc.Find(".rich_media_content").First()
	}
	if content.Length() == 0 {
		pageText := cleanText(doc.Text())
		if strings.Contains(pageText, "当前环境异常，完成验证后即可继续访问") {
			return nil, fmt.Errorf("%w: anti-crawler verification page for %s", ErrArticleUnavailable, articleURL)
		}
		return nil, fmt.Errorf("%w: #js_content not found for %s (article may be deleted or blocked)", ErrArticleUnavailable, articleURL)
	}

	article.HTML, article.MediaURLs = sanitizeArticleContent(content, articleURL)
	if strings.TrimSpace(article.HTML) == "" {
		return nil, fmt.Errorf("%w: #js_content is empty for %s (article may be deleted or blocked)", ErrArticleUnavailable, articleURL)
	}
	return article, nil
}

func sanitizeArticleContent(content *goquery.Selection, articleURL *url.URL) (string, []string) {
	content.Find("script, style, iframe, noscript, svg").Remove()

	content.Find("*").Each(func(_ int, selection *goquery.Selection) {
		node := selection.Get(0)
		if node == nil {
			return
		}
		remove := make([]string, 0)
		for _, attribute := range node.Attr {
			key := strings.ToLower(attribute.Key)
			if strings.HasPrefix(key, "on") {
				remove = append(remove, attribute.Key)
				continue
			}
			if key == "href" || key == "src" {
				value := strings.ToLower(strings.TrimSpace(attribute.Val))
				if strings.HasPrefix(value, "javascript:") {
					remove = append(remove, attribute.Key)
				}
			}
		}
		for _, key := range remove {
			selection.RemoveAttr(key)
		}
	})

	media := make([]string, 0)
	seen := make(map[string]bool)
	content.Find("img").Each(func(_ int, selection *goquery.Selection) {
		if dataSrc, ok := selection.Attr("data-src"); ok {
			if dataSrc = strings.TrimSpace(dataSrc); dataSrc != "" {
				selection.SetAttr("src", dataSrc)
			}
		}
		selection.RemoveAttr("data-src")
		selection.RemoveAttr("srcset")

		rawSrc, _ := selection.Attr("src")
		if resolved := resolveURL(articleURL, rawSrc); resolved != "" {
			selection.SetAttr("src", resolved)
			if !seen[resolved] {
				seen[resolved] = true
				media = append(media, resolved)
			}
		}
	})

	htmlBody, err := content.Html()
	if err != nil {
		htmlBody = ""
	}
	policy := bluemonday.UGCPolicy()
	return strings.TrimSpace(policy.Sanitize(htmlBody)), media
}

func resolveURL(base *url.URL, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	lower := strings.ToLower(value)
	if strings.HasPrefix(lower, "javascript:") || strings.HasPrefix(lower, "data:") {
		return ""
	}
	reference, err := url.Parse(value)
	if err != nil {
		return ""
	}
	if reference.Scheme != "" && !strings.EqualFold(reference.Scheme, "http") && !strings.EqualFold(reference.Scheme, "https") {
		return ""
	}
	if base != nil {
		reference = base.ResolveReference(reference)
	}
	if reference.Host == "" || (!strings.EqualFold(reference.Scheme, "http") && !strings.EqualFold(reference.Scheme, "https")) {
		return ""
	}
	return reference.String()
}

func parsePublishTime(value string) (time.Time, bool) {
	layouts := []string{
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
		"2006-1-2 15:04:05",
		"2006-1-2 15:04",
		"2006-1-2",
		"2006/01/02 15:04:05",
		"2006/01/02 15:04",
		"2006/01/02",
		"2006/1/2 15:04:05",
		"2006/1/2 15:04",
		"2006/1/2",
		"2006年01月02日 15:04:05",
		"2006年01月02日 15:04",
		"2006年01月02日",
		"2006年1月2日 15:04:05",
		"2006年1月2日 15:04",
		"2006年1月2日",
		time.RFC3339,
	}
	location := time.FixedZone("CST", 8*60*60)
	for _, layout := range layouts {
		if parsed, err := time.ParseInLocation(layout, value, location); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func cleanText(value string) string {
	return strings.Join(strings.Fields(value), " ")
}
