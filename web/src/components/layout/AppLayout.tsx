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
  const authStatus = useAuthStore((state) => state.status);
  useAppTheme(settings);

  useEffect(() => {
    void loadSettings();
    void loadSources();
    void loadStats();
  }, [loadSettings, loadSources, loadStats]);

  // 关注源的未读 dot 依赖源列表的 unread_count；定时抓取发生在服务端，
  // 页面需要轻量轮询才能在保持打开时看到新 dot。
  useEffect(() => {
    if (authStatus !== 'authenticated') return;
    const refresh = () => {
      if (document.visibilityState !== 'visible') return;
      void useSourcesStore.getState().load(true);
      void useReaderStore.getState().loadStats();
    };
    const timer = window.setInterval(refresh, 60_000);
    document.addEventListener('visibilitychange', refresh);
    return () => {
      window.clearInterval(timer);
      document.removeEventListener('visibilitychange', refresh);
    };
  }, [authStatus]);

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
