# Bug 台账

## BUG-008 部分邮件图片无法显示

- 状态：已修复（本地测试通过；真实邮件/浏览器待验收）
- 发现：2026-11-23
- 影响：部分邮件放行远程图片后仍有图片不显示或裂图；同批邮件中另一些来源（如招商银行）的图片正常。排除单一网络问题后，代码审计确认存在多处邮件 HTML 兼容性缺口。
- 关联：[REQ-041](requirements/REQ-041-newsletter-email-subscription.md)、[REQ-044](requirements/REQ-044-source-url-edit-and-email-image-allow.md)、[RSS-SOURCES](RSS-SOURCES.md) §2.17、[email.go](../server/internal/fetch/email.go)、[feed.go](../server/internal/fetch/feed.go)、[ArticleContent.tsx](../web/src/components/reader/ArticleContent.tsx)

### 根因

对照 `~/Documents/lzc-projects/AgentMail`（实际路径为 `~/Documents/gitlab-lzc-projects/lzc-projects/AgentMail`）的邮件阅读实现，Rill 的清洗与渲染链路丢掉了 AgentMail 会保留的邮件特性：

1. **内联 `style` 与 `background` 被清洗**：后端 `bluemonday.UGCPolicy()` 和前端 DOMPurify 都移除了 `style`、`background` 属性。邮件常用 `<table background=...>` 或 `style="background-image:url(...)"` 承载头图/横幅，这类图片因此完全消失；招商银行邮件若使用普通 `<img>` 则不受影响。
2. **`data:image/...;base64` 图片被丢弃**：`SanitizeContent` 只保留 http/https 图片，data URI 在进入媒体代理前就被移除。
3. **协议相对地址被丢弃**：`src="//cdn.example.com/a.png"` 没有显式 scheme，被 `isHTTPURL` 判为非 HTTP 后删除。
4. **只写在 `srcset` 的图片被丢弃**：清洗时移除 `srcset`，但没有先取其中的首个候选地址。
5. **Content-Type 标错的内联图未被识别**：部分邮件把 CID 图片标成 `application/octet-stream`，原逻辑只接受 `image/*`，导致 `cid:` 引用无法替换。
6. **放行后强制直连原图**：即使 `/media/{token}/{hash}` 代理可用，也会改为加载原始 URL；遇到 http 混合内容、Referer 防盗链、浏览器侧拦截时会失败，且没有回退。
7. **2026-11-23 进一步定位（真实邮箱库）**：现有邮箱源有 21 封邮件，其中 16 封含 `<img>`，但只有 1 封带 `data-remote-src`；这些邮件的图片地址大量是 `/media//{hash}`（token 为空）。根因是 `emailTargetSource` 对已存在的邮箱源只调用 `scanSource`，后者不加载 `feed_token`，导致新邮件/旧邮件都以空 token 生成媒体地址；同时 `decorateEmailImageAttrs` 只认当前 token 前缀，无法为 `/media//` 回填 `data-remote-src`。前端因此 `remoteCount=0`，既没有「N 张远程图片已拦截」提示，也没有「显示图片」入口，图片请求 `/media//...` 直接 404。招商银行邮件已带 `data-remote-src`，所以能看到 26 张图片的提示。

### 修复

- 新增邮件专用 `SanitizeEmailContent`：保留安全的内联样式、`background`、表格布局标签、字体标签与 `data:image/...;base64`；仍移除脚本、事件属性、iframe 等危险内容。非邮件文章继续使用原有更严格的清洗。
- 协议相对图片地址补全为 https（或跟随 Feed 的 http scheme）；`srcset` 无 `src` 时取首个候选；CID 图片按内容嗅探识别，兼容错误 Content-Type。
- 前端邮件 DOMPurify 同步允许安全的样式/背景属性；背景图纳入“远程图片已拦截”计数，未放行时移除 `background` 与 `background-image`，放行后恢复。
- 放行后优先保留 `/media` 代理地址，图片加载失败（error 或已失败）时再回退到 `data-remote-src` 原图，兼顾混合内容、Referer 防盗链和代理抓取失败。
- 修复邮箱源读取路径：`emailTargetSource` 对已存在源也返回 `sourceInternal`，确保 `feed_token` 被加载；详情读取时把历史 `/media/{任意 token}/{hash}` 修正为当前源 token，并从 `media_json.images` 回填 `data-remote-src`。列表封面改用修复后的 HTML。

