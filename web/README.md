# Rill 前端

React 19 + Vite + TypeScript + React Router 7 + Zustand。开发默认使用 mock API；生产构建读取 `.env.production` 并连接真实 Go API。登录只有懒猫 OIDC（按钮手动触发，不自动跳转）；本地联调用 `server/cmd/devoidc` 假提供方（DEC-017）。

## 运行

```bash
npm install
cp .env.example .env.local
npm run dev
```

打开 `http://127.0.0.1:5173`。mock 模式模拟懒猫 OIDC：点击「使用懒猫账号登录」直接建立演示会话；会话保存在当前标签页的 `sessionStorage`，不会写入真实凭据。当前 mock 覆盖登录、阅读、详情、专注、设置弹窗内的订阅源管理、20 个源诊断、6 个设置分类、添加源发现和 404；证据见 [`docs/requirements/REQ-009-mock-data.md`](../docs/requirements/REQ-009-mock-data.md) 与 [`REQ-011`](../docs/requirements/REQ-011-settings-dialog.md)。

`npm run preview` 预览的是生产构建（默认真实 API），需先在 `127.0.0.1:8080` 启动 `server/`；只预览界面时可用 `VITE_USE_MOCK=true npm run build && npm run preview`。

## 校验

```bash
npm run typecheck
npm run test
npm run build
```

`npm run build` 产出 `dist/`，由 Go 进程或 LPK 托管；根目录现已包含 `package.yml`、`lzc-manifest.yml`、`lzc-build.yml`、`lzc/`。生产构建读取 `.env.production`，默认 `VITE_USE_MOCK=false`；开发代理覆盖 `/api`、`/feed`、`/opml`、`/media`、`/healthz`、`/auth`，因此真实后端模式下 OIDC 回调与公开 Feed 也能直接验证（需要真实 OIDC 环境或本地假提供方）。

## 路由

| 路径 | 页面 |
|------|------|
| `/login` | 懒猫 OIDC 登录（唯一登录方式，DEC-017） |
| `/`、`/items/:itemId` | 三栏阅读 / 移动端列表与正文 |
| `/items/:itemId/focus` | 专注阅读 |
| `/sources` | 兼容入口；打开设置弹窗的「订阅源」 |
| `/sources/:sourceId` | 源诊断 |
| `/settings`、`/settings/:section` | 移动端两级设置页（桌面端访问时打开设置弹窗并回到阅读页） |
| 设置入口 | 桌面侧栏左下角齿轮（弹窗）/ 移动底栏「设置」（页面）；内容提供订阅源（含公众号接入）/ 阅读 / AI / 备份 / 安全 五类（REQ-023） |

## 约定

- 接口形状以 [`src/api/types.ts`](src/api/types.ts) 为准，mock 与真实客户端实现同一 `ApiClient`。
- 正文 HTML 先由后端清洗，前端再用 DOMPurify 做第二道白名单；前端不解析 Feed、不拼媒体地址。
- 阅读偏好通过 CSS 变量即时生效，同时写入后端 `settings`。
- 登录页进入时不自动跳转，点击「使用懒猫账号登录」后才发起 OIDC。
- 添加源为单输入：X 主页链接、`@用户名` 或无点用户名按 X 源创建；Apple Podcasts / 小宇宙 / 微博 / Medium / Mastodon 先走 `/sources/resolve`（小宇宙/微博的部分结果需确认）；其他 URL 再 discover，单候选直接创建；创建后后端同步抓取并刷新列表。
- 添加源另有「搜索创作者」模式：按名称走 `/sources/search`，按平台分组展示候选；可多选后走 `/sources/batch` 批量创建。X/YouTube/小宇宙/公众号只覆盖 BestBlogs 目录，界面必须保留平台状态和来源标签。
- 设置 → 订阅源为可折叠 filetree；分组支持新建、重命名、移动、删除，源支持编辑/移动、刷新、启停、诊断和删除。
- 阅读列表封面为 64px 正方形并与摘要同排；详情标题 24–34px，右侧为环形进度和独立回到顶部按钮。
- 视觉与文案规范见 [`docs/DESIGN.md`](../docs/DESIGN.md)；产品依据见根目录 [`PRD.md`](../PRD.md)、[`PDD.md`](../PDD.md)；源能力见 [`docs/RSS-SOURCES.md`](../docs/RSS-SOURCES.md)。
- 基础组件统一从 [`src/components/ui`](src/components/ui) 引入；搜索、提示、卡片和指标优先使用 `SearchField`、`Notice`、`Card`、`Metric`，不要在页面内重复造样式。
