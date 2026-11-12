import { describe, expect, it } from 'vitest';
import { markdownFileName } from '@/utils/markdownExport';

describe('markdownFileName', () => {
  it('sanitizes filesystem characters and keeps a readable title', () => {
    expect(markdownFileName('A / B: "C" ?')).toBe('A - B- -C- -.md');
  });

  it('falls back to article for empty titles', () => {
    expect(markdownFileName('   ')).toBe('article.md');
  });
});
