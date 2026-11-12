import { useEffect, useMemo, useRef } from 'react';
import { sanitizeArticleHtml } from '@/utils/sanitize';
import { cx } from '@/utils/cx';

interface ArticleContentProps {
  html: string;
  className?: string;
  highlights?: Array<{ quote: string; color: string }>;
}

const unwrapHighlights = (root: HTMLElement) => {
  root.querySelectorAll('mark[data-rill-highlight]').forEach((mark) => {
    const parent = mark.parentNode;
    if (!parent) return;
    while (mark.firstChild) parent.insertBefore(mark.firstChild, mark);
    parent.removeChild(mark);
    parent.normalize();
  });
};

const applyHighlights = (root: HTMLElement, highlights: Array<{ quote: string; color: string }>) => {
  unwrapHighlights(root);
  for (const highlight of highlights) {
    const quote = highlight.quote.trim();
    if (quote.length < 2) continue;
    const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
    let node = walker.nextNode();
    while (node) {
      const text = node.textContent ?? '';
      const index = text.indexOf(quote);
      if (index >= 0 && !(node.parentElement?.closest('mark[data-rill-highlight]'))) {
        const range = document.createRange();
        range.setStart(node, index);
        range.setEnd(node, index + quote.length);
        const mark = document.createElement('mark');
        mark.className = `reader-highlight is-${highlight.color || 'yellow'}`;
        mark.dataset.rillHighlight = 'true';
        range.surroundContents(mark);
        break;
      }
      node = walker.nextNode();
    }
  }
};

export const ArticleContent = ({ html, className, highlights = [] }: ArticleContentProps) => {
  const ref = useRef<HTMLDivElement>(null);
  const sanitized = useMemo(() => sanitizeArticleHtml(html), [html]);

  useEffect(() => {
    const root = ref.current;
    if (!root) return;
    root.querySelectorAll('a').forEach((anchor) => {
      anchor.setAttribute('rel', 'noopener noreferrer');
      anchor.setAttribute('target', '_blank');
    });
    applyHighlights(root, highlights);
  }, [sanitized, highlights]);

  return (
    <div
      ref={ref}
      className={cx('article-body', className)}
      dangerouslySetInnerHTML={{ __html: sanitized }}
    />
  );
};
