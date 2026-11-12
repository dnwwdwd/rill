/* 设置页：阅读、AI、公众号、备份、安全。 */
function QrPlaceholder({ scanned, onScan }) {
  const size = 21;
  const cells = [];
  const finder = (r, c) => {
    const inBox = (r0, c0) => r >= r0 && r < r0 + 7 && c >= c0 && c < c0 + 7;
    const outer = (r0, c0) => r === r0 || r === r0 + 6 || c === c0 || c === c0 + 6;
    const inner = (r0, c0) => r >= r0 + 2 && r <= r0 + 4 && c >= c0 + 2 && c <= c0 + 4;
    if (inBox(0, 0)) return outer(0, 0) || inner(0, 0);
    if (inBox(0, size - 7)) return outer(0, size - 7) || inner(0, size - 7);
    if (inBox(size - 7, 0)) return outer(size - 7, 0) || inner(size - 7, 0);
    return null;
  };
  for (let r = 0; r < size; r += 1) {
    for (let c = 0; c < size; c += 1) {
      const f = finder(r, c);
      const on = f === null ? ((r * 7 + c * 3 + r * c) % 11 < 5) : f;
      cells.push(<span key={`${r}-${c}`} className={on ? 'on' : ''}></span>);
    }
  }
  return (
    <button type="button" className={cx('qr-grid', scanned && 'is-scanned')} onClick={onScan} aria-label="二维码占位，点击模拟扫码">
      {cells}
      {scanned ? <span className="qr-scanned-mark"><Icon name="check" size={22} />已登录</span> : null}
    </button>
  );
}

function SettingsNav({ tab, onTab }) {
  const items = [
    { key: 'reading', label: '阅读', icon: 'type', desc: '字号、行宽、主题' },
    { key: 'ai', label: 'AI', icon: 'sparkles', desc: '摘要与翻译' },
    { key: 'wechat', label: '公众号', icon: 'qr', desc: '扫码登录与伴生服务' },
    { key: 'backup', label: '备份与恢复', icon: 'database', desc: '每日备份、保留策略' },
    { key: 'security', label: '安全', icon: 'shield', desc: '密码、会话、令牌' },
  ];
  return (
    <nav className="settings-nav" aria-label="设置分类">
      {items.map((item) => (
        <button
          type="button"
          key={item.key}
          className={cx('settings-nav-item', tab === item.key && 'is-active')}
          onClick={() => onTab(item.key)}
        >
          <Icon name={item.icon} size={17} />
          <span className="settings-nav-copy">
            <strong>{item.label}</strong>
            <span>{item.desc}</span>
          </span>
          <Icon name="chevron-right" size={14} className="settings-nav-arrow" />
        </button>
      ))}
    </nav>
  );
}

function SettingsSection({ eyebrow, title, description, children, className = '' }) {
  return (
    <section className={cx('settings-section', className)}>
      <header className="settings-section-head">
        {eyebrow ? <div className="eyebrow">{eyebrow}</div> : null}
        <h2>{title}</h2>
        {description ? <p>{description}</p> : null}
      </header>
      <div className="settings-section-body">{children}</div>
    </section>
  );
}

function SliderField({ label, value, min, max, step = 1, suffix, onChange, hint }) {
  return (
    <div className="slider-field">
      <div className="slider-head">
        <span>{label}</span>
        <code>{value}{suffix}</code>
      </div>
      <input type="range" min={min} max={max} step={step} value={value} onChange={(e) => onChange(Number(e.target.value))} aria-label={label} />
      {hint ? <span className="slider-hint">{hint}</span> : null}
    </div>
  );
}

