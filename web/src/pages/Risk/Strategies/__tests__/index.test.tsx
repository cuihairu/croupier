/**
 * 风控策略页单测（CDC 平台化批阶段四首项）。
 *
 * 锁定契约：
 * - 挂载链：getCaptureGates + listCaptureGatesVersions + listOpsNodes
 *   并行拉取 → 版本表（新→旧，当前版本绿「当前」Tag + 说明/发布人/时间）；
 * - 未发布（version=0）→ 未发布 Alert + 空态文案，不渲染生效对账行；
 * - 生效对账：复用 captureGatesApply 汇总（全生效绿 / 部分金 tag+tooltip）；
 * - 回滚链：Popconfirm 确认 → rollbackCaptureGates({version}) → 成功 toast
 *   + 整页重载；回滚失败透传服务端消息；
 * - 行内约束：当前版本行回滚按钮 disabled（回滚当前版本=重发同内容，归
 *   面板发布入口）；同一次只允许一个进行中的回滚；
 * - 版本历史/节点对账失败 → 空表 + 不断主面（版本表不可得 ≠ 策略不可用）；
 * - 主面加载失败 → extractErrorMessage 透传。
 *
 * mock 口径：services/api/opsCapture 三函数 + services/api/ops 均
 * jest.mock（service 边界）；@umijs/max 本地 defaultMessage mock（含
 * {version}/{count} 内插）；antd 真实实现。
 *
 * antd6 沿用坑：Popconfirm 确认锚 = 未隐藏 .ant-popover 内 .ant-btn-primary。
 */
import React from 'react';
import { configure, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App } from 'antd';
import RiskStrategiesPage from '../index';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

jest.mock('@/services/api/opsCapture', () => ({
  getCaptureGates: jest.fn(),
  listCaptureGatesVersions: jest.fn(),
  rollbackCaptureGates: jest.fn(),
}));

jest.mock('@/services/api/ops', () => ({
  listOpsNodes: jest.fn(),
}));

const mockIntl = {
  formatMessage: (opts: { defaultMessage?: string }, values?: Record<string, string | number>) => {
    let msg = opts.defaultMessage ?? '';
    if (values) {
      for (const [k, v] of Object.entries(values)) {
        msg = msg.split(`{${k}}`).join(String(v));
      }
    }
    return msg;
  },
};
jest.mock('@umijs/max', () => ({
  FormattedMessage: ({
    defaultMessage,
    values,
  }: {
    defaultMessage?: string;
    values?: Record<string, string | number>;
  }) => {
    let msg = defaultMessage ?? '';
    if (values) {
      for (const [k, v] of Object.entries(values)) {
        msg = msg.split(`{${k}}`).join(String(v));
      }
    }
    return <>{msg}</>;
  },
  useIntl: () => mockIntl,
}));

import {
  getCaptureGates,
  listCaptureGatesVersions,
  rollbackCaptureGates,
} from '@/services/api/opsCapture';
import { listOpsNodes } from '@/services/api/ops';

const mGet = getCaptureGates as jest.MockedFunction<typeof getCaptureGates>;
const mVersions = listCaptureGatesVersions as jest.MockedFunction<typeof listCaptureGatesVersions>;
const mRollback = rollbackCaptureGates as jest.MockedFunction<typeof rollbackCaptureGates>;
const mNodes = listOpsNodes as jest.MockedFunction<typeof listOpsNodes>;

const latest = {
  key: 'capture.gates',
  version: 3,
  content: '{"reconcile":{"rules":[]}}',
  namespace: 'ops',
  updatedAt: '2026-10-11 10:00:00',
  createdBy: 'admin',
};

const history = [
  { version: 3, message: 'third', createdBy: 'admin', updatedAt: '2026-10-11 10:00:00' },
  { version: 2, message: 'second', createdBy: 'admin', updatedAt: '2026-10-11 09:30:00' },
  { version: 1, message: 'first', createdBy: 'admin', updatedAt: '2026-10-11 09:00:00' },
];

beforeEach(() => {
  jest.clearAllMocks();
  mGet.mockResolvedValue(latest);
  mVersions.mockResolvedValue(history);
  mRollback.mockResolvedValue({ version: 4 });
  mNodes.mockResolvedValue({
    nodes: [
      { id: 'cap-1', labels: { role: 'capture', captureGatesRevision: '3' } },
      { id: 'cap-2', labels: { role: 'capture', captureGatesRevision: '3' } },
    ],
  } as never);
});

function renderPage() {
  return render(
    <App>
      <RiskStrategiesPage />
    </App>,
  );
}

async function waitTable() {
  await screen.findByText('v3');
}

async function confirmPopover() {
  const popover = await waitFor(() => {
    const visible = Array.from(document.querySelectorAll('.ant-popover')).find(
      (p) => !p.className.includes('ant-popover-hidden'),
    ) as HTMLElement;
    expect(visible).not.toBeUndefined();
    return visible;
  });
  fireEvent.click(popover.querySelector('.ant-btn-primary') as HTMLElement);
}

