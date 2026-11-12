# REQ-014 阅读列表头收紧与已读状态筛选

- 状态：已完成（本地验证）
- 优先级：P2
- 关联：[DESIGN.md](../DESIGN.md)、[PRD.md](../../PRD.md) F-30/F-35、[PDD.md](../../PDD.md) §6.5
- 影响代码：`web/src/components/reader/ArticleList.tsx`、`web/src/pages/ReaderPage.tsx`、`web/src/stores/readerStore.ts`、`web/src/types/ui.ts`、`web/src/styles/index.css`

## 用户要求

列表头高度仍占用过多；统一按发布时间降序，不显示数量；支持用一个 icon 过滤已读和未读文章。

## 范围与规则

| 项 | 规则 |
| --- | --- |
| 列表头 | 只保留标题、搜索、全部已读和已读状态筛选；移除数量徽标与排序切换。 |
| 高度 | 桌面 `12px 16px 9px`，标题 24px；移动端 `10px 14px 8px`，标题 23px。搜索或源告警展开时顶部间距 8px。 |
| 排序 | 列表统一请求 `sort=published`，按发布时间降序；不提供按抓取时间切换。缺少发布时间时沿用后端 `published_estimated` 规则，排在确定时间之后。 |
| 已读筛选 | 单个图标循环「全部 → 仅未读 → 仅已读 → 全部」；图标依次为 `filter` / `unread` / `read`（漏斗 / 圆点 / 对勾），非全部状态使用强调色激活态，`aria-label` 同时说明当前状态与下一步。 |
| 查询 | 复用现有 `unread=true` / `read=true` 查询参数；不新增 API、表或字段。筛选状态属于当前会话 UI 状态，切换侧栏筛选时保留，刷新后回到「全部」。 |
| 不变 | 搜索、全部标为已读、源/文件夹/收藏筛选、分页和详情操作不变。 |

## 实现落点

- 列表头与三态图标：`web/src/components/reader/ArticleList.tsx`
- 筛选状态与请求参数：`web/src/stores/readerStore.ts`
- 重载依赖：`web/src/pages/ReaderPage.tsx`
- 类型：`web/src/types/ui.ts`（`ReadStatusFilter`）
- 样式：`web/src/styles/index.css`（`v0.10 · 阅读列表头收紧与已读筛选`）

## 验收证据

- `npm run typecheck`：通过。
- `npm test`：当次 6 个测试文件、15 个用例通过；后续 REQ-016 增至 17 个用例。新增 `readerStore.test.ts` 覆盖发布时间降序与仅未读/仅已读查询。
- `npm run build`：通过。
- Playwright 本地 mock 检查（2026-11-01）：
  - 1440px 收藏列表头高度 50.8px，padding `12px 16px 9px`；390px 高度 47px，padding `10px 14px 8px`。
  - 列表头内不存在 `.list-count` 与 `.sort-btn`；操作区为搜索、全部已读、已读状态筛选三个按钮。
  - 收藏列表初始 7 条（4 未读 / 3 已读）；点击筛选图标后未读列表 4 条全部未读，再点后已读列表 3 条全部已读，第三次点击恢复 7 条。
  - 筛选图标在全部 / 仅未读 / 仅已读三态分别为漏斗、圆点、对勾；三态 `aria-label` 正确。
  - 以 30ms 间隔连续点击两次筛选图标，最终图标为「仅已读」，列表 3 条全部已读；取消请求守卫后由 `loadToken` 丢弃旧响应，图标与列表保持一致。
  - 0 个 JS 错误，0 横向溢出。

## 未验证

- 真机、真实外部源数据与真实 Go API 下的筛选组合。
