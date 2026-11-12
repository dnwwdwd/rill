import { useLocation, useNavigate } from 'react-router-dom';
import { Icon } from '@/components/ui';
import { useReaderStore } from '@/stores/readerStore';
import { cx } from '@/utils/cx';

export const MobileNav = () => {
  const navigate = useNavigate();
  const location = useLocation();
  const setFilter = useReaderStore((state) => state.setFilter);
  const stats = useReaderStore((state) => state.stats);
  const filter = useReaderStore((state) => state.filter);

  const onSettings = location.pathname.startsWith('/settings');
  const onSubscribe = location.pathname.startsWith('/subscribe');
  const onSources = filter.type === 'source' || filter.type === 'folder';
  const active = onSettings
    ? 'settings'
    : onSubscribe
      ? 'sources'
      : filter.type === 'starred'
        ? 'starred'
        : onSources
          ? 'sources'
          : 'inbox';

  const items = [
    { key: 'inbox', label: '全部', icon: 'inbox' as const, count: stats.total },
    { key: 'starred', label: '收藏', icon: 'star' as const, count: stats.starred },
    { key: 'sources', label: '订阅', icon: 'folder' as const, count: 0 },
    { key: 'settings', label: '设置', icon: 'settings' as const, count: 0 },
  ];

  return (
    <nav className="bottom-nav" aria-label="主要导航">
      {items.map((item) => (
        <button
          type="button"
          key={item.key}
          className={cx('bottom-nav-item', active === item.key && 'is-active')}
          onClick={() => {
            if (item.key === 'inbox') {
              setFilter({ type: 'inbox' });
              navigate('/');
            } else if (item.key === 'starred') {
              setFilter({ type: 'starred' });
              navigate('/');
            } else if (item.key === 'sources') {
              navigate('/subscribe');
            } else {
              navigate('/settings');
            }
          }}
        >
          <span className="bottom-nav-icon">
            <Icon name={item.icon} size={19} filled={item.key === 'starred' && active === 'starred'} />
            {item.count > 0 ? <span className="bottom-nav-count">{item.count}</span> : null}
          </span>
          <span>{item.label}</span>
        </button>
      ))}
    </nav>
  );
};
