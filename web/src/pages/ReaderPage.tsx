import { useEffect, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { ArticleList } from '@/components/reader/ArticleList';
import { ReaderPane } from '@/components/reader/ReaderPane';
import { MobileTopbar } from '@/components/layout/MobileTopbar';
import { IconButton } from '@/components/ui';
import { useDebouncedValue } from '@/hooks/useDebouncedValue';
import { useKeyboardNav } from '@/hooks/useKeyboardNav';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import { useReaderStore } from '@/stores/readerStore';
import { useSettingsStore } from '@/stores/settingsStore';
import { useSourcesStore } from '@/stores/sourcesStore';
import { cx } from '@/utils/cx';
import { loadLastItemId } from '@/utils/readerPersistence';

// Only restore the last article once per full page load. SPA navigation back
// to "/" (e.g. clicking a source) must stay on the list.
let restoredLastArticle = false;

export const ReaderPage = () => {
  const params = useParams<{ itemId?: string }>();
  const navigate = useNavigate();
  const isMobile = useMediaQuery('(max-width: 760px)');
  const isTablet = useMediaQuery('(max-width: 1040px)');
  const items = useReaderStore((state) => state.items);
  const filter = useReaderStore((state) => state.filter);
  const query = useReaderStore((state) => state.query);
  const setQuery = useReaderStore((state) => state.setQuery);
  const setFilter = useReaderStore((state) => state.setFilter);
  const readFilter = useReaderStore((state) => state.readFilter);
  const loadItems = useReaderStore((state) => state.loadItems);
  const loadDetail = useReaderStore((state) => state.loadDetail);
  const details = useReaderStore((state) => state.details);
  const toggleRead = useReaderStore((state) => state.toggleRead);
  const markRead = useReaderStore((state) => state.markRead);
  const toggleStar = useReaderStore((state) => state.toggleStar);
  const refreshSource = useSourcesStore((state) => state.refresh);
  const sources = useSourcesStore((state) => state.sources);
  const folders = useSourcesStore((state) => state.folders);
  const sourcesLoaded = useSourcesStore((state) => state.loaded);
  const theme = useSettingsStore((state) => state.settings.reader.theme);
  const autoRead = useSettingsStore((state) => state.settings.reader.auto_read);
  const setTheme = useSettingsStore((state) => state.setTheme);
  const [searchOpen, setSearchOpen] = useState(false);
  const debouncedQuery = useDebouncedValue(query, 280);
  const parsedId = params.itemId ? Number(params.itemId) : null;
  const selectedId = parsedId && Number.isFinite(parsedId) ? parsedId : null;
  const selected = selectedId
    ? items.find((item) => item.id === selectedId) ?? details[selectedId]
    : undefined;
  const displayId = selectedId ?? (isMobile ? null : items[0]?.id ?? null);
  const mobileReading = isMobile && Boolean(selectedId);

  useEffect(() => {
    void loadItems(true);
  }, [filter, debouncedQuery, readFilter, loadItems]);

  useEffect(() => {
    if (restoredLastArticle) return;
    restoredLastArticle = true;
    if (selectedId) return;
    const lastId = loadLastItemId();
    if (!lastId) return;
    void loadDetail(lastId).then((detail) => {
      if (detail) navigate(`/items/${lastId}`, { replace: true });
    });
  }, [loadDetail, navigate, selectedId]);

  useEffect(() => {
    if (!sourcesLoaded) return;
    if (filter.type === 'source' && !sources.some((source) => source.id === filter.sourceId)) {
      setFilter({ type: 'inbox' });
    } else if (filter.type === 'folder' && !folders.some((folder) => folder.id === filter.folderId)) {
      setFilter({ type: 'inbox' });
    }
  }, [filter, folders, setFilter, sources, sourcesLoaded]);

  useEffect(() => {
    setSearchOpen(false);
  }, [filter]);

  const toggleSearch = () => {
    if (searchOpen) {
      setSearchOpen(false);
      setQuery('');
    } else {
      setSearchOpen(true);
    }
  };

  const openFocus = (id: number) => {
    const item = items.find((entry) => entry.id === id) ?? details[id];
    if (autoRead && item && !item.read_at) void markRead(id);
    navigate(`/items/${id}/focus`);
  };
  const selectItem = (item: { id: number; read_at: string | null }) => {
    if (autoRead && !item.read_at) void markRead(item.id);
    navigate(`/items/${item.id}`);
  };

  const current = selected ?? items[0];
  const currentIndex = current ? items.findIndex((item) => item.id === current.id) : -1;

  useKeyboardNav({
    enabled: items.length > 0 && !searchOpen,
    onNext: () => {
      const next = items[Math.min(currentIndex + 1, items.length - 1)] ?? items[0];
      if (next) navigate(`/items/${next.id}`);
    },
    onPrevious: () => {
      const previous = items[Math.max(currentIndex - 1, 0)] ?? items[0];
      if (previous) navigate(`/items/${previous.id}`);
    },
    onOpen: () => { if (current) openFocus(current.id); },
    onFocus: () => { if (current) openFocus(current.id); },
    onToggleRead: () => { if (current) void toggleRead(current.id); },
    onToggleStar: () => { if (current) void toggleStar(current.id); },
    onRefresh: () => { if (current) void refreshSource(current.source_id); },
    onFirst: () => { if (items[0]) navigate(`/items/${items[0].id}`); },
  });

  const effectiveTheme = theme === 'system'
    ? (document.documentElement.dataset.theme === 'dark' ? 'dark' : 'light')
    : theme;

  let topbar = null;
  if (isTablet) {
    if (mobileReading && selected) {
      topbar = (
        <MobileTopbar
          title={selected.source.name}
          backTo="/"
          right={
            <div className="mobile-reader-actions">
              <IconButton icon="focus" label="专注阅读" size={17} onClick={() => openFocus(selected.id)} />
              <IconButton
                icon="star"
                label={selected.starred_at ? '取消收藏' : '收藏'}
                size={17}
                filled={Boolean(selected.starred_at)}
                active={Boolean(selected.starred_at)}
                onClick={() => { void toggleStar(selected.id); }}
              />
              <IconButton
                icon={selected.read_at ? 'unread' : 'check'}
                label={selected.read_at ? '标为未读' : '标为已读'}
                size={17}
                onClick={() => { void toggleRead(selected.id); }}
              />
            </div>
          }
        />
      );
    } else {
      topbar = (
        <MobileTopbar
          title="阅读"
          showBrand
          right={
            <div className="mobile-reader-actions">
              <IconButton
                icon={effectiveTheme === 'dark' ? 'sun' : 'moon'}
                label="切换主题"
                size={18}
                onClick={() => setTheme(effectiveTheme === 'dark' ? 'light' : 'dark')}
              />
            </div>
          }
        />
      );
    }
  }

  return (
    <>
      {topbar}
      <div className={cx('reader-screen', mobileReading && 'is-mobile-reading')}>
        <ArticleList
          selectedId={displayId}
          searchOpen={searchOpen}
          onToggleSearch={toggleSearch}
          onSelect={selectItem}
          onOpenFocus={(item) => openFocus(item.id)}
        />
        <ReaderPane
          itemId={displayId}
          isMobile={isMobile}
          onOpenFocus={(id) => openFocus(id)}
        />
      </div>
    </>
  );
};
