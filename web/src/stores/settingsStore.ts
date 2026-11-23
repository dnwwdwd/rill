import { create } from 'zustand';
import { api } from '@/api/client';
import type { AiTestResponse, Settings, SettingsPatch, WeReadScanStatus } from '@/api/types';
import { DEFAULT_SETTINGS, clampListWidth, clampRailWidth, mergeSettings } from '@/stores/defaults';
import { useSourcesStore } from '@/stores/sourcesStore';
import { useUiStore } from '@/stores/uiStore';

interface SettingsState {
  settings: Settings;
  loading: boolean;
  saving: boolean;
  loaded: boolean;
  error: string | null;
  load: (force?: boolean) => Promise<void>;
  update: (patch: SettingsPatch) => void;
  saveNow: () => Promise<void>;
  saveAiKey: (apiKey: string) => Promise<boolean>;
  saveEmailPassword: (accountId: string, password: string | null) => Promise<boolean>;
  setTheme: (theme: Settings['reader']['theme']) => void;
  setWechatEnabled: (enabled: boolean) => Promise<void>;
  startWereadScan: () => Promise<string>;
  pollWereadScanStatus: () => Promise<WeReadScanStatus>;
  scanWeread: () => Promise<void>;
  backupNow: () => Promise<void>;
  downloadBackup: () => Promise<void>;
  restoreBackup: (file?: File) => Promise<void>;
  testAiConnection: () => Promise<AiTestResponse>;
  reset: () => void;
}

const mergePatch = (left: SettingsPatch, right: SettingsPatch): SettingsPatch => ({
  reader: { ...left.reader, ...right.reader },
  ai: {
    ...left.ai,
    ...right.ai,
    ...(left.ai?.prompts || right.ai?.prompts
      ? { prompts: { ...left.ai?.prompts, ...right.ai?.prompts } }
      : {}),
  },
  wechat: { ...left.wechat, ...right.wechat },
  backup: { ...left.backup, ...right.backup },
  feed: { ...left.feed, ...right.feed },
  security: { ...left.security, ...right.security },
  mcp: { ...left.mcp, ...right.mcp },
  email: { ...left.email, ...right.email },
});

export const applySettingsToDocument = (settings: Settings) => {
  const root = document.documentElement;
  const media = window.matchMedia('(prefers-color-scheme: dark)');
  const effective = settings.reader.theme === 'system'
    ? (media.matches ? 'dark' : 'light')
    : settings.reader.theme;
  root.dataset.theme = effective;
  root.dataset.accent = settings.reader.accent || 'blue';
  root.style.setProperty('--reader-font-size', `${settings.reader.font_size}px`);
  root.style.setProperty('--reader-line-height', String(settings.reader.line_height));
  root.style.setProperty('--reader-line-width', `${settings.reader.line_width}px`);
  const railWidth = clampRailWidth(settings.reader.rail_width || 224);
  const listWidth = clampListWidth(settings.reader.list_width || 384);
  root.style.setProperty('--rail-width', `${railWidth}px`);
  root.style.setProperty('--list-width', `${listWidth}px`);
  try {
    localStorage.setItem('rill.theme', effective);
    localStorage.setItem('rill.theme.preference', settings.reader.theme);
    localStorage.setItem('rill.accent', settings.reader.accent || 'blue');
    localStorage.setItem('rill.rail_width', String(railWidth));
    localStorage.setItem('rill.list_width', String(listWidth));
  } catch {
    /* ignore privacy mode */
  }
};

let saveTimer: number | undefined;
let pendingPatch: SettingsPatch = {};
let revision = 0;

