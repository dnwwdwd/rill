import { useEffect, useMemo, useRef, useState } from 'react';
import { api } from '@/api/client';
import type {
  DigestPeriod,
  DigestReport,
  DigestRule,
  DigestRuleInput,
  FilterAction,
  FilterMatchField,
  FilterRule,
  FilterRuleInput,
  Folder,
  PushChannel,
  PushChannelInput,
  PushChannelKind,
  ReaderStats,
  RuleScope,
  Source,
  WatchMode,
} from '@/api/types';
import { Button, IconButton, Icon, Modal, Segmented, SourceIcon, Switch } from '@/components/ui';
import { RowGroup, Row } from '@/components/settings/SettingsSection';
import { WatchSourceDialog } from '@/components/sources/WatchSourceDialog';
import { useSourcesStore } from '@/stores/sourcesStore';
import { useUiStore } from '@/stores/uiStore';
import { cx } from '@/utils/cx';
import { renderMarkdown } from '@/utils/markdown';
import { formatDateTime, formatListTime } from '@/utils/date';

// REQ-036/037：自动化 = 定时报告 + 推送渠道 + 关键字过滤 + 已过滤条目。

const WEEKDAY_LABELS = ['', '周一', '周二', '周三', '周四', '周五', '周六', '周日'];

const CHANNEL_KIND_LABELS: Record<PushChannelKind, string> = {
  feishu: '飞书',
  generic_webhook: 'Webhook',
};

const FILTER_MATCH_LABELS: Record<FilterMatchField, string> = {
  title: '标题',
  content: '正文',
  any: '全部',
};

const FILTER_ACTION_LABELS: Record<FilterAction, string> = {
  mark_read: '标记已读',
  hide: '隐藏',
};

const WATCH_MODE_LABELS: Record<WatchMode, string> = {
  batch: '合并一条',
  each: '每篇一条',
};

const REPORT_STATUS_LABELS: Record<DigestReport['status'], string> = {
  ok: '已生成',
  empty: '空',
  partial: '部分',
  error: '失败',
};

const PUSH_STATUS_LABELS: Record<DigestReport['push_status'], string> = {
  none: '未推送',
  ok: '已推送',
  partial: '部分失败',
  failed: '推送失败',
};

const reportStatusTone = (status: DigestReport['status']): string => (
  status === 'ok' ? 'is-ok' : status === 'error' ? 'is-warn' : ''
);

const pushStatusTone = (status: DigestReport['push_status']): string => (
  status === 'ok' ? 'is-ok' : status === 'failed' || status === 'partial' ? 'is-warn' : ''
);

const isFolderEntry = (entry: number | string): entry is string =>
  typeof entry === 'string' && /^folder:\d+$/.test(entry);

const folderEntry = (id: number): string => `folder:${id}`;

const isSourceInScope = (source: Source, scope: RuleScope): boolean =>
  scope.includes(source.id) || (source.folder_id !== null && scope.includes(folderEntry(source.folder_id)));

// 范围选择：分段「全部源 / 自定义」+ 分组树勾选（分组 → folder:<id>，单源 → 源 ID）。
interface ScopeFieldProps {
  folders: Folder[];
  sources: Source[];
  scope: RuleScope;
  onChange: (scope: RuleScope) => void;
}

