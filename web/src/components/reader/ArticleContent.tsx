import { useEffect, useMemo, useRef, useState } from 'react';
import { sanitizeArticleHtml } from '@/utils/sanitize';
import { cx } from '@/utils/cx';
import { useSettingsStore } from '@/stores/settingsStore';

interface ArticleContentProps {
  html: string;
  className?: string;
  highlights?: Array<{ quote: string; color: string }>;
  /** 邮件正文使用更紧凑的布局，并启用远程图片放行。 */
  isEmail?: boolean;
  senderEmail?: string | null;
  /** 该邮件条目已选择「显示图片」，之后打开不再拦截。 */
  imagesAllowed?: boolean;
  onAllowImages?: () => void;
}

const isRemoteImageURL = (value: string): boolean => /^(?:https?:)?\/\//i.test(value.trim());

const styleHasRemoteBackground = (style: string): boolean =>
  /background(?:-image)?\s*:[^;]*url\(\s*['"]?(?:https?:)?\/\//i.test(style);

const stripRemoteBackgrounds = (root: ParentNode) => {
  root.querySelectorAll('[background]').forEach((element) => {
    if (isRemoteImageURL(element.getAttribute('background') ?? '')) element.removeAttribute('background');
  });
  root.querySelectorAll('[style]').forEach((element) => {
    const style = element.getAttribute('style') ?? '';
    if (!styleHasRemoteBackground(style)) return;
    const cleaned = style
      .split(';')
      .filter((declaration) => !(
        /^\s*background(?:-image)?\s*:/i.test(declaration) &&
        /url\(\s*['"]?(?:https?:)?\/\//i.test(declaration)
      ))
      .join(';');
    if (cleaned.trim()) element.setAttribute('style', cleaned);
    else element.removeAttribute('style');
  });
};

const countRemoteEmailImages = (html: string): number => {
  if (!html.includes('data-remote-src') && !html.includes('background')) return 0;
  const doc = new DOMParser().parseFromString(html, 'text/html');
  let count = 0;
  doc.querySelectorAll('img[data-remote-src]').forEach((image) => {
    if (!image.hasAttribute('data-inline-image')) count += 1;
  });
  doc.querySelectorAll('[background]').forEach((element) => {
    if (isRemoteImageURL(element.getAttribute('background') ?? '')) count += 1;
  });
  doc.querySelectorAll('[style]').forEach((element) => {
    if (styleHasRemoteBackground(element.getAttribute('style') ?? '')) count += 1;
  });
  return count;
};

const emailImagePolicy = (html: string, showRemote: boolean): string => {
  const doc = new DOMParser().parseFromString(html, 'text/html');
  doc.querySelectorAll('img[data-remote-src]').forEach((image) => {
    if (image.hasAttribute('data-inline-image')) return;
    const remote = image.getAttribute('data-remote-src') ?? '';
    if (!remote) return;
    if (showRemote) {
      // Prefer the /media proxy: it avoids mixed-content blocking and works
      // for hosts that reject browser Referer headers. The original URL stays
      // on the tag as a fallback when the proxy fails.
      if (!image.getAttribute('src')) image.setAttribute('src', remote);
      image.setAttribute('referrerpolicy', 'no-referrer');
      return;
    }
    const width = Number(image.getAttribute('width') ?? '0');
    const height = Number(image.getAttribute('height') ?? '0');
    // 追踪像素不值得占位。
    if ((width > 0 && width <= 2) || (height > 0 && height <= 2)) {
      image.remove();
      return;
    }
    const placeholder = doc.createElement('span');
    placeholder.className = 'email-image-blocked';
    placeholder.setAttribute('role', 'img');
    placeholder.setAttribute('aria-label', '远程图片已拦截');
    if (width > 0) placeholder.setAttribute('width', String(Math.min(width, 640)));
    if (height > 0) placeholder.setAttribute('height', String(Math.min(height, 480)));
    image.replaceWith(placeholder);
  });
  if (!showRemote) stripRemoteBackgrounds(doc);
  return doc.body.innerHTML;
};

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

export const ArticleContent = ({ html, className, highlights = [], isEmail = false, senderEmail, imagesAllowed = false, onAllowImages }: ArticleContentProps) => {
  const ref = useRef<HTMLDivElement>(null);
  const blockRemoteImages = useSettingsStore((state) => state.settings.email.block_remote_images);
  const trustedSenders = useSettingsStore((state) => state.settings.email.trusted_senders);
  const [allowOnce, setAllowOnce] = useState(false);
  const sanitized = useMemo(() => sanitizeArticleHtml(html, { email: isEmail }), [html, isEmail]);
  const sender = (senderEmail ?? '').trim().toLowerCase();
  const trusted = Boolean(sender) && trustedSenders.includes(sender);
  const showRemote = !isEmail || !blockRemoteImages || trusted || imagesAllowed || allowOnce;
  const remoteCount = useMemo(
    () => (isEmail ? countRemoteEmailImages(sanitized) : 0),
    [isEmail, sanitized],
  );
  const content = useMemo(
    () => (isEmail ? emailImagePolicy(sanitized, showRemote) : sanitized),
    [isEmail, sanitized, showRemote],
  );

  useEffect(() => {
    const root = ref.current;
    if (!root) return;
    root.querySelectorAll('a').forEach((anchor) => {
      anchor.setAttribute('rel', 'noopener noreferrer');
      anchor.setAttribute('target', '_blank');
    });
    root.querySelectorAll<HTMLImageElement>('img[data-remote-src]').forEach((image) => {
      if (image.hasAttribute('data-inline-image')) return;
      const remote = image.getAttribute('data-remote-src');
      if (!remote) return;
      const fallback = () => {
        if (image.getAttribute('src') !== remote) image.setAttribute('src', remote);
      };
      image.addEventListener('error', fallback, { once: true });
      if (image.complete && image.naturalWidth === 0 && (image.getAttribute('src') ?? '').startsWith('/media/')) {
        fallback();
      }
    });
    applyHighlights(root, highlights);
  }, [content, highlights]);

  useEffect(() => {
    setAllowOnce(false);
  }, [html]);

  const allowSender = () => {
    if (!sender) return;
    useSettingsStore.getState().update({
      email: { trusted_senders: Array.from(new Set([...trustedSenders, sender])) },
    });
  };

  const allowImages = () => {
    setAllowOnce(true);
    onAllowImages?.();
  };

  return (
    <>
      {isEmail && remoteCount > 0 && !showRemote ? (
        <div className="email-image-consent">
          <span>{remoteCount} 张远程图片已拦截</span>
          <button type="button" onClick={allowImages}>显示图片</button>
          {sender ? (
            <button type="button" className="email-image-consent-link" onClick={allowSender}>
              始终显示此发件人的图片
            </button>
          ) : null}
        </div>
      ) : null}
      <div
        ref={ref}
        className={cx('article-body', isEmail && 'is-email', className)}
        dangerouslySetInnerHTML={{ __html: content }}
      />
    </>
  );
};
