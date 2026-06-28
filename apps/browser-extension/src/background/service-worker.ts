import type { PageData } from '@shared/local';

interface ContextMenuInfo {
  menuItemId: string | number;
  pageUrl?: string;
  selectionText?: string;
  linkUrl?: string;
}

interface PrefilledData {
  type: 'webpage' | 'selection' | 'link';
  title?: string;
  url: string;
  selectedText?: string;
}

interface MenuItem {
  id: string;
  title: string;
  contexts: chrome.contextMenus.ContextType[];
}

const MENU_ITEMS: MenuItem[] = [
  { id: 'clip-page', title: 'Clip page to AgentVault', contexts: ['page'] },
  { id: 'clip-selection', title: 'Clip selection to AgentVault', contexts: ['selection'] },
  { id: 'clip-link', title: 'Clip link to AgentVault', contexts: ['link'] },
];

function createContextMenus(): void {
  chrome.contextMenus.removeAll(() => {
    for (const item of MENU_ITEMS) {
      chrome.contextMenus.create({
        id: item.id,
        title: item.title,
        contexts: item.contexts,
      });
    }
  });
}

function storePrefilledData(data: PrefilledData): void {
  chrome.storage.session.set({ prefilledData: data });
}

function openPopup(): void {
  // openPopup() must be called while the user gesture is still active. The
  // gesture is lost after awaiting tabs.query/sendMessage/storage, so open the
  // popup synchronously here; the popup then reads prefilledData from storage
  // (and falls back to extracting the page itself if it isn't ready yet).
  chrome.action.openPopup().catch(() => { /* popup may already be open or unsupported */ });
}

function prefillPage(fallbackUrl?: string, fallbackTitle?: string): void {
  chrome.tabs.query({ active: true, currentWindow: true }, (tabs) => {
    const tab = tabs[0];
    if (!tab?.id) {
      const fallback: PrefilledData = { type: 'webpage', url: fallbackUrl || tab?.url || '', title: fallbackTitle || tab?.title || 'Untitled' };
      storePrefilledData(fallback);
      return;
    }
    chrome.tabs.sendMessage(tab.id, { action: 'extractPage' }, (data?: PageData) => {
      if (chrome.runtime.lastError || !data) {
        const fallback: PrefilledData = { type: 'webpage', url: fallbackUrl || tab.url || '', title: fallbackTitle || tab.title || 'Untitled' };
        storePrefilledData(fallback);
        return;
      }
      const prefilled: PrefilledData = { type: 'webpage', url: data.url, title: data.title, selectedText: data.selectedText };
      storePrefilledData(prefilled);
    });
  });
}

function prefillSelection(selectionText?: string): void {
  chrome.tabs.query({ active: true, currentWindow: true }, (tabs) => {
    const tab = tabs[0];
    if (!tab?.id) {
      const fallback: PrefilledData = { type: 'selection', url: tab?.url || '', title: tab?.title || 'Untitled', selectedText: selectionText || '' };
      storePrefilledData(fallback);
      return;
    }
    chrome.tabs.sendMessage(tab.id, { action: 'extractPage' }, (data?: PageData) => {
      const prefilled: PrefilledData = {
        type: 'selection',
        url: data?.url || tab.url || '',
        title: data?.title || tab.title || 'Untitled',
        selectedText: selectionText || data?.selectedText || '',
      };
      storePrefilledData(prefilled);
    });
  });
}

function prefillLink(linkUrl: string, pageTitle?: string): void {
  const prefilled: PrefilledData = { type: 'webpage', url: linkUrl, title: pageTitle || 'Untitled' };
  storePrefilledData(prefilled);
}

function handleContextMenuClick(info: ContextMenuInfo): void {
  const itemId = String(info.menuItemId);

  openPopup();

  if (itemId === 'clip-page') {
    prefillPage(info.pageUrl);
  } else if (itemId === 'clip-selection') {
    prefillSelection(info.selectionText);
  } else if (itemId === 'clip-link') {
    chrome.tabs.query({ active: true, currentWindow: true }, (tabs) => {
      prefillLink(info.linkUrl || '', tabs[0]?.title);
    });
  }
}

function handleCommand(command: string): void {
  openPopup();
  if (command === 'clip-page') {
    prefillPage();
  } else if (command === 'clip-selection') {
    prefillSelection();
  }
}

chrome.runtime.onInstalled.addListener(() => {
  createContextMenus();
  chrome.alarms.create('retry-captures', { periodInMinutes: 1 });
});
chrome.runtime.onStartup.addListener(createContextMenus);

chrome.contextMenus.onClicked.addListener(handleContextMenuClick);
chrome.commands.onCommand.addListener(handleCommand);

chrome.runtime.onMessage.addListener((request, _sender, sendResponse) => {
  if (request.action === 'getPrefilledData') {
    chrome.storage.session.get('prefilledData').then((result) => {
      sendResponse(result.prefilledData || null);
      if (result.prefilledData) {
        chrome.storage.session.remove('prefilledData');
      }
    });
    return true;
  }
  if (request.action === 'retryCaptures') {
    import('@shared/capture-queue').then(({ retryQueuedCaptures }) => {
      retryQueuedCaptures().then((count) => sendResponse({ synced: count }));
    });
    return true;
  }
  return false;
});

chrome.alarms?.onAlarm.addListener((alarm) => {
  if (alarm.name === 'retry-captures') {
    import('@shared/capture-queue').then(({ retryQueuedCaptures }) => {
      retryQueuedCaptures().catch(() => { /* ignore background retry errors */ });
    });
  }
});
