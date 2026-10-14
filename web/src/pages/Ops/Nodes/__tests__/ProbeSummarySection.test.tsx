/*
 * 探针在线汇总卡测试（agent-health-recovery-review §13.2）
 *
 * 回归要点：
 *  - 异常行前置排序（先看有问题的）
 *  - checked=0 不显示 100%（没测过 ≠ 全成功）
 *  - 在线趋势「未接入」灰态常驻，不静默消失
 *  - 加载失败走行内 Alert，清空旧数据不残留陈旧结论
 */
import React from 'react';
import { configure, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App, ConfigProvider } from 'antd';
import ProbeSummarySection from '../ProbeSummarySection';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

jest.mock('@/services/api/opsProbe', () => ({
  getOpsProbeSummary: jest.fn(),
}));
jest.mock('@/hooks/useScopeReload', () => ({
  useScopeReload: jest.fn(),
}));
jest.mock('@umijs/max', () => ({
  useIntl: () => ({
    formatMessage: (
      opts: { defaultMessage?: string },
      values?: Record<string, string | number>,
    ) => {
      let text = opts.defaultMessage ?? '';
      if (values) {
        for (const [k, v] of Object.entries(values)) text = text.replace(`{${k}}`, String(v));
      }
      return text;
    },
  }),
}));

const { getOpsProbeSummary } = jest.requireMock('@/services/api/opsProbe') as {
  getOpsProbeSummary: jest.Mock;
};

const renderSection = () =>
  render(
    <App>
      <ConfigProvider>
        <ProbeSummarySection />
      </ConfigProvider>
    </App>,
  );

const ROW_OK = {
  agentId: 'agent-a',
  target: 'mysql',
  semantics: 'liveness',
  gameId: 'demo',
  env: 'prod',
  available: true,
  windowOpen: false,
  checked: 20,
  failed: 0,
  successRate: 1,
  lastReportAt: '2026-10-14T12:00:00Z',
};

const ROW_BAD = {
  agentId: 'agent-a',
  target: 'redis',
  semantics: 'readiness',
  gameId: 'demo',
  env: 'prod',
  available: false,
  windowOpen: true,
  windowDurationMs: 4200,
  checked: 10,
  failed: 2,
  successRate: 0.8,
  lastReason: 'dial tcp: i/o timeout',
  lastReportAt: '2026-10-14T12:00:00Z',
};

const ROW_NO_SAMPLE = {
  agentId: 'server',
  target: 'postgres',
  semantics: 'readiness',
  available: true,
  windowOpen: false,
  checked: 0,
  failed: 0,
  successRate: 0,
  lastReportAt: '2026-10-14T12:00:00Z',
};

beforeEach(() => {
  jest.clearAllMocks();
});

describe('Ops/Nodes 探针汇总卡', () => {
  it('渲染行数据，异常行排在正常行之前', async () => {
    getOpsProbeSummary.mockResolvedValue({
      onlineDataIntegrated: false,
      rows: [ROW_OK, ROW_BAD],
    });
    renderSection();

    await screen.findByText('redis');
    expect(screen.getByText('mysql')).toBeInTheDocument();
    // 异常证据照实呈现：不可用标签 + 窗口秒数 + 失败原因
    expect(screen.getByText('不可用')).toBeInTheDocument();
    expect(screen.getByText('4.2s')).toBeInTheDocument();
    expect(screen.getByText('dial tcp: i/o timeout')).toBeInTheDocument();
    expect(screen.getByText('可用')).toBeInTheDocument();

    const rows = screen.getAllByRole('row');
    // 表头 + 两行数据，异常行在前
    expect(rows[1]).toHaveTextContent('redis');
    expect(rows[2]).toHaveTextContent('mysql');
  });

  it('checked=0 不渲染 100%（指标缺失）', async () => {
    getOpsProbeSummary.mockResolvedValue({
      onlineDataIntegrated: false,
      rows: [ROW_NO_SAMPLE],
    });
    renderSection();

    await screen.findByText('postgres');
    expect(screen.queryByText('100.0%')).not.toBeInTheDocument();
    // 平台级行标注而不留空
    expect(screen.getByText('平台级')).toBeInTheDocument();
  });

  it('在线趋势未接入常驻灰态提示', async () => {
    getOpsProbeSummary.mockResolvedValue({
      onlineDataIntegrated: false,
      rows: [ROW_OK],
    });
    renderSection();

    await screen.findByText('在线趋势：未接入在线数据');
  });

  it('空态给出配置侧排查提示', async () => {
    getOpsProbeSummary.mockResolvedValue({ onlineDataIntegrated: false, rows: [] });
    renderSection();

    await screen.findByText(/暂无探针数据/);
  });

  it('加载失败走行内 Alert 并清空旧数据', async () => {
    getOpsProbeSummary.mockResolvedValueOnce({
      onlineDataIntegrated: false,
      rows: [ROW_BAD],
    });
    renderSection();
    await screen.findByText('redis');

    getOpsProbeSummary.mockRejectedValueOnce(new Error('boom'));
    fireEvent.click(screen.getByRole('button', { name: /刷新探针汇总/ }));

    await waitFor(() => expect(screen.getByText('boom')).toBeInTheDocument());
    expect(screen.queryByText('redis')).not.toBeInTheDocument();
  });
});
