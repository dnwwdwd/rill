# REQ-026 公众号内置核心链路（HTTP-only）

- 状态：已实现（2026-11-02，真机扫码待验收）
- 关联：DEC-018（取代 DEC-003）、[RSS-SOURCES](../RSS-SOURCES.md) §4、[PDD](../PDD.md) §7.4、[PRD](../../PRD.md) F-15/F-16
- 上游参考：`rachelos/we-mp-rss`（MIT），核心链路源码 `driver/wx_api.py`、`core/wx/model/app.py`、`core/wx/base.py`、`core/article_content.py`

## 背景

原方案强制依赖外部 `we-mp-rss` sidecar：用户需另装应用、填地址和管理账号密码，安装体积增加约 1.7GB 镜像。用户要求采用最轻量方案：Rill 单二进制内置公众号核心链路，不引入第二个容器。

## 目标

1. 内置「公众号后台扫码登录 → 公众号列表 → 文章列表 → 正文抓取」核心链路，纯 HTTP 实现，不含 Playwright。
2. 会话（token/cookies/fingerprint）AES-GCM 加密存 `settings`，不新增表或迁移。
3. 设置页提供扫码、公众号选择和添加订阅源；`kind=wechat` 源直接由内置抓取器更新。
4. 保留外部 sidecar 兼容：配置了 `sidecar_url` 的实例继续走原代理，不清空既有设置。

## 范围（只做核心链路）

- 扫码：`bizlogin startlogin` → `scanloginqrcode getqrcode/ask` → `bizlogin login` 提取 token 与 cookies。
- 账号：`switchacct get_acct_list`，字段容错解析（fakeid/username、nickname/name）。
- 列表：`appmsgpublish list_ex`，`publish_page` 双层 JSON 解析，aid 优先、link 兜底去重。
- 正文：文章 URL 直取 HTML，`#js_content` 提取 + 清洗，`data-src` 提升，图片走既有 `/media/` 代理。
- 错误：同一会话串行抓取、请求间隔 1s；`200013` 记录失败并按 `fail_count` 退避，同时触发 30 分钟会话级冷却；`200003` 会话失效将源置为 `auth`，提示重新扫码。外部第三方账号的 `200013` 视为平台列表能力限制，不再重试（BUG-001）。

## 不做（明确缺口）

- Playwright 相关模式：微信读书、`free_publish` 降级、反爬浏览器兜底。
- 多账号切换、级联、通知、PDF/docx 导出、sidecar 管理界面。
- 不承诺所有公众号/所有文章都能抓取；反爬命中时给明确状态，不静默失败。

## 接口

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/api/v1/wechat/scan` | 内置模式发起扫码，返回 data URL 二维码；外部 sidecar 模式保持原行为 |
| GET | `/api/v1/wechat/scan/status` | 轮询扫码状态；成功后落盘会话并返回公众号列表 |
| GET | `/api/v1/wechat/accounts` | 返回已登录账号的公众号列表（可刷新） |
| POST | `/api/v1/wechat/sources` | 按公众号创建 `kind=wechat` 源（fakeid 存 `config_json`） |

## 安全与合规

- 会话凭据仅在服务端加密存储，不返回前端；日志与错误信息不得包含 token/cookie。
- 使用非官方网页接口存在微信风控与账号限制风险；沿用「用户主动扫码、可随时关闭」的既有边界。
- 移植代码保留 we-mp-rss 的 MIT 版权声明与来源说明。

## 验收

- 包级单测：登录流程、账号解析、列表双层 JSON、错误码、正文清洗（httptest，不访问真实网络）。
- `go test ./...`、`go vet`、前端 typecheck/测试/build、LPK lint/build 通过。
- **真机扫码验收待做**：需要用户用绑定公众号的微信扫码；在完成前状态只能写「部分验证」。

## 回滚

- 清空内置会话或配置 `sidecar_url` 即回到外部 sidecar 路径；数据库不新增结构，回滚无需迁移。
