import React from 'react';

export interface TypeBadgeProps {
  type: string;
  className?: string;
}

const TYPE_COLORS: Record<string, { bg: string; text: string }> = {
  note:     { bg: 'rgba(34,197,94,0.15)',  text: '#4ade80' },
  decision: { bg: 'rgba(245,158,11,0.15)', text: '#fbbf24' },
  task:     { bg: 'rgba(59,130,246,0.15)', text: '#60a5fa' },
  meeting:  { bg: 'rgba(168,85,247,0.15)', text: '#c084fc' },
  source:   { bg: 'rgba(244,63,94,0.15)',  text: '#fb7185' },
};

const defaultColors = { bg: 'rgba(107,114,128,0.15)', text: '#9ca3af' };

export const TypeBadge: React.FC<TypeBadgeProps> = ({ type, className = '' }) => {
  const colors = TYPE_COLORS[type] ?? defaultColors;
  return (
    <span
      className={`inline-flex items-center px-2 py-0.5 text-xs font-medium rounded-full ${className}`}
      style={{
        backgroundColor: colors.bg,
        color: colors.text,
      }}
    >
      {type}
    </span>
  );
};

export default TypeBadge;
