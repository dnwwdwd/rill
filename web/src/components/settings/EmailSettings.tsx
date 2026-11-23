import { useState } from 'react';
import { api } from '@/api/client';
import { ApiError, type EmailAccount } from '@/api/types';
import { Button, Field, IconButton, Switch } from '@/components/ui';
import { RowGroup, Row } from '@/components/settings/SettingsSection';
import { useSettingsStore } from '@/stores/settingsStore';
import { useSourcesStore } from '@/stores/sourcesStore';
import { useUiStore } from '@/stores/uiStore';
import { flattenFolders, folderOptionLabel } from '@/utils/folders';
import { cx } from '@/utils/cx';

type TestState = 'idle' | 'testing' | 'ok' | 'error';

const PORT_RANGE = { min: 1, max: 65535 } as const;
const POLL_RANGE = { min: 5, max: 1440 } as const;
const SYNC_LIMITS = [10, 20, 30, 60, 100, 200, 500] as const;

// 常见厂商预设：选择即填好服务器/端口，hint 给出授权码获取路径。
const EMAIL_PRESETS = [
  {
    key: 'qq',
    label: 'QQ 邮箱 / Foxmail',
    host: 'imap.qq.com',
    port: 993,
    hint: 'QQ 邮箱网页版 → 设置 → 账户 → 开启「IMAP/SMTP 服务」，按提示发短信获取 16 位授权码',
  },
  {
    key: '163',
    label: '网易 163 / 126',
    host: 'imap.163.com',
    port: 993,
    hint: '163 网页版 → 设置 → POP3/SMTP/IMAP → 开启服务并生成「客户端授权密码」',
  },
  {
    key: 'gmail',
    label: 'Gmail',
    host: 'imap.gmail.com',
    port: 993,
    hint: 'Google 账号 → 安全性 → 两步验证 → 应用专用密码（16 位）；需微服能直连 Google',
  },
  {
    key: 'outlook',
    label: 'Outlook / Hotmail',
    host: 'outlook.office365.com',
    port: 993,
    hint: '微软账户 → 安全性 → 开启双重验证 → 创建「应用密码」',
  },
  {
    key: 'exmail',
    label: '腾讯企业邮箱',
    host: 'imap.exmail.qq.com',
    port: 993,
    hint: '企业管理后台或成员邮箱设置 → 客户端专用密码',
  },
  {
    key: 'aliyun',
    label: '阿里企业邮箱',
    host: 'imap.mxhichina.com',
    port: 993,
    hint: '邮箱设置 → 收发信客户端 → 开启 IMAP 并使用登录密码或专用密码',
  },
  {
    key: 'custom',
    label: '自定义 / 其他',
    host: '',
    port: 993,
    hint: '',
  },
] as const;

const newAccount = (): EmailAccount => ({
  id: typeof crypto !== 'undefined' && 'randomUUID' in crypto
    ? `mail-${crypto.randomUUID()}`
    : `mail-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`,
  host: '',
  port: 993,
  username: '',
  poll_interval_minutes: 15,
  sync_limit: 60,
  folder_id: null,
  password_set: false,
});

const clamp = (value: number, min: number, max: number) => Math.max(min, Math.min(max, value));

/**
 * REQ-041（F-57）：邮箱自动同步 —— 每个 IMAP 账号对应一个订阅源。
 * 支持多个邮箱、单轮同步数量、按发件人放行远程图片；授权码只写不读。
 */
