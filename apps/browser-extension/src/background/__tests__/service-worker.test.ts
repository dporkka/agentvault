// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

const mockRetryQueuedCaptures = vi.hoisted(() => vi.fn());
vi.mock('@shared/capture-queue', () => ({
  retryQueuedCaptures: mockRetryQueuedCaptures,
}));

interface ListenerMock<T = unknown> {
  listeners: ((...args: T[]) => void)[];
  addListener: (handler: (...args: T[]) => void) => void;
  trigger: (...args: T[]) => void;
}

function createListenerMock<T = unknown>(): ListenerMock<T> {
  const listeners: ((...args: T[]) => void)[] = [];
  return {
    listeners,
    addListener: (handler) => { listeners.push(handler); },
    trigger: (...args) => { listeners.forEach((h) => h(...args)); },
  };
}

describe('service-worker', () => {
  const installed = createListenerMock<chrome.runtime.InstalledDetails>();
  const startup = createListenerMock();
  const contextMenuClicked = createListenerMock<chrome.contextMenus.OnClickData>();
  const command = createListenerMock<string>();
  const alarm = createListenerMock<chrome.alarms.Alarm>();
  const message = createListenerMock<[unknown, chrome.runtime.MessageSender, (response: unknown) => void]>();

  const contextMenus: { items: chrome.contextMenus.CreateProperties[]; removeAll: (cb?: () => void) => void; create: (props: chrome.contextMenus.CreateProperties) => void } = {
    items: [],
    removeAll: vi.fn((cb?: () => void) => {
      contextMenus.items = [];
      cb?.();
    }),
    create: vi.fn((props) => { contextMenus.items.push(props); }),
  };

  const sessionStorage: Record<string, unknown> = {};
  const tabFixtures: chrome.tabs.Tab[] = [{ id: 1, title: 'Example', url: 'https://example.com' } as chrome.tabs.Tab];

  let messageHandler: ((request: unknown, sender: chrome.runtime.MessageSender, sendResponse: (response: unknown) => void) => boolean | undefined) | undefined;

  beforeEach(async () => {
    vi.resetModules();
    mockRetryQueuedCaptures.mockReset().mockResolvedValue(0);
    contextMenus.items = [];
    Object.keys(sessionStorage).forEach((k) => delete sessionStorage[k]);
    installed.listeners.length = 0;
    startup.listeners.length = 0;
    contextMenuClicked.listeners.length = 0;
    command.listeners.length = 0;
    alarm.listeners.length = 0;
    message.listeners.length = 0;
    messageHandler = undefined;

    (globalThis as unknown as Record<string, unknown>).chrome = {
      runtime: {
        onInstalled: { addListener: installed.addListener },
        onStartup: { addListener: startup.addListener },
        onMessage: {
          addListener: (handler: typeof messageHandler) => {
            messageHandler = handler;
            message.addListener(handler as never);
          },
        },
      },
      contextMenus: {
        removeAll: contextMenus.removeAll,
        create: contextMenus.create,
        onClicked: { addListener: contextMenuClicked.addListener },
      },
      commands: { onCommand: { addListener: command.addListener } },
      alarms: {
        create: vi.fn(),
        onAlarm: { addListener: alarm.addListener },
      },
      storage: {
        session: {
          set: vi.fn((items) => { Object.assign(sessionStorage, items); }),
          get: vi.fn((key) => Promise.resolve({ [key]: sessionStorage[key as string] })),
          remove: vi.fn((key) => { delete sessionStorage[key as string]; }),
        },
      },
      tabs: {
        query: vi.fn((_query, cb) => { cb(tabFixtures); }),
        sendMessage: vi.fn((_tabId, _message, cb) => { cb?.(undefined); }),
      },
      action: {
        openPopup: vi.fn().mockResolvedValue(undefined),
      },
    } as unknown as typeof chrome;

    await import('../service-worker');
  });

  afterEach(() => {
    vi.resetModules();
  });

  function sendMessage(request: unknown): Promise<unknown> {
    return new Promise((resolve) => {
      const handled = messageHandler?.(request, {}, resolve);
      if (!handled) {
        resolve('__not_handled__');
      }
    });
  }

  it('creates context menus on install', () => {
    installed.trigger({ reason: 'install' as chrome.runtime.OnInstalledReason });
    expect(contextMenus.removeAll).toHaveBeenCalled();
    expect(contextMenus.items).toHaveLength(3);
    expect(contextMenus.items.map((i) => i.id)).toEqual(['clip-page', 'clip-selection', 'clip-link']);
  });

  it('recreates context menus on startup', () => {
    startup.trigger();
    expect(contextMenus.removeAll).toHaveBeenCalled();
    expect(contextMenus.items).toHaveLength(3);
  });

  it('creates the retry alarm on install', () => {
    installed.trigger({ reason: 'install' as chrome.runtime.OnInstalledReason });
    expect(chrome.alarms.create).toHaveBeenCalledWith('retry-captures', { periodInMinutes: 1 });
  });

  it('handles clip-page context menu click', () => {
    installed.trigger({ reason: 'install' as chrome.runtime.OnInstalledReason });
    contextMenuClicked.trigger({ menuItemId: 'clip-page', pageUrl: 'https://example.com/page' } as chrome.contextMenus.OnClickData);
    expect(chrome.action.openPopup).toHaveBeenCalled();
    expect(sessionStorage.prefilledData).toEqual({ type: 'webpage', url: 'https://example.com/page', title: 'Example' });
  });

  it('handles clip-selection context menu click', () => {
    installed.trigger({ reason: 'install' as chrome.runtime.OnInstalledReason });
    contextMenuClicked.trigger({ menuItemId: 'clip-selection', selectionText: 'selected text' } as chrome.contextMenus.OnClickData);
    expect(sessionStorage.prefilledData).toMatchObject({ type: 'selection', selectedText: 'selected text' });
  });

  it('handles clip-link context menu click', () => {
    installed.trigger({ reason: 'install' as chrome.runtime.OnInstalledReason });
    contextMenuClicked.trigger({ menuItemId: 'clip-link', linkUrl: 'https://example.com/link' } as chrome.contextMenus.OnClickData);
    expect(sessionStorage.prefilledData).toEqual({ type: 'webpage', url: 'https://example.com/link', title: 'Example' });
  });

  it('handles keyboard commands', () => {
    command.trigger('clip-page');
    expect(chrome.action.openPopup).toHaveBeenCalled();
    expect(sessionStorage.prefilledData).toMatchObject({ type: 'webpage' });

    command.trigger('clip-selection');
    expect(sessionStorage.prefilledData).toMatchObject({ type: 'selection' });
  });

  it('returns prefilled data via runtime message and clears it', async () => {
    sessionStorage.prefilledData = { type: 'selection', url: 'https://example.com', title: 'Example' };
    const response = await sendMessage({ action: 'getPrefilledData' });
    expect(response).toEqual({ type: 'selection', url: 'https://example.com', title: 'Example' });
    expect(sessionStorage.prefilledData).toBeUndefined();
  });

  it('returns null when no prefilled data exists', async () => {
    const response = await sendMessage({ action: 'getPrefilledData' });
    expect(response).toBeNull();
  });

  it('retries queued captures via runtime message', async () => {
    mockRetryQueuedCaptures.mockResolvedValue(2);
    const response = await sendMessage({ action: 'retryCaptures' });
    expect(mockRetryQueuedCaptures).toHaveBeenCalled();
    expect(response).toEqual({ synced: 2 });
  });

  it('retries queued captures on the alarm', async () => {
    mockRetryQueuedCaptures.mockResolvedValue(1);
    alarm.trigger({ name: 'retry-captures' } as chrome.alarms.Alarm);
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(mockRetryQueuedCaptures).toHaveBeenCalled();
  });

  it('ignores unrelated runtime messages', async () => {
    const response = await sendMessage({ action: 'unknown' });
    expect(response).toBe('__not_handled__');
  });
});
