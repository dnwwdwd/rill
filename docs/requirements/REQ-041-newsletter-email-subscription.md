# REQ-041: 邮箱自动同步（IMAP 通道）实现方案

> 状态：已实现（本地验证；真实邮箱端到端验收待做） · 日期：2026-11-21
>
> 对应 PRD F-57；源登记见 [RSS-SOURCES §2.17](../RSS-SOURCES.md)；架构口径见 [PDD §7.7](../../PDD.md)。

## 1. 需求背景与痛点

很多内容（Substack 专栏、科技周刊、行业内参、普通来信）只通过电子邮件分发。用户希望把邮箱作为 Rill 的自动同步来源，而不是每换一个周刊就配置一次文件夹或发件人。Rill 需要把邮箱里的新邮件持续转成阅读条目，同时坚持自托管轻量、单二进制、免公网重型配置的原则。

2026-11-21 用户明确改造口径：

- 从「邮箱内 Newsletter 订阅」改为「邮箱自动同步」；
- 删除文件夹参数；
- 同步所有邮件文件夹（排除已发送、草稿、垃圾箱、垃圾邮件等系统文件夹）；
- 不改动邮箱里的已读状态；
- 每个邮箱账号对应一个以账号命名的订阅源，允许同时配置多个邮箱。

## 2. 协议边界：为什么是 IMAP 客户端轮询，而不是自建 SMTP

| 维度 | 自建 SMTP 接收服务器 | IMAP 客户端轮询（本方案） |
| :--- | :--- | :--- |
| 角色 | 服务端监听 25 端口等待投递 | 客户端出站连接外部 993 (TLS) 主动拉取 |
| 端口 | 家庭宽带/云厂商物理封死 25 端口 | 标准 HTTPS 类出站，微服开箱即通 |
| 公网依赖 | 需公网 IP、MX 记录、SPF/PTR | 零公网依赖，复用用户现成邮箱 |
| 复杂度 | 反垃圾、TLS 证书、IP 信誉 | 纯 Go IMAP 客户端，几百行自包含 |

结论：**唯一实用方案是 IMAP TLS 轮询**。不做 POP3，不做自建 SMTP。

## 3. 功能设计与交互

### 3.1 接入配置（设置 → 邮箱同步）

- 支持添加多个邮箱账号（上限 10 个），每个账号独立配置 IMAP 服务器（如 `imap.163.com`）、端口（默认 993 SSL）、账号、**客户端授权码**（非网页登录密码，AES-GCM 加密存储，接口只返回 `password_set`）。
- 不再提供文件夹参数。连接成功后自动遍历该账号所有可同步文件夹。
- 每个账号可单独设置轮询间隔（默认 15 分钟，下限 5 分钟）和单次同步数量（10–500 封，默认 60）。
- 组织方式：每个账号的邮件进入各自的 `imap://{username}` 源，源名 = 邮箱账号，发件人显示在条目作者栏。
- 分组：每个账号可单独归入一个 Rill 分组；不选则放在未分组，设置页支持「新建分组」。
- **不改动邮箱的已读状态**：使用 `BODY.PEEK[]`，不发送 `STORE \Seen`。
- 邮件正文按邮件阅读场景收敛布局：嵌套布局表格不再逐层描边，图片不显示边框；远程图片默认拦截，阅读时可「显示图片」（按条目持久化）或「始终显示此发件人的图片」（按发件人地址持久化）；两者之后都持续显示图片。

### 3.2 同步范围

- 同步 `INBOX` 与所有自定义文件夹。
- 排除系统文件夹：已发送、草稿、垃圾箱、垃圾邮件，以及 `\All` / `\Flagged` 等虚拟视图，避免重复导入。
- 首次同步：每个文件夹按账号设定的单次数量导入最新邮件作为基线（多文件夹平分单轮总量），更早历史不回溯。
- 后续同步：按 `UIDVALIDITY + 最后 UID` 增量拉取；单轮上限跟随账号设置（默认 60 封，最多 500 封），单文件夹最多 100 封，保证多个文件夹都能推进。

