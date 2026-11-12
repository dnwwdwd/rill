package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"rill/internal/database"
)

func (a *App) handleBackup(w http.ResponseWriter, r *http.Request) error {
	backup, err := a.createBackup(r.Context())
	if err != nil {
		return internalError(err)
	}
	writeJSON(w, http.StatusOK, backup)
	return nil
}

func (a *App) createBackup(ctx context.Context) (BackupSettings, error) {
	a.backupMu.Lock()
	defer a.backupMu.Unlock()
	backupDir := filepath.Join(a.cfg.DataDir, "backups")
	if !pathWithin(a.cfg.DataDir, backupDir) {
		return BackupSettings{}, errors.New("backup directory escapes data directory")
	}
	if err := os.MkdirAll(backupDir, 0o750); err != nil {
		return BackupSettings{}, err
	}
	now := a.now()
	name := "rill-" + now.Format("20060102-150405") + ".db"
	path := filepath.Join(backupDir, name)
	for suffix := 1; ; suffix++ {
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			break
		}
		path = filepath.Join(backupDir, fmt.Sprintf("rill-%s-%d.db", now.Format("20060102-150405"), suffix))
	}
	if err := database.VacuumInto(ctx, a.db, path); err != nil {
		return BackupSettings{}, err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return BackupSettings{}, err
	}
	internal, err := a.loadInternalSettings(ctx)
	if err != nil {
		return BackupSettings{}, err
	}
	stamp := now.Format(timeRFC3339)
	internal.Backup.LastBackupAt = &stamp
	if err := a.saveInternalSettings(ctx, internal); err != nil {
		return BackupSettings{}, err
	}
	if err := a.pruneBackups(backupDir, internal.Backup.Keep); err != nil {
		a.logger.Warn("prune backups failed", "err", err)
	}
	return internal.Backup, nil
}

func (a *App) pruneBackups(dir string, keep int) error {
	if keep < 1 {
		keep = 1
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	type fileInfo struct {
		name    string
		modTime time.Time
	}
	files := make([]fileInfo, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "rill-") || !strings.HasSuffix(name, ".db") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, fileInfo{name: name, modTime: info.ModTime()})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].modTime.After(files[j].modTime) })
	for index := keep; index < len(files); index++ {
		_ = os.Remove(filepath.Join(dir, files[index].name))
	}
	return nil
}

func (a *App) maybeDailyBackup(ctx context.Context) {
	internal, err := a.loadInternalSettings(ctx)
	if err != nil || !internal.Backup.Daily {
		return
	}
	if internal.Backup.LastBackupAt != nil {
		if parsed, err := time.Parse(time.RFC3339, *internal.Backup.LastBackupAt); err == nil {
			if parsed.UTC().Format("2006-01-02") == a.now().Format("2006-01-02") {
				return
			}
		}
	}
	if _, err := a.createBackup(ctx); err != nil {
		a.logger.Warn("daily backup failed", "err", err)
	}
}

