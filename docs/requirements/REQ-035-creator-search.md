# REQ-035 创作者名称搜索与批量订阅

- 状态：进行中（本地验证；真实浏览器待验收）
- 优先级：P1
- 日期：2026-11-03
- 关联：PRD F-49、PDD §5.2/§6.5/§7.2/§7.6、[RSS-SOURCES](../RSS-SOURCES.md) §2.15、[DECISIONS](../DECISIONS.md) DEC-025、REQ-030/REQ-032

## 1. 背景

现有添加源以链接、ID 和 `bestblogs:{名称}` 为主。用户在多个平台发布时，通常更容易记住昵称而不是各平台链接；逐个平台搜索并复制 Feed 的成本高。需要把「名称搜索」做成候选列表，并允许一次选择多个平台账号批量订阅。

## 2. 目标

1. 新增 `POST /api/v1/sources/search`：按名称并发查询平台，返回候选和每个平台的搜索状态。
2. 第一版覆盖：
   - Apple Podcasts：官方 iTunes Search API。
   - Bluesky：公开 AppView `app.bsky.actor.searchActors`。
   - 微博：精确昵称解析（匿名访客态，实验性）。
   - BestBlogs 四目录：公众号 / 小宇宙 / X / YouTube（只覆盖目录收录项）。
3. 前端添加源弹窗增加「搜索创作者」模式；候选按平台分组，支持多选后批量创建。
4. 候选显示平台、来源、限制和「已订阅」状态；平台失败/超时/无结果必须显式展示，不能静默省略。
5. 批量创建走 `POST /api/v1/sources/batch`，避免每个源同步等待首次抓取；创建后由后台触发首次抓取。
6. 不新增表、字段、迁移或源类型；所有候选仍创建为现有 `kind=rss` / `x` / `wechat` 源。

## 3. 非目标

- 不承诺所有平台都支持按名称搜索。X 没有免费全局搜索；YouTube/小宇宙/公众号只覆盖 BestBlogs 目录；Medium、Mastodon、Threads、Instagram 不在第一版。
- 不把同名候选自动认定为同一人；不默认全选。
- 不抓取登录态内容，不绕过平台反爬或付费墙。
- 不引入新的外部账号、API Key 或付费 API。
- 不做跨平台身份合并表；第一版用文件夹承载「同一人的多个源」。

## 4. 平台能力

| 平台 | 搜索方式 | 覆盖 | 状态 |
|---|---|---|---|
| Apple Podcasts | `itunes.apple.com/search?term=...&entity=podcast` | 播客目录 | 计划/待实测 |
| Bluesky | `public.api.bsky.app/xrpc/app.bsky.actor.searchActors` | 公开账号 | 计划/待实测 |
| 微博 | `m.weibo.cn/n/{昵称}` 精确解析 | 仅精确昵称 | 实验性 |
| BestBlogs 公众号 | 公开 OPML 本地搜索 | 约 375 个收录账号 | 部分验证 |
| BestBlogs 小宇宙 | 公开 OPML 本地搜索 | 约 30 个收录账号 | 部分验证 |
| BestBlogs X | 公开 OPML 本地搜索 | 约 160 个收录账号 | 部分验证 |
| BestBlogs YouTube | 公开 OPML 本地搜索 | 约 40 个收录账号 | 部分验证 |

## 5. API

```text
POST /api/v1/sources/search
{ "query": "阑夕", "platforms": ["apple", "bluesky", "weibo", "bestblogs"] }

200 {
  "query": "阑夕",
  "candidates": [{
    "platform": "weibo",
    "kind": "rss",
    "name": "阑夕",
    "handle": "1234567890",
    "feed_url": "weibo://user/1234567890",
    "site_url": "https://m.weibo.cn/u/1234567890",
    "icon_url": "",
    "note": "微博精确昵称 · 匿名访客模式，最近最多 100 条",
    "partial": true,
    "match": "exact",
    "source": "native",
    "existing_source_id": 0
  }],
  "platforms": [
    { "platform": "apple", "label": "Apple Podcasts", "status": "empty", "count": 0 },
    { "platform": "bluesky", "label": "Bluesky", "status": "ok", "count": 2 },
    { "platform": "weibo", "label": "微博", "status": "ok", "count": 1 },
    { "platform": "bestblogs", "label": "BestBlogs 目录", "status": "ok", "count": 3 }
  ]
}
```

