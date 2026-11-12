import { useEffect, useMemo, useState, type MouseEvent as ReactMouseEvent } from 'react';
import type { ItemSummary } from '@/api/types';
import { ResizeHandle } from '@/components/layout/ResizeHandle';
import { ArticleRow } from '@/components/reader/ArticleRow';
import {
  Button,
  EmptyState,
  IconButton,
  Modal,
  SearchField,
} from '@/components/ui';
import { useInfiniteScroll } from '@/hooks/useInfiniteScroll';
import { useReaderStore } from '@/stores/readerStore';
import { DEFAULT_LIST_WIDTH, LIST_WIDTH_RANGE } from '@/stores/defaults';
import { useSettingsStore } from '@/stores/settingsStore';
import { useSourcesStore } from '@/stores/sourcesStore';
import { readerTitle } from '@/utils/readerLabels';

// 与 readerStore 首页请求页大小一致：骨架条数 = 首屏真实渲染条数。
const ITEM_SKELETON_COUNT = 12;
const MERGE_KEY = 'rill.list.merge_duplicates';

const normalizeTitleKey = (title: string) => title
  .toLowerCase()
  .replace(/[\s\p{P}\p{S}]+/gu, '');

const canonicalKey = (item: ItemSummary): { key: string; byTitle: boolean } | null => {
  const rawURL = item.url?.trim();
  if (rawURL) {
    try {
      const url = new URL(rawURL);
      url.hash = '';
      for (const param of ['utm_source', 'utm_medium', 'utm_campaign', 'utm_term', 'utm_content', 'ref', 'ref_src']) {
        url.searchParams.delete(param);
      }
      return { key: `u:${url.toString().replace(/\/$/, '')}`, byTitle: false };
    } catch {
      /* fall through to title */
    }
  }
  const title = normalizeTitleKey(item.title);
  return title.length >= 12 ? { key: `t:${title}`, byTitle: true } : null;
};

const itemTime = (item: ItemSummary) => {
  const value = item.published_at ?? item.fetched_at;
  const time = Date.parse(value);
  return Number.isFinite(time) ? time : 0;
};

interface ArticleListProps {
  selectedId: number | null;
  onSelect: (item: ItemSummary, event?: ReactMouseEvent<HTMLElement>) => void;
  onOpenFocus: (item: ItemSummary) => void;
  searchOpen: boolean;
  onToggleSearch: () => void;
}

