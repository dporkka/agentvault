// Error classification helpers for the browser extension. Errors from
// @agentvault/contract are bucketed into user-actionable categories so the
// popup can show the right message (and recovery hint) without leaking
// implementation details.

import { ApiError } from '@agentvault/contract';

export type ErrorKind = 'network' | 'auth' | 'server' | 'client' | 'unknown';

export interface ClassifiedError {
  kind: ErrorKind;
  message: string;
  /** Whether the user can likely recover by retrying or fixing input. */
  recoverable: boolean;
}

function looksLikeNetworkError(err: Error): boolean {
  return /connect|network|fetch|offline|unreachable|eai_again/i.test(err.message);
}

function extractMessage(err: unknown): string {
  if (err instanceof Error) return err.message;
  if (typeof err === 'string') return err;
  return 'An unknown error occurred';
}

/**
 * Classify an unknown error into a small set of user-facing categories.
 * - network:  cannot reach the AgentVault server (status 0 or network-ish message)
 * - auth:     reached the server but the token is missing/invalid (401)
 * - server:   server-side failure (5xx)
 * - client:   request was rejected by the server (4xx other than 401)
 * - unknown:  anything else
 */
export function classifyError(err: unknown): ClassifiedError {
  if (err instanceof ApiError) {
    const message = err.message || extractMessage(err);
    if (err.status === 0 || looksLikeNetworkError(err)) {
      return { kind: 'network', message, recoverable: true };
    }
    if (err.status === 401) {
      return { kind: 'auth', message, recoverable: true };
    }
    if (err.status >= 500) {
      return { kind: 'server', message, recoverable: true };
    }
    if (err.status >= 400) {
      return { kind: 'client', message, recoverable: false };
    }
    return { kind: 'unknown', message, recoverable: false };
  }

  if (err instanceof Error) {
    if (looksLikeNetworkError(err)) {
      return { kind: 'network', message: err.message, recoverable: true };
    }
    return { kind: 'unknown', message: err.message, recoverable: false };
  }

  return { kind: 'unknown', message: extractMessage(err), recoverable: false };
}

/** Short, user-facing label for a classified error kind. */
export function errorKindLabel(kind: ErrorKind): string {
  switch (kind) {
    case 'network': return 'Connection error';
    case 'auth': return 'Auth error';
    case 'server': return 'Server error';
    case 'client': return 'Request error';
    case 'unknown': return 'Error';
  }
}

/** Suggested recovery action for a classified error kind. */
export function errorRecoveryHint(kind: ErrorKind): string {
  switch (kind) {
    case 'network': return 'Make sure AgentVault is running and try again.';
    case 'auth': return 'Check your token in Settings.';
    case 'server': return 'The AgentVault server failed. Try again later.';
    case 'client': return 'Check your input and try again.';
    case 'unknown': return 'Something went wrong. Try again.';
  }
}
