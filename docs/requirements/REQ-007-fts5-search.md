# REQ-007 FTS5 全文搜索

- 状态：已完成（本地验证）
- 优先级：P1
- 关联功能：F-39
- 事实来源：[PRD.md](../../PRD.md)、[PDD.md](../../PDD.md) §9、[DECISIONS](../DECISIONS.md) DEC-009

## 规则

- 迁移 `3` 建立 `items_fts` FTS5 表，使用 `trigram` tokenizer，索引标题、正文文本、作者。
- 插入 / 删除 / 更新相关字段由触发器同步；旧库迁移时回填现有条目。
- 查询 ≥3 个字符使用 FTS5 `MATCH`（整段短语），源名称仍用 LIKE；<3 个字符或 FTS 查询失败时回退到原有 LIKE 全字段搜索。
- 两字中文词仍可搜索（LIKE 回退），三字及以上中文/英文使用 FTS5。
- 不改变分页、排序、已读/收藏/来源筛选和 API 形状。

## 实现落点

- `server/internal/database/db.go`：SchemaVersion 3、`items_fts` 与触发器
- `server/internal/server/items.go`：`queryItems` / `queryItemsWithMode`、`ftsPhrase`、失败回退
- 前端搜索无需改动，继续使用 `/items?q=`。

## 证据

- `server/internal/server/search_test.go`：`网页排版`（FTS）、`网页`（LIKE 回退）、`张三`（作者）各返回 1 条；源名称搜索返回 2 条。
- `go test ./...`、`go vet ./...` 通过；modernc SQLite 的 `trigram` smoke test 通过。
- 备份恢复测试仍通过，触发器在恢复时重建 FTS 行。
- 真实 Go 服务 + 浏览器（2026-11-01）：搜索「网页排版」返回 1 条 E2E 中文条目；迁移 1→3 回填测试通过。

## 未验证

- 10 万条以上数据库的索引体积、写入放大和查询延迟。
- 日文、韩文、emoji、超长查询和复杂 FTS 语法前缀。
- 真机 SQLite 构建是否包含 FTS5（当前 modernc 本地已验证）。
