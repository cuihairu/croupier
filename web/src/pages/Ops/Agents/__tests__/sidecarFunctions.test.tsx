import React from 'react';
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import SidecarAgentsPage from '../sidecar';
import { listOpsAgentsByType } from '@/services/api/ops';
import { getFunctionInstances } from '@/services/api/functions-enhanced';
import type { FunctionInstance } from '@/services/api/functions-enhanced';
import { setScope } from '@/stores/scope';
import { history } from '@umijs/max';

jest.mock('@/services/api/ops', () => ({
  listOpsAgentsByType: jest.fn(),
}));
jest.mock('@/services/api/functions-enhanced', () => ({
  getFunctionInstances: jest.fn(),
}));
jest.mock('@umijs/max', () => ({
  FormattedMessage: ({ defaultMessage }: { defaultMessage?: string }) => (
    <>{defaultMessage ?? ''}</>
  ),
  useIntl: () => ({
    formatMessage: ({ defaultMessage }: { defaultMessage?: string }) => defaultMessage ?? '',
  }),
  history: { push: jest.fn() },
}));

jest.mock('../../Nodes/NodeDetailDrawer', () => ({
  __esModule: true,
  default: () => null,
}));
jest.mock('../../Nodes/SupervisorDrawer', () => ({
  __esModule: true,
  default: () => null,
}));

const { listOpsAgentsByType } = jest.requireMock('@/services/api/ops') as {
  listOpsAgentsByType: jest.Mock;
};
const { getFunctionInstances } = jest.requireMock('@/services/api/functions-enhanced') as {
  getFunctionInstances: jest.Mock;
};
const { history } = jest.requireMock('@umijs/max') as {
  history: { push: jest.Mock };
};

const instance = (over: Partial<FunctionInstance>): FunctionInstance => ({
  agentId: 'sidecar-1',
  serviceId: 'svc-a',
  addr: '10.0.0.4:9090',
  version: 'v1.2.3',
  functionId: 'player.ban',
  status: 'running',
  sdkName: 'croupier-sdk-go',
  sdkLang: 'go',
  sdkVersion: '1.2.3',
  lastSeen: '2026-10-20T00:00:00Z',
  gameId: 'demo',
  env: 'prod',
  ...over,
});

beforeEach(() => {
  listOpsAgentsByType.mockReset();
  listOpsAgentsByType.mockResolvedValue({ nodes: [] });
  getFunctionInstances.mockReset();
  history.push.mockReset();
  setScope({ gameId: 'demo', env: 'prod' }, { persist: false, emit: false });
});

it('renders registered function instances with sdk source and invoke links', async () => {
  getFunctionInstances.mockResolvedValue({
    instances: [
      instance(),
      instance({
        functionId: 'mail.send',
        agentName: 'sidecar-b',
        sdkLang: '',
        version: 'v1.3.0',
      }),
    ],
  });
  render(<SidecarAgentsPage />);

  // 实例列表（共享骨架）与函数注册区块同页渲染
  expect(await screen.findByText('函数注册')).toBeInTheDocument();
  expect(screen.getByText(/函数 scope · gameId=demo env=prod/)).toBeInTheDocument();
  expect(await screen.findByText('player.ban')).toBeInTheDocument();
  expect(screen.getByText('mail.send')).toBeInTheDocument();
  expect(screen.getByText('sidecar-b')).toBeInTheDocument();
  expect(screen.getByText('go 1.2.3')).toBeInTheDocument();
  expect(screen.getByText('v1.2.3')).toBeInTheDocument();
  expect(screen.getByText('v1.3.0')).toBeInTheDocument();
  expect(getFunctionInstances).toHaveBeenCalledWith({ gameId: 'demo', env: 'prod' });

  fireEvent.click(screen.getAllByRole('button', { name: /调\s*用/ })[0]);
  expect(history.push).toHaveBeenCalledWith('/functions/invoke?fid=player.ban');
  fireEvent.click(screen.getByText('打开函数目录'));
  expect(history.push).toHaveBeenCalledWith('/functions');
});

it('marks missing function ids and sdk source instead of guessing', async () => {
  getFunctionInstances.mockResolvedValue({
    instances: [instance({ functionId: '', sdkName: '', sdkLang: '', sdkVersion: '' })],
  });
  render(<SidecarAgentsPage />);

  await screen.findByText('sidecar-1');
  expect(screen.queryAllByRole('button', { name: /调\s*用/ })).toHaveLength(0);
  expect(screen.getAllByText('未上报').length).toBeGreaterThan(0);
});

it('shows a scope hint and requests nothing without a selected game/env', async () => {
  setScope({ gameId: '', env: '' }, { persist: false, emit: false });
  render(<SidecarAgentsPage />);

  expect(await screen.findByText('请先选择游戏与环境。')).toBeInTheDocument();
  expect(getFunctionInstances).not.toHaveBeenCalled();
});

it('ignores a stale function instance response from an earlier scope', async () => {
  setScope({ gameId: 'game-a', env: 'dev' }, { persist: false, emit: false });
  let resolveFirst!: (value: unknown) => void;
  let resolveSecond!: (value: unknown) => void;
  getFunctionInstances.mockImplementationOnce(
    () =>
      new Promise((resolve) => {
        resolveFirst = resolve;
      }),
  );
  getFunctionInstances.mockImplementationOnce(
    () =>
      new Promise((resolve) => {
        resolveSecond = resolve;
      }),
  );

  render(<SidecarAgentsPage />);
  await waitFor(() => expect(getFunctionInstances).toHaveBeenCalledTimes(1));

  act(() => {
    setScope({ gameId: 'game-b', env: 'prod' }, { persist: false, emit: true });
  });
  await waitFor(() => expect(getFunctionInstances).toHaveBeenCalledTimes(2));

  act(() => {
    resolveFirst({ instances: [instance({ functionId: 'stale-fn' })] });
  });
  await waitFor(() => expect(screen.queryByText('stale-fn')).not.toBeInTheDocument());

  act(() => {
    resolveSecond({ instances: [instance({ functionId: 'fresh-fn' })] });
  });
  expect(await screen.findByText('fresh-fn')).toBeInTheDocument();
});
