package server

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const (
	iconCacheTTL      = 7 * 24 * time.Hour
	iconFailureTTL    = time.Hour
	iconMaxBytes      = 512 << 10
	iconCacheMaxBytes = 8 << 20
	iconCacheMaxFiles = 500
)

type iconSourceRow struct {
	IconSourceURL string
	FeedURL       string
	SiteURL       string
}

func sourceIconPath(id int64) string {
	return fmt.Sprintf("/api/v1/sources/%d/icon", id)
}

func (a *App) handleSourceIcon(w http.ResponseWriter, r *http.Request) error {
	id, err := apiRouteID(r)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}
	var row iconSourceRow
	var iconSource, siteURL sql.NullString
	err = a.db.QueryRowContext(r.Context(), `
		SELECT COALESCE(icon_url, ''), feed_url, COALESCE(site_url, '')
		FROM sources WHERE id = ?`, id).Scan(&iconSource, &row.FeedURL, &siteURL)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return nil
	}
	if err != nil {
		return internalError(err)
	}
	row.IconSourceURL = iconSource.String
	row.SiteURL = siteURL.String

	if a.serveIcon(w, r, id, true) {
		return nil
	}
	_, refreshErr, _ := a.iconGroup.Do(strconv.FormatInt(id, 10), func() (any, error) {
		return nil, a.refreshSourceIcon(r.Context(), id, row)
	})
	if refreshErr != nil {
		if a.serveIcon(w, r, id, false) {
			return nil
		}
		http.NotFound(w, r)
		return nil
	}
	if !a.serveIcon(w, r, id, true) {
		http.NotFound(w, r)
	}
	return nil
}

func (a *App) iconDir() string {
	return filepath.Join(a.cfg.CacheDir, "icons")
}

func (a *App) iconPath(id int64) string {
	return filepath.Join(a.iconDir(), strconv.FormatInt(id, 10))
}

func (a *App) iconFailurePath(id int64) string {
	return filepath.Join(a.iconDir(), strconv.FormatInt(id, 10)+".failed")
}

func (a *App) iconFailureRecent(id int64) bool {
	info, err := os.Stat(a.iconFailurePath(id))
	return err == nil && time.Since(info.ModTime()) < iconFailureTTL
}

func (a *App) serveIcon(w http.ResponseWriter, r *http.Request, id int64, requireFresh bool) bool {
	path := a.iconPath(id)
	if !pathWithin(a.cfg.CacheDir, path) {
		return false
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	if requireFresh && time.Since(info.ModTime()) > iconCacheTTL {
		return false
	}
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()
	contentType := detectIconContentType(file)
	if contentType == "" {
		return false
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return false
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=604800")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, "", info.ModTime(), file)
	return true
}

func detectIconContentType(file *os.File) string {
	head := make([]byte, 512)
	n, err := file.Read(head)
	if err != nil && !errors.Is(err, io.EOF) {
		return ""
	}
	contentType := strings.ToLower(strings.TrimSpace(http.DetectContentType(head[:n])))
	if contentType == "application/octet-stream" && n >= 4 &&
		head[0] == 0 && head[1] == 0 && head[2] == 1 && head[3] == 0 {
		contentType = "image/x-icon"
	}
	if !supportedIconContentType(contentType) {
		return ""
	}
	return contentType
}

func supportedIconContentType(contentType string) bool {
	contentType = strings.ToLower(strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0]))
	if contentType == "" || strings.Contains(contentType, "svg") || strings.HasPrefix(contentType, "text/") {
		return false
	}
	return strings.HasPrefix(contentType, "image/")
}

func (a *App) refreshSourceIcon(ctx context.Context, id int64, row iconSourceRow) error {
	if a.iconFailureRecent(id) {
		return errors.New("icon recently failed")
	}
	candidates := make([]string, 0, 4)
	if row.IconSourceURL != "" {
		candidates = append(candidates, row.IconSourceURL)
	}
	if row.SiteURL != "" {
		if discovered := a.discoverSiteIcon(ctx, row.SiteURL); discovered != "" {
			candidates = append(candidates, discovered)
		}
	}
	if origin := faviconOrigin(row.SiteURL); origin != "" {
		candidates = append(candidates, origin)
	}
	if origin := faviconOrigin(row.FeedURL); origin != "" {
		candidates = append(candidates, origin)
	}
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if _, exists := seen[candidate]; exists {
			continue
		}
		seen[candidate] = struct{}{}
		data, contentType, err := a.fetchIcon(ctx, candidate)
		if err != nil {
			continue
		}
		if err := a.writeIconCache(id, data); err != nil {
			return err
		}
		a.clearIconFailure(id)
		a.logger.Info("source icon cached", "source_id", id, "content_type", contentType, "bytes", len(data))
		return nil
	}
	_ = os.MkdirAll(a.iconDir(), 0o750)
	_ = os.WriteFile(a.iconFailurePath(id), []byte(a.now().Format(timeRFC3339)), 0o640)
	return errors.New("no icon found")
}

