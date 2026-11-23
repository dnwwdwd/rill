package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"rill/internal/fetch"
)

var wereadBaseURL = "https://weread.qq.com"

const (
	settingWereadCookie    = "weread_cookie_enc"
	wereadRequestTimeout   = 20 * time.Second
	wereadCooldownDuration = 4 * time.Hour
	wereadMaxArticles      = 20
)

var (
	wereadSkillsURL     = "https://weread.qq.com/r/weread-skills"
	wereadLoginUIDURL   = "https://weread.qq.com/api/auth/getLoginUid"
	wereadLoginInfoURL  = "https://weread.qq.com/api/auth/getLoginInfo"
	wereadRenewalURL    = "https://weread.qq.com/web/login/renewal"
	wereadShelfURL      = "https://weread.qq.com/mp/shelf/addToShelf"
	wereadArticlesURL   = "https://weread.qq.com/web/mp/articles"
	wereadCoverURL      = "https://weread.qq.com/api/mp/cover"
	wereadContentURLFmt = "https://weread.qq.com/web/mp/content?reviewId=%s"
)

var (
	errWereadNotConfigured = errors.New("微信读书 Cookie 未配置")
	errWereadAuthRequired  = errors.New("微信读书登录态已失效，请重新扫码连接")
	// -2041 是微信读书接口的风控/频控，不等于账号需要用户完成验证码。
	errWereadCaptcha     = errors.New("微信读书接口被风控拦截（-2041）：列表与最新文章均不可读取，请稍后重试或重新扫码连接")
	errWereadCooldown    = errors.New("微信读书风控冷却中（-2041）")
	errWereadUnavailable = errors.New("微信读书暂未收录或不可读取该公众号")
)

type wereadArticle struct {
	ReviewID    string
	Title       string
	URL         string
	PublishedAt time.Time
	HasTime     bool
	ContentHTML string
}

type wereadResponse struct {
	wereadStatus
	Reviews []wereadReviewGp `json:"reviews"`
}

type wereadReviewGp struct {
	CreateTime int64             `json:"createTime"`
	SubReviews []wereadSubReview `json:"subReviews"`
}

type wereadSubReview struct {
	Review wereadReview `json:"review"`
}

type wereadReview struct {
	ReviewID   string       `json:"reviewId"`
	CreateTime int64        `json:"createTime"`
	MpInfo     wereadMpInfo `json:"mpInfo"`
}

type wereadMpInfo struct {
	Title      string `json:"title"`
	OriginalID string `json:"originalId"`
	Time       int64  `json:"time"`
}

type wereadCover struct {
	wereadStatus
	ReviewID string `json:"reviewId"`
	Title    string `json:"title"`
	Pic      string `json:"pic"`
	Name     string `json:"name"`
	Time     int64  `json:"time"`
}

func wereadBookID(fakeID string) (string, error) {
	value := strings.TrimSpace(fakeID)
	if value == "" {
		return "", badRequest("invalid_wechat_url", "公众号标识为空")
	}
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		decoded, err = base64.RawStdEncoding.DecodeString(value)
	}
	if err != nil {
		decoded, err = base64.URLEncoding.DecodeString(value)
	}
	if err != nil {
		decoded, err = base64.RawURLEncoding.DecodeString(value)
	}
	if err != nil {
		return "", badRequest("invalid_wechat_url", "无法从公众号标识推导微信读书 bookId")
	}
	bookID := strings.TrimSpace(string(decoded))
	if bookID == "" {
		return "", badRequest("invalid_wechat_url", "公众号标识解码为空")
	}
	return "MP_WXS_" + bookID, nil
}

func wereadCookiesHeader(cookie string) string {
	return strings.TrimSpace(strings.Trim(cookie, "\""))
}

func (a *App) wereadDo(ctx context.Context, method, rawURL string, cookie string, body io.Reader, contentType string) (*http.Response, []byte, error) {
	request, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return nil, nil, fmt.Errorf("构造微信读书请求失败：%w", err)
	}
	request.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	request.Header.Set("Accept", "application/json, text/plain, */*")
	request.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	request.Header.Set("Referer", wereadBaseURL+"/web/mp/reader/")
	if cookie := wereadCookiesHeader(cookie); cookie != "" {
		request.Header.Set("Cookie", cookie)
	}
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	client := &http.Client{Timeout: wereadRequestTimeout}
	response, err := client.Do(request)
	if err != nil {
		return nil, nil, fmt.Errorf("微信读书请求失败：%w", err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 8<<20))
	if err != nil {
		return nil, nil, fmt.Errorf("读取微信读书响应失败：%w", err)
	}
	if response.StatusCode == http.StatusUnauthorized {
		return response, data, errWereadAuthRequired
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return response, data, fmt.Errorf("微信读书返回 HTTP %d", response.StatusCode)
	}
	return response, data, nil
}

