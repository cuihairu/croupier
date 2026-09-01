import { render, waitFor } from '@testing-library/react';
import { useParams, history } from '@umijs/max';
import ConsolePage from './Page';

// canonical 重定向回归测试：pageKey 切换后（同路由组件不重挂载），
// 旧 page state 不得把新页面弹回上一个页面
// （线上 bug：打开 /console/player/resource--player 后，任何其他挂载页
//  都被 history.replace 弹回该页面）。

jest.mock('@umijs/max', () => ({
  useParams: jest.fn(),
  useIntl: () => ({ locale: 'zh-CN', formatMessage: (o: { id: string }) => o.id }),
  history: { replace: jest.fn(), push: jest.fn() },
  useModel: () => ({}),
}));

jest.mock('@/services/console', () => ({
  getPublishedPage: jest.fn(),
  executePageBinding: jest.fn(),
  queryTaskStatus: jest.fn(),
  queryApprovalStatus: jest.fn(),
  cancelTask: jest.fn(),
}));

jest.mock('@/stores/scope', () => ({
  getScope: () => ({ gameId: 'g', env: 'dev' }),
  subscribeScope: () => () => {},
}));

jest.mock('@/components/PageRenderer', () => ({
  __esModule: true,
  default: () => <div data-testid="page-renderer" />,
}));

import { getPublishedPage } from '@/services/console';

const mockedUseParams = useParams as unknown as jest.Mock;
const mockedReplace = history.replace as jest.Mock;
const mockedGet = getPublishedPage as jest.Mock;

function playerSpec() {
  return { pageKey: 'resource--player', category: { key: 'player' }, title: {} };
}

describe('ConsolePage canonical 重定向守卫', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it('分类与路由不匹配且 pageKey 一致时才重定向（canonical 语义）', async () => {
    mockedUseParams.mockReturnValue({ categoryKey: 'wrongcat', pageKey: 'resource--player' });
    mockedGet.mockResolvedValue(playerSpec());
    render(<ConsolePage />);
    await waitFor(() =>
      expect(mockedReplace).toHaveBeenCalledWith('/console/player/resource--player'),
    );
  });

  it('pageKey 切换后旧 page 不得把新页面弹回旧页（回归）', async () => {
    mockedUseParams.mockReturnValue({ categoryKey: 'player', pageKey: 'resource--player' });
    let resolveSecond: (v: unknown) => void = () => {};
    mockedGet.mockImplementation((key: string) => {
      if (key === 'resource--player') return Promise.resolve(playerSpec());
      // 新页面请求挂起：模拟慢加载窗口——旧 bug 在此窗口内就弹回。
      return new Promise((resolve) => {
        resolveSecond = resolve;
      });
    });

    const { rerender } = render(<ConsolePage />);
    await waitFor(() => expect(mockedGet).toHaveBeenCalledWith('resource--player'));

    // 用户点击另一个挂载页：params 变化，组件复用（同路由模式）。
    mockedUseParams.mockReturnValue({
      categoryKey: 'mail',
      pageKey: 'operation--mail-send',
    });
    rerender(<ConsolePage />);

    // 慢加载窗口内：不得出现指向旧页的 replace。
    await new Promise((r) => setTimeout(r, 50));
    expect(mockedReplace).not.toHaveBeenCalledWith('/console/player/resource--player');

    // 新页面数据到达（分类 mail 与路由一致）：无需重定向。
    resolveSecond({
      pageKey: 'operation--mail-send',
      category: { key: 'mail' },
      title: {},
    });
    await waitFor(() => expect(mockedGet).toHaveBeenCalledWith('operation--mail-send'));
    await new Promise((r) => setTimeout(r, 30));
    expect(mockedReplace).not.toHaveBeenCalled();
  });
});
