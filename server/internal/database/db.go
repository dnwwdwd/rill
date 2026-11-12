package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const SchemaVersion = 11

// Open opens the single SQLite file used by Rill. A single connection is
// deliberate: it keeps writes serialized and removes SQLITE_BUSY surprises.
func Open(ctx context.Context, path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}
	dsn := sqliteDSN(path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	if err := verifyPragmas(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("secure database permissions: %w", err)
	}
	return db, nil
}

func sqliteDSN(path string) string {
	u := url.URL{Scheme: "file", Path: path}
	q := u.Query()
	q.Add("_pragma", "journal_mode(WAL)")
	q.Add("_pragma", "foreign_keys(1)")
	q.Add("_pragma", "busy_timeout(5000)")
	q.Add("_pragma", "synchronous(NORMAL)")
	u.RawQuery = q.Encode()
	return u.String()
}

func verifyPragmas(ctx context.Context, db *sql.DB) error {
	var journal string
	if err := db.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&journal); err != nil {
		return fmt.Errorf("read journal_mode: %w", err)
	}
	if !strings.EqualFold(journal, "wal") {
		return fmt.Errorf("sqlite journal_mode is %q, want wal", journal)
	}
	var foreignKeys int
	if err := db.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
		return fmt.Errorf("read foreign_keys: %w", err)
	}
	if foreignKeys != 1 {
		return errors.New("sqlite foreign_keys is disabled")
	}
	var busyTimeout int
	if err := db.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&busyTimeout); err != nil {
		return fmt.Errorf("read busy_timeout: %w", err)
	}
	if busyTimeout < 5000 {
		return fmt.Errorf("sqlite busy_timeout is %d, want at least 5000", busyTimeout)
	}
	return nil
}

// Migrate applies schema migrations using PRAGMA user_version. It creates a
// safety copy with VACUUM INTO before upgrading an existing database.
func Migrate(ctx context.Context, db *sql.DB, dbPath, backupDir string) error {
	var version int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return fmt.Errorf("read user_version: %w", err)
	}
	if version > SchemaVersion {
		return fmt.Errorf("database schema %d is newer than this binary (%d)", version, SchemaVersion)
	}
	if version == SchemaVersion {
		return nil
	}
	if version > 0 {
		if err := os.MkdirAll(backupDir, 0o750); err != nil {
			return fmt.Errorf("create migration backup directory: %w", err)
		}
		name := fmt.Sprintf("pre-migrate-v%d-%s.db", version, time.Now().UTC().Format("20060102T150405Z"))
		if err := VacuumInto(ctx, db, filepath.Join(backupDir, name)); err != nil {
			return fmt.Errorf("backup before migration: %w", err)
		}
	}
	for next := version + 1; next <= SchemaVersion; next++ {
		sqlText, ok := migrations[next]
		if !ok {
			return fmt.Errorf("missing migration %d", next)
		}
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin migration %d: %w", next, err)
		}
		if _, err := tx.ExecContext(ctx, sqlText); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply migration %d: %w", next, err)
		}
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", next)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("set schema version %d: %w", next, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %d: %w", next, err)
		}
	}
	return nil
}

// VacuumInto writes a consistent backup without copying WAL files.
func VacuumInto(ctx context.Context, db *sql.DB, destination string) error {
	if _, err := db.ExecContext(ctx, "VACUUM INTO ?", destination); err != nil {
		return err
	}
	return nil
}

