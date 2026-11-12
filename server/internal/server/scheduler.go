package server

import (
	"context"
	"sync"
	"time"
)

const schedulerInterval = 30 * time.Second

func (a *App) runScheduler(ctx context.Context) {
	// Run immediately on startup, then every 30 seconds. Due state is stored
	// in SQLite so a restart does not trigger a full re-fetch.
	a.schedulerTick(ctx)
	ticker := time.NewTicker(schedulerInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.schedulerTick(ctx)
		}
	}
}

func (a *App) schedulerTick(ctx context.Context) {
	a.schedMu.Lock()
	a.schedLastTick = a.now()
	a.schedMu.Unlock()
	// 微信读书 keep-alive：内部自带 30 分钟限流，未连接时静默跳过。
	a.maybeWereadKeepAlive(ctx)
	// REQ-036：到期报告规则（状态落库，重启不重复生成）。
	a.maybeRunDigests(ctx)
	rows, err := a.db.QueryContext(ctx, `
		SELECT id FROM sources
		WHERE enabled = 1 AND next_fetch_at <= ?
		ORDER BY next_fetch_at ASC LIMIT 20`, a.now().Format(timeRFC3339))
	if err != nil {
		a.logger.Error("scheduler query failed", "err", err)
		return
	}
	ids := make([]int64, 0, 20)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			a.logger.Error("scheduler scan failed", "err", err)
			return
		}
		ids = append(ids, id)
	}
	if err := rows.Close(); err != nil {
		a.logger.Error("scheduler rows close failed", "err", err)
		return
	}
	semaphore := make(chan struct{}, 4)
	var wg sync.WaitGroup
	for _, id := range ids {
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		go func(sourceID int64) {
			defer wg.Done()
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-ctx.Done():
				return
			}
			if _, err := a.fetchSource(ctx, sourceID, false, ""); err != nil {
				a.logger.Warn("scheduled fetch failed", "source_id", sourceID, "err", err)
			}
		}(id)
	}
	wg.Wait()
	a.maybeApplyRetention(ctx)
	a.maybeDailyBackup(ctx)
}

func (a *App) dueCount(ctx context.Context) (int, error) {
	var count int
	err := a.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM sources WHERE enabled = 1 AND next_fetch_at <= ?`,
		a.now().Format(timeRFC3339)).Scan(&count)
	return count, err
}
