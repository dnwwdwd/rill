/* Rill 原型 · 演示数据。所有内容均为虚构示例，不含真实凭据。 */

const SCREENS = [
  { id: 'login', index: '01', name: '登录' },
  { id: 'reader', index: '02', name: '阅读' },
  { id: 'detail', index: '03', name: '详情' },
  { id: 'sources', index: '04', name: '源管理' },
  { id: 'health', index: '05', name: '源诊断' },
  { id: 'settings', index: '06', name: '设置' },
];

const FOLDERS = [
  { id: 'tech', name: '技术', index: '01' },
  { id: 'design', name: '设计', index: '02' },
  { id: 'news', name: '资讯', index: '03' },
  { id: 'wechat', name: '公众号', index: '04' },
  { id: 'follow', name: '关注', index: '05' },
];

const SOURCES = [
  {
    id: 'sspai', name: '少数派', kind: 'rss', folder: '技术', icon: '少', color: '#8C3B2A',
    url: 'sspai.com/feed', feedUrl: 'https://sspai.com/feed', siteUrl: 'https://sspai.com',
    enabled: true, status: 'ok', interval: '30 分钟', lastFetch: '2 分钟前', lastNew: '2 分钟前',
    nextFetch: '今天 11:12', failCount: 0, lastError: '', note: '', fallback: '', itemCount: 128,
  },
  {
    id: 'ruanyf', name: '阮一峰的网络日志', kind: 'rss', folder: '技术', icon: '阮', color: '#2F5D50',
    url: 'ruanyf.github.io/feed.xml', feedUrl: 'https://ruanyf.github.io/feed.xml', siteUrl: 'https://www.ruanyifeng.com',
    enabled: true, status: 'ok', interval: '1 小时', lastFetch: '18 分钟前', lastNew: '今天 09:56',
    nextFetch: '今天 11:30', failCount: 0, lastError: '', note: '', fallback: '', itemCount: 326,
  },
  {
    id: 'infoq', name: 'InfoQ 中文', kind: 'rss', folder: '技术', icon: 'I', color: '#3A4D7A',
    url: 'infoq.cn/feed', feedUrl: 'https://www.infoq.cn/feed', siteUrl: 'https://www.infoq.cn',
    enabled: true, status: 'ok', interval: '2 小时', lastFetch: '41 分钟前', lastNew: '昨天 14:05',
    nextFetch: '今天 12:10', failCount: 0, lastError: '', note: '', fallback: '', itemCount: 86,
  },
  {
    id: 'v2ex', name: 'V2EX', kind: 'rss', folder: '技术', icon: 'V', color: '#6B4A7A',
    url: 'v2ex.com/index.xml', feedUrl: 'https://www.v2ex.com/index.xml', siteUrl: 'https://www.v2ex.com',
    enabled: true, status: 'ok', interval: '30 分钟', lastFetch: '6 分钟前', lastNew: '前天 21:48',
    nextFetch: '今天 11:00', failCount: 0, lastError: '', note: '', fallback: '', itemCount: 44,
  },
  {
    id: 'juejin', name: '掘金', kind: 'rss', folder: '技术', icon: '掘', color: '#6F5B2B',
    url: 'juejin.cn/rss', feedUrl: 'https://juejin.cn/rss', siteUrl: 'https://juejin.cn',
    enabled: true, status: 'ok', interval: '1 小时', lastFetch: '12 分钟前', lastNew: '昨天 09:15',
    nextFetch: '今天 11:12', failCount: 0, lastError: '', note: '', fallback: '', itemCount: 73,
  },
  {
    id: 'github', name: 'GitHub 博客', kind: 'rss', folder: '技术', icon: 'G', color: '#2E5C6B',
    url: 'github.blog/feed', feedUrl: 'https://github.blog/feed/', siteUrl: 'https://github.blog',
    enabled: true, status: 'ok', interval: '6 小时', lastFetch: '3 小时前', lastNew: '昨天 18:40',
    nextFetch: '今天 15:20', failCount: 0, lastError: '', note: '', fallback: '', itemCount: 58,
  },
  {
    id: 'yunfeng', name: '云风的博客', kind: 'rss', folder: '技术', icon: '云', color: '#5A493F',
    url: 'codingnow.com/feed', feedUrl: 'https://blog.codingnow.com/feed', siteUrl: 'https://blog.codingnow.com',
    enabled: true, status: 'ok', interval: '6 小时', lastFetch: '2 小时前', lastNew: '3 天前',
    nextFetch: '今天 14:40', failCount: 0, lastError: '', note: '', fallback: '', itemCount: 31,
  },
  {
    id: 'designweekly', name: '设计周刊', kind: 'rss', folder: '设计', icon: '设', color: '#7A3E4B',
    url: 'design-weekly.example.com/feed.xml', feedUrl: 'https://design-weekly.example.com/feed.xml', siteUrl: 'https://design-weekly.example.com',
    enabled: true, status: 'warn', interval: '1 小时', lastFetch: '今天 10:30', lastNew: '6 小时前',
    nextFetch: '今天 12:30', failCount: 1, lastError: '连接超时（15 秒）',
    note: '可能已停止更新', fallback: 'Folo 镜像', itemCount: 112,
  },
  {
    id: 'uisdc', name: '优设', kind: 'rss', folder: '设计', icon: '优', color: '#4A5B3A',
    url: 'uisdc.com/feed', feedUrl: 'https://www.uisdc.com/feed', siteUrl: 'https://www.uisdc.com',
    enabled: true, status: 'ok', interval: '1 小时', lastFetch: '24 分钟前', lastNew: '3 天前',
    nextFetch: '今天 11:25', failCount: 0, lastError: '', note: '', fallback: '', itemCount: 96,
  },
  {
    id: 'figma', name: 'Figma 官方博客', kind: 'rss', folder: '设计', icon: 'F', color: '#3F4E6B',
    url: 'figma.com/blog/feed', feedUrl: 'https://www.figma.com/blog/feed/', siteUrl: 'https://www.figma.com/blog',
    enabled: true, status: 'ok', interval: '6 小时', lastFetch: '4 小时前', lastNew: '2 天前',
    nextFetch: '今天 16:20', failCount: 0, lastError: '', note: '', fallback: '', itemCount: 67,
  },
  {
    id: 'awwwards', name: 'Awwwards', kind: 'rss', folder: '设计', icon: 'A', color: '#6B3A3A',
    url: 'awwwards.com/feed', feedUrl: 'https://www.awwwards.com/feed', siteUrl: 'https://www.awwwards.com',
    enabled: false, status: 'paused', interval: '12 小时', lastFetch: '昨天 22:10', lastNew: '昨天 22:10',
    nextFetch: '已暂停', failCount: 0, lastError: '', note: '已暂停', fallback: '', itemCount: 39,
  },
  {
    id: 'ifanr', name: '爱范儿', kind: 'rss', folder: '资讯', icon: '爱', color: '#3A5A5A',
    url: 'ifanr.com/feed', feedUrl: 'https://www.ifanr.com/feed', siteUrl: 'https://www.ifanr.com',
    enabled: true, status: 'ok', interval: '30 分钟', lastFetch: '9 分钟前', lastNew: '今天 08:47',
    nextFetch: '今天 11:09', failCount: 0, lastError: '', note: '', fallback: '', itemCount: 151,
  },
  {
    id: '36kr', name: '36氪', kind: 'rss', folder: '资讯', icon: '36', color: '#5B3E6B',
    url: '36kr.com/feed', feedUrl: 'https://36kr.com/feed', siteUrl: 'https://36kr.com',
    enabled: true, status: 'ok', interval: '30 分钟', lastFetch: '4 分钟前', lastNew: '昨天 22:40',
    nextFetch: '今天 11:04', failCount: 0, lastError: '', note: '', fallback: '', itemCount: 204,
  },
  {
    id: 'verge', name: 'The Verge', kind: 'rss', folder: '资讯', icon: 'V', color: '#4F4A3A',
    url: 'theverge.com/rss/index.xml', feedUrl: 'https://www.theverge.com/rss/index.xml', siteUrl: 'https://www.theverge.com',
    enabled: true, status: 'ok', interval: '1 小时', lastFetch: '1 分钟前', lastNew: '今天 10:42',
    nextFetch: '今天 11:41', failCount: 0, lastError: '', note: '', fallback: '', itemCount: 189,
  },
  {
    id: 'jiemian', name: '界面新闻', kind: 'rss', folder: '资讯', icon: '界', color: '#2E4A5A',
    url: 'jiemian.com/feed', feedUrl: 'https://www.jiemian.com/feed', siteUrl: 'https://www.jiemian.com',
    enabled: true, status: 'ok', interval: '1 小时', lastFetch: '16 分钟前', lastNew: '昨天 21:10',
    nextFetch: '今天 11:16', failCount: 0, lastError: '', note: '', fallback: '', itemCount: 118,
  },
  {
    id: 'huxiu', name: '虎嗅', kind: 'rss', folder: '资讯', icon: '虎', color: '#6B4E2E',
    url: 'huxiu.com/rss', feedUrl: 'https://www.huxiu.com/rss/0.xml', siteUrl: 'https://www.huxiu.com',
    enabled: true, status: 'ok', interval: '1 小时', lastFetch: '29 分钟前', lastNew: '前天 19:20',
    nextFetch: '今天 11:29', failCount: 0, lastError: '', note: '', fallback: '', itemCount: 142,
  },
  {
    id: 'chaping', name: '差评', kind: 'wechat', folder: '公众号', icon: '差', color: '#3E5E4A',
    url: '公众号 · 差评', feedUrl: 'http://we-mp-rss.lzcapp:8001/feed/chaping.atom', siteUrl: '',
    enabled: true, status: 'ok', interval: '2 小时', lastFetch: '11 分钟前', lastNew: '今天 09:31',
    nextFetch: '今天 13:11', failCount: 0, lastError: '', note: '登录态有效', fallback: '', itemCount: 84,
  },
  {
    id: 'jiqizhixin', name: '机器之心', kind: 'wechat', folder: '公众号', icon: '机', color: '#5A3E4A',
    url: '公众号 · 机器之心', feedUrl: 'http://we-mp-rss.lzcapp:8001/feed/jiqizhixin.atom', siteUrl: '',
    enabled: true, status: 'auth', interval: '2 小时', lastFetch: '今天 09:10', lastNew: '18 小时前',
    nextFetch: '需要重新扫码', failCount: 0, lastError: '伴生服务返回 401：登录态已过期',
    note: '需要重新扫码', fallback: '', itemCount: 61,
  },
  {
    id: 'example_user', name: '@example_user', kind: 'x', folder: '关注', icon: 'X', color: '#4A4A6B',
    url: 'fxtwitter.com/example_user/feed.xml', feedUrl: 'https://fxtwitter.com/example_user/feed.xml?count=20',
    siteUrl: 'https://x.com/example_user', enabled: true, status: 'ok', interval: '30 分钟',
    lastFetch: '7 分钟前', lastNew: '今天 08:12', nextFetch: '今天 11:07', failCount: 0,
    lastError: '', note: '', fallback: '', itemCount: 20,
  },
  {
    id: 'levelsio', name: '@levelsio', kind: 'x', folder: '关注', icon: 'L', color: '#2E5A3E',
    url: 'fxtwitter.com/levelsio/feed.xml', feedUrl: 'https://fxtwitter.com/levelsio/feed.xml?count=20',
    siteUrl: 'https://x.com/levelsio', enabled: true, status: 'ok', interval: '30 分钟',
    lastFetch: '13 分钟前', lastNew: '昨天 11:30', nextFetch: '今天 11:13', failCount: 0,
    lastError: '', note: '', fallback: '', itemCount: 20,
  },
];

