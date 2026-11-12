import { useState } from 'react';
import { Navigate, useLocation } from 'react-router-dom';
import { Button, Wordmark } from '@/components/ui';
import { useAuthStore } from '@/stores/authStore';

const oidcErrorMessage = (code: string | null): string => {
  switch (code) {
    case 'oidc_access_denied':
      return '登录已取消。';
    case 'oidc_state_invalid':
      return '登录状态已过期，请重试。';
    default:
      return '懒猫账号登录未完成，请重试。';
  }
};

// 登录只有懒猫 OIDC 一条路径（DEC-017）：进入页面不自动跳转，点击按钮后发起。
export const LoginPage = () => {
  const location = useLocation();
  const status = useAuthStore((state) => state.status);
  const startOIDC = useAuthStore((state) => state.startOIDC);
  const [busy, setBusy] = useState(false);
  const from = (location.state as { from?: string } | null)?.from ?? '/';
  const oidcError = new URLSearchParams(location.search).get('oidc_error');

  if (status === 'authenticated') {
    return <Navigate to={from} replace />;
  }

  const signIn = async () => {
    setBusy(true);
    await startOIDC(from);
    setBusy(false);
  };

  return (
    <div className="login-screen">
      <section className="login-card" aria-label="登录 Rill">
        <img className="login-icon" src="/icon.png?v=3" alt="" width="72" height="72" />
        <Wordmark />
        <div className="login-oidc">
          {oidcError ? (
            <p className="form-error" role="alert">{oidcErrorMessage(oidcError)}</p>
          ) : null}
          <Button
            variant="primary"
            size="lg"
            icon="arrow-right"
            className="login-submit"
            disabled={busy || status === 'loading'}
            onClick={() => { void signIn(); }}
          >
            {busy ? '正在跳转' : oidcError ? '重新登录' : '使用懒猫账号登录'}
          </Button>
        </div>
      </section>
    </div>
  );
};
