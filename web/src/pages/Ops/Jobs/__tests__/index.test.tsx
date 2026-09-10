/**
 * Ops/Jobs 任务监控页回归测试
 *
 * 重点回归（本轮修复点）：
 * a. 概览「任务 N」必须用全量 total（分页口径），而非当前页 rows.length；
 *    运行中/成功/失败/函数为当前页口径并带「（当前页）」标注。
 * b. 取消任务必须经 modal.confirm 二次确认：表格行「取消」与详情抽屉两处入口，
 *    点确认（取消任务）才调 cancelTask，点「返回」不调。
 *
 * 同时覆盖：筛选联动（状态/函数/操作者/清空）、分页、刷新、详情抽屉、
 * 事件流订阅（自动订阅/手动连接/断开/onEvent/onError/onDone）、刷新结果等路径。
 */
import React from 'react';
import { act, configure, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { App, ConfigProvider } from 'antd';
import OpsTasksPage from '../index';
import type { OpsTask } from '@/services/api/ops';
import type { TaskEventHandlers } from '@/services/api/functions';
import type { ReactNode } from 'react';

// antd Table/Drawer/Modal 在 jsdom 下渲染较重，放宽异步等待与单测超时
jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

jest.mock('@/services/api/ops', () => ({
  listOpsTasks: jest.fn(),
  listOpsFunctions: jest.fn(),
}));

jest.mock('@/services/api/functions', () => ({
  cancelTask: jest.fn(),
  fetchTaskResult: jest.fn(),
  subscribeTaskEvents: jest.fn(),
}));

jest.mock('@ant-design/pro-components', () => ({
  PageContainer: (props: { children?: ReactNode }) => <div>{props.children}</div>,
}));

jest.mock('@/components', () => ({
  SummaryOverview: (props: {
    title?: ReactNode;
    items?: { text?: ReactNode; color?: string }[];
    hint?: ReactNode;
  }) => (
    <div data-testid="summary-overview">
      <h2>{props.title}</h2>
      {(props.items || []).map((item, idx) => (
        <span key={idx} className="summary-item">
          {item.text}
        </span>
      ))}
      {props.hint ? <span className="summary-hint">{props.hint}</span> : null}
    </div>
  ),
  StandardListSection: (props: { title?: ReactNode; extra?: ReactNode; children?: ReactNode }) => (
    <section data-testid="list-section">
      <h3>{props.title}</h3>
      {props.extra}
      {props.children}
    </section>
  ),
  StandardFilterBar: (props: { resultText?: ReactNode; controls?: ReactNode }) => (
    <div data-testid="filter-bar">
      <span className="filter-result">{props.resultText}</span>
      {props.controls}
    </div>
  ),
}));

const { listOpsTasks, listOpsFunctions } = jest.requireMock('@/services/api/ops') as {
  listOpsTasks: jest.Mock;
  listOpsFunctions: jest.Mock;
};
const { cancelTask, fetchTaskResult, subscribeTaskEvents } = jest.requireMock(
  '@/services/api/functions',
) as {
  cancelTask: jest.Mock;
  fetchTaskResult: jest.Mock;
  subscribeTaskEvents: jest.Mock;
};

const TASKS: OpsTask[] = [
  {
    id: 'task-run-1',
    functionId: 'fn-alpha',
    actor: 'alice',
    gameId: 'game-x',
    env: 'prod',
    state: 'running',
    startedAt: '2026-09-01T10:00:00Z',
    durationMs: 1500,
    addr: '10.0.0.1:9000',
    traceId: 'trace-1',
  },
  {
    id: 'task-ok-2',
    functionId: 'fn-beta',
    actor: 'bob',
    gameId: 'game-x',
    env: 'prod',
    state: 'succeeded',
    startedAt: '2026-09-01T11:00:00Z',
    endedAt: '2026-09-01T11:00:05Z',
    durationMs: 2500,
    addr: '10.0.0.2:9000',
  },
  {
    id: 'task-fail-3',
    functionId: 'fn-alpha',
    actor: 'carol',
    state: 'failed',
    error: 'boom failure',
    durationMs: 0,
  },
  { id: 'task-cancel-4', functionId: 'fn-beta', actor: 'dave', state: 'canceled' },
  { id: 'task-queued-5', functionId: 'fn-gamma', actor: 'erin', state: 'queued' },
  { id: 'task-blank-6', functionId: '', state: '' },
];

const renderPage = () =>
  render(
    <App>
      <ConfigProvider button={{ autoInsertSpace: false }}>
        <OpsTasksPage />
      </ConfigProvider>
    </App>,
  );

const awaitInitialLoad = async () => {
  await waitFor(() => expect(screen.getByText('共 57 条')).toBeInTheDocument());
};

/** 找到包含指定文本的表格行 */
const findRow = (text: string) => {
  const row = Array.from(document.querySelectorAll('.ant-table-row')).find((r) =>
    r.textContent?.includes(text),
  );
  if (!row) throw new Error(`table row containing ${text} not found`);
  return row as HTMLElement;
};

/** 详情抽屉内容容器（antd6 为 .ant-drawer-section） */
const drawerPanel = async () => {
  const panel = await waitFor(() => {
    const node = document.querySelector('.ant-drawer-section');
    expect(node).toBeTruthy();
    return node as HTMLElement;
  });
  return panel;
};

/** 最近一次 subscribeTaskEvents 的 handlers（页面以 (taskId, handlers) 调用） */
const lastSubscriptionHandlers = (): TaskEventHandlers => {
  const calls = subscribeTaskEvents.mock.calls as unknown as [string, TaskEventHandlers][];
  const last = calls[calls.length - 1];
  if (!last) throw new Error('subscribeTaskEvents not called yet');
  return last[1];
};

beforeEach(() => {
  listOpsTasks.mockReset();
  listOpsFunctions.mockReset();
  cancelTask.mockReset();
  fetchTaskResult.mockReset();
  subscribeTaskEvents.mockReset();
  listOpsTasks.mockResolvedValue({ tasks: TASKS.map((t) => ({ ...t })), total: 57 });
  listOpsFunctions.mockResolvedValue({
    functions: [{ id: 'fn-alpha' }, { id: 'fn-beta' }],
  });
  cancelTask.mockResolvedValue(undefined);
  fetchTaskResult.mockResolvedValue({ state: 'succeeded', payload: { ok: true } });
  subscribeTaskEvents.mockImplementation(() => ({ close: jest.fn() }));
});

describe('Ops/Jobs 任务监控页', () => {
  it('初始加载：概览「任务 N」用全量 total，四项指标为当前页口径', async () => {
    renderPage();
    await awaitInitialLoad();

    // 全量 57，而不是当前页行数 6
    expect(screen.getByText('任务 57')).toBeInTheDocument();
    expect(screen.queryByText('任务 6')).toBeNull();
    // 当前页口径 + 标注
    expect(screen.getByText('运行中 1（当前页）')).toBeInTheDocument();
    expect(screen.getByText('成功 1（当前页）')).toBeInTheDocument();
    expect(screen.getByText('失败 1（当前页）')).toBeInTheDocument();
    expect(screen.getByText('函数 3（当前页）')).toBeInTheDocument();

    // 首屏请求参数：仅 page/size
    await waitFor(() => expect(listOpsTasks).toHaveBeenCalledWith({ page: 1, size: 10 }));
    expect(listOpsFunctions).toHaveBeenCalledTimes(1);

    // 表格行渲染：状态 Tag（含未知状态与空状态）、耗时格式化、游戏/环境
    expect(screen.getByText('共 57 条')).toBeInTheDocument();
    expect(within(findRow('task-run-1')).getByText('运行中')).toBeInTheDocument();
    expect(within(findRow('task-ok-2')).getByText('已成功')).toBeInTheDocument();
    expect(within(findRow('task-fail-3')).getByText('已失败')).toBeInTheDocument();
    expect(within(findRow('task-cancel-4')).getByText('已取消')).toBeInTheDocument();
    expect(within(findRow('task-queued-5')).getByText('queued')).toBeInTheDocument();
    expect(
      within(findRow('task-blank-6')).getByText('-', { selector: '.ant-tag' }),
    ).toBeInTheDocument();
    expect(within(findRow('task-run-1')).getByText('game-x/prod')).toBeInTheDocument();
    expect(within(findRow('task-run-1')).getByText('1.50s')).toBeInTheDocument();
    expect(within(findRow('task-ok-2')).getByText('2.50s')).toBeInTheDocument();
    expect(within(findRow('task-ok-2')).getByText('10.0.0.2:9000')).toBeInTheDocument();
    // 非运行行不渲染取消按钮
    expect(within(findRow('task-ok-2')).queryByRole('button', { name: '取消' })).toBeNull();
    expect(within(findRow('task-run-1')).getByRole('button', { name: '取消' })).toBeTruthy();
  });

  it('total 缺失时回退为当前页行数', async () => {
    listOpsTasks.mockResolvedValue({ tasks: [TASKS[0]] });
    renderPage();
    await waitFor(() => expect(screen.getByText('任务 1')).toBeInTheDocument());
    expect(screen.getByText('当前结果 1 个任务')).toBeInTheDocument();
  });

  it('任务列表加载失败：提示错误信息', async () => {
    listOpsTasks.mockRejectedValue(new Error('服务不可用'));
    renderPage();
    expect(await screen.findByText('服务不可用')).toBeInTheDocument();
  });

  it('函数列表加载失败：静默不阻塞页面', async () => {
    listOpsFunctions.mockRejectedValue(new Error('fn error'));
    renderPage();
    await awaitInitialLoad();
    expect(screen.getByText('任务 57')).toBeInTheDocument();
  });

  it('无数据：展示空态文案', async () => {
    listOpsTasks.mockResolvedValue({ tasks: [], total: 0 });
    renderPage();
    expect(
      await screen.findByText('暂时没有任务数据，先触发任务后再回来查看。'),
    ).toBeInTheDocument();
    expect(screen.getByText('任务 0')).toBeInTheDocument();
  });

  it('取消任务（表格入口）：先弹确认，点「返回」不调 cancelTask', async () => {
    renderPage();
    await awaitInitialLoad();
    fireEvent.click(within(findRow('task-run-1')).getByRole('button', { name: '取消' }));

    // 弹出确认框，cancelTask 未被调
    expect(
      await screen.findByText('确定取消任务 task-run-1 吗？运行中的执行将被中止。'),
    ).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '取消任务' })).toBeInTheDocument();
    expect(cancelTask).not.toHaveBeenCalled();

    // modal.confirm 按钮不受 ConfigProvider autoInsertSpace 影响，实际渲染「返 回」
    fireEvent.click(screen.getByRole('button', { name: /返\s*回/ }));
    await waitFor(() =>
      expect(screen.queryByText('确定取消任务 task-run-1 吗？运行中的执行将被中止。')).toBeNull(),
    );
    expect(cancelTask).not.toHaveBeenCalled();
  });

  it('取消任务（表格入口）：点「取消任务」确认后调用 cancelTask 并刷新', async () => {
    renderPage();
    await awaitInitialLoad();
    const callsBefore = listOpsTasks.mock.calls.length;
    fireEvent.click(within(findRow('task-run-1')).getByRole('button', { name: '取消' }));
    fireEvent.click(await screen.findByRole('button', { name: '取消任务' }));

    await waitFor(() => expect(cancelTask).toHaveBeenCalledWith('task-run-1'));
    // 成功提示（表格里本就有一个「已取消」Tag，消息会新增一个）
    await waitFor(() => expect(screen.getAllByText('已取消').length).toBeGreaterThanOrEqual(2));
    // 确认后重新加载列表
    await waitFor(() => expect(listOpsTasks.mock.calls.length).toBeGreaterThan(callsBefore));
  });

  it('取消任务失败：提示错误信息', async () => {
    renderPage();
    await awaitInitialLoad();
    cancelTask.mockRejectedValue(new Error('网络中断'));
    fireEvent.click(within(findRow('task-run-1')).getByRole('button', { name: '取消' }));
    fireEvent.click(await screen.findByRole('button', { name: '取消任务' }));
    expect(await screen.findByText('网络中断')).toBeInTheDocument();
  });

  it('状态筛选：Select 选中已取消后生效并可清空', async () => {
    renderPage();
    await awaitInitialLoad();

    // 打开状态下拉并选中「已取消」
    const statusSelect = Array.from(document.querySelectorAll('.ant-select')).find((s) =>
      s.textContent?.includes('状态'),
    );
    if (!statusSelect) throw new Error('status select not found');
    fireEvent.mouseDown(statusSelect);
    const option = await waitFor(() => {
      const dropdown = document.querySelector(
        '.ant-select-dropdown:not(.ant-select-dropdown-hidden)',
      );
      expect(dropdown).toBeTruthy();
      const hit = Array.from(dropdown?.querySelectorAll('.ant-select-item-option') || []).find(
        (o) => o.textContent === '已取消',
      );
      expect(hit).toBeTruthy();
      return hit as HTMLElement;
    });
    fireEvent.click(option);

    // 请求带 status，Alert 显示生效条件，结果数为筛选后的当前页口径
    await waitFor(() =>
      expect(listOpsTasks).toHaveBeenLastCalledWith(
        expect.objectContaining({ status: 'canceled', page: 1 }),
      ),
    );
    expect(await screen.findByText('当前结果 1 个任务')).toBeInTheDocument();
    expect(screen.getByText('当前正在查看筛选后的任务范围')).toBeInTheDocument();
    expect(screen.getByText('已生效条件：状态 已取消')).toBeInTheDocument();

    // 清空筛选
    fireEvent.click(screen.getByRole('button', { name: '清空筛选' }));
    await waitFor(() => expect(listOpsTasks).toHaveBeenLastCalledWith({ page: 1, size: 10 }));
    expect(await screen.findByText('当前结果 6 个任务')).toBeInTheDocument();
    expect(screen.queryByText('当前正在查看筛选后的任务范围')).toBeNull();
  });

  it('函数筛选：选中函数后按 functionId 请求并过滤当前页', async () => {
    renderPage();
    await awaitInitialLoad();

    // 函数下拉（showSearch）与状态下拉都渲染 .ant-select-placeholder 文本节点；
    // 排除分页 size changer（其 placeholder 是每页条数）
    const fnSelect = Array.from(document.querySelectorAll('.ant-select')).find(
      (s) => s.querySelector('.ant-select-placeholder')?.textContent === '函数',
    );
    if (!fnSelect) throw new Error('function select not found');
    fireEvent.mouseDown(fnSelect);
    const option = await waitFor(() => {
      const dropdown = document.querySelector(
        '.ant-select-dropdown:not(.ant-select-dropdown-hidden)',
      );
      expect(dropdown).toBeTruthy();
      const hit = Array.from(dropdown?.querySelectorAll('.ant-select-item-option') || []).find(
        (o) => o.textContent === 'fn-alpha',
      );
      expect(hit).toBeTruthy();
      return hit as HTMLElement;
    });
    fireEvent.click(option);

    await waitFor(() =>
      expect(listOpsTasks).toHaveBeenLastCalledWith(
        expect.objectContaining({ functionId: 'fn-alpha' }),
      ),
    );
    // fn-alpha 有 task-run-1 / task-fail-3 两行
    expect(await screen.findByText('已生效条件：函数 fn-alpha')).toBeInTheDocument();
    expect(screen.getByText('当前结果 2 个任务')).toBeInTheDocument();
  });

  it('操作者筛选：大小写不敏感匹配，无匹配时展示筛选空态', async () => {
    renderPage();
    await awaitInitialLoad();

    const actorInput = screen.getByPlaceholderText('按操作者过滤');
    fireEvent.change(actorInput, { target: { value: 'ALICE' } });
    await waitFor(() =>
      expect(listOpsTasks).toHaveBeenLastCalledWith(expect.objectContaining({ actor: 'ALICE' })),
    );
    expect(await screen.findByText('已生效条件：操作者 ALICE')).toBeInTheDocument();
    expect(screen.getByText('当前结果 1 个任务')).toBeInTheDocument();

    // 无匹配：筛选空态文案
    fireEvent.change(actorInput, { target: { value: 'nobody' } });
    await waitFor(() => expect(screen.getByText('当前结果 0 个任务')).toBeInTheDocument());
    expect(screen.getByText('当前筛选条件下没有匹配任务，请调整筛选后重试。')).toBeInTheDocument();
  });

  it('刷新按钮：重新请求任务列表', async () => {
    renderPage();
    await awaitInitialLoad();
    const callsBefore = listOpsTasks.mock.calls.length;
    fireEvent.click(screen.getByRole('button', { name: '刷新' }));
    await waitFor(() => expect(listOpsTasks.mock.calls.length).toBe(callsBefore + 1));
  });

  it('分页：切换到第 2 页按新 page 请求', async () => {
    renderPage();
    await awaitInitialLoad();
    fireEvent.click(screen.getByTitle('2'));
    await waitFor(() =>
      expect(listOpsTasks).toHaveBeenLastCalledWith(expect.objectContaining({ page: 2, size: 10 })),
    );
  });

  it('双击行打开详情抽屉', async () => {
    renderPage();
    await awaitInitialLoad();
    fireEvent.doubleClick(findRow('task-ok-2'));
    const drawer = await drawerPanel();
    expect(within(drawer).getByText('task-ok-2')).toBeInTheDocument();
  });

  it('详情抽屉（运行中任务）：自动订阅事件流，onEvent/onError 追加输出', async () => {
    renderPage();
    await awaitInitialLoad();
    fireEvent.click(within(findRow('task-run-1')).getByRole('button', { name: '查看详情' }));

    const drawer = await drawerPanel();
    // 运行中：抽屉内有取消按钮、运行中提示
    expect(within(drawer).getAllByText('task-run-1').length).toBeGreaterThan(0);
    expect(
      within(drawer).getByText('任务仍在运行，优先观察事件流；只有确认需要中止时再取消。'),
    ).toBeInTheDocument();
    expect(within(drawer).getByText('耗时 1.50s')).toBeInTheDocument();
    // 自动订阅
    await waitFor(() =>
      expect(subscribeTaskEvents).toHaveBeenCalledWith('task-run-1', expect.anything()),
    );
    expect(
      within(drawer).getByText(
        '还没有事件流输出。运行中的任务可以先连接事件流；已结束任务可能不会再产生新事件。',
      ),
    ).toBeInTheDocument();

    const handlers = lastSubscriptionHandlers();
    await act(async () => {
      handlers.onEvent?.({
        seq: 1,
        type: 'log',
        progress: 0,
        message: 'step-1',
        payload: null,
        createdAt: '',
      });
    });
    expect(await within(drawer).findByText('log: step-1')).toBeInTheDocument();

    await act(async () => {
      handlers.onError?.(new Error('poll broken'));
    });
    expect(await within(drawer).findByText('error: Error: poll broken')).toBeInTheDocument();
  });

  it('详情抽屉：onDone 拉取最终结果并刷新列表', async () => {
    renderPage();
    await awaitInitialLoad();
    const callsBefore = listOpsTasks.mock.calls.length;
    fireEvent.click(within(findRow('task-run-1')).getByRole('button', { name: '查看详情' }));
    const drawer = await drawerPanel();

    const handlers = lastSubscriptionHandlers();
    await act(async () => {
      await handlers.onDone?.();
    });

    await waitFor(() => expect(fetchTaskResult).toHaveBeenCalledWith('task-run-1'));
    expect(await within(drawer).findByText('结果（succeeded）')).toBeInTheDocument();
    await waitFor(() => {
      // payload 以 JSON 缩进渲染在 <pre> 中，按子串匹配
      const matched = within(drawer).getAllByText(
        (_, el) => el?.tagName === 'PRE' && (el.textContent || '').includes('"ok": true'),
      );
      expect(matched.length).toBeGreaterThan(0);
    });
    await waitFor(() => expect(listOpsTasks.mock.calls.length).toBeGreaterThan(callsBefore));
  });

  it('详情抽屉：onDone 拉取结果失败时静默（仍刷新列表）', async () => {
    renderPage();
    await awaitInitialLoad();
    fetchTaskResult.mockRejectedValue(new Error('result gone'));
    fireEvent.click(within(findRow('task-run-1')).getByRole('button', { name: '查看详情' }));
    await drawerPanel();

    const handlers = lastSubscriptionHandlers();
    await act(async () => {
      await handlers.onDone?.();
    });
    await waitFor(() => expect(fetchTaskResult).toHaveBeenCalledWith('task-run-1'));
    // 结果未加载占位仍在
    expect(await screen.findByText('结果尚未加载')).toBeInTheDocument();
  });

  it('详情抽屉（失败任务）：不自动订阅，展示错误信息与 warning 提示', async () => {
    renderPage();
    await awaitInitialLoad();
    fireEvent.click(within(findRow('task-fail-3')).getByRole('button', { name: '查看详情' }));
    const drawer = await drawerPanel();

    expect(
      within(drawer).getByText('任务已经失败，建议先看错误信息，再核对结果和事件流。'),
    ).toBeInTheDocument();
    expect(within(drawer).getByText('错误：boom failure')).toBeInTheDocument();
    expect(within(drawer).getByText('耗时未知')).toBeInTheDocument();
    // 非运行中任务：抽屉内无取消按钮，也不订阅
    expect(within(drawer).queryByRole('button', { name: '取消' })).toBeNull();
    expect(subscribeTaskEvents).not.toHaveBeenCalled();
    expect(within(drawer).getByText('结果尚未加载')).toBeInTheDocument();
  });

  it('详情抽屉：刷新结果成功/失败与结果卡片分支', async () => {
    renderPage();
    await awaitInitialLoad();
    fireEvent.click(within(findRow('task-ok-2')).getByRole('button', { name: '查看详情' }));
    let drawer = await drawerPanel();
    expect(within(drawer).getByText('任务已结束，可以直接查看结果和事件流。')).toBeInTheDocument();

    // 成功：message 展示状态
    fireEvent.click(within(drawer).getByRole('button', { name: '刷新结果' }));
    expect(await screen.findByText('状态：succeeded')).toBeInTheDocument();
    expect(await within(drawer).findByText('结果（succeeded）')).toBeInTheDocument();

    // 失败：提示错误
    fetchTaskResult.mockRejectedValueOnce(new Error('查询超时'));
    fireEvent.click(within(drawer).getByRole('button', { name: '刷新结果' }));
    expect(await screen.findByText('查询超时')).toBeInTheDocument();
  });

  it('详情抽屉：结果含 error / 无 payload 的分支', async () => {
    renderPage();
    await awaitInitialLoad();
    fireEvent.click(within(findRow('task-ok-2')).getByRole('button', { name: '查看详情' }));
    const drawer = await drawerPanel();

    fetchTaskResult.mockResolvedValueOnce({ state: 'failed', error: 'exec boom' });
    fireEvent.click(within(drawer).getByRole('button', { name: '刷新结果' }));
    expect(await within(drawer).findByText('错误：exec boom')).toBeInTheDocument();

    fetchTaskResult.mockResolvedValueOnce({ state: 'succeeded' });
    fireEvent.click(within(drawer).getByRole('button', { name: '刷新结果' }));
    expect(await within(drawer).findByText('当前还没有结果数据')).toBeInTheDocument();
  });

  it('详情抽屉：手动连接/断开事件流', async () => {
    renderPage();
    await awaitInitialLoad();
    fireEvent.click(within(findRow('task-ok-2')).getByRole('button', { name: '查看详情' }));
    const drawer = await drawerPanel();

    fireEvent.click(within(drawer).getByRole('button', { name: '连接' }));
    await waitFor(() => expect(subscribeTaskEvents).toHaveBeenCalledTimes(1));
    const sub = subscribeTaskEvents.mock.results[0].value as { close: jest.Mock };

    // 再次连接：先关闭上一条订阅再新建
    fireEvent.click(within(drawer).getByRole('button', { name: '连接' }));
    await waitFor(() => {
      expect(sub.close).toHaveBeenCalled();
      expect(subscribeTaskEvents).toHaveBeenCalledTimes(2);
    });
    const sub2 = subscribeTaskEvents.mock.results[1].value as { close: jest.Mock };

    fireEvent.click(within(drawer).getByRole('button', { name: '断开' }));
    await waitFor(() => expect(sub2.close).toHaveBeenCalled());

    // 手动连接后 onDone 同样拉结果并刷新
    fetchTaskResult.mockResolvedValueOnce({ state: 'succeeded', payload: [1, 2] });
    const handlers = lastSubscriptionHandlers();
    const callsBefore = listOpsTasks.mock.calls.length;
    await act(async () => {
      await handlers.onDone?.();
    });
    expect(await within(drawer).findByText('结果（succeeded）')).toBeInTheDocument();
    await waitFor(() => expect(listOpsTasks.mock.calls.length).toBeGreaterThan(callsBefore));
  });

  it('详情抽屉：运行中任务的取消入口同样走确认', async () => {
    renderPage();
    await awaitInitialLoad();
    fireEvent.click(within(findRow('task-run-1')).getByRole('button', { name: '查看详情' }));
    let drawer = await drawerPanel();

    fireEvent.click(within(drawer).getByRole('button', { name: '取消' }));
    expect(
      await screen.findByText('确定取消任务 task-run-1 吗？运行中的执行将被中止。'),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: '取消任务' }));

    await waitFor(() => expect(cancelTask).toHaveBeenCalledWith('task-run-1'));
    // 抽屉内任务状态同步为已取消（概览 stub 与 Descriptions Tag 各一处）
    drawer = await drawerPanel();
    expect((await within(drawer).findAllByText('已取消')).length).toBeGreaterThanOrEqual(1);
  });

  it('关闭抽屉：清理订阅', async () => {
    renderPage();
    await awaitInitialLoad();
    fireEvent.click(within(findRow('task-run-1')).getByRole('button', { name: '查看详情' }));
    await drawerPanel();
    await waitFor(() => expect(subscribeTaskEvents).toHaveBeenCalledTimes(1));
    const sub = subscribeTaskEvents.mock.results[0].value as { close: jest.Mock };

    const closeBtn = await waitFor(() => {
      const btn = document.querySelector('.ant-drawer-close');
      expect(btn).toBeTruthy();
      return btn as HTMLElement;
    });
    fireEvent.click(closeBtn);
    await waitFor(() => expect(sub.close).toHaveBeenCalled());
    // 抽屉收起（jsdom 下 motion 不卸载 DOM，以 open class 消失为准）
    await waitFor(() => expect(document.querySelector('.ant-drawer-open')).toBeNull());
  });
});
