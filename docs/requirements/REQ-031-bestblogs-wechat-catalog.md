# REQ-031 BestBlogs 公众号目录优先解析

- 状态：已完成（本地验证；真实浏览器整链待验收）
- 优先级：P1
- 日期：2026-11-03
- 关联：PRD F-01/F-15、PDD §5.2/§7.4、[RSS-SOURCES](../RSS-SOURCES.md) §4.7、[REQ-030](REQ-030-platform-resolvers.md)

## 1. 背景

公众号后台列表在 2026-07 前后出现平台级 `200013` 收紧；用户明确选择「优先从 BestBlogs 订阅」，不希望先自建 Wechat2RSS。BestBlogs 在 `wechat2rss.bestblogs.dev` 托管了一批公众号 RSS，并提供公开目录/OPML。

## 2. 目标

1. 用户输入公众号名称时，**先查 BestBlogs 公开目录**；命中则直接创建标准 RSS 源，不要求公众号后台登录。
2. 用户粘贴 `https://wechat2rss.bestblogs.dev/feed/{id}.xml` 时，识别为 BestBlogs 公众号源并创建 `kind=rss`。
3. BestBlogs 未命中时，保持现有行为：公众号名称/文章链接继续走公众号后台搜索/解析；普通 URL 继续走 Feed 发现。
4. 不新增数据库表/字段/迁移，不新增 `sources.kind`，不改变 Feed/OPML/备份语义。
5. 候选列表标明「BestBlogs」和「仅近期文章」，避免被误认为全量历史。

## 3. 非目标

- 不支持任意公众号：只覆盖 BestBlogs 目录当前收录的账号。
- 不承诺历史回补：BestBlogs Feed 只保留近期约 10 条。
- 不调用 BestBlogs OpenAPI，不引入 API Key；只读公开 OPML。
- 不把 BestBlogs 内容用于公开聚合、批量转载或商业分发。
- 不替换公众号后台通道；它仍是未命中时的降级路径。

## 4. 方案

### 4.1 目录来源

- 默认：`https://cdn.jsdelivr.net/gh/ginobefun/BestBlogs@main/opml/bestblogs_wechat2rss_opml_all.opml`（jsDelivr 镜像；GitHub raw 可作备选）。
- 环境变量 `RILL_BESTBLOGS_CATALOG_URL` 可覆盖为 GitHub raw 或自建镜像。
- App 内缓存 6 小时；刷新失败时保留旧缓存；无缓存且拉取失败时静默回退到公众号后台搜索。
- 只接受 `https://wechat2rss.bestblogs.dev/feed/{id}.xml` 形式的 Feed，拒绝其他 host/path，避免目录被污染后引入任意抓取目标。

### 4.2 解析优先级

`POST /api/v1/sources/resolve` 内按以下顺序：

1. 显式 `bestblogs:` 前缀 → 只查 BestBlogs 目录。
2. BestBlogs Feed URL → 直接返回候选（尽可能读取 Feed 标题）。
3. 裸公众号名称 → 先查 BestBlogs；命中返回候选；未命中/目录不可用则返回未识别，让前端继续公众号后台搜索。
4. 其余平台（Apple、小宇宙、微博、Medium、Mastodon）逻辑不变。

候选字段：

```json
{
  "platform": "bestblogs",
  "kind": "rss",
  "name": "数字生命卡兹克",
  "feed_url": "https://wechat2rss.bestblogs.dev/feed/....xml",
  "note": "BestBlogs 托管公众号 · 仅近期文章"
}
```

### 4.3 前端

- 平台标签新增 `BestBlogs`。
- 命中 BestBlogs 时多候选/单候选复用现有 `resolvedCandidates` 交互。
- 添加后 `kind=rss`，刷新间隔 3600 秒。
- 订阅源列表对 BestBlogs host 显示「公众号」标签，但底层仍是 RSS，避免误走公众号登录态逻辑。

## 5. 验收

- `go test ./...`：目录解析、名称匹配、Feed URL 校验、目录失败回退、公众号后台回退。
- 前端 typecheck/test/build：平台标签与添加源逻辑。
- 真实 BestBlogs 目录：至少 5 个 Feed HTTP 200；样例 Feed 10 条、含 `content:encoded`、最新日期可见。
- Rill 端到端：输入「数字生命卡兹克」→ 选中 BestBlogs 候选 → 创建 RSS 源 → 抓取入库（需本地真实网络/浏览器）。
- 未验证项：BestBlogs 目录长期可用性、375 个 Feed 全量可用率、版权/商用边界。

## 6. 风险与回滚

- 外部依赖：GitHub raw 在部分地区可能不可达；可用 `RILL_BESTBLOGS_CATALOG_URL` 指向镜像，失败时回退公众号后台搜索。
- 数据陈旧：目录缓存 6 小时；新增账号可能暂时搜不到，可直接粘贴 Feed URL。
- 合规：仅个人阅读；不公开聚合、不商业分发。
- 回滚：删除 BestBlogs 解析分支即可；已创建的 `kind=rss` 源继续按标准 RSS 工作，无需迁移。
