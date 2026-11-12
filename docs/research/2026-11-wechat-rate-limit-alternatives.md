# 微信公众号限流与替代通道调研

> 日期：2026-11-03
> 问题：公众号采集频繁遇到 `ret=200013`，是否存在比 `mp.weixin.qq.com` 后台接口更稳定的替代方案？
> 关联：[`../RSS-SOURCES.md`](../RSS-SOURCES.md) §4、[`../requirements/REQ-026-wechat-builtin-core.md`](../requirements/REQ-026-wechat-builtin-core.md)、[`../DECISIONS.md`](../DECISIONS.md) DEC-018
> 状态：调研完成，待用户选型；不改变现行产品策略，不作为“已验证”能力。

---

## 1. 结论

**没有“任意公众号 + 完全免费 + 无风控 + 长期稳定”的方案。** 但当前问题不一定只是 Rill 的抓取频率，存在两层原因：

1. **平台层收紧**：2026-07-30 前后，多个独立项目同时报告公众号后台“跨公众号文章列表”在第一页、零结果时返回 `200013`。同一会话下 `searchbiz` 和单篇文章仍可用。这更像接口级能力收紧，而不是单纯累计频控；只调大退避时间不一定恢复。
2. **Rill 自身放大**：同一微信会话可被最多 4 个源并发使用；`200013` 只退避当前源，其他公众号源继续请求；手动刷新和添加源首抓还能绕过调度并发限制。当前实现会放大限流，并延长冷却。

**建议按三层处理：**

- **P0 先修自伤**：微信会话级串行、全局冷却、请求间隔、错误分级。成本最低，必须先做。
- **P1 BestBlogs 目录优先**：公众号裸名称先查 BestBlogs 公开 OPML；命中直接建标准 RSS，不要求公众号后台登录。只覆盖其收录的 375 个号、约 10 条近期文章，无历史回补。
- **P2 未收录/历史需求**：微信读书通道（Web 列表 + 正文、E-Ink 降级）或付费 Wechat2RSS 私有版；公众号后台扫码保留为最终回退。
- **不建议**：搜狗批量抓取作为主路径、WeWe RSS（已归档且中转已死）、单纯上代理池/浏览器自动化、把 `wechat-download-api` 当作“不同数据面”。

---

## 2. 根因证据

### 2.1 后台列表能力收紧

2026-07-30 至 07-31，至少三个独立项目出现同类报告：

- `wechat-article-exporter` #199：登录后同步，返回 `200013: freq control`。
- `wechat-download-api` #22：公众号搜索、单篇文章成功，文章列表失败 `ret=200013`。
- `we-mp-rss` #440：多个 Feed 在第一页 `stop at 0`，任务显示六个失败。

这些报告的共同点：**不是跑了几小时/几十页后才限流，而是第一页、零结果就被拒绝**；同一会话的搜索和单篇正文仍可用。因此 `200013` 至少有两种语义：

- 累计频控：等待/退避可能恢复；
- 接口级能力限制：等待不一定恢复，需要换数据面。

Rill 当前把所有 `200013` 都当作可退避的频控，无法区分这两种情况。

### 2.2 Rill 侧的放大点

