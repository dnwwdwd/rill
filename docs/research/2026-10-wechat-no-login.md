# 微信公众号「无登录 RSS」调研

> 日期：2026-10-31
> 问题：werss 是否必须登录微信公众号？有没有更好的替代方案？能否做到完全不登录？
> 相关文档：[`../RSS-SOURCES.md`](../RSS-SOURCES.md)
>
> **2026-11-03 复核：** 本文第 16 条与 §3.2 中“微信读书只能取最新一篇”的结论已过时。`/api/mp/cover` 确实只返回最新一篇，但 `/web/mp/articles` 在 2026-09 后被多个活跃实现作为列表主路径（会受 `-2041` 风控）；E-Ink `/mp/chapters` 也提供分页列表。限流替代方案与证据见 [`2026-11-wechat-rate-limit-alternatives.md`](2026-11-wechat-rate-limit-alternatives.md)。

---

## 1. 结论摘要

1. **werss 必须登录。** 三种采集模式（web / api / app）都依赖微信公众号后台的登录态；session 过期后需要重新扫码。README 和采集文档明确写了这一点。
2. **werss 上游没有新代码。** `git fetch` 后本地 HEAD 与 `origin/main` 一致，最后提交是 2026-04-28，距今约 6 个月。
3. **有比 werss 维护更好的替代品：`we-mp-rss`。** 最后提交 2026-10-21，5.1k stars，默认 SQLite，支持公众号扫码 + 微信读书扫码 + `weread_mp` 模式，RSS/Atom/OPML 完整。但**仍然需要登录**。
4. **曾经最好的微信读书方案 `WeWe RSS` 已归档**（2026-05-11），不建议新项目采用。
5. **任意公众号「完全不登录」的稳定方案不存在。** 无登录只能覆盖三类受限场景：公开专辑、公开主页、Wechat2RSS 精选源。搜狗搜索和文章直链都会被反爬拦截。
6. **最终决策（2026-10-31）**：公众号**强制扫码登录**，唯一依赖 `we-mp-rss`，固定使用「公众号后台扫码」路径（可拉历史）。不采用公开专辑/公开主页/Wechat2RSS 等无登录入口；不依赖 werss / WeWe RSS。微信读书路径只取最新一篇，不满足历史需求，放弃。

---

## 2. werss 更新检查

### 2.1 仓库状态

```bash
cd /path/to/werss
git fetch origin --prune
git rev-list --count HEAD..origin/main   # 0
git log -1 --format='%ci %s' origin/main
```

结果：

- 本地 `HEAD` 与 `origin/main` 一致，**没有落后提交**。
- 最后上游提交：`2026-04-28 14:47:52 +0800 refactor: update configuration and enhance article processing features`。
- 只有 `origin/main` 一个远端分支，没有新 tag。
- 本地有 47 个未提交变更（LPK 适配等），**不能直接 `git pull` 覆盖**。

### 2.2 登录依赖

来自项目文档 [`../../../werss/docs/features/article-collection.md`](../../../werss/docs/features/article-collection.md)：

- `web` 模式：Playwright 浏览器驱动，`driver/wxarticle.py`。
- `api` 模式：直接调用微信接口，依赖 Session + Token。
- `app` 模式：未完全实现。
- 三种模式都需要微信公众号后台登录态。
- 文档明确写："微信登录态过期会导致 api 模式失败，需重新扫码登录。"
- 开发日志写："由于微信限制每天还是要重新扫码登录。"

### 2.3 结论

werss **不满足**"不登录微信公众号即可查看/订阅"的目标。它的 RSS 输出端点本身可以是公开的，但数据采集必须登录。

---

## 3. 替代方案调研

### 3.1 登录型方案对比

