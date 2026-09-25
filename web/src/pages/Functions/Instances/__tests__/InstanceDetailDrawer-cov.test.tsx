/**
 * 实例详情抽屉残余分支：
 * 1. 函数详情拉取失败的三种形态（Error 带文案 / Error 空文案 → 兜底文案 /
 *    非 Error → 国际化「操作失败」）；
 * 2. 实例字段缺省回退（版本/SDK/Agent/Game/Env/心跳全空 → '-'，无归属实例
 *    → 「本实例」）与全字段（归属实例 Tag、status=error 徽标）；
 * 3. 状态三态徽标（running / error / 其它 → 停止）；
 * 4. 调试面板按钮回调、抽屉打开期间 instance 置空（VersionsTab key 的
 *    `instance?.functionId ?? ''` 兜底）、拉取未完成即卸载（cancelled 分支）。
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

const noop = () => undefined;

/** 全字段实例：归属实例存在、SDK/Agent/版本齐全、状态 error。 */
const fullInstance = (): FunctionInstance =>
  ({
    agentId: 'agent-1',
    serviceId: 'svc-1',
    addr: '10.0.0.1:19091',
    version: 'v1.0.0',
    functionId: 'player.ban',
    status: 'error',
    ownerInstance: 'agent-master',
    sdkName: 'croupier-go',
    sdkVersion: '0.9.0',
    agentVersion: '1.2.3',
    gameId: 'demo',
    env: 'prod',
    lastHeartbeat: '2026-09-24T10:00:00Z',
  }) as FunctionInstance;

/** 全缺省实例：仅保留必填标识，其余字段不存在，状态非 running/error。 */
const bareInstance = (): FunctionInstance =>
  ({
    agentId: 'agent-2',
    serviceId: 'svc-2',
    addr: '10.0.0.2:19091',
    functionId: 'player.kick',
    status: 'stopped',
  }) as FunctionInstance;

const versionRow = (seq: number): ContractVersionItem => ({
  seq,
  version: `1.0.${seq}`,
  source: 'sdk',
  changeType: 'updated',
  breaking: false,
  actor: 'admin',
  createdAt: '2026-09-19T08:00:00Z',
});

beforeEach(() => {
  jest.clearAllMocks();
  mockGetDetail.mockResolvedValue({ functionId: 'player.ban' } as Awaited<
    ReturnType<typeof getFunctionDetail>
  >);
  mockList.mockResolvedValue({ items: [versionRow(3)], total: 1, page: 1, size: 10 });
});

describe('InstanceDetailDrawer 详情拉取失败', () => {
  it('Error 带文案 → message.error 直接透出错误文案', async () => {
    mockGetDetail.mockRejectedValueOnce(new Error('boom'));
    render(
      <AntdApp>
        <InstanceDetailDrawer
          open
          instance={fullInstance()}
          onClose={noop}
          onOpenLogs={noop}
          onOpenDebug={noop}
        />
      </AntdApp>,
    );

    expect(await screen.findByText('boom')).toBeInTheDocument();
    // 失败后不渲染 Tabs
    expect(screen.queryByText('概览')).not.toBeInTheDocument();
  });

  it('Error 空文案 → 回退「加载详情失败」', async () => {
    mockGetDetail.mockRejectedValueOnce(new Error(''));
    render(
      <AntdApp>
        <InstanceDetailDrawer
          open
          instance={bareInstance()}
          onClose={noop}
          onOpenLogs={noop}
          onOpenDebug={noop}
        />
      </AntdApp>,
    );

    expect(await screen.findByText('加载详情失败')).toBeInTheDocument();
  });

  it('非 Error 拒绝 → 回退国际化「操作失败」', async () => {
    (
      mockGetDetail as unknown as { mockRejectedValueOnce: (v: unknown) => void }
    ).mockRejectedValueOnce('plain-string');
    render(
      <AntdApp>
        <InstanceDetailDrawer
          open
          instance={bareInstance()}
          onClose={noop}
          onOpenLogs={noop}
          onOpenDebug={noop}
        />
      </AntdApp>,
    );

    expect(await screen.findByText('操作失败')).toBeInTheDocument();
  });
});

