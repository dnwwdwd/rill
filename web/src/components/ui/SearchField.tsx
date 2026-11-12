import type { KeyboardEventHandler } from 'react';
import { Icon } from '@/components/ui/Icon';
import { IconButton } from '@/components/ui/Button';
import { cx } from '@/utils/cx';

interface SearchFieldProps {
  value: string;
  onChange: (value: string) => void;
  placeholder: string;
  ariaLabel: string;
  className?: string;
  autoFocus?: boolean;
  onKeyDown?: KeyboardEventHandler<HTMLInputElement>;
}

export const SearchField = ({
  value,
  onChange,
  placeholder,
  ariaLabel,
  className,
  autoFocus = false,
  onKeyDown,
}: SearchFieldProps) => (
  <label className={cx('search-field', className)}>
    <Icon name="search" size={16} />
    <input
      value={value}
      onChange={(event) => onChange(event.target.value)}
      onKeyDown={onKeyDown}
      placeholder={placeholder}
      aria-label={ariaLabel}
      autoFocus={autoFocus}
    />
    {value ? (
      <IconButton icon="x" label="清除搜索" size={14} onClick={() => onChange('')} />
    ) : null}
  </label>
);
