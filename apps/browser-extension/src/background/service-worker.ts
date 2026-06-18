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

chrome.runtime.onInstalled.addListener(() => {
  chrome.contextMenus.create({ id: 'clip-page', title: 'Clip page to AgentVault', contexts: ['page'] });
  chrome.contextMenus.create({ id: 'clip-selection', title: 'Clip selection to AgentVault', contexts: ['selection'] });
  chrome.contextMenus.create({ id: 'clip-link', title: 'Clip link to AgentVault', contexts: ['link'] });
});

chrome.contextMenus.onClicked.addListener((info: ContextMenuInfo) => {
  const itemId = String(info.menuItemId);

  // openPopup() must be called while the user gesture is still active. The
  // gesture is lost after awaiting tabs.query/sendMessage/storage, so open the
  // popup synchronously here; the popup then reads prefilledData from storage
  // (and falls back to extracting the page itself if it isn't ready yet).
  chrome.action.openPopup().catch(() => { /* popup may already be open or unsupported */ });

  if (itemId === 'clip-page') {
    chrome.tabs.query({ active: true, currentWindow: true }, (tabs) => {
      const tab = tabs[0];
      if (!tab?.id) return;
      chrome.tabs.sendMessage(tab.id, { action: 'extractPage' }, (data?: PageData) => {
        if (chrome.runtime.lastError || !data) {
          const fallback: PrefilledData = { type: 'webpage', url: tab.url || info.pageUrl || '', title: tab.title || 'Untitled' };
          chrome.storage.session.set({ prefilledData: fallback });
          return;
        }
        const prefilled: PrefilledData = { type: 'webpage', url: data.url, title: data.title, selectedText: data.selectedText };
        chrome.storage.session.set({ prefilledData: prefilled });
      });
    });
  }

  if (itemId === 'clip-selection') {
    chrome.tabs.query({ active: true, currentWindow: true }, (tabs) => {
      const tab = tabs[0];
      if (!tab?.id) return;
      chrome.tabs.sendMessage(tab.id, { action: 'extractPage' }, (data?: PageData) => {
        const prefilled: PrefilledData = {
          type: 'selection',
          url: data?.url || tab.url || info.pageUrl || '',
          title: data?.title || tab.title || 'Untitled',
          selectedText: info.selectionText || data?.selectedText || '',
        };
        chrome.storage.session.set({ prefilledData: prefilled });
      });
    });
  }

  if (itemId === 'clip-link') {
    chrome.tabs.query({ active: true, currentWindow: true }, (tabs) => {
      const tab = tabs[0];
      const prefilled: PrefilledData = { type: 'webpage', url: info.linkUrl || '', title: tab?.title || 'Untitled' };
      chrome.storage.session.set({ prefilledData: prefilled });
    });
  }
});

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
  return false;
});
