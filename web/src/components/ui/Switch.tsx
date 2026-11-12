import type { ReactNode } from 'react';
import { cx } from '@/utils/cx';

interface SwitchProps {
  checked: boolean;
  onChange: (checked: boolean) => void;
  label: string;
  disabled?: boolean;
}

export const Switch = ({ checked, onChange, label, disabled = false }: SwitchProps) => (
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

interface SwitchRowProps {
  title: string;
  description?: string;
  checked: boolean;
  onChange: (checked: boolean) => void;
  disabled?: boolean;
  trailing?: ReactNode;
}

export const SwitchRow = ({
  title,
  description,
  checked,
  onChange,
  disabled = false,
  trailing,
}: SwitchRowProps) => (
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