func (a *App) wereadJSON(ctx context.Context, rawURL string, cookie string, out any) error {
	_, data, err := a.wereadDo(ctx, http.MethodGet, rawURL, cookie, nil, "")
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, out); err != nil {
		// 过期会话可能被重定向到登录 HTML 页而不是返回 401/errCode；
		// 这类响应必须按登录态失效处理，否则续期逻辑永远不会被触发。
		if looksLikeWereadLoginPage(data) {
			return errWereadAuthRequired
		}
		return fmt.Errorf("解析微信读书响应失败：%w", err)
	}
	return nil
}

func looksLikeWereadLoginPage(data []byte) bool {
	head := data
	if len(head) > 4096 {
		head = head[:4096]
	}
	text := strings.ToLower(string(head))
	if !strings.Contains(text, "<html") && !strings.Contains(text, "<!doctype") {
		return false
	}
	return strings.Contains(text, "login") || strings.Contains(text, "authorize") || strings.Contains(text, "登录")
}

type wereadStatus struct {
	ErrCode int    `json:"errCode"`
	ErrMsg  string `json:"errMsg"`
}

func wereadError(resp wereadStatus) error {
	switch resp.ErrCode {
	case 0:
		return nil
	case -2010, -2012:
		return errWereadAuthRequired
	case -2041:
		return errWereadCaptcha
	case -2042:
		return errWereadUnavailable
	default:
		return fmt.Errorf("微信读书返回 %d：%s", resp.ErrCode, resp.ErrMsg)
	}
}

func (a *App) wereadAddToShelf(ctx context.Context, bookID string) error {
	cookie, err := a.wereadCookie(ctx)
	if err != nil {
		return err
	}
	payload := strings.NewReader(`{"bookIds":[` + fmt.Sprintf("%q", bookID) + `]}`)
	_, data, err := a.wereadDo(ctx, http.MethodPost, wereadBaseURL+"/mp/shelf/addToShelf", cookie, payload, "application/json;charset=UTF-8")
	if err != nil {
		return err
	}
	var envelope wereadStatus
	_ = json.Unmarshal(data, &envelope)
	if envelope.ErrCode != 0 {
		return wereadError(envelope)
	}
	return nil
}

// recoverWechatSources clears stale「登录态已失效/需重新扫码」errors left on
// wechat sources after a successful re-scan, and pulls their next fetch forward
// so the scheduler re-fetches them with the fresh login state right away.
// Errors are not cleared blindly for healthy sources; a failed re-fetch will
// write a new, accurate error anyway.
func (a *App) recoverWechatSources(ctx context.Context) {
	result, err := a.db.ExecContext(ctx, `
		UPDATE sources SET
			last_status = 'ok', last_error = NULL, fail_count = 0,
			next_fetch_at = ?
		WHERE kind = 'wechat' AND last_status = 'error'`,
		a.now().Format(timeRFC3339))
	if err != nil {
		a.logger.Warn("wechat source status reset failed", "err", err)
		return
	}
	if affected, _ := result.RowsAffected(); affected > 0 {
		a.logger.Info("wechat source auth errors cleared after rescan", "sources", affected)
	}
}

// fetchWechatSource fetches one `wechat://<fakeid>` source through the WeRead
// channel. WeRead is the only built-in channel since DEC-024 removed the MP
// back office; see fetchWeread for the per-source flow.
func (a *App) fetchWechatSource(ctx context.Context, source dbSource) (*fetch.FeedResult, error) {
	fakeID := strings.TrimSpace(strings.TrimPrefix(source.FeedURL, wechatBuiltinPrefix))
	if fakeID == "" {
		return nil, badRequest("invalid_wechat_url", "公众号标识为空")
	}
	result, err := a.fetchWeread(ctx, source)
	if err == nil {
		return result, nil
	}
	// wr_skey 过期但 wr_rt 仍有效时，任何一次抓取失败都先尝试续期再重试一次。
	// 不能只盯 auth 错误：过期会话在不同端点上可能表现为 401、-2012、HTML 登录页
	// 或解析失败，单一错误类型判断会漏掉真正的过期场景。官方网页版正是靠
	// 持续续期保持登录。
	if a.renewWereadSession(ctx) {
		a.logger.Info("weread session renewed after fetch failure; retrying", "source_id", source.ID)
		return a.fetchWeread(ctx, source)
	}
	return result, err
}

