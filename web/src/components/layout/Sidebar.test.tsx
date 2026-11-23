import { afterEach, describe, expect, it, vi } from 'vitest';
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { createSeedData } from '@/api/mock/seed';
import { api } from '@/api/client';
import { Sidebar } from '@/components/layout/Sidebar';
import { useSettingsStore } from '@/stores/settingsStore';
import { useSourcesStore } from '@/stores/sourcesStore';

const seed = createSeedData();

const renderSidebar = () => render(
  <MemoryRouter>
    <Sidebar />
  </MemoryRouter>,
);

afterEach(() => {
  act(() => {
    useSourcesStore.setState({ folders: [], sources: [], loaded: false, health: {} });
    useSettingsStore.setState((state) => ({
      settings: { ...state.settings, reader: { ...state.settings.reader, pinned_sources: [] } },
    }));
  });
  vi.restoreAllMocks();
});

describe('Sidebar source tree', () => {
  it('opens source and folder context menus with edit/delete actions', () => {
    useSourcesStore.setState({ folders: seed.folders, sources: seed.sources, loaded: true });
    renderSidebar();

    const sourceRow = screen.getByText('少数派').closest('.source-row');
    expect(sourceRow).not.toBeNull();
    fireEvent.contextMenu(sourceRow!);
    expect(screen.getByRole('menuitem', { name: '编辑订阅源' })).toBeInTheDocument();
    expect(screen.getByRole('menuitem', { name: '删除订阅源' })).toBeInTheDocument();

    fireEvent.click(screen.getByRole('menuitem', { name: '移动到分组…' }));
    expect(screen.getByRole('menuitem', { name: '未分组' })).toBeInTheDocument();
    expect(screen.getByRole('menuitem', { name: '设计' })).toBeInTheDocument();

    fireEvent.click(screen.getByRole('menuitem', { name: '返回' }));
    fireEvent.click(screen.getByRole('menuitem', { name: '编辑订阅源' }));
    expect(screen.getByRole('heading', { name: '编辑订阅源' })).toBeInTheDocument();
  });

  it('asks for confirmation before deleting a folder', () => {
    useSourcesStore.setState({ folders: seed.folders, sources: seed.sources, loaded: true });
    renderSidebar();

    fireEvent.contextMenu(screen.getByText('技术').closest('.folder-row')!);
    fireEvent.click(screen.getByRole('menuitem', { name: '删除分组' }));

    expect(screen.getByRole('heading', { name: '删除分组？' })).toBeInTheDocument();
    expect(screen.getByText(/会移到未分组/)).toBeInTheDocument();
  });

  it('moves a dragged source into another folder', async () => {
    useSourcesStore.setState({ folders: seed.folders, sources: seed.sources, loaded: true });
    const source = seed.sources.find((entry) => entry.id === 8)!;
    const update = vi.spyOn(api, 'updateSource').mockResolvedValue({ ...source, folder_id: 1 });
    renderSidebar();

    const sourceRow = screen.getByText('设计周刊').closest('.source-row')!;
    const folderRow = screen.getByText('技术').closest('.folder-row')!;
    const dataTransfer = {
      effectAllowed: 'move',
      dropEffect: 'move',
      setData: vi.fn(),
      getData: vi.fn(),
    };

    fireEvent.dragStart(sourceRow, { dataTransfer });
    fireEvent.dragOver(folderRow, { dataTransfer });
    fireEvent.drop(folderRow, { dataTransfer });

    await waitFor(() => expect(update).toHaveBeenCalledWith(8, { folder_id: 1 }));
  });

  it('opens the watch dialog from the source context menu', async () => {
    useSourcesStore.setState({ folders: seed.folders, sources: seed.sources, loaded: true });
    renderSidebar();

    const sourceRow = screen.getByText('InfoQ 中文').closest('.source-row');
    expect(sourceRow).not.toBeNull();
    fireEvent.contextMenu(sourceRow!);
    fireEvent.click(screen.getByRole('menuitem', { name: '关注订阅源' }));

    expect(await screen.findByRole('heading', { name: '关注订阅源' })).toBeInTheDocument();
  });

  it('shows pinned watched sources in a global section with a dot', () => {
    act(() => {
      useSettingsStore.setState((state) => ({
        settings: { ...state.settings, reader: { ...state.settings.reader, pinned_sources: [1] } },
      }));
    });
    useSourcesStore.setState({
      folders: seed.folders,
      sources: seed.sources.map((source) => (
        source.id === 1 ? { ...source, watch_enabled: true, unread_count: 3 } : source
      )),
      loaded: true,
    });
    renderSidebar();

    expect(screen.getByText('置顶')).toBeInTheDocument();
    const sourceRow = screen.getByText('少数派').closest('.source-row');
    expect(sourceRow).not.toBeNull();
    expect(sourceRow?.querySelector('.source-watch-dot')).not.toBeNull();
  });

  it('filters the tree and enters multi-select with Ctrl/Cmd click', () => {
    useSourcesStore.setState({ folders: seed.folders, sources: seed.sources, loaded: true });
    renderSidebar();

    fireEvent.click(screen.getByRole('button', { name: '搜索订阅源' }));
    fireEvent.change(screen.getByRole('textbox', { name: '搜索订阅源' }), { target: { value: '设计周刊' } });
    expect(screen.getByText('设计周刊')).toBeInTheDocument();
    expect(screen.queryByText('少数派')).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: '关闭订阅源搜索' }));
    fireEvent.click(screen.getByText('少数派'), { ctrlKey: true });
    expect(screen.getByText('已选 1 源')).toBeInTheDocument();
  });
});
