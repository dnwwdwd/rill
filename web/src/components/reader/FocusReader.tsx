import { useEffect, useRef, useState, type UIEvent } from 'react';
import { api } from '@/api/client';
import { AiSummary } from '@/components/reader/AiSummary';
import { ArticleContent } from '@/components/reader/ArticleContent';
import { NotesPanel } from '@/components/reader/NotesPanel';
import { PodcastPlayer } from '@/components/reader/PodcastPlayer';
import { TranslationPanel } from '@/components/reader/TranslationPanel';
import {
  Button,
  EmptyState,
  Icon,
  IconButton,
} from '@/components/ui';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import { useReadMarker } from '@/hooks/useReadMarker';
import { useSpeech } from '@/hooks/useSpeech';
import { useReaderStore } from '@/stores/readerStore';
import { useSettingsStore } from '@/stores/settingsStore';
import { useSourcesStore } from '@/stores/sourcesStore';
import { useUiStore } from '@/stores/uiStore';
import { copyFeedUrl, copyText } from '@/utils/clipboard';
import { formatListTime } from '@/utils/date';
import { downloadMarkdown } from '@/utils/markdownExport';

interface FocusReaderProps {
  itemId: number;
  onBack: () => void;
}

const ringCircumference = 2 * Math.PI * 18;

