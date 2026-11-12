# 微信公众号历史文章：GitHub 深度复查

> 日期：2026-11-03
> 问题：除了微信读书，是否还有办法获取他人公众号的历史文章列表？
> 关联：[`2026-11-wechat-rate-limit-alternatives.md`](2026-11-wechat-rate-limit-alternatives.md)、[`../RSS-SOURCES.md`](../RSS-SOURCES.md) §4、REQ-034 / DEC-024
> 状态：调研完成；不改变现行实现，不作为“已验证”能力。

---

## 1. 结论

**有，但都不是 Rill 可直接部署的服务端方案。**

2026-07-30 起，公众号后台的“跨公众号文章列表”接口被微信关闭。13.1k star 的 `wechat-article-exporter` 因此停止维护，README 与 issue #200 明确说明“不是改代码能绕过的问题”。这不只是频控：它让“登录自己的公众号后台，翻别人历史文章”这条服务端路线整体失效。

目前仍能拿到**他人完整历史列表**的，只有两类：

1. **微信客户端凭证**：从已登录的微信 PC 客户端截获 `uin`、`key`、`pass_ticket`、`appmsg_token`、`Cookie`，再调用：
   - 新作者列表：`GET https://mp.weixin.qq.com/mp/author?action=get_articles&author_id=...&limit=30&from_article_id=...`
   - 旧历史列表：`GET https://mp.weixin.qq.com/mp/profile_ext?action=getmsg&__biz=...&offset=...`
   这类方案需要桌面微信 + 本地 HTTPS 代理/证书注入或 UI 自动化，不能跑在 LPK 容器里。
2. **微信读书内部的分页链路**：仍是微信读书数据面，但比 Rill 当前只取 `/web/mp/articles?offset=0` 更完整。E-Ink `/mp/chapters?bookId=...&count=...&offset=...&pf=android` 支持游标分页，并可用 `mobileSync` 激活会话；参考 MIT 项目 `x554960766/wechat-mp-tools`。风险仍是 `-2041` 风控、签名/接口变动。

**没有“任意公众号 + 服务端自动 + 免费 + 完整历史 + 无风控”的五边形方案。**

---

## 2.  GitHub 候选清单（2026-11-03 查询）

| 项目 | 星数 / 最近更新 | 数据面 | 他人历史 | 前提 | 结论 |
|---|---:|---|---|---|---|
| `wechat-article/wechat-article-exporter` | 13.1k / 2026-10-31 | 公众号后台 `appmsgpublish` + `searchbiz` | 曾支持翻页 | 公众号后台扫码 | **已停止维护（2026-07-30）**，后台跨号列表接口被关闭；在线站 2026-10 下线 |
| `qiye45/wechatDownload` | 10.0k / 2026-11-01 | 微信 PC 客户端临时凭证 | **支持全历史**（README 声称） | Windows/macOS + 微信 PC，打开文章取密钥 | 活跃桌面工具；非服务端库，源码未完整公开 |
| `ltaoo/wx_channels_download` | 10.3k / 2026-11-03 | 本地 MITM + 微信客户端 | **支持**：`/mp/author` 与旧 `profile_ext` 翻页 | 管理员权限、证书、微信 PC | 最活跃的客户端抓取参考实现；不能进容器 |
| `dingaiminGIT/wechat-article-batch-downloader` | 22 / 2026-10-20 | 本地代理注入 + 微信客户端 | **支持**：`author_id` + `from_article_id` 游标，最多 2000 页 × 30 | macOS + 微信 PC | 2026-10 新链路；README 自测 210 篇；非商用许可 |
| `DevWiles/wechat-mp-archiver` | 1 / 2026-10-19 | Windows UI 自动化 + 进程内存扫描 | **支持**（滚动加载） | Windows + 微信 PC | 实验性，脆弱，仅作证据 |
| `zoro-build/wechat`（公号三刀） | 163 / 2026-10-30 | 商业客户端 + 本地密钥提取 | 同步已关闭；密钥提取可恢复 | 付费、Windows/macOS | 闭源商业工具，不能作为依赖 |
| `tmwgsicp/wechat-download-api` | 1.2k / 2026-07-27 | 公众号后台 + 代理池 / TLS 指纹 | 声称支持 | 公众号后台扫码 | 最后更新在关闭前；issue #22 报 `200013`，大概率失效；AGPL |
| `x554960766/wechat-mp-tools` | 331 / 2026-10-30 | **微信读书 E-Ink** `/mp/chapters` + `mobileSync` + 签名 | **支持分页** | 微信读书登录；多账号池 | 仍是微信读书；MIT；可参考分页/签名实现，不建议账号池 |
| `cooderl/wewe-rss` | 9.6k / 2026-03-20（已归档） | 微信读书 + 私有中转 | 曾支持 | 已死的中转 | 仅参考历史设计，不可用 |
| `ttttmr/Wechat2RSS` | 1.6k / 2026-07-31 | 闭源服务（微信读书） | 仅最新 20 条 | 公开目录或付费私有部署 | 无历史回补 |
| `rachelos/we-mp-rss` | 5.1k / 2026-10-20 | 公众号后台 + 微信读书 | 后台仅自有号；读书仅当前链路 | 扫码 | 已移除 sidecar；不是新数据面 |
| `Sheldon460/gzh-history-archive` | 1 / 2026-08-30 | 公众号后台 `appmsgpublish` | **否，仅自有号** | 自己的公众号 | 标题有误导性，不适用于他人 |
| `dumpling001/wechat-mp-exporter` | 1 / 2026-07-31 | 公众号后台 | **否，仅自有号** | 自己的公众号 | 同上 |

