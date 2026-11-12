// REQ-038：正文 Markdown 导出。下载使用 blob: 锚点，懒猫 file-chooser inject
// 会拦截带 download 属性的 blob 链接，让用户选择保存到本地或懒猫。
const sanitize = (title: string): string => {
  const name = title
    .replace(/[\\/:*?"<>|]/g, '-')
    .replace(/\s+/g, ' ')
    .trim()
    .replace(/^\.+|\.+$/g, '');
  return (name || 'article').slice(0, 80);
};

export const markdownFileName = (title: string): string => `${sanitize(title)}.md`;

export const downloadMarkdown = (title: string, content: string): void => {
  const blob = new Blob([content], { type: 'text/markdown;charset=utf-8' });
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement('a');
  anchor.href = url;
  anchor.download = markdownFileName(title);
  anchor.click();
  // 懒猫 file-chooser 会先弹选择框，用户确认后才 fetch blob；过早 revoke 会让保存失败。
  window.setTimeout(() => URL.revokeObjectURL(url), 60_000);
};
