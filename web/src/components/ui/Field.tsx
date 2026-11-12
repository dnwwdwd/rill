import type { ReactNode } from 'react';
import { cx } from '@/utils/cx';

interface FieldProps {
  label: string;
  hint?: string;
  children: ReactNode;
  className?: string;
}

export const Field = ({ label, hint, children, className }: FieldProps) => (
  <label className={cx('field', className)}>
    <span className="field-label">{label}</span>
    {children}
    {hint ? <span className="field-hint">{hint}</span> : null}
  </label>
);
