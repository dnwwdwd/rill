export type BestBlogsFeedKind = 'wechat' | 'x' | 'podcast' | 'youtube';

const WECHAT_FEED = /^https:\/\/wechat2rss\.bestblogs\.dev\/feed\/[0-9a-f]{40}\.xml$/i;
const X_FEED = /^https:\/\/api\.xgo\.ing\/rss\/user\/[0-9a-f]{32}$/i;
const PODCAST_FEED = /^https:\/\/rsshub\.bestblogs\.dev\/xiaoyuzhou\/podcast\/[0-9a-f]{24}$/i;
const YOUTUBE_FEED = /^https:\/\/(?:www\.)?youtube\.com\/feeds\/videos\.xml\?channel_id=UC[A-Za-z0-9_-]{22}$/i;

export const bestBlogsFeedKind = (value: string | null | undefined): BestBlogsFeedKind | null => {
  const url = value?.trim();
  if (!url) return null;
  if (WECHAT_FEED.test(url)) return 'wechat';
  if (X_FEED.test(url)) return 'x';
  if (PODCAST_FEED.test(url)) return 'podcast';
  if (YOUTUBE_FEED.test(url)) return 'youtube';
  return null;
};

export const isBestBlogsFeedUrl = (value: string | null | undefined): boolean =>
  bestBlogsFeedKind(value) !== null;
