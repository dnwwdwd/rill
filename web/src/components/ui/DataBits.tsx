import type { ReactNode } from 'react';
import { cx } from '@/utils/cx';

export const Kbd = ({ children }: { children: ReactNode }) => <kbd className="kbd">{children}</kbd>;

export const MiniBars = ({ values, label }: { values: number[]; label: string }) => {
  const max = Math.max(1, ...values);
  return (
    <div className="mini-bars" role="img" aria-label={label}>
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
};

export const Meter = ({ value, max, tone = 'accent' }: { value: number; max: number; tone?: 'accent' | 'warn' }) => {
  const percent = Math.min(100, Math.round((value / Math.max(1, max)) * 100));
  return (
    <div className={cx('meter', `meter-${tone}`)} role="progressbar" aria-valuemin={0} aria-valuemax={max} aria-valuenow={value}>
      <span style={{ width: `${percent}%` }}></span>
    </div>
  );
};

export const Divider = ({ label }: { label?: string }) => (
  <div className={cx('divider', label && 'has-label')}>{label ? <span>{label}</span> : null}</div>
);
