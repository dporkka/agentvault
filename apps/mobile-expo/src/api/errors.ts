import { ApiError } from '@agentvault/contract';

export class NetworkError extends Error {
  constructor(message = 'Cannot connect to the AgentVault server.') {
    super(message);
    this.name = 'NetworkError';
  }
}

export class AuthError extends Error {
  constructor(message = 'Invalid or missing auth token.') {
    super(message);
    this.name = 'AuthError';
  }
}

export class ServerError extends Error {
  status: number;
  constructor(status: number, message = `Server error (${status}).`) {
    super(message);
    this.name = 'ServerError';
    this.status = status;
  }
}

export function classifyApiError(err: unknown): Error {
  if (err instanceof NetworkError || err instanceof AuthError || err instanceof ServerError) {
    return err;
  }
  if (err instanceof ApiError) {
    if (err.status === 0) {
      return new NetworkError(err.message);
    }
    if (err.status === 401 || err.status === 403) {
      return new AuthError(err.message);
    }
    if (err.status >= 500) {
      return new ServerError(err.status, err.message);
    }
    return new Error(err.message);
  }
  if (err instanceof Error) {
    return err;
  }
  return new Error(String(err));
}
