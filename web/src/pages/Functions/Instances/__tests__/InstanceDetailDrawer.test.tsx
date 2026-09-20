/**
 * 实例详情抽屉「变更历史」tab：
 * 1. 复用函数详情页 VersionsTab，按实例的 functionId 拉取契约版本历史；
 * 2. 抽屉切换到另一函数的实例时 VersionsTab 重挂载（key 绑定 functionId），
 *    页码等内部状态不残留——否则上一函数翻到第 2 页后会以第 2 页请求新函数。
 */
import React from 'react';
import { App as AntdApp } from 'antd';
import { fireEvent, render, screen, waitFor, configure } from '@testing-library/react';
import InstanceDetailDrawer from '../InstanceDetailDrawer';
import { getFunctionDetail, type FunctionInstance } from '@/services/api';
import { listContractVersions, type ContractVersionItem } from '@/services/api/functions';

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
  return {
    __esModule: true,
    useIntl: () => ({ formatMessage }),
    FormattedMessage: ({ defaultMessage }: { defaultMessage: string }) => defaultMessage,
    history: { push: jest.fn() },
  };
});

jest.mock('@/services/api', () => ({
  getFunctionDetail: jest.fn(),
}));

jest.mock('@/services/api/functions', () => ({
  diffContractVersions: jest.fn(),
  getContractVersion: jest.fn(),
  getFunctionAnalytics: jest.fn(),
  // VersionsTab 挂载即拉函数级版本门槛（a92f2281e），mock 缺键会 TypeError
  getFunctionVersionFloor: jest.fn().mockResolvedValue({ minVersion: '' }),
  listContractVersions: jest.fn(),
  listFunctionWarnings: jest.fn(),
}));

jest.mock('@/services/api/executionLogs', () => ({
  getExecutionLog: jest.fn(),
  listExecutionLogs: jest.fn(),
}));

const mockGetDetail = jest.mocked(getFunctionDetail);
const mockList = jest.mocked(listContractVersions);

const makeInstance = (functionId: string): FunctionInstance => ({
  agentId: 'agent-1',
  serviceId: 'svc-1',
  addr: '10.0.0.1:19091',
  version: 'v1.0.0',
  functionId,
  status: 'running',
});

const versionRow = (seq: number): ContractVersionItem => ({
  seq,
  version: `1.0.${seq}`,
  source: 'sdk',
  changeType: 'updated',
  breaking: false,
  actor: 'admin',
  createdAt: '2026-09-19T08:00:00Z',
});

const noop = () => undefined;

beforeEach(() => {
  jest.clearAllMocks();
  mockGetDetail.mockResolvedValue({
    functionId: 'player.ban',
  } as Awaited<ReturnType<typeof getFunctionDetail>>);
});

describe('InstanceDetailDrawer 变更历史 tab', () => {
  it('点开 tab → 以实例的 functionId 拉取契约版本历史', async () => {
    mockList.mockResolvedValue({ items: [versionRow(3)], total: 1, page: 1, size: 10 });
    render(
      <AntdApp>
        <InstanceDetailDrawer
          open
          instance={makeInstance('player.ban')}
          onClose={noop}
          onOpenLogs={noop}
          onOpenDebug={noop}
        />
      </AntdApp>,
    );

    // 详情拉取完成后 Tabs 才渲染，先等 tab 出现再点击
    fireEvent.click(await screen.findByText('变更历史'));
    await waitFor(() =>
      expect(mockList).toHaveBeenCalledWith('player.ban', { page: 1, pageSize: 10 }),
    );
    expect(await screen.findByText('1.0.3')).toBeInTheDocument();
  });

  it('切换到另一函数的实例 → VersionsTab 重挂载，页码不残留（以第 1 页请求新函数）', async () => {
    mockList.mockImplementation((functionId: string) =>
      functionId === 'player.ban'
        ? Promise.resolve({ items: [versionRow(3)], total: 11, page: 1, size: 10 })
        : Promise.resolve({ items: [versionRow(1)], total: 1, page: 1, size: 10 }),
    );
    const { container, rerender } = render(
      <AntdApp>
        <InstanceDetailDrawer
          open
          instance={makeInstance('player.ban')}
          onClose={noop}
          onOpenLogs={noop}
          onOpenDebug={noop}
        />
      </AntdApp>,
    );

    // 详情拉取完成后 Tabs 才渲染，先等 tab 出现再点击
    fireEvent.click(await screen.findByText('变更历史'));
    await waitFor(() =>
      expect(mockList).toHaveBeenCalledWith('player.ban', { page: 1, pageSize: 10 }),
    );
    // 等首屏行渲染完成后分页器才在 DOM 里；Drawer 是 portal 渲染，
    // 必须经 screen（document.body）查询，render 的 container 里没有抽屉内容
    await screen.findByText('1.0.3');

    // total=11 > pageSize=10 → 翻到第 2 页（分页 li 的 title 为页码）
    fireEvent.click(screen.getByTitle('2'));
    await waitFor(() =>
      expect(mockList).toHaveBeenLastCalledWith('player.ban', { page: 2, pageSize: 10 }),
    );

    // 不关抽屉直接换实例：换实例期间 Drawer loading 骨架会卸载重挂内容，
    // Tabs 回到默认概览页——等内容回来后重新点开「变更历史」。
    // 断言行为契约：新函数以第 1 页请求（页码不残留；若残留会以 page 2 请求）。
    rerender(
      <AntdApp>
        <InstanceDetailDrawer
          open
          instance={makeInstance('player.kick')}
          onClose={noop}
          onOpenLogs={noop}
          onOpenDebug={noop}
        />
      </AntdApp>,
    );
    fireEvent.click(await screen.findByText('变更历史'));
    await waitFor(() =>
      expect(mockList).toHaveBeenLastCalledWith('player.kick', { page: 1, pageSize: 10 }),
    );
    expect(await screen.findByText('1.0.1')).toBeInTheDocument();
  });
});
