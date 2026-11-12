package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenMigrateAndBackup(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "rill.db")
	db, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var version int
	if err := db.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != 0 {
		t.Fatalf("fresh version=%d", version)
	}
	if err := Migrate(ctx, db, path, filepath.Join(dir, "backups")); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != SchemaVersion {
		t.Fatalf("version=%d want=%d", version, SchemaVersion)
	}
	var journal string
	if err := db.QueryRowContext(ctx, `PRAGMA journal_mode`).Scan(&journal); err != nil {
		t.Fatal(err)
	}
	if journal != "wal" {
		t.Fatalf("journal_mode=%s", journal)
	}
	// foreign_keys is enforced by the single configured connection.
	_, err = db.ExecContext(ctx, `INSERT INTO sources (kind, name, feed_url, folder_id, next_fetch_at, feed_token, created_at, updated_at)
		VALUES ('rss', 'bad', 'https://example.com', 999, '2026-01-01T00:00:00Z', 'token', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
	if err == nil {
		t.Fatal("foreign key violation was accepted")
	}
	backupPath := filepath.Join(dir, "backups", "test.db")
	if err := os.MkdirAll(filepath.Dir(backupPath), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := VacuumInto(ctx, db, backupPath); err != nil {
		t.Fatal(err)
	}
	if err := ValidateFile(ctx, backupPath, SchemaVersion); err != nil {
		t.Fatal(err)
	}
}

// TestMigrateFromFirstV013RepairsSecretEnc 复现 2026-11-10 微服故障：
// 首个 v0.1.3 包把库升到 schema 7（push_channels 无 secret_enc），
// 修订迁移 7 不会重跑，必须由迁移 8 补列。
func TestMigrateFromFirstV013RepairsSecretEnc(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "rill.db")
	db, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// 应用迁移 1–7（即首个 v0.1.3 的 schema），停在第 7 版。
	for v := 1; v <= 7; v++ {
		if _, err := db.ExecContext(ctx, migrations[v]); err != nil {
			t.Fatalf("apply migration %d: %v", v, err)
		}
		if _, err := db.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", v)); err != nil {
			t.Fatal(err)
		}
	}
	// 存量渠道数据（模拟已创建过的渠道）。
	if _, err := db.ExecContext(ctx, `
		INSERT INTO push_channels (id, name, kind, url_enc, enabled, created_at, updated_at)
		VALUES (1, '飞书', 'feishu', 'enc', 1, '2026-11-10T00:00:00Z', '2026-11-10T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, db, path, filepath.Join(dir, "backups")); err != nil {
		t.Fatal(err)
	}
	var version int
	if err := db.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != SchemaVersion {
		t.Fatalf("version=%d want=%d", version, SchemaVersion)
	}
	var name string
	var secretEnc sql.NullString
	if err := db.QueryRowContext(ctx, `SELECT name, secret_enc FROM push_channels WHERE id = 1`).Scan(&name, &secretEnc); err != nil {
		t.Fatalf("query after migration 8: %v", err)
	}
	if name != "飞书" || secretEnc.Valid {
		t.Fatalf("channel data not preserved: name=%q secret=%v", name, secretEnc)
	}
}

func TestMigrateFromV1BackfillsFTS(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "old.db")
	db, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.ExecContext(ctx, migrations[1]); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `PRAGMA user_version = 1`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO sources (kind, name, feed_url, next_fetch_at, feed_token, created_at, updated_at)
		VALUES ('rss', 'Old', 'https://example.com/feed', '2026-01-01T00:00:00Z', 'old-token', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO items (source_id, guid, title, content_text, fetched_at, updated_at)
		VALUES (1, 'old-item', '中文搜索迁移', '正文', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, db, path, filepath.Join(dir, "backups")); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM items_fts WHERE items_fts MATCH '"中文搜索"'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("fts backfill count=%d", count)
	}
	var retention int
	if err := db.QueryRowContext(ctx, `SELECT retention_days FROM sources WHERE id = 1`).Scan(&retention); err != nil {
		t.Fatal(err)
	}
}

// 迁移 6：微博渠道移除后，存量微博源自动停用并标注，文章数据保留。
func TestMigrateWeiboSourcesDisabled(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "old.db")
	db, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for version := 1; version <= 5; version++ {
		if _, err := db.ExecContext(ctx, migrations[version]); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.ExecContext(ctx, `PRAGMA user_version = 5`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO sources (kind, name, feed_url, next_fetch_at, feed_token, created_at, updated_at)
		VALUES ('rss', '微博源', 'weibo://user/2803301701', '2026-01-01T00:00:00Z', 'tok', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'),
		       ('rss', 'RSS源', 'https://example.com/feed', '2026-01-01T00:00:00Z', 'tok2', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, db, path, filepath.Join(dir, "backups")); err != nil {
		t.Fatal(err)
	}
	var enabled int
	var note string
	if err := db.QueryRowContext(ctx, `SELECT enabled, COALESCE(note, '') FROM sources WHERE feed_url = 'weibo://user/2803301701'`).Scan(&enabled, &note); err != nil {
		t.Fatal(err)
	}
	if enabled != 0 || !strings.Contains(note, "微博渠道已移除") {
		t.Fatalf("weibo source not disabled: enabled=%d note=%q", enabled, note)
	}
	var rssEnabled int
	if err := db.QueryRowContext(ctx, `SELECT enabled FROM sources WHERE feed_url = 'https://example.com/feed'`).Scan(&rssEnabled); err != nil {
		t.Fatal(err)
	}
	if rssEnabled != 1 {
		t.Fatalf("non-weibo source affected: enabled=%d", rssEnabled)
	}
}
