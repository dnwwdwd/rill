import { create } from 'zustand';
import { api } from '@/api/client';
import type {
  ItemDetail,
  ItemNote,
  ItemNoteInput,
  ItemQuery,
  ItemSummary,
  ReaderStats,
} from '@/api/types';
import type { ReadStatusFilter, ReaderFilter, TranslationMode } from '@/types/ui';
import { useSourcesStore } from '@/stores/sourcesStore';
import { useUiStore } from '@/stores/uiStore';

interface ReaderState {
  filter: ReaderFilter;
  query: string;
  readFilter: ReadStatusFilter;
  items: ItemSummary[];
  stats: ReaderStats;
  nextCursor: string | null;
  loading: boolean;
  loadingMore: boolean;
  error: string | null;
  details: Record<number, ItemDetail>;
  detailLoading: Record<number, boolean>;
  translationMode: TranslationMode;
  summaryBusy: Record<number, boolean>;
  translationBusy: Record<number, boolean>;
  fullTextBusy: Record<number, boolean>;
  summaryStream: Record<number, string>;
  translationStream: Record<number, { title?: string; segments: string[]; done: number; total: number }>;
  setFilter: (filter: ReaderFilter) => void;
  setQuery: (query: string) => void;
  setReadFilter: (readFilter: ReadStatusFilter) => void;
  loadItems: (reset?: boolean) => Promise<void>;
  loadMore: () => Promise<void>;
  loadStats: () => Promise<void>;
  loadDetail: (id: number, force?: boolean) => Promise<ItemDetail | null>;
  toggleRead: (id: number) => Promise<void>;
  toggleStar: (id: number) => Promise<void>;
  toggleLater: (id: number) => Promise<void>;
  markRead: (id: number) => Promise<void>;
  markAllRead: (target: { source_id?: number; folder_id?: number }) => Promise<number>;
  markItemsRead: (ids: number[]) => Promise<number>;
  batchState: (ids: number[], patch: { read?: boolean; starred?: boolean; later?: boolean; hidden?: boolean }) => Promise<number>;
  setTranslationMode: (mode: TranslationMode) => void;
  generateSummary: (id: number) => Promise<void>;
  translate: (id: number, force?: boolean) => Promise<void>;
  fetchFullText: (id: number) => Promise<void>;
  createNote: (id: number, input: ItemNoteInput) => Promise<ItemNote | null>;
  updateNote: (id: number, noteId: number, input: Partial<ItemNoteInput>) => Promise<ItemNote | null>;
  deleteNote: (id: number, noteId: number) => Promise<void>;
  reset: () => void;
}

const filterToQuery = (filter: ReaderFilter, query: string, readFilter: ReadStatusFilter): ItemQuery => {
  const readStatus = filter.type === 'unread'
    ? 'unread'
    : filter.type === 'read'
      ? 'read'
      : readFilter === 'all'
        ? null
        : readFilter;
  const base: ItemQuery = {
    sort: 'published',
    limit: 12,
    q: query.trim() || undefined,
    unread: readStatus === 'unread' || undefined,
    read: readStatus === 'read' || undefined,
  };
  switch (filter.type) {
    case 'starred':
      return { ...base, starred: true };
    case 'later':
      return { ...base, later: true };
    case 'source':
      return { ...base, source_id: filter.sourceId };
    case 'folder':
      return { ...base, folder_id: filter.folderId };
    default:
      return base;
  }
};

let loadToken = 0;

const updateSummary = (items: ItemSummary[], summary: ItemSummary): ItemSummary[] => (
  items.map((item) => item.id === summary.id ? { ...item, ...summary } : item)
);

