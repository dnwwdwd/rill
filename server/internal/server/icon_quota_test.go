package server

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestIconQuotaEvictsOldest(t *testing.T) {
	app, _ := newTestApp(t, true)
	dir := app.iconDir()
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	oldPath := filepath.Join(dir, "1")
	newPath := filepath.Join(dir, "2")
	old, err := os.Create(oldPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := old.Truncate(iconCacheMaxBytes + 1); err != nil {
		_ = old.Close()
		t.Fatal(err)
	}
	if err := old.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newPath, []byte("new-icon"), 0o640); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().Add(-time.Hour)
	if err := os.Chtimes(oldPath, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	if err := app.enforceIconQuota(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatalf("old icon should be removed: %v", err)
	}
	if _, err := os.Stat(newPath); err != nil {
		t.Fatalf("new icon should remain: %v", err)
	}
}
