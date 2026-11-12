# REQ-004 全站 Feed / OPML 完整 token 读取

- 状态：已完成（本地验证）
- 优先级：P0
- 关联功能：F-10、F-14
- 事实来源：[PRD.md](../../PRD.md)、[PDD.md](../../PDD.md) §5.2/§12、[DECISIONS](../DECISIONS.md) DEC-010

## 决策

完整全站 token 不进入普通 `GET /api/v1/settings` 响应。管理端通过独立的
`POST /api/v1/settings/feed-token/reveal` 按需读取：

- 必须已登录，且写请求必须带 `X-CSRF-Token`。
- 响应设置 `Cache-Control: no-store` 和 `Pragma: no-cache`，不写日志。
- UI 默认只显示 masked token；点击「显示完整令牌」才读取，关闭设置或点隐藏后清除页面状态。
- 不新增数据库字段，不改变公开 Feed/OPML/media 的 token 鉴权方式。

## 实现落点

- `server/internal/server/handlers_sources.go`：`handleRevealGlobalToken`
- `server/internal/server/routes.go`：`POST /api/v1/settings/feed-token/reveal`
- `web/src/components/settings/SecuritySettings.tsx`：显示 / 隐藏 / 复制完整令牌与公开路径

## 证据

- `server/internal/server/token_test.go`：登录后 reveal 返回完整 token，`Cache-Control: no-store`，且不等于 masked。
- Playwright（mock）：安全设置默认显示 `rill_9f3c…a71e`；点击显示后显示完整 token 与 `/feed/all/...`、`/opml/...`、`/media/...` 路径；点击隐藏恢复 masked；0 JS 错误。
- 真实 Go 服务 + 浏览器（2026-11-01）：masked `rill_kbxkT…J9Q8`，reveal 后长度 48 且与 masked 不同；0 JS 错误。

## 未验证

- 真实 Go 后端 + 浏览器的 reveal 已通过；剪贴板复制与外部阅读器读取端到端未验证。
- LPK 官方 file chooser 注入与真机验收（LPK 已按 REQ-019 开始构建）。
