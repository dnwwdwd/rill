export type SourceKind = 'rss' | 'x' | 'wechat';
export type SourceStatus = 'ok' | 'warn' | 'error' | 'auth' | 'paused';
export type FetchResult = 'ok' | 'not_modified' | 'error';
export type ThemePreference = 'light' | 'dark' | 'system';
export type AccentColor = 'green' | 'blue' | 'cyan' | 'teal' | 'indigo' | 'purple' | 'pink' | 'rose' | 'red' | 'orange' | 'amber' | 'slate';
export type SummaryStyle = 'bullets' | 'one';

/** REQ-042：空字符串表示使用系统默认任务指令。 */
export interface AIPromptSettings {
  summary_bullets: string;
  summary_one: string;
  digest_batch: string;
  digest_merge: string;
}
export type PushChannelKind = 'feishu' | 'generic_webhook';
export type DigestPeriod = 'daily' | 'weekly';
export type DigestReportStatus = 'ok' | 'empty' | 'partial' | 'error';
export type DigestPushStatus = 'none' | 'ok' | 'failed' | 'partial';
export type FilterMatchField = 'title' | 'content' | 'any';
export type FilterAction = 'mark_read' | 'hide';

// 规则范围：数字 = 源 ID；"folder:<id>" = 整个分组；空数组 = 全部源。
export type RuleScope = Array<number | string>;

export interface PushChannel {
  id: number;
  name: string;
  kind: PushChannelKind;
  url_masked: string;
  has_secret: boolean;
  enabled: boolean;
  created_at: string;
  updated_at: string;
}

export interface PushChannelInput {
  name: string;
  kind: PushChannelKind;
  /** 更新时留空/省略 = 不修改 URL。 */
  url?: string;
  /** 飞书签名校验密钥（选填）；更新时省略 = 不变，空串 = 清除。自定义 Webhook 不需要。 */
  secret?: string;
  enabled?: boolean;
}

export interface DigestRule {
  id: number;
  name: string;
  scope: RuleScope;
  period: DigestPeriod;
  run_at_local: string;
  /** 1-7 = 周一~周日，仅 weekly 使用。 */
  weekday?: number;
  only_unread: boolean;
  push_when_empty: boolean;
  enabled: boolean;
  channel_ids: number[];
  last_run_at?: string | null;
  next_run_at?: string | null;
  created_at: string;
  updated_at: string;
}

export interface DigestRuleInput {
  name: string;
  scope: RuleScope;
  period: DigestPeriod;
  run_at_local: string;
  weekday?: number;
  only_unread: boolean;
  push_when_empty: boolean;
  enabled?: boolean;
  channel_ids?: number[];
}

export interface DigestReport {
  id: number;
  rule_id: number;
  rule_name: string;
  period_start: string;
  period_end: string;
  item_count: number;
  status: DigestReportStatus;
  content_md: string;
  model?: string;
  push_status: DigestPushStatus;
  push_error?: string | null;
  pushed_at?: string | null;
  created_at: string;
}

export interface FilterRule {
  id: number;
  name: string;
  pattern: string;
  is_regex: boolean;
  match_field: FilterMatchField;
  action: FilterAction;
  scope: RuleScope;
  enabled: boolean;
  hit_count: number;
  created_at: string;
  updated_at: string;
}

export interface FilterRuleInput {
  name: string;
  pattern: string;
  is_regex: boolean;
  match_field: FilterMatchField;
  action: FilterAction;
  scope: RuleScope;
  enabled?: boolean;
}

export interface Folder {
  id: number;
  name: string;
  sort: number;
  parent_id?: number | null;
  source_count: number;
}

export interface SavedView {
  id: string;
  name: string;
  type: 'inbox' | 'unread' | 'read' | 'starred' | 'later' | 'source' | 'folder';
  source_id?: number | null;
  folder_id?: number | null;
  query?: string;
  read_filter?: 'all' | 'unread' | 'read';
}

export interface OpmlImportResult {
  imported: number;
  skipped: Array<{ url: string; reason: string }>;
}

export interface Source {
  id: number;
  kind: SourceKind;
  name: string;
  feed_url: string;
  site_url: string | null;
  folder_id: number | null;
  enabled: boolean;
  refresh_seconds: number;
  initial_limit: number;
  fallback_url: string | null;
  last_fetch_at: string | null;
  last_new_item_at: string | null;
  next_fetch_at: string;
  last_status: 'ok' | 'not_modified' | 'error' | null;
  last_error: string | null;
  fail_count: number;
  item_count: number;
  unread_count: number;
  retention_days?: number;
  retention_max_items?: number;
  icon_url: string | null;
  note?: string | null;
  backfill_mode: 'window' | 'full';
  full_text?: boolean;
  watch_enabled?: boolean;
  watch_mode?: WatchMode;
  watch_ai?: boolean;
  watch_channel_ids?: number[];
}

