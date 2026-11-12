import type { ReactNode } from 'react';
import { Button, Modal } from '@/components/ui';

interface ConfirmDialogProps {
  open: boolean;
  title: string;
  description: ReactNode;
  confirmLabel?: string;
  busy?: boolean;
  onConfirm: () => void;
  onClose: () => void;
}

export const ConfirmDialog = ({
  open,
  title,
  description,
  confirmLabel = '确认删除',
  busy = false,
  onConfirm,
  onClose,
}: ConfirmDialogProps) => (
  <Modal
    open={open}
    onClose={() => { if (!busy) onClose(); }}
    title={title}
    footer={(
      <>
        <Button onClick={onClose} disabled={busy}>取消</Button>
        <Button variant="danger" onClick={onConfirm} disabled={busy}>
          {busy ? '正在删除' : confirmLabel}
        </Button>
      </>
    )}
  >
    <p className="confirm-copy">{description}</p>
  </Modal>
);
