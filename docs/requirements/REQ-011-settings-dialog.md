# REQ-011 侧栏收敛与设置弹窗

- 状态：已完成（本地验证）
- 优先级：P1
- 关联：[DESIGN.md](../DESIGN.md)、[REQ-008](REQ-008-ui-design-system.md)、[REQ-010](REQ-010-theme-color.md)、[PRD.md](../../PRD.md) F-01/F-26/F-30/F-33、[PDD.md](../../PDD.md) §4/§6.5
- 影响代码：`web/src/components/layout/`、`web/src/components/settings/`、`web/src/pages/SettingsPage.tsx`、`web/src/app/router.tsx`

## 用户要求

1. 左侧只显示「全部」和「收藏」，不显示收件箱、未读、已读或订阅源树。
2. 订阅源管理放进设置。
3. 设置改为左下角图标入口，并替换原来的图标。
4. 深浅色切换与设置并排放在左下角，不单独占一行。
5. 设置改为弹窗，不再作为独立页面；弹窗菜单选中态需要有质感。

## 范围与规则

| 项 | 规则 |
| --- | --- |
| 主侧栏 | 仅「全部 / 收藏」两项；全部显示总条目数，收藏显示收藏数；移除订阅源树和「订阅源」入口。 |
| 左下角 | 一个紧凑圆角容器并排放置设置齿轮和浅色/深色图标按钮；两者都是 32px 图标按钮，带 `title` 与 `aria-label`。 |
| 设置入口 | 桌面侧栏、移动底栏、移动抽屉都打开 `SettingsDialog`；不渲染独立设置页面。 |
| 设置弹窗 | 桌面 1040px 宽浮层、20px 圆角、左侧分类栏 + 右侧滚动内容；移动端全屏、分类为 3×2 网格。 |
| 分类 | 订阅源 / 阅读 / AI / 公众号 / 备份 / 安全；选中项用强调浅底、实心强调图标和右侧 5px 圆点，不使用左侧色条。 |
| 订阅源 | 状态筛选、文件夹筛选、搜索、按文件夹分组的源行；保留添加入口、启停、立即刷新与进入源诊断。 |
| 兼容入口 | `/settings` 与 `/settings/:section` 打开弹窗后回到阅读页；`/sources` 打开订阅源分类后回到阅读页。 |
| 状态 | 设置弹窗开关与当前分类存内存；主题选择仍按 REQ-010 持久化到 `settings.reader.accent`。 |

## 不做

- 不把源诊断页面塞进弹窗；它仍是独立诊断页，返回时打开设置里的订阅源分类。
- 不恢复侧栏的未读/已读/来源筛选入口；后端条目查询能力保持不变。
- 不新增数据库表、字段或迁移。

## 实现落点

- 侧栏：`web/src/components/layout/Sidebar.tsx`
- 移动导航：`web/src/components/layout/MobileNav.tsx`
- 设置弹窗：`web/src/components/settings/SettingsDialog.tsx`
- 分类导航：`web/src/components/settings/SettingsNav.tsx`
- 订阅源设置：`web/src/components/settings/SourcesSettings.tsx`
- 兼容路由：`web/src/pages/SettingsPage.tsx`、`web/src/pages/LegacySourcesPage.tsx`、`web/src/app/router.tsx`
- 样式：`web/src/styles/index.css`（v0.6 侧栏收敛与设置弹窗段）

## 验收证据

- `npm run typecheck`：通过。
- `npm test`：5 个测试文件、14 个用例通过。
- `npm run build`：通过；生产包包含设置兼容入口与订阅源设置组件。
- Playwright 本地检查（最终 0 failures / 0 JS errors）：
  - 1440 / 1024 / 900 / 768 / 390 / 320px、浅色/深色：阅读、详情、专注、源诊断无横向溢出；设置弹窗无横向溢出。
  - 侧栏只有「全部 / 收藏」，旧 `.rail-tree`、`.rail-section-head` 数量为 0；左下角工具组为 2 个图标按钮。
  - 设置弹窗 6 个分类、订阅源 20 行、5 个文件夹分组；搜索与文件夹筛选可用（技术组 7 行）；390/320px 分类完整可见。
  - 设置菜单选中项切换正常；Esc 关闭；遮罩点击关闭。
  - 添加订阅源弹窗层级在设置弹窗之上；源诊断返回后回到订阅源分类。
  - 移动底栏只有「全部 / 收藏 / 设置」；主题切换按钮与设置按钮在同一工具组。
- 兼容入口 `/settings/sources`、`/sources` 均打开弹窗并落到 `/`。

## 未验证

- 真机 LPK、OIDC、外部源页面；真实后端数据下的订阅源滚动与刷新。
- 设置弹窗未实现完整焦点陷阱（只用 Esc / 点击遮罩关闭）；如后续需要严格键盘模态，再单独补。
