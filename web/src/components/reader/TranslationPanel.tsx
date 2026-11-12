import type { Translation } from '@/api/types';
import { Button, Icon, Segmented, Tag, TextLink } from '@/components/ui';
import type { TranslationMode } from '@/types/ui';
import { cx } from '@/utils/cx';

interface TranslationPanelProps {
  translation?: Translation;
  mode: TranslationMode;
  onMode: (mode: TranslationMode) => void;
  onTranslate: () => void;
  onRetranslate?: () => void;
  busy?: boolean;
  canTranslate: boolean;
  compact?: boolean;
  className?: string;
}

export const TranslationPanel = ({
  translation,
  mode,
  onMode,
  onTranslate,
  onRetranslate = onTranslate,
  busy = false,
  canTranslate,
  compact = false,
  className,
}: TranslationPanelProps) => {
  if (!canTranslate) return null;

  if (!translation) {
    return (
      <section className={cx('translation-panel', compact && 'is-compact', className)}>
        <div className="translation-head">
          <div className="translation-title">
            <span className="translation-icon"><Icon name="languages" size={16} /></span>
            <span className="translation-label">翻译</span>
          </div>
          <Button size="sm" icon="languages" onClick={onTranslate} disabled={busy}>
            {busy ? '正在翻译' : '生成译文'}
          </Button>
        </div>
      </section>
    );
  }

  return (
    <section className={cx('translation-panel', compact && 'is-compact', className)}>
      <div className="translation-head">
        <div className="translation-title">
          <span className="translation-icon"><Icon name="languages" size={16} /></span>
          <span className="translation-label">翻译</span>
          <Tag tone="accent" icon="check">已生成</Tag>
        </div>
        <Segmented
          ariaLabel="翻译显示模式"
          value={mode}
          onChange={onMode}
          options={[
            { value: 'translated', label: '译文' },
            { value: 'original', label: '原文' },
            { value: 'bilingual', label: '双语' },
          ]}
        />
      </div>
      {translation.needs_retranslate ? (
        <div className="translation-stale">
          <Icon name="alert-triangle" size={14} />
          <span>正文已更新，当前译文可能过期。</span>
          <TextLink onClick={onRetranslate}>重新翻译</TextLink>
        </div>
      ) : (
        <div className="translation-foot">
          <TextLink onClick={onRetranslate}>重新翻译</TextLink>
        </div>
      )}
    </section>
  );
};
