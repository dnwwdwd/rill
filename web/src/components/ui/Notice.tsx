import type { ReactNode } from 'react';
import { Icon, type IconName } from '@/components/ui/Icon';
import { cx } from '@/utils/cx';

interface NoticeProps {
  tone?: 'info' | 'warn' | 'danger';
  icon?: IconName;
  title: ReactNode;
  children?: ReactNode;
  action?: ReactNode;
  className?: string;
}

const iconFor = (tone: NoticeProps['tone']): IconName => {
  if (tone === 'warn') return 'alert-triangle';
  if (tone === 'danger') return 'alert-circle';
  return 'info';
};

export const Notice = ({
  tone = 'info',
  icon,
  title,
  children,
  action,
  className,
}: NoticeProps) => (
  <div className={cx('notice', tone !== 'info' && `notice-${tone}`, className)} role={tone === 'danger' ? 'alert' : undefined}>
    <span className="notice-icon"><Icon name={icon ?? iconFor(tone)} size={18} /></span>
    <div className="notice-copy">
      <strong>{title}</strong>
      {children ? <span>{children}</span> : null}
    </div>
    {action ? <div className="notice-action">{action}</div> : null}
  </div>
);
