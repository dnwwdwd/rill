/* 通用 UI 组件。所有组件显式挂到 window，供后续 Babel 文件使用。 */
function cx(...parts) {
  return parts.filter(Boolean).join(' ');
}

const STATUS_META = {
  ok: { label: '正常', className: 'ok' },
  warn: { label: '可能停更', className: 'warn' },
  error: { label: '连续失败', className: 'error' },
  auth: { label: '需重新扫码', className: 'auth' },
  paused: { label: '已暂停', className: 'paused' },
};

function SourceIcon({ source, size = 20 }) {
  const text = source ? source.icon : '?';
  return (
    <span
      className="source-icon"
      style={{
        '--icon-color': source ? source.color : '#6F6A60',
        width: size,
        height: size,
        fontSize: text.length > 1 ? Math.max(9, Math.round(size * 0.34)) : Math.round(size * 0.46),
        borderRadius: Math.max(3, Math.round(size * 0.18)),
      }}
      aria-hidden="true"
    >
      {text}
    </span>
  );
}

function StatusDot({ status = 'ok', label }) {
  const meta = STATUS_META[status] || STATUS_META.ok;
  return (
    <span className={`status-dot ${meta.className}`} title={label || meta.label} aria-label={label || meta.label}>
      <i></i>
    </span>
  );
}

function StatusBadge({ source, compact = false }) {
  const meta = STATUS_META[source.status] || STATUS_META.ok;
  return (
    <span className={cx('status-badge', meta.className, compact && 'compact')} title={source.note || meta.label}>
      <i></i>
      <span>{meta.label}</span>
    </span>
  );
}

function Button({ variant = 'secondary', size = 'md', icon, children, className = '', type = 'button', ...rest }) {
  return (
    <button type={type} className={cx('btn', `btn-${variant}`, `btn-${size}`, className)} {...rest}>
      {icon ? <Icon name={icon} size={size === 'sm' ? 15 : 17} /> : null}
      {children ? <span>{children}</span> : null}
    </button>
  );
}

function IconButton({ icon, label, active = false, filled = false, size = 18, className = '', ...rest }) {
  return (
    <button type="button" className={cx('icon-btn', active && 'is-active', className)} aria-label={label} title={label} {...rest}>
      <Icon name={icon} size={size} filled={filled} />
    </button>
  );
}

function TextLink({ children, className = '', ...rest }) {
  return <button type="button" className={cx('text-link', className)} {...rest}>{children}</button>;
}

function Segmented({ options, value, onChange, className = '', ariaLabel }) {
  return (
    <div className={cx('segmented', className)} role="group" aria-label={ariaLabel}>
      {options.map((opt) => (
        <button
          type="button"
          key={opt.value}
          className={cx('segment', value === opt.value && 'is-active')}
          aria-pressed={value === opt.value}
          onClick={() => onChange(opt.value)}
        >
          {opt.icon ? <Icon name={opt.icon} size={14} /> : null}
          <span>{opt.label}</span>
        </button>
      ))}
    </div>
  );
}

function Switch({ checked, onChange, label, disabled = false }) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      aria-label={label}
      className={cx('switch', checked && 'is-on')}
      onClick={() => !disabled && onChange(!checked)}
      disabled={disabled}
    >
      <span className="switch-thumb"></span>
    </button>
  );
}

function SwitchRow({ title, description, checked, onChange, disabled, trailing }) {
  return (
    <div className={cx('switch-row', disabled && 'is-disabled')}>
      <div className="switch-copy">
        <div className="switch-title">{title}</div>
        {description ? <div className="switch-desc">{description}</div> : null}
      </div>
      <div className="switch-control">
        {trailing}
        <Switch checked={checked} onChange={onChange} label={title} disabled={disabled} />
      </div>
    </div>
  );
}

function Field({ label, hint, children, className = '' }) {
  return (
    <label className={cx('field', className)}>
      <span className="field-label">{label}</span>
      {children}
      {hint ? <span className="field-hint">{hint}</span> : null}
    </label>
  );
}

function Modal({ open, onClose, title, subtitle, children, footer, wide = false }) {
  React.useEffect(() => {
    if (!open) return undefined;
    const onKey = (event) => {
      if (event.key === 'Escape') onClose();
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [open, onClose]);

  if (!open) return null;
  return (
    <div className="modal-backdrop" role="presentation" onMouseDown={(e) => { if (e.target === e.currentTarget) onClose(); }}>
      <div className={cx('modal', wide && 'modal-wide')} role="dialog" aria-modal="true" aria-label={title}>
        <div className="modal-head">
          <div>
            <h2>{title}</h2>
            {subtitle ? <p>{subtitle}</p> : null}
          </div>
          <IconButton icon="x" label="关闭" onClick={onClose} />
        </div>
        <div className="modal-body">{children}</div>
        {footer ? <div className="modal-foot">{footer}</div> : null}
      </div>
    </div>
  );
}

function Toast({ message, actionLabel, onAction }) {
  if (!message) return null;
  return (
    <div className="toast" role="status" aria-live="polite">
      <span className="toast-mark" aria-hidden="true"></span>
      <span className="toast-message">{message}</span>
      {actionLabel ? <button type="button" className="toast-action" onClick={onAction}>{actionLabel}</button> : null}
    </div>
  );
}

function Kbd({ children }) {
  return <kbd className="kbd">{children}</kbd>;
}

function Tag({ children, tone = 'neutral', className = '' }) {
  return <span className={cx('tag', `tag-${tone}`, className)}>{children}</span>;
}

function MiniBars({ values, label }) {
  const max = Math.max(1, ...values);
  return (
    <div className="mini-bars" role="img" aria-label={label || '趋势图'}>
      {values.map((value, index) => (
        <span
          key={index}
          className={cx('mini-bar', value === 0 && 'is-zero')}
          style={{ height: `${Math.max(4, (value / max) * 100)}%` }}
          title={`${index + 1}：${value} 条`}
        ></span>
      ))}
    </div>
  );
}

function Meter({ value, max, tone = 'accent' }) {
  const percent = Math.min(100, Math.round((value / Math.max(1, max)) * 100));
  return (
    <div className={cx('meter', `meter-${tone}`)} role="progressbar" aria-valuemin="0" aria-valuemax={max} aria-valuenow={value}>
      <span style={{ width: `${percent}%` }}></span>
    </div>
  );
}

function EmptyState({ icon = 'inbox', title, description, action }) {
  return (
    <div className="empty-state">
      <div className="empty-icon"><Icon name={icon} size={24} /></div>
      <h3>{title}</h3>
      {description ? <p>{description}</p> : null}
      {action}
    </div>
  );
}

function PageHeader({ eyebrow, title, meta, actions, children }) {
  return (
    <header className="page-head">
      <div className="page-head-main">
        {eyebrow ? <div className="eyebrow">{eyebrow}</div> : null}
        <div className="page-title-row">
          <h1>{title}</h1>
          {meta ? <span className="page-meta">{meta}</span> : null}
        </div>
        {children}
      </div>
      {actions ? <div className="page-actions">{actions}</div> : null}
    </header>
  );
}

function Divider({ label }) {
  return <div className={cx('divider', label && 'has-label')}>{label ? <span>{label}</span> : null}</div>;
}

Object.assign(window, {
  cx, STATUS_META, SourceIcon, StatusDot, StatusBadge, Button, IconButton, TextLink,
  Segmented, Switch, SwitchRow, Field, Modal, Toast, Kbd, Tag, MiniBars, Meter,
  EmptyState, PageHeader, Divider,
});
