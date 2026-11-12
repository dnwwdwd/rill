# REQ-028 公众号搜索订阅与 X 自动降级

- 状态：部分实现（公众号能力边界修正；2026-11-03 发现平台限制，见 BUG-001）
- 日期：2026-11-02（2026-11-03 修正）
- 关联：REQ-026、DEC-018、RSS-SOURCES §3.2.1 / §4.4、[BUG-001](../BUGS.md)

## 1. 背景

用户原目标是「登录自己的公众号后台，订阅其他人的公众号」。2026-11-03 真机数据确认：`searchbiz` 能搜索外部账号，但 `appmsgpublish` 对非当前后台管理账号固定返回 `200013`，因此内置链路只能抓取自己管理的公众号。外部未收录账号需 BestBlogs、WeRSs/微信读书或其他外部 RSS（BUG-001）。
同时 X 源在主源失败时应自动降级，而不是要求用户手工配置备用源。

## 2. 公众号

### 2.1 行为

- 扫码登录的是用户**自己的**公众号后台，会话复用 REQ-026 的 `wechat_session_enc`，不新增表。
- 在「添加源」输入公众号名称：
  - 调用 `searchbiz?action=search_biz` 搜索，但结果必须按名称映射到 `switchacct` 返回的自有账号；
  - 0 个结果：提示未找到；
  - 1 个可管理结果：创建 `kind=wechat`、`feed_url=wechat://<gh_username>` 的源；
  - 多个可管理结果：列出候选（名称/微信号），用户选择后创建；
  - 只有外部账号：返回 `wechat_account_unmanaged`，提示改用 BestBlogs/外部 RSS。
- 粘贴 `mp.weixin.qq.com/s/...` 文章链接：
  - 抓取文章页，优先从 URL `__biz`、其次从页面 `var biz = "..."` 解析目标 fakeid；
  - 账号名取 `#js_name` / `.rich_media_meta_nickname`；
  - 必须按 fakeid 或名称映射到自有账号，否则拒绝创建。
- `switchacct` 的「我管理的公众号」列表是**可抓取范围**，不再只是快捷入口。
- 未启用/未登录/外部 sidecar 模式下，添加源给出对应提示；sidecar 模式不支持内置搜索。

### 2.2 API

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/v1/wechat/search?q=` | 返回 `{accounts: [{fake_id,name,avatar,alias}]}`；仅含当前后台可管理账号，否则 409 |
| POST | `/api/v1/wechat/resolve` | body `{url}`，返回 `{account}`；只接受 `mp.weixin.qq.com`；外部账号 409 |

### 2.3 安全

- 搜索/解析均要求已登录管理会话。
- 文章链接只允许 `mp.weixin.qq.com` 主机，防止 SSRF。
- 不记录 token/cookie；错误信息不回传凭据。

## 3. X 自动降级

- 主源：`https://fxtwitter.com/{handle}/feed.xml?count=20`。
- 自动备用：`https://fixupx.com/{handle}/feed.xml?count=20`（FxEmbed 公共镜像）。
- 定时刷新：主源失败后等 10 秒重试，最多 2 次；仍失败再自动切换备用域名一次。手动刷新不等待重试，主源失败后直接切换备用域名；无需用户配置 `fallback_url`。
- 用户显式配置的备用源优先；两者都失败时错误同时包含主源/备用源主机与原因。
- 成功降级后源备注显示「已自动降级到 fixupx.com」。

## 4. 验收

- [x] `SearchAccounts` 解析 `fakeid/nickname/alias/round_head_img`（httptest）。
- [x] 文章 `biz` 从 URL 与脚本两种路径解析（httptest）。
- [x] 添加源名称搜索多结果候选、文章链接直接添加（mock Playwright）。
- [x] X 主源失败自动切换 fixupx；`fixupx.com` 实测 200 RSS。
- [x] 2026-11-12 定时刷新 X 主源失败后 10 秒重试 2 次再降级 fixupx；主备均失败写入 `last_status=error`，进入现有「需关注」列表；手动刷新不等待重试。
- [x] 2026-11-03 真机数据确认：同一会话自有账号列表 `ret=0`；外部账号「二哥狗腿子」「少数派」「晚点LatePost」均 `ret=200013`。
- [x] 代码改为只返回当前后台可管理账号；外部账号/文章链接返回 `wechat_account_unmanaged`；会话级串行、1 秒间隔、30 分钟冷却。
- [ ] 修复后真机部署与 UI 验收。
- [ ] 真实 X 主源故障时的自动降级（需故障场景或代理验证）。

## 5. 未验证与限制

- 任意公开公众号的**文章列表**不再由内置后台链路承诺；`searchbiz` 搜索成功不等于 `appmsgpublish` 可列出文章。
- 外部未收录账号需 BestBlogs 目录或外部 RSS（WeRSs/微信读书等）；WeRSs 通道尚未在 Rill 内实测。
- 文章链接反爬/删除时无法解析；只能映射到当前后台管理账号。
- ASCII 纯英文名可能优先被识别为 X handle；X 请使用 `@handle`，公众号建议中文名或文章链接。
