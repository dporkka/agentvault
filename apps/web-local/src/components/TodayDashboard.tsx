import React from 'react';
import { useNavigate } from 'react-router-dom';
import { api } from '@/api/client';
import { useApi } from '@/hooks/useApi';
import type { DashboardResponse, TaskResult, DecisionResult, CaptureSummary } from '@agentvault/contract';

function formatDueDate(dueDate: string): string {
  if (!dueDate) return 'No due date';
  const date = new Date(dueDate);
  if (isNaN(date.getTime())) return dueDate;

  const today = new Date();
  today.setHours(0, 0, 0, 0);
  const dateMidnight = new Date(date);
  dateMidnight.setHours(0, 0, 0, 0);

  const diffTime = dateMidnight.getTime() - today.getTime();
  const diffDays = Math.round(diffTime / (1000 * 60 * 60 * 24));

  if (diffDays === 0) return 'Today';
  if (diffDays === 1) return 'Tomorrow';
  if (diffDays === -1) return 'Yesterday';

  return date.toLocaleDateString('en-US', { month: 'short', day: 'numeric' });
}

function priorityClass(priority: string): string {
  switch (priority?.toLowerCase()) {
    case 'high':
      return 'bg-red-500/15 text-red-400';
    case 'medium':
      return 'bg-amber-500/15 text-amber-400';
    case 'low':
      return 'bg-emerald-500/15 text-emerald-400';
    default:
      return 'bg-gray-500/15 text-gray-400';
  }
}

function typeBadgeClass(type: string): string {
  switch (type) {
    case 'task':
      return 'type-badge-task';
    case 'decision':
      return 'type-badge-decision';
    case 'meeting':
      return 'type-badge-meeting';
    case 'capture':
      return 'type-badge-source';
    default:
      return 'type-badge-default';
  }
}

interface SectionProps {
  title: string;
  children: React.ReactNode;
  count?: number;
}

const Section: React.FC<SectionProps> = ({ title, children, count }) => (
  <section className="mb-6">
    <div className="flex items-center gap-2 mb-3">
      <h2 className="text-sm font-semibold text-vault-text-primary uppercase tracking-wider">{title}</h2>
      {count !== undefined && (
        <span className="text-xs text-vault-text-muted bg-vault-bg-tertiary px-2 py-0.5 rounded-full">{count}</span>
      )}
    </div>
    {children}
  </section>
);

interface TaskItemProps {
  task: TaskResult;
  onClick: () => void;
  urgent?: boolean;
}

const TaskItem: React.FC<TaskItemProps> = ({ task, onClick, urgent }) => (
  <button
    onClick={onClick}
    className={`w-full text-left p-3 rounded-lg mb-2 transition-colors group ${
      urgent ? 'bg-red-500/5 hover:bg-red-500/10 border border-red-500/10' : 'bg-vault-bg-secondary hover:bg-vault-bg-hover'
    }`}
  >
    <div className="flex items-start justify-between gap-3">
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2 mb-1">
          <h3 className="text-sm font-medium text-vault-text-primary group-hover:text-vault-accent transition-colors truncate">
            {task.title}
          </h3>
          <span className={`type-badge ${typeBadgeClass(task.type)}`}>{task.type}</span>
        </div>
        {task.snippet && (
          <p className="text-xs text-vault-text-secondary line-clamp-2 leading-relaxed">{task.snippet}</p>
        )}
      </div>
      <div className="flex flex-col items-end gap-1.5 flex-shrink-0">
        <span className={`text-xs font-medium ${urgent ? 'text-red-400' : 'text-vault-text-secondary'}`}>
          {formatDueDate(task.dueDate)}
        </span>
        <span className={`inline-flex items-center px-2 py-0.5 text-xs font-medium rounded-full ${priorityClass(task.priority)}`}>
          {task.priority || 'none'}
        </span>
      </div>
    </div>
  </button>
);

interface DecisionItemProps {
  decision: DecisionResult;
  onClick: () => void;
}

const DecisionItem: React.FC<DecisionItemProps> = ({ decision, onClick }) => (
  <button
    onClick={onClick}
    className="w-full text-left p-3 rounded-lg mb-2 bg-vault-bg-secondary hover:bg-vault-bg-hover transition-colors group"
  >
    <div className="flex items-center gap-2 mb-1">
      <h3 className="text-sm font-medium text-vault-text-primary group-hover:text-vault-accent transition-colors truncate">
        {decision.title}
      </h3>
      <span className={`type-badge ${typeBadgeClass(decision.type)}`}>{decision.type}</span>
    </div>
    {decision.snippet && (
      <p className="text-xs text-vault-text-secondary line-clamp-2 leading-relaxed">{decision.snippet}</p>
    )}
  </button>
);

