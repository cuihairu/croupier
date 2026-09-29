/**
 * 告警规则 Tab 单测（覆盖率巡检：Ops/Alerts/AlertRulesTab.tsx 600 行 0% → 收口，
 * 告警中心簇第二件；入口页另见 index.test.tsx，其中本组件为桩替身）。
 *
 * 锁定契约：
 * - 列表渲染矩阵：条件列 `<metric> <operator> <threshold>` code 文本、
 *   level Tag 三色 + 未知 level default 色、连续命中 forCount>1 '{count} 次' /
 *   =1 '立即'、冷却 Math.round(v/60) '{minutes} 分钟'（90→2、7260→121）、
 *   agentFilter 空 → '全部'、lastFiredAt formatDateTime（合法格式化 /
 *   缺省 '-'）、Switch 启停态、操作列 编辑/删除；
 * - load 失败三翼（Error.message 透传 / 非 Error 兜底「加载告警规则失败」/
 *   空 message 同兜底）+ `res?.items || []` 右翼（resolve undefined → 空表）；
 * - 新建主链：ModalForm 默认值（operator gt / metric cpu.usagePercent /
 *   threshold 90 / forCount 1 / cooldownSeconds 300 / level warning）、
 *   name required 拦截（不触达服务）、提交载荷（agentFilter 空 → ''）、
 *   成功「已创建，下次指标上报即生效」+ 重拉 + 弹窗关闭；
 * - 自定义指标链：外层 Select 切「自定义指标」→ 内层 Input 显形 + label
 *   「自定义指标 key」；值失前缀（queueDepth）自动落非 preset 手输形态
 *   （placeholder 切换、不带 pattern 校验）照常提交；custom.queueDepth
 *   提交通过 + description 透传；
 * - 编辑链：initialValues 回填（编辑标题 / name DisplayValue / 载荷含
 *   description 与 agentFilter 归一 ''）+ 非 preset 指标（heap.usage）回填
 *   手输 Input 形态照常提交；成功「已更新」；
 * - toggleEnabled：updateAlertRule(id, {enabled}) → 「已停用」/「已启用」
 *   + 重拉；失败「操作失败」不重拉；
 * - 删除 Popconfirm：确认文案、deleteAlertRule(id)、成功「已删除」+ 重拉、
 *   失败两翼（Error.message / 空 message「删除失败」）；
 * - 保存失败：create reject → toast + return false 弹窗保持打开。
 *
 * mock 口径：services/api/ops 四规则函数 jest.mock；@umijs/max 本地 mock
 * （defaultMessage 即文案 + {count}/{minutes} values 内插）；antd/
 * pro-components 真实实现。
 *
 * 登记不可达（防御分支，不造假用例不删分支）：
 * - 499 行 pattern 校验的失败分支：isCustom 谓词与 pattern 谓词等价
 *   （metric.startsWith('custom.') ⇔ /^custom\./ 命中），值失前缀时重渲染
 *   即摘除 pattern 规则、转非 preset 手输形态，规则永不失败（自证性双
 *   保险，同 r27 wechat openid 回退族）；
 * - 467 行 `getFieldValue('metric') || ''` 右翼：新建走 initialValues
 *   默认 metric、编辑走行数据 metric，字段求值时从不为 undefined。
 *
 * 坑实证（antd6 沿用）：ModalForm 弹窗内三个 Select 的 DOM 序 =
 * [metric, operator, level]；下拉打开须 mouseDown 落 .ant-select 根 +
 * 点可见 option content；同一弹窗内连续两次开下拉须留 ≥60ms 时间隙
 * （rc-select 宏任务竞态，见 index.test.tsx 坑档）；ModalForm 标题与入口
 * 按钮同文本（findByText 撞多元素，锚 .ant-modal-title 的 textContent）；
 * Popconfirm 确认按钮取未隐藏 .ant-popover 内 role=button name=OK——无
 * ConfigProvider zh locale 时 antd6 Popconfirm 默认按钮文案是 en。
 */
import React from 'react';
import { configure, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { App } from 'antd';
import AlertRulesTab from '../AlertRulesTab';
import type { AlertRuleItem } from '@/services/api/ops';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

jest.mock('@/services/api/ops', () => ({
  listAlertRules: jest.fn(),
  createAlertRule: jest.fn(),
  updateAlertRule: jest.fn(),
  deleteAlertRule: jest.fn(),
}));

// mock* 前缀变量：babel-jest hoist 白名单；须稳定实例（load 的 useCallback 依赖 intl）
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
  listAlertRules,
  createAlertRule,
  updateAlertRule,
  deleteAlertRule,
} from '@/services/api/ops';