func (a *App) discoverSiteIcon(ctx context.Context, siteURL string) string {
	response, err := a.fetcher.GetBytes(ctx, siteURL, 512<<10, http.Header{
		"Accept": []string{"text/html,application/xhtml+xml;q=0.9,*/*;q=0.1"},
	})
	if err != nil {
		return ""
	}
	contentType := strings.ToLower(strings.TrimSpace(strings.SplitN(response.ContentType, ";", 2)[0]))
	if contentType != "" && contentType != "text/html" && contentType != "application/xhtml+xml" {
		return ""
	}
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(response.Body))
	if err != nil {
		return ""
	}
	base, err := url.Parse(siteURL)
	if err != nil {
		return ""
	}
	const rels = "icon,shortcut icon,apple-touch-icon,apple-touch-icon-precomposed"
	href := ""
	doc.Find("link").EachWithBreak(func(_ int, selection *goquery.Selection) bool {
		rel, _ := selection.Attr("rel")
		if !iconRelMatches(rel, rels) {
			return true
		}
		value, ok := selection.Attr("href")
		if !ok {
			return true
		}
		value = strings.TrimSpace(value)
		if value == "" {
			return true
		}
		parsed, err := url.Parse(value)
		if err != nil {
			return true
		}
		resolved := base.ResolveReference(parsed)
		if resolved.Scheme != "http" && resolved.Scheme != "https" {
			return true
		}
		href = resolved.String()
		return false
	})
	return href
}

func iconRelMatches(rel, wanted string) bool {
	rel = strings.ToLower(strings.TrimSpace(rel))
	for _, part := range strings.Fields(rel) {
		for _, candidate := range strings.Split(wanted, ",") {
			if part == candidate {
				return true
			}
		}
	}
	return false
}

func faviconOrigin(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host + "/favicon.ico"
}

func (a *App) fetchIcon(ctx context.Context, rawURL string) ([]byte, string, error) {
	response, err := a.fetcher.GetBytes(ctx, rawURL, iconMaxBytes, http.Header{
		"Accept": []string{"image/png,image/jpeg,image/gif,image/webp,image/avif,image/x-icon;q=0.9,*/*;q=0.1"},
	})
	if err != nil {
		return nil, "", err
	}
	if len(response.Body) == 0 || len(response.Body) > iconMaxBytes {
		return nil, "", errors.New("invalid icon size")
	}
	contentType := strings.ToLower(strings.TrimSpace(strings.SplitN(response.ContentType, ";", 2)[0]))
	if contentType == "" || contentType == "application/octet-stream" {
		contentType = strings.ToLower(strings.TrimSpace(http.DetectContentType(response.Body)))
	}
	if !supportedIconContentType(contentType) {
		return nil, "", errors.New("unsupported icon content type")
	}
	return response.Body, contentType, nil
}

func (a *App) writeIconCache(id int64, data []byte) error {
	dir := a.iconDir()
	if !pathWithin(a.cfg.CacheDir, dir) {
		return errors.New("icon path escapes cache directory")
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	temp, err := os.CreateTemp(dir, strconv.FormatInt(id, 10)+".tmp-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		_ = os.Remove(tempPath)
		return err
	}
	if err := temp.Close(); err != nil {
		_ = os.Remove(tempPath)
		return err
	}
	if err := os.Chmod(tempPath, 0o640); err != nil {
		_ = os.Remove(tempPath)
		return err
	}
	if err := os.Rename(tempPath, a.iconPath(id)); err != nil {
		_ = os.Remove(tempPath)
		return err
	}
	if err := a.enforceIconQuota(); err != nil {
		a.logger.Warn("icon quota cleanup failed", "err", err)
	}
	return nil
}

func (a *App) enforceIconQuota() error {
	entries, err := os.ReadDir(a.iconDir())
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	type candidate struct {
		path    string
		size    int64
		modTime time.Time
	}
	files := make([]candidate, 0, len(entries))
	var total int64
	for _, entry := range entries {
		if entry.IsDir() || strings.HasSuffix(entry.Name(), ".failed") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, candidate{
			path:    filepath.Join(a.iconDir(), entry.Name()),
			size:    info.Size(),
			modTime: info.ModTime(),
		})
		total += info.Size()
	}
	sort.Slice(files, func(i, j int) bool { return files[i].modTime.Before(files[j].modTime) })
	for len(files) > 0 && (len(files) > iconCacheMaxFiles || total > iconCacheMaxBytes) {
		oldest := files[0]
		files = files[1:]
		if pathWithin(a.cfg.CacheDir, oldest.path) {
			_ = os.Remove(oldest.path)
			total -= oldest.size
		}
	}
	return nil
}

func (a *App) clearIconFailure(id int64) {
	_ = os.Remove(a.iconFailurePath(id))
}
