import React, { useEffect, useMemo, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { api } from '@/api/client';
import type { NoteLink, SearchResult } from '@agentvault/contract';

interface GraphNode {
  id: string;
  title: string;
  x: number;
  y: number;
  r: number;
}

interface GraphEdge {
  source: string;
  target: string;
}

const GraphView: React.FC = () => {
  const navigate = useNavigate();
  const svgRef = useRef<SVGSVGElement | null>(null);
  const [notes, setNotes] = useState<SearchResult[]>([]);
  const [notesLoading, setNotesLoading] = useState(true);
  const [notesError, setNotesError] = useState<string | null>(null);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [links, setLinks] = useState<{ backlinks: NoteLink[]; outgoing: NoteLink[] } | null>(null);
  const [linksLoading, setLinksLoading] = useState(false);
  const [linksError, setLinksError] = useState<string | null>(null);
  const [dimensions, setDimensions] = useState({ width: 800, height: 500 });

  // Fetch all notes once.
  useEffect(() => {
    let cancelled = false;
    api.search({ limit: 10000 })
      .then((res) => {
        if (!cancelled) setNotes(res);
      })
      .catch((err) => {
        if (!cancelled) setNotesError(err instanceof Error ? err.message : 'Failed to load notes');
      })
      .finally(() => {
        if (!cancelled) setNotesLoading(false);
      });
    return () => { cancelled = true; };
  }, []);

  // Default selection to the first note when notes load.
  useEffect(() => {
    if (!selectedId && notes.length > 0) {
      setSelectedId(notes[0].id);
    }
  }, [notes, selectedId]);

  // Fetch links for the selected note.
  useEffect(() => {
    if (!selectedId) return;
    let cancelled = false;
    setLinksLoading(true);
    setLinksError(null);
    api.getNoteLinks(selectedId)
      .then((res) => {
        if (!cancelled) setLinks(res);
      })
      .catch((err) => {
        if (!cancelled) setLinksError(err instanceof Error ? err.message : 'Failed to load links');
      })
      .finally(() => {
        if (!cancelled) setLinksLoading(false);
      });
    return () => { cancelled = true; };
  }, [selectedId]);

  // Track SVG container size.
  useEffect(() => {
    const updateSize = () => {
      const el = svgRef.current?.parentElement;
      if (el) {
        const rect = el.getBoundingClientRect();
        setDimensions({ width: Math.max(320, Math.floor(rect.width)), height: Math.max(320, Math.floor(rect.height)) });
      }
    };
    updateSize();
    window.addEventListener('resize', updateSize);
    return () => window.removeEventListener('resize', updateSize);
  }, []);

  const { nodes, edges } = useMemo(() => {
    const { width, height } = dimensions;
    const centerX = width / 2;
    const centerY = height / 2;
    const noteSet = new Map<string, SearchResult>();
    for (const note of notes) noteSet.set(note.id, note);

    const selectedNote = selectedId ? noteSet.get(selectedId) : undefined;
    if (!selectedNote || notes.length === 0) {
      return { nodes: [] as GraphNode[], edges: [] as GraphEdge[] };
    }

    const neighborIds = new Set<string>();
    const edgeList: GraphEdge[] = [];
    if (links) {
      for (const link of links.backlinks) {
        neighborIds.add(link.id);
        edgeList.push({ source: link.id, target: selectedNote.id });
      }
      for (const link of links.outgoing) {
        neighborIds.add(link.id);
        edgeList.push({ source: selectedNote.id, target: link.id });
      }
    }

    // Always include all notes as faint background nodes.
    const nodeMap = new Map<string, GraphNode>();
    const r = Math.min(width, height) * 0.35;
    const otherNotes = notes.filter((n) => n.id !== selectedNote.id);
    otherNotes.forEach((note, index) => {
      const angle = (index / Math.max(1, otherNotes.length)) * Math.PI * 2;
      nodeMap.set(note.id, {
        id: note.id,
        title: note.title,
        x: centerX + r * Math.cos(angle),
        y: centerY + r * Math.sin(angle),
        r: neighborIds.has(note.id) ? 7 : 4,
      });
    });

    // Place selected note at center on top.
    nodeMap.set(selectedNote.id, {
      id: selectedNote.id,
      title: selectedNote.title,
      x: centerX,
      y: centerY,
      r: 11,
    });

    return { nodes: Array.from(nodeMap.values()), edges: edgeList };
  }, [dimensions, notes, selectedId, links]);

  const handleNodeClick = (id: string) => {
    if (id === selectedId) {
      navigate(`/note/${encodeURIComponent(id)}`);
    } else {
      setSelectedId(id);
    }
  };

  if (notesLoading) {
    return (
      <div className="h-full flex items-center justify-center">
        <div className="flex items-center gap-3 text-vault-text-muted">
          <div className="w-5 h-5 border-2 border-vault-accent border-t-transparent rounded-full animate-spin" />
          Loading graph…
        </div>
      </div>
    );
  }

  if (notesError) {
    return (
      <div className="h-full flex items-center justify-center">
        <div className="text-center">
          <svg className="w-10 h-10 text-vault-error mx-auto mb-3" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={1}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m9-.75a9 9 0 1 1-18 0 9 9 0 0 1 18 0Zm-9 3.75h.008v.008H12v-.008Z" />
          </svg>
          <p className="text-sm text-vault-error mb-2">{notesError}</p>
        </div>
      </div>
    );
  }

  if (notes.length === 0) {
    return (
      <div className="h-full flex items-center justify-center">
        <p className="text-vault-text-muted">No notes available to graph.</p>
      </div>
    );
  }

  const selectedNote = notes.find((n) => n.id === selectedId);

  return (
    <div className="h-full flex flex-col animate-fade-in">
      <div className="border-b border-vault-border px-6 py-4 flex flex-col sm:flex-row sm:items-center justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold text-vault-text-primary">Graph</h1>
          <p className="text-sm text-vault-text-secondary">
            {selectedNote ? `Centered on “${selectedNote.title}”` : 'Select a note to explore links'}
          </p>
        </div>
        <div className="flex items-center gap-2">
          <label htmlFor="graph-note-select" className="text-sm text-vault-text-secondary">Focus:</label>
          <select
            id="graph-note-select"
            value={selectedId ?? ''}
            onChange={(e) => setSelectedId(e.target.value)}
            className="bg-vault-bg-tertiary border border-vault-border text-sm text-vault-text-primary rounded-lg px-3 py-1.5 focus:outline-none focus:border-vault-accent"
          >
            {notes.map((note) => (
              <option key={note.id} value={note.id}>{note.title}</option>
            ))}
          </select>
        </div>
      </div>

      <div className="flex-1 relative min-h-0 bg-vault-bg-primary">
        {linksLoading && (
          <div className="absolute top-3 right-3 z-10 flex items-center gap-2 text-xs text-vault-text-muted bg-vault-bg-secondary/80 px-2 py-1 rounded-md">
            <div className="w-3 h-3 border-2 border-vault-accent border-t-transparent rounded-full animate-spin" />
            Loading links…
          </div>
        )}
        {linksError && (
          <div className="absolute top-3 right-3 z-10 text-xs text-vault-error bg-vault-bg-secondary/80 px-2 py-1 rounded-md">
            {linksError}
          </div>
        )}
        <svg
          ref={svgRef}
          width={dimensions.width}
          height={dimensions.height}
          className="absolute inset-0 w-full h-full"
        >
          {edges.map((edge, idx) => {
            const source = nodes.find((n) => n.id === edge.source);
            const target = nodes.find((n) => n.id === edge.target);
            if (!source || !target) return null;
            return (
              <line
                key={`${edge.source}-${edge.target}-${idx}`}
                x1={source.x}
                y1={source.y}
                x2={target.x}
                y2={target.y}
                stroke="var(--border-color)"
                strokeWidth={1.5}
              />
            );
          })}
          {nodes.map((node) => {
            const isSelected = node.id === selectedId;
            const isNeighbor = edges.some((e) => (e.source === selectedId && e.target === node.id) || (e.target === selectedId && e.source === node.id));
            return (
              <g
                key={node.id}
                transform={`translate(${node.x}, ${node.y})`}
                className="cursor-pointer"
                onClick={() => handleNodeClick(node.id)}
              >
                <circle
                  r={node.r}
                  fill={isSelected ? 'var(--accent)' : isNeighbor ? 'var(--text-secondary)' : 'var(--border-color)'}
                  stroke={isSelected ? 'var(--accent-hover)' : 'var(--bg-secondary)'}
                  strokeWidth={2}
                />
                <text
                  y={node.r + 14}
                  textAnchor="middle"
                  className="fill-vault-text-secondary text-[11px] select-none"
                  style={{ pointerEvents: 'none' }}
                >
                  {node.title.length > 22 ? `${node.title.slice(0, 22)}…` : node.title}
                </text>
                <title>{node.title}{isSelected ? ' (click to open)' : ' (click to focus)'}</title>
              </g>
            );
          })}
        </svg>
      </div>

      <div className="border-t border-vault-border px-6 py-3 text-xs text-vault-text-muted">
        Click a node to focus; click the center node to open the note. A global graph endpoint will make this view richer in a future release.
      </div>
    </div>
  );
};

export default GraphView;
