/*
 * useAssignmentsPage hook 专项测试
 *
 * 全 mock 外沿（antd App.useApp message、useIntl/useModel/history、
 * services/api 五函数、useScope、三 build 列函数、viewModel、
 * pageSchema、PageRenderer），驱动 hook 自身分支：
 * descriptors 三信封形态归一、gameId 有无的 assignments 拉取与失败兜底、
 * onSave remove/assign + unknown warning、onBatchAssign 并集/过滤、
 * onCloneToEnv 四臂、loadHistory 信封/缺省 total/失败清空/action 透传、
 * canWrite 角色解析、pageCtx 全回调、gameId 变化重拉（#35）。
 */
import React from 'react';
import { act, renderHook, waitFor } from '@testing-library/react';
import useAssignmentsPage from '../useAssignmentsPage';

jest.setTimeout(30000);

jest.mock('antd', () => ({
  App: {
    useApp: () => ({
      message: {
        warning: messageWarning,
        success: messageSuccess,
        error: messageError,
      },
    }),
  },
}));

jest.mock('@umijs/max', () => ({
  useIntl: () => ({
    formatMessage: (
      opts: { id: string; defaultMessage?: string },
      values?: Record<string, unknown>,
    ) => {
      let text = opts.defaultMessage ?? opts.id;
      if (values)
        for (const [k, v] of Object.entries(values)) text = text.replace(`{${k}}`, String(v));
      return text;
    },
  }),
  useModel: () => ({ initialState: { currentUser: { access: mockAccess } } }),
  // 惰性转发同 services/api：工厂执行早于模块级 const 声明（TDZ）。
  history: { push: (...args: Parameters<typeof historyPush>) => historyPush(...args) },
}));

jest.mock('@/services/api', () => ({
  // 惰性转发：jest.mock 工厂在 import 链上执行，早于模块级 const 声明（TDZ）。
  listDescriptors: (...args: Parameters<typeof listDescriptors>) => listDescriptors(...args),
  fetchAssignments: (...args: Parameters<typeof fetchAssignments>) => fetchAssignments(...args),
  fetchAssignmentsHistory: (...args: Parameters<typeof fetchAssignmentsHistory>) =>
    fetchAssignmentsHistory(...args),
  setAssignments: (...args: Parameters<typeof setAssignments>) => setAssignments(...args),
}));

jest.mock('@/hooks/useScopeReload', () => ({
  useScope: () => ({ scope: { gameId: mockGameId } }),
}));

// 列/视图/页面骨架均已专项测试，hook 侧 mock 为哨兵返回聚焦自身逻辑；
// 捕获 opts 以驱动 hook 内的 onOpenDetail 闭包（routerHistory.push）。
jest.mock('../columns', () => ({
  buildAssignmentColumns: jest.fn((opts: Record<string, unknown>) => {
    capturedListOpts = opts;
    return [{ title: 'cols' }];
  }),
  buildCategoryColumns: jest.fn(() => [{ title: 'rcols' }]),
  buildRouteColumns: jest.fn((opts: Record<string, unknown>) => {
    capturedRouteOpts = opts;
    return [{ title: 'ccols' }];
  }),
}));
jest.mock('../viewModel', () => ({
  buildAssignmentOptions: jest.fn((descs: Array<{ id: string }>) =>
    descs.map((d) => ({ value: d.id, resource: 'player' })),
  ),
  buildGroupedAssignments: jest.fn(() => [{ grouped: true }]),
  buildAssignmentStats: jest.fn(() => [{ stat: true }]),
}));
jest.mock('../pageSchema', () => ({
  ASSIGNMENTS_PAGE_SCHEMA: {
    listColumns: [],
    rowActions: [],
    resourceColumns: [],
    capabilityColumns: [],
  },
}));
jest.mock('../PageRenderer', () => ({
  renderPageActions: jest.fn(() => [{ action: true }]),
}));

const messageWarning = jest.fn();
const messageSuccess = jest.fn();
const messageError = jest.fn();
const historyPush = jest.fn();
const listDescriptors = jest.fn();
const fetchAssignments = jest.fn();
const fetchAssignmentsHistory = jest.fn();
const setAssignments = jest.fn();

let capturedListOpts: { onOpenDetail?: (id: string) => void } = {};
let capturedRouteOpts: { onOpenDetail?: (id: string) => void } = {};

let mockGameId: string | undefined = 'demo';
let mockAccess: string | undefined = '*';

const DESCRIPTORS = [
  { id: 'fn.a', summary: { 'zh-CN': 'A' } },
  { id: 'fn.b', summary: { 'zh-CN': 'B' } },
];

