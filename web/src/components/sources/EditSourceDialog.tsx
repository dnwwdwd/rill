import { useEffect, useState } from 'react';
import type { Source } from '@/api/types';
import { Button, Field, Modal, Switch } from '@/components/ui';
import { useSettingsStore } from '@/stores/settingsStore';
import { useSourcesStore } from '@/stores/sourcesStore';
import { formatRefresh } from '@/utils/date';
import { flattenFolders, folderOptionLabel } from '@/utils/folders';

interface EditSourceDialogProps {
  open: boolean;
  source: Source | null;
  onClose: () => void;
}

const INTERVALS = [900, 1800, 3600, 7200, 21600, 43200];
const INITIAL_LIMITS = [10, 20, 30, 50, 100];

// 与后端 sourceSupportsFullBackfill 一致：X 源与 Bluesky 官方 RSS 可全量回补。
const supportsFullBackfill = (source: Source): boolean =>
  source.kind === 'x' || /bsky\.app\/profile\//i.test(source.feed_url);

export const EditSourceDialog = ({ open, source, onClose }: EditSourceDialogProps) => {
  const folders = useSourcesStore((state) => state.folders);
  const folderNodes = flattenFolders(folders);
  const update = useSourcesStore((state) => state.update);
  const updateSettings = useSettingsStore((state) => state.update);
  const [name, setName] = useState('');
  const [feedURL, setFeedURL] = useState('');
  const [folderId, setFolderId] = useState<number | null>(null);
  const [refresh, setRefresh] = useState(3600);
  const [fallback, setFallback] = useState('');
  const [retentionDays, setRetentionDays] = useState(0);
  const [retentionMaxItems, setRetentionMaxItems] = useState(0);
  const [initialLimit, setInitialLimit] = useState(20);
  const [backfillMode, setBackfillMode] = useState<'window' | 'full'>('window');
  const [fullText, setFullText] = useState(false);
  const [pinned, setPinned] = useState(false);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (!open || !source) return;
    setName(source.name);
    setFeedURL(source.feed_url);
    setFolderId(source.folder_id);
    setRefresh(source.refresh_seconds);
    setFallback(source.fallback_url ?? '');
    setRetentionDays(source.retention_days ?? 0);
    setRetentionMaxItems(source.retention_max_items ?? 0);
    setInitialLimit(source.initial_limit ?? 20);
    setBackfillMode(source.backfill_mode === 'full' ? 'full' : 'window');
    setFullText(Boolean(source.full_text));
    setPinned((useSettingsStore.getState().settings.reader.pinned_sources ?? []).includes(source.id));
    setSaving(false);
  }, [open, source]);

  if (!source) return null;

  const canBackfill = supportsFullBackfill(source);

  const save = async () => {
    setSaving(true);
    const patch: Parameters<typeof update>[1] = {
      name: name.trim() || source.name,
      folder_id: folderId,
      refresh_seconds: refresh,
      fallback_url: fallback.trim() || null,
      retention_days: retentionDays,
      retention_max_items: retentionMaxItems,
      full_text: fullText,
    };
    const nextFeedURL = feedURL.trim();
    if (nextFeedURL && nextFeedURL !== source.feed_url) {
      patch.feed_url = nextFeedURL;
    }
    if (canBackfill) {
      patch.backfill_mode = backfillMode;
      if (backfillMode === 'window') {
        patch.initial_limit = initialLimit;
      }
    }
    const updated = await update(source.id, patch);
    if (updated) {
      const currentPinned = useSettingsStore.getState().settings.reader.pinned_sources ?? [];
      const nextPinned = pinned
        ? (currentPinned.includes(source.id) ? currentPinned : [...currentPinned, source.id])
        : currentPinned.filter((id) => id !== source.id);
      if (nextPinned.length !== currentPinned.length || nextPinned.some((id, index) => id !== currentPinned[index])) {
        updateSettings({ reader: { pinned_sources: nextPinned } });
      }
    }
    setSaving(false);
    if (updated) onClose();
  };

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="编辑订阅源"
      subtitle={source.name}
      wide
      footer={
        <>
          <span className="modal-foot-note">保存后立即生效；修改订阅地址会重新抓取内容</span>
          <Button onClick={onClose}>取消</Button>
          <Button variant="primary" onClick={() => { void save(); }} disabled={saving}>
            {saving ? '正在保存' : '保存修改'}
          </Button>
        </>
      }
    >
      <div className="edit-source-form">
        <div className="source-form-section span-2">基本信息</div>
        <Field label="源名称" className="span-2">
          <input value={name} onChange={(event) => setName(event.target.value)} aria-label="源名称" />
        </Field>
        {!source.feed_url.startsWith('imap://') ? (
          <Field label="订阅地址" className="span-2" hint="修改后会自动更新源信息并重新抓取文章。">
            <input value={feedURL} onChange={(event) => setFeedURL(event.target.value)} aria-label="订阅地址" />
          </Field>
        ) : null}
        <Field label="分组">
          <select
            value={folderId ?? ''}
            onChange={(event) => setFolderId(event.target.value ? Number(event.target.value) : null)}
            aria-label="分组"
          >
            <option value="">不分组</option>
            {folderNodes.map((node) => <option key={node.folder.id} value={node.folder.id}>{folderOptionLabel(node)}</option>)}
          </select>
        </Field>
        <div className="field">
          <span className="field-label">置顶</span>
          <div className="edit-source-switch">
            <Switch checked={pinned} onChange={setPinned} label="置顶订阅源" />
          </div>
          <span className="field-hint">显示在订阅源树顶部，不跟随原分组。</span>
        </div>

        <div className="source-form-section span-2">同步</div>
        <Field label="刷新间隔">
          <select value={refresh} onChange={(event) => setRefresh(Number(event.target.value))} aria-label="刷新间隔">
            {INTERVALS.map((value) => <option key={value} value={value}>{formatRefresh(value)}</option>)}
          </select>
        </Field>
        {source.kind === 'rss' ? (
          <div className="field">
            <span className="field-label">抓取全文</span>
            <div className="edit-source-switch">
              <Switch checked={fullText} onChange={setFullText} label="抓取全文" />
            </div>
            <span className="field-hint">Feed 只有摘要时抓取原文；每次最多 12 条新文章，失败保留摘要。</span>
          </div>
        ) : null}
        {canBackfill ? (
          <Field label="抓取范围">
            <select
              value={backfillMode}
              onChange={(event) => setBackfillMode(event.target.value === 'full' ? 'full' : 'window')}
              aria-label="抓取范围"
            >
              <option value="window">最近窗口</option>
              <option value="full">全部历史</option>
            </select>
          </Field>
        ) : null}
        {canBackfill && backfillMode === 'window' ? (
          <Field label="窗口条数">
            <select
              value={initialLimit}
              onChange={(event) => setInitialLimit(Number(event.target.value))}
              aria-label="窗口条数"
            >
              {INITIAL_LIMITS.map((value) => <option key={value} value={value}>{value} 条</option>)}
            </select>
          </Field>
        ) : null}
        {!canBackfill ? (
          <Field label="首次回溯上限">
            <input value={`${source.initial_limit} 条`} readOnly aria-label="首次回溯上限" />
          </Field>
        ) : null}

        <div className="source-form-section span-2">保留</div>
        <Field label="保留天数">
          <input
            type="number"
            min={0}
            max={3650}
            value={retentionDays}
            onChange={(event) => setRetentionDays(Math.max(0, Math.min(3650, Number(event.target.value) || 0)))}
            aria-label="保留天数"
          />
        </Field>
        <Field label="最多保留条数">
          <input
            type="number"
            min={0}
            max={100000}
            value={retentionMaxItems}
            onChange={(event) => setRetentionMaxItems(Math.max(0, Math.min(100000, Number(event.target.value) || 0)))}
            aria-label="最多保留条数"
          />
        </Field>
        <p className="source-form-note span-2">
          0 表示不限；收藏和未读条目不会被清理。
          {canBackfill && backfillMode === 'full' ? '保存「全部历史」后立即开始回补，进度见源备注；X 最多回补约 3200 条。' : ''}
        </p>

        <div className="source-form-section span-2">高级</div>
        <Field label="备用源" className="span-2" hint="主源抓取失败时自动接管；留空则不启用。">
          <input
            value={fallback}
            onChange={(event) => setFallback(event.target.value)}
            placeholder="名称或订阅地址"
            aria-label="备用源"
          />
        </Field>
      </div>
    </Modal>
  );
};
