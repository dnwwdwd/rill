import type { ReactNode } from 'react';
import { cx } from '@/utils/cx';

/**
 * REQ-023 紧凑行式设置的基本件：
 * - RowGroup：一组相关行，组头是 11px eyebrow（前缀短横线），组内行用细线分隔；
 * - Row：标签居左、控件居右；label/hint 之外的 children 独占整行（输入框、二维码等）；
 * - SliderField：标签 + 当前值一行，滑杆整行。
 */

interface RowGroupProps {
  title: string;
  tone?: 'default' | 'danger';
  className?: string;
  children: ReactNode;
}

export const RowGroup = ({ title, tone = 'default', className, children }: RowGroupProps) => (
  <section className={cx('st-group', tone === 'danger' && 'st-group-danger', className)}>
    <h3 className="st-group-title">{title}</h3>
    <div className="st-group-body">{children}</div>
  </section>
);

interface RowProps {
  label?: ReactNode;
  hint?: ReactNode;
  control?: ReactNode;
  className?: string;
  children?: ReactNode;
}

export const Row = ({ label, hint, control, className, children }: RowProps) => (
  <div className={cx('st-row', !label && !hint && 'st-row-bare', className)}>
    {label || hint ? (
      <div className="st-row-label">
        {label ? <span className="st-row-name">{label}</span> : null}
        {hint ? <span className="st-row-hint">{hint}</span> : null}
      </div>
    ) : null}
    {control ? <div className="st-row-control">{control}</div> : null}
    {children}
  </div>
);

interface SliderFieldProps {
  label: string;
  value: number;
  min: number;
  max: number;
  step?: number;
  suffix?: string;
  hint?: string;
  onChange: (value: number) => void;
}

export const SliderField = ({
  label,
  value,
  min,
  max,
  step = 1,
  suffix = '',
  hint,
  onChange,
}: SliderFieldProps) => (
  <div className="st-row st-slider">
    <div className="st-row-label">
      <span className="st-row-name">{label}</span>
      <code className="st-slider-value">{value}{suffix}</code>
    </div>
    <input
      type="range"
      min={min}
      max={max}
      step={step}
      value={value}
      onChange={(event) => onChange(Number(event.target.value))}
      aria-label={label}
    />
    {hint ? <span className="st-row-hint">{hint}</span> : null}
  </div>
);
