import React from 'react';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import ApprovalsPage from './index';
import {
  approveApproval,
  getApproval,
  listApprovals,
  listDescriptors,
  rejectApproval,
} from '@/services/api';
import { history } from '@umijs/max';

const mockMessageApi = { error: jest.fn(), success: jest.fn(), warning: jest.fn() };

jest.mock('@/services/api', () => ({
  listApprovals: jest.fn(),
  getApproval: jest.fn(),
  listDescriptors: jest.fn(),
  approveApproval: jest.fn(),
  rejectApproval: jest.fn(),
}));
jest.mock('@/utils/antdApp', () => ({ getMessage: () => mockMessageApi }));

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

// ProTable 桩：挂载与 params 变化（视图/筛选/descs 晚到驱动）均以
// {current,pageSize,...params} 触发 request；actionRef 提供 reload/setPageInfo
// （审批动作与查询按钮走它）；行容器带 testid 供行数断言。
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
    actionRef?: { current?: Record<string, unknown> | undefined };
  };
  const ProTable = (props: ProTableStubProps) => {
    const { useState, useRef, useEffect } = React;
    const [rows, setRows] = useState<unknown[]>([]);
    const lastArgsRef = useRef<Record<string, unknown>>({});
    const fire = (args: Record<string, unknown>) => {
      lastArgsRef.current = args;
      let alive = true;
      props
        .request?.(args)
        .then((r: ReqResult) => {
          if (alive) setRows(r.data ?? []);
        })
        .catch(() => {});
      return () => {
        alive = false;
      };
    };
    const paramsKey = JSON.stringify(props.params ?? {});
    useEffect(() => {
      fire({ current: 1, pageSize: 20, ...(props.params ?? {}) });
      // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [paramsKey]);
    if (props.actionRef) {
      props.actionRef.current = {
        reload: () => fire(lastArgsRef.current),
        setPageInfo: () => fire(lastArgsRef.current),
      };
    }
    return (
      <div data-testid="approvals-table-stub">
        {rows.map((row, i) => (
          <div data-testid="approval-row" key={i}>
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
const mockedApproveApproval = approveApproval as jest.Mock;
const mockedRejectApproval = rejectApproval as jest.Mock;
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

describe('ApprovalsPage 筛选、审批动作与错误路径', () => {
  type PromptFn = typeof window.prompt;
  type MockedURL = typeof URL & { revokeObjectURL: jest.Mock };
  const origPrompt: PromptFn = window.prompt;
  const downloads: string[] = [];
  let anchorClickSpy: jest.SpyInstance;

  const pendingRow = {
    id: 'ap-1',
    createdAt: '2026-09-23T00:00:00Z',
    actor: 'alice',
    functionId: 'demo.fn',
    gameId: 'demo',
    env: 'prod',
    state: 'pending' as const,
    mode: 'manual',
    payloadPreview: '{"a":1}',
  };
  const fullApproved = {
    id: 'ap-2',
    createdAt: '2026-09-22T00:00:00Z',
    actor: 'dave',
    functionId: 'mail.send',
    gameId: 'demo',
    env: 'prod',
    state: 'approved' as const,
    mode: 'auto',
    approver: 'bob',
    reviewedAt: '2026-09-22T02:00:00Z',
    reviewedByOther: true,
    idempotencyKey: 'idem-9',
    route: '/mail/send',
    targetServiceId: 'svc-1',
    hashKey: 'hash-9',
    reason: '活动补偿',
    payloadPreview: '{"p":2}',
  };

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

  /** 等待抽屉打开并返回其容器 */
  async function openDrawer(): Promise<HTMLElement> {
    return waitFor(() => {
      const d = document.querySelector('.ant-drawer-open');
      expect(d).toBeTruthy();
      return d as HTMLElement;
    });
  }

  beforeEach(() => {
    jest.clearAllMocks();
    downloads.length = 0;
    mockedListApprovals.mockResolvedValue({ approvals: [pendingRow], total: 1 });
    mockedListDescriptors.mockResolvedValue([]);
    mockedGetApproval.mockResolvedValue(pendingRow);
    window.prompt = jest.fn().mockReturnValue('') as unknown as PromptFn;
    (URL.createObjectURL as jest.Mock).mockReturnValue('blob:mock');
    (URL as unknown as MockedURL).revokeObjectURL = jest.fn();
    anchorClickSpy = jest.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(function (
      this: HTMLAnchorElement,
    ) {
      downloads.push(this.download);
    });
    window.history.replaceState({}, '', '/');
  });

  afterEach(() => {
    anchorClickSpy.mockRestore();
    window.prompt = origPrompt;
    window.history.replaceState({}, '', '/');
  });

  it('我发起的 Tab：透传 mine=true 且隐藏申请人输入与通过/拒绝动作', async () => {
    render(<ApprovalsPage />);
    await waitFor(() => expect(mockedListApprovals).toHaveBeenCalled());
    fireEvent.click(screen.getByRole('tab', { name: '我发起的' }));
    await waitFor(() =>
      expect(mockedListApprovals).toHaveBeenLastCalledWith(
        expect.objectContaining({ mine: 'true' }),
      ),
    );
    expect(screen.queryByPlaceholderText('申请人')).not.toBeInTheDocument();
    await screen.findAllByTestId('approval-row');
    expect(screen.queryByRole('button', { name: /通\s*过/ })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /拒\s*绝/ })).not.toBeInTheDocument();
  });

  it('状态筛选：选择「已通过」透传 status 参数', async () => {
    render(<ApprovalsPage />);
    await waitFor(() =>
      expect(mockedListApprovals).toHaveBeenCalledWith(
        expect.objectContaining({ status: 'pending' }),
      ),
    );
    await chooseSelectOption(0, '已通过');
    await waitFor(() =>
      expect(mockedListApprovals).toHaveBeenLastCalledWith(
        expect.objectContaining({ status: 'approved' }),
      ),
    );
  });

  it('风险筛选：按 descriptor 风险客户端二次过滤，命中/无风险字段/无描述三分支', async () => {
    mockedListDescriptors.mockResolvedValue([
      { id: 'high.fn', risk: 'high' },
      { id: 'low.fn', risk: 'low' },
      { id: 'noRisk.fn' },
    ]);
    mockedListApprovals.mockResolvedValue({
      approvals: [
        { ...pendingRow, id: 'ap-h', functionId: 'high.fn' },
        { ...pendingRow, id: 'ap-l', functionId: 'low.fn' },
        { ...pendingRow, id: 'ap-n', functionId: 'noRisk.fn' },
        { ...pendingRow, id: 'ap-g', functionId: 'ghost.fn' },
      ],
      total: 4,
    });
    render(<ApprovalsPage />);
    await waitFor(() => expect(screen.getAllByTestId('approval-row')).toHaveLength(4));
    // descs 晚到进入 params 触发一次重查
    await waitFor(() => expect(mockedListApprovals.mock.calls.length).toBeGreaterThanOrEqual(2));

    await chooseSelectOption(1, '高');
    await waitFor(() =>
      expect(mockedListApprovals).toHaveBeenLastCalledWith(
        expect.objectContaining({ risk: 'high' }),
      ),
    );
    const rows = await waitFor(() => {
      const r = screen.getAllByTestId('approval-row');
      expect(r).toHaveLength(1);
      return r;
    });
    expect(rows[0]).toHaveTextContent('high.fn');
  });

  it('批准成功：OTP 输入 → approveApproval → 成功提示 → 重载并打开详情', async () => {
    render(<ApprovalsPage />);
    await screen.findAllByTestId('approval-row');
    fireEvent.click(screen.getByRole('button', { name: /通\s*过/ }));
    await waitFor(() =>
      expect(mockedApproveApproval).toHaveBeenCalledWith({ id: 'ap-1', otp: '' }),
    );
    expect(mockMessageApi.success).toHaveBeenCalledWith('已批准');
    await waitFor(() => expect(mockedGetApproval).toHaveBeenCalledWith('ap-1'));
    await openDrawer();
    await waitFor(() => expect(mockedListApprovals.mock.calls.length).toBeGreaterThanOrEqual(2));
  });

  it('批准失败：Error 与非 Error 均提示错误，且不重载不打开详情', async () => {
    render(<ApprovalsPage />);
    await screen.findAllByTestId('approval-row');
    const listCallsAfterLoad = mockedListApprovals.mock.calls.length;

    mockedApproveApproval.mockRejectedValueOnce(new Error('审批服务不可用'));
    fireEvent.click(screen.getByRole('button', { name: /通\s*过/ }));
    await waitFor(() => expect(mockMessageApi.error).toHaveBeenCalledWith('审批服务不可用'));

    mockedApproveApproval.mockRejectedValueOnce('weird-string');
    fireEvent.click(screen.getByRole('button', { name: /通\s*过/ }));
    await waitFor(() => expect(mockMessageApi.error).toHaveBeenCalledWith('批准失败'));

    expect(mockedGetApproval).not.toHaveBeenCalled();
    expect(mockedListApprovals.mock.calls.length).toBe(listCallsAfterLoad);
    expect(document.querySelector('.ant-drawer-open')).toBeNull();
  });

  it('高风险批准：函数 ID 不匹配警告并中止，匹配后携带 OTP 继续', async () => {
    mockedListDescriptors.mockResolvedValue([{ id: 'high.fn', risk: 'high' }]);
    mockedListApprovals.mockResolvedValue({
      approvals: [{ ...pendingRow, id: 'ap-h', functionId: 'high.fn' }],
      total: 1,
    });
    render(<ApprovalsPage />);
    await screen.findAllByTestId('approval-row');
    // 等 descMap 就绪（descs 进 params 触发的重查已发生）
    await waitFor(() => expect(mockedListApprovals.mock.calls.length).toBeGreaterThanOrEqual(2));

    (window.prompt as unknown as jest.Mock).mockReturnValueOnce('wrong-id');
    fireEvent.click(screen.getByRole('button', { name: /通\s*过/ }));
    await waitFor(() => expect(mockMessageApi.warning).toHaveBeenCalledWith('确认文本不匹配'));
    expect(mockedApproveApproval).not.toHaveBeenCalled();
    expect(window.prompt).toHaveBeenCalledTimes(1);

    (window.prompt as unknown as jest.Mock)
      .mockReturnValueOnce('high.fn')
      .mockReturnValueOnce('otp-9');
    fireEvent.click(screen.getByRole('button', { name: /通\s*过/ }));
    await waitFor(() =>
      expect(mockedApproveApproval).toHaveBeenCalledWith({ id: 'ap-h', otp: 'otp-9' }),
    );
  });

  it('拒绝成功：原因输入 → 成功提示，详情展示原因并支持拒绝审计跳转', async () => {
    mockedGetApproval.mockResolvedValue({
      ...pendingRow,
      state: 'rejected',
      approver: 'carol',
      reason: '活动违规',
    });
    render(<ApprovalsPage />);
    await screen.findAllByTestId('approval-row');
    (window.prompt as unknown as jest.Mock).mockReturnValue('活动违规');
    fireEvent.click(screen.getByRole('button', { name: /拒\s*绝/ }));
    await waitFor(() =>
      expect(mockedRejectApproval).toHaveBeenCalledWith({ id: 'ap-1', reason: '活动违规' }),
    );
    expect(mockMessageApi.success).toHaveBeenCalledWith('已拒绝');

    const drawer = await openDrawer();
    expect(within(drawer).getByText('活动违规')).toBeInTheDocument();
    const auditBtn = await within(drawer).findByRole('button', { name: '查看审计（拒绝）' });
    fireEvent.click(auditBtn);
    expect(mockedHistoryPush).toHaveBeenCalledWith(
      '/admin/operation-logs?actor=carol&kind=approval_reject',
    );
  });

  it('拒绝失败：Error 与非 Error 均提示错误，且不重载不打开详情', async () => {
    render(<ApprovalsPage />);
    await screen.findAllByTestId('approval-row');
    const listCallsAfterLoad = mockedListApprovals.mock.calls.length;

    mockedRejectApproval.mockRejectedValueOnce(new Error('权限不足'));
    fireEvent.click(screen.getByRole('button', { name: /拒\s*绝/ }));
    await waitFor(() => expect(mockMessageApi.error).toHaveBeenCalledWith('权限不足'));

    mockedRejectApproval.mockRejectedValueOnce('weird-string');
    fireEvent.click(screen.getByRole('button', { name: /拒\s*绝/ }));
    await waitFor(() => expect(mockMessageApi.error).toHaveBeenCalledWith('拒绝失败'));

    expect(mockedGetApproval).not.toHaveBeenCalled();
    expect(mockedListApprovals.mock.calls.length).toBe(listCallsAfterLoad);
    expect(document.querySelector('.ant-drawer-open')).toBeNull();
  });

  it('查看失败：Error 与非 Error 均提示错误且不开抽屉', async () => {
    render(<ApprovalsPage />);
    await screen.findAllByTestId('approval-row');

    mockedGetApproval.mockRejectedValueOnce(new Error('网络错误'));
    fireEvent.click(screen.getByRole('button', { name: /查\s*看/ }));
    await waitFor(() => expect(mockMessageApi.error).toHaveBeenCalledWith('网络错误'));
    expect(document.querySelector('.ant-drawer-open')).toBeNull();

    mockedGetApproval.mockRejectedValueOnce('weird-string');
    fireEvent.click(screen.getByRole('button', { name: /查\s*看/ }));
    await waitFor(() => expect(mockMessageApi.error).toHaveBeenCalledWith('加载失败'));
    expect(document.querySelector('.ant-drawer-open')).toBeNull();
  });

  it('列表加载失败：Error 与非 Error 均提示错误并保持空数据', async () => {
    mockedListApprovals.mockRejectedValueOnce(new Error('boom'));
    render(<ApprovalsPage />);
    await waitFor(() => expect(mockMessageApi.error).toHaveBeenCalledWith('boom'));

    mockedListApprovals.mockRejectedValueOnce('weird-string');
    fireEvent.click(screen.getByRole('button', { name: /查\s*询/ }));
    await waitFor(() => expect(mockMessageApi.error).toHaveBeenCalledWith('加载失败'));
    expect(screen.queryAllByTestId('approval-row')).toHaveLength(0);
  });

  it('深链 approvalId：渲染时直接打开详情抽屉', async () => {
    window.history.replaceState({}, '', '/approvals?approvalId=ap-7');
    mockedGetApproval.mockResolvedValue({ ...pendingRow, id: 'ap-7' });
    render(<ApprovalsPage />);
    await waitFor(() => expect(mockedGetApproval).toHaveBeenCalledWith('ap-7'));
    await openDrawer();
    expect(await screen.findByText('审批详情 ap-7')).toBeInTheDocument();
  });

  it('导出 JSON 与导出预览文本：创建下载并回收对象 URL', async () => {
    render(<ApprovalsPage />);
    await screen.findAllByTestId('approval-row');
    fireEvent.click(screen.getByRole('button', { name: /查\s*看/ }));
    const drawer = await openDrawer();

    fireEvent.click(within(drawer).getByRole('button', { name: '导出 JSON' }));
    fireEvent.click(within(drawer).getByRole('button', { name: '导出预览文本' }));

    expect(URL.createObjectURL).toHaveBeenCalledTimes(2);
    expect(downloads).toEqual(['approval_ap-1.json', 'approval_preview_ap-1.txt']);
    const revoke = (URL as unknown as MockedURL).revokeObjectURL;
    expect(revoke).toHaveBeenCalledTimes(2);
    expect(revoke).toHaveBeenCalledWith('blob:mock');
  });

  it('列与抽屉字段：风险/OTP 标签、两人复核与抽屉完整元数据', async () => {
    mockedListDescriptors.mockResolvedValue([{ id: 'mail.send', risk: 'high' }]);
    mockedListApprovals.mockResolvedValue({ approvals: [fullApproved], total: 1 });
    mockedGetApproval.mockResolvedValue(fullApproved);
    render(<ApprovalsPage />);
    const rows = await screen.findAllByTestId('approval-row');
    expect(rows[0]).toHaveTextContent('mail.send');
    // 等 descMap 就绪后风险/OTP 标签进入函数列
    await waitFor(() => expect(rows[0]).toHaveTextContent('OTP'));
    expect(rows[0]).toHaveTextContent('high');
    // 审批信息列：审批人 / 复核时间 + 两人复核标签
    expect(rows[0]).toHaveTextContent('bob / 2026-09-22T02:00:00Z');
    expect(rows[0]).toHaveTextContent('两人复核');
    expect(rows[0]).toHaveTextContent('demo/prod');

    fireEvent.click(within(rows[0]).getByRole('button', { name: /查\s*看/ }));
    const drawer = await openDrawer();
    expect(within(drawer).getByText('bob / 2026-09-22T02:00:00Z')).toBeInTheDocument();
    expect(within(drawer).getAllByText('两人复核')).toHaveLength(1);
    expect(within(drawer).getByText('idem-9')).toBeInTheDocument();
    expect(within(drawer).getByText('/mail/send')).toBeInTheDocument();
    expect(within(drawer).getByText('svc-1')).toBeInTheDocument();
    expect(within(drawer).getByText('hash-9')).toBeInTheDocument();
    expect(within(drawer).getByText('活动补偿')).toBeInTheDocument();
    expect(within(drawer).getByText('载荷预览')).toBeInTheDocument();
    expect(within(drawer).getByText('{"p":2}')).toBeInTheDocument();
  });

  const lastListArgs = (): Record<string, string | undefined> => {
    const calls = mockedListApprovals.mock.calls;
    return calls[calls.length - 1][0] as Record<string, string | undefined>;
  };

  it('四个筛选输入都有值：查询串拼接 functionId/gameId/env/actor', async () => {
    render(<ApprovalsPage />);
    await screen.findAllByTestId('approval-row');

    fireEvent.change(screen.getByPlaceholderText('函数ID'), { target: { value: 'demo.fn' } });
    fireEvent.change(screen.getByPlaceholderText('游戏'), { target: { value: 'demo' } });
    fireEvent.change(screen.getByPlaceholderText('环境'), { target: { value: 'prod' } });
    fireEvent.change(screen.getByPlaceholderText('申请人'), { target: { value: 'alice' } });

    await waitFor(() => {
      expect(lastListArgs()).toMatchObject({
        functionId: 'demo.fn',
        gameId: 'demo',
        env: 'prod',
        actor: 'alice',
      });
    });
  });

  it('listApprovals 响应缺 approvals 字段：按空列表处理', async () => {
    mockedListApprovals.mockResolvedValue({});
    render(<ApprovalsPage />);
    await waitFor(() => expect(mockedListApprovals).toHaveBeenCalled());
    expect(screen.getByTestId('approvals-table-stub')).toBeInTheDocument();
    expect(screen.queryAllByTestId('approval-row')).toHaveLength(0);
  });

  it('风险筛选：记录缺 functionId 时按空风险参与比较', async () => {
    mockedListDescriptors.mockResolvedValue([{ id: 'demo.fn', risk: 'high' }]);
    mockedListApprovals.mockResolvedValue({
      approvals: [
        { ...pendingRow, id: 'ap-nofn', functionId: undefined },
        { ...pendingRow, id: 'ap-fn', functionId: 'demo.fn' },
      ],
      total: 2,
    });
    render(<ApprovalsPage />);
    await waitFor(() => expect(screen.getAllByTestId('approval-row')).toHaveLength(2));
    await waitFor(() => expect(mockedListApprovals.mock.calls.length).toBeGreaterThanOrEqual(2));

    await chooseSelectOption(1, '高');
    await waitFor(() =>
      expect(mockedListApprovals).toHaveBeenLastCalledWith(
        expect.objectContaining({ risk: 'high' }),
      ),
    );
    const rows = await waitFor(() => {
      const r = screen.getAllByTestId('approval-row');
      expect(r).toHaveLength(1);
      return r;
    });
    expect(rows[0]).toHaveTextContent('demo.fn');
  });

  it('listDescriptors 失败：.catch 静默吞掉且不打断列表', async () => {
    mockedListDescriptors.mockRejectedValue(new Error('desc boom'));
    render(<ApprovalsPage />);
    await waitFor(() => expect(mockedListDescriptors).toHaveBeenCalledTimes(1));
    await screen.findAllByTestId('approval-row');
    expect(mockMessageApi.error).not.toHaveBeenCalled();
  });

  it('listDescriptors 解析空值：descs 兜底为空数组侧', async () => {
    mockedListDescriptors.mockResolvedValueOnce(undefined as never);
    render(<ApprovalsPage />);
    await waitFor(() => expect(mockedListDescriptors).toHaveBeenCalledTimes(1));
    await screen.findAllByTestId('approval-row');
    expect(mockMessageApi.error).not.toHaveBeenCalled();
  });

  it('切回「待我审批」Tab：state 由空回到 pending', async () => {
    render(<ApprovalsPage />);
    await screen.findAllByTestId('approval-row');

    fireEvent.click(screen.getByRole('tab', { name: '全部' }));
    await waitFor(() => expect(lastListArgs().status).toBeUndefined());

    fireEvent.click(screen.getByRole('tab', { name: '待我审批' }));
    await waitFor(() => expect(lastListArgs().status).toBe('pending'));
  });

  it('详情缺 id 与 payloadPreview：标题 / 文件名 / 预览下载均走空串兜底', async () => {
    mockedGetApproval.mockResolvedValue({ ...pendingRow, id: '', payloadPreview: undefined });
    render(<ApprovalsPage />);
    await screen.findAllByTestId('approval-row');
    fireEvent.click(screen.getByRole('button', { name: /查\s*看/ }));
    const drawer = await openDrawer();
    expect(within(drawer).getByText(/^审批详情\s*$/)).toBeInTheDocument();

    fireEvent.click(within(drawer).getByRole('button', { name: '导出 JSON' }));
    fireEvent.click(within(drawer).getByRole('button', { name: '导出预览文本' }));
    expect(downloads).toEqual(['approval_.json', 'approval_preview_.txt']);
  });

  it('已通过记录且 approver/actor 均为空：批准审计按空串跳转', async () => {
    const bare = {
      ...pendingRow,
      id: 'ap-bare-ok',
      state: 'approved',
      approver: undefined,
      actor: undefined,
    };
    mockedListApprovals.mockResolvedValue({ approvals: [bare], total: 1 });
    mockedGetApproval.mockResolvedValue(bare);
    render(<ApprovalsPage />);
    await screen.findAllByTestId('approval-row');
    fireEvent.click(screen.getByRole('button', { name: /查\s*看/ }));
    const drawer = await openDrawer();

    fireEvent.click(within(drawer).getByRole('button', { name: '查看审计（批准）' }));
    expect(mockedHistoryPush).toHaveBeenCalledWith(
      '/admin/operation-logs?actor=&kind=approval_approve',
    );
  });

  it('已拒绝记录且 approver/actor 均为空：拒绝审计按空串跳转', async () => {
    const bare = {
      ...pendingRow,
      id: 'ap-bare-ng',
      state: 'rejected',
      approver: undefined,
      actor: undefined,
    };
    mockedListApprovals.mockResolvedValue({ approvals: [bare], total: 1 });
    mockedGetApproval.mockResolvedValue(bare);
    render(<ApprovalsPage />);
    await screen.findAllByTestId('approval-row');
    fireEvent.click(screen.getByRole('button', { name: /查\s*看/ }));
    const drawer = await openDrawer();

    fireEvent.click(within(drawer).getByRole('button', { name: '查看审计（拒绝）' }));
    expect(mockedHistoryPush).toHaveBeenCalledWith(
      '/admin/operation-logs?actor=&kind=approval_reject',
    );
  });

  it('抽屉 onClose：点击关闭按钮收起详情', async () => {
    render(<ApprovalsPage />);
    await screen.findAllByTestId('approval-row');
    fireEvent.click(screen.getByRole('button', { name: /查\s*看/ }));
    const drawer = await openDrawer();

    const closeBtn =
      within(drawer).queryByRole('button', { name: /close/i }) ??
      drawer.querySelector('.ant-drawer-close');
    expect(closeBtn).toBeTruthy();
    fireEvent.click(closeBtn as HTMLElement);
    await waitFor(() => expect(document.querySelector('.ant-drawer-open')).toBeNull());
  });

  it('通过缺 functionId 的记录：风险未知不走高风险确认', async () => {
    mockedListApprovals.mockResolvedValue({
      approvals: [{ ...pendingRow, id: 'ap-nofn2', functionId: undefined }],
      total: 1,
    });
    render(<ApprovalsPage />);
    await screen.findAllByTestId('approval-row');

    fireEvent.click(screen.getByRole('button', { name: /通\s*过/ }));
    await waitFor(() =>
      expect(mockedApproveApproval).toHaveBeenCalledWith({ id: 'ap-nofn2', otp: '' }),
    );
    // 仅 OTP 一次 prompt，未触发高风险函数 ID 确认
    expect(window.prompt).toHaveBeenCalledTimes(1);
  });

  it('高风险确认弹窗返回空：|| 空串兜底后中止', async () => {
    mockedListDescriptors.mockResolvedValue([{ id: 'high.fn', risk: 'high' }]);
    mockedListApprovals.mockResolvedValue({
      approvals: [{ ...pendingRow, id: 'ap-h2', functionId: 'high.fn' }],
      total: 1,
    });
    render(<ApprovalsPage />);
    await screen.findAllByTestId('approval-row');
    await waitFor(() => expect(mockedListApprovals.mock.calls.length).toBeGreaterThanOrEqual(2));

    (window.prompt as unknown as jest.Mock).mockReturnValueOnce(null);
    fireEvent.click(screen.getByRole('button', { name: /通\s*过/ }));
    await waitFor(() => expect(mockMessageApi.warning).toHaveBeenCalledWith('确认文本不匹配'));
    expect(mockedApproveApproval).not.toHaveBeenCalled();
  });
});
