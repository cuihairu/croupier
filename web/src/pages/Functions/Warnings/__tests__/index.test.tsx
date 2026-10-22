/**
 * 函数注册告警页单测（覆盖率补缺轮：Functions/Warnings/index.tsx 328 行 0% → 收口）。
 *
 * 锁定契约：
 * - 挂载：URL 参数解析（function_id/agent_id/code/limit）→ 表单预填 +
 *   loadData({functionId,agentId,code,limit})，无参态四键 undefined/100；
 *   limit 非数字（Number('abc')=NaN）经 `NaN || 100` 回落 100；
 * - 查询：syncUrl 只写真值键（空串 Input 清除 → 键不进 URL）→ history.replace
 *   `${pathname}?${query}`；全空表单 → 裸 pathname（无 '?'）；随后 loadData；
 * - 刷新：仅 loadData(form.getFieldsValue())，不动 URL；
 * - 全部已读：markAllFunctionWarningsRead → 「已全部标为已读」toast +
 *   以当前表单条件重拉；
 * - 清空：Popconfirm 确认 → deleteAllFunctionWarnings → 「已清空」+ 重拉；
 * - 行内标为已读：仅未读行渲染按钮 → markFunctionWarningRead(key) → 本地
 *   setRows 翻转（不重拉列表）；已读行无按钮；
 * - 行内删除：Popconfirm 确认 → deleteFunctionWarning(key) → 本地过滤行
 *   （不重拉列表）；
 * - 列渲染：functionId/version/agentId 空串 '-' 兜底、code orange Tag +
 *   空串 '-'、count 原值、lastSeen formatDateTime('')='-' 兜底、message 列；
 * - scope 联动（#34 族）：scopeKey 变化 → 以当前表单条件重拉（URL 参数
 *   只在挂载时消费）；
 * - 挂载失败翼：listFunctionWarnings reject → effect 的 .catch(() => setRows([]))
 *   兜底空表不白屏。
 *
 * mock 口径：services/api/functions 五函数 jest.mock；useScopeReload 返回
 * 可变 mockScopeKey（rerender 驱动联动）；@umijs/max 本地 defaultMessage
 * mock（useLocation 读 mockSearch、history.replace 捕获断言）；pro-components
 * 仅换 PageContainer 桩。
 *
 * 坑实证（antd6/RTL 新档，两条）：
 * - RTL `within()` 返回查询 API 而非 DOM 元素——`within(pop).querySelector`
 *   直接 TypeError，querySelector 须落 HTMLElement 本体（Popconfirm 确认
 *   按钮锚 `pop.querySelector('.ant-btn-primary')`）；
 * - 行条件渲染按「字段存在性」计数：稀疏行无 read 字段 → `!record.read`
 *   走真翼、同样渲染「标为已读」——按钮计数须按夹具逐行推算，非只数
 *   显式置值的行。
 *
 * 现状锁定 / 边界（诚实清单，不造假用例不删防御分支）：
 * 1. loadData 的 `Array.isArray(res?.items)` 右翼（66 行）——service 归一层
 *    恒返 `{items: FunctionRegistrationWarning[]}`（缺省补 []），resolve {} /
 *    undefined 形态违反返回类型即造假，登记；lastSeen 列
 *    `formatDateTime(text ?? '')` 的 ?? 右翼（307 行）——列参类型 string，
 *    null/undefined 形态违反类型契约，空串左翼经 w3 稀疏行真实覆盖。
 * 2. loadData try/finally 无 catch——查询/刷新/全部已读/清空/行内动作的
 *    await loadData 或 await 动作函数 reject 均成 unhandled rejection
 *    （同族页面既有口径）；唯一 catch 翼（挂载 effect）以 reject 用例真实
 *    触达，其余不造假 reject 场景。
 * 3. markAll/deleteAll/markOne/deleteOne 动作自身 reject 同上（无 catch），
 *    登记。
 * 4. 行内动作的 setRows 更新器（prev.map/filter）箭头翼经行渲染锁定。
 */
