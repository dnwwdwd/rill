import type { ButtonHTMLAttributes, ReactNode } from 'react';
import { Icon, type IconName } from '@/components/ui/Icon';
import { cx } from '@/utils/cx';

type ButtonVariant = 'primary' | 'secondary' | 'danger' | 'ghost';
type ButtonSize = 'sm' | 'md' | 'lg';

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
  size?: ButtonSize;
  icon?: IconName;
}

export const Button = ({
  variant = 'secondary',
  size = 'md',
  icon,
  children,
  className,
  type = 'button',
  ...rest
}: ButtonProps) => (
  <button
    type={type}
    className={cx('btn', `btn-${variant}`, `btn-${size}`, className)}
    {...rest}
  >
    {icon ? <Icon name={icon} size={size === 'sm' ? 15 : 17} /> : null}
    {children ? <span>{children}</span> : null}
  </button>
);

interface IconButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  icon: IconName;
  label: string;
  active?: boolean;
  filled?: boolean;
  size?: number;
}

export const IconButton = ({
  icon,
  label,
  active = false,
  filled = false,
  size = 18,
  className,
  type = 'button',
  ...rest
}: IconButtonProps) => (
  <button
    type={type}
    className={cx('icon-btn', active && 'is-active', className)}
    aria-label={label}
    data-tooltip={label}
    {...rest}
  >
    <Icon name={icon} size={size} filled={filled} />
  </button>
);

interface TextLinkProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  children: ReactNode;
}

export const TextLink = ({ children, className, type = 'button', ...rest }: TextLinkProps) => (
  <button type={type} className={cx('text-link', className)} {...rest}>{children}</button>
);
