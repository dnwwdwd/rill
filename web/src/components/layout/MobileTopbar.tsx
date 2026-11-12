import type { ReactNode } from 'react';
import { useNavigate } from 'react-router-dom';
import { IconButton, Wordmark } from '@/components/ui';
import { useUiStore } from '@/stores/uiStore';

interface MobileTopbarProps {
  title: string;
  backTo?: string;
  onBack?: () => void;
  right?: ReactNode;
  showBrand?: boolean;
}

export const MobileTopbar = ({ title, backTo, onBack, right, showBrand = false }: MobileTopbarProps) => {
  const navigate = useNavigate();
  const openDrawer = useUiStore((state) => state.openDrawer);
  return (
    <header className="mobile-bar">
      <div className="mobile-bar-left">
        {backTo || onBack ? (
          <IconButton icon="arrow-left" label="返回" onClick={() => { if (onBack) onBack(); else if (backTo) navigate(backTo); }} />
        ) : (
          <IconButton icon="menu" label="打开导航" onClick={openDrawer} />
        )}
        <div className="mobile-bar-title">
          {showBrand ? <Wordmark small /> : null}
          {showBrand ? <span className="mobile-sep">/</span> : null}
          <span>{title}</span>
        </div>
      </div>
      <div className="mobile-bar-right">{right}</div>
    </header>
  );
};
