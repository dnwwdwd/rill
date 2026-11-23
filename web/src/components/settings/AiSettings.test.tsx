import { fireEvent, render, screen } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { AiSettings } from '@/components/settings/AiSettings';
import { DEFAULT_AI_PROMPTS } from '@/utils/aiPrompts';

const mocks = vi.hoisted(() => ({
  update: vi.fn(),
  notify: vi.fn(),
  settings: {
    ai: {
      enabled: true,
      base_url: 'https://api.example.com/v1',
      model: 'fake-model',
      api_key_set: true,
      summary_style: 'bullets',
      target_lang: 'zh-CN',
      daily_limit: 200,
      used_today: 0,
      tokens_month: 0,
      consent: true,
      prompts: {
        summary_bullets: '自定义要点指令',
        summary_one: '',
        digest_batch: '',
        digest_merge: '',
      },
      prompt_defaults: {
        summary_bullets: '',
        summary_one: '',
        digest_batch: '',
        digest_merge: '',
      },
    },
  },
}));

vi.mock('@/stores/settingsStore', () => ({
  useSettingsStore: (selector: (state: unknown) => unknown) => selector({
    settings: mocks.settings,
    update: mocks.update,
    saveAiKey: vi.fn(),
    testAiConnection: vi.fn(),
  }),
}));

vi.mock('@/stores/uiStore', () => ({
  useUiStore: (selector: (state: unknown) => unknown) => selector({ notify: mocks.notify }),
}));

describe('AiSettings prompts', () => {
  beforeEach(() => {
    mocks.update.mockClear();
    mocks.settings.ai.prompt_defaults = DEFAULT_AI_PROMPTS;
  });

  it('shows defaults and updates a prompt', () => {
    render(<AiSettings />);

    const field = screen.getByLabelText('要点列表提示词');
    expect(field).toHaveValue('自定义要点指令');
    expect(screen.getByText('已自定义')).toBeInTheDocument();

    fireEvent.change(field, { target: { value: '新的要点指令' } });
    expect(mocks.update).toHaveBeenCalledWith({
      ai: { prompts: { summary_bullets: '新的要点指令' } },
    });

    fireEvent.click(screen.getAllByRole('button', { name: '查看默认' })[0]);
    expect(screen.getByText(DEFAULT_AI_PROMPTS.summary_bullets)).toBeInTheDocument();
  });

  it('resets a prompt to the system default', () => {
    render(<AiSettings />);

    fireEvent.click(screen.getAllByRole('button', { name: '恢复默认' })[0]);
    expect(mocks.update).toHaveBeenCalledWith({
      ai: { prompts: { summary_bullets: '' } },
    });
  });
});
