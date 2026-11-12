import type {
  AiSummary,
  DigestReport,
  DigestRule,
  FetchLog,
  FilterRule,
  Folder,
  ItemNote,
  PushChannel,
  Settings,
  Source,
  Translation,
} from '@/api/types';

export interface SeedItem {
  id: number;
  source_id: number;
  title: string;
  original_title?: string;
  url: string | null;
  author: string | null;
  published_at: string | null;
  published_estimated: boolean;
  excerpt: string;
  cover_url: string | null;
  read_at: string | null;
  starred_at: string | null;
  later_at?: string | null;
  fetched_at: string;
  content_html: string;
  content_text: string;
  notes?: ItemNote[];
  audio_url?: string;
  audio_type?: string;
  podcast_position_seconds?: number;
  podcast_duration_seconds?: number;
  summary?: AiSummary;
  translation?: Translation;
  /** 被过滤规则隐藏的时间；null/undefined = 正常可见。 */
  hidden_at?: string | null;
}

export interface SeedData {
  folders: Folder[];
  sources: Source[];
  items: SeedItem[];
  fetchLogs: Record<number, FetchLog[]>;
  trends: Record<number, number[]>;
  settings: Settings;
  channels: PushChannel[];
  digestRules: DigestRule[];
  digestReports: DigestReport[];
  filterRules: FilterRule[];
}

const now = new Date();
const iso = (date: Date) => date.toISOString();
const minutesAgo = (minutes: number) => iso(new Date(now.getTime() - minutes * 60_000));
const hoursAgo = (hours: number) => minutesAgo(hours * 60);
const daysAgo = (days: number) => hoursAgo(days * 24);
const minutesFromNow = (minutes: number) => iso(new Date(now.getTime() + minutes * 60_000));
const hoursFromNow = (hours: number) => minutesFromNow(hours * 60);
const daysFromNow = (days: number) => hoursFromNow(days * 24);

const html = (blocks: string[]) => `<article>${blocks.join('')}</article>`;
const p = (text: string) => `<p>${text}</p>`;
const h2 = (text: string) => `<h2>${text}</h2>`;
const quote = (text: string) => `<blockquote><p>${text}</p></blockquote>`;
const list = (items: string[]) => `<ul>${items.map((item) => `<li>${item}</li>`).join('')}</ul>`;
const figure = (caption: string) => (
  `<figure><div class="article-figure-placeholder"><span>媒体代理</span><small>图片由微服缓存 · 源站删除不影响已缓存内容</small></div><figcaption>${caption}</figcaption></figure>`
);

const simpleArticle = (excerpt: string, sourceName: string) => html([
  p(excerpt),
  p(`这篇文章来自「${sourceName}」。Mock API 只返回已经清洗过的 HTML；真实后端会负责抓取、清洗与媒体代理。`),
  h2('为什么值得读'),
  p('前端不解析 Feed，也不拼接媒体地址。它只负责渲染、筛选、已读状态和阅读体验；任何源站问题都不应该让页面白屏。'),
  quote('工具不需要更聪明，只需要更诚实。'),
  p('以上内容为前端联调用的演示正文，不代表真实报道。'),
]);

export const seedFolders: Folder[] = [
  { id: 1, name: '技术', sort: 1, source_count: 7 },
  { id: 2, name: '设计', sort: 2, source_count: 4 },
  { id: 3, name: '资讯', sort: 3, source_count: 5 },
  { id: 4, name: '公众号', sort: 4, source_count: 2 },
  { id: 5, name: '关注', sort: 5, source_count: 2 },
];

