# REQ-006 条目保留策略

- 状态：已完成（本地验证）
- 优先级：P0
- 关联功能：F-28
- 事实来源：[PRD.md](../../PRD.md)、[PDD.md](../../PDD.md) §9.5、[DECISIONS](../DECISIONS.md) DEC-008

## 规则

- 每源可配置 `retention_days`（0–3650，0 不限）和 `retention_max_items`（0–100000，0 不限）。
- 只清理「已读且未收藏」的条目；未读和收藏永不因保留策略删除。
- 先按天数删除早于 `COALESCE(published_at, fetched_at)` 的条目，再按最大条数保留最新的已读未收藏条目。
- 调度器启动后立即执行一次，此后每 6 小时最多一次；不新增独立进程。
- 迁移 `2` 增加两个 `sources` 列和保留查询索引；不改既有数据语义。

## 实现落点

- `server/internal/database/db.go`：SchemaVersion 2、字段和索引
- `server/internal/server/retention.go`：`applyRetention` / `maybeApplyRetention`
- `server/internal/server/source_store.go`、`handlers_sources.go`：创建/更新与校验
- `server/internal/server/scheduler.go`：调度钩子
- `web/src/components/sources/EditSourceDialog.tsx`：每源保留天数与条数

## 证据

- `server/internal/server/retention_test.go`：40 天前已读被删除、未读与收藏保留、最大 2 条时第三条已读被删除；创建/更新 30 天、500 条、0/100 条 roundtrip 通过。
- `go test ./...` 通过；前端 `EditSourceDialog` 显示保留天数、最多保留条数与保护提示。
- Playwright（mock）：编辑源弹窗含「保留天数」「最多保留条数」，保存后提示「源设置已更新」。
- 真实 Go 服务 + 浏览器（2026-11-01）：创建源时 `retention_days=30` / `retention_max_items=50`，编辑弹窗回显 30 / 50；API 扫描与更新 roundtrip 通过。

## 未验证

- 真实调度周期的 6 小时间隔与长跑稳定性。
- 媒体缓存回收、VACUUM 和大量条目下的删除耗时。
- 真机/外部源数据。
