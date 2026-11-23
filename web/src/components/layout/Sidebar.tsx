import {
  useEffect,
  useRef,
  useState,
  type DragEvent,
  type KeyboardEvent as ReactKeyboardEvent,
  type MouseEvent as ReactMouseEvent,
  type PointerEvent as ReactPointerEvent,
} from 'react';
import { useNavigate } from 'react-router-dom';
import type { Folder, Source } from '@/api/types';
import { ResizeHandle } from '@/components/layout/ResizeHandle';
import { EditSourceDialog } from '@/components/sources/EditSourceDialog';
import { FolderEditDialog } from '@/components/sources/FolderEditDialog';
import { WatchSourceDialog } from '@/components/sources/WatchSourceDialog';
import {
  ConfirmDialog,
  ContextMenu,
  Icon,
  IconButton,
  Button,
  Field,
  Modal,
  SearchField,
  SourceIcon,
  Wordmark,
  type ContextMenuItem,
} from '@/components/ui';
import type { SavedView } from '@/api/types';
import { useReaderStore } from '@/stores/readerStore';
import { useSourcesStore } from '@/stores/sourcesStore';
import { DEFAULT_RAIL_WIDTH, RAIL_WIDTH_RANGE } from '@/stores/defaults';
import { useSettingsStore } from '@/stores/settingsStore';
import { useUiStore } from '@/stores/uiStore';
import { flattenFolders, folderSubtreeIds } from '@/utils/folders';
import { readerTitle } from '@/utils/readerLabels';
import { cx } from '@/utils/cx';

interface SidebarProps {
  inDrawer?: boolean;
  onCloseDrawer?: () => void;
}

type TreeTarget = { kind: 'source'; id: number } | { kind: 'folder'; id: number };
type DropPosition = 'inside' | 'before' | 'after';
type DropTarget = { kind: 'folder' | 'source' | 'orphan' | 'root'; id: number; position: DropPosition };
type MenuState = { target: TreeTarget; x: number; y: number; view: 'main' | 'move' };
type PendingDelete = { kind: 'source' | 'folder'; id: number; name: string; itemCount: number };

const sortFolders = (folders: Folder[]) => folders.sort((a, b) => a.sort - b.sort || a.id - b.id);
const COLLAPSED_KEY = 'rill.rail.collapsed.v1';

const loadCollapsed = (): Set<number> => {
  try {
    const raw = window.localStorage.getItem(COLLAPSED_KEY);
    if (!raw) return new Set();
    const value = JSON.parse(raw) as unknown;
    if (!Array.isArray(value)) return new Set();
    return new Set(value.filter((entry): entry is number => typeof entry === 'number' && entry > 0));
  } catch {
    return new Set();
  }
};

