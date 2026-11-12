/* 阅读列表、阅读栏与专注阅读。 */
function itemsForList(items, activeList, sources) {
  switch (activeList.type) {
    case 'unread':
      return items.filter((i) => !i.read);
    case 'starred':
      return items.filter((i) => i.starred);
    case 'read':
      return items.filter((i) => i.read);
    case 'source':
      return items.filter((i) => i.sourceId === activeList.sourceId);
    case 'folder': {
      const ids = sourceIdsInFolder(activeList.folder, sources);
      return items.filter((i) => ids.includes(i.sourceId));
    }
    default:
      return items;
  }
}

function listTitleFor(activeList, sources) {
  if (activeList.type === 'source') return sourceById(activeList.sourceId, sources).name;
  if (activeList.type === 'folder') return activeList.folder;
  if (activeList.type === 'unread') return '未读';
  if (activeList.type === 'starred') return '收藏';
  if (activeList.type === 'read') return '已读';
  return '收件箱';
}

function listEyebrowFor(activeList) {
  if (activeList.type === 'source') return '订阅源';
  if (activeList.type === 'folder') return '文件夹';
  return '阅读';
}

function ArticleRow({ item, sources, selected, onSelect, onOpenDetail, onToggleStar, onToggleRead }) {
  const source = sourceById(item.sourceId, sources);
  return (
    <article
      className={cx('item-row', selected && 'is-selected', !item.read && 'is-unread')}
      onClick={() => onSelect(item)}
      onDoubleClick={() => onOpenDetail(item)}
      tabIndex={0}
      onKeyDown={(e) => {
        if (e.key === 'Enter') onSelect(item);
      }}
      aria-current={selected ? 'true' : undefined}
    >
      <span className="item-unread-dot" aria-hidden="true"></span>
      <div className="item-top">
        <span className="item-source">
          <SourceIcon source={source} size={14} />
          <span>{source.name}</span>
        </span>
        {item.badge ? <Tag tone={item.badge === '公众号' ? 'wechat' : item.badge === 'X' ? 'x' : 'accent'}>{item.badge}</Tag> : null}
        <time className="item-time">{item.published}</time>
      </div>
      <h2 className="item-title">{item.title}</h2>
      <p className="item-excerpt">{item.excerpt}</p>
      <div className="item-foot">
        <span className="item-minutes">{item.minutes} 分钟</span>
        {item.bodyEn ? <Tag tone="neutral">有译文</Tag> : null}
        <span className="item-foot-spacer"></span>
        <div className="item-actions">
          <IconButton
            icon="external"
            label="打开详情"
            size={15}
            onClick={(e) => { e.stopPropagation(); onOpenDetail(item); }}
          />
          <IconButton
            icon="star"
            label={item.starred ? '取消收藏' : '收藏'}
            size={15}
            filled={item.starred}
            active={item.starred}
            onClick={(e) => { e.stopPropagation(); onToggleStar(item.id); }}
          />
          <IconButton
            icon={item.read ? 'unread' : 'check'}
            label={item.read ? '标为未读' : '标为已读'}
            size={15}
            onClick={(e) => { e.stopPropagation(); onToggleRead(item.id); }}
          />
        </div>
      </div>
    </article>
  );
}

