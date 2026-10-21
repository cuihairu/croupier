import React from 'react';
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App as AntdApp, ConfigProvider } from 'antd';
import { configure } from '@testing-library/react';
import OpsAgentsPage, { AGENT_META } from '../index';
import { listOpsAgentsByType, listOpsTasks } from '@/services/api/ops';
import { setScope } from '@/stores/scope';

jest.mock('@/services/api/ops', () => ({
  listOpsAgentsByType: jest.fn(),
  listOpsTasks: jest.fn(),
}));
jest.mock('@umijs/max', () => ({
  FormattedMessage: ({ defaultMessage }: { defaultMessage?: string }) => (
    <>{defaultMessage ?? ''}</>
  ),
  useIntl: () => ({
    formatMessage: ({ defaultMessage }: { defaultMessage?: string }) => defaultMessage ?? '',
  }),
}));

const { listOpsAgentsByType, listOpsTasks } = jest.requireMock('@/services/api/ops') as {
  listOpsAgentsByType: jest.Mock;
  listOpsTasks: jest.Mock;
};

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 10000 });

beforeEach(() => {
  listOpsAgentsByType.mockReset();
  listOpsTasks.mockReset();
  setScope({ gameId: 'demo', env: 'prod' }, { persist: false, emit: false });
});

jest.mock('../../Nodes/SupervisorDrawer', () => ({
  __esModule: true,
  default: ({
    node,
    onClose,
  }: {
    node: { agentId: string; gameId?: string; env?: string } | null;
    onClose: () => void;
  }) =>
    node ? (
      <div>
        <span>{`supervisor:${node.agentId}:${node.gameId ?? ''}:${node.env ?? ''}`}</span>
        <button type="button" onClick={onClose}>
          close-supervisor
        </button>
      </div>
    ) : null,
}));

it('shows the scope empty state for an agent type with no instances', async () => {
  listOpsAgentsByType.mockResolvedValue({ nodes: [] });
  render(
    <AntdApp>
      <ConfigProvider button={{ autoInsertSpace: false }}>
        <OpsAgentsPage agentType="synthetics" />
      </ConfigProvider>
    </AntdApp>,
  );
  await waitFor(() => expect(listOpsAgentsByType).toHaveBeenCalledWith('synthetics'));
  expect(await screen.findByText(AGENT_META.synthetics.empty)).toBeInTheDocument();
});

it('lists sidecar instances with explicit scope columns and type', async () => {
  listOpsAgentsByType.mockResolvedValue({
    nodes: [
      {
        id: 'sidecar-1',
        gameId: 'demo',
        env: 'prod',
        status: 'online',
        labels: { role: 'sidecar', hostname: 'host-a' },
        version: 'v1',
        lastSeen: '2026-10-18T00:00:00Z',
      },
    ],
  });
  render(
    <AntdApp>
      <ConfigProvider button={{ autoInsertSpace: false }}>
        <OpsAgentsPage agentType="sidecar" />
      </ConfigProvider>
    </AntdApp>,
  );
  expect(await screen.findByText('sidecar-1')).toBeInTheDocument();
  expect(screen.getAllByText(/gameId=demo env=prod/)[0]).toBeInTheDocument();
  expect(screen.getByText('demo')).toBeInTheDocument();
  expect(screen.getByText('prod')).toBeInTheDocument();
  expect(screen.getByText('sidecar')).toBeInTheDocument();
});

it('filters by agent id keyword', async () => {
  listOpsAgentsByType.mockResolvedValue({
    nodes: [
      { id: 'sidecar-1', gameId: 'demo', env: 'prod', labels: { role: 'sidecar' } },
      { id: 'sidecar-2', gameId: 'demo', env: 'prod', labels: { role: 'sidecar' } },
    ],
  });
  render(
    <AntdApp>
      <ConfigProvider button={{ autoInsertSpace: false }}>
        <OpsAgentsPage agentType="sidecar" />
      </ConfigProvider>
    </AntdApp>,
  );
  await screen.findByText('sidecar-1');
  const input = screen.getByPlaceholderText('搜索 Agent ID / 主机');
  fireEvent.change(input, { target: { value: 'sidecar-2' } });
  expect(screen.getByText('sidecar-2')).toBeInTheDocument();
  expect(screen.queryByText('sidecar-1')).not.toBeInTheDocument();
});

