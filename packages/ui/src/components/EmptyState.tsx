import React from 'react';

export interface EmptyStateProps {
  icon?: React.ReactNode;
  title: string;
  subtitle?: string;
  className?: string;
}

export const EmptyState: React.FC<EmptyStateProps> = ({
  icon,
  title,
  subtitle,
  className = '',
}) => {
  return (
    <div
      className={`flex flex-col items-center justify-center ${className}`}
      style={{ color: 'var(--av-text-muted)' }}
    >
      {icon && <div style={{ marginBottom: '0.75rem', opacity: 0.4 }}>{icon}</div>}
      <p style={{ fontSize: '0.875rem' }}>{title}</p>
      {subtitle && (
        <p style={{ fontSize: '0.75rem', marginTop: '0.25rem', opacity: 0.6 }}>
          {subtitle}
        </p>
      )}
    </div>
  );
};

export default EmptyState;
