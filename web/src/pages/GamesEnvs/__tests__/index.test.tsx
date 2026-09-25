/**
 * 游戏环境页：入口提示（定位副标题、作用域 Alert、Page Studio 跳转）+
 * 数据链（games 主备回退、localStorage 命中、scope 切换、envs 失败提示）
 * + 环境 CRUD（删除确认、新增必填与成功/失败、编辑回显提交）。
 * ModalForm 用 Form 替身复刻 open/onFinish/initialValues 契约。
 */
import React from 'react';
import { App as AntdApp, Form } from 'antd';
import { configure, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { act } from 'react-dom/test-utils';
import GamesEnvsPage from '../index';
import { setScope } from '@/stores/scope';

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
  return {
    __esModule: true,
    useIntl: () => intl,
    getIntl: () => intl,
    // 工厂内联创建：jest.mock 提升早于模块体 const 声明，外部引用会 TDZ 报错
    history: { push: jest.fn() },
    FormattedMessage: ({ defaultMessage }: { defaultMessage: string }) => defaultMessage,
  };
});

const umiMock = jest.requireMock('@umijs/max') as {
  history: { push: jest.Mock };
};

// jsdom 下 PageContainer 依赖 ProLayout 的 RouteContext，mock 成透传渲染；
// ModalForm 以 antd Form 替身复刻：open 才挂载、onFinish 成功才 onOpenChange(false)
jest.mock('@ant-design/pro-components', () => ({
  __esModule: true,
  PageContainer: ({
    subTitle,
    children,
  }: {
    title?: React.ReactNode;
    subTitle?: React.ReactNode;
    children: React.ReactNode;
  }) => (
    <div>
      <p data-testid="page-subtitle">{subTitle}</p>
      {children}
    </div>
  ),
  ModalForm: ({
    open,
    title,
    children,
    onFinish,
    onOpenChange,
    initialValues,
  }: {
    open: boolean;
    title?: React.ReactNode;
    children?: React.ReactNode;
    onFinish?: (values: Record<string, unknown>) => Promise<boolean | void> | boolean | void;
    onOpenChange?: (value: boolean) => void;
    initialValues?: Record<string, unknown>;
  }) => {
    if (!open) return null;
    return (
      <Form
        layout="vertical"
        initialValues={initialValues}
        onFinish={async (values: Record<string, unknown>) => {
          const ok = onFinish ? await onFinish(values) : true;
          if (ok) onOpenChange?.(false);
        }}
      >
        <div data-testid="modal-form-title">{title}</div>
        {children}
        <button type="submit" data-testid="modal-form-submit">
          提交
        </button>
      </Form>
    );
  },
}));

const mockListGamesMeta = jest.fn();
const mockListMyGames = jest.fn();
const mockListGameEnvs = jest.fn();
const mockAddGameEnv = jest.fn();
const mockUpdateGameEnv = jest.fn();
const mockDeleteGameEnv = jest.fn();

jest.mock('@/services/api', () => ({
  __esModule: true,
  listGamesMeta: (...args: unknown[]) => mockListGamesMeta(...args),
  listMyGames: (...args: unknown[]) => mockListMyGames(...args),
}));

jest.mock('@/services/api/envs', () => ({
  __esModule: true,
  listGameEnvs: (...args: unknown[]) => mockListGameEnvs(...args),
  addGameEnv: (...args: unknown[]) => mockAddGameEnv(...args),
  updateGameEnv: (...args: unknown[]) => mockUpdateGameEnv(...args),
  deleteGameEnv: (...args: unknown[]) => mockDeleteGameEnv(...args),
}));

const GAMES = [
  { id: 1, name: 'alpha', aliasName: '甲' },
  { id: 2, name: 'beta', aliasName: '乙' },
];

function renderPage() {
  return render(
    <AntdApp>
      <GamesEnvsPage />
    </AntdApp>,
  );
}

