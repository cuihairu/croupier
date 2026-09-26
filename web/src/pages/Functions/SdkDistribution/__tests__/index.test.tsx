/*
 * Functions/SdkDistribution 实例元数据（BUG-028 后续 feature）回归测试
 *
 * provider 注册时声明的用户元数据（如 serverId=s1）必须：
 * 1. 在「实例明细」表渲染为 k=v 标签（>3 个折叠为 +N，Tooltip 摘要）；
 * 2. 参与实例搜索——直接粘元数据值（"s1"）或键名/整对都能命中。
 */
import React from 'react';
import { configure, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App, ConfigProvider } from 'antd';
import SdkDistributionPage from '../index';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

jest.mock('@/services/api/sdkStats', () => ({
  fetchSdkStats: jest.fn(),
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
}));

const { fetchSdkStats } = jest.requireMock('@/services/api/sdkStats') as {
  fetchSdkStats: jest.Mock;
};

const INSTANCES = [
  {
    providerId: 'game-demo',
    agentId: 'agent-1',
    gameId: 'default',
    env: 'dev',
    sdkLanguage: 'go',
    sdkVersion: '1.4.0',
    sdkName: 'croupier-go-sdk',
    metadata: { serverId: 's1', pod: 'game-7c4d', region: 'cn-north', extra: 'x' },
    lastSeenUnix: 1789000000,
  },
  {
    providerId: 'prom-adapter',
    agentId: 'agent-2',
    gameId: 'default',
    env: 'dev',
    sdkLanguage: 'js',
    sdkVersion: '1.5.0',
    metadata: undefined,
    lastSeenUnix: 1789000001,
  },
];

const renderPage = () =>
  render(
    <App>
      <ConfigProvider>
        <SdkDistributionPage />
      </ConfigProvider>
    </App>,
  );

beforeEach(() => {
  jest.clearAllMocks();
  fetchSdkStats.mockResolvedValue({
    totalInstances: INSTANCES.length,
    languages: [
      { language: 'go', count: 1, versions: [{ version: '1.4.0', count: 1 }] },
      { language: 'js', count: 1, versions: [{ version: '1.5.0', count: 1 }] },
    ],
    instances: INSTANCES,
  });
});

describe('SdkDistribution 实例元数据', () => {
  it('渲染元数据 k=v 标签，超过 3 个折叠为 +N', async () => {
    renderPage();
    await screen.findByText('game-demo');

    expect(screen.getByText('serverId=s1')).toBeInTheDocument();
    expect(screen.getByText('pod=game-7c4d')).toBeInTheDocument();
    expect(screen.getByText('region=cn-north')).toBeInTheDocument();
    // 第 4 个键折叠进 +N，不再单独展开
    expect(screen.getByText('+1')).toBeInTheDocument();
    expect(screen.queryByText('extra=x')).not.toBeInTheDocument();
    // 无元数据实例显示占位
    expect(screen.getByText('prom-adapter')).toBeInTheDocument();
  });

  it('搜索元数据值可直接命中实例', async () => {
    renderPage();
    await screen.findByText('game-demo');

    const input = screen.getByPlaceholderText('搜索 provider / agent / 版本 / 元数据…');
    fireEvent.change(input, { target: { value: 's1' } });
    // Input.Search 回车触发 onSearch（antd v6 搜索按钮类名不稳定，回车等效）
    fireEvent.keyDown(input, { key: 'Enter' });

    await waitFor(() => {
      expect(screen.getByText('game-demo')).toBeInTheDocument();
      expect(screen.queryByText('prom-adapter')).not.toBeInTheDocument();
    });
  });
});
