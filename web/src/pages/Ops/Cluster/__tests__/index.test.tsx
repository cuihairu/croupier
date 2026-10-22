/**
 * 集群拓扑页单测（覆盖率补缺轮：Ops/Cluster/index.tsx 214 行 0% → 收口，
 * 零测试簇排行现席）。
 *
 * 锁定契约：
 * - 挂载：fetchClusterInfo()（零参）→ enabled=true 形态——统计三卡
 *   （实例总数=total / 在线实例=aliveCount 且 aliveCount!==total 落红色
 *   #cf1322 翼 / Agent 连接分布=items.reduce 求和 + 后缀「个」）+
 *   实例列表六列矩阵 + 无单实例提示卡；
 * - 列渲染：实例（instanceId strong + self→blue Tag 当前实例，非 self 行
 *   无 Tag）、互联地址 `v || '-'`（空串落 '-'）、Epoch 原值、启动时间
 *   formatDateTime（合法串 toLocaleString zh-CN / 缺省 ?? '' → '-'）、
 *   Agent 连接 `v || 0`（0 原样）、状态 alive 双翼（在线 Badge success /
 *   离线 Badge error + Tooltip 租约过期）；
 * - 统计色双翼：aliveCount===total → #3f8600 绿（重渲对照）；
 * - enabled=false：单实例提示卡（cluster.enabled=false 文案）+ 统计行
 *   不渲染 + 实例列表仍渲染 items；
 * - 首拉未决（info=null 真实瞬态）：空表 + 提示卡按 `!info?.enabled`
 *   显示（undefined 翼）——`info?.items || []` 右翼同覆盖；
 * - 加载失败：reject Error → extractErrorMessage 取 e.message toast；
 *   reject 非 Error（字符串）→ fallback「加载集群信息失败」；
 * - 刷新：按钮（icon 可访问名带 aria-label 前缀）→ 第 2 次拉取；
 * - 10s 自动刷新：fake timers 推进 10_000 → interval 回调重拉（真实
 *   setInterval 10s 等不起，用 jest.advanceTimersByTime 驱动）。
 *
 * mock 口径：services/api/ops 只 mock fetchClusterInfo（类型导入编译期擦除）；
 * @umijs/max 本地 mock——本页 intl/FormattedMessage 全带 defaultMessage，
 * 取 `defaultMessage ?? id ?? ''` 使文案确定性可见；pro-components spread
 * requireActual（StatisticCard 真实渲染统计值/后缀/样式）+ PageContainer 桩；
 * antd App 真实包裹（App.useApp().message 真实例，页面默认导出不自包 App）。
 * 页面经 intlRef 转发 intl（useCallback 依赖只 [message]），mock 的 useIntl
 * 每渲染新实例也不会触发 effect 重建循环——页面注释已言明此设计。
 *
 * 坑实证（antd 6.6.0 实测，四条）：
 * - 带图标 Button 可访问名前缀拼 icon aria-label（「reload 刷新」），
 *   getByRole name 须宽松正则；
 * - Badge 状态色渲染在内部 dot（`.ant-badge-status-success` /
 *   `.ant-badge-status-error`），非外层类名（R49-4b 坑档同源）；Statistic
 *   值与后缀同在 `.ant-statistic-content`，聚合 textContent 断言「3个」；
 * - PageContainer 桩必须透传 extra——刷新按钮在 extra 槽位，只渲 children
 *   的桩会把按钮整体丢出 DOM（getByRole 恒空的根因）；
 * - jsdom 把内联 style 的 hex 色归一为 rgb() 形态（#cf1322 →
 *   "rgb(207, 19, 34)"），色值断言按归一形态写。
 *
 * 现状锁定 / 边界（诚实清单）：本页无登记不可达分支——`info?.items || []`
 * 右翼与 `!info?.enabled` 的 info-null 翼经「首拉未决瞬态」真实覆盖
 * （pending promise 渲染），`advertiseAddr || '-'`、`startedAt ?? ''`、
 * `agentCount || 0`、reduce 内 `|| 0` 各翼均类型合法真实构造（空串/
 * 缺省/0），catch 双翼经 Error/字符串 reject 触达。fake timers 用例内
 * 不用 waitFor（其轮询计时器也被 fake，会挂死），以 await act 刷微任务。
 */
