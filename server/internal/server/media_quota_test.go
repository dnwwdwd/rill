package server

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMediaQuotaEvictsOldest(t *testing.T) {
	app, _ := newTestApp(t, true)
	dir := filepath.Join(app.cfg.CacheDir, "media")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	oldPath := filepath.Join(dir, "old")
	newPath := filepath.Join(dir, "new")
	if err := os.WriteFile(oldPath, []byte("old-media"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newPath, []byte("new-media"), 0o640); err != nil {
		t.Fatal(err)
	}
	now := app.now()
	if _, err := app.db.ExecContext(context.Background(), `
		INSERT INTO media_cache (url_hash, source_url, local_path, content_type, size_bytes, status, last_access_at)
		VALUES (?, 'https://example.com/old.png', ?, 'image/png', 9, 'ok', ?)`,
		"oldhash", oldPath, now.Add(-time.Hour).Format(timeRFC3339)); err != nil {
		t.Fatal(err)
	}
	if _, err := app.db.ExecContext(context.Background(), `
		INSERT INTO media_cache (url_hash, source_url, local_path, content_type, size_bytes, status, last_access_at)
		VALUES (?, 'https://example.com/new.png', ?, 'image/png', 9, 'ok', ?)`,
		"newhash", newPath, now.Format(timeRFC3339)); err != nil {
		t.Fatal(err)
	}
	app.cfg.MediaMaxBytes = 10
	if err := app.enforceMediaQuota(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatalf("old media should be removed: %v", err)
	}
	if _, err := os.Stat(newPath); err != nil {
		t.Fatalf("new media should remain: %v", err)
	}
	var status string
	if err := app.db.QueryRowContext(context.Background(), `SELECT status FROM media_cache WHERE url_hash = 'oldhash'`).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "pending" {
		t.Fatalf("old status=%s", status)
	}
}
