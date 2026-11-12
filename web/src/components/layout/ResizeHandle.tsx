import { useEffect, useRef, useState, type KeyboardEvent as ReactKeyboardEvent, type PointerEvent as ReactPointerEvent } from 'react';
import { cx } from '@/utils/cx';

interface ResizeHandleProps {
  label: string;
  value: number;
  min: number;
  max: number;
  defaultValue: number;
  cssVar: string;
  onCommit: (value: number) => void;
}

const clamp = (value: number, min: number, max: number) => Math.min(max, Math.max(min, Math.round(value)));

export const ResizeHandle = ({
  label,
  value,
  min,
  max,
  defaultValue,
  cssVar,
  onCommit,
}: ResizeHandleProps) => {
  const safeValue = clamp(value, min, max);
  const [current, setCurrent] = useState(safeValue);
  const [dragging, setDragging] = useState(false);
  const currentRef = useRef(safeValue);
  const draggingRef = useRef(false);
  const dragRef = useRef({ startX: 0, startWidth: safeValue });

  useEffect(() => {
    if (draggingRef.current) return;
    currentRef.current = safeValue;
    setCurrent(safeValue);
  }, [safeValue]);

  useEffect(() => () => {
    document.body.classList.remove('is-resizing');
  }, []);

  const apply = (next: number) => {
    currentRef.current = next;
    setCurrent(next);
    document.documentElement.style.setProperty(cssVar, `${next}px`);
  };

  const commit = (next: number) => {
    apply(next);
    onCommit(next);
  };

  const onPointerDown = (event: ReactPointerEvent<HTMLDivElement>) => {
    if (event.button !== 0) return;
    event.preventDefault();
    draggingRef.current = true;
    dragRef.current = { startX: event.clientX, startWidth: currentRef.current };
    setDragging(true);
    event.currentTarget.setPointerCapture(event.pointerId);
    document.body.classList.add('is-resizing');
  };

  const onPointerMove = (event: ReactPointerEvent<HTMLDivElement>) => {
    if (!draggingRef.current) return;
    const next = clamp(dragRef.current.startWidth + event.clientX - dragRef.current.startX, min, max);
    apply(next);
  };

  const finish = (event: ReactPointerEvent<HTMLDivElement>) => {
    if (!draggingRef.current) return;
    draggingRef.current = false;
    setDragging(false);
    document.body.classList.remove('is-resizing');
    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId);
    }
    onCommit(currentRef.current);
  };

  const onKeyDown = (event: ReactKeyboardEvent<HTMLDivElement>) => {
    let next = currentRef.current;
    if (event.key === 'ArrowLeft') next -= 12;
    else if (event.key === 'ArrowRight') next += 12;
    else if (event.key === 'Home') next = min;
    else if (event.key === 'End') next = max;
    else return;
    event.preventDefault();
    commit(clamp(next, min, max));
  };

  return (
    <div
      className={cx('resize-handle', dragging && 'is-dragging')}
      role="separator"
      aria-label={label}
      aria-orientation="vertical"
      aria-valuemin={min}
      aria-valuemax={max}
      aria-valuenow={current}
      tabIndex={0}
      title={`${label}（双击恢复默认）`}
      onPointerDown={onPointerDown}
      onPointerMove={onPointerMove}
      onPointerUp={finish}
      onPointerCancel={finish}
      onKeyDown={onKeyDown}
      onDoubleClick={() => commit(defaultValue)}
    />
  );
};
