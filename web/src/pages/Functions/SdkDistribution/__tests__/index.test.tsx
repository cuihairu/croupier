/*
 * Functions/SdkDistribution 实例元数据（BUG-028 后续 feature）回归测试
 *
 * provider 注册时声明的用户元数据（如 serverId=s1）必须：
 * 1. 在「实例明细」表渲染为 k=v 标签（>3 个折叠为 +N，Tooltip 摘要）；
 * 2. 参与实例搜索——直接粘元数据值（"s1"）或键名/整对都能命中；
 * 3. #2：元数据过滤走服务端聚合下拉（meta-options），不可从过滤后列表推导。
 */
import React from 'react';
import { act, configure, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App, ConfigProvider } from 'antd';
import SdkDistributionPage from '../index';
import { setScope } from '@/stores/scope';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

jest.mock('@/services/api/sdkStats', () => ({
  fetchSdkStats: jest.fn(),
  fetchProviderMetaOptions: jest.fn(),
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

const { fetchSdkStats, fetchProviderMetaOptions } = jest.requireMock('@/services/api/sdkStats') as {
  fetchSdkStats: jest.Mock;
  fetchProviderMetaOptions: jest.Mock;
};

// #2：meta-options 聚合（键→值→实例数），服务端 scoped
const META_OPTIONS = [
  { key: 'serverId', values: [{ value: 's1', count: 1 }] },
  { key: 'pod', values: [{ value: 'game-7c4d', count: 1 }] },
];

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
    firstSeenUnix: 1788900000,
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
    // 服务端零值归一回退后 firstSeen===lastSeen（旧快照无注册时间）
    firstSeenUnix: 1789000001,
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
  act(() => setScope({ gameId: 'default', env: 'dev' }));
  fetchSdkStats.mockResolvedValue({
    totalInstances: INSTANCES.length,
    languages: [
      { language: 'go', count: 1, versions: [{ version: '1.4.0', count: 1 }] },
      { language: 'js', count: 1, versions: [{ version: '1.5.0', count: 1 }] },
    ],
    instances: INSTANCES,
  });
  fetchProviderMetaOptions.mockResolvedValue(META_OPTIONS);
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

  // #38：SDK 实例按游戏隔离（服务端依 X-Game-ID/X-Env 过滤），
  // 顶栏切换游戏后页面必须重拉，否则列表停留在旧游戏。
  it('切换全局游戏 scope 后重新拉取 sdk-stats', async () => {
    renderPage();
    await screen.findByText('game-demo');
    const before = fetchSdkStats.mock.calls.length;

    act(() => setScope({ gameId: 'other-game', env: 'prod' }));
    await waitFor(() => expect(fetchSdkStats).toHaveBeenCalledTimes(before + 1));
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

  // #44：注册时间列——进程窗口语义（在线会话内存态，重启从零计起），
  // 与「最后活跃」对照可区分「实例活了多久」和「只是无响应」。
  it('注册时间列：firstSeenUnix 渲染为可悬停时间（含零值归一回退）', async () => {
    renderPage();
    await screen.findByText('game-demo');

    expect(screen.getByText('注册时间')).toBeInTheDocument();
    // 实例 1 的注册时间独立渲染；实例 2 是回退值 firstSeen===lastSeen，
    // 同一时刻同时出现在「注册时间」「最后活跃」两列 → 文本重复，用 *AllBy*
    expect(screen.getByText(new Date(1788900000 * 1000).toLocaleTimeString())).toBeInTheDocument();
    expect(screen.getAllByText(new Date(1789000001 * 1000).toLocaleTimeString()).length).toBe(2);
  });
});

describe('SdkDistribution 元数据过滤下拉（#2）', () => {
  const openKeySelect = async () => {
    renderPage();
    await screen.findByText('game-demo');
    const comboboxes = screen.getAllByRole('combobox');
    fireEvent.mouseDown(comboboxes[0]);
    await waitFor(() => {
      expect(screen.getByTitle('serverId (1)')).toBeInTheDocument();
    });
  };

  it('键下拉选项来自服务端 meta-options 聚合（含实例数 count）', async () => {
    await openKeySelect();
    expect(screen.getByTitle('pod (1)')).toBeInTheDocument();
    expect(screen.queryByTitle('不存在的键 (1)')).not.toBeInTheDocument();
    expect(fetchProviderMetaOptions).toHaveBeenCalled();
  });

  it('选键后以 metaKey 走服务端过滤并重拉；值下拉选项跟随所选键', async () => {
    await openKeySelect();
    fireEvent.click(screen.getByTitle('serverId (1)'));

    await waitFor(() => {
      expect(fetchSdkStats).toHaveBeenLastCalledWith({ metaKey: 'serverId', metaValue: undefined });
    });

    // 值下拉跟随所选键（epoch 重拉后仅含该键的值）
    const comboboxes = screen.getAllByRole('combobox');
    fireEvent.mouseDown(comboboxes[1]);
    await waitFor(() => {
      expect(screen.getByTitle('s1 (1)')).toBeInTheDocument();
    });
    expect(screen.queryByTitle('game-7c4d (1)')).not.toBeInTheDocument();

    fireEvent.click(screen.getByTitle('s1 (1)'));
    await waitFor(() => {
      expect(fetchSdkStats).toHaveBeenLastCalledWith({ metaKey: 'serverId', metaValue: 's1' });
    });
  });
});
