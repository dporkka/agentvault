import React from 'react';
import TestRenderer, { act } from 'react-test-renderer';
import CaptureCard from '../CaptureCard';
import type { Capture } from '../../types';

describe('CaptureCard', () => {
  const capture: Capture = {
    id: 'cap-1',
    type: 'text',
    title: 'Idea',
    text: 'A great idea for the project.',
    project: 'work',
    tags: ['idea', 'todo'],
    createdAt: new Date('2024-06-15T10:30:00Z').toISOString(),
    synced: false,
    syncStatus: 'unsynced',
  };

  function render(props: Partial<React.ComponentProps<typeof CaptureCard>> = {}) {
    let renderer: TestRenderer.ReactTestRenderer;
    act(() => {
      renderer = TestRenderer.create(<CaptureCard capture={capture} {...props} />);
    });
    return renderer!;
  }

  it('renders the title, preview, project, and tags', () => {
    const tree = render().toJSON();
    const strings = JSON.stringify(tree);
    expect(strings).toContain('Idea');
    expect(strings).toContain('A great idea for the project.');
    expect(strings).toContain('work');
    expect(strings).toContain('idea');
    expect(strings).toContain('todo');
  });

  it('calls onPress when pressed', () => {
    const onPress = jest.fn();
    const renderer = render({ onPress });
    const button = renderer.root.findByProps({ accessibilityLabel: 'Capture Idea' });

    act(() => button.props.onPress());

    expect(onPress).toHaveBeenCalledWith(capture);
  });

  it('calls onDelete when the delete button is pressed', () => {
    const onDelete = jest.fn();
    const renderer = render({ onDelete });
    const deleteBtn = renderer.root.findByProps({ accessibilityLabel: 'Delete capture' });

    act(() => deleteBtn.props.onPress());

    expect(onDelete).toHaveBeenCalledWith('cap-1');
  });

  it('calls onRetry when the retry button is pressed for failed captures', () => {
    const onRetry = jest.fn();
    const failedCapture: Capture = { ...capture, syncStatus: 'failed', syncError: 'network error' };
    const renderer = render({ capture: failedCapture, onRetry });
    const retryBtn = renderer.root.findByProps({ accessibilityLabel: 'Retry sync' });

    act(() => retryBtn.props.onPress());

    expect(onRetry).toHaveBeenCalledWith(failedCapture);
  });

  it('does not render a retry button for synced captures', () => {
    const syncedCapture: Capture = { ...capture, synced: true, syncStatus: 'synced' };
    const renderer = render({ capture: syncedCapture });
    expect(() => renderer.root.findByProps({ accessibilityLabel: 'Retry sync' })).toThrow();
  });

  it('renders sync error and retry count when present', () => {
    const failedCapture: Capture = {
      ...capture,
      syncStatus: 'failed',
      syncError: 'server rejected',
      retryCount: 3,
    };
    const renderer = render({ capture: failedCapture });
    const strings = JSON.stringify(renderer.toJSON());
    expect(strings).toContain('server rejected');
    expect(strings).toContain('Attempt');
    expect(strings).toContain('3');
  });
});
