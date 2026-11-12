/* 原型外壳、侧边栏与移动导航。 */
function PrototypeBar({ screen, onScreen, device, onDevice, onReset }) {
  return (
    <div className="proto-bar" role="banner">
      <div className="proto-brand">
        <span className="proto-mark" aria-hidden="true"><i></i><i></i><i></i></span>
        <span className="proto-name">Rill 原型</span>
        <span className="proto-sub">设计稿 · 2026-10-31</span>
      </div>
      <div className="proto-screens" role="tablist" aria-label="原型页面">
        {SCREENS.map((item) => (
          <button
            key={item.id}
            type="button"
            role="tab"
            aria-selected={screen === item.id}
            className={cx('proto-tab', screen === item.id && 'is-active')}
            onClick={() => onScreen(item.id)}
          >
            <span className="proto-index">{item.index}</span>
            <span>{item.name}</span>
          </button>
        ))}
      </div>
      <div className="proto-actions">
        <div className="proto-device" role="group" aria-label="预览设备">
          <button type="button" className={cx(device === 'desktop' && 'is-active')} onClick={() => onDevice('desktop')}>桌面</button>
          <button type="button" className={cx(device === 'phone' && 'is-active')} onClick={() => onDevice('phone')}>手机</button>
        </div>
        <button type="button" className="proto-reset" onClick={onReset} title="恢复初始演示状态">重置</button>
      </div>
    </div>
  );
}

function Sidebar({
  items, sources, activeList, onSelect, collapsedFolders, onToggleFolder,
  onOpenSettings, onOpenSources, onOpenAdd, theme, onToggleTheme,
  inDrawer = false, onCloseDrawer,
}) {
  const unread = items.filter((i) => !i.read).length;
  const starred = items.filter((i) => i.starred).length;
  const read = items.filter((i) => i.read).length;
  const navItems = [
    { key: 'inbox', label: '收件箱', icon: 'inbox', count: unread, hint: '全部条目' },
    { key: 'unread', label: '未读', icon: 'unread', count: unread },
    { key: 'starred', label: '收藏', icon: 'star', count: starred },
    { key: 'read', label: '已读', icon: 'read', count: read },
  ];
  const isNavActive = (key) => activeList.type === key;

  return (
    <aside className="rail">
      <div className="rail-brand">
        <div className="rail-brand-top">
          <Wordmark />
          {inDrawer ? <IconButton icon="x" label="关闭导航" onClick={onCloseDrawer} /> : null}
        </div>
        <div className="rail-brand-meta">
          <span>统一订阅入口</span>
          <span className="rail-live"><i></i>调度正常</span>
        </div>
      </div>

      <nav className="rail-nav" aria-label="阅读筛选">
        {navItems.map((item) => (
          <button
            type="button"
            key={item.key}
            className={cx('rail-nav-item', isNavActive(item.key) && 'is-active')}
            onClick={() => onSelect({ type: item.key })}
            title={item.hint}
          >
            <Icon name={item.icon} size={17} filled={item.key === 'starred' && isNavActive(item.key)} />
            <span className="rail-nav-label">{item.label}</span>
            <span className="rail-nav-count">{item.count}</span>
          </button>
        ))}
      </nav>

      <div className="rail-section-head">
        <span>订阅源</span>
        <span className="rail-section-tools">
          <span className="rail-source-total">{sources.length}</span>
          <IconButton icon="plus" label="添加订阅源" size={15} onClick={onOpenAdd} />
        </span>
      </div>

      <div className="rail-tree">
        {FOLDERS.map((folder) => {
          const folderSources = sources.filter((s) => s.folder === folder.name);
          const collapsed = collapsedFolders.includes(folder.id);
          return (
            <div className="folder-block" key={folder.id}>
              <div className={cx('folder-row', activeList.type === 'folder' && activeList.folder === folder.name && 'is-active')}>
                <button
                  type="button"
                  className={cx('folder-chevron', collapsed && 'is-collapsed')}
                  aria-label={collapsed ? `展开${folder.name}` : `收起${folder.name}`}
                  aria-expanded={!collapsed}
                  onClick={() => onToggleFolder(folder.id)}
                >
                  <Icon name="chevron-down" size={13} />
                </button>
                <button type="button" className="folder-name" onClick={() => onSelect({ type: 'folder', folder: folder.name })}>
                  <span className="folder-index">{folder.index}</span>
                  <span>{folder.name}</span>
                </button>
                <span className="folder-source-count">{folderSources.length}</span>
              </div>
              {!collapsed ? (
                <div className="folder-sources">
                  {folderSources.map((source) => {
                    const count = items.filter((i) => i.sourceId === source.id && !i.read).length;
                    const active = activeList.type === 'source' && activeList.sourceId === source.id;
                    return (
                      <button
                        type="button"
                        key={source.id}
                        className={cx('source-row', active && 'is-active', !source.enabled && 'is-paused')}
                        onClick={() => onSelect({ type: 'source', sourceId: source.id })}
                        title={source.note || source.name}
                      >
                        <SourceIcon source={source} size={18} />
                        <span className="source-name">{source.name}</span>
                        {count > 0 ? <span className="source-count">{count}</span> : null}
                        <StatusDot status={source.status} />
                      </button>
                    );
                  })}
                </div>
              ) : null}
            </div>
          );
        })}
      </div>

      <div className="rail-foot">
        <button type="button" className="rail-foot-btn" onClick={onOpenSettings}>
          <Icon name="sliders" size={16} />
          <span>设置</span>
        </button>
        <button type="button" className="rail-foot-btn" onClick={onOpenSources}>
          <Icon name="rss" size={16} />
          <span>订阅源</span>
          <span className="rail-foot-count">{sources.length}</span>
        </button>
        <button type="button" className="rail-theme" onClick={onToggleTheme} aria-label={theme === 'light' ? '切换到深色' : '切换到浅色'}>
          <Icon name={theme === 'light' ? 'moon' : 'sun'} size={15} />
          <span>{theme === 'light' ? '深色' : '浅色'}</span>
        </button>
      </div>
    </aside>
  );
}

