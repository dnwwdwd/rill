import { afterEach, describe, expect, it } from 'vitest';
import { act, fireEvent, render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { createSeedData } from '@/api/mock/seed';
import { SourcesSettings } from '@/components/settings/SourcesSettings';
import { useSourcesStore } from '@/stores/sourcesStore';

const seed = createSeedData();

const renderSettings = () => render(
  <MemoryRouter>
    <SourcesSettings />
  </MemoryRouter>,
);

afterEach(() => {
  act(() => {
    useSourcesStore.setState({ folders: [], sources: [], loaded: false, health: {} });
  });
});

describe('SourcesSettings tree', () => {
  it('renders folders and sources as a collapsible tree with row actions', () => {
    useSourcesStore.setState({ folders: seed.folders, sources: seed.sources, loaded: true });
    renderSettings();

    expect(screen.getByText('技术')).toBeInTheDocument();
    expect(screen.getByText('少数派')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '查看 少数派 详情' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '编辑或移动 少数派' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '删除 少数派' })).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: '收起分组 技术' }));
    expect(screen.queryByText('少数派')).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: '展开分组 技术' }));
    expect(screen.getByText('少数派')).toBeInTheDocument();
  });

  it('opens the source editor from the tree row', () => {
    useSourcesStore.setState({ folders: seed.folders, sources: seed.sources, loaded: true });
    renderSettings();

    fireEvent.click(screen.getByRole('button', { name: '编辑或移动 少数派' }));
    expect(screen.getByText('编辑订阅源')).toBeInTheDocument();
  });
});
