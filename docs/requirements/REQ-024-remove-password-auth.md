# REQ-024 移除本地密码登录，仅保留懒猫 OIDC

- 状态：已完成（本地验证）
- 优先级：P1
- 决策：[DEC-017](../DECISIONS.md)（取代 [DEC-013](../DECISIONS.md) 的「密码模式仅本地开发」）
- 关联：[REQ-019](REQ-019-lazycat-lpk-oidc.md)、[PDD.md](../../PDD.md) §12.3、[server/README.md](../../server/README.md)
- 影响代码：`server/internal/config/config.go`、`server/internal/server/{auth.go,oidc.go,routes.go,app.go,settings_store.go}`、`server/cmd/devoidc/`（新增）、`web/src/pages/LoginPage.tsx`、`web/src/stores/authStore.ts`、`web/src/api/{client.ts,types.ts,mock/mockApi.ts}`、`web/src/components/settings/SecuritySettings.tsx`

## 用户要求

移除本地登录和相关密码修改的功能，因为在懒猫环境直接懒猫 OIDC 登录了（2026-11-01）。

## 范围

| 项 | 处理 |
| --- | --- |
| 后端配置 | 删除 `RILL_AUTH_MODE`、`RILL_ADMIN_PASSWORD`、`Config.AuthMode`、`OIDCEnabled()`；启动强制校验 5 个 `RILL_OIDC_*` 变量，缺失即启动失败 |
| 后端端点 | 删除 `POST /api/v1/auth/login`（含登录限速器）与 `POST /api/v1/auth/password`；`/api/v1/auth/config` 固定返回 `{mode:"oidc", login_url:"/auth/oidc/login"}`；logout/CSRF/sessions 不变 |
| 密码存储 | 删除 `settings` 表 `admin_password_hash` 的写入与读取逻辑（`last_login_at` 改由 OIDC 回调写入，此前即如此） |
| 本地联调 | 新增 `server/cmd/devoidc`：环回假 OIDC 提供方（authorize 自动发放 `dev-code`、token、userinfo `sub=dev-user`），仅监听 `127.0.0.1:8091`，不做真实鉴权 |
| 登录页 | 只保留「使用懒猫账号登录」按钮（不自动跳转）；删除密码表单 |
| 设置·安全 | 删除改密表单与退出登录按钮；账号行固定为「懒猫账号」；会话信息保留 |
| API 客户端 | 删除 `login(password)` / `changePassword`；mock 客户端以 `signIn()` 模拟 OIDC 建立演示会话；真实客户端不提供 signIn |
| mock 模式 | 不受影响；mock 登录语义不变（sessionStorage 演示会话） |

## 验收证据

2026-11-01 本地验证：

- `go build ./...`、`go vet ./...` 通过；`go test ./...` 全绿；`go test -race ./internal/server/ ./internal/config/` 通过。
- 新增/改写测试：`TestAuthConfigSessionAndLogout`、`TestPasswordEndpointsRemoved`（未登录 401、登录后 404）、OIDC 端到端流程与 state 校验用例继续通过；测试 helper 改为直接种会话（密码登录端点已不存在）。
- 前端 `npm run typecheck`、`npm run test`（9 文件 26 用例）、`vite build` 通过；`LoginPage.test` 改为纯 OIDC 断言。
- 本地端到端：`devoidc` + OIDC 模式后端完成 authorize → token → userinfo → 回调建会话（curl 流程，见 PROGRESS v0.21）。

## 未验证

- 懒猫真机 OIDC 回调回归（本轮仅本地假提供方验证；真机此前已通过授权跳转，回调待真实浏览器验收，见 REQ-019）。
