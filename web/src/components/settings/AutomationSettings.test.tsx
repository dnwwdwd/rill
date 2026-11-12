import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/react';
import { api } from '@/api/client';
import { AutomationSettings } from '@/components/settings/AutomationSettings';

const rule = vi.hoisted(() => ({
  id: 1,
  name: '日报',
  scope: [] as Array<number | string>,
  period: 'daily' as const,
  run_at_local: '11:52',
  weekday: 1,
  only_unread: true,
  push_when_empty: false,
  enabled: true,
  channel_ids: [] as number[],
  last_run_at: null,
  next_run_at: '2026-11-10T11:52:00Z',
  created_at: '2026-11-10T03:00:00Z',
  updated_at: '2026-11-10T03:00:00Z',
}));

vi.mock('@/api/client', () => ({
  api: {
    getChannels: vi.fn().mockResolvedValue([]),
    getDigestRules: vi.fn().mockResolvedValue([rule]),
    getDigestReports: vi.fn().mockResolvedValue([]),
    getFilterRules: vi.fn().mockResolvedValue([]),
    getFolders: vi.fn().mockResolvedValue([]),
    getSources: vi.fn().mockResolvedValue([]),
    getReaderStats: vi.fn().mockResolvedValue({ hidden: 0 }),
  },
}));

const makeSource = (id: number, folderId: number) => ({
  id,
  kind: 'rss' as const,
  name: `源 ${id}`,
  feed_url: `https://example.com/${id}.xml`,
  site_url: null,
  folder_id: folderId,
  enabled: true,
  refresh_seconds: 3600,
  initial_limit: 20,
  fallback_url: null,
  last_fetch_at: null,
  last_new_item_at: null,
  next_fetch_at: '2026-11-10T04:00:00Z',
  last_status: 'ok' as const,
  last_error: null,
  fail_count: 0,
  item_count: 0,
  unread_count: 0,
  icon_url: null,
  backfill_mode: 'window' as const,
});

beforeEach(() => {
  vi.mocked(api.getDigestRules).mockResolvedValue([rule]);
  vi.mocked(api.getFolders).mockResolvedValue([]);
  vi.mocked(api.getSources).mockResolvedValue([]);
});

describe('AutomationSettings', () => {
  it('opens the rule editor when the edit button is clicked', async () => {
    render(<AutomationSettings />);
    await screen.findByText('日报');

    fireEvent.click(screen.getByRole('button', { name: '编辑 日报' }));

    expect(screen.getByLabelText('规则名称')).toHaveValue('日报');
  });

  it('does not repeat the source count for folder-only scopes', async () => {
    vi.mocked(api.getFolders).mockResolvedValue([
      { id: 1, name: '技术', sort: 0, source_count: 5 },
      { id: 2, name: '阅读', sort: 1, source_count: 5 },
    ]);
    vi.mocked(api.getSources).mockResolvedValue([
      ...[1, 2, 3, 4, 5].map((id) => makeSource(id, 1)),
      ...[6, 7, 8, 9, 10].map((id) => makeSource(id, 2)),
    ]);
    vi.mocked(api.getDigestRules).mockResolvedValue([{ ...rule, scope: ['folder:1', 'folder:2'] }]);

    render(<AutomationSettings />);

    expect(await screen.findByText(/2 个分组（10 源）/)).toBeInTheDocument();
    expect(screen.queryByText(/约 10 源/)).not.toBeInTheDocument();
  });
});