function ArticleList({
  allItems, sources, activeList, selectedId, query, onQuery, searchOpen, onToggleSearch,
  onSelect, onOpenDetail, onToggleStar, onToggleRead, onMarkAllRead,
  onOpenHealth, onOpenSources, onGoInbox, isMobile,
}) {
  const [sortDesc, setSortDesc] = React.useState(true);
  let items = itemsForList(allItems, activeList, sources);
  if (query.trim()) {
    const q = query.trim().toLowerCase();
    items = items.filter((item) => {
      const source = sourceById(item.sourceId, sources);
      return [item.title, item.excerpt, item.author, source.name, item.originalTitle || '']
        .join(' ').toLowerCase().includes(q);
    });
  }
  items = [...items].sort((a, b) => (sortDesc ? b.ts - a.ts : a.ts - b.ts));

  const title = listTitleFor(activeList, sources);
  const unread = items.filter((i) => !i.read).length;
  const activeSource = activeList.type === 'source' ? sourceById(activeList.sourceId, sources) : null;
  const sourceAlert = activeSource && (activeSource.status === 'warn' || activeSource.status === 'auth');

  const setFilter = (type) => {
    onQuery('');
    if (type === 'all') onGoInbox();
    else onGoInbox({ type });
  };

  return (
    <section className="list-pane" aria-label="条目列表">
      <header className="list-head">
        <div className="list-eyebrow">{listEyebrowFor(activeList)}</div>
        <div className="list-title-row">
          <h1>{title}</h1>
          <span className="list-count">{items.length}</span>
        </div>
        <div className="list-sub">
          {activeSource ? (
            <React.Fragment>
              <span>{activeSource.interval} 刷新</span>
              <span className="dot-sep">·</span>
              <span>上次抓取 {activeSource.lastFetch}</span>
              <span className="dot-sep">·</span>
              <span>{unread} 条未读</span>
            </React.Fragment>
          ) : (
            <React.Fragment>
              <span>{items.length} 条</span>
              <span className="dot-sep">·</span>
              <span>{unread} 条未读</span>
              {activeList.type !== 'inbox' ? (
                <React.Fragment>
                  <span className="dot-sep">·</span>
                  <TextLink onClick={() => onGoInbox()}>返回收件箱</TextLink>
                </React.Fragment>
              ) : null}
            </React.Fragment>
          )}
        </div>

        <div className="list-tools">
          <div className="list-chips" role="group" aria-label="筛选">
            {[
              { key: 'all', label: '全部' },
              { key: 'unread', label: '未读' },
              { key: 'starred', label: '收藏' },
              { key: 'read', label: '已读' },
            ].map((chip) => {
              const active = chip.key === 'all'
                ? activeList.type === 'inbox'
                : activeList.type === chip.key;
              return (
                <button
                  type="button"
                  key={chip.key}
                  className={cx('chip', active && 'is-active')}
                  aria-pressed={active}
                  onClick={() => setFilter(chip.key)}
                >
                  {chip.label}
                </button>
              );
            })}
          </div>
          <div className="list-tools-right">
            <IconButton
              icon="search"
              label={searchOpen ? '关闭搜索' : '搜索'}
              size={17}
              active={searchOpen}
              onClick={onToggleSearch}
            />
            <button type="button" className="sort-btn" onClick={() => setSortDesc(!sortDesc)} title="切换时间排序">
              <span>按发布时间</span>
              <Icon name="chevron-down" size={13} className={sortDesc ? '' : 'flip'} />
            </button>
          </div>
        </div>

        {searchOpen ? (
          <div className="list-search">
            <Icon name="search" size={16} />
            <input
              autoFocus
              value={query}
              onChange={(e) => onQuery(e.target.value)}
              placeholder="搜索标题、作者或源"
              aria-label="搜索条目"
            />
            {query ? <IconButton icon="x" label="清除搜索" size={15} onClick={() => onQuery('')} /> : null}
          </div>
        ) : null}

        {sourceAlert ? (
          <div className={cx('list-alert', activeSource.status)}>
            <Icon name={activeSource.status === 'auth' ? 'lock' : 'alert-triangle'} size={16} />
            <span>
              {activeSource.status === 'auth'
                ? '公众号登录态已过期，抓取已暂停。'
                : `已 ${activeSource.lastNew.replace('前', '')}没有新条目，可能已停止更新。`}
            </span>
            <TextLink onClick={() => onOpenHealth(activeSource.id)}>查看诊断</TextLink>
          </div>
        ) : null}
      </header>

      <div className="list-scroll">
        {unread > 0 && activeList.type !== 'read' ? (
          <div className="list-batch">
            <span>{unread} 条未读</span>
            <TextLink onClick={() => onMarkAllRead(items)}>全部标为已读</TextLink>
          </div>
        ) : null}

        {items.length === 0 ? (
          <EmptyState
            icon={query ? 'search' : 'inbox'}
            title={query ? '没有匹配的条目' : '这里还没有内容'}
            description={query ? '试试更短的关键词，或搜索源名称。' : '添加一个订阅源，或切换到其他筛选。'}
            action={
              <div className="empty-actions">
                {query ? <Button size="sm" onClick={() => onQuery('')}>清除搜索</Button> : null}
                <Button size="sm" variant="primary" icon="plus" onClick={onOpenSources}>添加订阅源</Button>
              </div>
            }
          />
        ) : (
          <div className="item-list">
            {items.map((item) => (
              <ArticleRow
                key={item.id}
                item={item}
                sources={sources}
                selected={selectedId === item.id}
                onSelect={onSelect}
                onOpenDetail={onOpenDetail}
                onToggleStar={onToggleStar}
                onToggleRead={onToggleRead}
              />
            ))}
          </div>
        )}

        {items.length > 0 ? (
          <div className="list-end">
            <span className="list-end-line"></span>
            <span>已显示 {items.length} 条</span>
            <span className="list-end-line"></span>
          </div>
        ) : null}
      </div>
    </section>
  );
}

