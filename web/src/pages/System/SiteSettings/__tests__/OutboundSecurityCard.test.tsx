/**
 * 出站安全与限制卡单测（OPEN-ISSUES #56）。
 *
 * 锁定契约：sec.* 三清单键读（回填）/写（文本直提、空值 = 清除覆盖回
 * 「不限」）/ssrfProtection 开关（开 = set true、关 = clear 回默认不拦
 * 截）/加载与保存失败路径。
 *
 * mock 口径沿用 SecurityTab.test.tsx：services/api/sites 四方法 jest.mock、
 * @umijs/max 本地 mock；message 提示经真实 antd App 渲染进 portal，用 DOM
 * 文本断言。
 */
import React from 'react';
import { configure, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { App, ConfigProvider } from 'antd';
import OutboundSecurityCard from '../OutboundSecurityCard';
import type { OutboundSettings } from '@/services/api/sites';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

jest.mock('@/services/api/sites', () => ({
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

import { clearSiteSetting, fetchOutboundSettings, setSiteSetting } from '@/services/api/sites';

const mFetch = fetchOutboundSettings as jest.MockedFunction<typeof fetchOutboundSettings>;
const mSet = setSiteSetting as jest.MockedFunction<typeof setSiteSetting>;
const mClear = clearSiteSetting as jest.MockedFunction<typeof clearSiteSetting>;

const baseSettings: OutboundSettings = {
  allowPorts: '',
  allowIPs: '',
  domainFilter: '',
  ssrfProtection: false,
  sources: {},
};

function renderCard() {
  return render(
    <App>
      <ConfigProvider>
        <OutboundSecurityCard />
      </ConfigProvider>
    </App>,
  );
}

/** 输入框同compact组内的「保存」按钮 */
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
});

describe('OutboundSecurityCard 加载', () => {
  it('成功：清单按快照回填，ssrfProtection 开关跟随', async () => {
    mFetch.mockResolvedValue({
      allowPorts: '443,8080',
      allowIPs: '10.0.0.0/8',
      domainFilter: 'example.com',
      ssrfProtection: true,
      sources: {},
    });
    renderCard();

    expect(await screen.findByText('出站安全与限制')).toBeInTheDocument();
    const [ports, ips, domains] = screen.getAllByRole('textbox');
    expect(ports).toHaveValue('443,8080');
    expect(ips).toHaveValue('10.0.0.0/8');
    expect(domains).toHaveValue('example.com');
    expect(screen.getByRole('switch')).toBeChecked();
  });

  it('默认全关：输入框空、开关未选、边界 Alert 在场', async () => {
    renderCard();

    expect(await screen.findByText('出站安全与限制')).toBeInTheDocument();
    screen.getAllByRole('textbox').forEach((box) => expect(box).toHaveValue(''));
    expect(screen.getByRole('switch')).not.toBeChecked();
    expect(screen.getByText(/守卫仅覆盖/)).toBeInTheDocument();
  });

  it('加载失败：错误提示走 extractErrorMessage 兜底文案', async () => {
    mFetch.mockRejectedValue(undefined); // 非 Error 对象：锁定 fallback 分支
    renderCard();

    expect(await screen.findByText('加载出站安全配置失败')).toBeInTheDocument();
  });
});

describe('OutboundSecurityCard 保存', () => {
  it('清单键文本直提：setSiteSetting(key, trimmed) + 「已保存」+ 重拉', async () => {
    renderCard();
    const ports = (await screen.findAllByRole('textbox'))[0];
    fireEvent.change(ports, { target: { value: ' 443,8080 ' } });
    fireEvent.click(saveButtonOf(ports));

    await waitFor(() => expect(mSet).toHaveBeenCalledWith('sec.allowPorts', '443,8080'));
    expect(await screen.findByText('已保存')).toBeInTheDocument();
    await waitFor(() => expect(mFetch).toHaveBeenCalledTimes(2));
    expect(mClear).not.toHaveBeenCalled();
  });

  it('空值 = 清除覆盖：走 clearSiteSetting 回「不限」', async () => {
    mFetch.mockResolvedValue({ ...baseSettings, allowPorts: '443' });
    renderCard();
    const ports = (await screen.findAllByRole('textbox'))[0];
    expect(ports).toHaveValue('443');
    fireEvent.change(ports, { target: { value: '' } });
    fireEvent.click(saveButtonOf(ports));

    await waitFor(() => expect(mClear).toHaveBeenCalledWith('sec.allowPorts'));
    expect(await screen.findByText('已保存')).toBeInTheDocument();
    expect(mSet).not.toHaveBeenCalled();
  });

  it('SSRF 开关开启：setSiteSetting(sec.ssrfProtection, true)', async () => {
    renderCard();
    const sw = await screen.findByRole('switch');
    fireEvent.click(sw);

    await waitFor(() => expect(mSet).toHaveBeenCalledWith('sec.ssrfProtection', true));
    expect(await screen.findByText('已保存')).toBeInTheDocument();
    expect(mClear).not.toHaveBeenCalled();
  });

  it('SSRF 开关关闭：clearSiteSetting 回默认不拦截', async () => {
    mFetch.mockResolvedValue({ ...baseSettings, ssrfProtection: true });
    renderCard();
    const sw = await screen.findByRole('switch');
    expect(sw).toBeChecked();
    fireEvent.click(sw); // → false

    await waitFor(() => expect(mClear).toHaveBeenCalledWith('sec.ssrfProtection'));
    expect(await screen.findByText('已保存')).toBeInTheDocument();
    expect(mSet).not.toHaveBeenCalled();
  });

  it('保存失败：错误提示透出后端 message、不重拉', async () => {
    mSet.mockRejectedValue(new Error('sec key readonly'));
    renderCard();
    const ports = (await screen.findAllByRole('textbox'))[0];
    fireEvent.change(ports, { target: { value: '443' } });
    fireEvent.click(saveButtonOf(ports));

    expect(await screen.findByText('sec key readonly')).toBeInTheDocument();
    await waitFor(() => expect(saveButtonOf(ports)).not.toHaveClass('ant-btn-loading'));
    expect(mFetch).toHaveBeenCalledTimes(1);
  });
});
