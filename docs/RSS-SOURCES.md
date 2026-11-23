# RSS 源实现登记表

> 这是本项目的核心文档。**每新增一个订阅源，必须先在本文档登记实现方式、鉴权、成本、限流、降级方案和实测记录。**
>
> 版本：0.8 · 最后更新：2026-11-03
>
> **2026-11-03 公众号能力边界修正（BUG-001）：** 真实会话确认 `searchbiz` 可搜外部账号，但
> `appmsgpublish` 对非当前后台管理账号固定返回 `200013`；内置链路只抓 `switchacct`
> 返回的自有公众号，外部未收录账号改走 BestBlogs/外部 RSS（§4.4/§4.5）。
>
> **2026-11-03 BestBlogs 多平台目录：** 公众号/小宇宙/X/YouTube 四类公开 OPML
> 已接入解析（§2.14）；公众号 BestBlogs 优先，其余平台保留官方 Feed / FxTwitter
> 主路径，目录只作候选/降级。四类目录 OPML 与公众号、X、小宇宙 Feed 已实测；
> YouTube Feed 从开发机直连超时，仍标 `部分验证`。
>
> **2026-11-02 新源调研：** Apple Podcasts（iTunes Lookup → RSS）、小宇宙
> （`feed.xyzfm.space` 优先，页面 SSR 最近 15 条降级）、微博（已于 2026-11-08 移除，见 §2.12；历史：匿名访客态最近最多
> 100 条）已完成公开端点与 Rill 端到端实测；Medium / Mastodon 完成 Feed 实测，
> 仍标 `部分验证`。Folo 对标结论见 §5。
>
> **2026-11-01 后端实现状态：** 标准 Feed、X（FxTwitter）、Bluesky（官方 RSS）、
> `rsshub://` 链接映射与 RSSHub 输出已由后端管线端到端实测（见各节实测记录）；
> 公众号扫码代理（we-mp-rss `/api/v1/wx/auth/qr/*`）已实现并通过 sidecar 模拟
> 服务 httptest 验证，真实 sidecar 联调待部署后验收。SSRF/私网拦截已按用户决策
> 移除（DEC-015）。
>
> **2026-11-02 公众号架构变更：** 默认改为 Rill 内置 HTTP 核心链路（DEC-018/REQ-026），
> 不再强制外部 sidecar；外部 we-mp-rss 保留为兼容模式。内置链路仅完成实现与包级单测，
> **真实扫码/列表/正文尚未真机实测**，状态不算「已验证」。
>
> **2026-11-03 公众号通道收敛（DEC-024/REQ-034）：** 微信后台内置链路与 we-mp-rss
> sidecar 一并移除；公众号只保留微信读书扫码通道（§4.4/§4.6）。添加路径为 BestBlogs
> 目录 + 文章链接（公开页提取 `biz`）。真实微信读书扫码与外部号抓取仍待验收。
>
> **2026-11-03 创作者名称搜索（REQ-035）：** 新增 `POST /sources/search`，并发查询
> Apple Podcasts、Bluesky 和 BestBlogs 目录（微博平台 2026-11-08 移除）；`POST /sources/batch`
> 批量创建。X/YouTube/小宇宙/公众号只覆盖 BestBlogs 目录，不是平台全网搜索（§2.15）。

---

## 0. 使用规则

- 状态只有四种：`已验证`、`部分验证`、`受阻`、`计划`。
- 没有实测证据的条目一律标 `计划` 或 `部分验证`，不得写 `已验证`。
- 实测记录必须包含：日期、请求、状态码、关键响应特征。
- 任何"免费""无需登录"的结论，都要写清楚边界（哪些账号、哪些页面、是否有限流）。
- 不记录真实 Cookie、API Key、token。

状态图例：

| 状态 | 含义 |
|------|------|
| 已验证 | 2026-10-31 / 2026-11-02 实测通过，可作为实现依据 |
| 部分验证 | 只验证了部分环节，或只对部分账号有效 |
| 受阻 | 实测被反爬/验证码/付费墙阻断 |
| 计划 | 尚未实测 |

---

## 1. 源总览

| 源 | 实现方式 | 鉴权 | 成本 | 稳定性 | 状态 | 降级 |
|----|----------|------|------|--------|------|------|
| RSS 2.0 / Atom / JSON Feed | `gofeed` + 条件请求 | 无 | 免费 | 高（取决于源站） | 已验证 | 无 |
| Bluesky | 官方 profile RSS（bsky.app），FxEmbed 镜像降级 | 无 | 免费 | 高 | 已验证 | fxbsky.app 镜像 |
| Threads | 自建 RSSHub 或公共 RSSHub `/threads/{user}`（匿名） | 无 | 免费 | 低-中（依赖实例与上游反爬） | 已验证（2026-11-02 公共实例 + 本系统抓取管线） | 公共实例 / 上游反爬（见 §2.6） |
| X / Twitter | FxTwitter 公共 API | 无 cookie | 免费 | 中 | 已验证 | 自动降级 `fixupx.com`（FxEmbed 镜像）；头像降级链含 unavatar.io（§3.3.2） |
| FxEmbed 自建 | Wrangler `dev --local` / Cloudflare Workers | 无（可选凭据） | 免费 | 中 | 部分验证 | 公共 API |
| 微信公众号（强制登录） | 微信读书扫码通道（唯一内置通道，`wechat://<fakeid>`）；BestBlogs 目录优先 | 微信读书扫码 Cookie（加密存 settings） | 免费 | 中 | 部分验证（httptest 全链；真实扫码/外部号待验收） | 列表 `-2041` 时降级 cover 最新一篇，cover 也失败才 4h 冷却；BestBlogs 未收录号粘贴文章链接添加 |
| 无登录公众号路径（不采用） | 公开专辑 / 公开主页 / Wechat2RSS | 无 | 免费 | 低-中 | 已验证（列表） | 已排除，仅备查 |
| BestBlogs 托管公众号 Feed | 标准 RSS（现有 `gofeed` 管线）；目录解析已实现 | 无 | 免费（BestBlogs 服务） | 中（第三方托管） | 部分验证（2026-11-03 抽查 5 个 Feed 均 200 / 10 条；目录解析单测通过） | 换其他 Feed / 自建 Wechat2RSS |
| 搜狗微信搜索 | `weixin.sogou.com` 解析 | 无 | 免费 | 低 | 受阻 | 不做 |
| 微信公众号文章直链 | `mp.weixin.qq.com/s?...` | 无 | 免费 | 低 | 受阻 | 不做 |
| RSSHub | `rsshub://` 链接映射为 https 后按标准 RSS 抓取 | 视路由 | 免费 | 低（公共实例） | 已验证（2026-11-02 公共实例 + 本系统解析） | 自建 / 自定义实例 |
| Instagram | 自建 RSSHub `/instagram/2/{user}` 或 RSS-Bridge，需要 Instagram 登录态 | 账号 Cookie（`IG_COOKIE`） | 免费 | 低 | 受阻（2026-11-02 公共实例 404/503；需登录态） | 不提供内置路径（见 §2.9） |
| Apple Podcasts | iTunes Lookup API 取 `feedUrl`，再走标准 RSS | 无 | 免费 | 高 | 已验证（2026-11-02 Lookup + Rill 端到端 5 条） | 直接填 RSS（见 §2.10）；feedUrl 为小宇宙托管时随 §2.11 降级 |
| 小宇宙 | 优先官方分发 Feed `feed.xyzfm.space/{token}`；页面 SSR 最近 15 条降级 | 无 | 免费 | 中（**2026-11-21 分发 Feed 被上游 IP 白名单拒绝，当前不可用**） | 部分验证（2026-11-02 Feed 200；2026-11-21 起 302 denied by IP ACL） | 页面 SSR 自动降级（site_url 可恢复 pid）/ 手动 RSS（见 §2.11） |
| 微博 | **已移除（2026-11-08）** | — | — | — | 历史：2026-11-02 访客态验证；间歇风控后移除 | 自建 RSSHub + Cookie（见 §2.12） |
| Medium | 主页 URL 归一化为 `/feed/@user` | 无 | 免费 | 高 | 部分验证（2026-11-02 Feed 200） | 手动 RSS |
| Mastodon | 主页 URL 归一化为 `@{user}.rss` | 无 | 免费 | 高（实例相关） | 部分验证（2026-11-02 Feed 200） | 手动 RSS |
| 邮箱自动同步（IMAP） | IMAP 客户端轮询（每个账号一个 `imap://{username}` 源，`go-imap/v2` + 标准库 MIME） | 用户邮箱客户端授权码（按账号 AES-GCM 加密存 settings） | 免费 | 高（取决于邮箱厂商） | 已实现（本地验证，2026-11-21；真实邮箱待验收） | 支持多账号；自动同步 INBOX 与自定义文件夹，排除已发送/草稿/垃圾箱/垃圾邮件；可设单次同步数量；UID 增量、不改邮箱已读状态；远程图片默认拦截、单封放行按条目持久化、按发件人放行持久化；授权码失效置 auth 并提示重配（见 §2.17） |

---

## 2. 标准 RSS / Atom / JSON Feed

### 2.1 实现方案

- Go 库：`github.com/mmcdole/gofeed`
  - 支持 RSS 0.90/0.91/0.92/1.0/2.0、Atom 0.3/1.0、JSON Feed 1.0/1.1。
- 请求：
  - `GET`，带 `User-Agent`、`Accept: application/rss+xml, application/atom+xml, application/feed+json, application/json;q=0.9, */*;q=0.8`。
  - 条件请求：`If-None-Match`、`If-Modified-Since`。
  - 超时 30 秒，响应体上限 10 MB。
  - 重定向最多 3 跳，每跳做 scheme 校验（2026-11-01 起不再拦截私网，DEC-015）。
- 解析：
  - 非 UTF-8 源站用 `golang.org/x/net/html/charset` 转码。
  - 解析失败时保留原始 body 到日志（截断），便于排查。
- 归一化：
  - `guid` → `link` → `sha256(title + published + content)` 三级去重键。
  - 正文经 `bluemonday` 清洗。
  - 相对链接按 Feed 的 `link` / 请求 URL 补全。
- 媒体：
  - `enclosure`、`media:content`、`media:thumbnail`、正文 `<img>`。
  - 图片 URL 重写为 `/media/{token}/{hash}`，按需代理缓存。
- 输出：
  - 每个源可输出 RSS 2.0、Atom 1.0、JSON Feed 1.1。
  - OPML 2.0 导入/导出。
- 地址编辑（2026-11-23，REQ-044）：编辑弹窗可修改 `feed_url`；保存时按源类型校验、大小写不敏感去重，清空 ETag/Last-Modified/图标与失败状态后立即抓取。成功后源名称（未显式改名时）、`site_url` 与图标跟随新 Feed 更新，旧条目保留。`imap://` 邮箱源地址由邮箱设置管理，不在此编辑。

