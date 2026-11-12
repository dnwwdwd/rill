package server

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"strings"

	"rill/internal/fetch"
)

// Full-history backfill. Enabled per source via backfill_mode = 'full'
// (currently X via the FxTwitter cursor API and Bluesky via the public
// AppView). The run happens in a background goroutine, serialized against
// the regular scheduler through the same per-source fetch lock; progress
// goes into sources.note so the UI can show it.

func (a *App) startBackfill(id int64) {
	if _, running := a.backfillRunning.LoadOrStore(id, struct{}{}); running {
		return
	}
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		defer a.backfillRunning.Delete(id)
		unlock := a.sourceFetchLock(id)
		defer unlock()
		a.runBackfill(a.ctx, id)
	}()
}

func (a *App) runBackfill(ctx context.Context, id int64) {
	source, err := a.sourceInternal(ctx, id)
	if errors.Is(err, context.Canceled) {
		return
	}
	if err != nil {
		a.logger.Error("backfill load source failed", "source_id", id, "err", err)
		return
	}
	if !source.Enabled {
		return
	}
	var items []fetch.FeedItem
	var fetchErr error
	note := func(total int) {
		a.setSourceNote(context.WithoutCancel(ctx), id, "正在回补全部历史…已获取 "+strconv.Itoa(total)+" 条")
	}
	switch {
	case source.Kind == "x":
		items, fetchErr = a.fetcher.FetchXTimeline(ctx, xHandleFromFeedURL(source.FeedURL), a.xAPIBase, note)
	case source.Kind == "rss" && strings.Contains(strings.ToLower(source.FeedURL), "bsky.app/profile/"):
		items, fetchErr = a.fetcher.FetchBskyTimeline(ctx, bskyHandleFromFeedURL(source.FeedURL), a.bskyAppViewBase, note)
	default:
		return
	}
	if fetchErr != nil {
		a.logger.Warn("backfill failed", "source_id", id, "err", fetchErr)
		a.recordBackfillFailure(ctx, source, fetchErr)
		return
	}
	if len(items) == 0 {
		a.setSourceNote(context.WithoutCancel(ctx), id, "回补完成：上游没有可回补的历史条目")
		return
	}
	result := &fetch.FeedResult{Title: source.Name, Items: items, HTTPStatus: 200}
	if _, _, err := a.applyFeedResult(ctx, source, result, a.now(), 0, ""); err != nil {
		a.logger.Error("backfill apply failed", "source_id", id, "err", err)
		a.setSourceNote(context.WithoutCancel(ctx), id, "回补失败：结果写入数据库时出错")
		return
	}
	a.logger.Info("backfill finished", "source_id", id, "items", len(items))
}

// recordBackfillFailure records a failed backfill without counting it against
// the source's regular fetch health: window fetching may still work fine.
func (a *App) recordBackfillFailure(ctx context.Context, source dbSource, err error) {
	message := err.Error()
	if len([]rune(message)) > 240 {
		message = string([]rune(message)[:240]) + "…"
	}
	a.setSourceNote(context.WithoutCancel(ctx), source.ID, "回补失败："+message)
	if _, logErr := a.db.ExecContext(ctx, `
		INSERT INTO fetch_logs (source_id, started_at, duration_ms, http_status, result, item_count, new_count, error)
		VALUES (?, ?, NULL, NULL, 'error', 0, 0, ?)`,
		source.ID, a.now().UTC().Format(timeRFC3339), "回补失败："+message); logErr != nil {
		a.logger.Warn("write backfill log failed", "source_id", source.ID, "err", logErr)
	}
}

func (a *App) setSourceNote(ctx context.Context, id int64, note string) {
	if _, err := a.db.ExecContext(ctx, `UPDATE sources SET note = ?, updated_at = ? WHERE id = ?`,
		note, a.now().UTC().Format(timeRFC3339), id); err != nil {
		a.logger.Warn("write backfill note failed", "source_id", id, "err", err)
	}
}

// xHandleFromFeedURL extracts the handle from an FxTwitter feed URL such as
// https://fxtwitter.com/{handle}/feed.xml. It matches the /{handle}/feed* or
// /{handle}/media.xml shape so profile-RSS URLs (e.g. Bluesky) never match.
func xHandleFromFeedURL(feedURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(feedURL))
	if err != nil {
		return ""
	}
	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(segments) < 2 || segments[0] == "" {
		return ""
	}
	if !strings.HasPrefix(segments[1], "feed") && segments[1] != "media.xml" {
		return ""
	}
	return segments[0]
}

// bskyHandleFromFeedURL extracts the handle from an official profile RSS URL
// such as https://bsky.app/profile/{handle}/rss.
func bskyHandleFromFeedURL(feedURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(feedURL))
	if err != nil {
		return ""
	}
	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	for i, segment := range segments {
		if strings.EqualFold(segment, "profile") && i+1 < len(segments) {
			return strings.TrimSuffix(segments[i+1], "/")
		}
	}
	return ""
}
