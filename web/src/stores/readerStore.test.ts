import { beforeEach, describe, expect, it } from 'vitest';
import { mockApi } from '@/api/mock/mockApi';
import { useReaderStore } from '@/stores/readerStore';
import { loadLastItemId, loadReaderFilter } from '@/utils/readerPersistence';

describe('readerStore list filters', () => {
  beforeEach(async () => {
    window.sessionStorage.clear();
    await mockApi.signIn?.();
    useReaderStore.getState().reset();
  });

  it('persists the selected source and last opened article across reloads', async () => {
    useReaderStore.getState().setFilter({ type: 'source', sourceId: 27 });
    useReaderStore.getState().setReadFilter('unread');
    expect(loadReaderFilter()).toEqual({ type: 'source', sourceId: 27 });

    await useReaderStore.getState().loadDetail(1);
    expect(loadLastItemId()).toBe(1);

    useReaderStore.getState().reset();
    expect(loadReaderFilter()).toEqual({ type: 'inbox' });
    expect(loadLastItemId()).toBeNull();
  });

  it('sorts by published time and filters read status through the list query', async () => {
    await useReaderStore.getState().loadItems(true);
    const allItems = useReaderStore.getState().items;
    expect(allItems.length).toBeGreaterThan(0);
    const published = allItems.map((item) => item.published_at ?? item.fetched_at);
    expect(published).toEqual([...published].sort().reverse());

    useReaderStore.getState().setReadFilter('unread');
    await useReaderStore.getState().loadItems(true);
    expect(useReaderStore.getState().items.length).toBeGreaterThan(0);
    expect(useReaderStore.getState().items.every((item) => item.read_at === null)).toBe(true);

    useReaderStore.getState().setReadFilter('read');
    await useReaderStore.getState().loadItems(true);
    expect(useReaderStore.getState().items.length).toBeGreaterThan(0);
    expect(useReaderStore.getState().items.every((item) => item.read_at !== null)).toBe(true);
  });
});
