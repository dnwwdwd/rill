import type {
  AiSummary,
  AiTestResponse,
  ApiClient,
  AuthConfig,
  BatchStateResponse,
  CreatorSearchResponse,
  DigestPushStatus,
  DigestReport,
  DigestRule,
  DigestRuleInput,
  FeedCandidate,
  FetchLog,
  FilterRule,
  FilterRuleInput,
  Folder,
  ItemDetail,
  ItemNote,
  ItemNoteInput,
  ItemPage,
  ItemQuery,
  ItemSummary,
  LoginResponse,
  MCPToken,
  MCPTokenCreated,
  OpmlImportResult,
  PublicHealth,
  PushChannel,
  PushChannelInput,
  ReaderStats,
  ResolveCandidate,
  RuleScope,
  SessionResponse,
  Settings,
  SettingsPatch,
  Source,
  SourceBatchCreateResponse,
  SourceBatchDeleteResponse,
  SourceBatchUpdateInput,
  SourceBatchUpdateResponse,
  SourceHealth,
  SourceInput,
  SourcePatch,
  SummaryStreamEvent,
  TokenResetResponse,
  Translation,
  TranslationStreamEvent,
  WechatAccount,
  WechatQrCode,
  WeReadScanStatus,
} from '@/api/types';

// 内置公众号模式（REQ-034）的演示账号；添加源用文章链接解析或 BestBlogs 目录。
const MOCK_WECHAT_ACCOUNTS: WechatAccount[] = [
  { fake_id: 'MzA5MTA0MDAwMTAz', name: '阮一峰的网络日志', avatar: '/icon.png' },
];

const mockWechatAccounts = (): WechatAccount[] => MOCK_WECHAT_ACCOUNTS;
import { ApiError } from '@/api/types';
import { createSeedData, type SeedData, type SeedItem } from '@/api/mock/seed';

const SESSION_KEY = 'rill.mock.session';
const SETTINGS_KEY = 'rill.mock.settings';
const AI_RESULTS_KEY = 'rill.mock.ai';
const PODCAST_PROGRESS_KEY = 'rill.mock.podcast';

type StoredAIResults = Record<string, { summary?: AiSummary; translation?: Translation }>;
type StoredPodcastProgress = Record<string, { position: number; duration: number }>;
const delay = (ms = 220) => new Promise<void>((resolve) => {
  window.setTimeout(resolve, import.meta.env.MODE === 'test' ? 0 : ms);
});

const clone = <T,>(value: T): T => JSON.parse(JSON.stringify(value)) as T;
const nowIso = () => new Date().toISOString();

const encodeCursor = (offset: number) => window.btoa(`offset:${offset}`);
const decodeCursor = (cursor?: string | null): number => {
  if (!cursor) return 0;
  try {
    const decoded = window.atob(cursor);
    const parsed = Number(decoded.replace('offset:', ''));
    return Number.isFinite(parsed) && parsed > 0 ? parsed : 0;
  } catch {
    return 0;
  }
};

const sourceSummary = (source: Source) => ({
  id: source.id,
  name: source.name,
  kind: source.kind,
});

const toSummary = (db: SeedData, item: SeedItem): ItemSummary => {
  const source = db.sources.find((entry) => entry.id === item.source_id);
  if (!source) throw new ApiError(500, 'source_missing', '条目关联的源不存在');
  return {
    id: item.id,
    source_id: item.source_id,
    source: sourceSummary(source),
    title: item.title,
    url: item.url,
    author: item.author,
    published_at: item.published_at,
    published_estimated: item.published_estimated,
    excerpt: item.excerpt,
    cover_url: item.cover_url,
    read_at: item.read_at,
    starred_at: item.starred_at,
    later_at: item.later_at ?? null,
    fetched_at: item.fetched_at,
  };
};

const toDetail = (db: SeedData, item: SeedItem): ItemDetail => ({
  ...toSummary(db, item),
  original_title: item.original_title,
  content_html: item.content_html,
  content_text: item.content_text,
  audio_url: item.audio_url,
  audio_type: item.audio_type,
  podcast_position_seconds: item.podcast_position_seconds,
  podcast_duration_seconds: item.podcast_duration_seconds,
  summary: item.summary ? clone(item.summary) : undefined,
  translation: item.translation ? clone(item.translation) : undefined,
  notes: item.notes ? clone(item.notes) : undefined,
});

const sourceUnread = (db: SeedData, sourceId: number) => db.items.filter((item) => item.source_id === sourceId && !item.read_at).length;

const publicSource = (db: SeedData, source: Source): Source => ({
  ...source,
  unread_count: sourceUnread(db, source.id),
});

const tokenMap = new Map<number, string>();
const tokenFor = (sourceId: number) => tokenMap.get(sourceId) ?? `rill_${String(sourceId).padStart(3, '0')}…7a2d`;

const makeLogs = (source: Source): FetchLog[] => {
  const base = new Date();
  return [0, 1, 2].map((index) => ({
    id: source.id * 100 + index,
    started_at: new Date(base.getTime() - (index + 1) * 30 * 60_000).toISOString(),
    duration_ms: 220 + index * 80,
    http_status: index === 0 ? 304 : 200,
    result: index === 0 ? 'not_modified' : 'ok',
    new_count: index === 2 ? 1 : 0,
    error: null,
  }));
};

const makeTrend = (sourceId: number) => Array.from({ length: 14 }, (_, index) => (sourceId * 3 + index * 5) % 4);

const assertAuthed = () => {
  if (window.sessionStorage.getItem(SESSION_KEY) !== '1') {
    window.dispatchEvent(new CustomEvent('rill:unauthorized'));
    throw new ApiError(401, 'unauthorized', '请先登录');
  }
};

const deepMergeSettings = (current: Settings, patch: SettingsPatch): Settings => ({
  reader: { ...current.reader, ...patch.reader },
  ai: { ...current.ai, ...patch.ai },
  wechat: { ...current.wechat, ...patch.wechat },
  backup: { ...current.backup, ...patch.backup },
  feed: { ...current.feed, ...patch.feed },
  security: { ...current.security, ...patch.security },
  mcp: { ...current.mcp, ...patch.mcp },
});

// REQ-036/037 mock 辅助：webhook URL 属于凭据，落库前就打码，任何返回都不含完整 URL。
const maskUrl = (url: string): string => {
  try {
    const parsed = new URL(url);
    const segments = parsed.pathname.split('/').filter(Boolean);
    const tail = segments.length > 0 ? segments[segments.length - 1] : '';
    const maskedTail = tail.length > 4 ? `****${tail.slice(-4)}` : '****';
    segments[segments.length - 1] = maskedTail;
    return `${parsed.origin}/${segments.join('/')}`;
  } catch {
    return '****';
  }
};

// 范围解析：空数组 = 全部源；数字 = 源 ID；"folder:<id>" = 分组内全部源。
const scopeSourceIds = (db: SeedData, scope: RuleScope): Set<number> | 'all' => {
  if (scope.length === 0) return 'all';
  const ids = new Set<number>();
  for (const entry of scope) {
    if (typeof entry === 'number') {
      ids.add(entry);
      continue;
    }
    const folderId = Number(String(entry).slice('folder:'.length));
    if (Number.isFinite(folderId)) {
      for (const source of db.sources) {
        if (source.folder_id === folderId) ids.add(source.id);
      }
    }
  }
  return ids;
};

const inScope = (db: SeedData, scope: RuleScope, item: SeedItem): boolean => {
  const ids = scopeSourceIds(db, scope);
  if (ids === 'all') return true;
  return ids.has(item.source_id);
};

const nextRunAt = (rule: Pick<DigestRule, 'period' | 'run_at_local' | 'weekday'>): string | null => {
  const [hours, minutes] = rule.run_at_local.split(':').map(Number);
  if (!Number.isFinite(hours) || !Number.isFinite(minutes)) return null;
  const candidate = new Date();
  candidate.setHours(hours, minutes, 0, 0);
  if (rule.period === 'weekly') {
    // weekday: 1-7 = 周一~周日；Date.getDay(): 0 = 周日。
    const target = ((rule.weekday ?? 1) % 7);
    let diff = target - candidate.getDay();
    if (diff < 0 || (diff === 0 && candidate.getTime() <= Date.now())) diff += 7;
    candidate.setDate(candidate.getDate() + diff);
  } else if (candidate.getTime() <= Date.now()) {
    candidate.setDate(candidate.getDate() + 1);
  }
  return candidate.toISOString();
};

const validateDigestInput = (input: DigestRuleInput): void => {
  if (!input.name.trim()) throw new ApiError(400, 'name_required', '名称不能为空');
  if (!/^\d{2}:\d{2}$/.test(input.run_at_local) || Number(input.run_at_local.slice(0, 2)) > 23) {
    throw new ApiError(400, 'invalid_run_at', '时间格式应为 HH:MM');
  }
  if (input.period === 'weekly' && !(Number(input.weekday) >= 1 && Number(input.weekday) <= 7)) {
    throw new ApiError(400, 'invalid_weekday', 'weekly 规则需选择 1-7（周一~周日）');
  }
  for (const entry of input.scope) {
    if (typeof entry === 'number') {
      if (!Number.isInteger(entry) || entry <= 0) throw new ApiError(400, 'invalid_scope', '范围里的源 ID 无效');
    } else if (!/^folder:\d+$/.test(String(entry))) {
      throw new ApiError(400, 'invalid_scope', '分组范围格式应为 folder:<id>');
    }
  }
};