export const useReaderStore = create<ReaderState>((set, get) => ({
  filter: { type: 'inbox' },
  query: '',
  readFilter: 'all',
  items: [],
  stats: { total: 0, unread: 0, starred: 0, read: 0, hidden: 0, later: 0 },
  nextCursor: null,
  loading: false,
  loadingMore: false,
  error: null,
  details: {},
  detailLoading: {},
  translationMode: 'translated',
  summaryBusy: {},
  translationBusy: {},
  fullTextBusy: {},
  summaryStream: {},
  translationStream: {},

  setFilter: (filter) => set({ filter, query: '' }),
  setQuery: (query) => set({ query }),
  setReadFilter: (readFilter) => set({ readFilter }),

  loadItems: async (reset = true) => {
    const token = ++loadToken;
    const { filter, query, readFilter, nextCursor } = get();
    if (reset) {
      set({ loading: true, loadingMore: false, nextCursor: null, error: null });
    } else {
      if (!nextCursor || get().loadingMore) return;
      set({ loadingMore: true, error: null });
    }
    try {
      const page = await api.getItems({
        ...filterToQuery(filter, query, readFilter),
        cursor: reset ? null : nextCursor,
      });
      if (token !== loadToken) return;
      set((state) => ({
        items: reset
          ? page.items
          : [...state.items, ...page.items.filter((item) => !state.items.some((existing) => existing.id === item.id))],
        nextCursor: page.next_cursor,
        loading: false,
        loadingMore: false,
        error: null,
      }));
    } catch (error) {
      if (token !== loadToken) return;
      set({
        loading: false,
        loadingMore: false,
        error: error instanceof Error ? error.message : '条目加载失败',
      });
    }
  },

  loadMore: async () => {
    await get().loadItems(false);
  },

  loadStats: async () => {
    try {
      const stats = await api.getReaderStats();
      set({ stats });
    } catch {
      /* 统计失败不阻塞阅读 */
    }
  },

  loadDetail: async (id, force = false) => {
    const cached = get().details[id];
    if (cached && !force) return cached;
    if (get().detailLoading[id]) return get().details[id] ?? null;
    set((state) => ({ detailLoading: { ...state.detailLoading, [id]: true } }));
    try {
      const detail = await api.getItem(id);
      set((state) => ({
        details: { ...state.details, [id]: detail },
        detailLoading: { ...state.detailLoading, [id]: false },
      }));
      return detail;
    } catch (error) {
      set((state) => ({ detailLoading: { ...state.detailLoading, [id]: false } }));
      useUiStore.getState().notify(error instanceof Error ? error.message : '条目加载失败');
      return null;
    }
  },

  toggleRead: async (id) => {
    const item = get().items.find((entry) => entry.id === id) ?? get().details[id];
    if (!item) return;
    const nextRead = !item.read_at;
    useSourcesStore.getState().adjustUnread(item.source_id, nextRead ? -1 : 1);
    set((state) => ({
      items: state.items.map((entry) => entry.id === id ? { ...entry, read_at: nextRead ? new Date().toISOString() : null } : entry),
      details: state.details[id]
        ? { ...state.details, [id]: { ...state.details[id], read_at: nextRead ? new Date().toISOString() : null } }
        : state.details,
      stats: {
        ...state.stats,
        unread: Math.max(0, state.stats.unread + (nextRead ? -1 : 1)),
        read: Math.max(0, state.stats.read + (nextRead ? 1 : -1)),
      },
    }));
    try {
      const updated = await api.patchItemState(id, { read: nextRead });
      set((state) => ({
        items: updateSummary(state.items, updated),
        details: state.details[id] ? { ...state.details, [id]: { ...state.details[id], read_at: updated.read_at } } : state.details,
      }));
      useUiStore.getState().notify(nextRead ? '已标为已读' : '已标为未读');
    } catch (error) {
      useSourcesStore.getState().adjustUnread(item.source_id, nextRead ? 1 : -1);
      set((state) => ({
        items: state.items.map((entry) => entry.id === id ? { ...entry, read_at: item.read_at } : entry),
        details: state.details[id] ? { ...state.details, [id]: { ...state.details[id], read_at: item.read_at } } : state.details,
        stats: {
          ...state.stats,
          unread: Math.max(0, state.stats.unread + (nextRead ? 1 : -1)),
          read: Math.max(0, state.stats.read + (nextRead ? -1 : 1)),
        },
      }));
      useUiStore.getState().notify(error instanceof Error ? error.message : '已读状态更新失败');
    }
  },

  toggleStar: async (id) => {
    const item = get().items.find((entry) => entry.id === id) ?? get().details[id];
    if (!item) return;
    const nextStarred = !item.starred_at;
    const optimistic = nextStarred ? new Date().toISOString() : null;
    set((state) => ({
      items: state.items.map((entry) => entry.id === id ? { ...entry, starred_at: optimistic } : entry),
      details: state.details[id]
        ? { ...state.details, [id]: { ...state.details[id], starred_at: optimistic } }
        : state.details,
      stats: { ...state.stats, starred: Math.max(0, state.stats.starred + (nextStarred ? 1 : -1)) },
    }));
    try {
      const updated = await api.patchItemState(id, { starred: nextStarred });
      set((state) => ({
        items: updateSummary(state.items, updated),
        details: state.details[id] ? { ...state.details, [id]: { ...state.details[id], starred_at: updated.starred_at } } : state.details,
      }));
      useUiStore.getState().notify(nextStarred ? '已收藏' : '已取消收藏');
    } catch (error) {
      set((state) => ({
        items: state.items.map((entry) => entry.id === id ? { ...entry, starred_at: item.starred_at } : entry),
        details: state.details[id] ? { ...state.details, [id]: { ...state.details[id], starred_at: item.starred_at } } : state.details,
        stats: { ...state.stats, starred: Math.max(0, state.stats.starred + (nextStarred ? -1 : 1)) },
      }));
      useUiStore.getState().notify(error instanceof Error ? error.message : '收藏状态更新失败');
    }
  },

  toggleLater: async (id) => {
    const item = get().items.find((entry) => entry.id === id) ?? get().details[id];
    if (!item) return;
    const nextLater = !item.later_at;
    const optimistic = nextLater ? new Date().toISOString() : null;
    set((state) => ({
      items: state.items.map((entry) => entry.id === id ? { ...entry, later_at: optimistic } : entry),
      details: state.details[id]
        ? { ...state.details, [id]: { ...state.details[id], later_at: optimistic } }
        : state.details,
      stats: { ...state.stats, later: Math.max(0, state.stats.later + (nextLater ? 1 : -1)) },
    }));
    try {
      const updated = await api.patchItemState(id, { later: nextLater });
      set((state) => ({
        items: updateSummary(state.items, updated),
        details: state.details[id] ? { ...state.details, [id]: { ...state.details[id], later_at: updated.later_at } } : state.details,
      }));
      useUiStore.getState().notify(nextLater ? '已加入稍后读' : '已移出稍后读');
    } catch (error) {
      set((state) => ({
        items: state.items.map((entry) => entry.id === id ? { ...entry, later_at: item.later_at } : entry),
        details: state.details[id] ? { ...state.details, [id]: { ...state.details[id], later_at: item.later_at } } : state.details,
        stats: { ...state.stats, later: Math.max(0, state.stats.later + (nextLater ? -1 : 1)) },
      }));
      useUiStore.getState().notify(error instanceof Error ? error.message : '稍后读更新失败');
    }
  },

  markRead: async (id) => {
    const item = get().items.find((entry) => entry.id === id) ?? get().details[id];
    if (!item || item.read_at) return;
    const readAt = new Date().toISOString();
    useSourcesStore.getState().adjustUnread(item.source_id, -1);
    set((state) => ({
      items: state.items.map((entry) => entry.id === id ? { ...entry, read_at: readAt } : entry),
      details: state.details[id] ? { ...state.details, [id]: { ...state.details[id], read_at: readAt } } : state.details,
      stats: { ...state.stats, unread: Math.max(0, state.stats.unread - 1), read: state.stats.read + 1 },
    }));
    try {
      await api.batchItemState([id], { read: true });
    } catch (error) {
      useSourcesStore.getState().adjustUnread(item.source_id, 1);
      set((state) => ({
        items: state.items.map((entry) => entry.id === id ? { ...entry, read_at: item.read_at } : entry),
        details: state.details[id] ? { ...state.details, [id]: { ...state.details[id], read_at: item.read_at } } : state.details,
        stats: { ...state.stats, unread: state.stats.unread + 1, read: Math.max(0, state.stats.read - 1) },
      }));
      useUiStore.getState().notify(error instanceof Error ? error.message : '标记已读失败');
    }
  },

  markAllRead: async (target) => {
    try {
      const response = await api.markAllRead(target);
      await Promise.all([get().loadItems(true), get().loadStats(), useSourcesStore.getState().load(true)]);
      useUiStore.getState().notify(`已标记 ${response.updated} 条为已读`, response.ids.length > 0 ? {
        label: '撤销',
        onClick: async () => {
          try {
            await api.batchItemState(response.ids, { read: false });
            await Promise.all([get().loadItems(true), get().loadStats(), useSourcesStore.getState().load(true)]);
            useUiStore.getState().notify('已撤销批量已读');
          } catch (error) {
            useUiStore.getState().notify(error instanceof Error ? error.message : '撤销失败');
          }
        },
      } : undefined);
      return response.updated;
    } catch (error) {
      useUiStore.getState().notify(error instanceof Error ? error.message : '批量标记失败');
      return 0;
    }
  },

  markItemsRead: async (ids) => {
    if (ids.length === 0) return 0;
    try {
      const response = await api.batchItemState(ids, { read: true });
      await Promise.all([get().loadItems(true), get().loadStats(), useSourcesStore.getState().load(true)]);
      useUiStore.getState().notify(`已标记 ${response.updated} 条为已读`, response.ids.length > 0 ? {
        label: '撤销',
        onClick: async () => {
          try {
            await api.batchItemState(response.ids, { read: false });
            await Promise.all([get().loadItems(true), get().loadStats(), useSourcesStore.getState().load(true)]);
            useUiStore.getState().notify('已撤销批量已读');
          } catch (error) {
            useUiStore.getState().notify(error instanceof Error ? error.message : '撤销失败');
          }
        },
      } : undefined);
      return response.updated;
    } catch (error) {
      useUiStore.getState().notify(error instanceof Error ? error.message : '批量标记失败');
      return 0;
    }
  },

  batchState: async (ids, patch) => {
    if (ids.length === 0) return 0;
    try {
      const response = await api.batchItemState(ids, patch);
      await Promise.all([get().loadItems(true), get().loadStats(), useSourcesStore.getState().load(true)]);
      useUiStore.getState().notify(`已更新 ${response.updated} 条`);
      return response.updated;
    } catch (error) {
      useUiStore.getState().notify(error instanceof Error ? error.message : '批量更新失败');
      return 0;
    }
  },

  fetchFullText: async (id) => {
    if (get().fullTextBusy[id]) return;
    set((state) => ({ fullTextBusy: { ...state.fullTextBusy, [id]: true } }));
    try {
      const detail = await api.fetchItemFullText(id);
      set((state) => ({
        details: { ...state.details, [id]: detail },
        items: updateSummary(state.items, detail),
      }));
      useUiStore.getState().notify('正文已更新');
    } catch (error) {
      useUiStore.getState().notify(error instanceof Error ? error.message : '抓取全文失败');
    } finally {
      set((state) => ({ fullTextBusy: { ...state.fullTextBusy, [id]: false } }));
    }
  },

  setTranslationMode: (translationMode) => set({ translationMode }),

  generateSummary: async (id) => {
    if (get().summaryBusy[id]) return;
    set((state) => ({
      summaryBusy: { ...state.summaryBusy, [id]: true },
      summaryStream: { ...state.summaryStream, [id]: '' },
    }));
    let failure = '';
    let completed = false;
    try {
      await api.streamSummary(id, (event) => {
        if (event.type === 'delta') {
          set((state) => ({
            summaryStream: { ...state.summaryStream, [id]: (state.summaryStream[id] ?? '') + event.text },
          }));
        } else if (event.type === 'done') {
          completed = true;
          set((state) => ({
            details: state.details[id] ? { ...state.details, [id]: { ...state.details[id], summary: event.summary } } : state.details,
          }));
        } else if (event.type === 'error') {
          failure = event.message;
        }
      });
    } catch (error) {
      failure = error instanceof Error ? error.message : '摘要生成失败';
    }
    set((state) => {
      const summaryStream = { ...state.summaryStream };
      delete summaryStream[id];
      return { summaryBusy: { ...state.summaryBusy, [id]: false }, summaryStream };
    });
    if (failure) {
      useUiStore.getState().notify(failure);
    } else if (completed) {
      useUiStore.getState().notify('摘要已生成');
    }
  },

  translate: async (id, force = false) => {
    if (get().translationBusy[id]) return;
    set((state) => ({
      translationBusy: { ...state.translationBusy, [id]: true },
      translationStream: { ...state.translationStream, [id]: { segments: [], done: 0, total: 0 } },
    }));
    let failure = '';
    let completed = false;
    try {
      await api.streamTranslation(id, (event) => {
        if (event.type === 'title') {
          set((state) => {
            const current = state.translationStream[id] ?? { segments: [], done: 0, total: 0 };
            return { translationStream: { ...state.translationStream, [id]: { ...current, title: event.title } } };
          });
        } else if (event.type === 'segment') {
          set((state) => {
            const current = state.translationStream[id] ?? { segments: [], done: 0, total: 0 };
            const segments = [...current.segments];
            segments[event.index] = event.text;
            return { translationStream: { ...state.translationStream, [id]: { ...current, segments } } };
          });
        } else if (event.type === 'progress') {
          set((state) => {
            const current = state.translationStream[id] ?? { segments: [], done: 0, total: 0 };
            return { translationStream: { ...state.translationStream, [id]: { ...current, done: event.done, total: event.total } } };
          });
        } else if (event.type === 'done') {
          completed = true;
          set((state) => ({
            details: state.details[id] ? { ...state.details, [id]: { ...state.details[id], translation: event.translation } } : state.details,
          }));
        } else if (event.type === 'error') {
          failure = event.message;
        }
      }, force);
    } catch (error) {
      failure = error instanceof Error ? error.message : '翻译失败';
    }
    set((state) => {
      const translationStream = { ...state.translationStream };
      delete translationStream[id];
      return { translationBusy: { ...state.translationBusy, [id]: false }, translationStream };
    });
    if (failure) {
      useUiStore.getState().notify(failure);
    } else if (completed) {
      useUiStore.getState().notify('译文已生成');
    }
  },

  createNote: async (id, input) => {
    try {
      const note = await api.createItemNote(id, input);
      set((state) => {
        const detail = state.details[id];
        if (!detail) return state;
        return { details: { ...state.details, [id]: { ...detail, notes: [...(detail.notes ?? []), note] } } };
      });
      useUiStore.getState().notify('笔记已保存');
      return note;
    } catch (error) {
      useUiStore.getState().notify(error instanceof Error ? error.message : '保存笔记失败');
      return null;
    }
  },

  updateNote: async (id, noteId, input) => {
    try {
      const note = await api.updateItemNote(id, noteId, input);
      set((state) => {
        const detail = state.details[id];
        if (!detail) return state;
        return {
          details: {
            ...state.details,
            [id]: { ...detail, notes: (detail.notes ?? []).map((entry) => entry.id === noteId ? note : entry) },
          },
        };
      });
      useUiStore.getState().notify('笔记已更新');
      return note;
    } catch (error) {
      useUiStore.getState().notify(error instanceof Error ? error.message : '更新笔记失败');
      return null;
    }
  },

  deleteNote: async (id, noteId) => {
    try {
      await api.deleteItemNote(id, noteId);
      set((state) => {
        const detail = state.details[id];
        if (!detail) return state;
        return {
          details: {
            ...state.details,
            [id]: { ...detail, notes: (detail.notes ?? []).filter((entry) => entry.id !== noteId) },
          },
        };
      });
      useUiStore.getState().notify('笔记已删除');
    } catch (error) {
      useUiStore.getState().notify(error instanceof Error ? error.message : '删除笔记失败');
    }
  },

  reset: () => {
    loadToken += 1;
    set({
      filter: { type: 'inbox' },
      query: '',
      readFilter: 'all',
      items: [],
      stats: { total: 0, unread: 0, starred: 0, read: 0, hidden: 0, later: 0 },
      nextCursor: null,
      loading: false,
      loadingMore: false,
      error: null,
      details: {},
      detailLoading: {},
      translationMode: 'translated',
      summaryBusy: {},
      translationBusy: {},
      fullTextBusy: {},
      summaryStream: {},
      translationStream: {},
    });
  },
}));