### 2.2 实测记录（2026-10-31）

| 测试 | 请求 | 结果 |
|------|------|------|
| Atom 解析 | `https://github.com/DIYgod/RSSHub/releases.atom` | 200，`application/atom+xml`，10 个 `<entry>` |
| 条件请求 | 同上，带 `If-None-Match` | **304 Not Modified** |
| JSON Feed | `https://www.jsonfeed.org/feed.json` | 200，`application/json`，ETag + Last-Modified |
| 字符集 | 未单独测试 GBK 源 | 计划 |

### 2.3 当前后端验证（2026-10-31）

- `go test -race ./...` 通过；标准 Feed 由 `httptest` 覆盖条件请求、解析、去重、清洗与媒体重写。
- 本地二进制 + 本地 RSS 源联调：首次抓取 200，再次刷新 304；1 条入库；正文脚本与事件属性被移除；图片经 `/media/{token}/{hash}` 返回 `200 image/png`。
- 这仍然不是外部源实测；真实 RSS 站点、X/FxTwitter 与公众号 sidecar 的端到端状态见文首说明。

### 2.4 RSSHub 路由订阅（2026-11-01 新增）

- 状态：已验证（2026-11-02：`rsshub://` URL 映射单测通过；公共实例 200，Rill 抓取管线可解析）
- 实现方式：不引入 RSSHub 专用适配器。`rsshub://` 分享链接在后端创建/discover 时
  归一化为 https URL 后按标准 RSS 抓取（`server/internal/server/source_store.go`
  `normalizeRSSHubURL`）：
  - `rsshub://baoyu/blog` → `https://rsshub.app/baoyu/blog`
  - 首段含点视为自定义实例：`rsshub://rsshub.example.com/route` → `https://rsshub.example.com/route`
  - 前端 discover 失败时按原地址直接创建源，抓取结果由源诊断状态呈现。
- 入口/端点：`POST /api/v1/sources`、`POST /api/v1/sources/discover`（都接受 `rsshub://`）。
- 鉴权：无；自定义实例按其自身要求。
- 成本：免费。
- 限流：视实例与路由；公共实例普遍有限流/反爬。
- 去重键：同标准 RSS（guid → link → content hash）。
- 媒体处理：同标准 RSS（enclosure + 正文图片走媒体代理）。
- 降级方案：自建 RSSHub（Docker）或填自定义实例完整 https 地址。
- 实测记录：
  - 2026-11-01 · `GET https://rsshub.app/baoyu/blog` · **403 text/html**，5,484 字节，
    响应为 Cloudflare 的「Just a moment...」人机校验页，不是 RSSHub 返回的 Feed。
  - 2026-11-01 · `GET https://rsshub.rssforever.com/60s/world` · **503**。
  - 2026-11-01 · 单元测试 `TestNormalizeRSSHubURL`（8 组映射用例）通过。
  - 2026-11-02 · `GET https://rsshub.rss.tips/threads/zuck` · **200 application/xml**，
    14,155 字节，10 条；`rsshub.ktachibana.party` 200 / 14,163 字节 / 10 条；
    `rsshub.rssforever.com` 200 / 7,368 字节 / 4 条。
  - 2026-11-02 · Rill `fetch.Client.FetchFeed` 抓取 `rsshub.rss.tips/threads/zuck`：
    200，标题 `Mark Zuckerberg (@zuck) on Threads`，解析 10 条，GUID/时间/正文正常。
- 已知限制：`rsshub.app` 被 Cloudflare 人机校验拦截（403），不是 RSSHub 路由本身的问题；
  应使用可用公共实例或自建。公共实例无 SLA、限流与反爬策略各异；Twitter 等路由需要
  额外凭据（见 §3.6）。
- 合规备注：RSSHub 只是上游公开页面的 RSS 包装；内容权利归原作者。

### 2.5 Bluesky 订阅（2026-11-01 新增）

- 状态：已验证（公共 profile RSS 经本系统抓取管线端到端实测）
- 实现方式：**不新增适配器**。Bluesky 官方为每个 profile 提供 RSS：
  `https://bsky.app/profile/{handle}/rss`，按标准 RSS 2.0 走既有 gofeed 管线。
  前端添加源弹窗识别 `bsky.app/profile/{handle}` 链接与 `@handle`（handle 必含点号），
  自动生成 feed URL 与 `@handle` 源名。
- 入口/端点：`GET https://bsky.app/profile/{handle}/rss`（官方）；降级
  `GET https://fxbsky.app/profile/{handle}/rss`（FxEmbed 镜像，实测内容一致）。
- 鉴权：无；公开 profile 即可，受保护账号不输出（未实测受保护行为）。
- 成本：免费。
- 限流：官方未公布；建议单账号刷新间隔 ≥15 分钟，与 X 相同口径。
- 数据字段：item 含 `link`（帖子永久链接）、`description`（纯文本正文）、
  `pubDate`（如 `30 Oct 2026 17:34 +0000`，gofeed 实测可解析）、
  `guid`（`at://did:plc:...`）。无 title 时由正文首行派生（既有逻辑）。
- 去重键：`guid`（at:// URI）→ link → content hash，同标准 RSS。
- 媒体处理：description 为纯文本，图片经帖子内嵌输出时走既有清洗 + 媒体代理
  （纯文本帖无图，实测样本即如此）。
- 降级方案：官方 bsky.app RSS 为主；fxbsky.app（FxEmbed）为备，可直接在
  EditSourceDialog 里把 feed_url 换成 fxbsky.app 形态。
- 实测记录：
  - 2026-11-01 · `GET https://bsky.app/profile/bsky.app/rss` · **200
    application/xml**，9 个 item，含 guid/pubDate/link。
  - 2026-11-01 · `GET https://fxbsky.app/profile/bsky.app/rss` · **200**，内容一致。
  - 2026-11-01 · 本系统 `fetch.Client.FetchFeed` 端到端解析：9 条入库形状正确，
    `published_estimated=false`，排序正确（`cmd/bskytest` 诊断输出）。
  - 2026-11-01 · 前端识别单元测试 5 项通过（`AddSourceDialog.test.tsx`）。
  - 2026-11-01 · **全量回补（REQ-022）实测**：本系统服务内对 `bsky.app` 源切到
    `full`，后台翻 AppView 分页 25 秒入库 **520 条**（原窗口 9 条）；状态 ok、备注
    清空。去重键 `at://` URI 与窗口 RSS 一致。
- 已知限制：官方 RSS 只输出最近帖子（实测 9 条），不做历史回补；纯文本为主；
  引用帖显示 `[contains quote post or other embedded content]` 占位。
- 合规备注：内容归发帖者；仅订阅公开 profile。

### 2.6 Threads / 其他 FxEmbed 平台（2026-11-01 调研，2026-11-02 复测）

- 状态：已验证（2026-11-02：公共 RSSHub 实例 200，Rill `FetchFeed` 解析 10 条；仍依赖实例与上游可用性）

**问题：有无稳定、不要求登录的 Threads 订阅方式？**

- **官方渠道：无。** Threads 不提供 RSS；官方 Threads API 只面向开发者自己的账号
  做发布/数据管理，需要 Meta OAuth 应用审核，不能订阅任意他人时间线。
- **可行路径（已实测）：RSSHub `/threads/{user}` 路由，全程匿名、无需任何登录凭据**
  （源码 `lib/routes/threads/index.ts`：直接匿名 GET `threads.com/@{user}`，解析页面
  SSR 内嵌的 `thread_items` JSON；无 config 要求）。
- Rill 侧接入方式：**无需新增代码**。自建 RSSHub 或使用可用公共实例后，把
  `https://<rsshub-host>/threads/{user}` 当普通 RSS 地址订阅即可；也支持
  `rsshub://<host>/threads/{user}` 映射。标题/时间/链接/正文形状与标准 RSS 一致。
- 实测记录（2026-11-01）：
  - 自建 RSSHub（Docker `diygod/rsshub:latest`，零凭据配置）·
    `GET /threads/zuck` → **200，10 条**，标题带作者前缀、pubDate、帖子链接齐全；
    `GET /threads/mosseri` 连续 3 次 200（命中缓存后 <10ms）；`GET /threads/nasa` →
    200，4 条。媒体 img/video 标签在样本中未出现（样本帖多为纯文本），媒体输出
    能力仅确认路由源码支持，未实测。
  - 公共实例：`rsshub.rss.tips/threads/zuck` → **200，10 条**；
    `rsshub.ktachibana.party/threads/zuck` → **200**；`rsshub.app` → 403（Cloudflare
    「Just a moment...」人机校验，不是 Threads 路由失败）；`rsshub.rssforever.com`
    当时 000 不可达（2026-11-02 复测已恢复 200，4 条）。
  - 直连 `threads.com/@zuck`（本机多种 UA，含 TelegramBot/facebookexternalhit）→
    返回 HTML **不含 `thread_items`**：匿名 SSR 数据是否出现取决于出口 IP 信誉与
    Meta 反爬，这是该方案最大的不稳定来源。
  - RSSHub 仓库 `/threads` 路由有历史故障 issue（#15183、#16910），印证上游随时间
    可能失效。
  - 2026-11-02 · 公共实例复测：`rsshub.rss.tips` **200 application/xml**（14,155 字节，
    10 条）、`rsshub.ktachibana.party` **200**（14,163 字节，10 条）、
    `rsshub.rssforever.com` **200**（7,368 字节，4 条）；三条均带 `ETag` 与
    `Last-Modified`。Rill `fetch.Client.FetchFeed` 抓取 `rsshub.rss.tips` 成功解析
    10 条，标题为 `Mark Zuckerberg (@zuck) on Threads`，GUID/发布时间/正文正常。
- **结论：「不登录」可以做到，「稳定」做不到绝对保证。** 数据可得性随 Meta 反爬与
  出口 IP 波动；公共实例无 SLA 且 `rsshub.app` 仍会触发 Cloudflare 校验。推荐形态：
  **自建 RSSHub（与 §2.4 `rsshub://` 的降级同栈），或选择可用的公共实例，订阅其
  `/threads/{user}` 输出**；实例不可用时源会显示抓取失败并可退避重试。
- 合规备注：内容归发帖者；匿名抓取受 Meta 反爬约束，仅限私有部署、低频轮询。

### 2.7 已知限制（标准 Feed）

- 部分源站输出损坏的 XML，`gofeed` 可能解析失败；需要保留原文并给出可读错误。
- 部分源站不返回 `ETag`/`Last-Modified`，只能全量拉取。
- 部分源站正文只有摘要，需要 P1 的全文提取。
- 部分源站图片有防盗链，需要 `Referer` 或代理。

### 2.8 播客 / 音频（2026-11-02 新增）

