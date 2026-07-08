import React from 'react';
import TestRenderer, { act } from 'react-test-renderer';
import { Text } from 'react-native';
import ErrorBoundary from '../ErrorBoundary';

function Bomb({ shouldThrow }: { shouldThrow: boolean }) {
  if (shouldThrow) {
    throw new Error('boom');
  }
  return <Text>safe</Text>;
}

describe('ErrorBoundary', () => {
  let consoleErrorSpy: jest.SpyInstance;

  beforeEach(() => {
    consoleErrorSpy = jest.spyOn(console, 'error').mockImplementation(() => {});
  });

  afterEach(() => {
    consoleErrorSpy.mockRestore();
  });

  it('renders children when there is no error', () => {
    let renderer: TestRenderer.ReactTestRenderer;
    act(() => {
      renderer = TestRenderer.create(
        <ErrorBoundary>
          <Bomb shouldThrow={false} />
        </ErrorBoundary>,
      );
    });
    expect(JSON.stringify(renderer!.toJSON())).toContain('safe');
  });

  it('renders the default fallback when an error is thrown', () => {
    let renderer: TestRenderer.ReactTestRenderer;
    act(() => {
      renderer = TestRenderer.create(
        <ErrorBoundary>
          <Bomb shouldThrow={true} />
        </ErrorBoundary>,
      );
    });
    const output = JSON.stringify(renderer!.toJSON());
    expect(output).toContain('Something went wrong');
    expect(output).toContain('boom');
  });

  it('renders a custom fallback when provided', () => {
    let renderer: TestRenderer.ReactTestRenderer;
    act(() => {
      renderer = TestRenderer.create(
        <ErrorBoundary fallback={<Text>custom fallback</Text>}>
          <Bomb shouldThrow={true} />
        </ErrorBoundary>,
      );
    });
    expect(JSON.stringify(renderer!.toJSON())).toContain('custom fallback');
  });
});
