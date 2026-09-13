/** Console/Page（运行控制台页面渲染器）分支覆盖。
 *
 * 覆盖面：
 * - 参数缺失（params 为 undefined / 空）停留加载态、pageKey 变化重新加载
 * - 错误码解析：404（含 '404' 与仅 'not found' 两种命中）、403（'403' 与
 *   非 Error 拒绝值走 String() 命中 'forbidden'）、通用错误
 * - 404/403/错误态的「返回控制台」、错误态「重试」（window.location.reload）
 * - canonical 重定向（分类不匹配）/ pageKey 守卫（page.pageKey !== 路由 pageKey）
 * - getPublishedPage 返回 null：不渲染 PageRenderer、面包屑回退
 * - 无 category：面包屑不渲染分类项
 * - 契约失效警示：functionId 有/无两个条目、三个处置按钮（前往处理 /
 *   打开 Inbox / 一键同步 Selector 开关 modal）
 * - onExecute 透传 executePageBinding、其余回调直传
 * - scope 切换整页 reload（等值 emit 不触发）、卸载退订
 * - 卸载后 resolve / reject 的 mounted 守卫
 *
 * canonical 重定向的回归场景（pageKey 切换后旧 page 不得弹回）在
 * page-redirect.test.tsx 单独守护，此处不重复。
 */
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { history, useParams } from '@umijs/max';
import ConsolePage from './Page';
import PageRenderer from '@/components/PageRenderer';
import SelectorSyncReportModal from '@/components/SelectorSync/SelectorSyncReportModal';
import {
  cancelTask,
  executePageBinding,
  getPublishedPage,
  queryApprovalStatus,
  queryTaskStatus,
} from '@/services/console';
import { getScope, subscribeScope } from '@/stores/scope';
import type { PublishedPageSpec } from '@/types/dashboard';

jest.mock('@umijs/max', () => {
  const interpolate = (msg: string, values?: Record<string, unknown>): string =>
    Object.entries(values || {}).reduce(
      (acc, [key, val]) => acc.split(`{${key}}`).join(String(val)),
      msg,
    );
  return {
    useParams: jest.fn(),
    useIntl: () => ({
      locale: 'zh-CN',
      formatMessage: (
        { defaultMessage }: { defaultMessage: string },
        values?: Record<string, unknown>,
      ) => interpolate(defaultMessage, values),
    }),
    FormattedMessage: ({
      defaultMessage,
      values,
    }: {
      defaultMessage: string;
      values?: Record<string, unknown>;
    }) => interpolate(defaultMessage, values),
    history: { push: jest.fn(), replace: jest.fn() },
  };
});

jest.mock('@/services/console', () => ({
  getPublishedPage: jest.fn(),
  executePageBinding: jest.fn(),
  queryTaskStatus: jest.fn(),
  queryApprovalStatus: jest.fn(),
  cancelTask: jest.fn(),
}));

jest.mock('@/stores/scope', () => ({
  getScope: jest.fn(() => ({ gameId: 'game-a', env: 'dev' })),
  subscribeScope: jest.fn(),
}));

jest.mock('@/components/PageRenderer', () => ({
  __esModule: true,
  default: jest.fn(() => <div data-testid="page-renderer" />),
}));

jest.mock('@/components/SelectorSync/SelectorSyncReportModal', () => ({
  __esModule: true,
  default: jest.fn(({ open }: { open: boolean }) => (
    <div data-testid="sync-modal" data-open={String(open)} />
  )),
}));

const mockedUseParams = useParams as unknown as jest.Mock<Record<string, string> | undefined, []>;
const mockedReplace = history.replace as unknown as jest.Mock;
const mockedHistoryPush = history.push as unknown as jest.Mock;
const mockedGetPublishedPage = jest.mocked(getPublishedPage);
const mockedExecutePageBinding = jest.mocked(executePageBinding);
const mockedRenderer = jest.mocked(PageRenderer);
const mockedSyncModal = jest.mocked(SelectorSyncReportModal);
const mockedSubscribeScope = jest.mocked(subscribeScope);