export const seedSources: Source[] = [
  {
    id: 1, kind: 'rss', name: '少数派', feed_url: 'https://sspai.com/feed', site_url: 'https://sspai.com',
    folder_id: 1, enabled: true, refresh_seconds: 1800, initial_limit: 20, fallback_url: null,
    backfill_mode: 'window',
    last_fetch_at: minutesAgo(2), last_new_item_at: minutesAgo(18), next_fetch_at: minutesFromNow(28),
    last_status: 'ok', last_error: null, fail_count: 0, item_count: 128, unread_count: 0, icon_url: null,
  },
  {
    id: 2, kind: 'rss', name: '阮一峰的网络日志', feed_url: 'https://ruanyf.github.io/feed.xml', site_url: 'https://www.ruanyifeng.com',
    folder_id: 1, enabled: true, refresh_seconds: 3600, initial_limit: 20, fallback_url: null,
    backfill_mode: 'window',
    last_fetch_at: minutesAgo(18), last_new_item_at: minutesAgo(64), next_fetch_at: minutesFromNow(42),
    last_status: 'ok', last_error: null, fail_count: 0, item_count: 326, unread_count: 0, icon_url: null,
  },
  {
    id: 3, kind: 'rss', name: 'InfoQ 中文', feed_url: 'https://www.infoq.cn/feed', site_url: 'https://www.infoq.cn',
    folder_id: 1, enabled: true, refresh_seconds: 7200, initial_limit: 20, fallback_url: null,
    backfill_mode: 'window',
    last_fetch_at: minutesAgo(41), last_new_item_at: hoursAgo(20), next_fetch_at: minutesFromNow(79),
    last_status: 'ok', last_error: null, fail_count: 0, item_count: 86, unread_count: 0, icon_url: null,
  },
  {
    id: 4, kind: 'rss', name: 'V2EX', feed_url: 'https://www.v2ex.com/index.xml', site_url: 'https://www.v2ex.com',
    folder_id: 1, enabled: true, refresh_seconds: 1800, initial_limit: 20, fallback_url: null,
    backfill_mode: 'window',
    last_fetch_at: minutesAgo(6), last_new_item_at: daysAgo(2), next_fetch_at: minutesFromNow(24),
    last_status: 'ok', last_error: null, fail_count: 0, item_count: 44, unread_count: 0, icon_url: null,
  },
  {
    id: 5, kind: 'rss', name: '掘金', feed_url: 'https://juejin.cn/rss', site_url: 'https://juejin.cn',
    folder_id: 1, enabled: true, refresh_seconds: 3600, initial_limit: 20, fallback_url: null,
    backfill_mode: 'window',
    last_fetch_at: minutesAgo(12), last_new_item_at: hoursAgo(24), next_fetch_at: minutesFromNow(48),
    last_status: 'ok', last_error: null, fail_count: 0, item_count: 73, unread_count: 0, icon_url: null,
  },
  {
    id: 6, kind: 'rss', name: 'GitHub 博客', feed_url: 'https://github.blog/feed/', site_url: 'https://github.blog',
    folder_id: 1, enabled: true, refresh_seconds: 21600, initial_limit: 20, fallback_url: null,
    backfill_mode: 'window',
    last_fetch_at: hoursAgo(3), last_new_item_at: hoursAgo(26), next_fetch_at: hoursFromNow(3),
    last_status: 'ok', last_error: null, fail_count: 0, item_count: 58, unread_count: 0, icon_url: null,
  },
  {
    id: 7, kind: 'rss', name: '云风的博客', feed_url: 'https://blog.codingnow.com/feed', site_url: 'https://blog.codingnow.com',
    folder_id: 1, enabled: true, refresh_seconds: 21600, initial_limit: 20, fallback_url: null,
    backfill_mode: 'window',
    last_fetch_at: hoursAgo(2), last_new_item_at: daysAgo(3), next_fetch_at: hoursFromNow(4),
    last_status: 'ok', last_error: null, fail_count: 0, item_count: 31, unread_count: 0, icon_url: null,
  },
  {
    id: 8, kind: 'rss', name: '设计周刊', feed_url: 'https://design-weekly.example.com/feed.xml', site_url: 'https://design-weekly.example.com',
    folder_id: 2, enabled: true, refresh_seconds: 3600, initial_limit: 20, fallback_url: 'https://folo.example.com/design-weekly/feed.xml',
    backfill_mode: 'window',
    last_fetch_at: minutesAgo(10), last_new_item_at: hoursAgo(6), next_fetch_at: minutesFromNow(50),
    last_status: 'ok', last_error: '连接超时（15 秒）', fail_count: 1, item_count: 112, unread_count: 0, icon_url: null,
    note: null,
  },
  {
    id: 9, kind: 'rss', name: '优设', feed_url: 'https://www.uisdc.com/feed', site_url: 'https://www.uisdc.com',
    folder_id: 2, enabled: true, refresh_seconds: 3600, initial_limit: 20, fallback_url: null,
    backfill_mode: 'window',
    last_fetch_at: minutesAgo(24), last_new_item_at: daysAgo(3), next_fetch_at: minutesFromNow(36),
    last_status: 'ok', last_error: null, fail_count: 0, item_count: 96, unread_count: 0, icon_url: null,
  },
  {
    id: 10, kind: 'rss', name: 'Figma 官方博客', feed_url: 'https://www.figma.com/blog/feed/', site_url: 'https://www.figma.com/blog',
    folder_id: 2, enabled: true, refresh_seconds: 21600, initial_limit: 20, fallback_url: null,
    backfill_mode: 'window',
    last_fetch_at: hoursAgo(4), last_new_item_at: daysAgo(2), next_fetch_at: hoursFromNow(2),
    last_status: 'ok', last_error: null, fail_count: 0, item_count: 67, unread_count: 0, icon_url: null,
  },
  {
    id: 11, kind: 'rss', name: 'Awwwards', feed_url: 'https://www.awwwards.com/feed', site_url: 'https://www.awwwards.com',
    folder_id: 2, enabled: false, refresh_seconds: 43200, initial_limit: 20, fallback_url: null,
    backfill_mode: 'window',
    last_fetch_at: hoursAgo(24), last_new_item_at: hoursAgo(24), next_fetch_at: hoursFromNow(12),
    last_status: 'ok', last_error: null, fail_count: 0, item_count: 39, unread_count: 0, icon_url: null,
    note: '已暂停',
  },
  {
    id: 12, kind: 'rss', name: '爱范儿', feed_url: 'https://www.ifanr.com/feed', site_url: 'https://www.ifanr.com',
    folder_id: 3, enabled: true, refresh_seconds: 1800, initial_limit: 20, fallback_url: null,
    backfill_mode: 'window',
    last_fetch_at: minutesAgo(9), last_new_item_at: hoursAgo(2), next_fetch_at: minutesFromNow(21),
    last_status: 'ok', last_error: null, fail_count: 0, item_count: 151, unread_count: 0, icon_url: null,
  },
  {
    id: 13, kind: 'rss', name: '36氪', feed_url: 'https://36kr.com/feed', site_url: 'https://36kr.com',
    folder_id: 3, enabled: true, refresh_seconds: 1800, initial_limit: 20, fallback_url: null,
    backfill_mode: 'window',
    last_fetch_at: minutesAgo(4), last_new_item_at: hoursAgo(12), next_fetch_at: minutesFromNow(26),
    last_status: 'ok', last_error: null, fail_count: 0, item_count: 204, unread_count: 0, icon_url: null,
  },
  {
    id: 14, kind: 'rss', name: 'The Verge', feed_url: 'https://www.theverge.com/rss/index.xml', site_url: 'https://www.theverge.com',
    folder_id: 3, enabled: true, refresh_seconds: 3600, initial_limit: 20, fallback_url: null,
    backfill_mode: 'window',
    last_fetch_at: minutesAgo(1), last_new_item_at: minutesAgo(18), next_fetch_at: minutesFromNow(59),
    last_status: 'ok', last_error: null, fail_count: 0, item_count: 189, unread_count: 0, icon_url: null,
  },
  {
    id: 15, kind: 'rss', name: '界面新闻', feed_url: 'https://www.jiemian.com/feed', site_url: 'https://www.jiemian.com',
    folder_id: 3, enabled: true, refresh_seconds: 3600, initial_limit: 20, fallback_url: null,
    backfill_mode: 'window',
    last_fetch_at: minutesAgo(16), last_new_item_at: hoursAgo(20), next_fetch_at: minutesFromNow(44),
    last_status: 'ok', last_error: null, fail_count: 0, item_count: 118, unread_count: 0, icon_url: null,
  },
  {
    id: 16, kind: 'rss', name: '虎嗅', feed_url: 'https://www.huxiu.com/rss/0.xml', site_url: 'https://www.huxiu.com',
    folder_id: 3, enabled: true, refresh_seconds: 3600, initial_limit: 20, fallback_url: null,
    backfill_mode: 'window',
    last_fetch_at: minutesAgo(29), last_new_item_at: daysAgo(2), next_fetch_at: minutesFromNow(31),
    last_status: 'ok', last_error: null, fail_count: 0, item_count: 142, unread_count: 0, icon_url: null,
  },
  {
    id: 17, kind: 'wechat', name: '阮一峰的网络日志', feed_url: 'wechat://MzA5MTA0MDAwMTAz', site_url: null,
    folder_id: 4, enabled: true, refresh_seconds: 7200, initial_limit: 20, fallback_url: null,
    backfill_mode: 'window',
    last_fetch_at: minutesAgo(11), last_new_item_at: hoursAgo(1), next_fetch_at: minutesFromNow(109),
    last_status: 'ok', last_error: null, fail_count: 0, item_count: 84, unread_count: 0, icon_url: null,
    note: '微信读书连接有效',
  },
  {
    id: 18, kind: 'wechat', name: '晚点LatePost', feed_url: 'wechat://MzA5MTA0MDAwMDAz', site_url: null,
    folder_id: 4, enabled: true, refresh_seconds: 7200, initial_limit: 20, fallback_url: null,
    backfill_mode: 'window',
    last_fetch_at: hoursAgo(2), last_new_item_at: hoursAgo(18), next_fetch_at: hoursFromNow(1),
    last_status: 'error', last_error: '微信读书登录态已失效，请重新扫码', fail_count: 0, item_count: 61, unread_count: 0, icon_url: null,
    note: '需要重新扫码',
  },
  {
    id: 19, kind: 'x', name: '@example_user', feed_url: 'https://fxtwitter.com/example_user/feed.xml?count=20', site_url: 'https://x.com/example_user',
    folder_id: 5, enabled: true, refresh_seconds: 1800, initial_limit: 20, fallback_url: null,
    backfill_mode: 'window',
    last_fetch_at: minutesAgo(7), last_new_item_at: hoursAgo(2), next_fetch_at: minutesFromNow(23),
    last_status: 'ok', last_error: null, fail_count: 0, item_count: 20, unread_count: 0, icon_url: null,
  },
  {
    id: 20, kind: 'x', name: '@levelsio', feed_url: 'https://fxtwitter.com/levelsio/feed.xml?count=20', site_url: 'https://x.com/levelsio',
    folder_id: 5, enabled: true, refresh_seconds: 1800, initial_limit: 20, fallback_url: null,
    backfill_mode: 'window',
    last_fetch_at: minutesAgo(13), last_new_item_at: hoursAgo(12), next_fetch_at: minutesFromNow(17),
    last_status: 'ok', last_error: null, fail_count: 0, item_count: 20, unread_count: 0, icon_url: null,
  },
];

