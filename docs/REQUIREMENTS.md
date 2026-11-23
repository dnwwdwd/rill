# 需求台账

> 从 2026-10-31 治理基线开始追踪新增或仍在推进的非 Bug 需求。已有功能不强制倒补历史 REQ。
>
> 导航见 [`DOCUMENT_MAP.md`](../DOCUMENT_MAP.md)，交付基线见 [`PROGRESS.md`](PROGRESS.md)。

## 状态

- `进行中`：已有明确范围，正在实现或部分实现。
- `待开发`：范围已确认，尚未开始。
- `待拆分需求`：方向明确，但还没有可执行 REQ。
- `待澄清`：事实或安全/接口边界不足，不能确定范围。
- `不纳入`：已明确排除，并记录原因。
- `已完成`：实现与验证证据已回填，且没有未决的验收项。

## 当前需求

| ID | 需求 | 优先级 | 状态 | 范围 / 当前事实 | 关联文档 | 下一步 |
|---|---|---|---|---|---|---|
| REQ-001 | X RSS 真实可用 | P0 | 进行中 | handle → FxTwitter Feed → 标准 RSS 的代码链路已存在；真实 fxtwitter 抓取未通过 | [REQ-001](requirements/REQ-001-x-rss.md)、PRD F-11/F-27/F-29/F-42、PDD §7.2 | 确认微服出口；实测抓取；决定出站代理/自建 FxEmbed 降级 |
| REQ-002 | 微信公众号 sidecar + 扫码 | P1 | 不纳入（2026-11-03 由 DEC-024 移除 sidecar 模式） | sidecar 扫码占位曾存在，真实扫码始终 503；sidecar 通道已随微信后台一并移除 | [REQ-002](requirements/REQ-002-wechat-sidecar.md)、[REQ-034](requirements/REQ-034-weread-only-channel.md) | 无；公众号走微信读书唯一通道 |
| REQ-003 | OPML 导入导出 + 文件夹 UI | P0 | 已完成（本地验证） | 后端 folders CRUD/嵌套 OPML 已实现；前端支持多级文件夹、缩进分组/筛选、创建/重命名/移动/删除，OPML 导入导出与 3 层 roundtrip 通过；LPK 已接入官方 file chooser inject | [REQ-003](requirements/REQ-003-opml-folder-ui.md)、PRD F-03/F-26 | 真机 file chooser 验收 |
| REQ-004 | 全站 Feed/OPML 完整 token 读取 | P0 | 已完成（本地验证） | 新增 CSRF 保护的 `POST /settings/feed-token/reveal`；普通设置只返回 masked；UI 按需显示/复制/隐藏，响应 no-store，不落库 | [REQ-004](requirements/REQ-004-full-feed-token.md)、PRD F-10/F-14、PDD §5.2/§12 | 剪贴板复制、外部阅读器与真机 |
| REQ-005 | 源图标抓取与缓存 | P0 | 已完成（本地验证） | Feed 图 → 站点 icon link → favicon；经 fetch.Client 抓取（scheme 校验，不拦私网，DEC-015），7 天缓存到 `cache/icons/<id>`；8 MB / 500 文件 LRU；API 返回同源受保护路径；失败回退首字母 | [REQ-005](requirements/REQ-005-source-icons.md)、PRD F-45、PDD §11 | 真实外部站点与慢响应验收 |
| REQ-006 | 条目保留策略 | P0 | 已完成（本地验证） | 每源 `retention_days` / `retention_max_items`；只删除已读未收藏；调度启动后及每 6 小时执行；迁移 2 | [REQ-006](requirements/REQ-006-retention.md)、PRD F-28、PDD §9.5 | 长跑、媒体回收与真机 |
| REQ-007 | FTS5 全文搜索 | P1 | 已完成（本地验证） | 迁移 3 建 `items_fts`（trigram）+ 触发器；≥3 字走 MATCH，短查询/失败回退 LIKE；源名称仍 LIKE | [REQ-007](requirements/REQ-007-fts5-search.md)、PRD F-39、PDD §9 | 大数据量与多语言验收 |
| REQ-008 | 视觉系统刷新与黑白微信绿主题 | P1 | 已完成（本地验证） | 黑白 + 微信绿 token、基础组件、图标与布局统一；阅读中栏去重复筛选；按钮无投影；非必要说明文案已收敛 | [REQ-008](requirements/REQ-008-ui-design-system.md)、[DESIGN.md](DESIGN.md) | 真机/外部源页面再验收 |
| REQ-009 | 全页面 Mock 数据 | P2 | 已完成（本地验证） | 登录、阅读、详情、专注、设置内订阅源、20 个源诊断、6 个设置分类、添加源、404 均有演示数据 | [REQ-009](requirements/REQ-009-mock-data.md)、`web/src/api/mock/seed.ts` | 随新增页面同步维护 |
| REQ-010 | 主题色与阅读卡片视觉刷新 | P1 | 已完成（本地验证） | 中栏行操作移除；AI/翻译卡重做；5 套主题色默认微信绿，设置项存 `settings.reader.accent`，无 DDL 迁移 | [REQ-010](requirements/REQ-010-theme-color.md)、[DESIGN.md](DESIGN.md) §主题色预设 | 真机/外部源页面再验收 |
| REQ-011 | 侧栏收敛与设置弹窗 | P1 | 已完成（本地验证） | 主侧栏仅全部/收藏；订阅源迁入设置；设置改弹窗；左下角设置/深浅色合并为图标组 | [REQ-011](requirements/REQ-011-settings-dialog.md)、[DESIGN.md](DESIGN.md) §页面结构 | 真机/外部源页面再验收 |
| REQ-012 | 新增源可选分组与侧栏品牌收紧 | P2 | 已完成（本地验证） | 添加源可选「不分组」或现有文件夹，默认不分组；侧栏品牌区高度收紧至 52px | [REQ-012](requirements/REQ-012-source-grouping.md)、PRD F-01/F-26 | 真机/外部源页面再验收 |
| REQ-013 | 侧栏去除横向分隔线与状态标签 | P2 | 已完成（本地验证） | 桌面侧栏隐藏「正常」胶囊；品牌区与底部工具组去掉横向边线 | [REQ-013](requirements/REQ-013-sidebar-clean.md)、[DESIGN.md](DESIGN.md) §页面结构 | 真机/外部源页面再验收 |
| REQ-014 | 阅读列表头收紧与已读状态筛选 | P2 | 已完成（本地验证） | 列表头移除数量与排序，固定发布时间降序；单图标（漏斗/圆点/对勾）循环全部/未读/已读，复用 `unread`/`read` 查询参数 | [REQ-014](requirements/REQ-014-reader-list-controls.md)、PRD F-30/F-35、[DESIGN.md](DESIGN.md) §页面结构 | 真机/外部源页面再验收 |
| REQ-015 | 设置弹窗布局收紧与分类图标重绘 | P2 | 已完成（本地验证） | 导航 208px/40px 行高；阅读排版滑杆三列；六枚专用分类图标用于菜单与分区标题 | [REQ-015](requirements/REQ-015-settings-layout-icons.md)、[DESIGN.md](DESIGN.md) §组件规范 | 真机再验收 |
| REQ-016 | 条目封面、源信息行未读点与可调栏宽 | P2 | 已完成（本地验证） | 未读点与订阅源 name 行垂直居中；列表右侧显示可选封面；左右栏默认 224/384，可拖动、可键盘调整并持久化 | [REQ-016](requirements/REQ-016-reader-covers-resizable-panes.md)、PRD F-07/F-33、PDD §4.2/§5.4/§6.5 | 真实外部源封面准确率与真机触控再验收 |
| REQ-017 | 详情页操作收敛与翻译上移 | P2 | 已完成（本地验证） | 移除底部重复操作；桌面工具栏新增翻译浮层；三点「源诊断」改为来源名称链接；已读筛选图标回退 | [REQ-017](requirements/REQ-017-detail-actions-translation.md)、PRD F-07/F-46、[DESIGN.md](DESIGN.md) §页面结构 | 真实 AI 翻译与真机验收 |
| REQ-018 | 详情阅读体验、设置图标与文件夹 / OPML | P1 | 已完成（本地验证，收尾待验收） | 点击未读立即已读；时间/来源/作者同排；AI 摘要常显；右侧进度与回到顶部；AI 图标重绘；阅读位置恢复；文件夹管理 + OPML 导入导出 | [REQ-018](requirements/REQ-018-reader-polish-and-folder-opml.md)、PRD F-03/F-07/F-26、[DESIGN.md](DESIGN.md) §页面结构 | 真机验收 |
| REQ-019 | 懒猫 LPK 打包与 OIDC 登录 | P0 | 已安装（真实浏览器 OIDC 待验收） | 应用内 OIDC 授权码 + PKCE；生产禁用密码登录；登录页收敛；favicon/登录/包图标统一；LPK V2 清单、构建脚本与官方 file chooser inject；真机容器 healthy | [REQ-019](requirements/REQ-019-lazycat-lpk-oidc.md)、PRD F-20/F-47、PDD §12.3/§13 | 打开 `https://rill.example.com` 验收真实 OIDC 回调与 file chooser |
| REQ-020 | 阅读与订阅源 UI 细化 | P0 | 已完成本地构建与升级安装（真实浏览器视觉待验收） | 绿色统一图标；方形封面与标题同排；中栏/topbar 密度；登录手动触发；正文标题与 AI 图标；正文去平台标签；单输入添加源 + 自动同步；订阅源 filetree；源详情质感；icon tooltip；环形进度 | [REQ-020](requirements/REQ-020-reader-source-ui-refinement.md)、PRD F-01/F-03/F-07/F-33/F-47、[DESIGN.md](DESIGN.md) | 真实浏览器视觉与 X 自动同步验收 |
| REQ-021 | 2026-11-01 体验与源能力批次 | P0 | 已完成（本地验证） | ① 移除 SSRF/私网拦截（DEC-015）；② 定时抓取失败重试 + 备用源自动接管 + 取消 fail_count≥5 冻结；③ `rsshub://` 链接与自定义 RSSHub 实例支持；④ Bluesky 官方 RSS 订阅（添加源识别 bsky.app 链接/@handle）；⑤ 公众号接入真实 we-mp-rss 扫码代理（JWT 缓存/二维码 data URL/状态轮询，sidecar 密码加密存储）；⑥ 订阅源设置精简（更多菜单、去筛选行、分组行收敛）；⑦ 阅读进度环与回顶按钮减半；⑧ 备份保留文案澄清；⑨ 视觉走查第二批：正文标题字号减半、回顶/进度间距、移动端去抽屉与顶栏搜索、移动端设置改独立页面、底部菜单选中样式、添加源去重复按钮+新建分组、空状态统一空箱「没有内容」、移除「可能已停止更新」状态、订阅源树左对齐去色带、翻译/AI 图标统一 sparkles 且 AI 摘要去边框 | [RSS-SOURCES](RSS-SOURCES.md) §2.4–2.7、[PDD](../PDD.md) §9.1/§7.4/§12.1、DEC-015 | 公众号真实 sidecar 联调、浏览器 UI 验收 |
| REQ-022 | X / Bluesky 全量历史回补 | P0 | 已完成（真实源端到端验证） | 迁移 4 加 `sources.backfill_mode`（`window` 默认 / `full`）；X 走 `api.fxtwitter.com/2/profile/{handle}/statuses` 游标翻页（实测每页 20 条、~3200 条平台上限），Bluesky 走公开 AppView `getAuthorFeed` 翻页（无上限）；编辑源弹窗可切换「抓取范围：最近窗口 / 全部历史」与窗口条数，切到 full 立即后台回补，进度写源备注；去重键与窗口抓取一致（X=推文链接、Bluesky=at:// URI）；普通 RSS/公众号不支持 | [RSS-SOURCES](RSS-SOURCES.md) §3/§2.5、[PDD](../PDD.md) §7.2/§9.1、DEC-016 | 回补时的列表加载性能与长跑 |
| REQ-023 | 设置页从 0 到 1 重设计 | P1 | 进行中（本地验证） | 用户决策：桌面保留弹窗、移动端页面；左目录+工作区骨架、紧凑行式视觉；六类重组为五类（公众号并入订阅源）；修正安全页 SSRF 过期文案；`/settings` 移动端改两级目录 | [REQ-023](requirements/REQ-023-settings-redesign.md)、[DESIGN.md](DESIGN.md) | 真实浏览器视觉验收 |
| REQ-024 | 移除本地密码登录，仅保留懒猫 OIDC | P1 | 已完成（本地验证） | DEC-017：删除 `RILL_AUTH_MODE`/`RILL_ADMIN_PASSWORD`/密码端点/密码哈希与登录页密码分支、设置内改密 UI；登录只走懒猫 OIDC；新增 `server/cmd/devoidc` 假提供方供本地联调；sessions/CSRF 不变 | [REQ-024](requirements/REQ-024-remove-password-auth.md)、[PDD](../PDD.md) §12.3、[server/README](../server/README.md) | 懒猫真机 OIDC 回调回归 |
| REQ-025 | 2026-11-02 产品行为与设置体验批次 | P1 | 已完成（本地验证） | 移除 AI 外发确认门槛与每日上限；安全页显示懒猫账号 + 退出登录；Feed 令牌 UI 移除（后端端点保留，媒体代理不变）；设置菜单手绘 icon 色随主题；Wordmark 换新图标；默认主题色改品牌蓝并新增 4 档；立即备份改为下载文件走 file-chooser；弹窗关闭遮挡 / 移动端返回 / 椭圆底部导航 / 公众号接入边距修复 | [REQ-025](requirements/REQ-025-2026-11-02-batch.md)、[DESIGN.md](DESIGN.md) | 真机浏览器验收 |
| REQ-026 | 公众号内置核心链路（HTTP-only） | P1 | 已替代（2026-11-03 由 DEC-024 移除后台链路） | Go 移植 we-mp-rss 扫码登录/账号列表/文章列表/正文抓取已实现并通过本地测试；因后台链路只能抓自有账号（BUG-001），随 sidecar 一并移除 | [REQ-026](requirements/REQ-026-wechat-builtin-core.md)、[REQ-034](requirements/REQ-034-weread-only-channel.md)、[DECISIONS.md](DECISIONS.md) DEC-018/DEC-024、[RSS-SOURCES.md](RSS-SOURCES.md) §4.5 | 无；`wechat://` 源改走微信读书通道 |
| REQ-027 | AI 摘要与翻译流式输出 | P1 | 已实现（本地验证） | 摘要 token 级 SSE、翻译段落级 SSE；流式动画与光标；生成后落库、打开文章强制刷新详情直接显示；翻译 icon 无译文时直接触发；mock 持久化 AI 结果 | [REQ-027](requirements/REQ-027-ai-streaming.md)、[PDD.md](../PDD.md) §6 | 真实供应商 SSE、真实浏览器动画观感 |
| REQ-028 | 公众号搜索订阅与 X 自动降级 | P1 | 部分实现（X 降级有效；公众号搜索 2026-11-03 随 DEC-024 移除） | X 主源失败自动降级 `fixupx.com`，无需用户配备用源，失败时同时显示主/备原因；公众号 `searchbiz` 名称搜索依赖后台登录，随微信后台通道移除，文章链接解析保留（公开页提取 biz） | [REQ-028](requirements/REQ-028-wechat-search-and-x-fallback.md)、[REQ-034](requirements/REQ-034-weread-only-channel.md)、[BUGS.md](BUGS.md)、[PDD.md](../PDD.md) §7.2、[RSS-SOURCES.md](RSS-SOURCES.md) §3.2.1 | 真实 X 故障降级；公众号改走 BestBlogs 目录 + 文章链接 |
| REQ-029 | 播客订阅与播放进度持久化 | P1 | 已完成（本地验证） | RSS audio enclosure 识别与播放器（播放/进度/±15 秒/倍速/续播）；迁移 5 新增 `podcast_progress`，服务端保存位置，`localStorage` 作即时缓存；RSS/Atom/JSON 输出保留音频附件 | [REQ-029](requirements/REQ-029-podcast-progress.md)、PRD F-23、[RSS-SOURCES](RSS-SOURCES.md) §2.8、DEC-019 | 真实播客 CORS/防盗链、真机播放与跨设备续播验收 |
| REQ-030 | 平台链接/ID 自动解析（Apple Podcasts、小宇宙、微博等） | P1 | 已完成核心（真实源端到端；Medium/Mastodon 与浏览器待验收） | 新增 `POST /sources/resolve`：Apple Lookup → RSS；小宇宙 `feed.xyzfm.space` 优先、页面 15 条降级；微博匿名访客态最近 100 条；Medium/Mastodon URL 归一化；前端候选列表；不新增 schema | [REQ-030](requirements/REQ-030-platform-resolvers.md)、PRD F-48、[RSS-SOURCES](RSS-SOURCES.md) §2.10–2.13、DEC-020 | 实现、端到端实测、原有源回归 |
| REQ-031 | BestBlogs 公众号目录优先解析 | P1 | 已完成（本地验证；真实浏览器待验收） | 公众号名称先查 BestBlogs 公开 OPML；命中直接创建 `kind=rss`，不要求公众号后台登录；`wechat2rss.bestblogs.dev` Feed URL 直接识别；未命中回退现有公众号后台搜索；目录缓存 6h、可配镜像；不新增 schema | [REQ-031](requirements/REQ-031-bestblogs-wechat-catalog.md)、[RSS-SOURCES](RSS-SOURCES.md) §4.7 | 端到端浏览器验收、目录长期可用性 |
| REQ-032 | BestBlogs 多平台目录 | P1 | 本地/API 整链验证通过（真实浏览器待验收） | `bestblogs:{名称}` 跨公众号/小宇宙/X/YouTube 目录搜索；识别四类 Feed URL，统一 `kind=rss`；公众号 BestBlogs 优先，小宇宙/X/YouTube 保留现有主源优先级，目录只作候选/降级；部分目录失败 10 分钟后重试；不新增 schema | [REQ-032](requirements/REQ-032-bestblogs-multiplatform-catalog.md)、[RSS-SOURCES](RSS-SOURCES.md) §2.14、DEC-022 | 真实浏览器整链、605 项目录全量可用率、目录长期可用性、YouTube 出口 |
| REQ-033 | 公众号双通道（微信后台 + 微信读书） | P0 | 部分替代（2026-11-03 由 DEC-024 收敛为微信读书唯一通道） | 微信读书通道（fakeid 推导 bookId、幂等加书架、`/web/mp/articles` 列表、`/web/mp/content` 正文、`-2041` 冷却 4h、Cookie 加密、UID 不返回前端）继续有效；微信后台通道已移除 | [REQ-033](requirements/REQ-033-weread-dual-channel.md)、[REQ-034](requirements/REQ-034-weread-only-channel.md)、DEC-023/DEC-024、[BUGS](BUGS.md) | 真实微信读书扫码、二哥狗腿子外部列表/正文、验证码与 Cookie 过期、真机 UI |
| REQ-034 | 公众号收敛为微信读书唯一通道 + 扫码回调修复 | P0 | 进行中（本地验证；真实扫码待验收） | 移除微信后台通道（内置链路 + we-mp-rss sidecar）；`wechat://` 源统一走微信读书；文章链接解析改公开页提取 biz，创建源要求微信读书已连接；微信读书扫码成功后刷新设置并提示；设置页删除「微信扫码登录」「伴生服务」与滞后文案 | [REQ-034](requirements/REQ-034-weread-only-channel.md)、DEC-024、[RSS-SOURCES](RSS-SOURCES.md) §4 | 真实微信读书扫码回调、外部公众号端到端、验证码与 Cookie 过期、真机 UI |
| REQ-035 | 创作者名称搜索与批量订阅 | P1 | 进行中（本地验证；真实浏览器待验收） | 新增 `POST /sources/search` 并发搜索 Apple Podcasts / Bluesky / BestBlogs 四目录，返回候选与平台状态；前端添加源增加「搜索创作者」模式，多选后走 `POST /sources/batch` 批量创建并后台首次抓取；不新增 schema，不承诺 X/YouTube/小宇宙/公众号全网名称搜索 | [REQ-035](requirements/REQ-035-creator-search.md)、PRD F-49、PDD §5.2/§6.5/§7.2/§7.6、[RSS-SOURCES](RSS-SOURCES.md) §2.15、DEC-025 | 真实浏览器批量订阅、微博泛搜索、平台限流与目录覆盖率；LPK v0.1.2 已重打包待安装 |
| REQ-036 | 定时报告与可配置推送渠道 | P1 | 进行中（后端完成本地验证；前端进行中） | 迁移 7 已实现：`push_channels`（URL 加密+打码）/`digest_rules`（scope 支持源与 folder:<id> 混选）/`digest_reports`；调度 tick 内生成报告（窗口=last_run_at→now，AI 批量小结+树状归并，条目上限 200，AI 失败降级列表）；飞书卡片/自定义 webhook 推送可测试可重推 | [REQ-036](requirements/REQ-036-digest-reports.md)、PRD F-19、PDD §4/§5/§9、DEC-027 | 前端「自动化」分类、真实渠道联调、真机验收 |
| REQ-037 | 关键字/正则过滤规则 | P1 | 进行中（后端完成本地验证；前端进行中） | 迁移 7 新增 `filter_rules` 与 `items.hidden`；入库后按源评估 mark_read/hide，支持 folder 范围、存量应用、命中计数；默认列表/统计/报告排除隐藏，`?hidden=1` 已过滤视图，PATCH state hidden:false 恢复 | [REQ-037](requirements/REQ-037-keyword-filters.md)、PRD F-18、PDD §4/§9、DEC-027 | 前端「自动化」分类、真机验收 |
| REQ-038 | MCP 服务与正文 Markdown 导出 | P1 | 已实现（本地验证；真实 Agent/真机待验收） | 设置新增「MCP」分类；迁移 10 新增 `mcp_tokens`（多命名 Token、SHA-256 哈希、可启停/吊销）；`/mcp` 使用 MCP Streamable HTTP（SSE）+ Bearer Token，工具覆盖读取与写入但不提供删除，也不暴露密钥/设置/备份恢复；`GET /items/{id}/markdown` 导出原文 Markdown，前端复制/下载（blob + 懒猫 file-chooser inject） | [REQ-038](requirements/REQ-038-mcp-and-markdown-export.md)、PRD F-50/F-51、PDD §4.2/§5.2/§12.3/§12.4、DEC-029 | 真实外部 Agent 连接、真机 file chooser、长连接与超时观测；LPK `0.1.4` 已打包（sha256 `31d960b8…`），未安装 |
| REQ-039 | 侧栏订阅源树与阅读效率扩展 | P1 | 已实现（本地验证；真实浏览器/触屏待验收） | 树右键/长按编辑删除与拖拽归类；侧栏多选批量、搜索、分组未读/失败聚合、全部已读、失败重试、折叠持久化、保存视图、分组 OPML；列表多选与跨源重复合并；`⌘K` 命令面板；每源全文抓取；迁移 11 新增稍后读/笔记/内联高亮/朗读；MCP 移动与重排工具。跳过搜索语法、过滤自动归组、源排序/删除撤销、Newsletter | [REQ-039](requirements/REQ-039-sidebar-tree-context-drag.md)、PRD F-17/F-26/F-30/F-52/F-53/F-54、DESIGN §页面结构、PDD §4.2/§5.2/§6.5 | 真实浏览器批量/拖拽/命令面板/OPML、真实站点全文提取、朗读、真实 MCP Agent；未重新打包 LPK |
| REQ-040 | 置顶订阅源与关注源即时提醒 | P1 | 已实现（本地验证；真实渠道/浏览器待验收） | 侧栏全局「置顶」区（`settings.reader.pinned_sources`，不新增 schema）；源级「关注更新」写入 `sources.config_json`，新条目入库后按「每篇一条 / 每次抓取合并一条」推送现有飞书/Webhook，可选 AI 单篇或批次总结；关注源未读显示 dot；首次抓取、回补、过滤已读/隐藏不提醒；关注源支持多选批量管理、添加时多选源 | [REQ-040](requirements/REQ-040-pinned-and-watched-sources.md)、PRD F-55/F-56、PDD §4.2/§5.2/§9.1、DEC-031 | 真实飞书/Webhook、AI 供应商、桌面/移动端交互验收 |
| REQ-041 | 邮箱自动同步（IMAP，多账号） | P1 | 已实现（本地验证；真实邮箱端到端待验收） | 设置新增「邮箱同步」分类：最多 10 个 IMAP 账号，各自配置服务器/授权码/轮询间隔/单次同步数量（10–500，默认 60）/分组；每个账号一个以账号命名的源；自动同步 INBOX 与所有自定义文件夹，排除已发送/草稿/垃圾箱/垃圾邮件；按 `UIDVALIDITY + LastUID` 增量拉取（单文件夹 100 封/轮），`Message-ID` 去重、RFC 2047 标题、CID 内嵌图入媒体缓存；使用 `BODY.PEEK`，不改动邮箱已读状态；邮件正文收敛嵌套表格边框，远程图片默认拦截并可单次/按发件人放行；`kind=rss` 不新增 schema；邮件条目复用过滤/关注提醒/保留管线；不自建 SMTP 收信 | [REQ-041](requirements/REQ-041-newsletter-email-subscription.md)、PRD F-57、PDD §7.7、[RSS-SOURCES](RSS-SOURCES.md) §2.17 | 多邮箱真实同步、同步数量生效、图片放行与正文视觉、真机浏览器 UI 验收 |
| REQ-042 | 自定义 AI 提示词 | P1 | 已实现（本地验证；真实 AI/浏览器待验收） | 设置 → AI 新增提示词编辑：文章摘要（要点列表/一句话）、定时报告分批与最终简报；关注源 AI 总结复用摘要/报告提示词；只开放任务指令，系统角色与不可信内容边界固定；`settings.ai.prompts` 存储、空值回退默认，API 同时返回只读 `prompt_defaults`；不新增 schema | [REQ-042](requirements/REQ-042-custom-ai-prompts.md)、PRD F-58、PDD §10、DEC-034 | 真实 AI 供应商生成效果、桌面/移动端设置交互验收 |
| REQ-043 | 订阅源 filetree 查看详情入口 | P2 | 已实现（本地验证；真实浏览器待验收） | 设置 → 订阅源每个源行新增 `activity` 图标按钮「查看 {源名} 详情」，跳转 `/sources/{id}`；多选模式不显示；桌面弹窗与移动端设置共用；不新增 API/schema | [REQ-043](requirements/REQ-043-source-tree-detail-entry.md)、PRD F-01/F-07、REQ-020 | 桌面/移动端入口位置与跳转验收 |
| REQ-044 | 订阅地址编辑与邮件图片放行持久化 | P1 | 已实现（本地验证；真实浏览器/邮箱待验收） | 编辑源弹窗可修改 `feed_url`（邮箱同步源除外）：校验/去重、清缓存与失败状态、立即抓取，成功后更新名称（未显式改名时）/站点/图标；邮件「显示图片」按条目写 `media_json.email_images_allowed`，按发件人放行继续写 `trusted_senders`，之后都持续显示；不新增 schema | [REQ-044](requirements/REQ-044-source-url-edit-and-email-image-allow.md)、PRD F-01/F-57、PDD §5.2/§7.7、DEC-035 | 真实浏览器编辑地址、真实邮箱图片放行验收 |
| REQ-045 | 刷新后恢复订阅源与文章阅读上下文 | P1 | 已实现（本地验证；真实浏览器待验收） | localStorage 记住阅读筛选（源/分组/全部/未读/收藏/稍后读/已读）、已读状态与最近文章 ID；刷新 `/items/{id}` 保持文章路由与源列表；刷新 `/` 自动回到最近文章；一次加载只恢复一次，应用内回列表不强制跳文章；删除的源/分组回退「全部」；退出登录清除；不新增 API/schema | [REQ-045](requirements/REQ-045-reader-context-restore.md)、PRD F-30/F-35、PDD §6.5 | 桌面/移动端刷新、登录跳转与删除源回退验收 |

