package server

import (
	"bytes"
	"context"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/PuerkitoBio/goquery"

	"rill/internal/fetch"
)

// REQ-039（2026-11-12）：按源开启全文抓取。
// Feed 只给摘要时，抓取原文页并提取正文；失败保留 Feed 摘要，不影响入库。
// 为避免首次抓取拖垮请求，单次最多处理 12 条新条目，并限制并发与超时。
const (
	fullTextPerFetch   = 12
	fullTextWorkers    = 4
	fullTextTimeout    = 7 * time.Second
	fullTextMaxBody    = 2 << 20
	fullTextMinRunes   = 160
	fullTextMinImprove = 1.25
)

var articleSelectors = []string{
	"article",
	"main",
	"[role=main]",
	".post-content",
	".article-content",
	".entry-content",
	".markdown-body",
	".content",
	"#content",
}

func (a *App) enrichFullText(ctx context.Context, source dbSource, items []fetch.FeedItem) {
	if !source.FullText || source.Kind != "rss" || len(items) == 0 {
		return
	}
	indexes := a.newFeedItemIndexes(ctx, source.ID, items)
	if len(indexes) > fullTextPerFetch {
		indexes = indexes[:fullTextPerFetch]
	}
	if len(indexes) == 0 {
		return
	}
	sem := make(chan struct{}, fullTextWorkers)
	var wg sync.WaitGroup
	for _, index := range indexes {
		index := index
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()
			item := &items[index]
			if len([]rune(item.ContentText)) > 1200 || !isHTTPItemURL(item.URL) {
				return
			}
			itemCtx, cancel := context.WithTimeout(ctx, fullTextTimeout)
			defer cancel()
			headers := http.Header{}
			headers.Set("Accept", "text/html,application/xhtml+xml;q=0.9,*/*;q=0.5")
			response, err := a.fetcher.GetBytes(itemCtx, item.URL, fullTextMaxBody, headers)
			if err != nil || response.StatusCode < 200 || response.StatusCode >= 300 {
				return
			}
			if response.ContentType != "" && !strings.Contains(strings.ToLower(response.ContentType), "html") {
				return
			}
			extracted := extractArticleHTML(response.Body)
			if strings.TrimSpace(extracted) == "" {
				return
			}
			baseURL, _ := url.Parse(response.FinalURL)
			if baseURL == nil {
				baseURL, _ = url.Parse(item.URL)
			}
			if baseURL == nil {
				return
			}
			html, text, mediaURLs := fetch.SanitizeContent(extracted, item.URL, baseURL, source.FeedToken)
			text = strings.Join(strings.Fields(text), " ")
			if utf8.RuneCountInString(text) < fullTextMinRunes {
				return
			}
			oldLen := float64(utf8.RuneCountInString(item.ContentText))
			if oldLen > 0 && float64(utf8.RuneCountInString(text)) < oldLen*fullTextMinImprove {
				return
			}
			item.ContentHTML = html
			item.ContentText = text
			// 正文 HTML 里的图片已被改写为 /media token URL；必须同时登记原图，
			// 否则 applyFeedResult 不会写入 media_cache，前端请求只会拿到 404。
			item.MediaURLs = uniqueMediaURLs(append(item.MediaURLs, mediaURLs...))
		}()
	}
	wg.Wait()
}

func isHTTPItemURL(value string) bool {
	parsed, err := url.Parse(strings.TrimSpace(value))
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}

func (a *App) newFeedItemIndexes(ctx context.Context, sourceID int64, items []fetch.FeedItem) []int {
	existing := map[string]struct{}{}
	rows, err := a.db.QueryContext(ctx, `SELECT guid FROM items WHERE source_id = ?`, sourceID)
	if err != nil {
		return nil
	}
	for rows.Next() {
		var guid string
		if err := rows.Scan(&guid); err != nil {
			_ = rows.Close()
			return nil
		}
		existing[guid] = struct{}{}
	}
	if err := rows.Close(); err != nil {
		return nil
	}
	indexes := make([]int, 0, len(items))
	for index, item := range items {
		if _, ok := existing[item.GUID]; ok {
			continue
		}
		indexes = append(indexes, index)
	}
	return indexes
}

func extractArticleHTML(raw []byte) string {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(raw))
	if err != nil {
		return ""
	}
	doc.Find("script,style,noscript,iframe,object,embed,form,svg,canvas,nav,header,footer,aside").Remove()
	best := doc.Find("body")
	bestScore := textLength(best)
	for _, selector := range articleSelectors {
		doc.Find(selector).Each(func(_ int, selection *goquery.Selection) {
			score := textLength(selection)
			if score > bestScore {
				best = selection
				bestScore = score
			}
		})
	}
	if best.Length() == 0 {
		return ""
	}
	html, err := best.Html()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(html)
}

func textLength(selection *goquery.Selection) int {
	return utf8.RuneCountInString(strings.Join(strings.Fields(selection.Text()), " "))
}
