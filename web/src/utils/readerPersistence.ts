import type { ReadStatusFilter, ReaderFilter } from '@/types/ui';

const FILTER_KEY = 'rill.reader.filter.v1';
const READ_FILTER_KEY = 'rill.reader.readFilter.v1';
const LAST_ITEM_KEY = 'rill.reader.lastItem.v1';

const positiveId = (value: unknown): number | null => (
  typeof value === 'number' && Number.isInteger(value) && value > 0 ? value : null
);

const readJSON = (key: string): unknown => {
  try {
    const raw = window.localStorage.getItem(key);
    return raw ? JSON.parse(raw) : null;
  } catch {
    return null;
  }
};

export const loadReaderFilter = (): ReaderFilter => {
  const value = readJSON(FILTER_KEY);
  if (!value || typeof value !== 'object') return { type: 'inbox' };
  const record = value as Record<string, unknown>;
  switch (record.type) {
    case 'inbox':
    case 'unread':
    case 'starred':
    case 'later':
    case 'read':
      return { type: record.type };
    case 'source': {
      const sourceId = positiveId(record.sourceId);
      return sourceId ? { type: 'source', sourceId } : { type: 'inbox' };
    }
    case 'folder': {
      const folderId = positiveId(record.folderId);
      return folderId ? { type: 'folder', folderId } : { type: 'inbox' };
    }
    default:
      return { type: 'inbox' };
  }
};

export const saveReaderFilter = (filter: ReaderFilter) => {
  try {
    window.localStorage.setItem(FILTER_KEY, JSON.stringify(filter));
  } catch {
    /* ignore storage errors */
  }
};

export const loadReadStatusFilter = (): ReadStatusFilter => {
  try {
    const value = window.localStorage.getItem(READ_FILTER_KEY);
    return value === 'unread' || value === 'read' ? value : 'all';
  } catch {
    return 'all';
  }
};

export const saveReadStatusFilter = (filter: ReadStatusFilter) => {
  try {
    window.localStorage.setItem(READ_FILTER_KEY, filter);
  } catch {
    /* ignore storage errors */
  }
};

export const loadLastItemId = (): number | null => {
  try {
    const value = Number(window.localStorage.getItem(LAST_ITEM_KEY));
    return Number.isInteger(value) && value > 0 ? value : null;
  } catch {
    return null;
  }
};

export const saveLastItemId = (id: number) => {
  try {
    if (Number.isInteger(id) && id > 0) window.localStorage.setItem(LAST_ITEM_KEY, String(id));
  } catch {
    /* ignore storage errors */
  }
};

export const clearReaderPersistence = () => {
  try {
    window.localStorage.removeItem(FILTER_KEY);
    window.localStorage.removeItem(READ_FILTER_KEY);
    window.localStorage.removeItem(LAST_ITEM_KEY);
  } catch {
    /* ignore storage errors */
  }
};
