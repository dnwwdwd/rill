# REQ-015 设置弹窗布局收紧与分类图标重绘

- 状态：已完成（本地验证）
- 优先级：P2
- 关联：[DESIGN.md](../DESIGN.md)、[PDD.md](../../PDD.md) §6.5
- 影响代码：`web/src/components/settings/SettingsDialog.tsx`、`web/src/components/settings/SettingsNav.tsx`、`web/src/components/settings/ReadingSettings.tsx`、`web/src/components/ui/Icon.tsx`、`web/src/types/ui.ts`、`web/src/styles/index.css`

## 用户要求

继续优化设置页面的布局，重新绘制设置菜单的每个图标。

## 范围与规则

| 项 | 规则 |
| --- | --- |
| 弹窗外壳 | 保持 1040×760 上限；标题栏 72px → 66px；左栏 224px → 208px；内容区 padding 24/28/32 → 20/24/28。 |
| 设置导航 | 单项高度 44px → 40px，图标框 30px → 28px，图标 18px；保留强调浅底、实心图标与右侧状态点。 |
| 内容卡片 | 卡片 padding 18/20 → 16/18，圆角 14px → 13px；卡片间距统一为 12px；分区标题与正文间距收紧。 |
| 阅读排版 | 字号、行宽、行高三条滑杆改为桌面三列网格，移动端单列；主题与预览保持左右分栏。 |
| 分类图标 | 新增 `settings-sources` / `settings-reading` / `settings-ai` / `settings-wechat` / `settings-backup` / `settings-security` 六枚专用线性图标，统一 24px 视觉网格、1.6 描边；菜单与对应分区标题共用。 |
| 不变 | 设置分类、保存逻辑、开关、表单和移动端三列导航结构不变；不新增设置项。 |

## 图标语义

| 分类 | 图形 |
| --- | --- |
| 订阅源 | 中心信号点 + 双侧同心弧线 |
| 阅读 | 打开的书页与书脊 |
| AI | 中心节点连接三个外围节点；v0.14 已改为圆环内四角星（见 [REQ-018](REQ-018-reader-polish-and-folder-opml.md)） |
| 公众号 | 带文章行的对话气泡 |
| 备份 | 带向下箭头的归档盒 |
| 安全 | 带锁孔的盾牌 |

## 实现落点

- 弹窗与导航：`web/src/components/settings/SettingsDialog.tsx`、`SettingsNav.tsx`
- 阅读排版网格：`web/src/components/settings/ReadingSettings.tsx`
- 图标与分类映射：`web/src/components/ui/Icon.tsx`、`web/src/types/ui.ts`
- 样式：`web/src/styles/index.css` 设置弹窗区块

## 验收证据

- `npm run typecheck`：通过。
- `npm test`：通过（当前 6 个测试文件、17 个用例）。
- `npm run build`：通过。
- Playwright 本地 mock 检查（2026-11-01）：
  - 1440px：弹窗 1040×760，导航宽 208px、单项高 40px，内容区 830px；阅读排版卡片 295px，阅读行为卡片 175px。
  - 六类菜单均渲染专用图标；菜单图标与分区标题图标一致。
  - 390px / 320px：导航保持 3 列两行，列表控件单列，无横向溢出、无 JS 错误。
  - 设置弹窗六个分类逐一切换，均正常渲染。

## 未验证

- 真机与更窄的设备安全区。