export type WatchMode = 'each' | 'batch';

export interface SourceSummary {
  id: number;
  name: string;
  kind: SourceKind;
}

export interface SourceView extends Source {
  folder_name: string;
  status: SourceStatus;
}

export interface ItemSummary {
  id: number;
  source_id: number;
  source: SourceSummary;
  title: string;
  url: string | null;
  author: string | null;
  published_at: string | null;
  published_estimated: boolean;
  excerpt: string;
  cover_url: string | null;
  read_at: string | null;
  starred_at: string | null;
  later_at: string | null;
  fetched_at: string;
}

export interface ItemNote {
  id: number;
  item_id: number;
  quote: string;
  note: string;
  color: 'yellow' | 'green' | 'blue' | 'pink';
  created_at: string;
  updated_at: string;
}

export interface ItemNoteInput {
  quote: string;
  note: string;
  color?: ItemNote['color'];
}

export interface AiSummary {
  model: string;
  generated_at: string;
  style: SummaryStyle;
  points: string[];
}

export interface Translation {
  title: string;
  target_lang: string;
  original_lang: string;
  model: string;
  generated_at: string;
  needs_retranslate: boolean;
  content_html: string;
}

export interface ItemDetail extends ItemSummary {
  original_title?: string;
  /** 邮件源为 imap://...，用于切换邮件正文布局与图片放行。 */
  source_feed_url?: string;
  sender_email?: string;
  /** 该邮件条目已选择「显示图片」，之后打开不再拦截。 */
  email_images_allowed?: boolean;
  content_html: string;
  content_text: string;
  audio_url?: string;
  audio_type?: string;
  podcast_position_seconds?: number;
  podcast_duration_seconds?: number;
  summary?: AiSummary;
  translation?: Translation;
  notes?: ItemNote[];
}

export interface ItemPage {
  items: ItemSummary[];
  next_cursor: string | null;
}

export interface ReaderStats {
  total: number;
  unread: number;
  starred: number;
  read: number;
  /** 被过滤规则隐藏的条目数。 */
  hidden: number;
  later: number;
}

export interface ItemQuery {
  folder_id?: number;
  source_id?: number;
  unread?: boolean;
  read?: boolean;
  starred?: boolean;
  /** true = 只返回稍后读条目。 */
  later?: boolean;
  /** true = 只返回被隐藏条目（过滤规则 hide 的结果）。 */
  hidden?: boolean;
  q?: string;
  cursor?: string | null;
  sort?: 'published' | 'fetched';
  limit?: number;
}

export interface FetchLog {
  id: number;
  started_at: string;
  duration_ms: number | null;
  http_status: number | null;
  result: FetchResult;
  new_count: number;
  error: string | null;
}

export interface FeedPaths {
  rss: string;
  atom: string;
  json: string;
}

export interface SourceHealth {
  source: Source;
  logs: FetchLog[];
  trend: number[];
  feed_token_masked: string;
  feed_paths: FeedPaths;
}

export interface FeedCandidate {
  title: string;
  url: string;
  format: 'RSS 2.0' | 'Atom 1.0' | 'JSON Feed';
}

export interface ResolveCandidate {
  platform: 'apple' | 'xiaoyuzhou' | 'medium' | 'mastodon' | 'bestblogs' | string;
  kind: SourceKind;
  name: string;
  feed_url: string;
  site_url?: string | null;
  icon_url?: string | null;
  icon_proxy_url?: string | null;
  note?: string;
  partial?: boolean;
}

export type CreatorSearchStatus = 'ok' | 'empty' | 'error' | 'timeout';

export interface CreatorSearchCandidate {
  platform: string;
  kind: SourceKind;
  name: string;
  handle?: string;
  feed_url: string;
  site_url?: string | null;
  icon_url?: string | null;
  icon_proxy_url?: string | null;
  note?: string;
  match?: 'exact' | 'prefix' | 'contains' | 'handle' | 'search';
  source?: 'native' | 'bestblogs';
  partial?: boolean;
  existing_source_id?: number;
}

export interface CreatorSearchPlatformStatus {
  platform: string;
  label: string;
  status: CreatorSearchStatus;
  count: number;
  error?: string;
}

