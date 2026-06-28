import React, { useState, useRef, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { api } from '@/api/client';
import type { AskResponse } from '@agentvault/contract';

interface ChatMessage {
  id: string;
  role: 'user' | 'assistant';
  content: string;
  sources?: AskResponse['sources'];
  confidence?: string;
}

const EXAMPLE_QUESTIONS = [
  'What are the recent decisions made?',
  'Summarize the project status',
  'What tasks are pending?',
  'What was discussed in the last meeting?',
];

const AskPanel: React.FC = () => {
  const [question, setQuestion] = useState('');
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLTextAreaElement>(null);
  const navigate = useNavigate();

  // Auto-scroll to bottom
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages, loading]);

  // Auto-resize textarea
  useEffect(() => {
    const el = inputRef.current;
    if (el) {
      el.style.height = 'auto';
      el.style.height = `${Math.min(el.scrollHeight, 120)}px`;
    }
  }, [question]);

  const handleAsk = async (e?: React.FormEvent) => {
    e?.preventDefault();
    const q = question.trim();
    if (!q || loading) return;

    const userMsg: ChatMessage = {
      id: `u-${Date.now()}`,
      role: 'user',
      content: q,
    };

    setMessages((prev) => [...prev, userMsg]);
    setQuestion('');
    setLoading(true);
    setError(null);

    try {
      const result = await api.ask({ question: q });
      const assistantMsg: ChatMessage = {
        id: `a-${Date.now()}`,
        role: 'assistant',
        content: result.answer,
        sources: result.sources,
        confidence: result.confidence,
      };
      setMessages((prev) => [...prev, assistantMsg]);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to get answer');
      // Remove user message on error
      setMessages((prev) => prev.slice(0, -1));
    } finally {
      setLoading(false);
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleAsk();
    }
  };

  const handleExampleClick = (q: string) => {
    setQuestion(q);
    inputRef.current?.focus();
  };

  const confidenceColor = (c?: string) => {
    if (!c) return 'text-vault-text-muted';
    if (c === 'high') return 'text-vault-success';
    if (c === 'medium') return 'text-vault-warning';
    return 'text-vault-error';
  };

  return (
    <div className="h-full flex flex-col">
      {/* Header */}
      <div className="border-b border-vault-border px-6 py-4">
        <div className="flex items-center gap-2">
          <svg className="w-5 h-5 text-vault-accent" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={1.5}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M9.813 15.904 9 18.75l-.813-2.846a4.5 4.5 0 0 0-3.09-3.09L2.25 12l2.846-.813a4.5 4.5 0 0 0 3.09-3.09L9 5.25l.813 2.846a4.5 4.5 0 0 0 3.09 3.09L15.75 12l-2.846.813a4.5 4.5 0 0 0-3.09 3.09ZM18.259 8.715 18 9.75l-.259-1.035a3.375 3.375 0 0 0-2.455-2.456L14.25 6l1.036-.259a3.375 3.375 0 0 0 2.455-2.456L18 2.25l.259 1.035a3.375 3.375 0 0 0 2.455 2.456L21.75 6l-1.036.259a3.375 3.375 0 0 0-2.455 2.456Z" />
          </svg>
          <h1 className="text-lg font-semibold text-vault-text-primary">AI Ask</h1>
        </div>
        <p className="text-xs text-vault-text-muted mt-1">Ask questions about your vault notes</p>
      </div>

      {/* Messages */}
      <div className="flex-1 overflow-y-auto px-6 py-4 space-y-4">
        {messages.length === 0 && (
          <div className="flex flex-col items-center justify-center h-full text-vault-text-muted">
            <svg className="w-12 h-12 mb-3 opacity-40" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={1}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M7.5 8.25h9m-9 3H12m-9.75 1.51c0 1.6 1.123 2.994 2.707 3.227 1.129.166 2.27.293 3.423.379.35.026.67.21.865.501L12 21l2.755-4.133a1.14 1.14 0 0 1 .865-.501 48.172 48.172 0 0 0 3.423-.379c1.584-.233 2.707-1.626 2.707-3.228V6.741c0-1.602-1.123-2.995-2.707-3.228A48.394 48.394 0 0 0 12 3c-2.392 0-4.744.015-7.01.063-1.584.233-2.707 1.626-2.707 3.228v6.741Z" />
            </svg>
            <p className="text-sm mb-4">Ask me anything about your notes</p>
            <div className="flex flex-wrap gap-2 justify-center max-w-md">
              {EXAMPLE_QUESTIONS.map((q) => (
                <button
                  key={q}
                  onClick={() => handleExampleClick(q)}
                  className="px-3 py-1.5 text-xs bg-vault-bg-tertiary text-vault-text-secondary rounded-lg hover:bg-vault-bg-hover hover:text-vault-text-primary transition-colors border border-vault-border"
                >
                  {q}
                </button>
              ))}
            </div>
          </div>
        )}

        {messages.map((msg) => (
          <div key={msg.id} className={`flex gap-3 ${msg.role === 'user' ? 'justify-end' : ''}`}>
            {msg.role === 'assistant' && (
              <div className="w-7 h-7 rounded-lg bg-vault-accent/20 flex items-center justify-center flex-shrink-0 mt-0.5">
                <svg className="w-4 h-4 text-vault-accent" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={2}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M9.813 15.904 9 18.75l-.813-2.846a4.5 4.5 0 0 0-3.09-3.09L2.25 12l2.846-.813a4.5 4.5 0 0 0 3.09-3.09L9 5.25l.813 2.846a4.5 4.5 0 0 0 3.09 3.09L15.75 12l-2.846.813a4.5 4.5 0 0 0-3.09 3.09Z" />
                </svg>
              </div>
            )}

            <div className={`max-w-[80%] ${msg.role === 'user' ? 'bg-vault-accent text-white' : 'bg-vault-bg-tertiary border border-vault-border'} rounded-lg px-4 py-3`}>
              <p className="text-sm whitespace-pre-wrap leading-relaxed">{msg.content}</p>

              {msg.role === 'assistant' && msg.sources && msg.sources.length > 0 && (
                <div className="mt-3 pt-3 border-t border-vault-border">
                  <p className="text-xs font-medium text-vault-text-muted mb-1.5">Sources</p>
                  <div className="space-y-1.5">
                    {msg.sources.map((source, i) => (
                      <button
                        key={i}
                        onClick={() => navigate(`/note/${encodeURIComponent(source.id)}`)}
                        className="block w-full text-left text-xs text-vault-accent hover:underline truncate"
                      >
                        {source.title || source.path}
                      </button>
                    ))}
                  </div>
                  {msg.confidence && (
                    <p className={`text-xs mt-2 ${confidenceColor(msg.confidence)}`}>
                      Confidence: {msg.confidence}
                    </p>
                  )}
                </div>
              )}
            </div>

            {msg.role === 'user' && (
              <div className="w-7 h-7 rounded-lg bg-vault-bg-tertiary border border-vault-border flex items-center justify-center flex-shrink-0 mt-0.5">
                <svg className="w-4 h-4 text-vault-text-secondary" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={2}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M15.75 6a3.75 3.75 0 1 1-7.5 0 3.75 3.75 0 0 1 7.5 0ZM4.501 20.118a7.5 7.5 0 0 1 14.998 0A17.933 17.933 0 0 1 12 21.75c-2.676 0-5.216-.584-7.499-1.632Z" />
                </svg>
              </div>
            )}
          </div>
        ))}

        {loading && (
          <div className="flex gap-3">
            <div className="w-7 h-7 rounded-lg bg-vault-accent/20 flex items-center justify-center flex-shrink-0">
              <svg className="w-4 h-4 text-vault-accent animate-pulse" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M9.813 15.904 9 18.75l-.813-2.846a4.5 4.5 0 0 0-3.09-3.09L2.25 12l2.846-.813a4.5 4.5 0 0 0 3.09-3.09L9 5.25l.813 2.846a4.5 4.5 0 0 0 3.09 3.09L15.75 12l-2.846.813a4.5 4.5 0 0 0-3.09 3.09Z" />
              </svg>
            </div>
            <div className="bg-vault-bg-tertiary border border-vault-border rounded-lg px-4 py-3">
              <div className="flex items-center gap-2">
                <div className="w-2 h-2 bg-vault-text-muted rounded-full animate-bounce" style={{ animationDelay: '0ms' }} />
                <div className="w-2 h-2 bg-vault-text-muted rounded-full animate-bounce" style={{ animationDelay: '150ms' }} />
                <div className="w-2 h-2 bg-vault-text-muted rounded-full animate-bounce" style={{ animationDelay: '300ms' }} />
              </div>
            </div>
          </div>
        )}

        {error && (
          <div className="flex items-center gap-2 text-sm text-vault-error bg-vault-error/10 rounded-lg px-4 py-3">
            <svg className="w-4 h-4 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m9-.75a9 9 0 1 1-18 0 9 9 0 0 1 18 0Zm-9 3.75h.008v.008H12v-.008Z" />
            </svg>
            {error}
          </div>
        )}

        <div ref={messagesEndRef} />
      </div>

      {/* Input */}
      <div className="border-t border-vault-border px-6 py-4">
        <form onSubmit={handleAsk} className="relative">
          <textarea
            ref={inputRef}
            value={question}
            onChange={(e) => setQuestion(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="Ask a question..."
            rows={1}
            className="w-full bg-vault-bg-tertiary border border-vault-border rounded-lg pl-4 pr-12 py-3 text-sm text-vault-text-primary placeholder-vault-text-muted focus:border-vault-accent focus:ring-1 focus:ring-vault-accent transition-colors outline-none resize-none"
          />
          <button
            type="submit"
            disabled={loading || !question.trim()}
            className="absolute right-2 bottom-2 p-1.5 bg-vault-accent hover:bg-vault-accent-hover disabled:opacity-50 disabled:cursor-not-allowed text-white rounded-md transition-colors"
          >
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M6 12 3.269 3.125A59.769 59.769 0 0 1 21.485 12 59.768 59.768 0 0 1 3.27 20.875L5.999 12Zm0 0h7.5" />
            </svg>
          </button>
        </form>
        <p className="text-xs text-vault-text-muted mt-2">Press Enter to send, Shift+Enter for new line</p>
      </div>
    </div>
  );
};

export default AskPanel;