function ReadingSettings({ settings, onChange }) {
  const change = (patch) => onChange(Object.assign({}, settings, patch));
  return (
    <div className="settings-panel">
      <SettingsSection
        eyebrow="阅读"
        title="让正文适合你的眼睛"
        description="这些偏好会写回后端设置，跨设备保留；切换时先用样式变量即时生效。"
      >
        <div className="settings-row settings-row-split">
          <div>
            <div className="settings-field-title">主题</div>
            <Segmented
              ariaLabel="阅读主题"
              value={settings.theme}
              onChange={(value) => change({ theme: value })}
              options={[{ value: 'light', label: '浅色' }, { value: 'dark', label: '深色' }]}
            />
            <p className="settings-field-hint">首屏前内联设置主题标记，避免深色模式闪白。</p>
          </div>
          <div className="reading-preview" aria-hidden="true">
            <span className="preview-kicker">预览</span>
            <p>山川是不卷收的文章，日月为你掌灯伴读。</p>
            <span className="preview-meta">字号 {settings.fontSize} 像素 · 行高 {settings.lineHeight} · 行宽 {settings.lineWidth} 像素</span>
          </div>
        </div>

        <SliderField label="正文字号" value={settings.fontSize} min={15} max={24} suffix=" 像素" onChange={(value) => change({ fontSize: value })} hint="中文正文建议不小于 16 像素。" />
        <SliderField label="行宽" value={settings.lineWidth} min={560} max={780} step={20} suffix=" 像素" onChange={(value) => change({ lineWidth: value })} hint="行宽越大，阅读时视线回扫距离越长。" />
        <SliderField label="行高" value={settings.lineHeight} min={1.6} max={2} step={0.05} suffix="" onChange={(value) => change({ lineHeight: Number(value.toFixed(2)) })} hint="中文长文建议 1.8–1.9。" />
      </SettingsSection>

      <SettingsSection eyebrow="行为" title="阅读时少做选择">
        <SwitchRow
          title="滚动标记已读"
          description="条目在阅读区停留 3 秒后标记已读；攒批提交，失败会回滚本地状态。"
          checked={settings.autoRead}
          onChange={(value) => change({ autoRead: value })}
        />
        <SwitchRow
          title="记住阅读位置"
          description="重新打开长文时回到上次位置。"
          checked={settings.rememberPosition !== false}
          onChange={(value) => change({ rememberPosition: value })}
        />
      </SettingsSection>
    </div>
  );
}

function AISettings({ settings, onChange, onOpenConsent }) {
  const change = (patch) => onChange(Object.assign({}, settings, patch));
  const [showKey, setShowKey] = React.useState(false);
  const [keyDraft, setKeyDraft] = React.useState('');
  const [testState, setTestState] = React.useState('idle');

  const test = () => {
    setTestState('testing');
    window.setTimeout(() => setTestState('ok'), 900);
  };

  return (
    <div className="settings-panel">
      <SettingsSection
        eyebrow="AI"
        title="摘要与翻译共用一套接口"
        description="只走你配置的 OpenAI 兼容接口，不依赖懒猫 AI 算力舱。关闭时全流程零外部 AI 请求。"
      >
        <SwitchRow
          title="启用 AI 能力"
          description="关闭后停止新的模型调用，已有结果仍然保留。"
          checked={settings.aiEnabled}
          onChange={(value) => change({ aiEnabled: value })}
        />
        <div className={cx('ai-config', !settings.aiEnabled && 'is-disabled')}>
          <div className="settings-field-grid">
            <Field label="接口地址" hint="标准 POST {base_url}/chat/completions">
              <input value="https://api.deepseek.com/v1" readOnly aria-label="接口地址" />
            </Field>
            <Field label="模型" hint="兼容 DeepSeek / OpenRouter / 自建 vLLM">
              <input value="deepseek-chat" readOnly aria-label="模型" />
            </Field>
          </div>
          <Field label="接口密钥" hint="AES-GCM 加密存储，日志脱敏；不会出现在订阅输出里。">
            <div className="secret-input">
              <input
                type={showKey ? 'text' : 'password'}
                value={keyDraft}
                onChange={(e) => setKeyDraft(e.target.value)}
                placeholder="已保存，输入新密钥可替换"
                aria-label="接口密钥"
                disabled={!settings.aiEnabled}
              />
              <IconButton icon={showKey ? 'eye-off' : 'eye'} label={showKey ? '隐藏密钥' : '显示密钥'} size={16} onClick={() => setShowKey(!showKey)} />
            </div>
          </Field>
          <div className="settings-row settings-row-inline">
            <Button size="sm" icon="refresh" onClick={test} disabled={!settings.aiEnabled || testState === 'testing'}>
              {testState === 'testing' ? '正在测试' : '测试连接'}
            </Button>
            {testState === 'ok' ? <span className="test-ok"><Icon name="check-circle" size={15} /> 连接成功 · 延迟 680 毫秒</span> : null}
            {testState === 'testing' ? <span className="test-pending">正在请求模型列表…</span> : null}
          </div>
        </div>
      </SettingsSection>

      <SettingsSection eyebrow="摘要" title="输出形态">
        <div className="settings-field-grid">
          <div>
            <div className="settings-field-title">默认摘要样式</div>
            <Segmented
              ariaLabel="默认摘要样式"
              value={settings.summaryStyle}
              onChange={(value) => change({ summaryStyle: value })}
              options={[{ value: 'bullets', label: '要点列表' }, { value: 'one', label: '一句话' }]}
            />
            <p className="settings-field-hint">摘要按源开启，默认关闭；入库后异步生成，失败不影响入库。</p>
          </div>
          <div>
            <div className="settings-field-title">每日调用上限</div>
            <div className="number-input">
              <input type="number" min="1" max="2000" value={settings.dailyLimit} onChange={(e) => change({ dailyLimit: Number(e.target.value) || 0 })} aria-label="每日调用上限" />
              <span>次 / 天</span>
            </div>
            <p className="settings-field-hint">超限停止并提示，不会静默消耗额度。</p>
          </div>
        </div>
        <div className="usage-block">
          <div className="usage-head"><span>今日调用</span><strong>{settings.usedToday} / {settings.dailyLimit}</strong></div>
          <Meter value={settings.usedToday} max={settings.dailyLimit} />
          <div className="usage-foot"><span>本月累计 120 万令牌</span><span>结果按「条目 + 类型 + 语言」缓存</span></div>
        </div>
      </SettingsSection>

      <SettingsSection eyebrow="翻译" title="默认目标语言">
        <div className="settings-field-grid">
          <Field label="目标语言" hint="同一条目 + 语言只消耗一次调用；切换显示模式不重复请求。">
            <select value={settings.translationTarget} onChange={(e) => change({ translationTarget: e.target.value })} aria-label="翻译目标语言">
              <option value="简体中文">简体中文</option>
              <option value="繁體中文">繁體中文</option>
            </select>
          </Field>
          <div className="consent-box">
            <div className={cx('consent-mark', settings.aiConsent && 'is-on')}><Icon name="shield" size={18} /></div>
            <div>
              <strong>内容外发确认</strong>
              <p>摘要是按源、翻译是按条目把正文发送到你配置的第三方接口。首次开启必须在设置页显式确认，并在条目上标注。</p>
              <label className="check-row">
                <input type="checkbox" checked={settings.aiConsent} onChange={(e) => change({ aiConsent: e.target.checked })} />
                <span>我已知晓并允许把订阅正文发送至外部模型</span>
              </label>
            </div>
          </div>
        </div>
      </SettingsSection>
    </div>
  );
}

