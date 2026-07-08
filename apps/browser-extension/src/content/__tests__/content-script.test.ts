// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

describe('content-script', () => {
  let messageHandler: ((request: unknown, sender: unknown, sendResponse: (data: unknown) => void) => boolean | undefined) | undefined;

  beforeEach(async () => {
    messageHandler = undefined;
    document.head.innerHTML = '';
    document.body.innerHTML = '';
    document.title = '';

    (globalThis as unknown as Record<string, unknown>).chrome = {
      runtime: {
        onMessage: {
          addListener: vi.fn((handler) => { messageHandler = handler; }),
        },
      },
    } as unknown as typeof chrome;

    // Import the content script for side effects; it registers the message listener.
    await import('../content-script');
  });

  afterEach(() => {
    vi.resetModules();
  });

  function extractPage(): Record<string, unknown> | undefined {
    let result: Record<string, unknown> | undefined;
    messageHandler?.({ action: 'extractPage' }, {}, (data) => { result = data as Record<string, unknown>; });
    return result;
  }

  it('returns basic page data when no metadata is present', () => {
    document.title = 'Plain Page';
    const data = extractPage();
    expect(data?.title).toBe('Plain Page');
    expect(data?.url).toBe(window.location.href);
    expect(data?.selectedText).toBe('');
  });

  it('uses the canonical URL when available', () => {
    document.title = 'Canonical Page';
    const link = document.createElement('link');
    link.setAttribute('rel', 'canonical');
    link.setAttribute('href', 'https://example.com/canonical');
    document.head.appendChild(link);
    const data = extractPage();
    expect(data?.url).toBe('https://example.com/canonical');
  });

  it('prefers Open Graph and Twitter titles', () => {
    document.title = 'Document Title';
    const og = document.createElement('meta');
    og.setAttribute('property', 'og:title');
    og.setAttribute('content', 'OG Title');
    document.head.appendChild(og);
    const data = extractPage();
    expect(data?.title).toBe('OG Title');
  });

  it('falls back to document.title when no social metadata exists', () => {
    document.title = 'Fallback Title';
    const data = extractPage();
    expect(data?.title).toBe('Fallback Title');
  });

  it('extracts description from meta tags', () => {
    const desc = document.createElement('meta');
    desc.setAttribute('name', 'description');
    desc.setAttribute('content', 'A description');
    document.head.appendChild(desc);
    const data = extractPage();
    expect(data?.description).toBe('A description');
  });

  it('extracts author from JSON-LD', () => {
    const script = document.createElement('script');
    script.setAttribute('type', 'application/ld+json');
    script.textContent = JSON.stringify({ author: 'Jane Doe', datePublished: '2024-01-01' });
    document.head.appendChild(script);
    const data = extractPage();
    expect(data?.author).toBe('Jane Doe');
    expect(data?.publishedDate).toBe('2024-01-01');
  });

  it('extracts author from meta tags', () => {
    const author = document.createElement('meta');
    author.setAttribute('name', 'author');
    author.setAttribute('content', 'John Smith');
    document.head.appendChild(author);
    const data = extractPage();
    expect(data?.author).toBe('John Smith');
  });

  it('ignores malformed JSON-LD', () => {
    document.title = '';
    const script = document.createElement('script');
    script.setAttribute('type', 'application/ld+json');
    script.textContent = 'not json';
    document.head.appendChild(script);
    const data = extractPage();
    expect(data?.title).toBe('Untitled');
  });

  it('returns the current selection', () => {
    document.body.innerHTML = '<p>selected text</p>';
    const range = document.createRange();
    range.selectNodeContents(document.querySelector('p')!);
    const selection = window.getSelection()!;
    selection.removeAllRanges();
    selection.addRange(range);
    const data = extractPage();
    expect(data?.selectedText).toBe('selected text');
  });
});
