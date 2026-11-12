package server

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"rill/internal/markdown"
)

// buildItemMarkdown returns the original article body as a standalone Markdown
// document. Translations are deliberately not included (REQ-038): copy/download
// always exports what the subscription source delivered. rootURL is the Rill
// origin used to keep /media/{token}/{hash} image links absolute.
func buildItemMarkdown(detail ItemDetail, rootURL string) string {
	var out strings.Builder
	title := strings.TrimSpace(detail.Title)
	if title == "" {
		title = "未命名文章"
	}
	out.WriteString("# " + title + "\n\n")

	meta := make([]string, 0, 5)
	if name := strings.TrimSpace(detail.Source.Name); name != "" {
		meta = append(meta, "- 来源："+name)
	}
	if detail.Author != nil && strings.TrimSpace(*detail.Author) != "" {
		meta = append(meta, "- 作者："+strings.TrimSpace(*detail.Author))
	}
	if detail.PublishedAt != nil && strings.TrimSpace(*detail.PublishedAt) != "" {
		meta = append(meta, "- 发布时间："+strings.TrimSpace(*detail.PublishedAt))
	}
	if detail.OriginalTitle != nil && strings.TrimSpace(*detail.OriginalTitle) != "" && *detail.OriginalTitle != detail.Title {
		meta = append(meta, "- 原文标题："+strings.TrimSpace(*detail.OriginalTitle))
	}
	if detail.URL != nil && strings.TrimSpace(*detail.URL) != "" {
		meta = append(meta, "- 原文链接："+strings.TrimSpace(*detail.URL))
	}
	if len(meta) > 0 {
		out.WriteString(strings.Join(meta, "\n"))
		out.WriteString("\n\n---\n\n")
	}

	body := ""
	if strings.TrimSpace(detail.ContentHTML) != "" {
		body = markdown.FromHTMLWithRoot(detail.ContentHTML, stringValue(detail.URL), rootURL)
	}
	if strings.TrimSpace(body) == "" {
		body = strings.TrimSpace(detail.ContentText)
	}
	if body == "" {
		body = "（正文为空，请打开原文阅读。）"
	}
	out.WriteString(body)
	out.WriteString("\n")
	return out.String()
}

// markdownFileName builds a filesystem-safe .md name from the item title.
func markdownFileName(detail ItemDetail) string {
	name := strings.TrimSpace(detail.Title)
	if name == "" {
		name = fmt.Sprintf("item-%d", detail.ID)
	}
	replacer := strings.NewReplacer(
		"/", "-", `\`, "-", ":", "-", "*", "-", "?", "-",
		`"`, "-", "<", "-", ">", "-", "|", "-", "\n", " ", "\r", " ",
	)
	name = strings.TrimSpace(replacer.Replace(name))
	name = strings.Trim(name, ". ")
	runes := []rune(name)
	if len(runes) > 80 {
		name = string(runes[:80])
	}
	if name == "" {
		name = fmt.Sprintf("item-%d", detail.ID)
	}
	return name + ".md"
}

func (a *App) handleGetItemMarkdown(w http.ResponseWriter, r *http.Request) error {
	id, err := apiRouteID(r)
	if err != nil {
		return err
	}
	detail, err := a.getItemDetail(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		return notFound("item_not_found", "条目不存在")
	}
	if err != nil {
		return internalError(err)
	}
	body := buildItemMarkdown(detail, requestOrigin(r))
	name := markdownFileName(detail)
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(
		`attachment; filename="rill-item-%d.md"; filename*=UTF-8''%s`, detail.ID, url.PathEscape(name)))
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(body))
	return nil
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