function MobileTopbar({ title, onMenu, onBack, right, showBrand = false }) {
  return (
    <header className="mobile-bar">
      <div className="mobile-bar-left">
        {onBack ? (
          <IconButton icon="arrow-left" label="返回" onClick={onBack} />
        ) : onMenu ? (
          <IconButton icon="menu" label="打开导航" onClick={onMenu} />
        ) : null}
        <div className="mobile-bar-title">
          {showBrand ? <span className="mobile-brand">Rill</span> : null}
          {showBrand ? <span className="mobile-sep">/</span> : null}
          <span>{title}</span>
        </div>
      </div>
      <div className="mobile-bar-right">{right}</div>
    </header>
  );
}

function MobileNav({ active, onChange, unreadCount, starredCount }) {
  const nav = [
    { key: 'inbox', label: '收件箱', icon: 'inbox', count: unreadCount },
    { key: 'starred', label: '收藏', icon: 'star', count: starredCount },
    { key: 'sources', label: '订阅源', icon: 'rss', count: 0 },
    { key: 'settings', label: '设置', icon: 'sliders', count: 0 },
  ];
  return (
    <nav className="bottom-nav" aria-label="主要导航">
      {nav.map((item) => (
        <button
          type="button"
          key={item.key}
          className={cx('bottom-nav-item', active === item.key && 'is-active')}
          onClick={() => onChange(item.key)}
        >
          <span className="bottom-nav-icon">
            <Icon name={item.icon} size={19} filled={item.key === 'starred' && active === item.key} />
            {item.count > 0 ? <span className="bottom-nav-count">{item.count}</span> : null}
          </span>
          <span>{item.label}</span>
        </button>
      ))}
    </nav>
  );
}

function Drawer({ open, onClose, children }) {
  return (
    <div className={cx('drawer-layer', open && 'is-open')} aria-hidden={!open}>
      <button type="button" className="drawer-scrim" aria-label="关闭导航" onClick={onClose}></button>
      <div className="drawer-panel">{children}</div>
    </div>
  );
}

Object.assign(window, { PrototypeBar, Sidebar, MobileTopbar, MobileNav, Drawer });
