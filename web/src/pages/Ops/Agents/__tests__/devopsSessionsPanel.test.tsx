import React from 'react';
import { configure, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { App as AntdApp } from 'antd';
import DevopsSessionsPanel from '../DevopsSessionsPanel';
import {
  closeDevopsSession,
  listDevopsSessions,
  openDevopsSession,
  type DevopsSessionItem,
} from '@/services/api/devopsSessions';
import { listOpsAgentsByType } from '@/services/api/ops';
import { getScope, setScope, type Scope } from '@/stores/scope';

// antd Table/Modal/cssinjs 首渲染在本机高负载下可超 jest 默认 5s（CI 毫秒级）；
// findBy*/waitFor 默认 1s 也不够（Modal/Select portal 经调度器延后渲染）。
jest.setTimeout(60000);
configure({ asyncUtilTimeout: 15000 });

// antd v6 两汉字按钮自动插空格（「接 入」「关 闭」），查询一律用正则容空格；
// Select 开拉靠 mouseDown（仓内 Certificates 先例），option 从非 hidden
// dropdown 里点（IncidentReports 同款辅助的 Select 版）。
const button = (name: RegExp) => screen.findByRole('button', { name });

async function pickAgentOption(label: string) {
  fireEvent.mouseDown(await screen.findByRole('combobox'));
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

jest.mock('@/services/api/devopsSessions', () => ({
  listDevopsSessions: jest.fn(),
  openDevopsSession: jest.fn(),
  closeDevopsSession: jest.fn(),
}));
jest.mock('@/services/api/ops', () => ({
  listOpsAgentsByType: jest.fn(),
}));
jest.mock('@umijs/max', () => ({
  FormattedMessage: ({ defaultMessage }: { defaultMessage?: string }) => (
    <>{defaultMessage ?? ''}</>
  ),
  useIntl: () => ({
    formatMessage: ({ defaultMessage }: { defaultMessage?: string }) => defaultMessage ?? '',
  }),
}));
// 终端本体由 devopsTerminal.test.tsx 单测覆盖；面板测试只验证会话编排。
jest.mock('../DevopsTerminal', () => ({
  __esModule: true,
  default: ({ sessionId }: { sessionId: string }) => (
    <div data-testid="devops-terminal-stub">{sessionId}</div>
  ),
}));

const mockedList = listDevopsSessions as jest.MockedFunction<typeof listDevopsSessions>;
const mockedOpen = openDevopsSession as jest.MockedFunction<typeof openDevopsSession>;
const mockedClose = closeDevopsSession as jest.MockedFunction<typeof closeDevopsSession>;
const mockedAgents = listOpsAgentsByType as jest.MockedFunction<typeof listOpsAgentsByType>;

const renderPanel = () =>
  render(
    <AntdApp>
      <DevopsSessionsPanel />
    </AntdApp>,
  );

describe('DevopsSessionsPanel', () => {
  beforeEach(() => {
    mockedList.mockReset().mockResolvedValue([]);
    mockedOpen.mockReset();
    mockedClose.mockReset().mockResolvedValue(undefined);
    mockedAgents.mockReset().mockResolvedValue({ nodes: [] } as never);
  });

  it('lists real sessions with target and protocol', async () => {
    mockedList.mockResolvedValue([
      {
        sessionId: 'sess-1',
        agentId: 'agent-a',
        operator: 'admin',
        protocol: 'ssh',
        host: '10.0.0.11',
        port: 22,
        channel: 'exec',
        createdAt: '2026-11-15 01:02:03',
      },
    ]);
    renderPanel();
    expect(await screen.findByText('sess-1')).toBeInTheDocument();
    expect(screen.getByText('10.0.0.11:22')).toBeInTheDocument();
    expect(screen.getByText('agent-a')).toBeInTheDocument();
    expect(screen.getByText('2026-11-15 01:02:03')).toBeInTheDocument();
  });

  it('shows an honest empty state instead of a fake session', async () => {
    renderPanel();
    expect(await screen.findByText('当前 scope 下没有进行中的 SSH 会话。')).toBeInTheDocument();
    expect(screen.queryByTestId('devops-terminal-stub')).not.toBeInTheDocument();
  });

  it('opens a session and attaches the terminal', async () => {
    mockedAgents.mockResolvedValue({ nodes: [{ id: 'agent-a', status: 'active' }] } as never);
    mockedOpen.mockResolvedValue({
      sessionId: 'sess-2',
      agentId: 'agent-a',
      protocol: 'ssh',
      host: '10.0.0.12',
      port: 2222,
    });
    renderPanel();
    fireEvent.click(await screen.findByRole('button', { name: '开会话' }));
    await pickAgentOption('agent-a');
    fireEvent.change(await screen.findByPlaceholderText('例如 10.0.0.11'), {
      target: { value: '10.0.0.12' },
    });
    fireEvent.click(await button(/打\s?开/));
    await waitFor(() =>
      expect(mockedOpen).toHaveBeenCalledWith(
        expect.objectContaining({
          agentId: 'agent-a',
          protocol: 'ssh',
          host: '10.0.0.12',
          port: 22,
        }),
      ),
    );
    expect(await screen.findByTestId('devops-terminal-stub')).toHaveTextContent('sess-2');
  });

  it('does not open a session when the form is invalid', async () => {
    renderPanel();
    fireEvent.click(await screen.findByRole('button', { name: '开会话' }));
    fireEvent.click(await button(/打\s?开/));
    await waitFor(() => expect(screen.getByText('请输入主机')).toBeInTheDocument());
    expect(mockedOpen).not.toHaveBeenCalled();
  });

  it('closes a session through the confirm dialog and reloads', async () => {
    mockedList.mockResolvedValue([
      {
        sessionId: 'sess-3',
        agentId: 'agent-a',
        operator: 'admin',
        protocol: 'ssh',
        host: '10.0.0.13',
        port: 22,
        createdAt: '2026-11-15 02:00:00',
      },
    ]);
    renderPanel();
    fireEvent.click(await button(/关\s?闭/));
    const confirm = await screen.findByRole('button', { name: '关闭会话' });
    fireEvent.click(confirm);
    await waitFor(() => expect(mockedClose).toHaveBeenCalledWith('sess-3'));
    await waitFor(() => expect(mockedList.mock.calls.length).toBeGreaterThan(1));
  });

  // 切 scope 后旧请求先返回也不能覆盖新 scope 的数据（requestId 守卫）。
  it('drops a stale session list response after a scope switch', async () => {
    const initial = getScope();
    const switchEnv = initial.env === 'demo-env' ? 'demo-env-b' : 'demo-env';
    const row = (id: string, host: string, at: string): DevopsSessionItem => ({
      sessionId: id,
      agentId: 'agent-a',
      operator: 'admin',
      protocol: 'ssh',
      host,
      port: 22,
      createdAt: at,
    });
    let resolveStale!: (value: DevopsSessionItem[]) => void;
    let resolveFresh!: (value: DevopsSessionItem[]) => void;
    mockedList
      .mockImplementationOnce(
        () =>
          new Promise<DevopsSessionItem[]>((resolve) => {
            resolveStale = resolve;
          }),
      )
      .mockImplementationOnce(
        () =>
          new Promise<DevopsSessionItem[]>((resolve) => {
            resolveFresh = resolve;
          }),
      )
      .mockResolvedValue([]);

    renderPanel();
    // 第一次拉取（挂载）已发出；切 scope 触发第二次。
    await waitFor(() => expect(mockedList).toHaveBeenCalledTimes(1));
    setScope({ ...initial, env: switchEnv }, { persist: false });
    await waitFor(() => expect(mockedList).toHaveBeenCalledTimes(2));

    // 新 scope 的结果先落地。
    resolveFresh([row('sess-fresh', '10.0.0.99', '2026-11-16 00:00:00')]);
    expect(await screen.findByText('sess-fresh')).toBeInTheDocument();
    // 旧 scope 的请求随后返回：不得覆盖。
    resolveStale([row('sess-stale', '10.0.0.1', '2026-11-16 01:00:00')]);
    expect(await screen.findByText('sess-fresh')).toBeInTheDocument();
    expect(screen.queryByText('sess-stale')).not.toBeInTheDocument();

    setScope({ ...initial, env: initial.env }, { persist: false });
  });
});
