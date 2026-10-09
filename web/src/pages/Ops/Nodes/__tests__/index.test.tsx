/*
 * Ops/Nodes 节点维护页回归测试
 *
 * #27④：支持 ?agentId= 深链（OpenAPI Sources 运行时导入的来源 Agent
 * 跳转），进入页面即把 agentId 预填进关键字过滤，列表只剩匹配节点。
 */
import React from 'react';
import { act, configure, fireEvent, render, screen } from '@testing-library/react';
import { App, ConfigProvider } from 'antd';
import OpsNodesPage from '../index';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

jest.mock('@/services/api/ops', () => ({
  // requireActual 展开：SupervisorDrawer 关闭态渲染也消费导出常量
  // （SUPERVISOR_EVENT_TYPES），部分 mock 缺键会让页面渲染直接炸。
  ...jest.requireActual('@/services/api/ops'),
  listOpsNodes: jest.fn(),
  drainOpsNode: jest.fn(),
  restartOpsNode: jest.fn(),
  undrainOpsNode: jest.fn(),
}));
jest.mock('@/services/api/registry', () => ({
  fetchRegistry: jest.fn(),
}));
jest.mock('@umijs/max', () => ({
  FormattedMessage: ({ defaultMessage }: { id: string; defaultMessage?: string }) => (
    <>{defaultMessage ?? ''}</>
  ),
  useIntl: () => ({
    formatMessage: (opts: { defaultMessage?: string }, values?: Record<string, string>) => {
      let text = opts.defaultMessage ?? '';
      if (values) {
        for (const [k, v] of Object.entries(values)) text = text.replace(`{${k}}`, v);
      }
      return text;
    },
  }),
  useLocation: () => ({ search: mockLocationSearch }),
}));

const { listOpsNodes } = jest.requireMock('@/services/api/ops') as {
  listOpsNodes: jest.Mock;
};

// 由用例改写以模拟不同入口 URL
let mockLocationSearch = '';

const NODES = [
  {
    id: 'agent-42',
    addr: '10.1.1.42:19091',
    status: 'active',
    gameId: 'demo',
    env: 'dev',
  },
  {
    id: 'agent-7',
    addr: '10.1.1.7:19091',
    status: 'active',
    gameId: 'demo',
    env: 'dev',
  },
];

const renderPage = () =>
  render(
    <App>
      <ConfigProvider>
        <OpsNodesPage />
      </ConfigProvider>
    </App>,
  );

beforeEach(() => {
  jest.clearAllMocks();
  mockLocationSearch = '';
  act(() => {
    listOpsNodes.mockResolvedValue({ nodes: NODES });
  });
});

describe('Ops/Nodes 节点维护页（#27④ 深链）', () => {
  it('无 query 时展示全部节点，关键字为空', async () => {
    renderPage();
    expect(await screen.findByText('agent-42')).toBeInTheDocument();
    expect(screen.getByText('agent-7')).toBeInTheDocument();
    const input = screen.getByPlaceholderText('搜索节点 ID / IP') as HTMLInputElement;
    expect(input.value).toBe('');
  });

  it('?agentId= 深链预填关键字过滤，只留匹配节点', async () => {
    mockLocationSearch = '?agentId=agent-42';
    renderPage();
    expect(await screen.findByText('agent-42')).toBeInTheDocument();

    const input = screen.getByPlaceholderText('搜索节点 ID / IP') as HTMLInputElement;
    expect(input.value).toBe('agent-42');
    expect(screen.queryByText('agent-7')).not.toBeInTheDocument();

    // 深链只是预填，用户仍可手动改关键字恢复全量视图
    fireEvent.change(input, { target: { value: '' } });
    await screen.findByText('agent-7');
  });
});
