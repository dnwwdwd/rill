# REQ-027 AI 摘要与翻译流式输出

- 状态：已实现（2026-11-02，真实供应商流式待验收）
- 关联：DEC-018 之外的 AI 交互增强；[PDD](../PDD.md) §6 API 列表、[REQ-025](REQ-025-2026-11-02-batch.md)

## 目标

1. AI 摘要按 token 流式显示，翻译按段落流式显示，带入场动画与光标。
2. 结果生成后立即持久化；重新打开文章直接显示已保存的摘要/译文，不重复生成。
3. 点击阅读页翻译 icon 且尚无译文时，直接开始翻译（不再先弹设置再点一次）。
4. mock 模式行为与真实后端一致：刷新后保留 AI 结果。

## 接口

```text
POST /api/v1/items/{id}/summary/stream
  data: {"type":"delta","text":"..."}
  data: {"type":"done","summary":{...}}
  data: {"type":"error","code":"...","message":"..."}

POST /api/v1/items/{id}/translation/stream
  data: {"type":"segment","index":0,"text":"..."}
  data: {"type":"progress","done":1,"total":12}
  data: {"type":"done","translation":{...}}
  data: {"type":"error","code":"...","message":"..."}
```

- 既有非流式端点保留，作为兼容/降级路径。
- 摘要走 OpenAI 兼容 `stream: true`；供应商忽略该参数时自动退化为单次 delta。
- 翻译复用现有「分段 JSON 数组」协议，每批 4 段，完成后重建 HTML 并落库。
- 摘要/译文在 `done` 前写入 `ai_results`，随后才推送 `done`。

## 前端

- `readerStore` 新增 `summaryStream`（累积 token）与 `translationStream`（段落/进度）。
- `AiSummary` 流式渲染要点、`生成中` 状态与闪烁光标；翻译在正文区显示段落级流式预览，完成后切换为最终 HTML。
- 动画：要点/段落淡入上移、光标闪烁、状态脉冲；`prefers-reduced-motion` 下全部关闭。
- 打开文章时强制刷新详情（`loadDetail(id, true)`），确保跨会话生成的结果直接显示。
- mock 端模拟 token/段落推送，并把 AI 结果写入 `localStorage`。

## 验收

- Go：`TestAIStreamingEndpoints` 覆盖 delta/segment/done、缓存命中不重复调用、结果落库。
- 前端：typecheck、26 测试、build；Playwright mock 验证流式预览、完成后切换、刷新后持久化。
- 待验收：真实 OpenAI 兼容供应商的 SSE（含 `[DONE]` 与 usage 形态）、真实浏览器动画观感。