const vergeOriginal = html([
  p('For the past two years, the feed has been teaching us the same lesson: more is not better. Recommendation systems have become better at guessing what we want, and worse at explaining why a particular story is in front of us. So some readers are walking back — not to a world without the web, but to a simpler promise: what you subscribe to is what you get.'),
  p('RSS never went away. It moved from the little icon next to the address bar into the plumbing: podcasts use it to distribute audio, developers use it to track releases, editors use it to watch a dozen industry sites. It is not good at surprise. It is good at keeping a promise.'),
  h2('From push back to pull'),
  p('The new readers of the past year rarely emphasize discovery. They emphasize what happens after you bring something in: read-later, full text, highlights, summaries, archiving. All of it circles a quieter question: when there is already enough content, what can a tool do less of?'),
  quote('RSS does not promise what you will see. It promises that what you subscribed to is what you can get back.'),
  p('That is also why self-hosted tools feel compelling again. The data is local, the subscription list is an OPML file you can take with you, and your read state does not disappear when a platform redesigns. The cost is obvious: you have to deal with fetch failures, expired logins, image caching, and backups yourself.'),
  figure('阅读器的价值正在从「聚合」转向「筛选与留存」 · 图片经微服代理缓存'),
  p('The more interesting shift is in how people use these tools. Fewer readers try to clear every unread item. They split sources into layers: the daily must-reads, the weekly scan, the search-only archive. A tool that still pushes a giant red badge at you is still thinking like the old feed.'),
  list([
    '把「是否打开」的决定权还给读者，而不是用推送争夺注意力。',
    '让内容以源为单位组织，而不是以一次推荐流为单位。',
    '把阅读进度、收藏与归档留在本地，允许用户随时带走。',
  ]),
  p('The return of the personal feed is not a nostalgia movement. It is a quiet transfer of control: choice away from the algorithm, pace away from the platform. The tool does not need to be smarter. It needs to be more honest.'),
]);