### 3.3 常见厂商支持

| 厂商 | IMAP 服务器 | 端口 | 密码类型 |
| :--- | :--- | :--- | :--- |
| 网易 163/126 | `imap.163.com` | 993 SSL | 客户端授权密码 |
| QQ/Foxmail | `imap.qq.com` | 993 SSL | 16 位授权码（短信获取） |
| Outlook/Hotmail | `outlook.office365.com` | 993 SSL | 应用专用密码 |
| Gmail | `imap.gmail.com` | 993 SSL | 应用专用密码（需微服可达 Google） |
| 阿里/腾讯企业邮 | `imap.mxhichina.com` 等 | 993 SSL | 账号密码或专用码 |

## 4. 技术实现

### 4.1 依赖

- `github.com/emersion/go-imap/v2`（IMAP4rev1 客户端：TLS、LIST、SELECT、SEARCH、FETCH）。
- MIME 解析用标准库：`net/mail`（信头）、`mime/multipart`（多段）、`mime.WordDecoder`（RFC 2047 标题）。不引入 go-message。

### 4.2 管线

1. 调度：邮箱源与普通源一致走 `next_fetch_at` 落库（重启不重抓）；`fetchSource` 对 `imap://` 前缀分支到 `syncEmailMailbox`。
2. `LIST "" "*"` 列出文件夹，按特殊用途标记与中英文名称过滤系统文件夹；`INBOX` 优先，其余按名称排序。
3. 逐文件夹 `SELECT`：
   - 首次或 `UIDVALIDITY` 变化：按序列号取账号同步上限内的最新邮件，建立 `UIDVALIDITY + LastUID` 基线；
   - 后续：`SEARCH UID {last+1}:*`，从最旧的未同步邮件开始，按账号同步上限拉取；
   - `BODY.PEEK[]` 拉全文，不改变 `\Seen`。
4. 同步游标按账号存 `settings` 的 `email_sync_state:{account_id}` 键：`{username, folders: {path: {uid_validity, last_uid}}}`；旧版单账号游标从 `email_sync_state` 兼容读取。不新增表/列/迁移。
5. 字段映射：
   - `guid` = `Message-ID`（缺失用 `SHA-256(From+Subject+Date)` 兜底）
   - `title` = `Subject`（RFC 2047 解码）
   - `author` = `From` 友好名（缺失用邮箱前缀）
   - `published_at` = `Date`
   - `link` 优先 `Archived-At`，次选正文「View in browser / 在浏览器中阅读」链接，兜底空
6. CID 内嵌图：`multipart/related` 内联附件写入媒体缓存目录，`media_cache` 直插 `status='ok'` 行；正文 `cid:` 引用重写为 `/media/{token}/{hash}`（伪 URL `imap://inline/{message-id}/{cid}` 计算 hash）。
7. 正文经邮件专用 `fetch.SanitizeEmailContent` 清洗：保留安全的 `style`/`background`、表格/字体布局标签与 `data:image/...;base64`，兼容协议相对 URL、仅 `srcset` 图片和 Content-Type 标错的 CID 图片；仍拦截脚本与事件属性。图片保留 `/media/{token}/{hash}` 代理地址，同时写入 `data-remote-src` 供阅读器在放行后加载；CID 内联图标记 `data-inline-image=1`，不参与远程图片拦截。组装 `fetch.FeedResult` 后复用 `applyFeedResult`——过滤规则（REQ-037）、关注提醒（REQ-040）、保留策略、`fetch_logs`、OPML/Feed 输出自动生效。
8. 发件人地址写入条目 `media_json.email_sender`；详情返回 `source_feed_url`（判断邮件源）与 `sender_email`，供前端按发件人放行图片。不新增 items 列。
8. 单封解析/入库失败时保留该文件夹旧游标，下轮重试；已入库条目按 `Message-ID` 去重。

### 4.3 API