const mList = listAlertRules as jest.MockedFunction<typeof listAlertRules>;
const mCreate = createAlertRule as jest.MockedFunction<typeof createAlertRule>;
const mUpdate = updateAlertRule as jest.MockedFunction<typeof updateAlertRule>;
const mDelete = deleteAlertRule as jest.MockedFunction<typeof deleteAlertRule>;

// 覆盖翼：critical 红、forCount>1、cooldown 整除、agentFilter 缺省 → ''、
// lastFiredAt 合法格式化、description 透传
const rule1: AlertRuleItem = {
  id: 1,
  name: 'cpu-rule',
  description: 'desc-1',
  metric: 'cpu.usagePercent',
  operator: 'gt',
  threshold: 90,
  forCount: 3,
  cooldownSeconds: 300,
  level: 'critical',
  enabled: true,
  hitCount: 5,
  lastFiredAt: '2026-09-01T00:00:00Z',
  createdBy: 'admin',
};
// 覆盖翼：info 蓝、停用态、forCount=1 '立即'、cooldown 90→2、agentFilter 空
// → '全部'、lastFiredAt 缺省 '-'
const rule2: AlertRuleItem = {
  id: 2,
  name: 'mem-rule',
  metric: 'memory.usagePercent',
  operator: 'gte',
  threshold: 80,
  forCount: 1,
  cooldownSeconds: 90,
  level: 'info',
  enabled: false,
  hitCount: 0,
};
// 覆盖翼：自定义指标、未知 level → default 色、agentFilter 命中、
// cooldown 7260→121
const rule3: AlertRuleItem = {
  id: 3,
  name: 'queue-rule',
  metric: 'custom.queueDepth',
  operator: 'lt',
  threshold: 5,
  forCount: 2,
  cooldownSeconds: 7260,
  level: 'weird' as never,
  enabled: true,
  agentFilter: 'agent-1',
  hitCount: 1,
};
// 覆盖翼：非 preset 指标（编辑时内层 Input 手输形态）
const rule4: AlertRuleItem = {
  id: 4,
  name: 'heap-rule',
  metric: 'heap.usage',
  operator: 'lte',
  threshold: 70,
  forCount: 1,
  cooldownSeconds: 120,
  level: 'warning',
  enabled: true,
  hitCount: 0,
};

const rules = [rule1, rule2, rule3, rule4];

const lastFiredAtText = new Date('2026-09-01T00:00:00Z').toLocaleString('zh-CN', {
  year: 'numeric',
  month: '2-digit',
  day: '2-digit',
  hour: '2-digit',
  minute: '2-digit',
  second: '2-digit',
});

beforeEach(() => {
  jest.clearAllMocks();
  mList.mockResolvedValue({ items: rules });
  mCreate.mockResolvedValue({} as never);
  mUpdate.mockResolvedValue({} as never);
  mDelete.mockResolvedValue({ ok: true });
});

function renderTab() {
  return render(
    <App>
      <AlertRulesTab />
    </App>,
  );
}

/** 等首拉落定 */
async function waitLoad() {
  expect(await screen.findByText('cpu-rule')).toBeInTheDocument();
  await waitFor(() => expect(mList).toHaveBeenCalledTimes(1));
}

function rowOf(text: string) {
  return screen.getByText(text).closest('tr') as HTMLElement;
}

/** 弹窗 footer 确认（submitText=确定） */
function submitModal() {
  const ok = document.querySelector('.ant-modal-footer .ant-btn-primary') as HTMLElement;
  expect(ok).not.toBeNull();
  fireEvent.click(ok);
}

/** 弹窗内第 idx 个 Select 选 option（antd6：mouseDown 根 + 点可见 option content；
 * 连续两次开下拉之间留 60ms 时间隙避 rc-select 宏任务竞态） */
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

