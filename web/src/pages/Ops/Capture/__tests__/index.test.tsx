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
 * - 发布按钮在 content 为空时 disabled；重载按钮重拉；
 * - 结构化表单（todo 4297）：Segmented 切 form/json，表单是 JSON 生产者
 *   （回填/编辑/增删行都同步回 content，发布与校验链只认 content）；坏
 *   JSON 拦在模式切换外留 JSON 面；表单模式下插入模板同步表单态。
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
  validateCaptureGates: jest.fn(),
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
  publishCaptureGates,
  validateCaptureGates,
} from '@/services/api/opsCapture';
import { listOpsNodes } from '@/services/api/ops';

const mGet = getCaptureGates as jest.MockedFunction<typeof getCaptureGates>;
const mPublish = publishCaptureGates as jest.MockedFunction<typeof publishCaptureGates>;
const mValidate = validateCaptureGates as jest.MockedFunction<typeof validateCaptureGates>;
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
  mValidate.mockResolvedValue({
    valid: true,
    reconcile: 1,
    cepOffHours: 0,
    cepBurst: 0,
    cepProbing: 0,
    lineage: 1,
    eventOps: 2,
  });
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
        baseVersion: 3,
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

  it('发布乐观锁冲突 → 透传服务端冲突消息', async () => {
    mPublish.mockRejectedValueOnce(new Error('config has been updated by another user'));
    renderPage();
    await waitLoad();

    fireEvent.click(screen.getByRole('button', { name: /发\s*布/ }));
    await confirmPopover();
    expect(await screen.findByText('config has been updated by another user')).toBeInTheDocument();
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

describe('capture 闸规则页 dry-run 校验（config.ValidateGates 同真值）', () => {
  it('校验通过：绿提示带各闸规则数预览，请求携带编辑器内容', async () => {
    renderPage();
    fireEvent.click(await screen.findByRole('button', { name: '校验' }));

    await waitFor(() => expect(mValidate).toHaveBeenCalledWith(published.content));
    expect(
      await screen.findByText(
        /校验通过（与 capture-agent 生效同一实现）：对账 1 · 时段 0 · 突变 0 · 试探 0 · 血缘 1 · 事件过滤 2/,
      ),
    ).toBeInTheDocument();
  });

  it('校验失败：红提示就地透传服务端错误（与 agent apply 同句），不弹 toast', async () => {
    mValidate.mockRejectedValueOnce(
      new Error('reconcile rule x: itemTable must be "db.table", got "bad"'),
    );
    renderPage();
    fireEvent.click(await screen.findByRole('button', { name: '校验' }));

    expect(
      await screen.findByText(/reconcile rule x: itemTable must be "db.table", got "bad"/),
    ).toBeInTheDocument();
  });

  it('编辑内容即作废上次校验结论（结论只对当时的 JSON 负责）', async () => {
    renderPage();
    fireEvent.click(await screen.findByRole('button', { name: '校验' }));
    expect(await screen.findByText(/校验通过/)).toBeInTheDocument();

    // 注意：fireEvent.change 的值必须与 React state 现值不同——同串会被
    // React value tracker 去重，onChange 根本不触发（假阴性陷阱）。
    fireEvent.change(screen.getByPlaceholderText(/capture.gates JSON/), {
      target: { value: '{"reconcile":{"rules":[{"name":"tweak"}]}}' },
    });
    await waitFor(() => expect(screen.queryByText(/校验通过/)).not.toBeInTheDocument());
  });
});

describe('capture 闸规则页 结构化表单模式（todo 4297）', () => {
  const fullGates = JSON.stringify({
    reconcile: {
      rules: [
        {
          name: 'r1',
          itemTable: 'game.items',
          ledgerTable: 'game.ledger',
          itemKey: 'id',
          ledgerKey: 'id',
          windowSec: 300,
        },
      ],
    },
    cep: { offHours: [], burst: [], probing: [] },
    lineage: {
      rules: [{ name: 'l1', table: 'game.items', allowAccounts: ['game-app'] }],
    },
    event: { ops: ['insert'] },
  });

  async function enterFormMode() {
    fireEvent.click(await screen.findByRole('radio', { name: '表单' }));
    await screen.findByLabelText('reconcile.add');
  }

  it('表单回填：content 解析进各段控件，textarea 收起', async () => {
    mGet.mockResolvedValue({ ...published, content: fullGates });
    renderPage();
    await waitLoad();
    await enterFormMode();

    expect(screen.getByLabelText('reconcile[0].name') as HTMLInputElement).toHaveValue('r1');
    expect(screen.getByLabelText('reconcile[0].itemTable') as HTMLInputElement).toHaveValue(
      'game.items',
    );
    expect(screen.getByLabelText('lineage[0].table') as HTMLInputElement).toHaveValue('game.items');
    expect(screen.queryByPlaceholderText(/capture.gates JSON/)).not.toBeInTheDocument();
  });

  it('表单编辑同步 content：改 name 后校验请求携带序列化结果', async () => {
    mGet.mockResolvedValue({ ...published, content: fullGates });
    renderPage();
    await waitLoad();
    await enterFormMode();

    fireEvent.change(screen.getByLabelText('reconcile[0].name'), {
      target: { value: 'renamed' },
    });
    fireEvent.click(screen.getByRole('button', { name: '校验' }));
    await waitFor(() =>
      expect(mValidate).toHaveBeenCalledWith(expect.stringContaining('"name": "renamed"')),
    );
  });

  it('增行+发布：cepBurst.add 新规则编辑后发布 payload 为序列化 content', async () => {
    renderPage();
    await waitLoad();
    await enterFormMode();

    fireEvent.click(screen.getByRole('button', { name: 'cepBurst.add' }));
    fireEvent.change(await screen.findByLabelText('cepBurst[0].name'), {
      target: { value: 'burst-a' },
    });

    fireEvent.click(screen.getByRole('button', { name: /发\s*布/ }));
    await confirmPopover();
    await waitFor(() =>
      expect(mPublish).toHaveBeenCalledWith({
        content: expect.stringContaining('"name": "burst-a"'),
        baseVersion: 3,
      }),
    );
  });

  it('删行：reconcile[0].remove 后 content 归空数组（只采不限）', async () => {
    mGet.mockResolvedValue({ ...published, content: fullGates });
    renderPage();
    await waitLoad();
    await enterFormMode();

    fireEvent.click(screen.getByRole('button', { name: 'reconcile[0].remove' }));
    expect(screen.queryByLabelText('reconcile[0].name')).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: '校验' }));
    await waitFor(() =>
      expect(mValidate).toHaveBeenCalledWith(expect.stringContaining('"rules": []')),
    );
  });

  it('坏 JSON 拦切换：留在 JSON 面并提示 parseFailed', async () => {
    renderPage();
    await waitLoad();
    fireEvent.change(screen.getByPlaceholderText(/capture.gates JSON/), {
      target: { value: '{broken' },
    });
    fireEvent.click(screen.getByRole('radio', { name: '表单' }));

    expect(
      await screen.findByText('JSON 解析失败，请先修正内容再切换表单模式'),
    ).toBeInTheDocument();
    expect(screen.getByPlaceholderText(/capture.gates JSON/)).toBeInTheDocument();
  });

  it('表单模式下插入模板同步表单态', async () => {
    renderPage();
    await waitLoad();
    await enterFormMode();

    fireEvent.click(screen.getByRole('button', { name: /插\s*入\s*模\s*板/ }));
    const itemTable = (await screen.findByLabelText('reconcile[0].itemTable')) as HTMLInputElement;
    expect(itemTable.value).toBe('game.items');
    expect(screen.getByLabelText('lineage[0].allowAccounts')).toBeInTheDocument();
  });
});