import React from 'react';
import { configure, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { App } from 'antd';
import FunctionWarningsPage from '../index';
import {
  deleteAllFunctionWarnings,
  deleteFunctionWarning,
  listFunctionWarningFilterOptions,
  listFunctionWarnings,
  markAllFunctionWarningsRead,
  markFunctionWarningRead,
} from '@/services/api/functions';
import { formatDateTime } from '@/utils/format';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

jest.mock('@/services/api/functions', () => ({
  deleteAllFunctionWarnings: jest.fn(),
  deleteFunctionWarning: jest.fn(),
  listFunctionWarningFilterOptions: jest.fn(),
  listFunctionWarnings: jest.fn(),
  markAllFunctionWarningsRead: jest.fn(),
  markFunctionWarningRead: jest.fn(),
}));

// scopeKey 可变：rerender 驱动 #34 族联动重拉。
// ServerOptionsSelect 内部调用 useScopeReload，mock 模块必须带上它。
const mockScope = { scope: { gameId: '', env: '' }, scopeKey: '::' };
jest.mock('@/hooks/useScopeReload', () => ({
  useScope: () => mockScope,
  useScopeReload: () => jest.fn(),
}));

const mockReplace = jest.fn();
let mockSearch = '';
jest.mock('@umijs/max', () => ({
  FormattedMessage: ({ defaultMessage }: { defaultMessage?: string }) => (
    <>{defaultMessage ?? ''}</>
  ),
  useIntl: () => ({ formatMessage: (o: { defaultMessage?: string }) => o.defaultMessage ?? '' }),
  useLocation: () => ({ search: mockSearch, pathname: '/functions/warnings' }),
  history: { replace: (...args: unknown[]) => mockReplace(...args) },
}));

jest.mock('@ant-design/pro-components', () => ({
  PageContainer: ({ children }: { children?: React.ReactNode }) => <div>{children}</div>,
}));

const mList = listFunctionWarnings as jest.MockedFunction<typeof listFunctionWarnings>;
const mFilterOptions = listFunctionWarningFilterOptions as jest.MockedFunction<
  typeof listFunctionWarningFilterOptions
>;
const mMarkAll = markAllFunctionWarningsRead as jest.MockedFunction<
  typeof markAllFunctionWarningsRead
>;
const mMarkOne = markFunctionWarningRead as jest.MockedFunction<typeof markFunctionWarningRead>;
const mDelAll = deleteAllFunctionWarnings as jest.MockedFunction<typeof deleteAllFunctionWarnings>;
const mDelOne = deleteFunctionWarning as jest.MockedFunction<typeof deleteFunctionWarning>;

// 三行：满字段未读 / 已读（无标为已读按钮）/ 稀疏行（空串兜底翼全家）
const ROWS = [
  {
    key: 'w1',
    gameId: 'demo',
    env: 'prod',
    agentId: 'agent-1',
    functionId: 'examples.player.create',
    version: '1.2.0',
    code: 'invalid_version',
    message: 'version is not semver',
    count: 3,
    firstSeen: '2026-09-01T00:00:00Z',
    lastSeen: '2026-09-02T10:00:00Z',
  },
  {
    key: 'w2',
    gameId: '',
    env: '',
    agentId: 'agent-2',
    functionId: 'dup.fn',
    version: '2.0.0',
    code: 'duplicate_function',
    message: 'duplicate registered',
    count: 1,
    firstSeen: '',
    lastSeen: '2026-09-03T10:00:00Z',
    read: true,
  },
  {
    key: 'w3',
    gameId: '',
    env: '',
    agentId: '',
    functionId: '',
    version: '',
    code: '',
    message: 'sparse row',
    count: 0,
    firstSeen: '',
    lastSeen: '',
  },
];

function renderPage() {
  return render(
    <App>
      <FunctionWarningsPage />
    </App>,
  );
}

// #34：函数ID/AgentID 已从 Input 换成 ServerOptionsSelect（antd6 Select）——
// placeholder 文本与表格单元格撞车（不能 getByText），经 .ant-select-placeholder 定位。
function selectPlaceholder(text: string): HTMLElement {
  const hit = Array.from(document.querySelectorAll<HTMLElement>('.ant-select-placeholder')).find(
    (el) => el.textContent === text,
  );
  expect(hit).toBeTruthy();
  return hit as HTMLElement;
}

/** 打开指定 placeholder 的过滤下拉并点选展示文本匹配的选项（count 后缀保证与表格文本不撞）。 */
async function chooseFilterOption(placeholder: string, optionText: string): Promise<void> {
  fireEvent.mouseDown(selectPlaceholder(placeholder));
  const dropdownOptions = () =>
    Array.from(
      document.querySelectorAll('.ant-select-dropdown .ant-select-item-option'),
    ) as HTMLElement[];
  await waitFor(() => {
    expect(dropdownOptions().some((el) => el.textContent === optionText)).toBe(true);
  });
  fireEvent.click(dropdownOptions().find((el) => el.textContent === optionText) as Element);
}

beforeEach(() => {
  jest.clearAllMocks();
  mockSearch = '';
  mockScope.scopeKey = '::';
  mList.mockResolvedValue({ items: ROWS });
  mFilterOptions.mockResolvedValue({
    functions: [
      { value: 'examples.player.create', count: 1 },
      { value: 'dup.fn', count: 1 },
    ],
    agents: [
      { value: 'agent-1', count: 1 },
      { value: 'agent-2', count: 1 },
    ],
  });
  mMarkAll.mockResolvedValue(undefined);
  mMarkOne.mockResolvedValue(undefined);
  mDelAll.mockResolvedValue(undefined);
  mDelOne.mockResolvedValue(undefined);
});

describe('函数注册告警 挂载与列表', () => {
  it('无参挂载：默认载荷四键缺省 + 列渲染矩阵 + 已读行无标为已读按钮', async () => {
    renderPage();
    await waitFor(() =>
      expect(mList).toHaveBeenCalledWith({
        functionId: undefined,
        agentId: undefined,
        code: undefined,
        limit: 100,
      }),
    );

    const table = document.querySelector('.ant-table') as HTMLElement;
    expect(within(table).getByText('examples.player.create')).toBeInTheDocument();
    expect(within(table).getByText('invalid_version').closest('.ant-tag')).toHaveClass(
      'ant-tag-orange',
    );
    expect(within(table).getByText('1.2.0')).toBeInTheDocument();
    expect(within(table).getByText('3')).toBeInTheDocument();
    expect(
      within(table).getAllByText(formatDateTime('2026-09-02T10:00:00Z')).length,
    ).toBeGreaterThan(0);
    expect(within(table).getByText('agent-1')).toBeInTheDocument();
    expect(within(table).getByText('version is not semver')).toBeInTheDocument();

    // 稀疏行空串兜底：functionId/version/agentId '-'、code Tag '-'、lastSeen '-'
    // （与 ProTable 无关的 '-' 单元格可能多命中，稀疏行按计数断言）
    expect(within(table).getAllByText('-').length).toBeGreaterThanOrEqual(5);

    // 未读行（w1 + 稀疏行 w3）有「标为已读」、已读行（w2）没有——恰 2 个
    expect(within(table).getAllByRole('button', { name: /标为已读/ })).toHaveLength(2);
    // 每行都有删除按钮
    expect(within(table).getAllByRole('button', { name: /删\s*除/ })).toHaveLength(3);
  });

  it('深链挂载：四参解析预填表单 + 载荷透传；limit 非数字回落 100', async () => {
    mockSearch =
      '?function_id=examples.player.create&agent_id=agent-1&code=invalid_version&limit=50';
    renderPage();
    await waitFor(() =>
      expect(mList).toHaveBeenCalledWith({
        functionId: 'examples.player.create',
        agentId: 'agent-1',
        code: 'invalid_version',
        limit: 50,
      }),
    );
    // Select 无 input placeholder 属性，预填值经选择器展示文本断言
    // （选项载入后 ServerOptionsSelect 渲染为「value (count)」形态）
    const selectTexts = Array.from(document.querySelectorAll('.ant-select')).map(
      (el) => el.textContent,
    );
    expect(selectTexts).toContain('examples.player.create (1)');
    expect(selectTexts).toContain('agent-1 (1)');
    expect((screen.getByPlaceholderText('invalid_version') as HTMLInputElement).value).toBe(
      'invalid_version',
    );
    expect((screen.getByRole('spinbutton') as HTMLInputElement).value).toBe('50');
  });

  it('limit 非数字深链：Number(NaN) || 100 回落', async () => {
    mockSearch = '?limit=abc';
    renderPage();
    await waitFor(() =>
      expect(mList).toHaveBeenLastCalledWith({
        functionId: undefined,
        agentId: undefined,
        code: undefined,
        limit: 100,
      }),
    );
  });

  it('挂载失败翼：reject → .catch 兜底空表，不白屏', async () => {
    mList.mockRejectedValueOnce(new Error('down'));
    renderPage();
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(1));
    const table = document.querySelector('.ant-table') as HTMLElement;
    await waitFor(() =>
      expect(within(table).queryByText('examples.player.create')).not.toBeInTheDocument(),
    );
    expect(screen.getByText('规则说明')).toBeInTheDocument();
  });
});

