import type { SVGProps } from 'react';

export type IconName =
  | 'inbox' | 'unread' | 'star' | 'read' | 'search' | 'sliders' | 'settings' | 'plus'
  | 'chevron-down' | 'chevron-right' | 'chevron-left' | 'arrow-left' | 'arrow-up'
  | 'arrow-up-right' | 'refresh' | 'rss' | 'check' | 'x' | 'more'
  | 'sun' | 'moon' | 'menu' | 'sparkles' | 'translate' | 'copy' | 'link'
  | 'alert-triangle' | 'alert-circle' | 'check-circle' | 'clock' | 'folder'
  | 'shield' | 'database' | 'lock' | 'logout' | 'key' | 'external'
  | 'pause' | 'play' | 'trash' | 'download' | 'upload' | 'filter'
  | 'calendar' | 'qr' | 'server' | 'eye' | 'eye-off' | 'command'
  | 'languages' | 'bot'
  | 'list' | 'type' | 'width' | 'height' | 'panel' | 'bell' | 'mail'
  | 'info' | 'focus' | 'minimize' | 'activity' | 'bar-chart' | 'at-sign' | 'message-circle'
  | 'layers' | 'zap' | 'arrow-right' | 'palette' | 'bookmark' | 'pencil' | 'file-text'
  | 'settings-sources' | 'settings-reading' | 'settings-ai'
  | 'settings-wechat' | 'settings-backup' | 'settings-security';

interface IconProps {
  name: IconName;
  size?: number;
  filled?: boolean;
  className?: string;
}

