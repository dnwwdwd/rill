import { describe, expect, it } from 'vitest';
import { render, waitFor } from '@testing-library/react';
import { ArticleContent } from '@/components/reader/ArticleContent';

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

  it('keeps semantic article markup for the stylesheet', () => {
    const { container } = render(
      <ArticleContent html={'<article><h2>标题</h2><p>正文</p><blockquote><p>引用</p></blockquote><figure><img src="/media/x/1" alt="图" /><figcaption>说明</figcaption></figure></article>'} />,
    );
    expect(container.querySelector('.article-body article h2')).toBeInTheDocument();
    expect(container.querySelector('figure figcaption')).toHaveTextContent('说明');
    expect(container.querySelector('img')).toHaveAttribute('alt', '图');
  });
});
