package server

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

const (
	watchMaxItems      = 20
	watchAITimeout     = 2 * time.Minute
	watchChannelTime   = 30 * time.Second
	watchExcerptRunes  = 180
	watchEachTitleTrim = 120
)

type watchedAlertItem struct {
	ID          int64
	Title       string
	URL         string
	ContentText string
}

// notifyWatchedSource 在抓取提交并执行过滤规则后，为关注源发送更新提醒。
// 失败只写日志，不影响抓取结果、条目入库或已读状态。
func (a *App) notifyWatchedSource(ctx context.Context, sourceID int64, newIDs []int64) {
	if len(newIDs) == 0 || ctx.Err() != nil {
		return
	}
	source, err := a.getSourceRow(ctx, sourceID)
	if err != nil {
		a.logger.Warn("watch alert source lookup failed", "source_id", sourceID, "err", err)
		return
	}
	if !source.Enabled || !source.WatchEnabled || len(source.WatchChannelIDs) == 0 {
		return
	}
	items, err := a.loadWatchedAlertItems(ctx, newIDs)
	if err != nil {
		a.logger.Warn("watch alert items lookup failed", "source_id", sourceID, "err", err)
		return
	}
	if len(items) == 0 {
		return
	}
	if len(newIDs) > watchMaxItems {
		a.logger.Info("watch alert truncated", "source_id", sourceID, "new_items", len(newIDs), "limit", watchMaxItems)
	}
	channels, err := a.loadWatchChannels(ctx, source.WatchChannelIDs)
	if err != nil {
		a.logger.Warn("watch alert channels lookup failed", "source_id", sourceID, "err", err)
		return
	}
	if len(channels) == 0 {
		a.logger.Info("watch alert skipped: no enabled channels", "source_id", sourceID)
		return
	}

	notifyCtx, cancel := context.WithTimeout(ctx, watchAITimeout)
	defer cancel()
	if source.WatchMode == "each" {
		a.sendEachWatchedItem(notifyCtx, source, items, channels)
		return
	}
	a.sendBatchWatchedItems(notifyCtx, source, items, channels)
}

func (a *App) loadWatchedAlertItems(ctx context.Context, ids []int64) ([]watchedAlertItem, error) {
	placeholders := make([]string, 0, len(ids))
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		placeholders = append(placeholders, "?")
		args = append(args, id)
	}
	query := fmt.Sprintf(`
		SELECT id, COALESCE(title, ''), COALESCE(url, ''), COALESCE(content_text, '')
		FROM items
		WHERE id IN (%s) AND hidden = 0 AND read_at IS NULL
		ORDER BY published_at DESC, id DESC
		LIMIT ?`, strings.Join(placeholders, ","))
	args = append(args, watchMaxItems)
	rows, err := a.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]watchedAlertItem, 0)
	for rows.Next() {
		var item watchedAlertItem
		if err := rows.Scan(&item.ID, &item.Title, &item.URL, &item.ContentText); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

type watchChannel struct {
	Kind   string
	URLEnc string
	Secret sql.NullString
}

func (a *App) loadWatchChannels(ctx context.Context, ids []int64) ([]watchChannel, error) {
	seen := make(map[int64]struct{}, len(ids))
	placeholders := make([]string, 0, len(ids))
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		placeholders = append(placeholders, "?")
		args = append(args, id)
	}
	if len(placeholders) == 0 {
		return nil, nil
	}
	rows, err := a.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT kind, url_enc, secret_enc FROM push_channels
		WHERE enabled = 1 AND id IN (%s)`, strings.Join(placeholders, ",")), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]watchChannel, 0, len(placeholders))
	for rows.Next() {
		var channel watchChannel
		if err := rows.Scan(&channel.Kind, &channel.URLEnc, &channel.Secret); err != nil {
			return nil, err
		}
		result = append(result, channel)
	}
	return result, rows.Err()
}

func (a *App) sendEachWatchedItem(ctx context.Context, source dbSource, items []watchedAlertItem, channels []watchChannel) {
	for _, item := range items {
		title := strings.TrimSpace(item.Title)
		if title == "" {
			title = "无标题"
		}
		body := "**" + truncateDigestRunes(title, watchEachTitleTrim) + "**"
		if source.WatchAI {
			summary, apiErr := a.generateSummary(ctx, item.ID, false)
			if apiErr == nil && len(summary.Points) > 0 {
				body += "\n\n" + strings.Join(summary.Points, "\n")
			}
		}
		if item.URL != "" {
			body += "\n\n[阅读原文](" + item.URL + ")"
		}
		a.postWatchAlert(ctx, channels, source.Name, body, 1)
	}
}

func (a *App) sendBatchWatchedItems(ctx context.Context, source dbSource, items []watchedAlertItem, channels []watchChannel) {
	var body strings.Builder
	fmt.Fprintf(&body, "**%d 条新内容**\n\n", len(items))
	if source.WatchAI {
		summary, ok := a.watchBatchSummary(ctx, source.Name, items)
		if ok && strings.TrimSpace(summary) != "" {
			body.WriteString(summary)
			body.WriteString("\n\n")
		}
	}
	for _, item := range items {
		title := strings.TrimSpace(item.Title)
		if title == "" {
			title = "无标题"
		}
		if item.URL != "" {
			fmt.Fprintf(&body, "- [%s](%s)\n", title, item.URL)
		} else {
			fmt.Fprintf(&body, "- %s\n", title)
		}
	}
	a.postWatchAlert(ctx, channels, source.Name, body.String(), len(items))
}

func (a *App) watchBatchSummary(ctx context.Context, sourceName string, items []watchedAlertItem) (string, bool) {
	runtime, apiErr := a.aiRuntime(ctx)
	if apiErr != nil {
		return "", false
	}
	digestItems := make([]digestItem, 0, len(items))
	for _, item := range items {
		digestItems = append(digestItems, digestItem{
			ID:      item.ID,
			Title:   item.Title,
			Source:  sourceName,
			URL:     item.URL,
			Excerpt: truncateDigestRunes(item.ContentText, watchExcerptRunes),
		})
	}
	return a.digestBatchSummary(ctx, runtime, digestItems)
}

func (a *App) postWatchAlert(ctx context.Context, channels []watchChannel, title, body string, itemCount int) {
	for _, channel := range channels {
		rawURL, err := a.secret.Decrypt(channel.URLEnc)
		if err != nil {
			a.logger.Warn("watch alert channel decrypt failed", "kind", channel.Kind)
			continue
		}
		var secret string
		if channel.Secret.Valid && channel.Secret.String != "" {
			secret, err = a.secret.Decrypt(channel.Secret.String)
			if err != nil {
				a.logger.Warn("watch alert channel secret decrypt failed", "kind", channel.Kind)
				continue
			}
		}
		channelCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), watchChannelTime)
		err = postToChannel(channelCtx, channel.Kind, rawURL, secret, title, body, itemCount)
		cancel()
		if err != nil {
			a.logger.Warn("watch alert push failed", "kind", channel.Kind, "err", err)
		}
	}
}
