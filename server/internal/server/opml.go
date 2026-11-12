package server

import (
	"context"
	"database/sql"
	"encoding/xml"
	"errors"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

type opmlDocument struct {
	XMLName xml.Name `xml:"opml"`
	Version string   `xml:"version,attr"`
	Head    opmlHead `xml:"head"`
	Body    opmlBody `xml:"body"`
}

type opmlHead struct {
	Title string `xml:"title"`
}

type opmlBody struct {
	Outlines []opmlOutline `xml:"outline"`
}

type opmlOutline struct {
	Text     string        `xml:"text,attr,omitempty"`
	Title    string        `xml:"title,attr,omitempty"`
	Type     string        `xml:"type,attr,omitempty"`
	XMLURL   string        `xml:"xmlUrl,attr,omitempty"`
	HTMLURL  string        `xml:"htmlUrl,attr,omitempty"`
	Outlines []opmlOutline `xml:"outline"`
}

type opmlFolderRow struct {
	ID       int64
	Name     string
	Sort     int
	ParentID sql.NullInt64
}

type opmlSourceRow struct {
	Name     string
	FeedURL  string
	SiteURL  sql.NullString
	FolderID sql.NullInt64
}

func (a *App) handleExportOPML(w http.ResponseWriter, r *http.Request) error {
	folderID, err := optionalFolderQuery(r)
	if err != nil {
		return err
	}
	if folderID != nil {
		if err := a.ensureFolderExists(r.Context(), *folderID); err != nil {
			return err
		}
	}
	data, err := a.buildOPML(r.Context(), folderID)
	if err != nil {
		return internalError(err)
	}
	w.Header().Set("Content-Type", "text/x-opml; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="rill-subscriptions.opml"`)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
	return nil
}

func optionalFolderQuery(r *http.Request) (*int64, error) {
	raw := strings.TrimSpace(r.URL.Query().Get("folder_id"))
	if raw == "" {
		return nil, nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		return nil, badRequest("invalid_folder", "folder_id 无效")
	}
	return &value, nil
}

func (a *App) handlePublicOPML(w http.ResponseWriter, r *http.Request) error {
	file := r.PathValue("file")
	if !strings.HasSuffix(file, ".opml") {
		http.NotFound(w, r)
		return nil
	}
	token := strings.TrimSuffix(file, ".opml")
	globalToken, err := a.globalFeedToken(r.Context())
	if err != nil || token == "" || !constantTimeEqual(token, globalToken) {
		http.NotFound(w, r)
		return nil
	}
	data, err := a.buildOPML(r.Context(), nil)
	if err != nil {
		http.Error(w, "cannot build OPML", http.StatusInternalServerError)
		return nil
	}
	w.Header().Set("Content-Type", "text/x-opml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=60")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
	return nil
}

func (a *App) buildOPML(ctx context.Context, folderID *int64) ([]byte, error) {
	folderRows, err := a.db.QueryContext(ctx, `SELECT id, name, sort, parent_id FROM folders ORDER BY sort ASC, id ASC`)
	if err != nil {
		return nil, err
	}
	folders := make([]opmlFolderRow, 0)
	for folderRows.Next() {
		var row opmlFolderRow
		if err := folderRows.Scan(&row.ID, &row.Name, &row.Sort, &row.ParentID); err != nil {
			_ = folderRows.Close()
			return nil, err
		}
		folders = append(folders, row)
	}
	if err := folderRows.Close(); err != nil {
		return nil, err
	}
	sourceRows, err := a.db.QueryContext(ctx, `SELECT name, feed_url, site_url, folder_id FROM sources ORDER BY name COLLATE NOCASE ASC, id ASC`)
	if err != nil {
		return nil, err
	}
	sources := make([]opmlSourceRow, 0)
	for sourceRows.Next() {
		var row opmlSourceRow
		if err := sourceRows.Scan(&row.Name, &row.FeedURL, &row.SiteURL, &row.FolderID); err != nil {
			_ = sourceRows.Close()
			return nil, err
		}
		sources = append(sources, row)
	}
	if err := sourceRows.Close(); err != nil {
		return nil, err
	}
	var rootFolder *opmlFolderRow
	if folderID != nil {
		for i := range folders {
			if folders[i].ID == *folderID {
				rootFolder = &folders[i]
				break
			}
		}
	}
	byParent := make(map[int64][]opmlFolderRow)
	for _, folder := range folders {
		parent := int64(0)
		if folder.ParentID.Valid {
			parent = folder.ParentID.Int64
		}
		byParent[parent] = append(byParent[parent], folder)
	}
	byFolder := make(map[int64][]opmlSourceRow)
	for _, source := range sources {
		folder := int64(0)
		if source.FolderID.Valid {
			folder = source.FolderID.Int64
		}
		byFolder[folder] = append(byFolder[folder], source)
	}
	var build func(parent int64, depth int) []opmlOutline
	build = func(parent int64, depth int) []opmlOutline {
		if depth > 20 {
			return nil
		}
		children := byParent[parent]
		sort.SliceStable(children, func(i, j int) bool {
			if children[i].Sort == children[j].Sort {
				return children[i].ID < children[j].ID
			}
			return children[i].Sort < children[j].Sort
		})
		result := make([]opmlOutline, 0, len(children)+len(byFolder[parent]))
		for _, folder := range children {
			outline := opmlOutline{Text: folder.Name, Title: folder.Name}
			outline.Outlines = build(folder.ID, depth+1)
			result = append(result, outline)
		}
		for _, source := range byFolder[parent] {
			outline := opmlOutline{Text: source.Name, Title: source.Name, Type: "rss", XMLURL: source.FeedURL}
			if source.SiteURL.Valid {
				outline.HTMLURL = source.SiteURL.String
			}
			result = append(result, outline)
		}
		return result
	}
	outlines := build(0, 0)
	title := "Rill subscriptions"
	if rootFolder != nil {
		outlines = []opmlOutline{{
			Text:     rootFolder.Name,
			Title:    rootFolder.Name,
			Outlines: build(rootFolder.ID, 1),
		}}
		title = rootFolder.Name
	}
	document := opmlDocument{
		Version: "2.0",
		Head:    opmlHead{Title: title},
		Body:    opmlBody{Outlines: outlines},
	}
	data, err := xml.MarshalIndent(document, "", "  ")
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), data...), nil
}

