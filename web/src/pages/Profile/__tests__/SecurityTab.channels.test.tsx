/**
 * SecurityTab 通知通道回归（docs/BUGS.md BUG-016）。
 *
 * 修复前「登录通知」是一个假开关：只要 `hasPhone` 为真就渲染
 * `<Tag color="success">已开启</Tag>`，且辅助文案写「用于登录提醒和短信验证」——
 * 而仓内根本没有短信服务商。用户据此以为自己在收短信登录提醒。
 *
 * 现在：通道状态完全由后端 GET /api/v1/profile/notification-channels 下发，
 * 未接入的通道显示「未接入」+ 原因，且开关 disabled。
 */
import React from 'react';
import { render, screen } from '@testing-library/react';
import SecurityTab from '../SecurityTab';
import { channelAvailability } from '../NotificationChannels';
import type { NotificationChannelState } from '../shared';

jest.mock('../MfaSettings', () => () => <div data-testid="mfa-stub" />);

const noop = () => {};

function ch(over: Partial<NotificationChannelState>): NotificationChannelState {
  return {
    key: 'sms',
    available: false,
    userEnabled: false,
    ...over,
  };
}

function renderTab(channels: NotificationChannelState[]) {
  return render(
    <SecurityTab
      hasSessions={false}
      notificationChannels={channels}
      onShowPasswordModal={noop}
    />,
  );
}

describe('channelAvailability（纯逻辑）', () => {
  const fmt = (_id: string, fallback: string) => fallback;

  it('通道不可用 → 禁用，且给出 sms 专用原因', () => {
    const r = channelAvailability(
      ch({ key: 'sms', available: false, reason: 'sms provider not configured' }),
      fmt,
    );
    expect(r.enabled).toBe(false);
    expect(r.reason).toBe('未接入短信服务');
  });

  it('邮件未配置 SMTP → 给出 smtp 原因', () => {
    const r = channelAvailability(
      ch({ key: 'email', available: false, reason: 'smtp_not_configured' }),
      fmt,
    );
    expect(r.reason).toBe('未配置 SMTP 服务器');
  });

  it('站内信被管理员关闭 → 给出对应原因', () => {
    const r = channelAvailability(
      ch({ key: 'in_app', available: true, userEnabled: false, reason: 'in_app_disabled' }),
      fmt,
    );
    expect(r.reason).toBe('站内信已被管理员关闭');
  });

  it('可用但没填手机号 → 禁用并提示先填', () => {
    const r = channelAvailability(
      ch({ key: 'sms', available: true, requiresTarget: true, hasTarget: false }),
      fmt,
    );
    expect(r.enabled).toBe(false);
    expect(r.reason).toBe('未填写手机号，填了才能接收');
  });

  it('可用且已填手机号 → 可勾选', () => {
    const r = channelAvailability(
      ch({ key: 'sms', available: true, requiresTarget: true, hasTarget: true, userEnabled: true }),
      fmt,
    );
    expect(r.enabled).toBe(true);
    expect(r.reason).toBeNull();
  });

  it('available=false 但缺 reason 时不得变成「可勾选」', () => {
    const r = channelAvailability(ch({ key: 'sms', available: false }), fmt);
    expect(r.enabled).toBe(false);
    expect(r.reason).toBe('未接入短信服务');
  });
});

describe('SecurityTab 通知通道渲染', () => {
  it('短信未接入：显示「未接入」且开关禁用，不显示「已开启」', () => {
    renderTab([
      ch({ key: 'in_app', available: true, userEnabled: true }),
      ch({ key: 'sms', available: false, reason: 'sms provider not configured' }),
    ]);
    const row = screen.getByTestId('channel-row-sms');
    expect(row).toHaveTextContent('短信通知');
    expect(row).toHaveTextContent('未接入短信服务');
    expect(screen.getByTestId('channel-status-sms')).toHaveTextContent('未接入');
    // 关键：绝不能再出现「已开启」
    expect(row).not.toHaveTextContent('已开启');
    const sw = screen.getByTestId('channel-switch-sms');
    expect(sw).toBeDisabled();
  });

  it('已填手机号但服务商未接入 → 仍必须禁用（这是原 bug 的核心）', () => {
    renderTab([
      ch({
        key: 'sms',
        available: false,
        reason: 'sms provider not configured',
        requiresTarget: true,
        hasTarget: true, // 旧逻辑正是只看这个就显示「已开启」
      }),
    ]);
    const row = screen.getByTestId('channel-row-sms');
    expect(row).not.toHaveTextContent('已开启');
    expect(screen.getByTestId('channel-switch-sms')).toBeDisabled();
  });

  it('已接入且已填手机号：显示「已开启」；开关只读（无用户侧写接口）', () => {
    renderTab([
      ch({
        key: 'sms',
        available: true,
        userEnabled: true,
        requiresTarget: true,
        hasTarget: true,
        info: { provider: 'aliyun' },
      }),
    ]);
    expect(screen.getByTestId('channel-status-sms')).toHaveTextContent('已开启');
    const sw = screen.getByTestId('channel-switch-sms');
    // 开关是 userEnabled 的只读呈现：可点击但无效的假交互同样是被禁的假状态
    expect(sw).toBeDisabled();
    expect(sw).toBeChecked();
  });

  it('通道可达但平台未开启：显示「已关闭」，不得渲染成绿色「已开启」', () => {
    // SMTP 已配置（可达）但 emailEnabled=false：旧实现会走到「已开启」分支，
    // 与未勾选的开关自相矛盾
    renderTab([
      ch({ key: 'email', available: true, userEnabled: false, requiresTarget: true, hasTarget: true }),
    ]);
    expect(screen.getByTestId('channel-status-email')).toHaveTextContent('已关闭');
    expect(screen.getByTestId('channel-status-email')).not.toHaveTextContent('已开启');
  });

  it('未填手机号：开关禁用并提示', () => {
    renderTab([
      ch({ key: 'sms', available: true, requiresTarget: true, hasTarget: false }),
    ]);
    expect(screen.getByTestId('channel-switch-sms')).toBeDisabled();
    expect(screen.getByTestId('channel-row-sms')).toHaveTextContent('未填写手机号');
  });

  it('通道状态未加载时显示占位，而不是任何「已开启」', () => {
    renderTab([]);
    expect(screen.getByTestId('channels-loading')).toBeInTheDocument();
    expect(screen.queryByText('已开启')).toBeNull();
  });

  it('站内信默认可用：开关只读呈现开启态', () => {
    renderTab([ch({ key: 'in_app', available: true, userEnabled: true })]);
    const sw = screen.getByTestId('channel-switch-in_app');
    expect(sw).toBeDisabled();
    expect(sw).toHaveAttribute('aria-checked', 'true');
    expect(screen.getByTestId('channel-row-in_app')).toHaveTextContent('站内消息');
  });

  it('站内信被管理员关闭：标签为「已关闭」并带原因', () => {
    renderTab([ch({ key: 'in_app', available: true, userEnabled: false, reason: 'in_app_disabled' })]);
    expect(screen.getByTestId('channel-status-in_app')).toHaveTextContent('已关闭');
    expect(screen.getByTestId('channel-row-in_app')).toHaveTextContent('站内信已被管理员关闭');
  });
});