const digestReportContent = (db: SeedData, rule: DigestRule, items: SeedItem[], periodStart: Date, periodEnd: Date): string => {
  const fmt = (date: Date) => `${date.getMonth() + 1}月${date.getDate()}日 ${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`;
  const head = `# ${rule.name}\n\n**时间范围**：${fmt(periodStart)} ~ ${fmt(periodEnd)}（本地时间）\n**收录条目**：${items.length} 条\n`;
  if (items.length === 0) {
    return `${head}\n该时段内没有${rule.only_unread ? '新的未读' : '新'}条目。`;
  }
  const sourceName = (item: SeedItem) => db.sources.find((source) => source.id === item.source_id)?.name ?? '未知源';
  const highlights = items.slice(0, 3).map((item) => `- **${item.title}**（${sourceName(item)}）— ${item.excerpt.slice(0, 40)}…`);
  const rest = items.slice(3).map((item) => `- ${item.title} — ${sourceName(item)}`);
  const sections = [
    head,
    items.length > 3 ? '\n## 重点推荐\n' : '\n## 本期更新\n',
    highlights.join('\n'),
  ];
  if (rest.length > 0) sections.push('\n## 其余更新\n', rest.join('\n'));
  sections.push('\n> 由 AI 生成，仅供参考。');
  return sections.join('\n');
};

