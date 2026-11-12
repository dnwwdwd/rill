import { beforeEach, describe, expect, it } from 'vitest';
import { waitFor } from '@testing-library/react';
import { mockApi } from '@/api/mock/mockApi';
import { useSettingsStore } from '@/stores/settingsStore';

describe('settingsStore', () => {
  beforeEach(async () => {
    window.sessionStorage.clear();
    await mockApi.signIn?.();
    useSettingsStore.getState().reset();
    await useSettingsStore.getState().load(true);
  });

  it('applies reader preferences before the debounced save completes', async () => {
    useSettingsStore.getState().update({ reader: { font_size: 21, line_height: 1.9 } });
    expect(useSettingsStore.getState().settings.reader.font_size).toBe(21);
    expect(document.documentElement.style.getPropertyValue('--reader-font-size')).toBe('21px');

    await waitFor(() => expect(useSettingsStore.getState().saving).toBe(false), { timeout: 2000 });
    expect(useSettingsStore.getState().settings.reader.line_height).toBe(1.9);
  });

  it('applies and persists the selected accent immediately', async () => {
    useSettingsStore.getState().update({ reader: { accent: 'purple' } });
    expect(document.documentElement.dataset.accent).toBe('purple');
    expect(window.localStorage.getItem('rill.accent')).toBe('purple');
    await waitFor(() => expect(useSettingsStore.getState().saving).toBe(false), { timeout: 2000 });
  });

  it('applies and persists resizable sidebar widths', async () => {
    useSettingsStore.getState().update({ reader: { rail_width: 240, list_width: 420 } });
    expect(document.documentElement.style.getPropertyValue('--rail-width')).toBe('240px');
    expect(document.documentElement.style.getPropertyValue('--list-width')).toBe('420px');
    expect(window.localStorage.getItem('rill.rail_width')).toBe('240');
    expect(window.localStorage.getItem('rill.list_width')).toBe('420');

    await waitFor(() => expect(useSettingsStore.getState().saving).toBe(false), { timeout: 2000 });
    const saved = await mockApi.getSettings();
    expect(saved.reader.rail_width).toBe(240);
    expect(saved.reader.list_width).toBe(420);

    useSettingsStore.getState().update({ reader: { rail_width: 224, list_width: 384 } });
    await waitFor(() => expect(useSettingsStore.getState().saving).toBe(false), { timeout: 2000 });
  });

  it('does not reset the global feed token when a source token changes', async () => {
    const before = useSettingsStore.getState().settings.feed.token_masked;
    await mockApi.resetFeedToken(1);
    expect(useSettingsStore.getState().settings.feed.token_masked).toBe(before);
  });
});
