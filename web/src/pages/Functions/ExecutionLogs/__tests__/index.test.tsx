/**
 * 执行留痕页面结构（对齐 StandardPage 规范）：概览说明/推荐路径、筛选栏结果计数、
 * 清空筛选与已生效条件提示、列表空态二分（筛选无结果 vs 全空引导）、
 * 加载失败错误 Alert 与重试。ProTable 用行为桩（挂载即触发 request、
 * 透传 locale.emptyText 渲染空态），参照 OperationLogs.test.tsx 先例。
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
  // 稳定引用：组件树内任何 effect 依赖 intl 都不能因新对象无限重建
  const intl = { formatMessage };
  return {
    __esModule: true,
    useIntl: () => intl,
    FormattedMessage: ({ defaultMessage }: { defaultMessage: string }) => defaultMessage,
  };
});

jest.mock('@/services/api/executionLogs', () => ({
  listExecutionLogs: jest.fn(),
  getExecutionLog: jest.fn(),
}));

// ProTable 行为桩：挂载即以 {current,pageSize,...params} 触发一次 request；
// actionRef 提供可用的 reload（重试/查询/刷新按钮走它）；空数据时透传
// locale.emptyText 渲染，支撑空态二分断言。
jest.mock('@ant-design/pro-components', () => {
  const React = require('react') as typeof import('react');
  const { useState, useRef, useEffect } = React;

  type StubColumn = {
    title?: React.ReactNode;
    dataIndex?: string;
    render?: (value: unknown, record: unknown, index: number) => React.ReactNode;
  };
  type StubRequestResult = { data?: unknown[]; total?: number; success?: boolean };
  type StubProps = {
    actionRef?: { current?: Record<string, unknown> | undefined };
    rowKey?: string;
    columns?: StubColumn[];
    params?: Record<string, unknown>;
    locale?: { emptyText?: React.ReactNode };
    request?: (args: Record<string, unknown>) => Promise<StubRequestResult>;
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

    useEffect(() => {
      fire({ current: 1, pageSize: 20, ...(props.params || {}) });
      // 仅挂载时模拟真实 ProTable 的首次请求
      // eslint-disable-next-line react-hooks/exhaustive-deps
    }, []);

    if (props.actionRef) {
      props.actionRef.current = {
        reload: () => fire(lastArgsRef.current),
        setPageInfo: () => undefined,
      };
    }

    return (
      <div data-testid="protable-stub">
        {rows.map((row, i: number) => (
          <div data-testid="stub-row" key={i}>
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

  const PageContainerStub: React.FC<{ title?: React.ReactNode; subTitle?: React.ReactNode }> = ({
    title,
    subTitle,
    children,
  }) => (
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

function renderPage() {
  return render(
    <AntdApp>
      <ExecutionLogsPage />
    </AntdApp>,
  );
}

describe('执行留痕页面结构（StandardPage 对齐）', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockedList.mockResolvedValue({ items: [sampleItem], total: 1 } as never);
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