export interface CreatorSearchResponse {
  query: string;
  candidates: CreatorSearchCandidate[];
  platforms: CreatorSearchPlatformStatus[];
}

export interface SourceBatchError {
  index: number;
  name?: string;
  feed_url?: string;
  code: string;
  message: string;
  existing_source_id?: number;
}

export interface SourceBatchCreateResponse {
  created: Source[];
  errors: SourceBatchError[];
}

// 批量修改：folder_id 省略 = 不变，0 = 移出（不分组），>0 = 移入该分组。
export interface SourceBatchUpdateInput {
  ids: number[];
  folder_id?: number;
  enabled?: boolean;
}

export interface SourceBatchUpdateResponse {
  updated: number;
  missing?: number[];
}

export interface SourceBatchDeleteResponse {
  deleted: number;
  missing?: number[];
}

export interface SourceInput {
  kind: SourceKind;
  name: string;
  feed_url: string;
  handle?: string;
  site_url?: string | null;
  folder_id: number | null;
  refresh_seconds?: number;
  initial_limit?: number;
  fallback_url?: string | null;
  icon_url?: string | null;
  note?: string | null;
  retention_days?: number;
  retention_max_items?: number;
  backfill_mode?: 'window' | 'full';
  full_text?: boolean;
  watch_enabled?: boolean;
  watch_mode?: WatchMode;
  watch_ai?: boolean;
  watch_channel_ids?: number[];
}

export interface SourcePatch {
  name?: string;
  feed_url?: string;
  folder_id?: number | null;
  refresh_seconds?: number;
  fallback_url?: string | null;
  enabled?: boolean;
  retention_days?: number;
  retention_max_items?: number;
  initial_limit?: number;
  backfill_mode?: 'window' | 'full';
  full_text?: boolean;
  watch_enabled?: boolean;
  watch_mode?: WatchMode;
  watch_ai?: boolean;
  watch_channel_ids?: number[];
}

// REQ-041：邮箱自动同步（IMAP）。支持多个账号；授权码只写不读，
// GET 只返回每个账号的 password_set 状态。
export interface EmailAccount {
  id: string;
  host: string;
  port: number;
  username: string;
  poll_interval_minutes: number;
  /** 单轮同步最多拉取多少封（跨文件夹）。 */
  sync_limit: number;
  folder_id: number | null;
  password_set: boolean;
}

export interface EmailAccountInput {
  id: string;
  host: string;
  port: number;
  username: string;
  poll_interval_minutes: number;
  sync_limit: number;
  folder_id: number | null;
  /** 仅用于前端保留已保存状态；提交时后端忽略。 */
  password_set?: boolean;
}

export interface EmailSettings {
  enabled: boolean;
  block_remote_images: boolean;
  trusted_senders: string[];
  accounts: EmailAccount[];
}

export interface EmailPatch {
  enabled?: boolean;
  block_remote_images?: boolean;
  trusted_senders?: string[];
  accounts?: EmailAccountInput[];
  /** 单独保存/清除某个账号的授权码。 */
  account_id?: string;
  password?: string | null;
}

export interface EmailTestInput {
  /** 已保存账号的 ID：省略 host/授权码时用账号配置补齐。 */
  account_id?: string;
  host?: string;
  port?: number;
  username?: string;
  /** 省略 = 用该账号已保存的授权码测试。 */
  password?: string;
}

export interface EmailTestResponse {
  ok: boolean;
  mailboxes: string[];
}

export interface Settings {
  reader: {
    font_size: number;
    line_width: number;
    line_height: number;
    theme: ThemePreference;
    accent: AccentColor;
    auto_read: boolean;
    remember_position: boolean;
    rail_width: number;
    list_width: number;
    views: SavedView[];
    pinned_sources: number[];
  };
  ai: {
    enabled: boolean;
    base_url: string;
    model: string;
    api_key_set: boolean;
    summary_style: SummaryStyle;
    target_lang: string;
    daily_limit: number;
    used_today: number;
    tokens_month: number;
    consent: boolean;
    prompts: AIPromptSettings;
    prompt_defaults: AIPromptSettings;
  };
  wechat: {
    enabled: boolean;
    weread_connected: boolean;
    weread_can_renew?: boolean;
  };
  backup: {
    daily: boolean;
    keep: number;
    last_backup_at: string | null;
  };
  feed: {
    token_masked: string;
  };
  security: {
    last_login_at: string | null;
    session_expires_at: string | null;
    account?: string;
  };
  mcp: {
    enabled: boolean;
    endpoint: string;
  };
  email: EmailSettings;
}