export const createMockApi = (): ApiClient => {
  let db = createSeedData();
  let mcpTokens: MCPToken[] = [{
    id: 1,
    name: '演示 Agent',
    masked: 'rill_mcp_demo…a71e',
    enabled: true,
    created_at: new Date(Date.now() - 6 * 3600_000).toISOString(),
    last_used_at: new Date(Date.now() - 18 * 60_000).toISOString(),
  }];
  try {
    const stored = window.localStorage.getItem(SETTINGS_KEY);
    if (stored) db.settings = deepMergeSettings(db.settings, JSON.parse(stored) as SettingsPatch);
  } catch {
    /* ignore corrupt demo settings */
  }
  const persistSettings = () => {
    try {
      window.localStorage.setItem(SETTINGS_KEY, JSON.stringify(db.settings));
    } catch {
      /* ignore privacy mode */
    }
  };

  // 演示模式也持久化 AI 结果，行为与真实后端一致：重新打开文章直接显示。
  try {
    const storedAI = window.localStorage.getItem(AI_RESULTS_KEY);
    if (storedAI) {
      const results = JSON.parse(storedAI) as StoredAIResults;
      for (const item of db.items) {
        const entry = results[String(item.id)];
        if (entry?.summary) item.summary = entry.summary;
        if (entry?.translation) item.translation = entry.translation;
      }
    }
  } catch {
    /* ignore corrupt demo AI results */
  }

  const persistAIResult = (itemId: number) => {
    try {
      const raw = window.localStorage.getItem(AI_RESULTS_KEY);
      const results = raw ? (JSON.parse(raw) as StoredAIResults) : {};
      const item = db.items.find((entry) => entry.id === itemId);
      if (!item) return;
      results[String(itemId)] = { summary: item.summary, translation: item.translation };
      window.localStorage.setItem(AI_RESULTS_KEY, JSON.stringify(results));
    } catch {
      /* ignore privacy mode */
    }
  };

  try {
    const storedProgress = window.localStorage.getItem(PODCAST_PROGRESS_KEY);
    if (storedProgress) {
      const progress = JSON.parse(storedProgress) as StoredPodcastProgress;
      for (const item of db.items) {
        const entry = progress[String(item.id)];
        if (entry?.position > 0) {
          item.podcast_position_seconds = entry.position;
          item.podcast_duration_seconds = entry.duration;
        }
      }
    }
  } catch {
    /* ignore corrupt demo podcast progress */
  }

  const persistPodcastProgress = (itemId: number) => {
    try {
      const raw = window.localStorage.getItem(PODCAST_PROGRESS_KEY);
      const progress = raw ? (JSON.parse(raw) as StoredPodcastProgress) : {};
      const item = db.items.find((entry) => entry.id === itemId);
      if (!item) return;
      progress[String(itemId)] = {
        position: item.podcast_position_seconds ?? 0,
        duration: item.podcast_duration_seconds ?? 0,
      };
      window.localStorage.setItem(PODCAST_PROGRESS_KEY, JSON.stringify(progress));
    } catch {
      /* ignore privacy mode */
    }
  };

  const createSourceRecord = (input: SourceInput): Source => {
    const handle = input.kind === 'x' ? input.handle?.replace(/^@/, '') : undefined;
    const feedURL = handle
      ? `https://fxtwitter.com/${handle}/feed.xml?count=20`
      : input.feed_url;
    const name = handle ? `@${handle}` : input.name;
    const siteURL = handle ? `https://x.com/${handle}` : input.site_url ?? null;
    const duplicate = db.sources.some((source) => (
      source.feed_url === feedURL
      || (input.kind === 'x' && source.kind === 'x' && source.name === name)
    ));
    if (duplicate) throw new ApiError(409, 'source_exists', '该源已在订阅列表中');
    const id = Math.max(0, ...db.sources.map((source) => source.id)) + 1;
    const source: Source = {
      id,
      kind: input.kind,
      name,
      feed_url: feedURL,
      site_url: siteURL,
      folder_id: input.folder_id,
      enabled: true,
      refresh_seconds: input.refresh_seconds ?? 3600,
      initial_limit: input.initial_limit ?? 100,
      backfill_mode: input.backfill_mode ?? 'window',
      full_text: input.full_text ?? false,
      fallback_url: input.fallback_url ?? null,
      last_fetch_at: nowIso(),
      last_new_item_at: null,
      next_fetch_at: new Date(Date.now() + 30_000).toISOString(),
      last_status: 'ok',
      last_error: null,
      fail_count: 0,
      item_count: 0,
      unread_count: 0,
      retention_days: input.retention_days ?? 0,
      retention_max_items: input.retention_max_items ?? 0,
      icon_url: input.icon_url ?? null,
      note: input.note ?? '等待首次抓取',
    };
    db.sources.push(source);
    return publicSource(db, source);
  };

  return {
    async getPublicHealth(): Promise<PublicHealth> {
      await delay(80);
      return { ok: true };
    },

    async getAuthConfig(): Promise<AuthConfig> {
      await delay(40);
      return { mode: 'oidc', login_url: '' };
    },

    async getSession(): Promise<SessionResponse> {
      await delay(80);
      const authenticated = window.sessionStorage.getItem(SESSION_KEY) === '1';
      return { authenticated, csrf_token: authenticated ? 'mock-csrf' : '' };
    },

    async signIn(): Promise<LoginResponse> {
      await delay(360);
      // 模拟懒猫 OIDC：登录页按钮直接建立演示会话，不涉及任何真实凭据。
      window.sessionStorage.setItem(SESSION_KEY, '1');
      db.settings.security.last_login_at = nowIso();
      persistSettings();
      return { csrf_token: 'mock-csrf' };
    },

    async logout(): Promise<void> {
      await delay(120);
      window.sessionStorage.removeItem(SESSION_KEY);
    },

    async getFolders(): Promise<Folder[]> {
      assertAuthed();
      await delay();
      return db.folders
        .map((folder) => ({
          ...folder,
          source_count: db.sources.filter((source) => source.folder_id === folder.id).length,
        }))
        .sort((a, b) => a.sort - b.sort);
    },

    async createFolder(name: string, parentId: number | null = null): Promise<Folder> {
      assertAuthed();
      await delay(180);
      const clean = name.trim();
      if (!clean) throw new ApiError(400, 'invalid_folder', '文件夹名称不能为空');
      if ([...clean].length > 200) throw new ApiError(400, 'invalid_folder', '文件夹名称不能超过 200 字');
      if (parentId != null && !db.folders.some((entry) => entry.id === parentId)) {
        throw new ApiError(400, 'invalid_folder', '上级文件夹不存在');
      }
      const folder: Folder = {
        id: Math.max(0, ...db.folders.map((entry) => entry.id)) + 1,
        name: clean,
        sort: Math.max(0, ...db.folders.map((entry) => entry.sort)) + 1,
        parent_id: parentId,
        source_count: 0,
      };
      db.folders.push(folder);
      return { ...folder };
    },

    async updateFolder(id: number, name: string, parentId: number | null = null, sort?: number): Promise<Folder> {
      assertAuthed();
      await delay(180);
      const folder = db.folders.find((entry) => entry.id === id);
      if (!folder) throw new ApiError(404, 'folder_not_found', '文件夹不存在');
      const clean = name.trim();
      if (!clean) throw new ApiError(400, 'invalid_folder', '文件夹名称不能为空');
      if ([...clean].length > 200) throw new ApiError(400, 'invalid_folder', '文件夹名称不能超过 200 字');
      if (parentId === id) throw new ApiError(400, 'invalid_folder', '文件夹不能作为自己的父级');
      if (parentId != null && !db.folders.some((entry) => entry.id === parentId)) {
        throw new ApiError(400, 'invalid_folder', '上级文件夹不存在');
      }
      folder.name = clean;
      folder.parent_id = parentId;
      if (sort != null) folder.sort = sort;
      return { ...folder, source_count: db.sources.filter((source) => source.folder_id === id).length };
    },

    async deleteFolder(id: number): Promise<void> {
      assertAuthed();
      await delay(180);
      if (!db.folders.some((entry) => entry.id === id)) {
        throw new ApiError(404, 'folder_not_found', '文件夹不存在');
      }
      for (const source of db.sources) {
        if (source.folder_id === id) source.folder_id = null;
      }
      db.folders = db.folders.filter((entry) => entry.id !== id);
    },

    async exportOpml(folderId?: number): Promise<string> {
      assertAuthed();
      await delay(180);
      const escape = (value: string) => value
        .replaceAll('&', '&amp;')
        .replaceAll('<', '&lt;')
        .replaceAll('>', '&gt;')
        .replaceAll('"', '&quot;');
      const sourceOutline = (source: Source) => (
        `<outline text="${escape(source.name)}" title="${escape(source.name)}" type="rss" xmlUrl="${escape(source.feed_url)}"` +
        `${source.site_url ? ` htmlUrl="${escape(source.site_url)}"` : ''}/>`
      );
      const folderOutline = (folder: Folder): string => {
        const rows = db.sources.filter((source) => source.folder_id === folder.id);
        const children = db.folders
          .filter((entry) => entry.parent_id === folder.id)
          .sort((a, b) => a.sort - b.sort)
          .map(folderOutline)
          .join('');
        const inner = `${children}${rows.map(sourceOutline).join('')}`;
        return inner ? `<outline text="${escape(folder.name)}" title="${escape(folder.name)}">${inner}</outline>` : '';
      };
      if (folderId) {
        const folder = db.folders.find((entry) => entry.id === folderId);
        if (!folder) throw new ApiError(404, 'folder_not_found', '文件夹不存在');
        const outline = folderOutline(folder);
        return `<?xml version="1.0" encoding="UTF-8"?>\n<opml version="2.0"><head><title>${escape(folder.name)}</title></head><body>${outline}</body></opml>`;
      }
      const groups = db.folders
        .filter((folder) => folder.parent_id == null)
        .sort((a, b) => a.sort - b.sort)
        .map(folderOutline)
        .join('');
      const orphans = db.sources.filter((source) => source.folder_id === null).map(sourceOutline).join('');
      return `<?xml version="1.0" encoding="UTF-8"?>\n<opml version="2.0"><head><title>Rill subscriptions</title></head><body>${groups}${orphans}</body></opml>`;
    },

    async importOpml(file: File, folderId?: number): Promise<OpmlImportResult> {
      assertAuthed();
      await delay(260);
      const text = await file.text();
      const doc = new DOMParser().parseFromString(text, 'text/xml');
      if (doc.querySelector('parsererror') || doc.documentElement.localName !== 'opml') {
        throw new ApiError(400, 'invalid_opml', 'OPML 解析失败');
      }
      const result: OpmlImportResult = { imported: 0, skipped: [] };
      const existing = new Set(db.sources.map((source) => source.feed_url));
      const ensureFolder = (name: string, parentId: number | null): Folder | null => {
        const clean = name.trim();
        if (!clean) return null;
        const found = db.folders.find((folder) => folder.name === clean && (folder.parent_id ?? null) === parentId);
        if (found) return found;
        const folder: Folder = {
          id: Math.max(0, ...db.folders.map((entry) => entry.id)) + 1,
          name: clean,
          sort: Math.max(0, ...db.folders.map((entry) => entry.sort)) + 1,
          parent_id: parentId,
          source_count: 0,
        };
        db.folders.push(folder);
        return folder;
      };
      const walk = (nodes: Element[], folderId: number | null) => {
        for (const node of Array.from(nodes)) {
          const xmlUrl = node.getAttribute('xmlUrl')?.trim() ?? '';
          if (xmlUrl) {
            if (existing.has(xmlUrl)) {
              result.skipped.push({ url: xmlUrl, reason: '已存在' });
              continue;
            }
            const name = (node.getAttribute('title') || node.getAttribute('text') || xmlUrl).trim();
            db.sources.push({
              id: Math.max(0, ...db.sources.map((entry) => entry.id)) + 1,
              kind: 'rss',
              name,
              feed_url: xmlUrl,
              backfill_mode: 'window',
              site_url: node.getAttribute('htmlUrl') || null,
              folder_id: folderId,
              enabled: true,
              refresh_seconds: 1800,
              initial_limit: 20,
              fallback_url: null,
              last_fetch_at: null,
              last_new_item_at: null,
              next_fetch_at: nowIso(),
              last_status: 'ok',
              last_error: null,
              fail_count: 0,
              item_count: 0,
              unread_count: 0,
              icon_url: null,
            });
            existing.add(xmlUrl);
            result.imported += 1;
          } else {
            const name = (node.getAttribute('title') || node.getAttribute('text') || '').trim();
            const folder = name ? ensureFolder(name, folderId) : null;
            walk(Array.from(node.children), folder?.id ?? folderId);
          }
        }
      };
      const body = doc.querySelector('body');
      if (!body) throw new ApiError(400, 'invalid_opml', 'OPML 缺少 body');
      walk(Array.from(body.children), folderId ?? null);
      return result;
    },

    async getSources(): Promise<Source[]> {
      assertAuthed();
      await delay();
      return db.sources.map((source) => publicSource(db, source));
    },

    async getItems(query: ItemQuery = {}): Promise<ItemPage> {
      assertAuthed();
      await delay(180);
      const limit = Math.min(Math.max(query.limit ?? 12, 1), 50);
      const offset = decodeCursor(query.cursor);
      const search = query.q?.trim().toLowerCase();

      const sourceById = new Map(db.sources.map((source) => [source.id, source]));
      const hiddenOnly = Boolean(query.hidden);
      let items = db.items.filter((item) => {
        const source = sourceById.get(item.source_id);
        if (!source) return false;
        // 默认列表排除被隐藏条目；hidden=1 只返回被隐藏条目。
        if (hiddenOnly !== Boolean(item.hidden_at)) return false;
        if (query.source_id && item.source_id !== query.source_id) return false;
        if (query.folder_id && source.folder_id !== query.folder_id) return false;
        if (query.unread && item.read_at) return false;
        if (query.read && !item.read_at) return false;
        if (query.starred && !item.starred_at) return false;
        if (query.later && !item.later_at) return false;
        if (search) {
          const haystack = [item.title, item.excerpt, item.author ?? '', source.name].join(' ').toLowerCase();
          if (!haystack.includes(search)) return false;
        }
        return true;
      });

      items = items.sort((a, b) => {
        const aValue = query.sort === 'fetched' ? a.fetched_at : a.published_at ?? a.fetched_at;
        const bValue = query.sort === 'fetched' ? b.fetched_at : b.published_at ?? b.fetched_at;
        return bValue.localeCompare(aValue);
      });

      const page = items.slice(offset, offset + limit).map((item) => toSummary(db, item));
      const nextOffset = offset + limit;
      return {
        items: page,
        next_cursor: nextOffset < items.length ? encodeCursor(nextOffset) : null,
      };
    },

    async getReaderStats(): Promise<ReaderStats> {
      assertAuthed();
      await delay(80);
      return {
        total: db.items.length,
        unread: db.items.filter((item) => !item.read_at).length,
        starred: db.items.filter((item) => Boolean(item.starred_at)).length,
        read: db.items.filter((item) => Boolean(item.read_at)).length,
        hidden: db.items.filter((item) => Boolean(item.hidden_at)).length,
        later: db.items.filter((item) => Boolean(item.later_at)).length,
      };
    },

    async getItem(id: number): Promise<ItemDetail> {
      assertAuthed();
      await delay(140);
      const item = db.items.find((entry) => entry.id === id);
      if (!item) throw new ApiError(404, 'item_not_found', '条目不存在');
      return toDetail(db, item);
    },

    async getItemMarkdown(id: number): Promise<string> {
      assertAuthed();
      await delay(120);
      const item = db.items.find((entry) => entry.id === id);
      if (!item) throw new ApiError(404, 'item_not_found', '条目不存在');
      const source = db.sources.find((entry) => entry.id === item.source_id);
      const meta = [
        source ? `- 来源：${source.name}` : '',
        item.author ? `- 作者：${item.author}` : '',
        item.published_at ? `- 发布时间：${item.published_at}` : '',
        item.url ? `- 原文链接：${item.url}` : '',
      ].filter(Boolean).join('\n');
      return `# ${item.title}\n\n${meta}${meta ? '\n\n---\n\n' : ''}${item.content_text || item.excerpt}\n`;
    },

    async fetchItemFullText(id: number): Promise<ItemDetail> {
      assertAuthed();
      await delay(360);
      const item = db.items.find((entry) => entry.id === id);
      if (!item) throw new ApiError(404, 'item_not_found', '条目不存在');
      item.content_text = `${item.content_text} 这是 mock 全文抓取补充的正文内容。`;
      item.content_html = `${item.content_html}<p>这是 mock 全文抓取补充的正文内容。</p>`;
      return toDetail(db, item);
    },

    async savePodcastProgress(id: number, positionSeconds: number, durationSeconds: number): Promise<void> {
      assertAuthed();
      await delay(60);
      const item = db.items.find((entry) => entry.id === id);
      if (!item) throw new ApiError(404, 'item_not_found', '条目不存在');
      item.podcast_position_seconds = Math.max(0, positionSeconds);
      item.podcast_duration_seconds = Math.max(0, durationSeconds);
      persistPodcastProgress(id);
    },

    async patchItemState(id: number, patch: { read?: boolean; starred?: boolean; later?: boolean; hidden?: boolean }): Promise<ItemSummary> {
      assertAuthed();
      await delay(100);
      const item = db.items.find((entry) => entry.id === id);
      if (!item) throw new ApiError(404, 'item_not_found', '条目不存在');
      if (patch.read !== undefined) item.read_at = patch.read ? nowIso() : null;
      if (patch.starred !== undefined) item.starred_at = patch.starred ? nowIso() : null;
      if (patch.later !== undefined) item.later_at = patch.later ? nowIso() : null;
      if (patch.hidden !== undefined) item.hidden_at = patch.hidden ? nowIso() : null;
      return toSummary(db, item);
    },

    async batchItemState(ids: number[], patch: { read?: boolean; starred?: boolean; later?: boolean; hidden?: boolean }): Promise<BatchStateResponse> {
      assertAuthed();
      await delay(120);
      let updated = 0;
      const changed: number[] = [];
      for (const item of db.items) {
        if (!ids.includes(item.id)) continue;
        if (patch.read !== undefined) item.read_at = patch.read ? nowIso() : null;
        if (patch.starred !== undefined) item.starred_at = patch.starred ? nowIso() : null;
        if (patch.later !== undefined) item.later_at = patch.later ? nowIso() : null;
        if (patch.hidden !== undefined) item.hidden_at = patch.hidden ? nowIso() : null;
        updated += 1;
        changed.push(item.id);
      }
      return { updated, ids: changed };
    },

    async getItemNotes(id: number): Promise<ItemNote[]> {
      assertAuthed();
      await delay(80);
      const item = db.items.find((entry) => entry.id === id);
      if (!item) throw new ApiError(404, 'item_not_found', '条目不存在');
      return clone(item.notes ?? []);
    },

    async createItemNote(id: number, input: ItemNoteInput): Promise<ItemNote> {
      assertAuthed();
      await delay(100);
      const item = db.items.find((entry) => entry.id === id);
      if (!item) throw new ApiError(404, 'item_not_found', '条目不存在');
      if (!input.quote.trim() && !input.note.trim()) throw new ApiError(400, 'invalid_note', '摘录和笔记至少填写一项');
      const note: ItemNote = {
        id: Math.max(0, ...(item.notes ?? []).map((entry) => entry.id)) + 1,
        item_id: id,
        quote: input.quote.trim(),
        note: input.note.trim(),
        color: input.color ?? 'yellow',
        created_at: nowIso(),
        updated_at: nowIso(),
      };
      item.notes = [...(item.notes ?? []), note];
      return clone(note);
    },

    async updateItemNote(id: number, noteId: number, input: Partial<ItemNoteInput>): Promise<ItemNote> {
      assertAuthed();
      await delay(90);
      const item = db.items.find((entry) => entry.id === id);
      const note = item?.notes?.find((entry) => entry.id === noteId);
      if (!item || !note) throw new ApiError(404, 'note_not_found', '笔记不存在');
      if (input.quote !== undefined) note.quote = input.quote.trim();
      if (input.note !== undefined) note.note = input.note.trim();
      if (input.color !== undefined) note.color = input.color;
      if (!note.quote && !note.note) throw new ApiError(400, 'invalid_note', '摘录和笔记至少填写一项');
      note.updated_at = nowIso();
      return clone(note);
    },

    async deleteItemNote(id: number, noteId: number): Promise<void> {
      assertAuthed();
      await delay(70);
      const item = db.items.find((entry) => entry.id === id);
      if (!item?.notes?.some((entry) => entry.id === noteId)) {
        throw new ApiError(404, 'note_not_found', '笔记不存在');
      }
      item.notes = item.notes.filter((entry) => entry.id !== noteId);
    },

    async markAllRead(target: { source_id?: number; folder_id?: number }): Promise<BatchStateResponse> {
      assertAuthed();
      await delay(160);
      const sourceIds = target.folder_id
        ? (() => {
            const tree = new Set<number>();
            const walk = (id: number) => {
              if (tree.has(id)) return;
              tree.add(id);
              for (const folder of db.folders.filter((entry) => entry.parent_id === id)) walk(folder.id);
            };
            walk(target.folder_id);
            return db.sources.filter((source) => source.folder_id != null && tree.has(source.folder_id)).map((source) => source.id);
          })()
        : null;
      let updated = 0;
      const changed: number[] = [];
      for (const item of db.items) {
        if (item.read_at) continue;
        if (target.source_id && item.source_id !== target.source_id) continue;
        if (sourceIds && !sourceIds.includes(item.source_id)) continue;
        item.read_at = nowIso();
        updated += 1;
        changed.push(item.id);
      }
      return { updated, ids: changed };
    },

    async getSourceHealth(id: number): Promise<SourceHealth> {
      assertAuthed();
      await delay(160);
      const source = db.sources.find((entry) => entry.id === id);
      if (!source) throw new ApiError(404, 'source_not_found', '订阅源不存在');
      const logs = db.fetchLogs[id]?.length ? db.fetchLogs[id] : makeLogs(source);
      const trend = db.trends[id] ?? makeTrend(id);
      return {
        source: publicSource(db, source),
        logs: clone(logs),
        trend: [...trend],
        feed_token_masked: tokenFor(id),
        feed_paths: {
          rss: `/feed/${id}/${tokenFor(id)}.xml`,
          atom: `/feed/${id}/${tokenFor(id)}.atom`,
          json: `/feed/${id}/${tokenFor(id)}.json`,
        },
      };
    },

    async refreshSource(id: number): Promise<Source> {
      assertAuthed();
      await delay(480);
      const source = db.sources.find((entry) => entry.id === id);
      if (!source) throw new ApiError(404, 'source_not_found', '订阅源不存在');
      if (!source.enabled) throw new ApiError(409, 'source_paused', '该源已暂停，请先启用');
      if (source.last_status === 'error' && source.last_error?.includes('401')) {
        throw new ApiError(401, 'wechat_auth_expired', '登录态已过期，请先重新扫码');
      }
      source.last_fetch_at = nowIso();
      source.next_fetch_at = new Date(Date.now() + source.refresh_seconds * 1000).toISOString();
      source.last_status = 'not_modified';
      source.last_error = null;
      db.fetchLogs[id] = [
        {
          id: Date.now(),
          started_at: source.last_fetch_at,
          duration_ms: 189,
          http_status: 304,
          result: 'not_modified' as const,
          new_count: 0,
          error: null,
        },
        ...(db.fetchLogs[id] ?? makeLogs(source)),
      ].slice(0, 50);
      return publicSource(db, source);
    },

    async switchSourceFallback(id: number): Promise<Source> {
      assertAuthed();
      await delay(520);
      const source = db.sources.find((entry) => entry.id === id);
      if (!source) throw new ApiError(404, 'source_not_found', '订阅源不存在');
      if (!source.fallback_url) throw new ApiError(409, 'fallback_missing', '这个源还没有配置备用地址');
      source.last_fetch_at = nowIso();
      source.last_new_item_at = nowIso();
      source.next_fetch_at = new Date(Date.now() + source.refresh_seconds * 1000).toISOString();
      source.last_status = 'ok';
      source.last_error = null;
      source.note = '备用源已接管';
      db.fetchLogs[id] = [
        {
          id: Date.now(),
          started_at: source.last_fetch_at,
          duration_ms: 386,
          http_status: 200,
          result: 'ok' as const,
          new_count: 0,
          error: null,
        },
        ...(db.fetchLogs[id] ?? []),
      ].slice(0, 50);
      return publicSource(db, source);
    },

    async updateSource(id: number, patch: SourcePatch): Promise<Source> {
      assertAuthed();
      await delay(220);
      const source = db.sources.find((entry) => entry.id === id);
      if (!source) throw new ApiError(404, 'source_not_found', '订阅源不存在');
      Object.assign(source, patch);
      return publicSource(db, source);
    },

    async createSource(input: SourceInput): Promise<Source> {
      assertAuthed();
      await delay(320);
      return createSourceRecord(input);
    },

    async createSources(inputs: SourceInput[]): Promise<SourceBatchCreateResponse> {
      assertAuthed();
      await delay(420);
      const created: Source[] = [];
      const errors: SourceBatchCreateResponse['errors'] = [];
      inputs.forEach((input, index) => {
        try {
          created.push(createSourceRecord(input));
        } catch (error) {
          const apiError = error instanceof ApiError ? error : null;
          errors.push({
            index,
            name: input.name,
            feed_url: input.feed_url,
            code: apiError?.code ?? 'internal_error',
            message: apiError?.message ?? '创建订阅源失败',
          });
        }
      });
      return { created, errors };
    },

    async deleteSource(id: number): Promise<void> {
      assertAuthed();
      await delay(220);
      const before = db.sources.length;
      db.sources = db.sources.filter((source) => source.id !== id);
      if (db.sources.length === before) throw new ApiError(404, 'source_not_found', '订阅源不存在');
      db.items = db.items.filter((item) => item.source_id !== id);
      delete db.fetchLogs[id];
      delete db.trends[id];
    },

    async batchUpdateSources(input: SourceBatchUpdateInput): Promise<SourceBatchUpdateResponse> {
      assertAuthed();
      await delay(320);
      if (!input.ids || input.ids.length === 0) {
        throw new ApiError(400, 'sources_required', '请先选择订阅源');
      }
      if (input.ids.length > 100) {
        throw new ApiError(400, 'too_many_sources', '一次最多修改 100 个订阅源');
      }
      if (input.folder_id === undefined && input.enabled === undefined) {
        throw new ApiError(400, 'invalid_update', '没有需要修改的内容');
      }
      if (input.folder_id !== undefined && input.folder_id > 0
        && !db.folders.some((entry) => entry.id === input.folder_id)) {
        throw new ApiError(400, 'folder_not_found', '文件夹不存在');
      }
      let updated = 0;
      const missing: number[] = [];
      for (const id of input.ids) {
        const source = db.sources.find((entry) => entry.id === id);
        if (!source) {
          missing.push(id);
          continue;
        }
        if (input.folder_id !== undefined) {
          source.folder_id = input.folder_id === 0 ? null : input.folder_id;
        }
        if (input.enabled !== undefined) source.enabled = input.enabled;
        updated += 1;
      }
      return missing.length > 0 ? { updated, missing } : { updated };
    },

    async batchDeleteSources(ids: number[]): Promise<SourceBatchDeleteResponse> {
      assertAuthed();
      await delay(320);
      if (!ids || ids.length === 0) {
        throw new ApiError(400, 'sources_required', '请先选择订阅源');
      }
      if (ids.length > 100) {
        throw new ApiError(400, 'too_many_sources', '一次最多删除 100 个订阅源');
      }
      let deleted = 0;
      const missing: number[] = [];
      for (const id of ids) {
        const before = db.sources.length;
        db.sources = db.sources.filter((source) => source.id !== id);
        if (db.sources.length === before) {
          missing.push(id);
          continue;
        }
        deleted += 1;
        db.items = db.items.filter((item) => item.source_id !== id);
        delete db.fetchLogs[id];
        delete db.trends[id];
      }
      return missing.length > 0 ? { deleted, missing } : { deleted };
    },

    async resolveSource(input: string): Promise<ResolveCandidate[]> {
      assertAuthed();
      await delay(420);
      const raw = input.trim();
      const bestblogsFeed = 'https://wechat2rss.bestblogs.dev/feed/ff621c3e98d6ae6fceb3397e57441ffc6ea3c17f.xml';
      const bestblogsXFeed = 'https://api.xgo.ing/rss/user/0c0856a69f9f49cf961018c32a0b0049';
      const bestblogsPodcastFeed = 'https://rsshub.bestblogs.dev/xiaoyuzhou/podcast/5e74b52c418a84a046ecaceb';
      const bestblogsYouTubeFeed = 'https://www.youtube.com/feeds/videos.xml?channel_id=UCrDwWp7EBBv4NwvScIpBDOA';
      if (raw === '数字生命卡兹克' || raw === bestblogsFeed || raw === 'bestblogs:数字生命卡兹克') {
        return [{
          platform: 'bestblogs',
          kind: 'rss',
          name: '数字生命卡兹克',
          feed_url: bestblogsFeed,
          note: 'BestBlogs 托管公众号 · 仅近期文章',
        }];
      }
      if (raw === 'bestblogs:OpenAI' || raw === bestblogsXFeed) {
        return [{
          platform: 'bestblogs-x',
          kind: 'rss',
          name: 'OpenAI(@OpenAI)',
          feed_url: bestblogsXFeed,
          note: 'BestBlogs X 目录 · 第三方 RSS（备用）',
        }];
      }
      if (raw === 'bestblogs:科技早知道' || raw === bestblogsPodcastFeed) {
        return [{
          platform: 'bestblogs-podcast',
          kind: 'rss',
          name: 'What’s Next｜科技早知道',
          feed_url: bestblogsPodcastFeed,
          note: 'BestBlogs 小宇宙目录 · RSSHub（官方 Feed 优先）',
        }];
      }
      if (raw === 'bestblogs:Andrej Karpathy' || raw === bestblogsYouTubeFeed) {
        return [{
          platform: 'bestblogs-youtube',
          kind: 'rss',
          name: 'Andrej Karpathy',
          feed_url: bestblogsYouTubeFeed,
          note: 'YouTube 官方 RSS（BestBlogs 目录收录）',
        }];
      }
      const apple = raw.match(/id(\d{5,})|^apple:(\d{5,})$/i);
      if (apple) {
        const id = apple[1] ?? apple[2];
        return [{
          platform: 'apple',
          kind: 'rss',
          name: `Apple Podcast ${id}`,
          feed_url: 'https://podcast.darknetdiaries.com',
          site_url: `https://podcasts.apple.com/podcast/id${id}`,
          note: 'Apple Podcasts 目录解析',
        }];
      }
      const xyz = raw.match(/xiaoyuzhoufm\.com\/podcast\/([0-9a-f]{24})|^([0-9a-f]{24})$/i);
      if (xyz) {
        const pid = xyz[1] ?? xyz[2];
        return [{
          platform: 'xiaoyuzhou',
          kind: 'rss',
          name: '小宇宙播客',
          feed_url: `xiaoyuzhou://podcast/${pid}`,
          site_url: `https://www.xiaoyuzhoufm.com/podcast/${pid}`,
          note: '小宇宙页面抓取，最近 15 条（非全量）',
          partial: true,
        }];
      }
      if (/^https?:\/\/medium\.com\/@/i.test(raw)) {
        const user = raw.match(/medium\.com\/(@[^/?#]+)/i)?.[1] ?? '@user';
        return [{ platform: 'medium', kind: 'rss', name: user, feed_url: `https://medium.com/feed/${user}`, site_url: raw, note: 'Medium RSS' }];
      }
      if (/^https?:\/\/[^/]+\/@[^/]+\/?$/i.test(raw)) {
        return [{ platform: 'mastodon', kind: 'rss', name: 'Mastodon', feed_url: `${raw.replace(/\/$/, '')}.rss`, site_url: raw, note: 'Mastodon RSS' }];
      }
      return [];
    },

    async searchCreators(query: string, platforms?: string[]): Promise<CreatorSearchResponse> {
      assertAuthed();
      await delay(620);
      const value = query.trim().replace(/^@/, '');
      if (!value) throw new ApiError(400, 'query_required', '请输入创作者名称');
      const normalized = value.toLowerCase();
      const wanted = platforms?.length ? platforms : ['apple', 'bluesky', 'bestblogs'];
      const candidates: CreatorSearchResponse['candidates'] = [];
      const add = (candidate: CreatorSearchResponse['candidates'][number]) => {
        const group = candidate.platform.startsWith('bestblogs') ? 'bestblogs' : candidate.platform;
        if (!wanted.includes(group)) return;
        candidates.push(candidate);
      };
      const bestblogsFeed = 'https://wechat2rss.bestblogs.dev/feed/ff621c3e98d6ae6fceb3397e57441ffc6ea3c17f.xml';
      const bestblogsXFeed = 'https://api.xgo.ing/rss/user/0c0856a69f9f49cf961018c32a0b0049';
      const bestblogsPodcastFeed = 'https://rsshub.bestblogs.dev/xiaoyuzhou/podcast/5e74b52c418a84a046ecaceb';
      const bestblogsYouTubeFeed = 'https://www.youtube.com/feeds/videos.xml?channel_id=UCrDwWp7EBBv4NwvScIpBDOA';

      if (normalized.includes('这集') || normalized.includes('播客')) {
        add({
          platform: 'apple', kind: 'rss', name: '这集我看过',
          feed_url: 'https://feed.xyzfm.space/j7d9mlny9ea6',
          site_url: 'https://podcasts.apple.com/podcast/id1651572616',
          note: 'Apple Podcasts 目录 · 这集我看过基地', match: 'exact', source: 'native',
        });
        add({
          platform: 'bestblogs-podcast', kind: 'rss', name: '这集我看过',
          feed_url: bestblogsPodcastFeed, site_url: 'https://www.xiaoyuzhoufm.com/podcast/634270332a992d56e91e5892',
          note: 'BestBlogs 小宇宙目录 · RSSHub', match: 'exact', source: 'bestblogs',
        });
      }
      if (normalized.includes('openai')) {
        add({
          platform: 'bluesky', kind: 'rss', name: 'OpenAI', handle: 'openai-demo.bsky.social',
          feed_url: 'https://bsky.app/profile/openai-demo.bsky.social/rss',
          site_url: 'https://bsky.app/profile/openai-demo.bsky.social',
          note: 'Bluesky 官方 RSS', match: 'exact', source: 'native',
        });
        add({
          platform: 'bestblogs-x', kind: 'rss', name: 'OpenAI(@OpenAI)', feed_url: bestblogsXFeed,
          note: 'BestBlogs X 目录 · 第三方 RSS（备用）', match: 'exact', source: 'bestblogs',
        });
        add({
          platform: 'bestblogs-youtube', kind: 'rss', name: 'OpenAI', feed_url: bestblogsYouTubeFeed,
          note: 'YouTube 官方 RSS（BestBlogs 目录可搜索）', match: 'exact', source: 'bestblogs',
        });
      }
      if (normalized.includes('阑夕') || normalized.includes('数字生命')) {
        add({
          platform: 'bestblogs', kind: 'rss', name: '数字生命卡兹克', feed_url: bestblogsFeed,
          note: 'BestBlogs 托管公众号 · 仅近期文章', match: 'exact', source: 'bestblogs',
        });
      }
      if (candidates.length === 0) {
        add({
          platform: 'bluesky', kind: 'rss', name: value, handle: 'demo.bsky.social',
          feed_url: 'https://bsky.app/profile/demo.bsky.social/rss',
          site_url: 'https://bsky.app/profile/demo.bsky.social',
          note: 'Bluesky 官方 RSS', match: 'exact', source: 'native',
        });
      }

      const existing = new Map(db.sources.map((source) => [source.feed_url, source.id]));
      for (const candidate of candidates) {
        const sourceID = existing.get(candidate.feed_url);
        if (sourceID) candidate.existing_source_id = sourceID;
      }
      const labels: Record<string, string> = {
        apple: 'Apple Podcasts', bluesky: 'Bluesky', bestblogs: 'BestBlogs 目录',
      };
      const responsePlatforms: CreatorSearchResponse['platforms'] = wanted.map((platform) => {
        const items = candidates.filter((candidate) => (
          platform === 'bestblogs' ? candidate.platform.startsWith('bestblogs') : candidate.platform === platform
        ));
        return {
          platform,
          label: labels[platform] ?? platform,
          status: items.length > 0 ? 'ok' : 'empty',
          count: items.length,
        };
      });
      return { query: value, candidates, platforms: responsePlatforms };
    },

    async discoverFeed(url: string): Promise<FeedCandidate[]> {
      assertAuthed();
      await delay(760);
      const raw = url.trim();
      if (!raw) throw new ApiError(400, 'url_required', '请输入站点地址或订阅地址');
      const cleaned = raw.replace(/^https?:\/\//, '').replace(/\/+$/, '');
      const host = cleaned.split('/')[0] || cleaned;
      const looksLikeFeed = /\.(xml|atom|rss)(\?.*)?$/i.test(cleaned) || cleaned.includes('/feed');
      if (looksLikeFeed) {
        return [{ title: host, url: raw.startsWith('http') ? raw : `https://${raw}`, format: 'RSS 2.0' }];
      }
      return [
        { title: host, url: `https://${cleaned}/feed.xml`, format: 'RSS 2.0' },
        { title: host, url: `https://${cleaned}/atom.xml`, format: 'Atom 1.0' },
        { title: host, url: `https://${cleaned}/rss`, format: 'RSS 2.0' },
      ];
    },

    async resetFeedToken(id: number): Promise<TokenResetResponse> {
      assertAuthed();
      await delay(160);
      const token = `rill_${String(id).padStart(3, '0')}${Math.random().toString(36).slice(2, 6)}…${Math.random().toString(36).slice(2, 6)}`;
      tokenMap.set(id, token);
      return { token_masked: token };
    },



    async getSettings(): Promise<Settings> {
      assertAuthed();
      await delay(140);
      return clone(db.settings);
    },

    async updateSettings(patch: SettingsPatch): Promise<Settings> {
      assertAuthed();
      await delay(160);
      db.settings = deepMergeSettings(db.settings, patch);
      persistSettings();
      return clone(db.settings);
    },

    async saveAiKey(apiKey: string): Promise<Settings> {
      assertAuthed();
      await delay(220);
      if (!apiKey.trim()) throw new ApiError(400, 'api_key_required', '请输入接口密钥');
      db.settings.ai.api_key_set = true;
      persistSettings();
      return clone(db.settings);
    },

    async testAiConnection(): Promise<AiTestResponse> {
      assertAuthed();
      await delay(820);
      if (!db.settings.ai.enabled) throw new ApiError(409, 'ai_disabled', 'AI 能力未启用');
      return { ok: true, latency_ms: 680 };
    },

    async generateSummary(itemId: number): Promise<AiSummary> {
      assertAuthed();
      await delay(1100);
      if (!db.settings.ai.enabled) throw new ApiError(409, 'ai_disabled', 'AI 能力未启用');
      const item = db.items.find((entry) => entry.id === itemId);
      if (!item) throw new ApiError(404, 'item_not_found', '条目不存在');
      const summary: AiSummary = {
        model: db.settings.ai.model,
        generated_at: nowIso(),
        style: db.settings.ai.summary_style,
        points: [
          '先判断内容是否值得读，再决定是否打开原文。',
          '工具的职责是减少选择，而不是制造更多未读。',
          '摘要失败不会影响条目入库，可以手动重跑。',
        ],
      };
      item.summary = summary;
      persistAIResult(itemId);
      return clone(summary);
    },

    async translateItem(itemId: number): Promise<Translation> {
      assertAuthed();
      await delay(1000);
      const item = db.items.find((entry) => entry.id === itemId);
      if (!item) throw new ApiError(404, 'item_not_found', '条目不存在');
      const translation: Translation = {
        title: `${item.title}（译文）`,
        target_lang: db.settings.ai.target_lang,
        original_lang: 'en',
        model: db.settings.ai.model,
        generated_at: nowIso(),
        needs_retranslate: false,
        content_html: `<article>${item.content_html
          .replace(/<h2>([^<]*)<\/h2>/g, '<h2>$1（译文）</h2>')
          .replace(/<p>([^<]*)<\/p>/g, '<p>这是演示译文：$1</p>')}</article>`,
      };
      item.translation = translation;
      persistAIResult(itemId);
      return clone(translation);
    },

    async streamSummary(itemId: number, onEvent: (event: SummaryStreamEvent) => void): Promise<void> {
      assertAuthed();
      await delay(200);
      if (!db.settings.ai.enabled) throw new ApiError(409, 'ai_disabled', 'AI 能力未启用');
      const item = db.items.find((entry) => entry.id === itemId);
      if (!item) throw new ApiError(404, 'item_not_found', '条目不存在');
      const points = [
        '先判断内容是否值得读，再决定是否打开原文。',
        '工具的职责是减少选择，而不是制造更多未读。',
        '摘要失败不会影响条目入库，可以手动重跑。',
      ];
      const text = points.join('\n');
      for (let index = 0; index < text.length; index += 5) {
        onEvent({ type: 'delta', text: text.slice(index, index + 5) });
        await delay(45);
      }
      const summary: AiSummary = {
        model: db.settings.ai.model,
        generated_at: nowIso(),
        style: db.settings.ai.summary_style,
        points,
      };
      item.summary = summary;
      persistAIResult(itemId);
      onEvent({ type: 'done', summary: clone(summary) });
    },

    async streamTranslation(itemId: number, onEvent: (event: TranslationStreamEvent) => void): Promise<void> {
      assertAuthed();
      await delay(200);
      const item = db.items.find((entry) => entry.id === itemId);
      if (!item) throw new ApiError(404, 'item_not_found', '条目不存在');
      const segments = ['这是演示译文：第一段。', '这是演示译文：第二段。', '这是演示译文：第三段。'];
      const title = `${item.title}（译文）`;
      await delay(320);
      onEvent({ type: 'title', title });
      for (let index = 0; index < segments.length; index += 1) {
        await delay(320);
        onEvent({ type: 'segment', index, text: segments[index] });
        onEvent({ type: 'progress', done: index + 1, total: segments.length });
      }
      const translation: Translation = {
        title,
        target_lang: db.settings.ai.target_lang,
        original_lang: 'en',
        model: db.settings.ai.model,
        generated_at: nowIso(),
        needs_retranslate: false,
        content_html: `<article>${item.content_html
          .replace(/<h2>([^<]*)<\/h2>/g, '<h2>$1（译文）</h2>')
          .replace(/<p>([^<]*)<\/p>/g, '<p>这是演示译文：$1</p>')}</article>`,
      };
      item.translation = translation;
      persistAIResult(itemId);
      onEvent({ type: 'done', translation: clone(translation) });
    },

    async scanWeread(): Promise<WechatQrCode> {
      assertAuthed();
      await delay(420);
      if (!db.settings.wechat.enabled) throw new ApiError(409, 'wechat_disabled', '请先启用公众号采集');
      db.settings.wechat.weread_connected = true;
      db.settings.wechat.weread_can_renew = true;
      for (const source of db.sources) {
        if (source.kind !== 'wechat') continue;
        source.last_status = 'ok';
        source.last_error = null;
        source.note = '微信读书连接有效';
      }
      persistSettings();
      return { qrcode: '' };
    },

    async wereadScanStatus(): Promise<WeReadScanStatus> {
      assertAuthed();
      await delay(140);
      return {
        state: db.settings.wechat.weread_connected ? 'done' : 'waiting',
        message: db.settings.wechat.weread_connected ? '微信读书登录成功' : '等待微信扫码',
      };
    },

    async wechatResolveArticle(url: string): Promise<WechatAccount> {
      assertAuthed();
      await delay(300);
      if (!db.settings.wechat.enabled) throw new ApiError(409, 'wechat_disabled', '请先启用公众号采集');
      if (!/^https?:\/\/mp\.weixin\.qq\.com\/s/i.test(url)) {
        throw new ApiError(400, 'invalid_wechat_url', '请粘贴微信公众号文章链接');
      }
      return { fake_id: 'MzA5MTA0MDAwMTAz', name: '阮一峰的网络日志', avatar: '/icon.png' };
    },

    async createWechatSource(input: { account: string; folder_id?: number | null }): Promise<Source> {
      assertAuthed();
      await delay(280);
      if (!db.settings.wechat.enabled) throw new ApiError(409, 'wechat_disabled', '请先启用公众号采集');
      if (!db.settings.wechat.weread_connected) {
        throw new ApiError(401, 'wechat_auth_required', '请先在设置中连接微信读书扫码');
      }
      const account = mockWechatAccounts().find((entry) => entry.fake_id === input.account)
        ?? { fake_id: input.account, name: input.account };
      const feedURL = `wechat://${account.fake_id}`;
      if (db.sources.some((source) => source.feed_url === feedURL)) {
        throw new ApiError(409, 'source_exists', '该公众号已在订阅列表中');
      }
      const id = Math.max(0, ...db.sources.map((source) => source.id)) + 1;
      const source: Source = {
        id,
        kind: 'wechat',
        name: account.name,
        feed_url: feedURL,
        site_url: null,
        folder_id: input.folder_id ?? null,
        enabled: true,
        refresh_seconds: 1800,
        initial_limit: 20,
        backfill_mode: 'window',
        fallback_url: null,
        last_fetch_at: nowIso(),
        last_new_item_at: nowIso(),
        next_fetch_at: new Date(Date.now() + 30 * 60_000).toISOString(),
        last_status: 'ok',
        last_error: null,
        fail_count: 0,
        item_count: 0,
        unread_count: 0,
        retention_days: 0,
        retention_max_items: 0,
        icon_url: null,
        note: '微信读书连接有效',
      };
      db.sources.push(source);
      return publicSource(db, source);
    },

    async downloadBackup(): Promise<void> {
      assertAuthed();
      await delay(200);
      const blob = new Blob(['rill mock backup'], { type: 'application/octet-stream' });
      const url = URL.createObjectURL(blob);
      const anchor = document.createElement('a');
      anchor.href = url;
      anchor.download = 'rill-backup-mock.db';
      anchor.click();
      window.setTimeout(() => URL.revokeObjectURL(url), 1000);
    },

    async backupNow(): Promise<Settings['backup']> {
      assertAuthed();
      await delay(640);
      db.settings.backup.last_backup_at = nowIso();
      persistSettings();
      return { ...db.settings.backup };
    },

    async restoreBackup(_file?: File): Promise<void> {
      assertAuthed();
      await delay(520);
    },

    async getSourceLogs(id: number, limit = 50): Promise<FetchLog[]> {
      assertAuthed();
      await delay(120);
      const source = db.sources.find((entry) => entry.id === id);
      if (!source) throw new ApiError(404, 'source_not_found', '订阅源不存在');
      const logs = db.fetchLogs[id]?.length ? db.fetchLogs[id] : makeLogs(source);
      return clone(logs.slice(0, limit));
    },

    // ===== REQ-036/037：推送渠道 =====

    async getChannels(): Promise<PushChannel[]> {
      assertAuthed();
      await delay(120);
      return clone([...db.channels].sort((a, b) => a.id - b.id));
    },

    async createChannel(input: PushChannelInput): Promise<{ id: number }> {
      assertAuthed();
      await delay(220);
      const name = input.name.trim();
      if (!name) throw new ApiError(400, 'name_required', '名称不能为空');
      const url = input.url?.trim() ?? '';
      if (!/^https?:\/\/.+/.test(url)) throw new ApiError(400, 'invalid_url', '请输入 http/https 开头的 webhook 地址');
      const now = nowIso();
      const hasSecret = Boolean(input.secret?.trim());
      const channel: PushChannel = {
        id: Math.max(0, ...db.channels.map((entry) => entry.id)) + 1,
        name,
        kind: input.kind,
        url_masked: maskUrl(url),
        has_secret: hasSecret,
        enabled: true,
        created_at: now,
        updated_at: now,
      };
      db.channels.push(channel);
      return { id: channel.id };
    },

    async updateChannel(id: number, input: PushChannelInput): Promise<void> {
      assertAuthed();
      await delay(200);
      const channel = db.channels.find((entry) => entry.id === id);
      if (!channel) throw new ApiError(404, 'channel_not_found', '推送渠道不存在');
      const name = input.name.trim();
      if (!name) throw new ApiError(400, 'name_required', '名称不能为空');
      channel.name = name;
      channel.kind = input.kind;
      if (input.enabled !== undefined) channel.enabled = input.enabled;
      if (input.secret !== undefined) channel.has_secret = Boolean(input.secret.trim());
      // url 留空/省略 = 不修改 URL（mock 内本就只保存打码形式）。
      const url = input.url?.trim() ?? '';
      if (url) {
        if (!/^https?:\/\/.+/.test(url)) throw new ApiError(400, 'invalid_url', '请输入 http/https 开头的 webhook 地址');
        channel.url_masked = maskUrl(url);
      }
      channel.updated_at = nowIso();
    },

    async deleteChannel(id: number): Promise<void> {
      assertAuthed();
      await delay(180);
      if (!db.channels.some((entry) => entry.id === id)) throw new ApiError(404, 'channel_not_found', '推送渠道不存在');
      db.channels = db.channels.filter((entry) => entry.id !== id);
      for (const rule of db.digestRules) {
        rule.channel_ids = rule.channel_ids.filter((channelId) => channelId !== id);
      }
    },

    async testChannel(id: number): Promise<{ ok: true }> {
      assertAuthed();
      await delay(560);
      const channel = db.channels.find((entry) => entry.id === id);
      if (!channel) throw new ApiError(404, 'channel_not_found', '推送渠道不存在');
      // 演示约定：打码地址含 "invalid" 的假渠道模拟推送失败，便于验证失败提示。
      if (channel.url_masked.includes('invalid')) {
        throw new ApiError(502, 'push_failed', 'webhook 返回 404，请检查地址');
      }
      return { ok: true };
    },

    // ===== REQ-036：报告规则与报告 =====

    async getDigestRules(): Promise<DigestRule[]> {
      assertAuthed();
      await delay(120);
      return clone([...db.digestRules].sort((a, b) => a.id - b.id));
    },

    async createDigestRule(input: DigestRuleInput): Promise<{ id: number }> {
      assertAuthed();
      await delay(240);
      validateDigestInput(input);
      const now = nowIso();
      const rule: DigestRule = {
        id: Math.max(0, ...db.digestRules.map((entry) => entry.id)) + 1,
        name: input.name.trim(),
        scope: [...input.scope],
        period: input.period,
        run_at_local: input.run_at_local,
        weekday: input.period === 'weekly' ? input.weekday : undefined,
        only_unread: input.only_unread,
        push_when_empty: input.push_when_empty,
        enabled: input.enabled ?? true,
        channel_ids: [...(input.channel_ids ?? [])],
        last_run_at: null,
        next_run_at: (input.enabled ?? true) ? nextRunAt(input) : null,
        created_at: now,
        updated_at: now,
      };
      db.digestRules.push(rule);
      return { id: rule.id };
    },

    async updateDigestRule(id: number, input: DigestRuleInput): Promise<void> {
      assertAuthed();
      await delay(220);
      const rule = db.digestRules.find((entry) => entry.id === id);
      if (!rule) throw new ApiError(404, 'rule_not_found', '报告规则不存在');
      validateDigestInput(input);
      rule.name = input.name.trim();
      rule.scope = [...input.scope];
      rule.period = input.period;
      rule.run_at_local = input.run_at_local;
      rule.weekday = input.period === 'weekly' ? input.weekday : undefined;
      rule.only_unread = input.only_unread;
      rule.push_when_empty = input.push_when_empty;
      rule.enabled = input.enabled ?? rule.enabled;
      rule.channel_ids = [...(input.channel_ids ?? [])];
      rule.updated_at = nowIso();
      rule.next_run_at = rule.enabled ? nextRunAt(rule) : null;
    },

    async deleteDigestRule(id: number): Promise<void> {
      assertAuthed();
      await delay(180);
      if (!db.digestRules.some((entry) => entry.id === id)) throw new ApiError(404, 'rule_not_found', '报告规则不存在');
      db.digestRules = db.digestRules.filter((entry) => entry.id !== id);
      db.digestReports = db.digestReports.filter((entry) => entry.rule_id !== id);
    },

    async runDigestRule(id: number): Promise<DigestReport> {
      assertAuthed();
      await delay(720);
      const rule = db.digestRules.find((entry) => entry.id === id);
      if (!rule) throw new ApiError(404, 'rule_not_found', '报告规则不存在');
      const periodEnd = new Date();
      const periodStart = new Date(periodEnd.getTime() - (rule.period === 'weekly' ? 7 : 1) * 24 * 60 * 60_000);
      const items = db.items
        .filter((item) => !item.hidden_at)
        .filter((item) => inScope(db, rule.scope, item))
        .filter((item) => {
          const at = item.published_at ?? item.fetched_at;
          return at >= periodStart.toISOString() && at <= periodEnd.toISOString();
        })
        .filter((item) => (rule.only_unread ? !item.read_at : true))
        .sort((a, b) => (b.published_at ?? b.fetched_at).localeCompare(a.published_at ?? a.fetched_at));
      const report: DigestReport = {
        id: Math.max(0, ...db.digestReports.map((entry) => entry.id)) + 1,
        rule_id: rule.id,
        rule_name: rule.name,
        period_start: periodStart.toISOString(),
        period_end: periodEnd.toISOString(),
        item_count: items.length,
        status: items.length === 0 ? 'empty' : 'ok',
        content_md: digestReportContent(db, rule, items, periodStart, periodEnd),
        model: db.settings.ai.enabled ? db.settings.ai.model : undefined,
        push_status: 'none',
        push_error: null,
        pushed_at: null,
        created_at: nowIso(),
      };
      db.digestReports.unshift(report);
      rule.last_run_at = report.created_at;
      rule.next_run_at = rule.enabled ? nextRunAt(rule) : null;
      return clone(report);
    },

    async getDigestReports(ruleId?: number, limit = 20): Promise<DigestReport[]> {
      assertAuthed();
      await delay(120);
      return clone(db.digestReports
        .filter((entry) => (ruleId ? entry.rule_id === ruleId : true))
        .sort((a, b) => b.created_at.localeCompare(a.created_at))
        .slice(0, Math.min(Math.max(limit, 1), 100)));
    },

    async getDigestReport(id: number): Promise<DigestReport> {
      assertAuthed();
      await delay(120);
      const report = db.digestReports.find((entry) => entry.id === id);
      if (!report) throw new ApiError(404, 'report_not_found', '报告不存在');
      return clone(report);
    },

    async pushDigestReport(id: number): Promise<{ push_status: DigestPushStatus }> {
      assertAuthed();
      await delay(520);
      const report = db.digestReports.find((entry) => entry.id === id);
      if (!report) throw new ApiError(404, 'report_not_found', '报告不存在');
      const rule = db.digestRules.find((entry) => entry.id === report.rule_id);
      const channels = db.channels.filter((entry) => entry.enabled && (rule ? rule.channel_ids.includes(entry.id) : true));
      if (channels.length === 0) throw new ApiError(400, 'channel_missing', '没有可用的推送渠道，请先在推送渠道里添加');
      // 演示约定：打码地址含 "invalid" 时模拟推送失败。
      const failed = channels.filter((entry) => entry.url_masked.includes('invalid'));
      if (failed.length === channels.length) {
        report.push_status = 'failed';
        report.push_error = 'webhook 返回 404，请检查渠道地址';
        throw new ApiError(502, 'push_failed', report.push_error);
      }
      report.push_status = failed.length > 0 ? 'partial' : 'ok';
      report.push_error = failed.length > 0 ? `${failed.length} 个渠道推送失败` : null;
      report.pushed_at = nowIso();
      return { push_status: report.push_status };
    },

    // ===== REQ-037：过滤规则 =====

    async getFilterRules(): Promise<FilterRule[]> {
      assertAuthed();
      await delay(120);
      return clone([...db.filterRules].sort((a, b) => a.id - b.id));
    },

    async createFilterRule(input: FilterRuleInput): Promise<{ id: number }> {
      assertAuthed();
      await delay(220);
      const pattern = input.pattern.trim();
      if (!pattern) throw new ApiError(400, 'pattern_required', '匹配模式不能为空');
      if (!input.name.trim()) throw new ApiError(400, 'name_required', '名称不能为空');
      if (input.is_regex) {
        try {
          new RegExp(pattern);
        } catch (error) {
          throw new ApiError(400, 'invalid_regex', `正则表达式无效：${error instanceof Error ? error.message : '语法错误'}`);
        }
      }
      const now = nowIso();
      const rule: FilterRule = {
        id: Math.max(0, ...db.filterRules.map((entry) => entry.id)) + 1,
        name: input.name.trim(),
        pattern,
        is_regex: input.is_regex,
        match_field: input.match_field,
        action: input.action,
        scope: [...input.scope],
        enabled: input.enabled ?? true,
        hit_count: 0,
        created_at: now,
        updated_at: now,
      };
      db.filterRules.push(rule);
      return { id: rule.id };
    },

    async updateFilterRule(id: number, input: FilterRuleInput): Promise<void> {
      assertAuthed();
      await delay(200);
      const rule = db.filterRules.find((entry) => entry.id === id);
      if (!rule) throw new ApiError(404, 'rule_not_found', '过滤规则不存在');
      const pattern = input.pattern.trim();
      if (!pattern) throw new ApiError(400, 'pattern_required', '匹配模式不能为空');
      if (!input.name.trim()) throw new ApiError(400, 'name_required', '名称不能为空');
      if (input.is_regex) {
        try {
          new RegExp(pattern);
        } catch (error) {
          throw new ApiError(400, 'invalid_regex', `正则表达式无效：${error instanceof Error ? error.message : '语法错误'}`);
        }
      }
      rule.name = input.name.trim();
      rule.pattern = pattern;
      rule.is_regex = input.is_regex;
      rule.match_field = input.match_field;
      rule.action = input.action;
      rule.scope = [...input.scope];
      rule.enabled = input.enabled ?? rule.enabled;
      rule.updated_at = nowIso();
    },

    async deleteFilterRule(id: number): Promise<void> {
      assertAuthed();
      await delay(180);
      if (!db.filterRules.some((entry) => entry.id === id)) throw new ApiError(404, 'rule_not_found', '过滤规则不存在');
      db.filterRules = db.filterRules.filter((entry) => entry.id !== id);
    },

    async applyFilterRule(id: number): Promise<{ affected: number }> {
      assertAuthed();
      await delay(480);
      const rule = db.filterRules.find((entry) => entry.id === id);
      if (!rule) throw new ApiError(404, 'rule_not_found', '过滤规则不存在');
      if (!rule.enabled) throw new ApiError(409, 'rule_disabled', '规则已停用，请先启用');
      let regex: RegExp | null = null;
      if (rule.is_regex) {
        try {
          regex = new RegExp(rule.pattern);
        } catch (error) {
          throw new ApiError(400, 'invalid_regex', `正则表达式无效：${error instanceof Error ? error.message : '语法错误'}`);
        }
      }
      const needle = rule.pattern.toLowerCase();
      const matches = (text: string): boolean => (regex ? regex.test(text) : text.toLowerCase().includes(needle));
      let affected = 0;
      for (const item of db.items) {
        if (item.hidden_at) continue;
        if (!inScope(db, rule.scope, item)) continue;
        const targets = rule.match_field === 'title' ? [item.title]
          : rule.match_field === 'content' ? [item.content_text]
            : [item.title, item.content_text];
        if (!targets.some(matches)) continue;
        if (rule.action === 'hide') {
          item.hidden_at = nowIso();
        } else {
          if (item.read_at) continue;
          item.read_at = nowIso();
        }
        affected += 1;
      }
      rule.hit_count += affected;
      rule.updated_at = nowIso();
      return { affected };
    },

    async getMcpTokens(): Promise<MCPToken[]> {
      assertAuthed();
      await delay(120);
      return clone(mcpTokens);
    },

    async createMcpToken(name: string): Promise<MCPTokenCreated> {
      assertAuthed();
      await delay(180);
      const trimmed = name.trim();
      if (!trimmed) throw new ApiError(400, 'invalid_mcp_token', '令牌名称不能为空');
      const id = Math.max(0, ...mcpTokens.map((token) => token.id)) + 1;
      const suffix = Math.random().toString(36).slice(2, 10);
      const token: MCPToken = {
        id,
        name: trimmed,
        masked: `rill_mcp_${suffix.slice(0, 4)}…${suffix.slice(-4)}`,
        enabled: true,
        created_at: nowIso(),
        last_used_at: null,
      };
      mcpTokens = [...mcpTokens, token];
      return { ...token, token: `rill_mcp_mock_${suffix}${Math.random().toString(36).slice(2, 18)}` };
    },

    async updateMcpToken(id: number, patch: { name?: string; enabled?: boolean }): Promise<void> {
      assertAuthed();
      await delay(120);
      const token = mcpTokens.find((entry) => entry.id === id);
      if (!token) throw new ApiError(404, 'mcp_token_not_found', 'MCP 令牌不存在');
      if (patch.name !== undefined) {
        const trimmed = patch.name.trim();
        if (!trimmed) throw new ApiError(400, 'invalid_mcp_token', '令牌名称不能为空');
        token.name = trimmed;
      }
      if (patch.enabled !== undefined) token.enabled = patch.enabled;
    },

    async deleteMcpToken(id: number): Promise<void> {
      assertAuthed();
      await delay(120);
      if (!mcpTokens.some((entry) => entry.id === id)) {
        throw new ApiError(404, 'mcp_token_not_found', 'MCP 令牌不存在');
      }
      mcpTokens = mcpTokens.filter((entry) => entry.id !== id);
    },
  };
};

export const mockApi = createMockApi();