export const EmailSettings = () => {
  const settings = useSettingsStore((state) => state.settings);
  const update = useSettingsStore((state) => state.update);
  const saveNow = useSettingsStore((state) => state.saveNow);
  const saveEmailPassword = useSettingsStore((state) => state.saveEmailPassword);
  const folders = useSourcesStore((state) => state.folders);
  const createFolder = useSourcesStore((state) => state.createFolder);
  const notify = useUiStore((state) => state.notify);
  const email = settings.email;

  const [passwordDrafts, setPasswordDrafts] = useState<Record<string, string>>({});
  const [passwordBusy, setPasswordBusy] = useState<Record<string, boolean>>({});
  const [testStates, setTestStates] = useState<Record<string, TestState>>({});
  const [testErrors, setTestErrors] = useState<Record<string, string>>({});
  const [creatingFolderFor, setCreatingFolderFor] = useState<string | null>(null);
  const [newFolderName, setNewFolderName] = useState('');
  const [folderBusy, setFolderBusy] = useState(false);

  const folderNodes = flattenFolders(folders);

  const patchAccount = (id: string, patch: Partial<EmailAccount>) => {
    update({
      email: {
        accounts: email.accounts.map((account) => (
          account.id === id ? { ...account, ...patch } : account
        )),
      },
    });
  };

  const addAccount = () => {
    update({ email: { accounts: [...email.accounts, newAccount()] } });
  };

  const removeAccount = (id: string) => {
    update({ email: { accounts: email.accounts.filter((account) => account.id !== id) } });
    setPasswordDrafts((current) => {
      const next = { ...current };
      delete next[id];
      return next;
    });
  };

  const savePassword = async (account: EmailAccount) => {
    const value = (passwordDrafts[account.id] ?? '').trim();
    if (!value) {
      notify('请输入新的授权码');
      return;
    }
    setPasswordBusy((current) => ({ ...current, [account.id]: true }));
    // 新账号可能还没落库，先 flush 账号列表，避免授权码指向不存在的账号。
    await saveNow();
    const saved = await saveEmailPassword(account.id, value);
    setPasswordBusy((current) => ({ ...current, [account.id]: false }));
    if (saved) {
      setPasswordDrafts((current) => ({ ...current, [account.id]: '' }));
    }
  };

  const clearPassword = async (account: EmailAccount) => {
    setPasswordBusy((current) => ({ ...current, [account.id]: true }));
    const saved = await saveEmailPassword(account.id, null);
    setPasswordBusy((current) => ({ ...current, [account.id]: false }));
    if (saved) {
      setPasswordDrafts((current) => ({ ...current, [account.id]: '' }));
    }
  };

  const submitNewFolder = async (account: EmailAccount) => {
    const name = newFolderName.trim();
    if (!name || folderBusy) return;
    setFolderBusy(true);
    const folder = await createFolder(name, null);
    setFolderBusy(false);
    if (folder) {
      patchAccount(account.id, { folder_id: folder.id });
      setCreatingFolderFor(null);
      setNewFolderName('');
    }
  };

  const test = async (account: EmailAccount) => {
    if (!account.host.trim() || !account.username.trim()) {
      setTestStates((current) => ({ ...current, [account.id]: 'error' }));
      setTestErrors((current) => ({ ...current, [account.id]: '请先填写 IMAP 服务器与邮箱账号' }));
      return;
    }
    setTestStates((current) => ({ ...current, [account.id]: 'testing' }));
    setTestErrors((current) => {
      const next = { ...current };
      delete next[account.id];
      return next;
    });
    const draftPassword = (passwordDrafts[account.id] ?? '').trim();
    try {
      await saveNow();
      const result = await api.testEmailConnection({
        account_id: account.id,
        host: account.host.trim(),
        port: account.port,
        username: account.username.trim(),
        ...(draftPassword ? { password: draftPassword } : {}),
      });
      if (draftPassword) {
        const saved = await saveEmailPassword(account.id, draftPassword);
        if (saved) setPasswordDrafts((current) => ({ ...current, [account.id]: '' }));
      }
      await useSourcesStore.getState().load(true);
      notify(`连接成功 · 可读取 ${result.mailboxes?.length ?? 0} 个文件夹，将自动同步`);
      setTestStates((current) => ({ ...current, [account.id]: 'idle' }));
    } catch (cause) {
      setTestStates((current) => ({ ...current, [account.id]: 'error' }));
      if (cause instanceof DOMException && (cause.name === 'TimeoutError' || cause.name === 'AbortError')) {
        setTestErrors((current) => ({ ...current, [account.id]: '测试超时：服务器无响应，请检查地址、端口与网络' }));
      } else {
        setTestErrors((current) => ({
          ...current,
          [account.id]: cause instanceof ApiError ? cause.message : '连接失败，请检查服务器、账号与授权码',
        }));
      }
    }
  };

  return (
    <>
      <RowGroup title="邮箱同步">
        <Row
          label="启用邮箱同步"
          hint="每个账号同步为一个订阅源；同步收件箱与自定义文件夹，不改动邮箱里的已读状态。"
          control={(
            <Switch
              checked={email.enabled}
              onChange={(enabled) => update({ email: { enabled } })}
              label="启用邮箱同步"
            />
          )}
        />
        <Row
          label="拦截远程图片"
          hint="默认拦截邮件里的远程图片；阅读时可对单封邮件或该发件人放行。"
          control={(
            <Switch
              checked={email.block_remote_images}
              onChange={(block_remote_images) => update({ email: { block_remote_images } })}
              label="拦截远程图片"
            />
          )}
        />
      </RowGroup>

      <RowGroup title="邮箱账号" className={cx(!email.enabled && 'is-disabled')}>
        {email.accounts.length === 0 ? (
          <Row label="还没有邮箱账号" hint="添加后填写服务器与授权码，测试连接成功即开始同步。" />
        ) : null}
        {email.accounts.map((account, index) => {
          const activePreset = EMAIL_PRESETS.find(
            (preset) => preset.key !== 'custom' && preset.host === account.host.trim() && preset.port === account.port,
          );
          const testState = testStates[account.id] ?? 'idle';
          const testError = testErrors[account.id];
          const creatingFolder = creatingFolderFor === account.id;
          return (
            <div className="email-account" key={account.id}>
              <div className="email-account-head">
                <span className="email-account-name">{account.username.trim() || `邮箱 ${index + 1}`}</span>
                <IconButton
                  icon="trash"
                  label="移除邮箱账号"
                  size={16}
                  onClick={() => removeAccount(account.id)}
                />
              </div>
              <Row label="常见邮箱" hint={activePreset?.hint || undefined}>
                <select
                  value={activePreset?.key ?? 'custom'}
                  aria-label={`邮箱 ${index + 1} 预设`}
                  onChange={(event) => {
                    const preset = EMAIL_PRESETS.find((item) => item.key === event.target.value);
                    if (!preset) return;
                    patchAccount(account.id, { host: preset.host, port: preset.port });
                  }}
                >
                  {EMAIL_PRESETS.map((preset) => (
                    <option key={preset.key} value={preset.key}>{preset.label}</option>
                  ))}
                </select>
              </Row>
              <Row label="IMAP 服务器">
                <input
                  value={account.host}
                  placeholder="imap.example.com"
                  aria-label={`邮箱 ${index + 1} IMAP 服务器`}
                  onChange={(event) => patchAccount(account.id, { host: event.target.value })}
                />
              </Row>
              <Row label="端口">
                <input
                  type="number"
                  min={PORT_RANGE.min}
                  max={PORT_RANGE.max}
                  value={account.port}
                  aria-label={`邮箱 ${index + 1} 端口`}
                  onChange={(event) => patchAccount(account.id, {
                    port: clamp(Number(event.target.value) || 993, PORT_RANGE.min, PORT_RANGE.max),
                  })}
                />
              </Row>
              <Row label="邮箱账号">
                <input
                  value={account.username}
                  placeholder="name@example.com"
                  aria-label={`邮箱 ${index + 1} 账号`}
                  onChange={(event) => patchAccount(account.id, { username: event.target.value })}
                />
              </Row>
              <Row label="授权码" hint={account.password_set ? '已加密保存，填写可覆盖' : undefined}>
                <div className="st-secret">
                  <input
                    type="password"
                    value={passwordDrafts[account.id] ?? ''}
                    placeholder={account.password_set ? '已配置，填写可覆盖' : '输入授权码'}
                    aria-label={`邮箱 ${index + 1} 授权码`}
                    onChange={(event) => setPasswordDrafts((current) => ({
                      ...current,
                      [account.id]: event.target.value,
                    }))}
                  />
                  <IconButton
                    icon="key"
                    label="保存授权码"
                    size={16}
                    disabled={!passwordDrafts[account.id]?.trim() || Boolean(passwordBusy[account.id])}
                    onClick={() => { void savePassword(account); }}
                  />
                  {account.password_set ? (
                    <IconButton
                      icon="trash"
                      label="清除授权码"
                      size={16}
                      disabled={Boolean(passwordBusy[account.id])}
                      onClick={() => { void clearPassword(account); }}
                    />
                  ) : null}
                </div>
              </Row>
              <Row
                label="连接测试"
                control={(
                  <div className="st-inline">
                    <Button
                      size="sm"
                      icon="refresh"
                      disabled={testState === 'testing'}
                      onClick={() => { void test(account); }}
                    >
                      {testState === 'testing' ? '正在测试' : '测试连接'}
                    </Button>
                    {testState === 'error' && testError ? <span className="form-error">{testError}</span> : null}
                  </div>
                )}
              />
              <Row label="单次同步数量" hint="跨所有文件夹的总上限；首次同步也按此数量建立基线。">
                <select
                  value={account.sync_limit}
                  aria-label={`邮箱 ${index + 1} 同步数量`}
                  onChange={(event) => patchAccount(account.id, { sync_limit: Number(event.target.value) })}
                >
                  {SYNC_LIMITS.map((limit) => (
                    <option key={limit} value={limit}>{limit} 封</option>
                  ))}
                </select>
              </Row>
              <Row label="轮询间隔（分钟）">
                <input
                  type="number"
                  min={POLL_RANGE.min}
                  max={POLL_RANGE.max}
                  value={account.poll_interval_minutes}
                  aria-label={`邮箱 ${index + 1} 轮询间隔`}
                  onChange={(event) => patchAccount(account.id, {
                    poll_interval_minutes: clamp(Number(event.target.value) || 15, POLL_RANGE.min, POLL_RANGE.max),
                  })}
                />
              </Row>
              <Row label="归入分组" hint="可选；不选则放在未分组">
                {creatingFolder ? (
                  <div className="st-inline st-inline-wide">
                    <Field label="新分组名称">
                      <input
                        value={newFolderName}
                        onChange={(event) => setNewFolderName(event.target.value)}
                        onKeyDown={(event) => {
                          if (event.key === 'Enter') {
                            event.preventDefault();
                            void submitNewFolder(account);
                          }
                        }}
                        placeholder="输入分组名称"
                        aria-label={`邮箱 ${index + 1} 新分组名称`}
                        autoFocus
                      />
                    </Field>
                    <Button
                      size="sm"
                      variant="primary"
                      icon="check"
                      disabled={!newFolderName.trim() || folderBusy}
                      onClick={() => { void submitNewFolder(account); }}
                    >
                      {folderBusy ? '创建中' : '创建'}
                    </Button>
                    <Button size="sm" onClick={() => { setCreatingFolderFor(null); setNewFolderName(''); }}>取消</Button>
                  </div>
                ) : (
                  <div className="st-inline st-inline-wide">
                    <select
                      value={account.folder_id ?? ''}
                      aria-label={`邮箱 ${index + 1} 归入分组`}
                      onChange={(event) => patchAccount(account.id, {
                        folder_id: event.target.value ? Number(event.target.value) : null,
                      })}
                    >
                      <option value="">不分组</option>
                      {folderNodes.map((node) => (
                        <option key={node.folder.id} value={node.folder.id}>{folderOptionLabel(node)}</option>
                      ))}
                    </select>
                    <Button size="sm" icon="plus" onClick={() => setCreatingFolderFor(account.id)}>新建分组</Button>
                  </div>
                )}
              </Row>
            </div>
          );
        })}
        <div className="st-row st-row-bare email-account-add">
          <Button size="sm" icon="plus" onClick={addAccount}>添加邮箱</Button>
        </div>
      </RowGroup>
    </>
  );
};
