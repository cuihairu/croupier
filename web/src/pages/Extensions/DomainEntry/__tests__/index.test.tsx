/**
 * Extensions/DomainEntry 页面回归（OPEN-ISSUES #46 批次 4）
 *
 * 批次 1 修复后（canonical 端点 installations/:id/pages + 移除 .catch 静默），
 * 本批补页面级真实渲染用例，替换「入口区块恒 Empty」的历史现状：
 * ① 有安装实例 → 拉页面绑定 → 入口卡渲染 title/route；
 * ② 无安装实例 → Empty 文案且不调 pages 端点；
 * ③ 拉取失败 → message.error 提示（批次 1 前被静默吞掉），页面不崩。
 */
import { configure, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App } from 'antd';
import React from 'react';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

jest.mock('@umijs/max', () => ({
  __esModule: true,
  FormattedMessage: ({ defaultMessage }: { defaultMessage?: string }) => (
    <>{defaultMessage ?? ''}</>
  ),
  useIntl: () => ({
    formatMessage: (opts: { defaultMessage?: string }) => opts.defaultMessage ?? '',
  }),
  useLocation: () => ({ pathname: '/extensions/domain/notifications' }),
  history: { push: jest.fn() },
}));

jest.mock('@ant-design/pro-components', () => ({
  PageContainer: ({ children, title }: { children?: React.ReactNode; title?: React.ReactNode }) => (
    <div>
      <h1>{title}</h1>
      {children}
    </div>
  ),
}));

jest.mock('@/services/api/extensions', () => ({
  __esModule: true,
  listExtensionInstallations: jest.fn(),
  listExtensionPages: jest.fn(),
}));

import { listExtensionInstallations, listExtensionPages } from '@/services/api/extensions';
import DomainEntryPage from '../index';

const mInstalls = jest.mocked(listExtensionInstallations);
const mPages = jest.mocked(listExtensionPages);

/** 包真实 antd App，Probe 先于页面渲染，把 message.error 换成 spy 供页面解构 */
function renderPage() {
  const messageError = jest.fn();
  const Probe: React.FC = () => {
    const api = App.useApp();
    api.message.error = messageError;
    return null;
  };
  const utils = render(
    <App>
      <Probe />
      <DomainEntryPage />
    </App>,
  );
  return { messageError, ...utils };
}

beforeEach(() => {
  jest.clearAllMocks();
});

describe('Extensions/DomainEntry 页面（#46 批次 4）', () => {
  it('有安装实例：拉页面绑定并渲染入口卡（title + route）', async () => {
    mInstalls.mockResolvedValue({
      total: 1,
      items: [{ id: 7, extensionId: 'official.notification' } as never],
    });
    mPages.mockResolvedValue({
      items: [{ title: '通知概览', route: '/ext/notif/overview' }],
    });
    renderPage();

    expect(await screen.findByText('扩展页面入口')).toBeInTheDocument();
    expect(screen.getByText('通知概览')).toBeInTheDocument();
    expect(screen.getByText('/ext/notif/overview')).toBeInTheDocument();
    expect(mPages).toHaveBeenCalledWith(7);
  });

  it('无安装实例：Empty 引导且不调 pages 端点', async () => {
    mInstalls.mockResolvedValue({ total: 0, items: [] });
    renderPage();

    expect(
      await screen.findByText('尚未发现扩展页面绑定，请先安装扩展或完成绑定。'),
    ).toBeInTheDocument();
    expect(mPages).not.toHaveBeenCalled();
  });

  it('拉取失败：message.error 提示（不再静默吞错）且页面不崩', async () => {
    mInstalls.mockRejectedValue(new Error('boom'));
    const { messageError } = renderPage();

    // 首屏 useEffect 已失败一次；点「刷新」再触发以捕获 spy
    fireEvent.click(await screen.findByRole('button', { name: '刷新' }));
    await waitFor(() => expect(messageError).toHaveBeenCalled());
    // 页面结构仍在：三个动作按钮
    expect(screen.getByRole('button', { name: '前往扩展商店' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '前往安装管理' })).toBeInTheDocument();
  });
});
