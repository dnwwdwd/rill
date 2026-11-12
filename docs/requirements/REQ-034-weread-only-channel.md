# REQ-034 公众号收敛为微信读书唯一通道 + 扫码回调修复

- 状态：进行中（本地验证；真实微信读书扫码与外部号抓取待验收）
- 优先级：P0
- 日期：2026-11-03
- 关联：[REQ-033](REQ-033-weread-dual-channel.md)、[REQ-026](REQ-026-wechat-builtin-core.md)、[REQ-031](REQ-031-bestblogs-wechat-catalog.md)、DEC-024、[BUG-001](../BUGS.md)

## 1. 背景

REQ-033 实现了「微信后台（自有账号）+ 微信读书（外部账号）」双通道。用户 2026-11-03 实际使用后确认：

1. 微信后台通道只能抓 `switchacct` 自有账号（BUG-001），对用户没有意义；
2. 微信读书扫码成功后前端没有回调逻辑，界面卡在原状态；
3. 「用于 BestBlogs 未收录的外部公众号；收录和更新可能滞后。」文案不需要。

用户决策（2026-11-03）：**移除微信后台通道（含内置链路与 we-mp-rss sidecar 伴生服务），公众号只保留微信读书扫码一条通道**；未收录公众号通过 BestBlogs 目录或粘贴文章链接添加。

## 2. 目标

1. 移除微信后台通道：
   - 删除内置后台扫码登录/账号列表/`searchbiz` 搜索/`appmsgpublish` 抓取与微信后台会话存储（`wechat_session_enc`）；
   - 删除 we-mp-rss sidecar 伴生服务模式（sidecar 扫码、sidecar Feed 校验、sidecar 设置字段 `sidecar_url` / `sidecar_username` / `sidecar_password`）；
   - 删除 `POST /api/v1/wechat/scan`、`GET /api/v1/wechat/scan/status`、`GET /api/v1/wechat/search`、`GET /api/v1/wechat/accounts`、`POST /api/v1/wechat/logout` 端点。
2. 公众号源抓取统一走微信读书：`wechat://<fakeid>` 源直接用微信读书 Cookie 抓取，`-2041` 冷却等 REQ-033 机制保持不变。
3. 文章链接添加保留：`POST /api/v1/wechat/resolve` 改为抓公开文章页提取 `biz`/账号名/头像，不再要求后台登录；创建源要求微信读书已连接。
4. 前端回调修复：微信读书扫码成功后刷新设置（`weread_connected` 变为 true）、提示连接成功，界面不再卡住。
   - 2026-11-03 真机追加根因：真实 `getLoginInfo` 的 `webLoginVid` 是数字、字段可能在 `data` 内层，Go 按 `string` 解析会失败，轮询永远拿不到 `succeed=true`。已改为容错解析（数字/字符串/嵌套/`PENDING`/`SCANNED`），轮询按长轮询处理并加 2.5s 上限。
   - 2026-11-03 真机追加：等待阶段 `logicCode=LOGIN_TIMEOUT` 是正常长轮询状态，不再判过期；登录成功先用 `/web/shelf/sync` 校验候选 Cookie（原始 `wr_skey` 优先，`refreshToken` 兜底，失败走 renewal），校验通过才置「已连接」；`-2041` 是列表接口的风控/频控码，**不是登录验证码**——列表被拦截时自动降级 `/api/mp/cover` 只同步最新一篇，cover 也失败才进入 4h 冷却，重登成功即清除冷却。
5. 设置页只保留「启用公众号采集」与「微信读书扫码」两行；移除「微信扫码登录」「伴生服务」与账号列表；移除上述滞后文案。

## 3. 非目标

- 不新增名称搜索能力（`searchbiz` 随后台通道移除）；BestBlogs 未收录的号只能粘贴文章链接。
- 不恢复微信后台抓取，即使对自有账号也不再支持。
- 不承诺微信读书全量历史、不绕过验证码（沿 REQ-033 非目标）。
- 不做 schema 迁移；`wechat_session_enc` 等遗留 settings 键不再读取，不主动清理。

## 4. 验收

- 本地：
  - 微信读书扫码成功后设置刷新、提示连接成功；
  - `wechat://` 源创建要求微信读书已连接；文章链接解析不再要求后台登录；
  - 已删除端点返回 404；`/settings` 不再返回 sidecar 字段与后台 status；
  - `go test ./...`、`go vet ./...`、前端 typecheck/test/build。
- 真机验收（待做）：真实微信读书扫码回调、外部公众号列表/正文、验证码与 Cookie 过期。

## 5. 风险

- 公众号能力完全依赖微信读书通道：验证码、收录滞后、Cookie 过期都会直接中断抓取（沿 REQ-033 已知限制）。
- 原 sidecar Feed 方式添加的存量源退化为普通 RSS 抓取；若 sidecar 已下线则自然失败，可在源诊断中看到。
