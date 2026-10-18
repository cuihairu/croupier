import React from 'react';
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import OpsAgentsPage, { AGENT_META } from '../index';
import { listOpsAgentsByType } from '@/services/api/ops';
import { setScope } from '@/stores/scope';

jest.mock('@/services/api/ops', () => ({
  listOpsAgentsByType: jest.fn(),
}));
jest.mock('@umijs/max', () => ({
  FormattedMessage: ({ defaultMessage }: { defaultMessage?: string }) => (
    <>{defaultMessage ?? ''}</>
  ),
  useIntl: () => ({
    formatMessage: ({ defaultMessage }: { defaultMessage?: string }) => defaultMessage ?? '',
  }),
}));

const { listOpsAgentsByType } = jest.requireMock('@/services/api/ops') as {
  listOpsAgentsByType: jest.Mock;
};

beforeEach(() => {
  listOpsAgentsByType.mockReset();
  setScope({ gameId: 'demo', env: 'prod' }, { persist: false, emit: false });
});

jest.mock('../../Nodes/NodeDetailDrawer', () => ({
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
        <span>{`detail:${node.agentId}:${node.gameId ?? ''}:${node.env ?? ''}`}</span>
        <button type="button" onClick={onClose}>
          close-detail
        </button>
      </div>
    ) : null,
}));

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
  render(<OpsAgentsPage agentType="synthetics" />);
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
  render(<OpsAgentsPage agentType="sidecar" />);
  expect(await screen.findByText('sidecar-1')).toBeInTheDocument();
  expect(screen.getByText(/gameId=demo env=prod/)).toBeInTheDocument();
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
  render(<OpsAgentsPage agentType="sidecar" />);
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
  render(<OpsAgentsPage agentType="sidecar" />);
  await screen.findByText('sidecar-1');
  fireEvent.click(await screen.findByText('详 情', { selector: 'button > span' }));
  expect(await screen.findByText('detail:sidecar-1:demo:prod')).toBeInTheDocument();
  fireEvent.click(screen.getByRole('button', { name: 'close-detail' }));

  fireEvent.click(screen.getByText('监 管', { selector: 'button > span' }));
  expect(await screen.findByText('supervisor:sidecar-1:demo:prod')).toBeInTheDocument();
});

it('opens detail without supervisor actions for non-sidecar types', async () => {
  listOpsAgentsByType.mockResolvedValue({
    nodes: [{ id: 'capture-1', gameId: 'demo', env: 'prod', labels: { role: 'capture' } }],
  });
  render(<OpsAgentsPage agentType="capture" />);
  await screen.findByText('capture-1');
  fireEvent.click(await screen.findByText('详 情', { selector: 'button > span' }));
  expect(await screen.findByText('detail:capture-1:demo:prod')).toBeInTheDocument();
  expect(screen.queryByText('监 管', { selector: 'button > span' })).not.toBeInTheDocument();
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

  render(<OpsAgentsPage agentType="sidecar" />);
  await waitFor(() => expect(listOpsAgentsByType).toHaveBeenCalledTimes(1));

  act(() => {
    setScope({ gameId: 'game-b', env: 'prod' }, { persist: false, emit: true });
  });
  await waitFor(() => expect(listOpsAgentsByType).toHaveBeenCalledTimes(2));
  expect(screen.getByText(/gameId=game-b env=prod/)).toBeInTheDocument();

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
