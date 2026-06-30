import type { PageData } from '@shared/local';

const ARTICLE_SELECTORS = [
  'article',
  'main',
  '[role="main"]',
  '.content',
  '.post',
  '.entry',
  '.entry-content',
  '.post-content',
  '.article-content',
  '.page-content',
  '#content',
  '#main',
  '#main-content',
  '.story',
  '.article',
];

const EXCLUDE_TAGS = new Set([
  'nav',
  'header',
  'footer',
  'aside',
  'script',
  'style',
  'noscript',
  'iframe',
  'form',
  'button',
  'input',
  'textarea',
  'select',
]);

const AD_SELECTORS = [
  '[class*="ad"]',
  '[class*="advert"]',
  '[id*="ad"]',
  '[id*="advert"]',
  '[class*="social"]',
  '[class*="share"]',
  '[class*="newsletter"]',
  '[class*="subscribe"]',
  '[class*="comment"]',
  '[class*="related"]',
  '[class*="sidebar"]',
];

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

function isExcludedElement(el: Element): boolean {
  const tag = el.tagName.toLowerCase();
  if (EXCLUDE_TAGS.has(tag)) return true;
  if (el.closest('nav, header, footer, aside, [role="navigation"], [role="banner"], [role="complementary"]')) return true;
  for (const selector of AD_SELECTORS) {
    if (el.matches(selector)) return true;
  }
  return false;
}

function cleanNode(node: Node): void {
  if (node.nodeType === Node.COMMENT_NODE) {
    node.parentNode?.removeChild(node);
    return;
  }
  if (node.nodeType !== Node.ELEMENT_NODE) return;
  const el = node as Element;
  if (isExcludedElement(el)) {
    el.remove();
    return;
  }
  Array.from(el.childNodes).forEach(cleanNode);
}

function elementTextLength(el: Element): number {
  return (el.textContent || '').trim().length;
}

function scoreElement(el: Element): number {
  const tag = el.tagName.toLowerCase();
  const textLen = elementTextLength(el);
  const links = el.querySelectorAll('a').length;
  const linkText = Array.from(el.querySelectorAll('a')).reduce((sum, a) => sum + (a.textContent || '').trim().length, 0);
  const linkDensity = textLen > 0 ? linkText / textLen : 0;

  let score = textLen;

  if (tag === 'article') score *= 2.5;
  else if (tag === 'main' || el.getAttribute('role') === 'main') score *= 2.0;
  else if (el.matches('.content, .post, .entry, .entry-content, .post-content, .article-content, .page-content, .story, .article')) score *= 1.6;
  else if (el.matches('#content, #main, #main-content')) score *= 1.4;

  // Penalize link-heavy and very short candidates.
  score *= 1 - Math.min(linkDensity, 0.9);
  if (textLen < 100) score *= 0.3;

  return score;
}

function extractArticleText(maxLength = 5000): string {
  const body = document.body;
  if (!body) return '';

  // Clone body so we don't mutate the live DOM.
  const clone = body.cloneNode(true) as HTMLElement;
  cleanNode(clone);

  const candidates: { el: Element; score: number }[] = [];
  for (const selector of ARTICLE_SELECTORS) {
    for (const el of Array.from(clone.querySelectorAll(selector))) {
      if (elementTextLength(el) >= 50) {
        candidates.push({ el, score: scoreElement(el) });
      }
    }
  }

  // Also consider paragraphs and divs that look like content blocks.
  if (candidates.length === 0) {
    for (const el of Array.from(clone.querySelectorAll('p, div, section'))) {
      const len = elementTextLength(el);
      if (len >= 200 && len <= 20000) {
        candidates.push({ el, score: scoreElement(el) });
      }
    }
  }

  let best = candidates.sort((a, b) => b.score - a.score)[0]?.el;
  if (!best) {
    best = clone;
  }

  const text = (best as HTMLElement).innerText || (best as HTMLElement).textContent || '';
  const cleaned = text.replace(/\s+/g, ' ').trim();
  return cleaned.length > maxLength ? cleaned.slice(0, maxLength) + '…' : cleaned;
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
    text: extractArticleText(),
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