### 验证

- `go test ./... -count=1`、`go vet ./...` 通过；新增 `TestBuildEmailFeedItemKeepsRichEmailMarkup` 覆盖 style/background、data URI、srcset 与协议相对地址，`TestEmailTargetSourceKeepsFeedToken` 覆盖已存在邮箱源的 token，`TestGetItemRepairsLegacyEmailMediaToken` 覆盖历史空 token 与 `data-remote-src` 回填。
- 真实库审计：邮箱源 21 封邮件、16 封含图、65 个媒体 hash 均为 `pending`；修复后详情读取会使用当前源 token 并恢复放行提示。
- 前端 `npm run typecheck`、`npm test -- --run`（21 文件/83 用例）、`npm run build` 通过；`ArticleContent` 用例覆盖背景图拦截/放行、代理优先与失败回退。
- 待验收：用户实际无法显示的那封邮件在浏览器中的表现、data URI 大图、背景图与混合内容场景。

## BUG-007 部分 X 源头像显示为旧照片 / 错误人物

- 状态：已修复（本地测试通过；真实微服与浏览器待验收）
- 发现：2026-11-22
- 影响：例如 `@poteto` 的当前头像是插画女性，Rill 侧栏却显示一张旧照片；部分 X 账号的 Feed 频道图长期不更新，导致看起来像另一个人。
- 关联：[RSS-SOURCES](RSS-SOURCES.md) §3.3.2、[icon.go](../server/internal/server/icon.go)、BUG-004

### 根因

X 源的头像候选链把 Feed 的 `<image>`（`sources.icon_url`）放在第一位。FxTwitter 的频道图会停留在旧头像：

- 2026-11-22 实测 `https://fxtwitter.com/poteto/feed.xml` 返回
  `pbs.twimg.com/profile_images/2069439174059364352/...`（旧照片）；
- `https://api.fxtwitter.com/poteto` 的 `avatar_url` 为
  `pbs.twimg.com/profile_images/2093473719830315008/...`（当前插画）；
- `https://unavatar.io/x/poteto` 返回的字节与当前 `pbs` 400x400 头像一致（38,024 bytes）。

旧候选顺序先缓存 Feed 旧图，unavatar 永远不会被尝试；图标文件缓存 7 天，X 源仅在抓取后按 6 小时失效，期间一直显示旧头像。重启启动清理可删除旧缓存，但重新请求仍会再次命中 Feed 旧图。

### 修复

- X 源头像候选顺序改为：`unavatar.io/x/{handle}` → Feed `<image>` 降级；非 X 源保持原 Feed 图标 → 站点 icon → favicon 链。
- 沿用既有缓存清理：启动时清除全部 X 源图标；抓取成功后 X 图标超过 6 小时失效；手动刷新源立即清缓存。升级重启或手动刷新后，旧头像会被替换为当前头像。
- 新增 `TestXIconPrefersCurrentAvatarOverStaleFeedImage`，并更新候选顺序断言。

### 验证

- `go test ./internal/server -run 'TestXIcon|TestSourceIconUnavatarFallback|TestManualRefreshClearsIconCache' -count=1` 通过。
- 待验收：微服出口下 `unavatar.io/x/poteto` 的实际返回、升级后旧缓存清理、浏览器与移动端头像显示。

## BUG-006 邮件订阅：源名/分组不同步、嵌套文件夹路径打不开、测试成功后仍需刷新

- 状态：已修复（本地测试通过；真实邮箱/微服待验收）
- 发现：2026-11-21
- 影响：① 连接测试成功后，「需关注」里的邮件源要刷新页面才消失；② 第二个邮箱填写文件夹 `Email` 报 `IMAP NO [NONEXISTENT] Folder not exist!`；③ 聚合箱源始终叫「邮件收集箱」且显示「未分组」，即使填写了邮箱文件夹；④ 分组只能从已有列表选择，不能直接新建。
- 关联：[REQ-041](requirements/REQ-041-newsletter-email-subscription.md)、[PDD](../PDD.md) §7.7、[RSS-SOURCES](RSS-SOURCES.md) §2.17

