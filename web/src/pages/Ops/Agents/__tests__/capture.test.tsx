import React from 'react';
import { act, configure, render, screen, waitFor } from '@testing-library/react';
import CaptureAgentsPage from '../capture';
import { getCaptureGates } from '@/services/api/opsCapture';
import { setScope } from '@/stores/scope';

jest.mock('@/services/api/opsCapture', () => ({
  getCaptureGates: jest.fn(),
}));
jest.mock('@/services/api/ops', () => ({
  listOpsAgentsByType: jest.fn(),
  listOpsNodes: jest.fn(),
}));
jest.mock('@umijs/max', () => ({
  FormattedMessage: ({ defaultMessage }: { defaultMessage?: string }) => (
    <>{defaultMessage ?? ''}</>
  ),
  useIntl: () => ({
    formatMessage: ({ defaultMessage }: { defaultMessage?: string }) => defaultMessage ?? '',
  }),
}));

jest.mock('../../Nodes/NodeDetailDrawer', () => ({
  __esModule: true,
  default: () => null,
}));
jest.mock('../../Nodes/SupervisorDrawer', () => ({
  __esModule: true,
  default: () => null,
}));

const { listOpsAgentsByType, listOpsNodes } = jest.requireMock('@/services/api/ops') as {
  listOpsAgentsByType: jest.Mock;
  listOpsNodes: jest.Mock;
};

const node = (over: Record<string, unknown> = {}) => ({
  id: 'capture-a',
  agentId: 'capture-a',
  gameId: 'game-a',
  env: 'dev',
  status: 'online',
  labels: { role: 'capture', captureGatesRevision: '3' },
  ...over,
});

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 10000 });

beforeEach(() => {
  listOpsAgentsByType.mockReset();
  listOpsAgentsByType.mockResolvedValue({ nodes: [] });
  listOpsNodes.mockReset();
  getCaptureGates.mockReset();
  setScope({ gameId: 'game-a', env: 'dev' }, { persist: false, emit: false });
});

it('shows current gate version and per-agent reconciliation in scope', async () => {
  getCaptureGates.mockResolvedValue({
    key: 'capture.gates',
    version: 3,
    content: '{}',
    createdBy: 'admin',
    updatedAt: '2026-10-22 10:00:00',
  });
  listOpsNodes.mockResolvedValue({
    nodes: [
      node(),
      node({ id: 'capture-b', agentId: 'capture-b', labels: { role: 'capture' } }),
      node({
        id: 'other-scope',
        agentId: 'other-scope',
        gameId: 'game-b',
        env: 'prod',
        labels: { role: 'capture', captureGatesRevision: '3' },
      }),
    ],
  });
  render(<CaptureAgentsPage />);

  expect(await screen.findByText('采集策略状态')).toBeInTheDocument();
  expect(screen.getByText('当前版本 v3')).toBeInTheDocument();
  expect(screen.getByText('发布人 admin')).toBeInTheDocument();
  expect(screen.getByText('1/2 台已生效')).toBeInTheDocument();
  expect(screen.getByText('capture-a')).toBeInTheDocument();
  expect(screen.getByText('capture-b')).toBeInTheDocument();
  expect(screen.queryByText('other-scope')).not.toBeInTheDocument();
  expect(getCaptureGates).toHaveBeenCalledTimes(1);
});

it('does not request or render gate data without a complete scope', async () => {
  setScope({ gameId: '', env: '' }, { persist: false, emit: false });
  render(<CaptureAgentsPage />);

  expect(await screen.findByText('请先选择游戏与环境。')).toBeInTheDocument();
  expect(getCaptureGates).not.toHaveBeenCalled();
  expect(listOpsNodes).not.toHaveBeenCalled();
});

it('clears stale gate rows and rejects late responses from the previous scope', async () => {
  let resolveGates!: (value: unknown) => void;
  getCaptureGates.mockImplementationOnce(
    () =>
      new Promise((resolve) => {
        resolveGates = resolve;
      }),
  );
  listOpsNodes.mockResolvedValue({ nodes: [node()] });
  render(<CaptureAgentsPage />);

  act(() => {
    setScope({ gameId: 'game-b', env: 'prod' }, { persist: false, emit: true });
  });
  await waitFor(() => expect(getCaptureGates).toHaveBeenCalledTimes(2));
  expect(screen.queryByText('capture-a')).not.toBeInTheDocument();

  act(() => {
    resolveGates({ key: 'capture.gates', version: 9, content: '{}' });
  });
  await waitFor(() => expect(screen.queryByText('当前版本 v9')).not.toBeInTheDocument());
  expect(screen.getByText('策略 scope · gameId=game-b env=prod')).toBeInTheDocument();
});
