import { useEffect } from 'react';
import type { Settings } from '@/api/types';
import { applySettingsToDocument } from '@/stores/settingsStore';

export const useAppTheme = (settings: Settings) => {
  useEffect(() => {
    const apply = () => {
      applySettingsToDocument(settings);
      const effective = document.documentElement.dataset.theme === 'dark' ? 'dark' : 'light';
      const meta = document.querySelector<HTMLMetaElement>('meta[name="theme-color"]');
      if (meta) meta.content = effective === 'dark' ? '#0f1210' : '#f3f3f0';
    };
    apply();
    if (typeof window.matchMedia !== 'function') return undefined;
    const media = window.matchMedia('(prefers-color-scheme: dark)');
    const onChange = () => {
      if (settings.reader.theme === 'system') apply();
    };
    media.addEventListener('change', onChange);
    return () => media.removeEventListener('change', onChange);
  }, [
    settings.reader.theme,
    settings.reader.accent,
    settings.reader.font_size,
    settings.reader.line_height,
    settings.reader.line_width,
  ]);
};
