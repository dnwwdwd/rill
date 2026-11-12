/* Rill 原型入口：状态、路由、登录与核心交互。 */
const USER_COLORS = ['#8C3B2A', '#2F5D50', '#3A4D7A', '#6B4A7A', '#6F5B2B', '#2E5C6B', '#7A3E4B', '#4A5B3A'];

function readStored(key, fallback) {
  try {
    const raw = window.localStorage.getItem(key);
    return raw ? JSON.parse(raw) : fallback;
  } catch (error) {
    return fallback;
  }
}

function useContainerWidth(ref) {
  const [width, setWidth] = React.useState(0);
  React.useEffect(() => {
    const el = ref.current;
    if (!el) return undefined;
    const update = () => setWidth(el.clientWidth);
    update();
    const observer = new ResizeObserver(update);
    observer.observe(el);
    return () => observer.disconnect();
  }, [ref]);
  return width;
}

function LoginScreen({ password, onPassword, error, busy, onLogin, sources, theme }) {
  return (
    <div className="login-screen">
      <section className="login-art" aria-label="Rill 介绍">
        <div className="login-brand"><Wordmark /></div>
        <div className="login-hero">
          <div className="eyebrow">统一订阅入口</div>
          <h1>把散落的订阅，<br />收成一条清流。</h1>
          <p>标准 RSS、X、公众号，收进同一个源管理、阅读与输出层。摘要与翻译走你自己的 OpenAI 兼容接口，数据留在微服。</p>
        </div>
        <div className="login-river" aria-hidden="true">
          <i></i><i></i><i></i><i></i><i></i>
        </div>
        <div className="login-meta">
          <span>自托管</span>
          <span>单用户</span>
          <span>数据留在微服</span>
        </div>
      </section>

      <section className="login-panel" aria-label="登录">
        <div className="login-form-wrap">
          <div className="login-form-head">
            <span className="login-kicker">登录</span>
            <h2>进入 Rill</h2>
            <p>一个实例，一个人。没有注册入口。</p>
          </div>
          <form onSubmit={(event) => { event.preventDefault(); onLogin(); }}>
            <label className="login-field">
              <span>访问密码</span>
              <input
                type="password"
                value={password}
                onChange={(event) => onPassword(event.target.value)}
                placeholder="输入访问密码"
                autoFocus
                autoComplete="current-password"
              />
            </label>
            {error ? <div className="form-error" role="alert">{error}</div> : null}
            <Button variant="primary" size="lg" type="submit" className="login-submit" disabled={busy}>
              {busy ? '正在验证' : '进入 Rill'}
            </Button>
          </form>
          <div className="login-help">
            <Icon name="key" size={15} />
            <span>初始密码见首次启动日志；同 IP 失败 5 次锁定 5 分钟。</span>
          </div>
          <div className="login-status">
            <span><i></i>数据库可写</span>
            <span><i></i>调度器运行中</span>
            <span><i></i>{sources.length} 个源在线</span>
          </div>
          <div className="login-demo">
            <Tag tone="accent">原型</Tag>
            <span>输入任意非空密码即可进入</span>
          </div>
          <div className="login-version">Rill 0.1 · 设计稿 · 懒猫微服</div>
        </div>
      </section>
    </div>
  );
}

function ReaderScreen({
  items, sources, activeList, selectedItem, selectedId, settings,
  query, onQuery, searchOpen, onToggleSearch,
  onSelectItem, onOpenDetail, onToggleStar, onToggleRead, onMarkAllRead,
  onOpenHealth, onOpenSources, onGoInbox,
  translationMode, onTranslationMode, summaryOpen, onToggleSummary,
  onOpenOriginal, onCopyFeed, onGenerateSummary, aiBusy, onRetranslate,
  isMobile, mobileReading,
}) {
  return (
    <div className={cx('reader-screen', isMobile && mobileReading && 'is-mobile-reading')}>
      <ArticleList
        allItems={items}
        sources={sources}
        activeList={activeList}
        selectedId={selectedId}
        query={query}
        onQuery={onQuery}
        searchOpen={searchOpen}
        onToggleSearch={onToggleSearch}
        onSelect={onSelectItem}
        onOpenDetail={onOpenDetail}
        onToggleStar={onToggleStar}
        onToggleRead={onToggleRead}
        onMarkAllRead={onMarkAllRead}
        onOpenHealth={onOpenHealth}
        onOpenSources={onOpenSources}
        onGoInbox={onGoInbox}
        isMobile={isMobile}
      />
      <ReaderPane
        item={selectedItem}
        sources={sources}
        settings={settings}
        translationMode={translationMode}
        onTranslationMode={onTranslationMode}
        summaryOpen={summaryOpen}
        onToggleSummary={onToggleSummary}
        onToggleStar={onToggleStar}
        onToggleRead={onToggleRead}
        onOpenDetail={onOpenDetail}
        onOpenOriginal={onOpenOriginal}
        onCopyFeed={onCopyFeed}
        onOpenHealth={onOpenHealth}
        onGenerateSummary={onGenerateSummary}
        aiBusy={aiBusy}
        onRetranslate={onRetranslate}
        isMobile={isMobile}
      />
    </div>
  );
}

