/*
 * 探针在线汇总卡测试（agent-health-recovery-review §13.2）
 *
 * 回归要点：
 *  - 异常行前置排序（先看有问题的）
 *  - checked=0 不显示 100%（没测过 ≠ 全成功）
 *  - 在线趋势「未接入」灰态常驻，不静默消失
 *  - 加载失败走行内 Alert，清空旧数据不残留陈旧结论
 *  - 协议目标按失败阶段区分 不可达 / 协议失败 / 已验证（§5.5.3 标注面）
 *  - 声明协议期望的 L1 目标标 可达≠可用 + 未接入协议探测（reachable-unverified）
 *  - 旧版 agent 未上报类型时不伪造类型值
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

// 协议目标：phase=dial = 未进协议交互（不可达），phase=login = 握手半途失败，
// 无 phase 且 available = 已验证。
const ROW_PROTOCOL_FAILED = {
  agentId: 'agent-b',
  target: 'mc-login',
  semantics: 'readiness',
  gameId: 'demo',
  env: 'prod',
  available: false,
  windowOpen: true,
  checked: 10,
  failed: 3,
  successRate: 0.7,
  lastReason: 'protocol failed at step 2 (login): timeout waiting for match',
  probeType: 'protocol',
  protocol: 'minecraft',
  phase: 'login',
  lastReportAt: '2026-10-14T12:00:00Z',
};

const ROW_PROTOCOL_UNREACHABLE = {
  ...ROW_PROTOCOL_FAILED,
  target: 'mc-dial',
  failed: 10,
  successRate: 0,
  phase: 'dial',
  lastReason: 'dial tcp 10.0.0.6:25565: connect: connection refused',
};

const ROW_PROTOCOL_OK = {
  ...ROW_PROTOCOL_FAILED,
  target: 'mc-ok',
  available: true,
  windowOpen: false,
  failed: 0,
  successRate: 1,
  phase: undefined,
  lastReason: undefined,
};

// reachable-unverified（§5.5.3 状态 2）：目标声明了协议期望（protocolExpected）
// 但只落在 tcp 层——L1 探到 ≠ 协议可用，卡上灰态标注而不是伪装成已验证。
const ROW_REACHABLE_UNVERIFIED = {
  agentId: 'agent-b',
  target: 'gw-tcp',
  semantics: 'readiness',
  gameId: 'demo',
  env: 'prod',
  available: true,
  windowOpen: false,
  checked: 20,
  failed: 0,
  successRate: 1,
  probeType: 'tcp',
  protocolExpected: true,
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

  it('协议目标按失败阶段区分 不可达 / 协议失败 / 已验证（§5.5.3 标注面）', async () => {
    getOpsProbeSummary.mockResolvedValue({
      onlineDataIntegrated: false,
      rows: [ROW_PROTOCOL_UNREACHABLE, ROW_PROTOCOL_FAILED, ROW_PROTOCOL_OK],
    });
    renderSection();

    await screen.findByText('mc-login');
    // dial 失败 = 未进协议交互 → 不可达；登录半途失败 → 协议失败·阶段名
    expect(screen.getByText('不可达')).toBeInTheDocument();
    expect(screen.getByText('协议失败·login')).toBeInTheDocument();
    // 协议目标成功 = 已验证（状态 4），不退回含糊的「可用」
    expect(screen.getByText('已验证')).toBeInTheDocument();
    expect(screen.queryByText('可用')).not.toBeInTheDocument();
    // 标注不取代证据：失败原因原文仍在同行
    expect(screen.getByText(/protocol failed at step 2 \(login\)/)).toBeInTheDocument();
  });

  it('协议目标标出适配器名；旧版 agent 未上报类型不伪造', async () => {
    getOpsProbeSummary.mockResolvedValue({
      onlineDataIntegrated: false,
      rows: [ROW_PROTOCOL_OK, ROW_OK],
    });
    renderSection();

    await screen.findByText('mc-ok');
    expect(screen.getByText('mysql')).toBeInTheDocument();
    // 仅协议行携带类型列文本：未上报类型的行不猜类型（恰好一处）
    expect(screen.getAllByText('protocol:minecraft')).toHaveLength(1);
    // 协议行标「已验证」，未声明协议期望的 L1 行仍标「可用」（不误标灰态）
    expect(screen.getByText('已验证')).toBeInTheDocument();
    expect(screen.getByText('可用')).toBeInTheDocument();
    expect(screen.queryByText('未接入协议探测')).not.toBeInTheDocument();
  });

  it('声明协议期望的 L1 目标标 可达≠可用 + 未接入协议探测（reachable-unverified）', async () => {
    getOpsProbeSummary.mockResolvedValue({
      onlineDataIntegrated: false,
      rows: [ROW_REACHABLE_UNVERIFIED, ROW_OK],
    });
    renderSection();

    await screen.findByText('gw-tcp');
    // 只有一处可达未验证 + 一处副灰标签：声明位驱动，不猜类型
    expect(screen.getByText('可达≠可用')).toBeInTheDocument();
    expect(screen.getByText('未接入协议探测')).toBeInTheDocument();
    expect(screen.getByText('可用')).toBeInTheDocument();
    expect(screen.queryByText('已验证')).not.toBeInTheDocument();
    // 声明位不污染正常态证据：计数照实、无失败原因
    expect(screen.getAllByText('20 / 0')).toHaveLength(2);
  });
});
