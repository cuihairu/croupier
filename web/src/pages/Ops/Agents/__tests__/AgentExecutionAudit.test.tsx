import React from 'react';
import { act, render, screen, waitFor } from '@testing-library/react';
import { App as AntdApp } from 'antd';
import AgentExecutionAudit from '../AgentExecutionAudit';
import { listOpsTasks } from '@/services/api/ops';
import { setScope } from '@/stores/scope';

jest.mock('@/services/api/ops', () => ({
  listOpsTasks: jest.fn(),
}));

const { listOpsTasks } = jest.requireMock('@/services/api/ops') as {
  listOpsTasks: jest.Mock;
};

beforeEach(() => {
  listOpsTasks.mockReset();
  setScope({ gameId: 'game-a', env: 'dev' }, { persist: false, emit: false });
});

it('rejects rows from another game or environment', async () => {
  listOpsTasks.mockResolvedValue({
    tasks: [
      {
        id: 'task-current',
        agentId: 'sidecar-a',
        gameId: 'game-a',
        env: 'dev',
        addr: 'sidecar-a',
        state: 'succeeded',
      },
      {
        id: 'task-other-game',
        agentId: 'sidecar-a',
        gameId: 'game-b',
        env: 'dev',
        addr: 'sidecar-a',
        state: 'succeeded',
      },
      {
        id: 'task-other-env',
        agentId: 'sidecar-a',
        gameId: 'game-a',
        env: 'prod',
        addr: 'sidecar-a',
        state: 'succeeded',
      },
    ],
  });

  render(
    <AntdApp>
      <AgentExecutionAudit agentId="sidecar-a" />
    </AntdApp>,
  );

  await screen.findByText('task-current');
  expect(screen.queryByText('task-other-game')).not.toBeInTheDocument();
  expect(screen.queryByText('task-other-env')).not.toBeInTheDocument();
});

it('keeps the latest request and rejects stale rows from a previous scope', async () => {
  let resolveSecond!: (value: unknown) => void;
  listOpsTasks
    .mockImplementationOnce(() => new Promise(() => undefined))
    .mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          resolveSecond = resolve;
        }),
    );

  render(
    <AntdApp>
      <AgentExecutionAudit agentId="sidecar-a" />
    </AntdApp>,
  );
  await waitFor(() => expect(listOpsTasks).toHaveBeenCalledTimes(1));
  act(() => {
    setScope({ gameId: 'game-b', env: 'prod' }, { persist: false, emit: true });
  });
  await waitFor(() => expect(listOpsTasks).toHaveBeenCalledTimes(2));

  await act(async () => {
    resolveSecond({
      tasks: [
        { id: 'task-b', gameId: 'game-b', env: 'prod', addr: 'sidecar-a', agentId: 'sidecar-a' },
      ],
    });
  });
  await screen.findByText('task-b');

  await act(async () => {
    resolveSecond({
      tasks: [
        { id: 'task-stale', gameId: 'game-a', env: 'dev', addr: 'sidecar-a', agentId: 'sidecar-b' },
      ],
    });
  });
  await waitFor(() => expect(screen.queryByText('task-stale')).not.toBeInTheDocument());
});