const ScopeField = ({ folders, sources, scope, onChange }: ScopeFieldProps) => {
  const [mode, setMode] = useState<'all' | 'custom'>(scope.length === 0 ? 'all' : 'custom');
  const [expanded, setExpanded] = useState<Set<number>>(() => new Set(folders.map((folder) => folder.id)));

  const switchMode = (next: 'all' | 'custom') => {
    setMode(next);
    if (next === 'all') {
      onChange([]);
      return;
    }
    // 从「全部源」切到自定义时，预选全部分组与未分组源，避免「自定义但为空」仍等于全部的歧义。
    if (scope.length === 0) {
      onChange([
        ...folders.map((folder) => folderEntry(folder.id)),
        ...sources.filter((source) => source.folder_id === null).map((source) => source.id),
      ]);
    }
  };

  const toggleFolder = (groupKey: number, groupSources: Source[]) => {
    const allSelected = groupSources.length > 0
      ? groupSources.every((source) => isSourceInScope(source, scope))
      : groupKey !== 0 && scope.includes(folderEntry(groupKey));
    const sourceIds = groupSources.map((source) => source.id);
    if (allSelected) {
      onChange(scope.filter((item) => !sourceIds.includes(item as number) && item !== folderEntry(groupKey)));
      return;
    }
    // 未分组没有 folder 条目，只能逐个源选中。
    if (groupKey === 0) {
      const merged = new Set(scope);
      sourceIds.forEach((id) => merged.add(id));
      onChange([...merged]);
      return;
    }
    // 选中父节点 = 整个分组（folder 条目），同时移除冗余的单源条目。
    const entry = folderEntry(groupKey);
    onChange([...scope.filter((item) => !sourceIds.includes(item as number) && item !== entry), entry]);
  };

  const toggleSource = (source: Source) => {
    const folderId = source.folder_id;
    const entry = folderId === null ? null : folderEntry(folderId);
    const selected = isSourceInScope(source, scope);
    if (selected) {
      if (entry && scope.includes(entry)) {
        // 取消分组内单个源：把分组展开为其余单源，父节点随之变为未全选。
        const next = scope.filter((item) => item !== entry && item !== source.id);
        const siblings = sources
          .filter((item) => item.folder_id === folderId && item.id !== source.id)
          .map((item) => item.id)
          .filter((id) => !next.includes(id));
        onChange([...next, ...siblings]);
      } else {
        onChange(scope.filter((item) => item !== source.id));
      }
      return;
    }
    const next = [...scope, source.id];
    // 子节点全部选中时收回为父节点，保持父子勾选双向一致。
    if (entry) {
      const siblingIds = sources.filter((item) => item.folder_id === folderId).map((item) => item.id);
      if (siblingIds.every((id) => next.includes(id))) {
        onChange([...next.filter((item) => !siblingIds.includes(item as number)), entry]);
        return;
      }
    }
    onChange(next);
  };

  const grouped = [
    ...folders.map((folder) => ({
      key: folder.id,
      name: folder.name,
      sources: sources.filter((source) => source.folder_id === folder.id),
    })),
    ...(sources.some((source) => source.folder_id === null)
      ? [{ key: 0, name: '未分组', sources: sources.filter((source) => source.folder_id === null) }]
      : []),
  ];

  return (
    <Row
      label="范围"
      control={(
        <Segmented
          ariaLabel="规则范围"
          value={mode}
          onChange={switchMode}
          options={[
            { value: 'all', label: '全部源' },
            { value: 'custom', label: '自定义' },
          ]}
        />
      )}
    >
      {mode === 'custom' ? (
        <div className="st-auto-tree">
          {grouped.map((group) => {
            const allSelected = group.sources.length > 0
              ? group.sources.every((source) => isSourceInScope(source, scope))
              : group.key !== 0 && scope.includes(folderEntry(group.key));
            const someSelected = group.sources.some((source) => isSourceInScope(source, scope));
            return (
              <div key={group.key} className="st-auto-tree-group">
                <label className="st-check st-auto-tree-folder">
                  <input
                    type="checkbox"
                    checked={allSelected}
                    ref={(element) => {
                      if (element) element.indeterminate = !allSelected && someSelected;
                    }}
                    onChange={() => toggleFolder(group.key, group.sources)}
                  />
                  <strong>{group.name}</strong>
                  <span className="st-tree-count">{group.sources.length}</span>
                </label>
                {expanded.has(group.key) || group.key === 0 ? group.sources.map((source) => (
                  <label key={source.id} className="st-check st-auto-tree-source">
                    <input
                      type="checkbox"
                      checked={isSourceInScope(source, scope)}
                      onChange={() => toggleSource(source)}
                    />
                    <SourceIcon source={source} size={24} />
                    <span className="st-auto-tree-name">{source.name}</span>
                  </label>
                )) : null}
                {group.key !== 0 && group.sources.length > 0 ? (
                  <button
                    type="button"
                    className="st-auto-tree-toggle"
                    onClick={() => {
                      const next = new Set(expanded);
                      if (next.has(group.key)) next.delete(group.key);
                      else next.add(group.key);
                      setExpanded(next);
                    }}
                  >
                    <Icon name={expanded.has(group.key) ? 'chevron-down' : 'chevron-right'} size={12} />
                    {expanded.has(group.key) ? '收起' : `展开 ${group.sources.length} 个源`}
                  </button>
                ) : null}
              </div>
            );
          })}
        </div>
      ) : null}
    </Row>
  );
};

const scopeSummary = (scope: RuleScope, sources: Source[]): string => {
  if (scope.length === 0) return '全部源';
  const folderIds = scope.filter(isFolderEntry).map((entry) => Number(entry.slice('folder:'.length)));
  const directIds = [...new Set(scope.filter((entry): entry is number => typeof entry === 'number'))];
  const folderSourceCount = sources.filter((source) => folderIds.includes(source.folder_id ?? -1)).length;
  const extraSourceCount = directIds.filter((id) => {
    const source = sources.find((entry) => entry.id === id);
    return source ? !folderIds.includes(source.folder_id ?? -1) : true;
  }).length;
  if (folderIds.length === 0) {
    return `${directIds.length} 个源`;
  }
  if (extraSourceCount === 0) {
    return `${folderIds.length} 个分组（${folderSourceCount} 源）`;
  }
  return `${folderIds.length} 个分组（${folderSourceCount} 源）+ ${extraSourceCount} 个源 · 共 ${folderSourceCount + extraSourceCount} 源`;
};

const periodSummary = (rule: DigestRule): string => (
  rule.period === 'weekly'
    ? `每周${WEEKDAY_LABELS[rule.weekday ?? 1]} ${rule.run_at_local}`
    : `每天 ${rule.run_at_local}`
);

interface ChannelDraft {
  id: number | null;
  name: string;
  kind: PushChannelKind;
  url: string;
  secret: string;
  secretTouched: boolean;
  editing?: PushChannel | null;
}

const EMPTY_CHANNEL: ChannelDraft = { id: null, name: '', kind: 'feishu', url: '', secret: '', secretTouched: false };

interface RuleDraft {
  id: number | null;
  name: string;
  scope: RuleScope;
  period: DigestPeriod;
  run_at_local: string;
  weekday: number;
  only_unread: boolean;
  push_when_empty: boolean;
  channel_ids: number[];
}

const EMPTY_RULE: RuleDraft = {
  id: null,
  name: '',
  scope: [],
  period: 'daily',
  run_at_local: '08:30',
  weekday: 1,
  only_unread: true,
  push_when_empty: false,
  channel_ids: [],
};

interface FilterDraft {
  id: number | null;
  name: string;
  pattern: string;
  is_regex: boolean;
  match_field: FilterMatchField;
  action: FilterAction;
  scope: RuleScope;
}

const EMPTY_FILTER: FilterDraft = {
  id: null,
  name: '',
  pattern: '',
  is_regex: false,
  match_field: 'title',
  action: 'hide',
  scope: [],
};

const errorMessage = (error: unknown, fallback: string): string => (
  error instanceof Error && error.message ? error.message : fallback
);