- 状态：已验证（2026-11-02：本地 podcast RSS 单测 + 真实播客源解析）
- 实现方式：不新增独立源类型，播客仍按标准 RSS 进入既有管线。解析 item 的
  `enclosure`，若 `type` 以 `audio/` 开头或 URL 以 `.mp3` / `.m4a` / `.aac` /
  `.ogg` / `.oga` / `.opus` / `.wav` / `.flac` 结尾，提取第一个音频地址与 MIME。
  音频地址与图片列表一起写入 `items.media_json`（对象格式，兼容旧的字符串数组）。
- 入口/端点：标准 RSS/Atom/JSON Feed 地址；详情接口返回 `audio_url` / `audio_type`。
- 鉴权：无；依赖源站音频 CDN。
- 成本：免费。
- 限流：取决于源站与音频 CDN。
- 播放：正文标题下方显示播放器，支持播放 / 暂停、拖动进度、±15 秒、
  0.75× / 1× / 1.25× / 1.5× / 2× 倍速；播放位置按条目写入服务端
  `podcast_progress`（迁移 5），浏览器 `localStorage`
  （`rill.podcast.position.{item_id}`）作为即时缓存与离线兜底。
- 输出：RSS `<enclosure>`、Atom `<link rel="enclosure">`、JSON Feed
  `attachments` 保留音频地址与 MIME。
- 降级方案：音频无法播放时播放器显示「音频无法播放」；条目正文仍可阅读。
- 实测记录：
  - 2026-11-02 · 本地测试 Feed `enclosure type="audio/mpeg"` → 详情
    `audio_url`/`audio_type` 正确，RSS 与 JSON 输出含音频附件；进度写入后
    详情返回 `podcast_position_seconds` / `podcast_duration_seconds`，负数返回 400。
  - 2026-11-02 · `GET https://feeds.megaphone.fm/darknetdiaries` · **200
    application/rss+xml**，182 条；Rill `FetchFeed` 解析首条 `181: Ricky`，
    `audio/mpeg` 地址正确。
  - 2026-11-02 · `GET https://feeds.npr.org/510289/podcast.xml` · **200
    application/xml**，355 条，首条含 `audio/mpeg` enclosure。
- 已知限制：音频走源站原始地址，不经 Rill 媒体代理；CORS 或防盗链严格时可能无法
  播放。播放进度按条目保存到当前微服，跨浏览器/设备可续播；`localStorage` 仅作
  即时缓存。视频 enclosure 暂不处理。
- 合规备注：音频版权归节目方；Rill 只保存节目提供的公开音频地址。

### 2.9 Instagram（2026-11-02 调研，受阻，不纳入）

- 状态：受阻
- 官方渠道：**无任意账号 RSS。** Instagram Graph API 只面向自己管理的商业/创作者
  账号，需要 Meta 应用审核，不能订阅他人时间线。
- RSSHub `/instagram/2/{user}`：源码要求配置 Instagram 登录态（`IG_COOKIE`）。
  2026-11-02 公共实例实测：
  - `rsshub.rss.tips/instagram/2/zuck` → **503**，错误
    `InvalidParameterError: Such feed is not supported`
  - `rsshub.ktachibana.party/instagram/2/zuck` → **503**
  - `rsshub.rssforever.com/instagram/2/zuck` → **503**
  - `/instagram/user/zuck` → 404 或超时；`/picuki/profile/zuck`、`/picuki/user/zuck`
    在三个实例上超时或 404/503
- RSS-Bridge：Instagram bridge 同样要求一个已登录的 Instagram 账号与 Cookie。
- Rill 侧：**不内置 Instagram 登录态或 Cookie 管理。** 如果用户自建 RSSHub /
  RSS-Bridge 并拿到标准 RSS 地址，可以把它当普通 RSS 订阅；这不构成 Rill 的
  Instagram 适配器。
- 风险：违反 Instagram 服务条款、账号限流/封禁、Cookie/2FA 失效、出口 IP 信誉、
  第三方桥接不可用；公共实例不会提供登录态。
- 结论：**不进入源支持矩阵与 package 描述。** 未来只有在用户明确自备登录态并接受
  上述风险时，才考虑作为实验路径单独登记。

### 2.10 Apple Podcasts（2026-11-02 调研）

- 状态：已验证（2026-11-02）
- Apple Podcasts 是目录，不是播客托管；节目本身有 RSS。
- 入口/解析：
  - 链接：`https://podcasts.apple.com/{cc}/podcast/{slug}/id{showID}`
  - 纯 ID：`id{showID}` 或 5–12 位数字（可加 `apple:` 前缀消歧）
  - 查询：`GET https://itunes.apple.com/lookup?id={showID}&entity=podcast`
- 鉴权：无；成本免费。
- 实测：
  - `lookup?id=1296350485&entity=podcast` → **200**，
    `collectionName=Darknet Diaries`，`feedUrl=https://podcast.darknetdiaries.com`
  - 该 feed → **200 application/rss+xml**，可被标准 RSS 适配器解析
  - `lookup?id=1320749985` → `resultCount=0`，说明错误 ID 必须明确报错，不能猜
  - Rill 端到端（2026-11-02）：Apple 链接 → 解析 `feedUrl` → 创建 RSS → 抓取，
    返回 5 条，首条「与移民史学者聊海外温州人」
- 去重键：由目标 RSS 自身的 `guid` 决定，和普通 RSS 一致。
- 媒体：播客 `enclosure` 走现有音频播放器；不镜像音频。
- 降级：Lookup 失败时提示用户直接粘贴节目 RSS；不抓 Apple 页面。
- 已知限制：Apple 目录只覆盖已提交到 Apple 的节目；`feedUrl` 可能指向任意托管商。
- 合规：只调用 Apple 公开目录 API，不绕过登录或付费。

### 2.11 小宇宙（2026-11-02 调研）

- 状态：部分验证（2026-11-21 分发 Feed 被上游 IP 白名单拒绝，页面降级路径可用；见 BUG-003）
- **2026-11-21 上游变更（BUG-003）**：`feed.xyzfm.space` 对非白名单出口 IP 返回
  302（`X-Tengine-Error: denied by IP ACL = not in whitelist`）→ HTTP 200 的
  HTML 拒绝页，与 User-Agent 无关（ApplePodcasts/Overcast/浏览器 UA 全部被拒）。
  本机 4 种 UA 复现一致。官方分发 Feed 当前对自托管出口不可用；Apple 解析出的
  小宇宙托管 feedUrl（如「忽左忽右」`feed.xyzfm.space/cv4bkgpuglwp`）同样命中。
  Rill 已做运行时自动降级（见下）。
- 官方按 podcast id 的公开 RSS/API：**未发现**。
- 首选入口（上游恢复前不可依赖）：`https://feed.xyzfm.space/{token}`，标准 RSS 2.0，无鉴权。
  - 实测 2026-11-02 `feed.xyzfm.space/j7d9mlny9ea6` → **200 application/xml**，41 条；
    `enclosure` 指向 `audio/mp4`；`guid` 等于小宇宙 `eid`。
  - 实测 2026-11-21 同一 URL → **302 denied by IP ACL**，跳转 HTML 页。
  - 不是每个节目都有：需要从 Apple 目录反查，并校验 feed `<link>` 回指
    `/podcast/{pid}`；Apple Search API 约 20 次/分钟。
- **自动替换订阅方式（2026-11-21，BUG-003 第二轮修复）**：`fetchSource` 对
  `feed.xyzfm.space/` 源抓取失败时，恢复节目 pid（site_url 节目页，或从条目里的
  单集链接 `xiaoyuzhoufm.com/episode/{eid}` 抓单集页反查——覆盖 Apple 解析源），
  然后**永久改写源**为 `xiaoyuzhou://podcast/{pid}` 页面抓取源（备注写明），
  此后不再请求被拒的 Feed；pid 恢复失败才报错。
- 降级入口：节目页面 `https://www.xiaoyuzhoufm.com/podcast/{pid}`
  - `__NEXT_DATA__` → `props.pageProps.podcast`。
  - 实测 `episodeCount=652` 的节目页面只返回 **15** 条 `episodes`；`?page=2` 被忽略。
  - `_next/data/{buildId}/podcast/{pid}.json` 同样只有 15 条，`buildId` 会变。
  - 只能作为「最近 15 条」降级，不能承诺全量。
- 单集链接：`/episode/{eid}` 可从页面数据反查 `pid`，最终订阅整个播客。
- Rill 端到端（2026-11-02）：
  - Apple Search 反查「这集我看过」→ `feed.xyzfm.space/j7d9mlny9ea6`，
    `<link>` 回链校验通过；创建 RSS 后抓取 5 条。
  - 无 Feed 的 `Echo.js` 页面降级 → `xiaoyuzhou://podcast/...`，抓取 11 条，
    首条「Episode 9」。
- 去重键：`eid`（分发 Feed 的 `guid` 也是 `eid`）。
- 媒体：只保存 `enclosure.url`，不镜像音频；音频体积可达 80 MB+。
- 合规：页面 SSR 和 `feed.xyzfm.space` 都是未文档化接口，可能改版；轮询间隔建议
  ≥15 分钟；只处理公开节目。

### 2.12 微博（2026-11-02 调研；**2026-11-08 已移除**）

> **状态：已移除（2026-11-08 用户决策 DEC-026）。** 2026-11-08 复测确认匿名访客链路本身可用（见下方历史记录），但用户实例频繁遇到间歇性风控（「微博时间线读取失败：微博要求登录或访客态已失效」），且微博只支持精确昵称、无翻页、无历史，产品价值有限。按用户要求整体移除：
> - 抓取器 `weibo.go`、resolve 解析、创作者搜索微博平台全部删除。
> - 迁移 6：存量 `weibo://user/%` 源自动停用（`enabled=0`、note 标注「微博渠道已移除」），文章数据保留，可在界面手动删除。
> - 再次粘贴微博链接/UID/`weibo:` 前缀会得到明确错误「微博渠道已移除」。
> - 替代方案：自建 RSSHub（需 `WEIBO_COOKIES`）作为普通 RSS 添加。

以下为移除前的历史调研记录，仅备查：

- 历史状态：已验证（2026-11-02，匿名访客态）
- 官方 RSS：**无**。公共 RSSHub 实例 `/weibo/user/{uid}` 实测 **503**，路由要求
  `WEIBO_COOKIES`，不能作为稳定公共依赖。
- 可行入口：匿名访客 Cookie + `m.weibo.cn` 用户时间线。
  - 未带访客 Cookie 直接请求 API：**432**，空响应。
  - `POST https://visitor.passport.weibo.cn/visitor/genvisitor2`
    （`cb=gen_callback` + `fp=...`）→ **200**，JSONP 内含 `SUB`/`SUBP`。
  - 带访客 Cookie 请求
    `GET /api/container/getIndex?type=uid&value={uid}&containerid=107603{uid}&count=100`
    → **200 application/json**，实测 100 个卡片。
  - `page=2` → `ok:-100`，要求登录；**不能匿名翻页**。
  - 不同账号可能只返回 10 条；匿名上限不保证 100。
  - 昵称解析：带访客 Cookie 请求 `m.weibo.cn/n/{name}`，读取 302 `Location: /u/{uid}`。
