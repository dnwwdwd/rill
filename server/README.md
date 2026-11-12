# Rill Go 后端 MVP

状态：本地可编译、可运行的 MVP，Go 1.23+。所有抓取测试都使用 `httptest`；
**没有对外部源做过实测**，X、公众号和真实站点能力仍以
[`docs/RSS-SOURCES.md`](../docs/RSS-SOURCES.md) 的状态为准。

登录只有懒猫 OIDC 一条路径（DEC-017）：应用内完成授权码 + PKCE 回调，密码
登录与密码修改端点已彻底移除。本地开发用内置假提供方 `cmd/devoidc`。

## 运行

本地开发（假 OIDC 提供方，仅监听环回，不做真实鉴权）：

```bash
cd server
go run ./cmd/devoidc                # 终端 1：127.0.0.1:8091
RILL_OIDC_CLIENT_ID=dev \
RILL_OIDC_CLIENT_SECRET=dev \
RILL_OIDC_AUTH_URI=http://127.0.0.1:8091/authorize \
RILL_OIDC_TOKEN_URI=http://127.0.0.1:8091/token \
RILL_OIDC_USERINFO_URI=http://127.0.0.1:8091/userinfo \
RILL_DATA_DIR=./data \
RILL_CACHE_DIR=./cache \
go run ./cmd/rill                   # 终端 2：127.0.0.1:8080
```

登录页点「使用懒猫账号登录」即以 `dev-user` 完成回调建会话。

懒猫 LPK 生产模式由 `lzc-manifest.yml` 从 `LAZYCAT_AUTH_OIDC_*` 注入同一组
`RILL_OIDC_*` 变量；回调路径固定为 `/auth/oidc/callback`。缺少任一 OIDC
变量时服务拒绝启动，不会静默变成无鉴权应用。

默认监听 `127.0.0.1:8080`，与 `web/vite.config.ts` 的 `/api` 代理一致。
首次启动会创建：

- `RILL_DATA_DIR/rill.db`：SQLite（WAL、foreign_keys、busy_timeout，单写连接）
- `RILL_DATA_DIR/secret.key`：AES-256-GCM 主密钥，权限 `0600`
- `RILL_DATA_DIR/backups/`：`VACUUM INTO` 备份
- `RILL_CACHE_DIR/media/`：图片代理缓存
- `RILL_CACHE_DIR/icons/`：源图标缓存（7 天，失败标记 1 小时）

`go build ./cmd/rill` 产出单个二进制。若 `web/dist/index.html` 存在，会自动
托管前端；也可用 `RILL_WEB_DIR` 指定目录。未知非 API 路径回落 `index.html`，
`/api/*`、`/feed/*`、`/opml/*`、`/media/*`、`/healthz`、`/auth/oidc/*`
不参与 SPA 回落。`rill -health` 请求本地 `/healthz` 并返回 0/1，供容器
healthcheck 使用，不依赖 curl/wget。

## 环境变量

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `RILL_OIDC_CLIENT_ID` | 无（必填） | 懒猫 OIDC client id；本地开发填 `dev` 配合 devoidc |
| `RILL_OIDC_CLIENT_SECRET` | 无（必填） | 懒猫 OIDC client secret；不写日志 |
| `RILL_OIDC_AUTH_URI` | 无（必填） | 授权端点 |
| `RILL_OIDC_TOKEN_URI` | 无（必填） | token 端点 |
| `RILL_OIDC_USERINFO_URI` | 无（必填） | userinfo 端点；以 `sub` 作为登录标识 |
| `RILL_OIDC_ISSUER` | 空 | issuer 信息，记录用；当前不做 discovery |
| `RILL_OIDC_SCOPES` | `openid profile email` | 授权 scope |
| `RILL_OIDC_REDIRECT_URL` | 从 Host/`X-Forwarded-Proto` 推导 | 覆盖回调地址，仅用于特殊部署 |
| `RILL_DATA_DIR` | `/lzcapp/var/data` | 数据库、密钥、备份 |
| `RILL_CACHE_DIR` | `/lzcapp/cache/media` | 图片缓存 |
| `RILL_LISTEN` | `127.0.0.1:8080` | HTTP 监听地址；生产容器通常显式设为 `:8080` |
| `RILL_COOKIE_SECURE` | 非回环监听时为 `true` | 显式设 `false` 仅用于本地/测试 |
| `RILL_WEB_DIR` | 自动查找 `web/dist` | 静态前端目录 |
| `RILL_USER_AGENT` | `Rill/0.1` | 抓取 UA |
| `RILL_BESTBLOGS_CATALOG_URL` | BestBlogs 公开 OPML（jsDelivr 镜像） | 公众号名称优先解析目录；可指向 GitHub raw/自建镜像；拉取失败时回退公众号后台搜索 |
| `RILL_MEDIA_MAX_BYTES` | `2147483648` | 图片缓存配额；超出后按最近访问时间淘汰 |
| `RILL_LOG_LEVEL` | `info` | `debug` / `info` / `warn` / `error` |

