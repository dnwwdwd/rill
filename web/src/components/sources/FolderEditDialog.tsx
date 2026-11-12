import { useEffect, useState } from 'react';
import type { Folder } from '@/api/types';
import { Button, Field, Modal } from '@/components/ui';
import { useSourcesStore } from '@/stores/sourcesStore';
import { flattenFolders, folderOptionLabel, folderSubtreeIds } from '@/utils/folders';

interface FolderEditDialogProps {
  open: boolean;
  folder: Folder | null;
  onClose: () => void;
}

export const FolderEditDialog = ({ open, folder, onClose }: FolderEditDialogProps) => {
  const folders = useSourcesStore((state) => state.folders);
  const renameFolder = useSourcesStore((state) => state.renameFolder);
  const removeFolder = useSourcesStore((state) => state.removeFolder);
  const [name, setName] = useState('');
  const [parent, setParent] = useState('none');
  const [confirming, setConfirming] = useState(false);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (!open || !folder) return;
    setName(folder.name);
    setParent(folder.parent_id != null ? String(folder.parent_id) : 'none');
    setConfirming(false);
    setBusy(false);
  }, [open, folder]);

  if (!folder) return null;

  const blocked = folderSubtreeIds(folders, folder.id);
  const parentOptions = flattenFolders(folders).filter((node) => !blocked.has(node.folder.id));

  const save = async () => {
    if (!name.trim() || busy) return;
    setBusy(true);
    const updated = await renameFolder(folder.id, name.trim(), parent === 'none' ? null : Number(parent));
    setBusy(false);
    if (updated) onClose();
  };

  const remove = async () => {
    if (busy) return;
    setBusy(true);
    await removeFolder(folder.id);
    setBusy(false);
    onClose();
  };

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="编辑分组"
      subtitle={folder.name}
      footer={
        confirming ? (
          <>
            <span className="modal-foot-note">分组内的源会移到未分组。</span>
            <Button onClick={() => setConfirming(false)}>取消</Button>
            <Button variant="danger" disabled={busy} onClick={() => { void remove(); }}>确认删除</Button>
          </>
        ) : (
          <>
            <Button variant="danger" icon="trash" onClick={() => setConfirming(true)}>删除分组</Button>
            <span className="modal-foot-note">保存后立即生效。</span>
            <Button onClick={onClose}>取消</Button>
            <Button variant="primary" onClick={() => { void save(); }} disabled={busy || !name.trim()}>
              {busy ? '正在保存' : '保存'}
            </Button>
          </>
        )
      }
    >
      <div className="edit-source-form">
        <Field label="分组名称">
          <input
            value={name}
            onChange={(event) => setName(event.target.value)}
            onKeyDown={(event) => { if (event.key === 'Enter') void save(); }}
            maxLength={200}
            autoFocus
            aria-label="分组名称"
          />
        </Field>
        <Field label="上级分组">
          <select value={parent} onChange={(event) => setParent(event.target.value)} aria-label="上级分组">
            <option value="none">无（根目录）</option>
            {parentOptions.map((node) => (
              <option key={node.folder.id} value={node.folder.id}>{folderOptionLabel(node)}</option>
            ))}
          </select>
        </Field>
      </div>
    </Modal>
  );
};