const vergeTranslated = html([
  p('过去两年，信息流一直在教我们同一件事：更多不等于更好。推荐系统越来越擅长猜你想看什么，也越来越难解释它为什么把某一条放在你面前。于是，一部分读者开始往回走——不是回到没有网络的时代，而是回到「我订阅了什么，就取回什么」的协议。'),
  p('RSS 从来没有消失。它只是从浏览器地址栏旁的小图标，退到了工具链的深处。播客用它分发音频，开发者用它追版本发布，编辑用它盯住十几个行业站点。它不擅长制造惊喜，但擅长兑现承诺。'),
  h2('从「推送」回到「取用」'),
  p('这一年出现的新阅读器，大多不再强调「发现」，而是强调「收进来之后怎么读」。稍后读、全文抓取、标注、摘要、归档，所有功能都围绕一个更安静的问题：当内容已经足够多，工具还能替你少做点什么？'),
  quote('RSS 不承诺你会看到什么。它只承诺：你订阅了什么，就能取回什么。'),
  p('这也是自托管工具重新变得性感的原因。数据在本地，订阅清单是一份可以带走的 OPML，已读状态和收藏不会被某个平台的改版清空。代价同样明显：你要自己处理抓取失败、登录态过期、图片缓存和备份。'),
  figure('阅读器的价值，正在从「聚合」转向「筛选与留存」 · 图片经微服代理缓存'),
  p('更值得注意的变化发生在使用方式上。人们不再追求「读完所有未读」，而是把订阅源分成几层：每天必看的、每周扫一眼的、只在需要时搜索的。工具如果还拿一个巨大的红点催你清空，就仍然停留在旧的信息流逻辑里。'),
  list([
    '把「是否打开」的决定权还给读者，而不是用推送争夺注意力。',
    '让内容以源为单位组织，而不是以一次推荐流为单位。',
    '把阅读进度、收藏与归档留在本地，允许用户随时带走。',
  ]),
  p('所以，个人 Feed 的回归并不是一场复古运动。它更像是一次安静的收权：把选择权从算法手里拿回来，把节奏从平台手里拿回来。工具不需要更聪明，只需要更诚实。'),
]);

