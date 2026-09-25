/**
 * useDirectoryPage 残余分支（hook 级 Harness，避开整页重型组件）：
 * 1. buildInvokePath 拼接与行操作「调用函数」的 history.push 接线；
 * 2. reload 的非 Error 拒绝 → 国际化「加载失败」；
 * 3. reloadFloors 拉取失败 → 静默降级（列保持旧值，不抛错）；
 * 4. headerActions「刷新」接线 → 触发 reload；
 * 5. drawerActions「调用函数」→ 跳转调用页并关闭抽屉。
 *
 * 已知不可达：handleViewDetail 外层 catch（201-208 行）——
 * `{...record}` 与其后语句不抛错，listFunctionInstances 的异常已被内层
 * try/catch 吞掉，外层 catch 在当前实现下无任何进入路径。
 */
import React from 'react';
import { App as AntdApp } from 'antd';
import { configure, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import useDirectoryPage from '../useDirectoryPage';
import { history } from '@umijs/max';
import { getFunctionSummary } from '@/services/api/functions-enhanced';
import { batchSetFunctionVersionFloor, listFunctionVersionFloors } from '@/services/api/functions';

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
  // 稳定引用：reload effect 依赖 intl，每次渲染新建对象会无限重建
  const intl = { formatMessage };
  return {
    __esModule: true,
    useIntl: () => intl,
    getIntl: () => intl,
    FormattedMessage: ({ defaultMessage }: { defaultMessage: string }) => defaultMessage,
    history: { push: jest.fn() },
  };
});

jest.mock('@/services/api', () => ({
  listDescriptors: jest.fn().mockResolvedValue({ functions: [] }),
  listFunctionInstances: jest.fn().mockResolvedValue({ instances: [] }),
}));

jest.mock('@/services/api/functions-enhanced', () => ({
  getFunctionSummary: jest.fn(),
}));

jest.mock('@/services/api/functions', () => ({
  batchSetFunctionVersionFloor: jest.fn(),
  listFunctionVersionFloors: jest.fn(),
}));

// PageSchemaRenderer 依赖图过重：目标是 onAction 路由分支而非渲染器本身
jest.mock('@/components/page-schema/PageSchemaRenderer', () => ({
  renderSchemaActions: (
    props: { onAction: (key: string) => void; flags: Record<string, boolean> },
    actions: Array<{ key: string; label: string; disabledWhen?: string[] }>,
  ) =>
    actions.map((action) => (
      <button
        key={action.key}
        type="button"
        disabled={action.disabledWhen?.some((flag) => props.flags[flag])}
        onClick={() => props.onAction(action.key)}
      >
        {action.label}
      </button>
    )),
}));

jest.mock('@/components/page-schema/icons', () => ({
  resolveSchemaIcon: () => null,
}));

const mockSummary = jest.mocked(getFunctionSummary);
const mockFloors = jest.mocked(listFunctionVersionFloors);
const mockBatch = jest.mocked(batchSetFunctionVersionFloor);
const mockPush = jest.mocked(history.push);

const summaryRow = {
  id: 'player.list',
  version: '1.0.0',
  enabled: true,
  displayName: '玩家列表',
  summary: '查询玩家',
  tags: [],
};

function Harness() {
  const page = useDirectoryPage();
  const actionsColumn = page.columns[page.columns.length - 1];
  const row = page.processedData[0];
  return (
    <div>
      <button
        type="button"
        data-testid="build-path"
        onClick={() => history.push(page.buildInvokePath('player list'))}
      >
        build
      </button>
      {page.processedData.map((item, rowIndex) => (
        <div key={rowIndex} data-testid={`row-${rowIndex}`}>
          {page.columns.map((column, index) =>
            column.render ? (
              <span key={index} data-testid="cell">
                {column.render(null, item, 0)}
              </span>
            ) : null,
          )}
        </div>
      ))}
      <div data-testid="row-actions">
        {row && actionsColumn.render ? actionsColumn.render(null, row, 0) : null}
      </div>
      <div data-testid="header-actions">{page.headerActions}</div>
      <div data-testid="drawer-actions">{page.drawerActions}</div>
      <div data-testid="detail-open">{page.detailVisible ? 'open' : 'closed'}</div>
      <button
        type="button"
        data-testid="open-detail"
        onClick={() => row && page.handleViewDetail(row)}
      >
        open
      </button>
      <button
        type="button"
        data-testid="select-row"
        onClick={() => page.setSelectedRowKeys(['player.list'])}
      >
        select
      </button>
      <button
        type="button"
        data-testid="apply-batch"
        onClick={() => void page.applyBatchFloor('1.0.0')}
      >
        batch
      </button>
    </div>
  );
}

const renderHarness = () =>
  render(
    <AntdApp>
      <Harness />
    </AntdApp>,
  );

beforeEach(() => {
  jest.clearAllMocks();
  mockSummary.mockResolvedValue([summaryRow] as never);
  mockFloors.mockResolvedValue({ 'player.list': '1.0.0' });
  mockBatch.mockResolvedValue({ updated: 1, failed: [] } as never);
});

