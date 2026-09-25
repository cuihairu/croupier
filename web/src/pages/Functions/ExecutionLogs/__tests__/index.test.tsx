/**
 * 执行留痕页面测试（对齐 StandardPage 规范）：
 * 1. 结构：概览说明/推荐路径、筛选栏结果计数、清空筛选与已生效条件提示、
 *    空态二分（筛选无结果 vs 全空引导）、加载失败错误 Alert 与重试。
 * 2. 筛选扩展：更多筛选展开/收起、Trace ID、时间范围（起止/仅起始/清空）
 *    进入请求参数与摘要；函数ID/来源/状态摘要分支。
 * 3. 列渲染：页面/调用来源、成功/失败状态、页面 Key、空操作人占位。
 * 4. 详情抽屉：完整字段与脱敏载荷、失败原因提取（对象/字符串/非字符串）、
 *    截断提示、页面来源、加载失败 toast、行点击与抽屉关闭。
 * ProTable 用行为桩（params 变化重查、onRow 透传、locale.emptyText 空态），
 * RangePicker 用受控桩（jsdom 面板交互不可靠），参照 OperationLogs.test.tsx 先例。
 */
import React from 'react';
import { App as AntdApp } from 'antd';
import { configure, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import ExecutionLogsPage from '../index';
import {
  getExecutionLog,
  listExecutionLogs,
  type ExecutionLogDetail,
  type ExecutionLogItem,
} from '@/services/api/executionLogs';
import { formatDateTime } from '@/utils/format';

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
  // 稳定引用：组件树内任何 effect 依赖 intl 都不能因新对象无限重建
  const intl = { formatMessage };
  return {
    __esModule: true,
    useIntl: () => intl,
    FormattedMessage: ({ defaultMessage }: { defaultMessage: string }) => defaultMessage,
  };
});

// RangePicker 在 jsdom 里带 showTime 的面板交互极不可靠，用受控桩替换：
// 按钮分别触发 onChange 的起止完整 / 仅起始 / 清空三种取值形态
jest.mock('antd', () => {
  const actual = jest.requireActual('antd');
  type StubDay = { toDate: () => Date };
  type RangeChange = (dates: [StubDay | null, StubDay | null] | null) => void;
  const asDay = (iso: string): StubDay => ({ toDate: () => new Date(iso) });
  const RangePickerStub = (props: { onChange?: RangeChange }) => (
    <div data-testid="range-stub">
      <button
        data-testid="range-full"
        onClick={() =>
          props.onChange?.([asDay('2026-09-01T08:09:10Z'), asDay('2026-09-02T09:30:00Z')])
        }
      />
      <button
        data-testid="range-start-only"
        onClick={() => props.onChange?.([asDay('2026-09-01T08:09:10Z'), null])}
      />
      <button data-testid="range-clear" onClick={() => props.onChange?.(null)} />
    </div>
  );
  return { ...actual, DatePicker: { RangePicker: RangePickerStub } };
});

jest.mock('@/services/api/executionLogs', () => ({
  listExecutionLogs: jest.fn(),
  getExecutionLog: jest.fn(),
}));