const vergeSummary: AiSummary = {
  model: 'deepseek-chat',
  generated_at: minutesAgo(2),
  style: 'bullets',
  points: [
    '推荐流让内容更多，却让「为什么看到」变得不可解释。',
    'RSS 的价值不在实时，而在可预期：订阅什么，就取回什么。',
    '自托管把数据与订阅清单还给读者，但把运维成本也一起交还。',
    '下一波阅读工具的机会，在「收进来」与「读得下去」之间。',
  ],
};

const vergeTranslation: Translation = {
  title: '个人订阅的安静回归',
  target_lang: 'zh-CN',
  original_lang: 'en',
  model: 'deepseek-chat',
  generated_at: minutesAgo(3),
  needs_retranslate: false,
  content_html: vergeTranslated,
};

interface ItemSeedInput {
  id: number;
  sourceId: number;
  title: string;
  originalTitle?: string;
  author: string;
  publishedMinutesAgo: number;
  excerpt: string;
  read?: boolean;
  starred?: boolean;
  coverUrl?: string;
  summary?: AiSummary;
  translation?: Translation;
  contentHtml?: string;
  contentText?: string;
  audioUrl?: string;
  audioType?: string;
  podcastPositionSeconds?: number;
  podcastDurationSeconds?: number;
  hidden?: boolean;
}

const sourceNameById = new Map(seedSources.map((source) => [source.id, source.name]));

const makeItem = (input: ItemSeedInput): SeedItem => {
  const sourceName = sourceNameById.get(input.sourceId) ?? 'Rill';
  const content = input.contentHtml ?? simpleArticle(input.excerpt, sourceName);
  return {
    id: input.id,
    source_id: input.sourceId,
    title: input.title,
    original_title: input.originalTitle,
    url: `https://example.com/rill/${input.id}`,
    author: input.author,
    published_at: minutesAgo(input.publishedMinutesAgo),
    published_estimated: false,
    excerpt: input.excerpt,
    cover_url: input.coverUrl ?? null,
    read_at: input.read ? hoursAgo(2) : null,
    starred_at: input.starred ? hoursAgo(3) : null,
    fetched_at: minutesAgo(Math.max(1, input.publishedMinutesAgo - 10)),
    content_html: content,
    content_text: input.contentText ?? `${input.excerpt}\n\n${content.replace(/<[^>]+>/g, ' ')}`,
    audio_url: input.audioUrl,
    audio_type: input.audioType,
    podcast_position_seconds: input.podcastPositionSeconds,
    podcast_duration_seconds: input.podcastDurationSeconds,
    summary: input.summary,
    translation: input.translation,
    hidden_at: input.hidden ? hoursAgo(5) : null,
  };
};