const BODY_VERGE_ZH = [
  { type: 'p', text: '过去两年，信息流一直在教我们同一件事：更多不等于更好。推荐系统越来越擅长猜你想看什么，也越来越难解释它为什么把某一条放在你面前。于是，一部分读者开始往回走——不是回到没有网络的时代，而是回到「我订阅了什么，就取回什么」的协议。' },
  { type: 'p', text: 'RSS 从来没有消失。它只是从浏览器地址栏旁的小图标，退到了工具链的深处。播客用它分发音频，开发者用它追版本发布，编辑用它盯住十几个行业站点。它不擅长制造惊喜，但擅长兑现承诺。' },
  { type: 'h2', text: '从「推送」回到「取用」' },
  { type: 'p', text: '这一年出现的新阅读器，大多不再强调「发现」，而是强调「收进来之后怎么读」。稍后读、全文抓取、标注、摘要、归档，所有功能都围绕一个更安静的问题：当内容已经足够多，工具还能替你少做点什么？' },
  { type: 'quote', text: 'RSS 不承诺你会看到什么。它只承诺：你订阅了什么，就能取回什么。' },
  { type: 'p', text: '这也是自托管工具重新变得性感的原因。数据在本地，订阅清单是一份可以带走的 OPML，已读状态和收藏不会被某个平台的改版清空。代价同样明显：你要自己处理抓取失败、登录态过期、图片缓存和备份。' },
  { type: 'image', caption: '阅读器的价值，正在从「聚合」转向「筛选与留存」 · 图片经微服代理缓存' },
  { type: 'p', text: '更值得注意的变化发生在使用方式上。人们不再追求「读完所有未读」，而是把订阅源分成几层：每天必看的、每周扫一眼的、只在需要时搜索的。工具如果还拿一个巨大的红点催你清空，就仍然停留在旧的信息流逻辑里。' },
  { type: 'ul', items: [
    '把「是否打开」的决定权还给读者，而不是用推送争夺注意力。',
    '让内容以源为单位组织，而不是以一次推荐流为单位。',
    '把阅读进度、收藏与归档留在本地，允许用户随时带走。',
  ] },
  { type: 'p', text: '所以，个人 Feed 的回归并不是一场复古运动。它更像是一次安静的收权：把选择权从算法手里拿回来，把节奏从平台手里拿回来。工具不需要更聪明，只需要更诚实。' },
];

