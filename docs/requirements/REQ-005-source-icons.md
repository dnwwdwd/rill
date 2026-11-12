# REQ-005 源图标抓取与缓存

- 状态：已完成（本地验证）
- 优先级：P0
- 关联功能：F-45
- 事实来源：[PRD.md](../../PRD.md)、[PDD.md](../../PDD.md) §11、[DECISIONS](../DECISIONS.md) DEC-007

## 规则

- 候选顺序：Feed 自带 `<image>` → 站点 `<link rel="icon">`（含 shortcut / apple-touch-icon）→ 站点 origin 的 `/favicon.ico`。
- 所有请求经过现有 `fetch.Client`：http/https、禁止私网/回环/链路本地、最多 3 跳、512KB 上限。
- 图标缓存到 `RILL_CACHE_DIR/icons/<source_id>`，不新增数据库列、不入库、不内联 base64。
- 缓存 7 天；失败写 `.failed` 标记，1 小时内不重复抓取；刷新失败可继续提供旧图。
- 图标目录独立配额：8 MB / 500 个文件，超限按 mtime 最旧淘汰。
- 不代理 SVG（与媒体代理一致），无法取得时前端回退首字母占位。
- API 返回的 `source.icon_url` 是受登录保护的同源路径 `/api/v1/sources/{id}/icon`，不是远端 URL。

## 实现落点

- `server/internal/server/icon.go`：图标端点、候选发现、抓取、缓存与 404 回退
- `server/internal/fetch/feed.go`：解析 Feed 图片并暴露 `FeedResult.IconURL`
- `server/internal/server/source_store.go`：外部图标源地址与 API 代理路径分离
- `web/src/components/ui/SourceIcon.tsx`：沿用 `icon_url`，加载失败回退首字母

## 证据

- `server/internal/server/fetch_test.go`：`TestSourceIconProxyAndCache`，本地 httptest feed 图标，首次与缓存请求均返回 `image/png`。
- `server/internal/server/icon_quota_test.go`：超过 8 MB 的最旧图标被删除，新图标保留。
- `go test ./...`、`go vet ./...` 通过（2026-11-01）。
- 前端类型检查通过；mock 源保持 `icon_url=null`，不影响首字母占位。
- 真实 Go 服务 + 浏览器（2026-11-01）：本地 feed `<image>` → `/api/v1/sources/1/icon`，`<img>` `naturalWidth=1440`、`complete=true`；缓存后再请求仍为 `image/png`。

## 未验证

- 真实外部站点、CDN 跳转、慢响应和 SVG-only 站点。
- 真机缓存目录权限与 LPK 运行环境。
- 图标刷新周期内的视觉变化。
