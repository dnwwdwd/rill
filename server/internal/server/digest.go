package server

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// REQ-036（DEC-027）：定时报告与可配置推送渠道。
// 规则/报告/渠道全部落库；窗口 = last_run_at → now；AI 失败降级为列表报告。

const (
	digestBatchSize    = 8   // 分批小结每批条数
	digestMaxItems     = 200 // 单次报告条目上限，超出截取最新
	digestMergeThresh  = 40  // 批小结超过此数走树状归并
	digestMergeGroup   = 20
	digestExcerptRunes = 600
	digestFeishuLimit  = 9000 // lark_md 正文截断长度（飞书卡片 JSON 约 30KB 上限）
)

type PushChannel struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	URLMasked string `json:"url_masked"`
	HasSecret bool   `json:"has_secret"`
	Enabled   bool   `json:"enabled"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type DigestRule struct {
	ID            int64   `json:"id"`
	Name          string  `json:"name"`
	Scope         []any   `json:"scope"`
	Period        string  `json:"period"`
	RunAtLocal    string  `json:"run_at_local"`
	Weekday       int     `json:"weekday"`
	OnlyUnread    bool    `json:"only_unread"`
	PushWhenEmpty bool    `json:"push_when_empty"`
	Enabled       bool    `json:"enabled"`
	ChannelIDs    []int64 `json:"channel_ids"`
	LastRunAt     *string `json:"last_run_at,omitempty"`
	NextRunAt     *string `json:"next_run_at,omitempty"`
	CreatedAt     string  `json:"created_at"`
	UpdatedAt     string  `json:"updated_at"`
}

type DigestReport struct {
	ID          int64   `json:"id"`
	RuleID      int64   `json:"rule_id"`
	RuleName    string  `json:"rule_name,omitempty"`
	PeriodStart string  `json:"period_start"`
	PeriodEnd   string  `json:"period_end"`
	ItemCount   int     `json:"item_count"`
	Status      string  `json:"status"`
	ContentMD   string  `json:"content_md"`
	Model       *string `json:"model,omitempty"`
	PushStatus  string  `json:"push_status"`
	PushError   *string `json:"push_error,omitempty"`
	PushedAt    *string `json:"pushed_at,omitempty"`
	CreatedAt   string  `json:"created_at"`
}

type channelInput struct {
	Name    string  `json:"name"`
	Kind    string  `json:"kind"`
	URL     string  `json:"url"`
	Secret  *string `json:"secret"` // 飞书签名校验密钥；nil=不变（更新时），空串=清除
	Enabled *bool   `json:"enabled"`
}

type digestRuleInput struct {
	Name          string  `json:"name"`
	Scope         []any   `json:"scope"`
	Period        string  `json:"period"`
	RunAtLocal    string  `json:"run_at_local"`
	Weekday       *int    `json:"weekday"`
	OnlyUnread    *bool   `json:"only_unread"`
	PushWhenEmpty *bool   `json:"push_when_empty"`
	Enabled       *bool   `json:"enabled"`
	ChannelIDs    []int64 `json:"channel_ids"`
}

// ---------- 渠道 ----------

func maskChannelURL(kind, raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return "****"
	}
	return parsed.Scheme + "://" + parsed.Host + "/****"
}

func (a *App) handleListChannels(w http.ResponseWriter, r *http.Request) error {
	channels, err := a.listChannels(r.Context())
	if err != nil {
		return internalError(err)
	}
	writeJSON(w, http.StatusOK, channels)
	return nil
}

func (a *App) listChannels(ctx context.Context) ([]PushChannel, error) {
	rows, err := a.db.QueryContext(ctx, `
		SELECT id, name, kind, url_enc, secret_enc, enabled, created_at, updated_at
		FROM push_channels ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PushChannel{}
	for rows.Next() {
		var ch PushChannel
		var kind, urlEnc string
		var secretEnc sql.NullString
		var enabled int
		if err := rows.Scan(&ch.ID, &ch.Name, &kind, &urlEnc, &secretEnc, &enabled, &ch.CreatedAt, &ch.UpdatedAt); err != nil {
			return nil, err
		}
		ch.Kind = kind
		ch.Enabled = enabled == 1
		ch.URLMasked = maskChannelURL(kind, urlEnc)
		ch.HasSecret = secretEnc.Valid && secretEnc.String != ""
		out = append(out, ch)
	}
	return out, rows.Err()
}

