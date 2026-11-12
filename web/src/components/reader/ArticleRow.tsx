import { useEffect, useState, type MouseEvent as ReactMouseEvent } from 'react';
import type { ItemSummary } from '@/api/types';
import { Icon, SourceIcon } from '@/components/ui';
import { useSourcesStore } from '@/stores/sourcesStore';
import { formatListTime } from '@/utils/date';
import { cx } from '@/utils/cx';

interface ArticleRowProps {
  item: ItemSummary;
  selected: boolean;
  selecting?: boolean;
  checked?: boolean;
  duplicateSources?: string[];
  onToggleSelect?: () => void;
  onSelect: (item: ItemSummary, event?: ReactMouseEvent<HTMLElement>) => void;
  onOpenFocus: (item: ItemSummary) => void;
}

export const ArticleRow = ({
  item,
  selected,
  selecting = false,
  checked = false,
  duplicateSources = [],
  onToggleSelect,
  onSelect,
  onOpenFocus,
}: ArticleRowProps) => {
  const source = useSourcesStore((state) => state.sources.find((entry) => entry.id === item.source_id));
  const sourceForIcon = source ?? item.source;
  const unread = !item.read_at;
  const starred = Boolean(item.starred_at);
  const later = Boolean(item.later_at);
  const [coverFailed, setCoverFailed] = useState(false);

  useEffect(() => {
    setCoverFailed(false);
  }, [item.cover_url]);

  return (
    <article
      className={cx('item-row', selected && 'is-selected', unread && 'is-unread', selecting && 'is-selecting', checked && 'is-checked')}
      onClick={(event) => onSelect(item, event)}
      onDoubleClick={() => onOpenFocus(item)}
      tabIndex={0}
      onKeyDown={(event) => {
        if (event.key === 'Enter') onSelect(item);
      }}
      aria-current={selected ? 'true' : undefined}
    >
      {selecting ? (
        <button
          type="button"
          className={cx('item-select', checked && 'is-checked')}
          aria-label={`${checked ? '取消选择' : '选择'} ${item.title}`}
          aria-pressed={checked}
          onClick={(event) => {
            event.stopPropagation();
            onToggleSelect?.();
          }}
        >
          <Icon name={checked ? 'check' : 'unread'} size={12} />
        </button>
      ) : null}
      <div className="item-body">
        <div className="item-main">
          <div className="item-top">
            <span className="item-unread-dot" aria-hidden="true"></span>
            <span className="item-source">
              <SourceIcon source={sourceForIcon} size={21} />
              <span>{item.source.name}</span>
            </span>
            {starred ? <span className="item-star-mark" title="已收藏"><Icon name="star" size={12} filled /></span> : null}
            {later ? <span className="item-star-mark" title="稍后读"><Icon name="bookmark" size={12} filled /></span> : null}
            {duplicateSources.length > 0 ? (
              <span className="item-duplicate-mark" title={`另有：${duplicateSources.join('、')}`}>
                +{duplicateSources.length} 源
              </span>
            ) : null}
            <time className="item-time">{formatListTime(item.published_at)}</time>
          </div>
          <div className="item-title-row">
            <h2 className="item-title">{item.title}</h2>
            {item.cover_url && !coverFailed ? (
              <div className="item-cover">
                <img
                  src={item.cover_url}
                  alt=""
                  loading="lazy"
                  decoding="async"
                  onError={() => setCoverFailed(true)}
                />
              </div>
            ) : null}
          </div>
          <p className="item-excerpt">{item.excerpt}</p>
        </div>
      </div>
    </article>
  );
};
