import { create } from 'zustand';
import { SELECTABLE_SETTINGS, type SettingsSection, type ToastState } from '@/types/ui';

interface UiState {
  toast: ToastState | null;
  drawerOpen: boolean;
  settingsOpen: boolean;
  settingsSection: SettingsSection;
  notify: (message: string, action?: { label: string; onClick: () => void }) => void;
  clearToast: () => void;
  openDrawer: () => void;
  closeDrawer: () => void;
  openSettings: (section?: SettingsSection) => void;
  closeSettings: () => void;
  setSettingsSection: (section: SettingsSection) => void;
}

let toastTimer: number | undefined;

// 设置分类：首次打开默认订阅源，之后记住上次停留的分类。
const SETTINGS_SECTION_KEY = 'rill.settings.section';

const isSettingsSection = (value: string | null): value is SettingsSection =>
  Boolean(value && (SELECTABLE_SETTINGS as readonly string[]).includes(value));

const storedSection = (() => {
  try {
    const value = localStorage.getItem(SETTINGS_SECTION_KEY);
    if (isSettingsSection(value)) return value;
  } catch {
    /* ignore storage errors */
  }
  return 'sources' as const;
})();

const rememberSection = (section: SettingsSection) => {
  try {
    localStorage.setItem(SETTINGS_SECTION_KEY, section);
  } catch {
    /* ignore storage errors */
  }
};

export const useUiStore = create<UiState>((set) => ({
  toast: null,
  drawerOpen: false,
  settingsOpen: false,
  settingsSection: storedSection,

  notify: (message, action) => {
    window.clearTimeout(toastTimer);
    set({
      toast: {
        id: Date.now(),
        message,
        actionLabel: action?.label,
        onAction: action?.onClick,
      },
    });
    toastTimer = window.setTimeout(() => set({ toast: null }), action ? 6000 : 3000);
  },

  clearToast: () => {
    window.clearTimeout(toastTimer);
    set({ toast: null });
  },

  openDrawer: () => set({ drawerOpen: true }),
  closeDrawer: () => set({ drawerOpen: false }),
  openSettings: (section) => set((state) => {
    const next = section ?? state.settingsSection;
    rememberSection(next);
    return { settingsOpen: true, settingsSection: next };
  }),
  closeSettings: () => set({ settingsOpen: false }),
  setSettingsSection: (settingsSection) => {
    rememberSection(settingsSection);
    set({ settingsSection });
  },
}));