func (a *App) handleRestore(w http.ResponseWriter, r *http.Request) error {
	const maxBackupBytes = 512 << 20
	r.Body = http.MaxBytesReader(w, r.Body, maxBackupBytes)
	backupDir := filepath.Join(a.cfg.DataDir, "backups")
	if err := os.MkdirAll(backupDir, 0o750); err != nil {
		return internalError(err)
	}
	temp, err := os.CreateTemp(backupDir, "restore-upload-*.db")
	if err != nil {
		return internalError(err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	defer os.Remove(tempPath + "-wal")
	defer os.Remove(tempPath + "-shm")
	var source io.Reader = r.Body
	if strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "multipart/form-data") {
		if err := r.ParseMultipartForm(8 << 20); err != nil {
			_ = temp.Close()
			return badRequest("invalid_backup", "无法读取上传文件")
		}
		file, _, err := r.FormFile("file")
		if err != nil {
			_ = temp.Close()
			return badRequest("invalid_backup", "请选择备份文件")
		}
		defer file.Close()
		source = file
	} else if r.Header.Get("Content-Type") != "" && !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "application/octet-stream") {
		// Accept raw SQLite bodies, but reject JSON-shaped requests early.
		if strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "application/json") {
			_ = temp.Close()
			return badRequest("invalid_backup", "请上传 SQLite 备份文件")
		}
	}
	if _, err := io.Copy(temp, io.LimitReader(source, maxBackupBytes)); err != nil {
		_ = temp.Close()
		return badRequest("invalid_backup", "读取上传文件失败")
	}
	if err := temp.Close(); err != nil {
		return internalError(err)
	}
	info, err := os.Stat(tempPath)
	if err != nil || info.Size() == 0 {
		return badRequest("invalid_backup", "上传的备份为空")
	}
	if err := database.PrepareForRestore(r.Context(), tempPath, backupDir); err != nil {
		return badRequest("invalid_backup", err.Error())
	}
	// A safety copy must succeed before replacing any live data.
	if _, err := a.createBackup(r.Context()); err != nil {
		return internalError(fmt.Errorf("create safety backup: %w", err))
	}
	if err := a.restoreFromFile(r.Context(), tempPath); err != nil {
		return internalError(err)
	}
	if err := a.bootstrapSettings(r.Context()); err != nil {
		return internalError(err)
	}
	a.cleanupSessions(r.Context())
	writeNoContent(w)
	return nil
}

func (a *App) restoreFromFile(ctx context.Context, path string) error {
	if !pathWithin(a.cfg.DataDir, path) {
		return errors.New("restore path escapes data directory")
	}
	if _, err := a.db.ExecContext(ctx, `ATTACH DATABASE ? AS restored`, path); err != nil {
		return fmt.Errorf("attach backup: %w", err)
	}
	defer func() {
		_, _ = a.db.ExecContext(context.Background(), `DETACH DATABASE restored`)
	}()
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	deletes := []string{"item_notes", "ai_results", "fetch_logs", "items", "media_cache", "sources", "folders", "settings"}
	for _, table := range deletes {
		if _, err := tx.ExecContext(ctx, `DELETE FROM main.`+table); err != nil {
			return fmt.Errorf("clear %s: %w", table, err)
		}
	}
	copies := []string{"folders", "sources", "items", "item_notes", "fetch_logs", "media_cache", "ai_results", "settings"}
	for _, table := range copies {
		if _, err := tx.ExecContext(ctx, `INSERT INTO main.`+table+` SELECT * FROM restored.`+table); err != nil {
			return fmt.Errorf("copy %s: %w", table, err)
		}
	}
	var badRows int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM pragma_foreign_key_check`).Scan(&badRows); err != nil {
		// FK check is not available in some SQLite builds; the inserts above
		// already ran under foreign_keys=ON.
		a.logDebug("foreign_key_check unavailable", "err", err)
	} else if badRows > 0 {
		return fmt.Errorf("restored database has %d foreign key violations", badRows)
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

// handleBackupDownload creates a one-off database copy and streams it to the
// browser as a download (the Lazycat file-chooser download inject intercepts
// the object URL). The temporary copy never lands in the backups directory.
func (a *App) handleBackupDownload(w http.ResponseWriter, r *http.Request) error {
	if err := os.MkdirAll(a.cfg.CacheDir, 0o750); err != nil {
		return internalError(err)
	}
	tmp, err := os.CreateTemp(a.cfg.CacheDir, "rill-backup-*.db")
	if err != nil {
		return internalError(err)
	}
	path := tmp.Name()
	tmp.Close()
	defer os.Remove(path)
	if err := database.VacuumInto(r.Context(), a.db, path); err != nil {
		return internalError(err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return internalError(err)
	}
	file, err := os.Open(path)
	if err != nil {
		return internalError(err)
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		return internalError(err)
	}
	name := "rill-backup-" + a.now().Format("20060102-150405") + ".db"
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, name))
	http.ServeContent(w, r, name, stat.ModTime(), file)
	return nil
}