- `platforms` 省略时查询全部；第一版支持 `apple` / `bluesky` / `weibo` / `bestblogs`。
- 状态：`ok` / `empty` / `error` / `timeout`。
- 总超时 8 秒，单平台 5 秒；一个平台失败不影响其他平台。
- 搜索缓存 10 分钟（失败结果 1 分钟），缓存不保存 `existing_source_id`，每次按当前订阅源重算。
- 候选按 Feed URL 去重；原生平台结果优先于 BestBlogs 目录结果。

```text
POST /api/v1/sources/batch
{ "sources": [{ "kind": "rss", "name": "...", "feed_url": "...", "folder_id": null }] }

200 {
  "created": [Source],
  "errors": [{ "index": 0, "name": "...", "feed_url": "...", "code": "source_exists", "message": "..." }]
}
```

- 单次最多 20 个；重复 Feed URL 返回 `source_exists`，不重复创建。
- 创建成功后返回，不等待首次抓取；后台最多 4 个并发、每个按 20 秒上限抓取一次。`DisableInitialFetch` 测试/运维开关同样生效。

## 6. 前端

- 添加源弹窗增加分段模式：「链接 / ID」保持现有行为，「搜索创作者」走名称搜索。
- 搜索输入按 Enter 或点「搜索」触发；结果按平台分组，每组显示数量。
- 候选行显示名称、handle/来源、平台和限制；`existing_source_id` 非空时禁用并显示「已订阅」。
- 多选后可批量订阅到当前选择的文件夹；部分失败保留弹窗并显示失败摘要。
- 平台状态用标签展示：无结果、超时、失败；BestBlogs 候选注明「目录」来源。

## 7. 验收

- 后端单测：Apple/Bluesky 解析、BestBlogs 目录转换、微博精确昵称、平台失败隔离、候选去重、已订阅标记、查询校验、批量创建去重/部分失败。
- 前端：typecheck、Vitest（搜索候选分组/选择/批量创建）、生产构建。
- 真实网络：Apple Search 与 Bluesky searchActors 返回 200；微博精确昵称走现有访客态链路；BestBlogs 四目录已在前序 REQ 实测。
- 未验证：真实浏览器完整交互、微博泛搜索（未实现）、YouTube/小宇宙/公众号全网名称搜索、平台长期限流。

## 7.1 实现与证据（2026-11-03）

- 后端：
  - `server/internal/server/creator_search.go`：四平台 searcher、并发/超时、状态、去重、排序、缓存、已订阅标记。
  - `server/internal/server/source_batch.go`：批量创建、Feed URL 去重、后台首次抓取。
  - `server/internal/server/creator_search_test.go` 覆盖 Apple/Bluesky、部分失败、缓存/已订阅、BestBlogs 四目录、参数校验、批量创建。
  - `go test -count=1 ./...`、`go vet ./...`、`go test -race ./internal/server`、`CGO_ENABLED=0 go build ./cmd/rill` 通过。
  - `go test -tags live -run TestLiveCreatorSearch -v`：Apple 搜索「这集我看过」→ `feed.xyzfm.space/j7d9mlny9ea6`（match=exact）；Bluesky 搜索 `jay.bsky.team` → `Jay 🦋`（match=exact）。
- 前端：
  - `AddSourceDialog` 增加「链接 / ID」「搜索创作者」；候选按平台分组、多选、已订阅禁用、平台状态标签；批量调用 `/sources/batch`。
  - `creatorSearch.ts` 负责平台顺序/标签、刷新间隔、状态文案；`creatorSearch.test.ts`、`AddSourceDialog.test.tsx`、`mockApi.test.ts` 覆盖分组、搜索、已订阅、批量创建。
  - `npm run typecheck`、37 项 Vitest、`npm run build` 通过。
- 未重新打包 LPK。

## 8. 风险与回滚

- 同名冲突：只返回候选，不自动全部选中；显示平台与来源。
- 目录覆盖误导：平台状态和候选 note 明确标注 BestBlogs 目录，不写成平台全网搜索。
- 平台反爬：单平台超时/失败不影响其他结果；搜索按需触发并缓存，不做后台轮询。
- 回滚：移除新路由与前端模式即可；已创建的源仍是现有 `rss/x/wechat` 源，无 schema 回滚。