export const seedItems: SeedItem[] = [
  makeItem({
    id: 1, sourceId: 14, title: '个人 Feed 的安静回归', originalTitle: 'The quiet return of the personal feed',
    author: 'David Pierce', publishedMinutesAgo: 18,
    excerpt: '当算法信息流让人疲惫，越来越多的人重新打开 RSS。不是因为它更快，而是因为它更可预期。',
    starred: true, summary: vergeSummary, translation: vergeTranslation,
    coverUrl: '/covers/cover-quiet-feed.svg',
    contentHtml: vergeOriginal,
    contentText: 'RSS never went away. It is good at keeping a promise.',
  }),
  makeItem({
    id: 2, sourceId: 1, title: '我把 RSS 阅读器换成了自托管服务，然后……', author: '张奕', publishedMinutesAgo: 42,
    excerpt: '从 300 个订阅源删到 18 个，最大的收获不是「读得更快」，而是终于知道自己为什么订阅。',
  }),
  makeItem({
    id: 3, sourceId: 2, title: '科技爱好者周刊（第 326 期）：慢下来', author: '阮一峰', publishedMinutesAgo: 64,
    excerpt: '本周话题：当所有工具都在追求更快，慢下来本身是不是一种功能？本期还有 12 条工具与 6 篇文章。',
  }),
  makeItem({
    id: 4, sourceId: 17, title: '微信终于更新了，但这次我劝你先别升级', author: '差评君', publishedMinutesAgo: 89,
    excerpt: '新版本把订阅号入口又挪了一次。对普通用户来说只是换了个位置，对内容创作者来说却是一次流量再分配。',
  }),
  makeItem({
    id: 5, sourceId: 12, title: '一台没有屏幕的阅读器，卖给了谁', author: '李然', publishedMinutesAgo: 133,
    excerpt: '在屏幕越来越多的年代，一家公司反其道而行：只做同步、不做显示。它的用户比想象中更具体。',
    read: true, starred: true,
    coverUrl: '/covers/cover-screenless-reader.svg',
  }),
  makeItem({
    id: 6, sourceId: 19, title: '做了一个很小的 RSS 工具，今晚开源', author: '@example_user', publishedMinutesAgo: 168,
    excerpt: '没有登录、没有推荐、没有信息流。输入一个地址，它就把 feed 收进来。代码很少，今晚整理完发出来。',
  }),
  makeItem({
    id: 7, sourceId: 13, title: 'AI 浏览器大战，没人想只做「摘要」', author: '36氪编辑部', publishedMinutesAgo: 240,
    excerpt: '浏览器厂商正在把 AI 从侧边栏搬到地址栏。但真正决定胜负的，可能不是模型能力，而是默认入口。',
  }),
  makeItem({
    id: 8, sourceId: 8, title: '被算法折叠的编辑设计', author: '设计周刊', publishedMinutesAgo: 300,
    excerpt: '当首页由推荐位组成，编辑的排版语言正在消失。我们还能不能设计「一份被读完的页面」？',
    read: true, starred: true,
    coverUrl: '/covers/cover-editorial-design.svg',
  }),
  makeItem({
    id: 9, sourceId: 9, title: '中文网页排版，先解决这 6 个细节', author: '优设', publishedMinutesAgo: 380,
    excerpt: '行高、字距、标点挤压、中西文混排、着重号与段间距。本文用一组可复用的数值把它们说清楚。',
    read: true, starred: true,
  }),
  makeItem({
    id: 10, sourceId: 18, title: '多模态模型的下一站：更小的上下文窗口？', author: '机器之心', publishedMinutesAgo: 460,
    excerpt: '当上下文窗口从 1M 继续膨胀，一些研究团队开始反向思考：更小、更结构化的上下文是否更有效？',
    read: true,
  }),
  makeItem({
    id: 11, sourceId: 3, title: '从 0 到 1 搭建可观测的抓取管线', author: 'InfoQ 中文', publishedMinutesAgo: 540,
    excerpt: '抓取失败不是异常，而是常态。如何用指标、日志和抓取历史，把「源站挂了」与「我们没抓」区分开？',
    read: true,
  }),
  makeItem({
    id: 12, sourceId: 20, title: '一个人的产品，如何做客户支持', author: '@levelsio', publishedMinutesAgo: 620,
    excerpt: '没有客服团队，也不打算有。我的做法是把 80% 的问题写进产品里，把剩下的 20% 当成产品访谈。',
    read: true,
    hidden: true,
  }),
  makeItem({
    id: 13, sourceId: 5, title: '2026 年前端状态管理，还需要独立状态库吗', author: '掘金前端', publishedMinutesAgo: 700,
    excerpt: '信号、原子化状态、服务端状态……当框架自己开始管理状态，独立状态库的边界正在变化。',
    read: true,
  }),
  makeItem({
    id: 14, sourceId: 4, title: '你还在用 RSS 吗？', author: 'V2EX', publishedMinutesAgo: 780,
    excerpt: '一个很朴素的问题，收到了两百多条回复。有人说每天看，有人说只用来追更新，还有人说自己写了一个。',
    read: true,
  }),
  makeItem({
    id: 15, sourceId: 15, title: '内容平台开始重新重视「订阅」', author: '界面新闻', publishedMinutesAgo: 900,
    excerpt: '在追逐推荐算法多年之后，几家内容平台悄悄把「关注」和「订阅」放回了更显眼的位置。',
  }),
  makeItem({
    id: 16, sourceId: 16, title: '信息过载之后，工具正在变轻', author: '虎嗅', publishedMinutesAgo: 1000,
    excerpt: '从「什么都能做」到「只做一件小事」，新一代效率工具正在收缩功能，换取更低的启动成本。',
  }),
  makeItem({
    id: 17, sourceId: 14, title: '浏览器的新战场：阅读模式', originalTitle: 'The browser’s new battleground: reader mode', author: 'The Verge', publishedMinutesAgo: 1200,
    excerpt: '阅读模式不再是「去掉广告」的小按钮。浏览器正在把它变成搜索、摘要与翻译的入口。',
    starred: true,
    coverUrl: '/covers/cover-browser-reading.svg',
  }),
  makeItem({
    id: 18, sourceId: 1, title: '我的信息减负实验：从 300 个源到 18 个', author: '少数派', publishedMinutesAgo: 1400,
    excerpt: '删订阅只是第一步。更难的是接受「有些内容永远不会被读到」，并让工具帮你承担这个决定。',
    starred: true,
  }),
  makeItem({
    id: 19, sourceId: 2, title: '开发者工具 2026 年回顾', author: '阮一峰', publishedMinutesAgo: 1600,
    excerpt: '今年最明显的变化不是新框架，而是工具开始围绕「本地优先」与「可迁移」重新设计。',
  }),
  makeItem({
    id: 20, sourceId: 9, title: '中文标题字距的 3 个常见错误', author: '优设', publishedMinutesAgo: 1800,
    excerpt: '字距不是越紧越高级。标题、正文、按钮需要三套不同的字距规则，混用会让版面失去节奏。',
  }),
  makeItem({
    id: 21, sourceId: 6, title: 'GitHub 开发者报告：本地优先工具正在变多', author: 'GitHub 博客', publishedMinutesAgo: 2100,
    excerpt: '过去一年增长最快的开源项目，大多在解决「把数据放回自己手里」这一件事。',
    read: true,
  }),
  makeItem({
    id: 22, sourceId: 7, title: '从零实现一个轻量协程调度器', author: '云风', publishedMinutesAgo: 2300,
    excerpt: '没有宏、没有复杂抽象，只保留最必要的栈切换与任务队列。本文记录一次周末实验。',
    starred: true,
  }),
  makeItem({
    id: 23, sourceId: 10, title: '设计系统不是组件库，而是一套决策记录', author: 'Figma 官方博客', publishedMinutesAgo: 2600,
    excerpt: '当组件数量增长，真正需要维护的不是样式，而是团队对「什么时候用哪个模式」的共同判断。',
  }),
  makeItem({
    id: 24, sourceId: 11, title: '本周最佳网页：把滚动做成叙事', author: 'Awwwards', publishedMinutesAgo: 3000,
    excerpt: '六个页面用不同的滚动节奏讲同一个故事。它们共同点是：动效服从阅读，而不是反过来。',
    read: true,
    hidden: true,
  }),
];

