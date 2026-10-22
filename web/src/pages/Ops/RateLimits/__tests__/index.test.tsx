/**
 * 限速管理页单测（覆盖率巡检：Ops/RateLimits/index.tsx 513 行 0% → 收口，
 * 零测试页排行第四）。
 *
 * 锁定契约：
 * - 挂载链：listRateLimits + listOpsFunctions（函数 id 下拉源）+
 *   listOpsNodes（type 过滤 agent + id/addr 双回退）三连拉；表格矩阵：
 *   scope 双 Tag（函数蓝/服务紫）、percent 缺省 100 右翼、match 对象
 *   Tag 展开 / 缺省 '-'；
 * - 编辑回填（match 拆解契约）：标准四键（gameId/env/region/zone）映射
 *   matchGameId/... 平铺字段，其余键还原为 labels JSON 文本
 *   （JSON.stringify(labels, null, 2)）；无多余键时 matchLabels 留空；
 * - 新建主链：resetFields + 默认值（scope function / limitQps 10 /
 *   percent 100）；scope 切换联动 key 清空 + key 列双 label/placeholder
 *   （函数ID ↔ agent_id）+ 选项源切换（functions ↔ agents）；
 * - 提交主链：required 拦截（scope/key/limitQps number min 1）→
 *   putRateLimits([rule]) 载荷组装（match 四键条件收集、percent
 *   0<v<=100 才入载荷、labels JSON 合并进 match）→「已保存」+ 弹窗关闭
 *   + 重拉；labels 非法 JSON → 警告「标签JSON解析失败，已忽略」且按
 *   无 labels 提交；labels 非对象（数组）→ 静默忽略；
 * - 预览：手动「预览命中」——function scope → info「仅支持服务级预览」
 *   不触达；service scope → previewRateLimit 载荷（match 四键透传）→
 *   命中实例文案 + agent 列表按 qps1m 降序 + 仅显示超限 Checkbox 过滤；
 *   失败两翼（Error.message 透传 / 非 Error「预览失败」）；
 * - 自动预览（onValuesChange → 200ms 防抖 effect）：service+key+limitQps
 *   齐备自动拉 previewRateLimit，缺一（scope function）→ setPreview(null)
 *   面板消失；
 * - CSV 导出：表头行 + agent 行（缺省字段补 ''、qps1m toFixed(2)）→
 *   exportToCSV('rate_limit_preview.csv', rows)；
 * - 删除：modal.confirm（标题「删除限速」替换、内容 scope:key 内插）→
 *   deleteRateLimit(scope, key) →「已删除」+ 重拉；
 * - load 失败：listRateLimits reject → 空表（finally 复位 loading）；
 *   listOpsFunctions/listOpsNodes 各自 reject 静默吞（不白屏）。
 *
 * mock 口径：services/api/ops 六函数 jest.mock；utils/export 的
 * exportToCSV jest.mock（jsdom 无下载）；@umijs/max 本地 mock（id→中文
 * 表 + defaultMessage 内插）；antd/pro-components 真实实现。
 *
 * 登记不可达（防御分支，不造假用例不删分支）：
 * - CSV 导出 onClick 的 try/catch catch 臂：rows 构造纯同步内存映射，
 *   preview.agents 已 normalize（无抛出路径）；
 * - 预览面板 IIFE 内 `preview?.agents` 的 null 侧：外层 `preview &&` 同
 *   渲染条件先行，进入时 preview 恒非空。
 *
 * 本轮修定的真缺陷（回归锁定）：
 * - named Form.Item 的子必须是「单一 React 元素」——原代码五个字段
 *   （limitQps/percent/matchGameId~matchZone）写成 `{' '}<Input/>{' '}`，
 *   children 成了三元素数组，antd 的 cloneElement value/onChange 注入
 *   直接跳过：InputNumber/Input 完全脱管（默认值 10/100 不显示、键入
 *   永不落库，提交恒按默认 limitQps 10 + percent 100，match 四键全丢，
 *   用户无法从 UI 配置限速参数）；Select/TextArea 是单子不受影响——
 *   这正是「看似正常、store 静默不更新」的半坏形态。回归锚：提交载荷
 *   percent 0 不入、region/zone 落 match、InputNumber 显示默认值。
 * - 编辑/删除按钮文案是双字中文 Button，antd 自动插空格渲染为「编 辑」
 *   /「删 除」，getByText 须带空格（Tag 内的双字不受此影响）。
 *
 * 坑实证（antd6 沿用）：本页是原生 antd Modal（非 ModalForm），onOk 提交
 * 锚 .ant-modal-footer .ant-btn-primary（确定）；modal.confirm 锚
 * .ant-modal-confirm-btns .ant-btn-primary（App.useApp hook 版）；
 * InputNumber 取值走内层 input fireEvent.change（min=1 下手输 0 只变更
 * 显示、不派发 onChange，falsy 臂仅编辑回填可达）；自动预览防抖 200ms 须
 * 等 >200ms 真实时钟；Form.Item shouldUpdate 渲染的 key Select 在 DOM 序
 * 上是弹窗内第 2 个 .ant-select；jest.clearAllMocks 不清
 * mockRejectedValueOnce 队列——外层 catch 短路内层拉取时预挂的 Once 会
 * 跨用例毒化函数源（下拉空 options），Once 必须在本用例内消费殆尽。
 */