> 2026-11-01：SSRF/私网拦截机制已按用户决策整体移除（DEC-015），`RILL_ALLOW_PRIVATE_NETWORK`
> 环境变量不再存在；内网 / 回环地址按普通目标抓取。

## 已实现端点

管理 API 统一前缀 `/api/v1`，除公开的 `GET /auth/config` 和密码模式的
`POST /auth/login` 外都要求 session cookie；所有写操作要求 `X-CSRF-Token`。
OIDC 模式通过 `GET /auth/oidc/login` 与 `/auth/oidc/callback` 建立同一套
session。

```text
GET    /api/v1/auth/config
POST   /api/v1/auth/login
POST   /api/v1/auth/logout
GET    /api/v1/auth/csrf
POST   /api/v1/auth/password
GET    /auth/oidc/login
GET    /auth/oidc/callback

GET    /api/v1/folders
POST   /api/v1/folders
PATCH  /api/v1/folders/{id}
DELETE /api/v1/folders/{id}

GET    /api/v1/sources?folder_id=
POST   /api/v1/sources
POST   /api/v1/sources/batch         # 批量创建（≤20），后台首次抓取
POST   /api/v1/sources/discover
POST   /api/v1/sources/resolve       # BestBlogs 公众号目录 / Apple Podcasts / 小宇宙 / 微博 / Medium / Mastodon
POST   /api/v1/sources/search        # 创作者名称搜索：Apple / Bluesky / 微博精确昵称 / BestBlogs 四目录
GET    /api/v1/sources/{id}
PATCH  /api/v1/sources/{id}
DELETE /api/v1/sources/{id}
POST   /api/v1/sources/{id}/fetch
POST   /api/v1/sources/{id}/fallback
POST   /api/v1/sources/{id}/token/reset
GET    /api/v1/sources/{id}/logs?limit=50
GET    /api/v1/sources/{id}/icon      # 受登录保护的源图标代理

GET    /api/v1/items
GET    /api/v1/items/stats
GET    /api/v1/items/{id}
GET    /api/v1/items/{id}/markdown  # 原文 Markdown 导出（attachment）
PUT    /api/v1/items/{id}/podcast-progress  # 播客播放位置
PATCH  /api/v1/items/{id}/state
PATCH  /api/v1/items/state
POST   /api/v1/items/mark-all-read
POST   /api/v1/items/{id}/summary
POST   /api/v1/items/{id}/translation
GET    /api/v1/search?q=&cursor=

GET    /api/v1/settings
PATCH  /api/v1/settings
POST   /api/v1/settings/ai/test
POST   /api/v1/settings/feed-token/reset
POST   /api/v1/settings/feed-token/reveal # CSRF + no-store，按需返回完整 token
POST   /api/v1/wechat/weread/scan
GET    /api/v1/wechat/weread/scan/status
POST   /api/v1/wechat/resolve
POST   /api/v1/wechat/sources
POST   /api/v1/backup
POST   /api/v1/backup/restore
GET    /api/v1/mcp/tokens            # MCP 命名令牌（只返回打码值）
POST   /api/v1/mcp/tokens            # 创建令牌；完整 token 仅本次响应
PATCH  /api/v1/mcp/tokens/{id}       # 启停/改名
DELETE /api/v1/mcp/tokens/{id}       # 吊销
GET    /api/v1/opml/export
POST   /api/v1/opml/import
GET    /api/v1/healthz
```

- `POST /api/v1/sources` 创建后立即同步抓取一次（20 秒上限）；失败只写回源状态，不回滚创建。
- 测试可用 `Options.DisableInitialFetch` 关闭首次抓取。

公开输出不走 session：