const BODY_VERGE_EN = [
  { type: 'p', text: 'For the past two years, the feed has been teaching us the same lesson: more is not better. Recommendation systems have become better at guessing what we want, and worse at explaining why a particular story is in front of us. So some readers are walking back — not to a world without the web, but to a simpler promise: what you subscribe to is what you get.' },
  { type: 'p', text: 'RSS never went away. It moved from the little icon next to the address bar into the plumbing: podcasts use it to distribute audio, developers use it to track releases, editors use it to watch a dozen industry sites. It is not good at surprise. It is good at keeping a promise.' },
  { type: 'h2', text: 'From push back to pull' },
  { type: 'p', text: 'The new readers of the past year rarely emphasize discovery. They emphasize what happens after you bring something in: read-later, full text, highlights, summaries, archiving. All of it circles a quieter question: when there is already enough content, what can a tool do less of?' },
  { type: 'quote', text: 'RSS does not promise what you will see. It promises that what you subscribed to is what you can get back.' },
  { type: 'p', text: 'That is also why self-hosted tools feel compelling again. The data is local, the subscription list is an OPML file you can take with you, and your read state does not disappear when a platform redesigns. The cost is obvious: you have to deal with fetch failures, expired logins, image caching, and backups yourself.' },
  { type: 'p', text: 'The more interesting shift is in how people use these tools. Fewer readers try to clear every unread item. They split sources into layers: the daily must-reads, the weekly scan, the search-only archive. A tool that still pushes a giant red badge at you is still thinking like the old feed.' },
  { type: 'p', text: 'The return of the personal feed is not a nostalgia movement. It is a quiet transfer of control: choice away from the algorithm, pace away from the platform. The tool does not need to be smarter. It needs to be more honest.' },
];