import React from 'react';
import { act, configure, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { App } from 'antd';
import ClusterPage from '../index';
import { fetchClusterInfo, type ClusterInfo } from '@/services/api/ops';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

jest.mock('@/services/api/ops', () => ({
  fetchClusterInfo: jest.fn(),
}));

// 本页 intl/FormattedMessage 全带 defaultMessage——回 defaultMessage 使文案可见
jest.mock('@umijs/max', () => ({
  FormattedMessage: ({ defaultMessage }: { defaultMessage?: string }) => (
    <>{defaultMessage ?? ''}</>
  ),
  useIntl: () => ({
    formatMessage: (opts: { defaultMessage?: string }) => opts.defaultMessage ?? opts.id ?? '',
  }),
}));

// StatisticCard 依赖真实实现（统计值/后缀/styles.content 色渲染）；PageContainer
// 桩须透传 extra——刷新按钮在 extra 槽位，只渲 children 会丢按钮
jest.mock('@ant-design/pro-components', () => ({
  ...jest.requireActual('@ant-design/pro-components'),
  PageContainer: ({
    children,
    extra,
  }: {
    children?: React.ReactNode;
    extra?: React.ReactNode[];
  }) => (
    <div>
      {extra}
      {children}
    </div>
  ),
}));

const mInfo = fetchClusterInfo as jest.MockedFunction<typeof fetchClusterInfo>;

// enabled=true：total 3 / alive 2（不等 → 红翼）；agentCount 2+0+1=3
const FULL: ClusterInfo = {
  enabled: true,
  self: 'srv-a',
  total: 3,
  aliveCount: 2,
  items: [
    {
      instanceId: 'srv-a',
      advertiseAddr: '10.0.0.1:7946',
      epoch: 7,
      startedAt: '2026-09-30T00:00:00Z',
      self: true,
      alive: true,
      agentCount: 2,
    },
    {
      instanceId: 'srv-b',
      advertiseAddr: '',
      epoch: 3,
      startedAt: undefined,
      self: false,
      alive: false,
      agentCount: 0,
    },
    {
      instanceId: 'srv-c',
      advertiseAddr: '10.0.0.3:7946',
      epoch: 5,
      startedAt: '2026-09-29T08:00:00Z',
      self: false,
      alive: true,
      agentCount: 1,
    },
  ],
};

/** 与页面同参数动态计算期望值（本机 TZ=UTC，勿硬编码格式化结果） */
function fmtDateTime(s: string) {
  return new Date(s).toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  });
}

function renderPage() {
  return render(
    <App>
      <ClusterPage />
    </App>,
  );
}

function waitLoaded() {
  return waitFor(() => expect(mInfo).toHaveBeenCalledTimes(1));
}

function rowOf(id: string): HTMLElement {
  return screen.getByText(id).closest('tr') as HTMLElement;
}

beforeEach(() => {
  jest.clearAllMocks();
  mInfo.mockResolvedValue(FULL);
});

describe('集群拓扑 挂载与渲染', () => {
  it('首拉无参 + 统计三卡（求和后缀/红色不等翼）+ 六列矩阵 + 无提示卡', async () => {
    renderPage();
    await waitLoaded();

    expect(mInfo).toHaveBeenCalledWith(); // 零参调用

    // 统计三卡标题 + 值（suffix 与值同容器，聚合断言）
    const titles = Array.from(document.querySelectorAll('.ant-statistic-title')).map(
      (el) => el.textContent,
    );
    expect(titles).toEqual(['实例总数', '在线实例', 'Agent 连接分布']);
    const contents = Array.from(document.querySelectorAll('.ant-statistic-content'));
    expect(contents[0]?.textContent).toBe('3');
    expect(contents[1]?.textContent).toBe('2');
    expect(contents[2]?.textContent).toBe('3个'); // 2+0+1 求和 + 后缀「个」

    // aliveCount(2) !== total(3) → 红翼（jsdom 归一 rgb 形态）
    expect((contents[1] as HTMLElement).style.color).toBe('rgb(207, 19, 34)');

    // enabled=true → 单实例提示卡不渲染
    expect(screen.queryByText(/单实例部署/)).not.toBeInTheDocument();

    // 列头（thead 收窄）
    const thead = document.querySelector('.ant-table-thead') as HTMLElement;
    for (const h of ['实例', '互联地址', 'Epoch', '启动时间', 'Agent 连接', '状态']) {
      expect(within(thead).getByText(h)).toBeInTheDocument();
    }

    // srv-a：self → blue Tag 当前实例 + 在线 Badge + 格式化时间 + epoch
    const rowA = rowOf('srv-a');
    expect(within(rowA).getByText('当前实例').className).toContain('ant-tag-blue');
    expect(within(rowA).getByText('在线')).toBeInTheDocument();
    expect(within(rowA).getByText(fmtDateTime('2026-09-30T00:00:00Z'))).toBeInTheDocument();
    expect(within(rowA).getByText('7')).toBeInTheDocument();
    expect(within(rowA).getByText('10.0.0.1:7946')).toBeInTheDocument();

    // srv-b：空互联地址 '-' + 缺省启动时间 '-'（同行双命中）+ agentCount 0 +
    // 离线 Badge（else 翼）；非 self 行无当前实例 Tag
    const rowB = rowOf('srv-b');
    expect(within(rowB).getAllByText('-')).toHaveLength(2);
    expect(within(rowB).getByText('0')).toBeInTheDocument();
    expect(within(rowB).getByText('离线')).toBeInTheDocument();
    expect(within(rowB).queryByText('当前实例')).not.toBeInTheDocument();
    // Badge 状态色在内部 dot 类（坑档：非外层类名）
    expect(rowB.querySelector('.ant-badge-status-error')).not.toBeNull();
    expect(rowA.querySelector('.ant-badge-status-success')).not.toBeNull();

    // srv-c：第二在线行 + addr 原值
    const rowC = rowOf('srv-c');
    expect(within(rowC).getByText('10.0.0.3:7946')).toBeInTheDocument();
    expect(within(rowC).getByText('在线')).toBeInTheDocument();
    expect(within(rowC).getByText('5')).toBeInTheDocument();
  });

  it('aliveCount===total → #3f8600 绿翼（重渲对照）', async () => {
    renderPage();
    await waitLoaded();

    mInfo.mockResolvedValue({
      ...FULL,
      total: 2,
      aliveCount: 2,
      items: FULL.items.filter((it) => it.alive),
    });
    fireEvent.click(screen.getByRole('button', { name: /刷新/ }));
    await waitFor(() => expect(mInfo).toHaveBeenCalledTimes(2));
    const contents = document.querySelectorAll('.ant-statistic-content');
    await waitFor(() => expect((contents[1] as HTMLElement).style.color).toBe('rgb(63, 134, 0)'));
  });
});

