import React from 'react';
import TestRenderer, { act } from 'react-test-renderer';
import { Text } from 'react-native';
import { SettingsProvider, useSettings, DEFAULT_SETTINGS } from '../SettingsContext';
import { loadSettings } from '../../storage/settingsStore';

jest.mock('../../storage/settingsStore', () => ({
  DEFAULT_APP_SETTINGS: {
    serverUrl: 'http://127.0.0.1:47321',
    defaultProject: '',
    token: '',
  },
  loadSettings: jest.fn(),
  persistSettings: jest.fn(),
}));

describe('SettingsContext', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  function Consumer() {
    const { settings, loaded } = useSettings();
    return (
      <>
        <Text testID="loaded">{loaded ? 'loaded' : 'loading'}</Text>
        <Text testID="serverUrl">{settings.serverUrl}</Text>
        <Text testID="defaultProject">{settings.defaultProject}</Text>
      </>
    );
  }

  function render() {
    let renderer: TestRenderer.ReactTestRenderer;
    act(() => {
      renderer = TestRenderer.create(
        <SettingsProvider>
          <Consumer />
        </SettingsProvider>,
      );
    });
    return renderer!;
  }

  it('loads settings and exposes them', async () => {
    (loadSettings as jest.Mock).mockResolvedValue({
      ...DEFAULT_SETTINGS,
      serverUrl: 'http://custom.com',
      defaultProject: 'work',
    });

    const renderer = render();
    expect(renderer.root.findByProps({ testID: 'loaded' }).props.children).toBe('loading');

    await act(async () => {
      await Promise.resolve();
    });

    expect(renderer.root.findByProps({ testID: 'loaded' }).props.children).toBe('loaded');
    expect(renderer.root.findByProps({ testID: 'serverUrl' }).props.children).toBe(
      'http://custom.com',
    );
    expect(renderer.root.findByProps({ testID: 'defaultProject' }).props.children).toBe('work');
  });

  it('uses defaults when loading fails', async () => {
    (loadSettings as jest.Mock).mockRejectedValue(new Error('locked'));

    const renderer = render();

    await act(async () => {
      await Promise.resolve();
    });

    expect(renderer.root.findByProps({ testID: 'loaded' }).props.children).toBe('loaded');
    expect(renderer.root.findByProps({ testID: 'serverUrl' }).props.children).toBe(
      DEFAULT_SETTINGS.serverUrl,
    );
  });

  it('throws when useSettings is used outside a provider', () => {
    function BadConsumer() {
      useSettings();
      return null;
    }

    expect(() => {
      act(() => {
        TestRenderer.create(<BadConsumer />);
      });
    }).toThrow('useSettings must be used within a SettingsProvider');
  });
});