function WeChatSettings({ settings, onChange, sources, onOpenHealth, onScan }) {
  const change = (patch) => onChange(Object.assign({}, settings, patch));
  const scanned = settings.wechatStatus === 'ok';
  const wechatSources = sources.filter((s) => s.kind === 'wechat');

  return (
    <div className="settings-panel">
      <SettingsSection
        eyebrow="公众号"
        title="强制扫码登录，不做无登录绕过"
        description="公众号是可选能力，唯一依赖 we-mp-rss 伴生服务；关闭时主应用保持轻量。"
      >
        <SwitchRow
          title="启用公众号采集"
          description="关闭伴生服务后，公众号源整体暂停，其他源不受影响。"
          checked={settings.sidecarEnabled}
          onChange={(value) => change({ sidecarEnabled: value, wechatStatus: value ? settings.wechatStatus : 'disabled' })}
        />
        <Field label="伴生服务地址" hint="跨应用包名能否直接解析待平台确认；验证前按同包多服务或反向代理配置。">
          <input value="http://we-mp-rss.lzcapp:8001" readOnly aria-label="伴生服务地址" />
        </Field>
      </SettingsSection>

      <SettingsSection eyebrow="登录态" title={scanned ? '已登录' : '需要扫码'} className="wechat-login-section">
        <div className="wechat-login-card">
          <div className="wechat-login-copy">
            <div className={cx('wechat-state', scanned ? 'ok' : 'expired')}>
              <StatusDot status={scanned ? 'ok' : 'auth'} />
              <span>{scanned ? '登录态有效' : settings.wechatStatus === 'disabled' ? '公众号未启用' : '登录态已过期'}</span>
            </div>
            <p>
              {scanned
                ? settings.wechatExpires + '。过期后抓取会暂停，并在源列表与诊断页提示重新扫码。'
                : settings.wechatStatus === 'disabled'
                  ? '启用公众号采集后才会出现二维码。'
                  : '请用公众号管理员微信扫码。二维码过期不静默失败，会一直提示重新扫码。'}
            </p>
            <div className="wechat-actions">
              <Button size="sm" icon="qr" onClick={onScan} disabled={!settings.sidecarEnabled}>刷新二维码</Button>
              {!scanned ? <Button size="sm" variant="primary" onClick={onScan} disabled={!settings.sidecarEnabled}>模拟扫码成功</Button> : null}
              {scanned ? <Button size="sm" icon="logout" onClick={() => change({ wechatStatus: 'expired', wechatExpires: '需要重新扫码' })}>退出登录</Button> : null}
            </div>
            <p className="wechat-fineprint">不缓存登录凭据；伴生服务的账号密码与 Cookie 不写入 Rill 数据库。</p>
          </div>
          <div className="wechat-login-qr">
            <QrPlaceholder scanned={scanned} onScan={() => { if (settings.sidecarEnabled) onScan(); }} />
            <span>原型中点击二维码模拟扫码</span>
          </div>
        </div>
      </SettingsSection>

      <SettingsSection eyebrow="已接入" title="公众号源">
        <div className="wechat-source-list">
          {wechatSources.map((source) => (
            <button type="button" key={source.id} className="wechat-source-row" onClick={() => onOpenHealth(source.id)}>
              <SourceIcon source={source} size={24} />
              <span className="wechat-source-copy">
                <strong>{source.name}</strong>
                <span>{source.status === 'auth' ? source.lastError : '上次抓取 ' + source.lastFetch}</span>
              </span>
              <StatusBadge source={source} />
              <Icon name="chevron-right" size={14} />
            </button>
          ))}
          {wechatSources.length === 0 ? <p className="muted">还没有接入公众号。启用后可在源管理中添加。</p> : null}
        </div>
        <div className="add-note">
          <Icon name="lock" size={15} />
          <span>唯一路径：we-mp-rss + 强制扫码。不接入公开专辑、公开主页、Wechat2RSS 或无登录方案。</span>
        </div>
      </SettingsSection>
    </div>
  );
}

