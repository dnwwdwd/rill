package server

import (
	"net/http"
	"strconv"
	"strings"
)

func (a *App) routes() http.Handler {
	api := http.NewServeMux()
	api.HandleFunc("GET /api/v1/auth/config", a.handler(a.handleAuthConfig))
	api.HandleFunc("POST /api/v1/auth/logout", a.handler(a.handleLogout))
	api.HandleFunc("GET /api/v1/auth/csrf", a.handler(a.handleCSRF))

	api.HandleFunc("GET /api/v1/folders", a.handler(a.handleListFolders))
	api.HandleFunc("POST /api/v1/folders", a.handler(a.handleCreateFolder))
	api.HandleFunc("PATCH /api/v1/folders/{id}", a.handler(a.handleUpdateFolder))
	api.HandleFunc("DELETE /api/v1/folders/{id}", a.handler(a.handleDeleteFolder))

	api.HandleFunc("GET /api/v1/sources", a.handler(a.handleListSources))
	api.HandleFunc("POST /api/v1/sources", a.handler(a.handleCreateSource))
	api.HandleFunc("POST /api/v1/sources/batch", a.handler(a.handleBatchCreateSources))
	api.HandleFunc("POST /api/v1/sources/batch-update", a.handler(a.handleBatchUpdateSources))
	api.HandleFunc("POST /api/v1/sources/batch-delete", a.handler(a.handleBatchDeleteSources))
	api.HandleFunc("POST /api/v1/sources/discover", a.handler(a.handleDiscoverFeed))
	api.HandleFunc("POST /api/v1/sources/resolve", a.handler(a.handleResolveSource))
	api.HandleFunc("POST /api/v1/sources/search", a.handler(a.handleSearchCreators))
	api.HandleFunc("GET /api/v1/sources/{id}", a.handler(a.handleGetSource))
	api.HandleFunc("PATCH /api/v1/sources/{id}", a.handler(a.handleUpdateSource))
	api.HandleFunc("DELETE /api/v1/sources/{id}", a.handler(a.handleDeleteSource))
	api.HandleFunc("POST /api/v1/sources/{id}/fetch", a.handler(a.handleFetchSource))
	api.HandleFunc("POST /api/v1/sources/{id}/fallback", a.handler(a.handleFallbackSource))
	api.HandleFunc("POST /api/v1/sources/{id}/token/reset", a.handler(a.handleResetSourceToken))
	api.HandleFunc("GET /api/v1/sources/{id}/logs", a.handler(a.handleSourceLogs))
	api.HandleFunc("GET /api/v1/sources/{id}/icon", a.handler(a.handleSourceIcon))
	api.HandleFunc("GET /api/v1/media/candidate/{hash}", a.handler(a.handleCandidateIcon))

	api.HandleFunc("GET /api/v1/items", a.handler(a.handleListItems))
	api.HandleFunc("GET /api/v1/items/stats", a.handler(a.handleItemStats))
	api.HandleFunc("PATCH /api/v1/items/state", a.handler(a.handleBatchItemState))
	api.HandleFunc("POST /api/v1/items/mark-all-read", a.handler(a.handleMarkAllRead))
	api.HandleFunc("GET /api/v1/items/{id}", a.handler(a.handleGetItem))
	api.HandleFunc("GET /api/v1/items/{id}/markdown", a.handler(a.handleGetItemMarkdown))
	api.HandleFunc("POST /api/v1/items/{id}/fulltext", a.handler(a.handleFetchItemFullText))
	api.HandleFunc("PUT /api/v1/items/{id}/podcast-progress", a.handler(a.handlePodcastProgress))
	api.HandleFunc("PATCH /api/v1/items/{id}/state", a.handler(a.handlePatchItemState))
	api.HandleFunc("PATCH /api/v1/items/{id}/email-images", a.handler(a.handlePatchItemEmailImages))
	api.HandleFunc("GET /api/v1/items/{id}/notes", a.handler(a.handleListItemNotes))
	api.HandleFunc("POST /api/v1/items/{id}/notes", a.handler(a.handleCreateItemNote))
	api.HandleFunc("PATCH /api/v1/items/{id}/notes/{note_id}", a.handler(a.handleUpdateItemNote))
	api.HandleFunc("DELETE /api/v1/items/{id}/notes/{note_id}", a.handler(a.handleDeleteItemNote))
	api.HandleFunc("POST /api/v1/items/{id}/summary", a.handler(a.handleGenerateSummary))
	api.HandleFunc("POST /api/v1/items/{id}/summary/stream", a.handler(a.handleSummaryStream))
	api.HandleFunc("POST /api/v1/items/{id}/translation", a.handler(a.handleTranslateItem))
	api.HandleFunc("POST /api/v1/items/{id}/translation/stream", a.handler(a.handleTranslationStream))

	api.HandleFunc("GET /api/v1/search", a.handler(a.handleSearch))
	api.HandleFunc("GET /api/v1/settings", a.handler(a.handleGetSettings))
	api.HandleFunc("PATCH /api/v1/settings", a.handler(a.handlePatchSettings))
	api.HandleFunc("POST /api/v1/settings/ai/test", a.handler(a.handleTestAI))
	api.HandleFunc("POST /api/v1/settings/email/test", a.handler(a.handleTestEmail))
	api.HandleFunc("POST /api/v1/settings/feed-token/reset", a.handler(a.handleResetGlobalToken))
	api.HandleFunc("POST /api/v1/settings/feed-token/reveal", a.handler(a.handleRevealGlobalToken))
	api.HandleFunc("POST /api/v1/wechat/resolve", a.handler(a.handleWechatResolveArticle))
	api.HandleFunc("POST /api/v1/wechat/sources", a.handler(a.handleWechatCreateSource))
	api.HandleFunc("POST /api/v1/wechat/weread/scan", a.handler(a.handleWereadScan))
	api.HandleFunc("GET /api/v1/wechat/weread/scan/status", a.handler(a.handleWereadScanStatus))
	api.HandleFunc("POST /api/v1/backup", a.handler(a.handleBackup))
	api.HandleFunc("GET /api/v1/backup/download", a.handler(a.handleBackupDownload))
	api.HandleFunc("POST /api/v1/backup/restore", a.handler(a.handleRestore))
	api.HandleFunc("GET /api/v1/opml/export", a.handler(a.handleExportOPML))
	api.HandleFunc("POST /api/v1/opml/import", a.handler(a.handleImportOPML))
	api.HandleFunc("GET /api/v1/channels", a.handler(a.handleListChannels))
	api.HandleFunc("POST /api/v1/channels", a.handler(a.handleCreateChannel))
	api.HandleFunc("PUT /api/v1/channels/{id}", a.handler(a.handleUpdateChannel))
	api.HandleFunc("DELETE /api/v1/channels/{id}", a.handler(a.handleDeleteChannel))
	api.HandleFunc("POST /api/v1/channels/{id}/test", a.handler(a.handleTestChannel))

	api.HandleFunc("GET /api/v1/digest/rules", a.handler(a.handleListDigestRules))
	api.HandleFunc("POST /api/v1/digest/rules", a.handler(a.handleCreateDigestRule))
	api.HandleFunc("PUT /api/v1/digest/rules/{id}", a.handler(a.handleUpdateDigestRule))
	api.HandleFunc("DELETE /api/v1/digest/rules/{id}", a.handler(a.handleDeleteDigestRule))
	api.HandleFunc("POST /api/v1/digest/rules/{id}/run", a.handler(a.handleRunDigestRule))
	api.HandleFunc("GET /api/v1/digest/reports", a.handler(a.handleListDigestReports))
	api.HandleFunc("GET /api/v1/digest/reports/{id}", a.handler(a.handleGetDigestReport))
	api.HandleFunc("POST /api/v1/digest/reports/{id}/push", a.handler(a.handlePushDigestReport))

	api.HandleFunc("GET /api/v1/filters/rules", a.handler(a.handleListFilterRules))
	api.HandleFunc("POST /api/v1/filters/rules", a.handler(a.handleCreateFilterRule))
	api.HandleFunc("PUT /api/v1/filters/rules/{id}", a.handler(a.handleUpdateFilterRule))
	api.HandleFunc("DELETE /api/v1/filters/rules/{id}", a.handler(a.handleDeleteFilterRule))
	api.HandleFunc("POST /api/v1/filters/rules/{id}/apply", a.handler(a.handleApplyFilterRule))

	api.HandleFunc("GET /api/v1/mcp/tokens", a.handler(a.handleListMCPTokens))
	api.HandleFunc("POST /api/v1/mcp/tokens", a.handler(a.handleCreateMCPToken))
	api.HandleFunc("PATCH /api/v1/mcp/tokens/{id}", a.handler(a.handleUpdateMCPToken))
	api.HandleFunc("DELETE /api/v1/mcp/tokens/{id}", a.handler(a.handleDeleteMCPToken))

	api.HandleFunc("GET /api/v1/healthz", a.handler(a.handleDetailedHealth))
	api.HandleFunc("/api/v1/", a.handler(func(w http.ResponseWriter, r *http.Request) error {
		return notFound("not_found", "接口不存在")
	}))

	root := http.NewServeMux()
	root.Handle("/api/v1/", a.apiMiddleware(api))
	root.HandleFunc("GET /healthz", a.handlePublicHealth)
	root.HandleFunc("GET /auth/oidc/login", a.handleOIDCLogin)
	root.HandleFunc("GET /auth/oidc/callback", a.handleOIDCCallback)
	root.HandleFunc("GET /feed/all/{file}", a.handler(a.handleAggregateFeed))
	root.HandleFunc("GET /feed/{id}/{file}", a.handler(a.handleSourceFeed))
	root.HandleFunc("GET /opml/{file}", a.handler(a.handlePublicOPML))
	root.HandleFunc("GET /media/{token}/{hash}", a.handler(a.handleMedia))
	root.Handle("/mcp", a.mcpMiddleware(a.mcpHTTPHandler()))
	root.Handle("/mcp/", a.mcpMiddleware(a.mcpHTTPHandler()))
	root.HandleFunc("/", a.handleStatic)
	return root
}

func apiRouteID(r *http.Request) (int64, error) {
	raw := strings.TrimSpace(r.PathValue("id"))
	if raw == "" {
		return 0, badRequest("invalid_id", "缺少资源 ID")
	}
	var id int64
	parsed, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || parsed <= 0 {
		return 0, badRequest("invalid_id", "资源 ID 无效")
	}
	id = parsed
	return id, nil
}
