# PROG-REQ-002 微信公众号 sidecar 与扫码

- 关联需求：[REQ-002](../requirements/REQ-002-wechat-sidecar.md)
- 状态：进行中（外部依赖阻塞）
- 最近更新：2026-10-31

## 阶段

| 阶段 | 目标 | 状态 | 交付物 / 证据 |
|---|---|---|---|
| 1 | 设置弹窗开关、sidecar 地址、登录态展示 | 已完成 | 前端设置弹窗；mock 扫码仅用于 UI |
| 2 | 接入真实 `we-mp-rss` 二维码与登录态 | 受阻 | 真实 `POST /wechat/scan` 返回 503 |
| 3 | 从 sidecar 枚举/添加具体公众号 Feed | 待开始 | 当前添加源弹层只跳设置弹窗 |
| 4 | 抓取真实文章、过期重扫、隔离验收 | 待开始 | 需要 sidecar 实机与文章样本 |

## 已完成证据

- `server/internal/server/wechat.go` 明确未联调时返回 `wechat_scan_unavailable`，不伪造成功。
- 后端对 `kind=wechat` 限制为已配置 sidecar 主机；401 会进入登录态错误处理。
- 前端 mock 模式可模拟扫码，但生产按钮不会伪造登录。

## DoD

- 真实扫码成功，登录态写入并可过期/恢复。
- 添加至少一个具体公众号并取得公开文章。
- 登录态过期时点击源暂停、其他源不受影响。
- 主应用日志/数据库/响应不出现微信 Cookie 或扫码 token。
- 在 `docs/RSS-SOURCES.md` §4 记录真实接口证据。

## 阻塞与下一步

- 阻塞：没有真实 sidecar 地址/接口契约/可扫码账号；属于外部依赖。
- 下一步：先获取 `we-mp-rss` 的扫码、轮询、账号 Feed API，写入 REQ 后再实现；在此之前保持不可用状态。
