import { useState, type CSSProperties } from 'react';
import type { SourceSummary } from '@/api/types';
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
  source: Pick<SourceSummary, 'id' | 'name'> & { icon_url?: string | null };
  size?: number;
  className?: string;
}

export const SourceIcon = ({ source, size = 20, className }: SourceIconProps) => {
  const [broken, setBroken] = useState(false);
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
      {source.icon_url && !broken ? (
        <img src={source.icon_url} alt="" onError={() => setBroken(true)} />
      ) : text}
    </span>
  );
};
