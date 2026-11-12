import { create } from 'zustand';
import { api } from '@/api/client';
import type {
  Folder,
  Source,
  SourceBatchUpdateInput,
  SourceBatchUpdateResponse,
  SourceHealth,
  SourceInput,
  SourcePatch,
} from '@/api/types';
import { useUiStore } from '@/stores/uiStore';
import { folderSubtreeIds } from '@/utils/folders';

interface SourcesState {
  folders: Folder[];
  sources: Source[];
  loading: boolean;
  loaded: boolean;
  error: string | null;
  health: Record<number, SourceHealth>;
  load: (force?: boolean) => Promise<void>;
  loadHealth: (id: number, force?: boolean) => Promise<SourceHealth | null>;
  refresh: (id: number) => Promise<void>;
  switchFallback: (id: number) => Promise<void>;
  toggleEnabled: (id: number) => Promise<void>;
  update: (id: number, patch: SourcePatch) => Promise<Source | null>;
  create: (input: SourceInput) => Promise<Source | null>;
  createWechat: (input: { account: string; name?: string; avatar?: string; folder_id?: number | null }) => Promise<Source | null>;
  createFolder: (name: string, parentId?: number | null) => Promise<Folder | null>;
  renameFolder: (id: number, name: string, parentId?: number | null) => Promise<Folder | null>;
  moveFolder: (id: number, parentId: number | null, beforeId?: number | null) => Promise<boolean>;
  batchMoveFolders: (ids: number[], parentId: number | null) => Promise<boolean>;
  moveSource: (id: number, folderId: number | null) => Promise<boolean>;
  removeFolder: (id: number) => Promise<void>;
  exportOpml: (folderId?: number) => Promise<string | null>;
  importOpml: (file: File, folderId?: number) => Promise<void>;
  remove: (id: number) => Promise<void>;
  batchUpdate: (ids: number[], patch: Omit<SourceBatchUpdateInput, 'ids'>) => Promise<SourceBatchUpdateResponse | null>;
  batchRemove: (ids: number[]) => Promise<boolean>;
  batchRefresh: (ids: number[]) => Promise<void>;
  batchRemoveFolders: (ids: number[]) => Promise<boolean>;
  resetToken: (id: number) => Promise<void>;
  adjustUnread: (sourceId: number, delta: number) => void;
  reset: () => void;
}

const replaceSource = (sources: Source[], source: Source): Source[] => sources.map((entry) => entry.id === source.id ? source : entry);

let loadQueue: Promise<void> = Promise.resolve();

