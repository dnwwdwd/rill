package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"rill/internal/wechat"
)

// REQ-034 / DEC-024: WeRead QR login is the only built-in channel for
// official-account sources (`wechat://<fakeid>`). The MP back-office chain
// (REQ-026) and the we-mp-rss sidecar were removed: the back office only
// serves accounts managed by the logged-in session (BUG-001).

const wechatBuiltinPrefix = "wechat://"

// wechatArticleHost is the only host accepted for article-link resolution.
// It is a variable so tests can point it at an httptest server.
var wechatArticleHost = "mp.weixin.qq.com"

var htmlTagPattern = regexp.MustCompile(`<[^>]+>`)

func isBuiltinWechatSource(source dbSource) bool {
	return source.Kind == "wechat" && strings.HasPrefix(strings.TrimSpace(source.FeedURL), wechatBuiltinPrefix)
}

// wechatPublicClient fetches public mp.weixin.qq.com article pages. No login
// state is involved; article pages are readable anonymously.
func wechatPublicClient() *wechat.Client {
	return wechat.NewClient()
}

// handleWechatResolveArticle resolves an mp.weixin.qq.com article link to its
// account. The public page carries `var biz = ...`, the account name and the
// avatar, so no back-office login is required. Creating the source still
// requires a connected WeRead login because `wechat://` sources sync through
// WeRead only.
func (a *App) handleWechatResolveArticle(w http.ResponseWriter, r *http.Request) error {
	settings, err := a.loadInternalSettings(r.Context())
	if err != nil {
		return internalError(err)
	}
	if !settings.Wechat.Enabled {
		return conflict("wechat_disabled", "请先启用公众号采集")
	}
	var input struct {
		URL string `json:"url"`
	}
	if err := decodeJSON(r, &input); err != nil {
		return err
	}
	raw := strings.TrimSpace(input.URL)
	parsed, err := url.Parse(raw)
	if err != nil || !strings.EqualFold(parsed.Hostname(), wechatArticleHost) {
		return badRequest("invalid_wechat_url", "请粘贴微信公众号文章链接")
	}
	article, err := wechatPublicClient().FetchArticle(r.Context(), raw)
	if err != nil {
		return unavailable("wechat_article_unavailable", "无法读取文章链接："+err.Error())
	}
	if strings.TrimSpace(article.Biz) == "" {
		return badRequest("wechat_account_unresolved", "无法从文章链接识别公众号")
	}
	account := wechat.Account{FakeID: article.Biz, Name: article.AccountName, Avatar: article.AccountAvatar}
	writeJSON(w, http.StatusOK, map[string]any{"account": account})
	return nil
}

// handleWechatCreateSource creates a `wechat://<fakeid>` source. It requires
// a connected WeRead login; otherwise the source could never sync.
func (a *App) handleWechatCreateSource(w http.ResponseWriter, r *http.Request) error {
	var input struct {
		Account  string `json:"account"`
		Name     string `json:"name"`
		Avatar   string `json:"avatar"`
		FolderID *int64 `json:"folder_id"`
	}
	if err := decodeJSON(r, &input); err != nil {
		return err
	}
	fakeID := strings.TrimSpace(input.Account)
	if fakeID == "" {
		return badRequest("invalid_wechat_account", "公众号标识为空，请粘贴公众号文章链接后重试")
	}
	if !a.wereadCookieConfigured(r.Context()) {
		return unauthorized("wechat_auth_required", "请先在设置中连接微信读书扫码")
	}
	name := strings.TrimSpace(input.Name)
	avatar := strings.TrimSpace(input.Avatar)
	if name == "" {
		return badRequest("invalid_wechat_account", "公众号名称为空，请重新解析后添加")
	}
	refresh := 1800
	var iconURL *string
	if avatar != "" {
		iconURL = &avatar
	}
	source, err := a.createSourceRow(r.Context(), sourceCreateInput{
		Kind:           "wechat",
		Name:           name,
		FeedURL:        wechatBuiltinPrefix + fakeID,
		FolderID:       input.FolderID,
		RefreshSeconds: &refresh,
		IconURL:        iconURL,
	})
	if err != nil {
		return err
	}
	if !a.disableInitialFetch {
		fetchCtx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		if _, fetchErr := a.fetchSource(fetchCtx, source.ID, false, ""); fetchErr != nil {
			a.logger.Warn("initial wechat fetch failed", "source_id", source.ID, "err", fetchErr)
		}
		cancel()
		if refreshed, refreshErr := a.getSource(r.Context(), source.ID); refreshErr == nil {
			source = refreshed
		}
	}
	writeJSON(w, http.StatusCreated, source)
	return nil
}

func (a *App) existingItemGUIDs(ctx context.Context, sourceID int64) (map[string]struct{}, error) {
	rows, err := a.db.QueryContext(ctx, `SELECT guid FROM items WHERE source_id = ?`, sourceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[string]struct{})
	for rows.Next() {
		var guid string
		if err := rows.Scan(&guid); err != nil {
			return nil, err
		}
		result[guid] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func htmlToText(raw string) string {
	text := htmlTagPattern.ReplaceAllString(raw, " ")
	text = html.UnescapeString(text)
	return strings.Join(strings.Fields(text), " ")
}

func contentHash(raw string) string {
	if raw == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