| 项目 | 最后提交 | Stars | 登录方式 | 数据库 | 结论 |
|------|----------|------:|----------|--------|------|
| `rachelos/we-mp-rss` | 2026-10-21 | 5.1k | 公众号扫码 / 微信读书扫码 | 默认 SQLite | **推荐**，维护活跃 |
| `letuswerss/werss` | 2026-04-28 | — | 公众号扫码 | SQLite | 可用但停滞；你本地已有适配 |
| `cooderl/wewe-rss` | 2026-05-11 归档 | 9.6k | 微信读书扫码 | MySQL/SQLite | **不推荐**，官方已归档 |
| `ttttmr/Wechat2RSS` | 2026-07-31 | 1.6k | 公开服务无需登录 | 商业闭源 | 公共源可用，私有部署付费 |
| `wechat-article-exporter` | 活跃 | — | 公众号扫码 | 浏览器 | 导出工具，不是 RSS 服务 |

#### 3.1.1 License 与商业使用

| 项目 | LICENSE | 商业限制 |
|------|---------|----------|
| `we-mp-rss` | MIT | README 未写商业限制，软件许可允许商用 |
| `werss` | MIT | README 有内容合规声明，禁止未经许可商业分发第三方内容；软件本身允许商用 |
| `WeWe RSS` | MIT | README 未写商业限制；仓库已归档 |
| `Wechat2RSS` | 无 LICENSE 文件 | 用户协议明确禁止商业用途和内容分发 |

注意：MIT 只解决软件著作权，不解决公众号文章的内容版权。商业使用内容仍需单独评估。


### 3.2 `we-mp-rss` 细节

- 技术栈：Python 3.13 + FastAPI + Playwright。
- 数据库：默认 `sqlite:///./data/db.db`，可选 MySQL。
- 采集模式：
  - 公众号后台扫码（web / api / app）。
  - `weread_mp`：微信读书 Web，需复制 Cookie；只返回最新一篇，无法回补历史。
  - `weread`：微信读书划线/笔记。
- 输出：RSS、Atom、OPML、Markdown/PDF/JSON 导出。
- 镜像：Dockerfile 安装 Playwright Chromium + WebKit，体积大。
- 结论：比 werss 更适合作为长期 sidecar，但**登录仍然绕不开**。

### 3.3 `WeWe RSS` 归档

- GitHub 显示 "This repository was archived by the owner on May 11, 2026."
- 基于微信读书，曾经是稳定性最好的方案。
- 归档后不再维护；fork 版本质量参差。
- 不建议作为新项目依赖。

### 3.4 `Wechat2RSS`

- 公共站点：`https://wechat2rss.xlab.app`。
- 收录 300+ 公众号，提供现成 RSS。
- 实测（2026-10-31）：
  - 列表页 200。
  - 抽样 6 个 Feed，均 200，20 条/源。
  - 部分源更新到 2026-10-30，部分源停留在 2025 年（账号停更）。
- 私有部署为付费产品；GitHub 仓库不含服务端源码。
- 适用：作为「零登录、精选源」入口，不能任意添加账号。

---

## 4. 无登录能力实测

### 4.1 公开专辑页

```text
GET https://mp.weixin.qq.com/mp/appmsgalbum?__biz=MzA3MDM3NjE5NQ==&action=getalbum&album_id=1375870284640911361
```

| 项目 | 结果 |
|------|------|
| 状态码 | 200 |
| 大小 | 326,134 字节 |
| 条目 | 92 个 `album__item` |
| 登录跳转 | 无 |
| 验证码 | 无 |
| 数据 | `data-link`、`data-title`、`data-create-time` |

结论：**列表可用，无需登录。** 只覆盖有专辑的公众号。

### 4.2 公开主页

```text
POST https://mp.weixin.qq.com/mp/homepage?__biz=MzA3MDM3NjE5NQ==&hid=4&begin=0&count=5&action=appmsg_list
```

| 项目 | 结果 |
|------|------|
| 状态码 | 200 |
| 大小 | 2,677 字节 |
| 响应 | `{"base_resp":{"ret":0},"appmsg_list":[...]}` |
| 登录跳转 | 无 |

结论：**JSON 列表可用，无需登录。** 只覆盖有首页模板的公众号。

### 4.3 文章直链

