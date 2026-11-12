# REQ-020 阅读与订阅源 UI 细化

- 状态：已完成（本地构建并升级安装；真实浏览器视觉待验收）
- 优先级：P0
- 关联：[REQ-019](REQ-019-lazycat-lpk-oidc.md)、[PRD.md](../../PRD.md) F-01/F-03/F-07/F-33/F-47、[PDD.md](../../PDD.md) §5.2/§6.5、[DESIGN.md](../DESIGN.md)
- 影响代码：`web/src/components/reader/*`、`web/src/components/sources/*`、`web/src/components/settings/SourcesSettings.tsx`、`web/src/components/layout/*`、`web/src/pages/LoginPage.tsx`、`web/src/styles/index.css`、`web/src/components/ui/Icon.tsx`、`web/src/components/ui/Button.tsx`、`server/internal/server/handlers_sources.go`

## 用户要求

1. favicon、登录图标与 LPK 图标统一为绿色品牌图标。
2. 封面图与文章标题同一行，封面改为正方形。
3. 缩小中栏文字；详情 topbar 高度与中栏顶部行一致。
4. 登录页不自动触发 OIDC，点击按钮后再跳转。
5. 调小正文标题字号；替换正文及全局 AI 图标。
6. 正文不显示平台标签（如 `@X`）。
7. 添加订阅源简化为单输入，支持 X 主页链接或用户名自动识别，添加后自动同步文章。
8. 设置 → 订阅源改为 filetree，支持分组和源的增删改移动。
9. 优化源详情布局与质感。
10. 侧栏与 topbar 图标增加 tooltip。
11. 阅读进度改为环形组件，并与回到顶部按钮分离。
12. 设置弹窗不显示「保存中 / 已同步」状态。
13. 环形进度组件与回到顶部按钮尺寸一致。
14. 全屏阅读不显示订阅来源与阅读设置；AI 摘要移到标题下方、正文上方；用退出全屏图标退出；全屏正文标题字号与普通阅读一致。
15. 中栏 topbar 与正文 topbar 同高、图标同排、无边框。

## 实现摘要

- 品牌图标改为微信绿圆角底 + 近黑水纹；favicon 使用 `?v=2` 避免旧缓存，Go 静态层对 icon/favicon 返回 `no-cache`。
- 列表改为「标题 + 方形封面」同一行，摘要独占一行；封面 64px（移动端 56px）。
- 中栏标题 20px，条目标题 15.5px，摘要 11px；详情 topbar 与列表顶部行均为 54px。
- 登录页只保留按钮；真实 OIDC 在点击后才跳转。
- 正文标题 `clamp(24px, 2.6vw, 34px)`；平台标签从正文元信息移除。
- `sparkles` 与 `settings-ai` 统一换成魔杖 + 星点图标。
- 添加源改为单输入：X 链接/`@handle`/无点用户名 → X 源；其他 URL → 发现候选，单候选直接添加；后端创建后同步抓取并回填源状态。
- 订阅源设置改为可折叠 filetree；分组支持新建、重命名、移动、删除；源支持编辑/移动、刷新、启停、删除。
- 源详情增加卡片化头部、渐变纸面、指标与卡片悬浮层次。
- `IconButton` 与侧栏工具按钮使用 `data-tooltip`，悬停/聚焦显示自定义 tooltip。
- 阅读进度改为 50px 环形进度，回到顶部为独立 50px 圆钮，两者尺寸一致。
- 设置弹窗标题栏移除「保存中 / 已同步」指示，只保留关闭按钮。
- 中栏 topbar 与正文 topbar 固定 54px（移动端 47px），图标同一水平行；搜索框和源告警作为浮层，不再撑高 topbar，也不绘制底边框。
- 全屏阅读移除订阅来源与阅读设置；AI 摘要移到标题下方、正文上方；退出使用 `minimize` 图标，标题字号与普通阅读同为 24–34px（移动端 23px）。

## 验收证据

- `npm run typecheck`：通过。
- `npm test -- --run`：9 个文件、23 个用例通过。
- `npm run build`：通过。
- `go test ./...`、`go vet ./...`：通过；新增 `TestCreateSourceFetchesImmediately`。
- `lzc-cli project lint`：无警告。
- `lzc-cli project build -o rill.lpk`：通过，版本 `0.1.2`。
- `lzc-cli app install rill.lpk --apk n`：升级与同版本重装成功；`cloudlazycatapprill-rill-1` / `app-1` 均 healthy；公网 `/healthz` 200；LPK icon 校验和与本地 `lzc-icon.png` 一致；容器内 `/icon.svg?v=2` 为绿色 SVG。

## 未验证

- 真实浏览器视觉验收：封面与标题同排、topbar 同高无边框、全屏阅读、环形进度与回到顶部尺寸、filetree 密度、tooltip 定位和绿色 favicon 缓存刷新。
- 真实 X/FxTwitter 添加、自动同步与网络失败状态。
- 真实分组/源移动、删除后的数据持久化与 OPML roundtrip。
- file chooser、升级/休眠与真实 OIDC 回调仍见 REQ-019。
