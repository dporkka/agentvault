import React from 'react';
import TestRenderer, { act } from 'react-test-renderer';
import ProjectPicker from '../ProjectPicker';
import { useSettings } from '../../context/SettingsContext';
import { getProjects } from '../../api/agentvault';

jest.mock('../../context/SettingsContext', () => ({
  useSettings: jest.fn(),
}));

jest.mock('../../api/agentvault', () => ({
  getProjects: jest.fn(),
}));

jest.useFakeTimers();

describe('ProjectPicker', () => {
  function render(props: Partial<React.ComponentProps<typeof ProjectPicker>> = {}) {
    const onChange = props.onChange ?? jest.fn();
    let renderer: TestRenderer.ReactTestRenderer;
    act(() => {
      renderer = TestRenderer.create(<ProjectPicker selected="" onChange={onChange} {...props} />);
    });
    return { renderer: renderer!, onChange };
  }

  beforeEach(() => {
    jest.clearAllMocks();
    (useSettings as jest.Mock).mockReturnValue({
      settings: { serverUrl: 'http://127.0.0.1:47321', token: '', defaultProject: '' },
    });
    (getProjects as jest.Mock).mockResolvedValue([]);
  });

  it('renders the current selection', () => {
    const { renderer } = render({ selected: 'work' });
    const tree = renderer.toJSON();
    expect(JSON.stringify(tree)).toContain('work');
  });

  it('opens the picker when pressed', () => {
    const { renderer } = render();
    const selector = renderer.root.findByProps({
      accessibilityLabel: 'Select project, current: none',
    });

    act(() => selector.props.onPress());

    const tree = renderer.toJSON();
    expect(JSON.stringify(tree)).toContain('Select Project');
  });

  it('loads and displays server projects', async () => {
    (getProjects as jest.Mock).mockResolvedValue(['alpha', 'beta']);
    const { renderer } = render();

    await act(async () => {
      await Promise.resolve();
    });

    act(() => {
      renderer.root
        .findByProps({ accessibilityLabel: 'Select project, current: none' })
        .props.onPress();
    });

    const tree = renderer.toJSON();
    const strings = JSON.stringify(tree);
    expect(strings).toContain('alpha');
    expect(strings).toContain('beta');
  });

  it('falls back to defaults when the server call fails', async () => {
    (getProjects as jest.Mock).mockRejectedValue(new Error('offline'));
    const { renderer } = render();

    await act(async () => {
      await Promise.resolve();
    });

    act(() => {
      renderer.root
        .findByProps({ accessibilityLabel: 'Select project, current: none' })
        .props.onPress();
    });

    const tree = renderer.toJSON();
    expect(JSON.stringify(tree)).toContain('personal');
  });

  it('selects a project from the list', () => {
    const onChange = jest.fn();
    const { renderer } = render({ onChange });

    act(() => {
      renderer.root
        .findByProps({ accessibilityLabel: 'Select project, current: none' })
        .props.onPress();
    });

    act(() => {
      renderer.root.findByProps({ accessibilityLabel: 'Select project work' }).props.onPress();
    });

    expect(onChange).toHaveBeenCalledWith('work');
  });

  it('adds a custom project', () => {
    const onChange = jest.fn();
    const { renderer } = render({ onChange });

    act(() => {
      renderer.root
        .findByProps({ accessibilityLabel: 'Select project, current: none' })
        .props.onPress();
    });

    const input = renderer.root.findByProps({ placeholder: 'New project...' });
    act(() => input.props.onChangeText('custom-project'));
    act(() => input.props.onSubmitEditing());

    expect(onChange).toHaveBeenCalledWith('custom-project');
  });

  it('ignores empty custom projects', () => {
    const onChange = jest.fn();
    const { renderer } = render({ onChange });

    act(() => {
      renderer.root
        .findByProps({ accessibilityLabel: 'Select project, current: none' })
        .props.onPress();
    });

    const input = renderer.root.findByProps({ placeholder: 'New project...' });
    act(() => input.props.onChangeText('   '));
    act(() => input.props.onSubmitEditing());

    expect(onChange).not.toHaveBeenCalled();
  });

  it('closes the picker without selecting', () => {
    const onChange = jest.fn();
    const { renderer } = render({ onChange });

    act(() => {
      renderer.root
        .findByProps({ accessibilityLabel: 'Select project, current: none' })
        .props.onPress();
    });
    act(() => {
      renderer.root.findByProps({ accessibilityLabel: 'Close project picker' }).props.onPress();
    });

    expect(onChange).not.toHaveBeenCalled();
  });
});
