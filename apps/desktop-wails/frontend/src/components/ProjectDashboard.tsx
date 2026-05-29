import { useState, useEffect, useCallback } from 'react';
import { Folder, FileText, ChevronRight, LayoutDashboard } from './Icons';
import type { SearchResult } from '../types';

interface Props {
  onOpenNote: (path: string) => void;
}

interface ProjectGroup {
  name: string;
  notes: SearchResult[];
  noteCount: number;
  decisionCount: number;
  taskCount: number;
}

export default function ProjectDashboard({ onOpenNote }: Props) {
  const [projects, setProjects] = useState<ProjectGroup[]>([]);
  const [expandedProject, setExpandedProject] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  const loadProjects = useCallback(async () => {
    try {
      const projectNames = await window.go.main.NoteService.GetProjects();
      const groups: ProjectGroup[] = [];

      for (const name of projectNames) {
        const notes = await window.go.main.NoteService.GetNotesByProject(name);
        groups.push({
          name,
          notes,
          noteCount: notes.length,
          decisionCount: notes.filter(n => n.type === 'decision').length,
          taskCount: notes.filter(n => n.type === 'task').length,
        });
      }

      setProjects(groups);
    } catch (err) {
      console.error('Failed to load projects:', err);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    loadProjects();
  }, [loadProjects]);

  if (loading) {
    return (
      <div className="flex items-center justify-center h-full">
        <div className="text-sm text-[var(--text-muted)]">Loading projects...</div>
      </div>
    );
  }

  if (projects.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center h-full text-[var(--text-muted)]">
        <LayoutDashboard className="w-12 h-12 mb-4 opacity-30" />
        <p className="text-sm">No projects yet</p>
        <p className="text-xs mt-1">Create notes with a project field to see them here</p>
      </div>
    );
  }

  return (
    <div className="flex flex-col h-full bg-[var(--bg-primary)]">
      {/* Header */}
      <div className="px-6 py-4 border-b border-[var(--border)] bg-[var(--bg-secondary)]">
        <h1 className="text-lg font-semibold text-[var(--text-primary)]">Projects</h1>
        <p className="text-xs text-[var(--text-muted)] mt-0.5">
          {projects.length} projects · {projects.reduce((a, p) => a + p.noteCount, 0)} notes
        </p>
      </div>

      {/* Project List */}
      <div className="flex-1 overflow-auto p-4">
        <div className="grid grid-cols-1 gap-3 max-w-3xl">
          {projects.map(project => (
            <div
              key={project.name}
              className="bg-[var(--bg-secondary)] rounded-lg border border-[var(--border)] overflow-hidden"
            >
              <button
                onClick={() => setExpandedProject(
                  expandedProject === project.name ? null : project.name
                )}
                className="w-full flex items-center justify-between px-4 py-3 hover:bg-[var(--bg-hover)] transition-colors"
              >
                <div className="flex items-center gap-3">
                  <Folder className="w-5 h-5 text-[var(--accent)]" />
                  <span className="text-sm font-medium text-[var(--text-primary)]">
                    {project.name}
                  </span>
                </div>
                <div className="flex items-center gap-3">
                  <div className="flex gap-2 text-xs text-[var(--text-muted)]">
                    <span>{project.noteCount} notes</span>
                    {project.decisionCount > 0 && (
                      <span>{project.decisionCount} decisions</span>
                    )}
                    {project.taskCount > 0 && (
                      <span>{project.taskCount} tasks</span>
                    )}
                  </div>
                  <ChevronRight className={`w-4 h-4 text-[var(--text-muted)] transition-transform ${
                    expandedProject === project.name ? 'rotate-90' : ''
                  }`} />
                </div>
              </button>

              {expandedProject === project.name && (
                <div className="border-t border-[var(--border)] px-4 py-2">
                  {project.notes.map(note => (
                    <button
                      key={note.id}
                      onClick={() => onOpenNote(note.path)}
                      className="w-full flex items-start gap-2.5 px-2 py-2 text-left hover:bg-[var(--bg-hover)] rounded transition-colors"
                    >
                      <FileText className="w-4 h-4 mt-0.5 text-[var(--text-muted)] flex-shrink-0" />
                      <div className="min-w-0">
                        <div className="text-sm text-[var(--text-primary)] truncate">
                          {note.title}
                        </div>
                        <div className="flex items-center gap-2 mt-0.5">
                          <span className="text-[10px] px-1.5 py-0.5 rounded bg-[var(--bg-tertiary)] text-[var(--text-muted)]">
                            {note.type}
                          </span>
                          <span className="text-[10px] text-[var(--text-muted)]">
                            {note.path}
                          </span>
                        </div>
                      </div>
                    </button>
                  ))}
                </div>
              )}
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
