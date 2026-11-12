# REQ-013 侧栏去除横向分隔线与状态标签

- 状态：已完成（本地验证）
- 优先级：P2
- 关联：[DESIGN.md](../DESIGN.md)、[REQ-011](REQ-011-settings-dialog.md)、[REQ-012](REQ-012-source-grouping.md)
- 影响代码：`web/src/components/layout/Sidebar.tsx`、`web/src/styles/index.css`

## 用户要求

去除侧边栏的横向分隔线和「正常」状态标签，让侧栏更干净。

## 范围与规则

| 项 | 规则 |
| --- | --- |
| 品牌区 | 移除 `.rail-brand` 的下边框；保留紧凑的品牌区，不再显示「正常」胶囊。 |
| 底部工具组 | 移除 `.rail-foot` 的上边框；设置/主题图标组继续保留。 |
| 状态标签 | 桌面侧栏不再渲染 `.rail-live`；抽屉中只保留关闭按钮。 |
| 不变 | 侧栏右侧的纵向分隔线、导航选中态、移动端底部导航和设置弹窗结构不变。 |

## 实现落点

- 侧栏：`web/src/components/layout/Sidebar.tsx`
- 样式：`web/src/styles/index.css`（`/* 侧栏不再使用横向分隔线 / 状态标签 */`）

## 验收证据

- `npm run typecheck`：通过。
- `npm test`：5 个测试文件、14 个用例通过。
- `npm run build`：通过。
- Playwright 本地检查：桌面侧栏不再包含 `.rail-live`；品牌区与底部工具组无横向边框；浅色/深色与 1440/390/320px 无横向溢出、无 JS 错误。

## 未验证

- 真机与外部源页面。
