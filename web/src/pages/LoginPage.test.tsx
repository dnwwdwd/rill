import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { LoginPage } from '@/pages/LoginPage';
import { useAuthStore } from '@/stores/authStore';

const renderLogin = (entry: string) => render(
  <MemoryRouter initialEntries={[entry]}>
    <Routes>
      <Route path="/login" element={<LoginPage />} />
    </Routes>
  </MemoryRouter>,
);

describe('LoginPage', () => {
  it('uses the shared brand icon, renders the OIDC button only, and maps provider errors', async () => {
    const startOIDC = vi.fn(async () => undefined);
    useAuthStore.setState({
      status: 'unauthenticated',
      mode: 'oidc',
      loginUrl: '/auth/oidc/login',
      error: null,
      startOIDC,
    });

    const { container } = renderLogin('/login?oidc_error=oidc_exchange_failed');

    expect(container.querySelector('.login-icon')).toHaveAttribute('src', '/icon.png?v=3');
    expect(container.textContent).not.toMatch(/0\.1/);
    expect(container.textContent).not.toContain('把散落的订阅');
    expect(screen.queryByText('访问密码')).not.toBeInTheDocument();
    expect(screen.getByText('懒猫账号登录未完成，请重试。')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: '重新登录' }));
    await waitFor(() => expect(startOIDC).toHaveBeenCalledWith('/'));
  });

  it('does not auto-redirect on mount; login starts only from the button', () => {
    const startOIDC = vi.fn(async () => undefined);
    useAuthStore.setState({
      status: 'unauthenticated',
      mode: 'oidc',
      loginUrl: '/auth/oidc/login',
      error: null,
      startOIDC,
    });

    renderLogin('/login');

    expect(startOIDC).not.toHaveBeenCalled();
    expect(screen.getByRole('button', { name: '使用懒猫账号登录' })).toBeInTheDocument();
  });
});
