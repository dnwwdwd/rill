# PROG-REQ-038：MCP 服务与正文 Markdown 导出

- 状态：实现完成，本地验证通过；真实外部 Agent 与真机 file chooser 待验收
- 日期：2026-11-11
- 关联：[REQ-038](../requirements/REQ-038-mcp-and-markdown-export.md)、DEC-029

## 已完成

- 迁移 10（SchemaVersion 10）新增 `mcp_tokens`；Token 只存 SHA-256 哈希与打码值。
- 设置新增 `mcp.enabled` / `mcp.endpoint`；管理接口 `GET/POST /api/v1/mcp/tokens`、`PATCH/DELETE /api/v1/mcp/tokens/{id}`。
- `/mcp` Streamable HTTP（SSE）：Bearer 中间件、总开关关闭 404、Token 缺失/无效/停用 401。
- MCP 工具：读取分组/源/文章/Markdown/统计/规则/报告/渠道；写入分组、源、文章状态、播客进度、AI 摘要/翻译、过滤规则、定时报告、推送渠道、备份；不注册删除、设置修改、密钥、备份恢复。
- `GET /api/v1/items/{id}/markdown`：原文 Markdown，含元信息；HTML 转换器支持标题/段落/列表/引用/代码/链接/图片/表格。
- 前端 MCP 设置分类：开关、地址、命名 Token 创建/启停/吊销、能力边界；完整 Token 只显示一次。
- 阅读详情与专注阅读工具栏：复制/下载 Markdown；下载 blob 锚点交给懒猫 file-chooser inject。
- `lzc-manifest.yml` `public_path` 增加 `/mcp`、`/mcp/`。
- 文档：PRD F-50/F-51、PDD §4.2/§5.2/§6.5/§12.3/§12.4/§13.3、DESIGN、CHANGELOG、PROGRESS。

## 验证证据

- `go vet ./...`、`go test ./... -count=1` 全绿。
- 新增 `TestMCPBearerAuthToolsAndTokenLifecycle`：关闭 404、无/错误 Token 401、有效 Token `tools/list`、`create_folder` 写入、`last_used_at`、吊销后 401、工具列表无删除。
- 新增 `TestMCPOfficialClientHandshake`：用官方 Go MCP Client 完成 initialize、`tools/list` 与 `tools/call`。
- 新增 `TestItemMarkdownEndpointExportsOriginalBody`：未登录 401、元信息/正文/相对链接转换。
- 新增 `internal/markdown` 转换单测。
- `npm run typecheck`、47 项 Vitest（新增 MCP 设置与文件名测试）、`npm run build` 全绿。

## 未验证

- Claude Desktop / Codex 等真实 MCP 客户端的 initialize、SSE 长连接、断线重连与超时。
- 真机懒猫 file-chooser 的「保存至本地 / 保存至懒猫」弹层。
- 超长文章、AI 生成、报告推送等长耗时工具调用的 MCP 客户端超时行为。
- LPK 已随版本 `0.1.4` 重新打包（2026-11-11，sha256 `31d960b8…`）；未安装到微服。
