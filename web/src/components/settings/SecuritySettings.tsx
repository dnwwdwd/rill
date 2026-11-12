import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Button, Icon } from '@/components/ui';
import { RowGroup, Row } from '@/components/settings/SettingsSection';
import { useAuthStore } from '@/stores/authStore';
import { useSettingsStore } from '@/stores/settingsStore';
import { useUiStore } from '@/stores/uiStore';
import { formatDateTime } from '@/utils/date';

// DEC-017：登录只有懒猫 OIDC，账号信息来自 OIDC userinfo；退出登录只清除
// Rill 会话，懒猫系统会话由微服管理。
export const SecuritySettings = () => {
  const navigate = useNavigate();
  const logout = useAuthStore((state) => state.logout);
  const security = useSettingsStore((state) => state.settings.security);
  const feedback = useUiStore((state) => state.notify);
  const [signingOut, setSigningOut] = useState(false);

  const signOut = async () => {
    setSigningOut(true);
    try {
      await logout();
      navigate('/login', { replace: true });
    } catch {
      feedback('退出失败，请重试');
      setSigningOut(false);
    }
  };

  return (
    <>
      <RowGroup title="账号与会话">
        <Row
          label="懒猫账号"
          control={<span className="st-dim">{security.account ?? '—'}</span>}
          hint="登录与账号由懒猫微服统一管理。"
        />
        <Row label="上次登录" control={<span className="st-dim">{formatDateTime(security.last_login_at ?? undefined)}</span>} />
        <Row label="会话有效至" control={<span className="st-dim">{formatDateTime(security.session_expires_at ?? undefined)}</span>} />
        <Row
          control={(
            <Button size="sm" variant="danger" icon="logout" disabled={signingOut} onClick={() => { void signOut(); }}>
              {signingOut ? '正在退出' : '退出登录'}
            </Button>
          )}
        />
      </RowGroup>

      <RowGroup title="安全边界">
        <Row className="st-facts-row">
          <ul className="st-facts st-facts-icons">
            <li><Icon name="panel" size={15} /><span><strong>出站校验</strong>抓取仅允许 http/https 地址。</span></li>
            <li><Icon name="shield" size={15} /><span><strong>HTML 双白名单</strong>后端 bluemonday 清洗，前端 DOMPurify 再过滤一次。</span></li>
            <li><Icon name="key" size={15} /><span><strong>密钥 AES-GCM</strong>AI 密钥加密落库，日志与接口响应只显示是否已设置。</span></li>
            <li><Icon name="database" size={15} /><span><strong>最小系统权限</strong>只映射数据与缓存目录，不接管系统端口。</span></li>
          </ul>
        </Row>
      </RowGroup>
    </>
  );
};