it('opens scoped detail and sidecar supervisor drawers', async () => {
  listOpsAgentsByType.mockResolvedValue({
    nodes: [
      {
        id: 'sidecar-1',
        gameId: 'demo',
        env: 'prod',
        labels: { role: 'sidecar' },
      },
    ],
  });
  listOpsTasks.mockResolvedValue({ tasks: [], total: 0 });
  render(
    <AntdApp>
      <ConfigProvider button={{ autoInsertSpace: false }}>
        <OpsAgentsPage agentType="sidecar" />
      </ConfigProvider>
    </AntdApp>,
  );
  await screen.findByText('sidecar-1');
  fireEvent.click(await screen.findByRole('button', { name: '详情' }));
  expect(await screen.findByText('身份 scope · gameId=demo env=prod')).toBeInTheDocument();

  fireEvent.click(screen.getByRole('button', { name: '监管' }));
  expect(await screen.findByText('supervisor:sidecar-1:demo:prod')).toBeInTheDocument();
});

it('opens detail without supervisor actions for non-sidecar types', async () => {
  listOpsAgentsByType.mockResolvedValue({
    nodes: [{ id: 'capture-1', gameId: 'demo', env: 'prod', labels: { role: 'capture' } }],
  });
  listOpsTasks.mockResolvedValue({ tasks: [], total: 0 });
  render(
    <AntdApp>
      <ConfigProvider button={{ autoInsertSpace: false }}>
        <OpsAgentsPage agentType="capture" />
      </ConfigProvider>
    </AntdApp>,
  );
  await screen.findByText('capture-1');
  fireEvent.click(await screen.findByRole('button', { name: '详情' }));
  expect(await screen.findByText('身份 scope · gameId=demo env=prod')).toBeInTheDocument();
  expect(screen.queryByRole('button', { name: '监管' })).not.toBeInTheDocument();
});

it('loads audit records for the selected agent and rejects another scope', async () => {
  listOpsAgentsByType.mockResolvedValue({
    nodes: [{ id: 'sidecar-1', gameId: 'demo', env: 'prod', labels: { role: 'sidecar' } }],
  });
  listOpsTasks.mockResolvedValue({
    tasks: [
      {
        id: 'task-1',
        functionId: 'demo.ping',
        state: 'succeeded',
        gameId: 'demo',
        env: 'prod',
        agentId: 'sidecar-1',
      },
      {
        id: 'task-stale',
        functionId: 'other.ping',
        state: 'failed',
        gameId: 'other',
        env: 'dev',
        agentId: 'sidecar-1',
      },
    ],
  });

  render(
    <AntdApp>
      <ConfigProvider button={{ autoInsertSpace: false }}>
        <OpsAgentsPage agentType="sidecar" />
      </ConfigProvider>
    </AntdApp>,
  );
  await screen.findByText('sidecar-1');
  fireEvent.click(await screen.findByRole('button', { name: '详情' }));

  expect(await screen.findByText('身份 scope · gameId=demo env=prod')).toBeInTheDocument();
  await act(async () => {
    fireEvent.click(screen.getByRole('button', { name: '查看执行记录' }));
  });

  expect(await screen.findByText('task-1')).toBeInTheDocument();
  expect(screen.queryByText('task-stale')).not.toBeInTheDocument();
  expect(screen.getByText('执行 scope · gameId=demo env=prod')).toBeInTheDocument();
  expect(listOpsTasks).toHaveBeenCalledWith({ page: 1, size: 100 });
});