export const Sidebar = ({ inDrawer = false, onCloseDrawer }: SidebarProps) => {
  const navigate = useNavigate();
  const stats = useReaderStore((state) => state.stats);
  const filter = useReaderStore((state) => state.filter);
  const setFilter = useReaderStore((state) => state.setFilter);
  const setQuery = useReaderStore((state) => state.setQuery);
  const setReadFilter = useReaderStore((state) => state.setReadFilter);
  const markAllRead = useReaderStore((state) => state.markAllRead);
  const sources = useSourcesStore((state) => state.sources);
  const folders = useSourcesStore((state) => state.folders);
  const remove = useSourcesStore((state) => state.remove);
  const removeFolder = useSourcesStore((state) => state.removeFolder);
  const moveSource = useSourcesStore((state) => state.moveSource);
  const moveFolder = useSourcesStore((state) => state.moveFolder);
  const batchMoveFolders = useSourcesStore((state) => state.batchMoveFolders);
  const batchUpdate = useSourcesStore((state) => state.batchUpdate);
  const batchRemove = useSourcesStore((state) => state.batchRemove);
  const batchRefresh = useSourcesStore((state) => state.batchRefresh);
  const batchRemoveFolders = useSourcesStore((state) => state.batchRemoveFolders);
  const exportOpml = useSourcesStore((state) => state.exportOpml);
  const importOpml = useSourcesStore((state) => state.importOpml);
  const setTheme = useSettingsStore((state) => state.setTheme);
  const updateSettings = useSettingsStore((state) => state.update);
  const settings = useSettingsStore((state) => state.settings);
  const railWidth = useSettingsStore((state) => state.settings.reader.rail_width) || DEFAULT_RAIL_WIDTH;
  const closeDrawer = useUiStore((state) => state.closeDrawer);
  const openSettings = useUiStore((state) => state.openSettings);
  const [collapsed, setCollapsed] = useState<Set<number>>(loadCollapsed);
  const [menu, setMenu] = useState<MenuState | null>(null);
  const [editingSource, setEditingSource] = useState<Source | null>(null);
  const [watchSource, setWatchSource] = useState<Source | null>(null);
  const [editingFolder, setEditingFolder] = useState<Folder | null>(null);
  const [pendingDelete, setPendingDelete] = useState<PendingDelete | null>(null);
  const [deleteBusy, setDeleteBusy] = useState(false);
  const [dragging, setDragging] = useState<TreeTarget | null>(null);
  const [dropTarget, setDropTarget] = useState<DropTarget | null>(null);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [lastSelected, setLastSelected] = useState<string | null>(null);
  const [batchConfirmOpen, setBatchConfirmOpen] = useState(false);
  const [batchBusy, setBatchBusy] = useState(false);
  const [searchOpen, setSearchOpen] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');
  const [failOpen, setFailOpen] = useState(false);
  const [viewOpen, setViewOpen] = useState(false);
  const [viewName, setViewName] = useState('');
  const [opmlFolder, setOpmlFolder] = useState<number | null>(null);
  const opmlRef = useRef<HTMLInputElement>(null);
  const dragRef = useRef<TreeTarget | null>(null);
  const longPressRef = useRef<{ timer: number; x: number; y: number } | null>(null);
  const suppressClickRef = useRef(false);

  const navItems = [
    { key: 'inbox' as const, label: '全部', icon: 'inbox' as const, count: stats.total },
    { key: 'starred' as const, label: '收藏', icon: 'star' as const, count: stats.starred },
    { key: 'later' as const, label: '稍后读', icon: 'bookmark' as const, count: stats.later },
  ];
  const views = settings.reader.views ?? [];
  const effectiveTheme = document.documentElement.dataset.theme === 'dark' ? 'dark' : 'light';
  const folderNodes = flattenFolders(folders);
  const folderById = new Map(folders.map((folder) => [folder.id, folder]));
  const pinnedIds = settings.reader.pinned_sources ?? [];
  const pinnedSet = new Set(pinnedIds);
  const pinnedSources = pinnedIds
    .map((id) => sources.find((source) => source.id === id))
    .filter((source): source is Source => Boolean(source));
  const unpinnedSources = sources.filter((source) => !pinnedSet.has(source.id));
  const orphans = unpinnedSources.filter((source) => source.folder_id == null || !folderById.has(source.folder_id));
  const query = searchQuery.trim().toLowerCase();
  const sourceMatches = (source: Source) => !query
    || source.name.toLowerCase().includes(query)
    || source.feed_url.toLowerCase().includes(query);
  const folderNameMatches = (folder: Folder) => folder.name.toLowerCase().includes(query);
  const childrenByParent = new Map<number | null, Folder[]>();
  for (const folder of folders) {
    const parent = folder.parent_id ?? null;
    const rows = childrenByParent.get(parent) ?? [];
    rows.push(folder);
    childrenByParent.set(parent, rows);
  }
  const folderMatchCache = new Map<number, boolean>();
  const folderMatches = (folder: Folder): boolean => {
    if (!query) return true;
    if (folderMatchCache.has(folder.id)) return folderMatchCache.get(folder.id)!;
    folderMatchCache.set(folder.id, false);
    const direct = unpinnedSources.some((source) => source.folder_id === folder.id && sourceMatches(source));
    const nested = (childrenByParent.get(folder.id) ?? []).some((child) => folderMatches(child));
    const result = folderNameMatches(folder) || direct || nested;
    folderMatchCache.set(folder.id, result);
    return result;
  };
  const folderStatsCache = new Map<number, { unread: number; count: number; errors: number }>();
  const folderStats = (folderId: number): { unread: number; count: number; errors: number } => {
    const cached = folderStatsCache.get(folderId);
    if (cached) return cached;
    const rows = unpinnedSources.filter((source) => source.folder_id === folderId);
    const result = {
      unread: rows.reduce((sum, source) => sum + (source.kind === 'wechat' ? 0 : source.unread_count), 0),
      count: rows.length,
      errors: rows.filter((source) => source.enabled && source.last_status === 'error').length,
    };
    for (const child of childrenByParent.get(folderId) ?? []) {
      const childStats = folderStats(child.id);
      result.unread += childStats.unread;
      result.count += childStats.count;
      result.errors += childStats.errors;
    }
    folderStatsCache.set(folderId, result);
    return result;
  };
  const hiddenByCollapsedAncestor = (folder: Folder): boolean => {
    let parent = folder.parent_id;
    while (parent != null) {
      if (collapsed.has(parent)) return true;
      parent = folderById.get(parent)?.parent_id ?? null;
    }
    return false;
  };
  const visibleFolderNodes = folderNodes.filter(({ folder }) => (
    query ? folderMatches(folder) : !hiddenByCollapsedAncestor(folder)
  ));
  const rowsForFolder = (folder: Folder): Source[] => {
    const rows = unpinnedSources.filter((source) => source.folder_id === folder.id);
    if (!query || folderNameMatches(folder)) return rows;
    return rows.filter(sourceMatches);
  };
  const visiblePinned = query ? pinnedSources.filter(sourceMatches) : pinnedSources;
  const visibleOrphans = query ? orphans.filter(sourceMatches) : orphans;
  const attentionSources = sources.filter((source) => source.enabled && source.last_status === 'error');
  const selectedSources = sources.filter((source) => selected.has(`source:${source.id}`));
  const selectedFolders = folders.filter((folder) => selected.has(`folder:${folder.id}`));
  const treeEntries: Array<{ key: string; kind: 'source' | 'folder'; id: number }> = [];
  for (const source of visiblePinned) {
    treeEntries.push({ key: `source:${source.id}`, kind: 'source', id: source.id });
  }
  for (const { folder } of visibleFolderNodes) {
    treeEntries.push({ key: `folder:${folder.id}`, kind: 'folder', id: folder.id });
    if (query || !collapsed.has(folder.id)) {
      for (const source of rowsForFolder(folder)) {
        treeEntries.push({ key: `source:${source.id}`, kind: 'source', id: source.id });
      }
    }
  }
  for (const source of visibleOrphans) {
    treeEntries.push({ key: `source:${source.id}`, kind: 'source', id: source.id });
  }

  useEffect(() => {
    try {
      window.localStorage.setItem(COLLAPSED_KEY, JSON.stringify([...collapsed]));
    } catch {
      /* ignore storage errors */
    }
  }, [collapsed]);

  const closeIfDrawer = () => {
    closeDrawer();
    onCloseDrawer?.();
  };

  const goFilter = (next: Parameters<typeof setFilter>[0]) => {
    setFilter(next);
    navigate('/');
    closeIfDrawer();
  };

  const toggleFolder = (id: number) => {
    setCollapsed((current) => {
      const next = new Set(current);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  const expandFolder = (id: number) => {
    setCollapsed((current) => {
      if (!current.has(id)) return current;
      const next = new Set(current);
      next.delete(id);
      return next;
    });
  };

  const togglePin = (sourceId: number) => {
    const next = pinnedSet.has(sourceId)
      ? pinnedIds.filter((id) => id !== sourceId)
      : [...pinnedIds, sourceId];
    updateSettings({ reader: { pinned_sources: next } });
  };

  const clearSelection = () => {
    setSelected(new Set());
    setLastSelected(null);
    setBatchConfirmOpen(false);
  };

  const toggleSelection = (key: string) => {
    setSelected((current) => {
      const next = new Set(current);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
    setLastSelected(key);
  };

  const selectRange = (key: string) => {
    if (!lastSelected) {
      toggleSelection(key);
      return;
    }
    const from = treeEntries.findIndex((entry) => entry.key === lastSelected);
    const to = treeEntries.findIndex((entry) => entry.key === key);
    if (from < 0 || to < 0) {
      toggleSelection(key);
      return;
    }
    const start = Math.min(from, to);
    const end = Math.max(from, to);
    setSelected((current) => {
      const next = new Set(current);
      for (const entry of treeEntries.slice(start, end + 1)) next.add(entry.key);
      return next;
    });
  };

  // 返回 true 表示这次点击已被选择逻辑消费，不应再导航。
  const handleSelectionClick = (event: ReactMouseEvent<HTMLElement>, key: string): boolean => {
    if (consumeLongPressClick()) return true;
    if (event.metaKey || event.ctrlKey) {
      event.preventDefault();
      toggleSelection(key);
      return true;
    }
    if (event.shiftKey) {
      event.preventDefault();
      selectRange(key);
      return true;
    }
    if (selected.size > 0) {
      clearSelection();
    }
    return false;
  };

  const applyView = (view: SavedView) => {
    let next: Parameters<typeof setFilter>[0] = { type: 'inbox' };
    if (view.type === 'source' && view.source_id) {
      if (!sources.some((source) => source.id === view.source_id)) {
        next = { type: 'inbox' };
      } else {
        next = { type: 'source', sourceId: view.source_id };
      }
    } else if (view.type === 'folder' && view.folder_id) {
      if (!folders.some((folder) => folder.id === view.folder_id)) {
        next = { type: 'inbox' };
      } else {
        next = { type: 'folder', folderId: view.folder_id };
      }
    } else if (view.type === 'starred' || view.type === 'later' || view.type === 'read' || view.type === 'unread') {
      next = { type: view.type };
    }
    setFilter(next);
    setQuery(view.query ?? '');
    setReadFilter(view.read_filter ?? 'all');
    navigate('/');
    closeIfDrawer();
  };

  const saveView = () => {
    const name = viewName.trim();
    if (!name) return;
    const view: SavedView = {
      id: window.crypto?.randomUUID?.() ?? `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`,
      name,
      type: filter.type,
      source_id: filter.type === 'source' ? filter.sourceId : null,
      folder_id: filter.type === 'folder' ? filter.folderId : null,
      query: searchQuery.trim() || undefined,
      read_filter: useReaderStore.getState().readFilter,
    };
    updateSettings({ reader: { views: [...views, view] } });
    setViewOpen(false);
    setViewName('');
    useUiStore.getState().notify(`已保存视图「${name}」`);
  };

  const removeView = (id: string) => {
    updateSettings({ reader: { views: views.filter((view) => view.id !== id) } });
    useUiStore.getState().notify('已删除视图');
  };

  const batchMoveSources = async (value: string) => {
    if (!value || selectedSources.length === 0) return;
    setBatchBusy(true);
    await batchUpdate(selectedSources.map((source) => source.id), { folder_id: Number(value) });
    setBatchBusy(false);
    clearSelection();
  };

  const batchToggleSources = async (enabled: boolean) => {
    if (selectedSources.length === 0) return;
    setBatchBusy(true);
    await batchUpdate(selectedSources.map((source) => source.id), { enabled });
    setBatchBusy(false);
  };

  const batchMoveFolderSelection = async (value: string) => {
    if (!value || selectedFolders.length === 0) return;
    setBatchBusy(true);
    await batchMoveFolders(selectedFolders.map((folder) => folder.id), value === '0' ? null : Number(value));
    setBatchBusy(false);
    clearSelection();
  };

  const batchRefreshSources = async () => {
    if (selectedSources.length === 0) return;
    setBatchBusy(true);
    await batchRefresh(selectedSources.map((source) => source.id));
    setBatchBusy(false);
  };

  const batchDelete = async () => {
    if (batchBusy) return;
    setBatchBusy(true);
    if (selectedSources.length > 0) {
      await batchRemove(selectedSources.map((source) => source.id));
    }
    if (selectedFolders.length > 0) {
      await batchRemoveFolders(selectedFolders.map((folder) => folder.id));
    }
    setBatchBusy(false);
    setBatchConfirmOpen(false);
    clearSelection();
  };

  const exportFolderOpml = async (folder: Folder) => {
    const text = await exportOpml(folder.id);
    if (!text) return;
    const blob = new Blob([text], { type: 'text/x-opml;charset=utf-8' });
    const url = URL.createObjectURL(blob);
    const anchor = document.createElement('a');
    anchor.href = url;
    anchor.download = `${folder.name || 'rill-group'}.opml`;
    anchor.click();
    window.setTimeout(() => URL.revokeObjectURL(url), 1000);
    useUiStore.getState().notify(`已导出「${folder.name}」`);
  };

  const importFolderOpml = async (file: File | undefined) => {
    if (!file || opmlFolder == null) return;
    await importOpml(file, opmlFolder);
    if (opmlRef.current) opmlRef.current.value = '';
    setOpmlFolder(null);
  };

  const clearLongPress = () => {
    if (!longPressRef.current) return;
    window.clearTimeout(longPressRef.current.timer);
    longPressRef.current = null;
  };

  useEffect(() => clearLongPress, []);

  const consumeLongPressClick = () => {
    if (!suppressClickRef.current) return false;
    suppressClickRef.current = false;
    return true;
  };

  const openMenuAt = (target: TreeTarget, x: number, y: number) => {
    setMenu({ target, x, y, view: 'main' });
  };

  const handleContextMenu = (event: ReactMouseEvent<HTMLElement>, target: TreeTarget) => {
    event.preventDefault();
    event.stopPropagation();
    openMenuAt(target, event.clientX, event.clientY);
  };

  const handleMenuKey = (event: ReactKeyboardEvent<HTMLElement>, target: TreeTarget) => {
    if (event.key !== 'ContextMenu' && !(event.shiftKey && event.key === 'F10')) return;
    event.preventDefault();
    const rect = event.currentTarget.getBoundingClientRect();
    openMenuAt(target, rect.left + 12, rect.top + rect.height);
  };

  const startLongPress = (event: ReactPointerEvent<HTMLElement>, target: TreeTarget) => {
    if (event.pointerType === 'mouse') return;
    clearLongPress();
    const startX = event.clientX;
    const startY = event.clientY;
    longPressRef.current = {
      x: startX,
      y: startY,
      timer: window.setTimeout(() => {
        longPressRef.current = null;
        suppressClickRef.current = true;
        openMenuAt(target, startX, startY);
        window.setTimeout(() => { suppressClickRef.current = false; }, 1200);
      }, 520),
    };
  };

  const moveLongPress = (event: ReactPointerEvent<HTMLElement>) => {
    const current = longPressRef.current;
    if (!current) return;
    if (Math.hypot(event.clientX - current.x, event.clientY - current.y) > 8) clearLongPress();
  };

  const beginDrag = (event: DragEvent<HTMLElement>, target: TreeTarget) => {
    dragRef.current = target;
    setDragging(target);
    setDropTarget(null);
    event.dataTransfer.effectAllowed = 'move';
    event.dataTransfer.setData('text/plain', `${target.kind}:${target.id}`);
  };

  const endDrag = () => {
    dragRef.current = null;
    setDragging(null);
    setDropTarget(null);
  };

  const clearDropIfLeft = (event: DragEvent<HTMLElement>) => {
    if (!event.currentTarget.contains(event.relatedTarget as Node | null)) setDropTarget(null);
  };

  const dropPosition = (event: DragEvent<HTMLElement>, target: TreeTarget): DropPosition => {
    if (target.kind === 'folder' && dragRef.current?.kind === 'source') return 'inside';
    const rect = event.currentTarget.getBoundingClientRect();
    const ratio = (event.clientY - rect.top) / Math.max(1, rect.height);
    if (ratio < 0.28) return 'before';
    if (ratio > 0.72) return 'after';
    return 'inside';
  };

  const handleFolderDragOver = (event: DragEvent<HTMLElement>, folder: Folder) => {
    const payload = dragRef.current;
    if (!payload) return;
    if (payload.kind === 'folder' && folderSubtreeIds(folders, payload.id).has(folder.id)) return;
    const position = dropPosition(event, { kind: 'folder', id: folder.id });
    event.preventDefault();
    event.stopPropagation();
    event.dataTransfer.dropEffect = 'move';
    setDropTarget({ kind: 'folder', id: folder.id, position });
    if (position === 'inside' && collapsed.has(folder.id)) expandFolder(folder.id);
  };

  const handleFolderDrop = async (event: DragEvent<HTMLElement>, folder: Folder) => {
    event.preventDefault();
    event.stopPropagation();
    const payload = dragRef.current;
    if (!payload) {
      endDrag();
      return;
    }
    if (payload.kind === 'source') {
      endDrag();
      await moveSource(payload.id, folder.id);
      return;
    }
    if (folderSubtreeIds(folders, payload.id).has(folder.id)) {
      endDrag();
      return;
    }
    const position = dropPosition(event, { kind: 'folder', id: folder.id });
    endDrag();
    if (position === 'inside') {
      await moveFolder(payload.id, folder.id, null);
      expandFolder(folder.id);
      return;
    }
    const parentId = folder.parent_id ?? null;
    const siblings = sortFolders(
      folders.filter((entry) => (entry.parent_id ?? null) === parentId && entry.id !== payload.id),
    );
    const index = siblings.findIndex((entry) => entry.id === folder.id);
    const beforeId = position === 'before' ? folder.id : (siblings[index + 1]?.id ?? null);
    await moveFolder(payload.id, parentId, beforeId);
  };

  const handleSourceDragOver = (event: DragEvent<HTMLElement>, source: Source) => {
    if (dragRef.current?.kind !== 'source' || dragRef.current.id === source.id) return;
    event.preventDefault();
    event.stopPropagation();
    event.dataTransfer.dropEffect = 'move';
    setDropTarget({ kind: 'source', id: source.id, position: 'inside' });
  };

  const handleSourceDrop = async (event: DragEvent<HTMLElement>, source: Source) => {
    event.preventDefault();
    event.stopPropagation();
    const payload = dragRef.current;
    endDrag();
    if (payload?.kind !== 'source' || payload.id === source.id) return;
    await moveSource(payload.id, source.folder_id ?? null);
  };

  const handleOrphanDragOver = (event: DragEvent<HTMLElement>) => {
    if (dragRef.current?.kind !== 'source') return;
    event.preventDefault();
    event.stopPropagation();
    event.dataTransfer.dropEffect = 'move';
    setDropTarget({ kind: 'orphan', id: 0, position: 'inside' });
  };

  const handleOrphanDrop = async (event: DragEvent<HTMLElement>) => {
    event.preventDefault();
    event.stopPropagation();
    const payload = dragRef.current;
    endDrag();
    if (payload?.kind === 'source') await moveSource(payload.id, null);
  };

  const handleRootDragOver = (event: DragEvent<HTMLElement>) => {
    if (!dragRef.current) return;
    event.preventDefault();
    event.stopPropagation();
    event.dataTransfer.dropEffect = 'move';
    setDropTarget({ kind: 'root', id: 0, position: 'inside' });
  };

  const handleRootDrop = async (event: DragEvent<HTMLElement>) => {
    event.preventDefault();
    event.stopPropagation();
    const payload = dragRef.current;
    endDrag();
    if (payload?.kind === 'source') await moveSource(payload.id, null);
    if (payload?.kind === 'folder') await moveFolder(payload.id, null, null);
  };

  const askDeleteSource = (source: Source) => {
    setPendingDelete({ kind: 'source', id: source.id, name: source.name, itemCount: source.item_count });
  };

  const askDeleteFolder = (folder: Folder) => {
    setPendingDelete({ kind: 'folder', id: folder.id, name: folder.name, itemCount: folder.source_count });
  };

  const runDelete = async () => {
    if (!pendingDelete || deleteBusy) return;
    const target = pendingDelete;
    setDeleteBusy(true);
    if (target.kind === 'source') {
      await remove(target.id);
      if (filter.type === 'source' && filter.sourceId === target.id) goFilter({ type: 'inbox' });
    } else {
      await removeFolder(target.id);
      if (filter.type === 'folder' && filter.folderId === target.id) goFilter({ type: 'inbox' });
    }
    setDeleteBusy(false);
    setPendingDelete(null);
  };

  const renderSource = (source: Source, depth = 0) => {
    const target: TreeTarget = { kind: 'source', id: source.id };
    const key = `source:${source.id}`;
    const isSelected = selected.has(key);
    const isDropTarget = dropTarget?.kind === 'source' && dropTarget.id === source.id;
    return (
      <button
        type="button"
        key={source.id}
        className={cx(
          'source-row',
          filter.type === 'source' && filter.sourceId === source.id && 'is-active',
          !source.enabled && 'is-paused',
          isSelected && 'is-multi-selected',
          dragging?.kind === 'source' && dragging.id === source.id && 'is-dragging',
          isDropTarget && 'is-drop-target',
        )}
        style={{ paddingLeft: 25 + depth * 12 }}
        draggable
        aria-selected={isSelected || undefined}
        onClick={(event) => {
          if (handleSelectionClick(event, key)) return;
          goFilter({ type: 'source', sourceId: source.id });
        }}
        onContextMenu={(event) => handleContextMenu(event, target)}
        onKeyDown={(event) => handleMenuKey(event, target)}
        onPointerDown={(event) => startLongPress(event, target)}
        onPointerMove={moveLongPress}
        onPointerUp={clearLongPress}
        onPointerCancel={clearLongPress}
        onDragStart={(event) => beginDrag(event, target)}
        onDragEnd={endDrag}
        onDragOver={(event) => handleSourceDragOver(event, source)}
        onDragLeave={clearDropIfLeft}
        onDrop={(event) => { void handleSourceDrop(event, source); }}
      >
        <SourceIcon source={source} size={17} />
        <span className="source-name">{source.name}</span>
        {source.watch_enabled && source.unread_count > 0 ? (
          <span className="source-watch-dot" title="关注源有新内容" aria-label="关注源有新内容" />
        ) : null}
        {(() => {
          const count = source.kind === 'wechat' ? source.item_count : source.unread_count;
          return count > 0 ? (
            <span className={cx('source-count', source.kind === 'wechat' && 'is-total')}>{count}</span>
          ) : null;
        })()}
      </button>
    );
  };

  const toggleTheme = () => {
    setTheme(effectiveTheme === 'dark' ? 'light' : 'dark');
  };

  const showSettings = () => {
    openSettings();
    closeIfDrawer();
  };

  const menuSource = menu?.target.kind === 'source'
    ? sources.find((source) => source.id === menu.target.id) ?? null
    : null;
  const menuFolder = menu?.target.kind === 'folder'
    ? folderById.get(menu.target.id) ?? null
    : null;
  let menuItems: ContextMenuItem[] = [];
  let menuTitle: string | undefined;
  if (menu && menuSource) {
    if (menu.view === 'move') {
      menuTitle = '移动到分组';
      menuItems = [
        { id: 'back', label: '返回', icon: 'chevron-left', keepOpen: true, onSelect: () => setMenu((current) => current ? { ...current, view: 'main' } : current) },
        {
          id: 'ungrouped',
          label: '未分组',
          checked: menuSource.folder_id == null,
          separator: true,
          onSelect: () => { void moveSource(menuSource.id, null); },
        },
        ...folderNodes.map(({ folder, depth }) => ({
          id: `move-folder-${folder.id}`,
          label: folder.name,
          checked: menuSource.folder_id === folder.id,
          indent: depth,
          onSelect: () => { void moveSource(menuSource.id, folder.id); },
        })),
      ];
    } else {
      menuItems = [
        { id: 'open', label: '打开', icon: 'inbox', onSelect: () => goFilter({ type: 'source', sourceId: menuSource.id }) },
        {
          id: 'pin',
          label: pinnedSet.has(menuSource.id) ? '取消置顶' : '置顶',
          icon: 'pin',
          onSelect: () => togglePin(menuSource.id),
        },
        { id: 'edit', label: '编辑订阅源', icon: 'pencil', onSelect: () => setEditingSource(menuSource) },
        {
          id: 'watch',
          label: menuSource.watch_enabled ? '关注设置' : '关注订阅源',
          icon: 'bell',
          onSelect: () => setWatchSource(menuSource),
        },
        { id: 'refresh', label: '立即刷新', icon: 'refresh', onSelect: () => { void useSourcesStore.getState().refresh(menuSource.id); } },
        {
          id: 'toggle',
          label: menuSource.enabled ? '暂停订阅' : '启用订阅',
          icon: menuSource.enabled ? 'pause' : 'play',
          onSelect: () => { void useSourcesStore.getState().toggleEnabled(menuSource.id); },
        },
        {
          id: 'move',
          label: '移动到分组…',
          icon: 'folder',
          separator: true,
          keepOpen: true,
          onSelect: () => setMenu((current) => current ? { ...current, view: 'move' } : current),
        },
        {
          id: 'delete',
          label: '删除订阅源',
          icon: 'trash',
          danger: true,
          separator: true,
          onSelect: () => askDeleteSource(menuSource),
        },
      ];
    }
  } else if (menu && menuFolder) {
    menuItems = [
      {
        id: 'open-folder',
        label: '打开分组',
        icon: 'folder',
        onSelect: () => goFilter({ type: 'folder', folderId: menuFolder.id }),
      },
      { id: 'edit-folder', label: '编辑分组', icon: 'pencil', onSelect: () => setEditingFolder(menuFolder) },
      {
        id: 'folder-read',
        label: `全部标为已读（${folderStats(menuFolder.id).unread}）`,
        icon: 'check',
        disabled: folderStats(menuFolder.id).unread === 0,
        onSelect: () => { void markAllRead({ folder_id: menuFolder.id }); },
      },
      {
        id: 'folder-export',
        label: '导出此分组 OPML',
        icon: 'download',
        onSelect: () => { void exportFolderOpml(menuFolder); },
      },
      {
        id: 'folder-import',
        label: '导入 OPML 到此分组',
        icon: 'upload',
        onSelect: () => {
          setOpmlFolder(menuFolder.id);
          window.setTimeout(() => opmlRef.current?.click(), 0);
        },
      },
      {
        id: 'settings',
        label: '打开订阅源设置',
        icon: 'settings-sources',
        onSelect: () => { openSettings('sources'); closeIfDrawer(); },
      },
      {
        id: 'delete-folder',
        label: '删除分组',
        icon: 'trash',
        danger: true,
        separator: true,
        onSelect: () => askDeleteFolder(menuFolder),
      },
    ];
  }

  return (
    <aside className="rail">
      <div className="rail-brand">
        <div className="rail-brand-top">
          <Wordmark />
          {inDrawer ? <IconButton icon="x" label="关闭导航" onClick={closeIfDrawer} /> : null}
        </div>
      </div>

      <nav className="rail-nav" aria-label="阅读筛选">
        {navItems.map((item) => (
          <button
            type="button"
            key={item.key}
            className={cx('rail-nav-item', filter.type === item.key && 'is-active')}
            onClick={() => goFilter({ type: item.key })}
          >
            <Icon name={item.icon} size={17} filled={item.key === 'starred' && filter.type === 'starred'} />
            <span className="rail-nav-label">{item.label}</span>
            <span className="rail-nav-count">{item.count}</span>
          </button>
        ))}
      </nav>

      <div className="rail-section-head">
        <span className="rail-section-title">视图</span>
        <IconButton
          icon="plus"
          label="保存当前视图"
          size={14}
          onClick={() => {
            setViewName(readerTitle(filter, sources, folders));
            setViewOpen(true);
          }}
        />
      </div>
      {views.length > 0 ? (
        <div className="rail-views" aria-label="保存的视图">
          {views.map((view) => (
            <div key={view.id} className="rail-view-row">
              <button type="button" className="rail-view-name" onClick={() => applyView(view)}>
                <Icon name="eye" size={14} />
                <span>{view.name}</span>
              </button>
              <button
                type="button"
                className="rail-view-remove"
                aria-label={`删除视图 ${view.name}`}
                onClick={() => removeView(view.id)}
              >
                <Icon name="x" size={12} />
              </button>
            </div>
          ))}
        </div>
      ) : null}

      <div className="rail-section-head">
        <span className="rail-section-title">订阅源</span>
        <div className="rail-section-tools">
          <IconButton
            icon="search"
            label={searchOpen ? '关闭订阅源搜索' : '搜索订阅源'}
            size={14}
            active={searchOpen}
            onClick={() => {
              setSearchOpen((value) => !value);
              if (searchOpen) setSearchQuery('');
            }}
          />
          <span className="rail-source-total">{sources.length}</span>
        </div>
      </div>
      {searchOpen ? (
        <div className="rail-search">
          <SearchField
            value={searchQuery}
            onChange={setSearchQuery}
            placeholder="搜索源或地址"
            ariaLabel="搜索订阅源"
            autoFocus
          />
        </div>
      ) : null}
      {attentionSources.length > 0 ? (
        <button type="button" className="rail-attention" onClick={() => setFailOpen(true)}>
          <Icon name="alert-triangle" size={14} />
          <span>{attentionSources.length} 个源需关注</span>
          <Icon name="chevron-right" size={13} />
        </button>
      ) : null}
      {selected.size > 0 ? (
        <div className="rail-batch" role="toolbar" aria-label="批量操作">
          <div className="rail-batch-head">
            <span>
              已选 {selectedSources.length} 源
              {selectedFolders.length > 0 ? ` · ${selectedFolders.length} 分组` : ''}
            </span>
            <button type="button" className="rail-batch-clear" onClick={clearSelection}>取消</button>
          </div>
          {selectedSources.length > 0 ? (
            <div className="rail-batch-actions">
              <select
                value=""
                aria-label="批量移动到分组"
                disabled={batchBusy}
                onChange={(event) => {
                  const value = event.target.value;
                  event.target.value = '';
                  void batchMoveSources(value);
                }}
              >
                <option value="" disabled>移动到分组…</option>
                <option value="0">未分组</option>
                {folderNodes.map((node) => (
                  <option key={node.folder.id} value={node.folder.id}>{`${'　'.repeat(node.depth)}${node.folder.name}`}</option>
                ))}
              </select>
              <button type="button" disabled={batchBusy} onClick={() => { void batchToggleSources(true); }}>启用</button>
              <button type="button" disabled={batchBusy} onClick={() => { void batchToggleSources(false); }}>暂停</button>
              <button type="button" disabled={batchBusy} onClick={() => { void batchRefreshSources(); }}>刷新</button>
            </div>
          ) : null}
          {selectedFolders.length > 0 ? (
            <div className="rail-batch-actions">
              <select
                value=""
                aria-label="批量移动分组"
                disabled={batchBusy}
                onChange={(event) => {
                  const value = event.target.value;
                  event.target.value = '';
                  void batchMoveFolderSelection(value);
                }}
              >
                <option value="" disabled>分组移动到…</option>
                <option value="0">根目录</option>
                {folderNodes
                  .filter((node) => !selected.has(`folder:${node.folder.id}`) && !selectedFolders.some((folder) => folderSubtreeIds(folders, folder.id).has(node.folder.id)))
                  .map((node) => (
                    <option key={node.folder.id} value={node.folder.id}>{`${'　'.repeat(node.depth)}${node.folder.name}`}</option>
                  ))}
              </select>
            </div>
          ) : null}
          <div className="rail-batch-actions">
            <button type="button" className="is-danger" disabled={batchBusy} onClick={() => setBatchConfirmOpen(true)}>
              删除所选
            </button>
          </div>
        </div>
      ) : null}
      <div className="rail-tree" aria-label="订阅源分组">
        {visiblePinned.length > 0 ? (
          <div className="folder-block rail-pinned">
            <div className="folder-row">
              <span className="folder-chevron" aria-hidden="true" />
              <span className="folder-name is-static">
                <Icon name="pin" size={12} />
                置顶
              </span>
              <span className="folder-source-count">{visiblePinned.length}</span>
            </div>
            <div className="folder-sources">
              {visiblePinned.map((source) => renderSource(source))}
            </div>
          </div>
        ) : null}
        {visibleFolderNodes.map(({ folder, depth }, index) => {
          const rows = rowsForFolder(folder);
          const isCollapsed = !query && collapsed.has(folder.id);
          const target: TreeTarget = { kind: 'folder', id: folder.id };
          const key = `folder:${folder.id}`;
          const isSelected = selected.has(key);
          const isDrop = dropTarget?.kind === 'folder' && dropTarget.id === folder.id;
          const metrics = folderStats(folder.id);
          return (
            <div key={folder.id} className="folder-block">
              <div
                className={cx(
                  'folder-row',
                  filter.type === 'folder' && filter.folderId === folder.id && 'is-active',
                  isSelected && 'is-multi-selected',
                  dragging?.kind === 'folder' && dragging.id === folder.id && 'is-dragging',
                  isDrop && `is-drop-${dropTarget.position}`,
                )}
                style={{ paddingLeft: depth * 12 }}
                draggable
                aria-selected={isSelected || undefined}
                onContextMenu={(event) => handleContextMenu(event, target)}
                onKeyDown={(event) => handleMenuKey(event, target)}
                onPointerDown={(event) => startLongPress(event, target)}
                onPointerMove={moveLongPress}
                onPointerUp={clearLongPress}
                onPointerCancel={clearLongPress}
                onDragStart={(event) => beginDrag(event, target)}
                onDragEnd={endDrag}
                onDragOver={(event) => handleFolderDragOver(event, folder)}
                onDragLeave={clearDropIfLeft}
                onDrop={(event) => { void handleFolderDrop(event, folder); }}
              >
                <button
                  type="button"
                  className={cx('folder-chevron', isCollapsed && 'is-collapsed')}
                  onClick={() => {
                    if (consumeLongPressClick()) return;
                    toggleFolder(folder.id);
                  }}
                  aria-label={isCollapsed ? `展开分组 ${folder.name}` : `收起分组 ${folder.name}`}
                  aria-expanded={!isCollapsed}
                >
                  <Icon name="chevron-down" size={12} />
                </button>
                <button
                  type="button"
                  className="folder-name"
                  onClick={(event) => {
                    if (handleSelectionClick(event, key)) return;
                    goFilter({ type: 'folder', folderId: folder.id });
                  }}
                >
                  <span className="folder-index">{String(index + 1).padStart(2, '0')}</span>
                  {folder.name}
                </button>
                <span
                  className={cx('folder-metrics', metrics.errors > 0 && 'has-error')}
                  title={`${metrics.unread} 条未读 · ${metrics.count} 个源${metrics.errors > 0 ? ` · ${metrics.errors} 个抓取失败` : ''}`}
                >
                  {metrics.errors > 0 ? <i className="folder-alert" aria-hidden="true" /> : null}
                  <span className="folder-unread">{metrics.unread}</span>
                  <span className="folder-source-count">{metrics.count}</span>
                </span>
              </div>
              {!isCollapsed && rows.length > 0 ? (
                <div className="folder-sources">{rows.map((source) => renderSource(source, depth))}</div>
              ) : null}
            </div>
          );
        })}

        {visibleOrphans.length > 0 ? (
          <div className="folder-block">
            <div
              className={cx('folder-row', dropTarget?.kind === 'orphan' && 'is-drop-inside')}
              onDragOver={handleOrphanDragOver}
              onDragLeave={clearDropIfLeft}
              onDrop={(event) => { void handleOrphanDrop(event); }}
            >
              <span className="folder-chevron" aria-hidden="true" />
              <span className="folder-name is-static">未分组</span>
              <span className="folder-source-count">{visibleOrphans.length}</span>
            </div>
            <div className="folder-sources">{visibleOrphans.map((source) => renderSource(source))}</div>
          </div>
        ) : null}

        {dragging ? (
          <div
            className={cx('rail-drop-root', dropTarget?.kind === 'root' && 'is-drop-target')}
            onDragOver={handleRootDragOver}
            onDragLeave={clearDropIfLeft}
            onDrop={(event) => { void handleRootDrop(event); }}
          >
            <Icon name={dragging.kind === 'source' ? 'inbox' : 'folder'} size={14} />
            <span>{dragging.kind === 'source' ? '拖到此处移出分组' : '拖到此处移到根目录'}</span>
          </div>
        ) : null}

        {sources.length === 0 ? <p className="rail-empty">还没有订阅源</p> : null}
        {sources.length > 0 && visiblePinned.length === 0 && visibleFolderNodes.length === 0 && visibleOrphans.length === 0 ? (
          <p className="rail-empty">没有匹配的订阅源</p>
        ) : null}
      </div>

      <div className="rail-foot">
        <div className="rail-tool-group">
          <button
            type="button"
            className="rail-tool-btn"
            onClick={showSettings}
            aria-label="设置"
            data-tooltip="设置"
          >
            <Icon name="settings" size={17} />
          </button>
          <button
            type="button"
            className="rail-tool-btn"
            onClick={toggleTheme}
            aria-label={effectiveTheme === 'dark' ? '切换到浅色' : '切换到深色'}
            data-tooltip={effectiveTheme === 'dark' ? '切换到浅色' : '切换到深色'}
          >
            <Icon name={effectiveTheme === 'dark' ? 'sun' : 'moon'} size={17} />
          </button>
        </div>
      </div>
      {!inDrawer ? (
        <ResizeHandle
          label="调整左侧栏宽度"
          value={railWidth}
          min={RAIL_WIDTH_RANGE.min}
          max={RAIL_WIDTH_RANGE.max}
          defaultValue={DEFAULT_RAIL_WIDTH}
          cssVar="--rail-width"
          onCommit={(value) => updateSettings({ reader: { rail_width: value } })}
        />
      ) : null}

      {menu ? (
        <ContextMenu
          x={menu.x}
          y={menu.y}
          items={menuItems}
          label={menuTitle ?? (menuSource ? menuSource.name : menuFolder?.name ?? '订阅源菜单')}
          title={menuTitle}
          onClose={() => setMenu(null)}
        />
      ) : null}

      <EditSourceDialog open={Boolean(editingSource)} source={editingSource} onClose={() => setEditingSource(null)} />
      <WatchSourceDialog
        open={Boolean(watchSource)}
        initialSourceId={watchSource?.id ?? null}
        sources={sources}
        onClose={() => setWatchSource(null)}
      />
      <FolderEditDialog open={Boolean(editingFolder)} folder={editingFolder} onClose={() => setEditingFolder(null)} />
      <ConfirmDialog
        open={Boolean(pendingDelete)}
        title={pendingDelete?.kind === 'folder' ? '删除分组？' : '删除订阅源？'}
        confirmLabel={pendingDelete?.kind === 'folder' ? '删除分组' : '删除订阅源'}
        busy={deleteBusy}
        description={pendingDelete?.kind === 'folder'
          ? `「${pendingDelete.name}」内的 ${pendingDelete.itemCount} 个订阅源会移到未分组，子分组会上移。此操作不会删除订阅源。`
          : `「${pendingDelete?.name ?? ''}」及其 ${pendingDelete?.itemCount ?? 0} 篇文章将被删除，无法恢复。`}
        onConfirm={() => { void runDelete(); }}
        onClose={() => setPendingDelete(null)}
      />
      <ConfirmDialog
        open={batchConfirmOpen}
        title="删除所选？"
        confirmLabel="删除所选"
        busy={batchBusy}
        description={(
          <>
            {selectedSources.length > 0 ? `${selectedSources.length} 个订阅源及其文章将被删除，无法恢复。` : ''}
            {selectedFolders.length > 0 ? `${selectedFolders.length} 个分组会被删除，组内订阅源移到未分组，子分组上移。` : ''}
          </>
        )}
        onConfirm={() => { void batchDelete(); }}
        onClose={() => setBatchConfirmOpen(false)}
      />
      <Modal
        open={viewOpen}
        onClose={() => setViewOpen(false)}
        title="保存当前视图"
        subtitle="保存筛选、搜索词与已读状态，之后一键回到这里。"
        footer={(
          <>
            <Button onClick={() => setViewOpen(false)}>取消</Button>
            <Button variant="primary" disabled={!viewName.trim()} onClick={saveView}>保存视图</Button>
          </>
        )}
      >
        <div className="edit-source-form">
          <Field label="视图名称">
            <input
              value={viewName}
              onChange={(event) => setViewName(event.target.value)}
              onKeyDown={(event) => { if (event.key === 'Enter') saveView(); }}
              maxLength={60}
              autoFocus
              aria-label="视图名称"
            />
          </Field>
          <p className="settings-field-hint">当前：{readerTitle(filter, sources, folders)}{searchQuery.trim() ? ` · 搜索「${searchQuery.trim()}」` : ''}</p>
        </div>
      </Modal>
      <input
        ref={opmlRef}
        type="file"
        accept=".opml,.xml,text/xml,application/xml"
        hidden
        onChange={(event) => { void importFolderOpml(event.target.files?.[0]); }}
      />
      <Modal
        open={failOpen}
        onClose={() => setFailOpen(false)}
        title="需要关注的订阅源"
        subtitle={`${attentionSources.length} 个源最近抓取失败`}
        footer={(
          <>
            <Button onClick={() => setFailOpen(false)}>关闭</Button>
            <Button
              variant="primary"
              icon="refresh"
              disabled={attentionSources.length === 0 || batchBusy}
              onClick={() => {
                setBatchBusy(true);
                void batchRefresh(attentionSources.map((source) => source.id)).finally(() => {
                  setBatchBusy(false);
                  setFailOpen(false);
                });
              }}
            >
              全部重试
            </Button>
          </>
        )}
      >
        <div className="rail-fail-list">
          {attentionSources.map((source) => (
            <button
              type="button"
              key={source.id}
              className="rail-fail-row"
              onClick={() => {
                setFailOpen(false);
                goFilter({ type: 'source', sourceId: source.id });
              }}
            >
              <SourceIcon source={source} size={26} />
              <span className="rail-fail-copy">
                <strong>{source.name}</strong>
                <small>{source.last_error || '抓取失败'}</small>
              </span>
              <Icon name="chevron-right" size={14} />
            </button>
          ))}
        </div>
      </Modal>
    </aside>
  );
};
