import { useEffect, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import type { FetchLog, FetchResult, SourceView } from '@/api/types';
import { MobileTopbar } from '@/components/layout/MobileTopbar';
import { EditSourceDialog } from '@/components/sources/EditSourceDialog';
import {
  Button,
  Card,
  EmptyState,
  Icon,
  IconButton,
  Metric,
  MiniBars,
  Modal,
  Notice,
  SectionHeading,
  SourceIcon,
  StatusBadge,
  StatusDot,
  Switch,
  TextLink,
} from '@/components/ui';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import { useSourcesStore } from '@/stores/sourcesStore';
import { useUiStore } from '@/stores/uiStore';
import { copyText } from '@/utils/clipboard';
import {
  formatAgo,
  formatDuration,
  formatListTime,
  formatNextFetch,
  formatNumber,
  formatRefresh,
} from '@/utils/date';
import { deriveSourceStatus, toSourceView } from '@/utils/sourceStatus';
import { cx } from '@/utils/cx';

const resultMeta = (result: FetchResult): { label: string; tone: string } => {
  if (result === 'ok') return { label: '成功', tone: 'ok' };
  if (result === 'not_modified') return { label: '未修改', tone: 'neutral' };
  return { label: '失败', tone: 'danger' };
};

const FetchResultBadge = ({ result }: { result: FetchResult }) => {
  const meta = resultMeta(result);
  return <span className={cx('fetch-result', meta.tone)}><i></i>{meta.label}</span>;
};

export const SourceHealthPage = () => {
  const params = useParams<{ sourceId: string }>();
  const navigate = useNavigate();
  const isTablet = useMediaQuery('(max-width: 1040px)');
  const health = useSourcesStore((state) => (params.sourceId ? state.health[Number(params.sourceId)] : undefined));
  const sources = useSourcesStore((state) => state.sources);
  const folders = useSourcesStore((state) => state.folders);
  const loadHealth = useSourcesStore((state) => state.loadHealth);
  const refresh = useSourcesStore((state) => state.refresh);
  const switchFallback = useSourcesStore((state) => state.switchFallback);
  const toggleEnabled = useSourcesStore((state) => state.toggleEnabled);
  const resetToken = useSourcesStore((state) => state.resetToken);
  const remove = useSourcesStore((state) => state.remove);
  const notify = useUiStore((state) => state.notify);
  const openSettings = useUiStore((state) => state.openSettings);
  const [loading, setLoading] = useState(true);
  const [editOpen, setEditOpen] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const sourceId = Number(params.sourceId);

  useEffect(() => {
    if (!Number.isFinite(sourceId)) {
      setLoading(false);
      return;
    }
    setLoading(true);
    void loadHealth(sourceId, true).finally(() => setLoading(false));
  }, [sourceId, loadHealth]);

  const source = health?.source ?? sources.find((entry) => entry.id === sourceId);
  const backToSources = () => {
    navigate('/');
    openSettings('sources');
  };
  if (!Number.isFinite(sourceId)) {
    return <EmptyState icon="server" title="没有选中的源" action={<Button onClick={backToSources}>返回源管理</Button>} />;
  }
  if (!source && loading) {
    return <div className="health-loading" aria-busy="true"><span></span><span></span><span></span></div>;
  }
  if (!source) {
    return <EmptyState icon="server" title="源不存在" action={<Button onClick={backToSources}>返回源管理</Button>} />;
  }

  const view: SourceView = toSourceView(source, folders);
  const status = deriveSourceStatus(source);
  const isAuth = status === 'auth';
  const logs = health?.logs ?? [];
  const trend = health?.trend ?? [];
  const latestLog = logs[0];
  const feedToken = health?.feed_token_masked ?? '—';
  const feedPaths = health?.feed_paths;
  const unread = source.unread_count;

  const copyPath = async (path: string, label: string) => {
    try {
      await copyText(new URL(path, window.location.origin).toString());
      notify(`${label} 地址已复制`);
    } catch (error) {
      notify(error instanceof Error ? error.message : '复制失败');
    }
  };

  return (
    <>
      {isTablet ? (
        <MobileTopbar
          title="源诊断"
          onBack={backToSources}
          right={<IconButton icon="refresh" label="立即刷新" size={18} onClick={() => { void refresh(source.id); }} />}
        />
      ) : null}
      <div className="main-pane health-screen">
        <div className="page-scroll">
          <header className="health-head">
            <div className="health-head-main">
              <button type="button" className="back-link" onClick={backToSources}>
                <Icon name="arrow-left" size={15} /> 订阅源
              </button>
              <div className="health-identity">
                <span className="health-identity-mark"><SourceIcon source={source} size={40} /></span>
                <div>
                  <div className="health-title-row">
                    <h1>{source.name}</h1>
                    <StatusBadge status={status} note={source.note} />
                  </div>
                  <div className="health-meta">
                    <span>{source.feed_url}</span>
                    <span className="dot-sep">·</span>
                    <span>{formatRefresh(source.refresh_seconds)} 刷新</span>
                    <span className="dot-sep">·</span>
                    <span>{unread} 条未读</span>
                  </div>
                </div>
              </div>
            </div>
            <div className="health-actions">
              <Button variant="primary" icon="refresh" onClick={() => { void refresh(source.id); }}>立即刷新</Button>
              <Button icon="sliders" onClick={() => setEditOpen(true)}>编辑</Button>
              <Button icon="plus" onClick={() => setEditOpen(true)}>设置备用源</Button>
              <span className="health-switch">
                <span>启用</span>
                <Switch checked={source.enabled} onChange={() => { void toggleEnabled(source.id); }} label="启用源" />
              </span>
            </div>
          </header>

          {isAuth ? (
            <Notice
              tone="danger"
              icon="lock"
              title="登录态已过期，抓取已暂停"
              action={<Button size="sm" onClick={() => openSettings('sources')}>打开设置</Button>}
            >
              {source.last_error ?? '需要重新扫码'}。
            </Notice>
          ) : null}
          {!source.enabled ? (
            <Notice
              icon="pause"
              title="该源已暂停"
              action={<Button size="sm" onClick={() => { void toggleEnabled(source.id); }}>重新启用</Button>}
            >
              暂停只跳过调度，不删除已有条目与收藏。
            </Notice>
          ) : null}

          <div className="health-stats">
            {[
              { label: '上次抓取', value: formatAgo(source.last_fetch_at), icon: 'clock' as const, tone: '' },
              { label: '抓取耗时', value: formatDuration(latestLog?.duration_ms), icon: 'refresh' as const, tone: '' },
              { label: '连续失败', value: `${source.fail_count} 次`, icon: 'alert-circle' as const, tone: source.fail_count > 0 ? 'warn' : '' },
              { label: '下次抓取', value: formatNextFetch(source.next_fetch_at), icon: 'calendar' as const, tone: '' },
              { label: '上次新条目', value: formatAgo(source.last_new_item_at), icon: 'rss' as const, tone: '' },
              { label: '条目总数', value: formatNumber(source.item_count), icon: 'database' as const, tone: '' },
            ].map((stat) => (
              <Metric
                key={stat.label}
                icon={stat.icon}
                label={stat.label}
                value={stat.value}
                tone={stat.tone === 'warn' ? 'warn' : ''}
              />
            ))}
          </div>

          <div className="health-grid">
            <Card className="health-card health-chart-card">
              <SectionHeading
                title="近 14 天新增"
                icon="bar-chart"
                action={<span className="chart-total">{trend.reduce((sum, value) => sum + value, 0)} 条</span>}
              />
              <MiniBars values={trend} label="近 14 天每日新增条目数" />
              <div className="chart-axis"><span>14 天前</span><span>今天</span></div>
            </Card>

            <Card className="health-card health-silent-card">
              <SectionHeading
                title={isAuth ? '无法判定' : '状态正常'}
                icon="activity"
                action={<StatusDot status={status} />}
              />
              <dl className="silent-list">
                <div><dt>刷新间隔</dt><dd>{formatRefresh(source.refresh_seconds)}</dd></div>
                <div><dt>距上次新条目</dt><dd>{formatAgo(source.last_new_item_at)}</dd></div>
                <div><dt>当前判定</dt><dd className={isAuth ? 'danger-text' : 'ok-text'}>{isAuth ? '登录态过期' : '正常'}</dd></div>
              </dl>
            </Card>
          </div>

          <div className="health-grid health-grid-lower">
            <Card className="health-card">
              <SectionHeading
                title={`最近 ${logs.length} 次抓取`}
                icon="clock"
                action={<TextLink onClick={() => { void refresh(source.id); }}>重新抓取</TextLink>}
              />
              <div className="fetch-log" role="table" aria-label="抓取历史">
                <div className="fetch-log-head" role="row">
                  <span>时间</span><span>结果</span><span>状态码</span><span>新增</span><span>耗时</span><span>说明</span>
                </div>
                {logs.map((log: FetchLog) => (
                  <div className="fetch-log-row" role="row" key={log.id}>
                    <span data-label="时间">{formatListTime(log.started_at)}</span>
                    <span data-label="结果"><FetchResultBadge result={log.result} /></span>
                    <span data-label="状态码"><code>{log.http_status ?? '—'}</code></span>
                    <span data-label="新增">{log.new_count}</span>
                    <span data-label="耗时"><code>{formatDuration(log.duration_ms)}</code></span>
                    <span data-label="说明" className={log.result === 'error' ? 'log-error' : ''}>{log.error ?? (log.result === 'not_modified' ? '命中条件请求，只更新抓取时间' : '抓取完成')}</span>
                  </div>
                ))}
              </div>
            </Card>

            <div className="health-side">
              <Card className="health-card">
                <SectionHeading
                  title="备用源"
                  icon="shield"
                  action={<span className="section-heading-value">{source.fallback_url ? '已配置' : '未配置'}</span>}
                />
                <div className="health-side-actions">
                  <Button size="sm" icon="refresh" onClick={() => { void switchFallback(source.id); }} disabled={!source.fallback_url}>切换备用源</Button>
                  <Button size="sm" icon="plus" onClick={() => setEditOpen(true)}>配置</Button>
                </div>
              </Card>

              <Card className="health-card">
                <SectionHeading title="订阅地址" icon="rss" />
                <div className="feed-output-list">
                  {([
                    { label: 'RSS 2.0', path: feedPaths?.rss, suffix: 'rss' },
                    { label: 'Atom 1.0', path: feedPaths?.atom, suffix: 'atom' },
                    { label: 'JSON Feed', path: feedPaths?.json, suffix: 'json' },
                  ]).map((format) => (
                    <div className="feed-output-row" key={format.suffix}>
                      <span className="feed-output-label">{format.label}</span>
                      <code>{format.path ?? '—'}</code>
                      <IconButton
                        icon="copy"
                        label={`复制 ${format.label} 地址`}
                        size={15}
                        onClick={() => format.path ? void copyPath(format.path, format.label) : undefined}
                      />
                    </div>
                  ))}
                </div>
                <div className="token-row">
                  <div>
                    <span className="token-label">输出令牌</span>
                    <code>{feedToken}</code>
                  </div>
                  <Button size="sm" onClick={() => { void resetToken(source.id); }}>重置</Button>
                </div>
                <p className="health-card-copy">重置后旧地址立即失效。</p>
              </Card>
            </div>
          </div>

          <Card className="danger-zone" tone="danger">
            <div>
              <div className="eyebrow">危险操作</div>
              <h2>删除订阅源</h2>
              <p>删除该源的条目、收藏与图标缓存，不可恢复。</p>
            </div>
            <Button variant="danger" icon="trash" onClick={() => setDeleteOpen(true)}>删除源</Button>
          </Card>
        </div>

        <EditSourceDialog open={editOpen} source={view} onClose={() => setEditOpen(false)} />

        <Modal
          open={deleteOpen}
          onClose={() => setDeleteOpen(false)}
          title="删除订阅源？"
          subtitle={source.name}
          footer={
            <>
              <span className="modal-foot-note">删除后不可恢复</span>
              <Button onClick={() => setDeleteOpen(false)}>取消</Button>
              <Button
                variant="danger"
                onClick={() => {
                  setDeleteOpen(false);
                  void remove(source.id).then(backToSources);
                }}
              >
                确认删除
              </Button>
            </>
          }
        >
          <p className="modal-copy">将删除 {formatNumber(source.item_count)} 条条目记录、阅读状态、收藏与图标缓存。其他源和订阅输出不受影响。</p>
        </Modal>
      </div>
    </>
  );
};