func wereadURLFromReviewID(reviewID, bookID string) string {
	token := strings.TrimSpace(reviewID)
	if token == "" {
		return ""
	}
	prefix := bookID + "_"
	if bookID != "" && strings.HasPrefix(token, prefix) {
		token = strings.TrimPrefix(token, prefix)
	} else if index := strings.LastIndex(token, "_"); index >= 0 {
		token = token[index+1:]
	}
	token = strings.ReplaceAll(token, "~", "_")
	if token == "" {
		return ""
	}
	return "https://mp.weixin.qq.com/s/" + url.PathEscape(token)
}

// wereadCoverArticle reads the latest article metadata for a book. It is the
// documented fallback when /web/mp/articles is blocked by risk control
// (we-mp-rss weread_mp mode uses cover as its primary list source).
func (a *App) wereadCoverArticle(ctx context.Context, cookie, bookID, sourceName string) (wereadArticle, bool, error) {
	coverURL := fmt.Sprintf("%s?bookId=%s", wereadCoverURL, url.QueryEscape(bookID))
	var cover wereadCover
	if err := a.wereadJSON(ctx, coverURL, cookie, &cover); err != nil {
		return wereadArticle{}, false, err
	}
	if err := wereadError(cover.wereadStatus); err != nil {
		return wereadArticle{}, false, err
	}
	reviewID := strings.TrimSpace(cover.ReviewID)
	if reviewID == "" {
		return wereadArticle{}, false, nil
	}
	title := firstNonEmpty(strings.TrimSpace(cover.Title), strings.TrimSpace(cover.Name), sourceName)
	item := wereadArticle{ReviewID: reviewID, Title: title}
	if cover.Time > 0 {
		item.PublishedAt = time.Unix(cover.Time, 0)
		item.HasTime = true
	}
	item.URL = wereadURLFromReviewID(reviewID, bookID)
	return item, true, nil
}

