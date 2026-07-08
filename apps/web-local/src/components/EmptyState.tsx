import React from 'react';

interface EmptyStateProps {
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
    <div className={`flex flex-col items-center justify-center text-vault-text-muted ${className}`}>
      {icon && <div className="mb-3 opacity-40">{icon}</div>}
      <p className="text-sm">{title}</p>
      {subtitle && <p className="text-xs mt-1 opacity-60">{subtitle}</p>}
    </div>
  );
};

export default EmptyState;