function ArticleHead({ item, source, focus = false }) {
  return (
    <header className={cx('article-head', focus && 'is-focus')}>
      <div className="article-kicker">
        <SourceIcon source={source} size={focus ? 24 : 20} />
        <span className="article-source-name">{source.name}</span>
        <span className="dot-sep">·</span>
        <time>{item.published}</time>
        {item.badge ? <Tag tone={item.badge === '公众号' ? 'wechat' : item.badge === 'X' ? 'x' : 'accent'}>{item.badge}</Tag> : null}
      </div>
      <h1 className="article-title">{item.title}</h1>
      {item.originalTitle ? (
        <p className="article-original">原文标题：{item.originalTitle}</p>
      ) : null}
      <div className="article-byline">
        <span>{item.author}</span>
        <span className="dot-sep">·</span>
        <span>约 {item.minutes} 分钟</span>
        {item.translation ? (
          <React.Fragment>
            <span className="dot-sep">·</span>
            <span className="translation-meta"><Icon name="translate" size={13} /> 译文由 {item.translation.model} 生成</span>
          </React.Fragment>
        ) : null}
      </div>
    </header>
  );
}

function AISummary({ item, open, onToggle, style = 'bullets', onGenerate, busy = false, className = '' }) {
  const hasSummary = Boolean(item.summary);
  return (
    <section className={cx('ai-summary', open && 'is-open', !hasSummary && 'is-empty', className)}>
      <button type="button" className="ai-summary-head" onClick={onToggle} aria-expanded={open}>
        <span className="ai-summary-label"><Icon name="sparkles" size={15} /> AI 摘要</span>
        {hasSummary ? (
          <span className="ai-summary-meta">{item.summary.model} · {item.summary.generatedAt}</span>
        ) : (
          <span className="ai-summary-meta">尚未生成</span>
        )}
        <Icon name="chevron-down" size={14} className={cx('ai-chevron', open && 'is-open')} />
      </button>
      {open ? (
        <div className="ai-summary-body">
          {busy ? (
            <div className="ai-skeleton" aria-label="正在生成摘要">
              <span></span><span></span><span></span>
            </div>
          ) : hasSummary ? (
            <React.Fragment>
              {style === 'one' ? (
                <p className="ai-summary-one">{item.summary.points[0]}</p>
              ) : (
                <ol className="ai-points">
                  {item.summary.points.map((point, index) => <li key={index}>{point}</li>)}
                </ol>
              )}
              <div className="ai-summary-foot">
                <span>内容已发送至外部模型 · 可手动重跑</span>
                <TextLink onClick={onGenerate}>重新生成</TextLink>
              </div>
            </React.Fragment>
          ) : (
            <div className="ai-empty">
              <p>这篇内容还没有摘要。生成会调用你配置的 OpenAI 兼容接口。</p>
              <Button size="sm" variant="primary" icon="sparkles" onClick={onGenerate}>生成摘要</Button>
            </div>
          )}
        </div>
      ) : null}
    </section>
  );
}

function TranslationPanel({ item, mode, onMode, onRetranslate, compact = false, className = '' }) {
  if (!item.bodyEn) return null;
  return (
    <section className={cx('translation-panel', compact && 'is-compact', className)}>
      <div className="translation-head">
        <span className="translation-label"><Icon name="translate" size={15} /> 翻译</span>
        <Segmented
          ariaLabel="翻译显示模式"
          value={mode}
          onChange={onMode}
          options={[
            { value: 'zh', label: '译文' },
            { value: 'en', label: '原文' },
            { value: 'both', label: '双语' },
          ]}
        />
      </div>
      <div className="translation-meta-line">
        <span>{item.translation ? `${item.translation.originalLang} → ${item.translation.target}` : '英文 → 简体中文'}</span>
        <span className="dot-sep">·</span>
        <span>{item.translation ? item.translation.model : 'deepseek-chat'}</span>
        <span className="dot-sep">·</span>
        <span>{item.translation ? item.translation.generatedAt : '刚刚'}</span>
      </div>
      {item.translation && item.translation.needsRetranslate ? (
        <div className="translation-stale">
          <Icon name="alert-triangle" size={14} />
          <span>正文已更新，当前译文可能过期。</span>
          <TextLink onClick={onRetranslate}>重新翻译</TextLink>
        </div>
      ) : null}
      <p className="translation-foot">切换不重复消耗额度 · 译文按段回填，不重建正文结构</p>
    </section>
  );
}

