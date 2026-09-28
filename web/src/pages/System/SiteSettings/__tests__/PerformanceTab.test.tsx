/**
 * 性能参数子 Tab 单测（OPEN-ISSUES #53）。
 *
 * 锁定契约：六键回填（cacheSize 字节↔MB 换算）、保存提交换算后的字节值、
 * 来源徽标（database→UI / config→配置文件 / default→无）、超限注记
 * （overload.cpu → 红色「超阈值」Tag）、失败翼 message 提示。
 *
 * mock 口径沿用同目录 MaintenanceTab.test.tsx：services 层 jest.mock、
 * @umijs/max 本地 mock；message 经真实 antd App 渲染进 portal。
 */
import React from 'react';
import { configure, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App, ConfigProvider } from 'antd';
import PerformanceTab from '../PerformanceTab';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

jest.mock('@/services/api/performance', () => ({
  fetchPerformance: jest.fn(),
  savePerformance: jest.fn(),
}));

jest.mock('@umijs/max', () => ({
  FormattedMessage: ({ defaultMessage }: { defaultMessage?: string }) => <>{defaultMessage}</>,
  useIntl: () => ({
    formatMessage: (opts: { defaultMessage?: string }) => opts.defaultMessage ?? '',
  }),
}));

import { fetchPerformance, savePerformance } from '@/services/api/performance';

const mFetch = fetchPerformance as jest.MockedFunction<typeof fetchPerformance>;
const mSave = savePerformance as jest.MockedFunction<typeof savePerformance>;

const MB = 1024 * 1024;

const baseSnapshot = {
  settings: {
    maxCpuPct: 0,
    maxMemoryPct: 0,
    maxDiskPct: 0,
    maxConcurrent: 0,
    maxThreadCount: 0,
    cacheSize: 0,
    sources: {},
  },
  runtime: {
    goMaxProcs: 14,
    goroutines: 42,
    heapAllocBytes: 128 * MB,
    heapSysBytes: 256 * MB,
    sysBytes: 512 * MB,
    numGC: 7,
    gcPauseMs: 12.3,
    uptimeSeconds: 3661,
  },
  host: {
    cpuPercent: 23.4,
    memoryUsedPct: 61.5,
    memoryTotalBytes: 32 * 1024 * MB,
    memoryUsedBytes: 19 * 1024 * MB,
    diskPath: '/data',
    diskUsedPct: 47.8,
    diskTotalBytes: 500 * 1024 * MB,
    diskUsedBytes: 230 * 1024 * MB,
  },
  overload: { cpu: false, memory: false, disk: false },
};

function renderTab() {
  return render(
    <App>
      <ConfigProvider>
        <PerformanceTab />
      </ConfigProvider>
    </App>,
  );
}

beforeEach(() => {
  jest.clearAllMocks();
  mFetch.mockResolvedValue(JSON.parse(JSON.stringify(baseSnapshot)));
  mSave.mockResolvedValue(JSON.parse(JSON.stringify(baseSnapshot)));
});

describe('PerformanceTab（OPEN-ISSUES #53）', () => {
  it('六键回填 + 运行时统计渲染（cacheSize 字节→MB）', async () => {
    mFetch.mockResolvedValue({
      ...baseSnapshot,
      settings: {
        ...baseSnapshot.settings,
        maxCpuPct: 80,
        maxConcurrent: 500,
        cacheSize: 256 * MB,
        sources: { maxCpuPct: 'database', maxConcurrent: 'config' },
      },
    });
    renderTab();

    expect(await screen.findByDisplayValue(80)).toBeInTheDocument();
    expect(screen.getByDisplayValue(500)).toBeInTheDocument();
    expect(screen.getByDisplayValue(256)).toBeInTheDocument(); // MB
    // 运行时
    expect(screen.getByText('14')).toBeInTheDocument();
    expect(screen.getByText('42')).toBeInTheDocument();
    expect(screen.getByText('128.0 MB')).toBeInTheDocument();
    expect(screen.getByText('512.0 MB')).toBeInTheDocument();
    expect(screen.getByText(/7 \/ 12\.3 ms/)).toBeInTheDocument();
    // 来源徽标：database→UI、config→配置文件、未覆盖键无徽标
    expect(screen.getByText('UI')).toBeInTheDocument();
    expect(screen.getByText('配置文件')).toBeInTheDocument();
  });

  it('保存：提交换算后的字节值，成功提示并回填新快照', async () => {
    mSave.mockResolvedValue({
      ...baseSnapshot,
      settings: { ...baseSnapshot.settings, maxCpuPct: 90, sources: { maxCpuPct: 'database' } },
    });
    renderTab();

    // 六键多数默认 0，CPU 输入框是 DOM 首个 0 值输入
    const cpuInput = (await screen.findAllByDisplayValue(0))[0];
    fireEvent.change(cpuInput, { target: { value: '90' } });
    fireEvent.change(screen.getByLabelText(/内存缓存/), { target: { value: '512' } });

    fireEvent.click(screen.getByRole('button', { name: '保存' }));

    await waitFor(() =>
      expect(mSave).toHaveBeenCalledWith(
        expect.objectContaining({
          'perf.maxCpuPct': 90,
          'perf.cacheSize': 512 * MB,
        }),
      ),
    );
    // 保存返回的新快照回填 + 成功提示
    expect(await screen.findByDisplayValue(90)).toBeInTheDocument();
    expect(await screen.findByText('性能参数已保存')).toBeInTheDocument();
  });

  it('超限注记：overload.cpu=true 渲染红色「超阈值」Tag', async () => {
    mFetch.mockResolvedValue({
      ...baseSnapshot,
      overload: { cpu: true, memory: false, disk: false },
    });
    renderTab();

    expect(await screen.findByText('超阈值')).toBeInTheDocument();
  });

  it('加载失败：message 提示、页面不白屏', async () => {
    mFetch.mockRejectedValue(new Error('boom'));
    renderTab();

    expect(await screen.findByText('boom')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '保存' })).toBeInTheDocument();
  });

  it('保存失败：message 提示、按钮退出 loading', async () => {
    mSave.mockRejectedValue(new Error('save down'));
    renderTab();

    fireEvent.click(await screen.findByRole('button', { name: '保存' }));
    expect(await screen.findByText('save down')).toBeInTheDocument();
    await waitFor(() => expect(screen.getByRole('button', { name: '保存' })).not.toBeDisabled());
  });
});
