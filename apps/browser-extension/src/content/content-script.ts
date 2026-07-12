import type { PageData } from '@shared/local';

function getMeta(selector: string): string | undefined {
  const value = document.querySelector(selector)?.getAttribute('content');
  return value || undefined;
}

function getJsonLdValue(...keys: string[]): string | undefined {
  const scripts = Array.from(document.querySelectorAll('script[type="application/ld+json"]'));
  for (const script of scripts) {
    try {
      const data = JSON.parse(script.textContent || '{}');
      const candidates: unknown[] = Array.isArray(data) ? data : [data];
      for (const item of candidates) {
        if (!item || typeof item !== 'object') continue;
        for (const key of keys) {
          const value = (item as Record<string, unknown>)[key];
          if (typeof value === 'string' && value.trim()) return value.trim();
        }
      }
    } catch {
      // Ignore malformed JSON-LD.
    }
  }
  return undefined;
}

function resolveUrl(href: string | undefined | null): string {
  if (!href) return window.location.href;
  try {
    return new URL(href, window.location.href).href;
  } catch {
    return window.location.href;
  }
}

function extractPageData(): PageData {
  const selectedText = window.getSelection()?.toString().trim() || '';
  const canonicalHref = document.querySelector('link[rel="canonical"]')?.getAttribute('href');
  const url = resolveUrl(canonicalHref);

  const ogTitle = getMeta('meta[property="og:title"]');
  const twitterTitle = getMeta('meta[name="twitter:title"]');
  const title = ogTitle || twitterTitle || document.title || 'Untitled';

  const ogDescription = getMeta('meta[property="og:description"]');
  const description = getMeta('meta[name="description"]') || ogDescription || undefined;

  const author =
    getJsonLdValue('author', 'creator') ||
    getMeta('meta[name="author"]') ||
    getMeta('meta[property="article:author"]') ||
    getMeta('meta[name="twitter:creator"]') ||
    undefined;

  const publishedDate =
    getJsonLdValue('datePublished', 'dateCreated') ||
    getMeta('meta[property="article:published_time"]') ||
    getMeta('meta[name="publish-date"]') ||
    getMeta('meta[name="date"]') ||
    undefined;

  return {
    title,
    url,
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
  if (request.action === 'captureSuccess') {
    showCaptureToast();
  }
  return true;
});

function showCaptureToast(): void {
  const toast = document.createElement('div');
  toast.textContent = '\u2714 Saved to AgentVault';
  Object.assign(toast.style, {
    position: 'fixed',
    bottom: '24px',
    left: '50%',
    transform: 'translateX(-50%)',
    background: '#1a1d27',
    color: '#e4e6eb',
    border: '1px solid #2e3344',
    borderRadius: '24px',
    padding: '10px 20px',
    fontSize: '13px',
    fontFamily: 'system-ui, sans-serif',
    fontWeight: '500',
    zIndex: '2147483647',
    boxShadow: '0 4px 12px rgba(0,0,0,0.4)',
    opacity: '0',
    transition: 'opacity 0.3s ease',
    pointerEvents: 'none',
  });
  document.body.appendChild(toast);
  requestAnimationFrame(() => { toast.style.opacity = '1'; });
  setTimeout(() => {
    toast.style.opacity = '0';
    setTimeout(() => toast.remove(), 300);
  }, 2000);
}