```text
GET https://mp.weixin.qq.com/s?__biz=...&mid=...&idx=...&sn=...
```

| 项目 | 结果 |
|------|------|
| 首次响应 | 302 → `/mp/wappoc_appmsgcaptcha?poc_token=...` |
| 最终响应 | 200，验证码页 |
| `js_content` | 不存在 |
| `环境异常` | 出现 |

结论：**本机被反爬拦截。** 住宅网络可能偶尔可用，但不能作为产品依赖。

### 4.4 搜狗微信搜索

```text
GET https://weixin.sogou.com/weixin?type=2&query=懒猫微服
```

| 项目 | 结果 |
|------|------|
| 搜索页 | 200，9 条结果可解析 |
| 结果链接 | `/link?url=...` |
| 跟随链接 | 302 → `/antispider/` |
| 最终 | 搜狗反爬页 |

结论：**搜索列表可读，但拿不到稳定文章 URL。不做。**

### 4.5 RSSHub 微信路由

源码位置：`lib/routes/wechat/`

| 路由 | 上游 | 登录 | 结论 |
|------|------|------|------|
| `/wechat/mp/msgalbum/:biz/:aid` | `appmsgalbum` | 无 | 与 4.1 相同 |
| `/wechat/mp/homepage/:biz/:hid/:cid?` | `/mp/homepage` | 无 | 与 4.2 相同 |
| `/wechat/wechat2rss/:id` | Wechat2RSS | 无 | 代理第三方服务 |
| `/wechat/sogou/:keyword` | 搜狗 | 无 | 受反爬限制 |
| `/wechat/ce/:id` | CareerEngine | 无 | 第三方来源，稳定性未知 |
| `/wechat/uread/:id` | 微信读书 | 需登录 | 不适用 |

实测：`rsshub.app` 公共实例从本机访问被 Cloudflare 403。RSSHub 不是额外魔法源，只是把上述公开页面包装成 RSS。

---

## 5. 最终策略

### 5.1 产品层

公众号只保留一条路径：

| 能力 | 登录 | 覆盖范围 | 默认 |
|------|------|----------|------|
| `we-mp-rss` sidecar（公众号后台扫码） | 微信扫码，强制 | 可拉历史，覆盖完整 | 公众号功能关闭；用户开启后必须扫码 |

不提供：

- 公开专辑 / 公开主页入口
- Wechat2RSS 精选源入口
- 微信读书扫码路径（只能取最新一篇）

### 5.2 工程层

- 主应用不内嵌 Playwright。
- 公众号采集交给外部 `we-mp-rss`，Rill 只消费其 RSS/Atom 输出。
- `WeChatSidecarAdapter` 只做 HTTP 拉取，不关心 sidecar 内部实现。
- 只依赖 `we-mp-rss`；werss / WeWe RSS 不作为依赖，本地 werss 仅作移植参考。
- 未登录时源状态为"待授权"；二维码过期或登录失效时提示"需重新扫码"。

### 5.3 UI 文案

- "公众号订阅需要微信扫码登录，登录过期后需重新扫码。"
- "需要微信绑定公众号（个人订阅号即可）。"
- "登录凭据由 we-mp-rss 管理，Rill 不保存。"

---

## 6. 未验证 / 后续

- 住宅网络下文章直链的成功率（本机被拦截，未做多 IP 测试）。
- 公开专辑是否支持分页（当前只验证了首屏 92 条）。
- `we-mp-rss` 的 `weread_mp` 模式在长时间运行下的 Cookie 存活时间。
- Wechat2RSS 公共服务的长期可用性。
- 是否存在新的 2026 年无登录项目（本次未发现）。

---

## 7. 合规声明

- 本文只做技术可行性调研，不鼓励绕过登录、验证码或访问控制。
- 公众号内容版权归原作者/平台所有；私有部署也应仅订阅有权访问的内容。
- 不将采集内容用于公开聚合、批量转载或商业分发。
- 上线前应自行评估平台协议、版权和隐私合规风险。
