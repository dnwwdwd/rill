import { useEffect, useRef, useState } from 'react';
import { api } from '@/api/client';
import type {
  CreatorSearchCandidate,
  CreatorSearchResponse,
  FeedCandidate,
  ResolveCandidate,
  SourceInput,
  WechatAccount,
} from '@/api/types';
import { Button, Field, Icon, Modal, Segmented, Tag } from '@/components/ui';
import { creatorCandidateKey, creatorMatchLabel, creatorRefreshSeconds, creatorStatusText, groupCreatorCandidates } from '@/components/sources/creatorSearch';
import { useReaderStore } from '@/stores/readerStore';
import { useSettingsStore } from '@/stores/settingsStore';
import { useSourcesStore } from '@/stores/sourcesStore';
import { useUiStore } from '@/stores/uiStore';
import { cx } from '@/utils/cx';
import { flattenFolders, folderOptionLabel } from '@/utils/folders';

interface AddSourceDialogProps {
  open: boolean;
  initialFolderId?: number | null;
  onClose: () => void;
}

type AddMode = 'link' | 'creator';

const X_HANDLE = /^[A-Za-z0-9_]{1,30}$/;

export const xHandleFromInput = (raw: string): string | null => {
  const value = raw.trim();
  if (!value) return null;
  // 纯数字优先交给平台解析器（Apple show ID），不当作 X handle。
  if (/^\d{5,12}$/.test(value)) return null;
  // 24 位十六进制是小宇宙 podcast ID，不是 X handle。
  if (/^[0-9a-f]{24}$/i.test(value)) return null;
  const profile = value.match(
    /^(?:https?:\/\/)?(?:www\.|mobile\.)?(?:x|twitter)\.com\/([A-Za-z0-9_]{1,30})(?:[/?#].*)?$/i,
  );
  if (profile) return profile[1];
  if (value.startsWith('@')) {
    const handle = value.slice(1);
    return X_HANDLE.test(handle) ? handle : null;
  }
  if (X_HANDLE.test(value) && !value.includes('.')) return value;
  return null;
};

// Bluesky handle 始终含点号（如 user.bsky.social 或自定义域）。
const BSKY_HANDLE = /^[A-Za-z0-9](?:[A-Za-z0-9-]*[A-Za-z0-9])?(?:\.[A-Za-z0-9](?:[A-Za-z0-9-]*[A-Za-z0-9])?)+$/;

export const bskyHandleFromInput = (raw: string): string | null => {
  const value = raw.trim();
  if (!value) return null;
  const profile = value.match(
    /^(?:https?:\/\/)?(?:www\.)?bsky\.app\/profile\/([A-Za-z0-9.-]+?)(?:\/rss)?(?:[/?#].*)?$/i,
  );
  if (profile) return profile[1].toLowerCase();
  if (value.startsWith('@')) {
    const handle = value.slice(1);
    return BSKY_HANDLE.test(handle) ? handle.toLowerCase() : null;
  }
  return null;
};

export const bskyRssUrl = (handle: string): string => `https://bsky.app/profile/${handle}/rss`;

const WECHAT_ARTICLE = /^https?:\/\/(?:mp\.)?weixin\.qq\.com\/s(?:[/?#]|$)/i;

const PLATFORM_LABEL: Record<string, string> = {
  apple: 'Apple Podcasts',
  xiaoyuzhou: '小宇宙',
  medium: 'Medium',
  mastodon: 'Mastodon',
  bestblogs: 'BestBlogs',
  'bestblogs-x': 'BestBlogs X',
  'bestblogs-podcast': 'BestBlogs 播客',
  'bestblogs-youtube': 'BestBlogs YouTube',
};

const normalizeUrl = (raw: string): string => {
  const value = raw.trim();
  if (/^https?:\/\//i.test(value) || /^rsshub:\/\//i.test(value)) return value;
  return `https://${value}`;
};

const hostFromUrl = (raw: string): string => {
  try {
    const parsed = new URL(raw);
    if (parsed.hostname) return parsed.hostname;
  } catch {
    // ignore, fall through to raw
  }
  return raw.slice(0, 80);
};

// 候选头像：优先服务端代理（绕开微博等 CDN 的外站 Referer 403），
// 加载失败回退到首字母。
const CreatorAvatar = ({ name, candidate }: {
  name: string;
  candidate: { icon_proxy_url?: string | null; icon_url?: string | null };
}) => {
  const [broken, setBroken] = useState(false);
  const src = !broken ? (candidate.icon_proxy_url || candidate.icon_url) : null;
  return (
    <span className="creator-avatar" aria-hidden="true">
      {src ? <img src={src} alt="" onError={() => setBroken(true)} /> : (name.trim().slice(0, 1) || 'R')}
    </span>
  );
};

export const AddSourceDialog = ({ open, initialFolderId = null, onClose }: AddSourceDialogProps) => {  const folders = useSourcesStore((state) => state.folders);
  const folderNodes = flattenFolders(folders);
  const create = useSourcesStore((state) => state.create);
  const createWechat = useSourcesStore((state) => state.createWechat);
  const createFolder = useSourcesStore((state) => state.createFolder);
  const wechat = useSettingsStore((state) => state.settings.wechat);
  const [value, setValue] = useState('');
  const [folderChoice, setFolderChoice] = useState('none');
  const [candidates, setCandidates] = useState<FeedCandidate[]>([]);
  const [selected, setSelected] = useState(-1);
  const [resolvedCandidates, setResolvedCandidates] = useState<ResolveCandidate[]>([]);
  const [resolvedSelected, setResolvedSelected] = useState(-1);
  const [busy, setBusy] = useState(false);
  const [discovering, setDiscovering] = useState(false);
  const [error, setError] = useState('');
  const [creatingFolder, setCreatingFolder] = useState(false);
  const [newFolderName, setNewFolderName] = useState('');
  const [folderBusy, setFolderBusy] = useState(false);
  const [mode, setMode] = useState<AddMode>('link');
  const [creatorQuery, setCreatorQuery] = useState('');
  const [creatorSearching, setCreatorSearching] = useState(false);
  const [creatorResults, setCreatorResults] = useState<CreatorSearchResponse | null>(null);
  const [creatorSelected, setCreatorSelected] = useState<Set<string>>(new Set());
  const [creatorError, setCreatorError] = useState('');
  const creatorSearchSeq = useRef(0);

  const submitNewFolder = async () => {
    const name = newFolderName.trim();
    if (!name || folderBusy) return;
    setFolderBusy(true);
    try {
      const folder = await createFolder(name, null);
      if (folder) {
        setFolderChoice(String(folder.id));
        setCreatingFolder(false);
        setNewFolderName('');
      }
    } finally {
      setFolderBusy(false);
    }
  };

  useEffect(() => {
    if (!open) return;
    setValue('');
    setFolderChoice(initialFolderId != null ? String(initialFolderId) : 'none');
    setCandidates([]);
    setSelected(-1);
    setResolvedCandidates([]);
    setResolvedSelected(-1);
    setBusy(false);
    setDiscovering(false);
    setError('');
    setCreatingFolder(false);
    setNewFolderName('');
    setMode('link');
    setCreatorQuery('');
    setCreatorSearching(false);
    setCreatorResults(null);
    setCreatorSelected(new Set());
    setCreatorError('');
    creatorSearchSeq.current += 1;
  }, [open, initialFolderId]);

  const selectedFolderId = folderChoice === 'none' ? null : Number(folderChoice);

  const finish = async (created: Awaited<ReturnType<typeof create>>) => {
    if (!created) return;
    await Promise.all([
      useReaderStore.getState().loadItems(true),
      useReaderStore.getState().loadStats(),
    ]);
    onClose();
  };

  const addWechatAccount = async (account: WechatAccount) => {
    const created = await createWechat({
      account: account.fake_id,
      name: account.name,
      avatar: account.avatar,
      folder_id: selectedFolderId,
    });
    await finish(created);
  };

  const addResolved = async (candidate: ResolveCandidate) => {
    const created = await create({
      kind: candidate.kind,
      name: candidate.name,
      feed_url: candidate.feed_url,
      site_url: candidate.site_url || null,
      icon_url: candidate.icon_url || null,
      note: candidate.note || null,
      folder_id: selectedFolderId,
      refresh_seconds: creatorRefreshSeconds(candidate.platform),
    });
    await finish(created);
  };

  const selectedCreatorCandidates = (creatorResults?.candidates ?? []).filter((candidate) => (
    !candidate.existing_source_id && creatorSelected.has(creatorCandidateKey(candidate))
  ));

  const toggleCreatorCandidate = (candidate: CreatorSearchCandidate) => {
    if (candidate.existing_source_id) return;
    const key = creatorCandidateKey(candidate);
    setCreatorSelected((previous) => {
      const next = new Set(previous);
      if (next.has(key)) {
        next.delete(key);
      } else {
        next.add(key);
      }
      return next;
    });
  };

  const runCreatorSearch = async () => {
    const query = creatorQuery.trim();
    if (!query || creatorSearching) {
      setCreatorError('请输入创作者名称。');
      return;
    }
    const sequence = creatorSearchSeq.current + 1;
    creatorSearchSeq.current = sequence;
    setCreatorSearching(true);
    setCreatorError('');
    try {
      const response = await api.searchCreators(query);
      if (sequence !== creatorSearchSeq.current) return;
      setCreatorResults(response);
      setCreatorSelected(new Set());
    } catch (searchError) {
      if (sequence !== creatorSearchSeq.current) return;
      setCreatorResults(null);
      setCreatorSelected(new Set());
      setCreatorError(searchError instanceof Error ? searchError.message : '搜索失败');
    } finally {
      if (sequence === creatorSearchSeq.current) {
        setCreatorSearching(false);
      }
    }
  };

  const submitCreator = async () => {
    const query = creatorQuery.trim();
    if (!query) {
      setCreatorError('请输入创作者名称。');
      return;
    }
    if (!creatorResults || creatorResults.query !== query) {
      await runCreatorSearch();
      return;
    }
    if (selectedCreatorCandidates.length === 0) {
      setCreatorError('请选择要订阅的创作者。');
      return;
    }
    setBusy(true);
    setCreatorError('');
    try {
      const inputs: SourceInput[] = selectedCreatorCandidates.map((candidate) => ({
        kind: candidate.kind,
        name: candidate.name,
        feed_url: candidate.feed_url,
        site_url: candidate.site_url || null,
        icon_url: candidate.icon_url || null,
        note: candidate.note || null,
        folder_id: selectedFolderId,
        refresh_seconds: creatorRefreshSeconds(candidate.platform),
      }));
      const result = await api.createSources(inputs);
      const createdByFeed = new Map(result.created.map((source) => [source.feed_url, source.id]));
      setCreatorResults((previous) => previous ? {
        ...previous,
        candidates: previous.candidates.map((candidate) => {
          const sourceID = createdByFeed.get(candidate.feed_url);
          return sourceID ? { ...candidate, existing_source_id: sourceID } : candidate;
        }),
      } : previous);
      setCreatorSelected(new Set());
      await useSourcesStore.getState().load(true);
      await Promise.all([
        useReaderStore.getState().loadItems(true),
        useReaderStore.getState().loadStats(),
      ]);
      if (result.created.length > 0) {
        useUiStore.getState().notify('内容已更新');
      }
      if (result.errors.length > 0) {
        const failed = result.errors
          .map((entry) => entry.name || entry.feed_url || `第 ${entry.index + 1} 个`)
          .join('、');
        setCreatorError(`已添加 ${result.created.length} 个，${result.errors.length} 个未添加：${failed}`);
        return;
      }
      onClose();
    } catch (batchError) {
      setCreatorError(batchError instanceof Error ? batchError.message : '批量添加失败');
    } finally {
      setBusy(false);
    }
  };

  const submit = async () => {
    if (mode === 'creator') {
      await submitCreator();
      return;
    }
    const raw = value.trim();
    if (!raw || busy) {
      setError('请输入网址、Apple Podcasts / 小宇宙 / X / Bluesky 链接或 ID，或公众号名称/文章链接。');
      return;
    }
    setError('');
    setBusy(true);
    try {
      // 公众号多结果：由 resolveSource 的 BestBlogs 候选承载。
      if (resolvedCandidates.length > 0) {
        const candidate = resolvedCandidates[resolvedSelected] ?? (resolvedCandidates.length === 1 ? resolvedCandidates[0] : undefined);
        if (!candidate) {
          setError('请选择一个订阅源。');
          return;
        }
        await addResolved(candidate);
        return;
      }

      const handle = xHandleFromInput(raw);
      if (handle) {
        const created = await create({
          kind: 'x',
          name: `@${handle}`,
          feed_url: '',
          handle,
          folder_id: selectedFolderId,
          refresh_seconds: 1800,
        });
        await finish(created);
        return;
      }

      // Bluesky：官方 profile RSS，FxEmbed（fxbsky.app）为降级镜像。
      const bsky = bskyHandleFromInput(raw);
      if (bsky) {
        const created = await create({
          kind: 'rss',
          name: `@${bsky}`,
          feed_url: bskyRssUrl(bsky),
          site_url: `https://bsky.app/profile/${bsky}`,
          folder_id: selectedFolderId,
        });
        await finish(created);
        return;
      }

      // 平台链接/ID：Apple Podcasts、小宇宙、Medium、Mastodon。
      // 普通网站返回空数组，继续走下面的 Feed 发现。
      const resolved = await api.resolveSource(raw);
      if (resolved.length > 0) {
        if (resolved.length > 1 || resolved.some((candidate) => candidate.partial)) {
          setResolvedCandidates(resolved);
          setResolvedSelected(0);
          setCandidates([]);
          setSelected(-1);
          return;
        }
        await addResolved(resolved[0]);
        return;
      }

      // 公众号：文章链接走公开页解析（无需后台登录），
      // 创建源要求已连接微信读书扫码（DEC-024 唯一通道）。
      if (WECHAT_ARTICLE.test(raw)) {
        if (!wechat.enabled) {
          setError('公众号采集未启用，请先在「公众号接入」启用。');
          return;
        }
        if (!wechat.weread_connected) {
          setError('请先在「公众号接入」连接微信读书扫码，再添加公众号。');
          return;
        }
        const account = await api.wechatResolveArticle(raw);
        await addWechatAccount(account);
        return;
      }
      if (/weixin\.qq\.com/i.test(raw)) {
        setError('请粘贴微信公众号文章链接（mp.weixin.qq.com/s/...）。');
        return;
      }
      if (!/[\s/:@.]/u.test(raw)) {
        // 裸名称：resolveSource 已优先查 BestBlogs 目录；未收录时只能粘贴文章链接。
        if (!wechat.enabled || !wechat.weread_connected) {
          setError('BestBlogs 目录没有找到这个公众号。请先在「公众号接入」连接微信读书扫码，再粘贴该公众号的文章链接添加。');
          return;
        }
        setError('BestBlogs 目录没有找到这个公众号。请粘贴该公众号任意一篇文章链接（mp.weixin.qq.com/s/...）添加。');
        return;
      }

      if (candidates.length === 0) {
        setDiscovering(true);
        let found: FeedCandidate[];
        try {
          found = await api.discoverFeed(normalizeUrl(raw));
        } catch {
          // 站点不可达 / 被反爬（如公共 RSSHub 实例）时 discover 会失败；
          // 仍按原地址直接创建，抓取结果通过源诊断状态呈现。
          const feedURL = normalizeUrl(raw);
          const created = await create({
            kind: 'rss',
            name: hostFromUrl(feedURL),
            feed_url: feedURL,
            folder_id: selectedFolderId,
          });
          await finish(created);
          return;
        } finally {
          setDiscovering(false);
        }
        if (found.length === 0) {
          setError('没有发现订阅源，请直接填写 RSS / Atom / JSON Feed 地址。');
          return;
        }
        if (found.length > 1) {
          setCandidates(found);
          setSelected(0);
          setResolvedCandidates([]);
          setResolvedSelected(-1);
          return;
        }
        const created = await create({
          kind: 'rss',
          name: found[0].title,
          feed_url: found[0].url,
          folder_id: selectedFolderId,
        });
        await finish(created);
        return;
      }

      const candidate = candidates[selected];
      if (!candidate) {
        setError('请选择一个订阅地址。');
        return;
      }
      const created = await create({
        kind: 'rss',
        name: candidate.title,
        feed_url: candidate.url,
        folder_id: selectedFolderId,
      });
      await finish(created);
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : '添加订阅失败');
    } finally {
      setDiscovering(false);
      setBusy(false);
    }
  };

  const creatorReady = Boolean(creatorResults && creatorResults.query === creatorQuery.trim());
  const creatorCandidateCount = creatorReady ? creatorResults?.candidates.length ?? 0 : 0;
  const creatorSelectedCount = selectedCreatorCandidates.length;
  const primaryDisabled = mode === 'creator'
    ? busy || creatorSearching || !creatorQuery.trim() || (creatorReady && creatorCandidateCount > 0 && creatorSelectedCount === 0)
    : busy || discovering || !value.trim();
  const primaryLabel = mode === 'creator'
    ? creatorSearching
      ? '正在搜索'
      : creatorReady && creatorCandidateCount > 0
        ? `订阅选中 (${creatorSelectedCount})`
        : '搜索'
    : busy
      ? '正在同步'
      : resolvedCandidates.length > 0
        ? '添加选中的订阅'
        : candidates.length > 1
          ? '添加选中的订阅'
          : '添加订阅';

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="添加订阅源"
      wide
      footer={
        <>
          <Button onClick={onClose}>取消</Button>
          <Button
            variant="primary"
            icon={mode === 'creator' && (!creatorReady || creatorCandidateCount === 0) ? 'search' : 'plus'}
            disabled={primaryDisabled}
            onClick={() => { void submit(); }}
          >
            {primaryLabel}
          </Button>
        </>
      }
    >
      <div className="add-simple">
        <Segmented
          options={[
            { value: 'link', label: '链接 / ID', icon: 'link' },
            { value: 'creator', label: '搜索创作者', icon: 'search' },
          ]}
          value={mode}
          onChange={(next) => {
            creatorSearchSeq.current += 1;
            setMode(next);
            setCreatorSearching(false);
            setError('');
            setCreatorError('');
          }}
          ariaLabel="添加方式"
          className="add-mode-row"
        />

        {mode === 'creator' ? (
          <>
            <div className="discover-row add-simple-row">
              <label className="input-with-icon">
                <Icon name="search" size={16} />
                <input
                  value={creatorQuery}
                  onChange={(event) => {
                    creatorSearchSeq.current += 1;
                    setCreatorQuery(event.target.value);
                    setCreatorResults(null);
                    setCreatorSelected(new Set());
                    setCreatorError('');
                    setCreatorSearching(false);
                  }}
                  onKeyDown={(event) => { if (event.key === 'Enter') { event.preventDefault(); void submit(); } }}
                  placeholder="输入创作者名称，如 阑夕、OpenAI、这集我看过"
                  aria-label="创作者名称"
                  autoFocus
                  disabled={busy}
                />
              </label>
            </div>

            <p className="add-hint">并发搜索 Apple Podcasts、Bluesky 和 BestBlogs 目录。X、YouTube、小宇宙、公众号只覆盖目录收录项，不是平台全网搜索。</p>

            {creatorError ? <div className="form-error">{creatorError}</div> : null}

            {creatorSearching ? (
              <div className="creator-searching" aria-live="polite">正在并发搜索 Apple Podcasts、Bluesky 和 BestBlogs 目录…</div>
            ) : null}

            {creatorResults ? (
              <>
                <div className="creator-status-row" aria-live="polite">
                  {creatorResults.platforms.map((status) => (
                    <Tag
                      key={status.platform}
                      tone={status.status === 'error' || status.status === 'timeout' ? 'danger' : 'neutral'}
                      title={status.error}
                    >
                      {status.label} · {creatorStatusText(status)}
                    </Tag>
                  ))}
                </div>

                {creatorResults.candidates.length === 0 ? (
                  <div className="add-placeholder">
                    <Icon name="search" size={18} />
                    <div>
                      <strong>没有找到候选</strong>
                      <span>可换一个名称，或切换到「链接 / ID」粘贴主页链接。</span>
                    </div>
                  </div>
                ) : (
                  groupCreatorCandidates(creatorResults.candidates).map((group) => (
                    <div className="creator-group" key={group.platform}>
                      <div className="creator-group-head">
                        <span>{group.label}</span>
                        <span>{group.candidates.length}</span>
                      </div>
                      {group.candidates.map((candidate) => {
                        const key = creatorCandidateKey(candidate);
                        const checked = creatorSelected.has(key);
                        const existing = Boolean(candidate.existing_source_id);
                        const matchLabel = creatorMatchLabel(candidate.match);
                        return (
                          <button
                            type="button"
                            key={key}
                            className={cx('candidate', checked && 'is-selected', existing && 'is-disabled')}
                            onClick={() => toggleCreatorCandidate(candidate)}
                            disabled={existing}
                            aria-pressed={checked}
                          >
                            <span className="candidate-check" aria-hidden="true"></span>
                            <CreatorAvatar name={candidate.name} candidate={candidate} />
                            <span className="candidate-copy">
                              <strong>
                                {candidate.name}
                                {matchLabel ? <span className="creator-match">{matchLabel}</span> : null}
                              </strong>
                              <span>{candidate.handle ? `@${candidate.handle}` : candidate.note || candidate.site_url || candidate.feed_url}</span>
                            </span>
                            {existing ? <Tag tone="neutral">已订阅</Tag> : candidate.partial ? <Tag tone="neutral">受限</Tag> : null}
                          </button>
                        );
                      })}
                    </div>
                  ))
                )}
              </>
            ) : null}
          </>
        ) : (
          <>
        <div className="discover-row add-simple-row">
          <label className="input-with-icon">
            <Icon name={xHandleFromInput(value) ? 'at-sign' : 'link'} size={16} />
            <input
              value={value}
              onChange={(event) => {
                setValue(event.target.value);
                setCandidates([]);
                setSelected(-1);
                setResolvedCandidates([]);
                setResolvedSelected(-1);
                setError('');
              }}
              onKeyDown={(event) => { if (event.key === 'Enter') void submit(); }}
              placeholder="粘贴网站 / Apple Podcasts / 小宇宙 / X / Bluesky 链接或 ID，或公众号名称"
              aria-label="订阅源地址或社交账号"
              autoFocus
            />
          </label>
        </div>

        <p className="add-hint">支持 RSS / Atom / JSON Feed、Apple Podcasts、小宇宙、X、Bluesky、Medium、Mastodon；公众号名称优先匹配 BestBlogs 目录，未收录时可粘贴公众号文章链接（需在「公众号接入」连接微信读书扫码）。输入 bestblogs:名称 可跨公众号、播客、X、YouTube 搜索 BestBlogs 目录。</p>

        {error ? <div className="form-error">{error}</div> : null}

        {resolvedCandidates.length > 0 ? (
          <div className="discover-results add-simple-candidates" aria-live="polite">
            <div className="discover-results-head">
              <span>找到 {resolvedCandidates.length} 个订阅源，请选择</span>
            </div>
            {resolvedCandidates.map((candidate, index) => (
              <button
                type="button"
                key={`${candidate.platform}:${candidate.feed_url}`}
                className={cx('candidate', resolvedSelected === index && 'is-selected')}
                onClick={() => setResolvedSelected(index)}
              >
                <span className="candidate-radio" aria-hidden="true"></span>
                <span className="candidate-copy">
                  <strong>{candidate.name}</strong>
                  <span>{candidate.note || candidate.site_url || candidate.feed_url}</span>
                </span>
                <Tag tone="neutral">{PLATFORM_LABEL[candidate.platform] ?? candidate.platform}</Tag>
              </button>
            ))}
          </div>
        ) : null}

        {candidates.length > 1 ? (
          <div className="discover-results add-simple-candidates" aria-live="polite">
            <div className="discover-results-head">
              <span>发现 {candidates.length} 个订阅地址，请选择</span>
            </div>
            {candidates.map((candidate, index) => (
              <button
                type="button"
                key={candidate.url}
                className={cx('candidate', selected === index && 'is-selected')}
                onClick={() => setSelected(index)}
              >
                <span className="candidate-radio" aria-hidden="true"></span>
                <span className="candidate-copy">
                  <strong>{candidate.title}</strong>
                  <span>{candidate.url}</span>
                </span>
                <Tag tone="neutral">{candidate.format}</Tag>
              </button>
            ))}
          </div>
        ) : null}
          </>
        )}

        <div className="add-folder-field">
          {creatingFolder ? (
            <div className="add-folder-create">
              <Field label="新分组名称">
                <input
                  value={newFolderName}
                  onChange={(event) => setNewFolderName(event.target.value)}
                  onKeyDown={(event) => { if (event.key === 'Enter') { event.preventDefault(); void submitNewFolder(); } }}
                  placeholder="输入分组名称"
                  aria-label="新分组名称"
                  autoFocus
                />
              </Field>
              <div className="add-folder-create-actions">
                <Button size="sm" variant="primary" icon="check" disabled={!newFolderName.trim() || folderBusy} onClick={() => { void submitNewFolder(); }}>
                  {folderBusy ? '创建中' : '创建'}
                </Button>
                <Button size="sm" onClick={() => { setCreatingFolder(false); setNewFolderName(''); }}>取消</Button>
              </div>
            </div>
          ) : (
            <div className="add-folder-row">
              <Field label="分组">
                <select
                  value={folderChoice}
                  onChange={(event) => setFolderChoice(event.target.value)}
                  aria-label="订阅源分组"
                >
                  <option value="none">不分组</option>
                  {folderNodes.map((node) => (
                    <option key={node.folder.id} value={node.folder.id}>{folderOptionLabel(node)}</option>
                  ))}
                </select>
              </Field>
              <Button size="sm" icon="plus" onClick={() => setCreatingFolder(true)}>新建分组</Button>
            </div>
          )}
        </div>
      </div>
    </Modal>
  );
};
