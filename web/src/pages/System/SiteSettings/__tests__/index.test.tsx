/**
 * 系统设置 / 系统信息页回归（主树最大零覆盖模块 2118 行的入口文件）。
 *
 * 核心契约：配置三层（代码默认 ← 配置文件 ← 数据库覆盖最高）——
 * a. sources 里 source==='database' 的字段显示「数据库覆盖」徽标 + 「恢复」按钮；
 *    'config' 显示「跟随配置文件」；无来源不显示徽标。
 * b. 保存走 setSiteSetting(L3 key, trim 后的值)，成功后重拉列表；
 *    值为空/纯空白时不提交。
 * c. 「恢复」走 clearSiteSetting(L3 key)，成功后重拉。
 * d. 加载/保存失败页面不崩，不重拉。
 *
 * 边界（诚实）：四个子 Tab（FeatureFlags/Auth/Notification/Observability）
 * 本轮以桩替换未覆盖，各自的服务 mock 矩阵留独立批次。
 */
import React from 'react';
import { configure, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App, ConfigProvider } from 'antd';
import SiteSettingsPage from '../index';
import type { ReactNode } from 'react';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

jest.mock('@/services/api/sites', () => ({
  fetchSiteConfig: jest.fn(),
  setSiteSetting: jest.fn(),
  clearSiteSetting: jest.fn(),
}));

jest.mock('@ant-design/pro-components', () => ({
  PageContainer: (props: { children?: ReactNode }) => <div>{props.children}</div>,
}));

jest.mock('@umijs/max', () => ({
  FormattedMessage: ({ defaultMessage }: { defaultMessage?: string }) => (
    <>{defaultMessage ?? ''}</>
  ),
  useIntl: () => ({
    formatMessage: (opts: { defaultMessage?: string }, values?: Record<string, string>) => {
      let text = opts.defaultMessage ?? '';
      if (values) {
        for (const [k, v] of Object.entries(values)) text = text.replace(`{${k}}`, v);
      }
      return text;
    },
  }),
}));

// 四个子 Tab 桩替换：antd Tabs 非激活面板不挂载，桩仅为 hermetic 兜底
jest.mock('../FeatureFlagsTab', () => () => <div data-testid="tab-features" />);
jest.mock('../AuthTab', () => () => <div data-testid="tab-auth" />);
jest.mock('../NotificationTab', () => () => <div data-testid="tab-notification" />);
jest.mock('../ObservabilityTab', () => () => <div data-testid="tab-observability" />);

import { clearSiteSetting, fetchSiteConfig, setSiteSetting } from '@/services/api/sites';

const mFetch = fetchSiteConfig as jest.MockedFunction<typeof fetchSiteConfig>;
const mSet = setSiteSetting as jest.MockedFunction<typeof setSiteSetting>;
const mClear = clearSiteSetting as jest.MockedFunction<typeof clearSiteSetting>;

const baseConfig = {
  siteName: 'Croupier',
  logoUrl: '/logo.svg',
  faviconUrl: '/favicon.ico',
  description: '游戏运营平台',
  serverUrl: 'https://gm.example.com',
  taskPublicUrl: 'https://tasks.example.com',
  docsUrl: 'https://docs.example.com',
  homeContent: '欢迎接入',
  userAgreement: '协议全文示例',
  privacyPolicy: '隐私政策全文示例',
  footerCopyright: '© 2026 Croupier',
  footerIcp: '京ICP备12345678号',
};

function renderPage() {
  return render(
    <App>
      <ConfigProvider>
        <SiteSettingsPage />
      </ConfigProvider>
    </App>,
  );
}

beforeEach(() => {
  jest.clearAllMocks();
  mFetch.mockResolvedValue({ ...baseConfig });
  mSet.mockResolvedValue(undefined);
  mClear.mockResolvedValue(undefined);
});