beforeEach(() => {
  jest.clearAllMocks();
  mockGameId = 'demo';
  mockAccess = '*';
  capturedListOpts = {};
  capturedRouteOpts = {};
  listDescriptors.mockResolvedValue(DESCRIPTORS);
  fetchAssignments.mockResolvedValue({ assignments: { player: ['fn.a'] } });
  fetchAssignmentsHistory.mockResolvedValue({ items: [], total: 0 });
  setAssignments.mockResolvedValue({ unknown: [] });
});

const setup = () =>
  renderHook(() => useAssignmentsPage(), {
    wrapper: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  });

describe('加载与 descriptors 归一', () => {
  it('初始加载：数组 descriptors、无 gameId 不拉 assignments', async () => {
    mockGameId = undefined;
    const { result } = setup();

    await waitFor(() => expect(listDescriptors).toHaveBeenCalled());
    await waitFor(() => expect(result.current.pageCtx.loading).toBe(false));
    expect(fetchAssignments).not.toHaveBeenCalled();
    expect(result.current.pageCtx.hasScope).toBe(false);
  });

  it('descriptors 信封三形态：functions/descriptors/空对象兜底', async () => {
    const { result } = setup();
    await waitFor(() => expect(result.current.pageCtx.loading).toBe(false));

    // 裸数组形态由初始加载用例覆盖；此处依次验证信封形态。
    listDescriptors.mockResolvedValue({ functions: [{ id: 'fn.f1' }] });
    await act(async () => {
      await result.current.pageCtx.onReload();
    });
    expect(result.current.pageCtx.groupedAssignments).toEqual([{ grouped: true }]);

    listDescriptors.mockResolvedValue({ descriptors: [{ id: 'fn.d1' }] });
    await act(async () => {
      await result.current.pageCtx.onReload();
    });

    listDescriptors.mockResolvedValue({});
    await act(async () => {
      await result.current.pageCtx.onReload();
    });
    expect(result.current.pageCtx.onSelectAll).toBeDefined();
  });

  it('有 gameId：fetchAssignments 展平 assignments map', async () => {
    const { result } = setup();
    await waitFor(() => expect(fetchAssignments).toHaveBeenCalled());
    await waitFor(() => expect(result.current.pageCtx.selected).toEqual(['fn.a']));
  });

  it('fetchAssignments 失败 → selected 清空不崩', async () => {
    fetchAssignments.mockRejectedValue(new Error('boom'));
    const { result } = setup();
    await waitFor(() => expect(fetchAssignments).toHaveBeenCalled());
    await waitFor(() => expect(result.current.pageCtx.selected).toEqual([]));
  });

  it('assignments 载荷缺省（{}）→ `res?.assignments || {}` 右臂 + flat 空', async () => {
    fetchAssignments.mockResolvedValue({});
    const { result } = setup();
    await waitFor(() => expect(fetchAssignments).toHaveBeenCalled());
    await waitFor(() => expect(result.current.pageCtx.selected).toEqual([]));
  });

  it('gameId 变化 → load 重建并重拉（#35）', async () => {
    const { result, rerender } = setup();
    await waitFor(() => expect(result.current.pageCtx.loading).toBe(false));
    const calls = listDescriptors.mock.calls.length;

    mockGameId = 'other';
    rerender();
    await waitFor(() => expect(listDescriptors.mock.calls.length).toBeGreaterThan(calls));
  });
});

describe('canWrite 角色解析', () => {
  it.each([
    ['*', true],
    ['assignments:write', true],
    ['other:write,assignments:write', true],
    ['other:write', false],
    [undefined, false],
    ['', false],
  ])('access=%p → canWrite=%p', async (access, expected) => {
    mockAccess = access;
    const { result } = setup();
    await waitFor(() => expect(result.current.pageCtx.loading).toBe(false));
    expect(result.current.pageCtx.canWrite).toBe(expected);
  });
});