export const useSourcesStore = create<SourcesState>((set, get) => ({
  folders: [],
  sources: [],
  loading: false,
  loaded: false,
  error: null,
  health: {},

  load: async (force = false) => {
    if (get().loaded && !force) return;
    if (get().loading && !force) return;
    // 强制刷新排队执行：等在途加载结束后必定再刷一次，
    // 避免「添加源成功但列表没刷新」的竞态。
    const run = async () => {
      if (get().loading) return;
      set({ loading: true, error: null });
      try {
        const [folders, sources] = await Promise.all([api.getFolders(), api.getSources()]);
        set({ folders, sources, loading: false, loaded: true });
      } catch (error) {
        set({ loading: false, error: error instanceof Error ? error.message : '订阅源加载失败' });
      }
    };
    const next = loadQueue.then(run, run);
    loadQueue = next.then(() => {}, () => {});
    await next;
  },

  loadHealth: async (id, force = false) => {
    const cached = get().health[id];
    if (cached && !force) return cached;
    try {
      const health = await api.getSourceHealth(id);
      set((state) => ({
        health: { ...state.health, [id]: health },
        sources: replaceSource(state.sources, health.source),
      }));
      return health;
    } catch (error) {
      useUiStore.getState().notify(error instanceof Error ? error.message : '源诊断加载失败');
      return null;
    }
  },

  refresh: async (id) => {
    try {
      const updated = await api.refreshSource(id);
      set((state) => ({
        sources: replaceSource(state.sources, updated),
        health: state.health[id]
          ? { ...state.health, [id]: { ...state.health[id], source: updated } }
          : state.health,
      }));
      await get().loadHealth(id, true);
      if (updated.last_status === 'error') {
        useUiStore.getState().notify(updated.last_error ? `抓取失败：${updated.last_error}` : '抓取失败，请查看诊断');
      } else if (updated.last_status === 'not_modified') {
        useUiStore.getState().notify('抓取完成：304，未发现新条目');
      } else {
        useUiStore.getState().notify('内容已更新');
      }
    } catch (error) {
      useUiStore.getState().notify(error instanceof Error ? error.message : '抓取失败');
    }
  },

  switchFallback: async (id) => {
    try {
      const updated = await api.switchSourceFallback(id);
      set((state) => ({
        sources: replaceSource(state.sources, updated),
        health: state.health[id]
          ? { ...state.health, [id]: { ...state.health[id], source: updated } }
          : state.health,
      }));
      useUiStore.getState().notify(`已切换至备用源${updated.fallback_url ? `「${updated.fallback_url}」` : ''}`);
    } catch (error) {
      useUiStore.getState().notify(error instanceof Error ? error.message : '切换备用源失败');
    }
  },

  toggleEnabled: async (id) => {
    const source = get().sources.find((entry) => entry.id === id);
    if (!source) return;
    try {
      const updated = await api.updateSource(id, { enabled: !source.enabled });
      set((state) => ({
        sources: replaceSource(state.sources, updated),
        health: state.health[id]
          ? { ...state.health, [id]: { ...state.health[id], source: updated } }
          : state.health,
      }));
      useUiStore.getState().notify(source.enabled ? `已暂停「${source.name}」` : `已启用「${source.name}」`);
    } catch (error) {
      useUiStore.getState().notify(error instanceof Error ? error.message : '状态更新失败');
    }
  },

  update: async (id, patch) => {
    try {
      const updated = await api.updateSource(id, patch);
      set((state) => ({
        sources: replaceSource(state.sources, updated),
        health: state.health[id]
          ? { ...state.health, [id]: { ...state.health[id], source: updated } }
          : state.health,
      }));
      useUiStore.getState().notify('内容已更新');
      return updated;
    } catch (error) {
      useUiStore.getState().notify(error instanceof Error ? error.message : '源设置保存失败');
      return null;
    }
  },

  create: async (input) => {
    try {
      const created = await api.createSource(input);
      await get().load(true);
      if (created.last_status === 'error') {
        useUiStore.getState().notify(`已添加「${created.name}」，首次同步失败，请查看诊断`);
      } else {
        useUiStore.getState().notify('内容已更新');
      }
      return created;
    } catch (error) {
      useUiStore.getState().notify(error instanceof Error ? error.message : '添加源失败');
      return null;
    }
  },

  // 公众号源与普通源走不同端点，但必须同样在创建后强刷列表，
  // 否则侧边栏要等页面刷新才能看到新源（用户实测反馈）。
  createWechat: async (input) => {
    try {
      const created = await api.createWechatSource(input);
      await get().load(true);
      useUiStore.getState().notify(`已添加「${created.name}」`);
      return created;
    } catch (error) {
      useUiStore.getState().notify(error instanceof Error ? error.message : '添加公众号源失败');
      return null;
    }
  },

  createFolder: async (name, parentId = null) => {
    try {
      const folder = await api.createFolder(name, parentId);
      await get().load(true);
      useUiStore.getState().notify(`已创建文件夹「${folder.name}」`);
      return folder;
    } catch (error) {
      useUiStore.getState().notify(error instanceof Error ? error.message : '创建文件夹失败');
      return null;
    }
  },

  renameFolder: async (id, name, parentId = null) => {
    try {
      const folder = await api.updateFolder(id, name, parentId);
      await get().load(true);
      useUiStore.getState().notify(`已重命名为「${folder.name}」`);
      return folder;
    } catch (error) {
      useUiStore.getState().notify(error instanceof Error ? error.message : '重命名失败');
      return null;
    }
  },

  // 拖拽分组：先算目标层级内的完整顺序，再逐个写回 parent_id/sort。
  // 后端 PATCH 只做单行修改，因此这里用顺序请求保证最终顺序可解释；
  // 任一请求失败都会重新加载真实数据，避免界面停在半完成状态。
  moveFolder: async (id, parentId, beforeId = null) => {
    const folders = get().folders;
    const dragged = folders.find((folder) => folder.id === id);
    if (!dragged) return false;
    if (parentId != null && folderSubtreeIds(folders, id).has(parentId)) {
      useUiStore.getState().notify('不能把分组移动到自身或子分组下');
      return false;
    }
    const siblings = folders
      .filter((folder) => folder.id !== id && (folder.parent_id ?? null) === parentId)
      .sort((a, b) => a.sort - b.sort || a.id - b.id);
    const found = beforeId == null ? -1 : siblings.findIndex((folder) => folder.id === beforeId);
    const index = found < 0 ? siblings.length : found;
    const order = [...siblings];
    order.splice(index, 0, dragged);
    try {
      await Promise.all(order.map((folder, i) => {
        const nextParent = folder.id === id ? parentId : (folder.parent_id ?? null);
        return api.updateFolder(folder.id, folder.name, nextParent, (i + 1) * 10);
      }));
      await get().load(true);
      const parentName = parentId == null
        ? null
        : get().folders.find((folder) => folder.id === parentId)?.name ?? null;
      if ((dragged.parent_id ?? null) === parentId) {
        useUiStore.getState().notify('分组顺序已更新');
      } else if (parentName) {
        useUiStore.getState().notify(`已移动分组「${dragged.name}」到「${parentName}」`);
      } else {
        useUiStore.getState().notify(`已移动分组「${dragged.name}」到根目录`);
      }
      return true;
    } catch (error) {
      await get().load(true);
      useUiStore.getState().notify(error instanceof Error ? error.message : '移动分组失败');
      return false;
    }
  },

  // 批量把分组挂到同一父级下；先排除目标自身/后代，再一次性重排目标层级。
  batchMoveFolders: async (ids, parentId) => {
    const folders = get().folders;
    const moving = folders.filter((folder) => ids.includes(folder.id));
    if (moving.length === 0) return false;
    for (const folder of moving) {
      if (parentId != null && folderSubtreeIds(folders, folder.id).has(parentId)) {
        useUiStore.getState().notify('不能把分组移动到自身或子分组下');
        return false;
      }
    }
    const movingIDs = new Set(moving.map((folder) => folder.id));
    const siblings = folders
      .filter((folder) => (folder.parent_id ?? null) === parentId && !movingIDs.has(folder.id))
      .sort((a, b) => a.sort - b.sort || a.id - b.id);
    const order = [...siblings, ...moving.sort((a, b) => a.sort - b.sort || a.id - b.id)];
    try {
      await Promise.all(order.map((folder, index) => {
        const nextParent = movingIDs.has(folder.id) ? parentId : (folder.parent_id ?? null);
        return api.updateFolder(folder.id, folder.name, nextParent, (index + 1) * 10);
      }));
      await get().load(true);
      useUiStore.getState().notify(`已移动 ${moving.length} 个分组`);
      return true;
    } catch (error) {
      await get().load(true);
      useUiStore.getState().notify(error instanceof Error ? error.message : '批量移动分组失败');
      return false;
    }
  },

  moveSource: async (id, folderId) => {
    const source = get().sources.find((entry) => entry.id === id);
    if (!source) return false;
    const target = folderId != null && folderId > 0 ? folderId : null;
    if ((source.folder_id ?? null) === target) return true;
    try {
      const updated = await api.updateSource(id, { folder_id: target });
      set((state) => ({
        sources: replaceSource(state.sources, updated),
        health: state.health[id]
          ? { ...state.health, [id]: { ...state.health[id], source: updated } }
          : state.health,
      }));
      const folder = target == null ? null : get().folders.find((entry) => entry.id === target);
      useUiStore.getState().notify(folder ? `已移动到「${folder.name}」` : '已移出分组');
      return true;
    } catch (error) {
      useUiStore.getState().notify(error instanceof Error ? error.message : '移动订阅源失败');
      return false;
    }
  },

  removeFolder: async (id) => {
    const folder = get().folders.find((entry) => entry.id === id);
    try {
      await api.deleteFolder(id);
      await get().load(true);
      useUiStore.getState().notify(`已删除文件夹「${folder?.name ?? ''}」，订阅源已移至未分组`);
    } catch (error) {
      useUiStore.getState().notify(error instanceof Error ? error.message : '删除文件夹失败');
    }
  },

  exportOpml: async (folderId) => {
    try {
      return await api.exportOpml(folderId);
    } catch (error) {
      useUiStore.getState().notify(error instanceof Error ? error.message : '导出 OPML 失败');
      return null;
    }
  },

  importOpml: async (file, folderId) => {
    try {
      const result = await api.importOpml(file, folderId);
      await get().load(true);
      const skipped = result.skipped.length > 0 ? `，跳过 ${result.skipped.length} 个` : '';
      useUiStore.getState().notify(`已导入 ${result.imported} 个订阅源${skipped}`);
    } catch (error) {
      useUiStore.getState().notify(error instanceof Error ? error.message : '导入 OPML 失败');
    }
  },

  remove: async (id) => {
    const source = get().sources.find((entry) => entry.id === id);
    try {
      await api.deleteSource(id);
      set((state) => {
        const health = { ...state.health };
        delete health[id];
        return { sources: state.sources.filter((entry) => entry.id !== id), health };
      });
      await get().load(true);
      useUiStore.getState().notify(`已删除「${source?.name ?? '源'}」`);
    } catch (error) {
      useUiStore.getState().notify(error instanceof Error ? error.message : '删除源失败');
    }
  },

  // 批量修改/删除走专用端点；missing = 请求里不存在的源 ID，需要提示部分失败。
  batchUpdate: async (ids, patch) => {
    try {
      const result = await api.batchUpdateSources({ ids, ...patch });
      await get().load(true);
      const missed = result.missing?.length ?? 0;
      useUiStore.getState().notify(
        missed > 0 ? `已更新 ${result.updated} 个订阅源，${missed} 个不存在` : `已更新 ${result.updated} 个订阅源`,
      );
      return result;
    } catch (error) {
      useUiStore.getState().notify(error instanceof Error ? error.message : '批量修改失败');
      return null;
    }
  },

  batchRemove: async (ids) => {
    try {
      const result = await api.batchDeleteSources(ids);
      set((state) => {
        const health = { ...state.health };
        for (const id of ids) delete health[id];
        const removed = new Set(ids);
        return { sources: state.sources.filter((entry) => !removed.has(entry.id)), health };
      });
      await get().load(true);
      const missed = result.missing?.length ?? 0;
      useUiStore.getState().notify(
        missed > 0 ? `已删除 ${result.deleted} 个订阅源，${missed} 个不存在` : `已删除 ${result.deleted} 个订阅源`,
      );
      return missed === 0;
    } catch (error) {
      useUiStore.getState().notify(error instanceof Error ? error.message : '批量删除失败');
      return false;
    }
  },

  batchRefresh: async (ids) => {
    if (ids.length === 0) return;
    try {
      await Promise.all(ids.map((id) => api.refreshSource(id)));
      await get().load(true);
      useUiStore.getState().notify(`已刷新 ${ids.length} 个订阅源`);
    } catch (error) {
      await get().load(true);
      useUiStore.getState().notify(error instanceof Error ? error.message : '批量刷新失败');
    }
  },

  // 删除分组只解除归属；逐项调用现有端点，成功后统一提示。
  batchRemoveFolders: async (ids) => {
    if (ids.length === 0) return true;
    try {
      await Promise.all(ids.map((id) => api.deleteFolder(id)));
      await get().load(true);
      useUiStore.getState().notify(`已删除 ${ids.length} 个分组，订阅源已移到未分组`);
      return true;
    } catch (error) {
      await get().load(true);
      useUiStore.getState().notify(error instanceof Error ? error.message : '批量删除分组失败');
      return false;
    }
  },

  resetToken: async (id) => {
    try {
      const response = await api.resetFeedToken(id);
      set((state) => ({
        health: state.health[id]
          ? { ...state.health, [id]: { ...state.health[id], feed_token_masked: response.token_masked } }
          : state.health,
      }));
      await get().loadHealth(id, true);
      useUiStore.getState().notify('输出令牌已重置，旧地址立即失效');
    } catch (error) {
      useUiStore.getState().notify(error instanceof Error ? error.message : '令牌重置失败');
    }
  },

  adjustUnread: (sourceId, delta) => {
    set((state) => ({
      sources: state.sources.map((source) => source.id === sourceId
        ? { ...source, unread_count: Math.max(0, source.unread_count + delta) }
        : source),
    }));
  },

  reset: () => set({ folders: [], sources: [], loading: false, loaded: false, error: null, health: {} }),
}));