func (a *App) fetchWeread(ctx context.Context, source dbSource) (*fetch.FeedResult, error) {
	cookie, err := a.wereadCookie(ctx)
	if err != nil || strings.TrimSpace(cookie) == "" {
		return nil, errWereadNotConfigured
	}
	if remaining := a.wereadCooldownRemaining(); remaining > 0 {
		return nil, fmt.Errorf("%w：剩余约 %d 分钟", errWereadCooldown, int(remaining.Minutes())+1)
	}

	fakeID := strings.TrimSpace(strings.TrimPrefix(source.FeedURL, wechatBuiltinPrefix))
	bookID, err := wereadBookID(fakeID)
	if err != nil {
		return nil, err
	}
	// External accounts are normally not on the WeRead shelf yet. Adding is
	// idempotent; a failure here must not abort the fetch because the list/cover
	// endpoints may still work.
	if err := a.wereadAddToShelf(ctx, bookID); err != nil {
		a.logger.Warn("weread add to shelf failed", "source_id", source.ID, "err", err)
	}

	listURL := fmt.Sprintf("%s?bookId=%s&offset=0", wereadArticlesURL, url.QueryEscape(bookID))
	var payload wereadResponse
	articles := make([]wereadArticle, 0, wereadMaxArticles)
	coverOnly := false
	listErr := a.wereadJSON(ctx, listURL, cookie, &payload)
	if listErr == nil {
		listErr = wereadError(payload.wereadStatus)
	}
	if listErr != nil {
		if !errors.Is(listErr, errWereadCaptcha) {
			return nil, listErr
		}
		// -2041 是微信读书列表接口的风控/频控，不代表账号需要用户做验证码。
		// 参考 we-mp-rss weread_mp 的降级策略：列表不可用时用 cover 只同步最新一篇。
		item, ok, coverErr := a.wereadCoverArticle(ctx, cookie, bookID, source.Name)
		if coverErr != nil || !ok {
			a.startWereadCooldown()
			if coverErr != nil {
				return nil, errors.Join(listErr, coverErr)
			}
			return nil, listErr
		}
		a.logger.Warn("weread article list blocked; syncing latest cover only", "source_id", source.ID, "book_id", bookID)
		articles = append(articles, item)
		coverOnly = true
	} else {
		for _, group := range payload.Reviews {
			for _, sub := range group.SubReviews {
				review := sub.Review
				title := strings.TrimSpace(review.MpInfo.Title)
				if title == "" {
					continue
				}
				item := wereadArticle{
					ReviewID: strings.TrimSpace(review.ReviewID),
					Title:    title,
				}
				timestamp := review.CreateTime
				if timestamp == 0 {
					timestamp = group.CreateTime
				}
				if timestamp == 0 && review.MpInfo.Time > 0 {
					timestamp = review.MpInfo.Time
				}
				if timestamp > 0 {
					item.PublishedAt = time.Unix(timestamp, 0)
					item.HasTime = true
				}
				// A 22-char originalId is the legacy short token. New opaque ids
				// are resolved by the content proxy below.
				if id := strings.TrimSpace(review.MpInfo.OriginalID); id != "" {
					if len(id) == 22 && !strings.ContainsAny(id, "~") {
						item.URL = "https://mp.weixin.qq.com/s/" + strings.ReplaceAll(id, "~", "_")
					}
				}
				articles = append(articles, item)
				if len(articles) >= wereadMaxArticles {
					break
				}
			}
			if len(articles) >= wereadMaxArticles {
				break
			}
		}
	}

	if len(articles) == 0 {
		// Some WeRead sessions/accounts only expose the latest issue through
		// the cover endpoint. It is enough for incremental updates even though
		// it cannot backfill history.
		item, ok, coverErr := a.wereadCoverArticle(ctx, cookie, bookID, source.Name)
		if coverErr != nil {
			if errors.Is(coverErr, errWereadCaptcha) {
				a.startWereadCooldown()
			}
			return nil, coverErr
		}
		if ok {
			articles = append(articles, item)
			coverOnly = true
		}
	}

	result := &fetch.FeedResult{Title: source.Name, HTTPStatus: http.StatusOK}
	existing, err := a.existingItemGUIDs(ctx, source.ID)
	if err != nil {
		return nil, err
	}
	for index, article := range articles {
		guid := article.ReviewID
		if guid == "" {
			guid = article.URL
		}
		if guid == "" {
			continue
		}
		if _, ok := existing[guid]; ok {
			continue
		}
		// Space content requests; the first article is allowed immediately.
		if index > 0 {
			select {
			case <-time.After(2 * time.Second):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		content, contentURL, contentErr := a.fetchWereadContent(ctx, cookie, article.ReviewID)
		if contentErr != nil && !errors.Is(contentErr, errWereadUnavailable) {
			if errors.Is(contentErr, errWereadCaptcha) {
				if coverOnly {
					// 列表本来就被风控，正文失败只保留标题/链接，避免整次抓取失败。
					a.logger.Warn("weread content blocked; keeping title only", "source_id", source.ID)
					content = wereadContent{Title: article.Title}
					contentErr = nil
				} else {
					a.startWereadCooldown()
					return nil, contentErr
				}
			}
			if contentErr != nil {
				return nil, contentErr
			}
		}
		itemURL := firstNonEmpty(article.URL, contentURL)
		contentHTML := content.ContentHTML
		contentText := htmlToText(contentHTML)
		var mediaURLs []string
		// 微信读书返回的是公众号原始正文 HTML，图片仍指向 mmbiz.qpic.cn。
		// 浏览器直接加载会被 Referer 防盗链替换成「未经允许不可引用」占位图；
		// 这里统一走与 RSS 相同的 /media 代理，由服务端无 Referer 下载并缓存。
		if base, parseErr := url.Parse(itemURL); strings.TrimSpace(contentHTML) != "" && parseErr == nil {
			if sanitized, text, media := fetch.SanitizeContent(contentHTML, itemURL, base, source.FeedToken); strings.TrimSpace(sanitized) != "" {
				contentHTML, contentText, mediaURLs = sanitized, text, media
			}
		}
		item := fetch.FeedItem{
			GUID:        guid,
			URL:         itemURL,
			Title:       firstNonEmpty(article.Title, content.Title),
			PublishedAt: article.PublishedAt,
			ContentHTML: contentHTML,
			ContentText: contentText,
			MediaURLs:   mediaURLs,
		}
		if article.HasTime {
			item.PublishedRaw = article.PublishedAt.Format(time.RFC3339)
		} else {
			item.PublishedAt = a.now()
			item.PublishedEstimated = true
		}
		item.ContentHash = contentHash(item.ContentHTML)
		result.Items = append(result.Items, item)
	}
	return result, nil
}

type wereadContent struct {
	Title       string
	URL         string
	ContentHTML string
}

func wereadExtractSingleQuotedJS(raw, marker string) string {
	start := strings.Index(raw, marker)
	if start < 0 {
		return ""
	}
	start += len(marker)
	out := strings.Builder{}
	for index := start; index < len(raw); index++ {
		char := raw[index]
		if char != '\\' {
			if char == '\'' {
				break
			}
			out.WriteByte(char)
			continue
		}
		if index+1 >= len(raw) {
			break
		}
		next := raw[index+1]
		switch next {
		case 'x':
			if index+3 < len(raw) {
				if value, err := strconv.ParseInt(raw[index+2:index+4], 16, 32); err == nil {
					out.WriteByte(byte(value))
					index += 3
					continue
				}
			}
			out.WriteByte('\\')
		case 'n':
			out.WriteByte('\n')
		case 't':
			out.WriteByte('\t')
		case 'r':
			out.WriteByte('\r')
		case '\'':
			out.WriteByte('\'')
		case '"':
			out.WriteByte('"')
		case '\\':
			out.WriteByte('\\')
		default:
			out.WriteByte('\\')
			out.WriteByte(next)
		}
		index++
	}
	return out.String()
}

func wereadNoEncodeContent(raw string) string {
	text := wereadExtractSingleQuotedJS(raw, "content_noencode: '")
	if text == "" {
		text = wereadExtractSingleQuotedJS(raw, "content_noencode:'")
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	paragraphs := make([]string, 0)
	for _, paragraph := range strings.Split(text, "\n") {
		paragraph = strings.TrimSpace(paragraph)
		if paragraph == "" {
			continue
		}
		paragraph = strings.ReplaceAll(paragraph, "&", "&amp;")
		paragraph = strings.ReplaceAll(paragraph, "<", "&lt;")
		paragraph = strings.ReplaceAll(paragraph, ">", "&gt;")
		paragraphs = append(paragraphs, "<p>"+paragraph+"</p>")
	}
	return strings.Join(paragraphs, "")
}

func (a *App) fetchWereadContent(ctx context.Context, cookie, reviewID string) (wereadContent, string, error) {
	reviewID = strings.TrimSpace(reviewID)
	if reviewID == "" {
		return wereadContent{}, "", errWereadUnavailable
	}
	rawURL := fmt.Sprintf(wereadContentURLFmt, url.QueryEscape(reviewID))
	_, data, err := a.wereadDo(ctx, http.MethodGet, rawURL, cookie, nil, "")
	if err != nil {
		return wereadContent{}, "", err
	}
	document, err := goquery.NewDocumentFromReader(strings.NewReader(string(data)))
	if err != nil {
		return wereadContent{}, "", fmt.Errorf("解析微信读书正文失败：%w", err)
	}
	content := wereadContent{URL: rawURL}
	if selection := document.Find(`meta[property="og:url"]`).First(); selection.Length() > 0 {
		content.URL = strings.TrimSpace(selection.AttrOr("content", ""))
	}
	if selection := document.Find(`meta[property="og:title"]`).First(); selection.Length() > 0 {
		content.Title = strings.TrimSpace(selection.AttrOr("content", ""))
	}
	body := document.Find("#js_content").First()
	if body.Length() == 0 {
		body = document.Find(".rich_media_content").First()
	}
	if body.Length() == 0 {
		if text := wereadNoEncodeContent(string(data)); text != "" {
			content.ContentHTML = text
			return content, content.URL, nil
		}
		return wereadContent{}, content.URL, errWereadUnavailable
	}
	body.Find("img").Each(func(_ int, selection *goquery.Selection) {
		for _, attr := range []string{"data-src", "src"} {
			if value := strings.TrimSpace(selection.AttrOr(attr, "")); value != "" {
				selection.SetAttr("src", value)
				break
			}
		}
	})
	html, err := body.Html()
	if err != nil {
		return wereadContent{}, content.URL, fmt.Errorf("提取微信读书正文失败：%w", err)
	}
	content.ContentHTML = html
	return content, content.URL, nil
}
