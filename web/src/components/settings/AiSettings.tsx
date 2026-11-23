import { useState } from 'react';
import {
  Button,
  IconButton,
  Segmented,
  Switch,
} from '@/components/ui';
import { RowGroup, Row } from '@/components/settings/SettingsSection';
import { useSettingsStore } from '@/stores/settingsStore';
import { useUiStore } from '@/stores/uiStore';
import { formatTokenCount } from '@/utils/date';
import { ApiError } from '@/api/types';
import { cx } from '@/utils/cx';

type TestState = 'idle' | 'testing' | 'ok' | 'error';

interface PromptFieldProps {
  label: string;
  value: string;
  defaultValue: string;
  disabled: boolean;
  onChange: (value: string) => void;
}

const PromptField = ({ label, value, defaultValue, disabled, onChange }: PromptFieldProps) => {
  const [showDefault, setShowDefault] = useState(false);
  return (
    <Row label={label}>
      <div className="st-prompt">
        <textarea
          value={value}
          onChange={(event) => onChange(event.target.value)}
          placeholder={defaultValue}
          aria-label={`${label}提示词`}
          maxLength={4000}
          rows={3}
          disabled={disabled}
        />
        <div className="st-prompt-actions">
          <button
            type="button"
            className="st-prompt-link"
            onClick={() => setShowDefault((current) => !current)}
          >
            {showDefault ? '收起默认' : '查看默认'}
          </button>
          <button
            type="button"
            className="st-prompt-link"
            disabled={!value}
            onClick={() => onChange('')}
          >
            恢复默认
          </button>
          {value ? <span className="st-dim">已自定义</span> : <span className="st-dim">留空使用默认</span>}
        </div>
        {showDefault ? <pre className="st-prompt-default">{defaultValue}</pre> : null}
      </div>
    </Row>
  );
};

