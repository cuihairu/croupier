/** Console/index（运行控制台首页）分支覆盖。
 *
 * 覆盖面：
 * - 权限（canConsoleRead true/false、useAccess 返回 undefined）
 * - 数据加载（成功/Error 失败/非 Error 失败/加载中/卸载后 resolve/卸载后 reject）
 * - pages 非数组回退、menu 为 null、menu.items 为空数组
 * - 分类标题解析三分支（无 title / 字符串 title / LocalizedText 精确命中
 *   与 locale 缺失回退）
 * - 根视图 / categoryKey 过滤命中 / categoryKey 无匹配 Empty
 * - 契约失效 Tag（有/无 freshness、未发布页）、空分类占位、卡片点击跳转
 */
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { history, useAccess, useParams } from '@umijs/max';
import ConsoleIndex from './index';
import { getConsoleMenu, listPublishedPages } from '@/services/console';
import type {
  BindingFreshnessDiagnostic,
  ConsoleMenuSpec,
  PublishedPageSpec,
} from '@/types/dashboard';

jest.mock('@umijs/max', () => {
  // 与 Support/Tickets Detail.test.tsx 同语义：返回 defaultMessage 并做
  // {placeholder} 插值（「契约失效 {count}」「分类 "{categoryKey}"」等断言依赖）
  const interpolate = (msg: string, values?: Record<string, unknown>): string =>
    Object.entries(values || {}).reduce(
      (acc, [key, val]) => acc.split(`{${key}}`).join(String(val)),
      msg,
    );
  return {
    useParams: jest.fn(),
    useAccess: jest.fn(),
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
  getConsoleMenu: jest.fn(),
  listPublishedPages: jest.fn(),
}));

const mockedUseParams = useParams as unknown as jest.Mock<Record<string, string> | undefined, []>;
const mockedUseAccess = useAccess as unknown as jest.Mock<Record<string, boolean> | undefined, []>;
const mockedHistoryPush = history.push as unknown as jest.Mock;
const mockedGetConsoleMenu = jest.mocked(getConsoleMenu);
const mockedListPublishedPages = jest.mocked(listPublishedPages);

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

/** menu fixture 允许运行时宽松形态（title 为 string/undefined 等），统一在此收口转换 */
const asMenu = (items: Array<Record<string, unknown>>): ConsoleMenuSpec =>
  ({ items }) as unknown as ConsoleMenuSpec;

const menuItem = (overrides: Record<string, unknown>): Record<string, unknown> => ({
  key: 'item',
  path: '/console/item',
  locale: false,
  ...overrides,
});

const publishedPage = (
  pageKey: string,
  bindingFreshness: BindingFreshnessDiagnostic[] = [],
): PublishedPageSpec => ({ pageKey, bindingFreshness }) as unknown as PublishedPageSpec;

const freshness = (bindingId: string, functionId?: string): BindingFreshnessDiagnostic => ({
  bindingId,
  functionId,
  status: 'input_schema_stale',
  diagnostic: {
    code: 'input_schema_stale',
    severity: 'error',
    message: `诊断-${bindingId}`,
  },
});

/** 主 fixture：4 个分类覆盖 getCategoryTitle 全部分支
 * - catA：LocalizedText 精确命中 zh-CN；三个子页覆盖 stale/fresh/未发布
 * - catB：title 缺失（回退 key）、children 为空数组
 * - catC：title 为字符串
 * - catD：LocalizedText 无 zh-CN（回退 localizedText → en-US） */
const mainMenu = (): ConsoleMenuSpec =>
  asMenu([
    menuItem({
      key: 'catA',
      path: '/console/catA',
      title: { 'zh-CN': '分类甲' },
      children: [
        menuItem({
          key: 'page-stale',
          path: '/console/catA/page-stale',
          title: { 'zh-CN': '玩家管理' },
        }),
        menuItem({
          key: 'page-fresh',
          path: '/console/catA/page-fresh',
          title: { 'zh-CN': '邮件发送' },
        }),
        menuItem({
          key: 'page-nopub',
          path: '/console/catA/page-nopub',
          title: { 'zh-CN': '草稿页' },
        }),
      ],
    }),
    menuItem({ key: 'catB', path: '/console/catB', children: [] }),
    menuItem({ key: 'catC', path: '/console/catC', title: '字符串分类' }),
    menuItem({ key: 'catD', path: '/console/catD', title: { 'en-US': 'Ops' } }),
  ]);

