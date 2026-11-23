import DOMPurify from 'dompurify';

const ALLOWED_TAGS = [
  'a', 'p', 'br', 'strong', 'em', 'b', 'i', 'u', 's', 'del', 'code', 'pre', 'article',
  'blockquote', 'ul', 'ol', 'li', 'h2', 'h3', 'h4', 'figure', 'figcaption',
  'img', 'hr', 'table', 'thead', 'tbody', 'tr', 'th', 'td', 'span', 'div',
  'small', 'sup', 'sub',
];

const ALLOWED_ATTR = [
  'href', 'src', 'alt', 'title', 'width', 'height', 'class', 'target', 'rel',
  'colspan', 'rowspan', 'loading', 'referrerpolicy', 'data-remote-src', 'data-inline-image',
];

// Email HTML relies on inline table layout, background images and legacy
// presentational tags. Keep these for mail only; article feeds stay tighter.
const EMAIL_ALLOWED_TAGS = [
  ...ALLOWED_TAGS,
  'center', 'font', 'col', 'colgroup', 'caption', 'tfoot', 'dd', 'dl', 'dt',
  'mark', 'q', 'abbr', 'cite',
];

const EMAIL_ALLOWED_ATTR = [
  ...ALLOWED_ATTR,
  'style', 'background', 'align', 'valign', 'bgcolor', 'border',
  'cellpadding', 'cellspacing', 'dir', 'color', 'face', 'size',
];

// 微信 mmbiz.qpic.cn 按 Referer 做防盗链：从 Rill 域名直接加载会返回
// 「此图片来自微信公众平台未经允许不可引用」占位图。存量正文里的远程图片
// 来不及回填为 /media 代理，因此渲染前统一去掉 Referer；lazycat-search 同做法。
DOMPurify.addHook('afterSanitizeAttributes', (node) => {
  if (node.tagName === 'IMG') {
    node.setAttribute('referrerpolicy', 'no-referrer');
  }
});

export const sanitizeArticleHtml = (html: string, options: { email?: boolean } = {}): string => {
  const email = Boolean(options.email);
  const sanitized = DOMPurify.sanitize(html, {
    ALLOWED_TAGS: email ? EMAIL_ALLOWED_TAGS : ALLOWED_TAGS,
    ALLOWED_ATTR: email ? EMAIL_ALLOWED_ATTR : ALLOWED_ATTR,
    FORBID_TAGS: ['script', 'style', 'iframe', 'object', 'embed', 'form', 'input', 'button'],
    FORBID_ATTR: email ? ['onerror', 'onload', 'onclick'] : ['style', 'onerror', 'onload', 'onclick'],
  });
  // 很多主题会在正文开头输出一条 <hr>，读起来像标题下多了一根横线，去掉。
  return sanitized.replace(/^\s*(?:<hr[^>]*>\s*)+/i, '');
};