describe('useDirectoryPage 残余分支', () => {
  it('buildInvokePath 对函数 id 做 URL 编码', async () => {
    renderHarness();
    fireEvent.click(screen.getByTestId('build-path'));
    expect(mockPush).toHaveBeenCalledWith('/functions/invoke?fid=player%20list');
  });

  it('行操作「调用函数」→ push 到调用页（buildInvokePath 接线）', async () => {
    renderHarness();
    await waitFor(() => expect(mockSummary).toHaveBeenCalled());

    const buttons = within(screen.getByTestId('row-actions')).getAllByRole('button');
    fireEvent.click(buttons[2]);
    expect(mockPush).toHaveBeenCalledWith('/functions/invoke?fid=player.list');
  });

  it('drawer「调用函数」→ push 调用页并关闭抽屉', async () => {
    renderHarness();
    await waitFor(() => expect(mockSummary).toHaveBeenCalled());

    fireEvent.click(screen.getByTestId('open-detail'));
    await waitFor(() => expect(screen.getByTestId('detail-open').textContent).toBe('open'));

    const drawerButton = within(screen.getByTestId('drawer-actions')).getByRole('button', {
      name: /调用函数/,
    });
    fireEvent.click(drawerButton);
    expect(mockPush).toHaveBeenCalledWith('/functions/invoke?fid=player.list');
    expect(screen.getByTestId('detail-open').textContent).toBe('closed');
  });

  it('header「刷新」→ 重新拉取汇总', async () => {
    renderHarness();
    await waitFor(() => expect(mockSummary).toHaveBeenCalledTimes(1));

    fireEvent.click(
      within(screen.getByTestId('header-actions')).getByRole('button', { name: /刷新/ }),
    );
    await waitFor(() => expect(mockSummary).toHaveBeenCalledTimes(2));
  });

  it('行操作「查看详情」「契约 Schema」→ onOpenDetail/onOpenSchema 接线', async () => {
    renderHarness();
    await waitFor(() => expect(mockSummary).toHaveBeenCalled());

    const buttons = within(screen.getByTestId('row-actions')).getAllByRole('button');
    fireEvent.click(buttons[0]);
    await waitFor(() => expect(screen.getByTestId('detail-open').textContent).toBe('open'));

    fireEvent.click(buttons[1]);
    expect(mockPush).toHaveBeenCalledWith('/functions/player.list?tab=config&subTab=schema');
  });

  it('未选中行时点抽屉动作 → 早退不跳转', async () => {
    renderHarness();
    await waitFor(() => expect(mockSummary).toHaveBeenCalled());

    fireEvent.click(
      within(screen.getByTestId('drawer-actions')).getByRole('button', { name: /详情页/ }),
    );
    expect(mockPush).not.toHaveBeenCalled();
  });

  it('未勾选任何行时批量设置 → 早退不发请求', async () => {
    renderHarness();
    await waitFor(() => expect(mockSummary).toHaveBeenCalled());

    fireEvent.click(screen.getByTestId('apply-batch'));
    await new Promise((resolve) => setTimeout(resolve, 40));
    expect(mockBatch).not.toHaveBeenCalled();
  });

  it('批量请求以非 Error 失败 → String(err) 文案', async () => {
    renderHarness();
    await waitFor(() => expect(mockSummary).toHaveBeenCalled());
    fireEvent.click(screen.getByTestId('select-row'));
    (mockBatch as unknown as { mockRejectedValueOnce: (v: unknown) => void }).mockRejectedValueOnce(
      'plain-batch-error',
    );

    fireEvent.click(screen.getByTestId('apply-batch'));
    expect(await screen.findByText('plain-batch-error')).toBeInTheDocument();
    expect(screen.getByTestId('detail-open').textContent).toBe('closed');
  });

  it('汇总接口非 Error 拒绝 → 提示国际化「加载失败」', async () => {
    mockSummary.mockRejectedValue('plain-string');
    renderHarness();
    expect(await screen.findByText('加载失败')).toBeInTheDocument();
  });

  it('门槛 map 缺该行 id → minVersion 回退 undefined；批量后拉取失败静默降级', async () => {
    mockSummary.mockResolvedValue([summaryRow, { ...summaryRow, id: 'player.kick' }] as never);
    renderHarness();
    await waitFor(() => expect(mockSummary).toHaveBeenCalled());

    // 第二行不在 floors map 里 → floors[r.id] || undefined 走右支
    expect(within(screen.getByTestId('row-1')).queryByText(/≥/)).toBeNull();

    // 之后门槛拉取失败 → reloadFloors 的 catch 分支：列保持旧值、不抛错
    mockFloors.mockRejectedValue(new Error('floors down'));
    fireEvent.click(screen.getByTestId('select-row'));
    fireEvent.click(screen.getByTestId('apply-batch'));

    await waitFor(() => expect(mockBatch).toHaveBeenCalledWith(['player.list'], '1.0.0'));
    await waitFor(() => expect(mockFloors).toHaveBeenCalledTimes(2));
    expect(within(screen.getByTestId('row-0')).getByText('≥ 1.0.0')).toBeInTheDocument();
    expect(screen.getByTestId('detail-open').textContent).toBe('closed');
  });
});