- `GET /api/v1/settings` 返回 `email` 节：`enabled/block_remote_images/trusted_senders/accounts[]`；每个账号包含 `id/host/port/username/poll_interval_minutes/sync_limit/folder_id/password_set`。
- `PATCH /api/v1/settings` 接受 `email` 节：`accounts` 为整体替换；`account_id + password` 单独保存/清除授权码（`null` 清除，同 AI Key 语义）；`trusted_senders` 为邮件图片放行列表。
- `PATCH /api/v1/items/{id}/email-images`（REQ-044）：单封邮件「显示图片」写入 `items.media_json.email_images_allowed`，详情返回 `email_images_allowed`；仅 `imap://` 源条目可用。
- `POST /api/v1/settings/email/test`：LOGIN + 列出文件夹，12 秒超时，返回 `ok` 与文件夹列表；可带 `account_id` 复用已保存的服务器/账号/授权码。连接失败返回可读原因。成功时清空该账号邮箱源的健康错误并安排立即重试。

## 5. 数据模型与安全

- **不新增表/列/迁移**：
  - 配置存 `settings.app_settings` 的 `email` 节（账号数组、远程图片开关、放行发件人）；每个账号的授权码独立键 `email_password_enc:{account_id}`（SecretBox AES-GCM），旧版单账号回退读取 `email_password_enc`；同步游标独立键 `email_sync_state:{account_id}`。
  - 源：`kind=rss`，`feed_url=imap://{username}`，每个账号一个源；`validateSourceURL` 对 `imap://` 前缀特殊放行。
  - 移除账号时停用对应源并删除其授权码，保留已抓内容；旧版 `imap://{username}/{sender}` 源不删除，保存设置/启动修复时自动停用并标注可手动删除。
- 邮箱凭据与同步游标不进 OPML 导出；备份/恢复沿用 settings 全量语义。

## 6. 降级与异常

- LOGIN 失败 → 源 `last_status=error`，错误含「检查客户端授权码」指引；不阻塞其他源。
- 网络超时按既有退避（`backoffFor`）。
- 单个文件夹打开失败只跳过该文件夹并写日志，其他文件夹继续；全部失败才把源置为错误。
- 被邮箱厂商投进垃圾箱的信件收不到；附件（非内联图）不入正文，仅登记媒体缓存。
- 远程图片默认拦截（含 CSS 背景图）；放行后优先加载 `/media` 代理并带 `referrerpolicy=no-referrer`，代理抓取失败或加载失败时回退原始地址。单封放行按条目持久化，按发件人放行写入 `trusted_senders`。
- 多个账号共用一个同步并发模型；单账号 IMAP 超时或授权码失效只影响该账号源。

## 7. 验收标准

1. 填写 163/QQ/Outlook 任一邮箱 + 授权码，「测试连接」12 秒内反馈状态；成功后侧栏「需关注」中的邮箱源立即消失，无需刷新页面。
2. 设置页不再出现文件夹参数与「按发件人/聚合收集箱」选择。
3. 首次同步后，每个账号的 INBOX 与自定义文件夹最近邮件进入各自以邮箱账号命名的源；已发送/草稿/垃圾箱/垃圾邮件不导入。
3a. 添加两个邮箱账号后，设置页保存两个账号并各生成一个源；移除一个账号只停用它的源，另一个账号继续同步。
3b. 修改某账号的「单次同步数量」后，下一次同步按该数量拉取；其他账号不受影响。
4. 在手机或网页邮箱把新邮件标为已读，下一轮仍能被 Rill 同步；Rill 不把邮箱邮件标为已读。
5. 邮件内嵌 CID 图片正确展示，不裂图；外部链接可点击。
5a. 邮件正文不再出现嵌套表格逐层边框；远程图片默认显示拦截提示，「显示图片」后重新打开同一邮件仍直接显示，「始终显示此发件人的图片」对后续邮件生效。
6. 连续多轮轮询不产生重复条目；UIDVALIDITY 变化后能重新建立基线。
7. 旧版按发件人源在升级后自动停用，内容保留、可手动删除。
8. 授权码改错后，源诊断显示清晰失败原因，其他 RSS 源不受影响。
9. 分组下拉可直接「新建分组」，创建后立即选中。