import React from 'react';
import { configure, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { App } from 'antd';
import OpsRateLimitsPage from '../index';
import type { RateLimitRule } from '@/services/api/ops';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

jest.mock('@/services/api/ops', () => ({
  listRateLimits: jest.fn(),
  putRateLimits: jest.fn(),
  deleteRateLimit: jest.fn(),
  previewRateLimit: jest.fn(),
  listOpsFunctions: jest.fn(),
  listOpsNodes: jest.fn(),
}));

jest.mock('@/utils/export', () => ({
  exportToCSV: jest.fn(),
}));

// id→中文表（本页 formatMessage 多数无 defaultMessage，锚点取真实文案）
const ZH: Record<string, string> = {
  'pages.scope': '范围',
  'pages.permissions.actions': '操作',
  'pages.permissions.edit.button': '编辑',
  'pages.permissions.save.success': '已保存权限配置',
  'pages.rate.limits.management': '限速管理',
  'pages.rate.limits.new.rule': '新建规则',
  'pages.rate.limits.edit.rule': '编辑限速规则',
  'pages.rate.limits.delete.confirm': '确定删除规则 {scope}:{key}?',
  'pages.rate.limits.qps': '限速 QPS',
  'pages.rate.limits.percentage': '比例(%)',
  'pages.rate.limits.match': '匹配（可选）',
  'pages.rate.limits.preview': '预览命中',
  'pages.rate.limits.export.csv': '导出 CSV',
  'pages.rate.limits.functions': '函数',
  'pages.rate.limits.services': '服务',
  'pages.rate.limits.key.function': 'Key（函数ID）',
  'pages.rate.limits.key.agent': 'Key（Agent ID）',
};
const mockIntl = {
  formatMessage: (
    opts: { id?: string; defaultMessage?: string },
    values?: Record<string, string | number>,
  ) => {
    let msg = opts.id && ZH[opts.id] ? ZH[opts.id] : (opts.defaultMessage ?? opts.id ?? '');
    if (values) {
      for (const [k, v] of Object.entries(values)) {
        msg = msg.split(`{${k}}`).join(String(v));
      }
    }
    return msg;
  },
};
jest.mock('@umijs/max', () => ({
  FormattedMessage: ({ id, defaultMessage }: { id?: string; defaultMessage?: string }) => (
    <>{(id && ZH[id]) || defaultMessage || id || ''}</>
  ),
  useIntl: () => mockIntl,
}));

import {
  listRateLimits,
  putRateLimits,
  deleteRateLimit,
  previewRateLimit,
  listOpsFunctions,
  listOpsNodes,
} from '@/services/api/ops';
import { exportToCSV } from '@/utils/export';

const mList = listRateLimits as jest.MockedFunction<typeof listRateLimits>;
const mPut = putRateLimits as jest.MockedFunction<typeof putRateLimits>;
const mDelete = deleteRateLimit as jest.MockedFunction<typeof deleteRateLimit>;
const mPreview = previewRateLimit as jest.MockedFunction<typeof previewRateLimit>;
const mFuncs = listOpsFunctions as jest.MockedFunction<typeof listOpsFunctions>;
const mNodes = listOpsNodes as jest.MockedFunction<typeof listOpsNodes>;
const mCsv = exportToCSV as jest.MockedFunction<typeof exportToCSV>;

const mk = (
  over: Partial<RateLimitRule> & Pick<RateLimitRule, 'scope' | 'key' | 'limitQps'>,
): RateLimitRule => ({
  ...over,
});

// 覆盖翼：函数域 + 标准四键 match + percent
const r1 = mk({
  scope: 'function',
  key: 'fn.reload_item',
  limitQps: 10,
  percent: 80,
  match: { gameId: 'demo', env: 'prod', region: 'cn-east', zone: 'z1' },
});
// 覆盖翼：服务域 + 多余键 match（labels JSON 回填臂）+ percent 缺省 → 100
const r2 = mk({
  scope: 'service',
  key: 'agent://game-1',
  limitQps: 500,
  match: { gameId: 'demo', channel: 'wechat', tier: 'vip' },
});
// 覆盖翼：无 match → '-'
const r3 = mk({ scope: 'function', key: 'fn.bare', limitQps: 5 });
// 覆盖翼：percent 0（falsy 左臂，仅编辑回填可达——min=1 下手输 0 被
// rc-inputnumber 扣为中间态不派发）
const r4 = mk({ scope: 'function', key: 'fn.pzero', limitQps: 7, percent: 0 });
// 覆盖翼：percent 超 100（>100 右臂，同因仅编辑回填可达）
const r5 = mk({ scope: 'function', key: 'fn.p150', limitQps: 9, percent: 150 });

beforeEach(() => {
  jest.clearAllMocks();
  mList.mockResolvedValue({ rules: [r1, r2, r3, r4, r5] });
  mFuncs.mockResolvedValue({
    functions: [
      { id: 'fn.reload_item' },
      { id: 'fn.kick_player' },
      // 覆盖翼：id 缺省被 filter(Boolean) 剔除
      { id: '' },
    ],
  });
  mNodes.mockResolvedValue({
    nodes: [
      { id: 'agent://game-1', type: 'agent' },
      { id: 'agent://game-2', type: 'agent' },
      // 覆盖翼：type 缺省默认按 agent
      { addr: '10.0.0.3:19091' },
      // 覆盖翼：非 agent 类型被剔除
      { id: 'srv://meta', type: 'server' },
    ],
  } as never);
  mPut.mockResolvedValue(undefined);
  mDelete.mockResolvedValue(undefined);
  mPreview.mockResolvedValue({ matched: 0, agents: [] });
});

function renderPage() {
  return render(
    <App>
      <OpsRateLimitsPage />
    </App>,
  );
}

/** 等三连拉落定 */
async function waitLoad() {
  expect(await screen.findByText('fn.reload_item')).toBeInTheDocument();
  await waitFor(() => expect(mNodes).toHaveBeenCalled());
}

/** 表格行 */
function rowOf(key: string) {
  return within(document.querySelector('.ant-table') as HTMLElement)
    .getByText(key)
    .closest('tr') as HTMLElement;
}

/** 弹窗 footer 确定按钮（原生 Modal onOk） */
function submitForm() {
  const ok = document.querySelector('.ant-modal-footer .ant-btn-primary') as HTMLElement;
  expect(ok).not.toBeNull();
  fireEvent.click(ok);
}

/** 弹窗内第 idx 个 Select 选 option（rc-select 选项渲染晚于容器去隐藏，须等） */
async function pickModalSelect(idx: number, matcher: (t: string) => boolean) {
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
  const option = await waitFor(() => {
    const hit = Array.from(dropdown.querySelectorAll('.ant-select-item-option-content')).find((o) =>
      matcher(o.textContent ?? ''),
    ) as HTMLElement;
    expect(hit).not.toBeUndefined();
    return hit;
  });
  fireEvent.click(option);
}

/** modal.confirm 确认按钮 */
function confirmDialog() {
  const btn = document.querySelector('.ant-modal-confirm-btns .ant-btn-primary') as HTMLElement;
  expect(btn).not.toBeNull();
  fireEvent.click(btn);
}

/** 以 service 域 + agent key + qps 填表（预览/提交共用） */
async function fillServiceRule() {
  await pickModalSelect(0, (t) => t === '服务');
  await pickModalSelect(1, (t) => t.includes('agent://game-1'));
  const qpsInput = document.querySelector('.ant-modal .ant-input-number-input') as HTMLInputElement;
  fireEvent.change(qpsInput, { target: { value: '500' } });
}

describe('限速管理 初始渲染', () => {
  it('三连拉 + 表格矩阵：scope 双 Tag / percent 缺省 100 / match 展开与「-」', async () => {
    renderPage();
    await waitLoad();

    // scope Tag 双态
    expect(within(rowOf('fn.reload_item')).getByText('函数').closest('.ant-tag')).toHaveClass(
      'ant-tag-blue',
    );
    expect(within(rowOf('agent://game-1')).getByText('服务').closest('.ant-tag')).toHaveClass(
      'ant-tag-purple',
    );

    // percent：显式 80 / 缺省 → 100
    expect(within(rowOf('fn.reload_item')).getByText('80')).toBeInTheDocument();
    expect(within(rowOf('agent://game-1')).getByText('100')).toBeInTheDocument();

    // match 对象 Tag 展开（标准键 + 多余键）与无 match '-'
    expect(within(rowOf('fn.reload_item')).getByText('gameId:demo')).toBeInTheDocument();
    expect(within(rowOf('fn.reload_item')).getByText('zone:z1')).toBeInTheDocument();
    expect(within(rowOf('agent://game-1')).getByText('channel:wechat')).toBeInTheDocument();
    expect(within(rowOf('fn.bare')).getByText('-')).toBeInTheDocument();

    // QPS 列
    expect(within(rowOf('fn.bare')).getByText('5')).toBeInTheDocument();

    // 工具栏
    expect(screen.getByRole('button', { name: '新建规则' })).toBeEnabled();
  });

  it('load 失败：listRateLimits reject → 提示 + 空表；funcs/nodes 各自 reject 静默吞', async () => {
    // 顶层 reject 走外层 catch，会短路 funcs/nodes 内层拉取——勿为其预挂
    // Once（jest.clearAllMocks 不清 Once 队列，未消费的 reject 会毒化后续
    // 用例的函数源，表象是 key 下拉空 options）
    mList.mockRejectedValueOnce(new Error('rl-down'));
    const { unmount } = renderPage();
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(1));
    expect(await screen.findByText('加载限速规则失败')).toBeInTheDocument();
    await waitFor(() =>
      expect(document.querySelector('.ant-table .ant-empty-description')).not.toBeNull(),
    );
    unmount();

    // 二连：funcs/nodes 双双 reject → 内层 catch 静默、规则仍渲染
    mFuncs.mockRejectedValueOnce(new Error('fn-down2'));
    mNodes.mockRejectedValueOnce(new Error('nd-down2'));
    renderPage();
    expect(await screen.findByText('fn.reload_item')).toBeInTheDocument();

    // 三连：响应缺省键（rules/functions/nodes 双 || 右臂）→ 空表不炸
    mList.mockResolvedValueOnce({} as never);
    mFuncs.mockResolvedValueOnce({} as never);
    mNodes.mockResolvedValueOnce({} as never);
    renderPage();
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(3));
    await waitFor(() =>
      expect(document.querySelector('.ant-table .ant-empty-description')).not.toBeNull(),
    );
  });
});

