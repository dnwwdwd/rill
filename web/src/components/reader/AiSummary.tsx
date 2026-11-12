import type { AiSummary as AiSummaryType } from '@/api/types';
import { Button, Icon, TextLink } from '@/components/ui';
import { cx } from '@/utils/cx';

interface AiSummaryProps {
  summary?: AiSummaryType;
  onGenerate: () => void;
  busy?: boolean;
  streamText?: string;
  canGenerate?: boolean;
  disabledReason?: string;
  className?: string;
}

export const AiSummary = ({
  summary,
  onGenerate,
  busy = false,
  streamText,
  canGenerate = true,
  disabledReason,
  className,
}: AiSummaryProps) => {
  const hasSummary = Boolean(summary);
  const streamPoints = (streamText ?? '')
    .split(/\r?\n/)
    .map((line) => line.replace(/^[-*\d.\s、)）]+/, '').trim())
    .filter(Boolean);
  const streaming = busy && streamPoints.length > 0;
  return (
    <section className={cx('ai-summary', !hasSummary && 'is-empty', className)}>
      <div className="ai-summary-head">
        <span className="ai-summary-icon"><Icon name="sparkles" size={16} /></span>
        <span className="ai-summary-label">AI 摘要</span>
        {streaming ? <span className="ai-stream-status">生成中</span> : null}
      </div>

      <div className="ai-summary-body">
        {busy ? (
          streaming ? (
            <ol className="ai-points is-streaming" aria-live="polite">
              {streamPoints.map((point, index) => (
                <li key={index} className="ai-stream-point">
                  {point}
                  {index === streamPoints.length - 1 ? <span className="ai-stream-caret" aria-hidden="true" /> : null}
                </li>
              ))}
            </ol>
          ) : (
            <div className="ai-skeleton" aria-label="正在生成摘要">
              <span></span><span></span><span></span>
            </div>
          )
        ) : summary ? (
          <>
            {summary.style === 'one' ? (
              <p className="ai-summary-one">{summary.points[0]}</p>
            ) : (
              <ol className="ai-points">
                {summary.points.map((point, index) => <li key={index}>{point}</li>)}
              </ol>
            )}
            <div className="ai-summary-foot">
              <TextLink onClick={onGenerate}>重新生成</TextLink>
            </div>
          </>
        ) : (
          <div className="ai-empty">
            <p>{disabledReason ?? '这篇内容还没有摘要。生成会调用你配置的 OpenAI 兼容接口。'}</p>
            <Button size="sm" variant="primary" icon="sparkles" onClick={onGenerate} disabled={!canGenerate || busy}>
              生成摘要
            </Button>
          </div>
        )}
      </div>
    </section>
  );
};
