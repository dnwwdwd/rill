import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/react';
import { Switch, SwitchRow } from '@/components/ui/Switch';

describe('Switch', () => {
  it('exposes switch semantics and toggles', () => {
    const onChange = vi.fn();
    render(<Switch checked={false} onChange={onChange} label="启用源" />);
    const control = screen.getByRole('switch', { name: '启用源' });
    expect(control).toHaveAttribute('aria-checked', 'false');
    fireEvent.click(control);
    expect(onChange).toHaveBeenCalledWith(true);
  });

  it('renders a titled row and respects disabled state', () => {
    const onChange = vi.fn();
    render(
      <SwitchRow
        title="停留标记已读"
        description="条目停留 3 秒后标记已读"
        checked
        onChange={onChange}
        disabled
      />,
    );
    expect(screen.getByText('停留标记已读')).toBeInTheDocument();
    expect(screen.getByText('条目停留 3 秒后标记已读')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('switch', { name: '停留标记已读' }));
    expect(onChange).not.toHaveBeenCalled();
  });
});
