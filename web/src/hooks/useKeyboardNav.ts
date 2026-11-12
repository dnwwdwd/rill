import { useEffect } from 'react';

interface Handlers {
  enabled: boolean;
  onNext: () => void;
  onPrevious: () => void;
  onOpen: () => void;
  onFocus: () => void;
  onToggleRead: () => void;
  onToggleStar: () => void;
  onRefresh: () => void;
  onFirst: () => void;
}

const isInteractiveTarget = (target: EventTarget | null): boolean => {
  if (!(target instanceof Element)) return false;
  return Boolean(target.closest('input, textarea, select, button, a, [contenteditable="true"]'));
};

export const useKeyboardNav = (handlers: Handlers) => {
  useEffect(() => {
    if (!handlers.enabled) return undefined;
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.metaKey || event.ctrlKey || event.altKey) return;
      if (isInteractiveTarget(event.target)) return;
      const key = event.key;
      const repeated = event.repeat;
      if (key === 'j' || key === 'ArrowDown') {
        event.preventDefault();
        handlers.onNext();
        return;
      }
      if (key === 'k' || key === 'ArrowUp') {
        event.preventDefault();
        handlers.onPrevious();
        return;
      }
      if (repeated) return;
      if (key === 'o' || key === 'Enter') {
        event.preventDefault();
        handlers.onOpen();
      } else if (key === 'f') {
        event.preventDefault();
        handlers.onFocus();
      } else if (key === 'm') {
        event.preventDefault();
        handlers.onToggleRead();
      } else if (key === 's') {
        event.preventDefault();
        handlers.onToggleStar();
      } else if (key === 'r') {
        event.preventDefault();
        handlers.onRefresh();
      } else if (key === 'g') {
        event.preventDefault();
        handlers.onFirst();
      }
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [handlers]);
};