function BackupSettings({ settings, onChange, onBackup, onRestore }) {
  const change = (patch) => onChange(Object.assign({}, settings, patch));
  return (
    <div className="settings-panel">
      <SettingsSection
        eyebrow="备份"
        title="数据库每日备份"
        description="备份文件落在持久化目录内；恢复前先用 schema 版本校验，避免把旧库灌进新结构。"
      >
        <SwitchRow
          title="每日自动备份"
          description="保留最近 7 份；升级迁移前会额外用 VACUUM INTO 生成一份旧库。"
          checked={settings.backupDaily}
          onChange={(value) => change({ backupDaily: value })}
        />
        <div className="settings-field-grid">
          <Field label="保留份数">
            <select value={settings.backupKeep} onChange={(e) => change({ backupKeep: Number(e.target.value) })} aria-label="备份保留份数">
              <option value="3">3 份</option>
              <option value="7">7 份</option>
              <option value="14">14 份</option>
            </select>
          </Field>
          <Field label="上次备份" hint="位于 /lzcapp/var/data/backups">
            <input value={settings.lastBackup} readOnly aria-label="上次备份时间" />
          </Field>
        </div>
        <div className="settings-row settings-row-inline">
          <Button icon="download" onClick={onBackup}>立即备份</Button>
          <Button icon="upload" onClick={onRestore}>从备份恢复</Button>
          <span className="settings-field-hint">恢复会用文件选择器导入；恢复后订阅与已读状态都在。</span>
        </div>
      </SettingsSection>

      <SettingsSection eyebrow="历史" title="最近备份">
        <div className="backup-list">
          {[
            { time: '今天 03:00', size: '12.4 MB', version: 'schema v3' },
            { time: '昨天 03:00', size: '12.1 MB', version: 'schema v3' },
            { time: '10 月 29 日 03:00', size: '11.8 MB', version: 'schema v3' },
          ].map((row) => (
            <div className="backup-row" key={row.time}>
              <Icon name="database" size={17} />
              <span className="backup-time">{row.time}</span>
              <span className="backup-size">{row.size}</span>
              <Tag tone="neutral">{row.version}</Tag>
              <TextLink onClick={onRestore}>恢复</TextLink>
            </div>
          ))}
        </div>
        <p className="settings-field-hint">备份包含密钥文件时，恢复后才能解密已存接口密钥；请在恢复文档中明确这个风险。</p>
      </SettingsSection>
    </div>
  );
}

