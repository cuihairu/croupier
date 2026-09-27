/*
 * OpenAPI Sources 运行时导入区块回归测试（#27①②③④）
 *
 * 运行时导入表必须补齐注册链观测列：实例元数据（k=v 标签 +N 折叠）、
 * 目标地址（被调用方 IP:端口）、注册版本/最新版本、导入时间；来源 Agent
 * 列可点击跳转 Ops 节点页深链定位。
 */
import React from 'react';
import { act, configure, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App, ConfigProvider } from 'antd';
import OpenAPISourcesPage from '../index';
import { formatDate } from '../shared';
import type { RuntimeProviderItem } from '@/services/api/openapi';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

jest.mock('@/services/api/openapi', () => ({
  listOpenAPISources: jest.fn(),
  listRuntimeSources: jest.fn(),
  getOpenAPISource: jest.fn(),
  createOpenAPISource: jest.fn(),
  updateOpenAPISource: jest.fn(),
  uploadOpenAPISourceFile: jest.fn(),
  bindOpenAPISourceProvider: jest.fn(),
  deleteOpenAPISourceBinding: jest.fn(),
}));
jest.mock('@/services/api/functions', () => ({
  listDescriptors: jest.fn(),
}));
jest.mock('@/stores/scope', () => ({
  isScopeReady: jest.fn(),
  subscribeScope: jest.fn(),
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
  useAccess: () => ({ canOpenAPISourcesWrite: false }),
  history: { push: jest.fn() },
}));

const { listOpenAPISources, listRuntimeSources } = jest.requireMock('@/services/api/openapi') as {
  listOpenAPISources: jest.Mock;
  listRuntimeSources: jest.Mock;
};
const { listDescriptors } = jest.requireMock('@/services/api/functions') as {
  listDescriptors: jest.Mock;
};
const { isScopeReady, subscribeScope } = jest.requireMock('@/stores/scope') as {
  isScopeReady: jest.Mock;
  subscribeScope: jest.Mock;
};
const { history } = jest.requireMock('@umijs/max') as { history: { push: jest.Mock } };

const RUNTIME_ITEMS: RuntimeProviderItem[] = [
  {
    providerId: 'provider:players',
    name: 'players',
    agentId: 'openapi-demo-agent',
    gameId: 'default',
    env: 'dev',
    version: '1.1.0',
    latestVersion: '1.4.0',
    serviceAddr: '10.0.0.8:9001',
    firstSeenUnix: 1789000000,
    metadata: { serverId: 's1', region: 'cn-north', pod: 'p1', extra: 'x' },
    functionCount: 2,
    functions: ['players.player.get', 'players.player.list'],
    lastSeenUnix: 1789000100,
  },
];

const renderPage = () =>
  render(
    <App>
      <ConfigProvider>
        <OpenAPISourcesPage />
      </ConfigProvider>
    </App>,
  );

beforeEach(() => {
  jest.clearAllMocks();
  act(() => {
    isScopeReady.mockReturnValue(true);
    subscribeScope.mockReturnValue(() => {});
    listOpenAPISources.mockResolvedValue({ items: [] });
    listRuntimeSources.mockResolvedValue({ items: RUNTIME_ITEMS });
    listDescriptors.mockResolvedValue([]);
  });
});

describe('OpenAPISources 运行时导入（#27①②③④）', () => {
  it('渲染元数据 k=v 标签（>3 折叠 +N）与目标地址', async () => {
    renderPage();
    await screen.findByText('players');

    expect(screen.getByText('serverId=s1')).toBeInTheDocument();
    expect(screen.getByText('region=cn-north')).toBeInTheDocument();
    expect(screen.getByText('pod=p1')).toBeInTheDocument();
    expect(screen.getByText('+1')).toBeInTheDocument();
    expect(screen.queryByText('extra=x')).not.toBeInTheDocument();

    expect(screen.getByText('10.0.0.8:9001')).toBeInTheDocument();
  });

  it('渲染注册版本/最新版本与导入时间', async () => {
    renderPage();
    await screen.findByText('players');

    // 注册版本如实展示；最新版本（高水位 1.4.0 ≠ 注册版本 1.1.0）高亮提示
    expect(screen.getByText('1.1.0')).toBeInTheDocument();
    expect(screen.getByText('1.4.0')).toBeInTheDocument();
    expect(
      screen.getByText(formatDate(new Date(RUNTIME_ITEMS[0].firstSeenUnix * 1000).toISOString())),
    ).toBeInTheDocument();
  });

  it('来源 Agent 可点击跳转 Ops 节点页深链', async () => {
    renderPage();
    await screen.findByText('players');

    fireEvent.click(screen.getByText('openapi-demo-agent'));
    await waitFor(() => {
      expect(history.push).toHaveBeenCalledWith('/ops/nodes?agentId=openapi-demo-agent');
    });
  });
});
