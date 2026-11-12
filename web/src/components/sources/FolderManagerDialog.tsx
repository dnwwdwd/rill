import { useEffect, useState } from 'react';
import { Button, Field, Icon, IconButton, Modal } from '@/components/ui';
import { useSourcesStore } from '@/stores/sourcesStore';
import { flattenFolders, folderOptionLabel, folderSubtreeIds } from '@/utils/folders';

interface FolderManagerDialogProps {
  open: boolean;
  onClose: () => void;
}

export const FolderManagerDialog = ({ open, onClose }: FolderManagerDialogProps) => {
  const folders = useSourcesStore((state) => state.folders);
  const createFolder = useSourcesStore((state) => state.createFolder);
  const renameFolder = useSourcesStore((state) => state.renameFolder);
  const removeFolder = useSourcesStore((state) => state.removeFolder);
  const [newName, setNewName] = useState('');
  const [newParent, setNewParent] = useState('none');
  const [editingId, setEditingId] = useState<number | null>(null);
  const [editName, setEditName] = useState('');
  const [editParent, setEditParent] = useState('none');
  const [confirmId, setConfirmId] = useState<number | null>(null);
  const [busy, setBusy] = useState(false);
  const folderNodes = flattenFolders(folders);
  const blockedParents = editingId == null ? new Set<number>() : folderSubtreeIds(folders, editingId);
  const editableParents = folderNodes.filter((node) => !blockedParents.has(node.folder.id));

  useEffect(() => {
    if (!open) return;
    setNewName('');
    setNewParent('none');
    setEditingId(null);
    setEditName('');
    setEditParent('none');
    setConfirmId(null);
    setBusy(false);
  }, [open]);

  const parentValue = (value: string): number | null => (
    value === 'none' ? null : Number(value)
  );

  const create = async () => {
    if (!newName.trim() || busy) return;
    setBusy(true);
    const created = await createFolder(newName.trim(), parentValue(newParent));
    setBusy(false);
    if (created) {
      setNewName('');
      setNewParent('none');
    }
  };

  const save = async (id: number) => {
    if (!editName.trim() || busy) return;
    setBusy(true);
    const updated = await renameFolder(id, editName.trim(), parentValue(editParent));
    setBusy(false);
    if (updated) {
      setEditingId(null);
      setEditName('');
      setEditParent('none');
    }
  };

  const remove = async (id: number) => {
    setBusy(true);
    await removeFolder(id);
    setBusy(false);
    setConfirmId(null);
  };

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="管理文件夹"
      subtitle="删除文件夹不会删除订阅源，源会回到「未分组」；支持多级文件夹。"
      footer={
        <>
          <span className="modal-foot-note">文件夹只影响订阅源的归类。</span>
          <Button onClick={onClose}>完成</Button>
        </>
      }
    >
      <div className="folder-manager">
        <div className="folder-manager-create">
          <Field label="新建文件夹">
            <input
              value={newName}
              onChange={(event) => setNewName(event.target.value)}
              onKeyDown={(event) => { if (event.key === 'Enter') void create(); }}
              placeholder="例如：播客、研究、每周必读"
              maxLength={200}
            />
          </Field>
          <Field label="上级文件夹">
            <select
              value={newParent}
              onChange={(event) => setNewParent(event.target.value)}
              aria-label="新文件夹的上级"
            >
              <option value="none">无（根目录）</option>
              {folderNodes.map((node) => (
                <option key={node.folder.id} value={node.folder.id}>{folderOptionLabel(node)}</option>
              ))}
            </select>
          </Field>
          <Button
            variant="primary"
            icon="plus"
            disabled={!newName.trim() || busy}
            onClick={() => { void create(); }}
          >
            新建
          </Button>
        </div>

        <div className="folder-manager-list">
          {folders.length === 0 ? (
            <div className="folder-manager-empty">
              <Icon name="folder" size={18} />
              <span>还没有文件夹。</span>
            </div>
          ) : null}

          {folderNodes.map(({ folder, depth }) => (
            <div
              className="folder-manager-row"
              key={folder.id}
              style={{ paddingLeft: 4 + depth * 15 }}
            >
              <span className="folder-manager-icon"><Icon name="folder" size={16} /></span>
              {editingId === folder.id ? (
                <div className="folder-manager-edit">
                  <input
                    className="folder-manager-input"
                    value={editName}
                    onChange={(event) => setEditName(event.target.value)}
                    onKeyDown={(event) => {
                      if (event.key === 'Enter') void save(folder.id);
                      if (event.key === 'Escape') { setEditingId(null); setEditName(''); setEditParent('none'); }
                    }}
                    autoFocus
                    maxLength={200}
                    aria-label={`重命名文件夹 ${folder.name}`}
                  />
                  <select
                    value={editParent}
                    onChange={(event) => setEditParent(event.target.value)}
                    aria-label={`设置 ${folder.name} 的上级文件夹`}
                  >
                    <option value="none">无（根目录）</option>
                    {editableParents.map((node) => (
                      <option key={node.folder.id} value={node.folder.id}>{folderOptionLabel(node)}</option>
                    ))}
                  </select>
                </div>
              ) : (
                <span className="folder-manager-name">{folder.name}</span>
              )}
              <span className="folder-manager-count">{folder.source_count} 个源</span>
              {editingId === folder.id ? (
                <div className="folder-manager-actions">
                  <IconButton icon="check" label="保存名称" size={15} disabled={busy || !editName.trim()} onClick={() => { void save(folder.id); }} />
                  <IconButton icon="x" label="取消重命名" size={15} onClick={() => { setEditingId(null); setEditName(''); setEditParent('none'); }} />
                </div>
              ) : confirmId === folder.id ? (
                <div className="folder-manager-actions">
                  <Button size="sm" variant="danger" disabled={busy} onClick={() => { void remove(folder.id); }}>删除</Button>
                  <Button size="sm" onClick={() => setConfirmId(null)}>取消</Button>
                </div>
              ) : (
                <div className="folder-manager-actions">
                  <Button
                    size="sm"
                    onClick={() => {
                      setEditingId(folder.id);
                      setEditName(folder.name);
                      setEditParent(folder.parent_id != null ? String(folder.parent_id) : 'none');
                    }}
                  >
                    重命名
                  </Button>
                  <IconButton icon="trash" label={`删除文件夹 ${folder.name}`} size={15} onClick={() => setConfirmId(folder.id)} />
                </div>
              )}
            </div>
          ))}
        </div>

        <p className="folder-manager-hint">
          <Icon name="info" size={13} /> 支持在添加源或编辑源时选择文件夹；删除父文件夹时子文件夹上移。
        </p>
      </div>
    </Modal>
  );
};