describe('告警规则 列表渲染', () => {
  it('条件/级别/连续命中/冷却/Agent/最近触发/启停/操作全列矩阵', async () => {
    renderTab();
    await waitLoad();

    // 条件列 code 文本
    expect(screen.getByText('cpu.usagePercent gt 90')).toBeInTheDocument();
    expect(screen.getByText('memory.usagePercent gte 80')).toBeInTheDocument();
    expect(screen.getByText('custom.queueDepth lt 5')).toBeInTheDocument();
    expect(screen.getByText('heap.usage lte 70')).toBeInTheDocument();

    // 级别 Tag：critical 红 / info 蓝 / warning 橙 / 未知 default（无三色类）
    expect(screen.getByText('critical').closest('.ant-tag')).toHaveClass('ant-tag-red');
    expect(screen.getByText('info').closest('.ant-tag')).toHaveClass('ant-tag-blue');
    const warnTag = screen.getByText('warning').closest('.ant-tag') as HTMLElement;
    expect(warnTag).toHaveClass('ant-tag-orange');
    const weirdTag = screen.getByText('weird').closest('.ant-tag') as HTMLElement;
    expect(weirdTag.className).not.toContain('ant-tag-red');
    expect(weirdTag.className).not.toContain('ant-tag-blue');
    expect(weirdTag.className).not.toContain('ant-tag-orange');

    // 连续命中：>1 '{count} 次' / =1 '立即'
    expect(screen.getByText('3 次')).toBeInTheDocument();
    expect(screen.getByText('2 次')).toBeInTheDocument();
    expect(screen.getAllByText('立即')).toHaveLength(2);

    // 冷却：Math.round(v/60) 分钟（300→5、90→2、7260→121、120→2）
    expect(screen.getByText('5 分钟')).toBeInTheDocument();
    expect(screen.getAllByText('2 分钟')).toHaveLength(2);
    expect(screen.getByText('121 分钟')).toBeInTheDocument();

    // Agent：空 → '全部'（rule1/rule2/rule4）/ 命中值
    expect(screen.getAllByText('全部')).toHaveLength(3);
    expect(screen.getByText('agent-1')).toBeInTheDocument();

    // 最近触发：合法格式化 / 缺省 '-'
    expect(screen.getByText(lastFiredAtText)).toBeInTheDocument();
    expect(screen.getAllByText('-')).toHaveLength(3);

    // 启停 Switch 4 个 + 操作列 编辑/删除
    expect(screen.getAllByRole('switch')).toHaveLength(4);
    expect(screen.getAllByText('编辑')).toHaveLength(4);
    expect(screen.getAllByText('删除')).toHaveLength(4);

    // 新建入口
    expect(screen.getByRole('button', { name: '新建规则' })).toBeInTheDocument();
  });

  it('load 失败三翼：Error.message / 非 Error 兜底 / 空 message 同兜底', async () => {
    mList.mockRejectedValueOnce(new Error('rules-down'));
    renderTab();
    expect(await screen.findByText('rules-down')).toBeInTheDocument();

    mList.mockRejectedValueOnce('plain' as never);
    renderTab();
    expect(await screen.findByText('加载告警规则失败')).toBeInTheDocument();
    expect(mList).toHaveBeenCalledTimes(2);

    // 空 message：errMsg '' → || 右翼兜底
    mList.mockRejectedValueOnce(new Error(''));
    const third = renderTab();
    expect(await screen.findByText('加载告警规则失败')).toBeInTheDocument();
    third.unmount();
  });

  it('响应缺省：res?.items || [] 右翼 → 空表', async () => {
    mList.mockResolvedValue(undefined as never);
    renderTab();
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(1));
    await waitFor(() =>
      expect(document.querySelector('.ant-table .ant-empty-description')).not.toBeNull(),
    );
  });
});

