package server

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
)

// REQ-039（2026-11-12）：正文摘录/高亮与个人笔记。
// 笔记属于条目，随条目级联删除；颜色只接受固定枚举，便于前端稳定渲染。

var itemNoteColors = map[string]struct{}{
	"yellow": {}, "green": {}, "blue": {}, "pink": {},
}

type itemNoteInput struct {
	Quote string `json:"quote"`
	Note  string `json:"note"`
	Color string `json:"color"`
}

type itemNotePatch struct {
	Quote *string `json:"quote"`
	Note  *string `json:"note"`
	Color *string `json:"color"`
}

func normalizeNoteColor(value string) (string, bool) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "yellow", true
	}
	_, ok := itemNoteColors[value]
	return value, ok
}

func validateNoteText(quote, note string) *apiError {
	if len([]rune(quote)) > 4000 {
		return badRequest("invalid_note", "摘录不能超过 4000 字")
	}
	if len([]rune(note)) > 20000 {
		return badRequest("invalid_note", "笔记不能超过 20000 字")
	}
	if strings.TrimSpace(quote) == "" && strings.TrimSpace(note) == "" {
		return badRequest("invalid_note", "摘录和笔记至少填写一项")
	}
	return nil
}

func (a *App) handleListItemNotes(w http.ResponseWriter, r *http.Request) error {
	id, err := apiRouteID(r)
	if err != nil {
		return err
	}
	if err := a.ensureItemExists(r.Context(), id); err != nil {
		return err
	}
	notes, err := a.listItemNotes(r.Context(), id)
	if err != nil {
		return internalError(err)
	}
	writeJSON(w, http.StatusOK, notes)
	return nil
}

func (a *App) handleCreateItemNote(w http.ResponseWriter, r *http.Request) error {
	id, err := apiRouteID(r)
	if err != nil {
		return err
	}
	if err := a.ensureItemExists(r.Context(), id); err != nil {
		return err
	}
	var body itemNoteInput
	if err := decodeJSON(r, &body); err != nil {
		return err
	}
	quote := strings.TrimSpace(body.Quote)
	note := strings.TrimSpace(body.Note)
	if err := validateNoteText(quote, note); err != nil {
		return err
	}
	color, ok := normalizeNoteColor(body.Color)
	if !ok {
		return badRequest("invalid_note", "高亮颜色无效")
	}
	now := a.now().Format(timeRFC3339)
	result, err := a.db.ExecContext(r.Context(), `
		INSERT INTO item_notes (item_id, quote, note, color, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		id, quote, note, color, now, now)
	if err != nil {
		return internalError(err)
	}
	noteID, _ := result.LastInsertId()
	created, err := a.getItemNote(r.Context(), id, noteID)
	if err != nil {
		return internalError(err)
	}
	writeJSON(w, http.StatusCreated, created)
	return nil
}

func (a *App) handleUpdateItemNote(w http.ResponseWriter, r *http.Request) error {
	id, err := apiRouteID(r)
	if err != nil {
		return err
	}
	noteID, err := noteRouteID(r)
	if err != nil {
		return err
	}
	current, err := a.getItemNote(r.Context(), id, noteID)
	if errors.Is(err, sql.ErrNoRows) {
		return notFound("note_not_found", "笔记不存在")
	}
	if err != nil {
		return internalError(err)
	}
	var body itemNotePatch
	if err := decodeJSON(r, &body); err != nil {
		return err
	}
	quote, note, color := current.Quote, current.Note, current.Color
	if body.Quote != nil {
		quote = strings.TrimSpace(*body.Quote)
	}
	if body.Note != nil {
		note = strings.TrimSpace(*body.Note)
	}
	if body.Color != nil {
		next, ok := normalizeNoteColor(*body.Color)
		if !ok {
			return badRequest("invalid_note", "高亮颜色无效")
		}
		color = next
	}
	if err := validateNoteText(quote, note); err != nil {
		return err
	}
	if _, err := a.db.ExecContext(r.Context(), `
		UPDATE item_notes SET quote = ?, note = ?, color = ?, updated_at = ?
		WHERE id = ? AND item_id = ?`,
		quote, note, color, a.now().Format(timeRFC3339), noteID, id); err != nil {
		return internalError(err)
	}
	updated, err := a.getItemNote(r.Context(), id, noteID)
	if err != nil {
		return internalError(err)
	}
	writeJSON(w, http.StatusOK, updated)
	return nil
}

func (a *App) handleDeleteItemNote(w http.ResponseWriter, r *http.Request) error {
	id, err := apiRouteID(r)
	if err != nil {
		return err
	}
	noteID, err := noteRouteID(r)
	if err != nil {
		return err
	}
	result, err := a.db.ExecContext(r.Context(), `DELETE FROM item_notes WHERE id = ? AND item_id = ?`, noteID, id)
	if err != nil {
		return internalError(err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return notFound("note_not_found", "笔记不存在")
	}
	writeNoContent(w)
	return nil
}

func noteRouteID(r *http.Request) (int64, error) {
	value, err := strconv.ParseInt(strings.TrimSpace(r.PathValue("note_id")), 10, 64)
	if err != nil || value <= 0 {
		return 0, badRequest("invalid_note", "笔记 ID 无效")
	}
	return value, nil
}

func (a *App) ensureItemExists(ctx context.Context, id int64) error {
	var exists int
	err := a.db.QueryRowContext(ctx, `SELECT 1 FROM items WHERE id = ?`, id).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return notFound("item_not_found", "条目不存在")
	}
	if err != nil {
		return internalError(err)
	}
	return nil
}

func (a *App) listItemNotes(ctx context.Context, itemID int64) ([]ItemNote, error) {
	rows, err := a.db.QueryContext(ctx, `
		SELECT id, item_id, quote, note, color, created_at, updated_at
		FROM item_notes WHERE item_id = ? ORDER BY created_at ASC, id ASC`, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	notes := make([]ItemNote, 0)
	for rows.Next() {
		var note ItemNote
		if err := rows.Scan(&note.ID, &note.ItemID, &note.Quote, &note.Note, &note.Color, &note.CreatedAt, &note.UpdatedAt); err != nil {
			return nil, err
		}
		notes = append(notes, note)
	}
	return notes, rows.Err()
}

func (a *App) getItemNote(ctx context.Context, itemID, noteID int64) (ItemNote, error) {
	var note ItemNote
	err := a.db.QueryRowContext(ctx, `
		SELECT id, item_id, quote, note, color, created_at, updated_at
		FROM item_notes WHERE id = ? AND item_id = ?`, noteID, itemID).
		Scan(&note.ID, &note.ItemID, &note.Quote, &note.Note, &note.Color, &note.CreatedAt, &note.UpdatedAt)
	return note, err
}
