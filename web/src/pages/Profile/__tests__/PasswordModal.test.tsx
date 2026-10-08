/**
 * PasswordModal 覆盖收口（web 覆盖率巡检：onFinish 成功/失败翼 + confirm
 * 校验器此前零触达）。文案取 zh-CN locale 值。
 */
import React from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App } from 'antd';
import PasswordModal from '../PasswordModal';
import { changeMyPassword } from '@/services/api/me';

jest.mock('@/services/api/me', () => ({
  changeMyPassword: jest.fn(),
}));
const mockChange = changeMyPassword as jest.MockedFunction<typeof changeMyPassword>;

function renderWithApp(onClose: () => void = () => {}) {
  return render(
    <App>
      <PasswordModal open onClose={onClose} />
    </App>,
  );
}

async function fillAndSubmit(values: { current: string; password: string; confirm?: string }) {
  // ModalForm（destroyOnHidden）的字段挂载在 open 后的异步帧，用 findBy 轮询
  fireEvent.change(await screen.findByPlaceholderText('请输入当前密码'), {
    target: { value: values.current },
  });
  fireEvent.change(await screen.findByPlaceholderText('请输入新密码'), {
    target: { value: values.password },
  });
  if (values.confirm !== undefined) {
    fireEvent.change(await screen.findByPlaceholderText('再次输入新密码'), {
      target: { value: values.confirm },
    });
  }
  fireEvent.click(await screen.findByText('确认修改'));
}

beforeEach(() => {
  jest.clearAllMocks();
});

describe('PasswordModal', () => {
  it('提交成功：调用 changeMyPassword（current+password）并经 onOpenChange(false) 关闭', async () => {
    const onClose = jest.fn();
    mockChange.mockResolvedValue(undefined);
    renderWithApp(onClose);

    await fillAndSubmit({ current: 'old-pass', password: 'new-pass-1', confirm: 'new-pass-1' });

    await waitFor(() =>
      expect(mockChange).toHaveBeenCalledWith({
        current: 'old-pass',
        password: 'new-pass-1',
      }),
    );
    // onFinish 返回 true → ModalForm 触发 onOpenChange(false) → onClose
    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(await screen.findByText('密码更新成功')).toBeInTheDocument();
  });

  it('提交失败：提示错误文案且弹窗保持开启（onClose 不被调）', async () => {
    const onClose = jest.fn();
    mockChange.mockRejectedValue(new Error('weak password'));
    renderWithApp(onClose);

    await fillAndSubmit({ current: 'old-pass', password: 'new-pass-1', confirm: 'new-pass-1' });

    await waitFor(() => expect(mockChange).toHaveBeenCalled());
    expect(await screen.findByText('更新密码失败')).toBeInTheDocument();
    // 失败按 ModalForm 语义保持开启
    await waitFor(() => expect(screen.getByText('修改密码')).toBeInTheDocument());
    expect(onClose).not.toHaveBeenCalled();
  });

  it('确认密码不一致：校验器拒绝且不发起请求', async () => {
    mockChange.mockResolvedValue(undefined);
    renderWithApp();

    await fillAndSubmit({ current: 'old-pass', password: 'new-pass-1', confirm: 'different' });

    expect(await screen.findByText('两次输入的密码不一致')).toBeInTheDocument();
    expect(mockChange).not.toHaveBeenCalled();
  });

  it('新密码不足 6 位：min 规则拦截', async () => {
    renderWithApp();

    await fillAndSubmit({ current: 'old-pass', password: '123', confirm: '123' });

    expect(await screen.findByText('至少 6 位字符')).toBeInTheDocument();
    expect(mockChange).not.toHaveBeenCalled();
  });
});
