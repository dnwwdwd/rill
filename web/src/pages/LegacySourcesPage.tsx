import { useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { useUiStore } from '@/stores/uiStore';

// 兼容旧的 /sources 链接：订阅源管理已迁入设置弹窗。
export const LegacySourcesPage = () => {
  const navigate = useNavigate();
  const openSettings = useUiStore((state) => state.openSettings);

  useEffect(() => {
    openSettings('sources');
    navigate('/', { replace: true });
  }, [navigate, openSettings]);

  return null;
};