export const Icon = ({ name, size = 18, filled = false, className = '' }: IconProps) => {
  const props: SVGProps<SVGSVGElement> = {
    width: size,
    height: size,
    viewBox: '0 0 24 24',
    fill: 'none',
    stroke: 'currentColor',
    strokeWidth: 1.6,
    strokeLinecap: 'round',
    strokeLinejoin: 'round',
    className: `icon ${className}`.trim(),
    'aria-hidden': true,
    focusable: false,
  };

  switch (name) {
    case 'inbox':
      return <svg {...props}><path d="M3.5 5.5h17v13h-17z" /><path d="M3.5 13h4l1.5 2h5l1.5-2h4" /></svg>;
    case 'unread':
      return <svg {...props}><circle cx="12" cy="12" r="7.5" /><circle cx="12" cy="12" r="2.4" fill="currentColor" stroke="none" /></svg>;
    case 'star':
      return <svg {...props}><path d="m12 3.8 2.5 5.1 5.6.8-4.1 4 1 5.6-5-2.6-5 2.6 1-5.6-4.1-4 5.6-.8z" fill={filled ? 'currentColor' : 'none'} /></svg>;
    case 'read':
      return <svg {...props}><circle cx="12" cy="12" r="8" /><path d="m8.4 12.2 2.3 2.3 4.9-5" /></svg>;
    case 'search':
      return <svg {...props}><circle cx="10.8" cy="10.8" r="6.3" /><path d="m15.5 15.5 4 4" /></svg>;
    case 'sliders':
      return <svg {...props}><path d="M4 7h6M14 7h6M4 17h10M18 17h2" /><circle cx="12" cy="7" r="2" /><circle cx="16" cy="17" r="2" /></svg>;
    case 'settings':
      return <svg {...props}><circle cx="12" cy="12" r="3" /><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z" /></svg>;
    case 'plus':
      return <svg {...props}><path d="M12 5v14M5 12h14" /></svg>;
    case 'chevron-down':
      return <svg {...props}><path d="m6 9 6 6 6-6" /></svg>;
    case 'chevron-right':
      return <svg {...props}><path d="m9 6 6 6-6 6" /></svg>;
    case 'chevron-left':
      return <svg {...props}><path d="m15 6-6 6 6 6" /></svg>;
    case 'arrow-left':
      return <svg {...props}><path d="M19 12H5M11 6l-6 6 6 6" /></svg>;
    case 'arrow-up':
      return <svg {...props}><path d="M12 19V5M6 11l6-6 6 6" /></svg>;
    case 'arrow-up-right':
      return <svg {...props}><path d="M7 17 17 7M8 7h9v9" /></svg>;
    case 'refresh':
      return <svg {...props}><path d="M19 8.5A8 8 0 1 0 20 14" /><path d="M19 4v4.5h-4.5" /></svg>;
    case 'rss':
      return <svg {...props}><circle cx="6" cy="18" r="1.4" fill="currentColor" stroke="none" /><path d="M5 11a8 8 0 0 1 8 8M5 5a14 14 0 0 1 14 14" /></svg>;
    case 'check':
      return <svg {...props}><path d="m5 12.5 4.2 4.2L19 6.8" /></svg>;
    case 'x':
      return <svg {...props}><path d="m6 6 12 12M18 6 6 18" /></svg>;
    case 'more':
      return <svg {...props}><circle cx="6" cy="12" r="1.3" fill="currentColor" stroke="none" /><circle cx="12" cy="12" r="1.3" fill="currentColor" stroke="none" /><circle cx="18" cy="12" r="1.3" fill="currentColor" stroke="none" /></svg>;
    case 'sun':
      return <svg {...props}><circle cx="12" cy="12" r="4" /><path d="M12 2.5v2M12 19.5v2M2.5 12h2M19.5 12h2M5.3 5.3l1.4 1.4M17.3 17.3l1.4 1.4M18.7 5.3l-1.4 1.4M6.7 17.3l-1.4 1.4" /></svg>;
    case 'moon':
      return <svg {...props}><path d="M20 14.4A8.4 8.4 0 0 1 9.6 4 8.4 8.4 0 1 0 20 14.4z" /></svg>;
    case 'menu':
      return <svg {...props}><path d="M4 7h16M4 12h16M4 17h10" /></svg>;
    case 'sparkles':
      return <svg {...props}><path d="m4.5 19.5 9.5-9.5" /><path d="m16.5 3.5 1.1 2.9 2.9 1.1-2.9 1.1-1.1 2.9-1.1-2.9-2.9-1.1 2.9-1.1z" /><path d="m20 12.5.7 1.8 1.8.7-1.8.7-.7 1.8-.7-1.8-1.8-.7 1.8-.7z" /></svg>;
    case 'translate':
    case 'languages':
      return (
        <svg {...props}>
          <text x="2.5" y="12" fontSize="11" fontWeight="600" fill="currentColor" stroke="none">文</text>
          <text x="12.5" y="21.5" fontSize="11" fontWeight="600" fill="currentColor" stroke="none">A</text>
        </svg>
      );
    case 'bot':
      return (
        <svg {...props}>
          <rect x="4.5" y="8" width="15" height="11.5" rx="3.5" />
          <path d="M12 8V4.8" />
          <circle cx="12" cy="3.2" r="1.2" />
          <circle cx="9" cy="13" r="1.15" fill="currentColor" stroke="none" />
          <circle cx="15" cy="13" r="1.15" fill="currentColor" stroke="none" />
          <path d="M9.5 16.6h5" />
        </svg>
      );
    case 'copy':
      return <svg {...props}><rect x="8" y="8" width="11" height="11" rx="1" /><path d="M16 8V5H5v11h3" /></svg>;
    case 'link':
      return <svg {...props}><path d="M10 14a4 4 0 0 0 5.7 0l2.5-2.5a4 4 0 0 0-5.7-5.7l-1.2 1.2" /><path d="M14 10a4 4 0 0 0-5.7 0l-2.5 2.5a4 4 0 0 0 5.7 5.7l1.2-1.2" /></svg>;
    case 'alert-triangle':
      return <svg {...props}><path d="M12 4 21 20H3z" /><path d="M12 9.5v4.5" /><circle cx="12" cy="17" r=".9" fill="currentColor" stroke="none" /></svg>;
    case 'alert-circle':
      return <svg {...props}><circle cx="12" cy="12" r="8.5" /><path d="M12 7.5v5.2" /><circle cx="12" cy="16.4" r=".9" fill="currentColor" stroke="none" /></svg>;
    case 'check-circle':
      return <svg {...props}><circle cx="12" cy="12" r="8.5" /><path d="m8.3 12.2 2.4 2.4 5-5.1" /></svg>;
    case 'clock':
      return <svg {...props}><circle cx="12" cy="12" r="8.5" /><path d="M12 7.5V12l3 2" /></svg>;
    case 'folder':
      return <svg {...props}><path d="M3.5 6.5h6l1.7 2h9.3v10h-17z" /></svg>;
    case 'shield':
      return <svg {...props}><path d="M12 3.5 19 6v5.2c0 4.4-2.8 7.6-7 9.3-4.2-1.7-7-4.9-7-9.3V6z" /><path d="m8.8 12 2.2 2.2 4.2-4.3" /></svg>;
    case 'database':
      return <svg {...props}><ellipse cx="12" cy="6" rx="7.5" ry="3" /><path d="M4.5 6v6c0 1.7 3.4 3 7.5 3s7.5-1.3 7.5-3V6" /><path d="M4.5 12v6c0 1.7 3.4 3 7.5 3s7.5-1.3 7.5-3v-6" /></svg>;
    case 'lock':
      return <svg {...props}><rect x="5.5" y="10" width="13" height="10" rx="1" /><path d="M8.5 10V7.5a3.5 3.5 0 0 1 7 0V10" /></svg>;
    case 'logout':
      return <svg {...props}><path d="M14 5H5v14h9M11 12h9M17 8l4 4-4 4" /></svg>;
    case 'key':
      return <svg {...props}><circle cx="8" cy="15.5" r="3.5" /><path d="m10.5 13 8-8M15.5 8l2 2M18 5.5l2 2" /></svg>;
    case 'external':
      return <svg {...props}><path d="M14 4h6v6M20 4l-8.5 8.5" /><path d="M18 14v5H5V6h5" /></svg>;
    case 'pause':
      return <svg {...props}><path d="M9 5v14M15 5v14" /></svg>;
    case 'play':
      return <svg {...props}><path d="m8 5 11 7-11 7z" /></svg>;
    case 'trash':
      return <svg {...props}><path d="M4.5 7h15M9 7V4.5h6V7M6.5 7l.8 13h9.4l.8-13M10 10.5v6M14 10.5v6" /></svg>;
    case 'download':
      return <svg {...props}><path d="M12 4v11M7.5 10.5 12 15l4.5-4.5M4.5 19.5h15" /></svg>;
    case 'upload':
      return <svg {...props}><path d="M12 20V9M7.5 13.5 12 9l4.5 4.5M4.5 4.5h15" /></svg>;
    case 'filter':
      return <svg {...props}><path d="M4 6h16l-6.2 7v5l-3.6 2v-7z" /></svg>;
    case 'calendar':
      return <svg {...props}><rect x="4" y="5.5" width="16" height="15" rx="1" /><path d="M4 10h16M8 3.5v4M16 3.5v4" /></svg>;
    case 'qr':
      return <svg {...props}><path d="M4 4h6v6H4zM14 4h6v6h-6zM4 14h6v6H4z" /><path d="M14 14h2v2h-2zM18 14h2v2h-2zM14 18h2v2h-2zM18 18h2v2h-2z" /></svg>;
    case 'server':
      return <svg {...props}><rect x="4" y="4" width="16" height="6" rx="1" /><rect x="4" y="14" width="16" height="6" rx="1" /><path d="M8 7h.01M8 17h.01M12 7h4M12 17h4" /></svg>;
    case 'eye':
      return <svg {...props}><path d="M2.8 12s3.4-6 9.2-6 9.2 6 9.2 6-3.4 6-9.2 6-9.2-6-9.2-6z" /><circle cx="12" cy="12" r="2.6" /></svg>;
    case 'eye-off':
      return <svg {...props}><path d="M4 4l16 16M9.5 6.4A9.8 9.8 0 0 1 12 6c5.8 0 9.2 6 9.2 6a16 16 0 0 1-2.4 3M6.2 8.2A15.7 15.7 0 0 0 2.8 12s3.4 6 9.2 6a9.7 9.7 0 0 0 3.8-.8" /></svg>;
    case 'command':
      return <svg {...props}><path d="M9 9h6v6H9z" /><path d="M9 9H7a2 2 0 1 1 2-2v2zM15 9h2a2 2 0 1 0-2-2v2zM9 15H7a2 2 0 1 0 2 2v-2zM15 15h2a2 2 0 1 1-2 2v-2z" /></svg>;
    case 'list':
      return <svg {...props}><path d="M8 6h12M8 12h12M8 18h12" /><circle cx="4.5" cy="6" r="1" fill="currentColor" stroke="none" /><circle cx="4.5" cy="12" r="1" fill="currentColor" stroke="none" /><circle cx="4.5" cy="18" r="1" fill="currentColor" stroke="none" /></svg>;
    case 'type':
      return <svg {...props}><path d="M5 6V4.5h14V6M12 4.5v15M9 19.5h6" /></svg>;
    case 'width':
      return <svg {...props}><path d="M4 6v12M20 6v12M8 12h8M8 12l2-2M8 12l2 2M16 12l-2-2M16 12l-2 2" /></svg>;
    case 'height':
      return <svg {...props}><path d="M6 4h12M6 20h12M12 8v8M12 8l-2 2M12 8l2 2M12 16l-2-2M12 16l2-2" /></svg>;
    case 'panel':
      return <svg {...props}><rect x="3.5" y="4.5" width="17" height="15" rx="1" /><path d="M9 4.5v15" /></svg>;
    case 'bell':
      return <svg {...props}><path d="M6.5 10a5.5 5.5 0 0 1 11 0c0 5 2 6 2 6h-15s2-1 2-6z" /><path d="M10 20h4" /></svg>;
    case 'mail':
      return <svg {...props}><rect x="3.5" y="5.5" width="17" height="13" rx="1" /><path d="m4.5 7 7.5 6 7.5-6" /></svg>;
    case 'info':
      return <svg {...props}><circle cx="12" cy="12" r="8.5" /><path d="M12 11v5.5" /><circle cx="12" cy="8" r=".9" fill="currentColor" stroke="none" /></svg>;
    case 'focus':
      return <svg {...props}><path d="M4 9V4h5M15 4h5v5M20 15v5h-5M9 20H4v-5" /></svg>;
    case 'minimize':
      return <svg {...props}><path d="M9 4v5H4M20 9h-5V4M15 20v-5h5M4 15h5v5" /></svg>;
    case 'activity':
      return <svg {...props}><path d="M3 12h4l2.2-5.5L13 18l2.2-6H21" /></svg>;
    case 'bar-chart':
      return <svg {...props}><path d="M4 20V10M10 20V4M16 20v-7M22 20H2" /></svg>;
    case 'at-sign':
      return <svg {...props}><circle cx="12" cy="12" r="4" /><path d="M16 12v1.5a3.5 3.5 0 0 0 7 0V12a11 11 0 1 0-4.2 8.7" /></svg>;
    case 'message-circle':
      return <svg {...props}><path d="M20.5 11.7a8.5 8.5 0 0 1-12.6 7.5L3.5 20.5l1.3-4.3a8.5 8.5 0 1 1 15.7-4.5z" /></svg>;
    case 'layers':
      return <svg {...props}><path d="m12 3 9 5-9 5-9-5z" /><path d="m3 12 9 5 9-5M3 16l9 5 9-5" /></svg>;
    case 'zap':
      return <svg {...props}><path d="M13 2.5 4.5 14H11l-1 7.5L19.5 10H13z" /></svg>;
    case 'arrow-right':
      return <svg {...props}><path d="M5 12h14M13 6l6 6-6 6" /></svg>;
    case 'palette':
      return <svg {...props}><path d="M12 3.5a8.5 8.5 0 1 0 0 17c1.4 0 2.2-.8 2.2-1.8 0-1.2-1.2-1.5-1.2-2.5 0-.8.7-1.4 1.6-1.4h1.6A4.3 4.3 0 0 0 20.5 10c0-3.6-3.8-6.5-8.5-6.5z" /><circle cx="7.5" cy="10" r=".9" fill="currentColor" stroke="none" /><circle cx="10" cy="6.8" r=".9" fill="currentColor" stroke="none" /><circle cx="14.2" cy="6.8" r=".9" fill="currentColor" stroke="none" /></svg>;
    case 'bookmark':
      return <svg {...props}><path d="M6.5 4.5h11v16L12 16.8 6.5 20.5z" /></svg>;
    case 'file-text':
      return <svg {...props}><path d="M6 3.5h8l4 4v13H6z" /><path d="M14 3.5v4h4M9 11h6M9 14.5h6M9 18h4" /></svg>;
    case 'pencil':
      return <svg {...props}><path d="M4 20h4l11-11-4-4L4 16z" /><path d="m13.5 6.5 4 4" /></svg>;
    case 'settings-sources':
      return <svg {...props}><circle cx="12" cy="12" r="1.7" fill="currentColor" stroke="none" /><path d="M8.7 8.7a4.7 4.7 0 0 0 0 6.6M15.3 8.7a4.7 4.7 0 0 1 0 6.6" /><path d="M5.8 5.8a8.8 8.8 0 0 0 0 12.4M18.2 5.8a8.8 8.8 0 0 1 0 12.4" /></svg>;
    case 'settings-reading':
      return <svg {...props}><path d="M12 7.3C10.2 5.9 7.8 5.7 5.5 6.5v11.2c2.3-.8 4.7-.6 6.5.7 1.8-1.3 4.2-1.5 6.5-.7V6.5c-2.3-.8-4.7-.6-6.5.8Z" /><path d="M12 7.3v10.7" /></svg>;
    case 'settings-ai':
      return <svg {...props}><path d="m4.5 19.5 9.5-9.5" /><path d="m16.5 3.5 1.1 2.9 2.9 1.1-2.9 1.1-1.1 2.9-1.1-2.9-2.9-1.1 2.9-1.1z" /><path d="m20 12.5.7 1.8 1.8.7-1.8.7-.7 1.8-.7-1.8-1.8-.7 1.8-.7z" /></svg>;
    case 'settings-wechat':
      return <svg {...props}><path d="M5.5 5.5h13a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2h-6.2L8.3 20v-2.5H5.5a2 2 0 0 1-2-2v-8a2 2 0 0 1 2-2Z" /><path d="M7.5 10h9M7.5 13.2h5.5" /><circle cx="16.8" cy="13.2" r="1" fill="currentColor" stroke="none" /></svg>;
    case 'settings-backup':
      return <svg {...props}><path d="M4.5 8.5h15v10h-15z" /><path d="M3.5 4.5h17v4h-17z" /><path d="M12 11v5M9.8 13.8 12 16l2.2-2.2" /></svg>;
    case 'settings-security':
      return <svg {...props}><path d="M12 3.5 19 6v5.2c0 4.4-2.8 7.6-7 9.3-4.2-1.7-7-4.9-7-9.3V6z" /><circle cx="12" cy="10.8" r="1.7" /><path d="M12 12.5v3.2" /></svg>;
    default:
      return <svg {...props}><circle cx="12" cy="12" r="8" /></svg>;
  }
};

export const Wordmark = ({ small = false }: { small?: boolean }) => (
  <span className={`wordmark ${small ? 'small' : ''}`} aria-label="Rill">
    <span className="wordmark-text">Rill</span>
    <img className="wordmark-mark" src="/icon.png?v=3" alt="" aria-hidden="true" />
  </span>
);
