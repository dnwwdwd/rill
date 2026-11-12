/* 线性图标集，统一 24 视口、1.6 描边。 */
function Icon({ name, size = 18, filled = false, className = '' }) {
  const p = {
    width: size, height: size, viewBox: '0 0 24 24', fill: 'none',
    stroke: 'currentColor', strokeWidth: 1.6, strokeLinecap: 'round',
    strokeLinejoin: 'round', className: `icon ${className}`.trim(), 'aria-hidden': 'true',
    focusable: 'false',
  };
  switch (name) {
    case 'inbox':
      return <svg {...p}><path d="M3.5 5.5h17v13h-17z" /><path d="M3.5 13h4l1.5 2h5l1.5-2h4" /></svg>;
    case 'unread':
      return <svg {...p}><circle cx="12" cy="12" r="7.5" /><circle cx="12" cy="12" r="2.4" fill="currentColor" stroke="none" /></svg>;
    case 'star':
      return <svg {...p}><path d="m12 3.8 2.5 5.1 5.6.8-4.1 4 1 5.6-5-2.6-5 2.6 1-5.6-4.1-4 5.6-.8z" fill={filled ? 'currentColor' : 'none'} /></svg>;
    case 'read':
      return <svg {...p}><circle cx="12" cy="12" r="8" /><path d="m8.4 12.2 2.3 2.3 4.9-5" /></svg>;
    case 'search':
      return <svg {...p}><circle cx="10.8" cy="10.8" r="6.3" /><path d="m15.5 15.5 4 4" /></svg>;
    case 'sliders':
      return <svg {...p}><path d="M4 7h6M14 7h6M4 17h10M18 17h2" /><circle cx="12" cy="7" r="2" /><circle cx="16" cy="17" r="2" /></svg>;
    case 'plus':
      return <svg {...p}><path d="M12 5v14M5 12h14" /></svg>;
    case 'chevron-down':
      return <svg {...p}><path d="m6 9 6 6 6-6" /></svg>;
    case 'chevron-right':
      return <svg {...p}><path d="m9 6 6 6-6 6" /></svg>;
    case 'chevron-left':
      return <svg {...p}><path d="m15 6-6 6 6 6" /></svg>;
    case 'arrow-left':
      return <svg {...p}><path d="M19 12H5M11 6l-6 6 6 6" /></svg>;
    case 'arrow-up-right':
      return <svg {...p}><path d="M7 17 17 7M8 7h9v9" /></svg>;
    case 'refresh':
      return <svg {...p}><path d="M19 8.5A8 8 0 1 0 20 14" /><path d="M19 4v4.5h-4.5" /></svg>;
    case 'rss':
      return <svg {...p}><circle cx="6" cy="18" r="1.4" fill="currentColor" stroke="none" /><path d="M5 11a8 8 0 0 1 8 8M5 5a14 14 0 0 1 14 14" /></svg>;
    case 'check':
      return <svg {...p}><path d="m5 12.5 4.2 4.2L19 6.8" /></svg>;
    case 'x':
      return <svg {...p}><path d="m6 6 12 12M18 6 6 18" /></svg>;
    case 'more':
      return <svg {...p}><circle cx="6" cy="12" r="1.3" fill="currentColor" stroke="none" /><circle cx="12" cy="12" r="1.3" fill="currentColor" stroke="none" /><circle cx="18" cy="12" r="1.3" fill="currentColor" stroke="none" /></svg>;
    case 'sun':
      return <svg {...p}><circle cx="12" cy="12" r="4" /><path d="M12 2.5v2M12 19.5v2M2.5 12h2M19.5 12h2M5.3 5.3l1.4 1.4M17.3 17.3l1.4 1.4M18.7 5.3l-1.4 1.4M6.7 17.3l-1.4 1.4" /></svg>;
    case 'moon':
      return <svg {...p}><path d="M20 14.4A8.4 8.4 0 0 1 9.6 4 8.4 8.4 0 1 0 20 14.4z" /></svg>;
    case 'menu':
      return <svg {...p}><path d="M4 7h16M4 12h16M4 17h10" /></svg>;
    case 'sparkles':
      return <svg {...p}><path d="m12 3 1.5 4.5L18 9l-4.5 1.5L12 15l-1.5-4.5L6 9l4.5-1.5z" /><path d="m18.5 15 .8 2.2 2.2.8-2.2.8-.8 2.2-.8-2.2-2.2-.8 2.2-.8z" /></svg>;
    case 'translate':
      return <svg {...p}><circle cx="12" cy="12" r="8.5" /><path d="M3.5 12h17M12 3.5c2.2 2.2 3.2 5.2 3.2 8.5s-1 6.3-3.2 8.5c-2.2-2.2-3.2-5.2-3.2-8.5s1-6.3 3.2-8.5z" /></svg>;
    case 'copy':
      return <svg {...p}><rect x="8" y="8" width="11" height="11" rx="1" /><path d="M16 8V5H5v11h3" /></svg>;
    case 'link':
      return <svg {...p}><path d="M10 14a4 4 0 0 0 5.7 0l2.5-2.5a4 4 0 0 0-5.7-5.7l-1.2 1.2" /><path d="M14 10a4 4 0 0 0-5.7 0l-2.5 2.5a4 4 0 0 0 5.7 5.7l1.2-1.2" /></svg>;
    case 'alert-triangle':
      return <svg {...p}><path d="M12 4 21 20H3z" /><path d="M12 9.5v4.5" /><circle cx="12" cy="17" r=".9" fill="currentColor" stroke="none" /></svg>;
    case 'alert-circle':
      return <svg {...p}><circle cx="12" cy="12" r="8.5" /><path d="M12 7.5v5.2" /><circle cx="12" cy="16.4" r=".9" fill="currentColor" stroke="none" /></svg>;
    case 'check-circle':
      return <svg {...p}><circle cx="12" cy="12" r="8.5" /><path d="m8.3 12.2 2.4 2.4 5-5.1" /></svg>;
    case 'clock':
      return <svg {...p}><circle cx="12" cy="12" r="8.5" /><path d="M12 7.5V12l3 2" /></svg>;
    case 'folder':
      return <svg {...p}><path d="M3.5 6.5h6l1.7 2h9.3v10h-17z" /></svg>;
    case 'shield':
      return <svg {...p}><path d="M12 3.5 19 6v5.2c0 4.4-2.8 7.6-7 9.3-4.2-1.7-7-4.9-7-9.3V6z" /><path d="m8.8 12 2.2 2.2 4.2-4.3" /></svg>;
    case 'database':
      return <svg {...p}><ellipse cx="12" cy="6" rx="7.5" ry="3" /><path d="M4.5 6v6c0 1.7 3.4 3 7.5 3s7.5-1.3 7.5-3V6" /><path d="M4.5 12v6c0 1.7 3.4 3 7.5 3s7.5-1.3 7.5-3v-6" /></svg>;
    case 'lock':
      return <svg {...p}><rect x="5.5" y="10" width="13" height="10" rx="1" /><path d="M8.5 10V7.5a3.5 3.5 0 0 1 7 0V10" /></svg>;
    case 'logout':
      return <svg {...p}><path d="M14 5H5v14h9M11 12h9M17 8l4 4-4 4" /></svg>;
    case 'key':
      return <svg {...p}><circle cx="8" cy="15.5" r="3.5" /><path d="m10.5 13 8-8M15.5 8l2 2M18 5.5l2 2" /></svg>;
    case 'external':
      return <svg {...p}><path d="M14 4h6v6M20 4l-8.5 8.5" /><path d="M18 14v5H5V6h5" /></svg>;
    case 'pause':
      return <svg {...p}><path d="M9 5v14M15 5v14" /></svg>;
    case 'play':
      return <svg {...p}><path d="m8 5 11 7-11 7z" /></svg>;
    case 'trash':
      return <svg {...p}><path d="M4.5 7h15M9 7V4.5h6V7M6.5 7l.8 13h9.4l.8-13M10 10.5v6M14 10.5v6" /></svg>;
    case 'download':
      return <svg {...p}><path d="M12 4v11M7.5 10.5 12 15l4.5-4.5M4.5 19.5h15" /></svg>;
    case 'upload':
      return <svg {...p}><path d="M12 20V9M7.5 13.5 12 9l4.5 4.5M4.5 4.5h15" /></svg>;
    case 'filter':
      return <svg {...p}><path d="M4 6h16l-6.2 7v5l-3.6 2v-7z" /></svg>;
    case 'calendar':
      return <svg {...p}><rect x="4" y="5.5" width="16" height="15" rx="1" /><path d="M4 10h16M8 3.5v4M16 3.5v4" /></svg>;
    case 'qr':
      return <svg {...p}><path d="M4 4h6v6H4zM14 4h6v6h-6zM4 14h6v6H4z" /><path d="M14 14h2v2h-2zM18 14h2v2h-2zM14 18h2v2h-2zM18 18h2v2h-2z" /></svg>;
    case 'server':
      return <svg {...p}><rect x="4" y="4" width="16" height="6" rx="1" /><rect x="4" y="14" width="16" height="6" rx="1" /><path d="M8 7h.01M8 17h.01M12 7h4M12 17h4" /></svg>;
    case 'eye':
      return <svg {...p}><path d="M2.8 12s3.4-6 9.2-6 9.2 6 9.2 6-3.4 6-9.2 6-9.2-6-9.2-6z" /><circle cx="12" cy="12" r="2.6" /></svg>;
    case 'eye-off':
      return <svg {...p}><path d="M4 4l16 16M9.5 6.4A9.8 9.8 0 0 1 12 6c5.8 0 9.2 6 9.2 6a16 16 0 0 1-2.4 3M6.2 8.2A15.7 15.7 0 0 0 2.8 12s3.4 6 9.2 6a9.7 9.7 0 0 0 3.8-.8" /></svg>;
    case 'command':
      return <svg {...p}><path d="M9 9h6v6H9z" /><path d="M9 9H7a2 2 0 1 1 2-2v2zM15 9h2a2 2 0 1 0-2-2v2zM9 15H7a2 2 0 1 0 2 2v-2zM15 15h2a2 2 0 1 1-2 2v-2z" /></svg>;
    case 'list':
      return <svg {...p}><path d="M8 6h12M8 12h12M8 18h12" /><circle cx="4.5" cy="6" r="1" fill="currentColor" stroke="none" /><circle cx="4.5" cy="12" r="1" fill="currentColor" stroke="none" /><circle cx="4.5" cy="18" r="1" fill="currentColor" stroke="none" /></svg>;
    case 'type':
      return <svg {...p}><path d="M5 6V4.5h14V6M12 4.5v15M9 19.5h6" /></svg>;
    case 'width':
      return <svg {...p}><path d="M4 6v12M20 6v12M8 12h8M8 12l2-2M8 12l2 2M16 12l-2-2M16 12l-2 2" /></svg>;
    case 'height':
      return <svg {...p}><path d="M6 4h12M6 20h12M12 8v8M12 8l-2 2M12 8l2 2M12 16l-2-2M12 16l2-2" /></svg>;
    case 'panel':
      return <svg {...p}><rect x="3.5" y="4.5" width="17" height="15" rx="1" /><path d="M9 4.5v15" /></svg>;
    case 'bell':
      return <svg {...p}><path d="M6.5 10a5.5 5.5 0 0 1 11 0c0 5 2 6 2 6h-15s2-1 2-6z" /><path d="M10 20h4" /></svg>;
    case 'mail':
      return <svg {...p}><rect x="3.5" y="5.5" width="17" height="13" rx="1" /><path d="m4.5 7 7.5 6 7.5-6" /></svg>;
    case 'info':
      return <svg {...p}><circle cx="12" cy="12" r="8.5" /><path d="M12 11v5.5" /><circle cx="12" cy="8" r=".9" fill="currentColor" stroke="none" /></svg>;
    default:
      return <svg {...p}><circle cx="12" cy="12" r="8" /></svg>;
  }
}

function Wordmark({ small = false }) {
  return (
    <span className={`wordmark ${small ? 'small' : ''}`} aria-label="Rill">
      <span className="wordmark-text">Rill</span>
      <span className="wordmark-mark" aria-hidden="true"><i></i><i></i><i></i></span>
    </span>
  );
}

Object.assign(window, { Icon, Wordmark });