type opmlImportResult struct {
	Imported int             `json:"imported"`
	Skipped  []opmlSkipEntry `json:"skipped"`
}

type opmlSkipEntry struct {
	URL    string `json:"url"`
	Reason string `json:"reason"`
}

func (a *App) handleImportOPML(w http.ResponseWriter, r *http.Request) error {
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	reader := io.Reader(r.Body)
	if strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "multipart/form-data") {
		if err := r.ParseMultipartForm(2 << 20); err != nil {
			return badRequest("invalid_opml", "无法读取上传的 OPML")
		}
		file, _, err := r.FormFile("file")
		if err != nil {
			return badRequest("invalid_opml", "请选择 OPML 文件")
		}
		defer file.Close()
		reader = file
	}
	var document opmlDocument
	decoder := xml.NewDecoder(io.LimitReader(reader, 10<<20))
	if err := decoder.Decode(&document); err != nil {
		return badRequest("invalid_opml", "OPML 解析失败")
	}
	if document.XMLName.Local != "opml" {
		return badRequest("invalid_opml", "文件不是 OPML 2.0")
	}
	result := opmlImportResult{Imported: 0, Skipped: []opmlSkipEntry{}}
	parentFolder, err := optionalFolderQuery(r)
	if err != nil {
		return err
	}
	if parentFolder != nil {
		if err := a.ensureFolderExists(r.Context(), *parentFolder); err != nil {
			return err
		}
	}
	a.importOutlines(r.Context(), document.Body.Outlines, parentFolder, &result)
	writeJSON(w, http.StatusOK, result)
	return nil
}

func (a *App) importOutlines(ctx context.Context, outlines []opmlOutline, parentFolder *int64, result *opmlImportResult) {
	for _, outline := range outlines {
		xmlURL := strings.TrimSpace(outline.XMLURL)
		if xmlURL == "" {
			name := strings.TrimSpace(firstNonEmpty(outline.Title, outline.Text))
			if name == "" {
				a.importOutlines(ctx, outline.Outlines, parentFolder, result)
				continue
			}
			folderID, err := a.ensureOPMLFolder(ctx, name, parentFolder)
			if err != nil {
				a.logger.Warn("OPML folder import failed", "folder", name, "err", err)
				a.importOutlines(ctx, outline.Outlines, parentFolder, result)
				continue
			}
			a.importOutlines(ctx, outline.Outlines, &folderID, result)
			continue
		}
		name := strings.TrimSpace(firstNonEmpty(outline.Title, outline.Text))
		var siteURL *string
		if strings.TrimSpace(outline.HTMLURL) != "" {
			value := strings.TrimSpace(outline.HTMLURL)
			siteURL = &value
		}
		input := sourceCreateInput{
			Kind:     "rss",
			Name:     name,
			FeedURL:  xmlURL,
			SiteURL:  siteURL,
			FolderID: parentFolder,
		}
		_, err := a.createSourceRow(ctx, input)
		if err != nil {
			var apiErr *apiError
			if errors.As(err, &apiErr) && apiErr.Status < 500 {
				result.Skipped = append(result.Skipped, opmlSkipEntry{URL: xmlURL, Reason: apiErr.Message})
				continue
			}
			result.Skipped = append(result.Skipped, opmlSkipEntry{URL: xmlURL, Reason: "导入失败"})
			a.logger.Warn("OPML source import failed", "url", xmlURL, "err", err)
			continue
		}
		result.Imported++
		a.importOutlines(ctx, outline.Outlines, parentFolder, result)
	}
}

func (a *App) ensureOPMLFolder(ctx context.Context, name string, parent *int64) (int64, error) {
	var id int64
	var err error
	if parent == nil {
		err = a.db.QueryRowContext(ctx, `SELECT id FROM folders WHERE name = ? AND parent_id IS NULL`, name).Scan(&id)
	} else {
		err = a.db.QueryRowContext(ctx, `SELECT id FROM folders WHERE name = ? AND parent_id = ?`, name, *parent).Scan(&id)
	}
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	var sortOrder int
	if parent == nil {
		_ = a.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(sort), 0) + 1 FROM folders WHERE parent_id IS NULL`).Scan(&sortOrder)
	} else {
		_ = a.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(sort), 0) + 1 FROM folders WHERE parent_id = ?`, *parent).Scan(&sortOrder)
	}
	result, err := a.db.ExecContext(ctx, `INSERT INTO folders (name, sort, parent_id, created_at) VALUES (?, ?, ?, ?)`,
		name, sortOrder, parent, a.now().Format(timeRFC3339))
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func constantTimeEqual(left, right string) bool {
	if len(left) != len(right) {
		return false
	}
	var result byte
	for i := 0; i < len(left); i++ {
		result |= left[i] ^ right[i]
	}
	return result == 0
}
