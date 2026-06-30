import { useState, useEffect, useCallback } from 'react';
import { getProjects } from '@shared/api';
import { sendOrQueueCapture, retryQueuedCaptures, getPendingCount } from '@shared/capture-queue';
import type { CapturePayload, CaptureResult } from '@shared/types';

interface ClipFormProps {
  initialTitle: string;
  initialUrl: string;
  initialSelectedText: string;
  initialText?: string;
  onSend?: () => void;
}

function Spinner() {
  return (
    <svg width="14" height="14" viewBox="0 0 50 50" aria-hidden="true">
      <g>
        <animateTransform
          attributeName="transform"
          type="rotate"
          from="0 25 25"
          to="360 25 25"
          dur="1s"
          repeatCount="indefinite"
        />
        <circle
          cx="25"
          cy="25"
          r="20"
          fill="none"
          stroke="currentColor"
          strokeWidth="5"
          strokeLinecap="round"
          strokeDasharray="80 20"
        />
      </g>
    </svg>
  );
}

export function ClipForm({ initialTitle, initialUrl, initialSelectedText, initialText, onSend }: ClipFormProps) {
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

  const clearSynced = useCallback(() => {
    setStatus('unsynced');
    setResult(null);
  }, []);

  const handleSend = useCallback(async () => {
    setStatus('syncing');
    setResult(null);
    const payload: CapturePayload = {
      type: selectedText ? 'selection' : 'webpage',
      title: title || 'Untitled',
      url,
      text: selectedText || initialText || undefined,
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
      setTimeout(clearSynced, 3000);
    }
  }, [title, url, selectedText, initialText, project, tagsInput, onSend, clearSynced]);

  const handleRetry = useCallback(async () => {
    setStatus('syncing');
    setResult(null);
    await retryQueuedCaptures();
    const remaining = await getPendingCount();
    setPendingCount(remaining);
    if (remaining === 0) {
      setStatus('synced');
      setResult({ state: 'synced' });
      setTimeout(clearSynced, 3000);
    } else {
      setStatus('failed');
      setResult({ state: 'failed', error: `${remaining} capture${remaining === 1 ? '' : 's'} still pending` });
    }
  }, [clearSynced]);

  const hasSelection = !!selectedText;
  const labelStyle: React.CSSProperties = { display: 'block', fontSize: '11px', fontWeight: 600, color: '#6b7280', marginBottom: '4px', textTransform: 'uppercase', letterSpacing: '0.5px' };
  const inputStyle: React.CSSProperties = { width: '100%', padding: '8px 10px', background: '#1a1d27', color: '#e4e6eb', border: '1px solid #2a2d3a', borderRadius: '6px', outline: 'none' };

  return (
    <div style={{ padding: '14px', display: 'flex', flexDirection: 'column', gap: '12px' }}>
      <div>
        <label style={labelStyle}>Title</label>
        <input type="text" value={title} onChange={(e) => setTitle(e.target.value)} placeholder="Page title" style={inputStyle} />
      </div>
      <div>
        <label style={labelStyle}>URL</label>
        <div style={{ ...inputStyle, background: '#14161f', color: '#6b7280', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{url}</div>
      </div>
      {hasSelection && (
        <div>
          <label style={labelStyle}>Selection</label>
          <div style={{ ...inputStyle, background: '#14161f', maxHeight: '100px', overflowY: 'auto', fontSize: '12px', lineHeight: '1.5', color: '#e4e6eb', whiteSpace: 'pre-wrap', wordBreak: 'break-word' }}>{selectedText}</div>
        </div>
      )}
      <div>
        <label style={labelStyle}>Project</label>
        <select value={project} onChange={(e) => setProject(e.target.value)} style={{ ...inputStyle, cursor: 'pointer' }}>
          <option value="">(none)</option>
          {projects.map(p => <option key={p} value={p}>{p}</option>)}
        </select>
      </div>
      <div>
        <label style={labelStyle}>Tags</label>
        <input type="text" value={tagsInput} onChange={(e) => setTagsInput(e.target.value)} placeholder="tag1, tag2, tag3" style={inputStyle} />
      </div>
      <button onClick={handleSend} disabled={status === 'syncing'}
        style={{ marginTop: '4px', padding: '10px 16px', background: '#4f7cff', color: '#fff', border: 'none', borderRadius: '6px', fontSize: '14px', fontWeight: 600, cursor: status === 'syncing' ? 'wait' : 'pointer', opacity: status === 'syncing' ? 0.7 : 1 }}>
        {status === 'syncing' ? 'Sending...' : 'Send to Vault'}
      </button>

      {status === 'syncing' && (
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px', padding: '8px 12px', background: 'rgba(79,124,255,0.1)', border: '1px solid rgba(79,124,255,0.3)', borderRadius: '6px', color: '#4f7cff', fontSize: '13px' }}>
          <Spinner />
          <span>Syncing...</span>
        </div>
      )}

      {status === 'synced' && (
        <div style={{ padding: '8px 12px', background: 'rgba(34,197,94,0.1)', border: '1px solid rgba(34,197,94,0.3)', borderRadius: '6px', color: '#22c55e', fontSize: '13px', textAlign: 'center' }}>
          Saved to AgentVault
          {result?.path && <div style={{ fontSize: '11px', marginTop: '4px', opacity: 0.9 }}>{result.path}</div>}
        </div>
      )}

      {status === 'unsynced' && result?.queued && (
        <div style={{ padding: '8px 12px', background: 'rgba(245,158,11,0.1)', border: '1px solid rgba(245,158,11,0.3)', borderRadius: '6px', color: '#f59e0b', fontSize: '12px' }}>
          Saved offline. Will retry when AgentVault is running.
          {result.error && <div style={{ marginTop: '4px', opacity: 0.9 }}>{result.error}</div>}
        </div>
      )}

      {status === 'failed' && (
        <div style={{ padding: '8px 12px', background: 'rgba(239,68,68,0.1)', border: '1px solid rgba(239,68,68,0.3)', borderRadius: '6px', color: '#ef4444', fontSize: '12px' }}>
          <div style={{ fontWeight: 600, marginBottom: '4px' }}>Failed</div>
          {result?.error && <div style={{ marginBottom: '6px' }}>{result.error}</div>}
          <button onClick={handleRetry} style={{ padding: '4px 10px', background: 'transparent', border: '1px solid #ef4444', borderRadius: '4px', color: '#ef4444', fontSize: '11px', cursor: 'pointer' }}>Retry</button>
        </div>
      )}

      {pendingCount > 0 && (
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '8px 12px', background: 'rgba(245,158,11,0.1)', border: '1px solid rgba(245,158,11,0.3)', borderRadius: '6px', color: '#f59e0b', fontSize: '12px' }}>
          <span>{pendingCount} pending capture{pendingCount === 1 ? '' : 's'}</span>
          <button onClick={handleRetry} disabled={status === 'syncing'} style={{ padding: '4px 10px', background: 'transparent', border: '1px solid #f59e0b', borderRadius: '4px', color: '#f59e0b', fontSize: '11px', cursor: 'pointer' }}>Retry</button>
        </div>
      )}
    </div>
  );
}