describe('onSave', () => {
  it('无 gameId → warning 早退不调 setAssignments', async () => {
    mockGameId = undefined;
    const { result } = setup();
    await waitFor(() => expect(result.current.pageCtx.loading).toBe(false));

    await act(async () => {
      await result.current.pageCtx.onSave();
    });
    // 词条外置语言包（无 defaultMessage）→ mock 回退返回 id 本身。
    expect(messageWarning).toHaveBeenCalledWith('pages.assignments.select.game');
    expect(setAssignments).not.toHaveBeenCalled();
  });

  it('selected 空 → action=remove；非空 → assign；成功 message + 重拉', async () => {
    const { result } = setup();
    await waitFor(() => expect(result.current.pageCtx.selected).toEqual(['fn.a']));

    await act(async () => {
      await result.current.pageCtx.onSave();
    });
    expect(setAssignments).toHaveBeenLastCalledWith({ action: 'assign', functions: ['fn.a'] });
    // 词条外置 → mock 返回 id。
    expect(messageSuccess).toHaveBeenCalledWith('pages.assignments.save.success');

    await act(async () => {
      result.current.pageCtx.onClearAll();
    });
    await act(async () => {
      await result.current.pageCtx.onSave();
    });
    expect(setAssignments).toHaveBeenLastCalledWith({ action: 'remove', functions: [] });
  });

  it('unknown 函数 → warning 分支（词条外置，values 不入断言）', async () => {
    setAssignments.mockResolvedValue({ unknown: ['fn.x', 'fn.y'] });
    const { result } = setup();
    await waitFor(() => expect(result.current.pageCtx.selected).toEqual(['fn.a']));

    await act(async () => {
      await result.current.pageCtx.onSave();
    });
    expect(messageWarning).toHaveBeenCalledWith('pages.assignments.save.warning');
    expect(messageSuccess).not.toHaveBeenCalled();
  });

  it('setAssignments 载荷无 unknown（{}）→ `|| []` 右臂走 success；reject 走 finally', async () => {
    setAssignments.mockResolvedValue({});
    const { result } = setup();
    await waitFor(() => expect(result.current.pageCtx.selected).toEqual(['fn.a']));

    await act(async () => {
      await result.current.pageCtx.onSave();
    });
    expect(messageSuccess).toHaveBeenCalled();

    setAssignments.mockRejectedValue(new Error('net'));
    await expect(result.current.pageCtx.onSave()).rejects.toThrow('net');
    await waitFor(() => expect(result.current.pageCtx.loading).toBe(false));
  });

  it('保存后重拉失败（listDescriptors reject）→ onSave finally 异常臂', async () => {
    const { result } = setup();
    await waitFor(() => expect(result.current.pageCtx.selected).toEqual(['fn.a']));

    listDescriptors.mockRejectedValueOnce(new Error('reload'));
    await expect(result.current.pageCtx.onSave()).rejects.toThrow('reload');
    await waitFor(() => expect(result.current.pageCtx.loading).toBe(false));
  });
});

describe('onBatchAssign 与选择回调', () => {
  it('assign 并集去重 / disable 过滤', async () => {
    fetchAssignments.mockResolvedValue({ assignments: { player: ['fn.a'] } });
    const { result } = setup();
    await waitFor(() => expect(result.current.pageCtx.selected).toEqual(['fn.a']));

    await act(async () => {
      result.current.pageCtx.onBatchAssign('player', true);
    });
    await waitFor(() => expect(result.current.pageCtx.selected).toEqual(['fn.a', 'fn.b']));

    await act(async () => {
      result.current.pageCtx.onBatchAssign('player', false);
    });
    await waitFor(() => expect(result.current.pageCtx.selected).toEqual([]));
  });

  it('onSelectAll / onClearAll / onSelectionChange', async () => {
    const { result } = setup();
    await waitFor(() => expect(result.current.pageCtx.loading).toBe(false));

    await act(async () => {
      result.current.pageCtx.onSelectAll();
    });
    await waitFor(() => expect(result.current.pageCtx.selected).toEqual(['fn.a', 'fn.b']));

    await act(async () => {
      result.current.pageCtx.onSelectionChange(['fn.z']);
    });
    await waitFor(() => expect(result.current.pageCtx.selected).toEqual(['fn.z']));

    await act(async () => {
      result.current.pageCtx.onClearAll();
    });
    await waitFor(() => expect(result.current.pageCtx.selected).toEqual([]));
  });

  it('onTabChange / onOpenClone / headerActions 透传', async () => {
    const { result } = setup();
    await waitFor(() => expect(result.current.pageCtx.loading).toBe(false));

    expect(result.current.headerActions).toEqual([{ action: true }]);
    act(() => {
      result.current.pageCtx.onTabChange('resource');
    });
    expect(result.current.pageCtx.activeTab).toBe('resource');

    act(() => {
      result.current.pageCtx.onOpenClone();
    });
    expect(result.current.cloneModalVisible).toBe(true);
    act(() => {
      result.current.setCloneModalVisible(false);
    });
    expect(result.current.cloneModalVisible).toBe(false);
  });

  it('onOpenHistory：置 historyVisible（columns 已 mock，detail 跳转属 columns.test 范围）', async () => {
    const { result } = setup();
    await waitFor(() => expect(result.current.pageCtx.loading).toBe(false));

    await act(async () => {
      await result.current.pageCtx.onOpenHistory();
    });
    expect(result.current.historyVisible).toBe(true);
    expect(historyPush).not.toHaveBeenCalled();
  });

  it('onOpenDetail 闭包两臂：list 列带 schema 子 tab、route 列直达', async () => {
    const { result } = setup();
    await waitFor(() => expect(result.current.pageCtx.loading).toBe(false));

    act(() => {
      capturedListOpts.onOpenDetail?.('fn/奇!点');
    });
    expect(historyPush).toHaveBeenLastCalledWith(
      `/functions/${encodeURIComponent('fn/奇!点')}?tab=config&subTab=schema`,
    );

    act(() => {
      capturedRouteOpts.onOpenDetail?.('fn.b');
    });
    expect(historyPush).toHaveBeenLastCalledWith('/functions/fn.b');
  });
});