describe('限速管理 新建与编辑', () => {
  it('新建主链：默认值 + function 选项源 + required 拦截 → 载荷 → 已保存 + 重拉 + 关闭', async () => {
    renderPage();
    await waitLoad();

    fireEvent.click(screen.getByRole('button', { name: '新建规则' }));
    await waitFor(() =>
      expect(document.querySelector('.ant-modal-title')?.textContent).toBe('编辑限速规则'),
    );

    // 默认：function 域 + key label（函数ID）
    expect(document.querySelector('.ant-modal .ant-select-content')?.textContent).toBe('函数');
    expect(screen.getByText('Key（函数ID）')).toBeInTheDocument();

    // 空提交：key required（scope/limitQps 有默认值）
    submitForm();
    await waitFor(() =>
      expect(document.querySelector('.ant-form-item-explain-error')).not.toBeNull(),
    );
    expect(mPut).not.toHaveBeenCalled();

    // 选函数 key + 填 match 四键
    await pickModalSelect(1, (t) => t === 'fn.kick_player');
    fireEvent.change(screen.getByPlaceholderText('game_id'), { target: { value: 'demo' } });
    fireEvent.change(screen.getByPlaceholderText('env'), { target: { value: 'prod' } });

    submitForm();
    await waitFor(() => expect(mPut).toHaveBeenCalledTimes(1));
    expect(mPut).toHaveBeenCalledWith([
      {
        scope: 'function',
        key: 'fn.kick_player',
        limitQps: 10,
        match: { gameId: 'demo', env: 'prod' },
        percent: 100,
      },
    ]);
    expect(await screen.findByText('已保存')).toBeInTheDocument();
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(document.querySelector('.ant-modal')).toBeNull());
  });

  it('scope 切换：key 清空 + 选项源切 agents + key label 切 Agent ID', async () => {
    renderPage();
    await waitLoad();

    fireEvent.click(screen.getByRole('button', { name: '新建规则' }));
    await waitFor(() =>
      expect(document.querySelector('.ant-modal-title')?.textContent).toBe('编辑限速规则'),
    );
    await pickModalSelect(1, (t) => t === 'fn.reload_item');
    await pickModalSelect(0, (t) => t === '服务');
    // 切域清 key：label 切 Agent ID，placeholder agent_id
    await waitFor(() => expect(screen.getByText('Key（Agent ID）')).toBeInTheDocument());
    expect(document.querySelector('.ant-modal .ant-select-show-search')).not.toBeNull();

    // agent 选项源（id 缺省回退 addr；server 被剔）
    await pickModalSelect(1, (t) => t === '10.0.0.3:19091');
    submitForm();
    await waitFor(() => expect(mPut).toHaveBeenCalledTimes(1));
    expect(mPut.mock.calls[0][0][0]).toMatchObject({ scope: 'service', key: '10.0.0.3:19091' });
  });

  it('percent 边界：编辑回填 0/超 100 不入载荷；labels JSON 合并进 match', async () => {
    renderPage();
    await waitLoad();

    fireEvent.click(screen.getByRole('button', { name: '新建规则' }));
    await waitFor(() =>
      expect(document.querySelector('.ant-modal-title')?.textContent).toBe('编辑限速规则'),
    );
    await pickModalSelect(1, (t) => t === 'fn.kick_player');

    // 新建链：region/zone 落 match + labels JSON 合并；percent 未动默认 100
    fireEvent.change(screen.getByPlaceholderText('region'), { target: { value: 'cn-east' } });
    fireEvent.change(screen.getByPlaceholderText('zone'), { target: { value: 'z1' } });
    fireEvent.change(document.querySelector('.ant-modal textarea') as HTMLElement, {
      target: { value: '{"channel":"wechat"}' },
    });
    submitForm();
    await waitFor(() => expect(mPut).toHaveBeenCalledTimes(1));
    expect(mPut).toHaveBeenCalledWith([
      {
        scope: 'function',
        key: 'fn.kick_player',
        limitQps: 10,
        percent: 100,
        match: { region: 'cn-east', zone: 'z1', channel: 'wechat' },
      },
    ]);

    // 编辑 percent=0 规则：falsy 左翼不收集（min=1 下手输 0 不派发，仅回填可达）
    fireEvent.click(within(rowOf('fn.pzero')).getByText(/编 辑/));
    await waitFor(() =>
      expect(document.querySelector('.ant-modal-title')?.textContent).toBe('编辑限速规则'),
    );
    submitForm();
    await waitFor(() => expect(mPut).toHaveBeenCalledTimes(2));
    expect(mPut.mock.calls[1][0][0]).toEqual({ scope: 'function', key: 'fn.pzero', limitQps: 7 });

    // 编辑 percent=150 规则：>100 右翼不收集
    fireEvent.click(within(rowOf('fn.p150')).getByText(/编 辑/));
    await waitFor(() =>
      expect(document.querySelector('.ant-modal-title')?.textContent).toBe('编辑限速规则'),
    );
    submitForm();
    await waitFor(() => expect(mPut).toHaveBeenCalledTimes(3));
    expect(mPut.mock.calls[2][0][0]).toEqual({ scope: 'function', key: 'fn.p150', limitQps: 9 });
  });

  it('labels 非法 JSON → 警告忽略；数组 JSON → 静默忽略', async () => {
    renderPage();
    await waitLoad();

    fireEvent.click(screen.getByRole('button', { name: '新建规则' }));
    await waitFor(() =>
      expect(document.querySelector('.ant-modal-title')?.textContent).toBe('编辑限速规则'),
    );
    await pickModalSelect(1, (t) => t === 'fn.kick_player');
    fireEvent.change(document.querySelector('.ant-modal textarea') as HTMLElement, {
      target: { value: 'not-json' },
    });
    submitForm();
    await waitFor(() => expect(mPut).toHaveBeenCalledTimes(1));
    expect(await screen.findByText('标签JSON解析失败，已忽略')).toBeInTheDocument();
    expect(mPut.mock.calls[0][0][0].match).toBeUndefined();

    // 数组：合法 JSON 但非对象 → 不合并不警告
    fireEvent.click(screen.getByRole('button', { name: '新建规则' }));
    await waitFor(() =>
      expect(document.querySelector('.ant-modal-title')?.textContent).toBe('编辑限速规则'),
    );
    await pickModalSelect(1, (t) => t === 'fn.kick_player');
    fireEvent.change(document.querySelector('.ant-modal textarea') as HTMLElement, {
      target: { value: '[1,2]' },
    });
    submitForm();
    await waitFor(() => expect(mPut).toHaveBeenCalledTimes(2));
    expect(mPut.mock.calls[1][0][0].match).toBeUndefined();

    // 仅 labels 无标准键：合并落 `rule.match || {}` 右臂（match 从零建起）
    fireEvent.click(screen.getByRole('button', { name: '新建规则' }));
    await waitFor(() =>
      expect(document.querySelector('.ant-modal-title')?.textContent).toBe('编辑限速规则'),
    );
    await pickModalSelect(1, (t) => t === 'fn.kick_player');
    fireEvent.change(document.querySelector('.ant-modal textarea') as HTMLElement, {
      target: { value: '{"solo":"1"}' },
    });
    submitForm();
    await waitFor(() => expect(mPut).toHaveBeenCalledTimes(3));
    expect(mPut.mock.calls[2][0][0].match).toEqual({ solo: '1' });
  });

  it('编辑回填：标准四键平铺 + 多余键 labels JSON 文本；提交重组 match', async () => {
    renderPage();
    await waitLoad();

    fireEvent.click(within(rowOf('agent://game-1')).getByText(/编 辑/));
    await waitFor(() =>
      expect(document.querySelector('.ant-modal-title')?.textContent).toBe('编辑限速规则'),
    );
    // 回填：scope 服务 + key 选中 + match 平铺 + labels JSON 文本
    expect(screen.getByText('Key（Agent ID）')).toBeInTheDocument();
    expect(screen.getByDisplayValue('demo')).toBeInTheDocument();
    const ta = document.querySelector('.ant-modal textarea') as HTMLTextAreaElement;
    expect(JSON.parse(ta.value)).toEqual({ channel: 'wechat', tier: 'vip' });

    // 原样提交（Form 重挂载 initialValues 兜底 percent=100）
    fireEvent.change(ta, { target: { value: '{"channel":"wechat","tier":"vip"}' } });
    submitForm();
    await waitFor(() => expect(mPut).toHaveBeenCalledTimes(1));
    expect(mPut).toHaveBeenCalledWith([
      {
        scope: 'service',
        key: 'agent://game-1',
        limitQps: 500,
        percent: 100,
        match: { gameId: 'demo', channel: 'wechat', tier: 'vip' },
      },
    ]);
    expect(await screen.findByText('已保存')).toBeInTheDocument();
  });
});

