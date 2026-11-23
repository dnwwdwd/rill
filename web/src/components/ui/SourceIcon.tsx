import { useEffect, useState, type CSSProperties } from 'react';
import type { SourceSummary } from '@/api/types';
import { Icon } from '@/components/ui/Icon';
import { cx } from '@/utils/cx';

const COLORS = [
  '#475569', '#52525B', '#3F4A45', '#55605A', '#4B5563',
  '#64748B', '#3F3F46', '#525B55', '#5B6470', '#6B7280',
];

export const sourceColor = (id: number): string => COLORS[Math.abs(id) % COLORS.length];

export const sourceInitials = (name: string): string => {
  const clean = name.trim();
  if (clean.startsWith('@')) return clean.slice(1, 2).toUpperCase();
  const numeric = clean.match(/^\d{2}/);
  if (numeric) return numeric[0];
  const first = clean.match(/[\u4e00-\u9fa5A-Za-z0-9]/)?.[0] ?? 'R';
  return /[A-Za-z]/.test(first) ? first.toUpperCase() : first;
};

interface SourceIconProps {
  source: Pick<SourceSummary, 'id' | 'name'> & {
    icon_url?: string | null;
    feed_url?: string | null;
    last_fetch_at?: string | null;
  };
  size?: number;
  className?: string;
}

export const SourceIcon = ({ source, size = 20, className }: SourceIconProps) => {
  const [broken, setBroken] = useState(false);
  // 源刷新后 last_fetch_at 变化 → 图标 URL 加版本参数强制浏览器重新请求；
  // 同时重置加载失败状态，否则旧的 broken=true 会一直显示首字母。
  const iconVersion = source.last_fetch_at ?? '';
  const iconSrc = source.icon_url
    ? `${source.icon_url}${source.icon_url.includes('?') ? '&' : '?'}v=${encodeURIComponent(iconVersion)}`
    : '';
  useEffect(() => {
    setBroken(false);
  }, [iconVersion, source.icon_url]);
  // REQ-041：IMAP 邮件源没有站点图标，统一用邮件图标代替首字母。
  const isMail = (source.feed_url ?? '').startsWith('imap://');
  const text = sourceInitials(source.name);
  const style = {
    '--icon-color': sourceColor(source.id),
    width: size,
    height: size,
    fontSize: text.length > 1 ? Math.max(9, Math.round(size * 0.34)) : Math.round(size * 0.46),
    borderRadius: Math.max(3, Math.round(size * 0.18)),
  } as CSSProperties;

  return (
    <span className={cx('source-icon', className)} style={style} aria-hidden="true">
      {iconSrc && !broken ? (
        <img src={iconSrc} alt="" onError={() => setBroken(true)} />
      ) : isMail ? (
        <Icon name="mail" size={Math.max(10, Math.round(size * 0.55))} />
      ) : text}
    </span>
  );
};
