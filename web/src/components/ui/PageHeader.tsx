import type { ReactNode } from 'react';
import { Icon, type IconName } from '@/components/ui/Icon';

interface PageHeaderProps {
  eyebrow?: string;
  title: string;
  icon?: IconName;
  meta?: string;
  actions?: ReactNode;
  children?: ReactNode;
}

export const PageHeader = ({ eyebrow, title, icon, meta, actions, children }: PageHeaderProps) => (
  <header className="page-head">
    <div className="page-head-main">
      {icon ? <span className="page-head-icon"><Icon name={icon} size={22} /></span> : null}
      <div className="page-head-copy">
        {eyebrow ? <div className="eyebrow">{eyebrow}</div> : null}
        <div className="page-title-row">
          <h1>{title}</h1>
          {meta ? <span className="page-meta">{meta}</span> : null}
        </div>
        {children}
      </div>
    </div>
    {actions ? <div className="page-actions">{actions}</div> : null}
  </header>
);
