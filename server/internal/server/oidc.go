package server

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"rill/internal/secure"
)

const (
	oidcStateCookieName = "rill_oidc_state"
	oidcStateTTL        = 10 * time.Minute
	oidcMaxResponseSize = 1 << 20
)

var oidcHTTPClient = &http.Client{Timeout: 15 * time.Second}

type authConfigResponse struct {
	Mode     string `json:"mode"`
	LoginURL string `json:"login_url,omitempty"`
}

type oidcState struct {
	State     string `json:"state"`
	Verifier  string `json:"verifier"`
	Nonce     string `json:"nonce"`
	ReturnTo  string `json:"return_to"`
	ExpiresAt int64  `json:"expires_at"`
}

type oidcTokenResponse struct {
	AccessToken      string `json:"access_token"`
	TokenType        string `json:"token_type"`
	IDToken          string `json:"id_token"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

type oidcUserInfo struct {
	Sub               string `json:"sub"`
	PreferredUsername string `json:"preferred_username"`
	Name              string `json:"name"`
	Email             string `json:"email"`
}

func (a *App) handleAuthConfig(w http.ResponseWriter, _ *http.Request) error {
	writeJSON(w, http.StatusOK, authConfigResponse{Mode: "oidc", LoginURL: "/auth/oidc/login"})
	return nil
}

// handleOIDCLogin starts the authorization-code flow. It is intentionally not
// wrapped by the API error handler: a browser should always leave this route
// with a redirect, never a JSON error page.
func (a *App) handleOIDCLogin(w http.ResponseWriter, r *http.Request) {
	redirectURL := a.oidcRedirectURL(r)
	state, err := secure.RandomSecret()
	if err != nil {
		a.oidcFail(w, r, "oidc_state_failed", err)
		return
	}
	verifier, err := secure.RandomSecret()
	if err != nil {
		a.oidcFail(w, r, "oidc_state_failed", err)
		return
	}
	nonce, err := secure.RandomSecret()
	if err != nil {
		a.oidcFail(w, r, "oidc_state_failed", err)
		return
	}
	transaction := oidcState{
		State:     state,
		Verifier:  verifier,
		Nonce:     nonce,
		ReturnTo:  safeReturnTo(r.URL.Query().Get("return_to")),
		ExpiresAt: a.now().Add(oidcStateTTL).Unix(),
	}
	encoded, err := json.Marshal(transaction)
	if err != nil {
		a.oidcFail(w, r, "oidc_state_failed", err)
		return
	}
	sealed, err := a.secret.Encrypt(string(encoded))
	if err != nil {
		a.oidcFail(w, r, "oidc_state_failed", err)
		return
	}
	a.setOIDCStateCookie(w, sealed)

	authURL, err := url.Parse(a.cfg.OIDC.AuthURI)
	if err != nil {
		a.oidcFail(w, r, "oidc_config_invalid", err)
		return
	}
	challenge := sha256.Sum256([]byte(verifier))
	query := authURL.Query()
	query.Set("response_type", "code")
	query.Set("client_id", a.cfg.OIDC.ClientID)
	query.Set("redirect_uri", redirectURL)
	query.Set("scope", a.cfg.OIDC.Scopes)
	query.Set("state", state)
	query.Set("nonce", nonce)
	query.Set("code_challenge", base64.RawURLEncoding.EncodeToString(challenge[:]))
	query.Set("code_challenge_method", "S256")
	authURL.RawQuery = query.Encode()
	http.Redirect(w, r, authURL.String(), http.StatusFound)
}

func (a *App) handleOIDCCallback(w http.ResponseWriter, r *http.Request) {
	transaction, err := a.readOIDCState(r)
	a.clearOIDCStateCookie(w)
	if err != nil {
		a.oidcFail(w, r, "oidc_state_invalid", err)
		return
	}
	if subtle.ConstantTimeCompare([]byte(transaction.State), []byte(r.URL.Query().Get("state"))) != 1 {
		a.oidcFail(w, r, "oidc_state_invalid", fmt.Errorf("state mismatch"))
		return
	}
	if providerError := strings.TrimSpace(r.URL.Query().Get("error")); providerError != "" {
		a.oidcFail(w, r, "oidc_"+sanitizeErrorCode(providerError), fmt.Errorf("provider returned %s: %s", providerError, r.URL.Query().Get("error_description")))
		return
	}
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	if code == "" {
		a.oidcFail(w, r, "oidc_code_missing", fmt.Errorf("authorization code is missing"))
		return
	}
	token, err := a.exchangeOIDCCode(r, code, a.oidcRedirectURL(r), transaction.Verifier)
	if err != nil {
		a.oidcFail(w, r, "oidc_exchange_failed", err)
		return
	}
	user, err := a.fetchOIDCUserInfo(r, token.AccessToken)
	if err != nil {
		a.oidcFail(w, r, "oidc_userinfo_failed", err)
		return
	}
	if strings.TrimSpace(user.Sub) == "" {
		a.oidcFail(w, r, "oidc_userinfo_failed", fmt.Errorf("userinfo is missing sub"))
		return
	}
	if previous, err := r.Cookie(sessionCookieName); err == nil && previous.Value != "" {
		_, _ = a.db.ExecContext(r.Context(), `DELETE FROM sessions WHERE token_hash = ?`, secure.HashToken(previous.Value))
	}
	tokenValue, session, err := a.newSession(r.Context())
	if err != nil {
		a.oidcFail(w, r, "oidc_session_failed", err)
		return
	}
	if err := a.writeSetting(r.Context(), settingLastLoginAt, a.now().UTC().Format(time.RFC3339)); err != nil {
		a.oidcFail(w, r, "oidc_session_failed", err)
		return
	}
	// 记录懒猫账号展示名（userinfo 优先 preferred_username，其次 name、sub）。
	account := strings.TrimSpace(user.PreferredUsername)
	if account == "" {
		account = strings.TrimSpace(user.Name)
	}
	if account == "" {
		account = strings.TrimSpace(user.Sub)
	}
	if account != "" {
		if err := a.writeSetting(r.Context(), settingAccount, account); err != nil {
			a.oidcFail(w, r, "oidc_session_failed", err)
			return
		}
	}
	a.setSessionCookie(w, tokenValue, session.ExpiresAt)
	http.Redirect(w, r, transaction.ReturnTo, http.StatusSeeOther)
}

func (a *App) oidcRedirectURL(r *http.Request) string {
	if strings.TrimSpace(a.cfg.OIDC.RedirectURL) != "" {
		return strings.TrimSpace(a.cfg.OIDC.RedirectURL)
	}
	scheme := "https"
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); forwarded != "" {
		scheme = strings.ToLower(forwarded)
	} else if r.TLS == nil {
		scheme = "http"
	}
	host := r.Host
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-Host")); forwarded != "" {
		host = forwarded
	}
	return scheme + "://" + host + "/auth/oidc/callback"
}

func (a *App) exchangeOIDCCode(r *http.Request, code, redirectURL, verifier string) (oidcTokenResponse, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", redirectURL)
	form.Set("client_id", a.cfg.OIDC.ClientID)
	form.Set("client_secret", a.cfg.OIDC.ClientSecret)
	form.Set("code_verifier", verifier)
	request, err := http.NewRequestWithContext(r.Context(), http.MethodPost, a.cfg.OIDC.TokenURI, strings.NewReader(form.Encode()))
	if err != nil {
		return oidcTokenResponse{}, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")
	response, err := oidcHTTPClient.Do(request)
	if err != nil {
		return oidcTokenResponse{}, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, oidcMaxResponseSize))
	if err != nil {
		return oidcTokenResponse{}, err
	}
	var token oidcTokenResponse
	if err := json.Unmarshal(body, &token); err != nil {
		return oidcTokenResponse{}, fmt.Errorf("decode token response: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return oidcTokenResponse{}, fmt.Errorf("token endpoint returned %d: %s", response.StatusCode, token.Error)
	}
	if strings.TrimSpace(token.AccessToken) == "" {
		return oidcTokenResponse{}, fmt.Errorf("token response is missing access_token")
	}
	return token, nil
}

func (a *App) fetchOIDCUserInfo(r *http.Request, accessToken string) (oidcUserInfo, error) {
	request, err := http.NewRequestWithContext(r.Context(), http.MethodGet, a.cfg.OIDC.UserinfoURI, nil)
	if err != nil {
		return oidcUserInfo{}, err
	}
	request.Header.Set("Authorization", "Bearer "+accessToken)
	request.Header.Set("Accept", "application/json")
	response, err := oidcHTTPClient.Do(request)
	if err != nil {
		return oidcUserInfo{}, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, oidcMaxResponseSize))
	if err != nil {
		return oidcUserInfo{}, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return oidcUserInfo{}, fmt.Errorf("userinfo endpoint returned %d", response.StatusCode)
	}
	var user oidcUserInfo
	if err := json.Unmarshal(body, &user); err != nil {
		return oidcUserInfo{}, fmt.Errorf("decode userinfo: %w", err)
	}
	return user, nil
}

func (a *App) setOIDCStateCookie(w http.ResponseWriter, sealed string) {
	http.SetCookie(w, &http.Cookie{
		Name:     oidcStateCookieName,
		Value:    sealed,
		Path:     "/auth/oidc",
		HttpOnly: true,
		Secure:   a.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(oidcStateTTL.Seconds()),
		Expires:  a.now().Add(oidcStateTTL).UTC(),
	})
}

func (a *App) clearOIDCStateCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     oidcStateCookieName,
		Value:    "",
		Path:     "/auth/oidc",
		HttpOnly: true,
		Secure:   a.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
		Expires:  time.Unix(1, 0).UTC(),
	})
}

func (a *App) readOIDCState(r *http.Request) (oidcState, error) {
	cookie, err := r.Cookie(oidcStateCookieName)
	if err != nil || cookie.Value == "" {
		return oidcState{}, fmt.Errorf("state cookie is missing")
	}
	plaintext, err := a.secret.Decrypt(cookie.Value)
	if err != nil {
		return oidcState{}, fmt.Errorf("decrypt state cookie: %w", err)
	}
	var transaction oidcState
	if err := json.Unmarshal([]byte(plaintext), &transaction); err != nil {
		return oidcState{}, fmt.Errorf("decode state cookie: %w", err)
	}
	if transaction.ExpiresAt == 0 || a.now().Unix() > transaction.ExpiresAt {
		return oidcState{}, fmt.Errorf("state cookie expired")
	}
	if transaction.State == "" || transaction.Verifier == "" || transaction.ReturnTo == "" {
		return oidcState{}, fmt.Errorf("state cookie is incomplete")
	}
	return transaction, nil
}

func (a *App) oidcFail(w http.ResponseWriter, r *http.Request, code string, err error) {
	if err != nil {
		a.logger.Warn("oidc login failed", "code", code, "err", err)
	}
	target := "/login?oidc_error=" + url.QueryEscape(sanitizeErrorCode(code))
	http.Redirect(w, r, target, http.StatusSeeOther)
}

func sanitizeErrorCode(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_', r == '-':
			builder.WriteRune(r)
		}
		if builder.Len() >= 64 {
			break
		}
	}
	if builder.Len() == 0 {
		return "unknown"
	}
	return builder.String()
}

func safeReturnTo(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || !strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "//") || strings.Contains(raw, "\\") {
		return "/"
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.IsAbs() || parsed.Host != "" {
		return "/"
	}
	return raw
}
