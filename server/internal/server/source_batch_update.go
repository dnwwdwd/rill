package server

import (
	"context"
	"net/http"
	"strings"
)

// 订阅源批量修改/删除（2026-11-10 用户要求）。
// batch-update 支持移动分组（folder_id：数字=移入该分组，0=不分组）与启用/停用；
// batch-delete 级联删除条目（既有 FK CASCADE）。

const sourceBatchOperateMaxItems = 100

type sourceBatchUpdateInput struct {
	IDs      []int64 `json:"ids"`
	FolderID *int64  `json:"folder_id"` // 省略 = 不变；0 = 不分组；>0 = 移入该分组
	Enabled  *bool   `json:"enabled"`
}

type sourceBatchResponse struct {
	Updated int64   `json:"updated,omitempty"`
	Deleted int64   `json:"deleted,omitempty"`
	Missing []int64 `json:"missing,omitempty"`
}

// sourceIDsPresent 返回 (存在的 id 集合, 缺失的 id 列表)。
func (a *App) sourceIDsPresent(ctx context.Context, ids []int64) (map[int64]struct{}, []int64, error) {
	placeholders := make([]string, 0, len(ids))
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		placeholders = append(placeholders, "?")
		args = append(args, id)
	}
	rows, err := a.db.QueryContext(ctx,
		`SELECT id FROM sources WHERE id IN (`+strings.Join(placeholders, ",")+")", args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	present := map[int64]struct{}{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, nil, err
		}
		present[id] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	missing := []int64{}
	for _, id := range ids {
		if _, ok := present[id]; !ok {
			missing = append(missing, id)
		}
	}
	return present, missing, nil
}

// sourcePlaceholderIN 为存在的 id 生成 IN 占位符与参数。
func sourcePlaceholderIN(present map[int64]struct{}) (string, []any) {
	ph := make([]string, 0, len(present))
	args := make([]any, 0, len(present))
	for id := range present {
		ph = append(ph, "?")
		args = append(args, id)
	}
	return "(" + strings.Join(ph, ",") + ")", args
}

func (a *App) handleBatchUpdateSources(w http.ResponseWriter, r *http.Request) error {
	var body sourceBatchUpdateInput
	if err := decodeJSON(r, &body); err != nil {
		return err
	}
	ids, perr := parseBatchSourceIDs(body.IDs)
	if perr != nil {
		return perr
	}
	if body.FolderID == nil && body.Enabled == nil {
		return badRequest("invalid_update", "至少提供 folder_id 或 enabled")
	}
	if body.FolderID != nil {
		if *body.FolderID < 0 {
			return badRequest("invalid_folder", "folder_id 无效")
		}
		if *body.FolderID > 0 {
			var exists int
			if err := a.db.QueryRowContext(r.Context(),
				`SELECT COUNT(*) FROM folders WHERE id = ?`, *body.FolderID).Scan(&exists); err != nil {
				return internalError(err)
			}
			if exists == 0 {
				return badRequest("folder_not_found", "目标分组不存在")
			}
		}
	}
	present, missing, err := a.sourceIDsPresent(r.Context(), ids)
	if err != nil {
		return internalError(err)
	}
	var updated int64
	if len(present) > 0 {
		in, args := sourcePlaceholderIN(present)
		now := a.now().Format(timeRFC3339)
		if body.FolderID != nil {
			var folderArg any
			if *body.FolderID == 0 {
				folderArg = nil
			} else {
				folderArg = *body.FolderID
			}
			if _, err := a.db.ExecContext(r.Context(),
				`UPDATE sources SET folder_id = ?, updated_at = ? WHERE id IN `+in,
				append([]any{folderArg, now}, args...)...); err != nil {
				return internalError(err)
			}
		}
		if body.Enabled != nil {
			if _, err := a.db.ExecContext(r.Context(),
				`UPDATE sources SET enabled = ?, updated_at = ? WHERE id IN `+in,
				append([]any{boolInt(*body.Enabled), now}, args...)...); err != nil {
				return internalError(err)
			}
		}
		updated = int64(len(present))
	}
	writeJSON(w, http.StatusOK, sourceBatchResponse{Updated: updated, Missing: missing})
	return nil
}

func (a *App) handleBatchDeleteSources(w http.ResponseWriter, r *http.Request) error {
	var body struct {
		IDs []int64 `json:"ids"`
	}
	if err := decodeJSON(r, &body); err != nil {
		return err
	}
	ids, perr := parseBatchSourceIDs(body.IDs)
	if perr != nil {
		return perr
	}
	present, missing, err := a.sourceIDsPresent(r.Context(), ids)
	if err != nil {
		return internalError(err)
	}
	var deleted int64
	if len(present) > 0 {
		in, args := sourcePlaceholderIN(present)
		result, err := a.db.ExecContext(r.Context(), `DELETE FROM sources WHERE id IN `+in, args...)
		if err != nil {
			return internalError(err)
		}
		deleted, _ = result.RowsAffected()
	}
	writeJSON(w, http.StatusOK, sourceBatchResponse{Deleted: deleted, Missing: missing})
	return nil
}

func parseBatchSourceIDs(ids []int64) ([]int64, *apiError) {
	clean := make([]int64, 0, len(ids))
	seen := map[int64]struct{}{}
	for _, id := range ids {
		if id <= 0 {
			return nil, badRequest("invalid_ids", "订阅源 ID 无效")
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		clean = append(clean, id)
	}
	if len(clean) == 0 {
		return nil, badRequest("sources_required", "请至少选择一个订阅源")
	}
	if len(clean) > sourceBatchOperateMaxItems {
		return nil, badRequest("too_many_sources", "单次最多操作 100 个订阅源")
	}
	return clean, nil
}
