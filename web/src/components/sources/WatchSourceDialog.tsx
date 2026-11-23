import { useEffect, useMemo, useState } from 'react';
import { api } from '@/api/client';
import type { PushChannel, Source, SourcePatch, WatchMode } from '@/api/types';
import { Button, Modal, Segmented, SourceIcon, Switch } from '@/components/ui';
import { useSourcesStore } from '@/stores/sourcesStore';
import { useUiStore } from '@/stores/uiStore';
import { formatRefresh } from '@/utils/date';
import { cx } from '@/utils/cx';

interface WatchSourceDialogProps {
  open: boolean;
  initialSourceId?: number | null;
  initialSourceIds?: number[];
  picker?: boolean;
  sources?: Source[];
  onClose: () => void;
  onSaved?: (sources: Source[]) => void;
}

const sourceKindLabel = (source: Source): string => {
  if (source.kind === 'wechat') return '公众号';
  if (source.kind === 'x') return 'X';
  return 'RSS';
};

const selectedNames = (sources: Source[]): string => {
  const names = sources.slice(0, 3).map((source) => source.name).join('、');
  return sources.length > 3 ? `${names} 等 ${sources.length} 个源` : names;
};

export const WatchSourceDialog = ({
  open,
  initialSourceId = null,
  initialSourceIds,
  picker = false,
  sources: sourceList,
  onClose,
  onSaved,
}: WatchSourceDialogProps) => {
  const storeSources = useSourcesStore((state) => state.sources);
  const sourceOptions = sourceList ?? storeSources;
  const [selectedIds, setSelectedIds] = useState<number[]>([]);
  const [query, setQuery] = useState('');
  const [watchEnabled, setWatchEnabled] = useState(false);
  const [watchMode, setWatchMode] = useState<WatchMode>('batch');
  const [watchAI, setWatchAI] = useState(false);
  const [watchChannelIDs, setWatchChannelIDs] = useState<number[]>([]);
  const [channels, setChannels] = useState<PushChannel[]>([]);
  const [channelsLoading, setChannelsLoading] = useState(false);
  const [channelsError, setChannelsError] = useState('');
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');

  const selectedSources = useMemo(
    () => sourceOptions.filter((source) => selectedIds.includes(source.id)),
    [sourceOptions, selectedIds],
  );
  const primarySource = selectedSources[0] ?? null;
  const enabledChannels = useMemo(
    () => channels.filter((channel) => channel.enabled),
    [channels],
  );
  const filteredOptions = useMemo(() => {
    const keyword = query.trim().toLowerCase();
    if (!keyword) return sourceOptions;
    return sourceOptions.filter((source) => source.name.toLowerCase().includes(keyword));
  }, [sourceOptions, query]);

  useEffect(() => {
    if (!open) return;
    const seed = initialSourceIds && initialSourceIds.length > 0
      ? initialSourceIds
      : initialSourceId !== null
        ? [initialSourceId]
        : [];
    setSelectedIds(seed);
    setQuery('');
    setWatchEnabled(picker);
    setWatchMode('batch');
    setWatchAI(false);
    setWatchChannelIDs([]);
    setError('');
    setSaving(false);
  }, [open, initialSourceId, initialSourceIds, picker]);

  useEffect(() => {
    if (!open || !primarySource) return;
    setWatchEnabled(picker ? true : Boolean(primarySource.watch_enabled));
    setWatchMode(primarySource.watch_mode === 'each' ? 'each' : 'batch');
    setWatchAI(Boolean(primarySource.watch_ai));
    setWatchChannelIDs(primarySource.watch_channel_ids ?? []);
    setError('');
  }, [open, picker, primarySource?.id]);

  useEffect(() => {
    if (!open) return;
    let active = true;
    setChannelsLoading(true);
    setChannelsError('');
    api.getChannels()
      .then((rows) => {
        if (active) setChannels(rows.filter((channel) => channel.enabled));
      })
      .catch(() => {
        if (active) {
          setChannels([]);
          setChannelsError('推送渠道加载失败，请稍后重试。');
        }
      })
      .finally(() => {
        if (active) setChannelsLoading(false);
      });
    return () => {
      active = false;
    };
  }, [open]);

  // 批量新增关注源时默认勾选全部启用渠道，避免用户误以为会推送却实际没有渠道。
  useEffect(() => {
    if (!open || !picker || enabledChannels.length === 0) return;
    setWatchChannelIDs((current) => (
      current.length === 0 ? enabledChannels.map((channel) => channel.id) : current
    ));
  }, [open, picker, enabledChannels, primarySource?.id]);

  const hasAvailableChannel = watchChannelIDs.some((id) => enabledChannels.some((channel) => channel.id === id));

  const toggleSelected = (id: number) => {
    setSelectedIds((current) => (
      current.includes(id) ? current.filter((entry) => entry !== id) : [...current, id]
    ));
  };

  const save = async () => {
    if (selectedSources.length === 0) {
      setError('请选择订阅源');
      return;
    }
    setSaving(true);
    setError('');
    const patch: SourcePatch = {
      watch_enabled: watchEnabled,
      watch_mode: watchMode,
      watch_ai: watchAI,
      watch_channel_ids: watchChannelIDs,
    };
    const updated: Source[] = [];
    const failed: string[] = [];
    for (const source of selectedSources) {
      try {
        updated.push(await api.updateSource(source.id, patch));
      } catch {
        failed.push(source.name);
      }
    }
    if (updated.length > 0) {
      onSaved?.(updated);
      await useSourcesStore.getState().load(true);
    }
    setSaving(false);
    if (failed.length > 0) {
      setError(`${failed.length} 个源保存失败：${failed.slice(0, 3).join('、')}${failed.length > 3 ? ' 等' : ''}`);
      return;
    }
    useUiStore.getState().notify(selectedSources.length > 1 ? `已更新 ${selectedSources.length} 个关注源` : '内容已更新');
    onClose();
  };

  const title = selectedSources.length > 1
    ? '批量关注设置'
    : primarySource?.watch_enabled
      ? '关注设置'
      : '关注订阅源';
  const subtitle = selectedSources.length === 1
    ? primarySource?.name
    : selectedSources.length > 1
      ? `${selectedSources.length} 个订阅源`
      : undefined;
  const footNote = selectedSources.length === 0
    ? '选择订阅源后设置提醒方式'
    : selectedSources.length === 1
      ? `提醒延迟取决于刷新间隔（${formatRefresh(primarySource?.refresh_seconds ?? 0)}）`
      : `提醒将应用到 ${selectedSources.length} 个源，延迟取决于各自刷新间隔`;

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={title}
      subtitle={subtitle}
      footer={
        <>
          <span className="modal-foot-note">{footNote}</span>
          <Button onClick={onClose}>取消</Button>
          <Button variant="primary" onClick={() => { void save(); }} disabled={saving || selectedSources.length === 0}>
            {saving ? '正在保存' : '保存'}
          </Button>
        </>
      }
    >
      <div className="watch-dialog">
        {picker ? (
          <div className="field">
            <span className="field-label">订阅源</span>
            {sourceOptions.length === 0 ? (
              <span className="field-hint">还没有订阅源，请先添加源。</span>
            ) : (
              <div className="watch-picker">
                <div className="watch-picker-toolbar">
                  <span className="watch-picker-count">已选 {selectedIds.length}</span>
                  <button
                    type="button"
                    className="st-batch-link"
                    onClick={() => setSelectedIds((current) => (
                      [...new Set([...current, ...filteredOptions.map((source) => source.id)])]
                    ))}
                    disabled={filteredOptions.length === 0}
                  >
                    全选
                  </button>
                  <button
                    type="button"
                    className="st-batch-link"
                    onClick={() => setSelectedIds([])}
                    disabled={selectedIds.length === 0}
                  >
                    清空
                  </button>
                  <input
                    className="watch-picker-search"
                    value={query}
                    onChange={(event) => setQuery(event.target.value)}
                    placeholder="搜索订阅源"
                    aria-label="搜索订阅源"
                  />
                </div>
                <div className="watch-picker-list" role="group" aria-label="选择订阅源">
                  {filteredOptions.map((source) => {
                    const checked = selectedIds.includes(source.id);
                    return (
                      <label key={source.id} className={cx('watch-picker-option', checked && 'is-selected')}>
                        <input
                          type="checkbox"
                          checked={checked}
                          onChange={() => toggleSelected(source.id)}
                          aria-label={`选择 ${source.name}`}
                        />
                        <SourceIcon source={source} size={22} />
                        <span className="watch-picker-name">{source.name}</span>
                        {source.watch_enabled ? <small>已关注</small> : null}
                      </label>
                    );
                  })}
                  {filteredOptions.length === 0 ? (
                    <span className="watch-picker-empty">没有匹配的订阅源</span>
                  ) : null}
                </div>
              </div>
            )}
          </div>
        ) : null}

        {selectedSources.length > 0 ? (
          <>
            {selectedSources.length === 1 && primarySource ? (
              <div className="watch-source-summary">
                <SourceIcon source={primarySource} size={30} />
                <div className="watch-source-summary-copy">
                  <strong>{primarySource.name}</strong>
                  <span>
                    {sourceKindLabel(primarySource)} · 每 {formatRefresh(primarySource.refresh_seconds)}刷新
                  </span>
                </div>
                <span className={cx('watch-source-state', watchEnabled && 'is-on')}>
                  {watchEnabled ? '已关注' : '未关注'}
                </span>
              </div>
            ) : (
              <div className="watch-source-summary is-multi">
                <div className="watch-source-summary-copy">
                  <strong>{selectedSources.length} 个订阅源</strong>
                  <span>{selectedNames(selectedSources)}</span>
                </div>
                <span className={cx('watch-source-state', watchEnabled && 'is-on')}>
                  {watchEnabled ? '批量关注' : '未关注'}
                </span>
              </div>
            )}

            <div className="watch-source-enable">
              <div className="watch-source-enable-copy">
                <strong>关注更新</strong>
                <span>
                  {selectedSources.length > 1
                    ? `对选中的 ${selectedSources.length} 个源生效；首次抓取、历史回补、过滤已读或隐藏的条目不提醒。`
                    : '新文章入库后推送；首次抓取、历史回补、过滤已读或隐藏的条目不提醒。'}
                </span>
              </div>
              <Switch
                checked={watchEnabled}
                onChange={(next) => {
                  setWatchEnabled(next);
                  if (next && watchChannelIDs.length === 0 && enabledChannels.length > 0) {
                    setWatchChannelIDs(enabledChannels.map((channel) => channel.id));
                  }
                }}
                label="关注更新"
              />
            </div>

            {watchEnabled ? (
              <div className="watch-source-options">
                <div className="field">
                  <span className="field-label">提醒方式</span>
                  <Segmented
                    ariaLabel="提醒方式"
                    value={watchMode}
                    onChange={setWatchMode}
                    options={[
                      { value: 'batch', label: '合并一条' },
                      { value: 'each', label: '每篇一条' },
                    ]}
                  />
                  <span className="field-hint">合并模式把本次抓取的新文章汇总成一条。</span>
                </div>
                <div className="field">
                  <span className="field-label">AI 总结</span>
                  <div className="edit-source-switch">
                    <Switch checked={watchAI} onChange={setWatchAI} label="AI 总结" />
                  </div>
                  <span className="field-hint">跟随提醒方式；AI 失败时退回标题列表。</span>
                </div>
                <div className="field span-2">
                  <span className="field-label">推送渠道</span>
                  {channelsLoading ? (
                    <span className="settings-field-hint">正在加载推送渠道…</span>
                  ) : channelsError ? (
                    <span className="field-hint form-error">{channelsError}</span>
                  ) : channels.length === 0 ? (
                    <span className="settings-field-hint">没有启用的推送渠道，请先到 设置 → 自动化 → 推送渠道 添加。</span>
                  ) : (
                    <div className="watch-channel-list">
                      {channels.map((channel) => {
                        const checked = watchChannelIDs.includes(channel.id);
                        return (
                          <label key={channel.id} className={cx('watch-channel-option', checked && 'is-selected')}>
                            <input
                              type="checkbox"
                              checked={checked}
                              onChange={(event) => {
                                setWatchChannelIDs((current) => (
                                  event.target.checked
                                    ? [...new Set([...current, channel.id])]
                                    : current.filter((id) => id !== channel.id)
                                ));
                              }}
                            />
                            <span>{channel.name}</span>
                            <small>{channel.kind === 'feishu' ? '飞书' : 'Webhook'}</small>
                          </label>
                        );
                      })}
                    </div>
                  )}
                </div>
                {!channelsLoading && !channelsError && !hasAvailableChannel ? (
                  <p className="watch-source-warning span-2">
                    {channels.length === 0
                      ? '未配置推送渠道：保存后只会在侧栏显示未读 dot，不会发送外部通知。'
                      : '所选渠道已停用或删除，请重新选择；否则不会发送外部通知。'}
                  </p>
                ) : null}
              </div>
            ) : null}
          </>
        ) : !picker ? (
          <p className="watch-source-warning">订阅源不存在或已被删除。</p>
        ) : null}

        {error ? <p className="form-error">{error}</p> : null}
      </div>
    </Modal>
  );
};
