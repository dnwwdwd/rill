import type { ReactNode } from 'react';
import { Icon, type IconName } from '@/components/ui/Icon';
import { cx } from '@/utils/cx';

type TagTone = 'neutral' | 'accent' | 'wechat' | 'x' | 'danger';

interface TagProps {
  children: ReactNode;
  tone?: TagTone;
  icon?: IconName;
  className?: string;
  title?: string;
}

export const Tag = ({ children, tone = 'neutral', icon, className, title }: TagProps) => (
  <span className={cx('tag', `tag-${tone}`, className)} title={title}>
    {icon ? <Icon name={icon} size={11} /> : null}
    {children}
  </span>
);