export interface MCPToken {
  id: number;
  name: string;
  masked: string;
  enabled: boolean;
  created_at: string;
  last_used_at?: string | null;
}

export interface MCPTokenCreated extends MCPToken {
  token: string;
}

export type SettingsPatch = {
  reader?: Partial<Settings['reader']>;
  ai?: Omit<Partial<Settings['ai']>, 'prompts'> & { prompts?: Partial<AIPromptSettings> };
  wechat?: Partial<Settings['wechat']>;
  backup?: Partial<Settings['backup']>;
  feed?: Partial<Settings['feed']>;
  security?: Partial<Settings['security']>;
  mcp?: Partial<Settings['mcp']>;
  email?: EmailPatch;
};

export interface WechatQrCode {
  qrcode: string;
}

export interface WechatAccount {
  fake_id: string;
  name: string;
  avatar?: string;
  alias?: string;
}

export type SummaryStreamEvent =
  | { type: 'delta'; text: string }
  | { type: 'done'; summary: AiSummary }
  | { type: 'error'; code?: string; message: string };

export type TranslationStreamEvent =
  | { type: 'title'; title: string }
  | { type: 'segment'; index: number; text: string }
  | { type: 'progress'; done: number; total: number }
  | { type: 'done'; translation: Translation }
  | { type: 'error'; code?: string; message: string };

export interface WechatSettings {
  enabled: boolean;
  weread_connected: boolean;
  // 落库 Cookie 是否带 wr_rt 刷新令牌；false 表示无法自动续期，需要重新扫码一次。
  weread_can_renew?: boolean;
}

export interface WeReadScanStatus {
  state: 'waiting' | 'done' | 'expired' | 'error';
  message?: string;
  qrcode?: string;
}

export type AuthMode = 'oidc';

export interface AuthConfig {
  mode: AuthMode;
  login_url?: string;
}

export interface SessionResponse {
  authenticated: boolean;
  csrf_token: string;
}

// mock 模式 signIn（模拟懒猫 OIDC）的返回；真实模式登录走 OIDC 重定向，不用它。
export interface LoginResponse {
  csrf_token: string;
}

export interface PublicHealth {
  ok: boolean;
}

export interface BatchStateResponse {
  updated: number;
  ids: number[];
}

export interface TokenResetResponse {
  token_masked: string;
}

export interface FeedTokenReveal {
  token: string;
}

export interface AiTestResponse {
  ok: boolean;
  latency_ms: number;
}

export interface ApiErrorBody {
  error: {
    code: string;
    message: string;
    details?: unknown;
  };
}

export class ApiError extends Error {
  status: number;
  code: string;
  details?: unknown;

  constructor(status: number, code: string, message: string, details?: unknown) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.code = code;
    this.details = details;
  }
}

