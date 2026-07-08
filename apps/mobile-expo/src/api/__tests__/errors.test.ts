import { ApiError } from '@agentvault/contract';
import { NetworkError, AuthError, ServerError, classifyApiError } from '../errors';

describe('classifyApiError', () => {
  it('passes through already-classified errors', () => {
    const network = new NetworkError('custom network');
    const auth = new AuthError('custom auth');
    const server = new ServerError(503, 'custom server');

    expect(classifyApiError(network)).toBe(network);
    expect(classifyApiError(auth)).toBe(auth);
    expect(classifyApiError(server)).toBe(server);
  });

  it('maps ApiError status 0 to NetworkError', () => {
    const err = new ApiError('connection refused', 0);
    const classified = classifyApiError(err);
    expect(classified).toBeInstanceOf(NetworkError);
    expect(classified.message).toBe('connection refused');
  });

  it('maps ApiError 401 to AuthError', () => {
    const err = new ApiError('unauthorized', 401);
    const classified = classifyApiError(err);
    expect(classified).toBeInstanceOf(AuthError);
    expect(classified.message).toBe('unauthorized');
  });

  it('maps ApiError 403 to AuthError', () => {
    const err = new ApiError('forbidden', 403);
    const classified = classifyApiError(err);
    expect(classified).toBeInstanceOf(AuthError);
    expect(classified.message).toBe('forbidden');
  });

  it('maps ApiError 5xx to ServerError', () => {
    const err = new ApiError('internal server error', 500);
    const classified = classifyApiError(err);
    expect(classified).toBeInstanceOf(ServerError);
    expect((classified as ServerError).status).toBe(500);
    expect(classified.message).toBe('internal server error');
  });

  it('maps other ApiError statuses to a plain Error', () => {
    const err = new ApiError('bad request', 400);
    const classified = classifyApiError(err);
    expect(classified).toBeInstanceOf(Error);
    expect(classified).not.toBeInstanceOf(NetworkError);
    expect(classified).not.toBeInstanceOf(AuthError);
    expect(classified).not.toBeInstanceOf(ServerError);
    expect(classified.message).toBe('bad request');
  });

  it('passes through plain Error instances', () => {
    const err = new Error('plain error');
    expect(classifyApiError(err)).toBe(err);
  });

  it('stringifies non-error values', () => {
    expect(classifyApiError('boom').message).toBe('boom');
    expect(classifyApiError(123).message).toBe('123');
    expect(classifyApiError(null).message).toBe('null');
  });
});

describe('error classes', () => {
  it('NetworkError has a default message', () => {
    const err = new NetworkError();
    expect(err.name).toBe('NetworkError');
    expect(err.message).toBe('Cannot connect to the AgentVault server.');
  });

  it('AuthError has a default message', () => {
    const err = new AuthError();
    expect(err.name).toBe('AuthError');
    expect(err.message).toBe('Invalid or missing auth token.');
  });

  it('ServerError captures the status and builds a default message', () => {
    const err = new ServerError(500);
    expect(err.name).toBe('ServerError');
    expect(err.status).toBe(500);
    expect(err.message).toBe('Server error (500).');
  });
});