describe('告警规则 新建', () => {
  it('默认值主链：name required 拦截 → 提交载荷（agentFilter 空 → 空串）→ 成功关闭 + 重拉', async () => {
    renderTab();
    await waitLoad();

    fireEvent.click(screen.getByRole('button', { name: '新建规则' }));
    await waitFor(() =>
      expect(document.querySelector('.ant-modal-title')?.textContent).toBe('新建规则'),
    );

    // 空 name 提交 → required 拦截，不触达服务
    submitModal();
    await waitFor(() =>
      expect(document.querySelector('.ant-form-item-explain-error')).not.toBeNull(),
    );
    expect(mCreate).not.toHaveBeenCalled();

    // 填 name 后提交：默认值全量载荷
    fireEvent.change(screen.getByPlaceholderText('CPU 持续高负载'), {
      target: { value: 'my-rule' },
    });
    submitModal();
    await waitFor(() => expect(mCreate).toHaveBeenCalledTimes(1));
    expect(mCreate).toHaveBeenCalledWith({
      name: 'my-rule',
      description: undefined,
      metric: 'cpu.usagePercent',
      operator: 'gt',
      threshold: 90,
      forCount: 1,
      cooldownSeconds: 300,
      level: 'warning',
      agentFilter: '',
    });
    expect(await screen.findByText('已创建，下次指标上报即生效')).toBeInTheDocument();
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(2));
    // onFinish true → 弹窗关闭（destroyOnHidden 卸载表单）
    await waitFor(() =>
      expect(screen.queryByPlaceholderText('CPU 持续高负载')).not.toBeInTheDocument(),
    );
  });

  it('自定义指标链：切「自定义指标」→ 失前缀被 pattern 拦截 → custom.queueDepth 提交', async () => {
    renderTab();
    await waitLoad();

    fireEvent.click(screen.getByRole('button', { name: '新建规则' }));
    await waitFor(() =>
      expect(document.querySelector('.ant-modal-title')?.textContent).toBe('新建规则'),
    );

    // 外层 Select（DOM 序 [metric, operator, level] 之首）切自定义指标
    await pickModalSelect(0, '自定义指标');
    // 内层 Input 显形：label 切「自定义指标 key」+ 专属 placeholder
    expect(await screen.findByText('自定义指标 key')).toBeInTheDocument();
    const customInput = screen.getByPlaceholderText('custom.queueDepth');
    expect(customInput).toBeInTheDocument();
    // 此时值 'custom.'（选择项 value）

    fireEvent.change(screen.getByPlaceholderText('CPU 持续高负载'), {
      target: { value: 'queue-rule' },
    });
    fireEvent.change(screen.getByPlaceholderText('用于…（可选）'), {
      target: { value: '队列深度告警' },
    });

    // 失前缀：改值为 queueDepth → isCustom 翻 false，同一内层 Input 切非
    // preset 手输形态（placeholder 切换、pattern 规则随重渲染摘除），照常提交
    fireEvent.change(customInput, { target: { value: 'queueDepth' } });
    await waitFor(() =>
      expect(
        screen.getByPlaceholderText('disk./data.usedPercent 或 custom.queueDepth'),
      ).toBeInTheDocument(),
    );

    // 带前缀提交通过
    fireEvent.change(customInput, { target: { value: 'custom.queueDepth' } });
    submitModal();
    await waitFor(() => expect(mCreate).toHaveBeenCalledTimes(1));
    expect(mCreate).toHaveBeenCalledWith(
      expect.objectContaining({
        name: 'queue-rule',
        description: '队列深度告警',
        metric: 'custom.queueDepth',
      }),
    );
    expect(await screen.findByText('已创建，下次指标上报即生效')).toBeInTheDocument();
  });

  it('保存失败：toast + 弹窗保持打开（return false）', async () => {
    renderTab();
    await waitLoad();

    fireEvent.click(screen.getByRole('button', { name: '新建规则' }));
    await waitFor(() =>
      expect(document.querySelector('.ant-modal-title')?.textContent).toBe('新建规则'),
    );
    fireEvent.change(screen.getByPlaceholderText('CPU 持续高负载'), {
      target: { value: 'my-rule' },
    });

    mCreate.mockRejectedValueOnce(new Error('save-boom'));
    submitModal();
    expect(await screen.findByText('save-boom')).toBeInTheDocument();
    expect(screen.getByPlaceholderText('CPU 持续高负载')).toBeInTheDocument();
    expect(mList).toHaveBeenCalledTimes(1);
  });
});

