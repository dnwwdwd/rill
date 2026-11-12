import type { Folder, Source, SourceStatus, SourceView } from '@/api/types';

export const deriveSourceStatus = (source: Source): SourceStatus => {
  if (!source.enabled) return 'paused';
  // 只有真正的登录失效才算 auth；-2041 等风控/频控错误按普通失败展示，
  // 避免出现误导性的「登录态已过期 / 需重新扫码」提示。
  const authExpired = source.kind === 'wechat'
    && source.last_status === 'error'
    && Boolean(source.last_error && /401|登录态已失效|登录态已过期/.test(source.last_error));
  if (authExpired) return 'auth';
  // 最近一次抓取失败就必须如实反映——绝不能在失败时显示「正常」，
  // 否则会出现「状态正常但内容已停止同步」的误导。
  if (source.last_status === 'error') return 'error';
  if (source.fail_count >= 3) return 'error';
  return 'ok';
};

export const toSourceView = (source: Source, folders: Folder[]): SourceView => ({
  ...source,
  folder_name: folders.find((folder) => folder.id === source.folder_id)?.name ?? '未分组',
  status: deriveSourceStatus(source),
});

export const isAttention = (status: SourceStatus): boolean => status === 'warn' || status === 'auth' || status === 'error';

export const statusLabel = (status: SourceStatus): string => {
  switch (status) {
    case 'ok': return '正常';
    case 'warn': return '可能停更';
    case 'auth': return '需重新扫码';
    case 'error': return '连续失败';
    case 'paused': return '已暂停';
  }
};

export const statusDescription = (source: SourceView): string => {
  switch (source.status) {
    case 'auth': return source.last_error ?? '登录态已过期，需要重新扫码';
    case 'warn': return '超过 2× 刷新间隔没有新条目';
    case 'error': return source.last_error ?? '连续抓取失败';
    case 'paused': return '已暂停调度';
    default: return '抓取正常';
  }
};