const designWeeklyLogs: FetchLog[] = [
  { id: 1, started_at: minutesAgo(10), duration_ms: 412, http_status: 200, result: 'ok', new_count: 0, error: null },
  { id: 2, started_at: minutesAgo(70), duration_ms: 189, http_status: 304, result: 'not_modified', new_count: 0, error: null },
  { id: 3, started_at: minutesAgo(130), duration_ms: 15000, http_status: null, result: 'error', new_count: 0, error: '连接超时（15 秒）' },
  { id: 4, started_at: hoursAgo(8), duration_ms: 638, http_status: 200, result: 'ok', new_count: 2, error: null },
  { id: 5, started_at: hoursAgo(9), duration_ms: 212, http_status: 304, result: 'not_modified', new_count: 0, error: null },
  { id: 6, started_at: hoursAgo(10), duration_ms: 522, http_status: 200, result: 'ok', new_count: 1, error: null },
];

const machineHeartLogs: FetchLog[] = [
  { id: 1, started_at: hoursAgo(2), duration_ms: 218, http_status: 401, result: 'error', new_count: 0, error: '公众号登录态已失效，请重新扫码' },
  { id: 2, started_at: hoursAgo(4), duration_ms: 198, http_status: 401, result: 'error', new_count: 0, error: '公众号登录态已失效，请重新扫码' },
  { id: 3, started_at: hoursAgo(6), duration_ms: 304, http_status: 200, result: 'ok', new_count: 1, error: null },
];

export const seedSettings: Settings = {
  reader: {
    font_size: 18,
    line_width: 660,
    line_height: 1.85,
    theme: 'system',
    accent: 'blue',
    auto_read: true,
    remember_position: true,
    rail_width: 224,
    list_width: 384,
    views: [],
  },
  ai: {
    enabled: true,
    base_url: 'https://api.deepseek.com/v1',
    model: 'deepseek-chat',
    api_key_set: true,
    summary_style: 'bullets',
    target_lang: 'zh-CN',
    daily_limit: 200,
    used_today: 34,
    tokens_month: 1_200_000,
    consent: true,
  },
  wechat: {
    enabled: true,
    weread_connected: false,
    weread_can_renew: false,
  },
  backup: {
    daily: true,
    keep: 7,
    last_backup_at: hoursAgo(8),
  },
  feed: {
    token_masked: 'rill_9f3c…a71e',
  },
  security: {
    last_login_at: hoursAgo(12),
    session_expires_at: daysFromNow(6),
  },
  mcp: {
    enabled: true,
    endpoint: '/mcp',
  },
};

