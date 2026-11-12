# REQ-012 新增源可选分组与侧栏品牌收紧

- 状态：已完成（本地验证）
- 优先级：P2
- 关联：[DESIGN.md](../DESIGN.md)、[REQ-011](REQ-011-settings-dialog.md)、[PRD.md](../../PRD.md) F-01/F-26
- 影响代码：`web/src/components/sources/AddSourceDialog.tsx`、`web/src/styles/index.css`

## 用户要求

1. 新增订阅源时可以选择是否分组，以及归入哪个文件夹。
2. 侧栏顶部的品牌图标与品牌容器高度再收紧，减少占用。

## 范围与规则

| 项 | 规则 |
| --- | --- |
| 分组选择 | URL / X 添加 tab 显示「分组」下拉；选项为「不分组」+ 现有文件夹；默认「不分组」。 |
| 提交 | 将选择结果写入 `SourceInput.folder_id`（`null` 表示不分组）；公众号 tab 不创建源，不显示该字段。 |
| 默认行为 | 不再自动把 RSS 放进「技术」或把 X 放进「关注」；用户明确选择后才分组。 |
| 结果 | 新源出现在对应文件夹组；选择「不分组」时出现在「未分组」。 |
| 侧栏顶部 | 品牌区 padding 收紧到 `14px 16px 9px`；Wordmark 24px、印章 22px；状态标签已由 REQ-013 移除。品牌区高度约 51px。 |
| 数据 | 沿用现有 `sources.folder_id`；不新增表、字段或迁移。 |

## 实现落点

- 添加源：`web/src/components/sources/AddSourceDialog.tsx`
- 样式：`web/src/styles/index.css`（侧栏品牌区与 `.add-folder-field`）
- 需求链：`PRD.md` F-01/F-26、[REQ-011](REQ-011-settings-dialog.md)

## 验收证据

- `npm run typecheck`：通过。
- `npm test`：5 个测试文件、14 个用例通过。
- `npm run build`：通过。
- Playwright 本地检查：
  - 添加源默认分组为「不分组」，选项为「不分组 / 技术 / 设计 / 资讯 / 公众号 / 关注」。
  - X 源选择「技术」后出现在「技术」组；第二个 X 源保持默认，出现在「未分组」。
  - 添加源弹窗在 1440 / 390 / 320px 无横向溢出、无 JS 错误。
  - 侧栏品牌区高度约 51px，顶部导航随之上移。

## 未验证

- 真实后端外部源创建、文件夹删除/重命名后的边界；真机 LPK 与 OIDC 仍未验收。
