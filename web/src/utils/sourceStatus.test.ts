import { describe, expect, it } from 'vitest';
import type { Folder, Source } from '@/api/types';
import { deriveSourceStatus, toSourceView } from '@/utils/sourceStatus';

const base: Source = {
  id: 1,
  kind: 'rss',
  name: '示例源',
  feed_url: 'https://example.com/feed.xml',
  site_url: null,
  backfill_mode: 'window',
  folder_id: 1,
  enabled: true,
  refresh_seconds: 1800,
  initial_limit: 20,
  fallback_url: null,
  last_fetch_at: new Date().toISOString(),
  last_new_item_at: new Date().toISOString(),
  next_fetch_at: new Date(Date.now() + 1800_000).toISOString(),
  last_status: 'ok',
  last_error: null,
  fail_count: 0,
  item_count: 10,
  unread_count: 3,
  icon_url: null,
};

const folders: Folder[] = [{ id: 1, name: '技术', sort: 1, source_count: 1 }];

describe('sourceStatus', () => {
  it('derives paused, auth and error states from the persisted source', () => {
    expect(deriveSourceStatus({ ...base, enabled: false })).toBe('paused');
    expect(deriveSourceStatus({ ...base, kind: 'wechat', last_status: 'error', last_error: '401 unauthorized', fail_count: 1 })).toBe('auth');
    expect(deriveSourceStatus({ ...base, last_status: 'error', last_error: 'timeout', fail_count: 2 })).toBe('error');
    // 最近一次抓取失败绝不能显示「正常」。
    expect(deriveSourceStatus({ ...base, last_status: 'error', last_error: '登录态已失效，请重新扫码连接', fail_count: 0 })).toBe('error');
    expect(deriveSourceStatus({ ...base, last_status: 'error', last_error: 'HTTP 502', fail_count: 1 })).toBe('error');
  });

  it('keeps a silent source normal: no new items is not a status problem', () => {
    const stale = { ...base, last_new_item_at: new Date(Date.now() - 2 * 1800_000 - 60_000).toISOString() };
    expect(deriveSourceStatus(stale)).toBe('ok');
    const view = toSourceView(stale, folders);
    expect(view.folder_name).toBe('技术');
    expect(view.status).toBe('ok');
  });
});
