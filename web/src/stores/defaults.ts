import type { AccentColor, Settings, SettingsPatch } from '@/api/types';

const storedTheme = (() => {
  try {
    const value = localStorage.getItem('rill.theme.preference');
    if (value === 'light' || value === 'dark' || value === 'system') return value;
  } catch {
    /* ignore storage errors */
  }
  return 'system' as const;
})();

const storedAccent = (() => {
  try {
    const value = localStorage.getItem('rill.accent');
    if (value === 'blue' || value === 'green' || value === 'cyan' || value === 'teal' || value === 'indigo' || value === 'purple' || value === 'pink' || value === 'rose' || value === 'red' || value === 'orange' || value === 'amber' || value === 'slate') {
      return value as AccentColor;
    }
  } catch {
    /* ignore storage errors */
  }
  return 'blue' as const;
})();

const storedWidth = (key: string, fallback: number, min: number, max: number): number => {
  try {
    const value = Number(localStorage.getItem(key));
    if (Number.isFinite(value) && value >= min && value <= max) return Math.round(value);
  } catch {
    /* ignore storage errors */
  }
  return fallback;
};

export const DEFAULT_RAIL_WIDTH = 224;
export const DEFAULT_LIST_WIDTH = 384;
export const RAIL_WIDTH_RANGE = { min: 180, max: 320 } as const;
export const LIST_WIDTH_RANGE = { min: 300, max: 520 } as const;

export const clampRailWidth = (value: number): number => (
  Math.min(RAIL_WIDTH_RANGE.max, Math.max(RAIL_WIDTH_RANGE.min, Math.round(value)))
);

export const clampListWidth = (value: number): number => (
  Math.min(LIST_WIDTH_RANGE.max, Math.max(LIST_WIDTH_RANGE.min, Math.round(value)))
);

export const DEFAULT_SETTINGS: Settings = {
  reader: {
    font_size: 18,
    line_width: 660,
    line_height: 1.85,
    theme: storedTheme,
    accent: storedAccent,
    auto_read: true,
    remember_position: true,
    rail_width: storedWidth('rill.rail_width', DEFAULT_RAIL_WIDTH, RAIL_WIDTH_RANGE.min, RAIL_WIDTH_RANGE.max),
    list_width: storedWidth('rill.list_width', DEFAULT_LIST_WIDTH, LIST_WIDTH_RANGE.min, LIST_WIDTH_RANGE.max),
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
  },
  backup: {
    daily: true,
    keep: 7,
    last_backup_at: null,
  },
  feed: {
    token_masked: 'rill_9f3c…a71e',
  },
  security: {
    last_login_at: null,
    session_expires_at: null,
  },
  mcp: {
    enabled: false,
    endpoint: '/mcp',
  },
};

export const mergeSettings = (current: Settings, patch: SettingsPatch): Settings => ({
  reader: { ...current.reader, ...patch.reader },
  ai: { ...current.ai, ...patch.ai },
  wechat: { ...current.wechat, ...patch.wechat },
  backup: { ...current.backup, ...patch.backup },
  feed: { ...current.feed, ...patch.feed },
  security: { ...current.security, ...patch.security },
  mcp: { ...current.mcp, ...patch.mcp },
});
