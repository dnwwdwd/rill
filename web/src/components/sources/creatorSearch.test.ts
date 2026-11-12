import { describe, expect, it } from 'vitest';
import type { CreatorSearchCandidate, CreatorSearchPlatformStatus } from '@/api/types';
import {
  creatorCandidateKey,
  creatorMatchLabel,
  creatorPlatformLabel,
  creatorRefreshSeconds,
  creatorStatusText,
  groupCreatorCandidates,
} from '@/components/sources/creatorSearch';

const candidate = (platform: string, name: string, feed: string): CreatorSearchCandidate => ({
  platform,
  kind: 'rss',
  name,
  feed_url: feed,
});

describe('creator search helpers', () => {
  it('groups candidates by platform in display order', () => {
    const groups = groupCreatorCandidates([
      candidate('bestblogs-x', 'OpenAI', 'https://example.com/x'),
      candidate('bluesky', 'OpenAI', 'https://example.com/bsky'),
      candidate('bestblogs', 'OpenAI', 'https://example.com/wx'),
      candidate('apple', 'OpenAI', 'https://example.com/apple'),
    ]);
    expect(groups.map((group) => group.platform)).toEqual(['apple', 'bluesky', 'bestblogs', 'bestblogs-x']);
    expect(groups.map((group) => group.label)).toEqual(['Apple Podcasts', 'Bluesky', 'BestBlogs 公众号', 'BestBlogs X']);
  });

  it('builds stable keys and platform labels', () => {
    expect(creatorCandidateKey(candidate('apple', 'A', 'https://example.com/a'))).toBe('apple:https://example.com/a');
    expect(creatorPlatformLabel('bestblogs-youtube')).toBe('BestBlogs YouTube');
    expect(creatorPlatformLabel('unknown')).toBe('unknown');
  });

  it('maps platform refresh intervals', () => {
      expect(creatorRefreshSeconds('bestblogs-x')).toBe(1800);
    expect(creatorRefreshSeconds('bestblogs')).toBe(3600);
    expect(creatorRefreshSeconds('apple')).toBeUndefined();
  });

  it('renders search status and match labels', () => {
    const status = (value: CreatorSearchPlatformStatus['status'], count = 0): CreatorSearchPlatformStatus => ({
      platform: 'apple', label: 'Apple Podcasts', status: value, count,
    });
    expect(creatorStatusText(status('ok', 2))).toBe('2 个');
    expect(creatorStatusText(status('empty'))).toBe('无结果');
    expect(creatorStatusText(status('timeout'))).toBe('超时');
    expect(creatorStatusText(status('error'))).toBe('失败');
    expect(creatorMatchLabel('exact')).toBe('精确');
    expect(creatorMatchLabel('contains')).toBe('相近');
    expect(creatorMatchLabel('handle')).toBeNull();
  });
});
