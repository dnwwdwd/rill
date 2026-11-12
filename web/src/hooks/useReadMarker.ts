import { useEffect, useRef, type RefObject } from 'react';

interface Options {
  itemId: number | null;
  isRead: boolean;
  enabled: boolean;
  targetRef: RefObject<HTMLElement | null>;
  onMarkRead: (id: number) => void;
  delayMs?: number;
}

export const useReadMarker = ({
  itemId,
  isRead,
  enabled,
  targetRef,
  onMarkRead,
  delayMs = 3000,
}: Options) => {
  const callbackRef = useRef(onMarkRead);
  callbackRef.current = onMarkRead;

  useEffect(() => {
    if (!enabled || isRead || !itemId) return undefined;
    const element = targetRef.current;
    if (!element || typeof IntersectionObserver === 'undefined') return undefined;
    let timer: number | undefined;
    let fired = false;
    const observer = new IntersectionObserver((entries) => {
      const visible = entries.some((entry) => entry.isIntersecting);
      if (visible && !timer && !fired) {
        timer = window.setTimeout(() => {
          fired = true;
          callbackRef.current(itemId);
        }, delayMs);
      } else if (!visible && timer) {
        window.clearTimeout(timer);
        timer = undefined;
      }
    }, { threshold: 0 });
    observer.observe(element);
    return () => {
      window.clearTimeout(timer);
      observer.disconnect();
    };
  }, [itemId, isRead, enabled, targetRef, delayMs]);
};