describe('函数注册告警 查询与 URL 同步', () => {
  it('查询主链：四键全填 → replace 带全参 URL + loadData 透传', async () => {
    renderPage();
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(1));

    await chooseFilterOption('examples.player.create', 'examples.player.create (1)');
    await chooseFilterOption('agent-1', 'agent-1 (1)');
    fireEvent.change(screen.getByPlaceholderText('invalid_version'), {
      target: { value: 'duplicate_function' },
    });
    fireEvent.change(screen.getByRole('spinbutton'), { target: { value: '20' } });
    fireEvent.click(screen.getByRole('button', { name: /查\s*询/ }));

    await waitFor(() =>
      expect(mockReplace).toHaveBeenCalledWith(
        '/functions/warnings?function_id=examples.player.create&agent_id=agent-1&code=duplicate_function&limit=20',
      ),
    );
    await waitFor(() =>
      expect(mList).toHaveBeenLastCalledWith({
        functionId: 'examples.player.create',
        agentId: 'agent-1',
        code: 'duplicate_function',
        limit: 20,
      }),
    );
  });

  it('空值键不进 URL：清除输入 → 全空表单 replace 裸 pathname（无 ?）', async () => {
    mockSearch = '?function_id=stale.fn&limit=7';
    renderPage();
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(1));

    // 清空预填的函数过滤（antd6 清除手柄 span.ant-select-clear，悬停后出现）+ limit 清空
    const fnSelect = Array.from(document.querySelectorAll('.ant-select')).find(
      (el) => el.textContent === 'stale.fn',
    );
    expect(fnSelect).toBeTruthy();
    fireEvent.mouseEnter(fnSelect as Element);
    const clearBtn = await waitFor(() => {
      const el = (fnSelect as Element).querySelector('.ant-select-clear');
      expect(el).toBeTruthy();
      return el as Element;
    });
    fireEvent.click(clearBtn);
    fireEvent.change(screen.getByRole('spinbutton'), { target: { value: '' } });
    fireEvent.click(screen.getByRole('button', { name: /查\s*询/ }));

    await waitFor(() => expect(mockReplace).toHaveBeenCalledWith('/functions/warnings'));
    // 空串/undefined 全部 || 兜底：undefined / 100
    await waitFor(() =>
      expect(mList).toHaveBeenLastCalledWith({
        functionId: undefined,
        agentId: undefined,
        code: undefined,
        limit: 100,
      }),
    );
  });

  it('刷新：仅重拉不动 URL', async () => {
    renderPage();
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(1));

    fireEvent.click(screen.getByRole('button', { name: /刷\s*新/ }));
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(2));
    expect(mockReplace).not.toHaveBeenCalled();
  });
});