describe('集群拓扑 形态与失败', () => {
  it('enabled=false：提示卡 + 无统计行 + 实例列表仍渲染', async () => {
    mInfo.mockResolvedValue({
      ...FULL,
      enabled: false,
      items: [FULL.items[0]],
      total: 1,
      aliveCount: 1,
    });
    renderPage();
    await waitLoaded();

    expect(await screen.findByText(/当前为单实例部署/)).toBeInTheDocument();
    expect(document.querySelectorAll('.ant-statistic-title')).toHaveLength(0);
    expect(await screen.findByText('srv-a')).toBeInTheDocument();
    expect(screen.getByText('当前实例')).toBeInTheDocument();
  });

  it('首拉未决（info=null 瞬态）：空表 + 提示卡按 !info?.enabled 显示', async () => {
    mInfo.mockReturnValueOnce(new Promise(() => {}) as never);
    const { unmount } = renderPage();
    await waitLoaded();

    // `info?.items || []` 右翼：无行数据 → Empty；`!info?.enabled` undefined 翼：提示卡
    expect(await screen.findByText(/单实例部署/)).toBeInTheDocument();
    expect(document.querySelector('.ant-table-tbody .ant-table-placeholder')).not.toBeNull();
    unmount();
  });

  it('加载失败：Error → e.message；非 Error 字符串 → fallback 兜底', async () => {
    mInfo.mockRejectedValueOnce(new Error('cluster down'));
    const { unmount } = renderPage();
    expect(await screen.findByText('cluster down')).toBeInTheDocument();
    unmount();

    mInfo.mockRejectedValueOnce('raw fail' as never);
    renderPage();
    expect(await screen.findByText('加载集群信息失败')).toBeInTheDocument();
  });

  it('刷新按钮 → fetchClusterInfo 第 2 次调用', async () => {
    renderPage();
    await waitLoaded();
    await screen.findByText('srv-a');

    fireEvent.click(screen.getByRole('button', { name: /刷新/ }));
    await waitFor(() => expect(mInfo).toHaveBeenCalledTimes(2));
  });

  it('10s interval 自动刷新：fake timers 推进 → 第 2 次拉取', async () => {
    jest.useFakeTimers();
    try {
      const { unmount } = renderPage();
      await act(async () => {}); // 刷首拉微任务
      expect(mInfo).toHaveBeenCalledTimes(1);

      act(() => {
        jest.advanceTimersByTime(10_000);
      });
      await act(async () => {}); // 刷 interval 回调的 load 微任务
      expect(mInfo).toHaveBeenCalledTimes(2);

      unmount(); // clearInterval（无泄漏断言面，行为由 RTL 收尾）
    } finally {
      jest.useRealTimers();
    }
  });
});
