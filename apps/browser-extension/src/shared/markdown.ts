/**
 * Minimal Markdown-to-HTML renderer for the browser extension.
 * Handles the most common formatting without a dependency:
 * - **bold**
 * - *italic*
 * - `code`
 * - URLs → links
 * - double newlines → paragraphs
 */

function escapeHtml(text: string): string {
  return text
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;');
}

function linkify(text: string): string {
  // Match URLs that aren't already inside <a> tags
  return text.replace(
    /(?<!["'>])(https?:\/\/[^\s<>"']+)/g,
    '<a href="$1" target="_blank" rel="noopener">$1</a>'
  );
}

function renderInline(text: string): string {
  let html = escapeHtml(text);
  // Bold: **text**
  html = html.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>');
  // Italic: *text* (but not **)
  html = html.replace(/(?<!\*)\*([^*]+)\*(?!\*)/g, '<em>$1</em>');
  // Inline code: `text`
  html = html.replace(/`([^`]+)`/g, '<code>$1</code>');
  return linkify(html);
}

export function renderMarkdown(text: string): string {
  // Split on double newlines for paragraph breaks
  const paragraphs = text.split(/\n\n+/);
  return paragraphs
    .map((p) => {
      const trimmed = p.trim();
      if (!trimmed) return '';
      // Check for code blocks (triple backtick)
      if (trimmed.startsWith('```') && trimmed.endsWith('```')) {
        const code = trimmed.slice(3, -3).replace(/^\w*\n?/, '');
        return `<pre><code>${escapeHtml(code)}</code></pre>`;
      }
      // Single newlines within paragraph → <br>
      const lines = trimmed.split('\n').map(renderInline);
      return `<p>${lines.join('<br>')}</p>`;
    })
    .filter(Boolean)
    .join('\n');
}
