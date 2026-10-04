/**
 * 新增/编辑游戏独立页面：表单提交载荷（icon 透传/清空回默认）、
 * 成功跳回列表 + games:changed 广播、编辑回显与标识只读。
 */
import React from 'react';
import { App as AntdApp } from 'antd';
import { configure, fireEvent, render, screen, waitFor } from '@testing-library/react';
import GameCreatePage from '../Create';
import GameEditPage from '../Edit';

configure({ asyncUtilTimeout: 5000 });
jest.setTimeout(20000);

const pushed: string[] = [];

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
    useParams: () => ({ id: '7' }),
    history: { push: jest.fn((path: string) => pushed.push(path)) },
  };
});

const mockUpsertGame = jest.fn();
const mockUpdateGame = jest.fn();
const mockGetGame = jest.fn();

jest.mock('@/services/api/games', () => ({
  __esModule: true,
  upsertGame: (...args: unknown[]) => mockUpsertGame(...args),
  updateGame: (...args: unknown[]) => mockUpdateGame(...args),
  getGame: (...args: unknown[]) => mockGetGame(...args),
}));

jest.mock('@/components/GameIcon', () => ({
  __esModule: true,
  default: () => <img data-testid="game-icon-preview" />,
  GAME_ICON_SIZE: { sm: 24, md: 32 },
}));

const renderNode = (node: React.ReactElement) => render(<AntdApp>{node}</AntdApp>);

describe('GamesManage 新增页', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    pushed.length = 0;
    mockUpsertGame.mockResolvedValue({ game: { id: 9 } });
  });

  it('提交载荷含 name/aliasName/icon，成功后广播并跳回列表', async () => {
    const changedListener = jest.fn();
    window.addEventListener('games:changed', changedListener);

    renderNode(<GameCreatePage />);

    fireEvent.change(screen.getByPlaceholderText('e.g. demo_game'), {
      target: { value: 'gamma' },
    });
    fireEvent.change(screen.getByPlaceholderText('https://...'), {
      target: { value: 'https://cdn.example.com/g.png' },
    });
    fireEvent.click(screen.getByTestId('game-create-submit'));

    await waitFor(() =>
      expect(mockUpsertGame).toHaveBeenCalledWith(
        expect.objectContaining({
          name: 'gamma',
          icon: 'https://cdn.example.com/g.png',
        }),
      ),
    );
    await waitFor(() => expect(changedListener).toHaveBeenCalled());
    expect(pushed).toContain('/system/games');
    window.removeEventListener('games:changed', changedListener);
  });

  it('标识格式非法时本地校验拦截提交', async () => {
    renderNode(<GameCreatePage />);

    fireEvent.change(screen.getByPlaceholderText('e.g. demo_game'), {
      target: { value: 'bad name!' },
    });
    fireEvent.click(screen.getByTestId('game-create-submit'));

    await waitFor(() => expect(screen.getByText('仅支持字母、数字和 _ - @')).toBeInTheDocument());
    expect(mockUpsertGame).not.toHaveBeenCalled();
  });
});

describe('GamesManage 编辑页', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    pushed.length = 0;
    mockGetGame.mockResolvedValue({
      game: { id: 7, name: 'alpha', aliasName: '阿尔法', icon: '' },
    });
    mockUpdateGame.mockResolvedValue({ game: { id: 7 } });
  });

  it('回显显示名与图标，保存载荷含 icon（清空即提交空串）', async () => {
    const changedListener = jest.fn();
    window.addEventListener('games:changed', changedListener);

    renderNode(<GameEditPage />);

    // 回显
    expect(await screen.findByDisplayValue('阿尔法')).toBeInTheDocument();
    // 标识只读
    const nameInput = screen.getByDisplayValue('alpha') as HTMLInputElement;
    expect(nameInput).toBeDisabled();

    // 改显示名 + 填图标 → 保存
    fireEvent.change(screen.getByDisplayValue('阿尔法'), {
      target: { value: '新显示名' },
    });
    fireEvent.change(screen.getByPlaceholderText('https://...'), {
      target: { value: 'https://cdn.example.com/new.png' },
    });
    fireEvent.click(screen.getByTestId('game-edit-submit'));

    await waitFor(() =>
      expect(mockUpdateGame).toHaveBeenCalledWith('7', {
        aliasName: '新显示名',
        icon: 'https://cdn.example.com/new.png',
      }),
    );
    await waitFor(() => expect(changedListener).toHaveBeenCalled());
    expect(pushed).toContain('/system/games');
    window.removeEventListener('games:changed', changedListener);
  });

  it('图标留空保存 = 提交空串清空（回落默认骰子）', async () => {
    renderNode(<GameEditPage />);

    expect(await screen.findByDisplayValue('阿尔法')).toBeInTheDocument();
    fireEvent.click(screen.getByTestId('game-edit-submit'));

    await waitFor(() =>
      expect(mockUpdateGame).toHaveBeenCalledWith('7', {
        aliasName: '阿尔法',
        icon: '',
      }),
    );
  });
});