export const AutomationSettings = () => {
  const notify = useUiStore((state) => state.notify);
  const [channels, setChannels] = useState<PushChannel[]>([]);
  const [rules, setRules] = useState<DigestRule[]>([]);
  const [reports, setReports] = useState<DigestReport[]>([]);
  const [filters, setFilters] = useState<FilterRule[]>([]);
  const [folders, setFolders] = useState<Folder[]>([]);
  const [sources, setSources] = useState<Source[]>([]);
  const [stats, setStats] = useState<ReaderStats | null>(null);
  const [loadError, setLoadError] = useState('');

  const [channelDraft, setChannelDraft] = useState<ChannelDraft | null>(null);
  const [channelError, setChannelError] = useState('');
  const [channelSaving, setChannelSaving] = useState(false);
  const [testingId, setTestingId] = useState<number | null>(null);

  const [ruleDraft, setRuleDraft] = useState<RuleDraft | null>(null);
  const [ruleError, setRuleError] = useState('');
  const [runningId, setRunningId] = useState<number | null>(null);

  const [filterDraft, setFilterDraft] = useState<FilterDraft | null>(null);
  const [filterError, setFilterError] = useState('');
  const [applyingId, setApplyingId] = useState<number | null>(null);

  const [reportDetail, setReportDetail] = useState<DigestReport | null>(null);
  const [pushingReport, setPushingReport] = useState(false);

  const [hiddenOpen, setHiddenOpen] = useState(false);
  const [hiddenItems, setHiddenItems] = useState<Array<{ id: number; title: string; source: { name: string } }>>([]);
  const [hiddenLoading, setHiddenLoading] = useState(false);

  const [watchDialog, setWatchDialog] = useState<{
    open: boolean;
    sourceIds: number[];
    picker: boolean;
  }>({
    open: false,
    sourceIds: [],
    picker: false,
  });
  const [watchSelectMode, setWatchSelectMode] = useState(false);
  const [watchSelected, setWatchSelected] = useState<Set<number>>(new Set());
  const [watchBusy, setWatchBusy] = useState(false);

  const ruleFormRef = useRef<HTMLDivElement>(null);
  const filterFormRef = useRef<HTMLDivElement>(null);
  const channelFormRef = useRef<HTMLDivElement>(null);

  // 编辑表单在列表末尾，长列表下点击编辑后必须主动滚动到表单，否则看起来像没反应。
  useEffect(() => {
    if (!ruleDraft) return;
    ruleFormRef.current?.scrollIntoView?.({ block: 'nearest', behavior: 'smooth' });
  }, [ruleDraft?.id, ruleDraft !== null]);

  useEffect(() => {
    if (!filterDraft) return;
    filterFormRef.current?.scrollIntoView?.({ block: 'nearest', behavior: 'smooth' });
  }, [filterDraft?.id, filterDraft !== null]);

  useEffect(() => {
    if (!channelDraft) return;
    channelFormRef.current?.scrollIntoView?.({ block: 'nearest', behavior: 'smooth' });
  }, [channelDraft?.id, channelDraft !== null]);

  const loadCore = async () => {
    const [channelList, ruleList, reportList, filterList, folderList, sourceList, statList] = await Promise.all([
      api.getChannels(),
      api.getDigestRules(),
      api.getDigestReports(undefined, 20),
      api.getFilterRules(),
      api.getFolders(),
      api.getSources(),
      api.getReaderStats(),
    ]);
    setChannels(channelList);
    setRules(ruleList);
    setReports(reportList);
    setFilters(filterList);
    setFolders(folderList);
    setSources(sourceList);
    setStats(statList);
  };

  useEffect(() => {
    loadCore().catch((error) => setLoadError(errorMessage(error, '自动化设置加载失败')));
  }, []);

  // ===== 推送渠道 =====

  const buildChannelInput = (): PushChannelInput => {
    if (!channelDraft) throw new Error('渠道草稿不存在');
    const input: PushChannelInput = {
      name: channelDraft.name,
      kind: channelDraft.kind,
      url: channelDraft.url.trim() || undefined,
    };
    // 签名密钥只在用户动过该输入框时提交：更新时省略 = 保留原值，空串 = 清除。
    if (channelDraft.kind === 'feishu' && channelDraft.secretTouched) {
      input.secret = channelDraft.secret.trim();
    }
    return input;
  };

  const saveChannel = async (testAfter = false) => {
    if (!channelDraft) return;
    setChannelError('');
    setChannelSaving(true);
    const creating = channelDraft.id === null;
    try {
      let channelId = channelDraft.id;
      if (channelId === null) {
        const created = await api.createChannel({ ...buildChannelInput(), url: channelDraft.url.trim() });
        channelId = created.id;
      } else {
        await api.updateChannel(channelId, buildChannelInput());
      }
      if (testAfter && channelId !== null) {
        try {
          await api.testChannel(channelId);
          notify('已保存，测试通知已发送');
        } catch (error) {
          notify(`已保存，测试失败：${errorMessage(error, '原因未知')}`);
        }
      } else {
        notify(creating ? '渠道已创建' : '渠道已保存');
      }
      setChannelDraft(null);
      setChannels(await api.getChannels());
    } catch (error) {
      setChannelError(errorMessage(error, '保存渠道失败'));
    }
    setChannelSaving(false);
  };

  const testChannel = async (id: number) => {
    setTestingId(id);
    try {
      await api.testChannel(id);
      notify('测试消息已发送');
    } catch (error) {
      notify(`测试失败：${errorMessage(error, '原因未知')}`);
    }
    setTestingId(null);
  };

  const deleteChannel = async (id: number) => {
    try {
      await api.deleteChannel(id);
      setChannels(await api.getChannels());
      notify('渠道已删除');
    } catch (error) {
      notify(errorMessage(error, '删除渠道失败'));
    }
  };

  // ===== 报告规则 =====

  const saveRule = async () => {
    if (!ruleDraft) return;
    setRuleError('');
    const input: DigestRuleInput = {
      name: ruleDraft.name,
      scope: ruleDraft.scope,
      period: ruleDraft.period,
      run_at_local: ruleDraft.run_at_local,
      weekday: ruleDraft.period === 'weekly' ? ruleDraft.weekday : undefined,
      only_unread: ruleDraft.only_unread,
      push_when_empty: ruleDraft.push_when_empty,
      channel_ids: ruleDraft.channel_ids,
    };
    try {
      if (ruleDraft.id === null) {
        await api.createDigestRule(input);
        notify('规则已创建');
      } else {
        await api.updateDigestRule(ruleDraft.id, input);
        notify('规则已保存');
      }
      setRuleDraft(null);
      setRules(await api.getDigestRules());
    } catch (error) {
      setRuleError(errorMessage(error, '保存规则失败'));
    }
  };

  const runRule = async (id: number) => {
    setRunningId(id);
    try {
      const report = await api.runDigestRule(id);
      setReports(await api.getDigestReports(undefined, 20));
      setRules(await api.getDigestRules());
      notify(report.status === 'empty' ? '报告已生成：本期没有条目' : '报告已生成');
    } catch (error) {
      notify(errorMessage(error, '生成报告失败'));
    }
    setRunningId(null);
  };

  const deleteRule = async (id: number) => {
    try {
      await api.deleteDigestRule(id);
      setRules(await api.getDigestRules());
      setReports(await api.getDigestReports(undefined, 20));
      notify('规则已删除');
    } catch (error) {
      notify(errorMessage(error, '删除规则失败'));
    }
  };

  // ===== 报告 =====

  const openReport = async (id: number) => {
    try {
      setReportDetail(await api.getDigestReport(id));
    } catch (error) {
      notify(errorMessage(error, '读取报告失败'));
    }
  };

  const openLatestReport = async (ruleId: number) => {
    try {
      const latest = await api.getDigestReports(ruleId, 1);
      if (latest.length === 0) {
        notify('这个规则还没有报告，点「立即生成」生成');
        return;
      }
      await openReport(latest[0].id);
    } catch (error) {
      notify(errorMessage(error, '读取报告失败'));
    }
  };

  const pushReport = async (id: number) => {
    setPushingReport(true);
    try {
      const result = await api.pushDigestReport(id);
      setReportDetail((current) => (current && current.id === id
        ? { ...current, push_status: result.push_status, push_error: null, pushed_at: new Date().toISOString() }
        : current));
      setReports(await api.getDigestReports(undefined, 20));
      notify(`推送${PUSH_STATUS_LABELS[result.push_status]}`);
    } catch (error) {
      notify(`推送失败：${errorMessage(error, '原因未知')}`);
    }
    setPushingReport(false);
  };

  // ===== 过滤规则 =====

  const saveFilter = async () => {
    if (!filterDraft) return;
    setFilterError('');
    const input: FilterRuleInput = {
      name: filterDraft.name,
      pattern: filterDraft.pattern,
      is_regex: filterDraft.is_regex,
      match_field: filterDraft.match_field,
      action: filterDraft.action,
      scope: filterDraft.scope,
    };
    try {
      if (filterDraft.id === null) {
        await api.createFilterRule(input);
        notify('规则已创建');
      } else {
        await api.updateFilterRule(filterDraft.id, input);
        notify('规则已保存');
      }
      setFilterDraft(null);
      setFilters(await api.getFilterRules());
    } catch (error) {
      setFilterError(errorMessage(error, '保存规则失败'));
    }
  };

  const applyFilter = async (id: number) => {
    setApplyingId(id);
    try {
      const result = await api.applyFilterRule(id);
      setFilters(await api.getFilterRules());
      setStats(await api.getReaderStats());
      setHiddenItems([]);
      notify(result.affected > 0 ? `已应用到 ${result.affected} 条存量条目` : '没有匹配的存量条目');
    } catch (error) {
      notify(errorMessage(error, '应用失败'));
    }
    setApplyingId(null);
  };

  const deleteFilter = async (id: number) => {
    try {
      await api.deleteFilterRule(id);
      setFilters(await api.getFilterRules());
      notify('规则已删除');
    } catch (error) {
      notify(errorMessage(error, '删除规则失败'));
    }
  };

  // ===== 已过滤条目 =====

  const toggleHidden = async () => {
    const next = !hiddenOpen;
    setHiddenOpen(next);
    if (!next) return;
    setHiddenLoading(true);
    try {
      const page = await api.getItems({ hidden: true, limit: 50 });
      setHiddenItems(page.items.map((item) => ({ id: item.id, title: item.title, source: item.source })));
    } catch (error) {
      notify(errorMessage(error, '读取已过滤条目失败'));
    }
    setHiddenLoading(false);
  };

  const restoreItem = async (id: number) => {
    try {
      await api.patchItemState(id, { hidden: false });
      const page = await api.getItems({ hidden: true, limit: 50 });
      setHiddenItems(page.items.map((item) => ({ id: item.id, title: item.title, source: item.source })));
      setStats(await api.getReaderStats());
      notify('已恢复显示');
    } catch (error) {
      notify(errorMessage(error, '恢复失败'));
    }
  };

  const hiddenCount = stats?.hidden ?? 0;
  const reportMarkdown = useMemo(
    () => (reportDetail ? renderMarkdown(reportDetail.content_md) : ''),
    [reportDetail],
  );
  const watchedSources = sources.filter((source) => source.watch_enabled);

  const openWatchDialog = (sourceIds: number[], picker = false) => {
    setWatchDialog({ open: true, sourceIds, picker });
  };

  const exitWatchSelect = () => {
    setWatchSelectMode(false);
    setWatchSelected(new Set());
  };

  const enterWatchSelect = () => {
    setWatchSelectMode(true);
    setWatchSelected(new Set());
  };

  const toggleWatchSelected = (id: number) => {
    setWatchSelected((current) => {
      const next = new Set(current);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  const selectAllWatched = () => {
    setWatchSelected(new Set(watchedSources.map((source) => source.id)));
  };

  const disableWatchBatch = async () => {
    if (watchSelected.size === 0 || watchBusy) return;
    setWatchBusy(true);
    let failed = 0;
    for (const id of watchSelected) {
      try {
        await api.updateSource(id, { watch_enabled: false });
      } catch {
        failed += 1;
      }
    }
    await useSourcesStore.getState().load(true);
    try {
      setSources(await api.getSources());
    } catch {
      // 列表刷新失败时保留旧数据，操作结果仍以通知为准。
    }
    setWatchBusy(false);
    if (failed > 0) {
      notify(`${failed} 个源取消关注失败`);
      return;
    }
    notify(`已取消关注 ${watchSelected.size} 个源`);
    setWatchSelected(new Set());
  };

  const watchChannelSummary = (ids?: number[]): string => {
    const selected = ids ?? [];
    if (selected.length === 0) return '未选择渠道';
    return selected.map((id) => {
      const channel = channels.find((entry) => entry.id === id);
      if (!channel) return '渠道已删除';
      return channel.enabled ? channel.name : `${channel.name}（已停用）`;
    }).join('、');
  };

  if (loadError) {
    return (
      <RowGroup title="自动化">
        <Row hint={loadError} />
      </RowGroup>
    );
  }

  return (
    <>
      <RowGroup title="关注订阅源">
        {watchSelectMode ? (
          <Row>
            <div className="st-batch watch-batch-bar" role="toolbar" aria-label="关注源批量操作">
              <span className="st-batch-count">已选 {watchSelected.size}</span>
              <button
                type="button"
                className="st-batch-link"
                onClick={selectAllWatched}
                disabled={watchedSources.length === 0 || watchBusy}
              >
                全选
              </button>
              <button
                type="button"
                className="st-batch-link"
                onClick={() => setWatchSelected(new Set())}
                disabled={watchSelected.size === 0 || watchBusy}
              >
                清空
              </button>
              <Button
                size="sm"
                disabled={watchSelected.size === 0 || watchBusy}
                onClick={() => openWatchDialog([...watchSelected])}
              >
                提醒设置
              </Button>
              <Button
                size="sm"
                variant="danger"
                disabled={watchSelected.size === 0 || watchBusy}
                onClick={() => { void disableWatchBatch(); }}
              >
                取消关注
              </Button>
              <span className="st-batch-spacer" />
              <Button size="sm" onClick={exitWatchSelect}>取消选择</Button>
            </div>
          </Row>
        ) : null}
        {watchedSources.map((source) => (
          <Row
            key={source.id}
            label={watchSelectMode ? (
              <label className="watch-source-row-main is-selectable">
                <input
                  type="checkbox"
                  className="watch-select-check"
                  checked={watchSelected.has(source.id)}
                  onChange={() => toggleWatchSelected(source.id)}
                  aria-label={`选择 ${source.name}`}
                />
                <SourceIcon source={source} size={24} />
                <span className="watch-source-row-name">{source.name}</span>
              </label>
            ) : (
              <span className="watch-source-row-main">
                <SourceIcon source={source} size={24} />
                <span className="watch-source-row-name">{source.name}</span>
              </span>
            )}
            hint={[
              WATCH_MODE_LABELS[source.watch_mode === 'each' ? 'each' : 'batch'],
              source.watch_ai ? 'AI 总结' : '仅标题',
              watchChannelSummary(source.watch_channel_ids),
            ].join(' · ')}
            control={watchSelectMode ? undefined : (
              <Button
                size="sm"
                aria-label={`设置 ${source.name} 的关注提醒`}
                onClick={() => openWatchDialog([source.id])}
              >
                设置
              </Button>
            )}
          />
        ))}
        {watchedSources.length === 0 ? (
          <Row hint="还没有关注源。关注后，新文章会按设置推送到飞书/Webhook，并在源列表显示未读 dot。" />
        ) : null}
        {watchSelectMode ? null : (
          <Row
            control={(
              <div className="watch-group-actions">
                {watchedSources.length > 0 ? (
                  <Button size="sm" icon="check" onClick={enterWatchSelect}>批量管理</Button>
                ) : null}
                <Button size="sm" icon="plus" onClick={() => openWatchDialog([], true)}>添加关注源</Button>
              </div>
            )}
          />
        )}
      </RowGroup>

      <RowGroup title="报告规则">
        {rules.map((rule) => (
          <RuleRow
            key={rule.id}
            rule={rule}
            sources={sources}
            running={runningId === rule.id}
            onToggle={(enabled) => {
              void api.updateDigestRule(rule.id, {
                name: rule.name,
                scope: rule.scope,
                period: rule.period,
                run_at_local: rule.run_at_local,
                weekday: rule.period === 'weekly' ? rule.weekday : undefined,
                only_unread: rule.only_unread,
                push_when_empty: rule.push_when_empty,
                channel_ids: rule.channel_ids,
                enabled,
              }).then(async () => {
                setRules(await api.getDigestRules());
              }).catch((error) => notify(errorMessage(error, '更新规则失败')));
            }}
            onEdit={() => setRuleDraft({
              id: rule.id,
              name: rule.name,
              scope: [...rule.scope],
              period: rule.period,
              run_at_local: rule.run_at_local,
              weekday: rule.weekday ?? 1,
              only_unread: rule.only_unread,
              push_when_empty: rule.push_when_empty,
              channel_ids: [...rule.channel_ids],
            })}
            onRun={() => { void runRule(rule.id); }}
            onOpenReport={() => { void openLatestReport(rule.id); }}
            onDelete={() => { void deleteRule(rule.id); }}
          />
        ))}
        {rules.length === 0 ? <Row hint="还没有报告规则。" /> : null}
        {ruleDraft ? (
          <div className="st-form" ref={ruleFormRef}>
            <Row>
              <input
                value={ruleDraft.name}
                placeholder="规则名称"
                autoFocus
                onChange={(event) => setRuleDraft({ ...ruleDraft, name: event.target.value })}
                aria-label="规则名称"
              />
            </Row>
            <ScopeField
              folders={folders}
              sources={sources}
              scope={ruleDraft.scope}
              onChange={(scope) => setRuleDraft({ ...ruleDraft, scope })}
            />
            <Row label="周期">
              <Segmented
                ariaLabel="报告周期"
                value={ruleDraft.period}
                onChange={(period) => setRuleDraft({ ...ruleDraft, period })}
                options={[
                  { value: 'daily', label: '每天' },
                  { value: 'weekly', label: '每周' },
                ]}
              />
            </Row>
            <Row
              label="时间"
              control={(
                <div className="st-inline">
                  <input
                    type="time"
                    value={ruleDraft.run_at_local}
                    onChange={(event) => setRuleDraft({ ...ruleDraft, run_at_local: event.target.value })}
                    aria-label="运行时间"
                  />
                  {ruleDraft.period === 'weekly' ? (
                    <select
                      value={ruleDraft.weekday}
                      onChange={(event) => setRuleDraft({ ...ruleDraft, weekday: Number(event.target.value) })}
                      aria-label="星期"
                    >
                      {WEEKDAY_LABELS.slice(1).map((label, index) => (
                        <option key={label} value={index + 1}>{label}</option>
                      ))}
                    </select>
                  ) : null}
                </div>
              )}
            />
            <Row
              label="仅未读"
              control={(
                <Switch
                  checked={ruleDraft.only_unread}
                  onChange={(onlyUnread) => setRuleDraft({ ...ruleDraft, only_unread: onlyUnread })}
                  label="仅未读"
                />
              )}
            />
            <Row
              label="空窗口也推送"
              hint="关闭：无新文章不推送；开启：无新文章也推送空报告，确认任务正常"
              control={(
                <Switch
                  checked={ruleDraft.push_when_empty}
                  onChange={(pushWhenEmpty) => setRuleDraft({ ...ruleDraft, push_when_empty: pushWhenEmpty })}
                  label="空窗口也推送"
                />
              )}
            />
            <Row label="推送渠道">
              <div className="st-inline">
                {channels.filter((channel) => channel.enabled).map((channel) => (
                  <label key={channel.id} className="st-check">
                    <input
                      type="checkbox"
                      checked={ruleDraft.channel_ids.includes(channel.id)}
                      onChange={() => setRuleDraft({
                        ...ruleDraft,
                        channel_ids: ruleDraft.channel_ids.includes(channel.id)
                          ? ruleDraft.channel_ids.filter((id) => id !== channel.id)
                          : [...ruleDraft.channel_ids, channel.id],
                      })}
                    />
                    {channel.name}
                  </label>
                ))}
                {channels.filter((channel) => channel.enabled).length === 0 ? (
                  <span className="st-dim">没有可用渠道，先在下方创建</span>
                ) : null}
              </div>
            </Row>
            {ruleError ? <Row><span className="form-error">{ruleError}</span></Row> : null}
            <Row
              control={(
                <div className="st-inline">
                  <Button variant="primary" size="sm" onClick={() => { void saveRule(); }}>保存</Button>
                  <Button size="sm" onClick={() => setRuleDraft(null)}>取消</Button>
                </div>
              )}
            />
          </div>
        ) : (
          <Row
            control={(
              <Button size="sm" icon="plus" onClick={() => { setRuleError(''); setRuleDraft({ ...EMPTY_RULE }); }}>
                新建规则
              </Button>
            )}
          />
        )}
      </RowGroup>

      <RowGroup title="报告">
        {reports.map((report) => (
          <Row
            key={report.id}
            label={(
              <button type="button" className="text-link" onClick={() => { void openReport(report.id); }}>
                {report.rule_name}
              </button>
            )}
            hint={[
              `${formatListTime(report.period_start)} ~ ${formatListTime(report.period_end)} · ${report.item_count} 条`,
              report.push_error,
            ].filter(Boolean).join(' · ')}
            control={(
              <div className="st-inline">
                <span className={cx('st-state', reportStatusTone(report.status))}>{REPORT_STATUS_LABELS[report.status]}</span>
                <span className={cx('st-state', pushStatusTone(report.push_status))}>{PUSH_STATUS_LABELS[report.push_status]}</span>
              </div>
            )}
          />
        ))}
        {reports.length === 0 ? <Row hint="还没有报告。报告保存在这里，与是否配置推送渠道无关；在规则行点「立即生成」即可查看。" /> : null}
      </RowGroup>

      <RowGroup title="推送渠道">
        {channels.map((channel) => (
          <ChannelRow
            key={channel.id}
            channel={channel}
            testing={testingId === channel.id}
            onToggle={(enabled) => {
              void api.updateChannel(channel.id, {
                name: channel.name,
                kind: channel.kind,
                enabled,
              }).then(async () => {
                setChannels(await api.getChannels());
              }).catch((error) => notify(errorMessage(error, '更新渠道失败')));
            }}
            onEdit={() => { setChannelError(''); setChannelDraft({ id: channel.id, name: channel.name, kind: channel.kind, url: '', secret: '', secretTouched: false, editing: channel }); }}
            onTest={() => { void testChannel(channel.id); }}
            onDelete={() => { void deleteChannel(channel.id); }}
          />
        ))}
        {channels.length === 0 ? <Row hint="还没有推送渠道。" /> : null}
        {channelDraft ? (
          <div className="st-form" ref={channelFormRef}>
            <Row>
              <input
                value={channelDraft.name}
                placeholder="渠道名称"
                autoFocus
                onChange={(event) => setChannelDraft({ ...channelDraft, name: event.target.value })}
                aria-label="渠道名称"
              />
            </Row>
            <Row label="类型">
              <Segmented
                ariaLabel="渠道类型"
                value={channelDraft.kind}
                onChange={(kind) => setChannelDraft({ ...channelDraft, kind })}
                options={[
                  { value: 'feishu', label: '飞书' },
                  { value: 'generic_webhook', label: 'Webhook' },
                ]}
              />
            </Row>
            <Row label={channelDraft.id === null ? 'Webhook 地址' : '新地址（留空 = 不修改）'}>
              <input
                value={channelDraft.url}
                placeholder="https://…"
                onChange={(event) => setChannelDraft({ ...channelDraft, url: event.target.value })}
                aria-label="Webhook 地址"
              />
            </Row>
            {channelDraft.kind === 'feishu' ? (
              <Row
                label="签名密钥（选填）"
                hint={channelDraft.id !== null && channelDraft.editing?.has_secret && !channelDraft.secretTouched
                  ? '已保存密钥，不动此处即保留；清空内容保存可移除'
                  : '飞书机器人开启「签名校验」时才填写；未开启则留空'}
              >
                <input
                  type="password"
                  value={channelDraft.secret}
                  onChange={(event) => setChannelDraft({ ...channelDraft, secret: event.target.value, secretTouched: true })}
                  aria-label="签名密钥"
                />
              </Row>
            ) : null}
            {channelError ? <Row><span className="form-error">{channelError}</span></Row> : null}
            <Row
              control={(
                <div className="st-inline">
                  <Button variant="primary" size="sm" disabled={channelSaving} onClick={() => { void saveChannel(); }}>
                    保存
                  </Button>
                  <Button size="sm" disabled={channelSaving} onClick={() => { void saveChannel(true); }}>
                    保存并测试
                  </Button>
                  <Button size="sm" onClick={() => setChannelDraft(null)}>取消</Button>
                </div>
              )}
            />
          </div>
        ) : (
          <Row
            control={(
              <Button size="sm" icon="plus" onClick={() => { setChannelError(''); setChannelDraft({ ...EMPTY_CHANNEL }); }}>
                添加渠道
              </Button>
            )}
          />
        )}
      </RowGroup>

      <RowGroup title="过滤规则">
        {filters.map((filter) => (
          <FilterRow
            key={filter.id}
            filter={filter}
            sources={sources}
            applying={applyingId === filter.id}
            onToggle={(enabled) => {
              void api.updateFilterRule(filter.id, {
                name: filter.name,
                pattern: filter.pattern,
                is_regex: filter.is_regex,
                match_field: filter.match_field,
                action: filter.action,
                scope: filter.scope,
                enabled,
              }).then(async () => {
                setFilters(await api.getFilterRules());
              }).catch((error) => notify(errorMessage(error, '更新规则失败')));
            }}
            onEdit={() => { setFilterError(''); setFilterDraft({ ...filter, id: filter.id }); }}
            onApply={() => { void applyFilter(filter.id); }}
            onDelete={() => { void deleteFilter(filter.id); }}
          />
        ))}
        {filters.length === 0 ? <Row hint="还没有过滤规则。" /> : null}
        {filterDraft ? (
          <div className="st-form" ref={filterFormRef}>
            <Row>
              <input
                value={filterDraft.name}
                placeholder="规则名称"
                autoFocus
                onChange={(event) => setFilterDraft({ ...filterDraft, name: event.target.value })}
                aria-label="规则名称"
              />
            </Row>
            <Row label="模式">
              <input
                value={filterDraft.pattern}
                placeholder={filterDraft.is_regex ? '正则表达式' : '关键字'}
                onChange={(event) => setFilterDraft({ ...filterDraft, pattern: event.target.value })}
                aria-label="匹配模式"
              />
            </Row>
            <Row label="匹配方式">
              <Segmented
                ariaLabel="匹配方式"
                value={filterDraft.is_regex ? 'regex' : 'keyword'}
                onChange={(value) => setFilterDraft({ ...filterDraft, is_regex: value === 'regex' })}
                options={[
                  { value: 'keyword', label: '关键字' },
                  { value: 'regex', label: '正则' },
                ]}
              />
            </Row>
            <Row label="匹配字段">
              <Segmented
                ariaLabel="匹配字段"
                value={filterDraft.match_field}
                onChange={(matchField) => setFilterDraft({ ...filterDraft, match_field: matchField })}
                options={[
                  { value: 'title', label: '标题' },
                  { value: 'content', label: '正文' },
                  { value: 'any', label: '全部' },
                ]}
              />
            </Row>
            <Row label="动作">
              <Segmented
                ariaLabel="过滤动作"
                value={filterDraft.action}
                onChange={(action) => setFilterDraft({ ...filterDraft, action })}
                options={[
                  { value: 'mark_read', label: '标记已读' },
                  { value: 'hide', label: '隐藏' },
                ]}
              />
            </Row>
            <ScopeField
              folders={folders}
              sources={sources}
              scope={filterDraft.scope}
              onChange={(scope) => setFilterDraft({ ...filterDraft, scope })}
            />
            {filterError ? <Row><span className="form-error">{filterError}</span></Row> : null}
            <Row
              control={(
                <div className="st-inline">
                  <Button variant="primary" size="sm" onClick={() => { void saveFilter(); }}>保存</Button>
                  <Button size="sm" onClick={() => setFilterDraft(null)}>取消</Button>
                </div>
              )}
            />
          </div>
        ) : (
          <Row
            control={(
              <Button size="sm" icon="plus" onClick={() => { setFilterError(''); setFilterDraft({ ...EMPTY_FILTER }); }}>
                新建规则
              </Button>
            )}
          />
        )}
      </RowGroup>

      <RowGroup title="已过滤条目">
        <Row
          label={`${hiddenCount} 条被隐藏`}
          control={(
            <Button size="sm" icon={hiddenOpen ? 'chevron-down' : 'chevron-right'} onClick={() => { void toggleHidden(); }}>
              {hiddenOpen ? '收起' : '展开'}
            </Button>
          )}
        />
        {hiddenOpen ? hiddenItems.map((item) => (
          <Row
            key={item.id}
            label={item.title}
            hint={item.source.name}
            control={(
              <Button size="sm" icon="eye" onClick={() => { void restoreItem(item.id); }}>恢复</Button>
            )}
          />
        )) : null}
        {hiddenOpen && hiddenLoading ? <Row hint="加载中…" /> : null}
        {hiddenOpen && !hiddenLoading && hiddenItems.length === 0 ? <Row hint="没有被隐藏的条目。" /> : null}
      </RowGroup>

      <Modal
        open={reportDetail !== null}
        onClose={() => setReportDetail(null)}
        title={reportDetail?.rule_name ?? '报告'}
        subtitle={reportDetail
          ? `${formatDateTime(reportDetail.period_start)} ~ ${formatDateTime(reportDetail.period_end)} · ${reportDetail.item_count} 条 · ${PUSH_STATUS_LABELS[reportDetail.push_status]}`
          : undefined}
        wide
        footer={reportDetail ? (
          <>
            {reportDetail.push_error ? <span className="form-error">{reportDetail.push_error}</span> : null}
            <Button
              variant="primary"
              size="sm"
              icon="arrow-up-right"
              disabled={pushingReport}
              onClick={() => { void pushReport(reportDetail.id); }}
            >
              {pushingReport ? '推送中' : '重新推送'}
            </Button>
          </>
        ) : null}
      >
        {reportDetail ? (
          <div className="st-auto-md" dangerouslySetInnerHTML={{ __html: reportMarkdown }} />
        ) : null}
      </Modal>

      <WatchSourceDialog
        open={watchDialog.open}
        initialSourceIds={watchDialog.sourceIds}
        picker={watchDialog.picker}
        sources={sources}
        onClose={() => setWatchDialog({ open: false, sourceIds: [], picker: false })}
        onSaved={(updated) => {
          const saved = new Map(updated.map((source) => [source.id, source]));
          setSources((current) => current.map((source) => saved.get(source.id) ?? source));
          setWatchSelected(new Set());
        }}
      />
    </>
  );
};

interface RuleRowProps {
  rule: DigestRule;
  sources: Source[];
  running: boolean;
  onToggle: (enabled: boolean) => void;
  onEdit: () => void;
  onRun: () => void;
  onOpenReport: () => void;
  onDelete: () => void;
}

const RuleRow = ({ rule, sources, running, onToggle, onEdit, onRun, onOpenReport, onDelete }: RuleRowProps) => {
  const [confirming, setConfirming] = useState(false);
  return (
    <Row
      label={rule.name}
      hint={[
        periodSummary(rule),
        scopeSummary(rule.scope, sources),
        rule.next_run_at && rule.enabled ? `下次 ${formatListTime(rule.next_run_at)}` : '已停用',
      ].join(' · ')}
      control={(
        <div className="st-inline">
          <Switch checked={rule.enabled} onChange={onToggle} label={`启用 ${rule.name}`} />
          <Button size="sm" disabled={running} onClick={onRun}>{running ? '生成中' : '立即生成'}</Button>
          <IconButton icon="list" label={`查看 ${rule.name} 的报告`} size={15} onClick={onOpenReport} />
          <IconButton icon="pencil" label={`编辑 ${rule.name}`} size={15} onClick={onEdit} />
          {confirming ? (
            <span className="st-confirm">
              <Button size="sm" variant="danger" onClick={onDelete}>删除</Button>
              <Button size="sm" onClick={() => setConfirming(false)}>取消</Button>
            </span>
          ) : (
            <IconButton icon="trash" label={`删除 ${rule.name}`} size={15} onClick={() => setConfirming(true)} />
          )}
        </div>
      )}
    />
  );
};

interface ChannelRowProps {
  channel: PushChannel;
  testing: boolean;
  onToggle: (enabled: boolean) => void;
  onEdit: () => void;
  onTest: () => void;
  onDelete: () => void;
}

const ChannelRow = ({ channel, testing, onToggle, onEdit, onTest, onDelete }: ChannelRowProps) => {
  const [confirming, setConfirming] = useState(false);
  return (
    <Row
      label={channel.name}
      hint={`${CHANNEL_KIND_LABELS[channel.kind]} · ${channel.url_masked}`}
      control={(
        <div className="st-inline">
          <Switch checked={channel.enabled} onChange={onToggle} label={`启用 ${channel.name}`} />
          <Button size="sm" disabled={testing} onClick={onTest}>{testing ? '发送中' : '测试通知'}</Button>
          <IconButton icon="pencil" label={`编辑 ${channel.name}`} size={15} onClick={onEdit} />
          {confirming ? (
            <span className="st-confirm">
              <Button size="sm" variant="danger" onClick={onDelete}>删除</Button>
              <Button size="sm" onClick={() => setConfirming(false)}>取消</Button>
            </span>
          ) : (
            <IconButton icon="trash" label={`删除 ${channel.name}`} size={15} onClick={() => setConfirming(true)} />
          )}
        </div>
      )}
    />
  );
};

interface FilterRowProps {
  filter: FilterRule;
  sources: Source[];
  applying: boolean;
  onToggle: (enabled: boolean) => void;
  onEdit: () => void;
  onApply: () => void;
  onDelete: () => void;
}

const FilterRow = ({ filter, sources, applying, onToggle, onEdit, onApply, onDelete }: FilterRowProps) => {
  const [confirming, setConfirming] = useState(false);
  return (
    <Row
      label={filter.name}
      hint={[
        filter.is_regex ? '正则' : '关键字',
        `${FILTER_MATCH_LABELS[filter.match_field]} · ${FILTER_ACTION_LABELS[filter.action]} · 命中 ${filter.hit_count}`,
        scopeSummary(filter.scope, sources),
      ].join(' · ')}
      control={(
        <div className="st-inline">
          <Switch checked={filter.enabled} onChange={onToggle} label={`启用 ${filter.name}`} />
          <Button size="sm" disabled={applying} onClick={onApply}>{applying ? '应用中' : '应用到存量'}</Button>
          <IconButton icon="pencil" label={`编辑 ${filter.name}`} size={15} onClick={onEdit} />
          {confirming ? (
            <span className="st-confirm">
              <Button size="sm" variant="danger" onClick={onDelete}>删除</Button>
              <Button size="sm" onClick={() => setConfirming(false)}>取消</Button>
            </span>
          ) : (
            <IconButton icon="trash" label={`删除 ${filter.name}`} size={15} onClick={() => setConfirming(true)} />
          )}
        </div>
      )}
    />
  );
};
