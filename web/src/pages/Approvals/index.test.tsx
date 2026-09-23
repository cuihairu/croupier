import React from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import ApprovalsPage from './index';
import { getApproval, listApprovals, listDescriptors } from '@/services/api';
import { history } from '@umijs/max';

jest.mock('@/services/api', () => ({
  listApprovals: jest.fn(),
  getApproval: jest.fn(),
  listDescriptors: jest.fn(),
  approveApproval: jest.fn(),
  rejectApproval: jest.fn(),
}));
jest.mock('@/utils/antdApp', () => ({ getMessage: () => undefined }));

// @umijs/max mock：formatMessage 直返 defaultMessage 便于中文断言；
// history.push 为可断言 spy（审计跳转不再用 window.open，改应用内跳转）
jest.mock('@umijs/max', () => ({
  FormattedMessage: ({
    defaultMessage,
  }: {
    id: string;
    defaultMessage?: string;
    values?: Record<string, string>;
  }) => <>{defaultMessage ?? ''}</>,
  useIntl: () => ({
    formatMessage: (opts: { defaultMessage?: string }, values?: Record<string, string>) => {
      let text = opts.defaultMessage ?? '';
      if (values) {
        for (const [k, v] of Object.entries(values)) text = text.replace(`{${k}}`, v);
      }
      return text;
    },
  }),
  history: { push: jest.fn() },
}));

// ProTable 桩：挂载即以默认分页参数触发一次 request，渲染行内「查看」按钮。
// 仅覆盖本用例链路（列表加载 → 打开抽屉），审批动作等重交互不在本套件范围。
jest.mock('@ant-design/pro-components', () => {
  const React = require('react') as typeof import('react');
  type ColumnDef = {
    render?: (value: unknown, record: unknown) => React.ReactNode;
  };
  type ReqResult = { data?: unknown[]; total?: number; success?: boolean };
  type ProTableStubProps = {
    columns?: ColumnDef[];
    request?: (args: Record<string, unknown>) => Promise<ReqResult>;
    params?: Record<string, unknown>;
  };
  const ProTable = (props: ProTableStubProps) => {
    const [rows, setRows] = React.useState<unknown[]>([]);
    React.useEffect(() => {
      let alive = true;
      props
        .request?.({
          current: 1,
          pageSize: 20,
          ...(props.params ?? {}),
        })
        .then((r: ReqResult) => {
          if (alive) setRows(r.data ?? []);
        })
        .catch(() => {});
      return () => {
        alive = false;
      };
      // eslint-disable-next-line react-hooks/exhaustive-deps
    }, []);
    return (
      <div data-testid="approvals-table-stub">
        {rows.map((row, i) => (
          <div key={i}>
            {(props.columns ?? []).map((col, j) => (
              <span key={j}>{col.render ? col.render(undefined, row) : null}</span>
            ))}
          </div>
        ))}
      </div>
    );
  };
  return { ProTable };
});

const approvedRow = {
  id: 'ap-1',
  createdAt: '2026-09-23T00:00:00Z',
  actor: 'alice',
  functionId: 'demo.fn',
  state: 'approved' as const,
  approver: 'bob',
  payloadPreview: '{}',
};

const mockedListApprovals = listApprovals as jest.Mock;
const mockedGetApproval = getApproval as jest.Mock;
const mockedListDescriptors = listDescriptors as jest.Mock;
const mockedHistoryPush = history.push as jest.Mock;

describe('ApprovalsPage 审计跳转', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockedListApprovals.mockResolvedValue({ approvals: [approvedRow], total: 1 });
    mockedListDescriptors.mockResolvedValue([]);
    mockedGetApproval.mockResolvedValue(approvedRow);
  });

  it('抽屉「查看审计（申请人）」应用内跳转操作日志并带 actor 过滤', async () => {
    render(<ApprovalsPage />);
    await waitFor(() => expect(screen.getByRole('button', { name: '查看' })).toBeInTheDocument(), {
      timeout: 15000,
    });
    fireEvent.click(screen.getByRole('button', { name: '查看' }));
    const actorBtn = await screen.findByRole('button', { name: '查看审计（申请人）' });
    fireEvent.click(actorBtn);
    expect(mockedHistoryPush).toHaveBeenCalledWith('/admin/operation-logs?actor=alice');
  });

  it('已通过记录的「查看审计（批准）」带 kind=approval_approve 聚焦事件类型', async () => {
    render(<ApprovalsPage />);
    await waitFor(() => expect(screen.getByRole('button', { name: '查看' })).toBeInTheDocument());
    fireEvent.click(screen.getByRole('button', { name: '查看' }));
    const approveBtn = await screen.findByRole('button', { name: '查看审计（批准）' });
    fireEvent.click(approveBtn);
    expect(mockedHistoryPush).toHaveBeenCalledWith(
      '/admin/operation-logs?actor=bob&kind=approval_approve',
    );
  });
});
