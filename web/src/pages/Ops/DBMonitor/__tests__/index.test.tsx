/**
 * 数据库监控页单测（覆盖率巡检：Ops/DBMonitor/index.tsx 667 行 0% → 收口，
 * 零测试页排行现席）。
 *
 * 锁定契约：
 * - 初始加载与卡片矩阵：驱动 Tag blue、kind 标签查表 + 未知 kind 回退原文、
 *   停用 Tag、gameId/env geekblue 与全局 Tag 两臂、dsnMask code 文本、
 *   无探测结果提示文案、canManage 门控（登记/编辑/删除三处按钮）；
 * - 空态（items 缺省 → `res.items || []` 右翼 → Empty）与 load 失败两翼
 *   （Error.message 透传 / 非 object 走「加载数据源失败」兜底）；
 * - 立即探测主链：probeAll → results 按 sourceId 归并 → 每卡指标矩阵
 *   （连接 current/max 与 max<=0 '?' 臂、connections 缺省 '-' 臂、锁等待
 *   红 Tag `{count} 条`/绿 0、死锁 volcano>0/绿 0/null 与 undefined 双翼
 *   「不可用」Tooltip、延迟 `?? '-'`ms、锁等待表 waitSecs>30 红 Tag 与
 *   普通文本两臂）；`res.results || []` 右翼（{} → 全卡停留提示）；
 *   探测失败两翼（Error.message / 非 object「探测失败」兜底）；
 * - 新建主链：ModalForm 标题、name/dsn required 拦截（dsn 仅新建必填）、
 *   driver/kind/enabled 默认值（mysql/self/true）、阈值双 InputNumber、
 *   createDBSource 载荷（dsn 必填下 `|| ''` 收窄）、「数据源已登记」+
 *   重拉 + 弹窗关闭；
 * - 编辑主链：initialValues 回填（DSN 掩码不回填、留空即不修改的 extra 文案、
 *   lockWaitWarn/connWarnRatio 为 0 时归一 undefined）→ updateDBSource(id, v)
 *   「数据源已更新」+ 重拉；保存失败两翼（Error.message / 兜底「保存失败」）
 *   → 弹窗保持打开不重拉；
 * - 删除 Popconfirm：标题 `删除「{name}」？`、确认 → deleteDBSource(id) +
 *   「已删除」+ 重拉；失败 Error.message 透传；
 * - 刷新按钮重拉。
 *
 * mock 口径：services/api/dbmon 五函数 + dbKindLabels jest.mock（真实模块
 * 在模块顶层 getIntl，mock 面更小更稳）；@umijs/max 本地 mock（defaultMessage
 * 即文案 + {count}/{error}/{name} values 内插 + useAccess 可控）；
 * antd/pro-components/extractErrorMessage 走真实实现。
 *
 * 登记不可达（防御分支，不造假用例不删分支）：
 * - 新建提交 `dsn: v.dsn || ''` 右翼：新建态 dsn 带 required 规则，空值被
 *   ModalForm 校验拦截、提交时 dsn 恒非空串（`|| ''` 仅满足类型收窄）。
 *
 * 坑实证（antd6 沿用）：ModalForm 标题与入口按钮同文本（锚 .ant-modal-title
 * 的 textContent）；弹窗 footer 主按钮走类名（submitText 保存渲染为「保 存」）；
 * Popconfirm 确认按钮锚未隐藏 .ant-popover 内 role=button name=/确/——本页
 * 默认按钮文案是 zh（确 定），与 AlertRulesTab 的 en（OK）不同源，name 取
 * 正则两态通吃；带图标的工具按钮 accessible name 前缀拼 icon aria-label
 * （「reload 刷新」），role 查询须用正则；弹窗内两个 Select 按 DOM 序
 * [driver, kind]，mouseDown 落 .ant-select 根 + 点可见 option content；
 * antd Form 序列化丢弃 undefined 值键——undefined 归一断言须直接读
 * mock.calls 载荷键而非 objectContaining。
 */
