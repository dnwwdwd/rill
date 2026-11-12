import type { SourceStatus } from '@/api/types';
import { statusLabel } from '@/utils/sourceStatus';
import { cx } from '@/utils/cx';

const classFor = (status: SourceStatus) => status;

export const StatusDot = ({ status, label }: { status: SourceStatus; label?: string }) => (
  <span className={cx('status-dot', classFor(status))} title={label ?? statusLabel(status)} aria-label={label ?? statusLabel(status)}>
    <i></i>
  </span>
);

export const StatusBadge = ({ status, note, compact = false }: { status: SourceStatus; note?: string | null; compact?: boolean }) => (
  <span className={cx('status-badge', classFor(status), compact && 'compact')} title={note ?? statusLabel(status)}>
    <i></i>
    <span>{statusLabel(status)}</span>
  </span>
);
