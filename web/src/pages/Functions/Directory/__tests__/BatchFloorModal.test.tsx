/**
 * BatchFloorModal 组件行为：空输入禁用提交（批量清除走独立按钮，弹窗内
 * 不允许空值）、提交回调带 trim 后的值、文案含选中数量、重开重置输入。
 */
import React from 'react';
import { App as AntdApp } from 'antd';
import { configure, fireEvent, render, screen, waitFor } from '@testing-library/react';
import BatchFloorModal from '../BatchFloorModal';

configure({ asyncUtilTimeout: 5000 });
jest.setTimeout(20000);

jest.mock('@umijs/max', () => {
  const formatMessage = jest.fn(
    ({ defaultMessage }: { defaultMessage: string }, values?: Record<string, unknown>) =>
      Object.entries(values || {}).reduce(
        (msg: string, [key, val]) => msg.split(`{${key}}`).join(String(val)),
        defaultMessage,
      ),
  );
  const intl = { formatMessage };
  return {
    __esModule: true,
    useIntl: () => intl,
    getIntl: () => intl,
    FormattedMessage: ({ defaultMessage }: { defaultMessage: string }) => defaultMessage,
  };
});

const mockOnSubmit = jest.fn();
const mockOnClose = jest.fn();

function renderModal(open: boolean) {
  return render(
    <AntdApp>
      <BatchFloorModal
        open={open}
        count={2}
        submitting={false}
        onSubmit={mockOnSubmit}
        onClose={mockOnClose}
      />
    </AntdApp>,
  );
}

describe('BatchFloorModal', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it('空输入时确认按钮禁用（不允许空提交）', () => {
    renderModal(true);
    const okButton = screen.getByRole('button', { name: /OK|确\s*定/ });
    expect(okButton).toBeDisabled();
    expect(mockOnSubmit).not.toHaveBeenCalled();
  });

  it('输入版本后提交：回调带 trim 后的值', async () => {
    renderModal(true);
    fireEvent.change(screen.getByPlaceholderText('如 0.3.0'), {
      target: { value: '  0.3.0  ' },
    });
    const okButton = screen.getByRole('button', { name: /OK|确\s*定/ });
    await waitFor(() => {
      expect(okButton).toBeEnabled();
    });
    fireEvent.click(okButton);
    expect(mockOnSubmit).toHaveBeenCalledWith('0.3.0');
  });

  it('文案含已选数量', () => {
    renderModal(true);
    expect(
      screen.getByText('将把已选 2 个函数的最低可注册 SDK 版本统一设为输入值。'),
    ).toBeInTheDocument();
  });

  it('重开弹窗时输入被重置（防残留误提交）', async () => {
    const { rerender } = renderModal(true);
    fireEvent.change(screen.getByPlaceholderText('如 0.3.0'), { target: { value: '0.3.0' } });
    rerender(
      <AntdApp>
        <BatchFloorModal
          open={false}
          count={2}
          submitting={false}
          onSubmit={mockOnSubmit}
          onClose={mockOnClose}
        />
      </AntdApp>,
    );
    rerender(
      <AntdApp>
        <BatchFloorModal
          open
          count={2}
          submitting={false}
          onSubmit={mockOnSubmit}
          onClose={mockOnClose}
        />
      </AntdApp>,
    );
    await waitFor(() => {
      const okButton = screen.getByRole('button', { name: /OK|确\s*定/ });
      expect(okButton).toBeDisabled();
    });
    expect(mockOnSubmit).not.toHaveBeenCalled();
  });
});
