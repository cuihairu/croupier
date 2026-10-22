/**
 * 函数调用历史页单测（覆盖率巡检：Functions/History/index.tsx 739 行 0% → 收口）。
 *
 * 锁定契约：
 * - 统计六卡（总调用/成功 /total 后缀/失败/运行中/平均耗时 formatDuration 复用/
 *   成功率 total>0 toFixed(1) 与 total=0 两臂）+ 加载失败静默（console.warn，卡片不渲染）；
 * - 列表渲染矩阵（六状态徽标 + 未知状态回退 pending、gameId/env 兜底、
 *   formatDuration 三段位（ms/s/m）与缺省 '-'、formatTime 合法/非法/缺省、
 *   errorMessage Tooltip 红字与 '-' 兜底、分页共 N 条）；
 * - request 载荷合并（工具栏筛选经 params.filters 与查询表单字段合并，表单显式
 *   输入优先）+ 失败翼（extractErrorMessage → message.error）+ 响应缺省翼
 *   （calls/total || 兜底）；
 * - 工具栏状态下拉（antd6 Select：mouseDown 根 + 可见 option content）与
 *   RangePicker（Enter 提交起止 + 清空图标双臂：设值带 startTime/endTime、
 *   清空剥键重查）；
 * - LightFilter 查询表单（chip → popover 输入 → 确 认 → 提交，functionId/
 *   status/gameId 三字段合并）；
 * - 刷新按钮（reload + fetchStats）；
 * - 详情抽屉（getFunctionCallDetail 成功富化渲染全字段 + response 缺省回落
 *   行数据 + 失败两翼（Error.message 透传 / 非 object 兜底文案）+ 空值形态
 *   （可选字段全 '-'、无错误项、无 payload/result 卡片、未知状态原文兜底））；
 * - 轮询（fake timers）：最近列表含 running/pending → 5s 自动重拉列表与统计，
 *   全终态不重拉。
 *
 * mock 口径：services/api/function-calls 三函数 jest.mock；pro-components/
 * antd/extractErrorMessage 走真实实现；@umijs/max 本地 mock（defaultMessage
 * 即文案，showTotal 的 defaultMessage 已内插 total）。
 *
 * 坑实证（antd6）：RangePicker 单面板，输入须 focus + change + keyDown Enter
 * 逐个提交（直接 change + OK 不提交）；清空图标须先 mouseEnter 才在 DOM 可点。
 * LightFilter 字段是 chip（.ant-pro-core-field-label，与表头同文本，按选择器
 * 区分），点开 popover（.ant-popover 取未隐藏实例）内为统一「请输入」输入框，
 * footer 确认按钮锚 button[data-type="confirm"]。统计卡的值与 % 后缀分属不同
 * 元素（getByText('80.0%') 不匹配），按 .ant-statistic-title 锚卡断内容拼接。
 * loading 态表格即渲染 Empty 壳——空态断言须先锚定请求已发。jsdom 下 Drawer
 * 关闭动效不收尾（壳文本残留），关闭翼以「可再打开且重拉」锁定连续性。
 * useIntl mock 须返回稳定实例——fetchStats 的 useCallback 依赖 intl，每次
 * 渲染新对象会让 mount effect 重复触发统计拉取（刷新/轮询计数断言失真）。
 */