import React from 'react';
import { configure, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { App } from 'antd';
import DBMonitorPage from '../index';
import type { DBSource, ProbeResult } from '@/services/api/dbmon';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

jest.mock('@/services/api/dbmon', () => ({
  listDBSources: jest.fn(),
  createDBSource: jest.fn(),
  updateDBSource: jest.fn(),
  deleteDBSource: jest.fn(),
  probeAll: jest.fn(),
  // 真实模块顶层 getIntl；mock 只需覆盖用例涉及的 key
  dbKindLabels: { self: '自建', aliyun: '阿里云', tencent: '腾讯云', huawei: '华为云' },
}));

// mock* 前缀变量：babel-jest hoist 白名单；useAccess 每用例可控 canManage
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
let mockCanOpsManage = true;

jest.mock('@umijs/max', () => ({
  FormattedMessage: ({ defaultMessage }: { defaultMessage?: string }) => (
    <>{defaultMessage ?? ''}</>
  ),
  useIntl: () => mockIntl,
  useAccess: () => ({ canOpsManage: mockCanOpsManage }),
}));

import {
  listDBSources,
  createDBSource,
  updateDBSource,
  deleteDBSource,
  probeAll,
} from '@/services/api/dbmon';

const mList = listDBSources as jest.MockedFunction<typeof listDBSources>;
const mCreate = createDBSource as jest.MockedFunction<typeof createDBSource>;
const mUpdate = updateDBSource as jest.MockedFunction<typeof updateDBSource>;
const mDelete = deleteDBSource as jest.MockedFunction<typeof deleteDBSource>;
const mProbe = probeAll as jest.MockedFunction<typeof probeAll>;

// 覆盖翼：enabled、gameId/env、阈值双字段、掩码
const src1: DBSource = {
  id: 1,
  name: '主库',
  driver: 'mysql',
  kind: 'self',
  dsnMask: 'mysql://***',
  gameId: 'demo',
  env: 'prod',
  enabled: true,
  sort: 0,
  lockWaitWarn: 5,
  connWarnRatio: 80,
  createdAt: '2026-09-01T00:00:00Z',
  updatedAt: '2026-09-01T00:00:00Z',
};
// 覆盖翼：未知 kind 回退原文、停用 Tag、gameId 缺省 → 全局
const src2: DBSource = {
  id: 2,
  name: '从库',
  driver: 'postgres',
  kind: 'weird' as never,
  dsnMask: 'pg://***',
  enabled: false,
  sort: 1,
  createdAt: '2026-09-01T00:00:00Z',
  updatedAt: '2026-09-01T00:00:00Z',
};
// 覆盖翼：阈值 0 → 编辑回填归一 undefined
const src3: DBSource = {
  id: 3,
  name: '只读副本',
  driver: 'mysql',
  kind: 'aliyun',
  dsnMask: 'mysql://ro***',
  enabled: true,
  sort: 2,
  lockWaitWarn: 0,
  connWarnRatio: 0,
  createdAt: '2026-09-01T00:00:00Z',
  updatedAt: '2026-09-01T00:00:00Z',
};
// 覆盖翼：探测 ok:false
const src4: DBSource = {
  id: 4,
  name: '报警库',
  driver: 'postgres',
  kind: 'self',
  enabled: true,
  sort: 3,
  createdAt: '2026-09-01T00:00:00Z',
  updatedAt: '2026-09-01T00:00:00Z',
};
// 覆盖翼：无探测结果 → 停留提示
const src5: DBSource = {
  id: 5,
  name: '新库',
  driver: 'mysql',
  kind: 'self',
  enabled: true,
  sort: 4,
  createdAt: '2026-09-01T00:00:00Z',
  updatedAt: '2026-09-01T00:00:00Z',
};

const sources = [src1, src2, src3, src4, src5];

// 探测矩阵：满指标 + 双锁等待行（45s 红 / 5s 普通）+ 死锁>0
const pr1: ProbeResult = {
  sourceId: 1,
  name: '主库',
  driver: 'mysql',
  kind: 'self',
  ok: true,
  latencyMs: 12,
  connections: { current: 8, max: 100, active: 2 },
  lockWaits: [
    { waitId: 'w1', blockedBy: 'b1', waitSecs: 45, query: 'SELECT * FROM orders' },
    { waitId: 'w2', blockedBy: 'b2', waitSecs: 5, query: 'UPDATE players SET gold = 1' },
  ],
  deadlockCount: 3,
  probedAt: '2026-09-01T00:00:00Z',
};
// 覆盖翼：max 0 → '?'、lockWaits 空 → 绿 0、deadlockCount null → 不可用、
// latencyMs 缺省 → '-'
const pr2: ProbeResult = {
  sourceId: 2,
  name: '从库',
  driver: 'postgres',
  kind: 'weird',
  ok: true,
  connections: { current: 3, max: 0, active: 0 },
  lockWaits: [],
  deadlockCount: null,
  probedAt: '2026-09-01T00:00:00Z',
};
// 覆盖翼：connections 缺省 → current '-' + max '?'、deadlockCount 缺省
const pr3: ProbeResult = {
  sourceId: 3,
  name: '只读副本',
  driver: 'mysql',
  kind: 'aliyun',
  ok: true,
  latencyMs: 5,
  deadlockCount: 0,
  probedAt: '2026-09-01T00:00:00Z',
};
// 覆盖翼：ok:false → danger 文案
const pr4: ProbeResult = {
  sourceId: 4,
  name: '报警库',
  driver: 'postgres',
  kind: 'self',
  ok: false,
  error: 'dial tcp timeout',
  probedAt: '2026-09-01T00:00:00Z',
};

beforeEach(() => {
  mockCanOpsManage = true;
  jest.clearAllMocks();
  mList.mockResolvedValue({ items: sources });
  mCreate.mockResolvedValue({} as never);
  mUpdate.mockResolvedValue({} as never);
  mDelete.mockResolvedValue(undefined);
  mProbe.mockResolvedValue({ results: [pr1, pr2, pr3, pr4] });
});

function renderPage() {
  return render(
    <App>
      <DBMonitorPage />
    </App>,
  );
}

/** 等首拉落定 */
async function waitLoad() {
  expect(await screen.findByText('主库')).toBeInTheDocument();
  await waitFor(() => expect(mList).toHaveBeenCalledTimes(1));
}

/** 按卡标题（含数据源名）锚定小卡 */
function cardOf(name: string) {
  const card = Array.from(document.querySelectorAll('.ant-card')).find((c) =>
    c.querySelector('.ant-card-head-title')?.textContent?.includes(name),
  ) as HTMLElement;
  expect(card).not.toBeUndefined();
  return card;
}

/** 指标行拼接文本：label span 的父 div（连接/锁等待/死锁累计/延迟） */
function metricText(card: HTMLElement, label: string) {
  const span = within(card).getAllByText(label)[0];
  return span.parentElement?.textContent ?? '';
}

/** 弹窗 footer 确认（submitText=保存 → 「保 存」） */
function submitModal() {
  const ok = document.querySelector('.ant-modal-footer .ant-btn-primary') as HTMLElement;
  expect(ok).not.toBeNull();
  fireEvent.click(ok);
}

/** 弹窗内第 idx 个 Select 选 option（antd6：mouseDown 根 + 点可见 option content） */
async function pickModalSelect(idx: number, label: string) {
  await new Promise((r) => setTimeout(r, 60));
  const selects = document.querySelectorAll('.ant-modal .ant-select');
  fireEvent.mouseDown(selects[idx] as HTMLElement);
  const dropdown = await waitFor(() => {
    const visible = Array.from(document.querySelectorAll('.ant-select-dropdown')).find(
      (d) => !d.className.includes('ant-select-dropdown-hidden'),
    ) as HTMLElement;
    expect(visible).not.toBeUndefined();
    return visible;
  });
  fireEvent.click(
    within(dropdown).getByText(label, { selector: '.ant-select-item-option-content' }),
  );
}

describe('数据库监控 初始渲染', () => {
  it('卡片矩阵：驱动/类型/停用/游戏/全局/掩码/提示/管理按钮', async () => {
    renderPage();
    await waitLoad();

    // 驱动 Tag blue + 类型标签查表 / 未知 kind 回退原文
    expect(within(cardOf('主库')).getByText('mysql').closest('.ant-tag')).toHaveClass(
      'ant-tag-blue',
    );
    expect(within(cardOf('主库')).getByText('自建')).toBeInTheDocument();
    expect(within(cardOf('从库')).getByText('weird')).toBeInTheDocument();

    // 停用 Tag + 全局 Tag（gameId 缺省）
    expect(within(cardOf('从库')).getByText('已停用')).toBeInTheDocument();
    expect(within(cardOf('主库')).getByText('demo/prod').closest('.ant-tag')).toHaveClass(
      'ant-tag-geekblue',
    );
    expect(within(cardOf('从库')).getByText('全局')).toBeInTheDocument();

    // 掩码 code 文本 + 无探测结果提示
    expect(within(cardOf('主库')).getByText('mysql://***')).toBeInTheDocument();
    expect(screen.getAllByText('点击「立即探测」获取实时状态')).toHaveLength(5);

    // canManage：登记 + 每卡编辑/删除
    expect(screen.getByRole('button', { name: '登记数据库' })).toBeInTheDocument();
    expect(screen.getAllByText('编辑')).toHaveLength(5);
    expect(screen.getAllByText('删除')).toHaveLength(5);

    // 刷新/探测工具按钮
    expect(screen.getByRole('button', { name: /刷新/ })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /立即探测/ })).toBeInTheDocument();
  });

  it('空态：items 缺省 → `res.items || []` 右翼 → Empty 文案', async () => {
    mList.mockResolvedValue({} as never);
    renderPage();
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(1));
    expect(
      await screen.findByText(
        '尚未登记游戏数据库。登记只读账号后即可在此查看连接/锁等待/死锁指标。',
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText('登记数据库')).toBeInTheDocument(); // 工具按钮仍在
  });

  it('load 失败两翼：Error.message 透传 / 非 object 兜底「加载数据源失败」', async () => {
    mList.mockRejectedValueOnce(new Error('db-down'));
    renderPage();
    expect(await screen.findByText('db-down')).toBeInTheDocument();

    mList.mockRejectedValueOnce('plain' as never);
    renderPage();
    expect(await screen.findByText('加载数据源失败')).toBeInTheDocument();
    expect(mList).toHaveBeenCalledTimes(2);
  });

  it('canManage false：登记/编辑/删除全隐藏，卡面正常', async () => {
    mockCanOpsManage = false;
    renderPage();
    await waitLoad();

    expect(screen.queryByRole('button', { name: '登记数据库' })).not.toBeInTheDocument();
    expect(screen.queryByText('编辑')).not.toBeInTheDocument();
    expect(screen.queryByText('删除')).not.toBeInTheDocument();
    expect(screen.getByText('主库')).toBeInTheDocument();
  });

  it('刷新按钮重拉', async () => {
    renderPage();
    await waitLoad();

    fireEvent.click(screen.getByRole('button', { name: /刷新/ }));
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(2));
  });
});

