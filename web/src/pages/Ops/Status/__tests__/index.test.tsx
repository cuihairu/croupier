/**
 * 运维状态页单测（覆盖率巡检：Ops/Status/index.tsx 437 行 0% → 收口，
 * 零测试页排行第七）。
 *
 * 锁定契约：
 * - 挂载链：Promise.all([getOpsHealth, getOpsServices, getOpsMQ]) 三连拉
 *   + getOpsMaintenance best-effort 回填维护表单（enabled Boolean 化 /
 *   message || '' / allowAdmins !== false 默认 true）；maintenance 拉取
 *   失败静默吞（best-effort）；
 * - 健康检查表：ID/名称/类型/目标列 + 启用 Switch（toggleCheck →
 *   updateOpsHealth({enabled:true, checks: 映射后的 next})，本地
 *   setChecks 乐观更新；失败「更新失败」不回滚）+ 执行按钮
 *   （disabled=!enabled）+ 运行结果 Tag（ok → 绿 `${latencyMs}ms` /
 *   !ok → 红「异常」）；
 * - 执行链：runOpsHealthCheck(id) → ok 成功 toast `${id} 正常（Nms）` /
 *   !ok 错误 toast `${id} 异常：${error || '失败'}`（error 缺省右臂）；
 *   reject →「执行失败」；
 * - 服务状态表：status Tag 三态（up/healthy 绿、truthy 红、falsy default
 *   '-'）、addr/version 列；
 * - 消息队列卡：lengths 空对象 → 「暂无队列数据」；有流 → 表格
 *   （积压 v>10000 红 Tag / 其余默认 Tag）；
 * - 维护模式保存：Popconfirm 确认 → validateFields →
 *   updateOpsMaintenance(表单值) →「维护模式已更新」；失败「更新失败」；
 * - load 失败：Promise.all reject →「加载状态失败」（extractErrorMessage
 *   透传 Error.message）；
 * - 刷新按钮重拉三连。
 *
 * mock 口径：services/api/opsStatus 七函数 jest.mock（在 service 边界
 * mock，模块内 || [] 归一不参与——直接回给已归一形态）；@umijs/max 本地
 * defaultMessage mock（formatMessage 支持 {id}/{latencyMs}/{error} 内插）；
 * antd/pro-components 真实实现。
 *
 * 全分支覆盖（100/100/100/100），无登记不可达。
 *
 * 坑实证（antd6 沿用）：Switch 交互用 fireEvent.click（role switch）；
 * Popconfirm 确认锚未隐藏 .ant-popover 内 .ant-btn-primary；带 icon 的
 * 双字中文 Button 的可访问名是「icon名 执 行」（如 play-circle 执行），
 * getByRole name 须用 /执\s*行/ 宽松正则；页内四张表——行锚文本跨全页
 * 查（querySelector('.ant-table') 只命中第一张）；mq 表在 lengths 空
 * 时整表不渲染（条件分支）。
 */
