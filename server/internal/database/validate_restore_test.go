package database

import (
	"context"
	"path/filepath"
	"testing"
)

func TestPrepareForRestoreUpgradesV1(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "old-backup.db")
	db, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, migrations[1]); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `PRAGMA user_version = 1`); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := PrepareForRestore(ctx, path, filepath.Join(dir, "backups")); err != nil {
		t.Fatal(err)
	}
	check, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer check.Close()
	var version int
	if err := check.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != SchemaVersion {
		t.Fatalf("version=%d want=%d", version, SchemaVersion)
	}
	var retention int
	if err := check.QueryRowContext(ctx, `SELECT retention_days FROM sources LIMIT 1`).Scan(&retention); err != nil {
		// No rows is fine; the column must still exist.
		if err.Error() != "sql: no rows in result set" {
			t.Fatal(err)
		}
	}
	var ftsTable string
	if err := check.QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'items_fts'`).Scan(&ftsTable); err != nil {
		t.Fatal(err)
	}
}