export const useSettingsStore = create<SettingsState>((set, get) => {
  const flushPending = async () => {
    window.clearTimeout(saveTimer);
    saveTimer = undefined;
    const patchToSave = pendingPatch;
    pendingPatch = {};
    if (Object.keys(patchToSave).length === 0) return;
    const currentRevision = revision;
    try {
      const saved = await api.updateSettings(patchToSave);
      if (revision === currentRevision) {
        set({ settings: saved, saving: false });
      } else {
        set({ saving: false });
      }
      // 邮件配置变化会影响源名称/分组/轮询：保存后立即刷新订阅源树，
      // 不要求用户刷新页面。
      const emailPatch = patchToSave.email;
      const affectsSources = emailPatch
        && ['enabled', 'accounts', 'password'].some((key) => key in emailPatch);
      if (affectsSources) {
        await useSourcesStore.getState().load(true);
      }
    } catch (error) {
      set({ saving: false, error: error instanceof Error ? error.message : '设置保存失败' });
      useUiStore.getState().notify('设置保存失败，请检查后端连接');
    }
  };

  return {
  settings: DEFAULT_SETTINGS,
  loading: false,
  saving: false,
  loaded: false,
  error: null,

  load: async (force = false) => {
    // force=true 表示扫码成功等关键回调；即使有在途加载也要确保拿到最新设置，
    // 否则 weread_connected 不会更新，界面会停留在「未连接」。
    if (get().loading && !force) return;
    if (get().loaded && !force) return;
    set({ loading: true, error: null });
    try {
      const settings = await api.getSettings();
      set({ settings, loading: false, loaded: true });
      applySettingsToDocument(settings);
    } catch (error) {
      set({ loading: false, error: error instanceof Error ? error.message : '设置加载失败' });
      applySettingsToDocument(get().settings);
    }
  },

  update: (patch) => {
    if (!patch) return;
    revision += 1;
    const next = mergeSettings(get().settings, patch);
    set({ settings: next, saving: true, error: null });
    applySettingsToDocument(next);
    pendingPatch = mergePatch(pendingPatch, patch);
    window.clearTimeout(saveTimer);
    saveTimer = window.setTimeout(() => { void flushPending(); }, 350);
  },

  // Flush the debounced PATCH immediately. Used after a successful email
  // connection test so the source tree reflects new grouping/errors right away
  // instead of waiting for a page refresh.
  saveNow: flushPending,

  saveAiKey: async (apiKey) => {
    set({ saving: true });
    try {
      const settings = await api.saveAiKey(apiKey);
      set({ settings, saving: false });
      useUiStore.getState().notify('接口密钥已加密保存');
      return true;
    } catch (error) {
      set({ saving: false });
      useUiStore.getState().notify(error instanceof Error ? error.message : '密钥保存失败');
      return false;
    }
  },

  // REQ-041：授权码只写不读，保存/清除成功后以后端返回的 password_set 为准。
  saveEmailPassword: async (accountId, password) => {
    set({ saving: true });
    try {
      const saved = await api.saveEmailPassword(accountId, password);
      set({ settings: saved, saving: false });
      // 授权码变化可能触发邮件源创建/恢复，源树立即刷新。
      await useSourcesStore.getState().load(true);
      useUiStore.getState().notify(password == null ? '授权码已清除' : '授权码已加密保存');
      return true;
    } catch (error) {
      set({ saving: false });
      useUiStore.getState().notify(error instanceof Error ? error.message : '授权码保存失败');
      return false;
    }
  },

  setTheme: (theme) => {
    get().update({ reader: { theme } });
  },

  setWechatEnabled: async (enabled) => {
    set({ saving: true });
    try {
      const saved = await api.updateSettings({ wechat: { enabled } });
      set({ settings: saved, saving: false });
      await useSourcesStore.getState().load(true);
      useUiStore.getState().notify(enabled ? '公众号采集已启用' : '公众号采集已关闭，其他源不受影响');
    } catch (error) {
      set({ saving: false });
      useUiStore.getState().notify(error instanceof Error ? error.message : '公众号设置保存失败');
    }
  },

  // 微信读书扫码：二维码由 Rill 后端生成；登录 Cookie 加密落库。
  startWereadScan: async () => {
    const { qrcode } = await api.scanWeread();
    return qrcode;
  },

  // 轮询微信读书扫码状态；成功后立即刷新设置与订阅源，
  // 这样「已连接」状态和公众号源抓取马上生效，不再停在扫码界面。
  pollWereadScanStatus: async () => {
    const status = await api.wereadScanStatus();
    if (status.state !== 'done') return status;
    await get().load(true);
    await useSourcesStore.getState().load(true);
    useUiStore.getState().notify('微信读书已连接，公众号源将通过微信读书抓取');
    return status;
  },

  // 仅 mock 模式：点击占位二维码模拟扫码完成。
  scanWeread: async () => {
    try {
      await api.scanWeread();
      await get().load(true);
      await useSourcesStore.getState().load(true);
      useUiStore.getState().notify('微信读书已连接');
    } catch (error) {
      useUiStore.getState().notify(error instanceof Error ? error.message : '扫码失败');
    }
  },

  downloadBackup: async () => {
    await api.downloadBackup();
  },

  backupNow: async () => {
    try {
      const backup = await api.backupNow();
      set((state) => ({ settings: { ...state.settings, backup } }));
      useUiStore.getState().notify('备份完成：数据库已写入持久化目录');
    } catch (error) {
      useUiStore.getState().notify(error instanceof Error ? error.message : '备份失败');
    }
  },

  restoreBackup: async (file) => {
    try {
      await api.restoreBackup(file);
      useUiStore.getState().notify('已提交恢复请求，请等待校验完成');
    } catch (error) {
      useUiStore.getState().notify(error instanceof Error ? error.message : '恢复失败');
    }
  },

  testAiConnection: async () => api.testAiConnection(),

  reset: () => {
    window.clearTimeout(saveTimer);
    pendingPatch = {};
    revision += 1;
    const next = mergeSettings(DEFAULT_SETTINGS, { reader: { theme: get().settings.reader.theme } });
    set({ settings: next, loading: false, saving: false, loaded: false, error: null });
    applySettingsToDocument(next);
  },
  };
});