it('ignores a stale response from an earlier scope', async () => {
  setScope({ gameId: 'game-a', env: 'dev' }, { persist: false, emit: false });
  let resolveFirst!: (value: unknown) => void;
  let resolveSecond!: (value: unknown) => void;
  listOpsAgentsByType.mockImplementationOnce(
    () =>
      new Promise((resolve) => {
        resolveFirst = resolve;
      }),
  );
  listOpsAgentsByType.mockImplementationOnce(
    () =>
      new Promise((resolve) => {
        resolveSecond = resolve;
      }),
  );

  listOpsTasks.mockResolvedValue({ tasks: [], total: 0 });
  render(
    <AntdApp>
      <ConfigProvider button={{ autoInsertSpace: false }}>
        <OpsAgentsPage agentType="sidecar" />
      </ConfigProvider>
    </AntdApp>,
  );
  await waitFor(() => expect(listOpsAgentsByType).toHaveBeenCalledTimes(1));

  act(() => {
    setScope({ gameId: 'game-b', env: 'prod' }, { persist: false, emit: true });
  });
  await waitFor(() => expect(listOpsAgentsByType).toHaveBeenCalledTimes(2));
  expect(screen.getAllByText(/gameId=game-b env=prod/)[0]).toBeInTheDocument();

  act(() => {
    resolveFirst({
      nodes: [{ id: 'sidecar-a', gameId: 'game-a', env: 'dev', labels: { role: 'sidecar' } }],
    });
  });
  await waitFor(() => expect(screen.queryByText('sidecar-a')).not.toBeInTheDocument());

  act(() => {
    resolveSecond({
      nodes: [{ id: 'sidecar-b', gameId: 'game-b', env: 'prod', labels: { role: 'sidecar' } }],
    });
  });
  expect(await screen.findByText('sidecar-b')).toBeInTheDocument();
});

it('closes open drawers when the scope changes', async () => {
  setScope({ gameId: 'game-a', env: 'dev' }, { persist: false, emit: false });
  listOpsAgentsByType.mockResolvedValue({
    nodes: [{ id: 'sidecar-a', gameId: 'game-a', env: 'dev', labels: { role: 'sidecar' } }],
  });
  listOpsTasks.mockResolvedValue({ tasks: [], total: 0 });

  render(
    <AntdApp>
      <ConfigProvider button={{ autoInsertSpace: false }}>
        <OpsAgentsPage agentType="sidecar" />
      </ConfigProvider>
    </AntdApp>,
  );
  await screen.findByText('sidecar-a');
  fireEvent.click(await screen.findByRole('button', { name: '详情' }));
  expect(await screen.findByText('身份 scope · gameId=game-a env=dev')).toBeInTheDocument();

  fireEvent.click(screen.getByRole('button', { name: '监管' }));
  expect(await screen.findByText('supervisor:sidecar-a:game-a:dev')).toBeInTheDocument();

  act(() => {
    setScope({ gameId: 'game-b', env: 'prod' }, { persist: false, emit: true });
  });
  await waitFor(() =>
    expect(screen.queryByText('supervisor:sidecar-a:game-a:dev')).not.toBeInTheDocument(),
  );
  expect(screen.queryByText('身份 scope · gameId=game-a env=dev')).not.toBeInTheDocument();
});

