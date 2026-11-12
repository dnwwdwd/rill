import { create } from 'zustand';
import { api, isMockMode } from '@/api/client';
import { type AuthMode } from '@/api/types';

type AuthStatus = 'idle' | 'loading' | 'authenticated' | 'unauthenticated';

interface AuthState {
  status: AuthStatus;
  csrfToken: string;
  error: string | null;
  mode: AuthMode;
  loginUrl: string;
  bootstrap: () => Promise<void>;
  startOIDC: (returnTo?: string) => Promise<void>;
  logout: () => Promise<void>;
  clearError: () => void;
}

let bootstrapPromise: Promise<void> | null = null;

export const useAuthStore = create<AuthState>((set, get) => ({
  status: 'idle',
  csrfToken: '',
  error: null,
  mode: 'oidc',
  loginUrl: '/auth/oidc/login',

  bootstrap: async () => {
    if (get().status === 'authenticated' || get().status === 'unauthenticated') return;
    if (bootstrapPromise) return bootstrapPromise;
    set({ status: 'loading' });
    bootstrapPromise = (async () => {
      try {
        const config = await api.getAuthConfig();
        set({ mode: config.mode, loginUrl: config.login_url ?? '' });
      } catch {
        // Keep the production default. The session request below will still
        // surface a readable error if the API is unavailable.
      }
      try {
        const session = await api.getSession();
        set({
          status: session.authenticated ? 'authenticated' : 'unauthenticated',
          csrfToken: session.csrf_token,
          error: null,
        });
      } catch {
        set({ status: 'unauthenticated', csrfToken: '', error: null });
      }
    })();
    await bootstrapPromise;
    bootstrapPromise = null;
  },

  startOIDC: async (returnTo = '/') => {
    const { loginUrl } = get();
    if (isMockMode) {
      // 演示会话：mock 客户端建立 sessionStorage 会话，等价于一次懒猫登录。
      const response = await api.signIn?.();
      set({ status: 'authenticated', csrfToken: response?.csrf_token ?? 'mock-csrf', error: null });
      return;
    }
    const target = new URL(loginUrl || '/auth/oidc/login', window.location.origin);
    target.searchParams.set('return_to', returnTo);
    window.location.assign(target.toString());
  },

  logout: async () => {
    try {
      await api.logout();
    } catch {
      // The session may already be gone; local state still needs clearing.
    } finally {
      set({ status: 'unauthenticated', csrfToken: '', error: null });
    }
  },

  clearError: () => set({ error: null }),
}));