> 星数和更新时间来自 GitHub API；`archived` 标记与作者公告可能不同步，以 README/issue 为准。

---

## 3. 关键证据

### 3.1 后台跨号列表已关闭

- `wechat-article-exporter` issue #200（2026-07-30）：作者明确说上游核心接口被关闭，“开箱即用的一键同步公众号文章已经不复存在”，仓库转只读。
- issue #199：同步返回 `200013: freq control`；不是单纯等待可恢复的频控，多个独立项目同期出现。
- `zoro-build/wechat` README：微信已关闭公众号文章列表接口，“同步”无法再获取历史与新文章。
- `tmwgsicp/wechat-download-api` 最后更新 2026-07-27；其 issue #22 同样报告列表 `200013`。
- 结论：**重写后台接口、换代理池、加浏览器自动化都不解决入口能力关闭。**

### 3.2 仍然可用的新链路：`/mp/author`

`ltaoo/wx_channels_download`（10.3k star，2026-11-03 仍在更新）与 `dingaiminGIT` 的实现都使用：

```text
GET https://mp.weixin.qq.com/mp/author
  ?action=get_articles
  &author_id=<作者/公众号 id>
  &scene=142
  &limit=30
  &from_article_id=<上一页最后一篇的 mid>
  &appmsg_token=<客户端临时 token>
```

请求还需要：

- `Cookie`
- `Referer` 中带 `uin`、`key`、`pass_ticket`
- `author_id` 来自文章页 `window.cgiData.author_id`

`dingaiminGIT` 用 `from_article_id` 游标翻页，安全上限 2000 页 × 30 篇；README 记录实测 210 篇。`ltaoo` 另有旧 `profile_ext?action=getmsg` 的 `offset` + `hasMore` 翻页实现。

**限制**：这些凭证只存在于已登录的微信客户端页面里。Mac 应用通过本地 HTTPS 代理向微信文章页注入 JS，读取 `window.pass_ticket`、`window.appmsg_token`、`cgiData.author_id`；普通服务端抓文章页拿不到这些值（本机实测公开文章页返回空标题，且不含 token 字段）。

### 3.3 微信读书 E-Ink 分页

`x554960766/wechat-mp-tools`（MIT）的主列表顺序：

1. `POST https://i.weread.qq.com/mobileSync` 激活会话（README 称可解除 `/mp/chapters` 的 499/-2041）
2. `GET https://i.weread.qq.com/mp/chapters?bookId=...&count=...&offset=...&pf=android`
3. 失败后降级 `i.weread.qq.com/article/list`
4. 第一页再降级 `/mp/cover`

这说明微信读书不是“只能抓一篇”，而是 Rill 当前只用了 Web 列表第一页 + cover 降级。要做历史回补，优先在微信读书通道内加分页，而不是再找第三条数据面。

#### E-Ink 翻页的限流实测（2026-10-11）

`x554960766/wechat-mp-tools` issue #22 给出了一组真实账号实测（不是本项目的 Rill 实测）：

