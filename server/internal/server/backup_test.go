package server

import (
	"bytes"
	"context"
	"encoding/xml"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"rill/internal/database"
)

func TestBackupAndRestore(t *testing.T) {
	app, client := newTestApp(t, true)
	client.login()
	var before Folder
	client.decode(http.MethodPost, "/api/v1/folders", map[string]any{"name": "Before"}, true, http.StatusCreated, &before)

	var backup BackupSettings
	client.decode(http.MethodPost, "/api/v1/backup", nil, true, http.StatusOK, &backup)
	if backup.LastBackupAt == nil {
		t.Fatal("last_backup_at missing")
	}
	entries, err := os.ReadDir(filepath.Join(app.cfg.DataDir, "backups"))
	if err != nil {
		t.Fatal(err)
	}
	var backupPath string
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "rill-") && strings.HasSuffix(entry.Name(), ".db") {
			backupPath = filepath.Join(app.cfg.DataDir, "backups", entry.Name())
		}
	}
	if backupPath == "" {
		t.Fatal("backup file not found")
	}
	if err := database.ValidateFile(context.Background(), backupPath, database.SchemaVersion); err != nil {
		t.Fatal(err)
	}

	client.decode(http.MethodPost, "/api/v1/folders", map[string]any{"name": "After"}, true, http.StatusCreated, nil)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "restore.db")
	if err != nil {
		t.Fatal(err)
	}
	fileData, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(fileData); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	response, data := client.raw(http.MethodPost, "/api/v1/backup/restore", body.Bytes(), writer.FormDataContentType(), client.csrf)
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("restore: %d %s", response.StatusCode, data)
	}
	var folders []Folder
	client.decode(http.MethodGet, "/api/v1/folders", nil, false, http.StatusOK, &folders)
	if len(folders) != 1 || folders[0].Name != "Before" {
		t.Fatalf("restored folders: %+v", folders)
	}
}

func TestOPMLNestedImportExportAndPublicToken(t *testing.T) {
	app, client := newTestApp(t, true)
	client.login()
	opml := `<?xml version="1.0" encoding="UTF-8"?>
<opml version="2.0"><head><title>Import</title></head><body>
  <outline text="Tech" title="Tech">
    <outline text="Web" title="Web">
      <outline text="Frontend" title="Frontend">
        <outline type="rss" text="Example" title="Example" xmlUrl="https://example.com/feed.xml" htmlUrl="https://example.com"/>
      </outline>
    </outline>
  </outline>
  <outline type="rss" text="Root" title="Root" xmlUrl="https://example.org/feed.xml"/>
</body></opml>`
	response, data := client.raw(http.MethodPost, "/api/v1/opml/import", []byte(opml), "text/xml", client.csrf)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("import: %d %s", response.StatusCode, data)
	}
	if !strings.Contains(string(data), `"imported":2`) {
		t.Fatalf("import result: %s", data)
	}

	response, export := client.raw(http.MethodGet, "/api/v1/opml/export", nil, "", "")
	if response.StatusCode != http.StatusOK {
		t.Fatalf("export: %d %s", response.StatusCode, export)
	}
	var doc opmlDocument
	if err := xml.Unmarshal(export, &doc); err != nil {
		t.Fatalf("export parse: %v\n%s", err, export)
	}
	if len(doc.Body.Outlines) != 2 {
		t.Fatalf("root outlines: %+v", doc.Body.Outlines)
	}
	var tech *opmlOutline
	for index := range doc.Body.Outlines {
		if doc.Body.Outlines[index].Title == "Tech" {
			tech = &doc.Body.Outlines[index]
		}
	}
	if tech == nil || len(tech.Outlines) != 1 ||
		len(tech.Outlines[0].Outlines) != 1 ||
		len(tech.Outlines[0].Outlines[0].Outlines) != 1 ||
		tech.Outlines[0].Outlines[0].Outlines[0].XMLURL != "https://example.com/feed.xml" {
		t.Fatalf("nested export: %+v", doc.Body.Outlines)
	}

	globalToken, err := app.globalFeedToken(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	response, public := client.raw(http.MethodGet, "/opml/"+globalToken+".opml", nil, "", "")
	if response.StatusCode != http.StatusOK || !bytes.Contains(public, []byte("example.com/feed.xml")) {
		t.Fatalf("public opml: %d %s", response.StatusCode, public)
	}
	response, _ = client.raw(http.MethodGet, "/opml/wrong-token.opml", nil, "", "")
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("bad opml token: %d", response.StatusCode)
	}

	response, data = client.raw(http.MethodPost, "/api/v1/opml/import", []byte(opml), "text/xml", client.csrf)
	if response.StatusCode != http.StatusOK || !strings.Contains(string(data), `"imported":0`) {
		t.Fatalf("duplicate import: %d %s", response.StatusCode, data)
	}
}
