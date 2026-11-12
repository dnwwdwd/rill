/* 源管理、添加源弹层与源诊断页。 */
function logsForSource(source) {
  if (source.id === 'designweekly') return FETCH_LOGS;
  const failed = source.status === 'auth';
  return [
    {
      time: source.lastFetch,
      result: failed ? 'error' : 'ok',
      status: failed ? '401' : '200',
      newCount: 0,
      duration: failed ? '218 ms' : '386 ms',
      note: failed ? source.lastError : '内容未变化，条目数 ' + source.itemCount,
    },
    { time: '上一周期', result: '304', status: '304', newCount: 0, duration: '201 ms', note: '命中 ETag，只更新抓取时间' },
    { time: '更早', result: 'ok', status: '200', newCount: 2, duration: '512 ms', note: '新增 2 条' },
  ];
}

function SourceRow({ source, unread, onOpenHealth, onToggleEnabled, onRefresh }) {
  return (
    <div
      className={cx('source-table-row', !source.enabled && 'is-paused')}
      role="button"
      tabIndex={0}
      onClick={() => onOpenHealth(source.id)}
      onKeyDown={(e) => { if (e.key === 'Enter') onOpenHealth(source.id); }}
    >
      <div className="source-cell-main">
        <SourceIcon source={source} size={28} />
        <div className="source-cell-copy">
          <div className="source-cell-title">
            <strong>{source.name}</strong>
            <Tag tone={source.kind === 'wechat' ? 'wechat' : source.kind === 'x' ? 'x' : 'neutral'}>
              {source.kind === 'wechat' ? '公众号' : source.kind === 'x' ? 'X' : 'RSS'}
            </Tag>
          </div>
          <span className="source-cell-url">{source.url}</span>
          {source.note && source.status !== 'ok' ? (
            <span className={cx('source-cell-note', source.status)}>{source.note}</span>
          ) : null}
        </div>
      </div>
      <div className="source-cell" data-label="文件夹">{source.folder}</div>
      <div className="source-cell" data-label="刷新间隔">{source.interval}</div>
      <div className="source-cell" data-label="上次抓取">{source.lastFetch}</div>
      <div className="source-cell" data-label="状态"><StatusBadge source={source} /></div>
      <div className="source-cell source-cell-unread" data-label="未读">
        {unread > 0 ? <span className="unread-number">{unread}</span> : <span className="muted">—</span>}
      </div>
      <div className="source-cell-actions" onClick={(e) => e.stopPropagation()}>
        <span className="switch-wrap">
          <Switch
            checked={source.enabled}
            onChange={() => onToggleEnabled(source.id)}
            label={source.enabled ? `暂停 ${source.name}` : `启用 ${source.name}`}
          />
        </span>
        <IconButton icon="refresh" label="立即刷新" size={16} onClick={() => onRefresh(source.id)} />
        <IconButton icon="chevron-right" label="查看诊断" size={16} onClick={() => onOpenHealth(source.id)} />
      </div>
    </div>
  );
}

