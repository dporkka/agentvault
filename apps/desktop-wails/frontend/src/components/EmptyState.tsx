import type { ReactNode } from 'react';

interface EmptyStateProps {
  icon: ReactNode;
  title: string;
  description?: string;
  action?: {
    label: string;
    onClick: () => void;
  };
}

export default function EmptyState({ icon, title, description, action }: EmptyStateProps) {
  return (
    <div className="flex flex-col items-center justify-center h-full text-center px-6 py-12">
      <div className="text-text-muted/40 mb-4">{icon}</div>
      <h3 className="text-sm font-medium text-text-primary">{title}</h3>
      {description && (
        <p className="text-xs text-text-secondary mt-1 max-w-xs">{description}</p>
      )}
      {action && (
        <button
          type="button"
          onClick={action.onClick}
          className="mt-4 px-3 py-1.5 text-xs font-medium rounded bg-accent text-white hover:bg-accent-hover transition-colors focus:outline-none focus:ring-2 focus:ring-accent/50"
        >
          {action.label}
        </button>
      )}
    </div>
  );
}
