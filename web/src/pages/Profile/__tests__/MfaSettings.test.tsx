/**
 * MfaSettings 回归（docs/BUGS.md BUG-013）。
 *
 * 修复前该组件只把 base32 密钥和 otpauth:// 串当**纯文本**展示，用户必须自行
 * 抄进 App——微软/谷歌 Authenticator 并不提供「粘贴 otpauth 链接」的入口，
 * 实际体验等同于没有绑定功能；且确认成功后**不展示任何恢复码**，用户一旦丢失
 * 验证器就永久锁在门外。
 *
 * 覆盖：扫码二维码（数据 = 后端给的 otpauth 串）、手动录入兜底、恢复码一次性
 * 展示、已启用态的剩余恢复码提示、关闭入口、外部 IdP 只读态。
 */
import React from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App } from 'antd';
import MfaSettings from '../MfaSettings';
import { confirmMfa, disableMfa, fetchMfaStatus, setupMfa } from '@/services/api/auth';

jest.mock('@/services/api/auth', () => ({
  fetchMfaStatus: jest.fn(),
  setupMfa: jest.fn(),
  confirmMfa: jest.fn(),
  disableMfa: jest.fn(),
}));

const mockStatus = fetchMfaStatus as jest.MockedFunction<typeof fetchMfaStatus>;
const mockSetup = setupMfa as jest.MockedFunction<typeof setupMfa>;
const mockConfirm = confirmMfa as jest.MockedFunction<typeof confirmMfa>;
const mockDisable = disableMfa as jest.MockedFunction<typeof disableMfa>;

const OTPAUTH =
  'otpauth://totp/Croupier:admin?secret=JBSWY3DPEHPK3PXP&issuer=Croupier&algorithm=SHA1&digits=6&period=30';

/** antd 的 App.useApp() 需要 <App> 祖先，否则 message.* 为空函数。 */
function renderWithApp() {
  return render(
    <App>
      <MfaSettings />
    </App>,
  );
}

beforeEach(() => {
  jest.clearAllMocks();
});

describe('MfaSettings 绑定流程', () => {
  it('未开启时只展示「开启两步验证」入口', async () => {
    mockStatus.mockResolvedValue({ enabled: false, local: true });
    renderWithApp();
    await waitFor(() => expect(mockStatus).toHaveBeenCalled());
    expect(await screen.findByTestId('mfa-enable')).toBeInTheDocument();
    expect(screen.getByText('未开启')).toBeInTheDocument();
  });

  it('点击开启后渲染二维码，其内容是后端返回的 otpauth URI（扫码即绑）', async () => {
    mockStatus.mockResolvedValue({ enabled: false, local: true });
    mockSetup.mockResolvedValue({
      secret: 'JBSWY3DPEHPK3PXP',
      otpauthUrl: OTPAUTH,
      alreadyEnabled: false,
      issuer: 'Croupier',
    });
    renderWithApp();
    await waitFor(() => expect(mockStatus).toHaveBeenCalled());

    fireEvent.click(await screen.findByTestId('mfa-enable'));
    await waitFor(() => expect(mockSetup).toHaveBeenCalled());

    // 二维码存在，且承载的就是 otpauth 串
    const qr = await screen.findByTestId('mfa-qrcode');
    expect(qr).toBeInTheDocument();
    // 手动录入密钥仍作为兜底保留
    expect(screen.getByTestId('mfa-secret')).toHaveTextContent('JBSWY3DPEHPK3PXP');
    expect(screen.getByText(OTPAUTH)).toBeInTheDocument();
  });

  it('确认成功后一次性展示恢复码，可下载与关闭', async () => {
    mockStatus.mockResolvedValueOnce({ enabled: false, local: true }).mockResolvedValueOnce({
      enabled: true,
      local: true,
      recoveryCodesRemaining: 10,
      recoveryCodeTotal: 10,
    });
    mockSetup.mockResolvedValue({
      secret: 'JBSWY3DPEHPK3PXP',
      otpauthUrl: OTPAUTH,
      alreadyEnabled: false,
      issuer: 'Croupier',
    });
    const codes = ['ABCD2345EF', 'GHIJ6789KL', 'MNOP2345QR'];
    mockConfirm.mockResolvedValue({ recoveryCodes: codes });

    renderWithApp();
    await waitFor(() => expect(mockStatus).toHaveBeenCalled());
    fireEvent.click(await screen.findByTestId('mfa-enable'));
    fireEvent.change(await screen.findByTestId('mfa-confirm-code'), {
      target: { value: '123456' },
    });
    fireEvent.click(await screen.findByTestId('mfa-confirm'));

    await waitFor(() => expect(mockConfirm).toHaveBeenCalledWith('123456'));
    // 恢复码逐条展示
    for (const c of codes) {
      expect(screen.getByTestId('mfa-recovery-codes')).toHaveTextContent(c);
    }
    expect(screen.getAllByTestId('mfa-recovery-code')).toHaveLength(codes.length);
    // 关闭后不再展示（不可二次查看）
    fireEvent.click(screen.getByTestId('mfa-recovery-dismiss'));
    await waitFor(() => expect(screen.queryByTestId('mfa-recovery-codes')).toBeNull());
  });

  it('确认接口不返回恢复码时不渲染恢复码区（不伪造）', async () => {
    mockStatus
      .mockResolvedValueOnce({ enabled: false, local: true })
      .mockResolvedValueOnce({ enabled: true, local: true, recoveryCodesRemaining: 0 });
    mockSetup.mockResolvedValue({
      secret: 'JBSWY3DPEHPK3PXP',
      otpauthUrl: OTPAUTH,
      alreadyEnabled: false,
    });
    mockConfirm.mockResolvedValue({});

    renderWithApp();
    await waitFor(() => expect(mockStatus).toHaveBeenCalled());
    fireEvent.click(await screen.findByTestId('mfa-enable'));
    fireEvent.change(await screen.findByTestId('mfa-confirm-code'), {
      target: { value: '123456' },
    });
    fireEvent.click(await screen.findByTestId('mfa-confirm'));

    await waitFor(() => expect(mockConfirm).toHaveBeenCalled());
    expect(screen.queryByTestId('mfa-recovery-codes')).toBeNull();
  });
});