const ITEMS = [
  {
    id: 'v1', sourceId: 'verge', title: '个人 Feed 的安静回归',
    originalTitle: 'The quiet return of the personal feed',
    author: 'David Pierce', published: '今天 10:42', ts: 100,
    excerpt: '当算法信息流让人疲惫，越来越多的人重新打开 RSS。不是因为它更快，而是因为它更可预期。',
    read: false, starred: true, minutes: 6, badge: '译',
    summary: {
      model: 'deepseek-chat', generatedAt: '2 分钟前',
      points: [
        '推荐流让内容更多，却让「为什么看到」变得不可解释。',
        'RSS 的价值不在实时，而在可预期：订阅什么，就取回什么。',
        '自托管把数据与订阅清单还给读者，但把运维成本也一起交还。',
        '下一波阅读工具的机会，在「收进来」与「读得下去」之间。',
      ],
    },
    translation: { model: 'deepseek-chat', generatedAt: '3 分钟前', target: '简体中文', originalLang: '英文', needsRetranslate: false },
    bodyZh: BODY_VERGE_ZH, bodyEn: BODY_VERGE_EN,
  },
  {
    id: 's1', sourceId: 'sspai', title: '我把 RSS 阅读器换成了自托管服务，然后……',
    author: '张奕', published: '今天 10:18', ts: 99,
    excerpt: '从 300 个订阅源删到 18 个，最大的收获不是「读得更快」，而是终于知道自己为什么订阅。',
    read: false, starred: false, minutes: 8,
    bodyZh: [
      { type: 'p', text: '我曾经有一个很漂亮的阅读器：三千多条未读，二十几个文件夹，每天早上打开都像面对一份永远做不完的工作。后来我把它换成了自托管服务，第一件事却是删订阅。' },
      { type: 'p', text: '从 300 个源删到 18 个，留下的标准只有一条：过去一个月里，我是否真的因为某一条内容改变过判断、做过笔记，或者转给过别人。' },
      { type: 'quote', text: '订阅不是收藏。订阅是你愿意让它占用下一次注意力的承诺。' },
      { type: 'p', text: '自托管并没有让阅读变快。它只是把「为什么读」这个问题重新交回到我手里。现在每周日我会花十分钟整理一次源，删掉那些只剩惯性存在的条目。' },
    ],
  },
  {
    id: 'r1', sourceId: 'ruanyf', title: '科技爱好者周刊（第 326 期）：慢下来',
    author: '阮一峰', published: '今天 09:56', ts: 98,
    excerpt: '本周话题：当所有工具都在追求更快，慢下来本身是不是一种功能？本期还有 12 条工具与 6 篇文章。',
    read: false, starred: false, minutes: 5,
    bodyZh: [
      { type: 'p', text: '本周想聊聊「慢」。不是效率低下的慢，而是主动留出缓冲的慢。软件更新越来越快，功能越来越多，但人的注意力并没有跟着扩容。' },
      { type: 'p', text: '一个工具如果每天让你做 50 次选择，它再快也会让人疲惫。相反，一个每天只出现一次、每次只让你决定一件事的工具，可能更接近「好用」。' },
      { type: 'h2', text: '本期内容' },
      { type: 'ul', items: ['一个用 SQLite 做全文搜索的开源阅读器', '如何给静态博客加上 Feed 自动发现', '一篇关于信息节食的访谈', '6 个新的命令行工具'] },
      { type: 'p', text: '最后引用一位读者的话：真正稀缺的不是信息，而是「不被打断的半小时」。' },
    ],
  },
  {
    id: 'c1', sourceId: 'chaping', title: '微信终于更新了，但这次我劝你先别升级',
    author: '差评君', published: '今天 09:31', ts: 97,
    excerpt: '新版本把订阅号入口又挪了一次。对普通用户来说只是换了个位置，对内容创作者来说却是一次流量再分配。',
    read: false, starred: false, minutes: 4, badge: '公众号',
    bodyZh: [
      { type: 'p', text: '微信又更新了。这次改动看起来很小：订阅号入口的位置变了，消息列表的排序也多了一个开关。但每次这种「小改动」，背后都对应着一轮新的流量分配。' },
      { type: 'p', text: '对读者来说，订阅号越来越像一个需要主动进入的房间；对创作者来说，这意味着「被看见」越来越依赖平台的分发，而不是关注关系本身。' },
      { type: 'p', text: '这也是为什么一部分公众号读者开始用 RSS 工具做备份：不是不看好微信，而是不想把唯一的阅读入口交给一次版本更新。' },
    ],
  },
  {
    id: 'x1', sourceId: 'example_user', title: '做了一个很小的 RSS 工具，今晚开源',
    author: '@example_user', published: '今天 08:12', ts: 96,
    excerpt: '没有登录、没有推荐、没有信息流。输入一个地址，它就把 feed 收进来。代码很少，今晚整理完发出来。',
    read: false, starred: false, minutes: 1, badge: 'X',
    bodyZh: [
      { type: 'p', text: '做了一个很小的 RSS 工具。没有登录、没有推荐、没有信息流。输入一个地址，它就把 feed 收进来，按时间排好。' },
      { type: 'p', text: '代码很少，今晚整理完发出来。它不解决「读不完」，只解决「东西散落在十几个网站里」。' },
    ],
  },
  {
    id: 'f1', sourceId: 'ifanr', title: '一台没有屏幕的阅读器，卖给了谁',
    author: '李然', published: '今天 08:47', ts: 95,
    excerpt: '在屏幕越来越多的年代，一家公司反其道而行：只做同步、不做显示。它的用户比想象中更具体。',
    read: true, starred: true, minutes: 5,
    bodyZh: [
      { type: 'p', text: '这台设备没有屏幕，也没有按钮。它唯一的工作是在后台把订阅内容抓下来，同步到局域网里的其他设备。听起来像是一个极客玩具，但它的首批用户里有一半是编辑和研究者。' },
      { type: 'p', text: '他们需要的不是另一块屏幕，而是一个稳定的内容底座：资料在那里，谁想看的时候都能取到，不必关心是手机、平板还是电纸书。' },
      { type: 'p', text: '这也许是「后设备时代」的一个小样本：硬件不再争夺注意力，而是退到基础设施的位置。' },
    ],
  },
  {
    id: 'k1', sourceId: '36kr', title: 'AI 浏览器大战，没人想只做「摘要」',
    author: '36氪编辑部', published: '昨天 22:40', ts: 94,
    excerpt: '浏览器厂商正在把 AI 从侧边栏搬到地址栏。但真正决定胜负的，可能不是模型能力，而是默认入口。',
    read: false, starred: false, minutes: 7,
    bodyZh: [
      { type: 'p', text: '过去一年，几乎所有浏览器都上线了 AI 侧边栏。它们能总结网页、回答问题、比较商品。但用户的使用频率并没有跟上功能数量。' },
      { type: 'p', text: '原因很简单：摘要是一个「可做可不做」的动作。只有当 AI 进入地址栏、搜索框、新标签页这些默认路径，它才可能变成习惯。' },
      { type: 'p', text: '这也是浏览器厂商真正的焦虑：如果用户不再从浏览器开始一天的上网，那么侧边栏再聪明，也只是房间里的一件家具。' },
    ],
  },
  {
    id: 'd1', sourceId: 'designweekly', title: '被算法折叠的编辑设计',
    author: '设计周刊', published: '昨天 20:05', ts: 93,
    excerpt: '当首页由推荐位组成，编辑的排版语言正在消失。我们还能不能设计「一份被读完的页面」？',
    read: true, starred: true, minutes: 6,
    bodyZh: [
      { type: 'p', text: '编辑设计的黄金时代，建立在「一页纸」的假设上：读者会从上到下看完，版面因此可以讲述一个完整的故事。信息流打破了这个假设，也带走了版面叙事的语法。' },
      { type: 'p', text: '今天的首页更像一个可以被无限折叠的列表。每一屏都在争夺下一次点击，而不是为整页负责。' },
      { type: 'quote', text: '当所有内容都可以被折叠，设计就不再是安排顺序，而是决定什么值得被展开。' },
      { type: 'p', text: '也许下一代的编辑设计，不是回到报纸，而是重新发明「一屏之内」的完整感：一幅图、一个标题、一段足够好的摘要，让读者在离开之前已经获得了一点东西。' },
    ],
  },
  {
    id: 'u1', sourceId: 'uisdc', title: '中文网页排版，先解决这 6 个细节',
    author: '优设', published: '昨天 18:22', ts: 92,
    excerpt: '行高、字距、标点挤压、中西文混排、着重号与段间距。本文用一组可复用的数值把它们说清楚。',
    read: true, starred: true, minutes: 9,
    bodyZh: [
      { type: 'p', text: '中文网页排版的问题，很少是「字体不够好看」，更多是细节没有被照顾：行高太紧、标点占位不对、中英文之间没有呼吸。' },
      { type: 'h2', text: '六个先改的地方' },
      { type: 'ul', items: ['正文行高不低于 1.7，长文建议 1.8–1.9', '中西文之间留 1/4 字宽，或使用字体自带间距', '标点挤压只用于标题与大字', '段间距稳定在 1em 左右，不靠空行', '链接下划线不要与文字贴死', '代码块与正文使用不同的字号节奏'] },
      { type: 'p', text: '这些数值不是教条，但它们能帮你快速排除掉 80% 的「说不清哪里别扭」。' },
    ],
  },
  {
    id: 'm1', sourceId: 'jiqizhixin', title: '多模态模型的下一站：更小的上下文窗口？',
    author: '机器之心', published: '昨天 16:10', ts: 91,
    excerpt: '当上下文窗口从 1M 继续膨胀，一些研究团队开始反向思考：更小、更结构化的上下文是否更有效？',
    read: true, starred: false, minutes: 6, badge: '公众号',
    bodyZh: [
      { type: 'p', text: '过去一年，上下文窗口几乎是模型发布的固定卖点：从 128K 到 1M，再到更长。但更长不等于更会用。' },
      { type: 'p', text: '一些团队开始把上下文当作一种「工作记忆」来设计：不是把所有材料都塞进去，而是先做结构化检索，再给模型一个更小、更干净的窗口。' },
      { type: 'p', text: '如果这条路成立，下一轮竞争可能不在「能放多少」，而在「知道该放什么」。' },
    ],
  },
  {
    id: 'i1', sourceId: 'infoq', title: '从 0 到 1 搭建可观测的抓取管线',
    author: 'InfoQ 中文', published: '昨天 14:05', ts: 90,
    excerpt: '抓取失败不是异常，而是常态。如何用指标、日志和抓取历史，把「源站挂了」与「我们没抓」区分开？',
    read: true, starred: false, minutes: 10,
    bodyZh: [
      { type: 'p', text: '抓取系统的第一课，是接受失败。超时、404、304、反爬、空 body、解析错误，每一种都对应不同的处理方式，也对应不同的用户解释。' },
      { type: 'p', text: '可观测性不是多打日志，而是让每一次失败都能回答三个问题：发生在哪个源、哪一跳、下一次什么时候重试。' },
      { type: 'p', text: '本文用一个单进程抓取器为例，拆解指标设计、抓取历史表与 UI 状态之间的对应关系。' },
    ],
  },
  {
    id: 'l1', sourceId: 'levelsio', title: '一个人的产品，如何做客户支持',
    author: '@levelsio', published: '昨天 11:30', ts: 89,
    excerpt: '没有客服团队，也不打算有。我的做法是把 80% 的问题写进产品里，把剩下的 20% 当成产品访谈。',
    read: true, starred: false, minutes: 3, badge: 'X',
    bodyZh: [
      { type: 'p', text: 'I do not have a support team, and I do not plan to build one. Most support questions are really product questions in disguise.' },
      { type: 'p', text: '我的做法很简单：同一个问题被问三次，就把它写进空状态、按钮文案或错误提示里。剩下的问题，则是最好的用户访谈。' },
    ],
  },
  {
    id: 'g1', sourceId: 'juejin', title: '2026 年前端状态管理，还需要 Pinia 吗',
    author: '掘金前端', published: '昨天 09:15', ts: 88,
    excerpt: '信号、原子化状态、服务端状态……当框架自己开始管理状态，独立状态库的边界正在变化。',
    read: true, starred: false, minutes: 7,
    bodyZh: [
      { type: 'p', text: '状态管理这件事，正在从「选一个库」变成「分层」：服务端状态交给查询缓存，UI 状态留在组件，跨页面的少量状态才需要全局 store。' },
      { type: 'p', text: 'Pinia 仍然是一个稳妥的选择，但它不再是默认答案。真正的问题是：你的应用里，哪些状态值得跨组件存在？' },
      { type: 'p', text: '当框架的信号系统越来越成熟，独立状态库的价值会从「响应式」转向「组织与调试」。' },
    ],
  },
  {
    id: 'v2', sourceId: 'v2ex', title: '你还在用 RSS 吗？',
    author: 'V2EX', published: '前天 21:48', ts: 87,
    excerpt: '一个很朴素的问题，收到了两百多条回复。有人说每天看，有人说只用来追更新，还有人说自己写了一个。',
    read: true, starred: false, minutes: 2,
    bodyZh: [
      { type: 'p', text: '一个很朴素的问题，收到了两百多条回复。有人说每天看，有人说只用来追更新，还有人说自己写了一个。' },
      { type: 'p', text: '最常被提到的理由不是「没有算法」，而是「可以按自己的节奏」。最常被提到的放弃理由也很一致：配置太麻烦。' },
    ],
  },
  {
    id: 'j1', sourceId: 'jiemian', title: '内容平台开始重新重视「订阅」',
    author: '界面新闻', published: '昨天 21:10', ts: 86,
    excerpt: '在追逐推荐算法多年之后，几家内容平台悄悄把「关注」和「订阅」放回了更显眼的位置。',
    read: false, starred: false, minutes: 5,
    bodyZh: [
      { type: 'p', text: '在追逐推荐算法多年之后，几家内容平台悄悄把「关注」和「订阅」放回了更显眼的位置。原因并不复杂：推荐流拉新，订阅关系留存。' },
      { type: 'p', text: '但平台所说的订阅，与 RSS 的订阅仍然不是一回事。前者是账号内的关系，后者是用户可带走的清单。' },
      { type: 'p', text: '对用户来说，区别可能只在某一天：当平台改版、账号失效或服务关停时，订阅还能不能跟着你走。' },
    ],
  },
  {
    id: 'h1', sourceId: 'huxiu', title: '信息过载之后，工具正在变轻',
    author: '虎嗅', published: '前天 19:20', ts: 85,
    excerpt: '从「什么都能做」到「只做一件小事」，新一代效率工具正在收缩功能，换取更低的启动成本。',
    read: false, starred: false, minutes: 4,
    bodyZh: [
      { type: 'p', text: '上一代效率工具相信「集成」：把笔记、任务、日历、数据库放进一个应用。新一代工具则在反向收缩：只做一件事，并且尽量让这件事不需要学习。' },
      { type: 'p', text: '这不是功能退化，而是对「启动成本」的重新定价。一个需要配置半小时的工具，即使功能再强，也很难进入日常。' },
      { type: 'p', text: '信息过载之后，工具的竞争力可能不再是「能装下多少」，而是「能不能让人愿意打开」。' },
    ],
  },
  {
    id: 'v3', sourceId: 'verge', title: '浏览器的新战场：阅读模式',
    author: 'The Verge', published: '前天 16:40', ts: 84,
    excerpt: '阅读模式不再是「去掉广告」的小按钮。浏览器正在把它变成搜索、摘要与翻译的入口。',
    read: false, starred: true, minutes: 5,
    bodyZh: [
      { type: 'p', text: '阅读模式曾经是一个很边缘的功能：去掉广告、放大字体、把文章变成一页干净的纸。现在，它正在变成浏览器的 AI 入口。' },
      { type: 'p', text: '当浏览器能理解页面正文，它就能在阅读模式里做摘要、翻译、追问。问题也随之而来：页面属于谁？摘要引用到哪里？原始链接还重不重要？' },
      { type: 'p', text: '这场竞争的胜负，可能不取决于模型，而取决于谁能在「读」这件事上建立信任。' },
    ],
  },
  {
    id: 's2', sourceId: 'sspai', title: '我的信息减负实验：从 300 个源到 18 个',
    author: '少数派', published: '前天 10:10', ts: 83,
    excerpt: '删订阅只是第一步。更难的是接受「有些内容永远不会被读到」，并让工具帮你承担这个决定。',
    read: false, starred: true, minutes: 6,
    bodyZh: [
      { type: 'p', text: '删订阅只是第一步。更难的是接受「有些内容永远不会被读到」，并让工具帮你承担这个决定。' },
      { type: 'p', text: '我给每个源设了一个保留窗口：超过 30 天没有打开过的源，自动进入「观察区」；连续两个观察期没有互动，就暂停。不是删除，而是让它先安静下来。' },
      { type: 'p', text: '这套规则运行了三个月，我的未读数从四位数降到了两位数。不是因为我读得更快，而是因为系统终于允许我「不读」。' },
    ],
  },
  {
    id: 'r2', sourceId: 'ruanyf', title: '开发者工具 2026 年回顾',
    author: '阮一峰', published: '前天 09:00', ts: 82,
    excerpt: '今年最明显的变化不是新框架，而是工具开始围绕「本地优先」与「可迁移」重新设计。',
    read: false, starred: false, minutes: 8,
    bodyZh: [
      { type: 'p', text: '今年最明显的变化不是新框架，而是工具开始围绕「本地优先」与「可迁移」重新设计。数据库、同步、搜索，都在回到单机可用的形态。' },
      { type: 'p', text: '另一个变化是「小工具」的回归：一个二进制、一个配置文件、一个 SQLite 文件，部署成本低到可以忽略。' },
      { type: 'p', text: '这不一定意味着云服务退场，但说明开发者对「可掌控」的需求正在变强。' },
    ],
  },
  {
    id: 'u2', sourceId: 'uisdc', title: '中文标题字距的 3 个常见错误',
    author: '优设', published: '3 天前', ts: 81,
    excerpt: '字距不是越紧越高级。标题、正文、按钮需要三套不同的字距规则，混用会让版面失去节奏。',
    read: false, starred: false, minutes: 4,
    bodyZh: [
      { type: 'p', text: '字距不是越紧越高级。标题、正文、按钮需要三套不同的字距规则，混用会让版面失去节奏。' },
      { type: 'p', text: '大字号标题可以适度收紧，但不要把中文压成一条线；正文应该保持默认字距，靠行高建立呼吸；按钮文字则要优先保证点击区域的稳定。' },
      { type: 'p', text: '最稳妥的做法，是先定正文，再定标题，最后才微调按钮。顺序反过来，通常会得到一套互相打架的数值。' },
    ],
  },
];