> 2026-11-21 后续改造：本功能已从「邮件订阅」改为「邮箱自动同步」，删除 IMAP 文件夹参数，自动遍历所有可同步文件夹且不再按文件夹自动分组。下面第 2/3 条的文件夹路径解析与自动分组已不再适用；测试连接实时刷新、源名 = 邮箱账号、新建分组仍保留。

### 根因

1. **前端没有刷新源列表**：`POST /settings/email/test` 成功时后端已清除该账号源的 `last_status/last_error` 并安排立即重试，但设置页只弹提示，`useSourcesStore` 仍持有旧的 `last_status=error` 数据，所以侧栏「需关注」不会消失，直到整页刷新重新请求 `/sources`。
2. **文件夹按显示名而不是 IMAP 完整路径 SELECT**：客户端常把子文件夹显示为 `Email`，服务器 `LIST` 返回的却是 `其他文件夹/Email`（分隔符由服务器决定，部分账号是 `.`）。`SELECT Email` 必然 `NONEXISTENT`；参考 `AgentMail`（ImapFlow）也是始终使用 `item.path`。
3. **源名与分组只在首次创建时写入**：`ensureEmailSources` 只在聚合源不存在时创建，已存在的源不会更新 `name/folder_id`；源名硬编码为「邮件收集箱」，未使用邮箱账号。设置里的 `folder_id` 对已存在的源不生效，于是显示「未分组」。
4. **设置页分组只能选已有项**：没有复用添加源弹窗的「新建分组」交互。

### 修复

- 测试连接成功后，前端先落库当前配置与授权码，再 `useSourcesStore.load(true)`；后端已重置的健康状态立即反映到侧栏，「需关注」实时消失。
- `fetch.selectMailbox`：先按原值 SELECT；`NONEXISTENT` 时 `LIST` 一次，按「大小写不敏感完整路径 → 唯一叶子名」重试；同名叶子有多个时明确报错并列出候选，不猜测。`LIST` 结果跳过 `\Noselect` 并排序，设置页用作文件夹候选。
- 文件夹输入改为可编辑下拉（datalist）：连接测试/「获取文件夹列表」后可直接选择服务器返回的完整路径；手填 `Email` 且存在唯一 `其他文件夹/Email` 时自动改成完整路径。
- 聚合箱源名改为邮箱账号；`ensureEmailSources` 对已存在的聚合源同步 `name/folder_id/refresh_seconds`，旧名「邮件收集箱」自动改为账号名。应用启动时执行一次 `normalizeEmailSources`，只补空名称/未分组，不覆盖用户手动改名或移动。
- 未显式选择分组时按邮箱文件夹自动分组：`INBOX` → 「邮件」，`其他文件夹/Email` → 「Email」；分组不存在时自动创建。显式选择的 `folder_id` 优先；已存在但未分组的邮件源也会归入该分组。
- 设置页「新源归入分组」复用添加源弹窗的交互：下拉选择 + 「新建分组」内联创建；默认项为「自动 · 按邮箱文件夹分组」。

### 验证

- `TestResolveMailboxName`（完整路径/唯一叶子/歧义拒绝/跳过 `\Noselect`）、`TestEmailFolderGroupName`、`TestEmailSettingsPatchValidationAndRedaction`（聚合源名 = 账号、INBOX → 「邮件」分组、改为 `其他文件夹/Email` 后源同步迁移到「Email」分组）通过；`go test ./...`、`go vet ./...` 全绿。
- 前端 `EmailSettings.test.tsx` 7 项通过（测试成功后 `saveNow` + `load(true)`、手填叶子名解析为完整路径）；全量 73 项 Vitest、`tsc -b` 通过。
- 待验收：真实第二个邮箱的嵌套文件夹抓取、微服侧栏「需关注」实时消失、自动分组与新建分组在真机上的观感。

## BUG-005 邮件订阅「测试连接」一直转圈 / 误报网络问题

- 状态：已修复（本地测试通过；微服实机待验收）
- 发现：2026-11-21
- 影响：邮件订阅填写服务器与授权码后，点击「测试连接」长时间停留在「正在测试」，或最终报网络错误，但浏览器访问外网正常。
- 关联：[REQ-041](requirements/REQ-041-newsletter-email-subscription.md)、[PDD](../PDD.md) §7.7

