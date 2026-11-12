package server

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestBatchUpdateAndDeleteSources(t *testing.T) {
	app, client := newTestApp(t, true)
	client.login()
	s1 := createTestSource(t, client, "S1", "https://example.com/s1.xml")
	s2 := createTestSource(t, client, "S2", "https://example.com/s2.xml")
	createTestSource(t, client, "S3", "https://example.com/s3.xml")

	var folder struct {
		ID int64 `json:"id"`
	}
	client.decode("POST", "/api/v1/folders", map[string]any{"name": "技术"}, true, http.StatusCreated, &folder)

	// 批量移动分组 + 停用。
	var updateResp sourceBatchResponse
	client.decode("POST", "/api/v1/sources/batch-update", map[string]any{
		"ids": []int64{s1.ID, s2.ID}, "folder_id": folder.ID, "enabled": false,
	}, true, http.StatusOK, &updateResp)
	if updateResp.Updated != 2 {
		t.Fatalf("updated %d, want 2", updateResp.Updated)
	}
	var folderID *int64
	var enabled int
	if err := app.db.QueryRowContext(context.Background(),
		`SELECT folder_id, enabled FROM sources WHERE id = ?`, s1.ID).Scan(&folderID, &enabled); err != nil {
		t.Fatal(err)
	}
	if folderID == nil || *folderID != folder.ID || enabled != 0 {
		t.Fatalf("batch update not applied: folder=%v enabled=%d", folderID, enabled)
	}

	// folder_id=0 表示移出不分组。
	client.decode("POST", "/api/v1/sources/batch-update", map[string]any{
		"ids": []int64{s1.ID}, "folder_id": 0,
	}, true, http.StatusOK, &updateResp)
	if err := app.db.QueryRowContext(context.Background(),
		`SELECT folder_id FROM sources WHERE id = ?`, s1.ID).Scan(&folderID); err != nil {
		t.Fatal(err)
	}
	if folderID != nil {
		t.Fatalf("folder_id should be NULL, got %v", *folderID)
	}

	// 删除级联清理条目：先插条目再删除源。
	item := insertTestItem(t, app, s2.ID, "x", "t", "c", app.now().Add(-time.Hour), 0)
	client.decode("POST", "/api/v1/sources/batch-delete", map[string]any{
		"ids": []int64{s2.ID, 99999},
	}, true, http.StatusOK, &updateResp)
	if updateResp.Deleted != 1 || len(updateResp.Missing) != 1 || updateResp.Missing[0] != 99999 {
		t.Fatalf("delete resp wrong: %+v", updateResp)
	}
	var count int
	if err := app.db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM items WHERE id = ?`, item).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("items should be cascaded")
	}

	// 校验：空 ids 与无变更字段。
	response, data := client.request("POST", "/api/v1/sources/batch-update", map[string]any{"ids": []int64{s1.ID}}, true)
	if response.StatusCode != 400 {
		t.Fatalf("no-op update status %d: %s", response.StatusCode, data)
	}
	response, data = client.request("POST", "/api/v1/sources/batch-delete", map[string]any{"ids": []int64{}}, true)
	if response.StatusCode != 400 || errorCode(t, data) != "sources_required" {
		t.Fatalf("empty delete status %d: %s", response.StatusCode, data)
	}

	// 不存在的分组。
	response, data = client.request("POST", "/api/v1/sources/batch-update", map[string]any{
		"ids": []int64{s1.ID}, "folder_id": 424242,
	}, true)
	if response.StatusCode != 400 || errorCode(t, data) != "folder_not_found" {
		t.Fatalf("bad folder status %d: %s", response.StatusCode, data)
	}

	_ = fmt.Sprint()
}