| 问题 | 位置 | 影响 |
|---|---|---|
| 调度器对到期源最多 4 并发，微信会话无全局锁 | [`scheduler.go:53`](../../server/internal/server/scheduler.go#L53) | 4 个公众号源可同时用同一 token 请求 |
| 手动刷新、添加源首抓不占调度信号量 | [`handlers_sources.go:292`](../../server/internal/server/handlers_sources.go#L292)、[`wechat_builtin.go:357`](../../server/internal/server/wechat_builtin.go#L357) | 可与调度抓取叠加 |
| `200013` 只更新当前源的 `fail_count/next_fetch_at` | [`fetch_service.go:230`](../../server/internal/server/fetch_service.go#L230) | 其他公众号源继续请求，冷却无法生效 |
| 首次退避仅 1 分钟，最高 1 小时 | [`fetch_service.go:274`](../../server/internal/server/fetch_service.go#L274) | 对持续能力限制过短，容易反复撞墙 |
| 每次固定拉第一页 5 条，再抓最多 20 篇正文 | [`wechat_builtin.go:397`](../../server/internal/server/wechat_builtin.go#L397)、[`wechat_builtin.go:444`](../../server/internal/server/wechat_builtin.go#L444) | 单源最多约 21 次请求；跨源叠加后形成突发 |
| 正文失败会丢弃已抓条目，下轮重抓 | [`wechat_builtin.go:444`](../../server/internal/server/wechat_builtin.go#L444) | 重复请求，进一步放大 |
| 公众号源允许 60 秒刷新 | [`source_store.go:241`](../../server/internal/server/source_store.go#L241) | 无类型下限，容易自伤 |

**结论：** 即使换数据面，P0 的会话级限速和熔断仍然必要；否则换到微信读书后也会以同样方式触发它的 `-2041/-2042`。

---

## 3. 候选方案对比

| 方案 | 数据面 | 任意公众号 | 历史回补 | 全文 | 鉴权 | 成本 | 主要风险 | 证据等级 |
|---|---|---|---|---|---|---|---|---|
| 公众号后台（现行） | `mp.weixin.qq.com/cgi-bin/*` | 是 | 可翻页 | 文章页抓取 | 公众号后台扫码 | 免费 | 2026-07 起列表能力收紧；`200013`；文章页验证码 | 已实现，真实源曾通过；近期限流 |
| 微信读书 Web | `weread.qq.com/web/mp/*` | 仅已收录/可加入书架的号 | 可翻页；部分号只能 cover 最新一篇 | `/web/mp/content` 代理全文，失败回源 mp | 微信读书扫码 Cookie | 免费 | `-2041` 验证码、`-2012` 过期、封号、收录延迟 | Go/TS 开源实现 + 社区实测；Rill 未认证实测 |
| 微信读书 E-Ink API | `i.weread.qq.com/mp/chapters` | 仅已收录/可加入书架的号 | 支持游标/翻页 | 需 Web 代理、mp 或 shareChapter | E-Ink 登录/refresh token；可能需 mobileSync 激活 | 免费 | 协议非公开、需签名/激活；正文仍可能回源 mp | `weread-omni` 源码 + 社区实现；未实测 |
| Wechat2RSS 私有部署 | 闭源服务，内部走微信读书 | 是（群发且被收录） | **只抓最新 20 篇，不抓历史** | 有全文/媒体代理 | 服务内扫码登录 WeChat Reading | 15 元/月或 150 元/年 | 闭源；仅个人学习研究，禁止商用/内容分发；依赖第三方授权 | 官方文档/定价/协议；公共 Feed 2026-11-02 仍更新 |
| Wechat2RSS 公共服务 | 第三方托管 | 仅已收录 300+ 号 | 20 条/源 | 有 | 无 | 免费 | 不能任意添加；无 SLA | 2026-10-31 抽样 6 源 200 |
| BestBlogs 托管公众号 Feed | 第三方托管的 Wechat2RSS 子域 | 仅 BestBlogs 收录的 375 个号 | 约 10 条/源，无历史 | 有，`content:encoded` | 无 | 免费 | 不能任意添加；无 SLA；第三方依赖 | 2026-11-03 抽查 5 源均 200/10 条 |
| 搜狗微信公开索引 | `weixin.sogou.com` | 按关键词发现 | 有限/不完整 | 需再抓正文 | 无 | 免费 | 反爬强；本机 `/link` 直接进 `/antispider/`；不可作为主路径 | 本机 2026-11-03 实测被拦 |
| 官方 `freepublish/batchget` | `api.weixin.qq.com` | **否，仅自有号** | 仅发布内容 | 有 | 公众号 access_token | 免费 | 不覆盖他人公众号；仅部分发布内容 | 官方文档 |
| 第三方数据 API（如极致了） | 服务商 API | 是 | 可查历史 | 可含正文 | API Key | 按调用计费 | 商业条款、延迟、数据合规；非官方 | 服务商文档/报价文章 |
| 浏览器自动化/代理池 | `mp.weixin.qq.com` | 是 | 取决于入口 | 可 | 扫码/代理 | 代理成本 | 军备竞赛、重镜像、账号风险；不解决入口能力收紧 | 社区实践，稳定性不足 |
| WeWe RSS | 原闭源中转 | 是 | 有 | 正文回源 | 微信读书 | 免费 | 仓库已归档；中转 `weread.111965.xyz` 全端点 502；不可自救 | 源码考古/实测 |
| wechat-download-api | 微信后台 + 代理池 | 是 | 有 | 有 | 公众号扫码 | 自部署/托管 | 仍是同一后台入口；AGPL；代理与封控成本 | 开源仓库/README |

---

## 4. 微信读书通道细节

### 4.1 两条可组合的链路

微信读书不是单一接口，存在两条可组合的链路：

```text
Web 链路（实现简单）
微信读书扫码登录
  → bookId = MP_WXS_<base64decode(__biz)>
  → GET weread.qq.com/web/mp/articles?bookId=...&offset=...
  → GET weread.qq.com/web/mp/content?reviewId=...
  → 从代理页提取正文、og:url canonical 原文链接和图集

E-Ink 链路（列表更接近 App 协议）
  → GET i.weread.qq.com/mp/chapters?bookId=...&count=...&offset=...
  → 正文仍需 /review/single、/book/shareChapter 或 Web /web/mp/content
```

与公众号后台列表相比，两条链路都绕开了 2026-07 的跨公众号列表收紧；与 `wewe-rss` 相比，发现和正文链路可以放在开源代码里，不依赖作者私有中转。代价是微信读书自己的风控：`-2041` 验证码、`-2012` 登录失效、`-2010` 无登录态、`-2042` 未收录。

**实现建议：不要只赌一条。** Web 列表可用时优先 Web；遇到 `-2041` 后进入账号冷却，冷却期只用 E-Ink `/mp/chapters` 或 `/api/mp/cover` 做低频探针；正文优先 `/web/mp/content`，失败再退 `mp.weixin.qq.com` 或 `/book/shareChapter`。`MyWechatRss` 的 Go 实现就是这种多级降级结构，可作为工程参考；但它把登录/续期放在私有 upstream，不能直接当作完整自托管依赖。

### 4.2 已确认的接口行为

本机 2026-11-03 无凭据探测（仅证明端点存在，不代表已登录可用）：

| 请求 | 状态 | 关键响应 |
|---|---|---|
| `GET https://weread.qq.com/api/auth/getLoginUid` | 200 | `{"uid":"..."}`，可用于发起扫码 |
| `GET /web/mp/articles?bookId=MP_WXS_...&offset=0` | 200 | `{"errCode":-2010,"errMsg":"用户不存在"}` |
| `GET /api/mp/cover?bookId=MP_WXS_...` | 401 | `LOGIN ERR` + `errcode:-2010` |
| `GET /web/mp/content?reviewId=...` | 200 | `{"errCode":-2010,"errMsg":"用户不存在"}` |
| `GET /web/shelf/bookIds` | 200 | `{"errCode":-2010,"errMsg":"用户不存在"}`，可作登录探针 |
| `GET i.weread.qq.com/mp/chapters` | 401 | `-2010`，端点在线但需登录 |
| `GET i.weread.qq.com/review/single` | 401 | `-2010`，端点在线但需登录 |

上游/社区证据：

- `we-mp-rss` 2026-09-04 提交把 `/web/mp/articles` 改为主路径，`/api/mp/cover` 作为兜底；此前 8 月的“列表已废弃”结论来自旧 Cookie 下的 `-2041` 误判。
- `MyWechatRss`（Go）把 `/web/mp/articles` 作为主列表、`/web/mp/content` 作为正文首选，并在 `-2041` 后做 4 小时账号冷却、只放一个探针订阅；它同时保留 `/book/articles`、`/review/single`、`/book/shareChapter` 作为降级。这直接证明 Web 列表不是“永久废弃”，而是会被验证码/风控拦截。
- `steptian/weread-mp` README（2026-10）：`/web/mp/articles` 会返回 `-2041` 并弹腾讯验证码；过掉后可用，但会重新武装；同一 session 的 `/web/shelf/sync` 不受影响。
- 该 README 同时确认：正文走 `/web/mp/content`，**不从 `mp.weixin.qq.com` 直接拿**；代理页 HTML 含完整正文、`og:url` canonical 链接和图集。`MyWechatRss` 也把它作为正文首选，失败才回源 mp。
- `weread-omni` 走 E-Ink `/mp/chapters` + `/review/single`，但正文会回源 `mp.weixin.qq.com`；它证明 E-Ink 列表可用，不证明 E-Ink 能独立拿全文。
- 2026-10 起 `mpInfo.originalId` 可能变成 75 字符不透明 ID，不能再直接拼 `mp.weixin.qq.com/s/{id}`；必须从 `/web/mp/content` 的 `og:url` 或 `doc_url` 取真实链接，并去掉 `chksm`。
- 社区共识：个人轻量（十几个号、低频）可用；上百号需要账号池 + 严格限频；微信读书收录可能滞后数小时到半个月。

### 4.3 不能忽略的限制

- `-2012`：登录超时，需重新扫码/复制 Cookie。
- `-2010`：无登录态/用户不存在。
- `-2041`：风控验证，需要在官方客户端完成腾讯验证码；不是扫码能解决，也不应自动绕过。
- `-2042`：文章未被微信读书收录。
- 列表接口可能只返回很少文章，甚至不可用；`/api/mp/cover` 只能兜底最新一篇。
- 微信读书对公众号的收录有延迟，不能承诺实时。
- 账号池、代理、频繁切换都会增加封号风险；不能把它包装成“无限稳定”。

### 4.4 与本项目现有模型的兼容性

- 现有 `sources.kind` 只有 `rss/x/wechat`，无需新增枚举；可用 `FeedURL` 前缀区分通道，例如 `weread://MP_WXS_...`。
- `bookId` 可由现有 `fakeid` 推导：`MP_WXS_ + base64decode(fakeid)`；已有 `wechat://<fakeid>` 源可在读取时映射，不必迁移数据。
- 微信读书会话可继续用 `settings` 的 AES-GCM 加密存储，不新增表。
- 正文/封面仍走既有 `/media/` 代理与缓存。
- 现有 `wechat://` 源可保留后台通道作为降级；新源默认走微信读书，或在设置中二选一。

---

## 5. Wechat2RSS 的适用边界

- 私有部署：Docker，`ttttmr/wechat2rss`，订阅制 15 元/月或 150 元/年。
- 支持任意公众号（以群发且被微信读书收录为前提）、全文、图片/音视频代理、公众号迁移跟随、异常通知。
- **只抓最新 20 篇，不抓历史文章**；只收录群发消息，非群发不收录。
- 服务通过微信读书获取公众号信息；需要微信读书授权和扫码登录。
- 用户协议：仅个人学习研究，禁止商用、禁止内容分发、禁止公开服务、禁止转售/共享授权。
- 风控处理：等待时间 15 分钟起，指数翻倍，最高 6 小时；支持手动解除风控。
- 公共 Feed 2026-11-02 仍有更新（抽查首条为全文，约 3.5k 字符），可作为服务存活的旁证；不代表私有部署或任意公众号均可用。
- 对 Rill 的接入成本最低：把其 RSS 地址作为普通 `rss` 源，不需要新增适配器；但它是外部闭源依赖，且不能回补历史。

### 5.1 BestBlogs 托管 Feed（2026-11-03 核实）

BestBlogs 在 `wechat2rss.bestblogs.dev` 托管了一批公众号 RSS，并公开了 375 个源的 OPML：

- 样例 `ff621c3e98d6ae6fceb3397e57441ffc6ea3c17f.xml`：HTTP 200、`application/xml`、489,418 字节、10 条，最新 2026-11-02 08:19 +0800，`content:encoded` 约 21k–58k 字符。
- 另抽查 4 个 Feed：均 200、各 10 条，最新到 2026-11-02/11-01。
- OPML：<https://github.com/ginobefun/BestBlogs/blob/main/opml/bestblogs_wechat2rss_opml_all.opml>，含 375 个 `outline`。
- Rill 的 `cmd/fetchtest` 对样例返回 200；尚未在 Rill 内完成加源、入库、阅读的整链验收。

**定位：** 这是「不想自建、不想登录公众号后台」时的零代码方案，适合 BestBlogs 已收录的账号。它不能任意添加公众号，每个 Feed 只保留近期约 10 条，无历史回补，且依赖 BestBlogs 的持续维护。Rill 侧按普通 RSS 接入，不新增 `kind` 或抓取器。

### 5.2 多平台优先级（2026-11-03 评估）

BestBlogs 不是「一个 RSSHub 代理所有平台」，而是按来源类型混用不同上游：

| 类型 | BestBlogs 提供的 Feed | 实测 | 建议 |
|---|---|---|---|
| 微信公众号 | 375 个 `wechat2rss.bestblogs.dev/feed/{40hex}.xml` | 5 个样本 200、各 10 条、含 `content:encoded` | **优先**；未收录回退公众号后台 |
| 小宇宙 | 30 个 `rsshub.bestblogs.dev/xiaoyuzhou/podcast/{pid}` | 样本 200，标题/单集正常 | 官方 `feed.xyzfm.space` 优先；BestBlogs 作无官方 Feed 时降级 |
| X | 160 个 `api.xgo.ing/rss/user/{id}` | OpenAI 样本 200、50 条 | FxTwitter 主源优先；BestBlogs 只作已收录账号的备用源 |
| YouTube | 40 个官方 `youtube.com/feeds/videos.xml` | 本机直连超时，未拿到响应 | 这不是 BestBlogs 代理；官方 RSS 优先，网络问题另行处理 |
| 普通 RSS / 文章 | 多为原始站点 Feed | 抽样与普通 RSS 相同 | 直接订阅原站；不需要经 BestBlogs |

结论：**公众号适合 BestBlogs 优先；播客/X 只适合做降级；YouTube/普通 RSS 应继续直连。** 不要做一个「所有平台都先走 BestBlogs」的全局开关。

---

## 6. 推荐路线

### P0：先修 Rill 自伤（建议立即做）

1. 微信会话级互斥：同一时间只允许一个公众号请求链路；手动刷新、首抓也走同一把锁。
2. 全局冷却：任一源收到 `200013`，整个微信会话进入 30–60 分钟冷却；冷却期内其他源快速失败，不发请求。
3. 请求间隔：列表→首篇、篇间、跨源都加 1–2 秒随机间隔；公众号源刷新下限提高到 30 分钟，批量添加时错峰。
4. 错误分级：`200013` 不再只按 1 分钟退避；`200003` 直接标记登录态失效；`200013` 文案改为“后台列表能力受限或频控”，不承诺“稍后自动恢复”。
5. 减少重复：正文失败时保留列表条目与摘要占位，不整批丢弃；已入库正文不重复抓。

这些改动不新增 schema，能显著降低自伤，但不能突破平台级配额。

### P1：BestBlogs 托管公众号 Feed（用户选择，2026-11-03）

1. 公众号裸名称先查 BestBlogs 公开 OPML（6h 缓存、可配镜像）；命中直接创建 `kind=rss`，不要求公众号后台登录。
2. `wechat2rss.bestblogs.dev/feed/{40hex}.xml` 直接识别；未命中/目录不可用时回退现有公众号后台搜索。
3. 只覆盖 BestBlogs 收录的 375 个号、每个 Feed 约 10 条近期文章，无历史回补；UI 标注第三方与限制。
4. 已实现目录解析与前端候选；真实浏览器整链、375 个 Feed 全量可用率和长期稳定性待验收。

### P2：未收录账号/历史需求

1. **微信读书通道**：若 BestBlogs 未收录且用户接受扫码/验证码，做一次真实账号 spike 后实现 Web 列表 + 正文、E-Ink 降级；细节见 §4。
2. **Wechat2RSS 私有版**：若用户接受付费闭源且只需近期文章，部署私有版并接入 RSS；仅限个人学习研究。
3. **公众号后台**：保留现有扫码通道作为最终回退，但先修 P0 的会话级串行/冷却。

### 不推荐

- 搜狗公开索引作为主路径：本机 2026-11-03 搜索页 200，但跟随 `/link` 立即进 `/antispider/`；不适合无人值守。
- WeWe RSS 及依赖 `weread.111965.xyz` 的部署：中转已 502，仓库已归档。
- 单纯代理池/浏览器自动化：不解决后台列表能力收紧，增加账号与运维风险。
- 把 `wechat-download-api` 当作替代数据面：它仍使用公众号后台入口，只是加了 TLS/代理/限频，且 AGPL。

---

## 7. 待确认

1. 用户能接受哪种登录身份：公众号后台扫码、微信读书扫码，还是付费第三方？
2. 是否需要历史回补？如果只需要“新文章不断更”，微信读书 cover 兜底也能工作；如果要完整历史，必须列表接口可用或有后台通道。
3. 公众号数量与刷新频率是多少？十几个号与上百个号的设计不同。
4. 是否接受第三方闭源服务与 15 元/月？是否仅个人使用？
5. 是否愿意先做一次真实微信读书账号的 spike，再决定是否改代码？

---

## 8. 参考

- 2026-07 平台变化与 `200013`：<https://lovstudio.ai/blog/wechat-cross-account-index-ret-200013-2026>
- `we-mp-rss` 微信读书通道：<https://github.com/rachelos/we-mp-rss/blob/main/docs/weread-mp.md>
- `we-mp-rss` #442 接口实测：<https://github.com/rachelos/we-mp-rss/issues/442>
- `MyWechatRss`（Go 多级降级实现，登录/续期依赖私有 upstream）：<https://github.com/cry0404/MyWechatRss>
- `wechat-mp-tools`（E-Ink `/mp/chapters` 与 mobileSync 激活参考）：<https://github.com/x554960766/wechat-mp-tools>
- `steptian/weread-mp`：<https://github.com/steptian/weread-mp>
- `Pengyf04/weread-mp-fetcher`：<https://github.com/Pengyf04/weread-mp-fetcher>
- `weread-omni`：<https://github.com/teng-lin/weread-omni>
- Wechat2RSS 私有部署：<https://wechat2rss.xlab.app/deploy/>
- Wechat2RSS 用户协议：<https://wechat2rss.xlab.app/deploy/agreement.html>
- Wechat2RSS Q&A：<https://wechat2rss.xlab.app/deploy/qa.html>
- 微信官方接口限制：<https://developers.weixin.qq.com/doc/subscription/guide/dev/api/limit.html>
- 微信官方发布能力：<https://developers.weixin.qq.com/doc/subscription/guide/product/publish.html>