function SourcesScreen({
  sources, items, onOpenHealth, onOpenAdd, onToggleEnabled, onRefresh,
  onGoSettings, onOpenDrawer, isMobile,
}) {
  const [filter, setFilter] = React.useState('all');
  const [query, setQuery] = React.useState('');
  const [folderFilter, setFolderFilter] = React.useState('all');

  const counts = {
    all: sources.length,
    ok: sources.filter((s) => s.status === 'ok').length,
    attention: sources.filter((s) => ['warn', 'auth', 'error'].includes(s.status)).length,
    paused: sources.filter((s) => !s.enabled).length,
  };
  const authSources = sources.filter((s) => s.status === 'auth');

  const filtered = sources.filter((source) => {
    if (filter === 'ok' && source.status !== 'ok') return false;
    if (filter === 'attention' && !['warn', 'auth', 'error'].includes(source.status)) return false;
    if (filter === 'paused' && source.enabled) return false;
    if (folderFilter !== 'all' && source.folder !== folderFilter) return false;
    if (query.trim()) {
      const q = query.trim().toLowerCase();
      return [source.name, source.url, source.folder].join(' ').toLowerCase().includes(q);
    }
    return true;
  });

  const groupFolders = FOLDERS.map((folder) => ({
    ...folder,
    rows: filtered.filter((source) => source.folder === folder.name),
  })).filter((folder) => folder.rows.length > 0);
  const orphanRows = filtered.filter((source) => !FOLDERS.some((folder) => folder.name === source.folder));

  return (
    <div className="main-pane sources-screen">
      <div className="page-scroll">
        <PageHeader
          eyebrow="源管理"
          title="订阅源"
          meta={`${sources.length} 个 · ${sources.filter((s) => s.enabled).length} 个启用`}
          actions={
            <Button variant="primary" icon="plus" onClick={onOpenAdd}>添加源</Button>
          }
        >
          <div className="source-summary">
            {[
              { key: 'all', label: '全部源', value: counts.all, tone: 'neutral' },
              { key: 'ok', label: '正常', value: counts.ok, tone: 'ok' },
              { key: 'attention', label: '需关注', value: counts.attention, tone: 'warn' },
              { key: 'paused', label: '已暂停', value: counts.paused, tone: 'paused' },
            ].map((item) => (
              <button
                type="button"
                key={item.key}
                className={cx('source-summary-item', filter === item.key && 'is-active')}
                onClick={() => setFilter(item.key)}
              >
                <span className={cx('summary-dot', item.tone)}></span>
                <span className="summary-label">{item.label}</span>
                <strong>{item.value}</strong>
              </button>
            ))}
          </div>
        </PageHeader>

        {authSources.length > 0 ? (
          <div className="notice notice-danger">
            <Icon name="lock" size={17} />
            <div className="notice-copy">
              <strong>{authSources.map((s) => `「${s.name}」`).join('、')}登录态已过期</strong>
              <span>公众号采集已暂停，其他源不受影响。重新扫码后会自动恢复。</span>
            </div>
            <Button size="sm" onClick={() => onGoSettings('wechat')}>去扫码</Button>
          </div>
        ) : null}

        <div className="source-toolbar">
          <div className="source-toolbar-left">
            <label className="inline-select">
              <span>文件夹</span>
              <select value={folderFilter} onChange={(e) => setFolderFilter(e.target.value)} aria-label="按文件夹筛选">
                <option value="all">全部</option>
                {FOLDERS.map((folder) => <option key={folder.id} value={folder.name}>{folder.name}</option>)}
              </select>
            </label>
          </div>
          <label className="source-search">
            <Icon name="search" size={16} />
            <input value={query} onChange={(e) => setQuery(e.target.value)} placeholder="搜索源名称或地址" aria-label="搜索订阅源" />
            {query ? <IconButton icon="x" label="清除" size={14} onClick={() => setQuery('')} /> : null}
          </label>
        </div>

        {groupFolders.length === 0 && orphanRows.length === 0 ? (
          <EmptyState
            icon="search"
            title="没有符合条件的源"
            description="调整筛选条件，或添加一个新的订阅源。"
            action={<Button size="sm" icon="plus" onClick={onOpenAdd}>添加源</Button>}
          />
        ) : null}

        <div className="source-table">
          <div className="source-table-head" aria-hidden="true">
            <span>订阅源</span>
            <span>文件夹</span>
            <span>刷新</span>
            <span>上次抓取</span>
            <span>状态</span>
            <span>未读</span>
            <span></span>
          </div>

          {groupFolders.map((folder) => (
            <div className="source-folder" key={folder.id}>
              <div className="source-folder-head">
                <span className="source-folder-index">{folder.index}</span>
                <h2>{folder.name}</h2>
                <span className="source-folder-count">{folder.rows.length}</span>
                <span className="source-folder-line"></span>
              </div>
              {folder.rows.map((source) => (
                <SourceRow
                  key={source.id}
                  source={source}
                  unread={items.filter((i) => i.sourceId === source.id && !i.read).length}
                  onOpenHealth={onOpenHealth}
                  onToggleEnabled={onToggleEnabled}
                  onRefresh={onRefresh}
                />
              ))}
            </div>
          ))}

          {orphanRows.length > 0 ? (
            <div className="source-folder">
              <div className="source-folder-head">
                <span className="source-folder-index">—</span>
                <h2>未分组</h2>
                <span className="source-folder-count">{orphanRows.length}</span>
                <span className="source-folder-line"></span>
              </div>
              {orphanRows.map((source) => (
                <SourceRow
                  key={source.id}
                  source={source}
                  unread={items.filter((i) => i.sourceId === source.id && !i.read).length}
                  onOpenHealth={onOpenHealth}
                  onToggleEnabled={onToggleEnabled}
                  onRefresh={onRefresh}
                />
              ))}
            </div>
          ) : null}
        </div>

        <div className="page-footnote">
          <Icon name="info" size={14} />
          <span>每个源都有独立的刷新间隔与输出令牌。图标抓取失败时显示首字占位，不影响抓取与健康状态。</span>
        </div>
      </div>
    </div>
  );
}