### 根因（2026-11-21 用真实 QQ 邮箱复现）

1. **客户端在读到服务器问候（greeting）之前就发送了 `LOGIN`**：go-imap v2 的命令不等待 `WaitGreeting()`。QQ 邮箱会丢弃 greeting 之前流水线发送的 LOGIN，客户端发完命令后收不到任何响应直至超时——表现为「网络问题」，但网络完全正常（手工 TLS 连接同一账号 1 秒内登录成功）。抓包对比确认：Go 客户端 `> T1 LOGIN ...` 后才 `< * OK ... ready`，随后无响应。
2. **QQ 返回的 `ENVELOPE` 不符合库解析假设**：`FETCH ... ENVELOPE` 报 `in envelope: expected ')', got "z"`。Rill 只需要原始 MIME（自己解析信头），ENVELOPE 是多余依赖。
3. 附带问题：`LOGIN` / `LIST` 的 `Wait()` 没有超时，服务器不响应时永久挂起；IMAP 拨号没有复刻 `fetch.Client` 的 IPv4 优先（微服 DNS 假 IPv6 会卡满拨号超时）。

### 修复

- `dialIMAP` 在登录前调用 `client.WaitGreeting()`；问候异常时明确报错。
- `FETCH` 去掉 `Envelope`，只取 `UID` + `BODY.PEEK[]`。
- `dialIMAP` 增加看门狗 goroutine：ctx 结束立即关闭连接，所有在途命令以错误解除阻塞；测试连接整体 12s、邮件拉取/标已读整体 60s。
- `dialIMAPAddr` 复刻 `fetch.Client` 的 IPv4 优先解析与逐地址拨号。
- 前端测试请求 15s `AbortSignal.timeout` 兜底；超时显示「测试超时：服务器无响应」。
- `/api/v1/healthz` 增加 `version`，便于确认微服实际运行版本。

### 验证（真实 QQ 邮箱，只读）

- 登录 + 文件夹列表：**393ms**，7 个文件夹（含中文文件夹 `其他文件夹/QQ邮件订阅`，UTF-7 编码正常）。
- INBOX 未读拉取：5 封 / **1.46s**；解析出中文标题、RFC 3339 时间、正文与内联图（1 和 7 张），纯文本邮件走降级。
- 未对真实邮箱执行 `STORE +FLAGS (\Seen)`（避免改动用户邮箱）；该命令为 IMAP 标准操作，由单测覆盖调用路径。
- `go test ./...`、`go vet ./...`、前端 `tsc -b` + 70 用例通过。

## BUG-003 小宇宙 / Apple Podcasts 源同步报「响应 Content-Type 不是 Feed：text/html」

- 状态：已修复（本地测试通过；微服实机降级待验收）
- 发现：2026-11-21
- 影响：`feed_url` 为 `https://feed.xyzfm.space/{token}` 的订阅源（含经 Apple Podcasts 解析创建、feedUrl 由小宇宙托管的节目）抓取失败，报「响应 Content-Type 不是 Feed：text/html」。
- 关联：[RSS-SOURCES](RSS-SOURCES.md) §2.11、[PDD](../PDD.md) §7.5

### 根因

2026-11-21 实测：`feed.xyzfm.space` 对非白名单出口 IP 返回 302（响应头 `X-Tengine-Error: denied by IP ACL = not in whitelist`），跳转到一个 **HTTP 200 的 HTML 拒绝页**（`other-qiniu.jellow.site/...html`）。小宇宙官方分发 Feed 上游加了 IP 白名单，与 User-Agent 无关（ApplePodcasts/Overcast/浏览器 UA 均被拒）。Rill 客户端跟随重定向拿到 200 HTML，于是报成 Content-Type 错误，掩盖了真实原因。本机用 4 种 UA 复现全部 302 拒绝；Apple iTunes Lookup 返回的 `feedUrl`（如「忽左忽右」→ `feed.xyzfm.space/cv4bkgpuglwp`）同样命中，两个报错同根因。

### 修复（2026-11-21 第二轮，按用户要求改为「自动替换订阅方式」而非降级报错）

