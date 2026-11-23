import { useEffect, useRef, useState, type UIEvent } from 'react';
import { useNavigate } from 'react-router-dom';
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
  SourceIcon,
} from '@/components/ui';
import { useReadMarker } from '@/hooks/useReadMarker';
import { useSpeech } from '@/hooks/useSpeech';
import { useReaderStore } from '@/stores/readerStore';
import { useSettingsStore } from '@/stores/settingsStore';
import { useSourcesStore } from '@/stores/sourcesStore';
import { useUiStore } from '@/stores/uiStore';
import { copyFeedUrl, copyText } from '@/utils/clipboard';
import { formatListTime } from '@/utils/date';
import { downloadMarkdown } from '@/utils/markdownExport';
import { cx } from '@/utils/cx';

interface ReaderPaneProps {
  itemId: number | null;
  isMobile: boolean;
  onOpenFocus: (id: number) => void;
}

const ringCircumference = 2 * Math.PI * 18;

export const ReaderPane = ({ itemId, isMobile, onOpenFocus }: ReaderPaneProps) => {
  const navigate = useNavigate();
  const details = useReaderStore((state) => state.details);
  const detailLoading = useReaderStore((state) => state.detailLoading);
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
  const allowEmailImages = useReaderStore((state) => state.allowEmailImages);
  const sources = useSourcesStore((state) => state.sources);
  const settings = useSettingsStore((state) => state.settings);
  const notify = useUiStore((state) => state.notify);
  const [translateOpen, setTranslateOpen] = useState(false);
  const [notesOpen, setNotesOpen] = useState(false);
  const [markdownBusy, setMarkdownBusy] = useState(false);
  const [progress, setProgress] = useState(0);
  const scrollRef = useRef<HTMLDivElement>(null);
  const articleRef = useRef<HTMLElement | null>(null);
  const translateRef = useRef<HTMLSpanElement>(null);
  const restoredRef = useRef<number | null>(null);
  const savedQuoteRef = useRef('');

  const detail = itemId ? details[itemId] : undefined;
  const ready = Boolean(detail);
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
  const loading = itemId ? detailLoading[itemId] : false;
  const source = detail ? sources.find((entry) => entry.id === detail.source_id) ?? detail.source : undefined;
  const isEmail = Boolean(detail?.source_feed_url?.startsWith('imap://'));

  useEffect(() => {
    if (itemId) void loadDetail(itemId, true);
  }, [itemId, loadDetail]);

  useEffect(() => {
    setTranslateOpen(false);
    setNotesOpen(false);
    savedQuoteRef.current = '';
    setProgress(0);
    restoredRef.current = null;
  }, [itemId]);

  useEffect(() => {
    if (!itemId || !ready) return undefined;
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

  useEffect(() => {
    const element = scrollRef.current;
    if (!element) return undefined;
    const update = () => {
      const max = element.scrollHeight - element.clientHeight;
      setProgress(max > 0 ? Math.min(1, Math.max(0, element.scrollTop / max)) : 0);
    };
    const frame = window.requestAnimationFrame(update);
    element.addEventListener('scroll', update, { passive: true });
    const observer = typeof ResizeObserver === 'undefined' ? null : new ResizeObserver(update);
    observer?.observe(element);
    return () => {
      window.cancelAnimationFrame(frame);
      element.removeEventListener('scroll', update);
      observer?.disconnect();
    };
  }, [itemId, detail?.id, translationMode, isMobile]);

  const scrollToTop = () => {
    scrollRef.current?.scrollTo({ top: 0, behavior: 'smooth' });
  };

  const retranslate = () => {
    if (detail) void translate(detail.id, true);
  };

  const onReaderScroll = (event: UIEvent<HTMLDivElement>) => {
    if (!itemId || !settings.reader.remember_position || restoredRef.current !== itemId) return;
    window.localStorage.setItem(
      `rill.reader.position.${itemId}`,
      String(Math.round(event.currentTarget.scrollTop)),
    );
  };

  useEffect(() => {
    if (!translateOpen) return undefined;
    const onPointerDown = (event: MouseEvent) => {
      if (translateRef.current && !translateRef.current.contains(event.target as Node)) {
        setTranslateOpen(false);
      }
    };
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setTranslateOpen(false);
    };
    document.addEventListener('mousedown', onPointerDown);
    window.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('mousedown', onPointerDown);
      window.removeEventListener('keydown', onKey);
    };
  }, [translateOpen]);

  useReadMarker({
    itemId,
    isRead: Boolean(detail?.read_at),
    enabled: settings.reader.auto_read && Boolean(detail),
    targetRef: articleRef,
    onMarkRead: (id) => { void markRead(id); },
    delayMs: 3000,
  });

  if (!itemId) {
    return (
      <section className="reader-pane">
        <EmptyState icon="inbox" title="没有内容" />
      </section>
    );
  }

  if (!detail && loading) {
    return (
      <section className="reader-pane">
        <div className="reader-scroll">
          <div className="article article-skeleton" aria-label="正在加载正文">
            <span className="skeleton-kicker"></span>
            <span className="skeleton-title"></span>
            <span className="skeleton-line"></span>
            <span className="skeleton-line short"></span>
            <span className="skeleton-block"></span>
            <span className="skeleton-line"></span>
            <span className="skeleton-line"></span>
            <span className="skeleton-line short"></span>
          </div>
        </div>
      </section>
    );
  }

  if (!detail) {
    return (
      <section className="reader-pane">
        <EmptyState
          icon="alert-circle"
          title="正文加载失败"
          action={<Button size="sm" icon="refresh" onClick={() => { void loadDetail(itemId, true); }}>重试</Button>}
        />
      </section>
    );
  }

  const translation = detail.translation;
  const translationStream = translationStreams[detail.id];
  const translatedTitle = translation?.title || translationStream?.title || '';
  const headingTitle = translationMode === 'translated' && translatedTitle ? translatedTitle : detail.title;
  const streamingTranslation = Boolean(
    translationBusy[detail.id]
    && translationStream
    && translationStream.segments.some((segment) => Boolean(segment)),
  );
  const canTranslate = Boolean(translation) || settings.ai.enabled;
  const canGenerate = settings.ai.enabled;
  const disabledReason = settings.ai.enabled ? undefined : 'AI 能力已关闭，请先在设置中启用。';
  const sourceForIcon = source ?? detail.source;

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
    <section className={cx('reader-pane', isMobile && 'is-mobile')} aria-label="阅读区">
      {!isMobile ? (
        <div className="reader-bar">
          <div className="reader-bar-left">
            <span className="reader-crumb">阅读</span>
            <span className="crumb-sep">/</span>
            <span className="reader-bar-source">{detail.source.name}</span>
          </div>
          <div className="reader-bar-right">
            <div className="reader-actions">
              <IconButton icon="focus" label="专注阅读" size={17} onClick={() => onOpenFocus(detail.id)} />
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
              <IconButton icon="external" label="在源站打开" size={17} onClick={openOriginal} />
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
              <IconButton icon="rss" label="复制订阅地址" size={17} onClick={() => { void copyFeed(); }} />
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
            </div>
          </div>
        </div>
      ) : null}

      <div className="reader-scroll" ref={scrollRef} onScroll={onReaderScroll}>
        <article className="article" ref={articleRef} onMouseUp={() => { const quote = getQuote(); if (quote) savedQuoteRef.current = quote; }}>
          <header className="article-head">
            <div className="article-kicker">
              <button
                type="button"
                className="article-source-link"
                onClick={() => navigate(`/sources/${detail.source_id}`)}
                title="查看源诊断"
              >
                <SourceIcon source={sourceForIcon} size={30} />
                <span className="article-source-name">{detail.source.name}</span>
              </button>
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
            <TranslationPanel
              translation={translation}
              mode={translationMode}
              onMode={setTranslationMode}
              onTranslate={() => { void translate(detail.id); }}
              onRetranslate={retranslate}
              busy={Boolean(translationBusy[detail.id])}
              canTranslate={canTranslate}
            />
          ) : null}

          {streamingTranslation ? (
            <div className="translation-stream" aria-live="polite">
              <div className="translation-stream-head">
                <span className="translation-stream-dot" aria-hidden="true" />
                正在翻译
                <span className="translation-stream-count">
                  {translationStream?.done ?? 0}/{translationStream?.total || '…'}
                </span>
              </div>
              {translationStream?.segments.filter(Boolean).map((segment, index) => (
                <p key={index} className="translation-stream-line">{segment}</p>
              ))}
              <span className="translation-stream-caret" aria-hidden="true" />
            </div>
          ) : translation && translationMode === 'bilingual' ? (
            <div className="article-bilingual">
              <div className="article-column">
                <span className="article-column-label">译文</span>
                <ArticleContent html={translation.content_html} highlights={detail.notes} isEmail={isEmail} senderEmail={detail.sender_email} imagesAllowed={detail.email_images_allowed} onAllowImages={() => { void allowEmailImages(detail.id); }} />
              </div>
              <div className="article-column">
                <span className="article-column-label">原文</span>
                <ArticleContent html={detail.content_html} highlights={detail.notes} isEmail={isEmail} senderEmail={detail.sender_email} />
              </div>
            </div>
          ) : (
            <ArticleContent
              html={translation && translationMode === 'translated' ? translation.content_html : detail.content_html}
              highlights={detail.notes}
              isEmail={isEmail}
              senderEmail={detail.sender_email}
              imagesAllowed={detail.email_images_allowed}
              onAllowImages={() => { void allowEmailImages(detail.id); }}
            />
          )}
        </article>
      </div>

      {!isMobile ? (
        <aside className="reader-progress-rail" aria-label="阅读进度">
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
    </section>
  );
};