- 短链：`t.cn/...` / `weibo.cn/...` 先跟随 3 次以内重定向，再按最终 URL 解析；失败时明确报错。本批未找到稳定短链样本，**未实测**。
  - 单条长文：`m.weibo.cn/statuses/show?id={mid}` 可取全文，但每篇多一次请求。
- 去重键：`mid`；展示链接用 `https://m.weibo.cn/status/{bid}`。
- 媒体：图片 `pics[].large.url` 走媒体代理；视频 URL 带过期签名，不长期缓存。
- 限流/降级：默认每源 30 分钟刷新；访客态只存内存、1 小时 TTL；432 或 `ok:-100` 时清空并重试（2026-11-08 起为 **3 次尝试、按 800ms/1.6s 退避**，每次换新访客 Cookie）；失败按源退避；不承诺历史回补。
- 间歇性风控（2026-11-08 复测）：用户实例出现「微博时间线读取失败：微博要求登录或访客态已失效」，同日在开发机与微服容器内分别复测完整链路（genvisitor2 → getIndex，含 `Rill/0.1` UA）均 `ok:1`、200、100 卡片，确认是**间歇性风控**而非链路失效；RSSHub（需 `WEIBO_COOKIES`）与官方 RSS 均无可用的替代方案，决定保留微博源并加强重试。
- 合规：只读取公开账号的最近内容；不保存用户微博 Cookie；不绕过登录读取受限内容。
- Rill 端到端（2026-11-02）：`weibo.com/u/2803301701` → 访客态 → 100 个卡片；
  源首次抓取按 `initial_limit` 入库，API 返回 5 条。
- 结论：作为**受限实验源**，UI 必须标注「最近最多 100 条」；不是完整时间线。（已于 2026-11-08 移除）

### 2.13 Medium / Mastodon（2026-11-02 调研）

- 状态：部分验证
- Medium：`https://medium.com/@user` → `https://medium.com/feed/@user`；
  实测 200 `text/xml`。
- Mastodon：`https://{instance}/@{user}` → `https://{instance}/@{user}.rss`；
  实测 `mastodon.social/@Mastodon.rss` → 200 `application/rss+xml`。
- 两者都只是 URL 归一化，不新增适配器；仍走标准 RSS。
- 已知限制：实例关闭 RSS、账号不存在或仅可见内容会失败；由源状态展示。

### 2.14 BestBlogs 多平台目录（2026-11-03，REQ-032）

BestBlogs 公开维护四类 OPML；Rill 用同一个 `POST /api/v1/sources/resolve` 做目录解析，**不新增源类型，所有候选仍 `kind=rss`**。平台优先级不改：公众号 BestBlogs 优先；小宇宙官方 Feed、X FxTwitter、YouTube 官方 RSS 仍优先，目录只作候选/降级。

| 平台 | 目录 | Feed host | 候选 `platform` | 实测 |
|---|---|---|---|---|
| 公众号 | `opml/bestblogs_wechat2rss_opml_all.opml`（375） | `wechat2rss.bestblogs.dev/feed/{40hex}.xml` | `bestblogs` | 5 个样本 200、各 10 条、含全文 |
| 小宇宙 | `BestBlogs_RSS_Podcasts.opml`（30） | `rsshub.bestblogs.dev/xiaoyuzhou/podcast/{24hex}` | `bestblogs-podcast` | 样本 200 |
| X | `BestBlogs_RSS_Twitters.opml`（160） | `api.xgo.ing/rss/user/{32hex}` | `bestblogs-x` | OpenAI 样本 200、50 条 |
| YouTube | `BestBlogs_RSS_Videos.opml`（40） | `youtube.com/feeds/videos.xml?channel_id=...` | `bestblogs-youtube` | 开发机直连超时，未验证 |

- 鉴权：无；只读公开 OPML，不使用 BestBlogs API Key。
- 入口：`bestblogs:{名称}` 跨目录搜索；公众号裸名称自动搜索；四类 Feed URL 直接识别。
- 缓存/限流：目录 6h 内存缓存；jsDelivr 失败时用 GitHub raw 兜底；单个目录失败只跳过该类并保留 stale，不影响其他平台；任一目录失败时 10 分钟后重试。
- 去重/媒体：按标准 RSS（`guid` → `link` → 内容哈希）；媒体走既有 `/media/` 代理。
- 已知限制：只覆盖目录收录项；公众号约 10 条、无历史；X 目录是 `api.xgo.ing` 第三方 RSS；YouTube 仍是官方 Feed，目录只帮助发现；第三方目录无 SLA。
- 实测（2026-11-03）：四类 jsDelivr OPML 与 GitHub raw 兜底均 200（97988/25480/4876/6163 字节）。
- 端到端（2026-11-03）：`TestLivePlatformResolvers/bestblogs_end_to_end` 经 `POST /sources/resolve` 解析 `bestblogs:数字生命卡兹克` 与 `bestblogs:OpenAI`，分别建源、抓取并各返回 5 条；公众号首条为 Agent 自动化长文，X 首条为 OpenAI 帖文。YouTube 目录检索成功，Feed 元数据 TLS 超时，未验证入库。
- 合规：仅个人阅读；不公开聚合、不商业分发。

### 2.15 创作者名称搜索（2026-11-03，REQ-035）

- 状态：实现完成；Apple Search 与 Bluesky `searchActors` 已用公开端点实测 200；微博平台已于 2026-11-08 移除（§2.12）；真实浏览器批量订阅待验收。
- 入口：`POST /api/v1/sources/search`，请求 `{query, platforms?}`；`platforms` 省略时查询全部。
- 并发：Apple Podcasts、Bluesky、BestBlogs 三目录并行；总超时 8s，单平台 5s；失败只影响该平台状态。
- 平台状态：`ok` / `empty` / `error` / `timeout`；前端必须展示，不能把失败当成「没有账号」。

| 平台 | 搜索方式 | 覆盖 | 鉴权/限制 | 状态 |
|---|---|---|---|---|
| Apple Podcasts | `itunes.apple.com/search?term=...&entity=podcast&limit=5` | 播客目录 | 无；公开 Search API | 已验证（2026-11-03 搜索 200，返回 2 条；Rill 单测覆盖） |
| Bluesky | `public.api.bsky.app/xrpc/app.bsky.actor.searchActors?q=...&limit=5` | 公开账号 | 无；公开 AppView | 已验证（2026-11-03 搜索 200；Rill 单测覆盖） |
| 微博 | 已移除（2026-11-08，§2.12） | — | — | 已移除 |
| BestBlogs 公众号 | 公开 OPML 进程内搜索 | 约 375 个收录账号 | 无；第三方目录 | 部分验证（§2.14） |
| BestBlogs 小宇宙 | 公开 OPML 进程内搜索 | 约 30 个收录账号 | 无；第三方目录 | 部分验证（§2.14） |
| BestBlogs X | 公开 OPML 进程内搜索 | 约 160 个收录账号 | 无；第三方目录 | 部分验证（§2.14） |
| BestBlogs YouTube | 公开 OPML 进程内搜索 | 约 40 个收录账号 | 无；第三方目录 | 部分验证（§2.14） |
| X 全网 | 无 | 不提供 | 无免费公开搜索 API | 不纳入 |
| YouTube 全网 | 无 | 不提供；目录仅 BestBlogs 收录 | 官方 Data API 需 Key，页面抓取脆弱 | 不纳入第一版 |
| 小宇宙全网 | 无公开搜索端点（`/search` 实测 404） | 仅 BestBlogs/Apple 反查 | 未文档化页面 | 不纳入第一版 |
| Medium / Mastodon / Threads / Instagram | 无名称搜索 | 仅链接/handle | 未文档化/需登录 | 不纳入 |

- 去重：按规范化 Feed URL；原生平台结果优先于 BestBlogs 目录。
- 候选头像（2026-11-08）：微博 `sinaimg.cn` 对外站 Referer 一律 403（实测），浏览器直连候选 `icon_url` 必裂。resolve/搜索响应新增 `icon_proxy_url`（`GET /api/v1/media/candidate/{hash}`，已认证），服务端先把候选头像登记进 `media_cache` 再由该端点经媒体缓存出图；未登记 hash 一律 404，不能抓任意 URL。`icon_url` 保持原始 URL，仍是创建源时落库的值。X/YouTube 全网搜索维持不纳入（无免费公开 API；微服从国内网络直连 youtube.com 实测 TLS 超时）。
- 缓存：10 分钟（存在失败时 1 分钟）；缓存不保存「已订阅」，每次按当前源重算。
- 批量创建：`POST /api/v1/sources/batch`，最多 20 个；重复 Feed URL 返回 `source_exists`/`duplicate_source`；创建后后台首次抓取。
- 合规：只读公开目录/API；不保存凭据；不绕过登录抓取受限内容。

---

## 3. X / Twitter

### 3.1 主方案：FxTwitter 公共 API

**结论：可用、免费、无需 cookie、无需 API key。**

入口：

```text
GET https://api.fxtwitter.com/2/profile/{handle}/statuses?count=20
GET https://api.fxtwitter.com/2/profile/{handle}/statuses?cursor={cursor}
```

也可以直接使用现成 Feed：

```text
https://fxtwitter.com/{handle}/feed.xml?count=50
https://fxtwitter.com/{handle}/feed.atom.xml?count=50
https://fxtwitter.com/{handle}/media.xml?count=50
```

字段与行为：

| 字段/参数 | 说明 |
|-----------|------|
| `count` | JSON API 每页 20；Feed 1–100，默认 90 |
| `cursor.bottom` | 下一页游标 |
| `reposted_by` | 非空表示转推，默认过滤 |
| `replying_to` | 非空表示回复，默认过滤 |
| `media.photos[]` | 图片数组，含 `url`、`width`、`height` |
| `media.videos[]` | 视频数组，MP4 直链 |
| `created_at` | `Sat Oct 31 05:13:48 +0000 2026` |
| `url` | 推文永久链接，作为 `guid` |

限流：官方文档写明 API v2 **1000 请求/分钟/IP**。单账号建议 15–30 分钟拉一次。

### 3.2 实测记录（2026-10-31，账号 `@example_user`）