export const ArticleList = ({ selectedId, onSelect, onOpenFocus, searchOpen, onToggleSearch }: ArticleListProps) => {
  const sources = useSourcesStore((state) => state.sources);
  const folders = useSourcesStore((state) => state.folders);
  const filter = useReaderStore((state) => state.filter);
  const query = useReaderStore((state) => state.query);
  const setQuery = useReaderStore((state) => state.setQuery);
  const readFilter = useReaderStore((state) => state.readFilter);
  const setReadFilter = useReaderStore((state) => state.setReadFilter);
  const items = useReaderStore((state) => state.items);
  const loading = useReaderStore((state) => state.loading);
  const loadingMore = useReaderStore((state) => state.loadingMore);
  const error = useReaderStore((state) => state.error);
  const nextCursor = useReaderStore((state) => state.nextCursor);
  const loadMore = useReaderStore((state) => state.loadMore);
  const loadItems = useReaderStore((state) => state.loadItems);
  const markAllRead = useReaderStore((state) => state.markAllRead);
  const markItemsRead = useReaderStore((state) => state.markItemsRead);
  const batchState = useReaderStore((state) => state.batchState);
  const updateSettings = useSettingsStore((state) => state.update);
  const listWidth = useSettingsStore((state) => state.settings.reader.list_width) || DEFAULT_LIST_WIDTH;
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [selectMode, setSelectMode] = useState(false);
  const [selected, setSelected] = useState<Set<number>>(new Set());
  const [lastSelected, setLastSelected] = useState<number | null>(null);
  const [mergeDuplicates, setMergeDuplicates] = useState(() => {
    try {
      return window.localStorage.getItem(MERGE_KEY) !== '0';
    } catch {
      return true;
    }
  });

  const source = filter.type === 'source' ? sources.find((entry) => entry.id === filter.sourceId) : undefined;
  const folderSources = filter.type === 'folder'
    ? sources.filter((entry) => entry.folder_id === filter.folderId)
    : [];
  const title = readerTitle(filter, sources, folders);
  const unreadInList = items.filter((item) => !item.read_at).length;
  const markAllCount = filter.type === 'source'
    ? source?.unread_count ?? unreadInList
    : filter.type === 'folder'
      ? folderSources.reduce((sum, entry) => sum + entry.unread_count, 0)
      : unreadInList;
  const sentinelRef = useInfiniteScroll({
    hasMore: Boolean(nextCursor),
    loading: loadingMore,
    onLoadMore: () => { void loadMore(); },
  });

  const displayItems = useMemo(() => {
    if (!mergeDuplicates) return items.map((item) => ({ ...item, duplicateSources: [] as string[] }));
    const seen = new Map<string, { item: ItemSummary & { duplicateSources: string[] }; time: number; byTitle: boolean }>();
    const result: Array<ItemSummary & { duplicateSources: string[] }> = [];
    for (const item of items) {
      const canonical = canonicalKey(item);
      const existing = canonical ? seen.get(canonical.key) : undefined;
      const sameWindow = !canonical?.byTitle
        || Math.abs(itemTime(item) - (existing?.time ?? 0)) < 7 * 24 * 60 * 60 * 1000;
      if (existing && existing.item.source_id !== item.source_id && sameWindow) {
        if (!existing.item.duplicateSources.includes(item.source.name)) {
          existing.item.duplicateSources.push(item.source.name);
        }
        continue;
      }
      const copy = { ...item, duplicateSources: [] as string[] };
      result.push(copy);
      if (canonical) seen.set(canonical.key, { item: copy, time: itemTime(item), byTitle: canonical.byTitle });
    }
    return result;
  }, [items, mergeDuplicates]);

  useEffect(() => {
    setSelected((current) => {
      const known = new Set(items.map((item) => item.id));
      const next = new Set([...current].filter((id) => known.has(id)));
      return next.size === current.size ? current : next;
    });
  }, [items]);

  const toggleSelect = (id: number) => {
    setSelected((current) => {
      const next = new Set(current);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
    setLastSelected(id);
  };

  const handleRowSelect = (item: ItemSummary, event?: ReactMouseEvent<HTMLElement>) => {
    if (!selectMode) {
      onSelect(item, event);
      return;
    }
    if (event?.shiftKey && lastSelected != null) {
      const from = displayItems.findIndex((entry) => entry.id === lastSelected);
      const to = displayItems.findIndex((entry) => entry.id === item.id);
      if (from >= 0 && to >= 0) {
        const start = Math.min(from, to);
        const end = Math.max(from, to);
        setSelected((current) => {
          const next = new Set(current);
          for (const entry of displayItems.slice(start, end + 1)) next.add(entry.id);
          return next;
        });
        return;
      }
    }
    toggleSelect(item.id);
  };

  const exitSelect = () => {
    setSelectMode(false);
    setSelected(new Set());
    setLastSelected(null);
  };

  const runBatch = (patch: { read?: boolean; starred?: boolean; later?: boolean; hidden?: boolean }) => {
    if (selected.size === 0) return;
    void batchState([...selected], patch).then(() => {
      setSelected(new Set());
      setLastSelected(null);
    });
  };

  const cycleReadFilter = () => {
    const next = readFilter === 'all' ? 'unread' : readFilter === 'unread' ? 'read' : 'all';
    setReadFilter(next);
  };
  const readFilterIcon = readFilter === 'unread' ? 'unread' : readFilter === 'read' ? 'read' : 'filter';
  const readFilterLabel = readFilter === 'all'
    ? '筛选已读状态：全部，点击只看未读'
    : readFilter === 'unread'
      ? '筛选已读状态：仅未读，点击只看已读'
      : '筛选已读状态：仅已读，点击查看全部';

  const confirmMarkAll = async () => {
    setConfirmOpen(false);
    if (filter.type === 'source' && source) {
      await markAllRead({ source_id: source.id });
    } else if (filter.type === 'folder') {
      await markAllRead({ folder_id: filter.folderId });
    } else {
      await markItemsRead(items.filter((item) => !item.read_at).map((item) => item.id));
    }
  };

  return (
    <section className="list-pane" aria-label="条目列表">
      <header className="list-head">
        <div className="list-head-top">
          <h1>{title}</h1>
          <div className="list-head-actions">
            <IconButton
              icon="search"
              label={searchOpen ? '关闭搜索' : '搜索'}
              size={17}
              active={searchOpen}
              onClick={onToggleSearch}
            />
            <IconButton
              icon="layers"
              label={mergeDuplicates ? '已合并跨源重复内容，点击关闭' : '合并跨源重复内容'}
              size={17}
              active={mergeDuplicates}
              aria-pressed={mergeDuplicates}
              onClick={() => {
                setMergeDuplicates((current) => {
                  const next = !current;
                  try {
                    window.localStorage.setItem(MERGE_KEY, next ? '1' : '0');
                  } catch {
                    /* ignore storage errors */
                  }
                  return next;
                });
              }}
            />
            <IconButton
              icon="check-circle"
              label={selectMode ? '退出多选' : '多选条目'}
              size={17}
              active={selectMode}
              onClick={() => {
                if (selectMode) exitSelect();
                else setSelectMode(true);
              }}
            />
            <IconButton
              icon="check"
              label="全部标为已读"
              size={17}
              disabled={unreadInList === 0}
              onClick={() => setConfirmOpen(true)}
            />
            <IconButton
              icon={readFilterIcon}
              label={readFilterLabel}
              size={17}
              active={readFilter !== 'all'}
              aria-pressed={readFilter !== 'all'}
              onClick={cycleReadFilter}
            />
          </div>
        </div>

        {searchOpen ? (
          <SearchField
            value={query}
            onChange={setQuery}
            placeholder="搜索标题、作者或源"
            ariaLabel="搜索条目"
            autoFocus
            className="list-search"
          />
        ) : null}

        {selectMode ? (
          <div className="list-batch" role="toolbar" aria-label="条目多选操作">
            <span className="list-batch-count">已选 {selected.size}</span>
            <Button size="sm" disabled={selected.size === 0} onClick={() => runBatch({ read: true })}>已读</Button>
            <Button size="sm" disabled={selected.size === 0} onClick={() => runBatch({ read: false })}>未读</Button>
            <Button size="sm" disabled={selected.size === 0} onClick={() => runBatch({ starred: true })}>收藏</Button>
            <Button size="sm" disabled={selected.size === 0} onClick={() => runBatch({ starred: false })}>取消收藏</Button>
            <Button size="sm" disabled={selected.size === 0} onClick={() => runBatch({ later: true })}>稍后读</Button>
            <Button size="sm" disabled={selected.size === 0} onClick={() => runBatch({ later: false })}>移出稍后读</Button>
            <Button size="sm" disabled={selected.size === 0} onClick={() => runBatch({ hidden: true })}>隐藏</Button>
            <button type="button" className="list-batch-link" onClick={exitSelect}>取消</button>
          </div>
        ) : null}

      </header>

      <div className="list-scroll">
        {loading && items.length === 0 ? (
          <div className="item-skeleton-list" aria-label="正在加载条目">
            {Array.from({ length: ITEM_SKELETON_COUNT }, (_, index) => (
              <div className="item-skeleton" key={index}>
                <span></span><span></span><span></span><span></span>
              </div>
            ))}
          </div>
        ) : null}

        {error ? (
          <EmptyState
            icon="alert-circle"
            title="条目加载失败"
            description={error}
            action={<Button size="sm" icon="refresh" onClick={() => { void loadItems(true); }}>重试</Button>}
          />
        ) : null}

        {!loading && !error && items.length === 0 ? (
          <EmptyState
            icon={query ? 'search' : 'inbox'}
            title={query ? '没有匹配的条目' : '没有内容'}
            action={query ? <Button size="sm" onClick={() => setQuery('')}>清除搜索</Button> : undefined}
          />
        ) : null}

        <div className="item-list">
          {displayItems.map((item) => (
            <ArticleRow
              key={item.id}
              item={item}
              selected={selectedId === item.id}
              selecting={selectMode}
              checked={selected.has(item.id)}
              duplicateSources={item.duplicateSources}
              onToggleSelect={() => toggleSelect(item.id)}
              onSelect={handleRowSelect}
              onOpenFocus={onOpenFocus}
            />
          ))}
        </div>

        {loadingMore ? (
          <div className="item-skeleton-list">
            {[0, 1].map((index) => (
              <div className="item-skeleton" key={index}>
                <span></span><span></span><span></span><span></span>
              </div>
            ))}
          </div>
        ) : null}

        <div ref={sentinelRef} aria-hidden="true" />
      </div>

      <Modal
        open={confirmOpen}
        onClose={() => setConfirmOpen(false)}
        title="全部标为已读？"
        subtitle={`${markAllCount} 条未读`}
        footer={
          <>
            <span className="modal-foot-note">可在提示条中撤销</span>
            <Button onClick={() => setConfirmOpen(false)}>取消</Button>
            <Button variant="primary" onClick={() => { void confirmMarkAll(); }}>确认标记</Button>
          </>
        }
      >
        <p className="modal-copy">只影响当前筛选下的条目，不会删除内容。收藏状态保持不变。</p>
      </Modal>

      <ResizeHandle
        label="调整文章列表宽度"
        value={listWidth}
        min={LIST_WIDTH_RANGE.min}
        max={LIST_WIDTH_RANGE.max}
        defaultValue={DEFAULT_LIST_WIDTH}
        cssVar="--list-width"
        onCommit={(value) => updateSettings({ reader: { list_width: value } })}
      />
    </section>
  );
};
