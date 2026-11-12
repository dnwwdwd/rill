# PROG-REQ-019 懒猫 LPK 打包与 OIDC

- 关联：[REQ-019](../requirements/REQ-019-lazycat-lpk-oidc.md)、DEC-012/DEC-013/DEC-014
- 日期：2026-11-01
- 状态：本地构建与假 OIDC 提供方验证通过；真机与真实懒猫 OIDC 待验收

## 已完成

### 后端

- `RILL_AUTH_MODE=password|oidc`；OIDC 模式不再要求 `RILL_ADMIN_PASSWORD`。
- `GET /api/v1/auth/config` 公开返回登录模式。
- `GET /auth/oidc/login`、`GET /auth/oidc/callback`：授权码 + PKCE、加密 state cookie、token 交换、userinfo `sub`、复用 `sessions`/CSRF。
- OIDC 模式下 `POST /auth/login`、`POST /auth/password` 返回 403；不写密码哈希。
- `rill -health` 调用本地 `/healthz`，供容器 healthcheck 使用。

### 前端

- 启动读取 auth config；OIDC 模式未登录时自动跳转懒猫登录。
- 登录页改为单卡片：品牌图标 + Wordmark + 登录状态/重试；移除产品描述、健康状态、初始密码提示和版本号。
- `favicon`、登录图标与 LPK 图标统一为微信绿圆角底 + 三根近黑水纹。
- 设置「账号与会话」在 OIDC 模式隐藏修改密码与退出登录。

### LPK

- `package.yml`、`lzc-manifest.yml`、`lzc-build.yml`、`lzc/build-package.sh`、`lzc/run.sh`。
- 构建期完成 `npm ci`、`vite build`、`CGO_ENABLED=0 go build`；运行期只启动二进制。
- 产物包含 `bin/rill`、`web/`、`lzc/run.sh`、`lzc/lazycat-injects/lzc-file-chooser-inject.js`、`ca-certificates.crt`。
- `public_path` 仅 `/healthz`、`/feed/`、`/opml/`、`/media/`。
- 已接入官方 file chooser inject（`fileSystemAccess` / `fileInput` / `download` / `objectUrl` 全开）；OPML/备份导入导出走统一选择器。

## 真机安装

- 目标：默认微服 `192.0.2.10:2222`（登录用户 root，管理员）。
- 命令：`lzc-cli app install rill.lpk --apk n`。
- 结果：`安装成功！`，`lzc-cli app status cloud.lazycat.app.rill` 返回 `Installed`；首次安装 `0.1.0`，UI 细化后升级到 `0.1.2`，两次容器均 healthy。
- 访问地址：`https://rill.example.com`。
- 容器：`cloudlazycatapprill-rill-1` 与 `cloudlazycatapprill-app-1` 均为 `Up ... (healthy)`。
- 公网健康检查：`https://rill.example.com/healthz` 返回 200 `{"ok":true}`。
- 容器内 `rill -health` 返回 0；`/api/v1/auth/config` 返回 `{"mode":"oidc","login_url":"/auth/oidc/login"}`。
- 容器内 7 个 OIDC 环境变量均非空：mode、client id、client secret、issuer、auth/token/userinfo URI。
- 容器内 `/auth/oidc/login` 返回 302 到 `https://example.com/sys/oauth/auth`，包含 PKCE challenge 与 state；直接访问容器 IP 时 `redirect_uri` 使用容器地址，符合 Host 推导逻辑，真实浏览器经 ingress 时应为公网域名。
- `lzc-cli app log` 仍显示 `not yet realized`（CLI 已知状态文案），但容器日志有 `Rill listening` 且 healthcheck 通过；以 `lzc-docker ps` / 公网 `/healthz` 为准。

## 验证证据

| 检查 | 结果 |
| --- | --- |
| `go test -count=1 ./...` | 通过 |
| `go vet ./...` | 通过 |
| `go test -race -count=1 ./...` | 通过（server 34.6s） |
| `CGO_ENABLED=0 go build ./cmd/rill` | 通过 |
| `npm run typecheck` | 通过 |
| `npm test -- --run` | 7 个文件、19 个用例通过 |
| `npm run build` | 通过 |
| `sh lzc/build-package.sh` | 通过，生成 `lzc-dist` |
| `lzc-cli project lint` | `No manifest lint warnings found.` |
| `lzc-cli project build -o rill.lpk` | 通过，生成 15MB `rill.lpk` |
| 本地假 OIDC 提供方 | `login → provider → callback → /items/1` 303；`/api/v1/auth/csrf` 200 |
| 构建二进制 `-health` | 本地 8080 返回 0；不存在的 19999 返回 1 |
| 静态资源 | `/icon.svg` 200 `image/svg+xml`；index 引用 `/icon.svg`、`/favicon-64.png`、`/icon-180.png` |
| LPK 内容 | `package.yml`、`manifest.yml`、`icon.png`（512×512 RGBA）、`content.tar`；`content.tar` 含 `lzc/lazycat-injects/lzc-file-chooser-inject.js` |
| `lzc-cli app install rill.lpk --apk n` | 成功；`lzc-cli app status` 返回 `Installed`；最终版本 `0.1.2` |
| 真机容器 | `cloudlazycatapprill-rill-1` / `app-1` 均 `Up (healthy)`；镜像成功拉取 |
| 公网 `/healthz` | 200 `{"ok":true}` |
| 容器 `/api/v1/auth/config` | `{"mode":"oidc","login_url":"/auth/oidc/login"}` |
| 真机 OIDC 环境 | 7 个变量全部非空；`/auth/oidc/login` 302 到懒猫授权端点 |

## 未验证 / 阻塞

- 真实浏览器中的懒猫 OIDC 回调、userinfo 字段和登录后页面仍需用户打开 `https://rill.example.com` 验收；容器内已确认跳转参数与 7 个 OIDC 环境变量正确。
- 真机 `/lzcapp/var/data` 已生成 `rill.db`、WAL、`secret.key` 与 `backups/`；`/lzcapp/cache/rill` 已创建。升级/休眠后的持久化与缓存恢复未验收。
- 本机 `docker pull registry.lazycat.cloud/.../debian:cb352a5223b8abc9` 两次失败（EOF），但真机安装时成功拉取并运行；本机容器运行验证仍缺。
- 登录页未做浏览器截图（当前环境缺少 Chromium 运行库）；已用 React Testing Library 覆盖图标、无密码/无版本文案和 OIDC 错误态。
- 官方 file chooser inject 已打包，但真机文件选择、下载拦截与 `/_lzc/files/home` 未验收。
- `lzc-cli app log` 显示 `not yet realized`，与容器实际运行状态不一致；以 `lzc-docker ps`、容器日志和公网 `/healthz` 为准。
- 真实外部 RSS/X/公众号仍未验收。
