/**
 * capture 闸规则下发页单测。
 *
 * 锁定契约：
 * - 挂载链：getCaptureGates → 版本 Tag（未发布 →「未发布（agent 用文件规
 *   则）」默认 Tag；已发布 → 绿「版本 N」+ updatedAt·createdBy）；content
 *   回填 textarea；
 * - 模板插入：点击插入模板 → textarea 填 GATES_TEMPLATE 骨架（含
 *   item-ledger 与 allowAccounts 锚文本）；
 * - 发布链：Popconfirm 确认 → publishCaptureGates({content}) → 成功 toast
 *   「已发布版本 N…」+ 重拉（getCaptureGates 第二次调用）；
 * - 发布失败 →「发布失败」toast（extractErrorMessage 透传）；
 * - 加载失败 → extractErrorMessage 透传「加载闸规则失败」右臂（透传
 *   Error.message）；
 * - 发布按钮在 content 为空时 disabled；重载按钮重拉。
 *
 * mock 口径：services/api/opsCapture 双函数 jest.mock（service 边界）；
 * @umijs/max 本地 defaultMessage mock（formatMessage 支持 {version} 内插）；
 * antd/pro-components 真实实现。
 *
 * antd6 沿用坑：Popconfirm 确认锚 = 未隐藏 .ant-popover 内
 * .ant-btn-primary；TextArea 用 placeholder 查询。
 */
import React from 'react';
import { configure, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App } from 'antd';
import OpsCapturePage from '../index';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