async function renderWithGames() {
  mockListGamesMeta.mockResolvedValue({ games: GAMES });
  mockListGameEnvs.mockResolvedValue({ envs: [{ env: 'prod', description: '线上' }] });
  const utils = renderPage();
  await waitFor(() => expect(mockListGameEnvs).toHaveBeenCalledWith(1));
  return utils;
}

describe('GamesEnvs 入口提示', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    setScope({ gameId: undefined, env: undefined }, { persist: false });
    mockListGamesMeta.mockResolvedValue({ games: [] });
    mockListMyGames.mockResolvedValue({ games: [] });
    mockListGameEnvs.mockResolvedValue({ envs: [] });
  });

  it('渲染定位副标题与作用域边界 Alert', async () => {
    renderPage();

    expect(screen.getByTestId('page-subtitle')).toHaveTextContent(
      '游戏与环境是所有能力的作用域；先选定作用域，再浏览函数、资源与页面',
    );
    expect(screen.getByText('环境只是作用域，不产生页面')).toBeInTheDocument();
    expect(
      screen.getByText(
        '选定游戏/环境后，函数目录、资源目录与 Page Studio 中的数据都会按当前作用域过滤；要编排运营页面，仍需进入 Page Studio。',
      ),
    ).toBeInTheDocument();
    // 回归：既有列表卡仍渲染
    await waitFor(() => expect(screen.getByText('游戏环境')).toBeInTheDocument());
  });

  it('Alert 按钮跳转 Page Studio', () => {
    renderPage();

    fireEvent.click(screen.getByRole('button', { name: '进入 Page Studio' }));
    expect(umiMock.history.push).toHaveBeenCalledWith('/functions/pages');
  });
});

describe('GamesEnvs 数据加载链', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    setScope({ gameId: undefined, env: undefined }, { persist: false });
    mockListGameEnvs.mockResolvedValue({ envs: [] });
  });

  it('listGamesMeta 返回空列表：回退 listMyGames 取游戏', async () => {
    mockListGamesMeta.mockResolvedValue({ games: [] });
    mockListMyGames.mockResolvedValue({ games: [{ id: 9, name: 'solo' }] });
    mockListGameEnvs.mockResolvedValue({ envs: [{ env: 'dev' }] });

    renderPage();

    await waitFor(() => expect(mockListMyGames).toHaveBeenCalledTimes(1));
    await waitFor(() => expect(mockListGameEnvs).toHaveBeenCalledWith(9));
    expect(await screen.findByText('dev')).toBeInTheDocument();
  });

  it('两个游戏源都为空：无默认游戏且新增按钮禁用', async () => {
    mockListGamesMeta.mockResolvedValue({ games: [] });
    mockListMyGames.mockResolvedValue({ games: [] });

    renderPage();

    await waitFor(() => expect(screen.getByRole('button', { name: /新增环境/ })).toBeDisabled());
    expect(mockListGameEnvs).not.toHaveBeenCalled();
  });

  it('localStorage game_id 命中游戏名：默认选中该并拉取其环境', async () => {
    (global.localStorage.getItem as jest.Mock).mockReturnValue('beta');
    mockListGamesMeta.mockResolvedValue({ games: GAMES });
    mockListGameEnvs.mockResolvedValue({ envs: [{ env: 'staging' }] });

    renderPage();

    await waitFor(() => expect(mockListGameEnvs).toHaveBeenCalledWith(2));
    expect(await screen.findByText('staging')).toBeInTheDocument();
    (global.localStorage.getItem as jest.Mock).mockReturnValue(null);
  });

  it('scope 切换到另一游戏：按作用域重新拉取环境', async () => {
    await renderWithGames();
    expect(mockListGameEnvs).toHaveBeenCalledWith(1);

    act(() => {
      setScope({ gameId: '2' });
    });

    await waitFor(() => expect(mockListGameEnvs).toHaveBeenCalledWith(2));
  });

  it('listGameEnvs 失败：展示错误信息且表格不中断', async () => {
    mockListGamesMeta.mockResolvedValue({ games: [{ id: 1, name: 'alpha' }] });
    mockListGameEnvs.mockRejectedValueOnce(new Error('env boom'));

    renderPage();

    expect(await screen.findByText('env boom')).toBeInTheDocument();
    expect(screen.getByText('游戏环境')).toBeInTheDocument();
  });
});