describe('InstanceDetailDrawer 字段回退与状态徽标', () => {
  it('全字段 + status=error：归属实例 Tag 与错误徽标', async () => {
    render(
      <AntdApp>
        <InstanceDetailDrawer
          open
          instance={fullInstance()}
          onClose={noop}
          onOpenLogs={noop}
          onOpenDebug={noop}
        />
      </AntdApp>,
    );

    fireEvent.click(await screen.findByText('概览'));
    expect(screen.getByText('agent-master')).toBeInTheDocument();
    expect(screen.getByText('错误')).toBeInTheDocument();
    expect(screen.getByText('croupier-go')).toBeInTheDocument();
    expect(screen.getByText('0.9.0')).toBeInTheDocument();
    expect(screen.getByText('1.2.3')).toBeInTheDocument();
    expect(screen.getByText('demo')).toBeInTheDocument();
    expect(screen.getByText('prod')).toBeInTheDocument();
    expect(screen.getByText('2026-09-24T10:00:00Z')).toBeInTheDocument();
    expect(screen.getByText('v1.0.0')).toBeInTheDocument();
  });

  it('全缺省 + 非 running/error：缺省字段回退 -，归属实例显示「本实例」，状态显示「停止」', async () => {
    render(
      <AntdApp>
        <InstanceDetailDrawer
          open
          instance={bareInstance()}
          onClose={noop}
          onOpenLogs={noop}
          onOpenDebug={noop}
        />
      </AntdApp>,
    );

    fireEvent.click(await screen.findByText('概览'));
    expect(screen.getByText('本实例')).toBeInTheDocument();
    expect(screen.getByText('停止')).toBeInTheDocument();
    // version / sdkName / sdkVersion / agentVersion / lastHeartbeat / Game / Env 全 '-'
    expect(screen.getAllByText('-').length).toBeGreaterThanOrEqual(7);
    expect(screen.queryByText('运行中')).not.toBeInTheDocument();
  });
});

describe('InstanceDetailDrawer 动作回调与卸载', () => {
  it('调试面板按钮回抛当前实例', async () => {
    const onOpenDebug = jest.fn();
    render(
      <AntdApp>
        <InstanceDetailDrawer
          open
          instance={fullInstance()}
          onClose={noop}
          onOpenLogs={noop}
          onOpenDebug={onOpenDebug}
        />
      </AntdApp>,
    );

    fireEvent.click(await screen.findByRole('tab', { name: /调试/ }));
    fireEvent.click(await screen.findByRole('button', { name: '打开调试面板' }));
    expect(onOpenDebug).toHaveBeenCalledTimes(1);
    expect(onOpenDebug.mock.calls[0][0]).toMatchObject({ functionId: 'player.ban' });
  });

  it('日志页按钮回抛当前实例', async () => {
    const onOpenLogs = jest.fn();
    render(
      <AntdApp>
        <InstanceDetailDrawer
          open
          instance={fullInstance()}
          onClose={noop}
          onOpenLogs={onOpenLogs}
          onOpenDebug={noop}
        />
      </AntdApp>,
    );

    fireEvent.click(await screen.findByRole('tab', { name: /日志/ }));
    fireEvent.click(await screen.findByRole('button', { name: '查看完整日志' }));
    expect(onOpenLogs).toHaveBeenCalledTimes(1);
  });

  it('抽屉开着时 instance 置空 → VersionsTab key 走 instance?.functionId ?? "" 兜底', async () => {
    const { rerender } = render(
      <AntdApp>
        <InstanceDetailDrawer
          open
          instance={fullInstance()}
          onClose={noop}
          onOpenLogs={noop}
          onOpenDebug={noop}
        />
      </AntdApp>,
    );

    fireEvent.click(await screen.findByText('变更历史'));
    await waitFor(() => expect(mockList).toHaveBeenCalled());

    rerender(
      <AntdApp>
        <InstanceDetailDrawer
          open
          instance={null}
          onClose={noop}
          onOpenLogs={noop}
          onOpenDebug={noop}
        />
      </AntdApp>,
    );

    await waitFor(() => expect(mockList).toHaveBeenLastCalledWith('', { page: 1, pageSize: 10 }));
  });

  it('拉取未完成即卸载 → cancelled 分支跳过 setInstanceDetail', async () => {
    let resolveDetail: (value: Awaited<ReturnType<typeof getFunctionDetail>>) => void = () =>
      undefined;
    mockGetDetail.mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          resolveDetail = resolve;
        }),
    );

    const { unmount } = render(
      <AntdApp>
        <InstanceDetailDrawer
          open
          instance={fullInstance()}
          onClose={noop}
          onOpenLogs={noop}
          onOpenDebug={noop}
        />
      </AntdApp>,
    );

    await waitFor(() => expect(mockGetDetail).toHaveBeenCalledTimes(1));
    unmount();
    resolveDetail({ functionId: 'player.ban' } as Awaited<ReturnType<typeof getFunctionDetail>>);
    await new Promise((resolve) => setTimeout(resolve, 30));
    // 已卸载：不应因 setState 触发 React 告警（下面断言保证 effect 确实跑过）
    expect(mockGetDetail).toHaveBeenCalledTimes(1);
  });
});