describe('MfaSettings 已启用态', () => {
  it('展示已开启标签与剩余恢复码数量', async () => {
    mockStatus.mockResolvedValue({
      enabled: true,
      local: true,
      recoveryCodesRemaining: 2,
      recoveryCodeTotal: 10,
    });
    renderWithApp();
    await waitFor(() => expect(mockStatus).toHaveBeenCalled());
    expect(await screen.findByText('已开启')).toBeInTheDocument();
    expect(await screen.findByText('剩余恢复码 2/10')).toBeInTheDocument();
  });

  it('提供关闭入口：验证码 + 密码双确认', async () => {
    mockStatus.mockResolvedValue({
      enabled: true,
      local: true,
      recoveryCodesRemaining: 10,
      recoveryCodeTotal: 10,
    });
    mockDisable.mockResolvedValue(undefined);
    renderWithApp();
    await waitFor(() => expect(mockStatus).toHaveBeenCalled());

    fireEvent.change(await screen.findByTestId('mfa-disable-code'), {
      target: { value: '123456' },
    });
    fireEvent.change(screen.getByTestId('mfa-disable-password'), { target: { value: 'admin123' } });
    fireEvent.click(screen.getByTestId('mfa-disable'));

    await waitFor(() => expect(mockDisable).toHaveBeenCalledWith('123456', 'admin123'));
  });

  it('缺验证码或密码时不发请求', async () => {
    mockStatus.mockResolvedValue({ enabled: true, local: true });
    renderWithApp();
    await waitFor(() => expect(mockStatus).toHaveBeenCalled());
    fireEvent.click(await screen.findByTestId('mfa-disable'));
    expect(mockDisable).not.toHaveBeenCalled();
  });
});

// 回车提交：三个输入框的 onPressEnter（R55 funcs 收口）
describe('MfaSettings 回车提交', () => {
  it('确认码输入框按 Enter 等价点确认', async () => {
    mockStatus.mockResolvedValue({ enabled: false, local: true });
    mockSetup.mockResolvedValue({
      secret: 'JBSWY3DPEHPK3PXP',
      otpauthUrl: OTPAUTH,
      alreadyEnabled: false,
    });
    mockConfirm.mockResolvedValue({ recoveryCodes: ['ABCD2345EF'] });
    renderWithApp();
    await waitFor(() => expect(mockStatus).toHaveBeenCalled());
    fireEvent.click(await screen.findByTestId('mfa-enable'));
    const code = await screen.findByTestId('mfa-confirm-code');
    fireEvent.change(code, { target: { value: '654321' } });
    fireEvent.keyDown(code, { key: 'Enter' });
    await waitFor(() => expect(mockConfirm).toHaveBeenCalledWith('654321'));
  });

  it('关闭表单验证码输入框按 Enter 触发关闭请求', async () => {
    mockStatus.mockResolvedValue({ enabled: true, local: true });
    mockDisable.mockResolvedValue(undefined);
    renderWithApp();
    await waitFor(() => expect(mockStatus).toHaveBeenCalled());
    const code = await screen.findByTestId('mfa-disable-code');
    fireEvent.change(code, { target: { value: '123456' } });
    fireEvent.change(screen.getByTestId('mfa-disable-password'), {
      target: { value: 'admin123' },
    });
    fireEvent.keyDown(code, { key: 'Enter' });
    await waitFor(() => expect(mockDisable).toHaveBeenCalledWith('123456', 'admin123'));
  });

  it('关闭表单密码输入框按 Enter 触发关闭请求', async () => {
    mockStatus.mockResolvedValue({ enabled: true, local: true });
    mockDisable.mockResolvedValue(undefined);
    renderWithApp();
    await waitFor(() => expect(mockStatus).toHaveBeenCalled());
    fireEvent.change(await screen.findByTestId('mfa-disable-code'), {
      target: { value: '123456' },
    });
    const password = screen.getByTestId('mfa-disable-password');
    fireEvent.change(password, { target: { value: 'admin123' } });
    fireEvent.keyDown(password, { key: 'Enter' });
    await waitFor(() => expect(mockDisable).toHaveBeenCalledWith('123456', 'admin123'));
  });
});

