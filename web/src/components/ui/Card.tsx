import type { HTMLAttributes, ReactNode } from 'react';
import { Icon, type IconName } from '@/components/ui/Icon';
import { cx } from '@/utils/cx';

interface CardProps extends HTMLAttributes<HTMLElement> {
  tone?: 'default' | 'warm' | 'danger';
  children: ReactNode;
}

export const Card = ({ tone = 'default', className, children, ...rest }: CardProps) => (
  <section className={cx('card', `card-${tone}`, className)} {...rest}>{children}</section>
);

interface SectionHeadingProps {
  title: string;
  icon?: IconName;
  eyebrow?: string;
  description?: string;
  action?: ReactNode;
  className?: string;
}

export const SectionHeading = ({ title, icon, eyebrow, description, action, className }: SectionHeadingProps) => (
  <div className={cx('section-heading', className)}>
    {icon ? <span className="section-heading-icon"><Icon name={icon} size={18} /></span> : null}
    <div className="section-heading-copy">
      {eyebrow ? <div className="eyebrow">{eyebrow}</div> : null}
      <h2>{title}</h2>
      {description ? <p>{description}</p> : null}
    </div>
    {action ? <div className="section-heading-action">{action}</div> : null}
  </div>
);

interface MetricProps {
  icon: IconName;
  label: string;
  value: string;
  tone?: '' | 'warn';
}

export const Metric = ({ icon, label, value, tone = '' }: MetricProps) => (
  <div className={cx('metric', tone)}>
    <span className="metric-icon"><Icon name={icon} size={16} /></span>
    <span className="metric-label">{label}</span>
    <strong className="metric-value">{value}</strong>
  </div>
);