var migrations = map[int]string{
	1: `
CREATE TABLE folders (
    id          INTEGER PRIMARY KEY,
    name        TEXT NOT NULL,
    sort        INTEGER NOT NULL DEFAULT 0,
    parent_id   INTEGER REFERENCES folders(id) ON DELETE SET NULL,
    created_at  TEXT NOT NULL
);
CREATE INDEX idx_folders_parent ON folders(parent_id, sort);

CREATE TABLE sources (
    id               INTEGER PRIMARY KEY,
    kind             TEXT NOT NULL CHECK (kind IN ('rss', 'x', 'wechat')),
    name             TEXT NOT NULL,
    feed_url         TEXT NOT NULL,
    site_url         TEXT,
    folder_id        INTEGER REFERENCES folders(id) ON DELETE SET NULL,
    config_json      TEXT NOT NULL DEFAULT '{}',
    enabled          INTEGER NOT NULL DEFAULT 1,
    refresh_seconds  INTEGER NOT NULL DEFAULT 1800,
    initial_limit    INTEGER NOT NULL DEFAULT 20,
    initial_fetched  INTEGER NOT NULL DEFAULT 0,
    fallback_url     TEXT,
    etag             TEXT,
    last_modified    TEXT,
    next_fetch_at    TEXT NOT NULL,
    last_fetch_at    TEXT,
    last_new_item_at TEXT,
    last_status      TEXT,
    last_error       TEXT,
    fail_count       INTEGER NOT NULL DEFAULT 0,
    note             TEXT,
    icon_url         TEXT,
    feed_token       TEXT NOT NULL UNIQUE,
    created_at       TEXT NOT NULL,
    updated_at       TEXT NOT NULL
);
CREATE INDEX idx_sources_due ON sources(enabled, next_fetch_at);
CREATE INDEX idx_sources_folder ON sources(folder_id, name);
CREATE INDEX idx_sources_feed_token ON sources(feed_token);

CREATE TABLE items (
    id                      INTEGER PRIMARY KEY,
    source_id               INTEGER NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
    guid                    TEXT NOT NULL,
    url                     TEXT,
    title                   TEXT,
    original_title          TEXT,
    author                  TEXT,
    published_at            TEXT,
    published_raw           TEXT,
    published_estimated     INTEGER NOT NULL DEFAULT 0,
    content_html            TEXT,
    content_text            TEXT,
    media_json              TEXT,
    content_hash            TEXT,
    read_at                 TEXT,
    starred_at              TEXT,
    fetched_at              TEXT NOT NULL,
    updated_at              TEXT NOT NULL,
    UNIQUE(source_id, guid)
);
CREATE INDEX idx_items_source_published ON items(source_id, published_at DESC);
CREATE INDEX idx_items_published ON items(published_at DESC, id DESC);
CREATE INDEX idx_items_fetched ON items(fetched_at DESC, id DESC);
CREATE INDEX idx_items_unread ON items(source_id, published_at DESC) WHERE read_at IS NULL;
CREATE INDEX idx_items_starred ON items(starred_at) WHERE starred_at IS NOT NULL;
CREATE INDEX idx_items_content_hash ON items(content_hash);

CREATE TABLE fetch_logs (
    id           INTEGER PRIMARY KEY,
    source_id    INTEGER NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
    started_at   TEXT NOT NULL,
    duration_ms  INTEGER,
    http_status  INTEGER,
    result       TEXT NOT NULL CHECK (result IN ('ok', 'not_modified', 'error')),
    item_count   INTEGER NOT NULL DEFAULT 0,
    new_count    INTEGER NOT NULL DEFAULT 0,
    error        TEXT
);
CREATE INDEX idx_fetch_logs_source ON fetch_logs(source_id, started_at DESC);

CREATE TABLE media_cache (
    id              INTEGER PRIMARY KEY,
    url_hash        TEXT NOT NULL UNIQUE,
    source_url      TEXT NOT NULL,
    local_path      TEXT,
    content_type    TEXT,
    size_bytes      INTEGER,
    status          TEXT NOT NULL CHECK (status IN ('pending', 'ok', 'failed')),
    fail_count      INTEGER NOT NULL DEFAULT 0,
    fetched_at      TEXT,
    last_access_at  TEXT NOT NULL
);

CREATE TABLE ai_results (
    item_id     INTEGER NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    kind        TEXT NOT NULL,
    model       TEXT,
    content     TEXT,
    tokens      INTEGER,
    created_at  TEXT NOT NULL,
    PRIMARY KEY (item_id, kind)
);

CREATE TABLE settings (
    key         TEXT PRIMARY KEY,
    value       TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);

CREATE TABLE sessions (
    token_hash   TEXT PRIMARY KEY,
    csrf_token   TEXT NOT NULL,
    created_at   TEXT NOT NULL,
    expires_at   TEXT NOT NULL
);
CREATE INDEX idx_sessions_expires ON sessions(expires_at);
`,
	2: `
ALTER TABLE sources ADD COLUMN retention_days INTEGER NOT NULL DEFAULT 0;
ALTER TABLE sources ADD COLUMN retention_max_items INTEGER NOT NULL DEFAULT 0;
CREATE INDEX idx_items_retention ON items(source_id, COALESCE(published_at, fetched_at));
`,
	3: `
CREATE VIRTUAL TABLE items_fts USING fts5(title, content_text, author, tokenize='trigram');
INSERT INTO items_fts(rowid, title, content_text, author)
SELECT id, COALESCE(title, ''), COALESCE(content_text, ''), COALESCE(author, '') FROM items;
CREATE TRIGGER items_fts_ai AFTER INSERT ON items BEGIN
	INSERT INTO items_fts(rowid, title, content_text, author)
	VALUES (new.id, COALESCE(new.title, ''), COALESCE(new.content_text, ''), COALESCE(new.author, ''));
END;
CREATE TRIGGER items_fts_ad AFTER DELETE ON items BEGIN
	DELETE FROM items_fts WHERE rowid = old.id;
END;
CREATE TRIGGER items_fts_au AFTER UPDATE OF title, content_text, author ON items BEGIN
	DELETE FROM items_fts WHERE rowid = old.id;
	INSERT INTO items_fts(rowid, title, content_text, author)
	VALUES (new.id, COALESCE(new.title, ''), COALESCE(new.content_text, ''), COALESCE(new.author, ''));
END;
`,
	4: `
ALTER TABLE sources ADD COLUMN backfill_mode TEXT NOT NULL DEFAULT 'window' CHECK (backfill_mode IN ('window', 'full'));
`,
	5: `
CREATE TABLE podcast_progress (
    item_id           INTEGER PRIMARY KEY REFERENCES items(id) ON DELETE CASCADE,
    position_seconds  REAL NOT NULL DEFAULT 0,
    duration_seconds  REAL NOT NULL DEFAULT 0,
    updated_at        TEXT NOT NULL
);
`,
	6: `
-- 微博渠道移除（2026-11-08 用户决策）：停用存量微博源并保留文章数据，
-- 源与条目都不删除，用户可在界面手动删除。
UPDATE sources SET enabled = 0,
    note = CASE WHEN note IS NULL OR note = '' THEN '微博渠道已移除'
                ELSE note || '；微博渠道已移除' END
WHERE feed_url LIKE 'weibo://user/%';
`,
	7: `
-- REQ-036 / REQ-037（DEC-027，2026-11-10）：定时报告、推送渠道与过滤规则。
ALTER TABLE items ADD COLUMN hidden INTEGER NOT NULL DEFAULT 0;
CREATE INDEX idx_items_hidden ON items(hidden) WHERE hidden = 1;

CREATE TABLE push_channels (
    id          INTEGER PRIMARY KEY,
    name        TEXT NOT NULL,
    kind        TEXT NOT NULL CHECK (kind IN ('feishu', 'generic_webhook')),
    url_enc     TEXT NOT NULL,
    enabled     INTEGER NOT NULL DEFAULT 1,
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);

CREATE TABLE digest_rules (
    id              INTEGER PRIMARY KEY,
    name            TEXT NOT NULL,
    scope_json      TEXT NOT NULL DEFAULT '[]',
    period          TEXT NOT NULL CHECK (period IN ('daily', 'weekly')),
    run_at_local    TEXT NOT NULL DEFAULT '09:00',
    weekday         INTEGER NOT NULL DEFAULT 1,
    only_unread     INTEGER NOT NULL DEFAULT 0,
    push_when_empty INTEGER NOT NULL DEFAULT 0,
    enabled         INTEGER NOT NULL DEFAULT 1,
    last_run_at     TEXT,
    next_run_at     TEXT,
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL
);
CREATE INDEX idx_digest_rules_due ON digest_rules(enabled, next_run_at);

CREATE TABLE digest_reports (
    id           INTEGER PRIMARY KEY,
    rule_id      INTEGER NOT NULL REFERENCES digest_rules(id) ON DELETE CASCADE,
    period_start TEXT NOT NULL,
    period_end   TEXT NOT NULL,
    item_count   INTEGER NOT NULL DEFAULT 0,
    status       TEXT NOT NULL CHECK (status IN ('ok', 'empty', 'partial', 'error')),
    content_md   TEXT NOT NULL,
    model        TEXT,
    push_status  TEXT NOT NULL DEFAULT 'none' CHECK (push_status IN ('none', 'ok', 'failed', 'partial')),
    push_error   TEXT,
    pushed_at    TEXT,
    created_at   TEXT NOT NULL
);
CREATE INDEX idx_digest_reports_rule ON digest_reports(rule_id, created_at DESC);

CREATE TABLE filter_rules (
    id          INTEGER PRIMARY KEY,
    name        TEXT NOT NULL,
    pattern     TEXT NOT NULL,
    is_regex    INTEGER NOT NULL DEFAULT 0,
    match_field TEXT NOT NULL DEFAULT 'any' CHECK (match_field IN ('title', 'content', 'any')),
    action      TEXT NOT NULL CHECK (action IN ('mark_read', 'hide')),
    scope_json  TEXT NOT NULL DEFAULT '[]',
    enabled     INTEGER NOT NULL DEFAULT 1,
    hit_count   INTEGER NOT NULL DEFAULT 0,
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);
`,
	8: `
-- 2026-11-10 修正：首个 v0.1.3 包的迁移 7 没有 secret_enc（微服已升到 schema 7，
-- 修订迁移 7 不会重跑），故以迁移 8 补列。飞书签名校验密钥（可选）；generic_webhook 不使用。
ALTER TABLE push_channels ADD COLUMN secret_enc TEXT;
`,
	9: `
-- REQ-036 修正：报告规则需要记录勾选的推送渠道（此前前端提交的 channel_ids 被后端忽略，
-- 实际推送到所有启用渠道）。补列并将存量规则回填为当前全部启用渠道，保持既有行为。
ALTER TABLE digest_rules ADD COLUMN channel_ids_json TEXT NOT NULL DEFAULT '[]';
UPDATE digest_rules
SET channel_ids_json = COALESCE((SELECT json_group_array(id) FROM push_channels WHERE enabled = 1), '[]')
WHERE channel_ids_json = '[]';
`,
	10: `
-- REQ-038（DEC-029，2026-11-11）：MCP 访问令牌。只保存 SHA-256 哈希与打码值，
-- 完整令牌仅创建时返回一次；删除令牌即立即吊销。
CREATE TABLE mcp_tokens (
    id           INTEGER PRIMARY KEY,
    name         TEXT NOT NULL,
    masked       TEXT NOT NULL,
    token_hash   TEXT NOT NULL UNIQUE,
    enabled      INTEGER NOT NULL DEFAULT 1,
    created_at   TEXT NOT NULL,
    last_used_at TEXT
);
CREATE INDEX idx_mcp_tokens_hash ON mcp_tokens(token_hash);
`,
	11: `
-- REQ-039（2026-11-12）：阅读增强。later_at 表示稍后读；item_notes 保存正文摘录、
-- 个人笔记与高亮颜色。笔记随条目级联删除，不进入 Feed/OPML 输出。
ALTER TABLE items ADD COLUMN later_at TEXT;
CREATE INDEX idx_items_later ON items(later_at) WHERE later_at IS NOT NULL;
CREATE TABLE item_notes (
    id         INTEGER PRIMARY KEY,
    item_id    INTEGER NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    quote      TEXT NOT NULL DEFAULT '',
    note       TEXT NOT NULL DEFAULT '',
    color      TEXT NOT NULL DEFAULT 'yellow' CHECK (color IN ('yellow', 'green', 'blue', 'pink')),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE INDEX idx_item_notes_item ON item_notes(item_id, created_at);
`,
}