const FETCH_LOGS = [
  { time: '今天 10:30', result: 'ok', status: '200', newCount: 0, duration: '412 ms', note: '内容未变化，条目数 112' },
  { time: '今天 09:30', result: '304', status: '304', newCount: 0, duration: '189 ms', note: '命中 ETag，只更新抓取时间' },
  { time: '今天 08:30', result: 'error', status: '—', newCount: 0, duration: '15000 ms', note: '连接超时（15 秒），已进入退避' },
  { time: '昨天 23:30', result: 'ok', status: '200', newCount: 2, duration: '638 ms', note: '新增 2 条，最早一条发布于 18:05' },
  { time: '昨天 22:30', result: '304', status: '304', newCount: 0, duration: '212 ms', note: '命中 If-Modified-Since' },
  { time: '昨天 21:30', result: 'ok', status: '200', newCount: 1, duration: '522 ms', note: '新增 1 条' },
];

const NEW_ITEM_TREND = [1, 0, 2, 1, 0, 0, 3, 2, 0, 0, 1, 0, 2, 0];

const DEFAULT_SETTINGS = {
  fontSize: 18,
  lineWidth: 660,
  lineHeight: 1.85,
  theme: 'light',
  autoRead: true,
  rememberPosition: true,
  aiEnabled: true,
  summaryStyle: 'bullets',
  translationTarget: '简体中文',
  dailyLimit: 200,
  usedToday: 34,
  tokensMonth: '1.2M',
  backupDaily: true,
  backupKeep: 7,
  lastBackup: '今天 03:00',
  feedToken: 'rill_9f3c…a71e',
  sidecarEnabled: true,
  wechatStatus: 'expired',
  wechatExpires: '需要重新扫码',
  aiConsent: true,
};

function sourceById(id, sources) {
  const list = sources || SOURCES;
  return list.find((s) => s.id === id) || list[0];
}

function sourceIdsInFolder(folderName, sources) {
  return (sources || SOURCES).filter((s) => s.folder === folderName).map((s) => s.id);
}

function unreadCountForSource(sourceId, items) {
  return items.filter((i) => i.sourceId === sourceId && !i.read).length;
}

function bodyForItem(item) {
  if (item.bodyZh) return item.bodyZh;
  return [
    { type: 'p', text: item.excerpt },
    { type: 'p', text: 'Rill 已抓取并清洗这篇文章的正文。原型中仅展示节选，完整内容以源站为准。' },
  ];
}

Object.assign(window, {
  SCREENS, FOLDERS, SOURCES, ITEMS, FETCH_LOGS, NEW_ITEM_TREND, DEFAULT_SETTINGS,
  sourceById, sourceIdsInFolder, unreadCountForSource, bodyForItem,
});