function ArticleFigure({ block }) {
  return (
    <figure className="article-figure">
      <div className="image-placeholder">
        <span className="image-label">媒体代理</span>
        <span className="image-line"></span>
        <span className="image-caption-mark">图片由微服缓存 · 源站删除不影响已缓存内容</span>
      </div>
      {block.caption ? <figcaption>{block.caption}</figcaption> : null}
    </figure>
  );
}

function ArticleBlock({ block, first }) {
  if (block.type === 'p') {
    return <p className={cx('article-p', first && 'is-lead')}>{block.text}</p>;
  }
  if (block.type === 'h2') {
    return <h2 className="article-h2">{block.text}</h2>;
  }
  if (block.type === 'quote') {
    return (
      <blockquote className="article-quote">
        <span className="article-quote-mark" aria-hidden="true">“</span>
        <p>{block.text}</p>
      </blockquote>
    );
  }
  if (block.type === 'ul') {
    return (
      <ul className="article-list">
        {block.items.map((item, index) => <li key={index}>{item}</li>)}
      </ul>
    );
  }
  if (block.type === 'image') {
    return <ArticleFigure block={block} />;
  }
  return null;
}

function ArticleBody({ item, mode = 'zh', className = '' }) {
  const zh = item.bodyZh || bodyForItem(item);
  const en = item.bodyEn || [];
  if (mode === 'en' && en.length === 0) mode = 'zh';

  return (
    <div className={cx('article-body', className)}>
      {zh.map((block, index) => {
        if (mode === 'en') {
          const enBlock = en[index];
          if (!enBlock) return null;
          return <ArticleBlock key={index} block={enBlock} first={index === 0} />;
        }
        if (mode === 'both') {
          const enBlock = en[index];
          return (
            <React.Fragment key={index}>
              <ArticleBlock block={block} first={index === 0} />
              {enBlock && enBlock.type === 'p' ? (
                <p className="article-p article-original-p">{enBlock.text}</p>
              ) : null}
            </React.Fragment>
          );
        }
        return <ArticleBlock key={index} block={block} first={index === 0} />;
      })}
    </div>
  );
}

function ArticleEnd({ item, source, onToggleStar, onOpenOriginal, onCopyFeed }) {
  return (
    <footer className="article-end">
      <div className="article-end-rule"><span>全文完</span></div>
      <div className="article-end-actions">
        <Button size="sm" icon="star" variant={item.starred ? 'primary' : 'secondary'} onClick={() => onToggleStar(item.id)}>
          {item.starred ? '已收藏' : '收藏'}
        </Button>
        <Button size="sm" icon="external" onClick={onOpenOriginal}>在源站打开</Button>
        <Button size="sm" icon="rss" onClick={onCopyFeed}>输出订阅</Button>
      </div>
      <p className="article-end-note">
        来自「{source.name}」 · 正文经清洗后渲染 · 图片经微服代理缓存
      </p>
    </footer>
  );
}

function ReaderActions({ item, source, onToggleStar, onToggleRead, onOpenOriginal, onCopyFeed, onOpenHealth }) {
  return (
    <div className="reader-actions">
      <IconButton icon="external" label="在源站打开" size={17} onClick={onOpenOriginal} />
      <IconButton icon="rss" label="复制订阅地址" size={17} onClick={onCopyFeed} />
      <IconButton
        icon="star"
        label={item.starred ? '取消收藏' : '收藏'}
        size={17}
        filled={item.starred}
        active={item.starred}
        onClick={() => onToggleStar(item.id)}
      />
      <IconButton
        icon={item.read ? 'unread' : 'check'}
        label={item.read ? '标为未读' : '标为已读'}
        size={17}
        onClick={() => onToggleRead(item.id)}
      />
      <IconButton icon="more" label="查看源诊断" size={17} onClick={() => onOpenHealth(source.id)} />
    </div>
  );
}