import React from 'react';
import { act, configure, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { App } from 'antd';
import FunctionHistoryPage from '../index';
import type { FunctionCallItem } from '@/services/api/function-calls';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

jest.mock('@/services/api/function-calls', () => ({
  listFunctionCalls: jest.fn(),
  getFunctionCallDetail: jest.fn(),
  getFunctionCallStats: jest.fn(),
}));

// mock* 前缀变量：babel-jest hoist 白名单
// intl 必须是稳定实例——fetchStats 的 useCallback 依赖 [filters, intl]，
// 每次渲染新对象会让 mount effect 重复触发统计拉取
const mockIntl = {
  formatMessage: (opts: { defaultMessage?: string }) => opts.defaultMessage ?? '',
};

jest.mock('@umijs/max', () => ({
  FormattedMessage: ({ defaultMessage }: { defaultMessage?: string }) => (
    <>{defaultMessage ?? ''}</>
  ),
  useIntl: () => mockIntl,
}));

import {
  listFunctionCalls,
  getFunctionCallDetail,
  getFunctionCallStats,
} from '@/services/api/function-calls';

const mList = listFunctionCalls as jest.MockedFunction<typeof listFunctionCalls>;
const mDetail = getFunctionCallDetail as jest.MockedFunction<typeof getFunctionCallDetail>;
const mStats = getFunctionCallStats as jest.MockedFunction<typeof getFunctionCallStats>;

const call1: FunctionCallItem = {
  id: 'c1',
  taskId: 'task-aaa',
  functionId: 'fn.demo.query',
  gameId: 'demo',
  env: 'prod',
  actorId: 'admin',
  actorType: 'admin',
  status: 'succeeded',
  agentId: 'agent-1',
  serviceId: 'svc-1',
  startedAt: '2026-09-01T08:30:00Z',
  finishedAt: '2026-09-01T08:30:02Z',
  durationMs: 2500,
  payload: { a: 1 },
  result: { ok: true },
  errorMessage: '',
  retryCount: 2,
  createdAt: '2026-09-01T08:29:59Z',
};
// 覆盖翼：running 徽标、gameId ''/env 缺省、500ms、startedAt 缺省、错误红字
const call2: FunctionCallItem = {
  id: 'c2',
  taskId: 'task-bbb',
  functionId: 'fn.demo.mutate',
  gameId: '',
  status: 'running',
  durationMs: 500,
  errorMessage: 'boom',
  createdAt: '2026-09-01T09:00:00Z',
};
// 覆盖翼：未知状态回退 pending、1.50m、非法时间串、errorMessage 缺省
const call3: FunctionCallItem = {
  id: 'c3',
  taskId: 'task-ccc',
  functionId: 'fn.other.exec',
  gameId: 'demo',
  env: 'dev',
  status: 'weird',
  durationMs: 90000,
  startedAt: 'not-a-date',
  createdAt: '2026-09-01T10:00:00Z',
};

const startedAtText = new Date('2026-09-01T08:30:00Z').toLocaleString('zh-CN', {
  hour12: false,
});

beforeEach(() => {
  jest.clearAllMocks();
  mList.mockResolvedValue({ calls: [call1, call2, call3], total: 3, page: 1, pageSize: 20 });
  mDetail.mockResolvedValue({ ...call1 } as never);
  mStats.mockResolvedValue({
    total: 10,
    succeeded: 8,
    failed: 1,
    running: 1,
    cancelled: 0,
    timeout: 0,
    other: 0,
    avgDurationMs: 2500,
  });
});

function renderPage() {
  return render(
    <App>
      <FunctionHistoryPage />
    </App>,
  );
}

/** 按统计卡标题取 .ant-statistic-content 的拼接文本（值/后缀可能分元素） */
function statContent(title: string) {
  const titleEl = Array.from(document.querySelectorAll('.ant-statistic-title')).find(
    (n) => n.textContent === title,
  ) as HTMLElement;
  expect(titleEl).not.toBeUndefined();
  const stat = titleEl.closest('.ant-statistic') as HTMLElement;
  return stat.querySelector('.ant-statistic-content')?.textContent ?? '';
}

/** 等首拉落定（ProTable 20ms 防抖：先等行文本再锁计数，后续断言锚 last call） */
async function waitFirstLoad() {
  expect(await screen.findByText('task-aaa')).toBeInTheDocument();
  await waitFor(() => expect(mList).toHaveBeenCalledTimes(1));
}

/** 点 LightFilter chip（与表头同文本，按 .ant-pro-core-field-label 锚定）→ popover */
async function openChip(label: string) {
  const chip = Array.from(document.querySelectorAll('.ant-pro-core-field-label')).find((c) =>
    c.textContent?.startsWith(label),
  ) as HTMLElement;
  expect(chip).not.toBeUndefined();
  fireEvent.click(chip);
  const popover = await waitFor(() => {
    const visible = Array.from(document.querySelectorAll('.ant-popover')).find(
      (p) => !p.className.includes('ant-popover-hidden'),
    ) as HTMLElement;
    expect(visible).not.toBeUndefined();
    return visible;
  });
  return within(popover);
}

/** LightFilter 字段查询：chip → 输入 → 确 认（提交触发重查） */
async function searchViaChip(label: string, value: string) {
  const popover = await openChip(label);
  fireEvent.change(popover.getByPlaceholderText('请输入'), { target: { value } });
  fireEvent.click(popover.getByRole('button', { name: /确/ }) as HTMLElement);
}

/** 工具栏状态下拉选 option（antd6：mouseDown 落 .ant-select 根，点可见 option） */
async function pickToolbarStatus(label: string) {
  const select = document.querySelector('.ant-pro-table-list-toolbar .ant-select') as HTMLElement;
  expect(select).not.toBeNull();
  fireEvent.mouseDown(select);
  const dropdown = await waitFor(() => {
    const visible = Array.from(document.querySelectorAll('.ant-select-dropdown')).find(
      (d) => !d.className.includes('ant-select-dropdown-hidden'),
    ) as HTMLElement;
    expect(visible).not.toBeUndefined();
    return visible;
  });
  fireEvent.click(
    within(dropdown).getByText(label, { selector: '.ant-select-item-option-content' }),
  );
}

/** 行内详情按钮（行尾 fixed-right 操作列：该行最后一个 button） */
function detailButtonOf(rowText: string) {
  const row = screen.getByText(rowText).closest('tr') as HTMLElement;
  const btns = row.querySelectorAll('button');
  const btn = btns[btns.length - 1] as HTMLElement;
  expect(btn).not.toBeFalsy();
  return btn;
}

describe('函数调用历史 统计卡片', () => {
  it('六卡渲染：总数/成功 /total/失败/运行中/平均耗时/成功率 toFixed', async () => {
    renderPage();
    await waitFirstLoad();

    expect(statContent('总调用')).toBe('10');
    expect(statContent('成功')).toBe('8/ 10');
    expect(statContent('失败')).toBe('1');
    expect(statContent('运行中')).toBe('1');
    expect(statContent('平均耗时')).toBe('2.50s'); // avgDurationMs 复用 formatDuration
    expect(statContent('成功率')).toBe('80.0%');
  });

  it('total=0 与 avgDurationMs 缺省：成功率 0、平均耗时 -', async () => {
    mStats.mockResolvedValue({
      total: 0,
      succeeded: 0,
      failed: 0,
      running: 0,
      cancelled: 0,
      timeout: 0,
      other: 0,
      avgDurationMs: 0,
    });
    renderPage();
    await waitFirstLoad();

    expect(statContent('成功率')).toBe('0%'); // total=0 → 数值 0 不走 toFixed
    expect(statContent('成功')).toBe('0/ 0');
    expect(statContent('平均耗时')).toBe('-'); // formatDuration(0) 兜底
  });

  it('统计加载失败：console.warn 静默、卡片不渲染、列表不受影响', async () => {
    const warnSpy = jest.spyOn(console, 'warn').mockImplementation(() => {});
    mStats.mockRejectedValue(new Error('stats-down'));
    renderPage();
    await waitFirstLoad();

    expect(warnSpy).toHaveBeenCalled();
    expect(screen.queryByText('总调用')).not.toBeInTheDocument();
    expect(screen.getByText('task-aaa')).toBeInTheDocument();
    warnSpy.mockRestore();
  });
});

describe('函数调用历史 列表渲染矩阵', () => {
  it('状态徽标（含未知回退 pending）/兜底列/时间与时长格式化/错误列/分页总数', async () => {
    renderPage();
    await waitFirstLoad();

    // 状态徽标：succeeded/running/未知回退 等待中（统计卡与徽标同文案，计数断言）
    expect(screen.getAllByText('成功')).toHaveLength(2); // 统计卡 + c1 徽标
    expect(screen.getAllByText('运行中')).toHaveLength(2); // 统计卡 + c2 徽标
    expect(screen.getByText('等待中')).toBeInTheDocument(); // c3 未知状态回退

    // 任务/函数列
    expect(screen.getByText('task-aaa')).toBeInTheDocument();
    expect(screen.getByText('fn.demo.query')).toBeInTheDocument();

    // 游戏/环境：c1/c3 游戏+Tag；c2 双兜底 '-'
    expect(screen.getAllByText('demo')).toHaveLength(2);
    expect(screen.getByText('prod')).toBeInTheDocument();
    expect(screen.getByText('dev')).toBeInTheDocument();

    // Agent/执行人
    expect(screen.getByText('agent-1')).toBeInTheDocument();
    expect(screen.getByText('admin')).toBeInTheDocument();

    // 持续时间三段位 + 缺省（2.50s 统计卡与行内双份）
    expect(screen.getAllByText('2.50s')).toHaveLength(2);
    expect(screen.getByText('500ms')).toBeInTheDocument();
    expect(screen.getByText('1.50m')).toBeInTheDocument();

    // 开始时间：合法格式化（与组件同表达式）/ 非法与缺省 '-'
    expect(screen.getAllByText(startedAtText)).toHaveLength(1);
    const dashes = screen.getAllByText('-');
    expect(dashes.length).toBeGreaterThanOrEqual(4); // c2 gameId/startedAt、c3 startedAt、无错误行

    // 错误列：红字 + 无错误兜底
    expect(screen.getByText('boom')).toBeInTheDocument();

    // 分页总数
    expect(screen.getByText('共 3 条记录')).toBeInTheDocument();

    // 首拉载荷
    expect(mList).toHaveBeenLastCalledWith({ page: 1, pageSize: 20 });
  });

  it('列表失败翼：message.error 透传后端 message', async () => {
    mList.mockRejectedValue({ response: { data: { message: 'boom-list' } } });
    renderPage();
    expect(await screen.findByText('boom-list')).toBeInTheDocument();
    await waitFor(() => expect(document.querySelector('.ant-empty-description')).not.toBeNull());
  });

  it('响应缺省翼：calls/total 缺省 → 空表 + 共 0 条', async () => {
    mList.mockResolvedValue({} as never);
    renderPage();
    // loading 态即有 Empty 壳，须先锚定请求已发且落定后再断言表内空态
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(1));
    await waitFor(() =>
      expect(document.querySelector('.ant-table .ant-empty-description')).not.toBeNull(),
    );
  });
});

