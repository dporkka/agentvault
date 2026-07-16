import { useEffect } from 'react';

export interface ToastProps {
  message: string;
  type?: 'success' | 'error' | 'info';
  onDismiss: () => void;
}

const TYPE_CLASSES: Record<string, string> = {
  success: 'toast--success',
  error: 'toast--error',
  info: 'toast--info',
};

export function Toast({ message, type = 'info', onDismiss }: ToastProps) {
  useEffect(() => {
    const timer = setTimeout(onDismiss, 3000);
    return () => clearTimeout(timer);
  }, [onDismiss]);

  return (
    <div className={`toast ${TYPE_CLASSES[type] ?? TYPE_CLASSES.info}`} role="alert">
      <span>{message}</span>
      <button onClick={onDismiss} className="toast__dismiss" aria-label="Dismiss">
        ×
      </button>
    </div>
  );
}
