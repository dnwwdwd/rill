import type { ReactNode } from 'react';
import { Icon, type IconName } from '@/components/ui/Icon';

interface EmptyStateProps {
  icon?: IconName;
  title: string;
  description?: string;
  action?: ReactNode;
}

export const EmptyState = ({ icon = 'inbox', title, description, action }: EmptyStateProps) => (
  <div className="empty-state">
    <div className="empty-icon"><Icon name={icon} size={24} /></div>
    <h3>{title}</h3>
    {description ? <p>{description}</p> : null}
    {action ? <div className="empty-actions">{action}</div> : null}
  </div>
);
