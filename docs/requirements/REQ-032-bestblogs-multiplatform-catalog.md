# REQ-032 BestBlogs 多平台目录

- 状态：进行中（实现、单测与 API 整链验证通过；真实浏览器/真机待验收）
- 优先级：P1
- 日期：2026-11-03
- 关联：PRD F-01/F-11/F-15/F-23、PDD §5.2/§7.2/§7.5、[RSS-SOURCES](../RSS-SOURCES.md) §2.14/§4.7、[REQ-031](REQ-031-bestblogs-wechat-catalog.md)、DEC-021

## 1. 背景

用户在 BestBlogs 公众号目录之后，要求「全面接多平台目录」。BestBlogs 公开维护多类 OPML：公众号 375、X 160、小宇宙 30、YouTube 40。它们不是同一个上游：公众号走 Wechat2RSS，小宇宙走 BestBlogs 的 RSSHub，X 走 `api.xgo.ing`，YouTube 是官方 RSS。

## 2. 目标

1. `bestblogs:{名称}` 跨公众号、小宇宙、X、YouTube 目录搜索，返回带平台标识的候选。
2. 识别四类 BestBlogs 目录 Feed URL；创建时统一 `kind=rss`，不新增源类型/schema。
3. 保留平台优先级：公众号 BestBlogs 优先；小宇宙官方 `feed.xyzfm.space` 优先；X FxTwitter 优先；YouTube 官方 RSS 优先。
4. 候选文案标明目录来源与限制，避免把第三方目录当成官方接口。
5. 公众号裸名称仍自动先查 BestBlogs；其他裸名称继续走现有平台解析，必要时用 `bestblogs:` 显式搜索。

## 3. 非目标

- 不把「所有平台都先走 BestBlogs」做成全局开关。
- 不承诺 BestBlogs 目录覆盖任意账号；只覆盖 OPML 收录项。
- 不保证历史回补：公众号约 10 条；X 目录最多 50 条；播客/YouTube 取决于上游。
- 不保存 BestBlogs API Key；不使用 OpenAPI。
- 不改变 X FxTwitter、小宇宙官方 Feed、YouTube 官方 Feed 的主路径。

## 4. 方案

### 4.1 目录

| 平台 | OPML | Feed host | 默认优先级 |
|---|---|---|---|
| 公众号 | `bestblogs_wechat2rss_opml_all.opml` | `wechat2rss.bestblogs.dev/feed/{40hex}.xml` | BestBlogs 优先 |
| 小宇宙 | `BestBlogs_RSS_Podcasts.opml` | `rsshub.bestblogs.dev/xiaoyuzhou/podcast/{24hex}` | 官方 Feed 优先，目录降级 |
| X | `BestBlogs_RSS_Twitters.opml` | `api.xgo.ing/rss/user/{32hex}` | FxTwitter 优先，目录降级 |
| YouTube | `BestBlogs_RSS_Videos.opml` | `youtube.com/feeds/videos.xml?channel_id=...` | 官方 RSS 优先（目录仅帮助发现） |

默认从 jsDelivr 镜像读取、GitHub raw 兜底，公众号目录可用 `RILL_BESTBLOGS_CATALOG_URL` 覆盖；其余目录固定从 BestBlogs 仓库读取。每个目录独立校验 host/path，解析失败只跳过该类，不影响其他平台。成功目录缓存 6h；任一类失败时 10 分钟后重试，避免一次瞬时失败把缺失目录锁死 6h。

### 4.2 候选

`platform` 区分：

- `bestblogs`：公众号
- `bestblogs-podcast`：小宇宙
- `bestblogs-x`：X
- `bestblogs-youtube`：YouTube

所有候选 `kind=rss`，`note` 标注来源与限制。`bestblogs:` 前缀跨目录搜索；裸中文名称仍自动搜索，ASCII handle/名称继续走平台主路径（X 走 FxTwitter），避免把 BestBlogs 变成全局主源。

### 4.3 前端

- 平台标签：`BestBlogs` / `BestBlogs 播客` / `BestBlogs X` / `BestBlogs YouTube`。
- 订阅源树按 Feed host 显示「公众号 / 小宇宙 / X / YouTube」标签，底层仍为 RSS。
- `bestblogs:` 前缀写入提示文案；普通 X 链接/用户名仍优先 FxTwitter。

## 5. 验收

- 后端单测：四类 Feed URL 校验、四类 OPML 解析、跨目录搜索、单目录失败不影响其他目录、部分失败 10 分钟重试。
- 前端：平台标签、host 识别、mock 候选。
- 真实网络：四类 jsDelivr OPML 均 200；公众号样例 10 条、X 样例 50 条、小宇宙样例 1 条以上；YouTube 从当前开发机 TLS 超时，标注未验证。
- `go test -tags live ./internal/server -run 'TestLivePlatformResolvers/bestblogs'`：四类目录搜索通过；公众号与 X 经 `POST /sources/resolve` → 建源 → 抓取 → 阅读 API 各入库 5 条。
- `go test ./...`、`go vet ./...`、`go test -race ./internal/server`、前端 typecheck、30 项 Vitest、生产构建。
- 未验证：真实浏览器整链、605 个目录项全量可用率、YouTube 出口、目录长期可用性、LPK 重打包。

## 6. 风险与回滚

- 第三方目录/Feed 变更：单目录失败跳过并保留 stale；10 分钟后重试；`bestblogs:` 显式搜索失败时提示。
- X `api.xgo.ing` 与 FxTwitter 都可能失效：保留双路径，不把目录当唯一源。
- 版权/合规：仅个人阅读；不公开聚合、不商业分发。
- 回滚：删除对应目录定义与平台识别即可；已创建源仍是标准 RSS。
