import { useEffect, useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Icon, Modal, SearchField, type IconName } from '@/components/ui';
import { useReaderStore } from '@/stores/readerStore';
import { useSourcesStore } from '@/stores/sourcesStore';
import { useUiStore } from '@/stores/uiStore';
import { SETTINGS_SECTIONS } from '@/types/ui';
import { flattenFolders } from '@/utils/folders';
import { cx } from '@/utils/cx';

interface Command {
  id: string;
  label: string;
  hint?: string;
  icon: IconName;
  run: () => void;
}

export const CommandPalette = () => {
  const navigate = useNavigate();
  const sources = useSourcesStore((state) => state.sources);
  const folders = useSourcesStore((state) => state.folders);
  const setFilter = useReaderStore((state) => state.setFilter);
  const setQuery = useReaderStore((state) => state.setQuery);
  const items = useReaderStore((state) => state.items);
  const filter = useReaderStore((state) => state.filter);
  const markAllRead = useReaderStore((state) => state.markAllRead);
  const markItemsRead = useReaderStore((state) => state.markItemsRead);
  const openSettings = useUiStore((state) => state.openSettings);
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState('');
  const [active, setActive] = useState(0);

  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') {
        event.preventDefault();
        setOpen((value) => !value);
        setSearch('');
        setActive(0);
      }
      if (event.key === 'Escape') setOpen(false);
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, []);

  const commands = useMemo<Command[]>(() => {
    const go = (filter: Parameters<typeof setFilter>[0]) => {
      setFilter(filter);
      navigate('/');
      setOpen(false);
    };
    const base: Command[] = [
      {
        id: 'add-source',
        label: '添加订阅源',
        hint: '设置',
        icon: 'plus',
        run: () => {
          openSettings('sources');
          setOpen(false);
        },
      },
      {
        id: 'mark-all-read',
        label: '全部标为已读',
        hint: '当前范围',
        icon: 'check',
        run: () => {
          if (filter.type === 'source') void markAllRead({ source_id: filter.sourceId });
          else if (filter.type === 'folder') void markAllRead({ folder_id: filter.folderId });
          else void markItemsRead(items.filter((item) => !item.read_at).map((item) => item.id));
          setOpen(false);
        },
      },
      { id: 'inbox', label: '全部', icon: 'inbox', run: () => go({ type: 'inbox' }) },
      { id: 'starred', label: '收藏', icon: 'star', run: () => go({ type: 'starred' }) },
      { id: 'later', label: '稍后读', icon: 'bookmark', run: () => go({ type: 'later' }) },
      ...flattenFolders(folders).map((node) => ({
        id: `folder-${node.folder.id}`,
        label: node.folder.name,
        hint: '分组',
        icon: 'folder' as IconName,
        run: () => go({ type: 'folder', folderId: node.folder.id }),
      })),
      ...sources.map((source) => ({
        id: `source-${source.id}`,
        label: source.name,
        hint: '订阅源',
        icon: 'rss' as IconName,
        run: () => go({ type: 'source', sourceId: source.id }),
      })),
      ...SETTINGS_SECTIONS.map((section) => ({
        id: `settings-${section.key}`,
        label: `设置：${section.label}`,
        hint: '设置',
        icon: section.icon,
        run: () => {
          openSettings(section.key);
          setOpen(false);
        },
      })),
    ];
    return base;
  }, [filter, folders, items, markAllRead, markItemsRead, navigate, openSettings, setFilter, sources]);

  const filtered = useMemo(() => {
    const clean = search.trim().toLowerCase();
    if (!clean) return commands.slice(0, 18);
    return commands
      .filter((command) => `${command.label} ${command.hint ?? ''}`.toLowerCase().includes(clean))
      .slice(0, 18);
  }, [commands, search]);

  const runSearch = () => {
    const clean = search.trim();
    if (!clean) return;
    setFilter({ type: 'inbox' });
    setQuery(clean);
    navigate('/');
    setOpen(false);
  };

  const runActive = () => {
    const command = filtered[active];
    if (command) {
      command.run();
      return;
    }
    if (search.trim()) runSearch();
  };

  return (
    <Modal open={open} onClose={() => setOpen(false)} title="快速跳转" subtitle="⌘K / Ctrl+K">
      <div className="command-palette">
        <SearchField
          value={search}
          onChange={(value) => { setSearch(value); setActive(0); }}
          placeholder="搜索订阅源、分组、设置，或直接搜索文章"
          ariaLabel="快速跳转搜索"
          autoFocus
          onKeyDown={(event) => {
            if (event.key === 'ArrowDown') {
              event.preventDefault();
              setActive((value) => Math.min(filtered.length - 1, value + 1));
            } else if (event.key === 'ArrowUp') {
              event.preventDefault();
              setActive((value) => Math.max(0, value - 1));
            } else if (event.key === 'Enter') {
              event.preventDefault();
              runActive();
            }
          }}
        />
        <div className="command-list" role="listbox" aria-label="跳转结果">
          {search.trim() ? (
            <button type="button" className="command-row is-primary" onClick={runSearch}>
              <Icon name="search" size={15} />
              <span>搜索文章「{search.trim()}」</span>
              <kbd>Enter</kbd>
            </button>
          ) : null}
          {filtered.map((command, index) => (
            <button
              type="button"
              key={command.id}
              className={cx('command-row', index === active && 'is-active')}
              onMouseEnter={() => setActive(index)}
              onClick={command.run}
            >
              <Icon name={command.icon} size={15} />
              <span>{command.label}</span>
              {command.hint ? <small>{command.hint}</small> : null}
            </button>
          ))}
          {filtered.length === 0 && !search.trim() ? (
            <p className="command-empty">没有可跳转的内容</p>
          ) : null}
        </div>
      </div>
    </Modal>
  );
};
