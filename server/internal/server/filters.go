package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

// REQ-037（DEC-027）：关键字/正则过滤规则。
// 新条目入库后按源评估；动作 mark_read / hide；不做相似文章聚合。

type FilterRule struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Pattern    string `json:"pattern"`
	IsRegex    bool   `json:"is_regex"`
	MatchField string `json:"match_field"`
	Action     string `json:"action"`
	Scope      []any  `json:"scope"`
	Enabled    bool   `json:"enabled"`
	HitCount   int64  `json:"hit_count"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

type filterRuleInput struct {
	Name       string `json:"name"`
	Pattern    string `json:"pattern"`
	IsRegex    bool   `json:"is_regex"`
	MatchField string `json:"match_field"`
	Action     string `json:"action"`
	Scope      []any  `json:"scope"`
	Enabled    *bool  `json:"enabled"`
}

func (a *App) handleListFilterRules(w http.ResponseWriter, r *http.Request) error {
	rows, err := a.db.QueryContext(r.Context(), `
		SELECT id, name, pattern, is_regex, match_field, action, scope_json,
		       enabled, hit_count, created_at, updated_at
		FROM filter_rules ORDER BY id`)
	if err != nil {
		return internalError(err)
	}
	defer rows.Close()
	out := []FilterRule{}
	for rows.Next() {
		var rule FilterRule
		var scope string
		var isRegex, enabled int
		var matchField, action string
		if err := rows.Scan(&rule.ID, &rule.Name, &rule.Pattern, &isRegex, &matchField,
			&action, &scope, &enabled, &rule.HitCount, &rule.CreatedAt, &rule.UpdatedAt); err != nil {
			return internalError(err)
		}
		_ = json.Unmarshal([]byte(scope), &rule.Scope)
		rule.IsRegex = isRegex == 1
		rule.MatchField = matchField
		rule.Action = action
		rule.Enabled = enabled == 1
		out = append(out, rule)
	}
	if err := rows.Err(); err != nil {
		return internalError(err)
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}

func validateFilterRuleInput(in *filterRuleInput) *apiError {
	in.Name = strings.TrimSpace(in.Name)
	in.Pattern = strings.TrimSpace(in.Pattern)
	in.MatchField = strings.TrimSpace(in.MatchField)
	in.Action = strings.TrimSpace(in.Action)
	if in.Name == "" {
		return badRequest("invalid_name", "规则名称不能为空")
	}
	if in.Pattern == "" {
		return badRequest("invalid_pattern", "匹配模式不能为空")
	}
	if in.MatchField != "title" && in.MatchField != "content" && in.MatchField != "any" {
		return badRequest("invalid_match_field", "匹配字段只支持 title、content 或 any")
	}
	if in.Action != "mark_read" && in.Action != "hide" {
		return badRequest("invalid_action", "动作只支持 mark_read 或 hide")
	}
	if in.IsRegex {
		if _, err := regexp.Compile(in.Pattern); err != nil {
			return badRequest("invalid_pattern", "正则表达式无效："+err.Error())
		}
	}
	for _, entry := range in.Scope {
		switch v := entry.(type) {
		case json.Number:
			if n, err := v.Int64(); err != nil || n <= 0 {
				return badRequest("invalid_scope", "范围条目必须是正整数源 ID 或 \"folder:<id>\"")
			}
		case string:
			if !strings.HasPrefix(v, "folder:") {
				return badRequest("invalid_scope", "字符串范围条目必须是 \"folder:<id>\"")
			}
		default:
			return badRequest("invalid_scope", "范围条目类型无效")
		}
	}
	return nil
}

func (a *App) handleCreateFilterRule(w http.ResponseWriter, r *http.Request) error {
	var in filterRuleInput
	if err := decodeJSON(r, &in); err != nil {
		return badRequest("invalid_body", "请求体无效")
	}
	if err := validateFilterRuleInput(&in); err != nil {
		return err
	}
	scopeJSON, _ := json.Marshal(in.Scope)
	enabled := in.Enabled == nil || *in.Enabled
	now := a.now().Format(timeRFC3339)
	result, err := a.db.ExecContext(r.Context(), `
		INSERT INTO filter_rules (name, pattern, is_regex, match_field, action, scope_json, enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		in.Name, in.Pattern, boolInt(in.IsRegex), in.MatchField, in.Action, string(scopeJSON), boolInt(enabled), now, now)
	if err != nil {
		return internalError(err)
	}
	id, _ := result.LastInsertId()
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
	return nil
}