## 暂不纳入当前基线

- 多用户与账号体系：只接入懒猫 OIDC，不做 Rill 自己的用户表、角色或邀请。
- 后续 P1/P2 能力（标签、视频代理、网盘归档等）：尚未进入当前 REQ。
- 通知通道（懒猫系统消息、浏览器 Web Push）暂不纳入 REQ-040；本轮只复用飞书/Webhook。

## 当前剩余阻塞

- REQ-001 X：依赖目标微服出站网络；真实 FxTwitter 抓取未通过，需决定出站代理或自建 FxEmbed。
- REQ-003：官方 file chooser inject 已打包，剩余真机验收。
- REQ-004～007：实现与本地测试已完成；真实后端浏览器级联调、外部站点、长跑和真机仍待验收。
- REQ-019：已安装到默认微服，容器 healthy，OIDC 环境与授权跳转在真机确认；真实浏览器回调、userinfo 与登录后页面需打开 `https://rill.example.com` 验收。
- REQ-020：v0.1.2 已升级安装；真实浏览器视觉、X 自动同步、filetree 移动/删除与 tooltip 定位待验收。
- REQ-034 公众号：真实微信读书扫码回调、外部公众号列表/正文端到端、验证码与 Cookie 过期待验收。
- REQ-035 创作者搜索：真实浏览器搜索→多选→批量订阅→源树刷新待验收；微博泛搜索未实现，YouTube/小宇宙/公众号仅覆盖 BestBlogs 目录；平台长期限流待观察。LPK v0.1.2 已于 2026-11-04 重打包（sha256 `0669603f…`），未安装。
- REQ-038 MCP / Markdown：本地单测与官方 MCP Go 客户端握手通过；真实 Claude Desktop/Codex 连接、SSE 长连接、超时行为与真机懒猫文件保存弹层待验收；LPK 未重打包。
- REQ-040 置顶/关注提醒：本地 Go 测试、前端类型检查/61 项 Vitest/构建通过；真实飞书/Webhook 推送、AI 供应商总结、桌面与移动端置顶交互待验收。

## 使用规则

- 同一需求涉及持久化数据、外部服务、权限或安全边界时，先在 REQ 中写方案并等待确认。
- 需求完成或状态变化时，同步 [`PROGRESS.md`](PROGRESS.md) 和对应功能进度。
- 不再推进的需求标为 `不纳入` 并记录原因，不静默删除。