describe('GamesEnvs 环境 CRUD', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    setScope({ gameId: undefined, env: undefined }, { persist: false });
    mockListGamesMeta.mockResolvedValue({ games: GAMES });
    mockListGameEnvs.mockResolvedValue({
      envs: [{ env: 'dev', description: '开发环境', color: 'blue' }],
    });
  });

  it('删除环境：二次确认后调用 deleteGameEnv 并刷新列表', async () => {
    mockDeleteGameEnv.mockResolvedValue(undefined);
    renderPage();
    await waitFor(() => expect(screen.getByText('dev')).toBeInTheDocument());

    fireEvent.click(screen.getByRole('button', { name: 'Delete' }));
    const okBtn = await screen.findByRole('button', { name: 'OK' });
    expect(screen.getByText('Delete env "dev"?')).toBeInTheDocument();

    fireEvent.click(okBtn);
    await waitFor(() => expect(mockDeleteGameEnv).toHaveBeenCalledWith(1, { env: 'dev' }));
    await waitFor(() => expect(mockListGameEnvs).toHaveBeenCalledTimes(2));
    expect(await screen.findByText('Deleted')).toBeInTheDocument();
  });

  it('新增环境：env 必填校验拦截空提交', async () => {
    renderPage();
    await waitFor(() => expect(screen.getByText('dev')).toBeInTheDocument());

    fireEvent.click(screen.getByRole('button', { name: /新增环境/ }));
    expect(await screen.findByTestId('modal-form-title')).toHaveTextContent('新增环境');

    fireEvent.click(screen.getByTestId('modal-form-submit'));
    expect(await screen.findByText('请输入环境名')).toBeInTheDocument();
    expect(mockAddGameEnv).not.toHaveBeenCalled();
  });

  it('新增环境成功：提交字段透传、提示成功并关闭弹窗', async () => {
    mockAddGameEnv.mockResolvedValue(undefined);
    renderPage();
    await waitFor(() => expect(screen.getByText('dev')).toBeInTheDocument());

    fireEvent.click(screen.getByRole('button', { name: /新增环境/ }));
    await screen.findByTestId('modal-form-title');
    fireEvent.change(screen.getByPlaceholderText('e.g. dev / test / stage / prod'), {
      target: { value: 'staging' },
    });
    fireEvent.change(screen.getByPlaceholderText('简单描述'), {
      target: { value: '预发环境' },
    });
    fireEvent.click(screen.getByTestId('modal-form-submit'));

    await waitFor(() =>
      expect(mockAddGameEnv).toHaveBeenCalledWith(1, 'staging', '预发环境', undefined),
    );
    expect(await screen.findByText('Added')).toBeInTheDocument();
    await waitFor(() => expect(screen.queryByTestId('modal-form-title')).not.toBeInTheDocument());
  });

  it('新增环境失败：返回 false 弹窗保持开启且无成功提示', async () => {
    mockAddGameEnv.mockRejectedValue(new Error('dup'));
    renderPage();
    await waitFor(() => expect(screen.getByText('dev')).toBeInTheDocument());

    fireEvent.click(screen.getByRole('button', { name: /新增环境/ }));
    await screen.findByTestId('modal-form-title');
    fireEvent.change(screen.getByPlaceholderText('e.g. dev / test / stage / prod'), {
      target: { value: 'staging' },
    });
    fireEvent.click(screen.getByTestId('modal-form-submit'));

    await waitFor(() => expect(mockAddGameEnv).toHaveBeenCalledTimes(1));
    // onFinish 抛错由 onAdd 捕获返回 false → 弹窗不关
    await waitFor(() => expect(screen.getByTestId('modal-form-title')).toBeInTheDocument());
    expect(screen.queryByText('Added')).not.toBeInTheDocument();
  });

  it('编辑环境：打开回显原值，改描述后提交 updateGameEnv(旧env→新值)', async () => {
    mockUpdateGameEnv.mockResolvedValue(undefined);
    renderPage();
    await waitFor(() => expect(screen.getByText('dev')).toBeInTheDocument());

    fireEvent.click(screen.getByRole('button', { name: 'Edit' }));
    const title = await screen.findByTestId('modal-form-title');
    expect(title).toHaveTextContent('编辑环境');
    expect(screen.getByDisplayValue('dev')).toBeInTheDocument();

    fireEvent.change(screen.getByDisplayValue('开发环境'), {
      target: { value: '改过的描述' },
    });
    fireEvent.click(screen.getByTestId('modal-form-submit'));

    await waitFor(() =>
      expect(mockUpdateGameEnv).toHaveBeenCalledWith(1, 'dev', 'dev', '改过的描述', 'blue'),
    );
    expect(await screen.findByText('Updated')).toBeInTheDocument();
  });
});

