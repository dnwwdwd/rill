import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { api } from '@/api/client';
import { ApiError } from '@/api/types';
import { EmailSettings } from '@/components/settings/EmailSettings';
import { useSettingsStore } from '@/stores/settingsStore';
import { useSourcesStore } from '@/stores/sourcesStore';
import { useUiStore } from '@/stores/uiStore';
import { DEFAULT_SETTINGS } from '@/stores/defaults';

vi.mock('@/api/client', () => ({
  api: {
    saveEmailPassword: vi.fn(),
    testEmailConnection: vi.fn(),
  },
}));

const account = {
  id: 'mail-test-account',
  host: 'imap.example.com',
  port: 993,
  username: 'me@example.com',
  poll_interval_minutes: 15,
  sync_limit: 60,
  folder_id: null,
  password_set: false,
};

const emailBase = {
  enabled: true,
  block_remote_images: true,
  trusted_senders: [],
  accounts: [account],
};

const folder = {
  id: 3,
  name: '资讯',
  sort: 0,
  parent_id: null,
  source_count: 2,
};

const createdFolder = {
  id: 9,
  name: '收件箱',
  sort: 1,
  parent_id: null,
  source_count: 0,
};

const updateSpy = vi.fn();
const saveNowSpy = vi.fn().mockResolvedValue(undefined);
const loadSourcesSpy = vi.fn().mockResolvedValue(undefined);
const createFolderSpy = vi.fn().mockResolvedValue(createdFolder);

const lastAccounts = () => {
  const calls = updateSpy.mock.calls.filter(([patch]) => patch?.email?.accounts);
  expect(calls.length).toBeGreaterThan(0);
  return calls[calls.length - 1][0].email.accounts as typeof emailBase.accounts;
};

beforeEach(() => {
  updateSpy.mockClear();
  saveNowSpy.mockClear();
  loadSourcesSpy.mockClear();
  createFolderSpy.mockClear();
  vi.mocked(api.testEmailConnection).mockReset();
  vi.mocked(api.saveEmailPassword).mockReset();
  useSettingsStore.setState({
    settings: { ...DEFAULT_SETTINGS, email: { ...emailBase } },
    update: updateSpy,
    saveNow: saveNowSpy,
    saving: false,
    error: null,
  });
  useSourcesStore.setState({ folders: [folder], load: loadSourcesSpy, createFolder: createFolderSpy });
});

