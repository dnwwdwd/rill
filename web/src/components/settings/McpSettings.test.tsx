import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { api } from '@/api/client';
import { McpSettings } from '@/components/settings/McpSettings';
import { useSettingsStore } from '@/stores/settingsStore';
import { DEFAULT_SETTINGS } from '@/stores/defaults';

vi.mock('@/api/client', () => ({
  api: {
    getMcpTokens: vi.fn(),
    createMcpToken: vi.fn(),
    updateMcpToken: vi.fn(),
    deleteMcpToken: vi.fn(),
  },
}));

const token = {
  id: 7,
  name: 'Claude Desktop',
  masked: 'rill_mcp_abcd…9f2e',
  enabled: true,
  created_at: '2026-11-11T01:00:00Z',
  last_used_at: null,
};

beforeEach(() => {
  vi.mocked(api.getMcpTokens).mockResolvedValue([token]);
  vi.mocked(api.createMcpToken).mockResolvedValue({
    ...token,
    id: 8,
    name: 'Codex',
    token: 'rill_mcp_full_once_9f2e',
  });
  useSettingsStore.setState({
    settings: { ...DEFAULT_SETTINGS, mcp: { enabled: true, endpoint: '/mcp' } },
  });
});

describe('McpSettings', () => {
  it('shows the endpoint, existing tokens and a one-time token after creation', async () => {
    render(<McpSettings />);

    expect(await screen.findByText('Claude Desktop')).toBeInTheDocument();
    expect(screen.getByText(`${window.location.origin}/mcp`)).toBeInTheDocument();
    expect(screen.getByRole('switch', { name: '启用 MCP' })).toBeChecked();

    fireEvent.change(screen.getByLabelText('MCP 令牌名称'), { target: { value: 'Codex' } });
    fireEvent.click(screen.getByRole('button', { name: /生成令牌/ }));

    await waitFor(() => expect(api.createMcpToken).toHaveBeenCalledWith('Codex'));
    expect(await screen.findByText('rill_mcp_full_once_9f2e')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '我已保存' })).toBeInTheDocument();
  });
});