func (a *App) handleUpdateFilterRule(w http.ResponseWriter, r *http.Request) error {
	id, err := apiRouteID(r)
	if err != nil {
		return err
	}
	var in filterRuleInput
	if err := decodeJSON(r, &in); err != nil {
		return badRequest("invalid_body", "请求体无效")
	}
	if err := validateFilterRuleInput(&in); err != nil {
		return err
	}
	scopeJSON, _ := json.Marshal(in.Scope)
	enabled := in.Enabled == nil || *in.Enabled
	if _, err := a.db.ExecContext(r.Context(), `
		UPDATE filter_rules SET name = ?, pattern = ?, is_regex = ?, match_field = ?, action = ?,
			scope_json = ?, enabled = ?, updated_at = ?
		WHERE id = ?`,
		in.Name, in.Pattern, boolInt(in.IsRegex), in.MatchField, in.Action,
		string(scopeJSON), boolInt(enabled), a.now().Format(timeRFC3339), id); err != nil {
		return internalError(err)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	return nil
}

func (a *App) handleDeleteFilterRule(w http.ResponseWriter, r *http.Request) error {
	id, err := apiRouteID(r)
	if err != nil {
		return err
	}
	if _, err := a.db.ExecContext(r.Context(), `DELETE FROM filter_rules WHERE id = ?`, id); err != nil {
		return internalError(err)
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (a *App) handleApplyFilterRule(w http.ResponseWriter, r *http.Request) error {
	id, err := apiRouteID(r)
	if err != nil {
		return err
	}
	affected, err := a.applyFilterRuleToExisting(r.Context(), id)
	if err != nil {
		return internalError(err)
	}
	writeJSON(w, http.StatusOK, map[string]any{"affected": affected})
	return nil
}

// ---------- 评估 ----------

type dbFilterRule struct {
	ID         int64
	Pattern    string
	IsRegex    bool
	MatchField string
	Action     string
	Scope      []any
}

func (a *App) loadFilterRulesForSource(ctx context.Context, sourceID int64, folderID sql.NullInt64) []dbFilterRule {
	rows, err := a.db.QueryContext(ctx, `
		SELECT id, pattern, is_regex, match_field, action, scope_json
		FROM filter_rules WHERE enabled = 1`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []dbFilterRule
	for rows.Next() {
		var rule dbFilterRule
		var scope string
		var isRegex int
		if err := rows.Scan(&rule.ID, &rule.Pattern, &isRegex, &rule.MatchField, &rule.Action, &scope); err != nil {
			continue
		}
		rule.IsRegex = isRegex == 1
		_ = json.Unmarshal([]byte(scope), &rule.Scope)
		if filterRuleCoversSource(rule.Scope, sourceID, folderID) {
			out = append(out, rule)
		}
	}
	return out
}

// filterRuleCoversSource：空 scope = 全部源；数字 = 指定源；"folder:<id>" = 分组（含子组）。
func filterRuleCoversSource(scope []any, sourceID int64, folderID sql.NullInt64) bool {
	if len(scope) == 0 {
		return true
	}
	hasFolder := false
	for _, entry := range scope {
		switch v := entry.(type) {
		case float64:
			if int64(v) == sourceID {
				return true
			}
		case json.Number:
			if n, err := v.Int64(); err == nil && n == sourceID {
				return true
			}
		case string:
			if strings.HasPrefix(v, "folder:") && folderID.Valid {
				var fid int64
				if _, err := fmt.Sscanf(strings.TrimPrefix(v, "folder:"), "%d", &fid); err == nil && fid == folderID.Int64 {
					hasFolder = true
				}
			}
		}
	}
	return hasFolder
}

// applyFiltersToItems 在新条目入库后调用：评估启用规则并按命中更新条目。
func (a *App) applyFiltersToItems(ctx context.Context, sourceID int64, folderID sql.NullInt64, itemIDs []int64, titles, contents map[int64]string) {
	rules := a.loadFilterRulesForSource(ctx, sourceID, folderID)
	now := a.now().Format(timeRFC3339)
	for _, rule := range rules {
		var matched []int64
		for _, id := range itemIDs {
			if filterRuleMatches(rule, titles[id], contents[id]) {
				matched = append(matched, id)
			}
		}
		if len(matched) == 0 {
			continue
		}
		a.execFilterActions(ctx, rule, matched, now)
	}
}

func filterRuleMatches(rule dbFilterRule, title, content string) bool {
	if rule.IsRegex {
		re, err := regexp.Compile(rule.Pattern)
		if err != nil {
			return false // 运行期非法正则跳过
		}
		switch rule.MatchField {
		case "title":
			return title != "" && re.MatchString(title)
		case "content":
			return content != "" && re.MatchString(content)
		default:
			return (title != "" && re.MatchString(title)) || (content != "" && re.MatchString(content))
		}
	}
	pattern := strings.ToLower(rule.Pattern)
	switch rule.MatchField {
	case "title":
		return title != "" && strings.Contains(strings.ToLower(title), pattern)
	case "content":
		return content != "" && strings.Contains(strings.ToLower(content), pattern)
	default:
		return (title != "" && strings.Contains(strings.ToLower(title), pattern)) ||
			(content != "" && strings.Contains(strings.ToLower(content), pattern))
	}
}

func (a *App) execFilterActions(ctx context.Context, rule dbFilterRule, itemIDs []int64, now string) {
	placeholders := make([]string, 0, len(itemIDs))
	args := make([]any, 0, len(itemIDs))
	for _, id := range itemIDs {
		placeholders = append(placeholders, "?")
		args = append(args, id)
	}
	in := "(" + strings.Join(placeholders, ",") + ")"
	if rule.Action == "mark_read" {
		query := "UPDATE items SET read_at = ? WHERE id IN " + in + " AND read_at IS NULL"
		updateArgs := append([]any{now}, args...)
		if _, err := a.db.ExecContext(ctx, query, updateArgs...); err != nil {
			a.logger.Warn("filter mark_read failed", "rule_id", rule.ID, "err", err)
			return
		}
	} else {
		query := "UPDATE items SET hidden = 1 WHERE id IN " + in
		if _, err := a.db.ExecContext(ctx, query, args...); err != nil {
			a.logger.Warn("filter hide failed", "rule_id", rule.ID, "err", err)
			return
		}
	}
	if _, err := a.db.ExecContext(ctx,
		`UPDATE filter_rules SET hit_count = hit_count + ? WHERE id = ?`, int64(len(itemIDs)), rule.ID); err != nil {
		a.logger.Warn("filter hit_count failed", "rule_id", rule.ID, "err", err)
	}
}

// applyFilterRuleToExisting 把规则应用到范围内全部存量条目（不取消已读/收藏）。
func (a *App) applyFilterRuleToExisting(ctx context.Context, ruleID int64) (int64, error) {
	var pattern string
	var isRegex int
	var matchField, action, scope string
	err := a.db.QueryRowContext(ctx, `
		SELECT pattern, is_regex, match_field, action, scope_json
		FROM filter_rules WHERE id = ?`, ruleID).
		Scan(&pattern, &isRegex, &matchField, &action, &scope)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, notFound("not_found", "规则不存在")
		}
		return 0, err
	}
	rule := dbFilterRule{
		ID: ruleID, Pattern: pattern, IsRegex: isRegex == 1,
		MatchField: matchField, Action: action,
	}
	_ = json.Unmarshal([]byte(scope), &rule.Scope)

	sourceIDs, err := a.resolveDigestScope(ctx, rule.Scope)
	if err != nil {
		return 0, err
	}
	if len(sourceIDs) == 0 {
		return 0, nil
	}
	placeholders := make([]string, 0, len(sourceIDs))
	args := make([]any, 0, len(sourceIDs))
	for _, id := range sourceIDs {
		placeholders = append(placeholders, "?")
		args = append(args, id)
	}
	rows, err := a.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT i.id, COALESCE(i.title, ''), COALESCE(i.content_text, '')
		FROM items i
		WHERE i.source_id IN (%s)`, strings.Join(placeholders, ",")), args...)
	if err != nil {
		return 0, err
	}
	type row struct {
		id      int64
		title   string
		content string
	}
	var all []row
	for rows.Next() {
		var it row
		if err := rows.Scan(&it.id, &it.title, &it.content); err != nil {
			_ = rows.Close()
			return 0, err
		}
		all = append(all, it)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	var matched []int64
	for _, it := range all {
		if filterRuleMatches(rule, it.title, it.content) {
			matched = append(matched, it.id)
		}
	}
	if len(matched) == 0 {
		return 0, nil
	}
	a.execFilterActions(ctx, rule, matched, a.now().Format(timeRFC3339))
	return int64(len(matched)), nil
}
