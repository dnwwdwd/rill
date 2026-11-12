# REQ-003 OPML 导入导出与文件夹 UI

- 状态：已完成（本地验证；LPK file chooser 真机待验收）
- 优先级：P0
- 关联功能：F-03（OPML 导入/导出）、F-26（文件夹分组）
- 事实来源：[PRD.md](../../PRD.md)、[PDD.md](../../PDD.md) §5.2/§8.2、[server/README.md](../../server/README.md)

## 目标

让用户能导入现有阅读器的 OPML、导出全部订阅、在 UI 中创建/重命名/删除文件夹，并把源归入文件夹；嵌套分组不能丢。

## 当前实现

- 后端已实现 `GET /opml/export`、`POST /opml/import`、folders CRUD 与 `folders.parent_id`。
- 后端导入支持嵌套 `<outline>`，删除文件夹后源回落到未分组。
- 前端已提供文件夹创建 / 重命名 / 删除和 OPML 导入 / 导出入口；删除文件夹后源回落未分组。
- 前端使用标准 `<input type=file>`；LPK 已打包官方 file chooser inject，四个 hook 全开，真机选择/下载拦截待验收。
- mock OPML 导入导出仍按扁平文件夹处理；真实后端已通过 3 层 A/B/C roundtrip，前端文件夹树按层级缩进。
- 添加源时若没有匹配文件夹，会显式传 `folder_id=null`，源进入「未分组」。

## 范围

### 必须

- 设置弹窗的「订阅源」分类提供导入 OPML、导出 OPML 的入口；导入使用文件选择（LPK 场景使用官方 file chooser inject）。
- 导入后显示新增文件夹/源与跳过数量，失败给出可读原因。
- 文件夹可创建、重命名、删除；删除文件夹后源进入未分组，不级联删源。
- 设置弹窗的订阅源按文件夹分组展示，并可按文件夹筛选；编辑源能改变文件夹。
- 导出→导入 roundtrip 保留 3 层嵌套分组与标题。

### 不做

- 不新增 OPML 之外的订阅格式。
- 不在本期实现文件夹拖拽排序、批量移动源。
- 不改变公开 OPML token 的安全模型；全站 token 展示仍由 REQ-004 决定。

## 验收矩阵

| 行为 | 验收证据 | 状态 |
|---|---|---|
| 导入含 3 层分组的 OPML | 文件夹树、源数量、层级快照 | 本地验证：后端 3 层测试 + 真实服务 A/B/C roundtrip；UI 文件夹树按层级缩进 |
| 导出后回导 | 标题与层级一致；源不重复或按规则跳过 | 本地验证：3 层导出、重复导入跳过；mock/真实服务均通过 |
| 新建/重命名/删除文件夹 | UI 操作 + `GET /folders` | 本地验证（mock + 真实 Go 服务，2026-11-01） |
| 删除文件夹不删源 | 源 `folder_id=null`，仍可在未分组看到 | 本地验证（mock）；真实后端删除接口已由 folders CRUD test 覆盖 |
| 移动源到文件夹 | 编辑源后侧边栏/筛选同步 | 后端已具备；添加/编辑源已可选文件夹 |
| 失败与无权限错误可见 | 文件格式错误、网络失败提示 | 本地验证（toast） |

真实 Go 服务浏览器验收（2026-11-01）：1 个源导出 `rill-subscriptions.opml` 含 `E2E Feed`；导入 `Real Folder/Imported` 后源数 1→2、分组出现 `Real Folder`；新建 `E2E Folder` 后删除，仅删除该文件夹；0 JS 错误。

三层验证：后端 `TestOPMLNestedImportExportAndPublicToken` 使用 `Tech > Web > Frontend > source`；真实服务导入 `A > B > C > source` 后导出仍保留三层；设置筛选下拉缩进为 `A / 　B / 　　C`。

## 未决问题

1. 导入冲突策略：同名文件夹合并还是新建？同 URL 源跳过还是更新？当前后端有现有行为，需在接入前确认。
2. 删除非空文件夹是否二次确认？是否需要“移动源到父级/未分组”选择？
3. LPK 官方 file chooser inject 已接入 `lzc-manifest.yml` 并打包；真机选择器、下载拦截与 `/_lzc/files/home` 仍需确认。

## 下一步

1. 先读后端 OPML/folders 实现和现有 mock，确认冲突/错误响应。
2. 在前端补导入导出入口与文件夹管理 UI；不新增 schema。
3. ~~用含嵌套分组的样本做 roundtrip 测试~~（已完成：后端 3 层测试 + 真实服务 A/B/C roundtrip）。

## 风险与边界

- 不改变数据语义：文件夹删除只回落到未分组。
- 导入文件属于用户数据，必须校验大小/格式，不执行文件内任意脚本。