| 测试 | 请求 | 结果 |
|------|------|------|
| 资料 | `/2/profile/example_user` | 200，`吃遍全国汉堡🍔`，267 条帖子 |
| 时间线 | `/2/profile/example_user/statuses?count=20` | 200，20 条 |
| 翻页 | 带 `cursor.bottom` | 200，再 20 条，游标继续可用 |
| 过滤 | 2 页共 40 条 | 按规则过滤转推/回复后剩 36 条 |
| 多图 | 首条 3 张图 | `<description>` 含 3 个 `<img>`，`<enclosure>` 只有第一张 |
| 图片下载 | `pbs.twimg.com/media/...?name=orig` | 200，281,599 字节，`image/jpeg` |
| 图片尺寸 | `orig` / `large` / `medium` / `small` | 282 KB / 282 KB / 135 KB / 53 KB |
| 视频（另一账号测试） | `video.twimg.com/...mp4` | 206 Range，`video/mp4`，4K 约 59.8 MB，CORS `*` |
| 错误 | 不存在的 handle | 404 |
| Feed | `fxtwitter.com/example_user/feed.xml?count=20` | 200，20 条，`application/rss+xml`，`cache-control: max-age=120` |
| Atom | `feed.atom.xml` | 200，20 条 |
| 媒体 Feed | `media.xml` | 200，仅媒体条目 |
| 自动降级 | `fixupx.com/{handle}/feed.xml?count=20` | 2026-11-02 实测 200，`application/rss+xml`，与 FxTwitter 同一 Feed；主源失败时 Rill 自动改用该域名，无需用户配备用源 |

### 3.2.1 自动降级（2026-11-02）

- 主源：`https://fxtwitter.com/{handle}/feed.xml?count=20`
- 自动域名备用：`https://fixupx.com/{handle}/feed.xml?count=20`（FxEmbed 公共域名）
- 定时刷新：主源失败后等 10 秒重试，最多 2 次（首次 + 2 次重试）；仍失败再自动切换备用域名一次。手动刷新不等待重试，主源失败后直接切换备用域名。成功时源备注写「已自动降级到 fixupx.com」。
- 2026-11-12 复测：`fixupx.com/example_user/feed.xml?count=20` 返回 200、`application/rss+xml`、约 43KB；此前一次 404 属瞬时响应，连续复测正常。
- 主源和备用域名都失败时写入 `last_status=error`，复用现有「需关注」列表；后续抓取成功自动清除。
- **重要限制（2026-11-02 实测）**：`fixupx.com` 与 `fxtwitter.com` 是同一套后端，域名级降级只能绕过单个域名/边缘 IP 不可达，不能绕过整体后端或网络封锁。本次从开发机测试：`fxtwitter.com` 解析到 IPv4 后 200、20 条；`fixupx.com` / `fxembed.com` 当前 TLS EOF（IPv4 也失败），属于网络侧阻断，不是解析器问题。自建 FxEmbed 才是真正的独立第二源（§3.5）。
- 抓取器新增 IPv4 优先拨号：部分微服/局域网 DNS 会返回不可路由的 IPv6，TCP 可连但 TLS 被重置；优先尝试 A 记录后再回退 AAAA，避免假 IPv6 拖死请求。
- 只有用户显式配置了 `fallback_url` 时才优先用配置地址；X 源不再需要手工准备备用源。
- 两个域名都失败时，错误同时显示主源和备用域的失败原因（HTTP 状态/网络错误）。

### 3.3 归一化规则

- `guid`：推文 URL（`https://x.com/{user}/status/{id}`）。
- 标题：正文第一行，截断 80 字符；无正文用 `(no text)`。
- 正文：`text` + 图片 `<img>` + 视频链接。
- 时间：`created_at` 转 RFC 3339。
- 媒体：图片默认 `?name=medium`，原图按需；视频只存 poster + 链接。
- 转推/回复：默认过滤，源配置可打开。
- 引用推文：当前 Feed 的 `<blockquote>` 只有文字，不含引用媒体；如需完整引用，走 JSON API 的 `quote` 字段。

### 3.3.1 全量回补（2026-11-01，REQ-022）

- 实现：`fetch.FetchXTimeline` 翻 `api.fxtwitter.com/2/profile/{handle}/statuses`，
  跟随 `cursor.bottom` 直到耗尽（平台上限约 3200 条）。
- **实测响应结构（2026-11-01）**：顶层为 `code/results/cursor`（早期调研记录的
  `tweets` 字段已改名，适配器两者兼容）；每页最多回 20 条（`count=100` 也只回
  20）；`media.all[]` 携带图片/视频（`type: photo/gif/video`），旧 `photos[]` 仍
  兼容解析。
- 去重键与 feed.xml 一致（推文永久链接），回补与窗口抓取可混跑不重复。
- 实测：`@example_user`（69 帖账号）在本系统切 `full` 后 60 秒内入库 **69 条**
  （窗口 20 → 全部 69），状态 ok。
- 限制：每页 20 条 → 3200 条需 ~160 页（页间 300ms，全程约 1 分钟）；仅公开账号。

### 3.3.2 源头像降级链（2026-11-21，BUG-004）

- 背景：部分 X 源头像显示失败（C1ownhjj、HiTw93 等）。头像候选链原为
  Feed `<image>`（`pbs.twimg.com`）→ `x.com` 页面 icon → `x.com/favicon.ico` →
  `fxtwitter.com/favicon.ico`；微服出口无法直连 `pbs.twimg.com` / `x.com` 时前三个
  候选全部失败，且每个候选 15s 超时，整链最长 ~60s，浏览器等待期间即判定失败。
  本机直连 `pbs.twimg.com/profile_images/...` 实测 200 `image/jpeg`，失败仅发生在
  微服出口网络。
- 修复：候选链在 Feed 图标之后插入 `https://unavatar.io/x/{handle}`
  （第三方聚合服务，实测 200 `image/jpeg` 返回真实头像），置于 `x.com` 站点探测
  之前；图标候选级超时收敛为 6s，最坏 ~30s 内落到可用候选。
- 已知取舍：unavatar.io 为第三方依赖，无 SLA；找不到头像时返回其占位图。
- 实测记录：2026-11-21 `unavatar.io/x/HiTw93` → 301 → 200 `image/jpeg`
  26,833 bytes；`fxtwitter.com/{handle}/favicon.ico` → 302 跳 `x.com`（不可用作
  头像）；`api.fxtwitter.com/{handle}` 返回的 avatar_url 仍是 `pbs.twimg.com`
  域名（无助于绕过）。
- 2026-11-22 BUG-007 修正候选顺序：FxTwitter Feed 的 `<image>` 会长期停留在旧
  头像（`@poteto` 实测 Feed 图为旧照片，`api.fxtwitter.com/poteto` 与
  `unavatar.io/x/poteto` 均为当前插画头像）。X 源改为先取
  `unavatar.io/x/{handle}`，Feed `<image>` 只作降级；启动清理、抓取后 6 小时失效与
  手动刷新清缓存继续生效。代价是头像正确性依赖 unavatar 的时效性。

### 3.4 已知限制

- **时间线不含回复推文（2026-11-05 实测）**：`feed.xml` 与
  `api.fxtwitter.com/2/profile/{handle}/statuses` 都只返回纯原创推文。用户
  `@example_user` 于 2026-11-04 14:39 UTC 发布推文 `2117990202375332073`（内容
  「@manateelazycat @imwritingbugs 已经将 Dim 移植到懒猫微服…」），单条接口
  返回 200 且 `replying_to: "manateelazycat"`，但该推文始终未出现在时间线与
  feed.xml（最新仍是 2026-11-01 的推文；fxtwitter.com / fixupx.com / api 三个
  端点交叉一致，可排除 Cloudflare 边缘缓存）。以 @ 提及开头经回复框发出的推文
  会被 X 归类为 reply，从时间线过滤。Rill 的 X 源（含全量回补）因此**收不到
  回复类推文**，刷新表现是「内容已更新但看不到新推」。无凭据的公共桥接均无
  含回复的时间线选项；如需覆盖回复，需要登录凭据类方案（见 §3.6 已排除）。
- 依赖第三方公共实例，无 SLA；实例变更、限流、封锁都会影响可用性。
- 无凭据时 NSFW 媒体可能不可用；受保护账号返回 404。
- 公共实例可能使用自己的凭据池；自建且不配凭据时限额更低。
- X 的 ToS 对抓取有限制；本项目仅私有部署、仅订阅公开账号。

### 3.5 降级方案：自建 FxEmbed（P2，未集成）

- 当前 Rill 代码**没有**部署或调用自建 FxEmbed；X 源降级仍使用公共 `fixupx.com`（与 `fxtwitter.com` 同一后端，只能绕开单域名/边缘 IP 故障）。
- 仓库：`FxEmbed/FxEmbed`（原 `FixTweet/FxTwitter`），MIT。
- 官方部署：Cloudflare Workers，免费额度 100,000 请求/天。
- 仓库自带 Dockerfile：

```dockerfile
CMD ["npx", "wrangler", "dev", "./dist/worker.js", "--local", "--ip", "0.0.0.0", "--port", "8787"]
```

- 可以不用 Cloudflare 账号，用 Docker 跑在懒猫微服或任意 Linux 上。
- **本次只读取了仓库的 Dockerfile 和配置，未实际构建/运行镜像；状态标记为"部分验证"。**
- 注意：
  - 这是 Wrangler 的本地开发运行时（workerd），不是生产 runtime。
  - 镜像基于 Node 26 + 完整构建产物 + workerd，体积和内存都不小。
  - 自建后需要把域名加入 `.env` 的 `STANDARD_DOMAIN_LIST` / `API_HOST_LIST`。
  - 凭据可选；不配凭据时限额更低、NSFW 不可用。
- Rill 侧只改 `XAdapter.BaseURL`，业务代码不变。

### 3.6 已排除的方案

| 方案 | 实测/结论 | 日期 |
|------|-----------|------|
| Nitter 公共实例 | `xcancel` 451、`nitter.poast` 000、`privacyredirect` 403、`lightbrd` 403、`nitter.net` 410 | 2026-10-31 |
| RSSHub 公共实例 Twitter 路由 | 302 到 Cloudflare；路由需要 `TWITTER_AUTH_TOKEN` cookie | 2026-10-31 |
| X 官方 API 免费层 | 读取额度不可用 | 2026-10-31 |
| X syndication 接口 | 本机 429，且需要 guest token | 2026-10-31 |

---

## 4. 微信公众号

> **2026-10-31 产品决策：** 公众号强制扫码登录，固定使用「公众号后台扫码」路径（可拉历史）。不提供公开专辑、公开主页、Wechat2RSS 等无登录入口。
>
> **2026-11-03 修正（BUG-001）：** 后台扫码链路只能抓 `switchacct` 返回的自有公众号；外部账号 `searchbiz` 可搜、`appmsgpublish` 返回 `200013`。外部未收录账号需 BestBlogs 或 WeRSs/微信读书等外部 RSS；内置链路不集成 WeRSs。
>
> **2026-11-03 产品决策（DEC-024 / REQ-034）：** 移除微信后台通道（内置链路与 we-mp-rss sidecar 一并删除）；公众号只保留微信读书扫码通道，BestBlogs 目录 + 文章链接为添加路径。下面 4.3 的无登录方案与 4.5 的后台链路保留作为调研/历史记录，不作为产品功能。

