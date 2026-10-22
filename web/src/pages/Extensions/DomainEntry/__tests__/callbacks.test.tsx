/**
 * Extensions/DomainEntry 回调与分派残余单测（覆盖率巡检：Extensions 簇
 * 余量第七批，index.tsx funcs 50% 起 → 收口）。
 *
 * 补齐既有 index.test.tsx（#46 批次 4）未触达的面：
 * ① resolveDomainMeta 三真翼——pathname 含 /approvals、/alerts、/backups
 *    的域元信息分派（标题/副标题/扩展 ID Tag），缺省翼由既有文件覆盖；
 * ② 两个导航回调——「前往扩展商店」「前往安装管理」的 history.push 目标；
 * ③ 「刷新」按钮 onClick={load} 再入加载链；
 * ④ pages map 的 fallback 翼——title 缺省 `Page ${idx+1}`、route 缺省 '-'；
 * ⑤ 安装实例 Tag 颜色双翼（>0 green/blue）。
 *
 * mock 口径：与既有文件同构（@umijs/pro-components 桩、services 双函数
 * mock、App 包裹）；useLocation 经 globalThis.__pathname 注入实现按用例
 * 切换（jest.mock 工厂提升，闭包变量会 TDZ）。
 *
 * 边界（诚实）：本文件与 index.test.tsx 合并后 index.tsx 无不可达分支
 * （load 的 try/catch/finally 全链已由既有失败用例覆盖）。
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
  useLocation: () => ({
    pathname:
      ((globalThis as Record<string, unknown>).__pathname as string | undefined) ??
      '/extensions/domain/notifications',
  }),
  history: { push: jest.fn() },
}));

jest.mock('@ant-design/pro-components', () => ({
  PageContainer: ({
    children,
    title,
    subTitle,
  }: {
    children?: React.ReactNode;
    title?: React.ReactNode;
    subTitle?: React.ReactNode;
  }) => (
    <div>
      <h1>{title}</h1>
      <p>{subTitle}</p>
      {children}
    </div>
  ),
}));

jest.mock('@/services/api/extensions', () => ({
  __esModule: true,
  listExtensionInstallations: jest.fn(),
  listExtensionPages: jest.fn(),
}));

import { history } from '@umijs/max';
import { listExtensionInstallations, listExtensionPages } from '@/services/api/extensions';
import DomainEntryPage from '../index';

const mInstalls = jest.mocked(listExtensionInstallations);
const mPages = jest.mocked(listExtensionPages);
const mPush = history.push as jest.Mock;

function setPathname(p: string) {
  (globalThis as Record<string, unknown>).__pathname = p;
}

function renderPage() {
  return render(
    <App>
      <DomainEntryPage />
    </App>,
  );
}

beforeEach(() => {
  jest.clearAllMocks();
  delete (globalThis as Record<string, unknown>).__pathname;
  mInstalls.mockResolvedValue({ total: 0, items: [] });
});

describe('DomainEntry resolveDomainMeta 分派翼', () => {
  it.each([
    ['/approvals', '审批中心（扩展化）', 'official.approval'],
    ['/alerts', '告警中心（扩展化）', 'official.alerting'],
    ['/backups', '备份管理（扩展化）', 'official.backup-advanced'],
    ['/notifications', '通知中心（扩展化）', 'official.notification'],
  ])('pathname 含 %s → 标题 + 扩展 ID Tag', async (frag, title, extId) => {
    setPathname(`/extensions/domain${frag}`);
    renderPage();

    expect(await screen.findByText(title)).toBeInTheDocument();
    expect(await screen.findByText(`扩展: ${extId}`)).toBeInTheDocument();
    expect(mInstalls).toHaveBeenCalledWith(expect.objectContaining({ extensionId: extId }));
  });
});

describe('DomainEntry 导航与刷新回调', () => {
  it('点「前往扩展商店」/「前往安装管理」→ history.push 对应路由', async () => {
    renderPage();
    await screen.findByText('尚未发现扩展页面绑定，请先安装扩展或完成绑定。');

    fireEvent.click(screen.getByRole('button', { name: '前往扩展商店' }));
    expect(mPush).toHaveBeenCalledWith('/system/extensions/store');

    fireEvent.click(screen.getByRole('button', { name: '前往安装管理' }));
    expect(mPush).toHaveBeenCalledWith('/system/extensions/installations');
  });

  it('点「刷新」→ load 再入：installations 再次拉取', async () => {
    renderPage();
    await waitFor(() => expect(mInstalls).toHaveBeenCalledTimes(1));

    fireEvent.click(screen.getByRole('button', { name: '刷新' }));
    await waitFor(() => expect(mInstalls).toHaveBeenCalledTimes(2));
    expect(mPush).not.toHaveBeenCalled();
  });
});

describe('DomainEntry pages fallback 翼与 Tag 颜色', () => {
  it('title/route 缺省回退：Page 序号 + ' - '；有实例时 Tag green/blue', async () => {
    mInstalls.mockResolvedValue({
      total: 2,
      items: [{ id: 7, extensionId: 'official.notification' } as never],
    });
    mPages.mockResolvedValue({
      items: [{ title: '通知概览', route: '/ext/notif/overview' }, {}],
    });
    renderPage();

    expect(await screen.findByText('通知概览')).toBeInTheDocument();
    expect(screen.getByText('/ext/notif/overview')).toBeInTheDocument();
    // item 缺 title/route → `Page ${idx+1}` 与 '-'
    expect(screen.getByText('Page 2')).toBeInTheDocument();
    expect(screen.getByText('-')).toBeInTheDocument();
    expect(mPages).toHaveBeenCalledWith(7);

    // installedCount > 0 双 Tag 上色翼
    expect(document.querySelector('.ant-tag-green')).not.toBeNull();
    expect(document.querySelector('.ant-tag-blue')).not.toBeNull();
  });

  it('pagesResp.items 缺省 → `|| []` 右翼回空态', async () => {
    mInstalls.mockResolvedValue({
      total: 1,
      items: [{ id: 7, extensionId: 'official.notification' } as never],
    });
    mPages.mockResolvedValue({} as never);
    renderPage();

    expect(
      await screen.findByText('尚未发现扩展页面绑定，请先安装扩展或完成绑定。'),
    ).toBeInTheDocument();
  });
});