export const AiSettings = () => {
  const settings = useSettingsStore((state) => state.settings);
  const update = useSettingsStore((state) => state.update);
  const saveAiKey = useSettingsStore((state) => state.saveAiKey);
  const testAiConnection = useSettingsStore((state) => state.testAiConnection);
  const notify = useUiStore((state) => state.notify);
  const [showKey, setShowKey] = useState(false);
  const [keyDraft, setKeyDraft] = useState('');
  const [savingKey, setSavingKey] = useState(false);
  const [testState, setTestState] = useState<TestState>('idle');
  const [latency, setLatency] = useState<number | null>(null);
  const [testError, setTestError] = useState<string | null>(null);
  const ai = settings.ai;

  const test = async () => {
    setTestState('testing');
    setLatency(null);
    setTestError(null);
    try {
      const result = await testAiConnection();
      setLatency(result.latency_ms);
      setTestState(result.ok ? 'ok' : 'error');
      if (!result.ok) setTestError('连接未成功');
    } catch (cause) {
      setTestState('error');
      // 后端返回的是具体原因（未启用/未确认外发/未配置/密钥无效/上游状态码），
      // 直接展示，不再折叠成一句通用提示。
      if (cause instanceof ApiError) {
        const details = cause.details as { upstream_status?: unknown } | undefined;
        const status = details && typeof details.upstream_status === 'number' ? details.upstream_status : null;
        setTestError(status ? `${cause.message}（上游状态 ${status}）` : cause.message);
      } else {
        setTestError('连接失败，请检查地址、密钥与模型');
      }
    }
  };

  const saveKey = async () => {
    if (!keyDraft.trim()) {
      notify('请输入新的接口密钥');
      return;
    }
    setSavingKey(true);
    const saved = await saveAiKey(keyDraft.trim());
    setSavingKey(false);
    if (saved) setKeyDraft('');
  };

  return (
    <>
      <RowGroup title="能力">
        <Row
          label="启用 AI 能力"
          control={(
            <Switch
              checked={ai.enabled}
              onChange={(value) => update({ ai: { enabled: value } })}
              label="启用 AI 能力"
            />
          )}
        />
      </RowGroup>

      <RowGroup title="连接" className={cx(!ai.enabled && 'is-disabled')}>
        <Row label="接口地址">
          <input
            value={ai.base_url}
            onChange={(event) => update({ ai: { base_url: event.target.value } })}
            aria-label="接口地址"
            disabled={!ai.enabled}
          />
        </Row>
        <Row label="模型">
          <input
            value={ai.model}
            onChange={(event) => update({ ai: { model: event.target.value } })}
            aria-label="模型"
            disabled={!ai.enabled}
          />
        </Row>
        <Row label="接口密钥" hint="AES-GCM 加密存储，仅替换时不回显">
          <div className="st-secret">
            <input
              type={showKey ? 'text' : 'password'}
              value={keyDraft}
              onChange={(event) => setKeyDraft(event.target.value)}
              placeholder={ai.api_key_set ? '已保存，输入新密钥可替换' : '输入接口密钥'}
              aria-label="接口密钥"
              disabled={!ai.enabled}
            />
            <IconButton
              icon={showKey ? 'eye-off' : 'eye'}
              label={showKey ? '隐藏密钥' : '显示密钥'}
              size={16}
              onClick={() => setShowKey((value) => !value)}
            />
          </div>
        </Row>
        <Row
          label="连接测试"
          control={(
            <div className="st-inline">
              <Button size="sm" icon="key" onClick={() => { void saveKey(); }} disabled={!ai.enabled || !keyDraft.trim() || savingKey}>
                {savingKey ? '正在保存' : '保存密钥'}
              </Button>
              <Button size="sm" icon="refresh" onClick={() => { void test(); }} disabled={!ai.enabled || testState === 'testing'}>
                {testState === 'testing' ? '正在测试' : '测试连接'}
              </Button>
              {testState === 'ok' ? <span className="st-ok-note">延迟 {latency ?? '—'} 毫秒</span> : null}
              {testState === 'error' && testError ? <span className="form-error">{testError}</span> : null}
            </div>
          )}
        />
      </RowGroup>

      <RowGroup title="摘要" className={cx(!ai.enabled && 'is-disabled')}>
        <Row
          label="默认样式"
          control={(
            <Segmented
              ariaLabel="默认摘要样式"
              value={ai.summary_style}
              onChange={(value) => update({ ai: { summary_style: value } })}
              options={[{ value: 'bullets', label: '要点列表' }, { value: 'one', label: '一句话' }]}
            />
          )}
        />
        <Row
          label="今日调用"
          hint={`本月累计 ${formatTokenCount(ai.tokens_month)} 令牌`}
          control={<strong className="st-meter-value">{ai.used_today} 次</strong>}
        />
      </RowGroup>

      <RowGroup title="提示词" className={cx(!ai.enabled && 'is-disabled')}>
        <PromptField
          label="要点列表"
          value={ai.prompts.summary_bullets}
          defaultValue={ai.prompt_defaults.summary_bullets}
          disabled={!ai.enabled}
          onChange={(value) => update({ ai: { prompts: { summary_bullets: value } } })}
        />
        <PromptField
          label="一句话"
          value={ai.prompts.summary_one}
          defaultValue={ai.prompt_defaults.summary_one}
          disabled={!ai.enabled}
          onChange={(value) => update({ ai: { prompts: { summary_one: value } } })}
        />
        <PromptField
          label="报告分批"
          value={ai.prompts.digest_batch}
          defaultValue={ai.prompt_defaults.digest_batch}
          disabled={!ai.enabled}
          onChange={(value) => update({ ai: { prompts: { digest_batch: value } } })}
        />
        <PromptField
          label="报告简报"
          value={ai.prompts.digest_merge}
          defaultValue={ai.prompt_defaults.digest_merge}
          disabled={!ai.enabled}
          onChange={(value) => update({ ai: { prompts: { digest_merge: value } } })}
        />
        <Row hint="关注源 AI 总结复用摘要与报告分批提示词；修改只影响之后生成的内容。" />
      </RowGroup>

      <RowGroup title="翻译" className={cx(!ai.enabled && 'is-disabled')}>
        <Row
          label="目标语言"
          control={(
            <select
              value={ai.target_lang}
              onChange={(event) => update({ ai: { target_lang: event.target.value } })}
              aria-label="翻译目标语言"
              disabled={!ai.enabled}
            >
              <option value="zh-CN">简体中文（默认）</option>
              <option value="zh-TW">繁體中文</option>
              <option value="en">English</option>
              <option value="ja">日本語</option>
            </select>
          )}
        />
      </RowGroup>
    </>
  );
};
