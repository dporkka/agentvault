import { useState, useCallback, useRef, useEffect } from 'react';
import { Sparkles, X, Send, FileText, AlertTriangle, CheckCircle } from './Icons';
import type { Answer, Source } from '../types';

interface Props {
  onClose: () => void;
  onOpenNote: (path: string) => void;
  vaultPath: string;
}

interface ChatMessage {
  id: string;
  role: 'user' | 'assistant';
  content: string;
  answer?: Answer;
}

export default function AIPanel({ onClose, onOpenNote }: Props) {
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [input, setInput] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const [aiEnabled, setAiEnabled] = useState<boolean | null>(null);
  const messagesEndRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    window.go.main.AIService.IsAIEnabled()
      .then(setAiEnabled)
      .catch(() => setAiEnabled(false));
  }, []);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages]);

  const handleSubmit = useCallback(async (e: React.FormEvent) => {
    e.preventDefault();
    if (!input.trim() || isLoading) return;

    const question = input.trim();
    const userMsg: ChatMessage = {
      id: `user-${Date.now()}`,
      role: 'user',
      content: question,
    };

    setMessages(prev => [...prev, userMsg]);
    setInput('');
    setIsLoading(true);

    try {
      const answer = await window.go.main.AIService.Ask(question);
      const assistantMsg: ChatMessage = {
        id: `assistant-${Date.now()}`,
        role: 'assistant',
        content: answer.answer,
        answer,
      };
      setMessages(prev => [...prev, assistantMsg]);
    } catch (err: any) {
      const errorMsg: ChatMessage = {
        id: `error-${Date.now()}`,
        role: 'assistant',
        content: `Error: ${err.message || 'Failed to get answer'}\n\nMake sure Ollama is running:\n1. Install Ollama: https://ollama.com\n2. Run: ollama pull llama3.1`,
      };
      setMessages(prev => [...prev, errorMsg]);
    } finally {
      setIsLoading(false);
    }
  }, [input, isLoading]);

  const getConfidenceColor = (c: string) => {
    switch (c) {
      case 'high': return 'text-green-400';
      case 'medium': return 'text-yellow-400';
      case 'low': return 'text-red-400';
      default: return 'text-gray-400';
    }
  };

  return (
    <div className="w-96 flex flex-col bg-[var(--bg-secondary)] border-l border-[var(--border)]">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-[var(--border)]">
        <div className="flex items-center gap-2">
          <Sparkles className="w-4 h-4 text-[var(--accent)]" />
          <span className="text-sm font-medium text-[var(--text-primary)]">AI Assistant</span>
          {aiEnabled === true && (
            <CheckCircle className="w-3.5 h-3.5 text-green-400" title="AI enabled" />
          )}
          {aiEnabled === false && (
            <AlertTriangle className="w-3.5 h-3.5 text-yellow-400" title="AI not configured" />
          )}
        </div>
        <button
          onClick={onClose}
          className="p-1 rounded hover:bg-[var(--bg-hover)] text-[var(--text-muted)]"
        >
          <X className="w-4 h-4" />
        </button>
      </div>

      {/* Messages */}
      <div className="flex-1 overflow-auto px-4 py-3 space-y-4">
        {messages.length === 0 && (
          <div className="text-center py-8">
            <Sparkles className="w-8 h-8 mx-auto mb-3 text-[var(--accent)] opacity-50" />
            <p className="text-sm text-[var(--text-muted)]">
              Ask anything about your vault
            </p>
            <div className="mt-3 space-y-1.5">
              {[
                'What have I decided about pricing?',
                'Summarize my project notes',
                'What tasks are open?',
              ].map(q => (
                <button
                  key={q}
                  onClick={() => setInput(q)}
                  className="block w-full text-xs text-left px-3 py-2 rounded-md bg-[var(--bg-tertiary)] text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-hover)] transition-colors"
                >
                  {q}
                </button>
              ))}
            </div>
          </div>
        )}

        {messages.map(msg => (
          <div key={msg.id} className={`${msg.role === 'user' ? 'ml-4' : 'mr-4'}`}>
            {msg.role === 'user' ? (
              <div className="bg-[var(--accent)]/10 rounded-lg px-3 py-2">
                <p className="text-sm text-[var(--text-primary)]">{msg.content}</p>
              </div>
            ) : (
              <div className="space-y-3">
                <div className="prose prose-invert prose-sm max-w-none">
                  <div className="text-sm text-[var(--text-primary)] whitespace-pre-wrap">
                    {msg.content}
                  </div>
                </div>

                {msg.answer?.sources && msg.answer.sources.length > 0 && (
                  <div className="border-t border-[var(--border)] pt-2">
                    <div className="text-xs font-medium text-[var(--text-muted)] mb-1.5">Sources</div>
                    <div className="space-y-1">
                      {msg.answer.sources.map((source: Source, i: number) => (
                        <button
                          key={i}
                          onClick={() => onOpenNote(source.path)}
                          className="w-full flex items-start gap-2 px-2 py-1.5 rounded text-left hover:bg-[var(--bg-hover)] transition-colors"
                        >
                          <FileText className="w-3 h-3 mt-0.5 text-[var(--text-muted)] flex-shrink-0" />
                          <div className="min-w-0">
                            <div className="text-xs font-medium text-[var(--accent)] truncate">
                              {source.title}
                            </div>
                            <div className="text-[10px] text-[var(--text-muted)] truncate">
                              {source.path}
                            </div>
                          </div>
                        </button>
                      ))}
                    </div>

                    <div className="flex items-center gap-2 mt-2 text-xs">
                      <span className="text-[var(--text-muted)]">Confidence:</span>
                      <span className={`font-medium ${getConfidenceColor(msg.answer.confidence)}`}>
                        {msg.answer.confidence}
                      </span>
                    </div>

                    {msg.answer.caveats && msg.answer.caveats.length > 0 && (
                      <div className="mt-1.5 text-[10px] text-[var(--text-muted)]">
                        {msg.answer.caveats.join(', ')}
                      </div>
                    )}
                  </div>
                )}
              </div>
            )}
          </div>
        ))}

        {isLoading && (
          <div className="flex items-center gap-2 text-sm text-[var(--text-muted)]">
            <div className="w-4 h-4 border-2 border-[var(--accent)] border-t-transparent rounded-full animate-spin" />
            Thinking...
          </div>
        )}

        <div ref={messagesEndRef} />
      </div>

      {/* Input */}
      <form onSubmit={handleSubmit} className="px-3 py-3 border-t border-[var(--border)]">
        <div className="flex items-center gap-2">
          <input
            type="text"
            value={input}
            onChange={(e) => setInput(e.target.value)}
            placeholder="Ask about your vault..."
            className="flex-1 input py-2 text-sm"
            disabled={isLoading}
          />
          <button
            type="submit"
            disabled={isLoading || !input.trim()}
            className="p-2 rounded-lg bg-[var(--accent)] text-white hover:bg-[var(--accent-hover)] disabled:opacity-50 transition-colors"
          >
            <Send className="w-4 h-4" />
          </button>
        </div>
      </form>
    </div>
  );
}
