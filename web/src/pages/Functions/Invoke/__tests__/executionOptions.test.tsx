import React from 'react';
import { render, screen, fireEvent } from '@testing-library/react';
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

describe('ExecutionOptions：控件回调（受控交互）', () => {
  beforeEach(() => jest.clearAllMocks());

  it('路由下拉选择触发 onRouteChange', async () => {
    render(<ExecutionOptions {...baseProps} />);
    openRouteDropdown();
    fireEvent.click(await screen.findByText('指定实例'));
    // antd Select onChange 透传 (value, option) 双参
    expect(baseProps.onRouteChange).toHaveBeenCalledWith(
      'targeted',
      expect.objectContaining({ value: 'targeted' }),
    );
  });

  it('targeted 输入 service_id / hash 输入 key 触发对应 onChange', () => {
    const { rerender } = render(<ExecutionOptions {...baseProps} route="targeted" />);
    fireEvent.change(screen.getByPlaceholderText('目标 service_id'), {
      target: { value: 'svc-1' },
    });
    expect(baseProps.onTargetServiceIdChange).toHaveBeenCalledWith('svc-1');

    rerender(<ExecutionOptions {...baseProps} route="hash" />);
    fireEvent.change(screen.getByPlaceholderText('hash key'), { target: { value: 'key-1' } });
    expect(baseProps.onHashKeyChange).toHaveBeenCalledWith('key-1');
  });

  it('同步/异步 Radio 切换触发 onAsyncModeChange', () => {
    // 异步模式下点「同步」radio（未选中态才会派发 change）
    const { rerender } = render(<ExecutionOptions {...baseProps} asyncMode />);
    fireEvent.click(screen.getByRole('radio', { name: /同步/ }));
    expect(baseProps.onAsyncModeChange).toHaveBeenCalledWith(false);

    // 同步模式下点「异步任务」radio
    rerender(<ExecutionOptions {...baseProps} />);
    fireEvent.click(screen.getByRole('radio', { name: /异步任务/ }));
    expect(baseProps.onAsyncModeChange).toHaveBeenCalledWith(true);
  });
});