describe('数据库监控 立即探测', () => {
  it('主链：results 归并 + 五卡指标矩阵 + 锁等待表双臂', async () => {
    renderPage();
    await waitLoad();

    fireEvent.click(screen.getByRole('button', { name: /立即探测/ }));
    await waitFor(() => expect(mProbe).toHaveBeenCalledTimes(1));
    expect(await screen.findByText('探测完成')).toBeInTheDocument();

    // 主库：满指标 + 红 Tag 计数 + volcano 死锁
    const c1 = cardOf('主库');
    expect(metricText(c1, '连接')).toBe('连接8/100');
    expect(within(c1).getByText('2 条').closest('.ant-tag')).toHaveClass('ant-tag-red');
    expect(within(c1).getByText('3').closest('.ant-tag')).toHaveClass('ant-tag-volcano');
    expect(metricText(c1, '延迟')).toBe('延迟12ms');
    // 锁等待表：45s 红 Tag / 5s 普通文本
    expect(within(c1).getByText('45').closest('.ant-tag')).toHaveClass('ant-tag-red');
    expect(within(c1).getByText('5').closest('.ant-tag')).toBeNull();
    expect(within(c1).getByText('SELECT * FROM orders')).toBeInTheDocument();
    expect(within(c1).getByText('UPDATE players SET gold = 1')).toBeInTheDocument();

    // 从库：max 0 → '?'、空锁等待 → 绿 0、null 死锁 → 不可用、延迟缺省 → '-'
    const c2 = cardOf('从库');
    expect(metricText(c2, '连接')).toBe('连接3/?');
    expect(within(c2).getByText('0').closest('.ant-tag')).toHaveClass('ant-tag-green');
    expect(within(c2).getByText('不可用')).toBeInTheDocument();
    expect(metricText(c2, '延迟')).toBe('延迟-ms');

    // 只读副本：connections 缺省 → '-/?'、死锁 0 → 绿 0
    const c3 = cardOf('只读副本');
    expect(metricText(c3, '连接')).toBe('连接-/?');
    expect(within(c3).getAllByText('0').length).toBeGreaterThanOrEqual(2);
    expect(metricText(c3, '延迟')).toBe('延迟5ms');

    // 报警库：ok:false → danger 文案
    expect(
      await within(cardOf('报警库')).findByText('探测失败：dial tcp timeout'),
    ).toBeInTheDocument();

    // 新库：无结果 → 停留提示
    expect(within(cardOf('新库')).getByText('点击「立即探测」获取实时状态')).toBeInTheDocument();
  });

  it('results 缺省 → `res.results || []` 右翼：全卡停留提示', async () => {
    mProbe.mockResolvedValue({} as never);
    renderPage();
    await waitLoad();

    fireEvent.click(screen.getByRole('button', { name: /立即探测/ }));
    await waitFor(() => expect(mProbe).toHaveBeenCalledTimes(1));
    expect(await screen.findByText('探测完成')).toBeInTheDocument();
    expect(screen.getAllByText('点击「立即探测」获取实时状态')).toHaveLength(5);
  });

  it('探测失败两翼：Error.message 透传 / 非 object 兜底「探测失败」', async () => {
    renderPage();
    await waitLoad();

    mProbe.mockRejectedValueOnce(new Error('probe-down'));
    fireEvent.click(screen.getByRole('button', { name: /立即探测/ }));
    expect(await screen.findByText('probe-down')).toBeInTheDocument();
    expect(screen.getAllByText('点击「立即探测」获取实时状态')).toHaveLength(5);

    mProbe.mockRejectedValueOnce('plain' as never);
    fireEvent.click(screen.getByRole('button', { name: /立即探测/ }));
    expect(await screen.findByText('探测失败')).toBeInTheDocument();
  });
});

