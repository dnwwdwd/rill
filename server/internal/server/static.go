package server

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func (a *App) handlePublicHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *App) handleStatic(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	clean := path.Clean("/" + r.URL.Path)
	if clean == "/api" || strings.HasPrefix(clean, "/api/") ||
		clean == "/feed" || strings.HasPrefix(clean, "/feed/") ||
		clean == "/opml" || strings.HasPrefix(clean, "/opml/") ||
		clean == "/media" || strings.HasPrefix(clean, "/media/") ||
		clean == "/healthz" || strings.HasPrefix(clean, "/healthz/") {
		http.NotFound(w, r)
		return
	}
	if a.static == "" {
		http.Error(w, "Rill API is running; frontend build was not found", http.StatusNotFound)
		return
	}
	relative := strings.TrimPrefix(clean, "/")
	if relative == "" || relative == "." {
		relative = "index.html"
	}
	full := filepath.Join(a.static, filepath.FromSlash(relative))
	if !pathWithin(a.static, full) {
		http.NotFound(w, r)
		return
	}
	info, err := os.Stat(full)
	if err != nil || info.IsDir() {
		full = filepath.Join(a.static, "index.html")
		relative = "index.html"
		if _, err := os.Stat(full); err != nil {
			http.NotFound(w, r)
			return
		}
	}
	if relative == "index.html" {
		w.Header().Set("Cache-Control", "no-cache")
	} else if strings.HasPrefix(relative, "assets/") || hashedAssetName(relative) {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else if strings.HasPrefix(relative, "icon") || strings.HasPrefix(relative, "favicon") {
		w.Header().Set("Cache-Control", "no-cache")
	} else {
		w.Header().Set("Cache-Control", "public, max-age=3600")
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeFile(w, r, full)
}

func pathWithin(root, candidate string) bool {
	root = filepath.Clean(root)
	candidate = filepath.Clean(candidate)
	if candidate == root {
		return true
	}
	return strings.HasPrefix(candidate, root+string(os.PathSeparator))
}

func hashedAssetName(relative string) bool {
	base := filepath.Base(relative)
	if len(base) < 20 {
		return false
	}
	// Vite commonly emits name-<hash>.ext. A long dash-delimited suffix is
	// enough to distinguish build output from public files like favicon.ico.
	dot := strings.LastIndexByte(base, '.')
	if dot <= 0 {
		return false
	}
	name := base[:dot]
	dash := strings.LastIndexByte(name, '-')
	return dash > 0 && len(name)-dash-1 >= 8
}
