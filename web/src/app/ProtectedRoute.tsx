import { Navigate, Outlet, useLocation } from 'react-router-dom';
import { useAuthStore } from '@/stores/authStore';

export const ProtectedRoute = () => {
  const status = useAuthStore((state) => state.status);
  const location = useLocation();

  // 会话恢复期间不渲染全局加载页（2026-11-10）：短暂空白，
  // 由 bootstrap 快速定性后进入阅读页骨架屏或登录页。
  if (status === 'idle' || status === 'loading') {
    return null;
  }

  if (status === 'unauthenticated') {
    return (
      <Navigate
        to="/login"
        replace
        state={{ from: `${location.pathname}${location.search}` }}
      />
    );
  }

  return <Outlet />;
};
