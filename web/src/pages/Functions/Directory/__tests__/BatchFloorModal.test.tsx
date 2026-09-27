/**
 * BatchFloorModal 组件行为（#26 改下拉后）：空选择禁用提交（批量清除走
 * 独立按钮，弹窗内不允许空值）、只能选历史版本选项、提交回调带所选值、
 * 文案含选中数量、重开重置选择。
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

const VERSION_OPTIONS = ['0.2.0', '0.3.0', '1.0.0'];

function renderModal(open: boolean, options: string[] = VERSION_OPTIONS) {
  return render(
    <AntdApp>
      <BatchFloorModal
        open={open}
        count={2}
        submitting={false}
        versionOptions={options}
        onSubmit={mockOnSubmit}
        onClose={mockOnClose}
      />
    </AntdApp>,
  );
}

async function chooseOption(label: string) {
  // antd Select：点击 combobox 打开下拉，再点选项
  fireEvent.mouseDown(screen.getByRole('combobox'));
  await waitFor(() => {
    expect(screen.getByTitle(label)).toBeInTheDocument();
  });
  fireEvent.click(screen.getByTitle(label));
}

describe('BatchFloorModal', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it('空选择时确认按钮禁用（不允许空提交）', () => {
    renderModal(true);
    const okButton = screen.getByRole('button', { name: /OK|确\s*定/ });
    expect(okButton).toBeDisabled();
    expect(mockOnSubmit).not.toHaveBeenCalled();
  });

  it('只能从历史版本选项中选择，提交回调带所选值', async () => {
    renderModal(true);
    await chooseOption('≥ v0.3.0');
    const okButton = screen.getByRole('button', { name: /OK|确\s*定/ });
    await waitFor(() => {
      expect(okButton).toBeEnabled();
    });
    fireEvent.click(okButton);
    expect(mockOnSubmit).toHaveBeenCalledWith('0.3.0');
  });

  it('选项来自服务端历史版本索引（不允许编造版本）', async () => {
    renderModal(true);
    fireEvent.mouseDown(screen.getByRole('combobox'));
    await waitFor(() => {
      expect(screen.getByTitle('≥ v1.0.0')).toBeInTheDocument();
    });
    expect(screen.queryByTitle('≥ v9.9.9')).not.toBeInTheDocument();
  });

  it('文案含已选数量', () => {
    renderModal(true);
    expect(
      screen.getByText('将把已选 2 个函数的最低可注册函数版本统一设为所选值。'),
    ).toBeInTheDocument();
  });

  it('无历史版本时选项为空且提交保持禁用', async () => {
    renderModal(true, []);
    fireEvent.mouseDown(screen.getByRole('combobox'));
    await waitFor(() => {
      expect(screen.getByText('暂无历史版本')).toBeInTheDocument();
    });
    expect(screen.getByRole('button', { name: /OK|确\s*定/ })).toBeDisabled();
  });

  it('重开弹窗时选择被重置（防残留误提交）', async () => {
    const { rerender } = renderModal(true);
    await chooseOption('≥ v0.3.0');
    rerender(
      <AntdApp>
        <BatchFloorModal
          open={false}
          count={2}
          submitting={false}
          versionOptions={VERSION_OPTIONS}
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
          versionOptions={VERSION_OPTIONS}
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
