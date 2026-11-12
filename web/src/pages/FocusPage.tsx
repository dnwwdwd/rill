import { useNavigate, useParams } from 'react-router-dom';
import { EmptyState, Button } from '@/components/ui';
import { FocusReader } from '@/components/reader/FocusReader';

export const FocusPage = () => {
  const { itemId } = useParams<{ itemId: string }>();
  const navigate = useNavigate();
  const id = Number(itemId);

  if (!Number.isFinite(id)) {
    return (
      <div className="focus-screen">
        <EmptyState
          icon="alert-circle"
          title="没有选中的条目"
          action={<Button onClick={() => navigate('/')}>返回阅读</Button>}
        />
      </div>
    );
  }

  return <FocusReader itemId={id} onBack={() => navigate(`/items/${id}`)} />;
};
