# REQ-019 懒猫 LPK 打包与 OIDC 登录

- 状态：已完成本地构建与真机安装；真实浏览器 OIDC 回调待验收
- 优先级：P0
- 关联：[PRD.md](../../PRD.md) F-20/F-47、[PDD.md](../../PDD.md) §12.3/§13、[DECISIONS.md](../DECISIONS.md) DEC-012/DEC-013、[DESIGN.md](../DESIGN.md) §页面结构
- 影响代码：`server/internal/config/*`、`server/internal/server/auth*.go`、`server/internal/server/routes.go`、`server/cmd/rill/main.go`、`web/src/api/*`、`web/src/stores/authStore.ts`、`web/src/pages/LoginPage.tsx`、`web/src/components/settings/SecuritySettings.tsx`、`web/index.html`、`web/public/icon.svg`、`package.yml`、`lzc-manifest.yml`、`lzc-build.yml`、`lzc/`

## 用户要求

1. 按懒猫移植规范打包可安装的 LPK。
2. 前端 favicon 与登录页图标使用同一个品牌图形。
3. 直接通过懒猫 OIDC 登录，生产环境不使用密码登录。
4. 登录页去除无关描述和版本号。

## 已确认决策

- 2026-11-01 用户明确要求开始 LPK 打包，替代 DEC-006 的「暂缓」状态。
- `package.yml` 不写 `author`、`homepage`、`license`（用户明确要求移除/暂不声明）。
- 包名使用 `cloud.lazycat.app.rill`，版本与 `web/package.json` 一致为 `0.1.1`，最低系统版本 `1.5.0`。
- 登录采用应用内 OIDC 授权码 + PKCE 回调；`lzc-manifest.yml` 声明 `oidc_redirect_path: /auth/oidc/callback` 并传递懒猫注入的 OIDC 环境变量。
- 密码登录只保留为本地开发/测试模式（`RILL_AUTH_MODE=password`），LPK 固定 `RILL_AUTH_MODE=oidc`，不暴露密码入口。

## 实现范围

### 后端 OIDC

- `RILL_AUTH_MODE`：`password`（默认，兼容本地测试）或 `oidc`。
- OIDC 模式必填：`RILL_OIDC_CLIENT_ID`、`RILL_OIDC_CLIENT_SECRET`、`RILL_OIDC_AUTH_URI`、`RILL_OIDC_TOKEN_URI`、`RILL_OIDC_USERINFO_URI`；`RILL_OIDC_ISSUER` 仅记录，不做 discovery。
- `GET /auth/oidc/login?return_to=...`：生成 state、nonce、PKCE verifier，写入加密短期 cookie 后跳转授权端点。
- `GET /auth/oidc/callback`：校验 state、交换授权码、读取 userinfo、建立现有 `sessions` 行并设置 `rill_session` cookie；`return_to` 只接受站内绝对路径。
- OIDC 模式下 `POST /api/v1/auth/login` 与 `POST /api/v1/auth/password` 返回 403，不修改数据库密码哈希。
- `GET /api/v1/auth/config` 公开返回 `{mode, login_url}`，供前端决定登录方式。
- 不新增表或字段；复用 `sessions`、`settings`。

### 前端

- 启动时读取 auth config：OIDC 模式且未登录时，登录页自动跳转 `/auth/oidc/login`；回调失败时展示简短错误与重试。
- OIDC 模式登录页只保留品牌图标、Wordmark、登录状态/按钮；移除产品描述、服务健康检查、初始密码提示和版本号。
- OIDC 模式安全设置隐藏修改密码表单与退出登录（登录由懒猫统一管理）；密码模式仅在本地开发时显示。
- favicon、登录页图标、LPK 图标使用同一品牌图形：微信绿圆角底 + 三根近黑横向水纹。

### LPK

- LPK V2 元数据与运行结构分离。
- 运行时镜像来自 `registry.lazycat.cloud`，使用 Debian 基础镜像执行静态 Go 二进制。
- 构建期完成前端构建和 Go 编译；运行脚本只启动服务，不安装依赖。
- 持久化 `/lzcapp/var/data`，缓存 `/lzcapp/cache/rill`，静态前端与二进制只从 `/lzcapp/pkg/content` 读取。
- `public_path` 只放行 `/feed/`、`/opml/`、`/media/`、`/healthz`；管理 API 与 OIDC 回调仍由懒猫网关保护。
- 打包官方 file chooser inject，开启 `fileSystemAccess` / `fileInput` / `download` / `objectUrl`；OPML 与备份导入导出走统一选择器。
- 健康检查调用二进制的 `-health` 子命令，不依赖 curl/wget。

## 验收标准

- `go test ./...`、`go vet ./...`、`CGO_ENABLED=0 go build ./cmd/rill` 通过。
- 新增 OIDC 测试覆盖：登录跳转参数、state 错误拒绝、授权码交换、userinfo、会话 cookie、密码端点 403。
- `npm run typecheck`、`npm test`、`npm run build` 通过。
- 登录页在 OIDC 模式无密码输入、无版本号、无产品描述；favicon 与登录图标同源。
- `lzc-cli project build` 能产出 LPK，且 `content.tar` 含官方 file chooser inject；若外部镜像仓库不可达，如实记录未验证项。
- LPK 真机安装、真实懒猫 OIDC 回调和 Feed 公网可达性需在目标微服验收。

## 验收证据

- `lzc-cli app install rill.lpk --apk n` 成功；`lzc-cli app status` 返回 `Installed`。
- 目标微服 `192.0.2.10:2222`；访问地址 `https://rill.example.com`。
- `cloudlazycatapprill-rill-1` 与 `cloudlazycatapprill-app-1` 均 `Up (healthy)`，镜像成功拉取。
- 公网 `/healthz` 200 `{"ok":true}`；容器内 `rill -health` 返回 0。
- 容器内 `/api/v1/auth/config` 返回 `{"mode":"oidc","login_url":"/auth/oidc/login"}`；7 个 OIDC 环境变量均非空。
- 容器内 `/auth/oidc/login` 302 到 `https://example.com/sys/oauth/auth`，含 PKCE challenge/state。
- `/lzcapp/var/data` 已生成 `rill.db`、WAL、`secret.key`、`backups/`；`/lzcapp/cache/rill` 已创建。

## 未验证 / 风险

- 真实浏览器打开 `https://rill.example.com` 后的懒猫 OIDC 回调、userinfo 字段与登录后页面仍需验收。
- 升级、休眠唤醒后的持久化/缓存恢复、file chooser 与公网 Feed/OPML/media 输出需真机验收。
- 本机拉取 `registry.lazycat.cloud` 镜像失败；真机安装时拉取成功，本机容器运行验证仍缺。
- 外部 Feed、X、公众号与 AI 的真实网络能力不在本 REQ 范围内。
