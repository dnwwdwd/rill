# REQ-023 设置页从 0 到 1 重设计（紧凑行式 + 五类重组）

- 状态：进行中
- 优先级：P1
- 关联：[DESIGN.md](../DESIGN.md)、[REQ-011](REQ-011-settings-dialog.md)（被本需求部分取代）、[REQ-015](REQ-015-settings-layout-icons.md)（被本需求取代）、[REQ-021](REQ-021-2026-11-01-batch.md) ⑥
- 影响代码：`web/src/components/settings/*`、`web/src/pages/SettingsPage.tsx`、`web/src/types/ui.ts`、`web/src/styles/index.css`、`web/src/components/ui/Icon.tsx`

## 用户要求

从 0 到 1 设计设置页面的样式和布局，推翻现有样式与布局，不参考不借鉴现有设置页的布局样式；实现前先理清需求和业务；完成后本地启动前后端服务。

2026-11-01 用户决策：

| 决策点 | 结论 |
| --- | --- |
| 桌面载体 | 保留弹窗形式（不改为全屏页）；移动端仍是独立页面 |
| 页面骨架 | 左目录 + 右工作区的空间关系保留，视觉与交互完全重绘 |
| 视觉方向 | 由实现方决定：采用「紧凑行式工具风」（见下） |
| 分类重组 | 六类重组为五类（公众号并入订阅源） |

## 需求与业务盘点（重设计依据）

设置域六类功能全集（重设计必须 1:1 保留数据与操作能力）：

1. 订阅源：源树（多级分组折叠）、添加/编辑/启停/刷新/删除、状态筛选、搜索、OPML 导入导出、分组管理、登录态过期告警。
2. 阅读：主题（浅/深/跟随系统）、5 套主题色、字号/行宽/行高滑杆 + 实时预览、停留已读、记忆位置。
3. AI：开关、接口地址/模型/密钥（加密存储、按需替换）、测试连接、摘要样式、每日上限与用量、目标语言、内容外发确认。
4. 公众号：采集开关、we-mp-rss 伴生服务连接（地址/账号/密码）、扫码登录与轮询、登录态展示、公众号源列表。
5. 备份：每日自动备份、保留份数、最近备份时间、立即备份/从文件恢复（破坏性）、备份范围说明。
6. 安全：账号与会话（OIDC/密码模式）、退出登录/改密码（密码模式）、Feed 令牌（按需显示/复制/重置）、安全边界说明。

业务归类问题：公众号是「内容采集渠道」，与订阅源同属内容接入；账号信息与令牌同属账号安全。

事实修正：安全页「SSRF 默认拒绝私网地址」文案与 DEC-015（移除私网拦截，仅保留 scheme 校验）矛盾，重设计时改为「出站抓取仅允许 http/https」。

## 新设计规约

视觉方向：**紧凑行式工具风**。13px 行式控件、细分隔线、无卡片、无图标方块、无投影；沿用 Rill 纸墨/微信绿 token 与现有共享组件（Button/Switch/Segmented/Field/SearchField/Notice/Tag/StatusBadge 等）。与现行「大衬线标题 + 图标方块 + 卡片分组」完全相反。