function SecuritySettings({ settings, onChange, onResetToken, onLogout }) {
  const [passwordState, setPasswordState] = React.useState('idle');
  const change = (patch) => onChange(Object.assign({}, settings, patch));
  return (
    <div className="settings-panel">
      <SettingsSection
        eyebrow="账号"
        title="单用户密码登录"
        description="第一版不做多用户与 OIDC。管理接口必须登录；订阅 / OPML / 媒体走独立令牌地址。"
      >
        <div className="settings-field-grid">
          <Field label="当前密码">
            <input type="password" placeholder="输入当前密码" aria-label="当前密码" />
          </Field>
          <Field label="新密码" hint="至少 12 位；修改后其他会话立即失效。">
            <input type="password" placeholder="输入新密码" aria-label="新密码" />
          </Field>
        </div>
        <div className="settings-row settings-row-inline">
          <Button
            onClick={() => {
              setPasswordState('saving');
              window.setTimeout(() => setPasswordState('ok'), 700);
            }}
          >
            {passwordState === 'saving' ? '正在更新' : '更新密码'}
          </Button>
          {passwordState === 'ok' ? <span className="test-ok"><Icon name="check-circle" size={15} /> 密码已更新，其他会话已失效</span> : null}
        </div>
        <div className="session-card">
          <div>
            <span className="settings-field-title">当前会话</span>
            <p>密码登录 · 会话 7 天 · CSRF 保护已开启 · 同 IP 失败 5 次锁定 5 分钟</p>
          </div>
          <Button size="sm" icon="logout" onClick={onLogout}>退出登录</Button>
        </div>
      </SettingsSection>

      <SettingsSection
        eyebrow="输出"
        title="输出令牌"
        description="订阅输出使用不可猜测的令牌地址，不暴露源配置；泄露后可立即重置。"
      >
        <div className="token-block">
          <div className="token-block-main">
            <span className="token-label">当前令牌</span>
            <code>{settings.feedToken}</code>
          </div>
          <Button size="sm" icon="refresh" onClick={onResetToken}>重置令牌</Button>
        </div>
        <div className="public-path-list">
          <span>/feed/*</span><span>/opml/*</span><span>/media/*</span><span>/healthz</span>
          <span className="public-path-note">public_path 只放行这些端点，管理接口不裸奔。</span>
        </div>
      </SettingsSection>

      <SettingsSection eyebrow="防护" title="默认安全">
        <div className="security-list">
          <div><Icon name="shield" size={17} /><span>SSRF：拒绝私网、回环、链路本地、组播与云 metadata，重定向逐跳校验。</span></div>
          <div><Icon name="lock" size={17} /><span>XSS：订阅正文清洗后渲染；AI 输出按纯文本 / Markdown 处理。</span></div>
          <div><Icon name="key" size={17} /><span>密钥：AI 接口密钥与伴生服务令牌加密存储，日志脱敏。</span></div>
          <div><Icon name="database" size={17} /><span>备份：每日备份、保留 7 份，恢复动作在 M4 实机演练。</span></div>
        </div>
      </SettingsSection>
    </div>
  );
}

function SettingsScreen({
  settings, onChange, tab, onTab, sources, onOpenHealth, onGoSources,
  onBackup, onRestore, onScan, onResetToken, onLogout, onOpenDrawer, isMobile,
}) {
  return (
    <div className="main-pane settings-screen">
      <div className="page-scroll">
        <PageHeader
          eyebrow="设置"
          title="偏好与连接"
          meta="自动保存"
          actions={<span className="saved-pill"><Icon name="check-circle" size={15} /> 已同步</span>}
        >
          <p className="page-lede">阅读偏好跨设备保留；密钥与登录凭据只留在本机。</p>
        </PageHeader>

        <div className="settings-layout">
          <SettingsNav tab={tab} onTab={onTab} />
          <div className="settings-content">
            {tab === 'reading' ? <ReadingSettings settings={settings} onChange={onChange} /> : null}
            {tab === 'ai' ? <AISettings settings={settings} onChange={onChange} /> : null}
            {tab === 'wechat' ? <WeChatSettings settings={settings} onChange={onChange} sources={sources} onOpenHealth={onOpenHealth} onScan={onScan} /> : null}
            {tab === 'backup' ? <BackupSettings settings={settings} onChange={onChange} onBackup={onBackup} onRestore={onRestore} /> : null}
            {tab === 'security' ? <SecuritySettings settings={settings} onChange={onChange} onResetToken={onResetToken} onLogout={onLogout} /> : null}
          </div>
        </div>

        <div className="page-footnote">
          <Icon name="lock" size={14} />
          <span>本页不会写入真实接口密钥、Cookie 或 OAuth 令牌；所有敏感字段均为演示占位。</span>
        </div>
      </div>
    </div>
  );
}

Object.assign(window, {
  QrPlaceholder, SettingsNav, SettingsSection, SliderField, ReadingSettings,
  AISettings, WeChatSettings, BackupSettings, SecuritySettings, SettingsScreen,
});