// REQ-036/037 演示数据：webhook 均为假地址，只保留打码形式。
export const seedChannels: PushChannel[] = [
  {
    id: 1,
    name: '飞书群机器人',
    kind: 'feishu',
    url_masked: 'https://open.feishu.cn/open-apis/bot/v2/hook/****a71e',
    has_secret: false,
    enabled: true,
    created_at: daysAgo(9),
    updated_at: daysAgo(2),
  },
];

export const seedDigestRules: DigestRule[] = [
  {
    id: 1,
    name: '每日晨报',
    scope: [],
    period: 'daily',
    run_at_local: '08:30',
    only_unread: true,
    push_when_empty: false,
    enabled: true,
    channel_ids: [1],
    last_run_at: hoursAgo(10),
    next_run_at: hoursFromNow(14),
    created_at: daysAgo(9),
    updated_at: daysAgo(1),
  },
  {
    id: 2,
    name: '设计周报',
    scope: ['folder:2'],
    period: 'weekly',
    run_at_local: '09:00',
    weekday: 1,
    only_unread: false,
    push_when_empty: true,
    enabled: false,
    channel_ids: [],
    last_run_at: null,
    next_run_at: null,
    created_at: daysAgo(6),
    updated_at: daysAgo(6),
  },
];

const morningReportMd = `# 每日晨报

**时间范围**：昨天 08:30 ~ 今天 08:30（本地时间）
**收录条目**：6 条 · 状态 ok

## 重点推荐

- **个人 Feed 的安静回归**（The Verge）— 当算法信息流让人疲惫，越来越多的人重新打开 RSS。
- **我把 RSS 阅读器换成了自托管服务，然后……**（少数派）— 从 300 个订阅源删到 18 个。

## 其余更新

- 科技爱好者周刊（第 326 期）：慢下来 — 阮一峰的网络日志
- 做了一个很小的 RSS 工具，今晚开源 — @example_user
- AI 浏览器大战，没人想只做「摘要」 — 36氪

> 由 AI 生成，仅供参考。`;

export const seedDigestReports: DigestReport[] = [
  {
    id: 1,
    rule_id: 1,
    rule_name: '每日晨报',
    period_start: hoursAgo(24),
    period_end: hoursAgo(1),
    item_count: 6,
    status: 'ok',
    content_md: morningReportMd,
    model: 'deepseek-chat',
    push_status: 'ok',
    push_error: null,
    pushed_at: hoursAgo(1),
    created_at: hoursAgo(1),
  },
  {
    id: 2,
    rule_id: 1,
    rule_name: '每日晨报',
    period_start: hoursAgo(48),
    period_end: hoursAgo(24),
    item_count: 0,
    status: 'empty',
    content_md: '# 每日晨报\n\n**时间范围**：前天 08:30 ~ 昨天 08:30\n\n该时段内没有新的未读条目。',
    model: 'deepseek-chat',
    push_status: 'none',
    push_error: null,
    pushed_at: null,
    created_at: hoursAgo(24),
  },
];

export const seedFilterRules: FilterRule[] = [
  {
    id: 1,
    name: '屏蔽促销标题',
    pattern: '促销|优惠|限时',
    is_regex: true,
    match_field: 'title',
    action: 'hide',
    scope: [],
    enabled: true,
    hit_count: 12,
    created_at: daysAgo(12),
    updated_at: daysAgo(3),
  },
];

export const createSeedData = (): SeedData => ({
  folders: seedFolders.map((folder) => ({ ...folder })),
  sources: seedSources.map((source) => ({ ...source })),
  items: seedItems.map((item) => ({ ...item })),
  fetchLogs: {
    8: designWeeklyLogs.map((log) => ({ ...log })),
    18: machineHeartLogs.map((log) => ({ ...log })),
  },
  trends: {
    8: [1, 0, 2, 1, 0, 0, 3, 2, 0, 0, 1, 0, 2, 0],
  },
  settings: {
    reader: { ...seedSettings.reader },
    ai: { ...seedSettings.ai },
    wechat: { ...seedSettings.wechat },
    backup: { ...seedSettings.backup },
    feed: { ...seedSettings.feed },
    security: { ...seedSettings.security },
    mcp: { ...seedSettings.mcp },
  },
  channels: seedChannels.map((channel) => ({ ...channel })),
  digestRules: seedDigestRules.map((rule) => ({ ...rule, scope: [...rule.scope], channel_ids: [...rule.channel_ids] })),
  digestReports: seedDigestReports.map((report) => ({ ...report })),
  filterRules: seedFilterRules.map((rule) => ({ ...rule, scope: [...rule.scope] })),
});
