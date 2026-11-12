package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

type folderInput struct {
	Name     string `json:"name"`
	Sort     *int   `json:"sort"`
	ParentID *int64 `json:"parent_id"`
}

type folderPatch struct {
	Name     *string         `json:"name"`
	Sort     *int            `json:"sort"`
	ParentID json.RawMessage `json:"parent_id"`
}

func (a *App) handleListFolders(w http.ResponseWriter, r *http.Request) error {
	folders, err := a.listFolders(r.Context())
	if err != nil {
		return internalError(err)
	}
	writeJSON(w, http.StatusOK, folders)
	return nil
}

func (a *App) listFolders(ctx context.Context) ([]Folder, error) {
	rows, err := a.db.QueryContext(ctx, `
		SELECT f.id, f.name, f.sort, f.parent_id,
		       (SELECT COUNT(*) FROM sources s WHERE s.folder_id = f.id) AS source_count
		FROM folders f
		ORDER BY f.sort ASC, f.id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]Folder, 0)
	for rows.Next() {
		var folder Folder
		var parent sql.NullInt64
		if err := rows.Scan(&folder.ID, &folder.Name, &folder.Sort, &parent, &folder.SourceCount); err != nil {
			return nil, err
		}
		if parent.Valid {
			value := parent.Int64
			folder.ParentID = &value
		}
		result = append(result, folder)
	}
	return result, rows.Err()
}

func (a *App) handleCreateFolder(w http.ResponseWriter, r *http.Request) error {
	var body folderInput
	if err := decodeJSON(r, &body); err != nil {
		return err
	}
	name := strings.TrimSpace(body.Name)
	if name == "" || len([]rune(name)) > 200 {
		return badRequest("invalid_folder", "文件夹名称不能为空且不能超过 200 字")
	}
	if body.ParentID != nil {
		if err := a.ensureFolderExists(r.Context(), *body.ParentID); err != nil {
			return err
		}
	}
	sort := 0
	if body.Sort != nil {
		sort = *body.Sort
	} else {
		_ = a.db.QueryRowContext(r.Context(), `SELECT COALESCE(MAX(sort), 0) + 1 FROM folders`).Scan(&sort)
	}
	result, err := a.db.ExecContext(r.Context(), `
		INSERT INTO folders (name, sort, parent_id, created_at) VALUES (?, ?, ?, ?)`,
		name, sort, body.ParentID, a.now().Format(timeRFC3339))
	if err != nil {
		return internalError(err)
	}
	id, _ := result.LastInsertId()
	folder, err := a.getFolder(r.Context(), id)
	if err != nil {
		return internalError(err)
	}
	writeJSON(w, http.StatusCreated, folder)
	return nil
}

func (a *App) handleUpdateFolder(w http.ResponseWriter, r *http.Request) error {
	id, err := apiRouteID(r)
	if err != nil {
		return err
	}
	if _, err := a.getFolder(r.Context(), id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return notFound("folder_not_found", "文件夹不存在")
		}
		return internalError(err)
	}
	var body folderPatch
	if err := decodeJSON(r, &body); err != nil {
		return err
	}
	updates := make([]string, 0, 3)
	args := make([]any, 0, 4)
	if body.Name != nil {
		name := strings.TrimSpace(*body.Name)
		if name == "" || len([]rune(name)) > 200 {
			return badRequest("invalid_folder", "文件夹名称不能为空且不能超过 200 字")
		}
		updates = append(updates, "name = ?")
		args = append(args, name)
	}
	if body.Sort != nil {
		updates = append(updates, "sort = ?")
		args = append(args, *body.Sort)
	}
	if len(body.ParentID) > 0 {
		var parent *int64
		if string(body.ParentID) != "null" {
			var value int64
			if err := json.Unmarshal(body.ParentID, &value); err != nil {
				return badRequest("invalid_folder", "parent_id 无效")
			}
			if value == id {
				return badRequest("invalid_folder", "文件夹不能作为自己的父级")
			}
			if err := a.ensureFolderExists(r.Context(), value); err != nil {
				return err
			}
			if err := a.ensureNoFolderCycle(r.Context(), id, value); err != nil {
				return err
			}
			parent = &value
		}
		updates = append(updates, "parent_id = ?")
		args = append(args, parent)
	}
	if len(updates) == 0 {
		folder, _ := a.getFolder(r.Context(), id)
		writeJSON(w, http.StatusOK, folder)
		return nil
	}
	args = append(args, id)
	if _, err := a.db.ExecContext(r.Context(), "UPDATE folders SET "+strings.Join(updates, ", ")+" WHERE id = ?", args...); err != nil {
		return internalError(err)
	}
	folder, err := a.getFolder(r.Context(), id)
	if err != nil {
		return internalError(err)
	}
	writeJSON(w, http.StatusOK, folder)
	return nil
}

func (a *App) handleDeleteFolder(w http.ResponseWriter, r *http.Request) error {
	id, err := apiRouteID(r)
	if err != nil {
		return err
	}
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		return internalError(err)
	}
	var parent sql.NullInt64
	if err := tx.QueryRowContext(r.Context(), `SELECT parent_id FROM folders WHERE id = ?`, id).Scan(&parent); err != nil {
		_ = tx.Rollback()
		if errors.Is(err, sql.ErrNoRows) {
			return notFound("folder_not_found", "文件夹不存在")
		}
		return internalError(err)
	}
	if _, err := tx.ExecContext(r.Context(), `UPDATE sources SET folder_id = NULL WHERE folder_id = ?`, id); err != nil {
		_ = tx.Rollback()
		return internalError(err)
	}
	if _, err := tx.ExecContext(r.Context(), `UPDATE folders SET parent_id = ? WHERE parent_id = ?`, nullableInt(parent), id); err != nil {
		_ = tx.Rollback()
		return internalError(err)
	}
	if _, err := tx.ExecContext(r.Context(), `DELETE FROM folders WHERE id = ?`, id); err != nil {
		_ = tx.Rollback()
		return internalError(err)
	}
	if err := tx.Commit(); err != nil {
		return internalError(err)
	}
	writeNoContent(w)
	return nil
}

func (a *App) getFolder(ctx context.Context, id int64) (Folder, error) {
	var folder Folder
	var parent sql.NullInt64
	err := a.db.QueryRowContext(ctx, `
		SELECT f.id, f.name, f.sort, f.parent_id,
		       (SELECT COUNT(*) FROM sources s WHERE s.folder_id = f.id)
		FROM folders f WHERE f.id = ?`, id).
		Scan(&folder.ID, &folder.Name, &folder.Sort, &parent, &folder.SourceCount)
	if err == nil && parent.Valid {
		value := parent.Int64
		folder.ParentID = &value
	}
	return folder, err
}

func (a *App) ensureFolderExists(ctx context.Context, id int64) error {
	var exists int
	err := a.db.QueryRowContext(ctx, `SELECT 1 FROM folders WHERE id = ?`, id).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return badRequest("invalid_folder", "文件夹不存在")
	}
	if err != nil {
		return internalError(err)
	}
	return nil
}

func (a *App) ensureNoFolderCycle(ctx context.Context, id, parent int64) error {
	current := parent
	for depth := 0; depth < 100; depth++ {
		if current == id {
			return badRequest("invalid_folder", "文件夹层级会形成循环")
		}
		var next sql.NullInt64
		err := a.db.QueryRowContext(ctx, `SELECT parent_id FROM folders WHERE id = ?`, current).Scan(&next)
		if errors.Is(err, sql.ErrNoRows) || !next.Valid {
			return nil
		}
		if err != nil {
			return internalError(err)
		}
		current = next.Int64
	}
	return badRequest("invalid_folder", "文件夹层级过深")
}

func nullableInt(value sql.NullInt64) any {
	if value.Valid {
		return value.Int64
	}
	return nil
}

const timeRFC3339 = "2006-01-02T15:04:05Z07:00"
