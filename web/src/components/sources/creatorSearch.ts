import type { CreatorSearchCandidate, CreatorSearchPlatformStatus } from '@/api/types';

export const creatorPlatformOrder = [
  'apple',
  'bluesky',
  'bestblogs',
  'bestblogs-podcast',
  'bestblogs-x',
  'bestblogs-youtube',
];

const creatorPlatformLabels: Record<string, string> = {
  apple: 'Apple Podcasts',
  bluesky: 'Bluesky',
  bestblogs: 'BestBlogs 公众号',
  'bestblogs-podcast': 'BestBlogs 小宇宙',
  'bestblogs-x': 'BestBlogs X',
  'bestblogs-youtube': 'BestBlogs YouTube',
  xiaoyuzhou: '小宇宙',
  medium: 'Medium',
  mastodon: 'Mastodon',
};

export const creatorPlatformLabel = (platform: string): string => creatorPlatformLabels[platform] ?? platform;

export const creatorCandidateKey = (candidate: CreatorSearchCandidate): string => (
  `${candidate.platform}:${candidate.feed_url}`
);

export const groupCreatorCandidates = (
  candidates: CreatorSearchCandidate[],
): Array<{ platform: string; label: string; candidates: CreatorSearchCandidate[] }> => {
  const byPlatform = new Map<string, CreatorSearchCandidate[]>();
  for (const candidate of candidates) {
    const group = byPlatform.get(candidate.platform);
    if (group) {
      group.push(candidate);
    } else {
      byPlatform.set(candidate.platform, [candidate]);
    }
  }
  return [...byPlatform.entries()]
    .sort(([left], [right]) => {
      const leftIndex = creatorPlatformOrder.indexOf(left);
      const rightIndex = creatorPlatformOrder.indexOf(right);
      return (leftIndex === -1 ? 999 : leftIndex) - (rightIndex === -1 ? 999 : rightIndex);
    })
    .map(([platform, entries]) => ({
      platform,
      label: creatorPlatformLabel(platform),
      candidates: entries,
    }));
};

export const creatorRefreshSeconds = (platform: string): number | undefined => {
  if (platform === 'bestblogs-x') return 1800;
  if (platform === 'bestblogs' || platform === 'bestblogs-podcast' || platform === 'bestblogs-youtube') return 3600;
  return undefined;
};

export const creatorStatusText = (status: CreatorSearchPlatformStatus): string => {
  switch (status.status) {
    case 'ok':
      return `${status.count} 个`;
    case 'empty':
      return '无结果';
    case 'timeout':
      return '超时';
    case 'error':
      return '失败';
    default:
      return status.status;
  }
};

export const creatorMatchLabel = (match?: string): string | null => {
  if (match === 'exact') return '精确';
  if (match === 'prefix' || match === 'contains') return '相近';
  return null;
};
