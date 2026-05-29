import type { PageData } from '@shared/types';

function extractPageData(): PageData {
  const selectedText = window.getSelection()?.toString().trim() || '';
  const author =
    document.querySelector('meta[name="author"]')?.getAttribute('content') ||
    document.querySelector('meta[property="article:author"]')?.getAttribute('content') ||
    document.querySelector('meta[name="twitter:creator"]')?.getAttribute('content') ||
    undefined;
  const description =
    document.querySelector('meta[name="description"]')?.getAttribute('content') ||
    document.querySelector('meta[property="og:description"]')?.getAttribute('content') ||
    undefined;
  const publishedDate =
    document.querySelector('meta[property="article:published_time"]')?.getAttribute('content') ||
    document.querySelector('meta[name="publish-date"]')?.getAttribute('content') ||
    document.querySelector('meta[name="date"]')?.getAttribute('content') ||
    undefined;
  return {
    title: document.title || 'Untitled',
    url: window.location.href,
    selectedText,
    author,
    description,
    publishedDate,
  };
}

chrome.runtime.onMessage.addListener((request, _sender, sendResponse) => {
  if (request.action === 'extractPage') {
    sendResponse(extractPageData());
  }
  return true;
});
