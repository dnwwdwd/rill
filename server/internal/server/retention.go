package server

import (
	"context"
	"strings"
	"time"
)

const retentionInterval = 6 * time.Hour

type retentionSource struct {
	ID                int64
	RetentionDays     int
	RetentionMaxItems int
}

func (a *App) maybeApplyRetention(ctx context.Context) {
	a.schedMu.Lock()
	last := a.schedLastRetention
	due := last.IsZero() || a.now().Sub(last) >= retentionInterval
	if due {
		a.schedLastRetention = a.now()
	}
	a.schedMu.Unlock()
	if !due {
		return
	}
	deleted, err := a.applyRetention(ctx)
	if err != nil {
		a.logger.Error("apply retention failed", "err", err)
		return
	}
	if deleted > 0 {
		a.logger.Info("retention cleanup", "deleted", deleted)
	}
}

func (a *App) applyRetention(ctx context.Context) (int, error) {
	rows, err := a.db.QueryContext(ctx, `
		SELECT id, retention_days, retention_max_items
		FROM sources
		WHERE retention_days > 0 OR retention_max_items > 0
		ORDER BY id ASC`)
	if err != nil {
		return 0, err
	}
	sources := make([]retentionSource, 0)
	for rows.Next() {
		var source retentionSource
		if err := rows.Scan(&source.ID, &source.RetentionDays, &source.RetentionMaxItems); err != nil {
			_ = rows.Close()
			return 0, err
		}
		sources = append(sources, source)
	}
	if err := rows.Close(); err != nil {
		return 0, err
	}
	total := 0
	for _, source := range sources {
		deleted, err := a.applySourceRetention(ctx, source)
		if err != nil {
			return total, err
		}
		total += deleted
	}
	return total, nil
}

func (a *App) applySourceRetention(ctx context.Context, source retentionSource) (int, error) {
	deleted := 0
	if source.RetentionDays > 0 {
		cutoff := a.now().AddDate(0, 0, -source.RetentionDays).Format(timeRFC3339)
		result, err := a.db.ExecContext(ctx, `
			DELETE FROM items
			WHERE source_id = ?
			  AND read_at IS NOT NULL
			  AND starred_at IS NULL
			  AND COALESCE(published_at, fetched_at) < ?`, source.ID, cutoff)
		if err != nil {
			return deleted, err
		}
		affected, _ := result.RowsAffected()
		deleted += int(affected)
	}
	if source.RetentionMaxItems > 0 {
		rows, err := a.db.QueryContext(ctx, `
			SELECT id FROM items
			WHERE source_id = ?
			  AND read_at IS NOT NULL
			  AND starred_at IS NULL
			ORDER BY COALESCE(published_at, fetched_at) DESC, id DESC
			LIMIT -1 OFFSET ?`, source.ID, source.RetentionMaxItems)
		if err != nil {
			return deleted, err
		}
		ids := make([]int64, 0)
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				_ = rows.Close()
				return deleted, err
			}
			ids = append(ids, id)
		}
		if err := rows.Close(); err != nil {
			return deleted, err
		}
		if len(ids) > 0 {
			placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
			args := make([]any, 0, len(ids))
			for _, id := range ids {
				args = append(args, id)
			}
			result, err := a.db.ExecContext(ctx, `DELETE FROM items WHERE id IN (`+placeholders+`)`, args...)
			if err != nil {
				return deleted, err
			}
			affected, _ := result.RowsAffected()
			deleted += int(affected)
		}
	}
	if deleted > 0 {
		a.logger.Info("source retention", "source_id", source.ID, "deleted", deleted,
			"days", source.RetentionDays, "max_items", source.RetentionMaxItems)
	}
	return deleted, nil
}