首轮修复只是抓取期降级，Apple 解析创建的源（site_url 是 Apple 页面）无法恢复 pid，仍然报错。第二轮改为**永久替换**：

- pid 恢复两条路径：① `site_url` 是小宇宙节目页（token 建源时由 Feed 内容写入）；② **从条目反查**——分发 Feed 可用期间入库的单集链接（`xiaoyuzhoufm.com/episode/{eid}`）取最新一条，抓单集页 `__NEXT_DATA__` 拿到所属节目 pid（覆盖 Apple 解析源）。
- 恢复成功后**改写源本身**：`feed_url` → `xiaoyuzhou://podcast/{pid}`、`site_url` → 节目页、备注「已自动改用小宇宙页面抓取（最近 15 条；原分发 Feed 被上游 IP 白名单拒绝）」；随后立即按页面适配器抓取。此后不再请求被拒的 Feed；若上游恢复也不回切（页面源稳定可用）。
- 已存在同一节目的页面源时拒绝替换并提示，避免双源。
- `fetch.FetchFeed` 的 Content-Type 错误保留重定向目标提示；pid 仍无法恢复的源报「无法自动恢复小宇宙节目 ID」。

### 第三轮修复（2026-11-21，用户实测仍报错）

用户实测 0.1.10 后同一源仍报「响应 Content-Type 不是 Feed…」，排查发现恢复流程写在了 `if fetchErr != nil` 失败分支内部：替换成功、页面抓取也成功之后，代码仍沿失败分支继续执行 `recordFetchFailure` 并返回 502，所以当次刷新必然报错，只有下一次调度才恢复。已把恢复流程前移到失败处理之前，成功即走正常入库路径；失败信息区分「无法恢复 pid」与「已迁移但节目页暂时不可访问」。真实网络验证：Feed 被拒 → 条目反查 pid → `feed_url` 改写 → 页面抓取 `status=ok`、12 条。新增 `TestFetchSourceXiaoyuzhouReplacesAndSucceeds` 回归。

### 验证

- `TestIsXiaoyuzhouFeedURLAndPidRecovery` / `TestXiaoyuzhouFeedAutoReplaceViaEpisodeItems`（Apple site_url + 条目反查 → DB 中 feed_url 被改写）/ `TestXiaoyuzhouFeedAutoReplaceViaSiteURL` / `TestXiaoyuzhouFeedReplaceRejectsDuplicate` / `TestFetchFeedRedirectToHTMLHint` 通过；`go test ./...` 全绿。
- 待验收：微服上 Apple 解析的小宇宙源下次调度后 feed_url 自动变为 `xiaoyuzhou://podcast/{pid}`、备注更新、内容恢复。

## BUG-004 部分 X 作者订阅源头像显示失败（C1ownhjj、HiTw93 等）

- 状态：已修复（本地测试通过；微服实机待验收）
- 发现：2026-11-21
- 影响：部分 X 源头像不显示（回退首字母占位）。
- 关联：[RSS-SOURCES](RSS-SOURCES.md) §3、[PDD](../PDD.md) §11

### 根因

X 头像链路依次尝试：Feed `<image>`（`pbs.twimg.com`）→ `x.com` 页面 icon → `x.com/favicon.ico` → `fxtwitter.com/favicon.ico`。微服出口若无法直连 `pbs.twimg.com` / `x.com`（X 系域名在部分国内网络不可达；`fxtwitter.com` 走 Cloudflare 可达，Feed 本身能抓），前三个候选全部失败且每个候选超时 15s，整条链最长约 60s，浏览器等待期间即判定失败；最终要么拿到对所有 X 源相同的 fxtwitter 站点图标，要么写入失败标记。本机直连 `pbs.twimg.com` 返回 200 `image/jpeg`，失败仅发生在微服出口网络。

### 修复（2026-11-21 第二轮补：失败标记拦截重试）

首轮修复漏了一个环节：图标失败标记（`.failed` 文件，TTL 1h）在升级后依然存在，`refreshSourceIcon` 会直接拒绝重试——用户「刷新源」也无效，因为刷新不清除标记。