describe('数据库监控 新建', () => {
  it('required 拦截 → 默认值全量载荷 → 已登记 + 重拉 + 关闭', async () => {
    renderPage();
    await waitLoad();

    fireEvent.click(screen.getByRole('button', { name: '登记数据库' }));
    await waitFor(() =>
      expect(document.querySelector('.ant-modal-title')?.textContent).toBe('登记游戏数据库'),
    );

    // 空提交：name + driver 双 required 拦截，不触达服务
    submitModal();
    await waitFor(() =>
      expect(document.querySelector('.ant-form-item-explain-error')).not.toBeNull(),
    );
    expect(mCreate).not.toHaveBeenCalled();

    // 填值：name + driver 选择 + dsn + 游戏/环境 + 阈值双字段
    fireEvent.change(screen.getByPlaceholderText('如 游戏主库-prod'), {
      target: { value: '新库登记' },
    });
    await pickModalSelect(0, 'MySQL');
    fireEvent.change(
      screen.getByPlaceholderText(
        'readonly:pass@tcp(10.0.0.1:3306)/game 或 postgres://ro:pass@10.0.0.2/game',
      ),
      { target: { value: 'mysql://ro@10.0.0.9/game' } },
    );
    fireEvent.change(screen.getByPlaceholderText('归属游戏'), { target: { value: 'demo' } });
    fireEvent.change(screen.getByPlaceholderText('prod'), { target: { value: 'prod' } });
    fireEvent.change(screen.getByPlaceholderText('5'), { target: { value: '7' } });
    fireEvent.change(screen.getByPlaceholderText('80'), { target: { value: '90' } });

    submitModal();
    await waitFor(() => expect(mCreate).toHaveBeenCalledTimes(1));
    expect(mCreate).toHaveBeenCalledWith(
      expect.objectContaining({
        name: '新库登记',
        driver: 'mysql',
        kind: 'self',
        dsn: 'mysql://ro@10.0.0.9/game',
        gameId: 'demo',
        env: 'prod',
        enabled: true,
        lockWaitWarn: 7,
        connWarnRatio: 90,
      }),
    );
    expect(await screen.findByText('数据源已登记')).toBeInTheDocument();
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(2));
    // onFinish true → 弹窗关闭（destroyOnHidden 卸载表单）
    await waitFor(() =>
      expect(screen.queryByPlaceholderText('如 游戏主库-prod')).not.toBeInTheDocument(),
    );
  });

  it('保存失败两翼：Error.message 透传 / 兜底「保存失败」，弹窗保持打开', async () => {
    renderPage();
    await waitLoad();

    fireEvent.click(screen.getByRole('button', { name: '登记数据库' }));
    await waitFor(() =>
      expect(document.querySelector('.ant-modal-title')?.textContent).toBe('登记游戏数据库'),
    );
    fireEvent.change(screen.getByPlaceholderText('如 游戏主库-prod'), {
      target: { value: 'x' },
    });
    await pickModalSelect(0, 'MySQL');
    fireEvent.change(
      screen.getByPlaceholderText(
        'readonly:pass@tcp(10.0.0.1:3306)/game 或 postgres://ro:pass@10.0.0.2/game',
      ),
      { target: { value: 'mysql://ro@h/x' } },
    );

    mCreate.mockRejectedValueOnce(new Error('save-boom'));
    submitModal();
    expect(await screen.findByText('save-boom')).toBeInTheDocument();
    expect(screen.getByPlaceholderText('如 游戏主库-prod')).toBeInTheDocument();
    expect(mList).toHaveBeenCalledTimes(1);

    mCreate.mockRejectedValueOnce('plain' as never);
    submitModal();
    expect(await screen.findByText('保存失败')).toBeInTheDocument();
    expect(screen.getByPlaceholderText('如 游戏主库-prod')).toBeInTheDocument();
  });
});