function ReaderPane({
  item, sources, settings, translationMode, onTranslationMode, summaryOpen, onToggleSummary,
  onToggleStar, onToggleRead, onOpenDetail, onOpenOriginal, onCopyFeed, onOpenHealth,
  onGenerateSummary, aiBusy, onRetranslate, isMobile,
}) {
  if (!item) {
    return (
      <section className="reader-pane">
        <div className="reader-empty">
          <div className="reader-empty-glyph" aria-hidden="true"><i></i><i></i><i></i></div>
          <h2>选择一篇文章</h2>
          <p>按 <Kbd>j</Kbd> <Kbd>k</Kbd> 在列表中移动，<Kbd>Enter</Kbd> 打开详情。</p>
        </div>
      </section>
    );
  }
  const source = sourceById(item.sourceId, sources);
  return (
    <section className={cx('reader-pane', isMobile && 'is-mobile')} aria-label="阅读区">
      {!isMobile ? (
        <div className="reader-bar">
          <div className="reader-bar-left">
            <span className="reader-crumb">阅读</span>
            <span className="crumb-sep">/</span>
            <span className="reader-bar-source">{source.name}</span>
          </div>
          <div className="reader-bar-right">
            <span className="reader-keyhint"><Kbd>j</Kbd><Kbd>k</Kbd> 切换 <span className="dot-sep">·</span> <Kbd>m</Kbd> 已读 <span className="dot-sep">·</span> <Kbd>s</Kbd> 收藏</span>
            <ReaderActions
              item={item} source={source}
              onToggleStar={onToggleStar} onToggleRead={onToggleRead}
              onOpenOriginal={onOpenOriginal} onCopyFeed={onCopyFeed}
              onOpenHealth={onOpenHealth}
            />
          </div>
        </div>
      ) : null}

      <div className="reader-scroll">
        <article className="article">
          <ArticleHead item={item} source={source} />
          <AISummary
            item={item}
            open={summaryOpen}
            onToggle={onToggleSummary}
            style={settings.summaryStyle}
            onGenerate={() => onGenerateSummary(item)}
            busy={aiBusy}
          />
          <TranslationPanel item={item} mode={translationMode} onMode={onTranslationMode} onRetranslate={() => onRetranslate(item)} />
          <ArticleBody item={item} mode={translationMode} />
          <ArticleEnd
            item={item} source={source}
            onToggleStar={onToggleStar}
            onOpenOriginal={onOpenOriginal}
            onCopyFeed={onCopyFeed}
          />
        </article>
      </div>
    </section>
  );
}

function SourceMarginCard({ source, onOpenHealth, onCopyFeed }) {
  return (
    <section className="margin-card source-margin">
      <div className="margin-label">来源</div>
      <div className="source-margin-head">
        <SourceIcon source={source} size={30} />
        <div>
          <strong>{source.name}</strong>
          <span>{source.url}</span>
        </div>
      </div>
      <dl className="source-margin-list">
        <div><dt>状态</dt><dd><StatusBadge source={source} compact /></dd></div>
        <div><dt>刷新间隔</dt><dd>{source.interval}</dd></div>
        <div><dt>上次抓取</dt><dd>{source.lastFetch}</dd></div>
        <div><dt>下次抓取</dt><dd>{source.nextFetch}</dd></div>
      </dl>
      <div className="margin-actions">
        <Button size="sm" onClick={() => onOpenHealth(source.id)}>查看诊断</Button>
        <Button size="sm" icon="rss" onClick={onCopyFeed}>输出</Button>
      </div>
    </section>
  );
}

function ReadingControls({ settings, onChange }) {
  const change = (patch) => onChange(Object.assign({}, settings, patch));
  return (
    <section className="margin-card reading-controls">
      <div className="margin-label">阅读设置</div>
      <div className="reading-control-row">
        <span>字号</span>
        <div className="stepper">
          <button type="button" onClick={() => change({ fontSize: Math.max(15, settings.fontSize - 1) })} aria-label="减小字号">−</button>
          <span>{settings.fontSize}</span>
          <button type="button" onClick={() => change({ fontSize: Math.min(24, settings.fontSize + 1) })} aria-label="增大字号">+</button>
        </div>
      </div>
      <div className="reading-control-row">
        <span>主题</span>
        <Segmented
          ariaLabel="阅读主题"
          value={settings.theme}
          onChange={(value) => change({ theme: value })}
          options={[{ value: 'light', label: '浅色' }, { value: 'dark', label: '深色' }]}
        />
      </div>
      <div className="reading-control-row">
        <span>行宽</span>
        <div className="stepper">
          <button type="button" onClick={() => change({ lineWidth: Math.max(560, settings.lineWidth - 20) })} aria-label="减小行宽">−</button>
          <span>{settings.lineWidth}</span>
          <button type="button" onClick={() => change({ lineWidth: Math.min(780, settings.lineWidth + 20) })} aria-label="增大行宽">＋</button>
        </div>
      </div>
    </section>
  );
}

