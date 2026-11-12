import { useNavigate } from 'react-router-dom';
import { MobileTopbar } from '@/components/layout/MobileTopbar';
import { IconButton, SourceIcon } from '@/components/ui';
import { useReaderStore } from '@/stores/readerStore';
import { useSourcesStore } from '@/stores/sourcesStore';
import { flattenFolders } from '@/utils/folders';
import { cx } from '@/utils/cx';

export const MobileSourcesPage = () => {
  const navigate = useNavigate();
  const sources = useSourcesStore((state) => state.sources);
  const folders = useSourcesStore((state) => state.folders);
  const filter = useReaderStore((state) => state.filter);
  const setFilter = useReaderStore((state) => state.setFilter);
  const nodes = flattenFolders(folders);

  const openSource = (sourceId: number) => {
    setFilter({ type: 'source', sourceId });
    navigate('/');
  };

  const openFolder = (folderId: number) => {
    setFilter({ type: 'folder', folderId });
    navigate('/');
  };

  const renderSource = (source: (typeof sources)[number]) => {
    const count = source.kind === 'wechat' ? source.item_count : source.unread_count;
    return (
      <button
        type="button"
        key={source.id}
        className={cx(
          'mobile-source-row',
          filter.type === 'source' && filter.sourceId === source.id && 'is-active',
          !source.enabled && 'is-paused',
        )}
        onClick={() => openSource(source.id)}
      >
        <SourceIcon source={source} size={20} />
        <span className="mobile-source-name">{source.name}</span>
        {count > 0 ? (
          <span className={cx('mobile-source-count', source.kind === 'wechat' && 'is-total')}>{count}</span>
        ) : null}
      </button>
    );
  };

  const grouped = nodes
    .map((node) => ({
      node,
      rows: sources.filter((source) => source.folder_id === node.folder.id),
    }))
    .filter((entry) => entry.rows.length > 0);
  const orphans = sources.filter((source) => source.folder_id == null);

  return (
    <div className="mobile-sources-page">
      <MobileTopbar
        title="订阅源"
        backTo="/"
        right={(
          <IconButton
            icon="plus"
            label="添加订阅源"
            onClick={() => navigate('/settings/sources')}
          />
        )}
      />
      <div className="mobile-sources-scroll">
        {grouped.length === 0 && orphans.length === 0 ? (
          <div className="mobile-sources-empty">
            <p>还没有订阅源</p>
            <button type="button" onClick={() => navigate('/settings/sources')}>去添加</button>
          </div>
        ) : null}

        {grouped.map(({ node, rows }) => (
          <section
            key={node.folder.id}
            className="mobile-source-group"
            style={{ paddingLeft: node.depth * 10 }}
          >
            <button
              type="button"
              className={cx(
                'mobile-source-folder',
                filter.type === 'folder' && filter.folderId === node.folder.id && 'is-active',
              )}
              onClick={() => openFolder(node.folder.id)}
            >
              <span>{node.folder.name}</span>
              <span className="mobile-source-folder-count">{rows.length}</span>
            </button>
            {rows.map(renderSource)}
          </section>
        ))}

        {orphans.length > 0 ? (
          <section className="mobile-source-group">
            <div className="mobile-source-folder is-static">
              <span>未分组</span>
              <span className="mobile-source-folder-count">{orphans.length}</span>
            </div>
            {orphans.map(renderSource)}
          </section>
        ) : null}
      </div>
    </div>
  );
};
