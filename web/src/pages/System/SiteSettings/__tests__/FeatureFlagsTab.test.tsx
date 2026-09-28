/**
 * 功能开关子 Tab 单测（覆盖率巡检：0% → 行覆盖 100%）。
 *
 * 锁定契约：五域开关读（合成值回显 + 来源三态徽标 + 缺省域兜底）/写
 * （toggle 开/关文案、成功链重拉 + fetchServerFeatures + setInitialState 全局
 * features 缓存刷新）/清除覆盖（Popconfirm 确认后 clearSiteSetting）与三条
 * 失败路径（加载/开关/恢复均「操作失败」兜底、失败不触达全局同步）。
 *
 * mock 口径沿用同目录 index.test.tsx / NotificationTab.test.tsx；本 Tab 额外
 * 依赖 useModel('@@initialState')（mock 闭包单例）与 fetchServerFeatures。
 *
 * 边界（诚实）：trimmedByConfig 且未开启时开关 disabled 属物理裁剪（L2），
 * 只能改配置重启——用例断言 disabled 与「裁剪但已开启仍可关」两翼，不mock 后端。
 */
import React from 'react';
import { configure, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { App, ConfigProvider } from 'antd';
import FeatureFlagsTab from '../FeatureFlagsTab';
import type { FeatureDomainState, FeatureSnapshot } from '@/services/api/sites';
import type { ServerFeatures } from '@/services/api/features';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

jest.mock('@/services/api/sites', () => ({
  fetchFeatureSettings: jest.fn(),
  setSiteSetting: jest.fn(),
  clearSiteSetting: jest.fn(),
}));

jest.mock('@/services/api/features', () => ({
  fetchServerFeatures: jest.fn(),
}));

// mock* 前缀变量：babel-jest hoist 白名单，允许 mock 工厂延迟绑定
const mockSetInitialState = jest.fn();

jest.mock('@umijs/max', () => ({
  FormattedMessage: ({
    defaultMessage,
    values,
  }: {
    defaultMessage?: string;
    values?: Record<string, string>;
  }) => {
    let text = defaultMessage ?? '';
    if (values) {
      for (const [k, v] of Object.entries(values)) text = text.replace(`{${k}}`, String(v));
    }
    return <>{text}</>;
  },
  useIntl: () => ({
    formatMessage: (opts: { defaultMessage?: string }, values?: Record<string, string>) => {
      let text = opts.defaultMessage ?? '';
      if (values) {
        for (const [k, v] of Object.entries(values)) text = text.replace(`{${k}}`, String(v));
      }
      return text;
    },
  }),
  useModel: () => ({ setInitialState: mockSetInitialState }),
}));

import { clearSiteSetting, fetchFeatureSettings, setSiteSetting } from '@/services/api/sites';
import { fetchServerFeatures } from '@/services/api/features';

const mFetch = fetchFeatureSettings as jest.MockedFunction<typeof fetchFeatureSettings>;
const mSet = setSiteSetting as jest.MockedFunction<typeof setSiteSetting>;
const mClear = clearSiteSetting as jest.MockedFunction<typeof clearSiteSetting>;
const mServerFeatures = fetchServerFeatures as jest.MockedFunction<typeof fetchServerFeatures>;
const mSetInitial = mockSetInitialState;

const domainState = (over: Partial<FeatureDomainState>): FeatureDomainState => ({
  enabled: true,
  trimmedByConfig: false,
  overridden: false,
  ...over,
});

const snapshotOf = (over: {
  dev?: Partial<FeatureDomainState>;
  support?: Partial<FeatureDomainState>;
  analytics?: Partial<FeatureDomainState>;
  ops?: Partial<FeatureDomainState>;
}): FeatureSnapshot => ({
  domains: {
    dev: domainState(over.dev ?? {}),
    support: domainState(over.support ?? {}),
    analytics: domainState(over.analytics ?? {}),
    ops: domainState(over.ops ?? {}),
    // extensions 故意缺省：走 snapshot?.domains?.[domain] undefined 翼（UI 默认开启）
  } as FeatureSnapshot['domains'],
});

const serverFeatures: ServerFeatures = {
  dev: true,
  support: true,
  analytics: true,
  ops: true,
  extensions: true,
};

/** 以「功能域」单元格文本定位表格行 */
function rowOf(domainLabel: string): HTMLElement {
  const cell = screen.getByText(domainLabel);
  const row = cell.closest('tr');
  expect(row).not.toBeNull();
  return row as HTMLElement;
}

function renderTab() {
  return render(
    <App>
      <ConfigProvider>
        <FeatureFlagsTab />
      </ConfigProvider>
    </App>,
  );
}

beforeEach(() => {
  jest.clearAllMocks();
  mFetch.mockResolvedValue(snapshotOf({}));
  mSet.mockResolvedValue(undefined);
  mClear.mockResolvedValue(undefined);
  mServerFeatures.mockResolvedValue(serverFeatures);
  mSetInitial.mockResolvedValue(undefined);
});

describe('FeatureFlagsTab 加载', () => {
  it('五域回显：来源三态徽标、开关合成值、裁剪禁用两翼、缺省域兜底、恢复按钮仅覆盖行可见', async () => {
    mFetch.mockResolvedValue(
      snapshotOf({
        dev: { enabled: true },
        support: { enabled: false, overridden: true },
        analytics: { enabled: false, trimmedByConfig: true },
        ops: { enabled: true, trimmedByConfig: true },
      }),
    );
    renderTab();

    expect(await screen.findByText('研发协作')).toBeInTheDocument();

    // dev：跟随部署配置（蓝）、开关开、无恢复按钮
    const dev = rowOf('研发协作');
    expect(within(dev).getByText('跟随部署配置')).toBeInTheDocument();
    expect(within(dev).getByRole('switch')).toBeChecked();
    expect(within(dev).queryByRole('button', { name: '恢复' })).not.toBeInTheDocument();

    // support：数据库覆盖（橙）、开关关、恢复按钮可见
    const support = rowOf('客服系统');
    expect(within(support).getByText('数据库覆盖')).toBeInTheDocument();
    expect(within(support).getByRole('switch')).not.toBeChecked();
    expect(within(support).getByRole('button', { name: '恢复' })).toBeInTheDocument();

    // analytics：部署已裁剪（红）且未开启 → 开关禁用（物理裁剪只能改配置重启）
    const analytics = rowOf('数据分析');
    expect(within(analytics).getByText('部署已裁剪（重启生效）')).toBeInTheDocument();
    expect(within(analytics).getByRole('switch')).toBeDisabled();

    // ops：裁剪但合成值已开 → 开关不禁用（trimmedByConfig && !enabled 右翼）
    const ops = rowOf('运维中心');
    expect(within(ops).getByRole('switch')).toBeEnabled();
    expect(within(ops).getByRole('switch')).toBeChecked();

    // extensions：snapshot 缺该域 → UI 兜底默认开启 + 跟随部署配置
    const extensions = rowOf('扩展中心');
    expect(within(extensions).getByRole('switch')).toBeChecked();
    expect(within(extensions).getByText('跟随部署配置')).toBeInTheDocument();
    expect(screen.getAllByText('跟随部署配置')).toHaveLength(2);
  });

  it('加载失败：错误提示走 extractErrorMessage 兜底，表格仍渲染且全行默认开启', async () => {
    mFetch.mockRejectedValue(undefined); // 非 Error 对象：锁定 fallback 分支
    renderTab();

    expect(await screen.findByText('加载功能开关失败')).toBeInTheDocument();
    expect(screen.getByText('研发协作')).toBeInTheDocument();
    expect(rowOf('研发协作').querySelector('[role="switch"]')).not.toBeNull();
    expect(rowOf('研发协作').querySelector('[role="switch"]')).toBeChecked();
  });
});

describe('FeatureFlagsTab 开关（toggle）', () => {
  it('开启 support：setSiteSetting(key, true) + 「已开启」文案 + 重拉并刷新全局 features 缓存', async () => {
    // support 初始为停用态：点击才触发 next=true 翼（默认快照全域开启会翻成 false）
    mFetch.mockResolvedValue(snapshotOf({ support: { enabled: false, overridden: true } }));
    renderTab();
    fireEvent.click(
      within(
        await screen.findByText('客服系统').then((el) => el.closest('tr') as HTMLElement),
      ).getByRole('switch'),
    );

    await waitFor(() => expect(mSet).toHaveBeenCalledWith('features.support', true));
    expect(await screen.findByText('已开启，界面菜单即时生效')).toBeInTheDocument();
    // 成功链：重拉快照 + 拉服务端合成值 + setInitialState 注入全局（菜单/路由显隐跟随）
    await waitFor(() => expect(mFetch).toHaveBeenCalledTimes(2));
    expect(mServerFeatures).toHaveBeenCalledTimes(1);
    expect(mSetInitial).toHaveBeenCalledTimes(1);
    expect(mClear).not.toHaveBeenCalled();
  });

  it('关闭 dev：setSiteSetting(key, false) + 「已停用」文案', async () => {
    renderTab();
    fireEvent.click(
      within(
        await screen.findByText('研发协作').then((el) => el.closest('tr') as HTMLElement),
      ).getByRole('switch'),
    );

    await waitFor(() => expect(mSet).toHaveBeenCalledWith('features.dev', false));
    expect(await screen.findByText('已停用，对应菜单与接口同步隐藏')).toBeInTheDocument();
  });

  it('开关失败：「操作失败」兜底，失败不触达全局 features 同步', async () => {
    mSet.mockRejectedValue(undefined); // 无可提取信息：锁定「操作失败」fallback
    renderTab();
    fireEvent.click(
      within(
        await screen.findByText('研发协作').then((el) => el.closest('tr') as HTMLElement),
      ).getByRole('switch'),
    );

    expect(await screen.findByText('操作失败')).toBeInTheDocument();
    expect(mFetch).toHaveBeenCalledTimes(1);
    expect(mServerFeatures).not.toHaveBeenCalled();
    expect(mSetInitial).not.toHaveBeenCalled();
  });
});

describe('FeatureFlagsTab 恢复（clearOverride）', () => {
  it('Popconfirm 确认后 clearSiteSetting + 「已恢复跟随部署配置」+ 重拉并同步全局', async () => {
    mFetch.mockResolvedValue(snapshotOf({ support: { enabled: false, overridden: true } }));
    renderTab();
    fireEvent.click(
      within(
        await screen.findByText('客服系统').then((el) => el.closest('tr') as HTMLElement),
      ).getByRole('button', { name: '恢复' }),
    );

    // 确认气泡：标题出现后点击 OK（antd 默认 locale）
    expect(await screen.findByText('删除数据库覆盖？')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'OK' }));

    await waitFor(() => expect(mClear).toHaveBeenCalledWith('features.support'));
    expect(await screen.findByText('已恢复跟随部署配置')).toBeInTheDocument();
    await waitFor(() => expect(mFetch).toHaveBeenCalledTimes(2));
    expect(mServerFeatures).toHaveBeenCalledTimes(1);
    expect(mSet).not.toHaveBeenCalled();
  });

  it('恢复失败：「操作失败」兜底，不触达全局同步', async () => {
    mClear.mockRejectedValue(new Error('db readonly'));
    mFetch.mockResolvedValue(snapshotOf({ support: { enabled: false, overridden: true } }));
    renderTab();
    fireEvent.click(
      within(
        await screen.findByText('客服系统').then((el) => el.closest('tr') as HTMLElement),
      ).getByRole('button', { name: '恢复' }),
    );
    fireEvent.click(await screen.findByRole('button', { name: 'OK' }));

    expect(await screen.findByText('db readonly')).toBeInTheDocument();
    expect(mFetch).toHaveBeenCalledTimes(1);
    expect(mSetInitial).not.toHaveBeenCalled();
  });
});

describe('FeatureFlagsTab 渲染分支', () => {
  it('Card loading 结束后才渲染表格（loading→false finally 分支）', async () => {
    let resolveFetch: (v: FeatureSnapshot) => void = () => {};
    mFetch.mockImplementation(
      () =>
        new Promise<FeatureSnapshot>((res) => {
          resolveFetch = res;
        }),
    );
    renderTab();
    expect(screen.queryByText('研发协作')).not.toBeInTheDocument();
    resolveFetch(snapshotOf({}));
    expect(await screen.findByText('研发协作')).toBeInTheDocument();
  });
});
