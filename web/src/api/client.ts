import type {
  AiSummary,
  AiTestResponse,
  ApiClient,
  ApiErrorBody,
  AuthConfig,
  BatchStateResponse,
  CreatorSearchResponse,
  DigestPushStatus,
  DigestReport,
  DigestRule,
  DigestRuleInput,
  EmailTestInput,
  EmailTestResponse,
  FeedCandidate,
  FetchLog,
  FilterRule,
  FilterRuleInput,
  Folder,
  ItemDetail,
  ItemNote,
  ItemPage,
  ItemQuery,
  ItemSummary,
  MCPToken,
  MCPTokenCreated,
  OpmlImportResult,
  PublicHealth,
  PushChannel,
  PushChannelInput,
  ReaderStats,
  ResolveCandidate,
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
import { ApiError } from '@/api/types';
import { mockApi } from '@/api/mock/mockApi';

const API_BASE = import.meta.env.VITE_API_BASE || '/api/v1';
const USE_MOCK = import.meta.env.VITE_USE_MOCK !== 'false';
let csrfToken = '';

export const setCsrfToken = (token: string) => {
  csrfToken = token;
};

const request = async <T>(path: string, init: RequestInit = {}): Promise<T> => {
  const method = (init.method ?? 'GET').toUpperCase();
  const headers = new Headers(init.headers);
  if (init.body && !(init.body instanceof FormData) && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json');
  }
  if (method !== 'GET' && csrfToken) headers.set('X-CSRF-Token', csrfToken);
  const response = await fetch(`${API_BASE}${path}`, {
    ...init,
    headers,
    credentials: 'include',
  });
  if (!response.ok) {
    if (response.status === 401 && path !== '/auth/logout') {
      window.dispatchEvent(new CustomEvent('rill:unauthorized'));
    }
    let body: ApiErrorBody | null = null;
    try {
      body = (await response.json()) as ApiErrorBody;
    } catch {
      body = null;
    }
    throw new ApiError(
      response.status,
      body?.error.code ?? `http_${response.status}`,
      body?.error.message ?? `请求失败（${response.status}）`,
      body?.error.details,
    );
  }
  if (response.status === 204) return undefined as T;
  return (await response.json()) as T;
};

// SSE reader for AI streaming endpoints. Frames use `data: {json}` and are
// separated by blank lines.
const streamRequest = async <T>(path: string, onEvent: (event: T) => void): Promise<void> => {
  const headers = new Headers({ Accept: 'text/event-stream' });
  if (csrfToken) headers.set('X-CSRF-Token', csrfToken);
  const response = await fetch(`${API_BASE}${path}`, { method: 'POST', headers, credentials: 'include' });
  if (!response.ok) {
    if (response.status === 401) window.dispatchEvent(new CustomEvent('rill:unauthorized'));
    let body: ApiErrorBody | null = null;
    try {
      body = (await response.json()) as ApiErrorBody;
    } catch {
      body = null;
    }
    throw new ApiError(
      response.status,
      body?.error.code ?? `http_${response.status}`,
      body?.error.message ?? `请求失败（${response.status}）`,
      body?.error.details,
    );
  }
  const reader = response.body?.getReader();
  if (!reader) throw new ApiError(500, 'stream_unavailable', '当前浏览器不支持流式响应');
  const decoder = new TextDecoder();
  let buffer = '';
  for (;;) {
    const { value, done } = await reader.read();
    if (done) break;
    buffer += decoder.decode(value, { stream: true });
    let boundary = buffer.indexOf('\n\n');
    while (boundary >= 0) {
      const frame = buffer.slice(0, boundary);
      buffer = buffer.slice(boundary + 2);
      const data = frame
        .split('\n')
        .filter((line) => line.startsWith('data:'))
        .map((line) => line.slice(5).trim())
        .join('\n');
      if (data) {
        try {
          onEvent(JSON.parse(data) as T);
        } catch {
          // Ignore malformed frames; the next event still renders.
        }
      }
      boundary = buffer.indexOf('\n\n');
    }
  }
};

const queryString = (query: ItemQuery): string => {
  const params = new URLSearchParams();
  if (query.folder_id) params.set('folder_id', String(query.folder_id));
  if (query.source_id) params.set('source_id', String(query.source_id));
  if (query.unread) params.set('unread', '1');
  if (query.read) params.set('read', '1');
  if (query.starred) params.set('starred', '1');
  if (query.hidden) params.set('hidden', '1');
  if (query.q) params.set('q', query.q);
  if (query.cursor) params.set('cursor', query.cursor);
  if (query.sort) params.set('sort', query.sort);
  if (query.limit) params.set('limit', String(query.limit));
  const value = params.toString();
  return value ? `?${value}` : '';
};

const httpClient: ApiClient = {
  getPublicHealth: () => fetch('/healthz', { credentials: 'include' }).then((response) => response.json() as Promise<PublicHealth>),
  getAuthConfig: async (): Promise<AuthConfig> => {
    const response = await fetch(`${API_BASE}/auth/config`, { credentials: 'include' });
    if (!response.ok) throw new ApiError(response.status, `http_${response.status}`, '读取登录配置失败');
    return (await response.json()) as AuthConfig;
  },
  getSession: async (): Promise<SessionResponse> => {
    try {
      const response = await fetch(`${API_BASE}/auth/csrf`, { credentials: 'include' });
      if (response.status === 401) return { authenticated: false, csrf_token: '' };
      const body = (await response.json()) as { csrf_token: string };
      setCsrfToken(body.csrf_token);
      return { authenticated: true, csrf_token: body.csrf_token };
    } catch {
      return { authenticated: false, csrf_token: '' };
    }
  },
  logout: async () => {
    await request<void>('/auth/logout', { method: 'POST' });
    setCsrfToken('');
  },
  getFolders: () => request<Folder[]>('/folders'),
  createFolder: (name, parentId = null) => request<Folder>('/folders', {
    method: 'POST',
    body: JSON.stringify({ name, parent_id: parentId }),
  }),
  updateFolder: (id, name, parentId = null, sort) => request<Folder>(`/folders/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(sort == null ? { name, parent_id: parentId } : { name, parent_id: parentId, sort }),
  }),
  deleteFolder: (id) => request<void>(`/folders/${id}`, { method: 'DELETE' }),
  exportOpml: async (folderId) => {
    const suffix = folderId ? `?folder_id=${folderId}` : '';
    const response = await fetch(`${API_BASE}/opml/export${suffix}`, { credentials: 'include' });
    if (!response.ok) throw new ApiError(response.status, `http_${response.status}`, '导出 OPML 失败');
    return response.text();
  },
  importOpml: (file, folderId) => {
    const form = new FormData();
    form.append('file', file, file.name);
    const suffix = folderId ? `?folder_id=${folderId}` : '';
    return request<OpmlImportResult>(`/opml/import${suffix}`, { method: 'POST', body: form });
  },
  getSources: () => request<Source[]>('/sources'),
  getItems: (query = {}) => request<ItemPage>(`/items${queryString(query)}`),
  getReaderStats: () => request<ReaderStats>('/items/stats'),
  getItem: (id) => request<ItemDetail>(`/items/${id}`),
  getItemMarkdown: async (id): Promise<string> => {
    const response = await fetch(`${API_BASE}/items/${id}/markdown`, { credentials: 'include' });
    if (!response.ok) {
      if (response.status === 401) window.dispatchEvent(new CustomEvent('rill:unauthorized'));
      let body: ApiErrorBody | null = null;
      try { body = (await response.json()) as ApiErrorBody; } catch { body = null; }
      throw new ApiError(
        response.status,
        body?.error.code ?? `http_${response.status}`,
        body?.error.message ?? `请求失败（${response.status}）`,
      );
    }
    return response.text();
  },
  fetchItemFullText: (id) => request<ItemDetail>(`/items/${id}/fulltext`, { method: 'POST' }),
  patchItemState: (id, patch) => request<ItemSummary>(`/items/${id}/state`, { method: 'PATCH', body: JSON.stringify(patch) }),
  setItemEmailImages: (id, allowed) => request<{ email_images_allowed: boolean }>(`/items/${id}/email-images`, { method: 'PATCH', body: JSON.stringify({ allowed }) }),
  batchItemState: (ids, patch) => request<BatchStateResponse>('/items/state', { method: 'PATCH', body: JSON.stringify({ ids, ...patch }) }),
  getItemNotes: (id) => request<ItemNote[]>(`/items/${id}/notes`),
  createItemNote: (id, input) => request<ItemNote>(`/items/${id}/notes`, { method: 'POST', body: JSON.stringify(input) }),
  updateItemNote: (id, noteId, input) => request<ItemNote>(`/items/${id}/notes/${noteId}`, { method: 'PATCH', body: JSON.stringify(input) }),
  deleteItemNote: (id, noteId) => request<void>(`/items/${id}/notes/${noteId}`, { method: 'DELETE' }),
  markAllRead: (target) => request<BatchStateResponse>('/items/mark-all-read', { method: 'POST', body: JSON.stringify(target) }),
  getChannels: () => request<PushChannel[]>('/channels'),
  createChannel: (input: PushChannelInput) => request<{ id: number }>('/channels', { method: 'POST', body: JSON.stringify(input) }),
  updateChannel: (id, input: PushChannelInput) => request<void>(`/channels/${id}`, { method: 'PUT', body: JSON.stringify(input) }),
  deleteChannel: (id) => request<void>(`/channels/${id}`, { method: 'DELETE' }),
  testChannel: (id) => request<{ ok: true }>(`/channels/${id}/test`, { method: 'POST' }),
  getDigestRules: () => request<DigestRule[]>('/digest/rules'),
  createDigestRule: (input: DigestRuleInput) => request<{ id: number }>('/digest/rules', { method: 'POST', body: JSON.stringify(input) }),
  updateDigestRule: (id, input: DigestRuleInput) => request<void>(`/digest/rules/${id}`, { method: 'PUT', body: JSON.stringify(input) }),
  deleteDigestRule: (id) => request<void>(`/digest/rules/${id}`, { method: 'DELETE' }),
  runDigestRule: (id) => request<DigestReport>(`/digest/rules/${id}/run`, { method: 'POST' }),
  getDigestReports: (ruleId, limit = 20) => {
    const params = new URLSearchParams();
    if (ruleId) params.set('rule_id', String(ruleId));
    params.set('limit', String(limit));
    return request<DigestReport[]>(`/digest/reports?${params.toString()}`);
  },
  getDigestReport: (id) => request<DigestReport>(`/digest/reports/${id}`),
  pushDigestReport: (id) => request<{ push_status: DigestPushStatus }>(`/digest/reports/${id}/push`, { method: 'POST' }),
  getFilterRules: () => request<FilterRule[]>('/filters/rules'),
  createFilterRule: (input: FilterRuleInput) => request<{ id: number }>('/filters/rules', { method: 'POST', body: JSON.stringify(input) }),
  updateFilterRule: (id, input: FilterRuleInput) => request<void>(`/filters/rules/${id}`, { method: 'PUT', body: JSON.stringify(input) }),
  deleteFilterRule: (id) => request<void>(`/filters/rules/${id}`, { method: 'DELETE' }),
  applyFilterRule: (id) => request<{ affected: number }>(`/filters/rules/${id}/apply`, { method: 'POST' }),
  getMcpTokens: () => request<MCPToken[]>('/mcp/tokens'),
  createMcpToken: (name) => request<MCPTokenCreated>('/mcp/tokens', { method: 'POST', body: JSON.stringify({ name }) }),
  updateMcpToken: (id, patch) => request<void>(`/mcp/tokens/${id}`, { method: 'PATCH', body: JSON.stringify(patch) }),
  deleteMcpToken: (id) => request<void>(`/mcp/tokens/${id}`, { method: 'DELETE' }),
  getSourceHealth: (id) => request<SourceHealth>(`/sources/${id}`),
  refreshSource: (id) => request<Source>(`/sources/${id}/fetch`, { method: 'POST' }),
  switchSourceFallback: (id) => request<Source>(`/sources/${id}/fallback`, { method: 'POST' }),
  updateSource: (id, patch: SourcePatch) => request<Source>(`/sources/${id}`, { method: 'PATCH', body: JSON.stringify(patch) }),
  createSource: (input: SourceInput) => request<Source>('/sources', { method: 'POST', body: JSON.stringify(input) }),
  deleteSource: (id) => request<void>(`/sources/${id}`, { method: 'DELETE' }),
  discoverFeed: (url) => request<FeedCandidate[]>('/sources/discover', { method: 'POST', body: JSON.stringify({ url }) }),
  resolveSource: async (input) => {
    const result = await request<{ candidates: ResolveCandidate[] }>('/sources/resolve', {
      method: 'POST',
      body: JSON.stringify({ input }),
    });
    return result.candidates;
  },
  searchCreators: (query, platforms) => request<CreatorSearchResponse>('/sources/search', {
    method: 'POST',
    body: JSON.stringify({ query, platforms }),
  }),
  createSources: (inputs) => request<SourceBatchCreateResponse>('/sources/batch', {
    method: 'POST',
    body: JSON.stringify({ sources: inputs }),
  }),
  batchUpdateSources: (input: SourceBatchUpdateInput) => request<SourceBatchUpdateResponse>('/sources/batch-update', {
    method: 'POST',
    body: JSON.stringify(input),
  }),
  batchDeleteSources: (ids) => request<SourceBatchDeleteResponse>('/sources/batch-delete', {
    method: 'POST',
    body: JSON.stringify({ ids }),
  }),
  resetFeedToken: (id) => request<TokenResetResponse>(`/sources/${id}/token/reset`, { method: 'POST' }),
  getSettings: () => request<Settings>('/settings'),
  updateSettings: (patch: SettingsPatch) => request<Settings>('/settings', { method: 'PATCH', body: JSON.stringify(patch) }),
  saveAiKey: (apiKey) => request<Settings>('/settings', { method: 'PATCH', body: JSON.stringify({ ai: { api_key: apiKey } }) }),
  testAiConnection: () => request<AiTestResponse>('/settings/ai/test', { method: 'POST' }),
  // REQ-041：授权码只写不读；按账号保存，null = 清除。
  saveEmailPassword: (accountId: string, password: string | null) => request<Settings>('/settings', {
    method: 'PATCH',
    body: JSON.stringify({ email: { account_id: accountId, password } }),
  }),
  testEmailConnection: (input: EmailTestInput) => request<EmailTestResponse>('/settings/email/test', {
    method: 'POST',
    body: JSON.stringify(input),
    // 后端 12s 内必然返回；15s 兜底中止，避免「正在测试」卡死。
    signal: AbortSignal.timeout(15000),
  }),
  generateSummary: (itemId) => request<AiSummary>(`/items/${itemId}/summary`, { method: 'POST' }),
  translateItem: (itemId) => request<Translation>(`/items/${itemId}/translation`, { method: 'POST' }),
  savePodcastProgress: (itemId, positionSeconds, durationSeconds) => request<void>(`/items/${itemId}/podcast-progress`, {
    method: 'PUT',
    body: JSON.stringify({ position_seconds: positionSeconds, duration_seconds: durationSeconds }),
  }),
  streamSummary: (itemId, onEvent) => streamRequest<SummaryStreamEvent>(`/items/${itemId}/summary/stream`, onEvent),
  streamTranslation: (itemId, onEvent, force = false) => streamRequest<TranslationStreamEvent>(
    `/items/${itemId}/translation/stream${force ? '?force=1' : ''}`,
    onEvent,
  ),
  scanWeread: () => request<WechatQrCode>('/wechat/weread/scan', { method: 'POST' }),
  wereadScanStatus: () => request<WeReadScanStatus>('/wechat/weread/scan/status'),
  wechatResolveArticle: async (url) => {
    const response = await request<{ account: WechatAccount }>('/wechat/resolve', {
      method: 'POST',
      body: JSON.stringify({ url }),
    });
    return response.account;
  },
  createWechatSource: (input) => request<Source>('/wechat/sources', { method: 'POST', body: JSON.stringify(input) }),
  backupNow: () => request<Settings['backup']>('/backup', { method: 'POST' }),
  downloadBackup: async (): Promise<void> => {
    const response = await fetch(`${API_BASE}/backup/download`, { credentials: 'include' });
    if (!response.ok) {
      let body: ApiErrorBody | null = null;
      try { body = await response.json() as ApiErrorBody; } catch { body = null; }
      throw new ApiError(
        response.status,
        body?.error.code ?? `http_${response.status}`,
        body?.error.message ?? `请求失败（${response.status}）`,
      );
    }
    const blob = await response.blob();
    const disposition = response.headers.get('Content-Disposition') ?? '';
    const match = disposition.match(/filename="?([^";]+)"?/);
    const name = match?.[1] ?? 'rill-backup.db';
    const url = URL.createObjectURL(blob);
    const anchor = document.createElement('a');
    anchor.href = url;
    anchor.download = name;
    anchor.click();
    window.setTimeout(() => URL.revokeObjectURL(url), 1000);
  },
  restoreBackup: (file) => {
    if (!file) return request<void>('/backup/restore', { method: 'POST' });
    const form = new FormData();
    form.append('file', file, file.name);
    return request<void>('/backup/restore', { method: 'POST', body: form });
  },
  getSourceLogs: (id, limit = 50) => request<FetchLog[]>(`/sources/${id}/logs?limit=${limit}`),
};

export const api: ApiClient = USE_MOCK ? mockApi : httpClient;
export const isMockMode = USE_MOCK;
