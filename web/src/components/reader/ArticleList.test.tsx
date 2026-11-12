import { afterEach, describe, expect, it, vi } from 'vitest';
import { act, fireEvent, render, screen } from '@testing-library/react';
import type { ItemSummary, Source } from '@/api/types';
import { createSeedData } from '@/api/mock/seed';
import { ArticleList } from '@/components/reader/ArticleList';
import { useReaderStore } from '@/stores/readerStore';
import { useSourcesStore } from '@/stores/sourcesStore';

const seed = createSeedData();

const summary = (overrides: Partial<ItemSummary>): ItemSummary => ({
  id: 1,
  source_id: 1,
  source: { id: 1, name: '少数派', kind: 'rss' },
  title: '标题',
  url: 'https://example.com/post',
  author: null,
  published_at: '2026-11-12T00:00:00Z',
  published_estimated: false,
  excerpt: '摘要',
  cover_url: null,
  read_at: null,
  starred_at: null,
  later_at: null,
  fetched_at: '2026-11-12T00:00:00Z',
  ...overrides,
});

const setup = (items: ItemSummary[]) => {
  useSourcesStore.setState({ sources: seed.sources as Source[], folders: seed.folders, loaded: true });
  act(() => {
    useReaderStore.setState({
      items,
      stats: { total: items.length, unread: items.length, starred: 0, read: 0, hidden: 0, later: 0 },
      loading: false,
      error: null,
      nextCursor: null,
    });
  });
};

afterEach(() => {
  useReaderStore.getState().reset();
  useSourcesStore.getState().reset();
  vi.restoreAllMocks();
});

describe('ArticleList', () => {
  it('merges cross-source duplicates and shows the extra source count', () => {
    setup([
      summary({ id: 1, title: '同一条新闻', url: 'https://example.com/story?utm_source=a' }),
      summary({ id: 2, source_id: 2, source: { id: 2, name: '另一家', kind: 'rss' }, title: '同一条新闻', url: 'https://example.com/story' }),
    ]);
    render(
      <ArticleList
        selectedId={null}
        onSelect={vi.fn()}
        onOpenFocus={vi.fn()}
        searchOpen={false}
        onToggleSearch={vi.fn()}
      />,
    );

    expect(screen.getByText('+1 源')).toBeInTheDocument();
    expect(screen.getAllByText('同一条新闻')).toHaveLength(1);
  });

  it('supports multi-select and batch actions', () => {
    setup([summary({ id: 1, title: '第一篇' }), summary({ id: 2, title: '第二篇' })]);
    render(
      <ArticleList
        selectedId={null}
        onSelect={vi.fn()}
        onOpenFocus={vi.fn()}
        searchOpen={false}
        onToggleSearch={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByRole('button', { name: '多选条目' }));
    fireEvent.click(screen.getByText('第一篇'));
    expect(screen.getByText('已选 1')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '已读' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '隐藏' })).toBeInTheDocument();
  });
});