function App() {
  const shellRef = React.useRef(null);
  const containerWidth = useContainerWidth(shellRef);
  const isMobile = containerWidth > 0 && containerWidth <= 760;
  const isTablet = containerWidth > 0 && containerWidth <= 1040;

  const [screen, setScreen] = React.useState('login');
  const [device, setDevice] = React.useState(() => readStored('rill-proto-device', 'desktop'));
  const [settings, setSettings] = React.useState(() => Object.assign({}, DEFAULT_SETTINGS, readStored('rill-proto-settings', {})));
  const [sources, setSources] = React.useState(() => SOURCES.map((source) => Object.assign({}, source)));
  const [items, setItems] = React.useState(() => ITEMS.map((item) => Object.assign({}, item)));
  const [selectedId, setSelectedId] = React.useState('v1');
  const [activeList, setActiveList] = React.useState({ type: 'inbox' });
  const [query, setQuery] = React.useState('');
  const [searchOpen, setSearchOpen] = React.useState(false);
  const [collapsedFolders, setCollapsedFolders] = React.useState([]);
  const [drawerOpen, setDrawerOpen] = React.useState(false);
  const [mobileReading, setMobileReading] = React.useState(false);
  const [translationMode, setTranslationMode] = React.useState('zh');
  const [summaryOpen, setSummaryOpen] = React.useState(true);
  const [settingsTab, setSettingsTab] = React.useState('reading');
  const [addSourceOpen, setAddSourceOpen] = React.useState(false);
  const [healthSourceId, setHealthSourceId] = React.useState('designweekly');
  const [markAllTarget, setMarkAllTarget] = React.useState(null);
  const [toast, setToast] = React.useState(null);
  const [aiBusyId, setAiBusyId] = React.useState(null);
  const [loginPassword, setLoginPassword] = React.useState('');
  const [loginError, setLoginError] = React.useState('');
  const [loginBusy, setLoginBusy] = React.useState(false);
  const toastTimer = React.useRef(null);
  const readTimer = React.useRef(null);

  const selectedItem = selectedId ? items.find((item) => item.id === selectedId) || null : null;
  const healthSource = sources.find((source) => source.id === healthSourceId) || sources[0];
  const totalUnread = items.filter((item) => !item.read).length;
  const totalStarred = items.filter((item) => item.starred).length;
  const markAllUnreadCount = markAllTarget
    ? markAllTarget.filter((target) => {
      const current = items.find((item) => item.id === target.id);
      return current && !current.read;
    }).length
    : 0;

  React.useEffect(() => {
    try {
      window.localStorage.setItem('rill-proto-settings', JSON.stringify(settings));
      window.localStorage.setItem('rill-proto-device', JSON.stringify(device));
    } catch (error) {
      /* 忽略隐私模式下的存储失败 */
    }
  }, [settings, device]);

  React.useEffect(() => () => {
    window.clearTimeout(toastTimer.current);
    window.clearTimeout(readTimer.current);
  }, []);

  const clearReadTimer = () => {
    window.clearTimeout(readTimer.current);
    readTimer.current = null;
  };

  const scheduleRead = (id) => {
    clearReadTimer();
    if (!settings.autoRead) return;
    readTimer.current = window.setTimeout(() => {
      setItems((prev) => prev.map((item) => item.id === id ? Object.assign({}, item, { read: true }) : item));
      readTimer.current = null;
    }, 3000);
  };

  const notify = (message, action) => {
    window.clearTimeout(toastTimer.current);
    setToast({ message, action });
    toastTimer.current = window.setTimeout(() => setToast(null), action ? 6000 : 2800);
  };

  const toggleStar = (id) => {
    const item = items.find((entry) => entry.id === id);
    if (!item) return;
    setItems((prev) => prev.map((entry) => entry.id === id ? Object.assign({}, entry, { starred: !entry.starred }) : entry));
    notify(item.starred ? '已取消收藏' : '已收藏');
  };

  const toggleRead = (id) => {
    const item = items.find((entry) => entry.id === id);
    if (!item) return;
    setItems((prev) => prev.map((entry) => entry.id === id ? Object.assign({}, entry, { read: !entry.read }) : entry));
    notify(item.read ? '已标为未读' : '已标为已读');
  };

  const openDetail = (item) => {
    if (!item) return;
    clearReadTimer();
    if (!item.read) {
      setItems((prev) => prev.map((entry) => entry.id === item.id ? Object.assign({}, entry, { read: true }) : entry));
    }
    setSelectedId(item.id);
    setScreen('detail');
    setMobileReading(true);
  };

  const openHealth = (sourceId) => {
    setHealthSourceId(sourceId);
    setScreen('health');
    setDrawerOpen(false);
    setMobileReading(false);
  };

  const openSources = () => {
    setScreen('sources');
    setDrawerOpen(false);
    setMobileReading(false);
  };

  const openSettings = (tab) => {
    setSettingsTab(tab || 'reading');
    setScreen('settings');
    setDrawerOpen(false);
    setMobileReading(false);
  };

  const openAdd = () => {
    setAddSourceOpen(true);
  };

  const toggleSearch = () => {
    if (searchOpen) setQuery('');
    setSearchOpen(!searchOpen);
  };

  const goInbox = (filter) => {
    clearReadTimer();
    const next = filter || { type: 'inbox' };
    setActiveList(next);
    setScreen('reader');
    setDrawerOpen(false);
    setMobileReading(false);
    setQuery('');
    setSearchOpen(false);
    const nextItems = itemsForList(items, next, sources);
    setSelectedId(nextItems[0] ? nextItems[0].id : null);
  };

  const selectList = (next) => {
    clearReadTimer();
    setActiveList(next);
    setScreen('reader');
    setDrawerOpen(false);
    setMobileReading(false);
    setQuery('');
    setSearchOpen(false);
    const nextItems = itemsForList(items, next, sources);
    setSelectedId(nextItems[0] ? nextItems[0].id : null);
  };

  const selectItem = (item) => {
    setSelectedId(item.id);
    if (isMobile) setMobileReading(true);
    scheduleRead(item.id);
  };

  const toggleFolder = (folderId) => {
    setCollapsedFolders((prev) => prev.includes(folderId) ? prev.filter((id) => id !== folderId) : prev.concat(folderId));
  };

  const changeSettings = (next) => {
    const sidecarChanged = next.sidecarEnabled !== settings.sidecarEnabled;
    setSettings(next);
    if (sidecarChanged) {
      setSources((prev) => prev.map((source) => {
        if (source.kind !== 'wechat') return source;
        if (!next.sidecarEnabled) {
          return Object.assign({}, source, {
            enabled: false,
            status: 'paused',
            note: '公众号采集已关闭',
            nextFetch: '已暂停',
          });
        }
        const restored = source.lastError ? 'auth' : 'ok';
        return Object.assign({}, source, {
          enabled: true,
          status: restored,
          note: restored === 'auth' ? '需要重新扫码' : '登录态有效',
          nextFetch: restored === 'auth' ? '需要重新扫码' : '今天 13:10',
        });
      }));
      notify(next.sidecarEnabled ? '公众号采集已启用' : '公众号采集已关闭，其他源不受影响');
    }
  };

  const toggleTheme = () => {
    setSettings((prev) => Object.assign({}, prev, { theme: prev.theme === 'light' ? 'dark' : 'light' }));
  };

  const addSource = (data) => {
    const duplicate = sources.some((source) => (
      source.feedUrl === data.feedUrl
      || (data.kind === 'x' && source.kind === 'x' && source.name === data.name)
    ));
    if (duplicate) {
      notify('该源已在订阅列表中');
      return false;
    }
    const name = data.name || '新订阅源';
    const first = name.startsWith('@') ? name.slice(1, 2) : (name.match(/[\u4e00-\u9fa5A-Za-z0-9]/) || ['R'])[0];
    const id = 'user-' + Date.now().toString(36) + '-' + Math.random().toString(36).slice(2, 5);
    const source = {
      id,
      name,
      kind: data.kind || 'rss',
      folder: data.folder || '技术',
      icon: first.toUpperCase(),
      color: USER_COLORS[sources.length % USER_COLORS.length],
      url: data.url || data.feedUrl.replace(/^https?:\/\//, ''),
      feedUrl: data.feedUrl,
      siteUrl: data.siteUrl || '',
      enabled: true,
      status: 'ok',
      interval: data.interval || '1 小时',
      lastFetch: '等待首次抓取',
      lastNew: '等待首次抓取',
      nextFetch: '30 秒内',
      failCount: 0,
      lastError: '',
      note: '',
      fallback: '',
      itemCount: 0,
    };
    setSources((prev) => prev.concat(source));
    setScreen('sources');
    setDrawerOpen(false);
    setMobileReading(false);
    setActiveList({ type: 'inbox' });
    notify(`已添加「${name}」，30 秒内开始首次抓取`);
    return true;
  };

  const toggleEnabled = (id) => {
    const source = sources.find((entry) => entry.id === id);
    if (!source) return;
    setSources((prev) => prev.map((entry) => {
      if (entry.id !== id) return entry;
      const enabled = !entry.enabled;
      const wasPaused = entry.status === 'paused';
      return Object.assign({}, entry, {
        enabled,
        status: enabled ? (wasPaused ? 'ok' : entry.status) : 'paused',
        note: enabled ? (wasPaused ? '' : entry.note) : '已暂停',
        nextFetch: enabled ? '30 秒内' : '已暂停',
      });
    }));
    notify(source.enabled ? `已暂停「${source.name}」` : `已启用「${source.name}」`);
  };

  const refreshSource = (id) => {
    const source = sources.find((entry) => entry.id === id);
    if (!source) return;
    if (!source.enabled) {
      notify('该源已暂停，请先启用');
      return;
    }
    if (source.status === 'auth') {
      notify('登录态已过期，请先在设置中重新扫码');
      return;
    }
    setSources((prev) => prev.map((entry) => entry.id === id ? Object.assign({}, entry, {
      lastFetch: '刚刚',
      nextFetch: '30 分钟后',
      lastError: '',
    }) : entry));
    notify(source.status === 'warn' ? '抓取完成：仍无新条目，保留标黄' : '抓取完成：304，未发现新条目');
  };

  const switchFallback = (id) => {
    const source = sources.find((entry) => entry.id === id);
    if (!source) return;
    if (!source.fallback) {
      notify('这个源还没有配置备用地址');
      return;
    }
    setSources((prev) => prev.map((entry) => entry.id === id ? Object.assign({}, entry, {
      status: 'ok',
      note: '备用源已接管',
      lastFetch: '刚刚',
      nextFetch: '30 分钟后',
      lastError: '',
    }) : entry));
    notify(`已切换至备用源「${source.fallback}」`);
  };

  const editSource = (id, patch) => {
    setSources((prev) => prev.map((entry) => entry.id === id ? Object.assign({}, entry, patch) : entry));
    notify('源设置已更新');
  };

  const copyFeed = (sourceId, suffix) => {
    const source = sources.find((entry) => entry.id === sourceId);
    notify(`${source ? source.name : '源'} 的 ${suffix === 'atom' ? 'Atom' : suffix === 'json' ? 'JSON Feed' : 'RSS'} 地址已复制（原型）`);
  };

  const resetToken = (sourceId) => {
    const token = 'rill_' + Math.random().toString(36).slice(2, 6) + '…' + Math.random().toString(36).slice(2, 6);
    setSettings((prev) => Object.assign({}, prev, { feedToken: token }));
    notify('输出令牌已重置，旧地址立即失效');
  };

  const backupNow = () => {
    setSettings((prev) => Object.assign({}, prev, { lastBackup: '刚刚' }));
    notify('备份完成：数据库已写入持久化目录（原型）');
  };

  const restoreBackup = () => {
    notify('已打开文件选择器（原型）');
  };

  const scanSuccess = () => {
    if (!settings.sidecarEnabled) {
      notify('请先启用公众号采集');
      return;
    }
    setSettings((prev) => Object.assign({}, prev, {
      wechatStatus: 'ok',
      wechatExpires: '有效期至 11 月 6 日 18:00',
    }));
    setSources((prev) => prev.map((source) => source.kind === 'wechat' && source.status === 'auth'
      ? Object.assign({}, source, {
        status: 'ok',
        note: '登录态有效',
        lastError: '',
        nextFetch: '今天 13:10',
      })
      : source));
    notify('扫码成功，公众号抓取已恢复');
  };

  const deleteSource = (id) => {
    clearReadTimer();
    const source = sources.find((entry) => entry.id === id);
    setSources((prev) => prev.filter((entry) => entry.id !== id));
    setItems((prev) => prev.filter((item) => item.sourceId !== id));
    if (selectedItem && selectedItem.sourceId === id) setSelectedId(null);
    setScreen('sources');
    setMobileReading(false);
    notify(`已删除「${source ? source.name : '源'}」`);
  };

  const markAllRead = (targetItems) => {
    setMarkAllTarget(targetItems);
  };

  const confirmMarkAll = () => {
    if (!markAllTarget) return;
    const targetIds = markAllTarget.map((item) => item.id);
    const ids = targetIds.filter((id) => {
      const current = items.find((item) => item.id === id);
      return current && !current.read;
    });
    setItems((prev) => prev.map((item) => ids.includes(item.id) ? Object.assign({}, item, { read: true }) : item));
    setMarkAllTarget(null);
    notify(`已标记 ${ids.length} 条为已读`, {
      label: '撤销',
      onClick: () => {
        setItems((prev) => prev.map((item) => ids.includes(item.id) ? Object.assign({}, item, { read: false }) : item));
        setToast(null);
      },
    });
  };

  const generateSummary = (item) => {
    if (!item) return;
    if (!settings.aiEnabled) {
      notify('AI 能力已关闭，请先在设置中启用');
      return;
    }
    if (!settings.aiConsent) {
      notify('请先在设置中确认内容外发');
      return;
    }
    setAiBusyId(item.id);
    window.setTimeout(() => {
      setItems((prev) => prev.map((entry) => entry.id === item.id ? Object.assign({}, entry, {
        summary: {
          model: 'deepseek-chat',
          generatedAt: '刚刚',
          points: entry.summary ? entry.summary.points : [
            '这是根据正文生成的演示摘要：先判断内容是否值得读，再决定是否打开原文。',
            '工具的职责是减少选择，而不是制造更多未读。',
            '摘要失败不会影响条目入库，可以手动重跑。',
          ],
        },
      }) : entry));
      setAiBusyId(null);
      notify('摘要已生成（原型）');
    }, 1200);
  };

  const retranslate = () => {
    notify('已按段重新翻译（原型）');
  };

  const login = () => {
    if (!loginPassword.trim()) {
      setLoginError('请输入访问密码。');
      return;
    }
    setLoginError('');
    setLoginBusy(true);
    window.setTimeout(() => {
      setLoginBusy(false);
      setScreen('reader');
      setMobileReading(false);
      clearReadTimer();
      notify('已登录');
    }, 650);
  };

  const logout = () => {
    clearReadTimer();
    setScreen('login');
    setLoginPassword('');
    setLoginError('');
    setDrawerOpen(false);
    setMobileReading(false);
    notify('已退出登录');
  };

  const resetDemo = () => {
    try {
      window.localStorage.removeItem('rill-proto-settings');
      window.localStorage.removeItem('rill-proto-device');
    } catch (error) {
      /* ignore */
    }
    window.location.href = window.location.pathname;
  };

  React.useEffect(() => {
    const onKey = (event) => {
      const target = event.target;
      if (target && (['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName) || target.isContentEditable)) return;
      if (screen === 'detail' && event.key === 'Escape') {
        event.preventDefault();
        setScreen('reader');
        setMobileReading(false);
        return;
      }
      if (screen !== 'reader') return;
      const visible = itemsForList(items, activeList, sources).slice().sort((a, b) => b.ts - a.ts);
      const index = visible.findIndex((item) => item.id === selectedId);
      if (event.key === 'j' || event.key === 'ArrowDown') {
        event.preventDefault();
        const next = visible[Math.min(index + 1, visible.length - 1)] || visible[0];
        if (next) {
          setSelectedId(next.id);
          if (isMobile) setMobileReading(true);
          scheduleRead(next.id);
        }
      } else if (event.key === 'k' || event.key === 'ArrowUp') {
        event.preventDefault();
        const next = visible[Math.max(index - 1, 0)] || visible[0];
        if (next) {
          setSelectedId(next.id);
          if (isMobile) setMobileReading(true);
          scheduleRead(next.id);
        }
      } else if (event.key === 'm' && selectedId) {
        event.preventDefault();
        toggleRead(selectedId);
      } else if (event.key === 's' && selectedId) {
        event.preventDefault();
        toggleStar(selectedId);
      } else if ((event.key === 'o' || event.key === 'Enter') && selectedItem) {
        event.preventDefault();
        openDetail(selectedItem);
      } else if (event.key === 'r' && selectedItem) {
        event.preventDefault();
        refreshSource(selectedItem.sourceId);
      } else if (event.key === 'g') {
        event.preventDefault();
        if (visible[0]) {
          setSelectedId(visible[0].id);
          if (isMobile) setMobileReading(true);
          scheduleRead(visible[0].id);
        }
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [screen, items, activeList, sources, selectedId, selectedItem, isMobile, settings]);

  let topbar = null;
  if (screen !== 'login' && screen !== 'detail' && isTablet) {
    let title = '';
    let onBack = null;
    let onMenu = () => setDrawerOpen(true);
    let right = null;
    if (screen === 'reader') {
      if (isMobile && mobileReading && selectedItem) {
        title = sourceById(selectedItem.sourceId, sources).name;
        onBack = () => setMobileReading(false);
        onMenu = null;
        right = (
          <div className="mobile-reader-actions">
            <IconButton icon="external" label="在源站打开" size={17} onClick={() => notify('已在原型中打开源站（示意）')} />
            <IconButton
              icon="star"
              label={selectedItem.starred ? '取消收藏' : '收藏'}
              filled={selectedItem.starred}
              active={selectedItem.starred}
              size={17}
              onClick={() => toggleStar(selectedItem.id)}
            />
            <IconButton
              icon={selectedItem.read ? 'unread' : 'check'}
              label={selectedItem.read ? '标为未读' : '标为已读'}
              size={17}
              onClick={() => toggleRead(selectedItem.id)}
            />
            <IconButton icon="more" label="查看源诊断" size={17} onClick={() => openHealth(selectedItem.sourceId)} />
          </div>
        );
      } else {
        title = listTitleFor(activeList, sources);
        right = (
          <div className="mobile-reader-actions">
            <IconButton icon="search" label={searchOpen ? '关闭搜索' : '搜索'} size={18} active={searchOpen} onClick={toggleSearch} />
            <IconButton icon={settings.theme === 'light' ? 'moon' : 'sun'} label="切换主题" size={18} onClick={toggleTheme} />
          </div>
        );
      }
    } else if (screen === 'sources') {
      title = '订阅源';
      right = <IconButton icon="plus" label="添加源" size={19} onClick={openAdd} />;
    } else if (screen === 'health') {
      title = healthSource ? healthSource.name : '源诊断';
      onBack = () => setScreen('sources');
      onMenu = null;
      right = <IconButton icon="refresh" label="立即刷新" size={18} onClick={() => refreshSource(healthSource.id)} />;
    } else if (screen === 'settings') {
      title = '设置';
      right = <IconButton icon={settings.theme === 'light' ? 'moon' : 'sun'} label="切换主题" size={18} onClick={toggleTheme} />;
    }
    topbar = <MobileTopbar title={title} onMenu={onMenu} onBack={onBack} right={right} showBrand={screen === 'reader' && !(isMobile && mobileReading)} />;
  }

  const showBottomNav = isMobile && screen !== 'detail' && !(screen === 'reader' && mobileReading);
  const bottomNavActive = screen === 'sources' || screen === 'health'
    ? 'sources'
    : screen === 'settings'
      ? 'settings'
      : activeList.type === 'starred'
        ? 'starred'
        : 'inbox';

  return (
    <div className="prototype">
      <PrototypeBar
        screen={screen}
        onScreen={(next) => {
          setScreen(next);
          setDrawerOpen(false);
          setMobileReading(false);
          if (next === 'sources') setActiveList({ type: 'inbox' });
          if (next === 'health') setHealthSourceId(healthSource ? healthSource.id : 'designweekly');
          if (next === 'settings') setSettingsTab('reading');
        }}
        device={device}
        onDevice={setDevice}
        onReset={resetDemo}
      />

      <div className={cx('proto-canvas', device)}>
        <div className={cx('device-shell', device)}>
          <div
            ref={shellRef}
            className={cx('app-shell', `theme-${settings.theme}`)}
            data-theme={settings.theme}
            style={{
              '--reader-font-size': settings.fontSize + 'px',
              '--reader-line-height': settings.lineHeight,
              '--reader-line-width': settings.lineWidth + 'px',
            }}
          >
            {screen === 'login' ? (
              <LoginScreen
                password={loginPassword}
                onPassword={setLoginPassword}
                error={loginError}
                busy={loginBusy}
                onLogin={login}
                sources={sources}
                theme={settings.theme}
              />
            ) : (
              <div className="app-layout">
                {!isTablet ? (
                  <Sidebar
                    items={items}
                    sources={sources}
                    activeList={activeList}
                    onSelect={selectList}
                    collapsedFolders={collapsedFolders}
                    onToggleFolder={toggleFolder}
                    onOpenSettings={() => openSettings('reading')}
                    onOpenSources={openSources}
                    onOpenAdd={openAdd}
                    theme={settings.theme}
                    onToggleTheme={toggleTheme}
                  />
                ) : null}

                <div className="app-main">
                  {topbar}
                  <div className="screen-area">
                    {screen === 'reader' ? (
                      <ReaderScreen
                        items={items}
                        sources={sources}
                        activeList={activeList}
                        selectedItem={selectedItem}
                        selectedId={selectedId}
                        settings={settings}
                        query={query}
                        onQuery={setQuery}
                        searchOpen={searchOpen}
                        onToggleSearch={toggleSearch}
                        onSelectItem={selectItem}
                        onOpenDetail={openDetail}
                        onToggleStar={toggleStar}
                        onToggleRead={toggleRead}
                        onMarkAllRead={markAllRead}
                        onOpenHealth={openHealth}
                        onOpenSources={openSources}
                        onGoInbox={goInbox}
                        translationMode={translationMode}
                        onTranslationMode={setTranslationMode}
                        summaryOpen={summaryOpen}
                        onToggleSummary={() => setSummaryOpen(!summaryOpen)}
                        onOpenOriginal={() => notify('已在原型中打开源站（示意）')}
                        onCopyFeed={() => copyFeed(selectedItem ? selectedItem.sourceId : '', 'xml')}
                        onGenerateSummary={generateSummary}
                        aiBusy={aiBusyId === (selectedItem ? selectedItem.id : '')}
                        onRetranslate={retranslate}
                        isMobile={isMobile}
                        mobileReading={mobileReading}
                      />
                    ) : null}

                    {screen === 'detail' ? (
                      <FocusReader
                        item={selectedItem}
                        sources={sources}
                        settings={settings}
                        onChangeSettings={changeSettings}
                        translationMode={translationMode}
                        onTranslationMode={setTranslationMode}
                        summaryOpen={summaryOpen}
                        onToggleSummary={() => setSummaryOpen(!summaryOpen)}
                        onToggleStar={toggleStar}
                        onToggleRead={toggleRead}
                        onOpenOriginal={() => notify('已在原型中打开源站（示意）')}
                        onCopyFeed={() => copyFeed(selectedItem ? selectedItem.sourceId : '', 'xml')}
                        onOpenHealth={openHealth}
                        onGenerateSummary={generateSummary}
                        aiBusy={aiBusyId === (selectedItem ? selectedItem.id : '')}
                        onRetranslate={retranslate}
                        onBack={() => { setScreen('reader'); setMobileReading(false); }}
                        isMobile={isMobile}
                      />
                    ) : null}

                    {screen === 'sources' ? (
                      <SourcesScreen
                        sources={sources}
                        items={items}
                        onOpenHealth={openHealth}
                        onOpenAdd={openAdd}
                        onToggleEnabled={toggleEnabled}
                        onRefresh={refreshSource}
                        onGoSettings={openSettings}
                        onOpenDrawer={() => setDrawerOpen(true)}
                        isMobile={isMobile}
                      />
                    ) : null}

                    {screen === 'health' ? (
                      <HealthScreen
                        source={healthSource}
                        items={items}
                        onBack={() => setScreen('sources')}
                        onRefresh={refreshSource}
                        onToggleEnabled={toggleEnabled}
                        onSwitchFallback={switchFallback}
                        onCopyFeed={copyFeed}
                        onResetToken={resetToken}
                        onGoSettings={openSettings}
                        onDelete={deleteSource}
                        onEditSource={editSource}
                        isMobile={isMobile}
                      />
                    ) : null}

                    {screen === 'settings' ? (
                      <SettingsScreen
                        settings={settings}
                        onChange={changeSettings}
                        tab={settingsTab}
                        onTab={setSettingsTab}
                        sources={sources}
                        onOpenHealth={openHealth}
                        onGoSources={openSources}
                        onBackup={backupNow}
                        onRestore={restoreBackup}
                        onScan={scanSuccess}
                        onResetToken={resetToken}
                        onLogout={logout}
                        onOpenDrawer={() => setDrawerOpen(true)}
                        isMobile={isMobile}
                      />
                    ) : null}
                  </div>

                  {showBottomNav ? (
                    <MobileNav
                      active={bottomNavActive}
                      unreadCount={totalUnread}
                      starredCount={totalStarred}
                      onChange={(key) => {
                        if (key === 'inbox') goInbox({ type: 'inbox' });
                        else if (key === 'starred') goInbox({ type: 'starred' });
                        else if (key === 'sources') openSources();
                        else if (key === 'settings') openSettings('reading');
                      }}
                    />
                  ) : null}
                </div>

                {isTablet ? (
                  <Drawer open={drawerOpen} onClose={() => setDrawerOpen(false)}>
                    <Sidebar
                      items={items}
                      sources={sources}
                      activeList={activeList}
                      onSelect={selectList}
                      collapsedFolders={collapsedFolders}
                      onToggleFolder={toggleFolder}
                      onOpenSettings={() => openSettings('reading')}
                      onOpenSources={openSources}
                      onOpenAdd={openAdd}
                      theme={settings.theme}
                      onToggleTheme={toggleTheme}
                      inDrawer
                      onCloseDrawer={() => setDrawerOpen(false)}
                    />
                  </Drawer>
                ) : null}
              </div>
            )}

            <Toast
              message={toast ? toast.message : ''}
              actionLabel={toast && toast.action ? toast.action.label : ''}
              onAction={() => {
                if (toast && toast.action) toast.action.onClick();
                setToast(null);
              }}
            />

            <AddSourceModal
              open={addSourceOpen}
              onClose={() => setAddSourceOpen(false)}
              onAdd={addSource}
              settings={settings}
              onGoSettings={openSettings}
            />

            <Modal
              open={Boolean(markAllTarget)}
              onClose={() => setMarkAllTarget(null)}
              title="全部标为已读？"
              subtitle={markAllTarget ? `${markAllUnreadCount} 条未读` : ''}
              footer={
                <React.Fragment>
                  <span className="modal-foot-note">可在提示条中撤销</span>
                  <Button onClick={() => setMarkAllTarget(null)}>取消</Button>
                  <Button variant="primary" onClick={confirmMarkAll}>确认标记</Button>
                </React.Fragment>
              }
            >
              <p className="modal-copy">只影响当前筛选下的条目，不会删除内容。收藏状态保持不变。</p>
            </Modal>
          </div>
        </div>
        {device === 'phone' ? <div className="device-caption">390 × 844 · 可滚动</div> : null}
      </div>
    </div>
  );
}

const root = ReactDOM.createRoot(document.getElementById('root'));
root.render(<App />);
