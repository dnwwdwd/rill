import { Suspense, useEffect } from 'react';
import { Outlet } from 'react-router-dom';
import { ToastHost } from '@/components/ui';
import { useAppTheme } from '@/hooks/useAppTheme';
import { useAuthStore } from '@/stores/authStore';
import { useSettingsStore } from '@/stores/settingsStore';

export const RootLayout = () => {
  const bootstrap = useAuthStore((state) => state.bootstrap);
  const settings = useSettingsStore((state) => state.settings);
  useAppTheme(settings);

  useEffect(() => {
    void bootstrap();
  }, [bootstrap]);

  return (
    <div className="app-shell">
      {/* 全局加载页已移除（2026-11-10）：页面内骨架屏负责加载态，
          路由分包加载期间渲染空白避免整屏 splash 闪烁。 */}
      <Suspense fallback={null}>
        <Outlet />
      </Suspense>
      <ToastHost />
    </div>
  );
};
