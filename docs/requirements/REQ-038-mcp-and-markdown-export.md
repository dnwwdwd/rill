# REQ-038：MCP 服务与正文 Markdown 导出

- 状态：已实现（本地验证；真实外部 Agent 与真机文件选择待验收）
- 日期：2026-11-11
- 关联：PRD F-50/F-51、PDD §4.2/§5.2/§6.5/§12.3/§12.4/§13.3、DEC-029

## 目标

1. 在设置新增「MCP」分类，把 Rill 的内容与常用写操作以 MCP 工具暴露给外部 Agent。
2. MCP 使用 Streamable HTTP（SSE）传输，`/mcp` 走 Bearer Token 认证，不依赖懒猫登录会话。
3. 支持多个命名 Token，可单独启停、吊销；完整 Token 只在创建时返回一次，服务端只保存 SHA-256 哈希与打码值。
4. 阅读正文支持复制 Markdown 与下载 `.md` 文件；下载使用 `blob:` + `<a download>`，由懒猫 file-chooser inject 接管保存位置。

## 已确认决策（2026-11-11）

- 工具范围：只读 + 写入，**不提供任何删除工具**；不暴露 AI 密钥、设置修改、备份恢复等敏感写操作。
- Token：多个命名 Token（迁移 10 新增 `mcp_tokens`），可启停/吊销。
- Markdown：始终导出订阅源原文，跟随译文模式的需求不纳入。

## MCP 传输与认证

- 端点：`/mcp`（同时注册 `/mcp/`），实现 MCP Streamable HTTP，POST 默认以 `text/event-stream` 返回。
- 认证：每次请求校验 `Authorization: Bearer <token>`；MCP 全局关闭时返回 404，Token 缺失/无效/停用返回 401 + `WWW-Authenticate: Bearer`。
- 会话：SDK 以 stateless 模式运行，不依赖 `Mcp-Session-Id`，GET 长连接返回 405；工具调用通过 POST + SSE 完成。
- LPK：`lzc-manifest.yml` 的 `public_path` 增加 `/mcp`、`/mcp/`，只放行该 Bearer 保护端点；管理 API 与 OIDC 回调仍不裸奔。
- 使用方式：设置 → MCP 打开开关，生成 Token，把 `https://<host>/mcp` 与 `Authorization: Bearer <token>` 配到 Agent。

## 工具清单

读取：

- `list_folders`、`list_sources`、`list_items`、`get_item`、`get_stats`
- `resolve_source`、`search_creators`
- `list_filter_rules`、`list_digest_rules`、`list_digest_reports`、`get_digest_report`、`list_push_channels`

写入（无删除）：

- 分组：`create_folder`、`update_folder`、`reorder_folders`
- 订阅源：`create_source`、`update_source`、`move_source`、`move_sources`、`fetch_source`、`switch_source_fallback`、`reset_source_feed_token`、`create_wechat_source`
- 文章：`update_item_state`、`mark_all_read`、`save_podcast_progress`、`generate_summary`、`translate_item`
- 自动化：`create_filter_rule`、`update_filter_rule`、`apply_filter_rule`、`create_digest_rule`、`update_digest_rule`、`run_digest_rule`、`push_digest_report`、`create_push_channel`、`update_push_channel`、`test_push_channel`
- 维护：`run_backup`

明确不暴露：删除分组/源/条目/规则/渠道、修改全局设置、AI 密钥、备份恢复、Token 管理。Token 管理只在实际登录的 Rill 设置页进行。

## Token 数据与接口

迁移 10（SchemaVersion 10）新增：

```sql
CREATE TABLE mcp_tokens (
    id           INTEGER PRIMARY KEY,
    name         TEXT NOT NULL,
    masked       TEXT NOT NULL,
    token_hash   TEXT NOT NULL UNIQUE,
    enabled      INTEGER NOT NULL DEFAULT 1,
    created_at   TEXT NOT NULL,
    last_used_at TEXT
);
```

- 生成：`rill_mcp_` + 32 字节随机值；完整值只在创建响应出现一次。
- 列表/创建/更新/吊销：`GET/POST /api/v1/mcp/tokens`、`PATCH/DELETE /api/v1/mcp/tokens/{id}`（session + CSRF）。
- `last_used_at` 最多每分钟写一次，避免 MCP 请求频繁写 SQLite。
- MCP 开关存 `settings.app_settings.mcp.enabled`，`GET/PATCH /api/v1/settings` 的 `mcp` 段返回 `enabled` 与固定端点 `/mcp`。

## 正文 Markdown

- 后端新增 `GET /api/v1/items/{id}/markdown`（session 保护），返回 `text/markdown; charset=utf-8`，`Content-Disposition: attachment`，`no-store`。
- 转换器 `server/internal/markdown` 支持标题、段落、列表、引用、代码块、粗体/斜体、链接、图片、表格和相对 URL；正文 HTML 为空时回退 `content_text`。
- 文档头包含标题、来源、作者、发布时间、原文标题/链接；正文始终使用原文，不包含译文。
- 前端详情工具栏与专注阅读顶部新增「复制 Markdown」「下载 Markdown」；下载生成 `blob:` URL 并点击带 `download` 的锚点，触发懒猫 file-chooser inject（保存到本地或懒猫）。

## 验收

- 设置出现「MCP」分类；开关关闭时 `/mcp` 404；打开后无 Token/错误 Token 401，正确 Token 可完成 `initialize`/`tools/list`/`tools/call`。
- Token 创建时完整显示一次，列表只显示打码值；停用或吊销后立即 401。
- `tools/list` 不包含任何删除工具。
- `GET /api/v1/items/{id}/markdown` 未登录 401；登录后返回原文 Markdown，含元信息与正文。
- 前端复制写入剪贴板；下载在懒猫环境弹出「保存至本地 / 保存至懒猫」。
- `go test ./...`、`npm run typecheck`、`npm test`、`npm run build` 通过。

## 未验证

- 真实外部 Agent（Claude Desktop / Codex 等）连接与 SSE 长连接行为。
- 真机懒猫 file-chooser 的下载保存弹层。
- MCP 工具在超大文章、长报告和 AI 生成场景下的超时/截断表现。