- X 源（`fxtwitter.com/{handle}/feed.xml`）头像候选链在 Feed 图标之后插入 `https://unavatar.io/x/{handle}`（第三方聚合服务；实测 C1ownhjj / HiTw93 均返回 200 `image/jpeg` 真实头像，字节数与 pbs 原图一致），置于 x.com 站点探测之前。
- 图标候选级超时收敛为 6s：被墙候选不再拖死整条链。
- **`fetchSource` 成功后清除该源的头像失败标记**：源刷新/调度抓取成功即让下一轮图标请求立即用新候选链重试，用户无需等 1 小时。
- 已知取舍：unavatar.io 是第三方依赖；unavatar 找不到头像时返回其占位图，与首字母回退二选一。

### 第三轮修复（2026-11-21，用户实测仍不显示）

前两轮忽略了**服务端图标缓存**：升级前成功缓存的通用 fxtwitter 站点图标会按 7 天 TTL 一直复用，候选链更新后也不会重抓；同时浏览器端 `max-age=604800` 会继续使用旧响应。修复：① 图标响应改为 `Cache-Control: no-cache`，浏览器每次回源验证（304 仍走本地缓存，开销可忽略）；② X 源在每次抓取成功后，把超过 6 小时的旧图标文件删除，下一轮图标请求自动重走候选链（pbs → unavatar → 站点图标）；③ 保留失败标记清除。新增 `TestFetchSourceInvalidatesOldXIcon`。若微服容器到 `pbs.twimg.com` 与 `unavatar.io` 均不可达（浏览器能访问不代表容器能访问），则仍只能回退首字母——需另行配置容器出站代理，这一点不再隐瞒。

### 第四轮修复（2026-11-21，用户反馈「还是蓝鸟旧头像」）

下载候选图片逐一核对确认：`fxtwitter.com/favicon.ico` 是 32×32、中心 `rgb(29,155,240)` 的 Twitter 蓝鸟图标；`unavatar.io/x/C1ownhjj` 与 `pbs.twimg.com` 原图字节完全一致（8952 bytes，真实头像）。用户看到的就是旧候选链最终缓存的 fxtwitter 站点 favicon。根因有二：① X 源候选链仍包含站点 favicon，头像全部失败时会缓存蓝鸟；② 旧缓存文件在 7 天 TTL 内继续复用，6 小时失效规则只在抓取成功时触发且按 `kind=="x"` 判断（URL 直加的 X 源 `kind=rss` 不命中）。

修复：
- X 源（按 host 判断 `fxtwitter.com/api.fxtwitter.com/fixupx.com`，兼容 `kind=rss`）候选链**只保留真实头像**：Feed `<image>`（pbs）→ `unavatar.io/x/{handle}`；不再尝试 `x.com` 页面与站点 favicon，失败时回退首字母而不是蓝鸟。
- 应用启动时删除所有 X 源的旧图标缓存与失败标记（一次性清掉历史蓝鸟）。
- 抓取成功后的 6 小时失效规则改为按 host 判断。

### 第五轮修复（2026-11-21，用户反馈「点刷新只刷内容不刷头像」）

刷新入口（工具栏/右键）只调用 `POST /sources/{id}/fetch`，服务端不清理图标缓存，前端 `<img>` 地址不变、`broken` 状态也不重置——所以头像永远停在旧图。修复闭环：
- 手动刷新成功后，服务端删除该源的图标缓存文件与失败标记（`clearSourceIcon`）。
- 前端 `SourceIcon` 用 `last_fetch_at` 作为图标 URL 的版本参数（`?v=`），刷新后 URL 变化强制浏览器重新请求；同时 `useEffect` 重置 `broken`，避免一直显示首字母。
- 图标候选全部失败时，服务端日志记录每个候选的失败原因（域名 + 错误），便于确认容器出口到 pbs/unavatar 的连通性。
- 注：普通抓取更新 `last_fetch_at` 也会让图标 URL 变化，但服务端有缓存时只是本地文件读取，开销可忽略。

### 验证

- `TestManualRefreshClearsIconCache`（刷新 → 缓存/失败标记清除 → 下次图标请求经 unavatar 重新缓存）、前端 `SourceIcon` 2 个用例（URL 版本参数、失败后刷新重试）通过。
- 待验收：微服上点一次刷新，C1ownhjj 头像应立即重新加载；若仍失败，服务端日志会打印具体候选失败原因。