### 4.1 结论先说

**任意公众号、完全不登录、长期稳定的方案不存在。** 这是平台限制，不是项目实现问题。

两条登录路径里：

- **公众号后台扫码**：可拉历史，但只覆盖当前后台管理的自有公众号；外部账号列表自 2026-07 起返回 `200013`。
- **微信读书扫码**：不需要公众号账号；`/api/mp/cover` 只能拿最新一篇；`/web/mp/articles` 可翻页但可能被 `-2041` 风控拦截（不是登录验证码）。Rill 在列表被拦截时自动降级 cover 最新一篇。

**本项目自 2026-11-03 起只采用微信读书扫码通道（DEC-024）；公众号后台通道已移除。** 搜狗搜索、文章直链批量抓取不可控，不做。2026-11-03 的限流替代复核见 [`docs/research/2026-11-wechat-rate-limit-alternatives.md`](research/2026-11-wechat-rate-limit-alternatives.md)；微信读书通道尚未在 Rill 真机实测，不得写成“已验证”。

### 4.2 登录型方案对比

| 项目 | 状态 | 技术栈 | 登录方式 | 数据库 | 优点 | 缺点 |
|------|------|--------|----------|--------|------|------|
| **we-mp-rss** | 活跃（最后提交 2026-10-21，5.1k stars） | Python 3.13 + FastAPI + Playwright | 公众号扫码 / 微信读书扫码 | 默认 SQLite，可选 MySQL | 维护活跃、功能多、默认 SQLite、RSS/OPML 完整 | 镜像含 Chromium + WebKit，重 |
| **werss** | 停滞（最后提交 2026-04-28） | Python + FastAPI + Playwright | 公众号扫码 | SQLite | 你本地已有 LPK 适配 | 上游半年未更新，session 每天可能过期 |
| **WeWe RSS** | **已归档（2026-05-11）** | Node/NestJS + MySQL/SQLite | 微信读书扫码 | MySQL/SQLite | 9.6k stars，曾经最稳的微信读书方案 | 官方仓库已归档，不建议新项目采用 |
| **Wechat2RSS 私有部署** | 活跃（最后提交 2026-07-31） | 未公开 | 未公开 | 未公开 | 公开服务稳定 | 私有部署要付费，不能自建免费版 |
| **wechat-article-exporter** | 未验证 | Web | 公众号扫码 | 浏览器 | 导出能力强 | 不是 RSS 服务 |

**本项目只采用 `we-mp-rss`。** werss 和 WeWe RSS 不再作为依赖；本地 `../werss` 仅作为移植参考保留。选择理由是 we-mp-rss 维护活跃、默认 SQLite、采集模式更多，并且同样提供 RSS/Atom/OPML 输出。代价是镜像更重（Playwright Chromium + WebKit）。

#### 4.2.1 License 与商业使用

| 项目 | LICENSE | README / 协议里的商业限制 | 结论 |
|------|---------|---------------------------|------|
| `we-mp-rss` | MIT | README 未写商业限制 | 软件许可允许商业使用；保留版权声明 |
| `werss` | MIT | 有合规声明：不得未经许可商业分发第三方内容；商业用途需自行法务评估 | **软件许可允许商业使用**，但内容分发受限 |
| `WeWe RSS` | MIT | README 未写商业限制 | 软件许可允许商业使用；但仓库已归档 |
| `Wechat2RSS` | 无 LICENSE 文件 | 用户协议明确写「仅限学习研究，禁止商业用途，禁止内容分发」 | **不可商用** |

需要区分两件事：

- **软件许可**：MIT 允许商用、修改、再分发，只要保留版权和许可声明。
- **内容权利**：公众号文章版权归原作者/平台，商业使用内容仍需单独评估，MIT 不会授予内容授权。

`werss` README 里的「不得用于未经许可的商业分发」指的是**第三方内容**，不是禁止使用该软件本身。`Wechat2RSS` 则是软件协议本身禁止商用，性质不同。

#### 4.2.2 we-mp-rss 与 werss 的关系

两者不是两套完全不同的架构，基本可以视为**同源/衍生关系**：

- 目录结构几乎一致：`apis/`、`core/`、`driver/`、`jobs/`、`web_ui/`、`init_sys.py`、`job.py`。
- 内部命名仍沿用 WeRSS：README 标题、`WERSS_AUTH_WEB`、`WeRSS授权二维码` 等。
- 核心 RSS/OPML 代码同源，`core/rss.py` 的结构和接口高度相似。

差异主要在维护状态和功能集：

| 维度 | `we-mp-rss` | `werss` |
|------|-------------|---------|
| 维护 | 活跃（2026-10-21） | 停滞（2026-04-28） |
| 采集模式 | web / api / app + `weread_mp` + `weread` | web / api / app |
| 数据库 | 默认 SQLite，可选 MySQL | SQLite |
| 额外功能 | 级联系统、Access Key、过滤规则、13 主题、Markdown/PDF/JSON 导出、Webhook、环境异常统计 | 标签、热点、AI 标签提取、消息通知 |
| 浏览器依赖 | Playwright Chromium + WebKit | Playwright Firefox |
| LPK 适配 | 无，需要重新做 | 你本地已有 |
| License | MIT | MIT |

**结论：**

- 它不是"另一个架构"，而是同一套代码的活跃分支/衍生版。换它主要是换维护来源和功能集，不是换技术路线。
- 你本地 werss 的 47 个改动大多是 LPK 适配（Dockerfile.lzc、manifest、run.sh、inject 等），这些需要在新项目上重做一遍。
- 如果只依赖 `we-mp-rss`，本地 `werss` 可以保留为移植参考，不再作为运行时依赖。

### 4.3 无登录方案（已排除，仅备查）

#### 4.3.1 公开专辑页（`appmsgalbum`）

请求：

```text
GET https://mp.weixin.qq.com/mp/appmsgalbum?__biz={biz}&action=getalbum&album_id={album_id}
```

实测（2026-10-31）：

- 200，326,134 字节。
- 页面含 **92 个 `album__item`**。
- 无 `环境异常`、无验证码、无登录跳转。
- 每个条目有 `data-link`、`data-title`、`data-create-time`。

限制：

- 只有创建了「合集/专辑」的公众号才有。
- 需要知道 `__biz` 和 `album_id`。
- 只能拿到该专辑内的文章，不是全量历史。
- 文章正文直链可能触发风控。

#### 4.3.2 公开主页（`/mp/homepage`）

请求：

```text
POST https://mp.weixin.qq.com/mp/homepage?__biz={biz}&hid={hid}&begin=0&count=5&action=appmsg_list
```

实测（2026-10-31）：

- 200，JSON，`base_resp.ret = 0`。
- 返回 `appmsg_list`，含标题、封面、链接。
- 无登录。

限制：

- 只适用于有「首页模板」的公众号（分享链接带 `homepage`）。
- 需要 `biz` + `hid`，部分账号还有 `cid`。
- 只覆盖模板展示的文章，不是全量历史。

#### 4.3.3 Wechat2RSS 公共源

请求：

```text
GET https://wechat2rss.xlab.app/feed/{id}.xml
```

实测（2026-10-31）：

- 200，20 条/源。
- 抽样 6 个源：部分源最新文章更新到 2026-10-30，部分源停留在 2025 年（可能是账号停更或服务未更新，未进一步区分）。
- 列表页可正常访问，收录 300+ 公众号。
- 私有部署为付费产品，仓库不含服务端源码。

限制：

- 只能订阅其已收录的账号，不能任意添加。
- 依赖第三方服务；免费服务无 SLA。
- 不能自建免费版。

#### 4.3.4 搜狗微信搜索

请求：

```text
GET https://weixin.sogou.com/weixin?type=2&query={keyword}
```

实测（2026-10-31）：

- 搜索页 200，可解析出 9 条结果。
- 但点击 `/link?url=...` 会 302 到 `/antispider/`。
- 无法稳定拿到文章真实 URL。

结论：**不做。**

#### 4.3.5 文章直链

请求：

```text
GET https://mp.weixin.qq.com/s?__biz=...&mid=...&idx=...&sn=...
```

实测（2026-10-31）：

- 302 到 `https://mp.weixin.qq.com/mp/wappoc_appmsgcaptcha?...`。
- 最终 200 是验证码页，拿不到 `js_content`。
- 受 IP/频率影响，住宅网络可能偶尔可用，但不能作为产品依赖。

结论：**不做批量直链抓取。**

#### 4.3.6 RSSHub 的微信路由

RSSHub 现有路由：

- `/wechat/mp/msgalbum/:biz/:aid` — 公开专辑页。
- `/wechat/mp/homepage/:biz/:hid/:cid?` — 公开主页 JSON。
- `/wechat/wechat2rss/:id` — 代理 Wechat2RSS。
- `/wechat/sogou/:keyword` — 搜狗搜索。
- `/wechat/ce/:id` — CareerEngine 来源。
- `/wechat/uread/:id` — 微信读书（需登录）。

实测（2026-10-31）：

- 公开专辑/主页的上游页面本身可用。
- `rsshub.app` 公共实例从本机访问被 Cloudflare 403。
- RSSHub 不是"额外的魔法源"，只是把上面这些公开页面包装成 RSS。

### 4.4 产品策略（2026-11-03，DEC-024 / REQ-034）

**公众号源强制扫码登录；通道只有一条：微信读书扫码。公众号后台通道（内置链路 + we-mp-rss sidecar）已整体移除。**

```text
微信读书通道（唯一）：
设置 → 公众号接入 → 微信读书扫码
  → Rill 后端生成二维码（getLoginUid → web/confirm?uid=…）
  → 轮询 getLoginInfo；确认后合并 Set-Cookie 并 renewal
  → Cookie AES-GCM 加密存 settings（weread_cookie_enc），UID 不返回前端
  → 添加源：BestBlogs 目录命中直接建 kind=rss；
    未收录时粘贴文章链接，公开页提取 biz 建 wechat://<fakeid> 源
  → Rill 由 fakeid 推导 MP_WXS_* bookId，抓取走 /web/mp/articles + /web/mp/content
```

要求：

- 创建 `wechat://` 源前必须已连接微信读书扫码；否则源永远无法同步，后端直接拒绝。
- 不再提供公众号后台名称搜索（`searchbiz` 随后台通道移除）；BestBlogs 未收录的号只能粘贴文章链接。
- 扫码轮询对暂时性网络错误/无法解析的响应保持等待，由二维码 TTL（5 分钟）兜底过期；扫码成功后设置立即反映 `weread_connected`，前端刷新并提示。
- 公众号源错误信息含「登录态/401」时置 `auth` 并提示重新扫码。
- 不提供公开专辑、公开主页、Wechat2RSS 等无登录入口。
- 用户可把 WeRSs 等导出的标准 RSS 作为普通源添加。
- 不把 Playwright 带进主镜像。
- Cookie 仅服务端加密存储，不返回前端、不写日志；旧后台会话键 `wechat_session_enc` 不再读取，保留在库中不主动清理。

