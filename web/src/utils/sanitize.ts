import DOMPurify from 'dompurify';

const ALLOWED_TAGS = [
  'a', 'p', 'br', 'strong', 'em', 'b', 'i', 'u', 's', 'del', 'code', 'pre', 'article',
  'blockquote', 'ul', 'ol', 'li', 'h2', 'h3', 'h4', 'figure', 'figcaption',
  'img', 'hr', 'table', 'thead', 'tbody', 'tr', 'th', 'td', 'span', 'div',
  'small', 'sup', 'sub',
];

const ALLOWED_ATTR = [
  'href', 'src', 'alt', 'title', 'width', 'height', 'class', 'target', 'rel',
  'colspan', 'rowspan', 'loading', 'referrerpolicy',
];

export const sanitizeArticleHtml = (html: string): string => {
  const sanitized = DOMPurify.sanitize(html, {
    ALLOWED_TAGS,
    ALLOWED_ATTR,
    FORBID_TAGS: ['script', 'style', 'iframe', 'object', 'embed', 'form', 'input', 'button'],
    FORBID_ATTR: ['style', 'onerror', 'onload', 'onclick'],
  });
  // 很多主题会在正文开头输出一条 <hr>，读起来像标题下多了一根横线，去掉。
  return sanitized.replace(/^\s*(?:<hr[^>]*>\s*)+/i, '');
};