describe('函数注册告警 行内与批量动作', () => {
  it('行内标为已读：markOne(key) + 本地翻转不重拉；已读按钮随之消失', async () => {
    renderPage();
    const table = document.querySelector('.ant-table') as HTMLElement;
    const row = (await within(table).findByText('examples.player.create')).closest(
      'tr',
    ) as HTMLElement;
    const btn = within(row).getByRole('button', { name: /标为已读/ });

    fireEvent.click(btn);
    await waitFor(() => expect(mMarkOne).toHaveBeenCalledWith('w1'));
    // 本地翻转：w1 行按钮消失（w3 稀疏行仍未读、按钮保留），列表不重拉
    await waitFor(() =>
      expect(within(row).queryByRole('button', { name: /标为已读/ })).not.toBeInTheDocument(),
    );
    expect(mList).toHaveBeenCalledTimes(1);
    expect(within(table).getByText('examples.player.create')).toBeInTheDocument();
    expect(within(table).getAllByRole('button', { name: /标为已读/ })).toHaveLength(1);
  });

  it('行内删除：Popconfirm 确认 → deleteOne(key) + 本地行消失不重拉', async () => {
    renderPage();
    const table = document.querySelector('.ant-table') as HTMLElement;
    const row = (await within(table).findByText('dup.fn')).closest('tr') as HTMLElement;

    fireEvent.click(within(row).getByRole('button', { name: /删\s*除/ }));
    const confirm = await waitFor(() => {
      const pop = Array.from(document.querySelectorAll('.ant-popover')).find(
        (p) => !p.className.includes('ant-popover-hidden'),
      ) as HTMLElement;
      expect(pop).not.toBeUndefined();
      return pop.querySelector('.ant-btn-primary') as HTMLElement;
    });
    fireEvent.click(confirm);

    await waitFor(() => expect(mDelOne).toHaveBeenCalledWith('w2'));
    await waitFor(() => expect(within(table).queryByText('dup.fn')).not.toBeInTheDocument());
    expect(mList).toHaveBeenCalledTimes(1);
  });

  it('全部已读：markAll → toast 已全部标为已读 + 以当前表单条件重拉', async () => {
    renderPage();
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(1));
    // 预置一个筛选值，断言重拉沿用表单条件
    fireEvent.change(screen.getByPlaceholderText('invalid_version'), {
      target: { value: 'invalid_version' },
    });

    fireEvent.click(screen.getByRole('button', { name: /全部已读/ }));
    await waitFor(() => expect(mMarkAll).toHaveBeenCalledTimes(1));
    expect(await screen.findByText('已全部标为已读')).toBeInTheDocument();
    await waitFor(() =>
      expect(mList).toHaveBeenLastCalledWith({
        functionId: undefined,
        agentId: undefined,
        code: 'invalid_version',
        limit: 100,
      }),
    );
  });

  it('清空：Popconfirm 确认 → deleteAll → toast 已清空 + 重拉', async () => {
    renderPage();
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(1));

    fireEvent.click(screen.getByRole('button', { name: /清\s*空/ }));
    const confirm = await waitFor(() => {
      const pop = Array.from(document.querySelectorAll('.ant-popover')).find(
        (p) => !p.className.includes('ant-popover-hidden'),
      ) as HTMLElement;
      expect(pop).not.toBeUndefined();
      return pop.querySelector('.ant-btn-primary') as HTMLElement;
    });
    fireEvent.click(confirm);

    await waitFor(() => expect(mDelAll).toHaveBeenCalledTimes(1));
    expect(await screen.findByText('已清空')).toBeInTheDocument();
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(2));
  });
});

describe('函数注册告警 scope 联动（#34 族）', () => {
  it('scopeKey 变化 → 以当前表单条件重拉（URL 参数不重复消费）', async () => {
    mockSearch = '?code=invalid_version';
    const { rerender } = renderPage();
    await waitFor(() =>
      expect(mList).toHaveBeenLastCalledWith({
        functionId: undefined,
        agentId: undefined,
        code: 'invalid_version',
        limit: 100,
      }),
    );

    mockScope.scopeKey = 'demo:prod';
    rerender(
      <App>
        <FunctionWarningsPage />
      </App>,
    );
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(2));
    expect(mList).toHaveBeenLastCalledWith({
      functionId: undefined,
      agentId: undefined,
      code: 'invalid_version',
      limit: 100,
    });
  });
});