describe('EmailSettings', () => {
  it('renders every saved account with sync limits and no folder filter', () => {
    render(<EmailSettings />);

    expect(screen.getByRole('switch', { name: '启用邮箱同步' })).toBeChecked();
    expect(screen.getByRole('switch', { name: '拦截远程图片' })).toBeChecked();
    expect(screen.getByLabelText('邮箱 1 IMAP 服务器')).toHaveValue('imap.example.com');
    expect(screen.getByLabelText('邮箱 1 端口')).toHaveValue(993);
    expect(screen.getByLabelText('邮箱 1 账号')).toHaveValue('me@example.com');
    expect(screen.queryByLabelText('文件夹')).not.toBeInTheDocument();
    expect(screen.getByLabelText('邮箱 1 轮询间隔')).toHaveValue(15);
    expect(screen.getByLabelText('邮箱 1 同步数量')).toHaveValue('60');
    expect(screen.getByLabelText('邮箱 1 授权码')).toHaveDisplayValue('');
    expect(screen.getByLabelText('邮箱 1 归入分组')).toHaveValue('');
    expect(screen.getByRole('option', { name: '资讯' })).toBeInTheDocument();
  });

  it('adds more accounts instead of replacing the only one', () => {
    render(<EmailSettings />);

    fireEvent.click(screen.getByRole('button', { name: '添加邮箱' }));

    const accounts = lastAccounts();
    expect(accounts).toHaveLength(2);
    expect(accounts[0].id).toBe(account.id);
    expect(accounts[1].id).toMatch(/^mail-/);
    expect(accounts[1].sync_limit).toBe(60);
  });

  it('sends edited fields as account patches', () => {
    render(<EmailSettings />);

    fireEvent.change(screen.getByLabelText('邮箱 1 IMAP 服务器'), { target: { value: 'imap.fastmail.com' } });
    expect(lastAccounts()[0].host).toBe('imap.fastmail.com');

    fireEvent.change(screen.getByLabelText('邮箱 1 归入分组'), { target: { value: '3' } });
    expect(lastAccounts()[0].folder_id).toBe(3);

    fireEvent.change(screen.getByLabelText('邮箱 1 同步数量'), { target: { value: '200' } });
    expect(lastAccounts()[0].sync_limit).toBe(200);
  });

  it('creates a new group and selects it for that account', async () => {
    render(<EmailSettings />);

    fireEvent.click(screen.getByRole('button', { name: '新建分组' }));
    fireEvent.change(screen.getByLabelText('邮箱 1 新分组名称'), { target: { value: '收件箱' } });
    fireEvent.click(screen.getByRole('button', { name: '创建' }));

    await waitFor(() => expect(createFolderSpy).toHaveBeenCalledWith('收件箱', null));
    expect(lastAccounts()[0].folder_id).toBe(createdFolder.id);
  });

  it('fills provider presets and shows the authorization-code hint', () => {
    // 该用例让 update 真正合并进 store，才能驱动受控的预设下拉与 hint。
    updateSpy.mockImplementation((patch: { email?: { accounts?: typeof emailBase.accounts } }) => {
      useSettingsStore.setState((state) => ({
        settings: {
          ...state.settings,
          email: {
            ...state.settings.email,
            accounts: patch.email?.accounts ?? state.settings.email.accounts,
          },
        },
      }));
    });
    render(<EmailSettings />);

    // host 与预设不匹配 → 默认「自定义 / 其他」，无 hint。
    expect(screen.getByLabelText('邮箱 1 预设')).toHaveValue('custom');

    // 选 QQ 邮箱 → 自动填好服务器与端口。
    fireEvent.change(screen.getByLabelText('邮箱 1 预设'), { target: { value: 'qq' } });
    expect(lastAccounts()[0].host).toBe('imap.qq.com');
    expect(screen.getByLabelText('邮箱 1 IMAP 服务器')).toHaveValue('imap.qq.com');

    // 选 Gmail → 不同服务器 + 对应授权码指引。
    fireEvent.change(screen.getByLabelText('邮箱 1 预设'), { target: { value: 'gmail' } });
    expect(lastAccounts()[0].host).toBe('imap.gmail.com');
    expect(screen.getByLabelText('邮箱 1 IMAP 服务器')).toHaveValue('imap.gmail.com');
    expect(screen.getByText(/应用专用密码/)).toBeInTheDocument();
  });

  it('saves a new password for the selected account', async () => {
    vi.mocked(api.saveEmailPassword).mockResolvedValue({
      ...DEFAULT_SETTINGS,
      email: { ...emailBase, accounts: [{ ...account, password_set: true }] },
    });
    render(<EmailSettings />);

    fireEvent.change(screen.getByLabelText('邮箱 1 授权码'), { target: { value: 'mock-app-password' } });
    fireEvent.click(screen.getByRole('button', { name: '保存授权码' }));

    await waitFor(() => expect(api.saveEmailPassword).toHaveBeenCalledWith(account.id, 'mock-app-password'));
    await waitFor(() => expect(useSettingsStore.getState().settings.email.accounts[0].password_set).toBe(true));
    expect(await screen.findByPlaceholderText('已配置，填写可覆盖')).toBeInTheDocument();
  });

  it('tests the connection with saved credentials when the password field is empty', async () => {
    vi.mocked(api.testEmailConnection).mockResolvedValue({ ok: true, mailboxes: ['INBOX', 'Newsletter', 'Archive'] });
    render(<EmailSettings />);

    fireEvent.click(screen.getByRole('button', { name: '测试连接' }));

    await waitFor(() => expect(api.testEmailConnection).toHaveBeenCalledWith({
      account_id: account.id,
      host: 'imap.example.com',
      port: 993,
      username: 'me@example.com',
    }));
    // 成功文案走全局提示语，不在设置页内联显示。
    await waitFor(() => expect(useUiStore.getState().toast?.message).toContain('连接成功'));
    expect(useUiStore.getState().toast?.message).toContain('3 个文件夹');
    expect(screen.queryByText(/连接成功/)).not.toBeInTheDocument();
    // 测试成功后立即落库设置并重载源列表，错误源不用等页面刷新才消失。
    expect(saveNowSpy).toHaveBeenCalled();
    expect(loadSourcesSpy).toHaveBeenCalledWith(true);
  });

  it('shows the backend message when the connection test fails', async () => {
    vi.mocked(api.testEmailConnection).mockRejectedValue(new ApiError(400, 'email_test_failed', '登录失败，请检查授权码'));
    render(<EmailSettings />);

    fireEvent.click(screen.getByRole('button', { name: '测试连接' }));

    expect(await screen.findByText('登录失败，请检查授权码')).toBeInTheDocument();
    expect(screen.queryByText(/连接成功/)).not.toBeInTheDocument();
  });

  it('removes an account from the patch', () => {
    render(<EmailSettings />);

    fireEvent.click(screen.getByRole('button', { name: '移除邮箱账号' }));
    expect(lastAccounts()).toEqual([]);
  });
});
