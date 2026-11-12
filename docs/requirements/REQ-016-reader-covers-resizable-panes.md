# REQ-016 条目封面、源信息行未读点与可调栏宽

- 状态：已完成（本地验证）
- 优先级：P2
- 关联：[DESIGN.md](../DESIGN.md)、[PRD.md](../../PRD.md) F-07/F-33、[PDD.md](../../PDD.md) §4.2/§5.4/§6.5
- 影响代码：`web/src/components/reader/ArticleRow.tsx`、`web/src/components/reader/ArticleList.tsx`、`web/src/components/layout/Sidebar.tsx`、`web/src/components/layout/ResizeHandle.tsx`、`web/src/stores/defaults.ts`、`web/src/stores/settingsStore.ts`、`web/src/styles/index.css`、`server/internal/server/items.go`、`server/internal/server/models.go`、`server/internal/server/settings_store.go`

## 用户要求

1. 已读/未读 dot 与订阅源 name 这一行垂直居中（用户后续澄清，不是与标题居中）。
2. 有封面的订阅条目在卡片右侧显示图片封面。
3. 左侧 sidebar 默认宽度调小；左右两个 sidebar 都可调整宽度，并持久化保存、自动应用。

## 范围与规则

### 未读点

- 未读点固定在源信息行的左侧空白区：`left: -11px`，大小为 6px；以 `top: 50% + translateY(-50%)` 与订阅源 name 行垂直居中。
- 未读点不占据正文布局；标题与订阅源 name 左缘保持一致（同一内容列）。
- 未读时显示强调色，已读时透明；不再使用标题行内的 flex 占位点。

### 封面

- `ItemSummary` 新增可选 `cover_url`，不新增数据库列或迁移。
- 列表查询从 `content_html` 前 16KB 中提取第一个 `/media/` 图片地址；无图返回 `null`，卡片不显示封面占位。
- 前端封面位于卡片右侧，宽度为内容区 24%，限制 72–96px，4:3；`loading="lazy"`，加载失败自动移除。
- 封面只是列表缩略图，详情页仍渲染完整正文；媒体仍由 `/media/{token}/{hash}` 代理。
- Mock 使用 `web/public/covers/` 下的 4 张本地 SVG 演示封面。

### 可调栏宽

| 项 | 默认 | 范围 | 存储 |
| --- | --- | --- | --- |
| 左侧栏 | 224px | 180–320px | `settings.reader.rail_width` |
| 文章列表 | 384px | 300–520px | `settings.reader.list_width` |

- 两个栏位右边界各有拖拽手柄；拖动时直接更新 CSS 变量即时预览，松手写回后端 settings。
- 键盘支持：左右方向键 ±12px，Home/End 跳到边界；双击恢复默认。
- 首屏优先读取 `localStorage['rill.rail_width']` / `['rill.list_width']` 防跳变，后端设置加载后为准。
- 服务端校验范围；旧设置缺少字段或越界时回落到 224 / 384。
- 移动端与抽屉侧栏不显示拖拽手柄，列表宽度回到 100%。

## 实现落点

- 条目行与封面：`web/src/components/reader/ArticleRow.tsx`
- 列表手柄：`web/src/components/reader/ArticleList.tsx`
- 侧栏手柄：`web/src/components/layout/Sidebar.tsx`、`web/src/components/layout/ResizeHandle.tsx`
- CSS 变量与布局：`web/src/stores/settingsStore.ts`、`web/src/stores/defaults.ts`、`web/src/styles/index.css`
- 封面 API：`server/internal/server/items.go`、`server/internal/server/models.go`
- 宽度设置：`server/internal/server/settings_store.go`、`web/src/api/types.ts`

## 验收证据

- `npm run typecheck`：通过。
- `npm test`：6 个测试文件、17 个用例通过；覆盖 mock 封面和栏宽持久化。
- `npm run build`：通过。
- `go test ./...`：通过；新增封面提取测试，抓取测试断言列表与详情返回 `cover_url`，设置测试覆盖栏宽合法值与越界拒绝。
- Playwright 本地 mock 检查（2026-11-01）：
  - 1440px 初始左侧栏 224px、列表 384px；两个手柄均可见。
  - 未读点 x=233、中心 y=74；订阅源 name 行中心 y=74，标题与源 name 左缘均为 244；双行标题下点仍停留在源信息行。已读点透明。
  - 3 张封面正常加载，宽度 83px；列表加宽后封面为 95px。
  - 拖拽左侧栏至 260px、列表至 436px 后刷新页面，宽度与手柄值保持 260 / 436。
  - 390px 下手柄隐藏，列表宽 390px；0 个 JS 错误、0 横向溢出。

## 未验证

- 真实外部 Feed 的封面选取准确率；首图可能是头像或跟踪像素，后续可改为显式封面字段。
- 媒体代理未按缩略图尺寸压缩，列表仍请求原图。
- 前端 ↔ 真实 Go API 的栏宽回写与封面加载端到端；真机触控拖拽与窄屏安全区。