describe('GamesEnvs 兜底与事件分支补齐', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    setScope({ gameId: undefined, env: undefined }, { persist: false });
    mockListGamesMeta.mockResolvedValue({ games: [] });
    mockListMyGames.mockResolvedValue({ games: [] });
    mockListGameEnvs.mockResolvedValue({ envs: [] });
  });

  it('两个游戏源都缺 games 字段：各自走 || [] 兜底', async () => {
    mockListGamesMeta.mockResolvedValue({});
    mockListMyGames.mockResolvedValue({});

    renderPage();

    await waitFor(() => expect(mockListMyGames).toHaveBeenCalledTimes(1));
    await waitFor(() => expect(screen.getByRole('button', { name: /新增环境/ })).toBeDisabled());
    expect(mockListGameEnvs).not.toHaveBeenCalled();
  });

  it('首个游戏缺 id：fallback?.id 回落 prev，不拉环境', async () => {
    mockListGamesMeta.mockResolvedValue({ games: [{ name: 'ghost' }] });

    renderPage();

    await waitFor(() => expect(screen.getByRole('button', { name: /新增环境/ })).toBeDisabled());
    expect(mockListGameEnvs).not.toHaveBeenCalled();
  });

  it('Select 切换游戏 + 响应缺 envs 字段：表格清空（onChange 与 || [] 兜底）', async () => {
    mockListGamesMeta.mockResolvedValue({ games: GAMES });
    mockListGameEnvs
      .mockResolvedValueOnce({ envs: [{ env: 'dev', description: '开发环境' }] })
      .mockResolvedValueOnce({});

    renderPage();
    await waitFor(() => expect(screen.getByText('dev')).toBeInTheDocument());

    fireEvent.mouseDown(screen.getByRole('combobox'));
    fireEvent.click(await screen.findByText('beta (乙)'));

    await waitFor(() => expect(mockListGameEnvs).toHaveBeenCalledWith(2));
    await waitFor(() =>
      expect(document.querySelector('.ant-table-tbody .ant-table-row')).toBeNull(),
    );
  });

  it('游戏 Select 搜索：filterOption 按 label 过滤', async () => {
    mockListGamesMeta.mockResolvedValue({ games: GAMES });

    renderPage();
    await waitFor(() => expect(screen.getByRole('button', { name: /新增环境/ })).toBeEnabled());

    fireEvent.mouseDown(screen.getByRole('combobox'));
    await waitFor(() => {
      const dropdown = document.querySelector(
        '.ant-select-dropdown:not(.ant-select-dropdown-hidden)',
      );
      expect(dropdown).toBeTruthy();
      expect(within(dropdown as HTMLElement).getByText('alpha (甲)')).toBeInTheDocument();
    });

    fireEvent.change(screen.getByRole('combobox'), { target: { value: 'beta' } });
    await waitFor(() => {
      const dropdown = document.querySelector(
        '.ant-select-dropdown:not(.ant-select-dropdown-hidden)',
      );
      expect(within(dropdown as HTMLElement).queryByText('alpha (甲)')).toBeNull();
      expect(within(dropdown as HTMLElement).getByText('beta (乙)')).toBeInTheDocument();
    });
  });

  it('listGameEnvs 以非 Error 值拒绝：走国际化失败文案且页面不中断', async () => {
    mockListGamesMeta.mockResolvedValue({ games: [{ id: 1, name: 'alpha' }] });
    mockListGameEnvs.mockRejectedValue('plain failure');

    renderPage();

    await waitFor(() => expect(mockListGameEnvs).toHaveBeenCalledWith(1));
    await waitFor(() => expect(document.querySelector('.ant-spin-spinning')).toBeNull());
    expect(screen.getByText('游戏环境')).toBeInTheDocument();
  });

  it('listGameEnvs 以空消息 Error 拒绝：errMsg || 走默认文案', async () => {
    mockListGamesMeta.mockResolvedValue({ games: [{ id: 1, name: 'alpha' }] });
    mockListGameEnvs.mockRejectedValue(new Error(''));

    renderPage();

    await waitFor(() => expect(mockListGameEnvs).toHaveBeenCalledWith(1));
    await waitFor(() => expect(document.querySelector('.ant-spin-spinning')).toBeNull());
    expect(screen.getByText('游戏环境')).toBeInTheDocument();
  });

  it('storage 事件：有值与无值两形态都刷新作用域', async () => {
    const getItem = global.localStorage.getItem as jest.Mock;
    mockListGamesMeta.mockResolvedValue({ games: GAMES });
    mockListGameEnvs.mockResolvedValue({ envs: [] });
    renderPage();
    await waitFor(() => expect(mockListGameEnvs).toHaveBeenCalledWith(1));

    getItem.mockImplementation((key: string) => (key === 'game_id' ? 'alpha' : null));
    act(() => {
      window.dispatchEvent(new Event('storage'));
    });
    getItem.mockReturnValue(null);
    act(() => {
      window.dispatchEvent(new Event('storage'));
    });

    // 两次事件均未触发重新拉取（作用域与当前 gameId 一致 / 缺失直接跳过）
    expect(mockListGameEnvs).toHaveBeenCalledTimes(1);
  });

  it('subscribeScope 回调 scope.gameId 缺失：不重新拉取环境', async () => {
    mockListGamesMeta.mockResolvedValue({ games: GAMES });
    mockListGameEnvs.mockResolvedValue({ envs: [{ env: 'dev' }] });
    renderPage();
    await waitFor(() => expect(screen.getByText('dev')).toBeInTheDocument());

    act(() => {
      setScope({ gameId: undefined, env: undefined }, { persist: false });
    });

    await new Promise((resolve) => {
      setTimeout(resolve, 0);
    });
    expect(mockListGameEnvs).toHaveBeenCalledTimes(1);
  });

  it('编辑环境失败：onEdit catch 返回 false，弹窗保持开启', async () => {
    mockListGamesMeta.mockResolvedValue({ games: GAMES });
    mockListGameEnvs.mockResolvedValue({
      envs: [{ env: 'dev', description: '开发环境', color: 'blue' }],
    });
    mockUpdateGameEnv.mockRejectedValue(new Error('update boom'));
    renderPage();
    await waitFor(() => expect(screen.getByText('dev')).toBeInTheDocument());

    fireEvent.click(screen.getByRole('button', { name: 'Edit' }));
    await screen.findByTestId('modal-form-title');
    fireEvent.change(screen.getByDisplayValue('开发环境'), { target: { value: '改过' } });
    fireEvent.click(screen.getByTestId('modal-form-submit'));

    await waitFor(() => expect(mockUpdateGameEnv).toHaveBeenCalledTimes(1));
    await waitFor(() => expect(screen.getByTestId('modal-form-title')).toBeInTheDocument());
    expect(screen.queryByText('Updated')).not.toBeInTheDocument();
  });
});
