export const TYPE_FILTERS = ['all', 'note', 'decision', 'task', 'meeting', 'source'] as const;
export type TypeFilter = (typeof TYPE_FILTERS)[number];

export const typeBadgeClass = (type: string): string => {
  switch (type) {
    case 'note': return 'type-badge-note';
    case 'decision': return 'type-badge-decision';
    case 'task': return 'type-badge-task';
    case 'meeting': return 'type-badge-meeting';
    case 'source': return 'type-badge-source';
    default: return 'type-badge-default';
  }
};
