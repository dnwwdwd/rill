# Bug 台账

## BUG-001 外部公众号添加后内容不同步（200013）

- 状态：已以通道收敛方式终结（2026-11-03 DEC-024/REQ-034 移除微信后台链路；微信读书为唯一通道，本地 httptest 通过，真实扫码与外部号抓取待验收）
- 发现：2026-11-03
- 影响：内置公众号链路添加的**非当前登录后台管理的公众号**（如「二哥狗腿子」）可创建源，但首次与后续抓取均失败，阅读器无内容。
- 关联：[REQ-026](requirements/REQ-026-wechat-builtin-core.md)、[REQ-028](requirements/REQ-028-wechat-search-and-x-fallback.md)、[RSS-SOURCES](RSS-SOURCES.md) §4.4、DEC-018

### 现场

- 源 `id=13`：`kind=wechat`、`feed_url=wechat://MzUxNzAzMTU4OQ==`、`name=二哥狗腿子`。
- `sources.last_status=error`，`last_error=公众号接口触发频率限制，请稍后重试`，`initial_fetched=0`，`fail_count=2`，items 为 0。
- 日志：`initial source fetch failed`、`scheduled fetch failed`，错误均为 `fetch_failed`。

### 复现与证据

同一份真实微信后台会话（2026-11-03，Rill 微服数据副本）直接请求微信接口：

| 目标 | 接口 | 结果 |
|---|---|---|
| 当前后台管理的「吃遍全国汉堡」`gh_3df5c6b01cc2` | `appmsgpublish` | `ret=0`，返回文章列表 |
| 「二哥狗腿子」`MzUxNzAzMTU4OQ==` | `appmsgpublish` | `ret=200013`，`freq control` |
| 「少数派」`MzU4Mjg3MDAyMQ==` | `appmsgpublish` | `ret=200013`，`freq control` |
| 「晚点LatePost」`MzU3Mjk1OTQ0Ng==` | `appmsgpublish` | `ret=200013`，`freq control` |
| 「二哥狗腿子」 | `searchbiz` | `ret=0`，能搜索到 fakeid |

结论：`searchbiz` 搜索外部公众号正常，但跨公众号文章列表接口从 2026-07 起对第三方/非本后台管理的账号收紧，`200013` 不再只是可等待恢复的累计频控。当前会话的 `switchacct` 只返回「吃遍全国汉堡」，因此「二哥狗腿子」不能通过内置后台链路抓取。

参考：<https://lovstudio.ai/blog/wechat-cross-account-index-ret-200013-2026>（2026-08-01，记录了同期多个独立项目的首屏 `200013` 与“搜索/单篇成功、列表失败”现象）。

### 根因

1. 产品把 `searchbiz` 能搜到等同于 `appmsgpublish` 能列出文章，误把外部账号加入 `wechat://` 源。
2. Rill 把所有 `200013` 都当作源级短期频控，只对当前源退避，未在创建或抓取前校验账号是否属于当前后台。
3. 外部账号没有可用内容源时，UI 仍显示“等待首次抓取”，用户会误以为稍后会同步。

### 修复

- 抓取前校验 `wechat://<fakeid>` 必须在当前 `session.Accounts` 中；否则快速失败，不再请求微信。
- 搜索只返回当前后台可管理的公众号；外部账号返回 `wechat_account_unmanaged` 与可操作说明。
- 文章链接解析出的账号按 fakeid/名称映射到当前后台账号；外部账号同样拒绝。
- 同一微信会话串行抓取并保持 1 秒请求间隔；真实 `200013` 触发会话级 30 分钟冷却，避免多个公众号源放大限流。
- 错误文案改为“平台列表能力受限/频控”，不再承诺稍后自动恢复。
- 新增微信读书扫码双通道：外部账号在 WeRead 已连接时可直接创建并抓取；详见 [REQ-033](requirements/REQ-033-weread-dual-channel.md)。
- 现有源不自动删除；用户可在确认后删除或等待 WeRead 通道验收。
- LPK `0.1.1` 已重新打包（sha256 `4defdb9e1d297b401ddb39de66a70b2db44d8a8bde22d81fa2e9677cd4ed84df`），未安装。

### 后续

- 2026-11-03 终结方式：用户确认后台通道「只能查看自己公众号没意义」，随 DEC-024 移除微信后台链路与 sidecar；`wechat://` 源统一走微信读书通道，未收录公众号走 BestBlogs 目录或文章链接（公开页提取 `biz`）。原「抓取前自有账号校验/会话冷却」逻辑随链路一并删除。
- 2026-11-03 尝试评估已安装的 WeRSs：源码显示其 `search`/`web`/`app`/`api` 模式仍调用 `mp.weixin.qq.com/cgi-bin/searchbiz`、`appmsgpublish`、`appmsg`，不是微信读书链路；其 `/mps/search/二哥狗腿子` 当前返回 `50001`，日志为 `200003 invalid session`，且两个 Feed 的文章数均为 0。因此切到这套 WeRSs 不能解决外部账号 `200013`。
- 真实浏览器 UI、真机部署与修复后抓取验证待做。
