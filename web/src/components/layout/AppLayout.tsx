import { useEffect } from 'react';
import { Outlet, useLocation } from 'react-router-dom';
import { Drawer } from '@/components/layout/Drawer';
import { CommandPalette } from '@/components/layout/CommandPalette';
import { MobileNav } from '@/components/layout/MobileNav';
import { Sidebar } from '@/components/layout/Sidebar';
import { SettingsDialog } from '@/components/settings/SettingsDialog';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import { useAppTheme } from '@/hooks/useAppTheme';
import { useAuthStore } from '@/stores/authStore';
import { useReaderStore } from '@/stores/readerStore';
import { useSettingsStore } from '@/stores/settingsStore';
import { useSourcesStore } from '@/stores/sourcesStore';
import { useUiStore } from '@/stores/uiStore';

export const AppLayout = () => {
  const isMobile = useMediaQuery('(max-width: 760px)');
  const isTablet = useMediaQuery('(max-width: 1040px)');
  const location = useLocation();
  const readingDetail = /^\/items\/[^/]+/.test(location.pathname);
  const settings = useSettingsStore((state) => state.settings);
  const loadSettings = useSettingsStore((state) => state.load);
  const loadSources = useSourcesStore((state) => state.load);
  const loadStats = useReaderStore((state) => state.loadStats);
  const notify = useUiStore((state) => state.notify);
  useAppTheme(settings);

  useEffect(() => {
    void loadSettings();
    void loadSources();
    void loadStats();
  }, [loadSettings, loadSources, loadStats]);

  useEffect(() => {
    const onUnauthorized = () => {
      notify('会话已过期，请重新登录');
      void useAuthStore.getState().logout();
      useUiStore.getState().closeSettings();
      useReaderStore.getState().reset();
      useSourcesStore.getState().reset();
      useSettingsStore.getState().reset();
    };
    window.addEventListener('rill:unauthorized', onUnauthorized);
    return () => window.removeEventListener('rill:unauthorized', onUnauthorized);
  }, [notify]);

  return (
    <div className="app-layout">
      {!isTablet ? <Sidebar /> : null}
      <div className="app-main">
        <Outlet />
        {isMobile && !readingDetail ? <MobileNav /> : null}
      </div>
      {/* 平板用抽屉；手机使用 /subscribe 页面。 */}
      {isTablet && !isMobile ? <Drawer /> : null}
      {!isMobile ? <SettingsDialog /> : null}
      <CommandPalette />
    </div>
  );
};
