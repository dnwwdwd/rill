import type { SourceStatus } from '@/api/types';
import type { IconName } from '@/components/ui/Icon';

export type ReaderFilter =
  | { type: 'inbox' }
  | { type: 'unread' }
  | { type: 'starred' }
  | { type: 'later' }
  | { type: 'read' }
  | { type: 'source'; sourceId: number }
  | { type: 'folder'; folderId: number };

export type TranslationMode = 'translated' | 'original' | 'bilingual';
export type ReadStatusFilter = 'all' | 'unread' | 'read';
export type SourceFilter = 'all' | 'ok' | 'attention' | 'paused';

export interface ToastState {
  id: number;
  message: string;
  actionLabel?: string;
  onAction?: () => void;
}

export interface SourceStatusMeta {
  status: SourceStatus;
  label: string;
}

export const SELECTABLE_SETTINGS = ['sources', 'reading', 'ai', 'automation', 'mcp', 'backup', 'security'] as const;
export type SettingsSection = typeof SELECTABLE_SETTINGS[number];

// 设置分类（REQ-023 + REQ-036/037）：公众号接入并入订阅源；自动化承载定时报告/推送渠道/过滤规则。
export const SETTINGS_SECTIONS: Array<{ key: SettingsSection; label: string; icon: IconName }> = [
  { key: 'sources', label: '订阅源', icon: 'settings-sources' },
  { key: 'reading', label: '阅读', icon: 'settings-reading' },
  { key: 'ai', label: 'AI', icon: 'sparkles' },
  { key: 'automation', label: '自动化', icon: 'zap' },
  { key: 'mcp', label: 'MCP', icon: 'bot' },
  { key: 'backup', label: '备份', icon: 'settings-backup' },
  { key: 'security', label: '安全', icon: 'settings-security' },
];