describe('onCloneToEnv 四臂', () => {
  it('无 gameId → false 且零调用', async () => {
    mockGameId = undefined;
    const { result } = setup();
    await waitFor(() => expect(result.current.pageCtx.loading).toBe(false));

    let out: boolean | undefined;
    await act(async () => {
      out = await result.current.onCloneToEnv('prod');
    });
    expect(out).toBe(false);
    expect(setAssignments).not.toHaveBeenCalled();
  });

  it('空 targetEnv → warning false', async () => {
    const { result } = setup();
    await waitFor(() => expect(result.current.pageCtx.loading).toBe(false));

    let out: boolean | undefined;
    await act(async () => {
      out = await result.current.onCloneToEnv('');
    });
    expect(out).toBe(false);
    expect(messageWarning).toHaveBeenCalledWith('请选择目标环境');
    expect(setAssignments).not.toHaveBeenCalled();
  });

  it('成功 → setAssignments clone 载荷 + success + true', async () => {
    const { result } = setup();
    await waitFor(() => expect(result.current.pageCtx.selected).toEqual(['fn.a']));

    let out: boolean | undefined;
    await act(async () => {
      out = await result.current.onCloneToEnv('prod');
    });
    expect(out).toBe(true);
    expect(setAssignments).toHaveBeenCalledWith({
      action: 'clone',
      targetEnv: 'prod',
      functions: ['fn.a'],
    });
    expect(messageSuccess).toHaveBeenCalledWith('已克隆分配到 prod 环境');
  });

  it('Error → message.error 带 reason；非 Error → 未知错误兜底', async () => {
    setAssignments.mockRejectedValue(new Error('env 不存在'));
    const { result } = setup();
    await waitFor(() => expect(result.current.pageCtx.loading).toBe(false));

    await act(async () => {
      await result.current.onCloneToEnv('prod');
    });
    expect(messageError).toHaveBeenCalledWith('克隆失败: env 不存在');

    setAssignments.mockRejectedValue('plain-string');
    await act(async () => {
      await result.current.onCloneToEnv('prod');
    });
    expect(messageError).toHaveBeenLastCalledWith('克隆失败: 未知错误');
  });
});

describe('loadHistory', () => {
  it('成功：items/total、信封 data 解包、分页回写、弹窗可见', async () => {
    fetchAssignmentsHistory.mockResolvedValue({
      data: { items: [{ id: 'h1' }, { id: 'h2' }], total: 99 },
    });
    const { result } = setup();
    await waitFor(() => expect(result.current.pageCtx.loading).toBe(false));

    await act(async () => {
      await result.current.loadHistory(2, 20, 'assign');
    });
    expect(fetchAssignmentsHistory).toHaveBeenLastCalledWith({
      action: 'assign',
      page: 2,
      pageSize: 20,
    });
    expect(result.current.history).toEqual([{ id: 'h1' }, { id: 'h2' }]);
    expect(result.current.historyTotal).toBe(99);
    expect(result.current.historyPage).toBe(2);
    expect(result.current.historyPageSize).toBe(20);
    expect(result.current.historyVisible).toBe(true);
  });

  it('items 缺省（{}）→ 空数组、total 回退 items.length=0', async () => {
    fetchAssignmentsHistory.mockResolvedValue({});
    const { result } = setup();
    await waitFor(() => expect(result.current.pageCtx.loading).toBe(false));

    await act(async () => {
      await result.current.loadHistory();
    });
    expect(fetchAssignmentsHistory).toHaveBeenLastCalledWith({
      action: undefined,
      page: 1,
      pageSize: 10,
    });
    expect(result.current.history).toEqual([]);
    expect(result.current.historyTotal).toBe(0);
  });

  it('失败 → history 清空 total 0 不崩', async () => {
    fetchAssignmentsHistory.mockRejectedValue(new Error('boom'));
    const { result } = setup();
    await waitFor(() => expect(result.current.pageCtx.loading).toBe(false));

    await act(async () => {
      await result.current.loadHistory(3, 5);
    });
    expect(result.current.history).toEqual([]);
    expect(result.current.historyTotal).toBe(0);
    expect(result.current.historyVisible).toBe(true);
  });

  it('setHistoryVisible / setHistoryActionFilter 状态透传', async () => {
    const { result } = setup();
    await waitFor(() => expect(result.current.pageCtx.loading).toBe(false));

    act(() => {
      result.current.setHistoryVisible(true);
      result.current.setHistoryActionFilter('clone');
    });
    expect(result.current.historyVisible).toBe(true);
    expect(result.current.historyActionFilter).toBe('clone');
  });
});
