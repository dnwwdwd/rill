# PROG-REQ-001 X RSS 真实可用

- 关联需求：[REQ-001](../requirements/REQ-001-x-rss.md)
- 状态：进行中
- 最近更新：2026-10-31

## 阶段

| 阶段 | 目标 | 状态 | 交付物 / 证据 |
|---|---|---|---|
| 1 | 建立 handle → FxTwitter Feed → 标准 RSS 的代码链路 | 已完成 | 前端 X 标签页；后端 `kind=x` URL 生成；标准 RSS 抓取 |
| 2 | 在真实环境取得至少 1 条帖子 | 受阻/未通过 | 本机调度抓取返回 `fetch_failed`，未取得真实帖子 |
| 3 | 明确失败降级（自建 FxEmbed 或出站代理） | 待方案确认 | OPEN-001；尚未实现自动切换 |
| 4 | 完成 PRD F-11 验收并更新源登记 | 待开始 | 需要时间、状态码、条目字段和媒体证据 |

## 已完成证据

- `server/internal/server/source_store.go` 将 X handle 转换为 FxTwitter Feed URL。
- 标准 RSS 抓取、条件请求、去重和媒体重写由统一管线处理。
- 本地 `httptest` 覆盖标准 Feed；真实 FxTwitter 未通过。

## DoD

- 真实 handle 取得 ≥1 条帖子，正文、时间、作者、图片/视频链接正确。
- 重复抓取不重复入库，失败隔离和诊断可见。
- 无 X Cookie/Token。
- 更新 `docs/RSS-SOURCES.md` §3 的当前后端验证记录。

## 阻塞与下一步

- 阻塞：微服出口/公共实例连通性未确认；降级方向未确认。
- 下一步：先在目标环境做一次最小 HTTP 探测，记录状态码和响应头，再决定是否进入 REQ-001 阶段 3。