describe('函数调用历史 筛选', () => {
  it('LightFilter 查询表单：functionId/status/gameId 合并进载荷', async () => {
    renderPage();
    await waitFirstLoad();

    await searchViaChip('函数ID', 'fn.demo');
    await waitFor(() =>
      expect(mList).toHaveBeenLastCalledWith(
        expect.objectContaining({ functionId: 'fn.demo', page: 1 }),
      ),
    );

    await searchViaChip('状态', 'failed');
    await waitFor(() =>
      expect(mList).toHaveBeenLastCalledWith(
        expect.objectContaining({ functionId: 'fn.demo', status: 'failed' }),
      ),
    );

    await searchViaChip('游戏/环境', 'demo');
    await waitFor(() =>
      expect(mList).toHaveBeenLastCalledWith(
        expect.objectContaining({ functionId: 'fn.demo', status: 'failed', gameId: 'demo' }),
      ),
    );
  });

  it('工具栏状态筛选下拉：status 进载荷', async () => {
    renderPage();
    await waitFirstLoad();

    await pickToolbarStatus('失败');
    await waitFor(() =>
      expect(mList).toHaveBeenLastCalledWith(
        expect.objectContaining({ status: 'failed', page: 1 }),
      ),
    );
  });

  it('RangePicker：Enter 提交起止（ISO 载荷）→ 清空剥键重查', async () => {
    renderPage();
    await waitFirstLoad();

    const picker = document.querySelector('.ant-picker-range') as HTMLElement;
    const inputs = Array.from(picker.querySelectorAll('input')) as HTMLInputElement[];
    fireEvent.focus(inputs[0]);
    fireEvent.change(inputs[0], { target: { value: '2026-09-01 00:00:00' } });
    fireEvent.keyDown(inputs[0], { key: 'Enter' });
    fireEvent.focus(inputs[1]);
    fireEvent.change(inputs[1], { target: { value: '2026-09-02 12:30:00' } });
    fireEvent.keyDown(inputs[1], { key: 'Enter' });

    await waitFor(() =>
      expect(mList).toHaveBeenLastCalledWith(
        expect.objectContaining({
          startTime: new Date('2026-09-01T00:00:00').toISOString(),
          endTime: new Date('2026-09-02T12:30:00').toISOString(),
        }),
      ),
    );

    // 清空：filters 剥除 startTime/endTime 后重查
    fireEvent.mouseEnter(picker);
    const clear = picker.querySelector('.ant-picker-clear') as HTMLElement;
    expect(clear).not.toBeNull();
    fireEvent.click(clear);
    await waitFor(() => expect(mList).toHaveBeenLastCalledWith({ page: 1, pageSize: 20 }));
  });

  it('刷新按钮：列表与统计双拉', async () => {
    renderPage();
    await waitFirstLoad();
    expect(mStats).toHaveBeenCalledTimes(1);

    fireEvent.click(screen.getByRole('button', { name: /刷新/ }));
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(mStats).toHaveBeenCalledTimes(2));
  });
});