import React from 'react';
import { configure, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { App } from 'antd';
import OpsStatusPage from '../index';
import type { HealthCheck, OpsServiceItem } from '@/services/api/opsStatus';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

jest.mock('@/services/api/opsStatus', () => ({
  getOpsHealth: jest.fn(),
  runOpsHealthCheck: jest.fn(),
  updateOpsHealth: jest.fn(),
  getOpsMaintenance: jest.fn(),
  updateOpsMaintenance: jest.fn(),
  getOpsServices: jest.fn(),
  getOpsMQ: jest.fn(),
  getSystemRuntime: jest.fn(),
  checkSystemUpdate: jest.fn(),
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
  FormattedMessage: ({ defaultMessage }: { defaultMessage?: string }) => (
    <>{defaultMessage ?? ''}</>
  ),
  useIntl: () => mockIntl,
}));

import {
  getOpsHealth,
  runOpsHealthCheck,
  updateOpsHealth,
  getOpsMaintenance,
  updateOpsMaintenance,
  getOpsServices,
  getOpsMQ,
} from '@/services/api/opsStatus';

const mHealth = getOpsHealth as jest.MockedFunction<typeof getOpsHealth>;
const mRun = runOpsHealthCheck as jest.MockedFunction<typeof runOpsHealthCheck>;
const mUpdHealth = updateOpsHealth as jest.MockedFunction<typeof updateOpsHealth>;
const mMaint = getOpsMaintenance as jest.MockedFunction<typeof getOpsMaintenance>;
const mUpdMaint = updateOpsMaintenance as jest.MockedFunction<typeof updateOpsMaintenance>;
const mServices = getOpsServices as jest.MockedFunction<typeof getOpsServices>;
const mMQ = getOpsMQ as jest.MockedFunction<typeof getOpsMQ>;

// 覆盖翼：启用 + 满字段
const ck1: HealthCheck = {
  id: 'db',
  name: '数据库',
  enabled: true,
  type: 'tcp',
  kind: 'tcp',
  target: '127.0.0.1:5432',
};
// 覆盖翼：禁用（执行按钮 disabled）+ kind/target 缺省
const ck2: HealthCheck = { id: 'redis', name: '缓存', enabled: false, type: 'tcp' };

// 覆盖翼：up 绿 / healthy 绿 / truthy 红 / falsy default '-'
const svcs: OpsServiceItem[] = [
  { name: 'server', status: 'up', addr: '10.0.0.1:18780', version: '1.2.3' },
  { name: 'agent', status: 'healthy', addr: '10.0.0.2:19091' },
  { name: 'worker', status: 'down', addr: '10.0.0.3:1', version: '0.9' },
  { name: 'ghost', status: '' },
];

beforeEach(() => {
  jest.clearAllMocks();
  mHealth.mockResolvedValue({ checks: [ck1, ck2] });
  mServices.mockResolvedValue(svcs);
  mMQ.mockResolvedValue({ lengths: { orders: 5, events: 99999 } });
  mMaint.mockResolvedValue({ enabled: false, message: '', allowAdmins: true });
  mUpdHealth.mockResolvedValue(undefined);
  mUpdMaint.mockResolvedValue(undefined);
  mRun.mockResolvedValue({ id: 'db', ok: true, latencyMs: 12 });
});

function renderPage() {
  return render(
    <App>
      <OpsStatusPage />
    </App>,
  );
}

/** 等三连拉落定（健康表首行出现） */
async function waitLoad() {
  expect(await screen.findByText('数据库')).toBeInTheDocument();
  await waitFor(() => expect(mMQ).toHaveBeenCalled());
}

/** 表行（锚文本跨全部表查——页内四张表，名称全局唯一） */
function rowOf(text: string) {
  return screen.getByText(text).closest('tr') as HTMLElement;
}

/** Popconfirm 确认 */
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

describe('运维状态 初始渲染', () => {
  it('三连拉矩阵：健康表（类型/目标/启用 Switch）+ 服务四态 + MQ 双档 + 维护回填', async () => {
    renderPage();
    await waitLoad();

    // 健康表：ID/名称/类型/目标 + 启用态
    expect(within(rowOf('数据库')).getByText('tcp')).toBeInTheDocument();
    expect(within(rowOf('数据库')).getByText('127.0.0.1:5432')).toBeInTheDocument();
    expect(within(rowOf('数据库')).getByRole('switch')).toHaveClass('ant-switch-checked');

    // 服务状态四态
    expect(within(rowOf('server')).getByText('up').closest('.ant-tag')).toHaveClass(
      'ant-tag-green',
    );
    expect(screen.getByText('healthy').closest('.ant-tag')).toHaveClass('ant-tag-green');
    expect(screen.getByText('down').closest('.ant-tag')).toHaveClass('ant-tag-red');
    expect(within(rowOf('ghost')).getByText('-').closest('.ant-tag')).not.toHaveClass(
      'ant-tag-red',
    );

    // MQ：流名 + 积压双档（5 默认 / 99999 红）
    expect(screen.getByText('orders')).toBeInTheDocument();
    expect(screen.getByText('5').closest('.ant-tag')).not.toHaveClass('ant-tag-red');
    expect(screen.getByText('99999').closest('.ant-tag')).toHaveClass('ant-tag-red');

    // 维护表单回填（enabled false / message '' / allowAdmins true）
    const maintSwitch = screen.getAllByRole('switch')[2];
    expect(maintSwitch).not.toHaveClass('ant-switch-checked');
    expect(screen.getByPlaceholderText('系统维护中，预计 30 分钟')).toHaveValue('');
  });

  it('maintenance 拉取失败静默：表单保持 initialValues；MQ 空 → 暂无队列数据', async () => {
    mMaint.mockRejectedValueOnce(new Error('maint-x'));
    mMQ.mockResolvedValueOnce({}); // lengths 缺省 → || {} 右臂
    renderPage();
    await waitLoad();

    // best-effort：无报错文案、页面不白屏
    expect(screen.queryByText('maint-x')).not.toBeInTheDocument();
    // MQ 空对象 → 条件分支空态文案（表不渲染）
    expect(screen.getByText('暂无队列数据')).toBeInTheDocument();
  });

  it('load 失败：Promise.all reject → 加载状态失败', async () => {
    mHealth.mockRejectedValueOnce(new Error('health-down'));
    renderPage();
    expect(await screen.findByText('health-down')).toBeInTheDocument();
  });
});

describe('运维状态 健康检查操作', () => {
  it('执行：ok → 绿 latency Tag + 成功 toast；!ok（error 缺省）→ 红 异常 Tag', async () => {
    renderPage();
    await waitLoad();

    fireEvent.click(within(rowOf('数据库')).getByRole('button', { name: /执\s*行/ }));
    await waitFor(() => expect(mRun).toHaveBeenCalledWith('db'));
    expect(await screen.findByText('db 正常（12ms）')).toBeInTheDocument();
    expect(within(rowOf('数据库')).getByText('12ms').closest('.ant-tag')).toHaveClass(
      'ant-tag-green',
    );

    // !ok 且 error 缺省 → `${id} 异常：失败`（|| 右臂）+ 红 异常 Tag
    mRun.mockResolvedValueOnce({ id: 'db', ok: false, latencyMs: 0 });
    fireEvent.click(within(rowOf('数据库')).getByRole('button', { name: /执\s*行/ }));
    expect(await screen.findByText('db 异常：失败')).toBeInTheDocument();
    expect(within(rowOf('数据库')).getByText('异常').closest('.ant-tag')).toHaveClass(
      'ant-tag-red',
    );
  });

  it('执行 reject → 执行失败；禁用项执行按钮 disabled', async () => {
    renderPage();
    await waitLoad();

    // 禁用检查项：按钮 disabled，点击不触达
    const disabledBtn = within(rowOf('缓存')).getByRole('button', { name: /执\s*行/ });
    expect(disabledBtn).toBeDisabled();
    expect(within(rowOf('缓存')).queryByText(/ms/)).toBeNull();

    mRun.mockRejectedValueOnce(new Error('run-x'));
    fireEvent.click(within(rowOf('数据库')).getByRole('button', { name: /执\s*行/ }));
    expect(await screen.findByText('run-x')).toBeInTheDocument();
  });

  it('启用 Switch：乐观更新 + updateOpsHealth 载荷；失败「更新失败」', async () => {
    renderPage();
    await waitLoad();

    fireEvent.click(within(rowOf('缓存')).getByRole('switch'));
    await waitFor(() =>
      expect(mUpdHealth).toHaveBeenCalledWith({
        enabled: true,
        checks: [ck1, { id: 'redis', name: '缓存', enabled: true, type: 'tcp' }],
      }),
    );
    // 乐观更新：本地立即勾选
    await waitFor(() =>
      expect(within(rowOf('缓存')).getByRole('switch')).toHaveClass('ant-switch-checked'),
    );

    mUpdHealth.mockRejectedValueOnce(new Error('upd-x'));
    fireEvent.click(within(rowOf('数据库')).getByRole('switch'));
    expect(await screen.findByText('upd-x')).toBeInTheDocument();
  });
});

describe('运维状态 维护模式', () => {
  it('保存：Popconfirm → updateOpsMaintenance(表单值) → 已更新 toast', async () => {
    renderPage();
    await waitLoad();

    // 开启维护模式 + 填公告
    fireEvent.click(screen.getAllByRole('switch')[2]);
    fireEvent.change(screen.getByPlaceholderText('系统维护中，预计 30 分钟'), {
      target: { value: '停机维护 10 分钟' },
    });
    fireEvent.click(screen.getByRole('button', { name: /保\s*存/ }));
    await confirmPopover();
    await waitFor(() =>
      expect(mUpdMaint).toHaveBeenCalledWith({
        enabled: true,
        message: '停机维护 10 分钟',
        allowAdmins: true,
      }),
    );
    expect(await screen.findByText('维护模式已更新')).toBeInTheDocument();
  });

  it('保存失败 →「更新失败」', async () => {
    renderPage();
    await waitLoad();

    mUpdMaint.mockRejectedValueOnce(new Error('save-x'));
    fireEvent.click(screen.getByRole('button', { name: /保\s*存/ }));
    await confirmPopover();
    expect(await screen.findByText('save-x')).toBeInTheDocument();
  });
});

describe('运维状态 刷新', () => {
  it('刷新按钮按重拉三连（maintenance 不重拉——仅挂载 best-effort 一次）', async () => {
    renderPage();
    await waitLoad();
    expect(mMaint).toHaveBeenCalledTimes(1);

    fireEvent.click(screen.getByRole('button', { name: /刷\s*新/ }));
    await waitFor(() => expect(mHealth).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(mServices).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(mMQ).toHaveBeenCalledTimes(2));
    expect(mMaint).toHaveBeenCalledTimes(1);
  });
});