UI 文案：

- 微信读书二维码旁："微信读书 Cookie 仅加密保存在本机 Rill 数据库；请用微信扫码并在手机确认。"
- 未收录公众号："BestBlogs 目录没有找到这个公众号。请粘贴该公众号任意一篇文章链接（mp.weixin.qq.com/s/...）添加。"

### 4.5 公众号后台内置链路（2026-11-02，REQ-026；2026-11-03 已随 DEC-024 移除）

> **状态：已移除。** 保留本节作为能力边界的历史记录。移除原因：后台链路只能抓 `switchacct` 自有账号（BUG-001），对用户没有意义；`searchbiz` 名称搜索随之消失，外部未收录账号改走 BestBlogs 目录 + 文章链接（§4.4）。

| 项 | 内容 |
|---|---|
| 实现方式（历史） | Go 移植 `rachelos/we-mp-rss`（MIT）核心链路：`bizlogin startlogin` → `scanloginqrcode getqrcode/ask` → `bizlogin login` → `switchacct get_acct_list` → `appmsgpublish list_ex` → 文章页 HTTP 抓取 |
| 入口/端点（历史） | `https://mp.weixin.qq.com/cgi-bin/*`；`/api/v1/wechat/scan`、`/wechat/scan/status`、`/wechat/search`、`/wechat/accounts`、`/wechat/logout` 已删除，返回 404 |
| 鉴权（历史） | 公众号后台扫码；会话加密存 `settings`（`wechat_session_enc`），该键不再读取 |
| 实测记录（保留） | **部分验证（2026-11-03）**：真实登录会话下，自有账号 `gh_3df5c6b01cc2` 的 `appmsgpublish` `ret=0`；「二哥狗腿子」「少数派」「晚点LatePost」等外部账号均 `ret=200013`；`searchbiz` 对外部账号仍 `ret=0` |
| 合规备注 | 上游 MIT，保留版权声明 |

### 4.6 微信读书通道（2026-11-03，REQ-033/REQ-034 / DEC-023/DEC-024，唯一公众号通道）

| 项 | 内容 |
|---|---|
| 状态 | **部分验证（2026-11-03 起为唯一公众号通道，DEC-024）**：本地 httptest 覆盖扫码 Cookie 加密与真实校验、`weread_connected` 设置回调、真实响应形状（数字 `webLoginVid`/嵌套 `data`/长令牌）、fakeid→bookId、`wechat://` 源直接抓取、列表 `-2041` → cover 降级、双失败冷却；真实扫码端到端回调与「二哥狗腿子」真实列表/正文待验收 |
| 实现方式 | 微信读书 Web Cookie 通道；公众号 fakeid 推导 `bookId = MP_WXS_<base64decode(fakeid)>`；幂等 `POST /mp/shelf/addToShelf`；`GET /web/mp/articles` 列表，空列表回退 `GET /api/mp/cover` 最新一篇；`GET /web/mp/content` 提取正文与 canonical 原文链接 |
| 入口/端点 | `https://weread.qq.com/*`；Rill API：`POST /api/v1/wechat/weread/scan`、`GET /api/v1/wechat/weread/scan/status` |
| 鉴权 | 微信读书扫码；`getLoginUid` → 手机确认 `getLoginInfo`（**长轮询，等待态常返回 `LOGIN_TIMEOUT`**）→ 合并 Set-Cookie → 候选 Cookie 校验（`/web/shelf/sync`）→ 失败再 `/web/login/renewal`；Cookie AES-GCM 加密存 `settings`（`weread_cookie_enc`）。真机响应容错（2026-11-03）：`webLoginVid` 可能是数字、字段可能在顶层或 `data` 内层、`logicCode` 有 `PENDING`/`SCANNED`；`wr_skey` 原始 Set-Cookie 短值优先，`refreshToken`（`web@...`）作为候选兜底，只有真实接口校验通过的组合才落库。**续期请求体（2026-11-09 修正）：** 必须使用 `{"rq":"%2Fweb%2Fbook%2Fread","ql":false}`。实测 `ql:true` 即使 `wr_rt` 有效也会返回 200 + `{"errCode":-2013,"errMsg":"鉴权失败"}` + 4 个空值 Set-Cookie（清场）；`ql:false`（或省略 `ql`）返回 `{"succ":1}` 与新下发的 `wr_vid/wr_skey/wr_pf/wr_rt`。2026-11-09 10:47 扫码后到 15:10 未续期，`wr_skey` 已过期但 `wr_rt` 用 `ql:false` 仍换票成功——此前的 `-2013` 是请求体错误，不是 `wr_rt` 过期。**keep-alive：** 调度器每 30 秒 tick，距上次续期 ≥30 分钟主动续期一次（扫码成功/续期成功重置计时），模拟官方网页常开；`renewWereadCookie` 跳过空值 Set-Cookie，且回写前必须通过 `/web/shelf/sync` 真实校验；失败日志带 HTTP 状态 + body errCode + set-cookie 数量 |
| 成本 | 免费；无额外容器/镜像 |
| 限流 | 文章正文间隔 2s；列表 `-2041` 先降级 cover，cover 也 `-2041` 才触发 4 小时通道冷却；公众号源刷新下限 30 分钟 |
| 去重键 | `reviewId`；缺失时 canonical 原文链接 |
| 媒体处理 | 正文 HTML 图片重写为 `/media/{token}/{hash}` 并登记 `media_cache`，由服务端无 Referer 下载缓存；`<img>` 同时带 `referrerpolicy="no-referrer"`。`mmbiz.qpic.cn` 按 Referer 防盗链，浏览器直连时若带 Rill 域名 Referer 会返回「未经允许不可引用」占位图（BUG-002）。`data:` 内联图片不代理 |
| 降级 | 列表 `-2041` → `/api/mp/cover` 最新一篇（正文也失败则保留标题/链接）；cover 也失败 → 4h 冷却。BestBlogs 目录源（kind=rss）不受影响 |
| 已知限制 | 接口非公开；收录不全且可能滞后；`/web/mp/articles` 可能被 `-2041` 风控/频控拦截（**不是登录验证码**，也不要求用户完成验证）——列表被拦截时自动降级 `/api/mp/cover` 只同步最新一篇，cover 也失败才进入 4h 冷却；`-2012` 过期（登录成功时先做 Cookie 校验，避免保存无效会话）；`-2042` 未收录；不能承诺历史回补。存量文章若在修复前已入库，正文仍保留远程图片地址，依赖前端 `no-referrer` 显示；只有删除源后重建或条目重新入库，才会转为 `/media` 缓存 |
| 合规备注 | 仅读取用户微信读书账号可见内容，个人阅读使用；Cookie 不返回前端、不写日志 |

### 4.7 BestBlogs 托管公众号 Feed（2026-11-03，仅标准 RSS）

BestBlogs 维护了一批微信公众号 RSS 源，域名形如 `wechat2rss.bestblogs.dev`。**Rill 不新增适配器**，直接按标准 RSS 抓取；这是当前「不自建 Wechat2RSS、不登录公众号后台」的最低成本接入方式。REQ-031 只增加目录解析：`POST /sources/resolve` 在公众号裸名称时先查公开 OPML，命中返回 `platform=bestblogs/kind=rss` 候选；实际抓取仍走标准 RSS。

| 项 | 内容 |
|---|---|
| 实现方式 | 标准 RSS 2.0，复用既有 `gofeed` 管线；不新增 `kind`、不新增抓取器 |
| 入口/端点 | `https://wechat2rss.bestblogs.dev/feed/{feed_id}.xml`；目录文章 <https://www.bestblogs.dev/article/812c610e>；OPML <https://github.com/ginobefun/BestBlogs/blob/main/opml/bestblogs_wechat2rss_opml_all.opml> |
| 鉴权 | 无；Feed URL 直接可读 |
| 成本 | 免费；BestBlogs 账号仅用于其站内功能，抓取 Feed 不需要 API Key |
| 限流 | Feed 侧未观察到明确公开配额；按普通 RSS 低频抓取，建议每源 ≥30 分钟，避免集中刷新 |
| 去重键 | 标准 RSS：`guid` → `link` → 内容哈希 |
| 媒体处理 | 正文通常在 `content:encoded`；图片走既有 `/media/` 代理与缓存 |
| 降级 | 换用另一个 BestBlogs Feed、改用自建 Wechat2RSS，或直接按文章 URL 导入；Rill 无专用回退 |
| 实测记录 | **部分验证（2026-11-03）**：样例 `ff621c3e98d6ae6fceb3397e57441ffc6ea3c17f.xml` HTTP 200 `application/xml`、489,418 字节、10 条；最新 `2026-11-02 08:19 +0800`；`content:encoded` 约 21k–58k 字符；与 Rill 同库的 `gofeed` 解析 10 条，首条正文 72,740 字符。另抽查 4 个 Feed 均 200、各 10 条，最新到 2026-11-02/11-01。OPML 含 375 个 `outline`。Rill `cmd/fetchtest` 对样例返回 200；尚未在 Rill 中完成“加源→入库→阅读”的整链验收 |
| 已知限制 | 只覆盖 BestBlogs 已收录的公众号（当前 375 个），不能任意添加；Feed ID 不透明，需从目录/OPML 获取；每个 Feed 只保留近期约 10 条，**无历史回补**；第三方托管，无 SLA，可能改地址或停止；文章版权归原作者 |
| 合规备注 | 仅个人阅读使用；不把 Feed 内容用于公开聚合、批量转载或商业分发；同时遵守 BestBlogs / Wechat2RSS 的服务条款 |

接入方式：

1. 在 BestBlogs 的公众号目录/OPML 中找到目标账号的 `xmlUrl`。
2. Rill「添加源」粘贴该 URL，或「订阅源 → 导入 OPML」导入筛选后的 OPML。
3. 不建议直接导入完整 375 个源；会一次创建大量订阅，按需裁剪后再导入。

---

## 5. 平台对标（2026-11-02，Folo）

Folo 的“平台支持”主要是 RSSHub 路由，不是原生适配器；其公开 Feed 端点
`GET https://api.folo.is/feeds?url=...` 可匿名预览。实测结论：

