import { useState } from 'react';
import type { ItemNote } from '@/api/types';
import { Button, Icon, IconButton } from '@/components/ui';
import { useReaderStore } from '@/stores/readerStore';
import { cx } from '@/utils/cx';

const COLORS: Array<{ value: ItemNote['color']; label: string }> = [
  { value: 'yellow', label: '黄色' },
  { value: 'green', label: '绿色' },
  { value: 'blue', label: '蓝色' },
  { value: 'pink', label: '粉色' },
];

interface NotesPanelProps {
  itemId: number;
  notes: ItemNote[];
  getQuote: () => string;
  composerOpen: boolean;
  onComposerChange: (open: boolean) => void;
  later: boolean;
  onToggleLater: () => void;
  speechSupported: boolean;
  speaking: boolean;
  paused: boolean;
  onToggleSpeech: () => void;
}

export const NotesPanel = ({
  itemId,
  notes,
  getQuote,
  composerOpen,
  onComposerChange,
  later,
  onToggleLater,
  speechSupported,
  speaking,
  paused,
  onToggleSpeech,
}: NotesPanelProps) => {
  const createNote = useReaderStore((state) => state.createNote);
  const updateNote = useReaderStore((state) => state.updateNote);
  const deleteNote = useReaderStore((state) => state.deleteNote);
  const [quote, setQuote] = useState('');
  const [note, setNote] = useState('');
  const [color, setColor] = useState<ItemNote['color']>('yellow');
  const [editingId, setEditingId] = useState<number | null>(null);
  const [busy, setBusy] = useState(false);

  const openComposer = () => {
    setQuote(getQuote());
    setNote('');
    setColor('yellow');
    setEditingId(null);
    onComposerChange(true);
  };

  const closeComposer = () => {
    setQuote('');
    setNote('');
    setEditingId(null);
    onComposerChange(false);
  };

  const edit = (entry: ItemNote) => {
    setEditingId(entry.id);
    setQuote(entry.quote);
    setNote(entry.note);
    setColor(entry.color);
    onComposerChange(true);
  };

  const save = async () => {
    if (busy || (!quote.trim() && !note.trim())) return;
    setBusy(true);
    if (editingId != null) {
      await updateNote(itemId, editingId, { quote, note, color });
    } else {
      await createNote(itemId, { quote, note, color });
    }
    setBusy(false);
    closeComposer();
  };

  return (
    <section className="article-notes" aria-label="笔记与高亮">
      <div className="article-notes-head">
        <span><Icon name="bookmark" size={14} /> 笔记与高亮 {notes.length > 0 ? `(${notes.length})` : ''}</span>
        <div className="article-notes-tools">
          <IconButton
            icon="bookmark"
            label={later ? '移出稍后读' : '加入稍后读'}
            size={14}
            filled={later}
            active={later}
            onClick={onToggleLater}
          />
          {speechSupported ? (
            <IconButton
              icon={speaking && !paused ? 'pause' : 'play'}
              label={speaking ? (paused ? '继续朗读' : '暂停朗读') : '朗读文章'}
              size={14}
              active={speaking}
              onClick={onToggleSpeech}
            />
          ) : null}
          <button type="button" className="text-link" onClick={openComposer}>添加</button>
        </div>
      </div>
      {composerOpen ? (
        <div className="note-composer">
          <textarea
            value={quote}
            onChange={(event) => setQuote(event.target.value)}
            placeholder="选中正文后会自动填入摘录，也可以留空"
            aria-label="摘录"
            rows={2}
            maxLength={4000}
          />
          <textarea
            value={note}
            onChange={(event) => setNote(event.target.value)}
            placeholder="写下你的想法"
            aria-label="笔记"
            rows={3}
            maxLength={20000}
          />
          <div className="note-composer-foot">
            <div className="note-colors" role="group" aria-label="高亮颜色">
              {COLORS.map((entry) => (
                <button
                  type="button"
                  key={entry.value}
                  className={cx('note-color', `is-${entry.value}`, color === entry.value && 'is-active')}
                  aria-label={entry.label}
                  aria-pressed={color === entry.value}
                  onClick={() => setColor(entry.value)}
                />
              ))}
            </div>
            <Button size="sm" onClick={closeComposer}>取消</Button>
            <Button size="sm" variant="primary" disabled={busy || (!quote.trim() && !note.trim())} onClick={() => { void save(); }}>
              {busy ? '保存中' : '保存'}
            </Button>
          </div>
        </div>
      ) : null}
      {notes.length > 0 ? (
        <div className="note-list">
          {notes.map((entry) => (
            <article key={entry.id} className={cx('note-row', `is-${entry.color}`)}>
              {entry.quote ? <blockquote>{entry.quote}</blockquote> : null}
              {entry.note ? <p>{entry.note}</p> : null}
              <div className="note-row-actions">
                <IconButton icon="pencil" label="编辑笔记" size={13} onClick={() => edit(entry)} />
                <IconButton icon="trash" label="删除笔记" size={13} onClick={() => { void deleteNote(itemId, entry.id); }} />
              </div>
            </article>
          ))}
        </div>
      ) : null}
    </section>
  );
};
