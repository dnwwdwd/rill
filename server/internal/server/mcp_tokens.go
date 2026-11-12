package server

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"rill/internal/secure"
)

const mcpTokenPrefix = "rill_mcp_"

func newMCPToken() (string, string, string, error) {
	secret, err := secure.RandomSecret()
	if err != nil {
		return "", "", "", err
	}
	token := mcpTokenPrefix + secret
	return token, secure.HashToken(token), maskMCPToken(token), nil
}

func maskMCPToken(token string) string {
	if len(token) <= 18 {
		return secure.MaskToken(token)
	}
	return token[:13] + "…" + token[len(token)-4:]
}

func (a *App) mcpEnabled(ctx context.Context) (bool, error) {
	internal, err := a.loadInternalSettings(ctx)
	if err != nil {
		return false, err
	}
	return internal.MCP.Enabled, nil
}

func (a *App) lookupMCPToken(ctx context.Context, token string) (int64, bool, error) {
	if strings.TrimSpace(token) == "" {
		return 0, false, nil
	}
	hash := secure.HashToken(token)
	var id int64
	var enabled int
	err := a.db.QueryRowContext(ctx, `SELECT id, enabled FROM mcp_tokens WHERE token_hash = ?`, hash).Scan(&id, &enabled)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	if enabled != 1 {
		return 0, false, nil
	}
	// Avoid a write on every MCP request while still showing roughly when an
	// agent last used the token.
	cutoff := a.now().Add(-time.Minute).Format(timeRFC3339)
	_, _ = a.db.ExecContext(ctx, `
		UPDATE mcp_tokens SET last_used_at = ?
		WHERE id = ? AND (last_used_at IS NULL OR last_used_at < ?)`,
		a.now().Format(timeRFC3339), id, cutoff)
	return id, true, nil
}

func bearerToken(value string) string {
	parts := strings.Fields(strings.TrimSpace(value))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return parts[1]
}

// mcpMiddleware authenticates /mcp with a Bearer token. The endpoint is listed
// in the LPK public_path so external agents do not need a Lazycat session; every
// request still requires an enabled, named token stored as a SHA-256 hash.
type mcpOriginContextKey struct{}

func (a *App) mcpMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		enabled, err := a.mcpEnabled(r.Context())
		if err != nil {
			http.Error(w, "mcp settings unavailable", http.StatusInternalServerError)
			return
		}
		if !enabled {
			http.NotFound(w, r)
			return
		}
		token := bearerToken(r.Header.Get("Authorization"))
		if token == "" {
			w.Header().Set("WWW-Authenticate", `Bearer realm="rill-mcp"`)
			http.Error(w, "missing bearer token", http.StatusUnauthorized)
			return
		}
		if _, ok, lookupErr := a.lookupMCPToken(r.Context(), token); lookupErr != nil {
			http.Error(w, "token lookup failed", http.StatusInternalServerError)
			return
		} else if !ok {
			w.Header().Set("WWW-Authenticate", `Bearer realm="rill-mcp"`)
			http.Error(w, "invalid bearer token", http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), mcpOriginContextKey{}, requestOrigin(r))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (a *App) handleListMCPTokens(w http.ResponseWriter, r *http.Request) error {
	rows, err := a.db.QueryContext(r.Context(), `
		SELECT id, name, masked, enabled, created_at, last_used_at
		FROM mcp_tokens ORDER BY id ASC`)
	if err != nil {
		return internalError(err)
	}
	defer rows.Close()
	result := make([]MCPToken, 0)
	for rows.Next() {
		var token MCPToken
		var enabled int
		var lastUsed sql.NullString
		if err := rows.Scan(&token.ID, &token.Name, &token.Masked, &enabled, &token.CreatedAt, &lastUsed); err != nil {
			return internalError(err)
		}
		token.Enabled = enabled == 1
		token.LastUsedAt = nullStringPtr(lastUsed)
		result = append(result, token)
	}
	if err := rows.Err(); err != nil {
		return internalError(err)
	}
	writeJSON(w, http.StatusOK, result)
	return nil
}

type mcpTokenCreateInput struct {
	Name string `json:"name"`
}

func (a *App) handleCreateMCPToken(w http.ResponseWriter, r *http.Request) error {
	var input mcpTokenCreateInput
	if err := decodeJSON(r, &input); err != nil {
		return err
	}
	name := strings.TrimSpace(input.Name)
	if name == "" || len([]rune(name)) > 100 {
		return badRequest("invalid_mcp_token", "令牌名称不能为空且不能超过 100 字")
	}
	token, hash, masked, err := newMCPToken()
	if err != nil {
		return internalError(err)
	}
	now := a.now().Format(timeRFC3339)
	result, err := a.db.ExecContext(r.Context(), `
		INSERT INTO mcp_tokens (name, masked, token_hash, enabled, created_at)
		VALUES (?, ?, ?, 1, ?)`, name, masked, hash, now)
	if err != nil {
		return internalError(err)
	}
	id, _ := result.LastInsertId()
	writeJSON(w, http.StatusCreated, MCPTokenCreated{
		MCPToken: MCPToken{ID: id, Name: name, Masked: masked, Enabled: true, CreatedAt: now},
		Token:    token,
	})
	return nil
}

type mcpTokenPatch struct {
	Name    *string `json:"name"`
	Enabled *bool   `json:"enabled"`
}

func (a *App) handleUpdateMCPToken(w http.ResponseWriter, r *http.Request) error {
	id, err := apiRouteID(r)
	if err != nil {
		return err
	}
	var patch mcpTokenPatch
	if err := decodeJSON(r, &patch); err != nil {
		return err
	}
	updates := make([]string, 0, 2)
	args := make([]any, 0, 3)
	if patch.Name != nil {
		name := strings.TrimSpace(*patch.Name)
		if name == "" || len([]rune(name)) > 100 {
			return badRequest("invalid_mcp_token", "令牌名称不能为空且不能超过 100 字")
		}
		updates = append(updates, "name = ?")
		args = append(args, name)
	}
	if patch.Enabled != nil {
		updates = append(updates, "enabled = ?")
		args = append(args, boolInt(*patch.Enabled))
	}
	if len(updates) == 0 {
		return badRequest("invalid_mcp_token", "至少提供 name 或 enabled")
	}
	args = append(args, id)
	result, err := a.db.ExecContext(r.Context(), "UPDATE mcp_tokens SET "+strings.Join(updates, ", ")+" WHERE id = ?", args...)
	if err != nil {
		return internalError(err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return notFound("mcp_token_not_found", "MCP 令牌不存在")
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	return nil
}

func (a *App) handleDeleteMCPToken(w http.ResponseWriter, r *http.Request) error {
	id, err := apiRouteID(r)
	if err != nil {
		return err
	}
	result, err := a.db.ExecContext(r.Context(), `DELETE FROM mcp_tokens WHERE id = ?`, id)
	if err != nil {
		return internalError(err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return notFound("mcp_token_not_found", "MCP 令牌不存在")
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