func (a *App) handleCreateChannel(w http.ResponseWriter, r *http.Request) error {
	var in channelInput
	if err := decodeJSON(r, &in); err != nil {
		return badRequest("invalid_body", "请求体无效")
	}
	if err := validateChannelInput(&in, true); err != nil {
		return err
	}
	enc, err := a.secret.Encrypt(strings.TrimSpace(in.URL))
	if err != nil {
		return internalError(err)
	}
	var secretEnc any
	if in.Secret != nil && strings.TrimSpace(*in.Secret) != "" {
		secret, secErr := a.secret.Encrypt(strings.TrimSpace(*in.Secret))
		if secErr != nil {
			return internalError(secErr)
		}
		secretEnc = secret
	}
	enabled := in.Enabled == nil || *in.Enabled
	now := a.now().Format(timeRFC3339)
	result, err := a.db.ExecContext(r.Context(), `
		INSERT INTO push_channels (name, kind, url_enc, secret_enc, enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		strings.TrimSpace(in.Name), in.Kind, enc, secretEnc, boolInt(enabled), now, now)
	if err != nil {
		return internalError(err)
	}
	id, _ := result.LastInsertId()
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
	return nil
}

func (a *App) handleUpdateChannel(w http.ResponseWriter, r *http.Request) error {
	id, err := apiRouteID(r)
	if err != nil {
		return err
	}
	var in channelInput
	if err := decodeJSON(r, &in); err != nil {
		return badRequest("invalid_body", "请求体无效")
	}
	if err := validateChannelInput(&in, strings.TrimSpace(in.URL) != ""); err != nil {
		return err
	}
	ctx := r.Context()
	setArgs := []any{strings.TrimSpace(in.Name)}
	setSQL := "name = ?"
	if strings.TrimSpace(in.URL) != "" {
		enc, encErr := a.secret.Encrypt(strings.TrimSpace(in.URL))
		if encErr != nil {
			return internalError(encErr)
		}
		setSQL += ", url_enc = ?"
		setArgs = append(setArgs, enc)
	}
	if in.Secret != nil {
		trimmed := strings.TrimSpace(*in.Secret)
		if trimmed == "" {
			setSQL += ", secret_enc = NULL"
		} else {
			secret, secErr := a.secret.Encrypt(trimmed)
			if secErr != nil {
				return internalError(secErr)
			}
			setSQL += ", secret_enc = ?"
			setArgs = append(setArgs, secret)
		}
	}
	if in.Enabled != nil {
		setSQL += ", enabled = ?"
		setArgs = append(setArgs, boolInt(*in.Enabled))
	}
	setSQL += ", updated_at = ?"
	setArgs = append(setArgs, a.now().Format(timeRFC3339), id)
	if _, err = a.db.ExecContext(ctx, `UPDATE push_channels SET `+setSQL+` WHERE id = ?`, setArgs...); err != nil {
		return internalError(err)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	return nil
}

func (a *App) handleDeleteChannel(w http.ResponseWriter, r *http.Request) error {
	id, err := apiRouteID(r)
	if err != nil {
		return err
	}
	if _, err := a.db.ExecContext(r.Context(), `DELETE FROM push_channels WHERE id = ?`, id); err != nil {
		return internalError(err)
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (a *App) handleTestChannel(w http.ResponseWriter, r *http.Request) error {
	id, err := apiRouteID(r)
	if err != nil {
		return err
	}
	var kind, urlEnc string
	var secretEnc sql.NullString
	err = a.db.QueryRowContext(r.Context(), `SELECT kind, url_enc, secret_enc FROM push_channels WHERE id = ?`, id).
		Scan(&kind, &urlEnc, &secretEnc)
	if err != nil {
		if err == sql.ErrNoRows {
			return notFound("not_found", "渠道不存在")
		}
		return internalError(err)
	}
	raw, decErr := a.secret.Decrypt(urlEnc)
	if decErr != nil {
		return &apiError{Status: http.StatusConflict, Code: "channel_url_unreadable", Message: "渠道 URL 无法用当前 secret.key 解密"}
	}
	var secret string
	if secretEnc.Valid && secretEnc.String != "" {
		if secret, decErr = a.secret.Decrypt(secretEnc.String); decErr != nil {
			return &apiError{Status: http.StatusConflict, Code: "channel_url_unreadable", Message: "渠道密钥无法用当前 secret.key 解密"}
		}
	}
	if perr := postToChannel(r.Context(), kind, raw, secret, "Rill 测试消息", "这是一条测试推送，收到即说明渠道配置正确。", 1); perr != nil {
		return &apiError{Status: http.StatusBadGateway, Code: "channel_push_failed", Message: perr.Error()}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	return nil
}

func validateChannelInput(in *channelInput, needURL bool) *apiError {
	in.Name = strings.TrimSpace(in.Name)
	in.Kind = strings.TrimSpace(in.Kind)
	in.URL = strings.TrimSpace(in.URL)
	if in.Name == "" {
		return badRequest("invalid_name", "渠道名称不能为空")
	}
	if in.Kind != "" && in.Kind != "feishu" && in.Kind != "generic_webhook" {
		return badRequest("invalid_kind", "渠道类型只支持 feishu 或 generic_webhook")
	}
	if needURL {
		if in.URL == "" {
			return badRequest("invalid_url", "webhook 地址不能为空")
		}
		parsed, err := url.Parse(in.URL)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return badRequest("invalid_url", "webhook 地址必须是 http/https URL")
		}
	}
	if needURL && in.Kind == "" {
		return badRequest("invalid_kind", "新建渠道必须指定类型")
	}
	return nil
}

// ---------- 报告规则 ----------

func (a *App) handleListDigestRules(w http.ResponseWriter, r *http.Request) error {
	rules, err := a.listDigestRules(r.Context())
	if err != nil {
		return internalError(err)
	}
	writeJSON(w, http.StatusOK, rules)
	return nil
}

func (a *App) listDigestRules(ctx context.Context) ([]DigestRule, error) {
	rows, err := a.db.QueryContext(ctx, `
		SELECT id, name, scope_json, period, run_at_local, weekday, only_unread,
		       push_when_empty, enabled, channel_ids_json, last_run_at, next_run_at, created_at, updated_at
		FROM digest_rules ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DigestRule{}
	for rows.Next() {
		var rule DigestRule
		var scope, channelIDs string
		var period, runAt string
		var weekday, onlyUnread, pushEmpty, enabled int
		if err := rows.Scan(&rule.ID, &rule.Name, &scope, &period, &runAt, &weekday,
			&onlyUnread, &pushEmpty, &enabled, &channelIDs, &rule.LastRunAt, &rule.NextRunAt,
			&rule.CreatedAt, &rule.UpdatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(scope), &rule.Scope)
		if rule.ChannelIDs == nil {
			rule.ChannelIDs = []int64{}
		}
		_ = json.Unmarshal([]byte(channelIDs), &rule.ChannelIDs)
		rule.Period = period
		rule.RunAtLocal = runAt
		rule.Weekday = weekday
		rule.OnlyUnread = onlyUnread == 1
		rule.PushWhenEmpty = pushEmpty == 1
		rule.Enabled = enabled == 1
		out = append(out, rule)
	}
	return out, rows.Err()
}

func validateDigestRuleInput(in *digestRuleInput) *apiError {
	in.Name = strings.TrimSpace(in.Name)
	in.Period = strings.TrimSpace(in.Period)
	in.RunAtLocal = strings.TrimSpace(in.RunAtLocal)
	if in.Name == "" {
		return badRequest("invalid_name", "规则名称不能为空")
	}
	if in.Period != "daily" && in.Period != "weekly" {
		return badRequest("invalid_period", "周期只支持 daily 或 weekly")
	}
	if _, err := time.Parse("15:04", in.RunAtLocal); err != nil {
		return badRequest("invalid_run_at", "运行时间必须是 HH:MM")
	}
	if in.Period == "weekly" {
		wd := 1
		if in.Weekday != nil {
			wd = *in.Weekday
		}
		if wd < 1 || wd > 7 {
			return badRequest("invalid_weekday", "weekday 需在 1（周一）–7（周日）之间")
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
	seenChannels := make(map[int64]bool, len(in.ChannelIDs))
	for _, id := range in.ChannelIDs {
		if id <= 0 {
			return badRequest("invalid_channel_ids", "推送渠道 ID 必须是正整数")
		}
		if seenChannels[id] {
			return badRequest("invalid_channel_ids", "推送渠道不能重复选择")
		}
		seenChannels[id] = true
	}
	return nil
}

func (a *App) handleCreateDigestRule(w http.ResponseWriter, r *http.Request) error {
	var in digestRuleInput
	if err := decodeJSON(r, &in); err != nil {
		return badRequest("invalid_body", "请求体无效")
	}
	if err := validateDigestRuleInput(&in); err != nil {
		return err
	}
	scopeJSON, _ := json.Marshal(in.Scope)
	if in.ChannelIDs == nil {
		in.ChannelIDs = []int64{}
	}
	channelJSON, _ := json.Marshal(in.ChannelIDs)
	weekday := 1
	if in.Weekday != nil {
		weekday = *in.Weekday
	}
	enabled := in.Enabled == nil || *in.Enabled
	onlyUnread := in.OnlyUnread != nil && *in.OnlyUnread
	pushEmpty := in.PushWhenEmpty != nil && *in.PushWhenEmpty
	now := a.now()
	next := nextDigestRun(now, in.Period, in.RunAtLocal, weekday, a.digestLocation)
	result, err := a.db.ExecContext(r.Context(), `
		INSERT INTO digest_rules (name, scope_json, period, run_at_local, weekday,
			only_unread, push_when_empty, enabled, channel_ids_json, next_run_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		in.Name, string(scopeJSON), in.Period, in.RunAtLocal, weekday,
		boolInt(onlyUnread), boolInt(pushEmpty), boolInt(enabled), string(channelJSON),
		next.Format(timeRFC3339), now.Format(timeRFC3339), now.Format(timeRFC3339))
	if err != nil {
		return internalError(err)
	}
	id, _ := result.LastInsertId()
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
	return nil
}

func (a *App) handleUpdateDigestRule(w http.ResponseWriter, r *http.Request) error {
	id, err := apiRouteID(r)
	if err != nil {
		return err
	}
	var in digestRuleInput
	if err := decodeJSON(r, &in); err != nil {
		return badRequest("invalid_body", "请求体无效")
	}
	if err := validateDigestRuleInput(&in); err != nil {
		return err
	}
	scopeJSON, _ := json.Marshal(in.Scope)
	var channelArg any
	if in.ChannelIDs != nil {
		channelJSON, _ := json.Marshal(in.ChannelIDs)
		channelArg = string(channelJSON)
	}
	weekday := 1
	if in.Weekday != nil {
		weekday = *in.Weekday
	}
	var onlyUnreadArg, pushEmptyArg, enabledArg any
	if in.OnlyUnread != nil {
		onlyUnreadArg = boolInt(*in.OnlyUnread)
	}
	if in.PushWhenEmpty != nil {
		pushEmptyArg = boolInt(*in.PushWhenEmpty)
	}
	if in.Enabled != nil {
		enabledArg = boolInt(*in.Enabled)
	}
	now := a.now()
	next := nextDigestRun(now, in.Period, in.RunAtLocal, weekday, a.digestLocation)
	if _, err := a.db.ExecContext(r.Context(), `
		UPDATE digest_rules SET name = ?, scope_json = ?, period = ?, run_at_local = ?, weekday = ?,
			only_unread = COALESCE(?, only_unread), push_when_empty = COALESCE(?, push_when_empty),
			enabled = COALESCE(?, enabled), channel_ids_json = COALESCE(?, channel_ids_json),
			next_run_at = ?, updated_at = ?
		WHERE id = ?`,
		in.Name, string(scopeJSON), in.Period, in.RunAtLocal, weekday,
		onlyUnreadArg, pushEmptyArg, enabledArg, channelArg,
		next.Format(timeRFC3339), now.Format(timeRFC3339), id); err != nil {
		return internalError(err)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	return nil
}

func (a *App) handleDeleteDigestRule(w http.ResponseWriter, r *http.Request) error {
	id, err := apiRouteID(r)
	if err != nil {
		return err
	}
	if _, err := a.db.ExecContext(r.Context(), `DELETE FROM digest_rules WHERE id = ?`, id); err != nil {
		return internalError(err)
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (a *App) handleRunDigestRule(w http.ResponseWriter, r *http.Request) error {
	id, err := apiRouteID(r)
	if err != nil {
		return err
	}
	report, err := a.runDigestRule(r.Context(), id)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, report)
	return nil
}

// ---------- 报告 ----------

func (a *App) handleListDigestReports(w http.ResponseWriter, r *http.Request) error {
	query := `
		SELECT r.id, r.rule_id, r.period_start, r.period_end, r.item_count, r.status,
		       r.content_md, r.model, r.push_status, r.push_error, r.pushed_at, r.created_at,
		       COALESCE(dr.name, '')
		FROM digest_reports r LEFT JOIN digest_rules dr ON dr.id = r.rule_id`
	var args []any
	conditions := []string{}
	if raw := strings.TrimSpace(r.URL.Query().Get("rule_id")); raw != "" {
		var ruleID int64
		if _, err := fmt.Sscanf(raw, "%d", &ruleID); err != nil || ruleID <= 0 {
			return badRequest("invalid_rule_id", "rule_id 无效")
		}
		conditions = append(conditions, "r.rule_id = ?")
		args = append(args, ruleID)
	}
	limit := 50
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if _, err := fmt.Sscanf(raw, "%d", &limit); err != nil || limit < 1 || limit > 200 {
			return badRequest("invalid_limit", "limit 需在 1–200 之间")
		}
	}
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += fmt.Sprintf(" ORDER BY r.created_at DESC, r.id DESC LIMIT %d", limit)
	rows, err := a.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		return internalError(err)
	}
	defer rows.Close()
	out := []DigestReport{}
	for rows.Next() {
		report, scanErr := scanDigestReport(rows)
		if scanErr != nil {
			return internalError(scanErr)
		}
		out = append(out, report)
	}
	if err := rows.Err(); err != nil {
		return internalError(err)
	}
	writeJSON(w, http.StatusOK, out)
	return nil
}

func scanDigestReport(rows *sql.Rows) (DigestReport, error) {
	var report DigestReport
	var status, pushStatus string
	var ruleName string
	if err := rows.Scan(&report.ID, &report.RuleID, &report.PeriodStart, &report.PeriodEnd,
		&report.ItemCount, &status, &report.ContentMD, &report.Model, &pushStatus,
		&report.PushError, &report.PushedAt, &report.CreatedAt, &ruleName); err != nil {
		return report, err
	}
	report.Status = status
	report.PushStatus = pushStatus
	report.RuleName = ruleName
	return report, nil
}

func (a *App) handleGetDigestReport(w http.ResponseWriter, r *http.Request) error {
	id, err := apiRouteID(r)
	if err != nil {
		return err
	}
	rows, err := a.db.QueryContext(r.Context(), `
		SELECT r.id, r.rule_id, r.period_start, r.period_end, r.item_count, r.status,
		       r.content_md, r.model, r.push_status, r.push_error, r.pushed_at, r.created_at,
		       COALESCE(dr.name, '')
		FROM digest_reports r LEFT JOIN digest_rules dr ON dr.id = r.rule_id
		WHERE r.id = ?`, id)
	if err != nil {
		return internalError(err)
	}
	defer rows.Close()
	if !rows.Next() {
		return notFound("not_found", "报告不存在")
	}
	report, err := scanDigestReport(rows)
	if err != nil {
		return internalError(err)
	}
	writeJSON(w, http.StatusOK, report)
	return nil
}

func (a *App) handlePushDigestReport(w http.ResponseWriter, r *http.Request) error {
	id, err := apiRouteID(r)
	if err != nil {
		return err
	}
	pushStatus, pushErr := a.pushReport(r.Context(), id)
	if pushErr != nil {
		return internalError(pushErr)
	}
	writeJSON(w, http.StatusOK, map[string]any{"push_status": pushStatus})
	return nil
}

// ---------- 生成管线 ----------

type digestItem struct {
	ID      int64
	Title   string
	Source  string
	URL     string
	PubAt   string
	Excerpt string
	Summary string // 已缓存的 AI 摘要（可能为空）
}

// resolveDigestScope 把 scope_json（源 ID / "folder:<id>"）解析成源 ID 集合。
func (a *App) resolveDigestScope(ctx context.Context, scope []any) ([]int64, error) {
	if len(scope) == 0 {
		rows, err := a.db.QueryContext(ctx, `SELECT id FROM sources`)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var ids []int64
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				return nil, err
			}
			ids = append(ids, id)
		}
		return ids, rows.Err()
	}
	var directIDs []int64
	var folderIDs []int64
	for _, entry := range scope {
		switch v := entry.(type) {
		case float64:
			directIDs = append(directIDs, int64(v))
		case json.Number:
			if n, err := v.Int64(); err == nil && n > 0 {
				directIDs = append(directIDs, n)
			}
		case string:
			if strings.HasPrefix(v, "folder:") {
				var fid int64
				if _, err := fmt.Sscanf(strings.TrimPrefix(v, "folder:"), "%d", &fid); err == nil && fid > 0 {
					folderIDs = append(folderIDs, fid)
				}
			}
		}
	}
	idSet := map[int64]struct{}{}
	for _, id := range directIDs {
		idSet[id] = struct{}{}
	}
	for _, fid := range folderIDs {
		// 含子文件夹：先收集后代文件夹。
		folderSet := map[int64]struct{}{fid: {}}
		changed := true
		for changed {
			changed = false
			rows, err := a.db.QueryContext(ctx, `SELECT id, parent_id FROM folders`)
			if err != nil {
				return nil, err
			}
			type fr struct {
				id       int64
				parentID sql.NullInt64
			}
			var all []fr
			for rows.Next() {
				var f fr
				if err := rows.Scan(&f.id, &f.parentID); err != nil {
					_ = rows.Close()
					return nil, err
				}
				all = append(all, f)
			}
			rows.Close()
			for _, f := range all {
				if f.parentID.Valid {
					if _, ok := folderSet[f.parentID.Int64]; ok {
						if _, seen := folderSet[f.id]; !seen {
							folderSet[f.id] = struct{}{}
							changed = true
						}
					}
				}
			}
		}
		placeholders := make([]string, 0, len(folderSet))
		args := make([]any, 0, len(folderSet))
		for id := range folderSet {
			placeholders = append(placeholders, "?")
			args = append(args, id)
		}
		rows, err := a.db.QueryContext(ctx,
			`SELECT id FROM sources WHERE folder_id IN (`+strings.Join(placeholders, ",")+")", args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				_ = rows.Close()
				return nil, err
			}
			idSet[id] = struct{}{}
		}
		rows.Close()
	}
	ids := make([]int64, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}
	return ids, nil
}

func (a *App) repairDigestNextRuns(ctx context.Context) {
	rows, err := a.db.QueryContext(ctx, `
		SELECT id, period, run_at_local, weekday, last_run_at, next_run_at FROM digest_rules`)
	if err != nil {
		a.logger.Warn("digest schedule repair skipped", "err", err)
		return
	}
	type row struct {
		id      int64
		period  string
		runAt   string
		weekday int
		lastRun sql.NullString
		nextRun sql.NullString
	}
	var list []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.period, &r.runAt, &r.weekday, &r.lastRun, &r.nextRun); err != nil {
			_ = rows.Close()
			a.logger.Warn("digest schedule repair scan failed", "err", err)
			return
		}
		list = append(list, r)
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		a.logger.Warn("digest schedule repair read failed", "err", err)
		return
	}
	for _, r := range list {
		if r.nextRun.Valid && r.nextRun.String != "" {
			if parsed, perr := time.Parse(timeRFC3339, r.nextRun.String); perr == nil {
				if parsed.In(a.digestLocation).Format("15:04") == r.runAt {
					continue
				}
			}
		}
		now := a.now()
		next := nextDigestRun(now, r.period, r.runAt, r.weekday, a.digestLocation)
		// 存量规则曾把本地墙钟时间当 UTC 计算。未运行过的规则如果目标
		// 时间今天已过，立即补跑一次，避免用户升级后还要等到明天。
		if !r.lastRun.Valid || r.lastRun.String == "" {
			localNow := now.In(a.digestLocation)
			if t, terr := time.Parse("15:04", r.runAt); terr == nil {
				intendedToday := time.Date(localNow.Year(), localNow.Month(), localNow.Day(),
					t.Hour(), t.Minute(), 0, 0, a.digestLocation)
				if !intendedToday.After(localNow) {
					next = now
				}
			}
		}
		if _, err := a.db.ExecContext(ctx, `
			UPDATE digest_rules SET next_run_at = ?, updated_at = ? WHERE id = ?`,
			next.Format(timeRFC3339), now.Format(timeRFC3339), r.id); err != nil {
			a.logger.Warn("digest schedule repair failed", "rule_id", r.id, "err", err)
			continue
		}
		a.logger.Info("digest schedule repaired", "rule_id", r.id, "next_run_at", next.Format(timeRFC3339))
	}
}

func nextDigestRun(now time.Time, period, runAtLocal string, weekday int, loc *time.Location) time.Time {
	if loc == nil {
		loc = time.Local
	}
	t, err := time.Parse("15:04", runAtLocal)
	if err != nil {
		t, _ = time.Parse("15:04", "09:00")
	}
	hour, minute := t.Hour(), t.Minute()
	// run_at_local 是用户看到的墙钟时间，必须在该时区里计算；返回 UTC
	// 绝对时间，保证 next_run_at 的字符串比较与跨时区显示一致。
	localNow := now.In(loc)
	candidate := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), hour, minute, 0, 0, loc)
	if period == "daily" {
		if !candidate.After(localNow) {
			candidate = candidate.AddDate(0, 0, 1)
		}
		return candidate.UTC()
	}
	// weekly：1=周一…7=周日；time.Weekday() 周日=0
	target := weekday % 7 // 1→1 … 6→6, 7→0
	for {
		if int(candidate.Weekday()) == target && candidate.After(localNow) {
			return candidate.UTC()
		}
		candidate = candidate.AddDate(0, 0, 1)
	}
}

func (a *App) maybeRunDigests(ctx context.Context) {
	now := a.now().Format(timeRFC3339)
	rows, err := a.db.QueryContext(ctx, `
		SELECT id FROM digest_rules WHERE enabled = 1 AND next_run_at IS NOT NULL AND next_run_at <= ?`, now)
	if err != nil {
		a.logger.Error("digest query failed", "err", err)
		return
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			a.logger.Error("digest scan failed", "err", err)
			return
		}
		ids = append(ids, id)
	}
	rows.Close()
	for _, id := range ids {
		if _, err := a.runDigestRule(ctx, id); err != nil {
			a.logger.Warn("digest run failed", "rule_id", id, "err", err)
		}
	}
}

func (a *App) runDigestRule(ctx context.Context, ruleID int64) (*DigestReport, error) {
	var name, scopeJSON, channelIDsJSON, period, runAt string
	var weekday, onlyUnread, pushEmpty, enabled int
	var lastRun, nextRun sql.NullString
	err := a.db.QueryRowContext(ctx, `
		SELECT name, scope_json, channel_ids_json, period, run_at_local, weekday, only_unread,
		       push_when_empty, enabled, last_run_at, next_run_at
		FROM digest_rules WHERE id = ?`, ruleID).
		Scan(&name, &scopeJSON, &channelIDsJSON, &period, &runAt, &weekday, &onlyUnread, &pushEmpty, &enabled, &lastRun, &nextRun)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, notFound("not_found", "规则不存在")
		}
		return nil, internalError(err)
	}
	var scope []any
	_ = json.Unmarshal([]byte(scopeJSON), &scope)
	var channelIDs []int64
	_ = json.Unmarshal([]byte(channelIDsJSON), &channelIDs)

	now := a.now()
	periodEnd := now
	var periodStart time.Time
	if lastRun.Valid && lastRun.String != "" {
		if parsed, perr := time.Parse(timeRFC3339, lastRun.String); perr == nil {
			periodStart = parsed
		} else {
			periodStart = now.AddDate(0, 0, -1)
		}
	} else {
		if period == "weekly" {
			periodStart = now.AddDate(0, 0, -7)
		} else {
			periodStart = now.AddDate(0, 0, -1)
		}
	}

	sourceIDs, err := a.resolveDigestScope(ctx, scope)
	if err != nil {
		return nil, internalError(err)
	}
	items, err := a.digestWindowItems(ctx, sourceIDs, periodStart, periodEnd, onlyUnread == 1)
	if err != nil {
		return nil, internalError(err)
	}

	truncated := false
	totalCount := len(items)
	if len(items) > digestMaxItems {
		items = items[:digestMaxItems]
		truncated = true
	}

	status := "ok"
	modelName := ""
	var bodyMD string
	if len(items) == 0 {
		status = "empty"
		bodyMD = fmt.Sprintf("窗口内（%s ~ %s）没有新文章。",
			periodStart.In(a.digestLocation).Format("01-02 15:04"),
			periodEnd.In(a.digestLocation).Format("01-02 15:04"))
	} else {
		bodyMD, modelName, status = a.generateDigestBody(ctx, name, items, periodStart, periodEnd)
		if truncated {
			bodyMD = fmt.Sprintf("> 窗口内共 %d 条，已截取最新 %d 条生成报告。\n\n%s", totalCount, digestMaxItems, bodyMD)
		}
	}

	nowStr := now.Format(timeRFC3339)
	var modelAny any
	if modelName != "" {
		modelAny = modelName
	}
	result, err := a.db.ExecContext(ctx, `
		INSERT INTO digest_reports (rule_id, period_start, period_end, item_count, status,
			content_md, model, push_status, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, 'none', ?)`,
		ruleID, periodStart.Format(timeRFC3339), periodEnd.Format(timeRFC3339),
		totalCount, status, bodyMD, modelAny, nowStr)
	if err != nil {
		return nil, internalError(err)
	}
	reportID, _ := result.LastInsertId()

	next := nextDigestRun(now, period, runAt, weekday, a.digestLocation)
	if _, err := a.db.ExecContext(ctx, `
		UPDATE digest_rules SET last_run_at = ?, next_run_at = ?, updated_at = ? WHERE id = ?`,
		nowStr, next.Format(timeRFC3339), nowStr, ruleID); err != nil {
		return nil, internalError(err)
	}

	// 空报告默认不推送；手动 run 也照常推送（用户显式要求）。
	if status != "empty" || pushEmpty == 1 {
		if _, perr := a.pushReport(ctx, reportID); perr != nil {
			a.logger.Warn("digest push failed", "rule_id", ruleID, "report_id", reportID, "err", perr)
		}
	}

	return a.getDigestReport(ctx, reportID)
}

func (a *App) getDigestReport(ctx context.Context, id int64) (*DigestReport, error) {
	rows, err := a.db.QueryContext(ctx, `
		SELECT r.id, r.rule_id, r.period_start, r.period_end, r.item_count, r.status,
		       r.content_md, r.model, r.push_status, r.push_error, r.pushed_at, r.created_at,
		       COALESCE(dr.name, '')
		FROM digest_reports r LEFT JOIN digest_rules dr ON dr.id = r.rule_id
		WHERE r.id = ?`, id)
	if err != nil {
		return nil, internalError(err)
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, notFound("not_found", "报告不存在")
	}
	report, err := scanDigestReport(rows)
	if err != nil {
		return nil, internalError(err)
	}
	return &report, nil
}

func (a *App) digestWindowItems(ctx context.Context, sourceIDs []int64, start, end time.Time, onlyUnread bool) ([]digestItem, error) {
	if len(sourceIDs) == 0 {
		return []digestItem{}, nil
	}
	placeholders := make([]string, 0, len(sourceIDs))
	args := make([]any, 0, len(sourceIDs)+3)
	for _, id := range sourceIDs {
		placeholders = append(placeholders, "?")
		args = append(args, id)
	}
	query := fmt.Sprintf(`
		SELECT i.id, COALESCE(i.title, ''), COALESCE(s.name, ''), COALESCE(i.url, ''),
		       COALESCE(COALESCE(i.published_at, i.fetched_at), ''),
		       COALESCE(i.content_text, '')
		FROM items i JOIN sources s ON s.id = i.source_id
		WHERE i.hidden = 0 AND i.source_id IN (%s)
		  AND COALESCE(i.published_at, i.fetched_at) >= ?
		  AND COALESCE(i.published_at, i.fetched_at) <= ?`,
		strings.Join(placeholders, ","))
	args = append(args, start.Format(timeRFC3339), end.Format(timeRFC3339))
	if onlyUnread {
		query += " AND i.read_at IS NULL"
	}
	query += fmt.Sprintf(" ORDER BY COALESCE(i.published_at, i.fetched_at) DESC LIMIT %d", digestMaxItems)
	rows, err := a.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []digestItem{}
	for rows.Next() {
		var item digestItem
		if err := rows.Scan(&item.ID, &item.Title, &item.Source, &item.URL, &item.PubAt, &item.Excerpt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func truncateDigestRunes(s string, max int) string {
	runes := []rune(strings.TrimSpace(s))
	if len(runes) <= max {
		return string(runes)
	}
	return string(runes[:max]) + "…"
}

// generateDigestBody 返回报告正文 Markdown、模型名与状态。
// AI 未启用/失败 → 列表降级（status error）；部分批失败 → partial。
func (a *App) generateDigestBody(ctx context.Context, ruleName string, items []digestItem, start, end time.Time) (string, string, string) {
	startLabel := start.In(a.digestLocation).Format("2006-01-02 15:04")
	endLabel := end.In(a.digestLocation).Format("01-02 15:04")
	header := fmt.Sprintf("# %s（%s ~ %s）\n\n共 %d 条新文章。\n",
		ruleName, startLabel, endLabel, len(items))
	listMD := "\n## 本期文章\n\n" + digestListMD(items)

	runtime, apiErr := a.aiRuntime(ctx)
	if apiErr != nil {
		return header + "\n（AI 未启用或不可用，以下为文章列表。）\n" + listMD, "", "error"
	}

	// 先取缓存摘要，缺失的进分批小结。
	chunks := make([]string, 0, len(items)/digestBatchSize+1)
	pending := make([]digestItem, 0, digestBatchSize)
	flushBatch := func() {
		if len(pending) == 0 {
			return
		}
		if md, ok := a.digestBatchSummary(ctx, runtime, pending); ok {
			chunks = append(chunks, md)
		}
		pending = pending[:0]
	}
	var cachedParts []string
	for _, item := range items {
		summary, err := a.loadSummary(ctx, item.ID)
		if err == nil && len(summary.Points) > 0 {
			cachedParts = append(cachedParts, fmt.Sprintf("### %s（%s）\n\n%s", item.Title, item.Source, truncateDigestRunes(strings.Join(summary.Points, "\n"), 400)))
			if len(cachedParts) >= digestBatchSize {
				chunks = append(chunks, strings.Join(cachedParts, "\n\n"))
				cachedParts = cachedParts[:0]
			}
			continue
		}
		pending = append(pending, item)
		if len(pending) >= digestBatchSize {
			flushBatch()
		}
	}
	flushBatch()
	if len(cachedParts) > 0 {
		chunks = append(chunks, strings.Join(cachedParts, "\n\n"))
	}

	allFailed := len(chunks) == 0 && len(items) > 0
	if allFailed {
		return header + "\n（AI 摘要生成失败，以下为文章列表。）\n" + listMD, runtime.settings.AI.Model, "error"
	}

	// 合并层：批小结 > digestMergeThresh 时树状归并。
	merged := chunks
	for len(merged) > digestMergeThresh {
		nextLevel := make([]string, 0, len(merged)/digestMergeGroup+1)
		for i := 0; i < len(merged); i += digestMergeGroup {
			end := i + digestMergeGroup
			if end > len(merged) {
				end = len(merged)
			}
			group := merged[i:end]
			prompt := "以下是一份订阅报告的分组小结。请把这几组合并成一份更精炼的小结：" +
				"只保留主题与关键事实，合并重复内容，不要逐条复述文章；" +
				"总长度不超过 800 字，用 Markdown 列表输出：\n\n" +
				strings.Join(group, "\n\n")
			if text, _, ok := a.digestChat(ctx, runtime, prompt, 2000); ok {
				nextLevel = append(nextLevel, text)
			} else {
				nextLevel = append(nextLevel, group...)
			}
		}
		if len(nextLevel) >= len(merged) {
			break // 无法收敛时直接用当前层
		}
		merged = nextLevel
	}

	mergePrompt := buildDigestMergePrompt(
		ruleName, startLabel, endLabel, len(items),
		runtime.settings.AI.Prompts.DigestMergeInstruction(),
		strings.Join(merged, "\n\n"))
	status := "ok"
	body, _, ok := a.digestChat(ctx, runtime, mergePrompt, 1800)
	if !ok {
		body = strings.Join(merged, "\n\n")
		status = "partial"
	}
	return header + "\n" + body + "\n" + listMD, runtime.settings.AI.Model, status
}

func digestListMD(items []digestItem) string {
	var b strings.Builder
	for _, item := range items {
		line := fmt.Sprintf("- [%s](%s) — %s · %s", item.Title, item.URL, item.Source, item.PubAt)
		if line == fmt.Sprintf("- [](%s) — %s · %s", item.URL, item.Source, item.PubAt) {
			line = fmt.Sprintf("- [未命名](%s) — %s · %s", item.URL, item.Source, item.PubAt)
		}
		b.WriteString(line + "\n")
	}
	return b.String()
}

// digestPushBody 去掉推送里冗余的完整文章清单：AI 已生成简报时，飞书只发简报，
// 完整列表留在 Rill 内查看，避免接近飞书卡片体积上限被截断。
func digestPushBody(contentMD, status string) string {
	if status == "error" {
		return contentMD // AI 不可用时列表就是报告本体，必须保留
	}
	if idx := strings.Index(contentMD, "\n## 本期文章"); idx >= 0 {
		return strings.TrimSpace(contentMD[:idx]) + "\n\n完整文章列表与链接请在 Rill 中查看。"
	}
	return contentMD
}

func (a *App) digestBatchSummary(ctx context.Context, runtime *aiRuntime, items []digestItem) (string, bool) {
	var b strings.Builder
	for i, item := range items {
		fmt.Fprintf(&b, "%d. 「%s」（来源：%s）\n   正文节选：%s\n\n",
			i+1, item.Title, item.Source, truncateDigestRunes(item.Excerpt, digestExcerptRunes))
	}
	prompt := buildDigestBatchPrompt(runtime.settings.AI.Prompts.DigestBatchInstruction(), b.String())
	if text, _, ok := a.digestChat(ctx, runtime, prompt, 1600); ok {
		return text, true
	}
	return "", false
}

func (a *App) digestChat(ctx context.Context, runtime *aiRuntime, prompt string, maxTokens int) (string, int64, bool) {
	text, tokens, apiErr := a.chatCompletion(ctx, runtime, []chatMessage{{Role: "user", Content: prompt}}, maxTokens)
	if apiErr != nil {
		a.logger.Warn("digest ai call failed", "code", apiErr.Code)
		return "", 0, false
	}
	if err := a.recordAIUsage(ctx, tokens); err != nil {
		a.logger.Warn("digest usage record failed", "err", err)
	}
	return text, tokens, true
}

// ---------- 推送 ----------

func (a *App) pushReport(ctx context.Context, reportID int64) (string, error) {
	var contentMD, title, status string
	var ruleID int64
	var channelIDsJSON sql.NullString
	err := a.db.QueryRowContext(ctx, `
		SELECT r.rule_id, r.content_md, r.status, COALESCE(dr.name, ''), dr.channel_ids_json
		FROM digest_reports r LEFT JOIN digest_rules dr ON dr.id = r.rule_id
		WHERE r.id = ?`, reportID).Scan(&ruleID, &contentMD, &status, &title, &channelIDsJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("报告不存在")
		}
		return "", err
	}
	var channelIDs []int64
	if channelIDsJSON.Valid {
		_ = json.Unmarshal([]byte(channelIDsJSON.String), &channelIDs)
	}
	if len(channelIDs) == 0 {
		if _, err := a.db.ExecContext(ctx, `
			UPDATE digest_reports SET push_status = 'none', push_error = '规则未选择推送渠道', pushed_at = NULL
			WHERE id = ?`, reportID); err != nil {
			return "", err
		}
		return "none", nil
	}
	placeholders := make([]string, 0, len(channelIDs))
	args := make([]any, 0, len(channelIDs))
	for _, id := range channelIDs {
		placeholders = append(placeholders, "?")
		args = append(args, id)
	}
	rows, err := a.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT kind, url_enc, secret_enc FROM push_channels
		WHERE enabled = 1 AND id IN (%s)`, strings.Join(placeholders, ",")), args...)
	if err != nil {
		return "", err
	}
	type target struct {
		kind   string
		urlEnc string
		secret sql.NullString
	}
	var targets []target
	for rows.Next() {
		var t target
		if err := rows.Scan(&t.kind, &t.urlEnc, &t.secret); err != nil {
			_ = rows.Close()
			return "", err
		}
		targets = append(targets, t)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return "", err
	}
	if len(targets) == 0 {
		if _, err := a.db.ExecContext(ctx, `
			UPDATE digest_reports SET push_status = 'none', push_error = '所选推送渠道均已停用或删除', pushed_at = NULL
			WHERE id = ?`, reportID); err != nil {
			return "", err
		}
		return "none", nil
	}
	okCount := 0
	var firstErr error
	for _, t := range targets {
		raw, decErr := a.secret.Decrypt(t.urlEnc)
		if decErr != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("渠道 URL 解密失败")
			}
			continue
		}
		var secret string
		if t.secret.Valid && t.secret.String != "" {
			if secret, decErr = a.secret.Decrypt(t.secret.String); decErr != nil {
				if firstErr == nil {
					firstErr = fmt.Errorf("渠道密钥解密失败")
				}
				continue
			}
		}
		bodyContent := contentMD
		if t.kind == "feishu" {
			bodyContent = digestPushBody(contentMD, status)
		}
		body := truncateDigestRunes(bodyContent, digestFeishuLimit)
		if perr := postToChannel(ctx, t.kind, raw, secret, title, body, 1); perr != nil {
			if firstErr == nil {
				firstErr = perr
			}
			continue
		}
		okCount++
	}
	pushedAt := a.now().Format(timeRFC3339)
	var pushStatus string
	var pushError any
	switch {
	case okCount == len(targets):
		pushStatus = "ok"
	case okCount == 0:
		pushStatus = "failed"
		if firstErr != nil {
			pushError = firstErr.Error()
		}
	default:
		pushStatus = "partial"
		if firstErr != nil {
			pushError = firstErr.Error()
		}
	}
	if _, err := a.db.ExecContext(ctx, `
		UPDATE digest_reports SET push_status = ?, push_error = ?, pushed_at = ? WHERE id = ?`,
		pushStatus, pushError, pushedAt, reportID); err != nil {
		return "", err
	}
	return pushStatus, nil
}

func postToChannel(ctx context.Context, kind, rawURL, secret, title, bodyMD string, itemCount int) error {
	client := &http.Client{Timeout: 30 * time.Second}
	var payload []byte
	switch kind {
	case "feishu":
		// 卡片 header 已显示规则名，正文不再重复标题，避免「X # X」。
		content := bodyMD
		if len([]rune(content)) > digestFeishuLimit {
			content = string([]rune(content)[:digestFeishuLimit]) + "\n\n…（Rill 为适配飞书消息长度截断了推送，完整报告请在 Rill 中查看）"
		}
		card := map[string]any{
			"config": map[string]any{"wide_screen_mode": true},
			"header": map[string]any{
				"title":    map[string]string{"tag": "plain_text", "content": title},
				"template": "green",
			},
			"elements": []any{
				map[string]any{"tag": "div", "text": map[string]string{"tag": "lark_md", "content": content}},
			},
		}
		body := map[string]any{"msg_type": "interactive", "card": card}
		if secret != "" {
			// 飞书自定义机器人签名校验：sign = base64(hmac_sha256(key=timestamp+"\n"+secret, message=""))
			timestamp := time.Now().Unix()
			mac := hmac.New(sha256.New, []byte(fmt.Sprintf("%d\n%s", timestamp, secret)))
			sign := base64.StdEncoding.EncodeToString(mac.Sum(nil))
			body["timestamp"] = fmt.Sprintf("%d", timestamp)
			body["sign"] = sign
		}
		payload, _ = json.Marshal(body)
	default: // generic_webhook
		payload, _ = json.Marshal(map[string]any{
			"app":     "rill",
			"title":   title,
			"content": bodyMD,
			"items":   itemCount,
		})
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("webhook 地址无效")
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("推送失败：%v", err)
	}
	defer response.Body.Close()
	replyBody, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("推送失败：HTTP %d %s", response.StatusCode, truncateDigestRunes(string(replyBody), 200))
	}
	// 飞书机器人即使在签名错误、频率超限时也返回 HTTP 200，必须检查业务码。
	if kind == "feishu" && len(replyBody) > 0 {
		var reply struct {
			Code          int    `json:"code"`
			StatusCode    int    `json:"StatusCode"`
			Msg           string `json:"msg"`
			StatusMessage string `json:"StatusMessage"`
		}
		if json.Unmarshal(replyBody, &reply) == nil {
			code := reply.Code
			if code == 0 {
				code = reply.StatusCode
			}
			if code != 0 {
				msg := reply.Msg
				if msg == "" {
					msg = reply.StatusMessage
				}
				return fmt.Errorf("飞书推送失败：%d %s", code, msg)
			}
		}
	}
	return nil
}
