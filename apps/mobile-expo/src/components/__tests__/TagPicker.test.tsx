import React from 'react';
import TestRenderer, { act } from 'react-test-renderer';
import { TextInput } from 'react-native';
import TagPicker from '../TagPicker';

describe('TagPicker', () => {
  function render(props: Partial<React.ComponentProps<typeof TagPicker>> = {}) {
    const onChange = props.onChange ?? jest.fn();
    let renderer: TestRenderer.ReactTestRenderer;
    act(() => {
      renderer = TestRenderer.create(<TagPicker selected={[]} onChange={onChange} {...props} />);
    });
    return { renderer: renderer!, onChange };
  }

  it('renders the default suggestions', () => {
    const { renderer } = render();
    const tree = renderer.toJSON();
    const strings = JSON.stringify(tree);
    expect(strings).toContain('idea');
    expect(strings).toContain('todo');
    expect(strings).toContain('meeting');
  });

  it('toggles a suggested tag on press', () => {
    const onChange = jest.fn();
    const { renderer } = render({ selected: [], onChange });
    const chip = renderer.root.findByProps({ accessibilityLabel: 'Toggle tag idea' });

    act(() => chip.props.onPress());

    expect(onChange).toHaveBeenCalledWith(['idea']);
  });

  it('removes a selected tag when pressed', () => {
    const onChange = jest.fn();
    const { renderer } = render({ selected: ['idea', 'todo'], onChange });
    const selectedTag = renderer.root.findByProps({ accessibilityLabel: 'Remove tag idea' });

    act(() => selectedTag.props.onPress());

    expect(onChange).toHaveBeenCalledWith(['todo']);
  });

  it('adds a custom tag from the input', () => {
    const onChange = jest.fn();
    const { renderer } = render({ selected: [], onChange });
    const input = renderer.root.findByType(TextInput);

    act(() => input.props.onChangeText('custom'));
    act(() => input.props.onSubmitEditing());

    expect(onChange).toHaveBeenCalledWith(['custom']);
  });

  it('adds a custom tag when the add button is pressed', () => {
    const onChange = jest.fn();
    const { renderer } = render({ selected: [], onChange });
    const input = renderer.root.findByType(TextInput);
    const addBtn = renderer.root.findByProps({ accessibilityLabel: 'Add tag' });

    act(() => input.props.onChangeText('urgent'));
    act(() => addBtn.props.onPress());

    expect(onChange).toHaveBeenCalledWith(['urgent']);
  });

  it('ignores duplicate custom tags', () => {
    const onChange = jest.fn();
    const { renderer } = render({ selected: ['idea'], onChange });
    const input = renderer.root.findByType(TextInput);

    act(() => input.props.onChangeText('idea'));
    act(() => input.props.onSubmitEditing());

    expect(onChange).not.toHaveBeenCalled();
  });

  it('ignores whitespace-only custom tags', () => {
    const onChange = jest.fn();
    const { renderer } = render({ selected: [], onChange });
    const input = renderer.root.findByType(TextInput);

    act(() => input.props.onChangeText('   '));
    act(() => input.props.onSubmitEditing());

    expect(onChange).not.toHaveBeenCalled();
  });

  it('merges provided suggestions with defaults', () => {
    const { renderer } = render({ suggestions: ['custom-tag'] });
    const tree = renderer.toJSON();
    const strings = JSON.stringify(tree);
    expect(strings).toContain('custom-tag');
    expect(strings).toContain('idea');
  });
});