describe('数据库监控 编辑与删除', () => {
  it('编辑回填：DSN 掩码不回填 + extra 文案 + 阈值 0 归一 undefined → 已更新', async () => {
    renderPage();
    await waitLoad();

    // 阈值 0 的源：lockWaitWarn/connWarnRatio → undefined 归一臂
    fireEvent.click(within(cardOf('只读副本')).getByText('编辑'));
    await waitFor(() =>
      expect(document.querySelector('.ant-modal-title')?.textContent).toBe('编辑数据源：只读副本'),
    );
    // dsn 无回填 + extra 提示编辑语义
    const dsnInput = screen.getByPlaceholderText(
      'readonly:pass@tcp(10.0.0.1:3306)/game 或 postgres://ro:pass@10.0.0.2/game',
    );
    expect(dsnInput).toHaveValue('');
    expect(
      screen.getByText('编辑时留空表示不修改。务必使用只读监控账号，禁止 root/superuser'),
    ).toBeInTheDocument();
    expect(screen.getByDisplayValue('只读副本')).toBeInTheDocument();

    submitModal();
    await waitFor(() => expect(mUpdate).toHaveBeenCalledTimes(1));
    expect(mUpdate).toHaveBeenLastCalledWith(
      3,
      expect.objectContaining({
        name: '只读副本',
        driver: 'mysql',
        kind: 'aliyun',
        enabled: true,
      }),
    );
    // 阈值 0 → undefined 归一（Form 序列化丢弃 undefined 键）
    const payload = mUpdate.mock.calls[0][1] as Record<string, unknown>;
    expect(payload.lockWaitWarn).toBeUndefined();
    expect(payload.connWarnRatio).toBeUndefined();
    expect(await screen.findByText('数据源已更新')).toBeInTheDocument();
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(2));
  });

  it('编辑停用开关：enabled false 透传', async () => {
    renderPage();
    await waitLoad();

    fireEvent.click(within(cardOf('主库')).getByText('编辑'));
    await waitFor(() =>
      expect(document.querySelector('.ant-modal-title')?.textContent).toBe('编辑数据源：主库'),
    );
    expect(screen.getByDisplayValue('主库')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('switch'));
    submitModal();
    await waitFor(() =>
      expect(mUpdate).toHaveBeenLastCalledWith(1, expect.objectContaining({ enabled: false })),
    );
    expect(await screen.findByText('数据源已更新')).toBeInTheDocument();
  });

  it('删除 Popconfirm：标题 + deleteDBSource + 已删除 + 重拉；失败透传', async () => {
    renderPage();
    await waitLoad();

    fireEvent.click(within(cardOf('主库')).getByText('删除'));
    expect(await screen.findByText('删除「主库」？')).toBeInTheDocument();
    const popover = Array.from(document.querySelectorAll('.ant-popover')).find(
      (p) => !p.className.includes('ant-popover-hidden'),
    ) as HTMLElement;
    fireEvent.click(within(popover).getByRole('button', { name: /确/ }));

    await waitFor(() => expect(mDelete).toHaveBeenCalledWith(1));
    expect(await screen.findByText('已删除')).toBeInTheDocument();
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(2));

    mDelete.mockRejectedValueOnce(new Error('del-boom'));
    fireEvent.click(within(cardOf('从库')).getByText('删除'));
    expect(await screen.findByText('删除「从库」？')).toBeInTheDocument();
    const popover2 = Array.from(document.querySelectorAll('.ant-popover')).find(
      (p) => !p.className.includes('ant-popover-hidden'),
    ) as HTMLElement;
    fireEvent.click(within(popover2).getByRole('button', { name: /确/ }));
    expect(await screen.findByText('del-boom')).toBeInTheDocument();
    expect(mList).toHaveBeenCalledTimes(2);
  });
});
