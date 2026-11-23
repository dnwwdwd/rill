import { describe, expect, it } from 'vitest';
import { fireEvent, render } from '@testing-library/react';
import { SourceIcon } from '@/components/ui/SourceIcon';

const base = {
  id: 7,
  name: 'C1ownhjj',
  icon_url: '/api/v1/sources/7/icon',
  feed_url: 'https://fxtwitter.com/C1ownhjj/feed.xml?count=20',
};

describe('SourceIcon', () => {
  it('adds the fetch timestamp as a cache-busting version', () => {
    const { container, rerender } = render(
      <SourceIcon source={{ ...base, last_fetch_at: '2026-11-21T05:00:00Z' }} />,
    );
    expect(container.querySelector('img')?.getAttribute('src')).toBe(
      '/api/v1/sources/7/icon?v=2026-11-21T05%3A00%3A00Z',
    );
    rerender(<SourceIcon source={{ ...base, last_fetch_at: '2026-11-21T06:00:00Z' }} />);
    expect(container.querySelector('img')?.getAttribute('src')).toBe(
      '/api/v1/sources/7/icon?v=2026-11-21T06%3A00%3A00Z',
    );
  });

  it('retries the image after a refresh instead of keeping the broken state', () => {
    const { container, rerender } = render(
      <SourceIcon source={{ ...base, last_fetch_at: '2026-11-21T05:00:00Z' }} />,
    );
    fireEvent.error(container.querySelector('img')!);
    expect(container.querySelector('img')).toBeNull();
    // 手动刷新后 last_fetch_at 变化：必须重新渲染 img，而不是一直显示首字母。
    rerender(<SourceIcon source={{ ...base, last_fetch_at: '2026-11-21T06:00:00Z' }} />);
    expect(container.querySelector('img')?.getAttribute('src')).toContain('v=2026-11-21T06');
  });
});
