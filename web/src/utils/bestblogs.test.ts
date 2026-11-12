import { describe, expect, it } from 'vitest';
import { bestBlogsFeedKind, isBestBlogsFeedUrl } from '@/utils/bestblogs';

const WECHAT = 'https://wechat2rss.bestblogs.dev/feed/ff621c3e98d6ae6fceb3397e57441ffc6ea3c17f.xml';
const X = 'https://api.xgo.ing/rss/user/0c0856a69f9f49cf961018c32a0b0049';
const PODCAST = 'https://rsshub.bestblogs.dev/xiaoyuzhou/podcast/5e74b52c418a84a046ecaceb';
const YOUTUBE = 'https://www.youtube.com/feeds/videos.xml?channel_id=UCrDwWp7EBBv4NwvScIpBDOA';

describe('bestBlogsFeedKind', () => {
  it('recognizes the four catalog feed types', () => {
    expect(bestBlogsFeedKind(WECHAT)).toBe('wechat');
    expect(bestBlogsFeedKind(X)).toBe('x');
    expect(bestBlogsFeedKind(PODCAST)).toBe('podcast');
    expect(bestBlogsFeedKind(YOUTUBE)).toBe('youtube');
    expect(isBestBlogsFeedUrl(WECHAT)).toBe(true);
  });

  it('rejects other hosts, paths, and schemes', () => {
    expect(bestBlogsFeedKind('https://example.com/feed/ff621c3e98d6ae6fceb3397e57441ffc6ea3c17f.xml')).toBeNull();
    expect(bestBlogsFeedKind('http://wechat2rss.bestblogs.dev/feed/ff621c3e98d6ae6fceb3397e57441ffc6ea3c17f.xml')).toBeNull();
    expect(bestBlogsFeedKind('https://wechat2rss.bestblogs.dev/feed/not-a-feed.xml')).toBeNull();
    expect(bestBlogsFeedKind('https://api.xgo.ing/rss/user/not-a-user')).toBeNull();
    expect(bestBlogsFeedKind('https://www.youtube.com/feeds/videos.xml?channel_id=bad')).toBeNull();
    expect(bestBlogsFeedKind(null)).toBeNull();
  });
});
