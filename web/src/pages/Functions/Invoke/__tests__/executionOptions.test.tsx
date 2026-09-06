import React from 'react';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import ExecutionOptions from '../ExecutionOptions';

const baseProps = {
  route: 'lb' as const,
  targetServiceId: '',
  hashKey: '',
  asyncMode: false,
  onRouteChange: jest.fn(),
  onTargetServiceIdChange: jest.fn(),
  onHashKeyChange: jest.fn(),
  onAsyncModeChange: jest.fn(),
};

function openRouteDropdown() {
  fireEvent.mouseDown(screen.getByRole('combobox'));
}

describe('ExecutionOptions：路由选项随 同步/异步 刷新', () => {
  beforeEach(() => jest.clearAllMocks());

  it('同步模式：广播选项可用（无禁用标记）', async () => {
    render(<ExecutionOptions {...baseProps} />);
    openRouteDropdown();
    const option = await screen.findByText('广播全部实例（仅同步）');
    expect(option.closest('.ant-select-item-option-disabled')).toBeNull();
  });

  it('异步任务模式：广播选项被禁用（服务端拒绝 broadcast+async）', async () => {
    render(<ExecutionOptions {...baseProps} asyncMode />);
    openRouteDropdown();
    const option = await screen.findByText('广播全部实例（仅同步）');
    expect(option.closest('.ant-select-item-option-disabled')).not.toBeNull();
  });

  it('broadcast + 切到异步 → 自动回落负载均衡（避免必然 400 的组合）', () => {
    const { rerender } = render(
      <ExecutionOptions {...baseProps} route="broadcast" asyncMode={false} />,
    );
    rerender(<ExecutionOptions {...baseProps} route="broadcast" asyncMode />);
    expect(baseProps.onRouteChange).toHaveBeenCalledWith('lb');
    expect(screen.getByText(/已自动切回负载均衡/)).toBeInTheDocument();
  });

  it('targeted 缺 service_id / hash 缺 key 的警告仍然生效', () => {
    const { rerender } = render(<ExecutionOptions {...baseProps} route="targeted" />);
    expect(screen.getByText('指定实例路由需要填写 service_id')).toBeInTheDocument();
    rerender(<ExecutionOptions {...baseProps} route="hash" />);
    expect(screen.getByText('哈希路由需要填写 hash key')).toBeInTheDocument();
  });
});
