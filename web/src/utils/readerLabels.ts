import type { Folder, Source } from '@/api/types';
import type { ReaderFilter } from '@/types/ui';

export const readerTitle = (filter: ReaderFilter, sources: Source[], folders: Folder[]): string => {
  if (filter.type === 'source') return sources.find((source) => source.id === filter.sourceId)?.name ?? '订阅源';
  if (filter.type === 'folder') return folders.find((folder) => folder.id === filter.folderId)?.name ?? '文件夹';
  if (filter.type === 'unread') return '未读';
  if (filter.type === 'starred') return '收藏';
  if (filter.type === 'later') return '稍后读';
  if (filter.type === 'read') return '已读';
  return '全部';
};

export const readerEyebrow = (filter: ReaderFilter): string => {
  if (filter.type === 'source') return '订阅源';
  if (filter.type === 'folder') return '文件夹';
  return '阅读';
};