describe('函数调用历史 详情抽屉', () => {
  it('主链：详情富化渲染全字段 + payload/result 卡片 + 关闭', async () => {
    mDetail.mockResolvedValue({
      ...call1,
      status: 'failed',
      retryCount: 3,
      serviceId: 'svc-9',
      finishedAt: '2026-09-01T08:30:09Z',
      errorMessage: 'det-boom',
    } as never);
    renderPage();
    await waitFirstLoad();

    fireEvent.click(detailButtonOf('task-aaa'));
    await waitFor(() => expect(mDetail).toHaveBeenCalledWith('c1'));
    expect(await screen.findByText('调用详情')).toBeInTheDocument();

    // 描述项（详情富化值优先于行数据）
    expect(screen.getByText('svc-9')).toBeInTheDocument();
    expect(screen.getByText('det-boom')).toBeInTheDocument();
    const retryCells = screen.getAllByText('3');
    expect(retryCells.length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText(startedAtText).length).toBeGreaterThanOrEqual(2);
    expect(screen.getAllByText('2.50s').length).toBeGreaterThanOrEqual(3);

    // payload/result 卡片（pre 原文断言键值即可）
    expect(screen.getByText('请求数据')).toBeInTheDocument();
    expect(screen.getByText('响应数据')).toBeInTheDocument();
    const pres = Array.from(document.querySelectorAll('.ant-drawer pre'));
    expect(pres.some((p) => p.textContent?.includes('"a": 1'))).toBe(true);
    expect(pres.some((p) => p.textContent?.includes('"ok": true'))).toBe(true);

    // 关闭：onClose 已触达（jsdom 下 antd6 Drawer 关闭动效不收尾、壳文本残留，
    // 隐藏态断言不可靠——以再打开重拉详情锁定交互连续性）
    fireEvent.click(document.querySelector('.ant-drawer-close') as HTMLElement);
    fireEvent.click(detailButtonOf('task-bbb'));
    await waitFor(() => expect(mDetail).toHaveBeenLastCalledWith('c2'));
  });

  it('详情 response 缺省 → 回落行数据（response || record 翼）', async () => {
    mDetail.mockResolvedValue(undefined as never);
    renderPage();
    await waitFirstLoad();

    fireEvent.click(detailButtonOf('task-bbb'));
    await waitFor(() => expect(mDetail).toHaveBeenCalledWith('c2'));
    // 行数据兜底渲染：c2 的 taskId 与错误信息
    expect(await screen.findByText('调用详情')).toBeInTheDocument();
    expect(screen.getAllByText('task-bbb').length).toBeGreaterThanOrEqual(2);
    expect(screen.getAllByText('boom').length).toBeGreaterThanOrEqual(2);
  });

  it('详情失败两翼：Error.message 透传 / 非 object 走兜底文案，抽屉不打开', async () => {
    renderPage();
    await waitFirstLoad();

    mDetail.mockRejectedValue(new Error('det-fail'));
    fireEvent.click(detailButtonOf('task-aaa'));
    expect(await screen.findByText('det-fail')).toBeInTheDocument();
    expect(screen.queryByText('调用详情')).not.toBeInTheDocument();

    mDetail.mockRejectedValue(undefined);
    fireEvent.click(detailButtonOf('task-aaa'));
    expect(await screen.findByText('获取详情失败')).toBeInTheDocument();
    expect(screen.queryByText('调用详情')).not.toBeInTheDocument();
  });

  it('空值形态：可选字段全兜底、无错误项、无数据卡片、未知状态原文兜底', async () => {
    mDetail.mockResolvedValue({
      id: 'c3',
      taskId: 'task-ccc',
      functionId: 'fn.other.exec',
      status: 'weird',
      createdAt: '',
    } as never);
    renderPage();
    await waitFirstLoad();

    fireEvent.click(detailButtonOf('task-ccc'));
    await waitFor(() => expect(mDetail).toHaveBeenCalledWith('c3'));
    expect(await screen.findByText('调用详情')).toBeInTheDocument();

    // 未知状态：Badge 文案回退原文（fallback = raw status）
    expect(screen.getAllByText('weird').length).toBeGreaterThanOrEqual(1);
    // 兜底 '-' 族（gameId/env/agent/service/actor/actorType/三时间/时长）
    expect(screen.getAllByText('-').length).toBeGreaterThanOrEqual(9);
    // 重试次数缺省 → 0
    expect(screen.getByText('0')).toBeInTheDocument();
    // 无错误项与数据卡片
    expect(screen.queryByText('请求数据')).not.toBeInTheDocument();
    expect(screen.queryByText('响应数据')).not.toBeInTheDocument();
  });
});