jest.mock('@/services/api/opsCapture', () => ({
  getCaptureGates: jest.fn(),
  publishCaptureGates: jest.fn(),
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

import { getCaptureGates, publishCaptureGates } from '@/services/api/opsCapture';
import { listOpsNodes } from '@/services/api/ops';

const mGet = getCaptureGates as jest.MockedFunction<typeof getCaptureGates>;
const mPublish = publishCaptureGates as jest.MockedFunction<typeof publishCaptureGates>;
const mNodes = listOpsNodes as jest.MockedFunction<typeof listOpsNodes>;

const published = {
  key: 'capture.gates',
  version: 3,
  content: '{"reconcile":{"rules":[]}}',
  namespace: 'ops',
  updatedAt: '2026-10-11 10:00:00',
  createdBy: 'admin',
};

beforeEach(() => {
  jest.clearAllMocks();
  mGet.mockResolvedValue({ ...published });
  mPublish.mockResolvedValue({ version: 4 });
  // 默认两台 capture agent 均已上报生效版本 3（与 published.version 对齐）
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
      <OpsCapturePage />
    </App>,
  );
}

async function waitLoad() {
  expect(await screen.findByText('版本 3')).toBeInTheDocument();
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

describe('capture 闸规则页 初始渲染', () => {
  it('已发布：绿版本 Tag + 元信息 + content 回填', async () => {
    renderPage();
    await waitLoad();

    expect(screen.getByText('版本 3').closest('.ant-tag')).toHaveClass('ant-tag-green');
    expect(screen.getByText(/2026-10-11 10:00:00 · admin/)).toBeInTheDocument();
    expect(
      screen.getByPlaceholderText('capture.gates JSON（reconcile / cep / lineage 三段）'),
    ).toHaveValue('{"reconcile":{"rules":[]}}');
    expect(screen.queryByText('未发布（agent 用文件规则）')).not.toBeInTheDocument();
  });

  it('未发布：默认 Tag + 空 textarea；发布按钮 disabled', async () => {
    mGet.mockResolvedValue({ key: 'capture.gates', version: 0, content: '' });
    renderPage();
    expect(await screen.findByText('未发布（agent 用文件规则）')).toBeInTheDocument();

    const publishBtn = screen.getByRole('button', { name: /发\s*布/ });
    expect(publishBtn).toBeDisabled();
    expect(
      screen.getByPlaceholderText('capture.gates JSON（reconcile / cep / lineage 三段）'),
    ).toHaveValue('');
  });

  it('加载失败：透传错误消息（extractErrorMessage）', async () => {
    mGet.mockRejectedValueOnce(new Error('gates-down'));
    renderPage();
    expect(await screen.findByText('gates-down')).toBeInTheDocument();
  });
});

describe('capture 闸规则页 编辑与发布', () => {
  it('插入模板：textarea 填骨架（reconcile/lineage 锚文本）', async () => {
    renderPage();
    await waitLoad();

    fireEvent.click(screen.getByRole('button', { name: /插\s*入\s*模\s*板/ }));
    const area = screen.getByPlaceholderText(
      'capture.gates JSON（reconcile / cep / lineage 三段）',
    ) as HTMLTextAreaElement;
    expect(area.value).toContain('"item-ledger"');
    expect(area.value).toContain('"allowAccounts"');
    // 模板非空 → 发布按钮解除禁用
    expect(screen.getByRole('button', { name: /发\s*布/ })).toBeEnabled();
  });

  it('发布链：Popconfirm → publishCaptureGates(content) → 成功 toast + 重拉', async () => {
    renderPage();
    await waitLoad();

    fireEvent.click(screen.getByRole('button', { name: /发\s*布/ }));
    await confirmPopover();

    await waitFor(() =>
      expect(mPublish).toHaveBeenCalledWith({
        content: '{"reconcile":{"rules":[]}}',
        message: undefined,
      }),
    );
    expect(
      await screen.findByText('已发布版本 4，capture-agent 将在轮询周期内热生效'),
    ).toBeInTheDocument();
    await waitFor(() => expect(mGet).toHaveBeenCalledTimes(2));
  });

  it('发布失败 → 发布失败 toast（透传服务端消息）', async () => {
    mPublish.mockRejectedValueOnce(new Error('lint-failed: rules not array'));
    renderPage();
    await waitLoad();

    fireEvent.click(screen.getByRole('button', { name: /发\s*布/ }));
    await confirmPopover();
    expect(await screen.findByText('lint-failed: rules not array')).toBeInTheDocument();
  });

  it('重载按钮重拉', async () => {
    renderPage();
    await waitLoad();
    fireEvent.click(screen.getByRole('button', { name: /重\s*载/ }));
    await waitFor(() => expect(mGet).toHaveBeenCalledTimes(2));
  });
});

describe('capture 闸规则页 生效对账（appliedRevision）', () => {
  it('全部生效：绿「{count}/{total} 台已生效」', async () => {
    renderPage();
    expect(await screen.findByText('2/2 台已生效')).toBeInTheDocument();
    const tag = screen.getByText('2/2 台已生效').closest('.ant-tag');
    expect(tag).toHaveClass('ant-tag-green');
  });

  it('部分生效：金 tag + tooltip 列出未生效 agent 与其自报版本', async () => {
    mNodes.mockResolvedValue({
      nodes: [
        { id: 'cap-1', labels: { role: 'capture', captureGatesRevision: '3' } },
        { id: 'cap-2', labels: { role: 'capture', captureGatesRevision: '1' } },
        { id: 'cap-3', labels: { role: 'capture' } },
      ],
    } as never);
    renderPage();
    expect(await screen.findByText('1/3 台已生效（其余未上报或滞后）')).toBeInTheDocument();
    // 非 capture 节点不进对账（下面这台 role 缺失，总数仍为 3）
  });

  it('节点对账失败：退化为「scope 内无在线 capture-agent」不打断编辑面', async () => {
    mNodes.mockRejectedValueOnce(new Error('nodes-down'));
    renderPage();
    expect(await screen.findByText('scope 内无在线 capture-agent')).toBeInTheDocument();
    expect(await screen.findByText('版本 3')).toBeInTheDocument();
  });

  it('未发布（desired=0）：不渲染对账行', async () => {
    mGet.mockResolvedValue({ key: 'capture.gates', version: 0, content: '' });
    renderPage();
    expect(await screen.findByText('未发布（agent 用文件规则）')).toBeInTheDocument();
    expect(screen.queryByText(/台已生效/)).not.toBeInTheDocument();
    expect(screen.queryByText('scope 内无在线 capture-agent')).not.toBeInTheDocument();
  });
});
