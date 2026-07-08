import React from 'react';
import TestRenderer, { act } from 'react-test-renderer';
import SearchResultCard from '../SearchResultCard';
import type { SearchResult } from '../../types';

describe('SearchResultCard', () => {
  const result: SearchResult = {
    id: 'note-1',
    title: 'Meeting notes',
    snippet: 'Discussed the roadmap.',
    path: 'work/meetings.md',
    type: 'meeting',
  };

  function render(props: Partial<React.ComponentProps<typeof SearchResultCard>> = {}) {
    let renderer: TestRenderer.ReactTestRenderer;
    act(() => {
      renderer = TestRenderer.create(<SearchResultCard result={result} {...props} />);
    });
    return renderer!;
  }

  it('renders the title, snippet, and path', () => {
    const tree = render().toJSON();
    const strings = JSON.stringify(tree);
    expect(strings).toContain('Meeting notes');
    expect(strings).toContain('Discussed the roadmap.');
    expect(strings).toContain('work/meetings.md');
  });

  it('calls onPress when pressed', () => {
    const onPress = jest.fn();
    const renderer = render({ onPress });
    const button = renderer.root.findByProps({ accessibilityLabel: 'Open Meeting notes' });

    act(() => button.props.onPress());

    expect(onPress).toHaveBeenCalledWith(result);
  });

  it('renders without an onPress handler', () => {
    const renderer = render();
    expect(renderer.toJSON()).toBeTruthy();
  });
});