| 项 | 规则 |
| --- | --- |
| 弹窗外壳 | 960px 宽（上限 92vw）、高 min(720px, 88vh)、14px 圆角；**取消顶部标题栏**，左列顶部放「设置」字标，关闭按钮悬浮于内容区右上 |
| 左目录 | 200px，surface-2 纸色，1px 右分隔线；纯文字行（14px、42px 高、9px 圆角，无图标）；选中 = accent-soft 底 + 强调色文字，无箭头、无圆点、无边框堆叠 |
| 工作区 | 白面；内容按「行组」组织：组头为 11px 弱化 eyebrow（前缀 12×2px 强调色短线）+ 可选组标题；行高 44px，左标签右控件，行间 1px 细线，组间 28px 留白 |
| 分类 | 五类：订阅源（含公众号接入）/ 阅读 / AI / 备份 / 安全；`SELECTABLE_SETTINGS` 去掉 `wechat` |
| 公众号接入 | 并入订阅源分类，作为「公众号接入」行组：启用开关、伴生服务连接（可折叠表单）、扫码登录（默认折叠，展开显示二维码/状态/轮询）；删除与源树重复的公众号源列表（源树已含公众号源与状态，点击仍进源诊断） |
| 订阅源工作区 | 工具行（搜索 + 添加源 + 更多菜单：管理分组/导入/导出）→ 登录态告警 → 源树行式列表（行高 40px、每级缩进 18px、hover 显示行操作，启停 switch 常驻） |
| 阅读工作区 | 外观（主题 Segmented、主题色色板、内联预览）/ 排版（三条滑杆纵向单列）/ 行为（两个开关行） |
| AI 工作区 | 能力（开关、地址/模型、密钥、测试连接 + 结果）/ 摘要（样式、每日上限、今日用量 meter）/ 翻译（目标语言、外发确认行） |
| 备份工作区 | 自动备份（开关、保留份数、最近备份）/ 手动操作（立即备份、从备份恢复）/ 范围与风险说明；恢复为危险操作行组 |
| 安全工作区 | 账号与会话（OIDC/密码模式、退出登录、改密码表单）/ Feed 令牌（masked、显示/复制/隐藏/重置、公开路径）/ 安全边界（修正 SSRF 文案） |
| 移动端 | `/settings` 改两级：根页为五个分类大行列表，`/settings/:section` 为详情页（返回头 + 同一工作区组件）；废除 3×2 网格导航 |
| 命名空间 | 新样式类统一 `st-*`；删除旧设置专属 CSS 块（settings-dialog/nav/panel/section、sources-settings、source-tree、wechat 登录卡等），共享组件样式不动 |
| 不变 | 设置数据结构、保存逻辑、mock 数据、OIDC/密码模式行为、所有既有 API；不新增设置项 |

## 实现落点

- 壳与导航：`SettingsDialog.tsx`、`SettingsNav.tsx`（重写，st-* 类）
- 行组基础件：`SettingsSection.tsx` 重写为 `RowGroup`/`Row` 语义（保留 `SliderField`）
- 五类工作区：`SourcesSettings.tsx`（合并公众号接入，`WechatSettings.tsx` 改为渠道行组组件）、`ReadingSettings.tsx`、`AiSettings.tsx`、`BackupSettings.tsx`、`SecuritySettings.tsx`
- 移动端：`SettingsPage.tsx` 两级结构
- 分类映射：`types/ui.ts`；`openSettings('wechat')` 调用点改 `openSettings('sources')`（`SourcesSettings.tsx`、`SourceHealthPage.tsx`）
- 样式：`web/src/styles/index.css`

## 2026-11-01 追加调整（用户反馈）

| 项 | 决策 |
| --- | --- |
| 设置菜单图标 | 桌面目录与移动端目录全部去除分类图标，只留文字（`SETTINGS_SECTIONS` 去掉 `icon` 字段；移动端保留右侧 chevron） |
| 设置菜单尺寸 | 目录项 34px→42px 高、13px→14px 字号，目录列 176px→200px，弹窗高 `min(720px, 88vh)`→`min(780px, 90vh)` |
| 正文标题横线 | 去除文章标题下的 42×2px 强调色短线（原 `.article-title::after`，含移动端覆盖）；正文末尾的操作区分隔线不属于此范围，保留 |
| 设置组头短线 | 去除全部 RowGroup 组头的 12×2px 强调色前缀短线（`.st-group-title::before`），组头只留文字 |
| 添加源弹窗 | 「新建分组」按钮与「分组」下拉同高同字号（38px / 13px） |

## 验收证据

2026-11-01 本地验证：

- `npm run typecheck`：通过。
- `npm run test`：9 个测试文件 26 个用例全部通过（含 `SourcesSettings.test.tsx` 源树折叠、行操作、编辑弹窗用例）。
- `npm run build`（vite build + tsc）：通过。
- 旧设置专属 CSS 选择器清理后扫描无残留（`qr-grid` / `qr-cell` / `qr-image` 为新样式重定义）。
- 本地服务自检：`go run ./cmd/rill` 监听 `127.0.0.1:8080`，`/healthz` 200；`VITE_USE_MOCK=false npm run dev` 监听 `127.0.0.1:5173`，页面 200，经 vite 代理的 `/healthz` 200，`/api/v1/settings` 未登录返回 401。

## 未验证

- 真实浏览器视觉验收：桌面弹窗五类切换、源树启停/删除确认、公众号扫码展开与轮询、移动端两级导航、320px 无横向溢出、深色主题下的新设置样式。
- LPK 内样式回归（本需求仅改前端，未重新打包）。
