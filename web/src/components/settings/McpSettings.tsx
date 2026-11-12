import { useCallback, useEffect, useState } from 'react';
import { api } from '@/api/client';
import type { MCPToken, MCPTokenCreated } from '@/api/types';
import { Button, IconButton, Switch } from '@/components/ui';
import { RowGroup, Row } from '@/components/settings/SettingsSection';
import { useSettingsStore } from '@/stores/settingsStore';
import { useUiStore } from '@/stores/uiStore';
import { copyText } from '@/utils/clipboard';
import { formatDateTime } from '@/utils/date';

// REQ-038（DEC-029）：MCP 设置。外部 Agent 通过 /mcp + Bearer Token 访问；
// 完整令牌只在创建时展示一次，服务端只保存哈希与打码值。
export const McpSettings = () => {
  const settings = useSettingsStore((state) => state.settings);
  const update = useSettingsStore((state) => state.update);
  const notify = useUiStore((state) => state.notify);
  const [tokens, setTokens] = useState<MCPToken[]>([]);
  const [name, setName] = useState('');
  const [created, setCreated] = useState<MCPTokenCreated | null>(null);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);

  const loadTokens = useCallback(async () => {
    try {
      setTokens(await api.getMcpTokens());
    } catch (error) {
      notify(error instanceof Error ? error.message : '读取 MCP 令牌失败');
    } finally {
      setLoading(false);
    }
  }, [notify]);

  useEffect(() => {
    void loadTokens();
  }, [loadTokens]);

  const endpoint = `${window.location.origin}${settings.mcp.endpoint}`;

  const copy = async (value: string, message: string) => {
    try {
      await copyText(value);
      notify(message);
    } catch {
      notify('复制失败，请手动选择');
    }
  };

  const create = async () => {
    const trimmed = name.trim();
    if (!trimmed) {
      notify('请先填写令牌名称');
      return;
    }
    setBusy(true);
    try {
      const token = await api.createMcpToken(trimmed);
      setCreated(token);
      setName('');
      await loadTokens();
    } catch (error) {
      notify(error instanceof Error ? error.message : '创建令牌失败');
    } finally {
      setBusy(false);
    }
  };

  const toggleToken = async (token: MCPToken, enabled: boolean) => {
    try {
      await api.updateMcpToken(token.id, { enabled });
      await loadTokens();
    } catch (error) {
      notify(error instanceof Error ? error.message : '更新令牌失败');
    }
  };

  const revoke = async (token: MCPToken) => {
    try {
      await api.deleteMcpToken(token.id);
      if (created?.id === token.id) setCreated(null);
      await loadTokens();
      notify('令牌已吊销');
    } catch (error) {
      notify(error instanceof Error ? error.message : '吊销令牌失败');
    }
  };

  return (
    <>
      <RowGroup title="MCP 服务">
        <Row
          label="启用 MCP"
          hint="开启后，外部 Agent 可通过 Bearer Token 读取和修改 Rill 内容；不提供删除工具。"
          control={(
            <Switch
              checked={settings.mcp.enabled}
              onChange={(enabled) => update({ mcp: { enabled } })}
              label="启用 MCP"
            />
          )}
        />
        <Row
          label="接入地址"
          hint={settings.mcp.enabled ? '请求头：Authorization: Bearer <token>' : '启用后外部 Agent 才能连接。'}
          control={(
            <div className="st-inline">
              <code className="st-mcp-endpoint">{endpoint}</code>
              <IconButton icon="copy" label="复制 MCP 地址" size={15} onClick={() => { void copy(endpoint, 'MCP 地址已复制'); }} />
            </div>
          )}
        />
      </RowGroup>

      <RowGroup title="访问令牌">
        <Row
          label="新建令牌"
          hint="令牌只在创建时完整显示一次，请立即复制保存。"
          control={(
            <Button size="sm" variant="primary" icon="plus" disabled={busy} onClick={() => { void create(); }}>
              {busy ? '生成中' : '生成令牌'}
            </Button>
          )}
        >
          <div className="st-inline st-mcp-create">
            <input
              value={name}
              maxLength={100}
              placeholder="令牌名称，例如 Claude Desktop"
              aria-label="MCP 令牌名称"
              onChange={(event) => setName(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === 'Enter') {
                  event.preventDefault();
                  void create();
                }
              }}
            />
          </div>
        </Row>

        {created ? (
          <Row label="新令牌" hint="关闭后无法再次查看，只能重新生成或吊销。">
            <div className="st-token">
              <code>{created.token}</code>
              <IconButton icon="copy" label="复制新令牌" size={15} onClick={() => { void copy(created.token, '令牌已复制'); }} />
              <Button size="sm" variant="ghost" onClick={() => setCreated(null)}>我已保存</Button>
            </div>
          </Row>
        ) : null}

        {loading ? (
          <Row label="令牌" control={<span className="st-dim">正在读取…</span>} />
        ) : tokens.length === 0 ? (
          <Row label="令牌" control={<span className="st-dim">还没有令牌</span>} />
        ) : tokens.map((token) => (
          <Row
            key={token.id}
            label={token.name}
            hint={`${token.masked}${token.last_used_at ? ` · 最近使用 ${formatDateTime(token.last_used_at)}` : ' · 尚未使用'}`}
            control={(
              <div className="st-inline">
                <Switch
                  checked={token.enabled}
                  onChange={(enabled) => { void toggleToken(token, enabled); }}
                  label={`${token.name} 启用状态`}
                />
                <IconButton icon="trash" label={`吊销 ${token.name}`} size={15} onClick={() => { void revoke(token); }} />
              </div>
            )}
          />
        ))}
      </RowGroup>

      <RowGroup title="可用能力">
        <Row className="st-facts-row">
          <ul className="st-facts st-facts-icons">
            <li><strong>读取</strong>分组、订阅源、文章、Markdown 正文、搜索与统计。</li>
            <li><strong>写入</strong>文章已读/收藏/隐藏、订阅源启停与分组、过滤规则、定时报告与推送渠道。</li>
            <li><strong>边界</strong>不提供删除工具，不暴露 AI 密钥、设置修改与备份恢复。</li>
            <li><strong>传输</strong>Streamable HTTP（SSE），Bearer Token 每次请求校验。</li>
          </ul>
        </Row>
      </RowGroup>
    </>
  );
};