- `TestXIconCandidatesExcludeSiteFavicons`（X 候选只有 pbs + unavatar，无 favicon；普通源保留 favicon 链）、`TestResetLegacyXSourceIcons`（启动清理 X 图标与标记，非 X 不动）、`TestFetchSourceInvalidatesOldXIcon`（X 域名抓取成功 → 6h 旧图标删除）通过；`go test ./...` 全绿。
- 待验收：微服启动 0.1.13 后刷新页面，C1ownhjj / HiTw93 显示真实头像。

### 验证（前两轮）

- `TestSourceIconUnavatarFallback` / `TestFetchSourceClearsIconFailureMarker`（旧标记 → fetchSource 成功 → 标记清除）通过；`go test ./...` 全绿。
- 待验收：微服上刷新 X 源后 C1ownhjj / HiTw93 等源头像实际恢复显示。

## BUG-002 公众号正文图片显示「此图片来自微信公众平台未经允许不可引用」

- 状态：已修复（本地测试通过；真实浏览器/存量文章待验收）
- 发现：2026-11-13
- 影响：微信读书通道抓取的公众号文章，正文图片在 Rill 中显示为微信防盗链占位图，正文文字正常。
- 关联：[REQ-034](requirements/REQ-034-weread-only-channel.md)、[RSS-SOURCES](RSS-SOURCES.md) §4.6、[PDD](../PDD.md) §4.2

### 根因

`mmbiz.qpic.cn` 按 `Referer` 做防盗链。浏览器在 Rill 页面里直接请求远程图片时会带上 Rill 域名 Referer，微信返回 140×140 的「未经允许不可引用」占位图；服务端或无 Referer 请求返回原图。

2026-11-13 用同一张真实公众号图片复现：

| 请求 | 结果 |
|---|---|
| 无 Referer | 200，557,304 bytes，1080×508 原图 |
| `Referer: https://rill.example/` | 200，2,090 bytes，140×140 占位图 |
| `Referer: https://mp.weixin.qq.com/` | 200，557,304 bytes，1080×508 原图 |

此外，微信读书正文路径此前把 `#js_content` 原样入库，图片仍指向 `mmbiz.qpic.cn`，没有像 RSS 正文那样改写为 `/media/{token}/{hash}`，也没有登记 `media_cache`。

### 修复

- 前端正文渲染统一给 `<img>` 加 `referrerpolicy="no-referrer"`；存量远程图片无需重新抓取即可正常显示。与 lazycat-search 的正文图片处理一致。
- 微信读书正文改为复用 `fetch.SanitizeContent`：图片重写为 `/media/{token}/{hash}`，原图登记 `items.media_json` 与 `media_cache`，由服务端无 Referer 下载并缓存。
- 后端清洗后的 `<img>` 同时写入 `referrerpolicy="no-referrer"`，覆盖 RSS、全文抓取与公众号正文。

### 验证

- `go test ./internal/fetch ./internal/server -run 'TestWeread|TestWechatSourceFetchesViaWeread|TestFetch' -count=1` 通过。
- `npm run test -- --run src/components/reader/ArticleContent.test.tsx` 通过（含远程微信图片 no-referrer 用例）。
- 待验收：真实浏览器打开存量公众号文章、新抓取文章图片落盘与 `/media` 缓存。

## BUG-001 外部公众号添加后内容不同步（200013）

- 状态：已以通道收敛方式终结（2026-11-03 DEC-024/REQ-034 移除微信后台链路；微信读书为唯一通道，本地 httptest 通过，真实扫码与外部号抓取待验收）
- 发现：2026-11-03
- 影响：内置公众号链路添加的**非当前登录后台管理的公众号**（如「二哥狗腿子」）可创建源，但首次与后续抓取均失败，阅读器无内容。
- 关联：[REQ-026](requirements/REQ-026-wechat-builtin-core.md)、[REQ-028](requirements/REQ-028-wechat-search-and-x-fallback.md)、[RSS-SOURCES](RSS-SOURCES.md) §4.4、DEC-018

### 现场