const mainPages = (): PublishedPageSpec[] => [
  publishedPage('page-stale', [freshness('binding-1', 'fn-a'), freshness('binding-2')]),
  publishedPage('page-fresh'),
];

function setAccess(value: Record<string, boolean> | undefined): void {
  mockedUseAccess.mockReturnValue(value);
}

function setParams(value: Record<string, string> | undefined): void {
  mockedUseParams.mockReturnValue(value);
}

describe('Console/index 运行控制台首页', () => {
  beforeEach(() => {
    jest.resetAllMocks();
    setAccess({ canConsoleRead: true });
    setParams(undefined);
    mockedGetConsoleMenu.mockResolvedValue(asMenu([]));
    mockedListPublishedPages.mockResolvedValue([]);
  });

  it('无权限（canConsoleRead=false）时渲染权限受限卡片', () => {
    setAccess({ canConsoleRead: false });
    setParams({});

    render(<ConsoleIndex />);

    expect(screen.getByText('权限受限')).toBeInTheDocument();
    expect(screen.getByText('你没有查看运行控制台的权限。')).toBeInTheDocument();
  });

  it('useAccess 返回 undefined 时同样视为无权限（可选链回退）', () => {
    setAccess(undefined);
    setParams({});

    render(<ConsoleIndex />);

    expect(screen.getByText('权限受限')).toBeInTheDocument();
  });

  it('加载中渲染 Spin 且不渲染列表内容', () => {
    const menu = deferred<ConsoleMenuSpec>();
    mockedGetConsoleMenu.mockReturnValue(menu.promise);
    mockedListPublishedPages.mockResolvedValue([]);
    setParams({});

    const { container } = render(<ConsoleIndex />);

    expect(container.querySelector('.ant-spin')).toBeInTheDocument();
    expect(screen.queryByText('已发布 0 个页面')).not.toBeInTheDocument();
  });

  it('根视图：分类/页面 Tag、stale 与已发布 Tag、分类标题四形态、空分类占位、卡片点击跳转', async () => {
    mockedGetConsoleMenu.mockResolvedValue(mainMenu());
    mockedListPublishedPages.mockResolvedValue(mainPages());
    setParams(undefined);

    const { container } = render(<ConsoleIndex />);

    // 概览区
    await waitFor(() => expect(screen.getByText('分类甲')).toBeInTheDocument());
    expect(screen.getByText('运行控制台')).toBeInTheDocument();
    expect(screen.getByText('已发布 2 个页面')).toBeInTheDocument();
    expect(screen.getByText('4 个菜单')).toBeInTheDocument();

    // 子页卡片：契约失效 Tag（count 插值）与已发布 Tag（fresh / 未发布两种来源）
    expect(screen.getByText('契约失效 2')).toBeInTheDocument();
    expect(screen.getAllByText('已发布').length).toBeGreaterThanOrEqual(2);

    // 分类标题：LocalizedText 精确命中 / title 缺失回退 key / 字符串 title / 无当前 locale 回退
    expect(screen.getByText('分类甲')).toBeInTheDocument();
    expect(screen.getAllByText('catB').length).toBeGreaterThan(0);
    expect(screen.getByText('字符串分类')).toBeInTheDocument();
    expect(screen.getByText('Ops')).toBeInTheDocument();

    // catB/catC/catD 均无子页：空分类占位（catB children=[]、catC/catD children 缺失）
    expect(screen.getAllByText('该菜单下暂无页面').length).toBe(3);

    // 卡片点击跳转
    fireEvent.click(screen.getByText('玩家管理'));
    expect(mockedHistoryPush).toHaveBeenCalledWith('/console/catA/page-stale');

    // 根视图不渲染「分类未匹配」Empty
    expect(screen.queryByText(/下暂无已发布页面/)).not.toBeInTheDocument();
    // 有分类数据时不渲染全量 Empty
    expect(screen.queryByText('暂无菜单')).not.toBeInTheDocument();
    expect(container.querySelector('.ant-empty')).not.toBeInTheDocument();
  });

  it('子菜单组（pagesByKey 未命中且带 children）渲染菜单组卡并跳转其 categoryKey 页', async () => {
    mockedGetConsoleMenu.mockResolvedValue(
      asMenu([
        menuItem({
          key: 'root',
          path: '/console/root',
          title: { 'zh-CN': '运营' },
          children: [
            menuItem({
              key: 'sub-audit',
              path: '/console/sub-audit',
              title: { 'zh-CN': '审计中心' },
              children: [
                menuItem({
                  key: 'page-x',
                  path: '/console/sub-audit/page-x',
                  title: { 'zh-CN': '审计页' },
                }),
              ],
            }),
          ],
        }),
      ]),
    );
    mockedListPublishedPages.mockResolvedValue([publishedPage('page-x')]);
    setParams(undefined);

    render(<ConsoleIndex />);

    // 子菜单节点渲染为菜单组卡（Tag 带子项计数），不误标「已发布」
    await waitFor(() => expect(screen.getByText('菜单组 · 1 项')).toBeInTheDocument());
    expect(screen.getByText('审计中心')).toBeInTheDocument();
    // 点击菜单组卡进入子菜单的 categoryKey 页
    fireEvent.click(screen.getByText('审计中心'));
    expect(mockedHistoryPush).toHaveBeenCalledWith('/console/sub-audit');
  });

  it('categoryKey 为子菜单 key 时递归命中（任意层级），渲染该子菜单的挂载页面', async () => {
    mockedGetConsoleMenu.mockResolvedValue(
      asMenu([
        menuItem({
          key: 'root',
          path: '/console/root',
          title: { 'zh-CN': '运营' },
          children: [
            menuItem({
              key: 'sub-deep',
              path: '/console/sub-deep',
              title: { 'zh-CN': '深层菜单' },
              children: [
                menuItem({
                  key: 'page-deep',
                  path: '/console/sub-deep/page-deep',
                  title: { 'zh-CN': '深层页面' },
                }),
              ],
            }),
          ],
        }),
      ]),
    );
    mockedListPublishedPages.mockResolvedValue([publishedPage('page-deep')]);
    setParams({ categoryKey: 'sub-deep' });

    render(<ConsoleIndex />);

    await waitFor(() => expect(screen.getByText('运行控制台 / 深层菜单')).toBeInTheDocument());
    expect(screen.getByText('深层页面')).toBeInTheDocument();
    // 根级其他菜单不出现
    expect(screen.queryByText('运营')).not.toBeInTheDocument();
  });

  it('空菜单空态：渲染「去菜单管理」引导按钮并跳转菜单管理页', async () => {
    mockedGetConsoleMenu.mockResolvedValue(asMenu([]));
    mockedListPublishedPages.mockResolvedValue([]);
    setParams(undefined);

    render(<ConsoleIndex />);

    await waitFor(() => expect(screen.getByText('暂无菜单')).toBeInTheDocument());
    const goButton = screen.getByRole('button', { name: '去菜单管理' });
    fireEvent.click(goButton);
    expect(mockedHistoryPush).toHaveBeenCalledWith('/functions/menus');
  });

  it('categoryKey 命中时只渲染该分类并按分类标题拼 pageTitle', async () => {
    mockedGetConsoleMenu.mockResolvedValue(mainMenu());
    mockedListPublishedPages.mockResolvedValue(mainPages());
    setParams({ categoryKey: 'catA' });

    render(<ConsoleIndex />);

    await waitFor(() => expect(screen.getByText('运行控制台 / 分类甲')).toBeInTheDocument());
    expect(screen.getByText('分类甲')).toBeInTheDocument();
    expect(screen.queryByText('字符串分类')).not.toBeInTheDocument();
    expect(screen.queryByText('Ops')).not.toBeInTheDocument();
  });

  it('categoryKey 无匹配时 pageTitle 回退 key 并渲染未匹配 Empty', async () => {
    mockedGetConsoleMenu.mockResolvedValue(mainMenu());
    mockedListPublishedPages.mockResolvedValue(mainPages());
    setParams({ categoryKey: 'ghost' });

    render(<ConsoleIndex />);

    await waitFor(() => expect(screen.getByText('运行控制台 / ghost')).toBeInTheDocument());
    expect(screen.getByText('菜单 "ghost" 下暂无已发布页面')).toBeInTheDocument();
    expect(screen.queryByText('分类甲')).not.toBeInTheDocument();
  });

  it('pages 返回非数组时回退空数组；menu.items 为空数组时渲染全量 Empty', async () => {
    mockedGetConsoleMenu.mockResolvedValue(asMenu([]));
    mockedListPublishedPages.mockResolvedValue(undefined as unknown as PublishedPageSpec[]);
    setParams(undefined);

    render(<ConsoleIndex />);

    await waitFor(() => expect(screen.getByText('0 个菜单')).toBeInTheDocument());
    expect(screen.getByText('已发布 0 个页面')).toBeInTheDocument();
    expect(screen.getByText('暂无菜单')).toBeInTheDocument();
    expect(screen.getByText('去菜单管理')).toBeInTheDocument();
    expect(
      screen.getByText(
        '请先在菜单管理中创建菜单并挂载已发布页面（页面工作台发布），然后在这里查看。',
      ),
    ).toBeInTheDocument();
  });

  it('menu 为 null 时不渲染分类 Tag，渲染全量 Empty', async () => {
    mockedGetConsoleMenu.mockResolvedValue(null as unknown as ConsoleMenuSpec);
    mockedListPublishedPages.mockResolvedValue([]);
    setParams(undefined);

    render(<ConsoleIndex />);

    await waitFor(() => expect(screen.getByText('暂无菜单')).toBeInTheDocument());
    expect(screen.queryByText(/个菜单/)).not.toBeInTheDocument();
  });

  it('加载失败（Error 实例）渲染错误 Alert 与错误消息', async () => {
    mockedGetConsoleMenu.mockRejectedValue(new Error('网络错误'));
    mockedListPublishedPages.mockResolvedValue([]);
    setParams(undefined);

    render(<ConsoleIndex />);

    await waitFor(() => expect(screen.getByText('加载失败')).toBeInTheDocument());
    expect(screen.getByText('网络错误')).toBeInTheDocument();
  });

  it('加载失败（非 Error）回退 intl 默认文案', async () => {
    mockedGetConsoleMenu.mockRejectedValue('服务端炸了');
    mockedListPublishedPages.mockResolvedValue([]);
    setParams(undefined);

    render(<ConsoleIndex />);

    await waitFor(() => expect(screen.getByText('加载失败')).toBeInTheDocument());
    expect(screen.getByText('加载控制台失败')).toBeInTheDocument();
  });

  it('卸载后 resolve 不触发状态更新（mounted 守卫-成功路径）', async () => {
    const menu = deferred<ConsoleMenuSpec>();
    mockedGetConsoleMenu.mockReturnValue(menu.promise);
    mockedListPublishedPages.mockResolvedValue([]);
    setParams({});

    const { unmount } = render(<ConsoleIndex />);
    unmount();

    await act(async () => {
      menu.resolve(asMenu([]));
    });
    expect(mockedGetConsoleMenu).toHaveBeenCalled();
  });

  it('卸载后 reject 不触发状态更新（mounted 守卫-失败路径）', async () => {
    const menu = deferred<ConsoleMenuSpec>();
    mockedGetConsoleMenu.mockReturnValue(menu.promise);
    mockedListPublishedPages.mockResolvedValue([]);
    setParams({});

    const { unmount } = render(<ConsoleIndex />);
    unmount();

    await act(async () => {
      menu.reject(new Error('late failure'));
    });
    expect(mockedGetConsoleMenu).toHaveBeenCalled();
  });
});
