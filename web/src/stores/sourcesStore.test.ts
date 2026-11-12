import { beforeEach, describe, expect, it, vi } from 'vitest';
import { api } from '@/api/client';
import type { Folder, Source } from '@/api/types';
import { useSourcesStore } from '@/stores/sourcesStore';

const folder: Folder = { id: 1, name: '默认分组', parent_id: null, sort: 0, source_count: 0 };

const sourceA: Source = {
  id: 1,
  kind: 'rss',
  name: 'A',
  feed_url: 'https://example.com/a.xml',
  site_url: null,
  icon_url: null,
  note: null,
  folder_id: null,
  enabled: true,
  refresh_seconds: 1800,
  etag: null,
  last_modified: null,
  fallback_url: null,
  initial_limit: 20,
  initial_fetched: true,
  last_fetch_at: null,
  last_status: 'ok',
  last_error: null,
  last_new_item_at: null,
  next_fetch_at: null,
  fail_count: 0,
  unread_count: 0,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
} as unknown as Source;

const sourceB = { ...sourceA, id: 2, name: 'B', feed_url: 'https://example.com/b.xml' };

describe('sourcesStore.load', () => {
  beforeEach(() => {
    useSourcesStore.getState().reset();
  });

  it('runs a forced reload queued behind an in-flight load instead of dropping it', async () => {
    let releaseFirst: ((value: Source[]) => void) | undefined;
    const first = new Promise<Source[]>((resolve) => {
      releaseFirst = resolve;
    });
    const getSources = vi.spyOn(api, 'getSources')
      .mockImplementationOnce(() => first)
      .mockImplementationOnce(() => Promise.resolve([sourceB]));
    vi.spyOn(api, 'getFolders').mockResolvedValue([folder]);

    const inFlight = useSourcesStore.getState().load();
    const forced = useSourcesStore.getState().load(true);

    // 首次加载仍在途时放行；强制刷新必须排队等它结束再刷一次。
    releaseFirst!([sourceA]);
    await inFlight;
    expect(useSourcesStore.getState().sources.map((s) => s.name)).toEqual(['A']);

    await forced;
    expect(useSourcesStore.getState().sources.map((s) => s.name)).toEqual(['B']);
    expect(getSources).toHaveBeenCalledTimes(2);

    vi.restoreAllMocks();
  });

  it('skips non-forced loads once loaded', async () => {
    const getSources = vi.spyOn(api, 'getSources').mockResolvedValue([sourceA]);
    vi.spyOn(api, 'getFolders').mockResolvedValue([folder]);
    await useSourcesStore.getState().load();
    await useSourcesStore.getState().load();
    expect(getSources).toHaveBeenCalledTimes(1);
    vi.restoreAllMocks();
  });
});