describe('MfaSettings 外部 IdP 账号', () => {
  it('只展示说明，不提供任何配置入口', async () => {
    mockStatus.mockResolvedValue({ enabled: false, local: false });
    renderWithApp();
    await waitFor(() => expect(mockStatus).toHaveBeenCalled());
    expect(await screen.findByTestId('mfa-external')).toBeInTheDocument();
    expect(screen.queryByTestId('mfa-enable')).toBeNull();
    expect(screen.queryByTestId('mfa-qrcode')).toBeNull();
  });
});

// R55 覆盖批次：错误流（加载/确认/关闭）、alreadyEnabled、空码 warning、
// 恢复码下载、取消开启——此前 funcs 61.5% 的缺口全部在这些分支上。
describe('MfaSettings 错误与分支流', () => {
  it('状态加载失败走错误提示（extractErrorMessage 优先错误 message）', async () => {
    mockStatus.mockRejectedValue(new Error('status boom'));
    renderWithApp();
    // 失败只提示不崩：组件保持未开启态渲染（不会误入已开启 UI）
    expect(await screen.findByText('status boom')).toBeInTheDocument();
    expect(screen.getByTestId('mfa-status')).toHaveTextContent('未开启');
    expect(screen.getByTestId('mfa-enable')).toBeInTheDocument();
  });

  it('setup 返回 alreadyEnabled：直接回已启用态并警示，不渲染二维码', async () => {
    mockStatus.mockResolvedValueOnce({ enabled: false, local: true }).mockResolvedValue({
      enabled: true,
      local: true,
      recoveryCodesRemaining: 10,
      recoveryCodeTotal: 10,
    });
    mockSetup.mockResolvedValue({
      secret: 'JBSWY3DPEHPK3PXP',
      otpauthUrl: OTPAUTH,
      alreadyEnabled: true,
    });
    renderWithApp();
    await waitFor(() => expect(mockStatus).toHaveBeenCalled());
    fireEvent.click(await screen.findByTestId('mfa-enable'));
    expect(await screen.findByText('两步验证已处于开启状态')).toBeInTheDocument();
    expect(screen.queryByTestId('mfa-qrcode')).toBeNull();
    // refresh 后回到已启用态（status 被再取一次），入口消失
    await waitFor(() => expect(mockStatus).toHaveBeenCalledTimes(2));
    expect(screen.queryByTestId('mfa-enable')).toBeNull();
  });

  it('setupMfa 失败走错误提示，不渲染二维码', async () => {
    mockStatus.mockResolvedValue({ enabled: false, local: true });
    mockSetup.mockRejectedValue(new Error('secret boom'));
    renderWithApp();
    await waitFor(() => expect(mockStatus).toHaveBeenCalled());
    fireEvent.click(await screen.findByTestId('mfa-enable'));
    expect(await screen.findByText('secret boom')).toBeInTheDocument();
    expect(screen.queryByTestId('mfa-qrcode')).toBeNull();
    expect(screen.getByTestId('mfa-enable')).toBeInTheDocument();
  });

  it('空验证码点确认：只警告不发请求', async () => {
    mockStatus.mockResolvedValue({ enabled: false, local: true });
    mockSetup.mockResolvedValue({
      secret: 'JBSWY3DPEHPK3PXP',
      otpauthUrl: OTPAUTH,
      alreadyEnabled: false,
    });
    renderWithApp();
    await waitFor(() => expect(mockStatus).toHaveBeenCalled());
    fireEvent.click(await screen.findByTestId('mfa-enable'));
    await screen.findByTestId('mfa-qrcode');
    fireEvent.click(screen.getByTestId('mfa-confirm'));
    expect(await screen.findByText('请输入验证器 App 中的 6 位验证码')).toBeInTheDocument();
    expect(mockConfirm).not.toHaveBeenCalled();
  });

  it('确认失败走错误提示，不渲染恢复码区', async () => {
    mockStatus.mockResolvedValue({ enabled: false, local: true });
    mockSetup.mockResolvedValue({
      secret: 'JBSWY3DPEHPK3PXP',
      otpauthUrl: OTPAUTH,
      alreadyEnabled: false,
    });
    mockConfirm.mockRejectedValue(new Error('bad token'));
    renderWithApp();
    await waitFor(() => expect(mockStatus).toHaveBeenCalled());
    fireEvent.click(await screen.findByTestId('mfa-enable'));
    fireEvent.change(await screen.findByTestId('mfa-confirm-code'), {
      target: { value: '123456' },
    });
    fireEvent.click(screen.getByTestId('mfa-confirm'));
    expect(await screen.findByText('bad token')).toBeInTheDocument();
    expect(screen.queryByTestId('mfa-recovery-codes')).toBeNull();
  });

  it('关闭失败走错误提示，表单仍可重试', async () => {
    mockStatus.mockResolvedValue({ enabled: true, local: true });
    mockDisable.mockRejectedValue(new Error('disable boom'));
    renderWithApp();
    await waitFor(() => expect(mockStatus).toHaveBeenCalled());
    fireEvent.change(await screen.findByTestId('mfa-disable-code'), {
      target: { value: '123456' },
    });
    fireEvent.change(screen.getByTestId('mfa-disable-password'), { target: { value: 'admin123' } });
    fireEvent.click(screen.getByTestId('mfa-disable'));
    expect(await screen.findByText('disable boom')).toBeInTheDocument();
    expect(screen.getByTestId('mfa-disable')).toBeInTheDocument();
  });

  it('恢复码下载触发 createObjectURL/revokeObjectURL', async () => {
    const created = jest.fn(() => 'blob:recovery');
    const revoked = jest.fn();
    const origCreate = URL.createObjectURL;
    const origRevoke = URL.revokeObjectURL;
    Object.assign(URL, { createObjectURL: created, revokeObjectURL: revoked });
    try {
      mockStatus.mockResolvedValueOnce({ enabled: false, local: true }).mockResolvedValue({
        enabled: true,
        local: true,
        recoveryCodesRemaining: 10,
        recoveryCodeTotal: 10,
      });
      mockSetup.mockResolvedValue({
        secret: 'JBSWY3DPEHPK3PXP',
        otpauthUrl: OTPAUTH,
        alreadyEnabled: false,
      });
      mockConfirm.mockResolvedValue({ recoveryCodes: ['ABCD2345EF', 'GHIJ6789KL'] });
      renderWithApp();
      await waitFor(() => expect(mockStatus).toHaveBeenCalled());
      fireEvent.click(await screen.findByTestId('mfa-enable'));
      fireEvent.change(await screen.findByTestId('mfa-confirm-code'), {
        target: { value: '123456' },
      });
      fireEvent.click(await screen.findByTestId('mfa-confirm'));
      await waitFor(() => expect(mockConfirm).toHaveBeenCalled());
      fireEvent.click(await screen.findByRole('button', { name: /下载全部恢复码/ }));
      expect(created).toHaveBeenCalledTimes(1);
      expect(revoked).toHaveBeenCalledWith('blob:recovery');
    } finally {
      Object.assign(URL, { createObjectURL: origCreate, revokeObjectURL: origRevoke });
    }
  });

  it('取消开启清空二维码回到初始态', async () => {
    mockStatus.mockResolvedValue({ enabled: false, local: true });
    mockSetup.mockResolvedValue({
      secret: 'JBSWY3DPEHPK3PXP',
      otpauthUrl: OTPAUTH,
      alreadyEnabled: false,
    });
    renderWithApp();
    await waitFor(() => expect(mockStatus).toHaveBeenCalled());
    fireEvent.click(await screen.findByTestId('mfa-enable'));
    await screen.findByTestId('mfa-qrcode');
    fireEvent.click(screen.getByRole('button', { name: /^取\s*消$/ }));
    expect(screen.queryByTestId('mfa-qrcode')).toBeNull();
    expect(await screen.findByTestId('mfa-enable')).toBeInTheDocument();
  });
});
