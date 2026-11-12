import { Sidebar } from '@/components/layout/Sidebar';
import { useUiStore } from '@/stores/uiStore';
import { cx } from '@/utils/cx';

export const Drawer = () => {
  const open = useUiStore((state) => state.drawerOpen);
  const close = useUiStore((state) => state.closeDrawer);
  return (
    <div className={cx('drawer-layer', open && 'is-open')} aria-hidden={!open}>
      <button type="button" className="drawer-scrim" aria-label="关闭导航" onClick={close} />
      <div className="drawer-panel">
        <Sidebar inDrawer onCloseDrawer={close} />
      </div>
    </div>
  );
};
