package database

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"
)

// ValidateFile opens a candidate backup read-only, runs integrity_check and
// verifies that it uses the schema version this binary understands.
func ValidateFile(ctx context.Context, path string, expectedVersion int) error {
	version, err := fileVersion(ctx, path)
	if err != nil {
		return err
	}
	if version != expectedVersion {
		return fmt.Errorf("备份 schema 版本为 %d，当前仅支持恢复版本 %d", version, expectedVersion)
	}
	return validateCoreTables(ctx, path)
}

func fileVersion(ctx context.Context, path string) (int, error) {
	u := url.URL{Scheme: "file", Path: path}
	q := u.Query()
	q.Set("mode", "ro")
	u.RawQuery = q.Encode()
	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		return 0, fmt.Errorf("打开备份失败：%w", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err := db.PingContext(ctx); err != nil {
		return 0, fmt.Errorf("备份不是可读取的 SQLite 文件")
	}
	var check string
	if err := db.QueryRowContext(ctx, `PRAGMA integrity_check`).Scan(&check); err != nil {
		return 0, fmt.Errorf("备份完整性检查失败：%w", err)
	}
	if !strings.EqualFold(strings.TrimSpace(check), "ok") {
		return 0, fmt.Errorf("备份完整性检查未通过：%s", check)
	}
	var version int
	if err := db.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version); err != nil {
		return 0, fmt.Errorf("读取备份 schema 版本失败：%w", err)
	}
	return version, nil
}

func validateCoreTables(ctx context.Context, path string) error {
	u := url.URL{Scheme: "file", Path: path}
	q := u.Query()
	q.Set("mode", "ro")
	u.RawQuery = q.Encode()
	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		return fmt.Errorf("打开备份失败：%w", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	required := []string{"folders", "sources", "items", "item_notes", "fetch_logs", "media_cache", "ai_results", "settings", "podcast_progress"}
	for _, table := range required {
		var name string
		err := db.QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&name)
		if err != nil {
			return fmt.Errorf("备份缺少表 %s", table)
		}
	}
	return nil
}

// PrepareForRestore validates an uploaded database and upgrades older schema
// versions in place before it is attached to the live connection. The caller
// remains responsible for taking a safety backup of the live database first.
func PrepareForRestore(ctx context.Context, path, backupDir string) error {
	version, err := fileVersion(ctx, path)
	if err != nil {
		return err
	}
	if version > SchemaVersion {
		return fmt.Errorf("备份 schema 版本为 %d，当前仅支持版本 %d 或更旧版本", version, SchemaVersion)
	}
	if version == SchemaVersion {
		return validateCoreTables(ctx, path)
	}
	db, err := Open(ctx, path)
	if err != nil {
		return fmt.Errorf("打开待升级备份失败：%w", err)
	}
	migrateErr := Migrate(ctx, db, path, backupDir)
	closeErr := db.Close()
	if migrateErr != nil {
		return fmt.Errorf("升级备份 schema 失败：%w", migrateErr)
	}
	if closeErr != nil {
		return fmt.Errorf("关闭升级后的备份失败：%w", closeErr)
	}
	return ValidateFile(ctx, path, SchemaVersion)
}
