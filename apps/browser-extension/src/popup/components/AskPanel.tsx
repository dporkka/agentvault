import { useState, useCallback, useRef, useEffect } from 'react';
import { ask } from '@shared/api';
import { renderMarkdown } from '@shared/markdown';
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
    <div className="sources-box">
      <div className="sources-box__title">
        Sources ({sources.length})
      </div>
      <div className="sources-box__list">
        {sources.map((s) => (
          <div key={s.id}>
            <div className="source-item__title">
              {s.title || s.path}
            </div>
            {s.excerpt && (
              <div className="source-item__excerpt">
                {s.excerpt}
              </div>
            )}
            <div className="source-item__path">
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
    <div className="answer-meta">
      {response.confidence && (
        <span className="badge">
          {response.confidence} confidence
        </span>
      )}
      {response.caveats && response.caveats.length > 0 && (
        <ul className="caveats-list">
          {response.caveats.map((c, i) => <li key={i}>{c}</li>)}
        </ul>
      )}
      {response.missingInfo && (
        <div className="hint--inline">
          Missing info: {response.missingInfo}
        </div>
      )}
      {response.suggestedActions && response.suggestedActions.length > 0 && (
        <div className="meta-tags">
          {response.suggestedActions.map((a, i) => (
            <span key={i} className="meta-tags__item">
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
    <div className="message-bubble message-bubble--assistant message-bubble--loading">
      Thinking…
    </div>
  );
}

const STORAGE_KEY = 'askpanel.messages';

export function AskPanel() {
  const [input, setInput] = useState('');
  const [messages, setMessages] = useState<Message[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const endRef = useRef<HTMLDivElement>(null);

  // Restore messages from session storage on mount
  useEffect(() => {
    chrome.storage.session.get(STORAGE_KEY).then((result) => {
      if (result[STORAGE_KEY]) setMessages(result[STORAGE_KEY]);
    });
  }, []);

  // Persist messages on every change
  useEffect(() => {
    chrome.storage.session.set({ [STORAGE_KEY]: messages });
  }, [messages]);

  // Listen for external storage changes
  useEffect(() => {
    const listener = (changes: Record<string, chrome.storage.StorageChange>) => {
      if (changes[STORAGE_KEY]?.newValue) setMessages(changes[STORAGE_KEY].newValue);
    };
    chrome.storage.session.onChanged.addListener(listener);
    return () => chrome.storage.session.onChanged.removeListener(listener);
  }, []);

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
  return (
    <div className="ask-panel">
      <div className="ask-panel__messages" aria-live="polite" aria-atomic="false">
        {messages.length === 0 && !loading && (
          <div className="empty-state">
            Ask your vault a question. Answers are grounded in your notes.
          </div>
        )}
        {messages.map((m) => {
          const isUser = m.role === 'user';
          return (
            <div
              key={m.id}
              className={`message-bubble ${isUser ? 'message-bubble--user' : 'message-bubble--assistant'}`}
            >
              {isUser ? <span>{m.text}</span> : <span dangerouslySetInnerHTML={{ __html: renderMarkdown(m.text) }} />}
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
        <div className="banner banner-error">
          {error}
        </div>
      )}
      <div className="ask-panel__input-row">
        <input
          type="text"
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && handleAsk()}
          placeholder="Ask a question about your vault..."
          disabled={loading}
          className="input flex-1"
        />
        <button
          onClick={handleAsk}
          disabled={loading || !input.trim()}
          aria-label="Ask question"
          aria-busy={loading}
          className="btn btn-primary"
        >
          {loading ? '...' : 'Ask'}
        </button>
      </div>
    </div>
  );
}
