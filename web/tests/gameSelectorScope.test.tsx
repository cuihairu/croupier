import React from 'react';
import { render, waitFor } from '@testing-library/react';

jest.mock('@/services/api', () => ({
  listMyGames: jest.fn(),
}));
jest.mock('@/services/api/me', () => ({
  persistMyScope: jest.fn(() => Promise.resolve()),
}));

// Mock scope store：记录 setScope / markScopeReady 的调用顺序与最终 scope 状态。
// 回归点：markScopeReady 不允许早于 env 校验（setScope 纠正陈旧 env）发生。
const mockCalls: string[] = [];
let mockScopeState: { gameId?: string; env?: string } = {};

jest.mock('@/stores/scope', () => ({
  getScope: () => ({ ...mockScopeState }),
  setScope: (next: { gameId?: string; env?: string }) => {
    mockScopeState = { ...mockScopeState, ...next };
    mockCalls.push(`setScope:${JSON.stringify(next)}`);
    return { ...mockScopeState };
  },
  markScopeReady: () => {
    mockCalls.push('markScopeReady');
  },
  isScopeReady: () => mockCalls.includes('markScopeReady'),
  subscribeScope: () => () => {},
}));

import { listMyGames } from '@/services/api';
import GameSelector from '@/components/GameSelector';

const listMyGamesMock = listMyGames as jest.Mock;

const demoGame = {
  name: 'demo-game',
  envMeta: [{ env: 'development', description: 'dev' }],
};

beforeEach(() => {
  jest.clearAllMocks();
  mockCalls.length = 0;
  mockScopeState = {};
  // GameSelector 直接读 localStorage 判断是否有 token
  (window.localStorage.getItem as jest.Mock).mockImplementation((key: string) =>
    key === 'token' ? 'test-token' : null,
  );
});

describe('GameSelector scope readiness', () => {
  it('games 加载完成之前不标记 ready', async () => {
    mockScopeState = { gameId: 'demo-game', env: 'development' };
    listMyGamesMock.mockResolvedValue({ games: [demoGame] });

    render(<GameSelector variant="header" />);

    // 挂载完成后 listMyGames 仍在 pending，此时绝不能 ready
    expect(mockCalls).not.toContain('markScopeReady');

    await waitFor(() => expect(mockCalls).toContain('markScopeReady'));
  });

  it('纠正陈旧 env 之后才标记 ready（回归：首屏请求带错 X-Env 导致空白页）', async () => {
    // localStorage 残留上一个游戏的 env=stage，而 demo-game 只有 development
    mockScopeState = { gameId: 'demo-game', env: 'stage' };
    listMyGamesMock.mockResolvedValue({ games: [demoGame] });

    render(<GameSelector variant="header" />);

    await waitFor(() => expect(mockCalls).toContain('markScopeReady'));

    const envFixIndex = mockCalls.indexOf('setScope:{"env":"development"}');
    const readyIndex = mockCalls.indexOf('markScopeReady');
    expect(envFixIndex).toBeGreaterThanOrEqual(0);
    expect(envFixIndex).toBeLessThan(readyIndex);
    expect(mockScopeState).toEqual({ gameId: 'demo-game', env: 'development' });
  });

  it('合法 scope 不触发纠正，直接标记 ready', async () => {
    mockScopeState = { gameId: 'demo-game', env: 'development' };
    listMyGamesMock.mockResolvedValue({ games: [demoGame] });

    render(<GameSelector variant="header" />);

    await waitFor(() => expect(mockCalls).toContain('markScopeReady'));
    expect(mockCalls.filter((call) => call.startsWith('setScope'))).toEqual([]);
    expect(mockScopeState).toEqual({ gameId: 'demo-game', env: 'development' });
  });

  it('无效 gameId 回退到 games[0]，且 gameId/env 纠正都早于 ready', async () => {
    mockScopeState = { gameId: 'removed-game', env: 'stage' };
    listMyGamesMock.mockResolvedValue({ games: [demoGame] });

    render(<GameSelector variant="header" />);

    await waitFor(() => expect(mockCalls).toContain('markScopeReady'));

    const readyIndex = mockCalls.indexOf('markScopeReady');
    expect(mockCalls.indexOf('setScope:{"gameId":"demo-game"}')).toBeLessThan(readyIndex);
    expect(mockCalls.indexOf('setScope:{"env":"development"}')).toBeLessThan(readyIndex);
    expect(mockScopeState).toEqual({ gameId: 'demo-game', env: 'development' });
  });

  it('games 为空时同样标记 ready，避免页面永久空白', async () => {
    listMyGamesMock.mockResolvedValue({ games: [] });

    render(<GameSelector variant="header" />);

    await waitFor(() => expect(mockCalls).toContain('markScopeReady'));
  });

  it('games 加载失败时标记 ready，避免页面永久空白', async () => {
    listMyGamesMock.mockRejectedValue(new Error('network down'));

    render(<GameSelector variant="header" />);

    await waitFor(() => expect(mockCalls).toContain('markScopeReady'));
  });
});
