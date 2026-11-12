import type { CSSProperties } from 'react';
import type { AccentColor } from '@/api/types';
import { Segmented, Switch } from '@/components/ui';
import { RowGroup, Row, SliderField } from '@/components/settings/SettingsSection';
import { useSettingsStore } from '@/stores/settingsStore';
import { cx } from '@/utils/cx';

const ACCENTS: Array<{ value: AccentColor; label: string; color: string }> = [
  { value: 'green', label: '微信绿', color: '#07C160' },
  { value: 'blue', label: '蓝色', color: '#3B82F6' },
  { value: 'cyan', label: '天青', color: '#06B6D4' },
  { value: 'teal', label: '青色', color: '#14B8A6' },
  { value: 'indigo', label: '靛蓝', color: '#6366F1' },
  { value: 'purple', label: '紫色', color: '#8B5CF6' },
  { value: 'pink', label: '粉色', color: '#EC4899' },
  { value: 'rose', label: '玫瑰', color: '#F43F5E' },
  { value: 'red', label: '红色', color: '#EF4444' },
  { value: 'orange', label: '橙色', color: '#F97316' },
  { value: 'amber', label: '琥珀', color: '#F59E0B' },
  { value: 'slate', label: '石墨', color: '#64748B' },
];

export const ReadingSettings = () => {
  const settings = useSettingsStore((state) => state.settings);
  const update = useSettingsStore((state) => state.update);
  const reader = settings.reader;

  return (
    <>
      <RowGroup title="外观">
        <Row
          label="主题"
          control={(
            <Segmented
              ariaLabel="阅读主题"
              value={reader.theme}
              onChange={(value) => update({ reader: { theme: value } })}
              options={[
                { value: 'light', label: '浅色' },
                { value: 'dark', label: '深色' },
                { value: 'system', label: '跟随系统' },
              ]}
            />
          )}
        />
        <Row
          label="主题色"
          control={(
            <div className="st-swatches" role="group" aria-label="主题色">
              {ACCENTS.map((accent) => (
                <button
                  type="button"
                  key={accent.value}
                  className={cx('st-swatch', reader.accent === accent.value && 'is-active')}
                  style={{ '--swatch-color': accent.color } as CSSProperties}
                  aria-label={`主题色：${accent.label}`}
                  aria-pressed={reader.accent === accent.value}
                  title={accent.label}
                  onClick={() => update({ reader: { accent: accent.value } })}
                />
              ))}
            </div>
          )}
        />
        <Row label="预览" className="st-preview">
          <p
            style={{
              fontSize: `${reader.font_size}px`,
              lineHeight: reader.line_height,
              maxWidth: `${reader.line_width}px`,
            }}
          >
            山川是不卷收的文章，日月为你掌灯伴读。
          </p>
        </Row>
      </RowGroup>

      <RowGroup title="排版">
        <SliderField
          label="正文字号"
          value={reader.font_size}
          min={15}
          max={24}
          suffix=" 像素"
          onChange={(value) => update({ reader: { font_size: value } })}
        />
        <SliderField
          label="行宽"
          value={reader.line_width}
          min={560}
          max={780}
          step={20}
          suffix=" 像素"
          onChange={(value) => update({ reader: { line_width: value } })}
        />
        <SliderField
          label="行高"
          value={reader.line_height}
          min={1.6}
          max={2}
          step={0.05}
          onChange={(value) => update({ reader: { line_height: Number(value.toFixed(2)) } })}
        />
      </RowGroup>

      <RowGroup title="行为">
        <Row
          label="停留标记已读"
          control={(
            <Switch
              checked={reader.auto_read}
              onChange={(value) => update({ reader: { auto_read: value } })}
              label="停留标记已读"
            />
          )}
        />
        <Row
          label="记住阅读位置"
          control={(
            <Switch
              checked={reader.remember_position}
              onChange={(value) => update({ reader: { remember_position: value } })}
              label="记住阅读位置"
            />
          )}
        />
      </RowGroup>
    </>
  );
};
