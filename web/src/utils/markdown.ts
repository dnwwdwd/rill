import DOMPurify from 'dompurify';

/**
 * REQ-036 报告详情渲染：轻量 Markdown → 安全 HTML。
 * 只支持标题（#/##/###）、无序列表、引用、粗体、行内代码与链接；
 * 不引入重量级 Markdown 依赖，输出经 DOMPurify 白名单过滤。
 */

const escapeHtml = (text: string): string => text
  .replace(/&/g, '&amp;')
  .replace(/</g, '&lt;')
  .replace(/>/g, '&gt;')
  .replace(/"/g, '&quot;');

// 行内：先转义，再渲染粗体/行内代码/链接（URL 只允许 http/https）。
const renderInline = (text: string): string => {
  let output = escapeHtml(text);
  output = output.replace(/`([^`]+)`/g, '<code>$1</code>');
  output = output.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>');
  output = output.replace(/\[([^\]]+)\]\((https?:\/\/[^)\s]+)\)/g, '<a href="$2" target="_blank" rel="noopener noreferrer">$1</a>');
  return output;
};

export const renderMarkdown = (markdown: string): string => {
  const lines = markdown.replace(/\r\n/g, '\n').split('\n');
  const blocks: string[] = [];
  let listBuffer: string[] = [];
  let quoteBuffer: string[] = [];
  let paragraphBuffer: string[] = [];

  const flushList = () => {
    if (listBuffer.length === 0) return;
    blocks.push(`<ul>${listBuffer.map((item) => `<li>${renderInline(item)}</li>`).join('')}</ul>`);
    listBuffer = [];
  };
  const flushQuote = () => {
    if (quoteBuffer.length === 0) return;
    blocks.push(`<blockquote>${quoteBuffer.map((line) => `<p>${renderInline(line)}</p>`).join('')}</blockquote>`);
    quoteBuffer = [];
  };
  const flushParagraph = () => {
    if (paragraphBuffer.length === 0) return;
    blocks.push(`<p>${paragraphBuffer.map(renderInline).join('<br>')}</p>`);
    paragraphBuffer = [];
  };
  const flushAll = () => {
    flushList();
    flushQuote();
    flushParagraph();
  };

  for (const rawLine of lines) {
    const line = rawLine.trimEnd();
    if (!line.trim()) {
      flushAll();
      continue;
    }
    const heading = /^(#{1,4})\s+(.*)$/.exec(line);
    if (heading) {
      flushAll();
      const level = Math.min(heading[1].length + 1, 5); // # → h2，往下顺延
      blocks.push(`<h${level}>${renderInline(heading[2])}</h${level}>`);
      continue;
    }
    const listItem = /^[-*]\s+(.*)$/.exec(line.trim());
    if (listItem) {
      flushQuote();
      flushParagraph();
      listBuffer.push(listItem[1]);
      continue;
    }
    const quote = /^>\s?(.*)$/.exec(line.trim());
    if (quote) {
      flushList();
      flushParagraph();
      quoteBuffer.push(quote[1]);
      continue;
    }
    flushList();
    flushQuote();
    paragraphBuffer.push(line.trim());
  }
  flushAll();

  return DOMPurify.sanitize(blocks.join(''), {
    ALLOWED_TAGS: ['h2', 'h3', 'h4', 'h5', 'p', 'br', 'ul', 'li', 'strong', 'em', 'code', 'a', 'blockquote'],
    ALLOWED_ATTR: ['href', 'target', 'rel'],
  });
};