```text
GET /healthz
GET /feed/{id}/{token}.xml
GET /feed/{id}/{token}.atom
GET /feed/{id}/{token}.json
GET /feed/all/{token}.xml
GET /opml/{token}.opml
GET /media/{token}/{hash}
POST /mcp                 # MCP Streamable HTTP（SSE），Bearer Token，不走 session
```

`/healthz` 只返回 `{"ok":true}`；`/api/v1/healthz` 才返回任务状态、到期源数量
和最近成功抓取时间。Feed 输出带 `ETag`/`Last-Modified`，支持 304；媒体只代理
`http/https` 图片，拒绝 SVG，单文件上限 20 MB，支持 Range。

## 数据能力

- SchemaVersion 10。迁移 2 为 `sources` 增加 `retention_days` / `retention_max_items`；
  迁移 3 建立 `items_fts`（FTS5 trigram）与 INSERT/UPDATE/DELETE 触发器；迁移 10
  新增 `mcp_tokens`（MCP 命名令牌，仅存 SHA-256 哈希与打码值）。
- 保留策略启动后执行一次、此后每 6 小时最多一次；只删除已读且未收藏的条目。
- 搜索 ≥3 个字符走 FTS5，短查询或 FTS 查询失败回退 LIKE；源名称仍用 LIKE。
- 源图标按 Feed 图 → 站点 `<link rel="icon">` → `/favicon.ico` 抓取，经现有 SSRF
  客户端；缓存文件不落库，API 只返回 `/api/v1/sources/{id}/icon`。
- 全站完整 token 不进入普通 settings 响应；reveal 端点要求登录和 CSRF，响应
  `Cache-Control: no-store`，不写日志。
- `GET /api/v1/folders` 返回 `parent_id`；前端按多级树缩进，删除文件夹时后端
  把源置为未分组、子文件夹上移到被删文件夹的父级。
- 媒体缓存按 `RILL_MEDIA_MAX_BYTES` 淘汰最旧文件并把行标记为 pending，下次请求
  可重新下载；图标缓存 8 MB / 500 文件，按 mtime 淘汰。
- 上传备份先用 `database.PrepareForRestore` 校验完整性；若 schema 较旧，先在
  上传副本上执行迁移，再做当前库安全备份并替换。

## 抓取与 AI

- 标准 RSS/Atom/JSON Feed：`gofeed`，条件请求 `ETag` / `If-Modified-Since`，
  5 MB / 15 s / 3 跳限制；相对链接补全、`data-src` 还原、bluemonday 清洗、
  图片重写为 `/media/{source_token}/{sha256}`。
- X：没有专用适配器；`kind="x"` 只负责把 handle 拼成
  `https://fxtwitter.com/{handle}/feed.xml`，抓取完全走标准 RSS。
- 公众号：`wechat://<fakeid>` 源由微信读书扫码 Cookie 采集（fakeid 推导
  `MP_WXS_*` bookId、幂等加书架、`/web/mp/articles` 列表、`/web/mp/content`
  正文，`-2041` 4h 冷却）；文章链接经公开页解析 `biz`；微信后台与 sidecar
  通道已于 2026-11-03（DEC-024）移除。
- 调度器每 30 秒查一次 `next_fetch_at`，启动时立即扫一次；成功按刷新间隔加
  ±10% 抖动，失败按 1m/2m/5m/15m/1h 退避，连续失败 5 次后跳过自动调度。
- AI：OpenAI 兼容 `POST {base_url}/chat/completions`；API Key AES-GCM 加密；
  默认关闭且必须确认 `consent`；每日上限、token 计数；摘要按 item 缓存，
  翻译按 item + 目标语言缓存，正文哈希变化后旧译文标记为需重译。

## 测试

```bash
cd server
go test ./...
go vet ./...
go test -race ./...
```

本地测试覆盖：SQLite PRAGMA/迁移/备份校验、登录与 CSRF、登录限速、
folders/sources CRUD、外部私网 SSRF 拒绝、RSS 条件请求/去重/清洗、
cursor 分页、批量与全部已读、Feed 三种输出与 token 重置、媒体代理缓存、
设置脱敏与 API Key 加密、嵌套 OPML、备份恢复、静态前端回落、AI 摘要/翻译缓存。
所有外部 URL 均由 `httptest` 提供，不依赖真实站点。

## 契约差异与已知取舍

