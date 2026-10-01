/**
 * LBMonitor 组件加载分支翼（reconciliation.test.tsx 锁 BUG-003 对账卡片，
 * 本文件锁其余加载路径分支）：
 * - 未配置 Prometheus：空态早退，LB 查询零发起，轮询/可见性重拉停止；
 * - LB 查询失败：message.error 降级，页面不白屏；
 * - nodes 拉取失败：吞错不打断 LB 图表；
 * - 不健康后端告警卡（UP=0 的 server）；
 * - 页签重新可见时触发重拉。
 */
import React from 'react';
import { act, render, screen, waitFor } from '@testing-library/react';
import { App as AntdApp } from 'antd';

// 组件内含 30s 轮询 + 多个异步加载，渲染与断言都偏慢
jest.setTimeout(60000);

const mockListOpsNodes = jest.fn();
const mockQueryLbStats = jest.fn();
const mockFetchClusterInfo = jest.fn();
const mockMessageError = jest.fn();

jest.mock('@/services/api/ops', () => ({
  listOpsNodes: (...args: unknown[]) => mockListOpsNodes(...args),
  fetchClusterInfo: (...args: unknown[]) => mockFetchClusterInfo(...args),
}));

// 全局 setup 的 useIntl mock 不做 ICU 插值；本页文案大量带 {values} 模板
//（「归属 {nodes} / LB 后端 {backends}」「不健康后端：{backends}」），按真实
// intl 语义做插值断言
jest.mock('@umijs/max', () => ({
  useIntl: () => ({
    formatMessage: (
      { defaultMessage }: { defaultMessage?: string },
      values?: Record<string, unknown>,
    ) =>
      Object.entries(values || {}).reduce(
        (msg: string, [key, val]) => msg.split(`{${key}}`).join(String(val)),
        defaultMessage,
      ),
  }),
  FormattedMessage: ({ defaultMessage }: { defaultMessage?: string }) => defaultMessage ?? '',
}));
jest.mock('@/services/api/lbStats', () => ({
  // 必须透传入参：组件按 query 内容区分「会话数」与「server_status」两次调用
  queryLbStats: (args: { query: string }) => mockQueryLbStats(args),
}));

// message 断言走 useApp 注入口。引用必须稳定：组件 load 的 useCallback 依赖
// [message]，mock 每次渲染返回新对象会让 effect 无限重建（测试 60s 挂死）。
const stableMessageApi = {
  error: mockMessageError,
  success: jest.fn(),
  warning: jest.fn(),
  info: jest.fn(),
};
jest.mock('antd', () => {
  const actual = jest.requireActual<typeof import('antd')>('antd');
  const AppSpy: typeof actual.App = Object.assign(
    (props: { children?: React.ReactNode }) => <actual.App>{props.children}</actual.App>,
    {
      useApp: () => ({ message: stableMessageApi }),
    },
  );
  return { ...actual, App: AppSpy };
});

// Line 图表依赖 canvas，jsdom 下无渲染实现；与被测分支无关，替换为占位。
jest.mock('@ant-design/charts', () => ({
  Line: () => <div data-testid="mock-line-chart" />,
}));

const nowSec = () => Math.floor(Date.now() / 1000);

beforeEach(() => {
  jest.clearAllMocks();
  mockListOpsNodes.mockResolvedValue({ nodes: [{ agentId: 'a1' }] });
  mockFetchClusterInfo.mockResolvedValue({ lbStats: { enabled: true } });
  mockQueryLbStats.mockImplementation((args: { query: string }) => {
    if (args.query.includes('haproxy_backend_current_sessions')) {
      return Promise.resolve({
        data: {
          result: [
            { metric: { proxy: 'web-1' }, value: [nowSec(), '3'] },
            { metric: { proxy: 'web-2' }, value: [nowSec(), '2'] },
          ],
        },
      });
    }
    // haproxy_server_status：per-state 指标族，UP 且值为 1 = 健康；
    // web-2 带 UP=0 行（不健康）+ DOWN 行（state 非 UP，不参与判定）
    return Promise.resolve({
      data: {
        result: [
          { metric: { proxy: 'web-1', server: 'web-1', state: 'UP' }, value: [nowSec(), '1'] },
          { metric: { proxy: 'web-2', server: 'web-2', state: 'UP' }, value: [nowSec(), '0'] },
          { metric: { proxy: 'web-2', server: 'web-2', state: 'DOWN' }, value: [nowSec(), '0'] },
        ],
      },
    });
  });
});

async function renderPage(): Promise<HTMLElement> {
  const LBMonitor = (await import('../index')).default;
  let container: HTMLElement = document.body;
  await act(async () => {
    ({ container } = render(
      <AntdApp>
        <LBMonitor />
      </AntdApp>,
    ));
  });
  return container;
}

describe('LBMonitor 加载分支翼', () => {
  it('未配置 Prometheus：空态早退，LB 查询零发起且可见性重拉停止', async () => {
    mockFetchClusterInfo.mockResolvedValue({ lbStats: { enabled: false } });
    const container = await renderPage();

    expect(
      await screen.findByText('未配置 Prometheus（ops.lbPrometheusUrl），LB 监控不可用'),
    ).toBeInTheDocument();
    await act(async () => {
      await Promise.resolve();
    });
    expect(mockQueryLbStats).not.toHaveBeenCalled();

    // disabled 后 visibilitychange 不再触发重拉（后台零请求契约）
    const callsBefore = mockFetchClusterInfo.mock.calls.length;
    await act(async () => {
      document.dispatchEvent(new Event('visibilitychange'));
      await Promise.resolve();
    });
    expect(mockFetchClusterInfo.mock.calls.length).toBe(callsBefore);
    expect(container.textContent).not.toMatch(/NaN|Infinity/);
  });

  it('LB 查询失败：message.error 提示提取到的错误消息，页面不白屏', async () => {
    mockQueryLbStats.mockRejectedValue(new Error('prometheus down'));
    const container = await renderPage();

    await waitFor(() => expect(mockMessageError).toHaveBeenCalledWith('prometheus down'));
    expect(container.textContent).not.toMatch(/NaN|Infinity/);
  });

  it('nodes 拉取失败：吞错不打断 LB 图表，归属计数回落 0', async () => {
    mockListOpsNodes.mockRejectedValue(new Error('nodes down'));
    const container = await renderPage();

    expect(await screen.findByText(/归属\s*0\s*\/\s*LB 后端\s*2/)).toBeInTheDocument();
    expect(screen.getByTestId('mock-line-chart')).toBeInTheDocument();
    expect(container.textContent).not.toMatch(/NaN|Infinity/);
  });

  it('不健康后端告警卡：UP=0 的 server 进告警文本，DOWN 行不参与判定', async () => {
    await renderPage();
    expect(await screen.findByText(/不健康后端：web-2/)).toBeInTheDocument();
    // 不健康计数染色语义：统计卡数值 1
    expect(screen.getAllByText('1').length).toBeGreaterThan(0);
  });

  it('页签重新可见时触发重拉（disabled=false 侧）', async () => {
    await renderPage();
    await screen.findByText(/归属\s*1\s*\/\s*LB 后端\s*2/);
    const callsBefore = mockFetchClusterInfo.mock.calls.length;

    await act(async () => {
      document.dispatchEvent(new Event('visibilitychange'));
      await Promise.resolve();
    });
    expect(mockFetchClusterInfo.mock.calls.length).toBeGreaterThan(callsBefore);
  });
});