function deferred<T>(): {
  promise: Promise<T>;
  resolve: (value: T) => void;
  reject: (reason: unknown) => void;
} {
  let resolve: (value: T) => void = () => {};
  let reject: (reason: unknown) => void = () => {};
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

const pageSpec = (overrides: Record<string, unknown> = {}): PublishedPageSpec =>
  ({
    pageKey: 'resource--player',
    title: { 'zh-CN': '玩家管理' },
    category: { key: 'player' },
    version: 3,
    publishedAt: '2026-09-01T00:00:00Z',
    rendererSchemaVersion: 'v1',
    bindingContracts: [],
    bindings: [],
    bindingFreshness: [],
    ...overrides,
  }) as unknown as PublishedPageSpec;

function setParams(value: Record<string, string> | undefined): void {
  mockedUseParams.mockReturnValue(value);
}

/** jsdom 的 location.reload 不可改写，调用会经 console.error 输出
 * "Not implemented: navigation"——用 spy 同时静音与计数 */
function countReloads(spy: jest.SpyInstance): number {
  return spy.mock.calls.filter((call) => String(call[0]).includes('Not implemented: navigation'))
    .length;
}

describe('Console/Page 页面渲染器', () => {
  beforeEach(() => {
    jest.resetAllMocks();
    // resetAllMocks 会剥掉模块工厂里 jest.fn(impl) 的实现，统一在此重建
    jest.mocked(getScope).mockReturnValue({ gameId: 'game-a', env: 'dev' });
    mockedRenderer.mockImplementation(() => <div data-testid="page-renderer" />);
    mockedSyncModal.mockImplementation(({ open }: { open: boolean }) => (
      <div data-testid="sync-modal" data-open={String(open)} />
    ));
    setParams(undefined);
    mockedGetPublishedPage.mockResolvedValue(pageSpec());
  });

  it('params 为 undefined 时 pageKey 为空，停留加载态且不发请求（可选链回退）', () => {
    setParams(undefined);

    const { container } = render(<ConsolePage />);

    expect(container.querySelector('.ant-spin')).toBeInTheDocument();
    expect(mockedGetPublishedPage).not.toHaveBeenCalled();
  });

  it('params 为空对象时同样停留加载态', () => {
    setParams({});

    const { container } = render(<ConsolePage />);

    expect(container.querySelector('.ant-spin')).toBeInTheDocument();
    expect(mockedGetPublishedPage).not.toHaveBeenCalled();
  });

  it('加载成功：渲染 PageRenderer、透传回调、onExecute 调 executePageBinding', async () => {
    setParams({ categoryKey: 'player', pageKey: 'resource--player' });

    render(<ConsolePage />);

    await waitFor(() => expect(screen.getByTestId('page-renderer')).toBeInTheDocument());

    expect(mockedRenderer).toHaveBeenCalledTimes(1);
    const rendererProps = mockedRenderer.mock.calls[0][0];
    expect(rendererProps.pageSpec.pageKey).toBe('resource--player');
    expect(rendererProps.onQueryStatus).toBe(queryTaskStatus);
    expect(rendererProps.onCancelTask).toBe(cancelTask);
    expect(rendererProps.onQueryApprovalStatus).toBe(queryApprovalStatus);

    // 分类一致：不做 canonical 重定向
    expect(mockedReplace).not.toHaveBeenCalled();

    // onExecute 闭包透传 pageKey + bindingId + context
    await act(async () => {
      await rendererProps.onExecute('binding-1', { form: { vipLevel: 3 } });
    });
    expect(mockedExecutePageBinding).toHaveBeenCalledWith('resource--player', 'binding-1', {
      form: { vipLevel: 3 },
    });

    // 无契约失效时不渲染警示；同步 modal 恒挂载（open=false）
    expect(screen.queryByText('页面绑定的函数契约已变化，执行已被阻断')).not.toBeInTheDocument();
    expect(screen.getByTestId('sync-modal')).toHaveAttribute('data-open', 'false');
  });

  it('契约失效警示：functionId 有/无两形态 + 三个处置动作', async () => {
    setParams({ categoryKey: 'player', pageKey: 'resource--player' });
    mockedGetPublishedPage.mockResolvedValue(
      pageSpec({
        bindingFreshness: [
          {
            bindingId: 'binding-stale-1',
            functionId: 'fn-player-query',
            status: 'input_schema_stale',
            diagnostic: {
              code: 'input_schema_stale',
              severity: 'error',
              message: '诊断一',
            },
          },
          {
            bindingId: 'binding-stale-2',
            status: 'function_missing',
            diagnostic: {
              code: 'function_missing',
              severity: 'error',
              message: '诊断二',
            },
          },
        ],
      }),
    );

    render(<ConsolePage />);

    await waitFor(() =>
      expect(screen.getByText('页面绑定的函数契约已变化，执行已被阻断')).toBeInTheDocument(),
    );
    expect(screen.getByText('binding-stale-1')).toBeInTheDocument();
    expect(screen.getByText('binding-stale-2')).toBeInTheDocument();
    expect(screen.getByText('fn-player-query')).toBeInTheDocument();
    expect(screen.getByText('诊断一')).toBeInTheDocument();
    expect(screen.getByText('诊断二')).toBeInTheDocument();
    expect(screen.getByText('input_schema_stale')).toBeInTheDocument();
    expect(screen.getByText('function_missing')).toBeInTheDocument();

    // 前往处理：focus 定位
    fireEvent.click(screen.getByText('前往处理（diff / 合并 / 重新发布）'));
    expect(mockedHistoryPush).toHaveBeenCalledWith('/functions/pages?focus=resource--player');

    // 打开 Inbox：focus + inbox=1（只定位不自动打开编辑器）
    fireEvent.click(screen.getByText('打开 Proposal Inbox'));
    expect(mockedHistoryPush).toHaveBeenCalledWith(
      '/functions/pages?focus=resource--player&inbox=1',
    );

    // 一键同步 Selector：打开报告 modal，onClose 关闭
    fireEvent.click(screen.getByText('一键同步 Selector'));
    expect(screen.getByTestId('sync-modal')).toHaveAttribute('data-open', 'true');
    const lastProps = mockedSyncModal.mock.calls[mockedSyncModal.mock.calls.length - 1][0];
    expect(lastProps.pageKey).toBe('resource--player');
    await act(async () => {
      lastProps.onClose();
    });
    expect(screen.getByTestId('sync-modal')).toHaveAttribute('data-open', 'false');
  });

  it('canonical 重定向：分类不匹配时 replace 到发布分类路径', async () => {
    setParams({ categoryKey: 'wrongcat', pageKey: 'resource--player' });

    const { container } = render(<ConsolePage />);

    await waitFor(() =>
      expect(mockedReplace).toHaveBeenCalledWith('/console/player/resource--player'),
    );
    // 重定向期间的过渡 UI：不渲染页面内容
    expect(screen.queryByTestId('page-renderer')).not.toBeInTheDocument();
    expect(container.querySelector('.ant-spin')).toBeInTheDocument();
  });

  it('canonical 守卫：page.pageKey 与路由 pageKey 不一致时不 replace', async () => {
    setParams({ categoryKey: 'player', pageKey: 'resource--player' });
    // 已加载 spec 属于另一页面且其分类与路由不符：shouldRedirect 为真，
    // 但 pageKey 不一致（新页数据未到的窗口）必须直接返回
    mockedGetPublishedPage.mockResolvedValue(
      pageSpec({ pageKey: 'other--page', category: { key: 'othercat' } }),
    );

    const { container } = render(<ConsolePage />);

    await waitFor(() => expect(container.querySelector('.ant-spin')).toBeInTheDocument());
    expect(mockedReplace).not.toHaveBeenCalled();
  });

  it('404（消息含 404）：渲染页面不存在并可返回控制台', async () => {
    setParams({ categoryKey: 'player', pageKey: 'resource--player' });
    mockedGetPublishedPage.mockRejectedValue(new Error('HTTP 404: nope'));

    render(<ConsolePage />);

    await waitFor(() => expect(screen.getAllByText('页面不存在').length).toBeGreaterThan(0));
    expect(screen.getByText('已发布页面 "resource--player" 未找到')).toBeInTheDocument();
    fireEvent.click(screen.getByText('返回控制台'));
    expect(mockedHistoryPush).toHaveBeenCalledWith('/console');
  });

  it('404（仅命中 not found 字样）：同样进入 not_found 分支', async () => {
    setParams({ pageKey: 'resource--player' });
    mockedGetPublishedPage.mockRejectedValue(new Error('page not found, sorry'));

    render(<ConsolePage />);

    await waitFor(() => expect(screen.getAllByText('页面不存在').length).toBeGreaterThan(0));
  });

  it('403（消息含 403）：渲染无权限并可返回控制台', async () => {
    setParams({ pageKey: 'resource--player' });
    mockedGetPublishedPage.mockRejectedValue(new Error('HTTP 403: denied'));

    render(<ConsolePage />);

    await waitFor(() => expect(screen.getAllByText('无访问权限').length).toBeGreaterThan(0));
    expect(screen.getByText('您没有权限访问此页面')).toBeInTheDocument();
    fireEvent.click(screen.getByText('返回控制台'));
    expect(mockedHistoryPush).toHaveBeenCalledWith('/console');
  });

  it('403（仅命中 forbidden 字样）：同样进入 forbidden 分支', async () => {
    setParams({ pageKey: 'resource--player' });
    mockedGetPublishedPage.mockRejectedValue(new Error('access forbidden, no code'));

    render(<ConsolePage />);

    await waitFor(() => expect(screen.getAllByText('无访问权限').length).toBeGreaterThan(0));
    expect(screen.getByText('您没有权限访问此页面')).toBeInTheDocument();
  });

  it('非 Error 拒绝值走 String() 渲染原始消息并落入通用错误分支', async () => {
    setParams({ pageKey: 'resource--player' });
    mockedGetPublishedPage.mockRejectedValue(500);

    render(<ConsolePage />);

    await waitFor(() => expect(screen.getByText('500')).toBeInTheDocument());
    expect(screen.getAllByText('加载失败').length).toBeGreaterThan(0);
  });

  it('通用错误：渲染重试/返回，重试触发 window.location.reload', async () => {
    const originalError = console.error;
    const errSpy = jest.spyOn(console, 'error').mockImplementation((...args: unknown[]) => {
      if (!String(args[0]).includes('Not implemented: navigation')) {
        originalError(...args);
      }
    });
    setParams({ pageKey: 'resource--player' });
    mockedGetPublishedPage.mockRejectedValue(new Error('boom'));

    render(<ConsolePage />);

    await waitFor(() => expect(screen.getByText('boom')).toBeInTheDocument());
    fireEvent.click(screen.getByText('重试'));
    expect(countReloads(errSpy)).toBe(1);
    fireEvent.click(screen.getByText('返回控制台'));
    expect(mockedHistoryPush).toHaveBeenCalledWith('/console');
    errSpy.mockRestore();
  });

  it('pageKey 变化（组件复用）触发重新加载', async () => {
    setParams({ categoryKey: 'player', pageKey: 'a--one' });
    mockedGetPublishedPage.mockResolvedValue(
      pageSpec({ pageKey: 'a--one', category: { key: 'player' } }),
    );

    const { rerender } = render(<ConsolePage />);
    await waitFor(() => expect(mockedGetPublishedPage).toHaveBeenCalledWith('a--one'));

    setParams({ categoryKey: 'mail', pageKey: 'b--two' });
    mockedGetPublishedPage.mockResolvedValue(
      pageSpec({ pageKey: 'b--two', category: { key: 'mail' } }),
    );
    rerender(<ConsolePage />);

    await waitFor(() => expect(mockedGetPublishedPage).toHaveBeenCalledWith('b--two'));
    expect(mockedReplace).not.toHaveBeenCalled();
  });

  it('getPublishedPage 返回 null：不渲染 PageRenderer，面包屑回退路由分类', async () => {
    setParams({ categoryKey: 'player', pageKey: 'resource--player' });
    mockedGetPublishedPage.mockResolvedValue(null as unknown as PublishedPageSpec);

    render(<ConsolePage />);

    await waitFor(() => expect(screen.getByTestId('sync-modal')).toBeInTheDocument());
    expect(screen.queryByTestId('page-renderer')).not.toBeInTheDocument();
    expect(mockedReplace).not.toHaveBeenCalled();
  });

  it('页面无 category 且路由无分类参数：面包屑不渲染分类项', async () => {
    setParams({ pageKey: 'bare--page' });
    mockedGetPublishedPage.mockResolvedValue(
      pageSpec({ pageKey: 'bare--page', category: undefined }),
    );

    render(<ConsolePage />);

    await waitFor(() => expect(screen.getByTestId('page-renderer')).toBeInTheDocument());
    expect(mockedReplace).not.toHaveBeenCalled();
  });

  it('scope 等值 emit 不刷新、变化触发整页 reload、卸载退订', async () => {
    const originalError = console.error;
    const errSpy = jest.spyOn(console, 'error').mockImplementation((...args: unknown[]) => {
      if (!String(args[0]).includes('Not implemented: navigation')) {
        originalError(...args);
      }
    });
    const unsub = jest.fn();
    mockedSubscribeScope.mockImplementation(() => unsub);
    setParams({ categoryKey: 'player', pageKey: 'resource--player' });

    const { unmount } = render(<ConsolePage />);
    await waitFor(() => expect(screen.getByTestId('page-renderer')).toBeInTheDocument());

    const listener = mockedSubscribeScope.mock.calls[0][0];
    expect(getScope()).toEqual({ gameId: 'game-a', env: 'dev' });

    // 等值 emit（登录流程 setScope 两次的形态）：不 reload
    await act(async () => {
      listener({ gameId: 'game-a', env: 'dev' });
    });
    expect(countReloads(errSpy)).toBe(0);

    // scope 实际变化：整页 reload
    await act(async () => {
      listener({ gameId: 'game-b', env: 'dev' });
    });
    expect(countReloads(errSpy)).toBe(1);

    unmount();
    expect(unsub).toHaveBeenCalled();
    errSpy.mockRestore();
  });

  it('卸载后 resolve 不触发状态更新（mounted 守卫-成功路径）', async () => {
    const pending = deferred<PublishedPageSpec>();
    mockedGetPublishedPage.mockReturnValue(pending.promise);
    setParams({ pageKey: 'resource--player' });

    const { unmount } = render(<ConsolePage />);
    await waitFor(() => expect(mockedGetPublishedPage).toHaveBeenCalled());
    unmount();

    await act(async () => {
      pending.resolve(pageSpec());
    });
    expect(mockedGetPublishedPage).toHaveBeenCalledTimes(1);
  });

  it('卸载后 reject 不触发状态更新（mounted 守卫-失败路径）', async () => {
    const pending = deferred<PublishedPageSpec>();
    mockedGetPublishedPage.mockReturnValue(pending.promise);
    setParams({ pageKey: 'resource--player' });

    const { unmount } = render(<ConsolePage />);
    await waitFor(() => expect(mockedGetPublishedPage).toHaveBeenCalled());
    unmount();

    await act(async () => {
      pending.reject(new Error('late failure'));
    });
    expect(mockedGetPublishedPage).toHaveBeenCalledTimes(1);
  });
});
