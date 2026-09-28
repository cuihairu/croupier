/**
 * SMTP 邮件服务卡单测（OPEN-ISSUES #55，运维 Tab 内）。
 *
 * 锁定契约：emailEnabled 关闭时仅渲染开关（零表单）、开启渲染传输细节
 * （服务器/端口/用户名/密码徽标/发件人/加密 Select/认证 Select/跳过校验
 * Switch）；端口数字直提；加密/认证空值保存走 clearSiteSetting（恢复自动/
 * 默认）；迁移后 NotificationTab 不再有这些字段。
 *
 * mock 口径沿用同目录 NotificationTab.test.tsx：services/api/sites 三方法
 * jest.mock、@umijs/max 本地 mock；message 经真实 antd App 渲染进 portal。
 */
import React from 'react';
import { configure, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { App, ConfigProvider } from 'antd';
import SmtpCard from '../SmtpCard';
import type { NotificationSettings } from '@/services/api/sites';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

jest.mock('@/services/api/sites', () => ({
  fetchNotificationSettings: jest.fn(),
  setSiteSetting: jest.fn(),
  clearSiteSetting: jest.fn(),
  sendTestEmail: jest.fn(),
}));

jest.mock('@umijs/max', () => ({
  FormattedMessage: ({
    defaultMessage,
    values,
  }: {
    defaultMessage?: string;
    values?: Record<string, string>;
  }) => {
    let text = defaultMessage ?? '';
    if (values) {
      for (const [k, v] of Object.entries(values)) text = text.replace(`{${k}}`, String(v));
    }
    return <>{text}</>;
  },
  useIntl: () => ({
    formatMessage: (opts: { defaultMessage?: string }) => opts.defaultMessage ?? '',
  }),
}));

import {
  clearSiteSetting,
  fetchNotificationSettings,
  sendTestEmail,
  setSiteSetting,
} from '@/services/api/sites';

const mFetch = fetchNotificationSettings as jest.MockedFunction<typeof fetchNotificationSettings>;
const mSet = setSiteSetting as jest.MockedFunction<typeof setSiteSetting>;
const mClear = clearSiteSetting as jest.MockedFunction<typeof clearSiteSetting>;
const mSend = sendTestEmail as jest.MockedFunction<typeof sendTestEmail>;

const baseSettings: NotificationSettings = {
  emailEnabled: true,
  smtpHost: 'smtp.example.com',
  smtpPort: 465,
  smtpUser: 'noreply@example.com',
  smtpFrom: 'Croupier <noreply@example.com>',
  smtpPasswordSet: true,
  smtpPasswordMasked: '•••ab12',
  smtpEncryption: 'ssl',
  smtpAuthType: 'login',
  smtpInsecureSkipVerify: true,
  dingtalkUrl: '',
  dingtalkSecretSet: false,
  webhookUrl: '',
  webhookSecretSet: false,
  wecomUrl: '',
  feishuUrl: '',
  feishuSecretSet: false,
  inAppEnabled: true,
};

function renderCard() {
  return render(
    <App>
      <ConfigProvider>
        <SmtpCard />
      </ConfigProvider>
    </App>,
  );
}

function saveButtonOf(control: HTMLElement): HTMLElement {
  const item = control.closest('.ant-form-item');
  expect(item).not.toBeNull();
  return within(item as HTMLElement).getByRole('button', { name: '保存' });
}

beforeEach(() => {
  jest.clearAllMocks();
  mFetch.mockResolvedValue({ ...baseSettings });
  mSet.mockResolvedValue(undefined);
  mClear.mockResolvedValue(undefined);
  mSend.mockResolvedValue(undefined);
});

describe('SmtpCard（OPEN-ISSUES #55）', () => {
  it('开启态：传输字段回填 + 密码徽标 + Select 值 + 跳过校验开关', async () => {
    renderCard();

    expect(await screen.findByDisplayValue('smtp.example.com')).toBeInTheDocument();
    expect(screen.getByDisplayValue(465)).toBeInTheDocument();
    expect(screen.getByDisplayValue('noreply@example.com')).toBeInTheDocument();
    expect(screen.getByText('已配置 •••ab12')).toBeInTheDocument();
    // Select 显示所选标签（ssl → SSL/TLS（隐式））
    expect(screen.getAllByText('SSL/TLS（隐式）').length).toBeGreaterThan(0);
    expect(screen.getAllByText('AUTH LOGIN（强制）').length).toBeGreaterThan(0);
    // 开关：邮件通知 + 跳过校验
    const switches = screen.getAllByRole('switch');
    expect(switches).toHaveLength(2);
    expect(switches[0]).toBeChecked();
    expect(switches[1]).toBeChecked();
  });

  it('关闭态：仅渲染邮件开关，无表单字段', async () => {
    mFetch.mockResolvedValue({ ...baseSettings, emailEnabled: false });
    renderCard();

    expect(await screen.findByText('邮件通知')).toBeInTheDocument();
    expect(screen.queryByText('SMTP 服务器')).not.toBeInTheDocument();
    expect(screen.getByRole('switch')).not.toBeChecked();
  });

  it('端口数字直提：InputNumber 值不经 trim 直接提交', async () => {
    renderCard();
    const port = await screen.findByDisplayValue(465);
    fireEvent.change(port, { target: { value: '25' } });
    fireEvent.click(saveButtonOf(port));

    await waitFor(() => expect(mSet).toHaveBeenCalledWith('notification.smtpPort', 25));
    expect(await screen.findByText('已保存')).toBeInTheDocument();
    expect(mClear).not.toHaveBeenCalled();
  });

  it('文本字段 trim 提交；密文字段不回显', async () => {
    renderCard();
    const host = await screen.findByDisplayValue('smtp.example.com');
    expect(
      [...screen.getAllByPlaceholderText('••••••••')].every(
        (el) => (el as HTMLInputElement).value === '',
      ),
    ).toBe(true);

    fireEvent.change(host, { target: { value: '  smtp2.example.com  ' } });
    fireEvent.click(saveButtonOf(host));
    await waitFor(() =>
      expect(mSet).toHaveBeenCalledWith('notification.smtpHost', 'smtp2.example.com'),
    );
  });

  it('跳过校验开关：setSiteSetting(key, false) + 「已关闭」', async () => {
    renderCard();
    const [, skip] = await screen.findAllByRole('switch');
    fireEvent.click(skip);

    await waitFor(() =>
      expect(mSet).toHaveBeenCalledWith('notification.smtpInsecureSkipVerify', false),
    );
    expect(await screen.findByText('已关闭')).toBeInTheDocument();
  });

  it('邮件开关开启：setSiteSetting(key, true) + 「已开启」+ 重拉', async () => {
    mFetch.mockResolvedValue({ ...baseSettings, emailEnabled: false });
    renderCard();
    const [emailSwitch] = await screen.findAllByRole('switch');
    fireEvent.click(emailSwitch);

    await waitFor(() => expect(mSet).toHaveBeenCalledWith('notification.emailEnabled', true));
    expect(await screen.findByText('已开启')).toBeInTheDocument();
    await waitFor(() => expect(mFetch).toHaveBeenCalledTimes(2));
  });

  it('加载失败：错误提示、卡片不白屏', async () => {
    mFetch.mockRejectedValue(new Error('db down'));
    renderCard();

    expect(await screen.findByText('db down')).toBeInTheDocument();
    expect(screen.getByText('邮件通知')).toBeInTheDocument();
  });
});

// ---- 发送测试邮件（OPEN-ISSUES #51c 补欠 #55 边界） ----

describe('SmtpCard 测试邮件', () => {
  async function openAndFill(email: string) {
    renderCard();
    await screen.findByDisplayValue('smtp.example.com');
    const input = screen.getByPlaceholderText('you@example.com');
    fireEvent.change(input, { target: { value: email } });
    return input;
  }

  it('合法邮箱：调 sendTestEmail + 成功提示', async () => {
    await openAndFill('ops@example.com');
    fireEvent.click(screen.getByRole('button', { name: '发送' }));

    await waitFor(() => expect(mSend).toHaveBeenCalledWith('ops@example.com'));
    expect(await screen.findByText('测试邮件已发送，请查收')).toBeInTheDocument();
  });

  it('非法邮箱：内联校验拦截，不发请求', async () => {
    await openAndFill('not-an-email');
    fireEvent.click(screen.getByRole('button', { name: '发送' }));

    expect(await screen.findByText('邮箱格式无效')).toBeInTheDocument();
    expect(mSend).not.toHaveBeenCalled();
  });

  it('发送失败：透出后端 message', async () => {
    mSend.mockRejectedValue(new Error('smtp dial refused'));
    await openAndFill('ops@example.com');
    fireEvent.click(screen.getByRole('button', { name: '发送' }));

    expect(await screen.findByText('smtp dial refused')).toBeInTheDocument();
  });
});