describe('函数调用历史 自动刷新轮询（fake timers）', () => {
  beforeEach(() => {
    jest.useFakeTimers();
  });
  afterEach(() => {
    jest.useRealTimers();
  });

  it('最近列表含 running/pending → 5s 自动重拉列表与统计', async () => {
    renderPage();
    await act(async () => {
      await jest.advanceTimersByTimeAsync(100);
    });
    expect(mList).toHaveBeenCalledTimes(1);
    expect(mStats).toHaveBeenCalledTimes(1);

    await act(async () => {
      await jest.advanceTimersByTimeAsync(5000);
    });
    expect(mList).toHaveBeenCalledTimes(2);
    expect(mStats).toHaveBeenCalledTimes(2);
  });

  it('全终态列表 → 5s 不自动重拉', async () => {
    mList.mockResolvedValue({ calls: [call1], total: 1, page: 1, pageSize: 20 });
    renderPage();
    await act(async () => {
      await jest.advanceTimersByTimeAsync(100);
    });
    expect(mList).toHaveBeenCalledTimes(1);

    await act(async () => {
      await jest.advanceTimersByTimeAsync(5000);
    });
    expect(mList).toHaveBeenCalledTimes(1);
    expect(mStats).toHaveBeenCalledTimes(1);
  });
});