- 源 `id=13`：`kind=wechat`、`feed_url=wechat://MzUxNzAzMTU4OQ==`、`name=二哥狗腿子`。
- `sources.last_status=error`，`last_error=公众号接口触发频率限制，请稍后重试`，`initial_fetched=0`，`fail_count=2`，items 为 0。
- 日志：`initial source fetch failed`、`scheduled fetch failed`，错误均为 `fetch_failed`。

### 复现与证据

同一份真实微信后台会话（2026-11-03，Rill 微服数据副本）直接请求微信接口：

| 目标 | 接口 | 结果 |
|---|---|---|
| 当前后台管理的「吃遍全国汉堡」`gh_3df5c6b01cc2` | `appmsgpublish` | `ret=0`，返回文章列表 |
| 「二哥狗腿子」`MzUxNzAzMTU4OQ==` | `appmsgpublish` | `ret=200013`，`freq control` |
| 「少数派」`MzU4Mjg3MDAyMQ==` | `appmsgpublish` | `ret=200013`，`freq control` |
| 「晚点LatePost」`MzU3Mjk1OTQ0Ng==` | `appmsgpublish` | `ret=200013`，`freq control` |
| 「二哥狗腿子」 | `searchbiz` | `ret=0`，能搜索到 fakeid |

结论：`searchbiz` 搜索外部公众号正常，但跨公众号文章列表接口从 2026-07 起对第三方/非本后台管理的账号收紧，`200013` 不再只是可等待恢复的累计频控。当前会话的 `switchacct` 只返回「吃遍全国汉堡」，因此「二哥狗腿子」不能通过内置后台链路抓取。

参考：<https://lovstudio.ai/blog/wechat-cross-account-index-ret-200013-2026>（2026-08-01，记录了同期多个独立项目的首屏 `200013` 与“搜索/单篇成功、列表失败”现象）。

### 根因

1. 产品把 `searchbiz` 能搜到等同于 `appmsgpublish` 能列出文章，误把外部账号加入 `wechat://` 源。
2. Rill 把所有 `200013` 都当作源级短期频控，只对当前源退避，未在创建或抓取前校验账号是否属于当前后台。
3. 外部账号没有可用内容源时，UI 仍显示“等待首次抓取”，用户会误以为稍后会同步。

### 修复

- 抓取前校验 `wechat://<fakeid>` 必须在当前 `session.Accounts` 中；否则快速失败，不再请求微信。
- 搜索只返回当前后台可管理的公众号；外部账号返回 `wechat_account_unmanaged` 与可操作说明。
- 文章链接解析出的账号按 fakeid/名称映射到当前后台账号；外部账号同样拒绝。
- 同一微信会话串行抓取并保持 1 秒请求间隔；真实 `200013` 触发会话级 30 分钟冷却，避免多个公众号源放大限流。
- 错误文案改为“平台列表能力受限/频控”，不再承诺稍后自动恢复。
- 新增微信读书扫码双通道：外部账号在 WeRead 已连接时可直接创建并抓取；详见 [REQ-033](requirements/REQ-033-weread-dual-channel.md)。
- 现有源不自动删除；用户可在确认后删除或等待 WeRead 通道验收。
- LPK `0.1.1` 已重新打包（sha256 `4defdb9e1d297b401ddb39de66a70b2db44d8a8bde22d81fa2e9677cd4ed84df`），未安装。

### 后续

- 2026-11-03 终结方式：用户确认后台通道「只能查看自己公众号没意义」，随 DEC-024 移除微信后台链路与 sidecar；`wechat://` 源统一走微信读书通道，未收录公众号走 BestBlogs 目录或文章链接（公开页提取 `biz`）。原「抓取前自有账号校验/会话冷却」逻辑随链路一并删除。
- 2026-11-03 尝试评估已安装的 WeRSs：源码显示其 `search`/`web`/`app`/`api` 模式仍调用 `mp.weixin.qq.com/cgi-bin/searchbiz`、`appmsgpublish`、`appmsg`，不是微信读书链路；其 `/mps/search/二哥狗腿子` 当前返回 `50001`，日志为 `200003 invalid session`，且两个 Feed 的文章数均为 0。因此切到这套 WeRSs 不能解决外部账号 `200013`。
- 真实浏览器 UI、真机部署与修复后抓取验证待做。
