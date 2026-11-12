# REQ-010 主题色与阅读卡片视觉刷新

- 状态：已完成（本地验证）
- 优先级：P1
- 关联：[DESIGN.md](../DESIGN.md)、[REQ-008](REQ-008-ui-design-system.md)、[PRD.md](../../PRD.md) F-33、[PDD.md](../../PDD.md) §6.5
- 影响代码：`web/`、`server/internal/server/models.go`、`server/internal/server/settings_store.go`

## 目标

1. 阅读中栏的条目行不再放置全屏阅读、收藏、已读操作；这些操作只保留在阅读详情右上角。
2. 阅读详情与专注阅读中的 AI 摘要、翻译卡片使用同一套有层次的卡片语言，不再只是扁平的色块。
3. 增加可保存的主题强调色，默认仍是微信绿；用户可在阅读设置中选择蓝 / 紫 / 橙 / 玫瑰。

## 范围与规则

| 项 | 规则 |
| --- | --- |
| 列表行 | 移除 hover/选中时出现的 `.item-actions`；保留源、时间、标题、摘要和收藏标记。双击条目仍进入专注阅读。 |
| 详情操作 | 全屏、收藏、标为已读/未读继续只在详情右上角提供。 |
| AI 摘要 | 图标方块 + 标题 + 折叠箭头；编号圆点列表；底栏保留「内容已发送至外部模型」和重新生成；空态保留外发确认提示。 |
| 翻译 | 图标方块 + 标题 + 已生成标记 + 译文/原文/双语分段；过期提示独立显示；不新增装饰性说明。 |
| 主题色 | `green`（默认）/ `blue` / `purple` / `orange` / `rose`；色板选择，无需输入自定义色值。 |
| 语义色 | 主题色只影响强调色层（按钮、链接、开关、未读、进度、选中背景）；成功/警告/危险、正文和纸张层次不随主题色改变。 |
| 源图标 | 改用中性石板灰哈希色，避免和任意主题色冲突。 |

## 数据与接口

- 设置字段：`Settings.reader.accent`，JSON 枚举值 `green | blue | purple | orange | rose`。
- 默认值：后端 `defaultInternalSettings()` 与前端默认设置均为 `green`；读取旧设置时缺失或空值归一为 `green`。
- 校验：`PATCH /api/v1/settings` 的 reader patch 接受 `accent`；非法值返回 `400 invalid_settings`，文案为「主题色只能是 green、blue、purple、orange 或 rose」。
- 持久化：沿用 `settings` 表的 JSON 值，**不新增表、字段、索引或迁移**。
- 首屏：`localStorage['rill.accent']` 只用于内联脚本预置 `data-accent` 防闪烁；加载后端设置后以后端值为准。

## 实现落点

- 前端类型与默认值：`web/src/api/types.ts`、`web/src/stores/defaults.ts`
- 主题应用：`web/src/stores/settingsStore.ts`、`web/src/hooks/useAppTheme.ts`、`web/index.html`
- 设置色板：`web/src/components/settings/ReadingSettings.tsx`
- 样式与预设：`web/src/styles/index.css`
- 列表行：`web/src/components/reader/ArticleRow.tsx`
- 卡片：`web/src/components/reader/AiSummary.tsx`、`web/src/components/reader/TranslationPanel.tsx`
- 后端：`server/internal/server/models.go`、`server/internal/server/settings_store.go`、`server/internal/server/settings_test.go`

## 验收证据

- `npm run typecheck`：通过。
- `npm test`：5 个测试文件、14 个用例通过（新增主题色即时应用和持久化用例）。
- `go test ./...`：通过（settings 测试覆盖合法 accent 返回值和非法 accent 400）。
- 真实 Go API 本地请求：登录后 `PATCH /api/v1/settings` 设 `orange` 返回 200，GET 读取为 `orange`；非法 `pink` 返回 400 和预期文案；测试后恢复原值 `green`。
- Playwright 本地检查：五套主题色在浅色/深色、1440/390/320px 下可切换并持久化；阅读详情、专注阅读与设置弹窗无 JS 错误和横向溢出。
- 视觉检查：列表行 `.item-actions` 数量为 0；详情右上角操作组仍有 6 个图标按钮；AI/翻译卡在普通阅读和专注右栏均生效。

## 不包含

- 任意自定义颜色、按用户分主题、颜色导入导出。
- 真机 LPK、OIDC、外部源页面验收。
- 修改未读、收藏或已读的数据语义。