function FocusReader({
  item, sources, settings, onChangeSettings, translationMode, onTranslationMode,
  summaryOpen, onToggleSummary, onToggleStar, onToggleRead, onOpenOriginal,
  onCopyFeed, onOpenHealth, onGenerateSummary, aiBusy, onRetranslate, onBack, isMobile,
}) {
  const scrollRef = React.useRef(null);
  const [progress, setProgress] = React.useState(0);
  React.useEffect(() => {
    const el = scrollRef.current;
    if (!el) return;
    const onScroll = () => {
      const max = el.scrollHeight - el.clientHeight;
      setProgress(max > 0 ? Math.min(1, el.scrollTop / max) : 0);
    };
    el.addEventListener('scroll', onScroll, { passive: true });
    onScroll();
    return () => el.removeEventListener('scroll', onScroll);
  }, [item.id]);

  if (!item) {
    return (
      <div className="focus-screen">
        <div className="reader-empty">
          <h2>没有可阅读的条目</h2>
          <p>返回列表选择一篇文章。</p>
          <Button onClick={onBack}>返回阅读</Button>
        </div>
      </div>
    );
  }
  const source = sourceById(item.sourceId, sources);

  return (
    <div className="focus-screen">
      <div className="focus-bar">
        <div className="focus-bar-left">
          <IconButton icon="arrow-left" label="返回阅读" onClick={onBack} />
          <span className="focus-bar-title">专注阅读</span>
          <span className="crumb-sep">/</span>
          <span className="focus-bar-source">{source.name}</span>
        </div>
        <div className="focus-bar-right">
          <span className="focus-progress-label">{Math.round(progress * 100)}%</span>
          <ReaderActions
            item={item} source={source}
            onToggleStar={onToggleStar} onToggleRead={onToggleRead}
            onOpenOriginal={onOpenOriginal} onCopyFeed={onCopyFeed}
            onOpenHealth={onOpenHealth}
          />
        </div>
        <span className="focus-progress" style={{ width: `${progress * 100}%` }} aria-hidden="true"></span>
      </div>

      <div className="focus-scroll" ref={scrollRef}>
        <div className="focus-grid">
          <article className="focus-article">
            <ArticleHead item={item} source={source} focus />
            {isMobile ? (
              <div className="focus-mobile-tools">
                <AISummary
                  item={item}
                  open={summaryOpen}
                  onToggle={onToggleSummary}
                  style={settings.summaryStyle}
                  onGenerate={() => onGenerateSummary(item)}
                  busy={aiBusy}
                />
                <TranslationPanel item={item} mode={translationMode} onMode={onTranslationMode} onRetranslate={() => onRetranslate(item)} compact />
              </div>
            ) : null}
            <ArticleBody item={item} mode={translationMode} />
            <ArticleEnd
              item={item} source={source}
              onToggleStar={onToggleStar}
              onOpenOriginal={onOpenOriginal}
              onCopyFeed={onCopyFeed}
            />
          </article>

          {!isMobile ? (
            <aside className="margin-rail">
              <AISummary
                item={item}
                open={summaryOpen}
                onToggle={onToggleSummary}
                style={settings.summaryStyle}
                onGenerate={() => onGenerateSummary(item)}
                busy={aiBusy}
                className="is-rail"
              />
              <TranslationPanel
                item={item}
                mode={translationMode}
                onMode={onTranslationMode}
                onRetranslate={() => onRetranslate(item)}
                compact
                className="is-rail"
              />
              <SourceMarginCard source={source} onOpenHealth={onOpenHealth} onCopyFeed={onCopyFeed} />
              <ReadingControls settings={settings} onChange={onChangeSettings} />
            </aside>
          ) : null}
        </div>
      </div>
    </div>
  );
}

Object.assign(window, {
  itemsForList, listTitleFor, listEyebrowFor, ArticleRow, ArticleList, ArticleHead,
  AISummary, TranslationPanel, ArticleBlock, ArticleBody, ArticleEnd, ReaderActions,
  ReaderPane, SourceMarginCard, ReadingControls, FocusReader,
});
