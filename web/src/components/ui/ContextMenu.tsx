import {
  Fragment,
  useEffect,
  useLayoutEffect,
  useRef,
  type KeyboardEvent,
} from 'react';
import { createPortal } from 'react-dom';
import { Icon, type IconName } from '@/components/ui/Icon';
import { cx } from '@/utils/cx';

export interface ContextMenuItem {
  id: string;
  label: string;
  icon?: IconName;
  danger?: boolean;
  disabled?: boolean;
  /** 传 undefined 时不显示勾选列；true/false 显示勾选状态。 */
  checked?: boolean;
  separator?: boolean;
  indent?: number;
  /** 切换菜单视图时保持打开（如「移动到分组…」/「返回」）。 */
  keepOpen?: boolean;
  onSelect: () => void;
}

interface ContextMenuProps {
  x: number;
  y: number;
  items: ContextMenuItem[];
  label: string;
  title?: string;
  onClose: () => void;
}

const VIEWPORT_MARGIN = 8;

export const ContextMenu = ({ x, y, items, label, title, onClose }: ContextMenuProps) => {
  const menuRef = useRef<HTMLDivElement>(null);
  const closeRef = useRef(onClose);
  closeRef.current = onClose;

  useLayoutEffect(() => {
    const menu = menuRef.current;
    if (!menu) return;
    const rect = menu.getBoundingClientRect();
    const left = Math.max(VIEWPORT_MARGIN, Math.min(x, window.innerWidth - rect.width - VIEWPORT_MARGIN));
    const top = Math.max(VIEWPORT_MARGIN, Math.min(y, window.innerHeight - rect.height - VIEWPORT_MARGIN));
    menu.style.left = `${left}px`;
    menu.style.top = `${top}px`;
  }, [x, y, items]);

  useEffect(() => {
    const menu = menuRef.current;
    menu?.querySelector<HTMLButtonElement>('.tree-menu-item:not(:disabled)')?.focus();
    const onPointerDown = (event: MouseEvent) => {
      if (!menuRef.current?.contains(event.target as Node)) closeRef.current();
    };
    const onKey = (event: globalThis.KeyboardEvent) => {
      if (event.key === 'Escape') {
        event.preventDefault();
        closeRef.current();
      }
    };
    const close = () => closeRef.current();
    const onScroll = (event: Event) => {
      if (!menuRef.current?.contains(event.target as Node)) closeRef.current();
    };
    document.addEventListener('mousedown', onPointerDown, true);
    window.addEventListener('keydown', onKey);
    window.addEventListener('resize', close);
    window.addEventListener('blur', close);
    window.addEventListener('scroll', onScroll, true);
    return () => {
      document.removeEventListener('mousedown', onPointerDown, true);
      window.removeEventListener('keydown', onKey);
      window.removeEventListener('resize', close);
      window.removeEventListener('blur', close);
      window.removeEventListener('scroll', onScroll, true);
    };
  }, [items.length, title]);

  const moveFocus = (event: KeyboardEvent<HTMLDivElement>, direction: 1 | -1) => {
    if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp') return;
    event.preventDefault();
    const buttons = [...(menuRef.current?.querySelectorAll<HTMLButtonElement>('.tree-menu-item:not(:disabled)') ?? [])];
    if (buttons.length === 0) return;
    const current = buttons.indexOf(document.activeElement as HTMLButtonElement);
    const next = (current + direction + buttons.length) % buttons.length;
    buttons[next]?.focus();
  };

  return createPortal(
    <div
      ref={menuRef}
      className="tree-menu"
      role="menu"
      aria-label={label}
      style={{ left: x, top: y }}
      onKeyDown={(event) => moveFocus(event, event.key === 'ArrowUp' ? -1 : 1)}
    >
      {title ? <div className="tree-menu-title">{title}</div> : null}
      {items.map((item) => (
        <Fragment key={item.id}>
          {item.separator ? <div className="tree-menu-sep" role="separator" /> : null}
          <button
            type="button"
            role="menuitem"
            className={cx('tree-menu-item', item.danger && 'is-danger')}
            style={item.indent ? { paddingLeft: 10 + item.indent * 14 } : undefined}
            disabled={item.disabled}
            onClick={() => {
              if (!item.keepOpen) closeRef.current();
              item.onSelect();
            }}
          >
            {item.checked === undefined ? (
              <span className="tree-menu-icon">
                {item.icon ? <Icon name={item.icon} size={14} /> : null}
              </span>
            ) : (
              <span className="tree-menu-icon">
                {item.checked ? <Icon name="check" size={13} /> : null}
              </span>
            )}
            <span className="tree-menu-label">{item.label}</span>
          </button>
        </Fragment>
      ))}
    </div>,
    document.body,
  );
};
