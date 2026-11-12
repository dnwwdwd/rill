import { useUiStore } from '@/stores/uiStore';

export const ToastHost = () => {
  const toast = useUiStore((state) => state.toast);
  const clearToast = useUiStore((state) => state.clearToast);
  if (!toast) return null;
  return (
    <div className="toast" role="status" aria-live="polite">
      <span className="toast-mark" aria-hidden="true"></span>
      <span className="toast-message">{toast.message}</span>
      {toast.actionLabel ? (
        <button
          type="button"
          className="toast-action"
          onClick={() => {
            toast.onAction?.();
            clearToast();
          }}
        >
          {toast.actionLabel}
        </button>
      ) : null}
    </div>
  );
};
