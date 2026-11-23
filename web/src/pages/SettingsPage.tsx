import { useEffect } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { Icon } from '@/components/ui';
import { MobileTopbar } from '@/components/layout/MobileTopbar';
import { SourcesSettings } from '@/components/settings/SourcesSettings';
import { ReadingSettings } from '@/components/settings/ReadingSettings';
import { AiSettings } from '@/components/settings/AiSettings';
import { EmailSettings } from '@/components/settings/EmailSettings';
import { AutomationSettings } from '@/components/settings/AutomationSettings';
import { McpSettings } from '@/components/settings/McpSettings';
import { BackupSettings } from '@/components/settings/BackupSettings';
import { SecuritySettings } from '@/components/settings/SecuritySettings';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import { useUiStore } from '@/stores/uiStore';
import { SETTINGS_SECTIONS, SELECTABLE_SETTINGS, type SettingsSection } from '@/types/ui';

const isSettingsSection = (value: string | undefined): value is SettingsSection =>
  Boolean(value && (SELECTABLE_SETTINGS as readonly string[]).includes(value));

const sections: Record<SettingsSection, () => React.JSX.Element> = {
  sources: SourcesSettings,
  reading: ReadingSettings,
  ai: AiSettings,
  email: EmailSettings,
  automation: AutomationSettings,
  mcp: McpSettings,
  backup: BackupSettings,
  security: SecuritySettings,
};

// REQ-023：移动端设置是两级结构 —— /settings 分类目录，/settings/:section 详情。
// 桌面端访问 /settings 时回落到弹窗，保持弹窗载体不变。
export const SettingsPage = () => {
  const params = useParams<{ section: string }>();
  const navigate = useNavigate();
  const isMobile = useMediaQuery('(max-width: 760px)');
  const openSettings = useUiStore((state) => state.openSettings);
  const closeSettings = useUiStore((state) => state.closeSettings);

  useEffect(() => {
    if (!isMobile) {
      openSettings(isSettingsSection(params.section) ? params.section : undefined);
      navigate('/', { replace: true });
    }
  }, [isMobile, navigate, openSettings, params.section]);

  if (!isMobile) return null;
  const active: SettingsSection | null = isSettingsSection(params.section) ? params.section : null;

  if (active) {
    const Section = sections[active];
    const meta = SETTINGS_SECTIONS.find((item) => item.key === active);
    return (
      <div className="st-mobile-page">
        <MobileTopbar
          title={meta?.label ?? '设置'}
          backTo="/settings"
          onBack={() => navigate('/settings')}
        />
        <div className="st-mobile-scroll">
          <Section />
        </div>
      </div>
    );
  }

  return (
    <div className="st-mobile-page">
      <MobileTopbar title="设置" backTo="/" onBack={() => { closeSettings(); navigate('/'); }} />
      <div className="st-mobile-scroll">
        <nav className="st-mobile-nav" aria-label="设置分类">
          {SETTINGS_SECTIONS.map((item) => (
            <button
              type="button"
              key={item.key}
              className="st-mobile-item"
              onClick={() => navigate(`/settings/${item.key}`)}
            >
              <span>{item.label}</span>
              <Icon name="chevron-right" size={15} />
            </button>
          ))}
        </nav>
      </div>
    </div>
  );
};
