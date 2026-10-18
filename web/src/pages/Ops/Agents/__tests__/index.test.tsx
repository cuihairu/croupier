import React from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import OpsAgentsPage, { AGENT_META } from '../index';
import { listOpsAgentsByType } from '@/services/api/ops';

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