interface CaptureItemProps {
  capture: CaptureSummary;
  onClick: () => void;
}

const CaptureItem: React.FC<CaptureItemProps> = ({ capture, onClick }) => (
  <button
    onClick={onClick}
    className="w-full text-left p-2 rounded-lg mb-1 bg-vault-bg-secondary hover:bg-vault-bg-hover transition-colors group"
  >
    <div className="flex items-center gap-2">
      <span className={`type-badge ${typeBadgeClass(capture.type)}`}>{capture.type}</span>
      <span className="text-sm text-vault-text-primary group-hover:text-vault-accent transition-colors truncate">
        {capture.title}
      </span>
    </div>
  </button>
);

const EmptyState: React.FC<{ message: string }> = ({ message }) => (
  <div className="p-4 rounded-lg bg-vault-bg-secondary text-center">
    <p className="text-sm text-vault-text-muted">{message}</p>
  </div>
);

const TodayDashboard: React.FC = () => {
  const navigate = useNavigate();
  const { data: dashboard, loading, error } = useApi<DashboardResponse>(
    () => api.getDashboard(),
    []
  );

  if (loading) {
    return (
      <div className="h-full flex items-center justify-center">
        <div className="flex items-center gap-3 text-vault-text-muted">
          <div className="w-5 h-5 border-2 border-vault-accent border-t-transparent rounded-full animate-spin" />
          Loading today&apos;s focus...
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="h-full flex items-center justify-center px-6">
        <div className="text-center max-w-md">
          <svg className="w-10 h-10 text-vault-error mx-auto mb-3" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={1}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m9-.75a9 9 0 1 1-18 0 9 9 0 0 1 18 0Zm-9 3.75h.008v.008H12v-.008Z" />
          </svg>
          <p className="text-sm text-vault-error mb-2">{error}</p>
          <p className="text-xs text-vault-text-muted">
            Make sure the AgentVault server is running and supports the dashboard endpoint.
          </p>
        </div>
      </div>
    );
  }

  if (!dashboard) {
    return (
      <div className="h-full flex items-center justify-center text-vault-text-muted">
        <p className="text-sm">No dashboard data available.</p>
      </div>
    );
  }

  const { overdueTasks, upcomingTasks, pendingDecisions, recentCaptures } = dashboard;

  return (
    <div className="h-full overflow-y-auto px-6 py-5">
      <div className="max-w-3xl mx-auto">
        <header className="mb-6">
          <h1 className="text-xl font-semibold text-vault-text-primary">Today</h1>
          <p className="text-sm text-vault-text-secondary mt-1">
            {new Date().toLocaleDateString('en-US', { weekday: 'long', month: 'long', day: 'numeric' })}
          </p>
        </header>

        {overdueTasks.length > 0 && (
          <Section title="Overdue" count={overdueTasks.length}>
            {overdueTasks.map((task) => (
              <TaskItem
                key={task.id}
                task={task}
                urgent
                onClick={() => navigate(`/note/${encodeURIComponent(task.id)}`)}
              />
            ))}
          </Section>
        )}

        <Section title="Upcoming" count={upcomingTasks.length}>
          {upcomingTasks.length > 0 ? (
            upcomingTasks.map((task) => (
              <TaskItem
                key={task.id}
                task={task}
                onClick={() => navigate(`/note/${encodeURIComponent(task.id)}`)}
              />
            ))
          ) : (
            <EmptyState message="No upcoming tasks. You're all caught up!" />
          )}
        </Section>

        <Section title="Pending Decisions" count={pendingDecisions.length}>
          {pendingDecisions.length > 0 ? (
            pendingDecisions.map((decision) => (
              <DecisionItem
                key={decision.id}
                decision={decision}
                onClick={() => navigate(`/note/${encodeURIComponent(decision.id)}`)}
              />
            ))
          ) : (
            <EmptyState message="No pending decisions." />
          )}
        </Section>

        {recentCaptures && recentCaptures.length > 0 && (
          <Section title="Recent Captures" count={recentCaptures.length}>
            {recentCaptures.map((capture) => (
              <CaptureItem
                key={capture.id}
                capture={capture}
                onClick={() => navigate(`/note/${encodeURIComponent(capture.id)}`)}
              />
            ))}
          </Section>
        )}
      </div>
    </div>
  );
};

export default TodayDashboard;
