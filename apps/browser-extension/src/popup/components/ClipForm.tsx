import { useState, useEffect, useCallback } from 'react';
import { getProjects } from '@shared/api';
import { sendOrQueueCapture, retryQueuedCaptures, getPendingCount } from '@shared/capture-queue';
import type { CapturePayload, CaptureResult } from '@shared/types';

interface ClipFormProps {
  initialTitle: string;
  initialUrl: string;
  initialSelectedText: string;
  onSend?: () => void;
}

export function ClipForm({ initialTitle, initialUrl, initialSelectedText, onSend }: ClipFormProps) {
  const [title, setTitle] = useState(initialTitle);
  const [url] = useState(initialUrl);
  const [selectedText] = useState(initialSelectedText);
  const [project, setProject] = useState('');
  const [projects, setProjects] = useState<string[]>([]);
  const [tagsInput, setTagsInput] = useState('');
  const [status, setStatus] = useState<CaptureResult['state']>('unsynced');
  const [result, setResult] = useState<CaptureResult | null>(null);
  const [pendingCount, setPendingCount] = useState(0);

  useEffect(() => { setTitle(initialTitle); }, [initialTitle]);
  useEffect(() => { getProjects().then(setProjects).catch(() => setProjects([])); }, []);
  useEffect(() => {
    getPendingCount().then(setPendingCount);
  }, [status]);

  const handleSend = useCallback(async () => {
    setStatus('syncing'); setResult(null);
    const payload: CapturePayload = {
      type: selectedText ? 'selection' : 'webpage',
      title: title || 'Untitled', url,
      text: selectedText || undefined,
      selectedText: selectedText || undefined,
      project: project || undefined,
      tags: tagsInput.split(',').map(t => t.trim()).filter(Boolean),
      capturedAt: new Date().toISOString(),
    };
    const res = await sendOrQueueCapture(payload);
    setResult(res);
    setStatus(res.state);
    if (res.state === 'synced') {
      onSend?.();
      setTimeout(() => setStatus('unsynced'), 3000);
    }
  }, [title, url, selectedText, project, tagsInput, onSend]);

  const handleRetry = useCallback(async () => {
    setStatus('syncing');
    await retryQueuedCaptures();
    const remaining = await getPendingCount();
    setPendingCount(remaining);
    if (remaining === 0) {
      setStatus('synced');
      setResult({ state: 'synced' });
      setTimeout(() => setStatus('unsynced'), 3000);
    } else {
      setStatus('failed');
      setResult({ state: 'failed', error: `${remaining} capture${remaining === 1 ? '' : 's'} still pending` });
    }
  }, []);

  const hasSelection = !!selectedText;

  return (
    <div className="form">
      <div className="form-group">
        <label className="form-label">Title</label>
        <input
          type="text"
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          placeholder="Page title"
          className="input"
        />
      </div>

      <div className="form-group">
        <label className="form-label">URL</label>
        <div className="readonly-field">{url}</div>
      </div>

      {hasSelection && (
        <div className="form-group">
          <label className="form-label">Selection</label>
          <div className="readonly-field readonly-field--multiline">{selectedText}</div>
        </div>
      )}

      <div className="form-group">
        <label className="form-label">Project</label>
        <select value={project} onChange={(e) => setProject(e.target.value)} className="select">
          <option value="">(none)</option>
          {projects.map(p => <option key={p} value={p}>{p}</option>)}
        </select>
      </div>

      <div className="form-group">
        <label className="form-label">Tags</label>
        <input
          type="text"
          value={tagsInput}
          onChange={(e) => setTagsInput(e.target.value)}
          placeholder="tag1, tag2, tag3"
          className="input"
        />
      </div>

      <button
        onClick={handleSend}
        disabled={status === 'syncing'}
        aria-label="Send capture to vault"
        aria-busy={status === 'syncing'}
        className="btn btn-primary btn--mt-1"
      >
        {status === 'syncing' ? (
          <>
            <span className="spinner" aria-hidden="true" />
            Sending...
          </>
        ) : (
          'Send to Vault'
        )}
      </button>

      {status === 'synced' && (
        <div className="banner banner-success">
          Sent to AgentVault!
          {result?.path && <div className="banner__detail">{result.path}</div>}
        </div>
      )}

      {(status === 'unsynced' || status === 'failed') && result?.error && (
        <div className="banner banner-error">
          {result.error}
          {result.queued && <span> Saved offline.</span>}
        </div>
      )}

      {pendingCount > 0 && (
        <div className="banner banner-warning banner--row">
          <span>{pendingCount} pending capture{pendingCount === 1 ? '' : 's'}</span>
          <button onClick={handleRetry} disabled={status === 'syncing'} aria-label="Retry pending captures" className="btn btn-sm btn-secondary">
            Retry
          </button>
        </div>
      )}
    </div>
  );
}