// ProTable 行为桩：挂载与 params 变化（筛选状态驱动）均以
// {current,pageSize,...params} 触发一次 request；actionRef 提供可用的
// reload（重试/查询/刷新按钮走它）；onRow 透传到行容器（行点击开详情）；
// 空数据时透传 locale.emptyText 渲染，支撑空态二分断言。
jest.mock('@ant-design/pro-components', () => {
  const React = require('react') as typeof import('react');
  const { useState, useRef, useEffect } = React;

  type StubColumn = {
    title?: React.ReactNode;
    dataIndex?: string;
    render?: (value: unknown, record: unknown, index: number) => React.ReactNode;
  };
  type StubRequestResult = { data?: unknown[]; total?: number; success?: boolean };
  type StubRowProps = { onClick?: () => void; style?: React.CSSProperties };
  type StubProps = {
    actionRef?: { current?: Record<string, unknown> | undefined };
    rowKey?: string;
    columns?: StubColumn[];
    params?: Record<string, unknown>;
    locale?: { emptyText?: React.ReactNode };
    request?: (args: Record<string, unknown>) => Promise<StubRequestResult>;
    onRow?: (record: unknown) => StubRowProps;
  };

  const ProTableStub: React.FC<StubProps> = (props) => {
    const [rows, setRows] = useState<unknown[]>([]);
    const lastArgsRef = useRef<Record<string, unknown>>({});

    const fire = (args: Record<string, unknown>) => {
      lastArgsRef.current = args;
      void props.request?.(args).then((res: StubRequestResult) => {
        setRows(res?.data || []);
      });
    };

    // 挂载首查 + params 变化重查（模拟真实 ProTable 对 params 的响应）
    const paramsKey = JSON.stringify(props.params ?? {});
    useEffect(() => {
      fire({ current: 1, pageSize: 20, ...(props.params || {}) });
      // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [paramsKey]);

    if (props.actionRef) {
      props.actionRef.current = {
        reload: () => fire(lastArgsRef.current),
        setPageInfo: () => undefined,
      };
    }

    return (
      <div data-testid="protable-stub">
        {rows.map((row, i: number) => (
          <div data-testid="stub-row" key={i} {...(props.onRow?.(row) ?? {})}>
            {(props.columns || []).map((col: StubColumn, ci: number) => (
              <span data-testid="stub-cell" key={ci}>
                {col.render
                  ? col.render(
                      col.dataIndex ? (row as Record<string, unknown>)[col.dataIndex] : undefined,
                      row,
                      i,
                    )
                  : null}
              </span>
            ))}
          </div>
        ))}
        {rows.length === 0 ? <div data-testid="stub-empty">{props.locale?.emptyText}</div> : null}
      </div>
    );
  };

  const PageContainerStub: React.FC<{
    title?: React.ReactNode;
    subTitle?: React.ReactNode;
    children?: React.ReactNode;
  }> = ({ title, subTitle, children }) => (
    <div>
      <div data-testid="page-header">
        {title}
        {subTitle ? <span data-testid="page-subtitle">{subTitle}</span> : null}
      </div>
      {children}
    </div>
  );

  return { __esModule: true, ProTable: ProTableStub, PageContainer: PageContainerStub };
});

const mockedList = jest.mocked(listExecutionLogs);
const mockedGetDetail = jest.mocked(getExecutionLog);

const sampleItem = {
  id: 1,
  actor: 'admin',
  functionId: 'player.kick',
  pageKey: '',
  gameId: 'demo',
  env: 'prod',
  source: 'invoke',
  status: 'ok',
  durationMs: 12,
  createdAt: '2026-09-01T10:00:00Z',
};

function makeItem(over: Partial<ExecutionLogItem> = {}): ExecutionLogItem {
  return {
    id: 1,
    actor: 'admin',
    functionId: 'player.kick',
    gameId: 'demo',
    env: 'prod',
    source: 'invoke',
    status: 'ok',
    durationMs: 12,
    createdAt: '2026-09-01T10:00:00Z',
    ...over,
  };
}

function listResponse(
  items: ExecutionLogItem[],
  total = items.length,
): { items: ExecutionLogItem[]; total: number; page: number; size: number } {
  return { items, total, page: 1, size: 20 };
}

/** 打开第 comboIndex 个 Select 下拉并点选 optionText（waitFor 内完成检索以容忍关旧开新的过渡帧） */
async function chooseSelectOption(comboIndex: number, optionText: string): Promise<void> {
  fireEvent.mouseDown(screen.getAllByRole('combobox')[comboIndex]);
  const option = await waitFor(() => {
    const dropdown = document.querySelector(
      '.ant-select-dropdown:not(.ant-select-dropdown-hidden)',
    );
    expect(dropdown).toBeTruthy();
    const hit = Array.from(
      (dropdown as HTMLElement).querySelectorAll<HTMLElement>('.ant-select-item-option'),
    ).find((o) => o.textContent === optionText);
    expect(hit).toBeTruthy();
    return hit as HTMLElement;
  });
  fireEvent.click(option);
}

/** 与页面 toLocalInput 同语义的本地时间格式（避免依赖机器时区的字面量断言） */
function localStamp(value: Date): string {
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${value.getFullYear()}-${pad(value.getMonth() + 1)}-${pad(value.getDate())} ${pad(
    value.getHours(),
  )}:${pad(value.getMinutes())}:${pad(value.getSeconds())}`;
}

function renderPage() {
  return render(
    <AntdApp>
      <ExecutionLogsPage />
    </AntdApp>,
  );
}

function lastListArgs(): Record<string, string | number | boolean | undefined> {
  const calls = mockedList.mock.calls;
  return calls[calls.length - 1][0];
}

describe('执行留痕页面结构（StandardPage 对齐）', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockedList.mockResolvedValue({ items: [sampleItem], total: 1 } as never);
    mockedGetDetail.mockReset();
  });

  it('页头副标题、概览说明与推荐路径渲染', async () => {
    renderPage();
    expect(screen.getByTestId('page-subtitle')).toHaveTextContent(
      '按操作人、函数、来源、状态与时间范围排查函数执行记录',
    );
    expect(await screen.findByText('执行留痕概览')).toBeInTheDocument();
    expect(
      screen.getByText(/集中查询函数执行的留痕：谁通过调用或页面触发了哪个函数/),
    ).toBeInTheDocument();
    expect(screen.getByText(/推荐路径：先按操作人或状态筛选定位可疑执行/)).toBeInTheDocument();
    expect(mockedList).toHaveBeenCalledTimes(1);
  });

  it('request 成功后更新结果计数与概览记录总数', async () => {
    mockedList.mockResolvedValue({ items: [sampleItem], total: 7 } as never);
    renderPage();
    expect(await screen.findByText('当前结果 7 条')).toBeInTheDocument();
    expect(screen.getByText('记录总数 7')).toBeInTheDocument();
  });

  it('筛选生效后出现清空筛选与已生效条件提示，清空后二者消失', async () => {
    renderPage();
    await screen.findByText('当前结果 1 条');
    expect(screen.queryByText('清空筛选')).not.toBeInTheDocument();

    fireEvent.change(screen.getByPlaceholderText('操作人'), { target: { value: 'alice' } });
    expect(screen.getByText('清空筛选')).toBeInTheDocument();
    expect(screen.getByText('当前正在查看筛选后的执行记录')).toBeInTheDocument();
    expect(screen.getByText('已生效条件：操作人 alice')).toBeInTheDocument();

    fireEvent.click(screen.getByText('清空筛选'));
    await waitFor(() => {
      expect(screen.queryByText('清空筛选')).not.toBeInTheDocument();
    });
    expect(screen.queryByText('已生效条件：操作人 alice')).not.toBeInTheDocument();
  });

  it('空态二分：无筛选提示引导、有筛选提示调整条件', async () => {
    mockedList.mockResolvedValue({ items: [], total: 0 } as never);
    renderPage();
    expect(await screen.findByTestId('stub-empty')).toHaveTextContent(
      '暂无执行留痕。函数被调用或页面发起执行后',
    );

    fireEvent.change(screen.getByPlaceholderText('操作人'), { target: { value: 'ghost' } });
    await waitFor(() => {
      expect(screen.getByTestId('stub-empty')).toHaveTextContent(
        '当前筛选条件下没有匹配的执行记录',
      );
    });
  });

  it('加载失败显示错误 Alert 与重试，重试再次拉取', async () => {
    mockedList.mockRejectedValueOnce(new Error('boom'));
    renderPage();
    expect(await screen.findByText('boom')).toBeInTheDocument();
    expect(screen.getByText('加载失败')).toBeInTheDocument();

    mockedList.mockClear();
    mockedList.mockResolvedValue({ items: [sampleItem], total: 1 } as never);
    // antd zh-CN 对双字按钮自动插入空格（「重 试」），正则容忍空白
    fireEvent.click(screen.getByRole('button', { name: /重\s*试/ }));
    await waitFor(() => {
      expect(mockedList).toHaveBeenCalledTimes(1);
    });
  });
});

describe('执行留痕筛选扩展与详情抽屉', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockedList.mockResolvedValue(listResponse([makeItem()]));
    mockedGetDetail.mockReset();
  });

  it('更多筛选：展开后 Trace ID 进入请求与摘要，收起后隐藏但保留条件', async () => {
    renderPage();
    await screen.findByText('当前结果 1 条');
    expect(screen.queryByPlaceholderText('Trace ID')).not.toBeInTheDocument();
    expect(screen.queryByTestId('range-stub')).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /更多筛选/ }));
    const traceInput = await screen.findByPlaceholderText('Trace ID');
    expect(screen.getByTestId('range-stub')).toBeInTheDocument();

    fireEvent.change(traceInput, { target: { value: 'tr-abc' } });
    await waitFor(() =>
      expect(mockedList).toHaveBeenLastCalledWith(expect.objectContaining({ traceId: 'tr-abc' })),
    );
    expect(screen.getByText('已生效条件：Trace tr-abc')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /收起筛选/ }));
    await waitFor(() => {
      expect(screen.queryByPlaceholderText('Trace ID')).not.toBeInTheDocument();
    });
    expect(screen.queryByTestId('range-stub')).not.toBeInTheDocument();
    // 收起只是隐藏控件，已生效条件保留
    expect(screen.getByText('已生效条件：Trace tr-abc')).toBeInTheDocument();
  });

  it('筛选摘要：函数ID/来源/状态聚合进已生效条件并透传请求', async () => {
    renderPage();
    await screen.findByText('当前结果 1 条');

    fireEvent.change(screen.getByPlaceholderText('函数ID'), { target: { value: 'player.ban' } });
    await chooseSelectOption(0, '页面');
    await chooseSelectOption(1, '失败');
    await waitFor(() => {
      expect(screen.getByText('已生效条件：函数 player.ban / page / error')).toBeInTheDocument();
    });
    expect(mockedList).toHaveBeenLastCalledWith(
      expect.objectContaining({ functionId: 'player.ban', source: 'page', status: 'error' }),
    );

    // 切换来源/状态覆盖摘要与请求参数的另一侧分支
    await chooseSelectOption(0, '调用');
    await chooseSelectOption(1, '成功');
    await waitFor(() => {
      expect(screen.getByText('已生效条件：函数 player.ban / invoke / ok')).toBeInTheDocument();
    });
    expect(mockedList).toHaveBeenLastCalledWith(
      expect.objectContaining({ source: 'invoke', status: 'ok' }),
    );
  });

  it('时间范围：起止完整进入 from/to，仅起始省略 to，清空后全部移除', async () => {
    renderPage();
    await screen.findByText('当前结果 1 条');
    fireEvent.click(screen.getByRole('button', { name: /更多筛选/ }));
    await screen.findByTestId('range-stub');

    const start = new Date('2026-09-01T08:09:10Z');
    const end = new Date('2026-09-02T09:30:00Z');
    fireEvent.click(screen.getByTestId('range-full'));
    await waitFor(() =>
      expect(mockedList).toHaveBeenLastCalledWith(
        expect.objectContaining({ from: localStamp(start), to: localStamp(end) }),
      ),
    );
    expect(
      screen.getByText(
        `已生效条件：时间 ${formatDateTime(start.toISOString())} ~ ${formatDateTime(
          end.toISOString(),
        )}`,
      ),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByTestId('range-start-only'));
    await waitFor(() =>
      expect(mockedList).toHaveBeenLastCalledWith(
        expect.objectContaining({ from: localStamp(start) }),
      ),
    );
    expect(lastListArgs()).not.toHaveProperty('to');
    expect(
      screen.getByText(`已生效条件：时间 ${formatDateTime(start.toISOString())} ~ …`),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: '清空筛选' }));
    await waitFor(() => {
      expect(lastListArgs()).not.toHaveProperty('from');
    });
    expect(screen.queryByText(/已生效条件：/)).not.toBeInTheDocument();
  });

  it('列表列渲染：页面来源/失败状态/页面Key/空操作人占位', async () => {
    const failedPage = makeItem({
      id: 2,
      actor: '',
      functionId: 'mail.send',
      pageKey: 'home.dash',
      gameId: 'demo',
      env: 'dev',
      source: 'page',
      status: 'error',
      durationMs: 30,
    });
    mockedList.mockResolvedValue(listResponse([makeItem({ pageKey: '' }), failedPage]));
    renderPage();
    const rows = await screen.findAllByTestId('stub-row');
    expect(rows).toHaveLength(2);

    const okRow = within(rows[0]);
    expect(okRow.getByText('成功')).toBeInTheDocument();
    expect(okRow.getByText('调用')).toBeInTheDocument();
    expect(okRow.getByText('player.kick')).toBeInTheDocument();
    expect(okRow.getByText('admin')).toBeInTheDocument();
    expect(okRow.getByText('-')).toBeInTheDocument(); // pageKey 缺省占位
    expect(okRow.getByText('demo/prod')).toBeInTheDocument();

    const failRow = within(rows[1]);
    expect(failRow.getByText('失败')).toBeInTheDocument();
    expect(failRow.getByText('页面')).toBeInTheDocument();
    expect(failRow.getByText('home.dash')).toBeInTheDocument();
    expect(failRow.getByText('-')).toBeInTheDocument(); // 空操作人占位
    expect(failRow.getByText('demo/dev')).toBeInTheDocument();
  });

  it('点击详情打开抽屉：完整字段、脱敏载荷与关闭', async () => {
    const detail: ExecutionLogDetail = {
      ...makeItem({ traceId: 'tr-9' }),
      requestPayload: { playerId: 'p1' },
      responseBody: { ok: true },
    };
    mockedGetDetail.mockResolvedValue(detail);
    renderPage();
    const rows = await screen.findAllByTestId('stub-row');
    fireEvent.click(within(rows[0]).getByRole('button', { name: /详\s*情/ }));

    expect(await screen.findByText('执行留痕 #1')).toBeInTheDocument();
    expect(mockedGetDetail).toHaveBeenCalledWith(1);
    const drawer = document.querySelector('.ant-drawer-open') as HTMLElement;
    expect(within(drawer).getByText('admin')).toBeInTheDocument();
    expect(within(drawer).getByText('成功')).toBeInTheDocument();
    expect(within(drawer).getByText('player.kick')).toBeInTheDocument();
    expect(within(drawer).getByText('demo/prod')).toBeInTheDocument();
    expect(within(drawer).getByText('调用')).toBeInTheDocument();
    expect(within(drawer).getByText('tr-9')).toBeInTheDocument();
    expect(within(drawer).getByText(/· 12ms/)).toBeInTheDocument();
    expect(within(drawer).getByText(/"playerId": "p1"/)).toBeInTheDocument();
    expect(within(drawer).getByText(/"ok": true/)).toBeInTheDocument();
    expect(within(drawer).queryByText('失败原因')).not.toBeInTheDocument();

    fireEvent.click(drawer.querySelector('.ant-drawer-close') as HTMLElement);
    await waitFor(() => expect(document.querySelector('.ant-drawer-open')).toBeNull());
  });

  it('失败详情：失败原因提取、截断/页面来源与非字符串错误兜底', async () => {
    const errObject = makeItem({
      id: 1,
      actor: '',
      functionId: 'mail.send',
      pageKey: 'home.dash',
      bindingId: 'bind-1',
      source: 'page',
      status: 'error',
      durationMs: 5,
      traceId: '',
      truncated: true,
      responseBody: { error: 'quota exceeded' },
    });
    const errPlainString = makeItem({
      id: 2,
      actor: 'bob',
      functionId: 'player.ban',
      source: 'invoke',
      status: 'error',
      durationMs: 7,
      traceId: 'tr-2',
      responseBody: 'plain-text failure',
    });
    const errNonString = makeItem({
      id: 3,
      actor: 'bob',
      functionId: 'player.ban',
      source: 'invoke',
      status: 'error',
      durationMs: 9,
      traceId: 'tr-3',
      responseBody: { error: 123 },
    });
    mockedList.mockResolvedValue(listResponse([errObject, errPlainString, errNonString]));
    // 按 id 分派详情：三种失败形态各走一次
    mockedGetDetail.mockImplementation((id: number) =>
      Promise.resolve(
        (id === 1 ? errObject : id === 2 ? errPlainString : errNonString) as ExecutionLogDetail,
      ),
    );
    renderPage();
    const rows = await screen.findAllByTestId('stub-row');
    expect(rows).toHaveLength(3);

    fireEvent.click(within(rows[0]).getByRole('button', { name: /详\s*情/ }));
    expect(await screen.findByText('页面（home.dash / bind-1）')).toBeInTheDocument();
    let drawer = document.querySelector('.ant-drawer-open') as HTMLElement;
    expect(within(drawer).getByText('失败原因')).toBeInTheDocument();
    expect(within(drawer).getAllByText(/quota exceeded/).length).toBeGreaterThanOrEqual(2);
    expect(within(drawer).getByText('（载荷已截断）')).toBeInTheDocument();
    expect(within(drawer).getAllByText('（无）')).toHaveLength(1); // 仅请求载荷为空
    expect(within(drawer).getAllByText('-')).toHaveLength(2); // 空操作人 + 空 Trace
    expect(within(drawer).getByText(/· 5ms/)).toBeInTheDocument();

    // responseBody 为裸字符串：失败原因直接取字符串本身
    fireEvent.click(within(rows[1]).getByRole('button', { name: /详\s*情/ }));
    expect(await screen.findByText('plain-text failure')).toBeInTheDocument();
    drawer = document.querySelector('.ant-drawer-open') as HTMLElement;
    expect(within(drawer).getByText('失败原因')).toBeInTheDocument();
    expect(within(drawer).getByText('tr-2')).toBeInTheDocument();

    // responseBody.error 非字符串：不提取失败原因，仅展示响应载荷
    fireEvent.click(within(rows[2]).getByRole('button', { name: /详\s*情/ }));
    expect(await screen.findByText(/"error": 123/)).toBeInTheDocument();
    drawer = document.querySelector('.ant-drawer-open') as HTMLElement;
    expect(within(drawer).queryByText('失败原因')).not.toBeInTheDocument();
  });

  it('详情加载失败：Error 与非 Error 均提示错误且不开抽屉', async () => {
    mockedGetDetail.mockRejectedValue(new Error('detail boom'));
    renderPage();
    const rows = await screen.findAllByTestId('stub-row');
    // 用行点击（单次触发 viewDetail）：按钮点击会冒泡到 onRow 产生双 toast，
    // 而 findByText 的单数 getter 在多匹配时抛 multiple 错误、永远无法 resolve
    fireEvent.click(rows[0]);
    expect(await screen.findByText('detail boom')).toBeInTheDocument();
    expect(document.querySelector('.ant-drawer-open')).toBeNull();

    mockedGetDetail.mockRejectedValue('weird-string');
    fireEvent.click(rows[0]);
    expect(await screen.findByText('详情加载失败')).toBeInTheDocument();
    expect(document.querySelector('.ant-drawer-open')).toBeNull();
  });

  it('点击行打开详情（onRow）；刷新与查询按钮重新拉取列表', async () => {
    mockedGetDetail.mockResolvedValue({
      ...makeItem(),
      requestPayload: null,
      responseBody: null,
    });
    renderPage();
    const rows = await screen.findAllByTestId('stub-row');
    fireEvent.click(rows[0]);
    expect(await screen.findByText('执行留痕 #1')).toBeInTheDocument();
    expect(mockedGetDetail).toHaveBeenCalledWith(1);
    const drawer = document.querySelector('.ant-drawer-open') as HTMLElement;
    expect(within(drawer).getAllByText('（无）')).toHaveLength(2); // 请求/响应载荷均为空

    const afterRowClick = mockedList.mock.calls.length;
    fireEvent.click(screen.getByRole('button', { name: /刷\s*新/ }));
    await waitFor(() => expect(mockedList.mock.calls.length).toBeGreaterThan(afterRowClick));

    const afterRefresh = mockedList.mock.calls.length;
    fireEvent.click(screen.getByRole('button', { name: /查\s*询/ }));
    await waitFor(() => expect(mockedList.mock.calls.length).toBeGreaterThan(afterRefresh));
  });
});