it('rejects late audit responses and clears rows when scope changes', async () => {
  setScope({ gameId: 'game-a', env: 'dev' }, { persist: false, emit: false });
  listOpsAgentsByType.mockResolvedValue({
    nodes: [{ id: 'sidecar-a', gameId: 'game-a', env: 'dev', labels: { role: 'sidecar' } }],
  });
  let resolveFirst!: (value: unknown) => void;
  let resolveSecond!: (value: unknown) => void;
  listOpsTasks.mockImplementationOnce(
    () =>
      new Promise((resolve) => {
        resolveFirst = resolve;
      }),
  );
  listOpsTasks.mockImplementationOnce(
    () =>
      new Promise((resolve) => {
        resolveSecond = resolve;
      }),
  );

  render(
    <AntdApp>
      <ConfigProvider button={{ autoInsertSpace: false }}>
        <OpsAgentsPage agentType="sidecar" />
      </ConfigProvider>
    </AntdApp>,
  );
  await screen.findByText('sidecar-a');
  fireEvent.click(screen.getByRole('button', { name: '详情' }));
  await act(async () => {
    fireEvent.click(screen.getByRole('button', { name: '查看执行记录' }));
  });
  await waitFor(() => expect(listOpsTasks).toHaveBeenCalledTimes(1));

  act(() => {
    setScope({ gameId: 'game-b', env: 'prod' }, { persist: false, emit: true });
  });
  await waitFor(() => {
    expect(listOpsTasks).toHaveBeenCalledTimes(2);
  });
  expect(screen.queryByText('task-a')).not.toBeInTheDocument();

  await act(async () => {
    resolveFirst({
      tasks: [],
    });
  });
  await act(async () => {
    resolveSecond({
      tasks: [
        {
          id: 'task-b',
          agentId: 'sidecar-a',
          addr: 'sidecar-a',
          gameId: 'game-b',
          env: 'prod',
          state: 'succeeded',
        },
      ],
    });
  });
  await screen.findByText('task-b');
  expect(listOpsTasks.mock.calls[0][0]).toEqual({ page: 1, size: 100 });
  expect(listOpsTasks.mock.calls[1][0]).toEqual({ page: 1, size: 100 });
  expect(screen.getByText('执行 scope · gameId=game-b env=prod')).toBeInTheDocument();
});

it('keeps audit-only stale rows hidden after a late scope response', async () => {
  setScope({ gameId: 'game-a', env: 'dev' }, { persist: false, emit: false });
  listOpsAgentsByType.mockResolvedValue({
    nodes: [{ id: 'sidecar-a', gameId: 'game-a', env: 'dev', labels: { role: 'sidecar' } }],
  });
  let resolveFirst!: (value: unknown) => void;
  let resolveSecond!: (value: unknown) => void;
  listOpsTasks.mockImplementationOnce(
    () =>
      new Promise((resolve) => {
        resolveFirst = resolve;
      }),
  );
  listOpsTasks.mockImplementationOnce(
    () =>
      new Promise((resolve) => {
        resolveSecond = resolve;
      }),
  );

  render(
    <AntdApp>
      <ConfigProvider button={{ autoInsertSpace: false }}>
        <OpsAgentsPage agentType="sidecar" />
      </ConfigProvider>
    </AntdApp>,
  );
  await screen.findByText('sidecar-a');
  fireEvent.click(screen.getByRole('button', { name: '详情' }));
  await act(async () => {
    fireEvent.click(screen.getByRole('button', { name: '查看执行记录' }));
  });
  await waitFor(() => expect(listOpsTasks).toHaveBeenCalledTimes(1));

  act(() => {
    setScope({ gameId: 'game-b', env: 'prod' }, { persist: false, emit: true });
  });
  await waitFor(() => expect(listOpsTasks).toHaveBeenCalledTimes(2));

  await act(async () => {
    resolveFirst({
      tasks: [
        {
          id: 'task-a',
          gameId: 'game-a',
          env: 'dev',
          addr: 'sidecar-a',
          agentId: 'sidecar-a',
          state: 'running',
        },
      ],
    });
    await Promise.resolve();
  });
  await waitFor(() => expect(screen.queryByText('task-a')).not.toBeInTheDocument());

  await act(async () => {
    resolveSecond({
      tasks: [
        {
          id: 'task-b',
          gameId: 'game-b',
          env: 'prod',
          addr: 'sidecar-a',
          agentId: 'sidecar-a',
          state: 'succeeded',
        },
      ],
    });
    await Promise.resolve();
  });
  await screen.findByText('task-b');
  expect(screen.getByText('执行 scope · gameId=game-b env=prod')).toBeInTheDocument();
});
