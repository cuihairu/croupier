/**
 * Agent 扩展同步调试页单测（覆盖率巡检：AgentSync/index.tsx 93 行 0% →
 * 收口，Extensions 簇余量顺序第三位）。
 *
 * 锁定契约：空输入/纯空白拦截（warning + 不触达服务）、trim 后查询
 * （getAgentSyncPayload 收归一值）、载荷 pretty-print（JSON.stringify
 * null,2）与 payload 缺省 '{}' 兜底、onPressEnter 等价查询、清空双态
 * 复位（输入 + 载荷回「暂无数据」）、Card 标题与空态文案。
 *
 * mock 口径：services/api/extensions 的 getAgentSyncPayload jest.mock；
 * @umijs/max 本地 mock；PageContainer/antd 真实渲染（App 包裹供 message）。
 *
 * 边界（诚实）：runSyncQuery 是 try/finally 无 catch——查询 reject 会产生
 * unhandled rejection（现状行为，与 InstallationDetailDrawer/openDetail 巡检
 * 结论同族，不改组件），不造假 reject 场景；`resp?.payload || {}` 的右翼经
 * payload undefined 的 resolve 形态覆盖。
 */
import React from 'react';
import { configure, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App } from 'antd';
import ExtensionAgentSyncPage from '../index';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

jest.mock('@/services/api/extensions', () => ({
  getAgentSyncPayload: jest.fn(),
}));

jest.mock('@umijs/max', () => ({
  FormattedMessage: ({ defaultMessage }: { defaultMessage?: string }) => (
    <>{defaultMessage ?? ''}</>
  ),
  useIntl: () => ({
    formatMessage: (opts: { defaultMessage?: string }) => opts.defaultMessage ?? '',
  }),
}));

import { getAgentSyncPayload } from '@/services/api/extensions';

const mSync = getAgentSyncPayload as jest.MockedFunction<typeof getAgentSyncPayload>;

function renderPage() {
  return render(
    <App>
      <ExtensionAgentSyncPage />
    </App>,
  );
}

function agentInput() {
  return screen.getByPlaceholderText('例如: agent-001') as HTMLInputElement;
}

beforeEach(() => {
  jest.clearAllMocks();
  mSync.mockResolvedValue({ payload: { extensions: ['chatops@1.4.0'], revision: 7 } });
});

describe('Agent 扩展同步调试页', () => {
  it('页头与初始空态：标题/副标题/Agent ID 表单/暂无数据', () => {
    renderPage();
    expect(screen.getByText('Agent 扩展同步调试')).toBeInTheDocument();
    expect(screen.getByText('查看指定 Agent 的扩展同步载荷')).toBeInTheDocument();
    expect(screen.getByText('Agent ID')).toBeInTheDocument();
    expect(screen.getByText('Payload JSON')).toBeInTheDocument();
    expect(screen.getByText('暂无数据')).toBeInTheDocument();
    expect(screen.queryByDisplayValue(/.+/s)).not.toBeInTheDocument();
  });

  it('空输入与纯空白拦截：warning + 不触达服务', async () => {
    renderPage();

    fireEvent.click(screen.getByRole('button', { name: '查询同步载荷' }));
    expect(await screen.findByText('请输入 Agent ID')).toBeInTheDocument();
    expect(mSync).not.toHaveBeenCalled();

    fireEvent.change(agentInput(), { target: { value: '   ' } });
    fireEvent.click(screen.getByRole('button', { name: '查询同步载荷' }));
    expect(await screen.findByText('请输入 Agent ID')).toBeInTheDocument();
    expect(mSync).not.toHaveBeenCalled();
  });

  it('查询主链：trim 归一 + 载荷 pretty-print 到只读 TextArea', async () => {
    renderPage();

    fireEvent.change(agentInput(), { target: { value: '  agent-001  ' } });
    fireEvent.click(screen.getByRole('button', { name: '查询同步载荷' }));

    await waitFor(() => expect(mSync).toHaveBeenCalledWith('agent-001'));
    // getByDisplayValue 会对 value 做空白归一（折叠连续空白）——多行
    // pretty JSON 以「单空格折叠形态」断言（坑记：字面换行正则不匹配）
    expect(
      await screen.findByDisplayValue('{ "extensions": [ "chatops@1.4.0" ], "revision": 7 }'),
    ).toBeInTheDocument();
    expect(screen.queryByText('暂无数据')).not.toBeInTheDocument();
  });

  it('payload 缺省兜底：resp.payload undefined → {} 落 TextArea', async () => {
    mSync.mockResolvedValue({ payload: undefined } as never);
    renderPage();

    fireEvent.change(agentInput(), { target: { value: 'agent-002' } });
    fireEvent.click(screen.getByRole('button', { name: '查询同步载荷' }));

    await waitFor(() => expect(mSync).toHaveBeenCalledWith('agent-002'));
    expect(await screen.findByDisplayValue(/^\{\}$/)).toBeInTheDocument();
  });

  it('onPressEnter 等价查询', async () => {
    renderPage();

    fireEvent.change(agentInput(), { target: { value: 'agent-003' } });
    fireEvent.keyDown(agentInput(), { key: 'Enter' });

    await waitFor(() => expect(mSync).toHaveBeenCalledWith('agent-003'));
  });

  it('清空：输入与载荷双复位', async () => {
    renderPage();

    fireEvent.change(agentInput(), { target: { value: 'agent-001' } });
    fireEvent.click(screen.getByRole('button', { name: '查询同步载荷' }));
    await waitFor(() => expect(mSync).toHaveBeenCalledTimes(1));

    fireEvent.click(screen.getByRole('button', { name: '清空' }));
    expect(agentInput().value).toBe('');
    expect(screen.getByText('暂无数据')).toBeInTheDocument();
  });
});