1. PDD 已统一为 `kind = wechat`（公众号，微信读书唯一通道），与 API、SQLite 和前端一致。
2. 为支持 OPML 嵌套分组，`folders` 增加了 `parent_id`（PDD 表未列出）。文件夹
   API 仍只返回前端的 `{id,name,sort,source_count}`。
3. 为支撑前端字段，增加 `sources.initial_fetched`、`note`、`icon_url` 和
   `items.original_title`；另建 `sessions` 表。
4. `GET /sources/{id}` 的 `feed_paths` 返回可直接订阅的完整 token 路径；
   `feed_token_masked` 只用于展示。前端 token reset 后会强制重载源诊断，
   直接使用新的完整 `feed_paths`。
5. 全局 Feed/OPML token 目前只以 `settings.feed.token_masked` 暴露掩码，没有
   读取完整全局 token 的管理端点；`/feed/all/...` 与 `/opml/...` 的完整 URL
   需要从数据库设置中取得，UI 尚未接上。
6. 前端“保存密钥”已通过 `PATCH /settings {"ai":{"api_key":"..."}}` 发送只写
   字段；响应只返回 `api_key_set`，不回传密钥。
7. `POST /settings/feed-token/reset` 是为了对齐当前前端新增的
   `resetGlobalFeedToken()` 而补充的端点。
8. 备份只包含 SQLite 数据库，不包含 `secret.key`。用另一份 `secret.key` 恢复
   后，已保存的 AI API Key 会无法解密；数据库备份与密钥需分别保管。
9. 恢复在单个事务中从上传库复制表，不重启进程；`sessions` 表保留，当前会话
   不会因恢复被踢出。恢复前会自动 `VACUUM INTO` 一份当前库。
10. 摘要/翻译触发为同步返回，未使用 PDD 允许的 `202` 后台任务形态；失败会
    明确返回错误。
11. 详情里的 `original_title` 只在“显示标题已本地化、另需原始标题”时才有值。
    前端已改为按内容与 AI 开关显示翻译入口，不再依赖 `original_title`。
12. 全新数据库没有文件夹。前端添加源时在无匹配文件夹场景显式传 `null`，
    源进入「未分组」，侧边栏与源管理页均可看到。
13. 公众号只剩微信读书扫码通道（REQ-034/DEC-024）；`/wechat/scan`、
    `/wechat/search`、`/wechat/accounts`、`/wechat/logout` 端点已删除，
    返回 404。

## 未实现 / 待实测

- **外部源实测**：本交付只证明本地 httptest 通过，没有验证真实 RSS、X、
  fxtwitter 或微信读书真实扫码。
- **微信读书扫码联调**：扫码协议（`getLoginUid` → `web/confirm?uid=` →
  `getLoginInfo` → renewal）已按参考实现核对并有 httptest 覆盖；真实微信
  扫码、外部公众号列表/正文、`-2041` 验证码与 Cookie 过期仍待真机验收。
- X 转推/回复过滤字段、真实 fxtwitter 更新静默失败、微服出口连通性均未实测。
- 全文提取、关键词过滤、通知、标签、阅读统计未实现；源图标已实现 8 MB / 500 文件的 LRU 配额。
- 条目保留策略（F-28）已实现；删除源仍会级联删除条目和抓取日志。
- 搜索已启用 FTS5 trigram；两字中文和 FTS 失败走 LIKE 回退，大数据量性能待测。
- 自动切换备用源未实现；`POST /sources/{id}/fallback` 只做一次手动备用源抓取。
- per-host 串行/限速、429 `Retry-After`、出站代理（`RILL_OUTBOUND_PROXY`）未实现。
- 媒体只做按文件大小限制与简单 LRU 配额，不代理视频/音频；源图标已实现 8 MB / 500 文件的 LRU 配额。
- 静态前端未开启 gzip/brotli；多用户、PWA、单篇分享、Fever/GReader API
  不在范围内。
- OIDC 已在假提供方上做本地回调测试；真实懒猫 issuer、userinfo 字段、回调
  域名和 LPK 真机登录未验收。id_token 只接收不校验，身份以 userinfo 的 `sub`
  为准。
- 密码模式仅用于本地开发/测试；LPK 固定 OIDC，不提供密码入口。
- `RILL_ADMIN_PASSWORD` 变更不会覆盖数据库内密码；没有首次登录强制改密引导。
- OPML 导出不包含 per-source feed token；默认按 feed URL 导出。