describe('SiteSettings 系统信息', () => {
  it('加载回填表单并按来源渲染分层徽标：database→覆盖+恢复、config→跟随', async () => {
    mFetch.mockResolvedValue({
      ...baseConfig,
      sources: { 'site.name': 'database', 'footer.icp': 'config' },
    });
    renderPage();

    expect(await screen.findByDisplayValue('Croupier')).toBeInTheDocument();
    expect(screen.getByDisplayValue('/logo.svg')).toBeInTheDocument();
    expect(screen.getByDisplayValue('京ICP备12345678号')).toBeInTheDocument();

    // site.name=database：数据库覆盖徽标 + 恢复按钮
    expect(screen.getByText('数据库覆盖')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '恢复' })).toBeInTheDocument();
    // footer.icp=config：跟随配置文件徽标，无恢复按钮
    expect(screen.getByText('跟随配置文件')).toBeInTheDocument();
    expect(screen.getAllByRole('button', { name: '保存' })).toHaveLength(12);
    // 系统信息扩展字段（OPEN-ISSUES #49）回填
    expect(screen.getByDisplayValue('https://gm.example.com')).toBeInTheDocument();
    expect(screen.getByDisplayValue('https://tasks.example.com')).toBeInTheDocument();
    expect(screen.getByDisplayValue('https://docs.example.com')).toBeInTheDocument();
    expect(screen.getByDisplayValue('欢迎接入')).toBeInTheDocument();
    expect(screen.getByDisplayValue('协议全文示例')).toBeInTheDocument();
    expect(screen.getByDisplayValue('隐私政策全文示例')).toBeInTheDocument();
  });

  it('系统信息扩展字段保存：docsUrl 提交 trim 后的值，homeContent 清空不提交', async () => {
    renderPage();
    const docs = await screen.findByDisplayValue('https://docs.example.com');
    fireEvent.change(docs, { target: { value: '  https://docs.new  ' } });
    // 字段 DOM 序：siteName(0) logoUrl(1) faviconUrl(2) description(3)
    // serverUrl(4) taskPublicUrl(5) docsUrl(6)…
    fireEvent.click(screen.getAllByRole('button', { name: '保存' })[6]);
    await waitFor(() => expect(mSet).toHaveBeenCalledWith('site.docsUrl', 'https://docs.new'));
    expect(mFetch).toHaveBeenCalledTimes(2);

    // 空值不提交（saveField 空值守卫），homeContent 保存钮为索引 7
    const home = screen.getByDisplayValue('欢迎接入');
    fireEvent.change(home, { target: { value: '   ' } });
    fireEvent.click(screen.getAllByRole('button', { name: '保存' })[7]);
    expect(mSet).not.toHaveBeenCalledWith('site.homeContent', '');
  });

  it('保存：提交 trim 后的值并重拉；空值不提交', async () => {
    renderPage();
    const input = await screen.findByDisplayValue('Croupier');
    fireEvent.change(input, { target: { value: '  新站名  ' } });
    fireEvent.click(screen.getAllByRole('button', { name: '保存' })[0]);

    await waitFor(() => expect(mSet).toHaveBeenCalledWith('site.name', '新站名'));
    // 成功后重拉（初始 1 次 + 保存后 1 次）
    await waitFor(() => expect(mFetch).toHaveBeenCalledTimes(2));

    // 清空后保存：空值守卫，不提交
    fireEvent.change(input, { target: { value: '   ' } });
    fireEvent.click(screen.getAllByRole('button', { name: '保存' })[0]);
    expect(mSet).toHaveBeenCalledTimes(1);
  });

  it('恢复：clearSiteSetting 删除覆盖并重拉', async () => {
    mFetch.mockResolvedValue({
      ...baseConfig,
      sources: { 'site.name': 'database' },
    });
    renderPage();
    fireEvent.click(await screen.findByRole('button', { name: '恢复' }));

    await waitFor(() => expect(mClear).toHaveBeenCalledWith('site.name'));
    await waitFor(() => expect(mFetch).toHaveBeenCalledTimes(2));
    expect(mSet).not.toHaveBeenCalled();
  });

  it('保存失败：页面不崩、不重拉、按钮退出 loading', async () => {
    mSet.mockRejectedValue(new Error('boom'));
    renderPage();
    const input = await screen.findByDisplayValue('Croupier');
    fireEvent.change(input, { target: { value: 'X' } });
    fireEvent.click(screen.getAllByRole('button', { name: '保存' })[0]);

    await waitFor(() => expect(mSet).toHaveBeenCalled());
    expect(screen.getAllByRole('button', { name: '保存' })[0]).not.toBeDisabled();
    expect(mFetch).toHaveBeenCalledTimes(1);
    expect(screen.getByDisplayValue('X')).toBeInTheDocument();
  });

  it('恢复失败：页面不崩、不重拉（clearSiteSetting 拒绝路径）', async () => {
    mClear.mockRejectedValue(new Error('boom'));
    mFetch.mockResolvedValue({
      ...baseConfig,
      sources: { 'site.name': 'database' },
    });
    renderPage();
    fireEvent.click(await screen.findByRole('button', { name: '恢复' }));

    await waitFor(() => expect(mClear).toHaveBeenCalledWith('site.name'));
    expect(mFetch).toHaveBeenCalledTimes(1);
    expect(screen.getByRole('button', { name: '恢复' })).toBeInTheDocument();
  });

  it('无来源字段渲染「默认」徽标（三层最低层可见）', async () => {
    mFetch.mockResolvedValue({
      ...baseConfig,
      sources: { 'site.logoUrl': 'default' },
    });
    renderPage();
    expect(await screen.findByDisplayValue('Croupier')).toBeInTheDocument();
    expect(screen.getByText('默认')).toBeInTheDocument();
    // default 来源不提供恢复按钮
    expect(screen.queryByRole('button', { name: '恢复' })).not.toBeInTheDocument();
  });

  it('加载失败：页面仍渲染（空表单），不白屏', async () => {
    mFetch.mockRejectedValue(new Error('boom'));
    renderPage();

    await waitFor(() => expect(mFetch).toHaveBeenCalled());
    expect(screen.getByText('系统信息')).toBeInTheDocument();
    expect(screen.getAllByRole('button', { name: '保存' })).toHaveLength(12);
    expect(screen.queryByDisplayValue('Croupier')).not.toBeInTheDocument();
  });
});
