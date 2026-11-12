const pad = (value: number) => String(value).padStart(2, '0');

const isValidDate = (date: Date) => !Number.isNaN(date.getTime());

export const formatTime = (value: string | Date | null | undefined): string => {
  if (!value) return '—';
  const date = value instanceof Date ? value : new Date(value);
  if (!isValidDate(date)) return '—';
  return `${pad(date.getHours())}:${pad(date.getMinutes())}`;
};

export const formatListTime = (value: string | null | undefined, now = new Date()): string => {
  if (!value) return '—';
  const date = new Date(value);
  if (!isValidDate(date)) return '—';
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  const target = new Date(date.getFullYear(), date.getMonth(), date.getDate());
  const diffDays = Math.round((today.getTime() - target.getTime()) / 86_400_000);
  if (diffDays === 0) return `今天 ${formatTime(date)}`;
  if (diffDays === 1) return `昨天 ${formatTime(date)}`;
  if (date.getFullYear() === now.getFullYear()) return `${date.getMonth() + 1} 月 ${date.getDate()} 日`;
  return `${date.getFullYear()} 年 ${date.getMonth() + 1} 月 ${date.getDate()} 日`;
};

export const formatAgo = (value: string | null | undefined, now = new Date()): string => {
  if (!value) return '—';
  const date = new Date(value);
  if (!isValidDate(date)) return '—';
  const diff = Math.max(0, now.getTime() - date.getTime());
  const minutes = Math.floor(diff / 60_000);
  if (minutes < 1) return '刚刚';
  if (minutes < 60) return `${minutes} 分钟前`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours} 小时前`;
  const days = Math.floor(hours / 24);
  if (days < 7) return `${days} 天前`;
  return formatListTime(value, now);
};

export const formatDateTime = (value: string | null | undefined): string => {
  if (!value) return '—';
  const date = new Date(value);
  if (!isValidDate(date)) return '—';
  return `${date.getFullYear()} 年 ${date.getMonth() + 1} 月 ${date.getDate()} 日 ${formatTime(date)}`;
};

export const formatNumber = (value: number): string => new Intl.NumberFormat('zh-CN').format(value);

export const formatTokenCount = (value: number): string => {
  if (value >= 10_000) return `${(value / 10_000).toFixed(value >= 1_000_000 ? 0 : 1)} 万`;
  return formatNumber(value);
};

export const formatDuration = (ms: number | null | undefined): string => {
  if (ms === null || ms === undefined) return '—';
  if (ms >= 1000) return `${(ms / 1000).toFixed(1)} 秒`;
  return `${ms} 毫秒`;
};

export const formatRefresh = (seconds: number): string => {
  if (seconds < 3600) return `${Math.round(seconds / 60)} 分钟`;
  if (seconds % 3600 === 0) return `${seconds / 3600} 小时`;
  return `${(seconds / 3600).toFixed(1)} 小时`;
};

export const formatNextFetch = (value: string | null | undefined): string => {
  if (!value) return '—';
  const date = new Date(value);
  if (!isValidDate(date)) return '—';
  if (date.getTime() <= Date.now()) return '即将抓取';
  return `今天 ${formatTime(date)}`;
};

export const formatExpiry = (value: string | null | undefined): string => {
  if (!value) return '—';
  const date = new Date(value);
  if (!isValidDate(date)) return '—';
  return `${date.getMonth() + 1} 月 ${date.getDate()} 日 ${formatTime(date)}`;
};

export const minutesBetween = (from: string | null | undefined, to = new Date()): number => {
  if (!from) return 0;
  const date = new Date(from);
  if (!isValidDate(date)) return 0;
  return Math.max(0, Math.floor((to.getTime() - date.getTime()) / 60_000));
};