- **`synckey=0` 是必填参数**：`/mp/chapters?...&pf=android` 不带 `synckey` 会直接返回 HTTP 499 / `{"errcode":-2041}`；同一凭证加上 `&synckey=0` 立即恢复 200。**499/-2041 不一定代表翻页太快，也可能只是参数缺失。**
- **`-2014` 才是频率过高**：`/mp/chapters` 限频较严，短时间 **3~4 个请求**即触发 `-2014`，惩罚期约 **10 分钟**。
- **`count/offset` 语义偏松**：实测 `offset=0&count=50` 返回 96 条，`offset=50` 返回 262 条；不能简单按 `offset += count` 翻页，应按实际返回条数推进，并用 `reviewId` 去重。
- **响应结构**：`{"synckey":...,"clearAll":1,"data":[{reviewId, mpInfo:{title,pic_url,payType}}]}`；`mpInfo` 没有 `doc_url`，每条正文还要走 `review/single`，会进一步放大请求量。
- 该 issue 的后续评论也确认：长时间未使用的账号，即使补了 `synckey`、刷新 token，也可能仍然失败；`synckey` 不是万能修复。

对 Rill 的含义：历史回补不能“一次翻到底”。单账号串行、每轮只取 1 页、轮次间隔至少 10~30 分钟；`-2014` 冷却 10~30 分钟，`-2041` 冷却 4 小时；Web 列表和 E-Ink 分页共享同一账号的风控预算，不能并行跑。

### 3.4 只有自有号的“全量导出”

`Sheldon460/gzh-history-archive` 使用 `/cgi-bin/appmsgpublish`，`dumpling001/wechat-mp-exporter` 使用公众号后台发表记录；两者都要求**自己的公众号管理员权限**，只能导出自己的文章，不能获取他人历史。不要被“全部历史”标题误导。

---

## 4. 对 Rill 的建议

按可行性排序：

1. **微信读书 E-Ink 历史回补（推荐，服务端）**
   - 在现有 `wechat://` 源上增加“历史回补”模式；
   - 用 `/mp/chapters` 的 `offset` 分页，持久化游标；
   - 先做只读验证：2~3 个号、每页少量、记录 `-2041` 与签名行为；
   - 限速、单账号串行、冷却；不做验证码绕过；不引入账号池。
   - 工程量：中高（签名算法移植 + 分页 + 游标 + UI）；风险：接口/签名变动、风控。

2. **桌面导出 + Rill 导入（最快落地，零服务端风控）**
   - 用 `qiye45/wechatDownload`（Windows/macOS）或 `dingaiminGIT`（macOS）导出 CSV/JSON/Markdown；
   - Rill 增加历史列表导入（标题/URL/发布时间），正文按需再抓；
   - 优点：今天就能拿到完整历史，不碰微信凭证，不需要新数据面；
   - 缺点：手动、非实时、需要桌面微信。

3. **伴生工具（自动，但工程量大）**
   - 在用户 PC 上运行本地 helper，复用 `ltaoo`/`dingaiminGIT` 的凭证捕获思路；
   - helper 只把文章元数据推给 Rill，凭证不落服务端；
   - 需要新安装包、证书信任、局域网/鉴权边界；不建议本轮做。

4. **不推荐**
   - 手动粘贴微信 `Cookie/pass_ticket/key`：短命、敏感、易失效；
   - 重写公众号后台接口：入口已关闭；
   - 搜狗/代理池：不稳定且不解决入口；
   - 商业数据 API：可作备选，但需付费/合同，未验证覆盖与合规。

---

## 5. 未验证项

- 本机没有 Rill 内真实微信读书 Cookie，也没有微信客户端凭证；上述“可翻历史”来自项目代码/README，不是 Rill 端到端实测。
- `/mp/chapters` 的签名、`mobileSync` 激活、`-2041` 冷却在 Rill 环境中的实际表现未验证。
- `/mp/author` 新链路仅由第三方项目自测；微信可能随时调整。
- 各桌面工具的全历史能力、速率、封号风险未由本项目复现。

## 6. 链接

- <https://github.com/wechat-article/wechat-article-exporter/issues/200>
- <https://github.com/wechat-article/wechat-article-exporter/issues/199>
- <https://github.com/ltaoo/wx_channels_download/blob/main/pkg/scraper/wxmp/client.go>
- <https://github.com/dingaiminGIT/wechat-article-batch-downloader/blob/main/mp_article_downloader_src/internal/officialaccount/client.go>
- <https://github.com/qiye45/wechatDownload>
- <https://github.com/DevWiles/wechat-mp-archiver>
- <https://github.com/zoro-build/wechat>
- <https://github.com/tmwgsicp/wechat-download-api>
- <https://github.com/x554960766/wechat-mp-tools/blob/main/backend/articles.py>
- <https://github.com/x554960766/wechat-mp-tools/blob/main/backend/weread_signer.py>
- <https://github.com/cooderl/wewe-rss>
- <https://github.com/ttttmr/Wechat2RSS>
