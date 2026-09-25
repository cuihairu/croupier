/**
 * 执行留痕页残余分支：
 * 1. 列表拉取以「非 Error」形态拒绝 → loadError 走国际化「加载失败」
 *    （既有用例只覆盖了 Error 形态）；
 * 2. 分页器 showTotal 文案拼接（ProTable 桩显式调用 pagination.showTotal）。
 * ProTable / RangePicker 用行为桩，参照 index.test.tsx 先例。
 */
import React from 'react';
import { App as AntdApp } from 'antd';
import { configure, fireEvent, render, screen, waitFor } from '@testing-library/react';
import ExecutionLogsPage from '../index';
import { listExecutionLogs } from '@/services/api/executionLogs';

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
  const intl = { formatMessage };
  return {
    __esModule: true,
    useIntl: () => intl,
    FormattedMessage: ({ defaultMessage }: { defaultMessage: string }) => defaultMessage,
  };
});

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
        data-testid="range-end-only"
        onClick={() => props.onChange?.([null, asDay('2026-09-02T09:30:00Z')])}
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

// ProTable 桩：挂载触发一次 request，并把 pagination.showTotal 的返回值
// 渲染出来（真实 ProTable 只在有数据且分页开启时求值，此处无条件求值以
// 覆盖 showTotal 拼接分支）
jest.mock('@ant-design/pro-components', () => {
  const React = require('react') as typeof import('react');
  const { useEffect } = React;

  type StubColumn = {
    title?: React.ReactNode;
    dataIndex?: string;
    render?: (value: unknown, record: unknown, index: number) => React.ReactNode;
  };
  type StubRequestResult = { data?: unknown[]; total?: number; success?: boolean };
  type StubProps = {
    columns?: StubColumn[];
    params?: Record<string, unknown>;
    locale?: { emptyText?: React.ReactNode };
    pagination?: {
      pageSize?: number;
      showSizeChanger?: boolean;
      showTotal?: (t: number) => string;
    };
    request?: (args: Record<string, unknown>) => Promise<StubRequestResult>;
    onRow?: (record: unknown) => { onClick?: () => void; style?: React.CSSProperties };
  };

  const ProTableStub: React.FC<StubProps> = (props) => {
    const [rows, setRows] = React.useState<unknown[]>([]);
    const [total, setTotal] = React.useState(0);

    const paramsKey = JSON.stringify(props.params ?? {});
    useEffect(() => {
      void props
        .request?.({
          current: 1,
          pageSize: props.pagination?.pageSize ?? 20,
          ...(props.params || {}),
        })
        .then((res: StubRequestResult) => {
          setRows(res?.data || []);
          setTotal(res?.total ?? 0);
        });
      // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [paramsKey]);

    return (
      <div data-testid="protable-stub">
        <div data-testid="show-total">{props.pagination?.showTotal?.(total)}</div>
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

beforeEach(() => {
  jest.clearAllMocks();
  mockedList.mockResolvedValue({ items: [sampleItem], total: 42 });
});

describe('执行留痕页残余分支', () => {
  it('列表拉取非 Error 拒绝 → 错误 Alert 显示国际化「加载失败」', async () => {
    (
      mockedList as unknown as { mockRejectedValueOnce: (v: unknown) => void }
    ).mockRejectedValueOnce('weird-string');

    render(
      <AntdApp>
        <ExecutionLogsPage />
      </AntdApp>,
    );

    const hits = await screen.findAllByText('加载失败');
    expect(hits.length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByRole('alert').length).toBeGreaterThanOrEqual(1);
  });

  it('分页 showTotal 按 total 拼接「共 N 条」', async () => {
    render(
      <AntdApp>
        <ExecutionLogsPage />
      </AntdApp>,
    );

    await waitFor(() => expect(screen.getByTestId('show-total').textContent).toBe('共 42 条'));
    expect(await screen.findByText('player.kick')).toBeInTheDocument();
  });

  const renderPage = () =>
    render(
      <AntdApp>
        <ExecutionLogsPage />
      </AntdApp>,
    );

  it('时间范围仅结束 → 摘要起始省略为 …，请求只带 to；清空后移除', async () => {
    renderPage();
    await screen.findByText('player.kick');

    fireEvent.click(screen.getByRole('button', { name: /更多筛选/ }));
    fireEvent.click(await screen.findByTestId('range-end-only'));

    expect(await screen.findByText(/已生效条件：时间 … ~/)).toBeInTheDocument();
    const afterPartial = mockedList.mock.calls[mockedList.mock.calls.length - 1][0];
    expect(afterPartial.from).toBeUndefined();
    expect(afterPartial.to).toBeDefined();

    fireEvent.click(screen.getByTestId('range-clear'));
    await waitFor(() => {
      const last = mockedList.mock.calls[mockedList.mock.calls.length - 1][0];
      expect(last.from).toBeUndefined();
      expect(last.to).toBeUndefined();
    });
    expect(screen.queryByText(/已生效条件：/)).not.toBeInTheDocument();
  });

  it('列表响应缺 items/total → 空数组与 0 兜底', async () => {
    (
      mockedList as unknown as { mockResolvedValueOnce: (v: unknown) => void }
    ).mockResolvedValueOnce({});
    renderPage();

    await waitFor(() => expect(mockedList).toHaveBeenCalled());
    expect(await screen.findByTestId('stub-empty')).toBeInTheDocument();
    await waitFor(() => expect(screen.getByTestId('show-total').textContent).toBe('共 0 条'));
  });

  it('来源/状态下拉可清空 → 空串兜底', async () => {
    const { container } = renderPage();
    await screen.findByText('player.kick');

    await chooseSelectOption(0, '页面');
    await waitFor(() =>
      expect(mockedList).toHaveBeenLastCalledWith(expect.objectContaining({ source: 'page' })),
    );
    const sourceClear = container.querySelector('.ant-select-clear');
    expect(sourceClear).toBeTruthy();
    fireEvent.click(sourceClear as HTMLElement);
    await waitFor(() =>
      expect(mockedList.mock.calls[mockedList.mock.calls.length - 1][0].source).toBeUndefined(),
    );

    await chooseSelectOption(1, '失败');
    await waitFor(() =>
      expect(mockedList).toHaveBeenLastCalledWith(expect.objectContaining({ status: 'error' })),
    );
    const statusClear = container.querySelector('.ant-select-clear');
    expect(statusClear).toBeTruthy();
    fireEvent.click(statusClear as HTMLElement);
    await waitFor(() =>
      expect(mockedList.mock.calls[mockedList.mock.calls.length - 1][0].status).toBeUndefined(),
    );
  });
});

/** 打开第 comboIndex 个 Select 下拉并点选 optionText */
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