| 平台 | Folo 入口 | 结果 | Rill 取舍 |
|---|---|---|---|
| Apple Podcasts | 无专门入口，直接贴 Apple 页面返回 400 | 不解析 | **做得比 Folo 好**：iTunes Lookup → `feedUrl` |
| 小宇宙 | `rsshub://xiaoyuzhou/podcast/{id}` | 200 | 优先 `feed.xyzfm.space`，页面 15 条降级 |
| 微博 | `rsshub://weibo/user/{uid}` | 200；12 个热门源 2 个错误 | 匿名访客态最近 100 条，不承诺全量 |
| Threads | `rsshub://threads/{user}` | 200；3 个热门源 0 错误 | 已有 `rsshub://` 支持 |
| Bluesky | `rsshub://bsky/profile/{handle}` / 官方 RSS | 200 | 已有官方 RSS |
| YouTube | `rsshub://youtube/...` | 200；15 个热门源 3 个错误 | 本机出口超时，暂不纳入 |
| Bilibili | `rsshub://bilibili/user/video/{uid}` | 200；68 个热门源 41 个错误 | 暂不纳入 |
| Telegram | `rsshub://telegram/channel/{name}` | 200；相对稳定 | 后续按需 |
| 知乎 | `rsshub://zhihu/...` | 分化；39 个热门源 28 个错误 | 暂不纳入 |
| 小红书 | `rsshub://xiaohongshu/...` | `Failed to fetch`，仅缓存 | 不做 |
| Instagram | `rsshub://instagram/...` | `400 challenge_required`，仅缓存 | 不做 |
| 微信公众号 | RSSHub 第三方路由 | 15 个热门源 9 个错误 | 继续用扫码登录，不用第三方路由 |
| X | RSSHub 付费路由 | Free 用户 402 | 继续用 FxTwitter |

结论：Rill 不按“RSSHub 路由数量”宣称平台支持；只把有独立验证证据的源写进
支持矩阵。Medium / Mastodon 做 URL 归一化；YouTube / Bilibili / Telegram 需要
RSSHub 实例与凭据，后续单独立项，不计入本批“稳定订阅”。

---

## 6. 未来可能新增的源

| 源 | 实现方式 | 状态 |
|----|----------|------|
| Telegram 频道 | `t.me/s/{channel}` HTML 解析或 RSSHub | 计划 |
| YouTube 频道 | 原生 `feeds/videos.xml?channel_id=` 或 RSSHub | 受阻（本机出口超时） |
| Bilibili 用户 | RSSHub `bilibili/user/video/{uid}` | 计划（依赖实例/Cookie） |
| Substack | `{site}/feed` | 计划 |
| 网页变更监控 | 类似 changedetection.io | 计划 |

---

## 7. 新增源登记模板

复制以下模板到本文档末尾，填写后再写代码：

```markdown
## <源名称>

- 状态：计划 / 部分验证 / 已验证 / 受阻
- 实现方式：
- 入口/端点：
- 鉴权：
- 成本：
- 限流：
- 数据字段：
- 去重键：
- 媒体处理：
- 降级方案：
- 实测记录（日期 / 请求 / 状态码 / 关键特征）：
- 已知限制：
- 合规备注：
```

### 新增源检查清单

- [ ] 是否验证了无需登录/需要登录的真实边界？
- [ ] 是否记录了状态码和响应特征？
- [ ] 是否写明了限流和最小抓取间隔？
- [ ] 是否有降级方案？
- [ ] 是否评估了媒体代理与出站抓取风险（scheme 校验、大小上限；2026-11-01 起不再拦截私网，DEC-015）？
- [ ] 是否同步更新了 PRD 的源支持矩阵和 PDD 的适配器清单？
- [ ] 是否避免在文档中写入真实凭据？

---

## 8. 决策记录

| 日期 | 决策 | 理由 |
|------|------|------|
| 2026-10-31 | X 主用 FxTwitter 公共 API，FxEmbed 自建为降级 | 公共 API 实测可用、无 cookie、免费；自建成本高 |
| 2026-10-31 | 标准协议用 `gofeed` + 条件请求 | 覆盖全、成熟、304 实测通过 |
| 2026-10-31 | 公众号不做"任意账号免登录"承诺 | 平台限制，无稳定方案 |
| 2026-10-31 | **公众号强制扫码登录，唯一依赖 `we-mp-rss`** | 用户决策；只走公众号后台扫码，可拉历史 |
| 2026-10-31 | 不依赖 werss / WeWe RSS | werss 上游停滞；WeWe RSS 已归档；we-mp-rss 同源且活跃 |
| 2026-10-31 | 不采用公开专辑/主页/Wechat2RSS 作为产品入口 | 用户要求强制登录；这些方案仅保留为调研记录 |

### 2.16 全文抓取（2026-11-12，REQ-039 / F-17）

- 范围：仅标准 RSS 源，按源开关；存量 `sources.config_json.full_text` 保存，不新增列。
- 触发：① 自动模式：Feed 抓取成功且有条目时，仅对本次新入库条目抓原文，Feed 正文已足够长（>1200 字）时跳过；② 手动模式：正文/专注阅读右上角「抓取全文」对当前文章即时抓取（15 秒超时，最小 60 字）。
- 提取：抓取原文页（2MB 上限、7 秒/条、并发 4、单次最多 12 条），优先 `article` / `main` / `[role=main]` / 常见正文容器，回退整页正文；移除导航、页头、页脚、侧栏、脚本与表单。
- 清洗：复用 `SanitizeContent`，相对链接转绝对、图片走 `/media/{token}/{hash}` 代理；提取文本少于 160 字或未明显优于 Feed 摘要时保留原文摘要。
- 降级：原文页 4xx/5xx、反爬、超时、非 HTML、提取失败均不影响条目入库，保留 Feed 摘要。
- 限制：首次抓取最多处理 12 条新文章，剩余条目仍是摘要；不保证登录墙、付费墙、强反爬站点；不改变 Feed 输出与全文搜索索引的既有语义。
- 实测：2026-11-12 本地单测覆盖 article 选择、导航/页脚剔除与按需抓取端点；真实站点批量提取待验收。

### 2.17 邮箱自动同步（IMAP，2026-11-21，REQ-041）

- 状态：已实现（本地验证；真实邮箱端到端验收前不得标「已验证」）。
- 实现方式：`github.com/emersion/go-imap/v2` 客户端经 TLS 连接用户配置的邮箱（993），`LIST` 后遍历所有可同步文件夹；`BODY.PEEK[]` 拉全文，不发送 `STORE \Seen`。首次按序列号取账号同步上限内的最新邮件建立基线，之后按 `UIDVALIDITY + LastUID` 增量拉取。MIME 解析用标准库 `net/mail` + `mime/multipart` + `mime.WordDecoder`。
- 入口/端点：设置页「邮箱同步」可添加多个账号（上限 10 个），各自配置 host/port/账号/授权码/轮询间隔/单次同步数量/可选分组；不再提供文件夹参数。每个账号一个 `imap://{username}` 源，`kind=rss`（不新增 kind、无 schema 迁移）。
- 同步范围：`INBOX` 与所有自定义文件夹；排除已发送、草稿、垃圾箱、垃圾邮件及 `\All`/`\Flagged` 等虚拟视图。单文件夹最多 100 封/轮，单轮上限跟随账号设置（10–500，默认 60）。
- 源命名与分组：源名 = 邮箱账号；每个账号可选归入一个 Rill 分组，不选则未分组。旧版按发件人源保留内容但自动停用；移除账号时停用其源并保留内容。
- 鉴权：IMAP LOGIN，使用邮箱客户端授权码/应用密码；SecretBox AES-GCM 按账号加密存 `settings`（键 `email_password_enc:{account_id}`，旧版回退 `email_password_enc`），接口只返回 `password_set`。
- 成本：仅微服出站网络与内存计算，零外部 API 成本。
- 限流：每个账号独立轮询，默认 15 分钟（下限 5 分钟）；空轮询交互 < 1KB。同步游标按账号存 `settings` 键 `email_sync_state:{account_id}`。
- 数据字段：`guid=Message-ID`（缺失用 `SHA-256(From+Subject+Date)`）；`title=Subject`（RFC 2047 解码）；`author=From` 友好名；`published_at=Date`；`link` 优先 `Archived-At`，次选正文「View in browser」链接，兜底空。
- 去重键：`Message-ID`；入库复用 `applyFeedResult` 的 `(source_id, guid)` upsert，过滤规则/关注提醒/保留策略自动生效。
- 媒体处理：`multipart/related` 内联附件（`cid:`）写入媒体缓存目录并直插 `media_cache` `status='ok'` 行，正文引用重写为 `/media/{token}/{hash}`，并标记 `data-inline-image=1`；远程图片也保留 `data-remote-src` 原始地址。非内联附件仅登记缓存不入正文。邮件正文使用专用清洗：保留安全的内联样式、`background`、表格/字体布局标签与 `data:image/...;base64`；协议相对地址补全、仅 `srcset` 时取首个候选、CID 图片按内容嗅探兼容错误 Content-Type（BUG-008）。
- 正文展示：邮件正文使用 `is-email` 样式，嵌套布局表格不再逐层描边；远程图片默认拦截（含 CSS 背景图），阅读器提供「显示图片」与「始终显示此发件人的图片」。「显示图片」把该条目 `media_json.email_images_allowed` 置为 true（`PATCH /items/{id}/email-images`），之后重新打开该邮件仍直接显示；「始终显示此发件人的图片」把发件人地址写入 `settings.email.trusted_senders`，后续该发件人的邮件自动放行。放行后优先使用 `/media` 代理，加载失败时回退 `data-remote-src` 原图。
- 降级方案：仅 `text/plain` 时按段落转 `<p>`；正文经 `SanitizeContent` 清洗；LOGIN 失败置 `last_status=error` 并提示「检查客户端授权码」；单文件夹失败跳过并保留其旧游标，单封失败下轮重试。
- 实测记录：2026-11-21 本地单测覆盖（`TestParseEmailMessage` multipart/CID/RFC2047、`TestBuildEmailFeedItemRewritesCID` 清洗与媒体重写、`TestParseEmailQuotedPrintableAndCharset` GBK、`TestEmailSettingsPatchValidationAndRedaction` 多账号/加密/打码/停用旧源/放行列表、`TestEmailItemRoutingWithoutIMAPServer` 入库与去重、`TestSyncableMailboxesExcludesSystemFolders` 文件夹过滤、`TestEmailSyncStateResetsOnAccountChange` 游标隔离）；2026-11-23 新增 `TestPatchItemEmailImagesPersists` / `TestPatchItemEmailImagesRejectsNonEmail` 覆盖单封放行持久化（本地通过）。真实邮箱（163/QQ/Outlook 任一）端到端同步待验收后补记（日期/账号类型/文件夹数/拉取条数）。
- 已知限制：首次同步不回溯全部历史；单次同步数量受账号设置约束；不自建 SMTP 收信（25 端口在家庭宽带/云环境封禁，无 MX/PTR）；Gmail 需微服出口可达 Google；被邮箱厂商投进垃圾箱的信件收不到；授权码与邮箱密码是两个东西，网页登录密码不可用。
- 合规备注：用户授权读取自己邮箱中的邮件，属个人阅读用途；凭据与同步游标只在微服本地加密保管，不进 OPML 导出。
