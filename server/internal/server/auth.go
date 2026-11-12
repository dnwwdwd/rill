package server

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"errors"
	"net/http"
	"time"

	"rill/internal/secure"
)

const sessionCookieName = "rill_session"

type sessionInfo struct {
	TokenHash string
	CSRF      string
	ExpiresAt time.Time
}

type sessionContextKey struct{}

func sessionFromContext(ctx context.Context) (*sessionInfo, bool) {
	value, ok := ctx.Value(sessionContextKey{}).(*sessionInfo)
	return value, ok
}

func (a *App) newSession(ctx context.Context) (token string, info *sessionInfo, err error) {
	token, err = secure.RandomSecret()
	if err != nil {
		return "", nil, err
	}
	csrf, err := secure.RandomSecret()
	if err != nil {
		return "", nil, err
	}
	expires := a.now().UTC().Add(a.cfg.SessionTTL)
	info = &sessionInfo{TokenHash: secure.HashToken(token), CSRF: csrf, ExpiresAt: expires}
	if _, err := a.db.ExecContext(ctx, `
		INSERT INTO sessions (token_hash, csrf_token, created_at, expires_at)
		VALUES (?, ?, ?, ?)`,
		info.TokenHash, csrf, a.now().UTC().Format(time.RFC3339), expires.Format(time.RFC3339)); err != nil {
		return "", nil, err
	}
	return token, info, nil
}

func (a *App) lookupSession(ctx context.Context, token string) (*sessionInfo, error) {
	if token == "" {
		return nil, sql.ErrNoRows
	}
	hash := secure.HashToken(token)
	var csrf, expiresRaw string
	err := a.db.QueryRowContext(ctx, `
		SELECT csrf_token, expires_at FROM sessions WHERE token_hash = ?`, hash).Scan(&csrf, &expiresRaw)
	if err != nil {
		return nil, err
	}
	expires, err := time.Parse(time.RFC3339, expiresRaw)
	if err != nil || !expires.After(a.now().UTC()) {
		_, _ = a.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, hash)
		return nil, sql.ErrNoRows
	}
	return &sessionInfo{TokenHash: hash, CSRF: csrf, ExpiresAt: expires}, nil
}

func (a *App) setSessionCookie(w http.ResponseWriter, token string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   a.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		Expires:  expires.UTC(),
		MaxAge:   int(a.cfg.SessionTTL.Seconds()),
	})
}

func (a *App) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   a.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
		Expires:  time.Unix(1, 0).UTC(),
	})
}

func (a *App) cleanupSessions(ctx context.Context) {
	_, _ = a.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= ?`, a.now().UTC().Format(time.RFC3339))
}

// apiMiddleware protects every /api/v1 route except the public auth config.
// Login happens through the Lazycat OIDC redirect flow (oidc.go); the only
// session-bearing requests are the browser's own cookies. State-changing
// methods additionally require X-CSRF-Token.
func (a *App) apiMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/v1/auth/config" {
			next.ServeHTTP(w, r)
			return
		}
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil || cookie.Value == "" {
			writeAPIError(w, &apiError{Status: http.StatusUnauthorized, Code: "unauthorized", Message: "请先登录"})
			return
		}
		session, err := a.lookupSession(r.Context(), cookie.Value)
		if errors.Is(err, sql.ErrNoRows) {
			a.clearSessionCookie(w)
			writeAPIError(w, &apiError{Status: http.StatusUnauthorized, Code: "unauthorized", Message: "会话已过期，请重新登录"})
			return
		}
		if err != nil {
			writeAPIError(w, internalError(err))
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions {
			provided := r.Header.Get("X-CSRF-Token")
			if provided == "" || subtle.ConstantTimeCompare([]byte(provided), []byte(session.CSRF)) != 1 {
				writeAPIError(w, forbidden("csrf_invalid", "CSRF token 缺失或无效"))
				return
			}
		}
		ctx := context.WithValue(r.Context(), sessionContextKey{}, session)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type csrfResponse struct {
	CSRFToken string `json:"csrf_token"`
}

func (a *App) handleLogout(w http.ResponseWriter, r *http.Request) error {
	session, ok := sessionFromContext(r.Context())
	if !ok {
		return &apiError{Status: http.StatusUnauthorized, Code: "unauthorized", Message: "请先登录"}
	}
	if _, err := a.db.ExecContext(r.Context(), `DELETE FROM sessions WHERE token_hash = ?`, session.TokenHash); err != nil {
		return internalError(err)
	}
	a.clearSessionCookie(w)
	writeNoContent(w)
	return nil
}

func (a *App) handleCSRF(w http.ResponseWriter, r *http.Request) error {
	session, ok := sessionFromContext(r.Context())
	if !ok {
		return &apiError{Status: http.StatusUnauthorized, Code: "unauthorized", Message: "请先登录"}
	}
	writeJSON(w, http.StatusOK, csrfResponse{CSRFToken: session.CSRF})
	return nil
}
