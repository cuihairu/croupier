/**
 * 观测配置子 Tab 单测（覆盖率巡检：0% → 行覆盖 100%）。
 *
 * 锁定契约：三个观测平台入口（Alertmanager/Grafana Explore/Jaeger）的读
 * （回填 + 来源三态徽标 database→数据库覆盖 / config→环境变量 / 缺省→未配置）、
 * 写（trim 落库、空值=清除覆盖走 clearSiteSetting、恢复按钮置空即存）、
 * 失败路径（加载/保存兜底文案、失败不重拉）与 Card loading 骨架收尾。
 *
 * mock 口径沿用同目录 index.test.tsx / NotificationTab.test.tsx。
 *
 * 边界（诚实）：saveField 的 `if (!meta) return;` 守卫经 UI 不可达——按钮
 * onClick 闭包只传 FIELDS 自有键，不造假用例；help 渲染为字段定义恒有项，
 * 无 help 缺省分支可补。
 */
import React from 'react';
import { configure, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { App, ConfigProvider } from 'antd';
import ObservabilityTab from '../ObservabilityTab';
import type { ObservabilitySettings } from '@/services/api/sites';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

jest.mock('@/services/api/sites', () => ({
  fetchObservabilitySettings: jest.fn(),
  setSiteSetting: jest.fn(),
  clearSiteSetting: jest.fn(),
}));

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
}));

import { clearSiteSetting, fetchObservabilitySettings, setSiteSetting } from '@/services/api/sites';

const mFetch = fetchObservabilitySettings as jest.MockedFunction<typeof fetchObservabilitySettings>;
const mSet = setSiteSetting as jest.MockedFunction<typeof setSiteSetting>;
const mClear = clearSiteSetting as jest.MockedFunction<typeof clearSiteSetting>;

const baseSettings: ObservabilitySettings = {
  alertmanagerUrl: 'http://alertmanager:9093',
  grafanaExploreUrl: 'http://grafana:3000/explore',
  jaegerUrl: '',
  sources: {
    'obs.alertmanagerUrl': 'database',
    'obs.grafanaExploreUrl': 'config',
  },
};

function renderTab() {
  return render(
    <App>
      <ConfigProvider>
        <ObservabilityTab />
      </ConfigProvider>
    </App>,
  );
}

/** 定位控件所在 Form.Item 内的按钮（三行各有「保存」，禁止全局 [0]） */
function buttonIn(control: HTMLElement, name: string): HTMLElement {
  const item = control.closest('.ant-form-item');
  expect(item).not.toBeNull();
  return within(item as HTMLElement).getByRole('button', { name });
}

beforeEach(() => {
  jest.clearAllMocks();
  mFetch.mockResolvedValue({ ...baseSettings, sources: { ...baseSettings.sources } });
  mSet.mockResolvedValue(undefined);
  mClear.mockResolvedValue(undefined);
});

describe('ObservabilityTab 加载', () => {
  it('回填三入口 + 来源三态徽标（数据库覆盖/环境变量/未配置）+ 恢复按钮仅覆盖行 + help 渲染', async () => {
    renderTab();

    expect(await screen.findByDisplayValue('http://alertmanager:9093')).toBeInTheDocument();
    expect(screen.getByDisplayValue('http://grafana:3000/explore')).toBeInTheDocument();
    // jaegerUrl 为空串：表单不回填（输入保持空）
    expect(screen.getByPlaceholderText('http://jaeger:16686')).toHaveValue('');

    expect(screen.getByText('数据库覆盖')).toBeInTheDocument();
    expect(screen.getByText('环境变量')).toBeInTheDocument();
    expect(screen.getByText('未配置')).toBeInTheDocument();

    // 恢复按钮仅在 database 来源行（alertmanager）
    const am = screen
      .getByDisplayValue('http://alertmanager:9093')
      .closest('.ant-form-item') as HTMLElement;
    expect(within(am).getByRole('button', { name: '恢复' })).toBeInTheDocument();
    const gr = screen
      .getByDisplayValue('http://grafana:3000/explore')
      .closest('.ant-form-item') as HTMLElement;
    expect(within(gr).queryByRole('button', { name: '恢复' })).not.toBeInTheDocument();

    // help 经 intl 渲染（字段定义恒有项）
    expect(screen.getByText('链路追踪查询入口')).toBeInTheDocument();
  });

  it('加载失败：错误提示走 extractErrorMessage 兜底，表单空、徽标全落「未配置」', async () => {
    mFetch.mockRejectedValue(undefined); // 非 Error 对象：锁定 fallback 分支
    renderTab();

    expect(await screen.findByText('加载观测配置失败')).toBeInTheDocument();
    expect(screen.getByPlaceholderText('http://alertmanager:9093')).toHaveValue('');
    expect(screen.getAllByText('未配置')).toHaveLength(3);
    expect(screen.queryByRole('button', { name: '恢复' })).not.toBeInTheDocument();
  });
});

