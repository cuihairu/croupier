/**
 * 账号安全策略子 Tab 单测。
 *
 * 锁定契约：六键快照读（回填 + 未配置回零，唯一例外 approvalStepUpOtp
 * 默认开）/写（bool 开关与数字直提、空值/false/0 = 清除覆盖回默认全关，
 * defaultOn 键关闭时显式落 false 不清除）/加载与保存失败路径。
 *
 * mock 口径沿用 __tests__/index.test.tsx：services/api/sites 三方法 jest.mock、
 * @umijs/max 本地 mock。message 提示经真实 antd App 渲染进 portal，用 DOM
 * 文本断言；开关无 accessible name，按 DOM 序索引定位（approvalStepUpOtp
 * 在前，其后 mfaRequired）。
 *
 * 边界（诚实）：保存按钮在 Form.Item 之外的 Row 内（与 NotificationTab 不同，
 * 不能用 .ant-form-item 定位），按 .ant-row 定位并断言所在行含对应控件。
 */
import React from 'react';
import { configure, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { App, ConfigProvider } from 'antd';
import SecurityTab from '../SecurityTab';
import type { SecuritySettings } from '@/services/api/sites';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

jest.mock('@/services/api/sites', () => ({
  fetchSecuritySettings: jest.fn(),
  fetchOutboundSettings: jest.fn(),
  setSiteSetting: jest.fn(),
  clearSiteSetting: jest.fn(),
}));

jest.mock('@umijs/max', () => ({
  FormattedMessage: ({ defaultMessage }: { defaultMessage?: string }) => <>{defaultMessage}</>,
  useIntl: () => ({
    formatMessage: (opts: { defaultMessage?: string }) => opts.defaultMessage ?? '',
  }),
}));

import {
  clearSiteSetting,
  fetchOutboundSettings,
  fetchSecuritySettings,
  setSiteSetting,
} from '@/services/api/sites';

const mFetch = fetchSecuritySettings as jest.MockedFunction<typeof fetchSecuritySettings>;
const mFetchOutbound = fetchOutboundSettings as jest.MockedFunction<typeof fetchOutboundSettings>;
const mSet = setSiteSetting as jest.MockedFunction<typeof setSiteSetting>;
const mClear = clearSiteSetting as jest.MockedFunction<typeof clearSiteSetting>;

const baseSettings: SecuritySettings = {
  approvalStepUpOtp: true,
  mfaRequired: false,
  passwordMinLength: 0,
  passwordRequireUppercase: false,
  passwordRequireSpecial: false,
  passwordMaxAgeDays: 0,
};

function renderTab() {
  return render(
    <App>
      <ConfigProvider>
        <SecurityTab />
      </ConfigProvider>
    </App>,
  );
}

/** 控件所在 Row 内的「保存」按钮（按钮在 Form.Item 外；排除 Form.Item 内部的 ant-form-item-row） */
function saveButtonOf(control: HTMLElement): HTMLElement {
  const row = control.closest('.ant-row:not(.ant-form-item-row)');
  expect(row).not.toBeNull();
  return within(row as HTMLElement).getByRole('button', { name: '保存' });
}

/** 按卡标题取卡片根节点（#56 起双卡并存：账号安全策略 + 出站安全与限制，查询须按卡收窄） */
function cardOf(title: string): HTMLElement {
  const heading = screen.getByText(title);
  return heading.closest('.ant-card') as HTMLElement;
}

beforeEach(() => {
  jest.clearAllMocks();
  mFetch.mockResolvedValue({ ...baseSettings });
  mFetchOutbound.mockResolvedValue({
    allowPorts: '',
    allowIPs: '',
    domainFilter: '',
    ssrfProtection: false,
    sources: {},
  });
  mSet.mockResolvedValue(undefined);
  mClear.mockResolvedValue(undefined);
});

describe('SecurityTab 加载', () => {
  it('成功：开关与数字按快照回填（approvalStepUpOtp 默认开，其余默认关）', async () => {
    mFetch.mockResolvedValue({
      approvalStepUpOtp: true,
      mfaRequired: true,
      passwordMinLength: 12,
      passwordRequireUppercase: true,
      passwordRequireSpecial: false,
      passwordMaxAgeDays: 90,
    });
    renderTab();

    expect(await screen.findByText('高危审批二次验证 (TOTP)')).toBeInTheDocument();
    const [stepUp, mfa, upper, special] = within(cardOf('账号安全策略')).getAllByRole('switch');
    expect(stepUp).toBeChecked();
    expect(mfa).toBeChecked();
    expect(upper).toBeChecked();
    expect(special).not.toBeChecked();
    const [minLength, maxAge] = screen.getAllByRole('spinbutton');
    expect(minLength).toHaveValue('12');
    expect(maxAge).toHaveValue('90');
  });

  it('快照缺省字段：approvalStepUpOtp 缺省回开（!== false），其余回零（?? 兜底分支）', async () => {
    mFetch.mockResolvedValue({});
    renderTab();

    await screen.findByText('高危审批二次验证 (TOTP)');
    const [stepUp, mfa, upper, special] = within(cardOf('账号安全策略')).getAllByRole('switch');
    expect(stepUp).toBeChecked();
    expect(mfa).not.toBeChecked();
    expect(upper).not.toBeChecked();
    expect(special).not.toBeChecked();
    screen.getAllByRole('spinbutton').forEach((num) => expect(num).toHaveValue('0'));
    expect(within(cardOf('账号安全策略')).getByText(/全部默认关闭/)).toBeInTheDocument();
  });

  it('加载失败：错误提示走 extractErrorMessage 兜底文案，表单落默认关闭态', async () => {
    mFetch.mockRejectedValue(undefined); // 非 Error 对象：锁定 fallback 分支
    renderTab();

    expect(await screen.findByText('加载账号安全策略失败')).toBeInTheDocument();
    // 表单仍渲染（Card 只在 loading 期出骨架），字段未回填 = 默认全关
    within(cardOf('账号安全策略'))
      .getAllByRole('switch')
      .forEach((sw) => expect(sw).not.toBeChecked());
  });
});

describe('SecurityTab 保存（saveKey）', () => {
  it('开关开启：setSiteSetting(key, true) + 「已保存」+ 重拉', async () => {
    renderTab();
    const mfa = (await within(cardOf('账号安全策略')).findAllByRole('switch'))[1];
    fireEvent.click(mfa);
    fireEvent.click(saveButtonOf(mfa));

    await waitFor(() => expect(mSet).toHaveBeenCalledWith('security.mfaRequired', true));
    expect(await screen.findByText('已保存')).toBeInTheDocument();
    await waitFor(() => expect(mFetch).toHaveBeenCalledTimes(2));
    expect(mClear).not.toHaveBeenCalled();
  });

  it('开关关闭：false 走 clearSiteSetting 回默认', async () => {
    mFetch.mockResolvedValue({ ...baseSettings, mfaRequired: true });
    renderTab();
    const mfa = (await within(cardOf('账号安全策略')).findAllByRole('switch'))[1];
    expect(mfa).toBeChecked();
    fireEvent.click(mfa); // → false
    fireEvent.click(saveButtonOf(mfa));

    await waitFor(() => expect(mClear).toHaveBeenCalledWith('security.mfaRequired'));
    expect(await screen.findByText('已保存')).toBeInTheDocument();
    expect(mSet).not.toHaveBeenCalled();
  });

  // #61：defaultOn 键的保存语义与其余 bool 键相反——默认开，关闭必须显式
  // 落 false（走 clear 会回到开、关不掉）。
  it('approvalStepUpOtp 关闭：显式 setSiteSetting(key, false)，不走 clear', async () => {
    renderTab(); // baseSettings.approvalStepUpOtp = true（默认开）
    const stepUp = (await within(cardOf('账号安全策略')).findAllByRole('switch'))[0];
    expect(stepUp).toBeChecked();
    fireEvent.click(stepUp); // → false
    fireEvent.click(saveButtonOf(stepUp));

    await waitFor(() => expect(mSet).toHaveBeenCalledWith('security.approvalStepUpOtp', false));
    expect(await screen.findByText('已保存')).toBeInTheDocument();
    expect(mClear).not.toHaveBeenCalled();
  });

  it('approvalStepUpOtp 回开：setSiteSetting(key, true)', async () => {
    mFetch.mockResolvedValue({ ...baseSettings, approvalStepUpOtp: false });
    renderTab();
    const stepUp = (await within(cardOf('账号安全策略')).findAllByRole('switch'))[0];
    expect(stepUp).not.toBeChecked();
    fireEvent.click(stepUp); // → true
    fireEvent.click(saveButtonOf(stepUp));

    await waitFor(() => expect(mSet).toHaveBeenCalledWith('security.approvalStepUpOtp', true));
    expect(await screen.findByText('已保存')).toBeInTheDocument();
    expect(mClear).not.toHaveBeenCalled();
  });

  it('数字字段直提（非字符串分支）：12 提交 setSiteSetting(key, 12)', async () => {
    renderTab();
    const minLength = (await screen.findAllByRole('spinbutton'))[0];
    fireEvent.change(minLength, { target: { value: '12' } });
    fireEvent.click(saveButtonOf(minLength));

    await waitFor(() => expect(mSet).toHaveBeenCalledWith('security.passwordMinLength', 12));
    expect(await screen.findByText('已保存')).toBeInTheDocument();
    expect(mClear).not.toHaveBeenCalled();
  });

  it('数字 0 = 清除覆盖：走 clearSiteSetting 且不带 0', async () => {
    mFetch.mockResolvedValue({ ...baseSettings, passwordMaxAgeDays: 30 });
    renderTab();
    const [, maxAge] = await screen.findAllByRole('spinbutton'); // DOM 序：minLength 在前
    expect(maxAge).toHaveValue('30');
    fireEvent.change(maxAge, { target: { value: '0' } });
    fireEvent.click(saveButtonOf(maxAge));

    await waitFor(() => expect(mClear).toHaveBeenCalledWith('security.passwordMaxAgeDays'));
    expect(await screen.findByText('已保存')).toBeInTheDocument();
    expect(mSet).not.toHaveBeenCalled();
  });

  it('保存失败：错误提示透出后端 message、不重拉、按钮退出 loading', async () => {
    mSet.mockRejectedValue(new Error('policy key readonly'));
    renderTab();
    const mfa = (await within(cardOf('账号安全策略')).findAllByRole('switch'))[1];
    fireEvent.click(mfa);
    fireEvent.click(saveButtonOf(mfa));

    expect(await screen.findByText('policy key readonly')).toBeInTheDocument();
    await waitFor(() => expect(saveButtonOf(mfa)).not.toHaveClass('ant-btn-loading'));
    expect(mFetch).toHaveBeenCalledTimes(1);
  });

  it('保存失败（无可提取信息）：「保存失败」兜底文案', async () => {
    mSet.mockRejectedValue(undefined);
    renderTab();
    const mfa = (await within(cardOf('账号安全策略')).findAllByRole('switch'))[1];
    fireEvent.click(mfa);
    fireEvent.click(saveButtonOf(mfa));

    expect(await screen.findByText('保存失败')).toBeInTheDocument();
  });
});
