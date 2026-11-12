import { Fragment, useEffect, useMemo, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import type { Folder, SourceView } from '@/api/types';
import { AddSourceDialog } from '@/components/sources/AddSourceDialog';
import { EditSourceDialog } from '@/components/sources/EditSourceDialog';
import { FolderManagerDialog } from '@/components/sources/FolderManagerDialog';
import {
  Button,
  EmptyState,
  Icon,
  IconButton,
  SearchField,
  SourceIcon,
  StatusBadge,
  Switch,
  Tag,
} from '@/components/ui';
import { WechatChannel } from '@/components/settings/WechatSettings';
import { useSourcesStore } from '@/stores/sourcesStore';
import { useUiStore } from '@/stores/uiStore';
import type { SourceFilter } from '@/types/ui';
import { formatAgo, formatRefresh } from '@/utils/date';
import { flattenFolders, folderOptionLabel } from '@/utils/folders';
import { isAttention, toSourceView } from '@/utils/sourceStatus';
import { bestBlogsFeedKind } from '@/utils/bestblogs';
import { cx } from '@/utils/cx';

const matchesFilter = (source: SourceView, filter: SourceFilter): boolean => {
  if (filter === 'all') return true;
  if (filter === 'ok') return source.status === 'ok';
  if (filter === 'attention') return isAttention(source.status);
  return source.status === 'paused';
};

const sourceKind = (source: SourceView) => {
  if (source.kind === 'wechat') return { tone: 'wechat' as const, icon: 'message-circle' as const, label: '公众号' };
  if (source.kind === 'x') return { tone: 'x' as const, icon: 'at-sign' as const, label: 'X' };
  const bestBlogsKind = bestBlogsFeedKind(source.feed_url);
  if (bestBlogsKind === 'wechat') return { tone: 'wechat' as const, icon: 'message-circle' as const, label: '公众号' };
  if (bestBlogsKind === 'x') return { tone: 'x' as const, icon: 'at-sign' as const, label: 'X' };
  if (bestBlogsKind === 'podcast') return { tone: 'neutral' as const, icon: 'rss' as const, label: '小宇宙' };
  if (bestBlogsKind === 'youtube') return { tone: 'neutral' as const, icon: 'rss' as const, label: 'YouTube' };
  if (source.feed_url.startsWith('xiaoyuzhou://')) return { tone: 'neutral' as const, icon: 'rss' as const, label: '小宇宙' };
  // 存量微博源已停用（迁移 6），保留类型标签方便识别后手动删除。
  if (source.feed_url.startsWith('weibo://')) return { tone: 'neutral' as const, icon: 'at-sign' as const, label: '微博' };
  return { tone: 'neutral' as const, icon: 'rss' as const, label: 'RSS' };
};

export const SourcesSettings = () => {
  const navigate = useNavigate();
  const sources = useSourcesStore((state) => state.sources);
  const folders = useSourcesStore((state) => state.folders);
  const toggleEnabled = useSourcesStore((state) => state.toggleEnabled);
  const refresh = useSourcesStore((state) => state.refresh);
  const remove = useSourcesStore((state) => state.remove);
  const batchUpdate = useSourcesStore((state) => state.batchUpdate);
  const batchRemove = useSourcesStore((state) => state.batchRemove);
  const exportOpml = useSourcesStore((state) => state.exportOpml);
  const importOpml = useSourcesStore((state) => state.importOpml);
  const closeSettings = useUiStore((state) => state.closeSettings);
  const notify = useUiStore((state) => state.notify);
  const opmlRef = useRef<HTMLInputElement>(null);
  const menuRef = useRef<HTMLDivElement>(null);
  const [filter, setFilter] = useState<SourceFilter>('all');
  const [query, setQuery] = useState('');
  const [collapsed, setCollapsed] = useState<Set<number>>(new Set());
  const [addOpen, setAddOpen] = useState(false);
  const [addFolderId, setAddFolderId] = useState<number | null>(null);
  const [foldersOpen, setFoldersOpen] = useState(false);
  const [menuOpen, setMenuOpen] = useState(false);
  const [editingSource, setEditingSource] = useState<SourceView | null>(null);
  const [opmlBusy, setOpmlBusy] = useState(false);
  const [selectMode, setSelectMode] = useState(false);
  const [selected, setSelected] = useState<Set<number>>(new Set());
  const [batchConfirm, setBatchConfirm] = useState(false);
  const [batchBusy, setBatchBusy] = useState(false);

  useEffect(() => {
    setCollapsed((current) => {
      const known = new Set(folders.map((folder) => folder.id));
      const next = new Set([...current].filter((id) => known.has(id)));
      return next.size === current.size ? current : next;
    });
  }, [folders]);

  useEffect(() => {
    if (!menuOpen) return undefined;
    const onPointerDown = (event: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(event.target as Node)) {
        setMenuOpen(false);
      }
    };
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setMenuOpen(false);
    };
    document.addEventListener('mousedown', onPointerDown);
    window.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('mousedown', onPointerDown);
      window.removeEventListener('keydown', onKey);
    };
  }, [menuOpen]);

  const folderNodes = useMemo(() => flattenFolders(folders), [folders]);
  const sourceViews = useMemo(() => sources.map((source) => toSourceView(source, folders)), [sources, folders]);
  const counts = {
    all: sourceViews.length,
    ok: sourceViews.filter((source) => source.status === 'ok').length,
    attention: sourceViews.filter((source) => isAttention(source.status)).length,
    paused: sourceViews.filter((source) => source.status === 'paused').length,
  };
  const filtered = sourceViews.filter((source) => {
    if (!matchesFilter(source, filter)) return false;
    if (!query.trim()) return true;
    const search = query.trim().toLowerCase();
    return [source.name, source.feed_url, source.folder_name].join(' ').toLowerCase().includes(search);
  });
  const isFiltering = query.trim() !== '';
  const visibleFolderIds = new Set<number>();
  if (!isFiltering) {
    for (const folder of folders) visibleFolderIds.add(folder.id);
  } else {
    for (const source of filtered) {
      if (source.folder_id != null) visibleFolderIds.add(source.folder_id);
    }
  }
  const folderRows = folderNodes.filter(({ folder }) => visibleFolderIds.has(folder.id));
  const orphans = filtered.filter((source) => source.folder_id == null || !folders.some((folder) => folder.id === source.folder_id));
  const byId = new Map(folders.map((folder) => [folder.id, folder]));

  const hiddenByCollapsedAncestor = (folder: Folder): boolean => {
    let parent = folder.parent_id;
    while (parent != null) {
      if (collapsed.has(parent)) return true;
      parent = byId.get(parent)?.parent_id ?? null;
    }
    return false;
  };

  const toggleFolder = (id: number) => {
    setCollapsed((current) => {
      const next = new Set(current);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  const openAdd = (folderId: number | null = null) => {
    setAddFolderId(folderId);
    setAddOpen(true);
  };

  const openSource = (source: SourceView) => {
    closeSettings();
    navigate(`/sources/${source.id}`);
  };

  const downloadOpml = async () => {
    setMenuOpen(false);
    setOpmlBusy(true);
    const text = await exportOpml();
    setOpmlBusy(false);
    if (!text) return;
    const blob = new Blob([text], { type: 'text/x-opml;charset=utf-8' });
    const url = URL.createObjectURL(blob);
    const anchor = document.createElement('a');
    anchor.href = url;
    anchor.download = 'rill-subscriptions.opml';
    anchor.click();
    window.setTimeout(() => URL.revokeObjectURL(url), 1000);
    notify('OPML 已导出');
  };

  const uploadOpml = async (file: File | undefined) => {
    if (!file) return;
    setOpmlBusy(true);
    await importOpml(file);
    setOpmlBusy(false);
    if (opmlRef.current) opmlRef.current.value = '';
  };

  const exitSelect = () => {
    setSelectMode(false);
    setSelected(new Set());
    setBatchConfirm(false);
  };

  const enterSelect = () => {
    setSelectMode(true);
    setSelected(new Set());
    setBatchConfirm(false);
  };

  const toggleSelect = (id: number) => {
    setSelected((current) => {
      const next = new Set(current);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      if (next.size === 0) setBatchConfirm(false);
      return next;
    });
  };

  const runBatch = async (action: () => Promise<unknown>) => {
    if (selected.size === 0 || batchBusy) return;
    setBatchBusy(true);
    try {
      await action();
    } finally {
      setBatchBusy(false);
      setBatchConfirm(false);
    }
  };

  const moveBatch = (folderId: number) => runBatch(() => batchUpdate([...selected], { folder_id: folderId }));
  const toggleBatch = (enabled: boolean) => runBatch(() => batchUpdate([...selected], { enabled }));
  const deleteBatch = () => runBatch(async () => {
    await batchRemove([...selected]);
    exitSelect();
  });

  return (
    <>
      <div className="st-toolbar">
        <SearchField
          value={query}
          onChange={setQuery}
          placeholder="搜索源名称或地址"
          ariaLabel="搜索订阅源"
          className="st-search"
        />
        <div className="st-pills" role="group" aria-label="订阅源状态概览">
          <button type="button" className={cx('st-pill', filter === 'all' && 'is-active')} onClick={() => setFilter('all')}>
            全部 <strong>{counts.all}</strong>
          </button>
          <button type="button" className={cx('st-pill', filter === 'ok' && 'is-active')} onClick={() => setFilter('ok')}>
            正常 <strong>{counts.ok}</strong>
          </button>
          <button type="button" className={cx('st-pill', filter === 'attention' && 'is-active')} onClick={() => setFilter('attention')}>
            需关注 <strong>{counts.attention}</strong>
          </button>
          <button type="button" className={cx('st-pill', filter === 'paused' && 'is-active')} onClick={() => setFilter('paused')}>
            已暂停 <strong>{counts.paused}</strong>
          </button>
        </div>
        <Button variant="primary" size="sm" icon="plus" onClick={() => openAdd()}>添加源</Button>
        {selectMode ? null : (
          <Button size="sm" icon="check" onClick={enterSelect}>批量管理</Button>
        )}
        <div className="st-more" ref={menuRef}>
          <IconButton
            icon="more"
            label="更多操作"
            size={16}
            active={menuOpen}
            aria-haspopup="menu"
            aria-expanded={menuOpen}
            onClick={() => setMenuOpen((value) => !value)}
          />
          {menuOpen ? (
            <div className="st-menu" role="menu" aria-label="更多订阅源操作">
              <button type="button" role="menuitem" onClick={() => { setMenuOpen(false); setFoldersOpen(true); }}>
                <Icon name="folder" size={15} /> 管理分组
              </button>
              <button type="button" role="menuitem" disabled={opmlBusy} onClick={() => { setMenuOpen(false); opmlRef.current?.click(); }}>
                <Icon name="upload" size={15} /> 导入 OPML
              </button>
              <button type="button" role="menuitem" disabled={opmlBusy} onClick={() => { setMenuOpen(false); void downloadOpml(); }}>
                <Icon name="download" size={15} /> 导出 OPML
              </button>
            </div>
          ) : null}
        </div>
        <input
          ref={opmlRef}
          type="file"
          accept=".opml,.xml,text/xml,application/xml"
          hidden
          onChange={(event) => { void uploadOpml(event.target.files?.[0]); }}
        />
      </div>

      {selectMode ? (
        <div className="st-batch" role="toolbar" aria-label="批量操作">
          <span className="st-batch-count">已选 {selected.size}</span>
          <button
            type="button"
            className="st-batch-link"
            onClick={() => setSelected(new Set(filtered.map((source) => source.id)))}
            disabled={filtered.length === 0 || batchBusy}
          >
            全选
          </button>
          <button
            type="button"
            className="st-batch-link"
            onClick={() => { setSelected(new Set()); setBatchConfirm(false); }}
            disabled={selected.size === 0 || batchBusy}
          >
            清空
          </button>
          <select
            className="st-batch-select"
            value=""
            aria-label="移动到分组"
            disabled={selected.size === 0 || batchBusy}
            onChange={(event) => {
              const value = event.target.value;
              event.target.value = '';
              if (value !== '') moveBatch(Number(value));
            }}
          >
            <option value="" disabled>移动到分组…</option>
            <option value="0">不分组</option>
            {folderNodes.map((node) => (
              <option key={node.folder.id} value={node.folder.id}>{folderOptionLabel(node)}</option>
            ))}
          </select>
          <Button size="sm" disabled={selected.size === 0 || batchBusy} onClick={() => toggleBatch(true)}>启用</Button>
          <Button size="sm" disabled={selected.size === 0 || batchBusy} onClick={() => toggleBatch(false)}>停用</Button>
          {batchConfirm ? (
            <span className="st-confirm">
              <Button size="sm" variant="danger" onClick={() => { void deleteBatch(); }}>确认删除</Button>
              <Button size="sm" onClick={() => setBatchConfirm(false)}>取消</Button>
            </span>
          ) : (
            <Button size="sm" variant="danger" disabled={selected.size === 0 || batchBusy} onClick={() => setBatchConfirm(true)}>删除</Button>
          )}
          <span className="st-batch-spacer" />
          <Button size="sm" onClick={exitSelect}>取消选择</Button>
        </div>
      ) : null}

      <div className="st-tree" role="tree" aria-label="订阅源树">
        {folderRows.map(({ folder, depth }) => {
          if (hiddenByCollapsedAncestor(folder)) return null;
          const rows = filtered.filter((source) => source.folder_id === folder.id);
          const isCollapsed = collapsed.has(folder.id);
          const hasChildren = folders.some((entry) => entry.parent_id === folder.id) || rows.length > 0;
          return (
            <Fragment key={folder.id}>
              <div
                className="st-tree-folder"
                style={{ paddingLeft: 10 + depth * 18 }}
                role="treeitem"
                aria-expanded={hasChildren ? !isCollapsed : undefined}
                onClick={() => { if (hasChildren) toggleFolder(folder.id); }}
              >
                {hasChildren ? (
                  <button
                    type="button"
                    className="st-tree-toggle"
                    onClick={(event) => { event.stopPropagation(); toggleFolder(folder.id); }}
                    aria-label={isCollapsed ? `展开分组 ${folder.name}` : `收起分组 ${folder.name}`}
                  >
                    <Icon name={isCollapsed ? 'chevron-right' : 'chevron-down'} size={14} />
                  </button>
                ) : (
                  <span className="st-tree-toggle is-leaf" aria-hidden="true" />
                )}
                <span className="st-tree-folder-name">{folder.name}</span>
                <span className="st-tree-count">{rows.length}</span>
                <div className="st-tree-actions" onClick={(event) => event.stopPropagation()}>
                  <IconButton
                    icon="plus"
                    label={`添加订阅到「${folder.name}」`}
                    size={15}
                    onClick={() => openAdd(folder.id)}
                  />
                </div>
              </div>
              {!isCollapsed ? rows.map((source) => (
                <SourceItem
                  key={source.id}
                  source={source}
                  depth={depth}
                  selecting={selectMode}
                  checked={selected.has(source.id)}
                  onToggleSelect={() => toggleSelect(source.id)}
                  onOpen={() => openSource(source)}
                  onToggle={() => { void toggleEnabled(source.id); }}
                  onRefresh={() => { void refresh(source.id); }}
                  onEdit={() => setEditingSource(source)}
                  onDelete={() => { void remove(source.id); }}
                />
              )) : null}
            </Fragment>
          );
        })}

        {orphans.length > 0 ? (
          <>
            <div className="st-tree-folder is-orphan" style={{ paddingLeft: 10 }} role="treeitem" aria-expanded="true">
              <span className="st-tree-toggle is-leaf" aria-hidden="true" />
              <span className="st-tree-folder-name">未分组</span>
              <span className="st-tree-count">{orphans.length}</span>
              <div className="st-tree-actions" onClick={(event) => event.stopPropagation()}>
                <IconButton icon="plus" label="添加未分组订阅" size={15} onClick={() => openAdd(null)} />
              </div>
            </div>
            {orphans.map((source) => (
              <SourceItem
                key={source.id}
                source={source}
                depth={0}
                selecting={selectMode}
                checked={selected.has(source.id)}
                onToggleSelect={() => toggleSelect(source.id)}
                onOpen={() => openSource(source)}
                onToggle={() => { void toggleEnabled(source.id); }}
                onRefresh={() => { void refresh(source.id); }}
                onEdit={() => setEditingSource(source)}
                onDelete={() => { void remove(source.id); }}
              />
            ))}
          </>
        ) : null}

        {folderRows.length === 0 && orphans.length === 0 ? (
          <EmptyState icon="inbox" title="没有内容" />
        ) : null}
      </div>

      <WechatChannel />

      <AddSourceDialog open={addOpen} initialFolderId={addFolderId} onClose={() => setAddOpen(false)} />
      <FolderManagerDialog open={foldersOpen} onClose={() => setFoldersOpen(false)} />
      <EditSourceDialog open={Boolean(editingSource)} source={editingSource} onClose={() => setEditingSource(null)} />
    </>
  );
};

interface SourceItemProps {
  source: SourceView;
  depth: number;
  selecting?: boolean;
  checked?: boolean;
  onToggleSelect: () => void;
  onOpen: () => void;
  onToggle: () => void;
  onRefresh: () => void;
  onEdit: () => void;
  onDelete: () => void;
}

const SourceItem = ({
  source,
  depth,
  selecting = false,
  checked = false,
  onToggleSelect,
  onOpen,
  onToggle,
  onRefresh,
  onEdit,
  onDelete,
}: SourceItemProps) => {
  const kind = sourceKind(source);
  const [confirming, setConfirming] = useState(false);
  const activate = selecting ? onToggleSelect : onOpen;
  return (
    <div
      className={cx('st-tree-source', !source.enabled && 'is-paused', selecting && 'is-selecting', selecting && checked && 'is-checked')}
      style={{ paddingLeft: 10 + depth * 18 }}
      role="treeitem"
      aria-selected={selecting ? checked : undefined}
      tabIndex={0}
      onClick={activate}
      onKeyDown={(event) => {
        if (event.key === 'Enter' || event.key === ' ') {
          event.preventDefault();
          activate();
        }
      }}
    >
      {selecting ? (
        <input
          type="checkbox"
          className="st-check"
          checked={checked}
          onChange={onToggleSelect}
          onClick={(event) => event.stopPropagation()}
          aria-label={`选择 ${source.name}`}
        />
      ) : null}
      <SourceIcon source={source} size={26} />
      <div className="st-tree-copy">
        <span className="st-tree-name">
          <strong>{source.name}</strong>
          <Tag tone={kind.tone} icon={kind.icon}>{kind.label}</Tag>
        </span>
        <span className="st-tree-meta">
          <StatusBadge status={source.status} note={source.note} />
          {formatRefresh(source.refresh_seconds)} · {formatAgo(source.last_fetch_at)}
        </span>
      </div>
      {selecting ? null : (
        <div
          className="st-tree-actions"
          onClick={(event) => event.stopPropagation()}
          onKeyDown={(event) => event.stopPropagation()}
        >
          <Switch
            checked={source.enabled}
            onChange={onToggle}
            label={source.enabled ? `暂停 ${source.name}` : `启用 ${source.name}`}
          />
          <IconButton icon="pencil" label={`编辑或移动 ${source.name}`} size={15} onClick={onEdit} />
          <IconButton icon="refresh" label="立即刷新" size={15} onClick={onRefresh} />
          {confirming ? (
            <span className="st-confirm">
              <Button size="sm" variant="danger" onClick={onDelete}>删除</Button>
              <Button size="sm" onClick={() => setConfirming(false)}>取消</Button>
            </span>
          ) : (
            <IconButton icon="trash" label={`删除 ${source.name}`} size={15} onClick={() => setConfirming(true)} />
          )}
        </div>
      )}
    </div>
  );
};
