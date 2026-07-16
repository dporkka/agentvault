import React, { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { api } from '@/api/client';
import { useApi } from '@/hooks/useApi';
import EmptyState from '@/components/EmptyState';
import type { SearchResult } from '@agentvault/contract';
import { typeBadgeClass } from '@/utils/styles';

interface ProjectWithCount {
  name: string;
  noteCount: number | null; // null = loading
}

const ProjectDashboard: React.FC = () => {
  const { data: projectNames, loading: projectsLoading, error: projectsError } = useApi(
    () => api.getProjects(),
    [],
    true,
  );

  const [projects, setProjects] = useState<ProjectWithCount[]>([]);
  const [selectedProject, setSelectedProject] = useState<string | null>(null);
  const [notes, setNotes] = useState<SearchResult[]>([]);
  const [notesLoading, setNotesLoading] = useState(false);
  const [notesError, setNotesError] = useState<string | null>(null);
  const navigate = useNavigate();

  // When project names load, fetch counts
  useEffect(() => {
    if (!projectNames || projectNames.length === 0) {
      setProjects([]);
      return;
    }

    // Initialize with null counts
    setProjects(projectNames.map((name) => ({ name, noteCount: null })));

    let cancelled = false;

    async function loadCounts() {
      const counts = await Promise.all(
        projectNames!.map(async (name) => {
          try {
            const results = await api.search({ project: name, limit: 100 });
            return { name, count: results.length };
          } catch {
            return { name, count: 0 };
          }
        }),
      );

      if (cancelled) return;

      setProjects(
        projectNames!.map((name) => ({
          name,
          noteCount: counts.find((c) => c.name === name)?.count ?? 0,
        })),
      );
    }

    loadCounts();
    return () => { cancelled = true; };
  }, [projectNames]);

  // Load notes when project is selected
  useEffect(() => {
    if (!selectedProject) {
      setNotes([]);
      return;
    }

    let cancelled = false;
    setNotesLoading(true);
    setNotesError(null);

    async function loadNotes() {
      try {
        const results = await api.search({ q: '', project: selectedProject!, limit: 100 });
        if (!cancelled) {
          setNotes(results);
        }
      } catch (err) {
        if (!cancelled) {
          setNotesError(err instanceof Error ? err.message : 'Failed to load notes');
        }
      } finally {
        if (!cancelled) setNotesLoading(false);
      }
    }

    loadNotes();
    return () => { cancelled = true; };
  }, [selectedProject]);

  return (
    <div className="h-full flex flex-col">
      {/* Header */}
      <div className="border-b border-vault-border px-6 py-4">
        <h1 className="text-lg font-semibold text-vault-text-primary">Projects</h1>
        <p className="text-xs text-vault-text-muted mt-1">
          {projects?.length ?? 0} project{projects?.length !== 1 ? 's' : ''} found
        </p>
      </div>

      {/* Content */}
      <div className="flex-1 overflow-y-auto px-6 py-4">
        {projectsLoading && (
          <div className="flex items-center justify-center py-12">
            <div className="w-6 h-6 border-2 border-vault-accent border-t-transparent rounded-full animate-spin" />
          </div>
        )}

        {projectsError && (
          <div className="flex items-center gap-2 text-sm text-vault-error bg-vault-error/10 rounded-lg p-4">
            <svg className="w-4 h-4 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m9-.75a9 9 0 1 1-18 0 9 9 0 0 1 18 0Zm-9 3.75h.008v.008H12v-.008Z" />
            </svg>
            {projectsError}
          </div>
        )}

        {!projectsLoading && !projectsError && projects && projects.length === 0 && (
          <EmptyState
            className="py-12"
            icon={
              <svg className="w-12 h-12" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={1}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M2.25 12.75V12A2.25 2.25 0 0 1 4.5 9.75h15A2.25 2.25 0 0 1 21.75 12v.75m-8.69-6.44-2.12-2.12a1.5 1.5 0 0 0-1.061-.44H4.5A2.25 2.25 0 0 0 2.25 6v12a2.25 2.25 0 0 0 2.25 2.25h15A2.25 2.25 0 0 0 21.75 18V9a2.25 2.25 0 0 0-2.25-2.25h-5.379a1.5 1.5 0 0 1-1.06-.44Z" />
              </svg>
            }
            title="No projects found"
          />
        )}

        {!projectsLoading && !projectsError && projects && projects.length > 0 && (
          <div className="grid grid-cols-1 lg:grid-cols-2 xl:grid-cols-3 gap-3">
            {projects.map((project) => (
              <button
                key={project.name}
                onClick={() => setSelectedProject(selectedProject === project.name ? null : project.name)}
                className={`text-left p-4 rounded-lg border transition-all ${
                  selectedProject === project.name
                    ? 'border-vault-accent bg-vault-accent-muted ring-1 ring-vault-accent/30'
                    : 'border-vault-border bg-vault-bg-secondary hover:bg-vault-bg-hover hover:border-vault-border/80'
                }`}
              >
                <div className="flex items-center gap-3">
                  <div className={`w-10 h-10 rounded-lg flex items-center justify-center flex-shrink-0 ${
                    selectedProject === project.name ? 'bg-vault-accent/20' : 'bg-vault-bg-tertiary'
                  }`}>
                    <svg className={`w-5 h-5 ${selectedProject === project.name ? 'text-vault-accent' : 'text-vault-text-muted'}`} fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={1.5}>
                      <path strokeLinecap="round" strokeLinejoin="round" d="M2.25 12.75V12A2.25 2.25 0 0 1 4.5 9.75h15A2.25 2.25 0 0 1 21.75 12v.75m-8.69-6.44-2.12-2.12a1.5 1.5 0 0 0-1.061-.44H4.5A2.25 2.25 0 0 0 2.25 6v12a2.25 2.25 0 0 0 2.25 2.25h15A2.25 2.25 0 0 0 21.75 18V9a2.25 2.25 0 0 0-2.25-2.25h-5.379a1.5 1.5 0 0 1-1.06-.44Z" />
                    </svg>
                  </div>
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center justify-between">
                      <h3 className={`text-sm font-medium truncate ${selectedProject === project.name ? 'text-vault-accent' : 'text-vault-text-primary'}`}>
                        {project.name}
                      </h3>
                      <span className="text-xs text-vault-text-muted ml-2 flex-shrink-0">
                        {project.noteCount !== null ? `${project.noteCount} note${project.noteCount !== 1 ? 's' : ''}` : '...'}
                      </span>
                    </div>
                    <button
                      onClick={(e) => {
                        e.stopPropagation();
                        navigate(`/search?project=${encodeURIComponent(project.name)}`);
                      }}
                      className="text-xs text-vault-accent hover:text-vault-accent-hover mt-1 transition-colors"
                    >
                      Browse
                    </button>
                  </div>
                </div>
              </button>
            ))}
          </div>
        )}

        {/* Project Notes */}
        {selectedProject && (
          <div className="mt-6 animate-fade-in">
            <div className="flex items-center justify-between mb-3">
              <h2 className="text-sm font-semibold text-vault-text-primary">
                Notes in <span className="text-vault-accent">{selectedProject}</span>
              </h2>
              <button
                onClick={() => setSelectedProject(null)}
                className="text-xs text-vault-text-muted hover:text-vault-text-primary transition-colors"
              >
                Close
              </button>
            </div>

            {notesLoading && (
              <div className="flex items-center justify-center py-6">
                <div className="w-5 h-5 border-2 border-vault-accent border-t-transparent rounded-full animate-spin" />
              </div>
            )}

            {notesError && (
              <div className="text-sm text-vault-error bg-vault-error/10 rounded-lg p-3">{notesError}</div>
            )}

            {!notesLoading && !notesError && notes.length === 0 && (
              <EmptyState className="py-4" title="No notes in this project" />
            )}

            {!notesLoading && !notesError && notes.length > 0 && (
              <div className="space-y-1">
                {notes.map((note) => (
                  <button
                    key={note.id}
                    onClick={() => navigate(`/note/${encodeURIComponent(note.id)}`)}
                    className="w-full text-left p-3 rounded-lg hover:bg-vault-bg-hover transition-colors group"
                  >
                    <div className="flex items-center gap-2">
                      <span className={`type-badge ${typeBadgeClass(note.type)}`}>{note.type}</span>
                      <span className="text-sm text-vault-text-primary group-hover:text-vault-accent transition-colors truncate">
                        {note.title}
                      </span>
                    </div>
                    {note.snippet && (
                      <p className="text-xs text-vault-text-secondary mt-1 line-clamp-1">{note.snippet}</p>
                    )}
                  </button>
                ))}
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
};

export default ProjectDashboard;
