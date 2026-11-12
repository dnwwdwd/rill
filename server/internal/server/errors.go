package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type apiError struct {
	Status  int
	Code    string
	Message string
	Details any
	Err     error
}

func (e *apiError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Code, e.Err)
	}
	return e.Code
}

func (e *apiError) Unwrap() error { return e.Err }

func badRequest(code, message string) *apiError {
	return &apiError{Status: http.StatusBadRequest, Code: code, Message: message}
}

func notFound(code, message string) *apiError {
	return &apiError{Status: http.StatusNotFound, Code: code, Message: message}
}

func conflict(code, message string) *apiError {
	return &apiError{Status: http.StatusConflict, Code: code, Message: message}
}

func forbidden(code, message string) *apiError {
	return &apiError{Status: http.StatusForbidden, Code: code, Message: message}
}

func unauthorized(code, message string) *apiError {
	return &apiError{Status: http.StatusUnauthorized, Code: code, Message: message}
}

func unavailable(code, message string) *apiError {
	return &apiError{Status: http.StatusServiceUnavailable, Code: code, Message: message}
}

func internalError(err error) *apiError {
	return &apiError{Status: http.StatusInternalServerError, Code: "internal_error", Message: "服务器内部错误", Err: err}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if value == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(value)
}

func writeNoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

func writeAPIError(w http.ResponseWriter, err error) {
	var apiErr *apiError
	if !errors.As(err, &apiErr) {
		apiErr = internalError(err)
	}
	status := apiErr.Status
	if status == 0 {
		status = http.StatusInternalServerError
	}
	code := apiErr.Code
	if code == "" {
		code = "internal_error"
	}
	message := apiErr.Message
	if message == "" {
		message = http.StatusText(status)
	}
	payload := map[string]any{
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
	}
	if apiErr.Details != nil {
		payload["error"].(map[string]any)["details"] = apiErr.Details
	}
	writeJSON(w, status, payload)
}

func decodeJSON(r *http.Request, destination any) error {
	limited := io.LimitReader(r.Body, (1<<20)+1)
	decoder := json.NewDecoder(limited)
	decoder.UseNumber()
	if err := decoder.Decode(destination); err != nil {
		return badRequest("invalid_json", "请求体不是合法 JSON")
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return badRequest("invalid_json", "请求体只能包含一个 JSON 对象")
	}
	return nil
}

func (a *App) handler(fn func(http.ResponseWriter, *http.Request) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				a.logger.Error("http handler panic", "method", r.Method, "path", r.URL.Path, "panic", recovered)
				writeAPIError(w, internalError(errors.New("handler panic")))
			}
		}()
		if err := fn(w, r); err != nil {
			var apiErr *apiError
			if errors.As(err, &apiErr) && apiErr.Status >= 500 {
				a.logger.Error("request failed", "method", r.Method, "path", r.URL.Path, "err", err)
			}
			writeAPIError(w, err)
		}
	}
}

func (a *App) logDebug(message string, args ...any) {
	if strings.EqualFold(a.cfg.LogLevel, "debug") {
		a.logger.Debug(message, args...)
	}
}
