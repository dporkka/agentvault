import { useState, useCallback, useRef, useEffect } from 'react';
import { ask } from '@shared/api';
import type { AskResponse, AskSource } from '@agentvault/contract';

interface Message {
  id: string;
  role: 'user' | 'assistant';
  text: string;
  response?: AskResponse;
}

function makeId(): string {
  return `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
}

function Sources({ sources }: { sources: AskSource[] }) {
  if (!sources?.length) return null;
  return (
    <div style={{ marginTop: '8px', padding: '8px 10px', background: '#14161d', border: '1px solid #2a2d3a', borderRadius: '6px' }}>
      <div style={{ fontSize: '11px', fontWeight: 600, color: '#9ca3af', marginBottom: '6px' }}>
        Sources ({sources.length})
      </div>
      <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
        {sources.map((s) => (
          <div key={s.id}>
            <div style={{ fontSize: '12px', fontWeight: 600, color: '#4f7cff', lineHeight: '1.4' }}>
              {s.title || s.path}
            </div>
            {s.excerpt && (
              <div style={{ fontSize: '11px', color: '#6b7280', lineHeight: '1.4', marginTop: '2px' }}>
                {s.excerpt}
              </div>
            )}
            <div style={{ fontSize: '10px', color: '#6b7280', marginTop: '2px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
              {s.path}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

function AnswerMeta({ response }: { response: AskResponse }) {
  return (
    <div style={{ marginTop: '8px', display: 'flex', flexDirection: 'column', gap: '6px' }}>
      {response.confidence && (
        <span style={{
          alignSelf: 'flex-start',
          fontSize: '10px',
          fontWeight: 600,
          textTransform: 'uppercase',
          padding: '2px 6px',
          background: 'rgba(79,124,255,0.15)',
          color: '#4f7cff',
          borderRadius: '4px',
        }}>
          {response.confidence} confidence
        </span>
      )}
      {response.caveats && response.caveats.length > 0 && (
        <ul style={{ margin: 0, paddingLeft: '16px', color: '#9ca3af', fontSize: '11px', lineHeight: '1.4' }}>
          {response.caveats.map((c, i) => <li key={i}>{c}</li>)}
        </ul>
      )}
      {response.missingInfo && (
        <div style={{ fontSize: '11px', color: '#9ca3af', lineHeight: '1.4' }}>
          Missing info: {response.missingInfo}
        </div>
      )}
      {response.suggestedActions && response.suggestedActions.length > 0 && (
        <div style={{ display: 'flex', flexWrap: 'wrap', gap: '6px' }}>
          {response.suggestedActions.map((a, i) => (
            <span key={i} style={{
              fontSize: '10px',
              padding: '3px 7px',
              background: '#1f2330',
              border: '1px solid #2a2d3a',
              borderRadius: '4px',
              color: '#6b7280',
            }}>
              {a}
            </span>
          ))}
        </div>
      )}
    </div>
  );
}

function LoadingBubble() {
  return (
    <div style={{ alignSelf: 'flex-start', maxWidth: '70%' }}>
      <div style={{
        padding: '10px 14px',
        background: '#1a1d27',
        border: '1px solid #2a2d3a',
        borderRadius: '12px 12px 12px 4px',
        color: '#6b7280',
        fontSize: '13px',
      }}>
        Thinking…
      </div>
    </div>
  );
}

export function AskPanel() {
  const [input, setInput] = useState('');
  const [messages, setMessages] = useState<Message[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const endRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    endRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages, loading]);

  const handleAsk = useCallback(async () => {
    const question = input.trim();
    if (!question || loading) return;

    const userMessage: Message = { id: makeId(), role: 'user', text: question };
    setMessages((prev) => [...prev, userMessage]);
    setInput('');
    setLoading(true);
    setError('');

    try {
      const response = await ask({ question });
      setMessages((prev) => [...prev, {
        id: makeId(),
        role: 'assistant',
        text: response.answer,
        response,
      }]);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Ask failed');
    } finally {
      setLoading(false);
    }
  }, [input, loading]);

  const inputStyle: React.CSSProperties = {
    flex: 1,
    padding: '8px 10px',
    background: '#1a1d27',
    color: '#e4e6eb',
    border: '1px solid #2a2d3a',
    borderRadius: '6px',
    outline: 'none',
    fontSize: '13px',
  };

  return (
    <div style={{ padding: '14px', display: 'flex', flexDirection: 'column', gap: '12px', height: '100%' }}>
      <div style={{
        flex: 1,
        overflowY: 'auto',
        display: 'flex',
        flexDirection: 'column',
        gap: '12px',
        minHeight: '180px',
      }}>
        {messages.length === 0 && !loading && (
          <div style={{ textAlign: 'center', color: '#6b7280', fontSize: '13px', padding: '20px' }}>
            Ask your vault a question. Answers are grounded in your notes.
          </div>
        )}
        {messages.map((m) => {
          const isUser = m.role === 'user';
          return (
            <div key={m.id} style={{ alignSelf: isUser ? 'flex-end' : 'flex-start', maxWidth: '85%', width: '100%' }}>
              <div style={{
                padding: '10px 12px',
                background: isUser ? '#4f7cff' : '#1a1d27',
                color: isUser ? '#fff' : '#e4e6eb',
                border: isUser ? '1px solid #4f7cff' : '1px solid #2a2d3a',
                borderRadius: isUser ? '12px 12px 4px 12px' : '12px 12px 12px 4px',
                fontSize: '13px',
                lineHeight: '1.45',
                whiteSpace: 'pre-wrap',
                wordBreak: 'break-word',
              }}>
                {m.text}
              </div>
              {m.role === 'assistant' && m.response && (
                <>
                  <Sources sources={m.response.sources} />
                  <AnswerMeta response={m.response} />
                </>
              )}
            </div>
          );
        })}
        {loading && <LoadingBubble />}
        <div ref={endRef} />
      </div>
      {error && (
        <div style={{
          padding: '8px 12px',
          background: 'rgba(239,68,68,0.1)',
          border: '1px solid rgba(239,68,68,0.3)',
          borderRadius: '6px',
          color: '#ef4444',
          fontSize: '12px',
        }}>
          {error}
        </div>
      )}
      <div style={{ display: 'flex', gap: '8px' }}>
        <input
          type="text"
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && handleAsk()}
          placeholder="Ask a question about your vault..."
          disabled={loading}
          style={inputStyle}
        />
        <button
          onClick={handleAsk}
          disabled={loading || !input.trim()}
          style={{
            padding: '8px 14px',
            background: '#4f7cff',
            color: '#fff',
            border: 'none',
            borderRadius: '6px',
            fontSize: '13px',
            fontWeight: 600,
            cursor: loading || !input.trim() ? 'not-allowed' : 'pointer',
            opacity: loading || !input.trim() ? 0.6 : 1,
          }}
        >
          {loading ? '...' : 'Ask'}
        </button>
      </div>
    </div>
  );
}
