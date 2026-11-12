# PROG-REQ-004～007 P0/P1 后端收尾

- 关联需求：[REQ-004](../requirements/REQ-004-full-feed-token.md)、[REQ-005](../requirements/REQ-005-source-icons.md)、[REQ-006](../requirements/REQ-006-retention.md)、[REQ-007](../requirements/REQ-007-fts5-search.md)
- 状态：已完成（本地验证）
- 最近更新：2026-11-01

## 阶段

| 阶段 | 目标 | 状态 | 交付物 / 证据 |
|---|---|---|---|
| 1 | 源图标抓取、缓存、受保护代理与首字母回退 | 已完成 | 迁移 0；`icon.go`；`TestSourceIconProxyAndCache`；真实浏览器 `naturalWidth>0` |
| 2 | 每源保留字段、清理任务与未读/收藏保护 | 已完成 | SchemaVersion 2；`retention.go`；`TestApplyRetention...`、`TestRetentionSettingsRoundTrip` |
| 3 | FTS5 trigram 索引、触发器与短查询回退 | 已完成 | SchemaVersion 3；`items_fts`；`TestItemSearch...`、`TestMigrateFromV1BackfillsFTS` |
| 4 | 全站 token 安全 reveal 与 UI | 已完成 | `POST /settings/feed-token/reveal`；`TestRevealGlobalFeedToken`；真实浏览器 masked→完整 |
| 5 | 真实外部 X/公众号与 LPK | 暂缓/外部依赖 | 用户确认暂不打包 LPK；真实 FxTwitter 与 sidecar 仍待外部环境 |

## 验证

- `go test -count=1 ./...`、`go vet ./...`、`go test -race -count=1 ./...`、`CGO_ENABLED=0 go build ./cmd/rill`：通过。
- `npm run typecheck`、17 个 Vitest 用例、`npm run build`：通过。
- 真实 Go 服务（本地 feed + 浏览器）：源图标加载、FTS 搜索、token reveal、保留字段、OPML/文件夹 roundtrip 均通过；0 JS 错误。
- 文档：[DEC-007～010](../DECISIONS.md)、PRD v0.10、PDD v0.8。

## 未验证

- 真实外部站点图标、长跑 6 小时保留任务、10 万条 FTS 性能和多语言、剪贴板复制、真机缓存权限。
- 真实 X/FxTwitter 与 `we-mp-rss` sidecar；LPK 按用户确认暂不打包。