function AddSourceModal({ open, onClose, onAdd, settings, onGoSettings }) {
  const [tab, setTab] = React.useState('url');
  const [url, setUrl] = React.useState('');
  const [discovering, setDiscovering] = React.useState(false);
  const [candidates, setCandidates] = React.useState([]);
  const [selected, setSelected] = React.useState(-1);
  const [error, setError] = React.useState('');
  const [handle, setHandle] = React.useState('');
  const [withReplies, setWithReplies] = React.useState(false);
  const [mediaOnly, setMediaOnly] = React.useState(false);
  const [safeMode, setSafeMode] = React.useState(true);

  React.useEffect(() => {
    if (!open) return;
    setTab('url');
    setUrl('');
    setDiscovering(false);
    setCandidates([]);
    setSelected(-1);
    setError('');
    setHandle('');
    setWithReplies(false);
    setMediaOnly(false);
    setSafeMode(true);
  }, [open]);

  const handleValid = /^[A-Za-z0-9_]{1,30}$/.test(handle.replace(/^@/, ''));

  const discover = () => {
    if (!url.trim()) {
      setError('请输入站点地址或订阅地址。');
      return;
    }
    setError('');
    setDiscovering(true);
    setCandidates([]);
    setSelected(-1);
    window.setTimeout(() => {
      const raw = url.trim();
      const cleaned = raw.replace(/^https?:\/\//, '').replace(/\/+$/, '');
      const host = cleaned.split('/')[0] || cleaned;
      const looksLikeFeed = /\.(xml|atom|rss)(\?.*)?$/i.test(cleaned) || cleaned.includes('/feed');
      const next = looksLikeFeed
        ? [{ title: host, url: raw.startsWith('http') ? raw : 'https://' + raw, format: 'RSS 2.0' }]
        : [
          { title: host, url: 'https://' + cleaned + '/feed.xml', format: 'RSS 2.0' },
          { title: host, url: 'https://' + cleaned + '/atom.xml', format: 'Atom 1.0' },
          { title: host, url: 'https://' + cleaned + '/rss', format: 'RSS 2.0' },
        ];
      setCandidates(next);
      setSelected(0);
      setDiscovering(false);
    }, 850);
  };

  const submit = () => {
    let added = false;
    if (tab === 'url') {
      if (selected < 0 || !candidates[selected]) return;
      const candidate = candidates[selected];
      added = onAdd({
        kind: 'rss',
        name: candidate.title.split(' · ')[0],
        url: candidate.url.replace(/^https?:\/\//, '').replace(/\/feed\.xml$/, ''),
        feedUrl: candidate.url,
        folder: '技术',
        interval: '1 小时',
      });
    } else if (tab === 'x') {
      if (!handleValid) {
        setError('请输入有效的 X 账号，只允许字母、数字与下划线。');
        return;
      }
      const clean = handle.replace(/^@/, '');
      const feedFile = mediaOnly ? 'media.xml' : 'feed.xml';
      added = onAdd({
        kind: 'x',
        name: '@' + clean,
        url: 'fxtwitter.com/' + clean + '/' + feedFile,
        feedUrl: 'https://fxtwitter.com/' + clean + '/' + feedFile + '?count=20' + (withReplies ? '&with_replies=1' : '') + (safeMode ? '&safe=1' : ''),
        folder: '关注',
        interval: '30 分钟',
        siteUrl: 'https://x.com/' + clean,
      });
    }
    if (added !== false) onClose();
  };

  const canSubmit = (tab === 'url' && selected >= 0) || (tab === 'x' && handleValid) || tab === 'wechat';

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="添加订阅源"
      subtitle="先登记源，再决定它怎么被读。"
      wide
      footer={
        <React.Fragment>
          <span className="modal-foot-note">
            {tab === 'url' ? '首次只入库最近 20 条' : tab === 'x' ? '不登录 X，不填写 Cookie' : '强制扫码登录'}
          </span>
          <Button onClick={onClose}>取消</Button>
          <Button
            variant="primary"
            icon={tab === 'wechat' ? 'qr' : 'plus'}
            disabled={!canSubmit}
            onClick={() => {
              if (tab === 'wechat') {
                onClose();
                onGoSettings('wechat');
              } else {
                submit();
              }
            }}
          >
            {tab === 'wechat' ? '前往扫码' : '添加订阅'}
          </Button>
        </React.Fragment>
      }
    >
      <div className="add-tabs" role="tablist" aria-label="添加源类型">
        {[
          { key: 'url', label: '网址', hint: 'RSS / Atom / JSON Feed' },
          { key: 'x', label: 'X', hint: '账号转 RSS' },
          { key: 'wechat', label: '公众号', hint: '扫码登录' },
        ].map((item) => (
          <button
            key={item.key}
            type="button"
            role="tab"
            aria-selected={tab === item.key}
            className={cx('add-tab', tab === item.key && 'is-active')}
            onClick={() => { setTab(item.key); setError(''); }}
          >
            <span className="add-tab-label">{item.label}</span>
            <span className="add-tab-hint">{item.hint}</span>
          </button>
        ))}
      </div>

      {tab === 'url' ? (
        <div className="add-panel">
          <div className="discover-row">
            <label className="input-with-icon">
              <Icon name="link" size={16} />
              <input
                value={url}
                onChange={(e) => setUrl(e.target.value)}
                onKeyDown={(e) => { if (e.key === 'Enter') discover(); }}
                placeholder="粘贴博客首页或订阅地址"
                aria-label="站点地址"
                autoFocus
              />
            </label>
            <Button icon="search" onClick={discover} disabled={discovering}>
              {discovering ? '正在发现' : '发现订阅'}
            </Button>
          </div>
          <p className="add-hint">输入网站首页时，会解析 <code>&lt;link rel="alternate"&gt;</code>；也可以直接粘贴订阅地址。</p>
          {error ? <div className="form-error">{error}</div> : null}

          {discovering ? (
            <div className="discover-loading" aria-live="polite">
              <span></span><span></span><span></span>
            </div>
          ) : null}

          {candidates.length > 0 ? (
            <div className="discover-results" aria-live="polite">
              <div className="discover-results-head">
                <span>发现 {candidates.length} 个订阅地址</span>
                <span>选择要订阅的一个</span>
              </div>
              {candidates.map((candidate, index) => (
                <button
                  type="button"
                  key={candidate.url}
                  className={cx('candidate', selected === index && 'is-selected')}
                  onClick={() => setSelected(index)}
                >
                  <span className="candidate-radio" aria-hidden="true"></span>
                  <span className="candidate-copy">
                    <strong>{candidate.title}</strong>
                    <span>{candidate.url}</span>
                  </span>
                  <Tag tone="neutral">{candidate.format}</Tag>
                </button>
              ))}
            </div>
          ) : null}

          {!discovering && candidates.length === 0 ? (
            <div className="add-placeholder">
              <Icon name="rss" size={20} />
              <div>
                <strong>还没有选择订阅地址</strong>
                <span>先发现，再选择一个地址。发现不到时可以手动粘贴订阅地址。</span>
              </div>
            </div>
          ) : null}
        </div>
      ) : null}

      {tab === 'x' ? (
        <div className="add-panel">
          <label className="field">
            <span className="field-label">X 账号</span>
            <span className="handle-input">
              <span className="handle-prefix">@</span>
              <input
                value={handle}
                onChange={(e) => setHandle(e.target.value.replace(/^@/, ''))}
                placeholder="example_user"
                aria-label="X 账号"
                autoFocus
              />
            </span>
            <span className="field-hint">不需要 Cookie，也不需要接口密钥。抓取与普通 RSS 完全相同。</span>
          </label>

          <div className="x-preview">
            <div className="x-preview-label">将订阅</div>
            <code>https://fxtwitter.com/{handle || 'example_user'}/{mediaOnly ? 'media.xml' : 'feed.xml'}{withReplies ? '?with_replies=1' : ''}</code>
            <Tag tone="neutral">RSS 2.0</Tag>
          </div>

          <div className="option-list">
            <SwitchRow title="包含回复" description="with_replies=1，回复也会进入条目列表" checked={withReplies} onChange={setWithReplies} />
            <SwitchRow title="只要带媒体" description="仅保留带图片或视频的帖子" checked={mediaOnly} onChange={setMediaOnly} />
            <SwitchRow title="过滤敏感内容" description="safe=1，由公共实例按自身规则过滤" checked={safeMode} onChange={setSafeMode} />
          </div>

          <div className="add-note">
            <Icon name="info" size={15} />
            <span>首次只取最近 20 条；没有增量游标，后续靠 guid 去重。转推字段是否稳定仍待实测，v1 默认全部保留。</span>
          </div>
          {error ? <div className="form-error">{error}</div> : null}
        </div>
      ) : null}

      {tab === 'wechat' ? (
        <div className="add-panel">
          {settings.sidecarEnabled ? (
            <div className="wechat-connect">
              <div className="wechat-connect-head">
                <div>
                  <strong>公众号采集已启用</strong>
                  <span>唯一依赖 we-mp-rss 伴生服务，强制扫码登录。</span>
                </div>
                <StatusDot status={settings.wechatStatus === 'ok' ? 'ok' : 'auth'} />
              </div>
              <div className="wechat-qr-row">
                <div className="qr-mini" aria-label="二维码占位">
                  {Array.from({ length: 49 }).map((_, index) => <span key={index} className={index % 3 === 0 || index % 7 === 0 ? 'on' : ''}></span>)}
                </div>
                <div className="wechat-qr-copy">
                  <strong>{settings.wechatStatus === 'ok' ? '登录态有效' : '需要重新扫码'}</strong>
                  <span>{settings.wechatStatus === 'ok' ? settings.wechatExpires : '登录态过期后，抓取会暂停，不会静默失败。'}</span>
                  <Button size="sm" icon="qr" onClick={() => { onClose(); onGoSettings('wechat'); }}>打开扫码页</Button>
                </div>
              </div>
            </div>
          ) : (
            <EmptyState
              icon="lock"
              title="公众号采集未启用"
              description="公众号是可选能力，关闭时主应用保持轻量。启用后仍需扫码登录，不提供无登录路径。"
              action={<Button size="sm" variant="primary" onClick={() => { onClose(); onGoSettings('wechat'); }}>前往设置启用</Button>}
            />
          )}
          <div className="add-note">
            <Icon name="info" size={15} />
            <span>不依赖 werss / WeWe RSS；不接入公开专辑、主页或 Wechat2RSS 作为产品功能。</span>
          </div>
        </div>
      ) : null}
    </Modal>
  );
}

function FetchResultBadge({ result }) {
  const meta = result === 'ok' ? { label: '成功', tone: 'ok' } : result === '304' ? { label: '未修改', tone: 'neutral' } : { label: '失败', tone: 'danger' };
  return <span className={cx('fetch-result', meta.tone)}><i></i>{meta.label}</span>;
}

function HealthScreen({
  source, items, onBack, onRefresh, onToggleEnabled, onSwitchFallback,
  onCopyFeed, onResetToken, onGoSettings, onDelete, onEditSource, isMobile,
}) {
  const [confirmDelete, setConfirmDelete] = React.useState(false);
  const [tokenNonce, setTokenNonce] = React.useState(0);
  const [editOpen, setEditOpen] = React.useState(false);
  const [editFocusFallback, setEditFocusFallback] = React.useState(false);
  const [editForm, setEditForm] = React.useState({ name: '', folder: '技术', interval: '1 小时', fallback: '' });
  if (!source) {
    return (
      <div className="main-pane">
        <EmptyState icon="server" title="没有选中的源" description="回到订阅源列表选择一个源。" action={<Button onClick={onBack}>返回源管理</Button>} />
      </div>
    );
  }
  const logs = logsForSource(source);
  const unread = items.filter((i) => i.sourceId === source.id && !i.read).length;
  const isWarn = source.status === 'warn';
  const isAuth = source.status === 'auth';
  const maskedToken = settingsTokenMask(source.id + (tokenNonce ? '-' + tokenNonce : ''));

  const openEdit = (focusFallback = false) => {
    setEditForm({ name: source.name, folder: source.folder, interval: source.interval, fallback: source.fallback });
    setEditFocusFallback(focusFallback);
    setEditOpen(true);
  };

  const saveEdit = () => {
    onEditSource(source.id, {
      name: editForm.name.trim() || source.name,
      folder: editForm.folder,
      interval: editForm.interval,
      fallback: editForm.fallback.trim(),
    });
    setEditOpen(false);
  };

  return (
    <div className="main-pane health-screen">
      <div className="page-scroll">
        <header className="health-head">
          <div className="health-head-main">
            <button type="button" className="back-link" onClick={onBack}>
              <Icon name="arrow-left" size={15} /> 订阅源
            </button>
            <div className="health-identity">
              <SourceIcon source={source} size={44} />
              <div>
                <div className="health-title-row">
                  <h1>{source.name}</h1>
                  <StatusBadge source={source} />
                </div>
                <div className="health-meta">
                  <span>{source.url}</span>
                  <span className="dot-sep">·</span>
                  <span>{source.interval} 刷新</span>
                  <span className="dot-sep">·</span>
                  <span>{unread} 条未读</span>
                </div>
              </div>
            </div>
          </div>
          <div className="health-actions">
            <Button variant="primary" icon="refresh" onClick={() => onRefresh(source.id)}>立即刷新</Button>
            <Button icon="sliders" onClick={() => openEdit(false)}>编辑</Button>
            <Button icon="plus" onClick={() => openEdit(true)}>设置备用源</Button>
            <span className="health-switch">
              <span>启用</span>
              <Switch checked={source.enabled} onChange={() => onToggleEnabled(source.id)} label="启用源" />
            </span>
          </div>
        </header>

        {isWarn ? (
          <div className="notice notice-warn">
            <Icon name="alert-triangle" size={17} />
            <div className="notice-copy">
              <strong>可能已停止更新</strong>
              <span>距离上次新条目已 {source.lastNew.replace('前', '')}，超过 2× 刷新间隔。源站可能挂了，也可能只是暂时没有新内容。</span>
            </div>
            <Button size="sm" onClick={() => onSwitchFallback(source.id)} disabled={!source.fallback}>切换备用源</Button>
          </div>
        ) : null}
        {isAuth ? (
          <div className="notice notice-danger">
            <Icon name="lock" size={17} />
            <div className="notice-copy">
              <strong>登录态已过期，抓取已暂停</strong>
              <span>{source.lastError}。重新扫码后会从中断处继续，不影响其他源。</span>
            </div>
            <Button size="sm" onClick={() => onGoSettings('wechat')}>去扫码</Button>
          </div>
        ) : null}
        {!source.enabled ? (
          <div className="notice notice-paused">
            <Icon name="pause" size={17} />
            <div className="notice-copy">
              <strong>该源已暂停</strong>
              <span>暂停只跳过调度，不会删除已有条目与收藏。</span>
            </div>
            <Button size="sm" onClick={() => onToggleEnabled(source.id)}>重新启用</Button>
          </div>
        ) : null}

        <div className="health-stats">
          {[
            { label: '上次抓取', value: source.lastFetch, icon: 'clock' },
            { label: '抓取耗时', value: isWarn ? '15.0 秒' : isAuth ? '218 毫秒' : '386 毫秒', icon: 'refresh' },
            { label: '连续失败', value: source.failCount + ' 次', icon: 'alert-circle', tone: source.failCount > 0 ? 'warn' : '' },
            { label: '下次抓取', value: source.nextFetch, icon: 'calendar' },
            { label: '上次新条目', value: source.lastNew, icon: 'rss', tone: isWarn ? 'warn' : '' },
            { label: '条目总数', value: source.itemCount.toLocaleString('zh-CN'), icon: 'database' },
          ].map((stat) => (
            <div className={cx('health-stat', stat.tone)} key={stat.label}>
              <div className="health-stat-label"><Icon name={stat.icon} size={14} />{stat.label}</div>
              <strong>{stat.value}</strong>
            </div>
          ))}
        </div>

        <div className="health-grid">
          <section className="health-card health-chart-card">
            <div className="health-card-head">
              <div>
                <div className="eyebrow">抓取趋势</div>
                <h2>近 14 天新增</h2>
              </div>
              <span className="chart-total">{NEW_ITEM_TREND.reduce((a, b) => a + b, 0)} 条</span>
            </div>
            <MiniBars values={NEW_ITEM_TREND} label="近 14 天每日新增条目数" />
            <div className="chart-axis"><span>14 天前</span><span>今天</span></div>
          </section>

          <section className="health-card health-silent-card">
            <div className="health-card-head">
              <div>
                <div className="eyebrow">静默失败检测</div>
                <h2>{isWarn ? '已超过阈值' : isAuth ? '无法判定' : '状态正常'}</h2>
              </div>
              <StatusDot status={source.status} />
            </div>
            <dl className="silent-list">
              <div><dt>刷新间隔</dt><dd>{source.interval}</dd></div>
              <div><dt>距上次新条目</dt><dd>{source.lastNew}</dd></div>
              <div><dt>判定阈值</dt><dd>2× 刷新间隔</dd></div>
              <div><dt>当前判定</dt><dd className={isWarn ? 'warn-text' : isAuth ? 'danger-text' : 'ok-text'}>{isWarn ? '可能已停止更新' : isAuth ? '登录态过期' : '正常'}</dd></div>
            </dl>
          </section>
        </div>

        <div className="health-grid health-grid-lower">
          <section className="health-card">
            <div className="health-card-head">
              <div>
                <div className="eyebrow">抓取历史</div>
                <h2>最近 {logs.length} 次</h2>
              </div>
              <TextLink onClick={() => onRefresh(source.id)}>重新抓取</TextLink>
            </div>
            <div className="fetch-log" role="table" aria-label="抓取历史">
              <div className="fetch-log-head" role="row">
                <span>时间</span><span>结果</span><span>状态码</span><span>新增</span><span>耗时</span><span>说明</span>
              </div>
              {logs.map((log, index) => (
                <div className="fetch-log-row" role="row" key={index}>
                  <span data-label="时间">{log.time}</span>
                  <span data-label="结果"><FetchResultBadge result={log.result} /></span>
                  <span data-label="状态码"><code>{log.status}</code></span>
                  <span data-label="新增">{log.newCount}</span>
                  <span data-label="耗时"><code>{log.duration}</code></span>
                  <span data-label="说明" className={log.result === 'error' ? 'log-error' : ''}>{log.note}</span>
                </div>
              ))}
            </div>
          </section>

          <div className="health-side">
            <section className="health-card">
              <div className="health-card-head">
                <div>
                  <div className="eyebrow">备用源</div>
                  <h2>{source.fallback || '未配置'}</h2>
                </div>
                <Icon name="shield" size={18} className="card-icon" />
              </div>
              <p className="health-card-copy">
                {source.fallback
                  ? '主源连续失败或静默停更时可手动切换一次；切换会写入抓取日志，不覆盖已有条目。'
                  : '为此源配置一个备用订阅地址，主源失败时可在诊断页切换。'}
              </p>
              <div className="health-side-actions">
                <Button size="sm" icon="refresh" onClick={() => onSwitchFallback(source.id)} disabled={!source.fallback}>切换备用源</Button>
                <Button size="sm" icon="plus" onClick={() => openEdit(true)}>配置</Button>
              </div>
            </section>

            <section className="health-card">
              <div className="health-card-head">
                <div>
                  <div className="eyebrow">输出与鉴权</div>
                  <h2>订阅地址</h2>
                </div>
                <Icon name="rss" size={18} className="card-icon" />
              </div>
              <div className="feed-output-list">
                {[
                  { label: 'RSS 2.0', suffix: 'xml' },
                  { label: 'Atom 1.0', suffix: 'atom' },
                  { label: 'JSON Feed', suffix: 'json' },
                ].map((fmt) => (
                  <div className="feed-output-row" key={fmt.suffix}>
                    <span className="feed-output-label">{fmt.label}</span>
                    <code>/feed/{source.id}/{maskedToken}.{fmt.suffix}</code>
                    <IconButton icon="copy" label={`复制 ${fmt.label} 地址`} size={15} onClick={() => onCopyFeed(source.id, fmt.suffix)} />
                  </div>
                ))}
              </div>
              <div className="token-row">
                <div>
                  <span className="token-label">输出令牌</span>
                  <code>{maskedToken}</code>
                </div>
                <Button size="sm" onClick={() => { onResetToken(source.id); setTokenNonce((n) => n + 1); }}>重置</Button>
              </div>
              <p className="health-card-copy">不可猜测的令牌地址；重置后旧地址立即 404。媒体走独立令牌，不暴露源站域名。</p>
            </section>
          </div>
        </div>

        <section className="danger-zone">
          <div>
            <div className="eyebrow">危险操作</div>
            <h2>删除订阅源</h2>
            <p>会删除该源的条目、收藏与图标缓存，不影响其他源。此操作不可恢复。</p>
          </div>
          <Button variant="danger" icon="trash" onClick={() => setConfirmDelete(true)}>删除源</Button>
        </section>

        <div className="page-footnote">
          <Icon name="info" size={14} />
          <span>抓取失败按指数退避；连续失败 5 次自动暂停调度，并在此页标红。单源失败不会影响其他源。</span>
        </div>
      </div>

      <Modal
        open={confirmDelete}
        onClose={() => setConfirmDelete(false)}
        title="删除订阅源？"
        subtitle={source.name}
        footer={
          <React.Fragment>
            <span className="modal-foot-note">删除后不可恢复</span>
            <Button onClick={() => setConfirmDelete(false)}>取消</Button>
            <Button variant="danger" onClick={() => { setConfirmDelete(false); onDelete(source.id); }}>确认删除</Button>
          </React.Fragment>
        }
      >
        <p className="modal-copy">将删除 {source.itemCount} 条条目记录、阅读状态、收藏与图标缓存。其他源和订阅输出不受影响。</p>
      </Modal>

      <Modal
        open={editOpen}
        onClose={() => setEditOpen(false)}
        title="编辑订阅源"
        subtitle={source.name}
        footer={
          <React.Fragment>
            <span className="modal-foot-note">修改会在下一次调度时生效</span>
            <Button onClick={() => setEditOpen(false)}>取消</Button>
            <Button variant="primary" onClick={saveEdit}>保存修改</Button>
          </React.Fragment>
        }
      >
        <div className="edit-source-form">
          <Field label="源名称">
            <input value={editForm.name} onChange={(e) => setEditForm(Object.assign({}, editForm, { name: e.target.value }))} aria-label="源名称" />
          </Field>
          <Field label="文件夹">
            <select value={editForm.folder} onChange={(e) => setEditForm(Object.assign({}, editForm, { folder: e.target.value }))} aria-label="文件夹">
              {FOLDERS.map((folder) => <option key={folder.id} value={folder.name}>{folder.name}</option>)}
            </select>
          </Field>
          <Field label="刷新间隔">
            <select value={editForm.interval} onChange={(e) => setEditForm(Object.assign({}, editForm, { interval: e.target.value }))} aria-label="刷新间隔">
              {['15 分钟', '30 分钟', '1 小时', '2 小时', '6 小时', '12 小时'].map((value) => <option key={value} value={value}>{value}</option>)}
            </select>
          </Field>
          <Field label="首次回溯上限" hint="新源只入库最近 N 条，避免一次灌入太多历史。">
            <input value="20 条" readOnly aria-label="首次回溯上限" />
          </Field>
          <Field label="备用源" hint="可填名称或订阅地址；留空表示不配置。主源静默停更时可手动切换。" className="span-2">
            <input
              value={editForm.fallback}
              onChange={(e) => setEditForm(Object.assign({}, editForm, { fallback: e.target.value }))}
              placeholder="名称或订阅地址"
              aria-label="备用源"
              autoFocus={editFocusFallback}
            />
          </Field>
        </div>
      </Modal>
    </div>
  );
}

function settingsTokenMask(sourceId) {
  const suffix = String(sourceId).replace(/[^a-z0-9]/gi, '').slice(0, 3) || 'src';
  return 'rill_' + suffix + '…7a2d';
}

Object.assign(window, {
  logsForSource, SourceRow, SourcesScreen, AddSourceModal, FetchResultBadge, HealthScreen, settingsTokenMask,
});
