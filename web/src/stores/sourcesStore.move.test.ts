import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { api } from '@/api/client';
import { createSeedData } from '@/api/mock/seed';
import { useSourcesStore } from '@/stores/sourcesStore';

const seed = createSeedData();

describe('sourcesStore tree moves', () => {
  beforeEach(() => {
    useSourcesStore.setState({ folders: seed.folders, sources: seed.sources, loaded: true, health: {} });
  });

  afterEach(() => {
    useSourcesStore.getState().reset();
    vi.restoreAllMocks();
  });

  it('moves a source to a folder and updates local state', async () => {
    const source = seed.sources.find((entry) => entry.id === 8)!;
    const update = vi.spyOn(api, 'updateSource').mockResolvedValue({ ...source, folder_id: 1 });

    const ok = await useSourcesStore.getState().moveSource(8, 1);

    expect(ok).toBe(true);
    expect(update).toHaveBeenCalledWith(8, { folder_id: 1 });
    expect(useSourcesStore.getState().sources.find((entry) => entry.id === 8)?.folder_id).toBe(1);
  });

  it('reorders and re-parents folders with sequential sort values', async () => {
    const update = vi.spyOn(api, 'updateFolder').mockImplementation(async (id, name, parentId) => ({
      id,
      name,
      parent_id: parentId ?? null,
      sort: 0,
      source_count: 0,
    }));
    vi.spyOn(api, 'getFolders').mockResolvedValue(seed.folders);
    vi.spyOn(api, 'getSources').mockResolvedValue(seed.sources);

    const ok = await useSourcesStore.getState().moveFolder(3, null, 1);

    expect(ok).toBe(true);
    expect(update).toHaveBeenCalledWith(3, '资讯', null, 10);
    expect(update).toHaveBeenCalledWith(1, '技术', null, 20);
  });

  it('batch moves folders under a target parent', async () => {
    const update = vi.spyOn(api, 'updateFolder').mockImplementation(async (id, name, parentId) => ({
      id,
      name,
      parent_id: parentId ?? null,
      sort: 0,
      source_count: 0,
    }));
    vi.spyOn(api, 'getFolders').mockResolvedValue(seed.folders);
    vi.spyOn(api, 'getSources').mockResolvedValue(seed.sources);

    const ok = await useSourcesStore.getState().batchMoveFolders([2, 3], 1);

    expect(ok).toBe(true);
    expect(update).toHaveBeenCalledWith(2, '设计', 1, expect.any(Number));
    expect(update).toHaveBeenCalledWith(3, '资讯', 1, expect.any(Number));
  });

  it('rejects moving a folder under its own descendant', async () => {
    useSourcesStore.setState({
      folders: [
        ...seed.folders,
        { id: 20, name: '子分组', sort: 1, parent_id: 1, source_count: 0 },
      ],
    });
    const update = vi.spyOn(api, 'updateFolder');

    const ok = await useSourcesStore.getState().moveFolder(1, 20, null);

    expect(ok).toBe(false);
    expect(update).not.toHaveBeenCalled();
  });
});