describe('限速管理 预览与导出', () => {
  it('手动预览：function 域 info 不触达；service 域 → 载荷 + 命中列表降序 + 仅超限过滤 + CSV', async () => {
    mPreview.mockResolvedValue({
      matched: 3,
      agents: [
        {
          agentId: 'a1',
          gameId: 'demo',
          env: 'prod',
          region: 'cn',
          zone: 'z1',
          addr: '1.1.1.1:1',
          qps: 100,
          qps1m: 5,
        },
        { agentId: 'a2', gameId: 'demo', env: 'dev', qps: 10, qps1m: 50 },
        { agentId: 'a3', qps: 20, qps1m: 200 },
        // 覆盖翼：缺省 qps/qps1m 双右臂（渲染 '' 与 0 兜底）
        { agentId: 'a4' },
      ],
    } as never);
    renderPage();
    await waitLoad();

    fireEvent.click(screen.getByRole('button', { name: '新建规则' }));
    await waitFor(() =>
      expect(document.querySelector('.ant-modal-title')?.textContent).toBe('编辑限速规则'),
    );

    // 空表单点预览：validateFields 拒绝 → onPreview 早退（无 info、不触达）
    fireEvent.click(
      within(document.querySelector('.ant-modal') as HTMLElement).getByText('预览命中'),
    );
    await waitFor(() =>
      expect(document.querySelector('.ant-form-item-explain-error')).not.toBeNull(),
    );
    expect(screen.queryByText('仅支持服务级预览')).not.toBeInTheDocument();
    expect(mPreview).not.toHaveBeenCalled();

    // function 域：预览 → info 不触达
    await pickModalSelect(1, (t) => t === 'fn.kick_player');
    fireEvent.click(
      within(document.querySelector('.ant-modal') as HTMLElement).getByText('预览命中'),
    );
    expect(await screen.findByText('仅支持服务级预览')).toBeInTheDocument();
    expect(mPreview).not.toHaveBeenCalled();

    // 切 service 域 + agent + qps；防抖自动预览先落定再清零，锁定手动链
    await fillServiceRule();
    await new Promise((r) => setTimeout(r, 400));
    mPreview.mockClear();
    fireEvent.click(
      within(document.querySelector('.ant-modal') as HTMLElement).getByText('预览命中'),
    );
    await waitFor(() => expect(mPreview).toHaveBeenCalledTimes(1));
    expect(mPreview.mock.calls[0][0]).toMatchObject({
      scope: 'service',
      key: 'agent://game-1',
      limitQps: 500,
    });

    // 命中实例 + 降序列表（a3 200 → a2 50 → a1 5 → a4 0 兜底）
    expect(await screen.findByText('命中实例：3')).toBeInTheDocument();
    const modal = document.querySelector('.ant-modal') as HTMLElement;
    const order = Array.from(modal.querySelectorAll('div[style*="max-height"] > div')).map(
      (d) => d.querySelector('.ant-tag')?.textContent,
    );
    expect(order).toEqual(['a3', 'a2', 'a1', 'a4']);

    // 仅显示超限：勾选 → 只剩 a2（50>10）与 a3（200>20）
    fireEvent.click(screen.getByLabelText('仅显示超限（当前QPS>限速）'));
    await waitFor(() => {
      const list = Array.from(
        (document.querySelector('.ant-modal') as HTMLElement).querySelectorAll(
          'div[style*="max-height"] > div',
        ),
      ).map((d) => d.querySelector('.ant-tag')?.textContent);
      expect(list).toEqual(['a3', 'a2']);
    });

    // CSV：表头 + 行（缺省补 ''、qps1m toFixed）
    fireEvent.click(within(modal).getByText('导出 CSV'));
    await waitFor(() => expect(mCsv).toHaveBeenCalledTimes(1));
    const [name, rows] = mCsv.mock.calls[0];
    expect(name).toBe('rate_limit_preview.csv');
    expect(rows[0]).toEqual([
      'agentId',
      'gameId',
      'env',
      'region',
      'zone',
      'addr',
      'qpsLimit',
      'qps1m',
    ]);
    expect(rows[1]).toEqual(['a1', 'demo', 'prod', 'cn', 'z1', '1.1.1.1:1', 100, '5.00']);
    // 缺省 qps → ''、qps1m → 0.00 兜底
    expect(rows[4]).toEqual(['a4', '', '', '', '', '', '', '0.00']);
  });

  it('预览失败三翼：Error.message 透传 / 空 message「预览失败」/ 非 Error「操作失败」', async () => {
    renderPage();
    await waitLoad();
    fireEvent.click(screen.getByRole('button', { name: '新建规则' }));
    await waitFor(() =>
      expect(document.querySelector('.ant-modal-title')?.textContent).toBe('编辑限速规则'),
    );
    await fillServiceRule();
    await new Promise((r) => setTimeout(r, 400));
    mPreview.mockClear();

    mPreview.mockRejectedValueOnce(new Error('pv-x'));
    fireEvent.click(
      within(document.querySelector('.ant-modal') as HTMLElement).getByText('预览命中'),
    );
    expect(await screen.findByText('pv-x')).toBeInTheDocument();

    // Error 空 message：errMsg falsy → 预览失败 兜底
    mPreview.mockRejectedValueOnce(new Error(''));
    fireEvent.click(
      within(document.querySelector('.ant-modal') as HTMLElement).getByText('预览命中'),
    );
    expect(await screen.findByText('预览失败')).toBeInTheDocument();

    // 非 Error：操作失败 兜底（truthy，不再落到 预览失败）
    mPreview.mockRejectedValueOnce('plain' as never);
    fireEvent.click(
      within(document.querySelector('.ant-modal') as HTMLElement).getByText('预览命中'),
    );
    expect(await screen.findByText('操作失败')).toBeInTheDocument();
  });

  it('自动预览防抖：service+key+qps 齐备 200ms 后自动拉；条件不齐 → 面板不出现', async () => {
    renderPage();
    await waitLoad();
    fireEvent.click(screen.getByRole('button', { name: '新建规则' }));
    await waitFor(() =>
      expect(document.querySelector('.ant-modal-title')?.textContent).toBe('编辑限速规则'),
    );

    // 默认 function 域：即便 key/qps 齐 → 不自动预览
    await pickModalSelect(1, (t) => t === 'fn.kick_player');
    await new Promise((r) => setTimeout(r, 400));
    expect(mPreview).not.toHaveBeenCalled();

    await fillServiceRule();
    await waitFor(() => expect(mPreview).toHaveBeenCalled(), { timeout: 3000 });
    expect(await screen.findByText('命中实例：0')).toBeInTheDocument();
  });

  it('自动预览 reject 静默吞；弹窗取消 onClose 关闭 + 预览面板清空', async () => {
    renderPage();
    await waitLoad();
    fireEvent.click(screen.getByRole('button', { name: '新建规则' }));
    await waitFor(() =>
      expect(document.querySelector('.ant-modal-title')?.textContent).toBe('编辑限速规则'),
    );

    // 自动预览 reject → effect catch 静默（无报错文案、不白屏）
    mPreview.mockRejectedValueOnce(new Error('pv-auto-x'));
    await fillServiceRule();
    await waitFor(() => expect(mPreview).toHaveBeenCalledTimes(1), { timeout: 3000 });
    await new Promise((r) => setTimeout(r, 150));
    expect(screen.queryByText('pv-auto-x')).not.toBeInTheDocument();

    // 恢复后（响应无 agents 键：双 || [] 右臂）触发新值变更 → 自动预览成功
    mPreview.mockResolvedValueOnce({ matched: 2 } as never);
    const qpsInput = document.querySelector(
      '.ant-modal .ant-input-number-input',
    ) as HTMLInputElement;
    fireEvent.change(qpsInput, { target: { value: '600' } });
    await waitFor(() => expect(screen.getByText('命中实例：2')).toBeInTheDocument(), {
      timeout: 3000,
    });
    const cancel = document.querySelector(
      '.ant-modal-footer .ant-btn:not(.ant-btn-primary)',
    ) as HTMLElement;
    fireEvent.click(cancel);
    await waitFor(() => expect(document.querySelector('.ant-modal')).toBeNull());
  });
});

describe('限速管理 删除', () => {
  it('modal.confirm → deleteRateLimit(scope, key) → 已删除 + 重拉', async () => {
    renderPage();
    await waitLoad();

    fireEvent.click(within(rowOf('fn.bare')).getByText(/删 除/));
    await waitFor(() =>
      expect(document.querySelector('.ant-modal-confirm-title')?.textContent).toBe('删除限速'),
    );
    expect(document.querySelector('.ant-modal-confirm-content')?.textContent).toContain(
      '确定删除规则 function:fn.bare?',
    );
    confirmDialog();
    await waitFor(() => expect(mDelete).toHaveBeenCalledWith('function', 'fn.bare'));
    expect(await screen.findByText('已删除')).toBeInTheDocument();
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(2));
  });
});
