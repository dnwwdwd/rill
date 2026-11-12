import { Icon, type IconName } from '@/components/ui/Icon';
import { SETTINGS_SECTIONS, type SettingsSection } from '@/types/ui';
import { cx } from '@/utils/cx';

interface SettingsNavProps {
  active: SettingsSection;
  onSelect: (section: SettingsSection) => void;
}

// REQ-023：纯文字目录 + 手绘线性 icon，icon 颜色跟随主题色（--accent）。
export const SettingsNav = ({ active, onSelect }: SettingsNavProps) => (
  <nav className="st-nav" aria-label="设置分类">
    {SETTINGS_SECTIONS.map((item) => (
      <button
        type="button"
        key={item.key}
        className={cx('st-nav-item', active === item.key && 'is-active')}
        onClick={() => onSelect(item.key)}
        aria-current={active === item.key ? 'page' : undefined}
      >
        <Icon name={item.icon as IconName} size={14} />
        <span>{item.label}</span>
      </button>
    ))}
  </nav>
);