describe('ObservabilityTab 保存（saveField）', () => {
  it('有值 trim 后提交 setSiteSetting + 「已保存并即时生效」+ 重拉', async () => {
    renderTab();
    const input = await screen.findByDisplayValue('http://grafana:3000/explore');
    fireEvent.change(input, { target: { value: '  http://grafana-new:3000/explore  ' } });
    fireEvent.click(buttonIn(input, '保存'));

    await waitFor(() =>
      expect(mSet).toHaveBeenCalledWith('obs.grafanaExploreUrl', 'http://grafana-new:3000/explore'),
    );
    expect(await screen.findByText('已保存并即时生效')).toBeInTheDocument();
    await waitFor(() => expect(mFetch).toHaveBeenCalledTimes(2));
    expect(mClear).not.toHaveBeenCalled();
  });

  it('空值提交 = 清除覆盖：走 clearSiteSetting 恢复跟随环境变量/默认', async () => {
    renderTab();
    const input = await screen.findByDisplayValue('http://grafana:3000/explore');
    fireEvent.change(input, { target: { value: '   ' } });
    fireEvent.click(buttonIn(input, '保存'));

    await waitFor(() => expect(mClear).toHaveBeenCalledWith('obs.grafanaExploreUrl'));
    expect(await screen.findByText('已保存并即时生效')).toBeInTheDocument();
    expect(mSet).not.toHaveBeenCalled();
  });

  it('保存失败：错误提示透出后端 message、不重拉、按钮退出 loading', async () => {
    mSet.mockRejectedValue(new Error('obs url invalid'));
    renderTab();
    const input = await screen.findByDisplayValue('http://grafana:3000/explore');
    fireEvent.change(input, { target: { value: 'http://bad' } });
    fireEvent.click(buttonIn(input, '保存'));

    expect(await screen.findByText('obs url invalid')).toBeInTheDocument();
    // finally 分支：savingKey 复位，按钮退出 loading
    await waitFor(() => expect(buttonIn(input, '保存')).not.toHaveClass('ant-btn-loading'));
    expect(mFetch).toHaveBeenCalledTimes(1);
  });
});

describe('ObservabilityTab 恢复', () => {
  it('恢复按钮：置空输入后立即保存 → clearSiteSetting 清除数据库覆盖', async () => {
    renderTab();
    const input = await screen.findByDisplayValue('http://alertmanager:9093');
    fireEvent.click(buttonIn(input, '恢复'));

    await waitFor(() => expect(mClear).toHaveBeenCalledWith('obs.alertmanagerUrl'));
    expect(await screen.findByText('已保存并即时生效')).toBeInTheDocument();
    await waitFor(() => expect(mFetch).toHaveBeenCalledTimes(2));
    expect(mSet).not.toHaveBeenCalled();
  });
});

describe('ObservabilityTab 渲染分支', () => {
  it('Card loading 结束后才渲染表单（loading→false finally 分支）', async () => {
    let resolveFetch: (v: ObservabilitySettings) => void = () => {};
    mFetch.mockImplementation(
      () =>
        new Promise<ObservabilitySettings>((res) => {
          resolveFetch = res;
        }),
    );
    renderTab();
    expect(screen.queryByPlaceholderText('http://alertmanager:9093')).not.toBeInTheDocument();
    resolveFetch({ ...baseSettings });
    expect(await screen.findByPlaceholderText('http://alertmanager:9093')).toBeInTheDocument();
  });
});
