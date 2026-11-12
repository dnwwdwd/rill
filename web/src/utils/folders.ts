import type { Folder } from '@/api/types';

export interface FolderNode {
  folder: Folder;
  depth: number;
}

export const flattenFolders = (folders: Folder[]): FolderNode[] => {
  const byParent = new Map<number | null, Folder[]>();
  const known = new Set(folders.map((folder) => folder.id));
  for (const folder of folders) {
    const parent = folder.parent_id != null && known.has(folder.parent_id) ? folder.parent_id : null;
    const rows = byParent.get(parent) ?? [];
    rows.push(folder);
    byParent.set(parent, rows);
  }
  for (const rows of byParent.values()) {
    rows.sort((a, b) => a.sort - b.sort || a.id - b.id);
  }
  const result: FolderNode[] = [];
  const visited = new Set<number>();
  const walk = (parent: number | null, depth: number) => {
    for (const folder of byParent.get(parent) ?? []) {
      if (visited.has(folder.id)) continue;
      visited.add(folder.id);
      result.push({ folder, depth });
      walk(folder.id, depth + 1);
    }
  };
  walk(null, 0);
  // Defensive fallback for legacy/cyclic data: never lose a folder row.
  for (const folder of folders) {
    if (!visited.has(folder.id)) {
      visited.add(folder.id);
      result.push({ folder, depth: 0 });
      walk(folder.id, 1);
    }
  }
  return result;
};

export const folderOptionLabel = (node: FolderNode): string => (
  `${'　'.repeat(node.depth)}${node.folder.name}`
);

export const folderSubtreeIds = (folders: Folder[], rootId: number): Set<number> => {
  const children = new Map<number, number[]>();
  for (const folder of folders) {
    if (folder.parent_id == null) continue;
    const rows = children.get(folder.parent_id) ?? [];
    rows.push(folder.id);
    children.set(folder.parent_id, rows);
  }
  const result = new Set<number>();
  const walk = (id: number) => {
    if (result.has(id)) return;
    result.add(id);
    for (const child of children.get(id) ?? []) walk(child);
  };
  walk(rootId);
  return result;
};

export const folderAncestorIds = (folders: Folder[], id: number | null): number[] => {
  if (id == null) return [];
  const byId = new Map(folders.map((folder) => [folder.id, folder]));
  const result: number[] = [];
  const seen = new Set<number>();
  let current = byId.get(id);
  while (current && !seen.has(current.id)) {
    seen.add(current.id);
    result.push(current.id);
    current = current.parent_id != null ? byId.get(current.parent_id) : undefined;
  }
  return result;
};
