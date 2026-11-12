import { api } from '@/api/client';

export const copyText = async (text: string): Promise<void> => {
  if (navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(text);
    return;
  }
  const textarea = document.createElement('textarea');
  textarea.value = text;
  textarea.style.position = 'fixed';
  textarea.style.opacity = '0';
  document.body.appendChild(textarea);
  textarea.select();
  document.execCommand('copy');
  textarea.remove();
};

export const copyFeedUrl = async (sourceId: number): Promise<string> => {
  const health = await api.getSourceHealth(sourceId);
  const url = new URL(health.feed_paths.rss, window.location.origin).toString();
  await copyText(url);
  return url;
};