describe('告警规则 编辑', () => {
  it('回填 + 提交载荷（description 透传 / agentFilter 缺省 → 空串）→ 已更新', async () => {
    renderTab();
    await waitLoad();

    fireEvent.click(within(rowOf('cpu-rule')).getByText('编辑'));
    await waitFor(() =>
      expect(document.querySelector('.ant-modal-title')?.textContent).toBe('编辑规则'),
    );
    expect(screen.getByDisplayValue('cpu-rule')).toBeInTheDocument();
    expect(screen.getByDisplayValue('desc-1')).toBeInTheDocument();
    expect(screen.getByDisplayValue('90')).toBeInTheDocument();

    submitModal();
    await waitFor(() => expect(mUpdate).toHaveBeenCalledTimes(1));
    expect(mUpdate).toHaveBeenCalledWith(1, {
      name: 'cpu-rule',
      description: 'desc-1',
      metric: 'cpu.usagePercent',
      operator: 'gt',
      threshold: 90,
      forCount: 3,
      cooldownSeconds: 300,
      level: 'critical',
      agentFilter: '',
    });
    expect(await screen.findByText('已更新')).toBeInTheDocument();
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(2));
  });

  it('非 preset 指标（heap.usage）：内层 Input 手输形态照常提交 + agentFilter 透传', async () => {
    renderTab();
    await waitLoad();

    fireEvent.click(within(rowOf('heap-rule')).getByText('编辑'));
    await waitFor(() =>
      expect(document.querySelector('.ant-modal-title')?.textContent).toBe('编辑规则'),
    );
    // 非 preset 且非 custom：内层 Input 以手输 placeholder 可见
    expect(
      screen.getByPlaceholderText('disk./data.usedPercent 或 custom.queueDepth'),
    ).toBeInTheDocument();

    submitModal();
    await waitFor(() => expect(mUpdate).toHaveBeenCalledTimes(1));
    expect(mUpdate).toHaveBeenLastCalledWith(
      4,
      expect.objectContaining({ metric: 'heap.usage', operator: 'lte', agentFilter: '' }),
    );
  });

  it('自定义指标行编辑：值保留 + agentFilter 命中透传', async () => {
    renderTab();
    await waitLoad();

    fireEvent.click(within(rowOf('queue-rule')).getByText('编辑'));
    await waitFor(() =>
      expect(document.querySelector('.ant-modal-title')?.textContent).toBe('编辑规则'),
    );
    expect(await screen.findByText('自定义指标 key')).toBeInTheDocument();

    submitModal();
    await waitFor(() =>
      expect(mUpdate).toHaveBeenLastCalledWith(
        3,
        expect.objectContaining({ metric: 'custom.queueDepth', agentFilter: 'agent-1' }),
      ),
    );
    expect(await screen.findByText('已更新')).toBeInTheDocument();
  });
});

describe('告警规则 启停', () => {
  it('关 → 已停用 / 开 → 已启用（各带重拉）；失败「操作失败」不重拉', async () => {
    renderTab();
    await waitLoad();

    const switches = screen.getAllByRole('switch');
    // rule1 enabled=true → 关
    fireEvent.click(switches[0]);
    await waitFor(() => expect(mUpdate).toHaveBeenCalledWith(1, { enabled: false }));
    expect(await screen.findByText('已停用')).toBeInTheDocument();
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(2));

    // rule2 enabled=false → 开
    fireEvent.click(switches[1]);
    await waitFor(() => expect(mUpdate).toHaveBeenLastCalledWith(2, { enabled: true }));
    expect(await screen.findByText('已启用')).toBeInTheDocument();

    // 失败：Error('') → 兜底「操作失败」
    mUpdate.mockRejectedValueOnce(new Error(''));
    fireEvent.click(switches[2]);
    expect(await screen.findByText('操作失败')).toBeInTheDocument();
  });
});

describe('告警规则 删除', () => {
  it('Popconfirm 确认 → deleteAlertRule + 已删除 + 重拉', async () => {
    renderTab();
    await waitLoad();

    fireEvent.click(within(rowOf('heap-rule')).getByText('删除'));
    expect(await screen.findByText('确认删除该规则？')).toBeInTheDocument();
    const popover = Array.from(document.querySelectorAll('.ant-popover')).find(
      (p) => !p.className.includes('ant-popover-hidden'),
    ) as HTMLElement;
    fireEvent.click(within(popover).getByRole('button', { name: 'OK' }));

    await waitFor(() => expect(mDelete).toHaveBeenCalledWith(4));
    expect(await screen.findByText('已删除')).toBeInTheDocument();
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(2));
  });

  it('删除失败两翼：Error.message / 空 message「删除失败」', async () => {
    renderTab();
    await waitLoad();

    mDelete.mockRejectedValueOnce(new Error('del-boom'));
    fireEvent.click(within(rowOf('heap-rule')).getByText('删除'));
    expect(await screen.findByText('确认删除该规则？')).toBeInTheDocument();
    let popover = Array.from(document.querySelectorAll('.ant-popover')).find(
      (p) => !p.className.includes('ant-popover-hidden'),
    ) as HTMLElement;
    fireEvent.click(within(popover).getByRole('button', { name: 'OK' }));
    expect(await screen.findByText('del-boom')).toBeInTheDocument();

    mDelete.mockRejectedValueOnce(new Error(''));
    fireEvent.click(within(rowOf('heap-rule')).getByText('删除'));
    expect(await screen.findByText('确认删除该规则？')).toBeInTheDocument();
    popover = Array.from(document.querySelectorAll('.ant-popover')).find(
      (p) => !p.className.includes('ant-popover-hidden'),
    ) as HTMLElement;
    fireEvent.click(within(popover).getByRole('button', { name: 'OK' }));
    expect(await screen.findByText('删除失败')).toBeInTheDocument();
    expect(mList).toHaveBeenCalledTimes(1);
  });
});
