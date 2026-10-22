/**
 * 游戏管理独立页面：games:write 门控（按钮级）、列表渲染（图标/标识/显示名/环境数）、
 * 删除门禁（有环境禁删 + 无环境确认删除）、新增/编辑跳页。
 */
import React from 'react';
import { App as AntdApp } from 'antd';
import { configure, fireEvent, render, screen, waitFor } from '@testing-library/react';
import GamesManagePage from '../index';

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
  const intl = { formatMessage };
  let access: Record<string, boolean> = { canGamesWrite: false };
  return {
    __esModule: true,
    useIntl: () => intl,
    FormattedMessage: ({ defaultMessage }: { defaultMessage: string }) => defaultMessage,
    useAccess: () => access,
    __setAccess: (next: Record<string, boolean>) => {
      access = next;
    },
    history: { push: jest.fn() },
  };
});

const umiMock = jest.requireMock('@umijs/max') as {
  history: { push: jest.Mock };
  __setAccess: (next: Record<string, boolean>) => void;
};

const mockListGamesMeta = jest.fn();
const mockDeleteGame = jest.fn();

jest.mock('@/services/api/games', () => ({
  __esModule: true,
  listGamesMeta: (...args: unknown[]) => mockListGamesMeta(...args),
  deleteGame: (...args: unknown[]) => mockDeleteGame(...args),
}));

jest.mock('@/components/GameIcon', () => ({
  __esModule: true,
  default: ({ name }: { name?: string }) => <img data-testid="game-icon" alt={name} />,
  GAME_ICON_SIZE: { sm: 24, md: 32 },
}));

const GAMES = [
  { id: 1, name: 'alpha', aliasName: '阿尔法', icon: '', envs: ['prod', 'dev'], status: 'dev' },
  { id: 2, name: 'beta', aliasName: undefined, icon: 'https://cdn.example.com/b.png', envs: [] },
];

const renderPage = () =>
  render(
    <AntdApp>
      <GamesManagePage />
    </AntdApp>,
  );

describe('GamesManage 游戏管理页', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockListGamesMeta.mockResolvedValue({ games: GAMES });
  });

  it('渲染列表：显示名兜底标识、环境数、默认状态', async () => {
    renderPage();

    expect(await screen.findByText('alpha')).toBeInTheDocument();
    expect(screen.getByText('阿尔法')).toBeInTheDocument();
    // beta 无显示名 → 兜底显示标识
    expect(screen.getAllByText('beta').length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText('2')).toBeInTheDocument();
    expect(screen.getAllByTestId('game-icon').length).toBe(2);
  });

  it('canGamesWrite=false：无新增按钮、无行操作', async () => {
    umiMock.__setAccess({ canGamesWrite: false });

    renderPage();

    expect(await screen.findByText('alpha')).toBeInTheDocument();
    expect(screen.queryByTestId('game-create')).not.toBeInTheDocument();
    expect(screen.queryByTestId('game-delete-1')).not.toBeInTheDocument();
  });

  it('canGamesWrite=true：新增跳页、编辑跳页', async () => {
    umiMock.__setAccess({ canGamesWrite: true });

    renderPage();

    fireEvent.click(await screen.findByTestId('game-create'));
    expect(umiMock.history.push).toHaveBeenCalledWith('/system/games/new');

    fireEvent.click(screen.getByTestId('game-edit-1'));
    expect(umiMock.history.push).toHaveBeenCalledWith('/system/games/1/edit');
  });

  it('删除门禁：有环境禁删（tooltip 提示），无环境确认后删除', async () => {
    umiMock.__setAccess({ canGamesWrite: true });
    mockDeleteGame.mockResolvedValue(undefined);

    renderPage();

    expect(await screen.findByTestId('game-delete-1')).toBeDisabled();
    expect(screen.getByTestId('game-delete-2')).toBeEnabled();

    fireEvent.click(screen.getByTestId('game-delete-2'));
    // modal.confirm 确认按钮出现在 portal 中，行内删除按钮不参与匹配
    const okButton = await screen.findByRole('button', { name: /^(OK|确\s*定)$/ });
    fireEvent.click(okButton);

    await waitFor(() => expect(mockDeleteGame).toHaveBeenCalledWith(2));
    // 删除成功后重拉列表
    await waitFor(() => expect(mockListGamesMeta).toHaveBeenCalledTimes(2));
  });
});