describe('风控策略页 初始渲染', () => {
  it('版本历史倒序 + 当前版本绿 Tag + 元信息列', async () => {
    renderPage();
    await waitTable();

    expect(screen.getByText('v3').closest('.ant-space')).toContainElement(screen.getByText('当前'));
    expect(screen.getByText('当前').closest('.ant-tag')).toHaveClass('ant-tag-green');
    expect(screen.getByText('v2')).toBeInTheDocument();
    expect(screen.getByText('v1')).toBeInTheDocument();
    expect(screen.getByText('third')).toBeInTheDocument();
    expect(screen.getByText('second')).toBeInTheDocument();
    expect(screen.getByText('first')).toBeInTheDocument();
    expect(screen.getByText('2026-10-11 09:00:00')).toBeInTheDocument();
    expect(screen.queryByText('未发布（agent 用文件规则）')).not.toBeInTheDocument();
  });

  it('未发布（desired=0）：未发布提示 + 空表 + 不渲染生效对账行', async () => {
    mGet.mockResolvedValue({ key: 'capture.gates', version: 0, content: '' });
    mVersions.mockResolvedValue([]);
    renderPage();
    expect(await screen.findByText('未发布（agent 用文件规则）')).toBeInTheDocument();
    expect(screen.getByText('该 scope 尚未发布过策略版本')).toBeInTheDocument();
    expect(screen.queryByText(/台已生效/)).not.toBeInTheDocument();
    expect(screen.queryByText('scope 内无在线 capture-agent')).not.toBeInTheDocument();
  });

  it('版本历史接口失败：空表退化，不打断主面', async () => {
    mVersions.mockRejectedValueOnce(new Error('versions-down'));
    renderPage();
    expect(await screen.findByText('该 scope 尚未发布过策略版本')).toBeInTheDocument();
    expect(screen.getByText('风控策略')).toBeInTheDocument();
  });

  it('主面加载失败：透传错误消息（extractErrorMessage）', async () => {
    mGet.mockRejectedValueOnce(new Error('gates-down'));
    renderPage();
    expect(await screen.findByText('gates-down')).toBeInTheDocument();
  });
});

describe('风控策略页 生效对账（appliedRevision）', () => {
  it('全部生效：绿「{count}/{total} 台已生效」', async () => {
    renderPage();
    expect(await screen.findByText('2/2 台已生效')).toBeInTheDocument();
    expect(screen.getByText('2/2 台已生效').closest('.ant-tag')).toHaveClass('ant-tag-green');
  });

  it('部分生效：金 tag + tooltip 点名未生效 agent 与其自报版本', async () => {
    mNodes.mockResolvedValue({
      nodes: [
        { id: 'cap-1', labels: { role: 'capture', captureGatesRevision: '3' } },
        { id: 'cap-2', labels: { role: 'capture', captureGatesRevision: '1' } },
        { id: 'cap-3', labels: { role: 'capture' } },
      ],
    } as never);
    renderPage();
    expect(await screen.findByText('1/3 台已生效（其余未上报或滞后）')).toBeInTheDocument();
    expect(screen.getByText('1/3 台已生效（其余未上报或滞后）').closest('.ant-tag')).toHaveClass(
      'ant-tag-gold',
    );
  });
});

describe('风控策略页 回滚', () => {
  it('当前版本行回滚按钮 disabled（重发同内容归面板发布入口）', async () => {
    renderPage();
    await waitTable();

    const buttons = screen.getAllByRole('button', { name: /回\s*滚/ });
    expect(buttons.length).toBe(3);
    // 表格渲染顺序 = 数据顺序（v3 当前行第一个）
    expect(buttons[0]).toBeDisabled();
    expect(buttons[1]).toBeEnabled();
    expect(buttons[2]).toBeEnabled();
  });

  it('回滚链：Popconfirm → rollbackCaptureGates({version}) → 成功 toast + 重载', async () => {
    renderPage();
    await waitTable();

    // 点 v1（历史首行）所在行的回滚按钮
    const row = screen.getByText('v1').closest('.ant-table-row') as HTMLElement;
    const btn = row.querySelector('button') as HTMLElement;
    fireEvent.click(btn);
    await confirmPopover();

    await waitFor(() => expect(mRollback).toHaveBeenCalledWith({ version: 1 }));
    expect(
      await screen.findByText('已按版本 1 内容发布新版本，agent 将在轮询周期内热生效'),
    ).toBeInTheDocument();
    await waitFor(() => expect(mGet).toHaveBeenCalledTimes(2));
  });

  it('回滚失败 → 回滚失败 toast（透传服务端消息）', async () => {
    mRollback.mockRejectedValueOnce(new Error('not-found: version 7 missing'));
    renderPage();
    await waitTable();

    const row = screen.getByText('v2').closest('.ant-table-row') as HTMLElement;
    const btn = row.querySelector('button') as HTMLElement;
    fireEvent.click(btn);
    await confirmPopover();
    expect(await screen.findByText('not-found: version 7 missing')).toBeInTheDocument();
  });
});