export const FocusReader = ({ itemId, onBack }: FocusReaderProps) => {
  const isMobile = useMediaQuery('(max-width: 760px)');
  const details = useReaderStore((state) => state.details);
  const loadDetail = useReaderStore((state) => state.loadDetail);
  const translationMode = useReaderStore((state) => state.translationMode);
  const setTranslationMode = useReaderStore((state) => state.setTranslationMode);
  const summaryBusy = useReaderStore((state) => state.summaryBusy);
  const translationBusy = useReaderStore((state) => state.translationBusy);
  const fullTextBusy = useReaderStore((state) => state.fullTextBusy);
  const fetchFullText = useReaderStore((state) => state.fetchFullText);
  const summaryStreams = useReaderStore((state) => state.summaryStream);
  const translationStreams = useReaderStore((state) => state.translationStream);
  const generateSummary = useReaderStore((state) => state.generateSummary);
  const translate = useReaderStore((state) => state.translate);
  const toggleStar = useReaderStore((state) => state.toggleStar);
  const toggleLater = useReaderStore((state) => state.toggleLater);
  const toggleRead = useReaderStore((state) => state.toggleRead);
  const markRead = useReaderStore((state) => state.markRead);
  const settings = useSettingsStore((state) => state.settings);
  const sources = useSourcesStore((state) => state.sources);
  const notify = useUiStore((state) => state.notify);
  const [progress, setProgress] = useState(0);
  const [translateOpen, setTranslateOpen] = useState(false);
  const [notesOpen, setNotesOpen] = useState(false);
  const [markdownBusy, setMarkdownBusy] = useState(false);
  const scrollRef = useRef<HTMLDivElement>(null);
  const translateRef = useRef<HTMLSpanElement>(null);
  const articleRef = useRef<HTMLElement | null>(null);
  const restoredRef = useRef<number | null>(null);
  const savedQuoteRef = useRef('');
  const detail = details[itemId];
  const ready = Boolean(detail);
  const loading = useReaderStore((state) => state.detailLoading[itemId]);
  const source = sources.find((entry) => entry.id === detail?.source_id);
  const speech = useSpeech(detail?.content_text ?? '');
  const getQuote = () => {
    const selection = window.getSelection();
    if (selection && !selection.isCollapsed) {
      const text = selection.toString().trim();
      const anchor = selection.anchorNode;
      if (text && anchor && articleRef.current?.contains(anchor)) return text.slice(0, 4000);
    }
    return savedQuoteRef.current;
  };

  useEffect(() => {
    void loadDetail(itemId, true);
  }, [itemId, loadDetail]);

  useEffect(() => {
    setNotesOpen(false);
    savedQuoteRef.current = '';
  }, [itemId]);

  useEffect(() => {
    const element = scrollRef.current;
    if (!element) return undefined;
    const onScroll = () => {
      const max = element.scrollHeight - element.clientHeight;
      setProgress(max > 0 ? Math.min(1, element.scrollTop / max) : 0);
    };
    element.addEventListener('scroll', onScroll, { passive: true });
    onScroll();
    return () => element.removeEventListener('scroll', onScroll);
  }, [itemId, detail?.id]);

  useEffect(() => {
    if (!translateOpen) return undefined;
    const onPointerDown = (event: MouseEvent) => {
      if (translateRef.current && !translateRef.current.contains(event.target as Node)) {
        setTranslateOpen(false);
      }
    };
    document.addEventListener('mousedown', onPointerDown);
    return () => document.removeEventListener('mousedown', onPointerDown);
  }, [translateOpen]);

  useEffect(() => {
    if (!ready) return undefined;
    const element = scrollRef.current;
    if (!element) return undefined;
    if (!settings.reader.remember_position) {
      element.scrollTop = 0;
      restoredRef.current = itemId;
      return undefined;
    }
    const key = `rill.reader.position.${itemId}`;
    const saved = Number(window.localStorage.getItem(key) ?? 0);
    const frame = window.requestAnimationFrame(() => {
      element.scrollTop = Number.isFinite(saved) && saved > 0 ? saved : 0;
      restoredRef.current = itemId;
    });
    return () => window.cancelAnimationFrame(frame);
  }, [itemId, ready, settings.reader.remember_position]);

  useReadMarker({
    itemId,
    isRead: Boolean(detail?.read_at),
    enabled: settings.reader.auto_read && Boolean(detail),
    targetRef: articleRef,
    onMarkRead: (id) => { void markRead(id); },
    delayMs: 3000,
  });

  if (!detail && loading) {
    return (
      <div className="focus-screen">
        <div className="focus-bar">
          <div className="focus-bar-left">
            <span className="focus-bar-title">专注阅读</span>
          </div>
          <div className="focus-bar-right">
            <IconButton icon="minimize" label="退出全屏" onClick={onBack} />
          </div>
        </div>
        <div className="focus-scroll">
          <div className="article article-skeleton" aria-label="正在加载正文">
            <span className="skeleton-title"></span>
            <span className="skeleton-line"></span>
            <span className="skeleton-line short"></span>
            <span className="skeleton-block"></span>
          </div>
        </div>
      </div>
    );
  }

  if (!detail) {
    return (
      <div className="focus-screen">
        <EmptyState
          icon="alert-circle"
          title="正文加载失败"
          action={<Button icon="minimize" onClick={onBack}>退出全屏</Button>}
        />
      </div>
    );
  }

  const translation = detail.translation;
  const translatedTitle = translation?.title || translationStreams[itemId]?.title || '';
  const headingTitle = translationMode === 'translated' && translatedTitle ? translatedTitle : detail.title;
  const canTranslate = Boolean(translation) || settings.ai.enabled;
  const canGenerate = settings.ai.enabled;
  const disabledReason = settings.ai.enabled ? undefined : 'AI 能力已关闭，请先在设置中启用。';

  const scrollToTop = () => {
    scrollRef.current?.scrollTo({ top: 0, behavior: 'smooth' });
  };

  const retranslate = () => {
    void translate(detail.id, true);
  };

  const onFocusScroll = (event: UIEvent<HTMLDivElement>) => {
    if (!settings.reader.remember_position || restoredRef.current !== itemId) return;
    window.localStorage.setItem(
      `rill.reader.position.${itemId}`,
      String(Math.round(event.currentTarget.scrollTop)),
    );
  };

  const copyFeed = async () => {
    try {
      await copyFeedUrl(detail.source_id);
      notify('订阅地址已复制');
    } catch (error) {
      notify(error instanceof Error ? error.message : '复制失败');
    }
  };

  const openOriginal = () => {
    if (!detail.url) {
      notify('这篇内容没有可打开的源站地址');
      return;
    }
    window.open(detail.url, '_blank', 'noopener,noreferrer');
  };

  const copyMarkdown = async () => {
    setMarkdownBusy(true);
    try {
      await copyText(await api.getItemMarkdown(detail.id));
      notify('Markdown 已复制');
    } catch (error) {
      notify(error instanceof Error ? error.message : '复制 Markdown 失败');
    } finally {
      setMarkdownBusy(false);
    }
  };

  const saveMarkdown = async () => {
    setMarkdownBusy(true);
    try {
      downloadMarkdown(detail.title, await api.getItemMarkdown(detail.id));
      notify('Markdown 已开始下载');
    } catch (error) {
      notify(error instanceof Error ? error.message : '下载 Markdown 失败');
    } finally {
      setMarkdownBusy(false);
    }
  };

  return (
    <div className="focus-screen">
      <div className="focus-bar">
        <div className="focus-bar-left">
          <span className="focus-bar-title">专注阅读</span>
        </div>
        <div className="focus-bar-right">
          <IconButton icon="minimize" label="退出全屏" onClick={onBack} />
          {!isMobile ? (
            <span className="reader-translate" ref={translateRef}>
              <IconButton
                icon="languages"
                label={translation ? '翻译设置' : '翻译文章'}
                size={17}
                active={translateOpen || Boolean(translationBusy[detail.id])}
                aria-expanded={translateOpen}
                aria-haspopup="dialog"
                onClick={() => {
                  if (!canTranslate) {
                    setTranslateOpen(true);
                    return;
                  }
                  if (!translation) {
                    setTranslateOpen(true);
                    void translate(detail.id);
                    return;
                  }
                  setTranslateOpen((value) => !value);
                }}
              />
              {translateOpen ? (
                <div className="reader-translate-popover" role="dialog" aria-label="翻译">
                  {canTranslate ? (
                    <TranslationPanel
                      translation={translation}
                      mode={translationMode}
                      onMode={(mode) => {
                        setTranslationMode(mode);
                        setTranslateOpen(false);
                      }}
                      onTranslate={() => { void translate(detail.id); }}
                      onRetranslate={retranslate}
                      busy={Boolean(translationBusy[detail.id])}
                      canTranslate={canTranslate}
                      compact
                    />
                  ) : (
                    <p className="reader-translate-note">{disabledReason}</p>
                  )}
                </div>
              ) : null}
            </span>
          ) : null}
          {isMobile ? (
            <IconButton
              icon="arrow-up"
              label="回到顶部"
              size={17}
              disabled={progress <= 0.001}
              onClick={scrollToTop}
            />
          ) : null}
          <IconButton
            icon={speech.speaking && !speech.paused ? 'pause' : 'play'}
            label={speech.speaking ? (speech.paused ? '继续朗读' : '暂停朗读') : '朗读文章'}
            size={17}
            active={speech.speaking}
            disabled={!speech.supported}
            onClick={speech.toggle}
          />
          <IconButton
            icon="bookmark"
            label={detail.later_at ? '移出稍后读' : '加入稍后读'}
            size={17}
            filled={Boolean(detail.later_at)}
            active={Boolean(detail.later_at)}
            onClick={() => { void toggleLater(detail.id); }}
          />
          <IconButton icon="pencil" label="笔记与高亮" size={17} active={notesOpen} onClick={() => setNotesOpen(true)} />
          <IconButton
            icon="star"
            label={detail.starred_at ? '取消收藏' : '收藏'}
            size={17}
            filled={Boolean(detail.starred_at)}
            active={Boolean(detail.starred_at)}
            onClick={() => { void toggleStar(detail.id); }}
          />
          <IconButton
            icon={detail.read_at ? 'unread' : 'check'}
            label={detail.read_at ? '标为未读' : '标为已读'}
            size={17}
            onClick={() => { void toggleRead(detail.id); }}
          />
          <IconButton icon="rss" label="复制订阅地址" size={17} onClick={() => { void copyFeed(); }} />
          {source?.kind === 'rss' && detail.url ? (
            <IconButton
              icon={fullTextBusy[detail.id] ? 'refresh' : 'file-text'}
              label={fullTextBusy[detail.id] ? '正在抓取全文' : '抓取全文'}
              size={17}
              disabled={Boolean(fullTextBusy[detail.id])}
              onClick={() => { void fetchFullText(detail.id); }}
            />
          ) : null}
          <IconButton icon="copy" label="复制 Markdown" size={17} disabled={markdownBusy} onClick={() => { void copyMarkdown(); }} />
          <IconButton icon="download" label="下载 Markdown" size={17} disabled={markdownBusy} onClick={() => { void saveMarkdown(); }} />
          <IconButton icon="external" label="在源站打开" size={17} onClick={openOriginal} />
        </div>
        {isMobile ? (
          <span className="focus-progress" style={{ width: `${progress * 100}%` }} aria-hidden="true"></span>
        ) : null}
      </div>

      <div className="focus-scroll" ref={scrollRef} onScroll={onFocusScroll}>
        <div className="focus-grid">
          <article className="focus-article" ref={articleRef} onMouseUp={() => { const quote = getQuote(); if (quote) savedQuoteRef.current = quote; }}>
            <header className="article-head is-focus">
              <div className="article-kicker">
                <time>{formatListTime(detail.published_at)}</time>
                {detail.author ? <span className="article-author">{detail.author}</span> : null}
              </div>
              <h1 className="article-title">{headingTitle}</h1>
              {translationMode === 'bilingual' && translatedTitle ? (
                <p className="article-title-translation">{translatedTitle}</p>
              ) : null}
              {detail.original_title ? <p className="article-original">原文标题：{detail.original_title}</p> : null}
            </header>

            {detail.audio_url ? (
              <PodcastPlayer itemId={detail.id} src={detail.audio_url} type={detail.audio_type} initialPosition={detail.podcast_position_seconds} />
            ) : null}

            <AiSummary
              summary={detail.summary}
              streamText={summaryStreams[detail.id]}
              onGenerate={() => { void generateSummary(detail.id); }}
              busy={Boolean(summaryBusy[detail.id])}
              canGenerate={canGenerate}
              disabledReason={disabledReason}
            />

            <NotesPanel
              itemId={detail.id}
              notes={detail.notes ?? []}
              getQuote={getQuote}
              composerOpen={notesOpen}
              onComposerChange={setNotesOpen}
              later={Boolean(detail.later_at)}
              onToggleLater={() => { void toggleLater(detail.id); }}
              speechSupported={speech.supported}
              speaking={speech.speaking}
              paused={speech.paused}
              onToggleSpeech={speech.toggle}
            />

            {isMobile ? (
              <div className="focus-mobile-tools">
                <TranslationPanel
                  translation={translation}
                  mode={translationMode}
                  onMode={setTranslationMode}
                  onTranslate={() => { void translate(detail.id); }}
                  onRetranslate={retranslate}
                  busy={Boolean(translationBusy[detail.id])}
                  canTranslate={canTranslate}
                  compact
                />
              </div>
            ) : null}

            {translation && translationMode === 'bilingual' ? (
              <div className="article-bilingual">
                <div className="article-column">
                  <span className="article-column-label">译文</span>
                  <ArticleContent html={translation.content_html} highlights={detail.notes} />
                </div>
                <div className="article-column">
                  <span className="article-column-label">原文</span>
                  <ArticleContent html={detail.content_html} highlights={detail.notes} />
                </div>
              </div>
            ) : (
              <ArticleContent
                html={translation && translationMode === 'translated' ? translation.content_html : detail.content_html}
              />
            )}

          </article>
        </div>
      </div>

      {!isMobile ? (
        <aside className="focus-float-rail" aria-label="阅读进度">
          <div
            className="reader-progress-ring"
            role="progressbar"
            aria-label="文章阅读进度"
            aria-valuemin={0}
            aria-valuemax={100}
            aria-valuenow={Math.round(progress * 100)}
          >
            <svg viewBox="0 0 44 44" aria-hidden="true">
              <circle className="reader-progress-ring-track" cx="22" cy="22" r="18" />
              <circle
                className="reader-progress-ring-value"
                cx="22"
                cy="22"
                r="18"
                style={{
                  strokeDasharray: ringCircumference,
                  strokeDashoffset: ringCircumference * (1 - progress),
                }}
              />
            </svg>
          </div>
          <button
            type="button"
            className="reader-progress-top"
            onClick={scrollToTop}
            disabled={progress <= 0.001}
            aria-label="回到顶部"
            title="回到顶部"
          >
            <Icon name="arrow-up" size={20} />
          </button>
        </aside>
      ) : null}
    </div>
  );
};
