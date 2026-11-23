import { describe, expect, it } from 'vitest';
import { fireEvent, render, waitFor } from '@testing-library/react';
import { ArticleContent } from '@/components/reader/ArticleContent';
import { useSettingsStore } from '@/stores/settingsStore';

describe('ArticleContent', () => {
  it('sanitizes dangerous markup and hardens external links', async () => {
    const { container } = render(
      <ArticleContent html={'<p onclick="steal()">正文<script>alert(1)</script></p><p><a href="https://example.com/post">原文</a></p>'} />,
    );

    expect(container.querySelector('script')).toBeNull();
    expect(container.querySelector('p')?.getAttribute('onclick')).toBeNull();
    await waitFor(() => {
      const anchor = container.querySelector('a');
      expect(anchor).toHaveAttribute('rel', 'noopener noreferrer');
      expect(anchor).toHaveAttribute('target', '_blank');
    });
  });

  it('drops the referrer on article images so WeChat CDNs do not return the hotlink placeholder', () => {
    const { container } = render(
      <ArticleContent html={'<p><img src="https://mmbiz.qpic.cn/sz_mmbiz_jpg/example/640?wx_fmt=jpeg" alt="微信图" /></p>'} />,
    );
    expect(container.querySelector('img')).toHaveAttribute('referrerpolicy', 'no-referrer');
  });

  it('keeps semantic article markup for the stylesheet', () => {
    const { container } = render(
      <ArticleContent html={'<article><h2>标题</h2><p>正文</p><blockquote><p>引用</p></blockquote><figure><img src="/media/x/1" alt="图" /><figcaption>说明</figcaption></figure></article>'} />,
    );
    expect(container.querySelector('.article-body article h2')).toBeInTheDocument();
    expect(container.querySelector('figure figcaption')).toHaveTextContent('说明');
    expect(container.querySelector('img')).toHaveAttribute('alt', '图');
  });
});

describe('ArticleContent email images', () => {
  it('blocks remote email images until the reader allows them', () => {
    const html = '<p><img src="/media/token/hash" data-remote-src="https://cdn.example.com/logo.png" width="120" height="40" alt="logo" /></p>';
    const onAllowImages = vi.fn();
    const { container, getByRole } = render(
      <ArticleContent html={html} isEmail senderEmail="news@example.com" onAllowImages={onAllowImages} />,
    );

    expect(container.querySelector('.email-image-consent')).toHaveTextContent('1 张远程图片已拦截');
    expect(container.querySelector('img')).toBeNull();
    expect(container.querySelector('.email-image-blocked')).toBeInTheDocument();

    fireEvent.click(getByRole('button', { name: '显示图片' }));

    const image = container.querySelector('img');
    // Proxy-first: the proxied src stays as-is; the original URL is only used
    // if the proxy request fails.
    expect(image).toHaveAttribute('src', '/media/token/hash');
    expect(image).toHaveAttribute('data-remote-src', 'https://cdn.example.com/logo.png');
    expect(container.querySelector('.email-image-consent')).toBeNull();
    expect(onAllowImages).toHaveBeenCalledTimes(1);

    fireEvent.error(image!);
    expect(image).toHaveAttribute('src', 'https://cdn.example.com/logo.png');
  });

  it('keeps rich email markup such as inline styles, data images and backgrounds', () => {
    const html = [
      '<table background="https://cdn.example.com/bg.png" style="background-image:url(\'https://cdn.example.com/bg2.png\')">',
      '<tr><td><center><font color="#336699">标题</font></center>',
      '<img src="data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9Wl2n4sAAAAASUVORK5CYII=" data-inline-image="1">',
      '</td></tr></table>',
    ].join('');
    const { container } = render(<ArticleContent html={html} isEmail imagesAllowed />);

    const table = container.querySelector('table');
    expect(table).toHaveAttribute('background', 'https://cdn.example.com/bg.png');
    expect(table?.getAttribute('style')).toContain('background-image');
    expect(container.querySelector('center font')).toHaveTextContent('标题');
    expect(container.querySelector('center font')).toHaveAttribute('color', '#336699');
    expect(container.querySelector('img')?.getAttribute('src')).toContain('data:image/png;base64,');
  });

  it('blocks CSS background images until the reader allows them', () => {
    const html = `<table background="https://cdn.example.com/bg.png" style="background-image:url('https://cdn.example.com/bg2.png');color:red"><tr><td>正文</td></tr></table>`;
    const { container, getByRole } = render(<ArticleContent html={html} isEmail senderEmail="news@example.com" />);

    const table = container.querySelector('table');
    expect(container.querySelector('.email-image-consent')).toHaveTextContent('2 张远程图片已拦截');
    expect(table).not.toHaveAttribute('background');
    expect(table?.getAttribute('style')).not.toContain('background-image');
    expect(table?.getAttribute('style')).toContain('color:red');

    fireEvent.click(getByRole('button', { name: '显示图片' }));
    expect(container.querySelector('table')).toHaveAttribute('background', 'https://cdn.example.com/bg.png');
    expect(container.querySelector('table')?.getAttribute('style')).toContain('background-image');
  });

  it('keeps showing images for an email that was already allowed', () => {
    const html = '<p><img src="/media/token/hash" data-remote-src="https://cdn.example.com/logo.png" alt="logo" /></p>';
    const { container } = render(
      <ArticleContent html={html} isEmail senderEmail="news@example.com" imagesAllowed />,
    );

    expect(container.querySelector('.email-image-consent')).toBeNull();
    expect(container.querySelector('img')).toHaveAttribute('src', '/media/token/hash');
  });

  it('offers to trust the sender and keeps inline images visible', () => {
    const update = vi.fn();
    useSettingsStore.setState((state) => ({ settings: { ...state.settings, email: { ...state.settings.email, block_remote_images: true, trusted_senders: [] } }, update }));
    const html = '<p><img src="/media/token/inline" data-remote-src="https://rill-email-inline.invalid/a/b" data-inline-image="1" alt="inline" /><img src="/media/token/hash" data-remote-src="https://cdn.example.com/logo.png" alt="logo" /></p>';
    const { container, getByRole } = render(
      <ArticleContent html={html} isEmail senderEmail="News@Example.com" />,
    );

    expect(container.querySelector('img')).toHaveAttribute('src', '/media/token/inline');
    fireEvent.click(getByRole('button', { name: '始终显示此发件人的图片' }));
    expect(update).toHaveBeenCalledWith({ email: { trusted_senders: ['news@example.com'] } });
  });
});
