# REQ-018 详情阅读体验、设置图标与文件夹 / OPML

- 状态：已完成（本地验证，部分收尾项待验收）
- 优先级：P1
- 关联：[REQ-003](REQ-003-opml-folder-ui.md)、[REQ-017](REQ-017-detail-actions-translation.md)、[DESIGN.md](../DESIGN.md)、[PRD.md](../../PRD.md) F-03/F-07/F-26/F-30、[PDD.md](../../PDD.md) §6.5
- 影响代码：`web/src/components/reader/*`、`web/src/components/settings/SourcesSettings.tsx`、`web/src/components/sources/FolderManagerDialog.tsx`、`web/src/pages/ReaderPage.tsx`、`web/src/stores/readerStore.ts`、`web/src/stores/sourcesStore.ts`、`web/src/api/*`、`web/src/styles/index.css`

## 用户要求

1. 点击未读条目没有变成已读。
2. 详情页的时间、订阅源和作者名显示在同一行，不要分隔符。
3. AI 摘要去掉折叠。
4. 右侧显示整篇文章的阅读进度，并支持回到顶部。
5. 更换设置中的 AI 图标。
6. 开始实现尚未实现的功能，直到全部完成。

## 实现范围

### 已读点击

- 列表点击（鼠标、Enter、移动端）在 `auto_read` 开启时立即乐观标记已读，再进入详情；失败时回滚。
- 直接访问 URL、键盘 `j/k` 等不经过列表点击的路径，仍由停留观察器在 3 秒后标记；观察器只在详情加载完成后启动，修复此前详情晚到导致 effect 不再执行的问题。
- `markRead` 支持仅存在于 `details`、尚未出现在当前列表的条目，失败回滚保留原 `read_at`。
- 关闭「停留标记已读」后，点击只进入详情，不自动改状态。

### 详情元信息

- 来源、时间、作者同排；移除 `·` 分隔符和单独的作者行。
- 无作者时不显示空作者；公众号 / X 标签仍在该行末尾。
- 普通详情页与专注阅读页使用同一规则。

### AI 摘要

- 移除折叠按钮、chevron 与 `summaryOpen` 状态；摘要卡常显。
- 无摘要时直接显示生成入口；生成中、已生成、无权限状态保持原有逻辑。

### 阅读进度与回到顶部

- 桌面普通详情页右侧显示竖向进度轨：百分比、进度填充、回到顶部按钮。
- 进度以 `.reader-scroll` 的实际滚动高度计算，翻译 / 双语切换后重新测量。
- 回到顶部使用平滑滚动；顶部时按钮禁用；≤1040px 隐藏，移动端不遮挡正文。
- 专注阅读页顶部已有百分比与进度条，新增「回到顶部」图标按钮。

### 设置 AI 图标

- `settings-ai` 改为圆环内四角星，替代原来的节点网络图；设置导航与分区标题同步使用。

### 尚未实现功能的推进

- 「记住阅读位置」从仅有设置项变为真实行为：按条目把滚动位置写入 `localStorage`（`rill.reader.position.{id}`），重新进入时恢复；普通详情与专注阅读共用同一位置。
- 订阅源设置新增「文件夹」管理：创建、重命名、删除文件夹；删除后源回落未分组。
- 订阅源设置新增 OPML 导入 / 导出：使用现有 `GET /api/v1/opml/export` 与 `POST /api/v1/opml/import`；导入后刷新源与文件夹并提示新增 / 跳过数量。
- 开发与 mock 环境使用标准文件输入；LPK 已打包官方 file chooser inject，真机选择器待确认。

## 验收证据

- `npm run typecheck`：通过。
- `npm test`：6 个测试文件、17 个用例通过。
- `npm run build`：通过（v0.14 构建）。
- Playwright（2026-11-01，mock API，1440×900 / 390×844）：
  - 点击未读条目 500ms 后选中行 `is-unread=false`，详情按钮变为「标为未读」；4s 后保持。
  - 直接访问 `/items/7`，1.2s 时仍为未读，4.7s 后自动变为已读。
  - 元信息三个子项 top 分别 114 / 117 / 117，行高 20px；`.article-byline` 与 `.article-kicker .dot-sep` 均为 0。
  - AI 摘要头部为 `DIV`，桌面与移动端 `.ai-summary-body` 均为 1，`.ai-summary-chevron` 为 0。
  - 阅读进度从 0% 滚到 80%，回到顶部后 scrollTop=0、进度 0%、按钮禁用；进度轨在桌面显示，390px 隐藏。
  - 设置 AI 导航与分区图标均输出新的圆环四角星 SVG。
  - 文件夹：创建「播客」、重命名为「播客与音频」、删除后行消失并提示「源已移至未分组」；0 JS 错误。
  - OPML：导出下载 `rill-subscriptions.opml` 且包含现有源；导入含「测试导入」文件夹和 1 个源的 OPML 后源数 20→21，提示「已导入 1 个订阅源」。
  - 阅读位置：条目 1 滚到约 873px，切到条目 2 再切回，恢复到约 891px 并写入 `rill.reader.position.1`。

## 未验证 / 阻塞

- 前端 ↔ 真实 Go 后端的 OPML 导入导出、文件夹 CRUD 已完成浏览器 roundtrip；嵌套 3 层 OPML 仍待验收。
- mock 的 OPML 导入导出按扁平文件夹处理；真实后端保留嵌套层级，嵌套 roundtrip 仍需验收。
- LPK 官方 file chooser inject 已打包；真机文件选择、下载拦截与 `/_lzc/files/home` 仍未验收。
- 真实 AI、真实外部源与真机验收仍未完成；源图标、保留策略、FTS5、全站 token reveal 已在后续 v0.15 实现，证据见 REQ-004～007。