export interface ApiClient {
  getPublicHealth(): Promise<PublicHealth>;
  getAuthConfig(): Promise<AuthConfig>;
  getSession(): Promise<SessionResponse>;
  /** 仅 mock 模式实现：模拟懒猫 OIDC 建立演示会话；真实模式走 OIDC 重定向。 */
  signIn?(): Promise<LoginResponse>;
  logout(): Promise<void>;
  getFolders(): Promise<Folder[]>;
  createFolder(name: string, parentId?: number | null): Promise<Folder>;
  updateFolder(id: number, name: string, parentId?: number | null, sort?: number): Promise<Folder>;
  deleteFolder(id: number): Promise<void>;
  exportOpml(folderId?: number): Promise<string>;
  importOpml(file: File, folderId?: number): Promise<OpmlImportResult>;
  getSources(): Promise<Source[]>;
  getItems(query?: ItemQuery): Promise<ItemPage>;
  getReaderStats(): Promise<ReaderStats>;
  getItem(id: number): Promise<ItemDetail>;
  getItemMarkdown(id: number): Promise<string>;
  fetchItemFullText(id: number): Promise<ItemDetail>;
  patchItemState(id: number, patch: { read?: boolean; starred?: boolean; later?: boolean; hidden?: boolean }): Promise<ItemSummary>;
  setItemEmailImages(id: number, allowed: boolean): Promise<{ email_images_allowed: boolean }>;
  batchItemState(ids: number[], patch: { read?: boolean; starred?: boolean; later?: boolean; hidden?: boolean }): Promise<BatchStateResponse>;
  getItemNotes(id: number): Promise<ItemNote[]>;
  createItemNote(id: number, input: ItemNoteInput): Promise<ItemNote>;
  updateItemNote(id: number, noteId: number, input: Partial<ItemNoteInput>): Promise<ItemNote>;
  deleteItemNote(id: number, noteId: number): Promise<void>;
  markAllRead(target: { source_id?: number; folder_id?: number }): Promise<BatchStateResponse>;
  getSourceHealth(id: number): Promise<SourceHealth>;
  refreshSource(id: number): Promise<Source>;
  switchSourceFallback(id: number): Promise<Source>;
  updateSource(id: number, patch: SourcePatch): Promise<Source>;
  createSource(input: SourceInput): Promise<Source>;
  deleteSource(id: number): Promise<void>;
  discoverFeed(url: string): Promise<FeedCandidate[]>;
  resolveSource(input: string): Promise<ResolveCandidate[]>;
  searchCreators(query: string, platforms?: string[]): Promise<CreatorSearchResponse>;
  createSources(inputs: SourceInput[]): Promise<SourceBatchCreateResponse>;
  batchUpdateSources(input: SourceBatchUpdateInput): Promise<SourceBatchUpdateResponse>;
  batchDeleteSources(ids: number[]): Promise<SourceBatchDeleteResponse>;
  resetFeedToken(id: number): Promise<TokenResetResponse>;
  getSettings(): Promise<Settings>;
  updateSettings(patch: SettingsPatch): Promise<Settings>;
  saveAiKey(apiKey: string): Promise<Settings>;
  testAiConnection(): Promise<AiTestResponse>;
  /** REQ-041：null = 清除授权码；字符串 = 覆盖保存。 */
  saveEmailPassword(accountId: string, password: string | null): Promise<Settings>;
  testEmailConnection(input: EmailTestInput): Promise<EmailTestResponse>;
  generateSummary(itemId: number): Promise<AiSummary>;
  translateItem(itemId: number): Promise<Translation>;
  savePodcastProgress(itemId: number, positionSeconds: number, durationSeconds: number): Promise<void>;
  streamSummary(itemId: number, onEvent: (event: SummaryStreamEvent) => void): Promise<void>;
  streamTranslation(itemId: number, onEvent: (event: TranslationStreamEvent) => void, force?: boolean): Promise<void>;
  scanWeread(): Promise<WechatQrCode>;
  wereadScanStatus(): Promise<WeReadScanStatus>;
  wechatResolveArticle(url: string): Promise<WechatAccount>;
  createWechatSource(input: { account: string; name?: string; avatar?: string; folder_id?: number | null }): Promise<Source>;
  backupNow(): Promise<Settings['backup']>;
  downloadBackup(): Promise<void>;
  restoreBackup(file?: File): Promise<void>;
  getSourceLogs(id: number, limit?: number): Promise<FetchLog[]>;
  // REQ-036/037：定时报告、推送渠道与关键字过滤（自动化）。
  getChannels(): Promise<PushChannel[]>;
  createChannel(input: PushChannelInput): Promise<{ id: number }>;
  updateChannel(id: number, input: PushChannelInput): Promise<void>;
  deleteChannel(id: number): Promise<void>;
  testChannel(id: number): Promise<{ ok: true }>;
  getDigestRules(): Promise<DigestRule[]>;
  createDigestRule(input: DigestRuleInput): Promise<{ id: number }>;
  updateDigestRule(id: number, input: DigestRuleInput): Promise<void>;
  deleteDigestRule(id: number): Promise<void>;
  runDigestRule(id: number): Promise<DigestReport>;
  getDigestReports(ruleId?: number, limit?: number): Promise<DigestReport[]>;
  getDigestReport(id: number): Promise<DigestReport>;
  pushDigestReport(id: number): Promise<{ push_status: DigestPushStatus }>;
  getFilterRules(): Promise<FilterRule[]>;
  createFilterRule(input: FilterRuleInput): Promise<{ id: number }>;
  updateFilterRule(id: number, input: FilterRuleInput): Promise<void>;
  deleteFilterRule(id: number): Promise<void>;
  applyFilterRule(id: number): Promise<{ affected: number }>;
  // REQ-038：MCP Bearer Token 管理。
  getMcpTokens(): Promise<MCPToken[]>;
  createMcpToken(name: string): Promise<MCPTokenCreated>;
  updateMcpToken(id: number, patch: { name?: string; enabled?: boolean }): Promise<void>;
  deleteMcpToken(id: number): Promise<void>;
}
