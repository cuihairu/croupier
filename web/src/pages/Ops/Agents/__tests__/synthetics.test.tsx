import React from 'react';
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import SyntheticsAgentsPage from '../synthetics';
import { listOpsAgentsByType, listSyntheticsResults } from '@/services/api/ops';
import { setScope } from '@/stores/scope';
import type { SyntheticsResult } from '@/services/api/ops';

jest.mock('@/services/api/ops', () => ({
  listOpsAgentsByType: jest.fn(),
  listSyntheticsResults: jest.fn(),
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

const { listOpsAgentsByType, listSyntheticsResults } = jest.requireMock('@/services/api/ops') as {
  listOpsAgentsByType: jest.Mock;
  listSyntheticsResults: jest.Mock;
};

const row = (over: Partial<SyntheticsResult>): SyntheticsResult => ({
  id: 1,
  agentId: 'syn-1',
  gameId: 'demo',
  env: 'prod',
  kind: 'http',
  target: 'game-db',
  status: 'failed',
  latencyMs: 120,
  errorClass: 'connect',
  error: 'dial timeout',
  createdAt: 1760000000000,
  ...over,
});

beforeEach(() => {
  listOpsAgentsByType.mockReset();
  listOpsAgentsByType.mockResolvedValue({ nodes: [] });
  listSyntheticsResults.mockReset();
  setScope({ gameId: 'demo', env: 'prod' }, { persist: false, emit: false });
});

it('renders probe results under the shared agent skeleton with category tags', async () => {
  listSyntheticsResults.mockResolvedValue({
    items: [
      row({ id: 1, targetCategory: 'game', target: 'game-db' }),
      row({ id: 2, targetCategory: 'third_party', target: 'pay-api', status: 'recovered' }),
      row({ id: 3, target: 'legacy-svc', status: 'unavailable' }),
    ],
  });
  render(<SyntheticsAgentsPage />);

  // 实例列表（共享骨架）与探测结果区块同页渲染
  expect(await screen.findByText('最近探测结果')).toBeInTheDocument();
  expect(screen.getByText(/结果 scope · gameId=demo env=prod/)).toBeInTheDocument();
  expect(await screen.findByText('game-db')).toBeInTheDocument();
  expect(screen.getByText('pay-api')).toBeInTheDocument();
  expect(screen.getByText('legacy-svc')).toBeInTheDocument();

  // 类别列：game / third_party 有类别位；无类别位 → 未分类，不猜类别
  expect(screen.getAllByText('游戏').length).toBeGreaterThan(0);
  expect(screen.getAllByText('第三方服务').length).toBeGreaterThan(0);
  expect(screen.getAllByText('未分类').length).toBeGreaterThan(0);
  expect(listSyntheticsResults).toHaveBeenCalledWith('demo', 'prod', 100);
});

it('filters probe results by target category', async () => {
  listSyntheticsResults.mockResolvedValue({
    items: [
      row({ id: 1, targetCategory: 'game', target: 'game-db' }),
      row({ id: 2, targetCategory: 'third_party', target: 'pay-api' }),
      row({ id: 3, target: 'legacy-svc' }),
    ],
  });
  render(<SyntheticsAgentsPage />);
  await screen.findByText('game-db');
  expect(screen.getByText('pay-api')).toBeInTheDocument();
  expect(screen.getByText('legacy-svc')).toBeInTheDocument();

  fireEvent.click(screen.getByRole('radio', { name: '游戏' }));
  expect(screen.getByText('game-db')).toBeInTheDocument();
  expect(screen.queryByText('pay-api')).not.toBeInTheDocument();
  expect(screen.queryByText('legacy-svc')).not.toBeInTheDocument();

  fireEvent.click(screen.getByRole('radio', { name: '全部' }));
  expect(await screen.findByText('pay-api')).toBeInTheDocument();
  expect(screen.getByText('legacy-svc')).toBeInTheDocument();
});

it('shows a scope hint and requests nothing without a selected game/env', async () => {
  setScope({ gameId: '', env: '' }, { persist: false, emit: false });
  render(<SyntheticsAgentsPage />);

  expect(await screen.findByText('请先选择游戏与环境。')).toBeInTheDocument();
  expect(listSyntheticsResults).not.toHaveBeenCalled();
});

it('ignores a stale probe result response from an earlier scope', async () => {
  setScope({ gameId: 'game-a', env: 'dev' }, { persist: false, emit: false });
  let resolveFirst!: (value: unknown) => void;
  let resolveSecond!: (value: unknown) => void;
  listSyntheticsResults.mockImplementationOnce(
    () =>
      new Promise((resolve) => {
        resolveFirst = resolve;
      }),
  );
  listSyntheticsResults.mockImplementationOnce(
    () =>
      new Promise((resolve) => {
        resolveSecond = resolve;
      }),
  );

  render(<SyntheticsAgentsPage />);
  await waitFor(() => expect(listSyntheticsResults).toHaveBeenCalledTimes(1));

  act(() => {
    setScope({ gameId: 'game-b', env: 'prod' }, { persist: false, emit: true });
  });
  await waitFor(() => expect(listSyntheticsResults).toHaveBeenCalledTimes(2));
  expect(listSyntheticsResults).toHaveBeenLastCalledWith('game-b', 'prod', 100);

  act(() => {
    resolveFirst({ items: [row({ id: 11, target: 'stale-row' })] });
  });
  await waitFor(() => expect(screen.queryByText('stale-row')).not.toBeInTheDocument());

  act(() => {
    resolveSecond({ items: [row({ id: 12, target: 'fresh-row' })] });
  });
  expect(await screen.findByText('fresh-row')).toBeInTheDocument();
});
