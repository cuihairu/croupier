/**
 * 运维/系统维护子 Tab 单测（OPEN-ISSUES #52）。
 *
 * 锁定契约：运行快照渲染（版本/Git 提交/构建时间/启动时间/在线时长人性化）、
 * 文档链接按配置存在才渲染（#49 划归入口）、「检查更新」按钮三态结果
 * （未配置源→info / 已是最新→success / 发现新版本→warning）与失败路径。
 *
 * mock 口径沿用同目录 SecurityTab.test.tsx：services 层 jest.mock、
 * @umijs/max 本地 mock；message 提示经真实 antd App 渲染进 portal，用
 * DOM 文本断言。
 */
import React from 'react';
import { configure, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App, ConfigProvider } from 'antd';
import MaintenanceTab, { humanizeUptime } from '../MaintenanceTab';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

jest.mock('@/services/api/opsStatus', () => ({
  getSystemRuntime: jest.fn(),
  checkSystemUpdate: jest.fn(),
}));

jest.mock('@/services/api/sites', () => ({
  fetchSiteConfig: jest.fn(),
}));

jest.mock('@umijs/max', () => ({
  FormattedMessage: ({ defaultMessage }: { defaultMessage?: string }) => <>{defaultMessage}</>,
  useIntl: () => ({
    formatMessage: (opts: { defaultMessage?: string }) => opts.defaultMessage ?? '',
  }),
}));

import { checkSystemUpdate, getSystemRuntime } from '@/services/api/opsStatus';
import { fetchSiteConfig } from '@/services/api/sites';

const mRuntime = getSystemRuntime as jest.MockedFunction<typeof getSystemRuntime>;
const mCheck = checkSystemUpdate as jest.MockedFunction<typeof checkSystemUpdate>;
const mCfg = fetchSiteConfig as jest.MockedFunction<typeof fetchSiteConfig>;

const baseRuntime = {
  version: 'v1.4.0',
  gitCommit: 'abc1234',
  buildTime: '2026-09-27T20:30:00Z',
  startedAt: '2026-09-28T08:00:00Z',
  uptimeSeconds: 90061,
};

function renderTab() {
  return render(
    <App>
      <ConfigProvider>
        <MaintenanceTab />
      </ConfigProvider>
    </App>,
  );
}

beforeEach(() => {
  jest.clearAllMocks();
  mRuntime.mockResolvedValue({ ...baseRuntime });
  mCfg.mockResolvedValue({ docsUrl: 'https://docs.example.com' } as Awaited<
    ReturnType<typeof fetchSiteConfig>
  >);
});

describe('humanizeUptime', () => {
  it('分段人性化且省略前导零单位', () => {
    expect(humanizeUptime(90061)).toBe('1 天 1 小时 1 分钟 1 秒');
    expect(humanizeUptime(3600)).toBe('1 小时');
    expect(humanizeUptime(59)).toBe('59 秒');
  });

  it('未知/非法值回「—」', () => {
    expect(humanizeUptime(undefined)).toBe('—');
    expect(humanizeUptime(-1)).toBe('—');
    expect(humanizeUptime(0)).toBe('0 秒');
  });
});

describe('MaintenanceTab', () => {
  it('渲染运行快照与文档链接', async () => {
    renderTab();
    expect(await screen.findByText('v1.4.0')).toBeInTheDocument();
    expect(screen.getByText('abc1234')).toBeInTheDocument();
    expect(screen.getByText('2026-09-28T08:00:00Z')).toBeInTheDocument();
    expect(screen.getByText('1 天 1 小时 1 分钟 1 秒')).toBeInTheDocument();
    const docs = screen.getByRole('link', { name: 'https://docs.example.com' });
    expect(docs).toHaveAttribute('href', 'https://docs.example.com');
    expect(docs).toHaveAttribute('target', '_blank');
  });

  it('文档链接未配置时显示占位', async () => {
    mCfg.mockResolvedValue({ docsUrl: '' } as Awaited<ReturnType<typeof fetchSiteConfig>>);
    renderTab();
    await screen.findByText('v1.4.0');
    expect(screen.queryByRole('link')).not.toBeInTheDocument();
  });

  it('检查更新（未配置源）→ info 注记含「未配置」与「不自动执行升级」', async () => {
    mCheck.mockResolvedValue({
      currentVersion: 'v1.4.0',
      hasUpdate: false,
      checked: false,
      note: '未配置更新检查源（settings: system.updateCheckUrl）。自动更新未实现：本入口仅做版本检查，不自动执行升级（#52 最小收口）。',
    });
    renderTab();
    fireEvent.click(await screen.findByRole('button', { name: /检查更新/ }));
    await waitFor(() => {
      expect(screen.getByText(/未配置更新检查源/)).toBeInTheDocument();
    });
    expect(screen.getByText(/不自动执行升级/)).toBeInTheDocument();
  });

  it('检查更新（发现新版本）→ warning 结果', async () => {
    mCheck.mockResolvedValue({
      currentVersion: 'v1.4.0',
      latestVersion: 'v9.9.9',
      hasUpdate: true,
      checked: true,
      note: '发现新版本 v9.9.9（当前 v1.4.0）。',
    });
    renderTab();
    fireEvent.click(await screen.findByRole('button', { name: /检查更新/ }));
    await waitFor(() => {
      expect(screen.getByText('发现新版本')).toBeInTheDocument();
    });
    expect(screen.getByText(/v9.9.9/)).toBeInTheDocument();
  });

  it('检查更新（已是最新）→ success 结果', async () => {
    mCheck.mockResolvedValue({
      currentVersion: 'v1.4.0',
      hasUpdate: false,
      checked: true,
      note: '当前已是最新版本（v1.4.0）。',
    });
    renderTab();
    fireEvent.click(await screen.findByRole('button', { name: /检查更新/ }));
    await waitFor(() => {
      expect(screen.getByText(/已是最新版本/)).toBeInTheDocument();
    });
  });

  it('检查更新接口失败 → portal 内错误提示', async () => {
    mCheck.mockRejectedValue(new Error('boom'));
    renderTab();
    fireEvent.click(await screen.findByRole('button', { name: /检查更新/ }));
    // 错误 toast 文本随环境波动，按 house 口径断言错误 notice 弹出
    await waitFor(() => expect(document.querySelector('.ant-message-notice-error')).toBeTruthy());
  });

  it('运行信息加载失败 → portal 内错误提示', async () => {
    mRuntime.mockRejectedValue(new Error('net down'));
    renderTab();
    await waitFor(() => expect(document.querySelector('.ant-message-notice-error')).toBeTruthy());
  });
});
