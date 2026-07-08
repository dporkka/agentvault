import { describe, it, expect } from 'vitest';
import { ApiError } from '@agentvault/contract';
import { classifyError, errorKindLabel, errorRecoveryHint } from '../errors';

describe('classifyError', () => {
  it('classifies an ApiError with status 0 as network/recoverable', () => {
    const err = new ApiError('Connection failed', 0);
    const classified = classifyError(err);
    expect(classified.kind).toBe('network');
    expect(classified.recoverable).toBe(true);
    expect(classified.message).toBe('Connection failed');
  });

  it('classifies an ApiError with status 401 as auth/recoverable', () => {
    const err = new ApiError('Unauthorized', 401);
    const classified = classifyError(err);
    expect(classified.kind).toBe('auth');
    expect(classified.recoverable).toBe(true);
  });

  it('classifies an ApiError with status 500 as server/recoverable', () => {
    const err = new ApiError('Internal error', 500);
    const classified = classifyError(err);
    expect(classified.kind).toBe('server');
    expect(classified.recoverable).toBe(true);
  });

  it('classifies an ApiError with status 400 as client/non-recoverable', () => {
    const err = new ApiError('Bad request', 400);
    const classified = classifyError(err);
    expect(classified.kind).toBe('client');
    expect(classified.recoverable).toBe(false);
  });

  it('classifies network-looking Error as network/recoverable', () => {
    const err = new Error('Unable to fetch resource');
    const classified = classifyError(err);
    expect(classified.kind).toBe('network');
    expect(classified.recoverable).toBe(true);
  });

  it('classifies other Errors as unknown/non-recoverable', () => {
    const err = new Error('Something else');
    const classified = classifyError(err);
    expect(classified.kind).toBe('unknown');
    expect(classified.recoverable).toBe(false);
  });

  it('classifies string errors', () => {
    const classified = classifyError('boom');
    expect(classified.message).toBe('boom');
    expect(classified.kind).toBe('unknown');
  });

  it('classifies unknown primitives with a default message', () => {
    const classified = classifyError(42);
    expect(classified.kind).toBe('unknown');
    expect(classified.message).toBe('An unknown error occurred');
  });
});

describe('errorKindLabel', () => {
  it('returns a label for every kind', () => {
    expect(errorKindLabel('network')).toBe('Connection error');
    expect(errorKindLabel('auth')).toBe('Auth error');
    expect(errorKindLabel('server')).toBe('Server error');
    expect(errorKindLabel('client')).toBe('Request error');
    expect(errorKindLabel('unknown')).toBe('Error');
  });
});

describe('errorRecoveryHint', () => {
  it('returns a hint for every kind', () => {
    expect(errorRecoveryHint('network')).toContain('running');
    expect(errorRecoveryHint('auth')).toContain('token');
    expect(errorRecoveryHint('server')).toContain('later');
    expect(errorRecoveryHint('client')).toContain('input');
    expect(errorRecoveryHint('unknown')).toContain('Something went wrong');
  });
});
