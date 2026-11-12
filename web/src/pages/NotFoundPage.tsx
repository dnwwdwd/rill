import { useNavigate } from 'react-router-dom';
import { Button, Icon } from '@/components/ui';
import { useUiStore } from '@/stores/uiStore';

export const NotFoundPage = () => {
  const navigate = useNavigate();
  const openSettings = useUiStore((state) => state.openSettings);
  return (
    <div className="main-pane not-found-screen">
      <div className="not-found">
        <span className="not-found-mark"><Icon name="rss" size={28} /></span>
        <div className="not-found-code">404</div>
        <h1>这里没有页面</h1>
        <div className="not-found-actions">
          <Button variant="primary" icon="arrow-left" onClick={() => navigate('/')}>回到阅读</Button>
          <Button icon="settings" onClick={() => openSettings()}>打开设置</Button>
        </div>
      </div>
    </div>
  );
};
