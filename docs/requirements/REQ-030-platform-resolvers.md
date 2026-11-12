# REQ-030 平台链接/ID 自动解析与 Apple Podcasts、小宇宙、微博订阅

- 状态：已完成核心（Apple/小宇宙/微博真实源端到端；Medium/Mastodon 仅 Feed 实测；真实浏览器 UI 待验收）
- 优先级：P1
- 日期：2026-11-02
- 关联：PRD F-01/F-23/F-48、PDD §5.2/§7.2/§7.5、RSS-SOURCES §2.10–2.13、DEC-020

## 1. 目标

把「添加源」从只识别 RSS/X/Bluesky/公众号，扩展为统一的平台解析入口：

1. 粘贴 Apple Podcasts 链接或 show ID → 用 iTunes Lookup API 解析 `feedUrl`，按标准 RSS 订阅。
2. 粘贴小宇宙节目/单集链接或 24 位 podcast ID → 优先解析 `feed.xyzfm.space` 官方分发 Feed；没有 Feed 时用页面 SSR 最近 15 条降级。
3. 粘贴微博主页/昵称/UID → 用匿名访客 Cookie 读取最近最多 100 条微博；不承诺历史回补。
4. Medium / Mastodon 主页 URL 归一化为官方 RSS。
5. 所有解析先返回候选，前端展示平台、名称、来源和限制，用户确认后再创建源。
6. 对标 Folo：不把 RSSHub 路由数量包装成原生支持；只接入有公开证据、可降级的源。

## 2. 非目标

- 不接入 Instagram、小红书、知乎用户动态、Bilibili 动态等需要登录态/Playwright 且当前不稳定的源。
- 不做微博登录 Cookie 管理；本批只用匿名访客态，不读取受限内容。
- 不做小宇宙全量历史回补；页面 SSR 只给最近 15 条。
- 不新增数据库表/字段/迁移；不改 `sources.kind` 的 CHECK 约束，小宇宙/微博用 `kind=rss` + 自定义 scheme。
- 不改变 Feed 输出、OPML、备份/恢复语义。

## 3. 方案

### 3.1 统一解析 API

`POST /api/v1/sources/resolve`

```json
{ "input": "https://podcasts.apple.com/us/podcast/darknet-diaries/id1296350485" }
```

```json
{
  "candidates": [
    {
      "platform": "apple",
      "kind": "rss",
      "name": "Darknet Diaries",
      "feed_url": "https://podcast.darknetdiaries.com",
      "site_url": "https://podcasts.apple.com/us/podcast/darknet-diaries/id1296350485",
      "icon_url": "https://...",
      "note": "Apple Podcasts 目录解析"
    }
  ]
}
```

- 未识别为平台输入的普通 URL 返回 `{"candidates":[]}`，前端继续走现有 HTML Feed 发现。
- 识别为平台但解析失败时返回 502/404 和明确错误，前端不再把原页面当 RSS 兜底。
- 多候选（例如纯数字 ID 同时命中 Apple / 微博）由前端列表选择。

### 3.2 Apple Podcasts

- 解析 `podcasts.apple.com/.../id{showID}`、`apple:{showID}`、纯数字。
- `GET https://itunes.apple.com/lookup?id={showID}&entity=podcast`。
- 取 `feedUrl`、`collectionName`、`artworkUrl600`、`collectionViewUrl`。
- 无 `feedUrl` 时明确报错；不抓 Apple 页面。
- 创建 `kind=rss` 源，后续完全走标准 RSS。

### 3.3 小宇宙

- 解析 `xiaoyuzhoufm.com/podcast/{pid}`、`/episode/{eid}`、`feed.xyzfm.space/{token}`、24 位 hex。
- 有 `feed.xyzfm.space` 时直接创建 `kind=rss`。
- 没有 Feed 时：
  1. 抓节目页，解析 `__NEXT_DATA__` 的 `pageProps.podcast`；
  2. 用标题查 Apple Search API（`country=cn`），只接受 `feed.xyzfm.space` 且回链包含 `/podcast/{pid}` 的结果；
  3. 仍找不到则创建 `kind=rss` + `feed_url=xiaoyuzhou://podcast/{pid}`（内部适配器），抓取最近 15 条；
  4. 源备注写「最近 15 条，非全量」。
- 单集链接通过页面数据反查 `pid`，最终订阅整个播客。

### 3.4 微博

- 解析 `weibo.com/u/{uid}`、`/n/{name}`、`m.weibo.cn/u/{uid}`、帖子链接、`weibo:{uid}`、5–10 位数字。
- 匿名访客态：
  1. `POST https://visitor.passport.weibo.cn/visitor/genvisitor2` 取 `SUB/SUBP`；
  2. 带 Cookie 读取 `m.weibo.cn/api/container/getIndex?...&count=100`；
  3. 昵称通过 `m.weibo.cn/n/{name}` 的 302 `Location` 解析 UID。
- 创建 `kind=rss` + `feed_url=weibo://user/{uid}`（内部适配器）。
- 只取最近最多 100 条，`page=2` 不匿名翻页；源备注和添加确认必须显示「最近最多 100 条」。
- 访客 Cookie 只存内存，1 小时 TTL；432/`ok:-100` 时清空并重试一次。
- 图片走既有媒体代理；视频签名 URL 不长期缓存；长文列表里只显示摘要和原文链接。

### 3.5 Medium / Mastodon

- `medium.com/@user` → `https://medium.com/feed/@user`。
- `https://{instance}/@{user}` → `https://{instance}/@{user}.rss`。
- 只做 URL 归一化，创建 `kind=rss`；实例关闭 RSS 时由源诊断显示错误。

### 3.6 前端

- 添加源输入框先调用 `/sources/resolve`；单候选直接创建，多候选展示平台/名称/限制列表。
- 现有 X / Bluesky / 公众号逻辑保留；平台解析放在这些逻辑之后、HTML Feed 发现之前。
- 候选列表显示「Apple Podcasts」「小宇宙」「微博」「Medium」「Mastodon」标签；小宇宙/微博显示限制说明。
- 订阅源设置图标增加小宇宙/微博类型标识；不新增筛选行。

## 4. 验收

- [x] Apple 链接、`apple:ID`、纯 ID → 正确 `feedUrl`，创建后能抓到音频条目。
- [x] 小宇宙节目链接：有 `feed.xyzfm.space` 时创建 RSS；无 Feed 时创建 `xiaoyuzhou://` 源并抓到最近 15 条。
- [x] 微博 `u/{uid}`、`n/{name}`、纯 UID → 正确昵称/头像，抓到最近最多 100 条，分页限制提示存在。
- [ ] Medium/Mastodon 链接 → 归一化 RSS（Feed 2026-11-02 实测；未单独建源端到端）。
- [x] 普通网站 URL 仍走现有发现；未知平台不误判。
- [x] 现有 X/Bluesky/公众号/OPML/备份测试不回归。
- [x] 文档同步 RSS-SOURCES/PRD/PDD/PROGRESS，未验证项如实标注。

## 5. 风险

- 小宇宙页面 SSR 和 `feed.xyzfm.space` 非公开文档；可能改版或 token 失效。
- 微博匿名访客态是非文档化接口；432/风控、100 条上限、账号差异都不可控。
- Apple Search API 约 20 次/分钟；只在添加源时调用，不进入定时抓取。
- 小宇宙/微博用 `rss` + 自定义 scheme，OPML 导出这些源时其他阅读器无法直接识别；UI 类型标签按 scheme 推断。
- 不引入用户微博 Cookie；未来若要做，必须单独评审加密存储、授权和撤销。
