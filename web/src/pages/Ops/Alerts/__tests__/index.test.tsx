/**
 * 告警中心页单测（覆盖率巡检：Ops/Alerts/index.tsx 765 行 0% → 收口，
 * Ops/Alerts 簇首发；AlertRulesTab 本体另见 AlertRulesTab.test.tsx）。
 *
 * 锁定契约：
 * - 初始加载三请求（alerts/config/silences）与告警表渲染矩阵（severity
 *   Tag 三色 + 空 severity 空串、firing/silenced 状态 Tag、静默行无
 *   静默1h/1d 按钮、cfg 双 URL 条件按钮、静默列表 ID/创建者/起止拼接）；
 * - 筛选矩阵（severity/service 下拉精确匹配、关键词对 summary+labels
 *   JSON 小写包含、labelKey 命中判空 + labelValue String 精确、labelKey
 *   缺失键全滤空）；
 * - 双刷新按钮（告警卡 load+silences 重拉 / 静默卡仅 silences 重拉）；
 * - 外链三入口（打开 Grafana / 打开 AM 带 /#/alerts / 静默查看拼
 *   `去除尾斜杠 + /#/silences/<encodeURIComponent(id)>` 两形态）；
 * - 行内静默 1h/24h：modal.confirm 文案、matchers 经 toStringRecord
 *   归一（number→串、null→''）、comment 取 summary、成功「已静默」+重拉、
 *   失败三翼（Error.message / 非 Error「操作失败」/ 空 message「静默失败」）；
 * - 解除静默：确认文案 {id} 内插、deleteSilence(id)、成功「已解除」+
 *   重拉、失败两翼；
 * - 详情抽屉：行点击打开、severity/服务实例/摘要/起止/状态/标签/注释
 *   渲染、runbook typeof string 条件按钮、cfg.grafanaExploreUrl 条件按钮、
 *   静默行无三档静默按钮、三档（1h/6h/24h）成功链（load + 关闭可重开）、
 *   失败两翼（Error.message / 空 message「失败」）；
 * - load 失败三翼（Error.message / 非 Error「操作失败」/ 空 message
 *   「加载失败」）、silences/config 静默失败不白屏；
 * - Tabs 切换到规则页（AlertRulesTab 桩替身，本体独立套件）。
 *
 * mock 口径：services/api/ops 五函数 jest.mock；AlertRulesTab 桩替身；
 * @umijs/max 本地 mock（defaultMessage 即文案 + {id} values 内插）；
 * window.open spy。
 *
 * 坑实证（antd6 沿用）：Select mouseDown 落 .ant-select 根、点可见
 * option content，两次连开须留 ≥60ms 真实时间隙（rc-select 开/关都走
 * message 宏任务，立即重开与上一次关闭竞态、第二次 dropdown 打不开）；
 * modal.confirm 确认锚 .ant-modal-confirm-btns .ant-btn-primary、标题
 * 双渲染断言须 selector 收窄；表格行按钮点击会冒泡触发 onRow（抽屉随之
 * 打开，属真实行为，断言不受影响，行锚须限首卡表格）；Drawer 关闭动效
 * jsdom 不收尾，关闭翼以「可重开且重新拉取」锁定；getByText 只对内容做
 * trim 归一、查询串不 trim（' -> ' 单元格须用 '->' 查询）。
 *
 * 登记不可达（防御分支，不造假用例不删分支）：
 * - 81/88 `(rows || [])` 右翼：rows 是 useState 数组、setRows 只赋
 *   `r.alerts || []` 产出的数组，永不为 falsy；
 * - 742 `(detail.annotations || {}).runbook_url` 右翼：Runbook 按钮仅在
 *   runbook_url 为 string（即 annotations 已是对象）时渲染，onClick 时
 *   annotations 不可能缺省。
 */
import React from 'react';
import { configure, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { App } from 'antd';
import OpsAlertsPage from '../index';
import type { OpsAlert, OpsConfig, OpsSilence } from '@/services/api/ops';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

jest.mock('@/services/api/ops', () => ({
  fetchOpsAlerts: jest.fn(),
  fetchOpsConfig: jest.fn(),
  listSilences: jest.fn(),
  silenceOpsAlert: jest.fn(),
  deleteSilence: jest.fn(),
}));

jest.mock('../AlertRulesTab', () => ({
  __esModule: true,
  default: () => <div data-testid="rules-tab-stub">rules-stub</div>,
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
  fetchOpsAlerts,
  fetchOpsConfig,
  listSilences,
  silenceOpsAlert,
  deleteSilence,
} from '@/services/api/ops';

const mAlerts = fetchOpsAlerts as jest.MockedFunction<typeof fetchOpsAlerts>;
const mConfig = fetchOpsConfig as jest.MockedFunction<typeof fetchOpsConfig>;
const mSilences = listSilences as jest.MockedFunction<typeof listSilences>;
const mSilenceOps = silenceOpsAlert as jest.MockedFunction<typeof silenceOpsAlert>;
const mDeleteSilence = deleteSilence as jest.MockedFunction<typeof deleteSilence>;

// 覆盖翼：critical 红 + number/null label 值（toStringRecord 归一）、runbook 注释
const alert1: OpsAlert = {
  severity: 'critical',
  service: 'game-api',
  instance: 'i-1',
  summary: 'CPU high',
  startsAt: '2026-09-01T00:00:00Z',
  duration: '5m',
  silenced: false,
  labels: { job: 'game', severity: 'critical', num: 7, nilv: null },
  annotations: { runbook_url: 'https://rb.example/run', note: 'see runbook' },
};
// 覆盖翼：warning 金、silenced 状态、无 labels/annotations、无 runbook 按钮
const alert2: OpsAlert = {
  severity: 'warning',
  service: 'agent-gw',
  instance: 'i-2',
  summary: 'mem pressure',
  startsAt: '2026-09-01T01:00:00Z',
  duration: '1m',
  silenced: true,
  labels: { job: 'gw' },
};
// 覆盖翼：info 蓝、空 labels、无 startsAt
const alert3: OpsAlert = {
  severity: 'info',
  service: 'game-api',
  instance: 'i-3',
  summary: 'disk usage',
  duration: '2m',
  silenced: false,
  labels: {},
  annotations: {},
};
// 覆盖翼：severity 缺省 → 空串渲染
const alert4: OpsAlert = {
  service: 'worker',
  instance: 'i-4',
  summary: 'odd alert',
  silenced: false,
};

// 覆盖翼：service/instance/summary/startsAt/labels/annotations 全缺省
// （rowKey 三兜底、抽屉 服务/实例 双 '-'、行/抽屉三档 comment 空、
// toStringRecord 无 labels、关键词 summary 空、svc 筛选 service 空臂）
const alert5: OpsAlert = {
  severity: 'warning',
  duration: '9m',
  silenced: false,
};

const cfg: OpsConfig = {
  grafanaExploreUrl: 'https://grafana.example/explore',
  alertmanagerUrl: 'https://am.example',
};

const silences: OpsSilence[] = [
  {
    id: 'sil-1',
    createdBy: 'admin',
    startAt: '2026-09-01T00:00:00Z',
    endAt: '2026-09-01T01:00:00Z',
  },
  // 覆盖翼：startAt/endAt 缺省 → 时间列 ' -> ' 空臂
  { id: 'sil-2', createdBy: 'ops' },
];

beforeEach(() => {
  jest.clearAllMocks();
  mAlerts.mockResolvedValue({ alerts: [alert1, alert2, alert3, alert4, alert5] });
  mConfig.mockResolvedValue(cfg);
  mSilences.mockResolvedValue({ silences });
  mSilenceOps.mockResolvedValue(undefined);
  mDeleteSilence.mockResolvedValue(undefined);
  jest.spyOn(window, 'open').mockImplementation(() => null);
});

afterEach(() => {
  jest.restoreAllMocks();
});

function renderPage() {
  return render(
    <App>
      <OpsAlertsPage />
    </App>,
  );
}

/** 等首拉落定 */
async function waitLoad() {
  expect(await screen.findByText('CPU high')).toBeInTheDocument();
  await waitFor(() => expect(mAlerts).toHaveBeenCalledTimes(1));
  await waitFor(() => expect(mSilences).toHaveBeenCalledTimes(1));
  await waitFor(() => expect(mConfig).toHaveBeenCalledTimes(1));
}

/** 告警卡 extra 内第 idx 个 Select 选 option（antd6 mouseDown 根 + 可见 option）。
 * 两次连开之间须留真实时间隙：rc-select 的开/关都走 message 宏任务，
 * 立即重开会与上一次的关闭宏任务竞态（实测第二次 dropdown 打不开） */
async function pickCardSelect(idx: number, label: string) {
  await new Promise((r) => setTimeout(r, 60));
  const selects = document.querySelectorAll('.ant-card-extra .ant-select');
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

/** modal.confirm 确认按钮 */
function confirmOk() {
  const ok = document.querySelector('.ant-modal-confirm-btns .ant-btn-primary') as HTMLElement;
  expect(ok).not.toBeNull();
  fireEvent.click(ok);
}

/** 告警表行（锚首卡内表格——行按钮点击冒泡会顺带打开抽屉，同文本双处出现） */
function rowOf(text: string) {
  const table = document.querySelector('.ant-card .ant-table') as HTMLElement;
  return within(table).getByText(text).closest('tr') as HTMLElement;
}

/** 静默列表行（第二卡内表格） */
function silenceRow(text: string) {
  const cards = document.querySelectorAll('.ant-card');
  return within(cards[1] as HTMLElement)
    .getByText(text)
    .closest('tr') as HTMLElement;
}

function drawerEl() {
  return document.querySelector('.ant-drawer') as HTMLElement;
}

/** 抽屉内按整行 textContent 精确匹配（值与加粗标签同 div，getByText 单串不可达）。
 * antd Space 会把每个子项再包一层 .ant-space-item，同一内容命中两个元素，
 * 取首个即可 */
function drawerLine(text: string) {
  const els = within(drawerEl()).getAllByText((_, el) => el?.textContent === text);
  expect(els.length).toBeGreaterThanOrEqual(1);
  return els[0];
}

describe('告警中心 初始渲染', () => {
  it('Tabs/双卡/告警表矩阵/静默列表/cfg 条件按钮', async () => {
    renderPage();
    await waitLoad();

    expect(screen.getByText('告警列表')).toBeInTheDocument();
    expect(screen.getByText('告警规则')).toBeInTheDocument();
    expect(screen.getByText('告警中心')).toBeInTheDocument();
    expect(screen.getByText('静默列表')).toBeInTheDocument();

    // severity Tag 三色 + 空串兜底（undefined 不渲染 Tag）
    expect(screen.getByText('critical').closest('.ant-tag')).toHaveClass('ant-tag-red');
    // warning 双行（alert2 + alert5），首行断色即可
    expect(screen.getAllByText('warning')[0].closest('.ant-tag')).toHaveClass('ant-tag-gold');
    expect(screen.getByText('info').closest('.ant-tag')).toHaveClass('ant-tag-blue');

    // 状态列：未静默 firing、静默 silenced
    expect(screen.getAllByText('firing')).toHaveLength(4);
    expect(screen.getAllByText('silenced')).toHaveLength(1);

    // 行内静默按钮仅未静默行
    expect(screen.getAllByText('静默1h')).toHaveLength(4);
    expect(screen.getAllByText('静默1d')).toHaveLength(4);

    // cfg 条件按钮
    expect(screen.getByRole('button', { name: '打开 Grafana' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '打开 AM' })).toBeInTheDocument();

    // 静默列表
    expect(screen.getByText('sil-1')).toBeInTheDocument();
    expect(screen.getByText('admin')).toBeInTheDocument();
    expect(screen.getByText('2026-09-01T00:00:00Z -> 2026-09-01T01:00:00Z')).toBeInTheDocument();
    // sil-2：起止缺省 → ' -> '（匹配器只对内容做 trim 归一，查询串须用去空格形态）
    expect(screen.getByText('->')).toBeInTheDocument();
  });

  it('cfg 缺省（undefined → {}）：两个外链按钮不渲染；查看仍可点（URL 前缀空串）', async () => {
    mConfig.mockResolvedValue(undefined as never);
    renderPage();
    await waitLoad();

    expect(screen.queryByRole('button', { name: '打开 Grafana' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: '打开 AM' })).not.toBeInTheDocument();

    fireEvent.click(within(silenceRow('sil-1')).getByRole('button', { name: '查看' }));
    expect(window.open).toHaveBeenCalledWith('/#/silences/sil-1', '_blank');
  });
});

describe('告警中心 筛选', () => {
  it('severity 下拉精确过滤（critical → info）', async () => {
    renderPage();
    await waitLoad();

    await pickCardSelect(0, 'critical');
    expect(screen.getByText('CPU high')).toBeInTheDocument();
    expect(screen.queryByText('mem pressure')).not.toBeInTheDocument();
    expect(screen.queryByText('disk usage')).not.toBeInTheDocument();

    await pickCardSelect(0, 'info');
    expect(screen.getByText('disk usage')).toBeInTheDocument();
    expect(screen.queryByText('CPU high')).not.toBeInTheDocument();

    // 清空图标：onChange(undefined) → v || '' 右翼复位
    const sevSelect = document.querySelector('.ant-card-extra .ant-select') as HTMLElement;
    fireEvent.mouseEnter(sevSelect);
    fireEvent.click(sevSelect.querySelector('.ant-select-clear') as HTMLElement);
    await waitFor(() => expect(screen.getByText('CPU high')).toBeInTheDocument());
  });

  it('service 下拉（选项来自行内去重）精确过滤', async () => {
    renderPage();
    await waitLoad();

    await pickCardSelect(1, 'agent-gw');
    expect(screen.getByText('mem pressure')).toBeInTheDocument();
    expect(screen.queryByText('CPU high')).not.toBeInTheDocument();
    expect(screen.queryByText('odd alert')).not.toBeInTheDocument();

    // 清空图标：onChange(undefined) → v || '' 右翼复位
    const selects = document.querySelectorAll('.ant-card-extra .ant-select');
    fireEvent.mouseEnter(selects[1] as HTMLElement);
    fireEvent.click((selects[1] as HTMLElement).querySelector('.ant-select-clear') as HTMLElement);
    await waitFor(() => expect(screen.getByText('CPU high')).toBeInTheDocument());
  });

  it('关键词：summary 命中与 labels JSON 命中', async () => {
    renderPage();
    await waitLoad();

    fireEvent.change(screen.getByPlaceholderText('关键词'), { target: { value: 'cpu' } });
    expect(screen.getByText('CPU high')).toBeInTheDocument();
    expect(screen.getAllByText('firing')).toHaveLength(1);

    fireEvent.change(screen.getByPlaceholderText('关键词'), { target: { value: 'gw' } });
    expect(screen.getByText('mem pressure')).toBeInTheDocument();
    expect(screen.queryByText('CPU high')).not.toBeInTheDocument();
  });

  it('标签筛选：labelKey 命中 + labelValue String 精确（number 7 → "7"）', async () => {
    renderPage();
    await waitLoad();

    fireEvent.change(screen.getByPlaceholderText('标签键'), { target: { value: 'job' } });
    fireEvent.change(screen.getByPlaceholderText('标签值(可选)'), { target: { value: 'game' } });
    expect(screen.getByText('CPU high')).toBeInTheDocument();
    expect(screen.queryByText('mem pressure')).not.toBeInTheDocument();

    // number label 值经 String() 与输入串比较
    fireEvent.change(screen.getByPlaceholderText('标签键'), { target: { value: 'num' } });
    fireEvent.change(screen.getByPlaceholderText('标签值(可选)'), { target: { value: '7' } });
    expect(screen.getByText('CPU high')).toBeInTheDocument();

    // labelValue 不匹配 → 全滤空
    fireEvent.change(screen.getByPlaceholderText('标签值(可选)'), { target: { value: '9' } });
    expect(screen.queryByText('CPU high')).not.toBeInTheDocument();

    // labelKey 缺失（v == null 翼：无该键或值 null）→ 全滤空
    fireEvent.change(screen.getByPlaceholderText('标签键'), { target: { value: 'nilv' } });
    fireEvent.change(screen.getByPlaceholderText('标签值(可选)'), { target: { value: '' } });
    expect(screen.queryByText('CPU high')).not.toBeInTheDocument();
    expect(screen.queryByText('mem pressure')).not.toBeInTheDocument();
  });
});

describe('告警中心 工具栏动作', () => {
  it('告警卡刷新：alerts + silences 双拉', async () => {
    renderPage();
    await waitLoad();

    const extras = document.querySelectorAll('.ant-card-extra');
    fireEvent.click(within(extras[0] as HTMLElement).getByRole('button', { name: '刷新' }));
    await waitFor(() => expect(mAlerts).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(mSilences).toHaveBeenCalledTimes(2));
  });

  it('静默卡刷新：仅 silences 重拉（{} 响应 → silences || [] 兜底）', async () => {
    mSilences.mockResolvedValue({} as never);
    renderPage();
    await waitLoad();

    const extras = document.querySelectorAll('.ant-card-extra');
    fireEvent.click(within(extras[1] as HTMLElement).getByRole('button', { name: '刷新' }));
    await waitFor(() => expect(mSilences).toHaveBeenCalledTimes(2));
    expect(mAlerts).toHaveBeenCalledTimes(1);

    // 告警卡刷新在 {} 响应下同样走 s.silences || [] 右翼
    fireEvent.click(within(extras[0] as HTMLElement).getByRole('button', { name: '刷新' }));
    await waitFor(() => expect(mSilences).toHaveBeenCalledTimes(3));
  });

  it('打开 Grafana / 打开 AM：window.open 载荷', async () => {
    renderPage();
    await waitLoad();

    fireEvent.click(screen.getByRole('button', { name: '打开 Grafana' }));
    expect(window.open).toHaveBeenCalledWith('https://grafana.example/explore', '_blank');

    fireEvent.click(screen.getByRole('button', { name: '打开 AM' }));
    expect(window.open).toHaveBeenCalledWith('https://am.example/#/alerts', '_blank');
  });
});

describe('告警中心 静默列表动作', () => {
  it('查看：alertmanagerUrl 拼接（无/有尾斜杠两形态）', async () => {
    const first = renderPage();
    await waitLoad();

    fireEvent.click(within(silenceRow('sil-1')).getByRole('button', { name: '查看' }));
    expect(window.open).toHaveBeenCalledWith('https://am.example/#/silences/sil-1', '_blank');
    first.unmount();

    // 尾斜杠形态：replace(/\/$/,'') 后拼接
    mConfig.mockResolvedValue({ alertmanagerUrl: 'https://am.example/' });
    renderPage();
    expect(await screen.findByText('sil-1')).toBeInTheDocument();
    fireEvent.click(within(silenceRow('sil-1')).getByRole('button', { name: '查看' }));
    expect(window.open).toHaveBeenCalledWith('https://am.example/#/silences/sil-1', '_blank');
  });

  it('解除静默主链：确认文案 {id} 内插 + deleteSilence + 已解除 + 重拉', async () => {
    renderPage();
    await waitLoad();

    mSilences.mockResolvedValue({} as never);
    fireEvent.click(within(silenceRow('sil-1')).getByRole('button', { name: '解除' }));
    expect(await screen.findByText('确定解除静默 sil-1?')).toBeInTheDocument();
    confirmOk();

    await waitFor(() => expect(mDeleteSilence).toHaveBeenCalledWith('sil-1'));
    expect(await screen.findByText('已解除')).toBeInTheDocument();
    await waitFor(() => expect(mSilences).toHaveBeenCalledTimes(2));
  });

  it('解除静默失败两翼：Error.message / 非 Error 兜底', async () => {
    renderPage();
    await waitLoad();

    mDeleteSilence.mockRejectedValueOnce(new Error('del-boom'));
    fireEvent.click(within(silenceRow('sil-1')).getByRole('button', { name: '解除' }));
    expect(await screen.findByText('确定解除静默 sil-1?')).toBeInTheDocument();
    confirmOk();
    expect(await screen.findByText('del-boom')).toBeInTheDocument();

    mDeleteSilence.mockRejectedValueOnce('plain' as never);
    fireEvent.click(within(silenceRow('sil-1')).getByRole('button', { name: '解除' }));
    expect(await screen.findByText('确定解除静默 sil-1?')).toBeInTheDocument();
    confirmOk();
    expect(await screen.findByText('操作失败')).toBeInTheDocument();

    // 空 message：errMsg '' → || 右翼兜底「操作失败」
    mDeleteSilence.mockRejectedValueOnce(new Error(''));
    fireEvent.click(within(silenceRow('sil-1')).getByRole('button', { name: '解除' }));
    expect(await screen.findByText('确定解除静默 sil-1?')).toBeInTheDocument();
    confirmOk();
    expect(await screen.findByText('操作失败')).toBeInTheDocument();
    expect(mSilences).toHaveBeenCalledTimes(1);
  });
});

describe('告警中心 行内静默', () => {
  it('静默1d 主链：duration 24h + 已静默 + 重拉', async () => {
    renderPage();
    await waitLoad();

    fireEvent.click(within(rowOf('CPU high')).getByRole('button', { name: '静默1d' }));
    expect(await screen.findByText('静默 24 小时？')).toBeInTheDocument();
    confirmOk();

    await waitFor(() =>
      expect(mSilenceOps).toHaveBeenCalledWith(
        expect.objectContaining({ duration: '24h', comment: 'CPU high' }),
      ),
    );
    expect(await screen.findByText('已静默')).toBeInTheDocument();
    await waitFor(() => expect(mAlerts).toHaveBeenCalledTimes(2));
  });

  it('静默1h 失败两翼：Error.message / 空 message「静默失败」（1h 拷贝的 catch）', async () => {
    renderPage();
    await waitLoad();

    mSilenceOps.mockRejectedValueOnce(new Error('one-h-x'));
    fireEvent.click(within(rowOf('CPU high')).getByRole('button', { name: '静默1h' }));
    expect(await screen.findByText('静默 1 小时？')).toBeInTheDocument();
    confirmOk();
    expect(await screen.findByText('one-h-x')).toBeInTheDocument();

    mSilenceOps.mockRejectedValueOnce(new Error(''));
    fireEvent.click(within(rowOf('CPU high')).getByRole('button', { name: '静默1h' }));
    expect(await screen.findByText('静默 1 小时？')).toBeInTheDocument();
    confirmOk();
    expect(await screen.findByText('静默失败')).toBeInTheDocument();

    // 非 Error → 「操作失败」（1h 拷贝的三元右臂）
    mSilenceOps.mockRejectedValueOnce('plain' as never);
    fireEvent.click(within(rowOf('CPU high')).getByRole('button', { name: '静默1h' }));
    expect(await screen.findByText('静默 1 小时？')).toBeInTheDocument();
    confirmOk();
    expect(await screen.findByText('操作失败')).toBeInTheDocument();
    expect(mAlerts).toHaveBeenCalledTimes(1);
  });

  it('静默1h 主链：matchers 归一（number/null）+ duration/comment + 已静默 + 重拉', async () => {
    renderPage();
    await waitLoad();

    fireEvent.click(within(rowOf('CPU high')).getByRole('button', { name: '静默1h' }));
    expect(await screen.findByText('静默 1 小时？')).toBeInTheDocument();
    confirmOk();

    await waitFor(() =>
      expect(mSilenceOps).toHaveBeenCalledWith({
        matchers: { job: 'game', severity: 'critical', num: '7', nilv: '' },
        duration: '1h',
        comment: 'CPU high',
      }),
    );
    expect(await screen.findByText('已静默')).toBeInTheDocument();
    await waitFor(() => expect(mAlerts).toHaveBeenCalledTimes(2));
  });

  it('静默1d 失败三翼：Error.message / 非 Error「操作失败」/ 空 message「静默失败」', async () => {
    renderPage();
    await waitLoad();

    mSilenceOps.mockRejectedValueOnce(new Error('sil-boom'));
    fireEvent.click(within(rowOf('CPU high')).getByRole('button', { name: '静默1d' }));
    expect(await screen.findByText('静默 24 小时？')).toBeInTheDocument();
    confirmOk();
    expect(await screen.findByText('sil-boom')).toBeInTheDocument();
    expect(mAlerts).toHaveBeenCalledTimes(1);

    mSilenceOps.mockRejectedValueOnce('plain' as never);
    fireEvent.click(within(rowOf('CPU high')).getByRole('button', { name: '静默1d' }));
    expect(await screen.findByText('静默 24 小时？')).toBeInTheDocument();
    confirmOk();
    expect(await screen.findByText('操作失败')).toBeInTheDocument();

    mSilenceOps.mockRejectedValueOnce(new Error(''));
    fireEvent.click(within(rowOf('CPU high')).getByRole('button', { name: '静默1d' }));
    expect(await screen.findByText('静默 24 小时？')).toBeInTheDocument();
    confirmOk();
    expect(await screen.findByText('静默失败')).toBeInTheDocument();
    expect(mAlerts).toHaveBeenCalledTimes(1);
  });
});

describe('告警中心 详情抽屉', () => {
  it('行点击打开：全字段渲染 + runbook/grafana 条件按钮 + 外链', async () => {
    renderPage();
    await waitLoad();

    fireEvent.click(rowOf('CPU high'));
    expect(await screen.findByText('告警详情')).toBeInTheDocument();

    expect(drawerLine('严重度: critical')).toBeInTheDocument();
    expect(drawerLine('服务/实例: game-api / i-1')).toBeInTheDocument();
    expect(drawerLine('摘要: CPU high')).toBeInTheDocument();
    expect(drawerLine('开始时间: 2026-09-01T00:00:00Z 时长: 5m')).toBeInTheDocument();
    expect(drawerLine('状态: firing')).toBeInTheDocument();
    expect(within(drawerEl()).getByText('job:game')).toBeInTheDocument();
    expect(within(drawerEl()).getByText('severity:critical')).toBeInTheDocument();
    expect(within(drawerEl()).getByText('num:7')).toBeInTheDocument();
    expect(drawerLine('note: see runbook')).toBeInTheDocument();

    fireEvent.click(within(drawerEl()).getByRole('button', { name: '打开 Runbook' }));
    expect(window.open).toHaveBeenCalledWith('https://rb.example/run', '_blank');
    fireEvent.click(within(drawerEl()).getByRole('button', { name: '打开 Grafana' }));
    expect(window.open).toHaveBeenCalledWith('https://grafana.example/explore', '_blank');
  });

  it('三档静默（1h/6h/24h）：成功链 + 抽屉关闭可重开', async () => {
    renderPage();
    await waitLoad();

    // 1h
    fireEvent.click(rowOf('CPU high'));
    expect(await screen.findByText('告警详情')).toBeInTheDocument();
    fireEvent.click(within(drawerEl()).getByRole('button', { name: '静默1h' }));
    expect(
      await screen.findByText('静默 1 小时', { selector: '.ant-modal-confirm-title' }),
    ).toBeInTheDocument();
    confirmOk();
    await waitFor(() =>
      expect(mSilenceOps).toHaveBeenLastCalledWith(expect.objectContaining({ duration: '1h' })),
    );
    expect(await screen.findByText('已静默')).toBeInTheDocument();
    await waitFor(() => expect(mAlerts).toHaveBeenCalledTimes(2));
    // setDetail(null) → 内容即刻卸载（detail && 条件渲染）
    await waitFor(() =>
      expect(within(drawerEl()).queryByRole('button', { name: '静默6h' })).not.toBeInTheDocument(),
    );

    // 6h（重开）
    fireEvent.click(rowOf('CPU high'));
    expect(await screen.findByText('告警详情')).toBeInTheDocument();
    fireEvent.click(within(drawerEl()).getByRole('button', { name: '静默6h' }));
    expect(
      await screen.findByText('静默 6 小时', { selector: '.ant-modal-confirm-title' }),
    ).toBeInTheDocument();
    confirmOk();
    await waitFor(() =>
      expect(mSilenceOps).toHaveBeenLastCalledWith(
        expect.objectContaining({ duration: '6h', comment: 'CPU high' }),
      ),
    );

    // 24h（重开）
    fireEvent.click(rowOf('CPU high'));
    expect(await screen.findByText('告警详情')).toBeInTheDocument();
    fireEvent.click(within(drawerEl()).getByRole('button', { name: '静默1d' }));
    expect(
      await screen.findByText('静默 24 小时', { selector: '.ant-modal-confirm-title' }),
    ).toBeInTheDocument();
    confirmOk();
    await waitFor(() =>
      expect(mSilenceOps).toHaveBeenLastCalledWith(expect.objectContaining({ duration: '24h' })),
    );
    expect(mAlerts).toHaveBeenCalledTimes(4);
  });

  it('抽屉静默失败两翼：Error.message / 空 message「失败」，抽屉保持可操作', async () => {
    renderPage();
    await waitLoad();

    mSilenceOps.mockRejectedValueOnce(new Error('drawer-sil-x'));
    fireEvent.click(rowOf('CPU high'));
    expect(await screen.findByText('告警详情')).toBeInTheDocument();
    fireEvent.click(within(drawerEl()).getByRole('button', { name: '静默1h' }));
    expect(
      await screen.findByText('静默 1 小时', { selector: '.ant-modal-confirm-title' }),
    ).toBeInTheDocument();
    confirmOk();
    expect(await screen.findByText('drawer-sil-x')).toBeInTheDocument();
    expect(within(drawerEl()).getByRole('button', { name: '静默6h' })).toBeInTheDocument();

    mSilenceOps.mockRejectedValueOnce(new Error(''));
    fireEvent.click(within(drawerEl()).getByRole('button', { name: '静默6h' }));
    expect(
      await screen.findByText('静默 6 小时', { selector: '.ant-modal-confirm-title' }),
    ).toBeInTheDocument();
    confirmOk();
    expect(await screen.findByText('失败')).toBeInTheDocument();
  });

  it('三档失败臂补全：1h 空 message / 6h Error.message / 24h 两翼（各档独立 catch 拷贝）', async () => {
    renderPage();
    await waitLoad();

    fireEvent.click(rowOf('CPU high'));
    expect(await screen.findByText('告警详情')).toBeInTheDocument();

    // 1h 空 message → 「失败」
    mSilenceOps.mockRejectedValueOnce(new Error(''));
    fireEvent.click(within(drawerEl()).getByRole('button', { name: '静默1h' }));
    expect(
      await screen.findByText('静默 1 小时', { selector: '.ant-modal-confirm-title' }),
    ).toBeInTheDocument();
    confirmOk();
    expect(await screen.findByText('失败')).toBeInTheDocument();

    // 1h 非 Error → 「操作失败」
    mSilenceOps.mockRejectedValueOnce('plain' as never);
    fireEvent.click(within(drawerEl()).getByRole('button', { name: '静默1h' }));
    expect(
      await screen.findByText('静默 1 小时', { selector: '.ant-modal-confirm-title' }),
    ).toBeInTheDocument();
    confirmOk();
    expect(await screen.findByText('操作失败')).toBeInTheDocument();

    // 6h Error.message
    mSilenceOps.mockRejectedValueOnce(new Error('six-x'));
    fireEvent.click(within(drawerEl()).getByRole('button', { name: '静默6h' }));
    expect(
      await screen.findByText('静默 6 小时', { selector: '.ant-modal-confirm-title' }),
    ).toBeInTheDocument();
    confirmOk();
    expect(await screen.findByText('six-x')).toBeInTheDocument();

    // 24h Error.message
    mSilenceOps.mockRejectedValueOnce(new Error('t24-x'));
    fireEvent.click(within(drawerEl()).getByRole('button', { name: '静默1d' }));
    expect(
      await screen.findByText('静默 24 小时', { selector: '.ant-modal-confirm-title' }),
    ).toBeInTheDocument();
    confirmOk();
    expect(await screen.findByText('t24-x')).toBeInTheDocument();

    // 6h 非 Error → 「操作失败」
    mSilenceOps.mockRejectedValueOnce('plain' as never);
    fireEvent.click(within(drawerEl()).getByRole('button', { name: '静默6h' }));
    expect(
      await screen.findByText('静默 6 小时', { selector: '.ant-modal-confirm-title' }),
    ).toBeInTheDocument();
    confirmOk();
    expect(await screen.findByText('操作失败')).toBeInTheDocument();

    // 24h 空 message → 「失败」
    mSilenceOps.mockRejectedValueOnce(new Error(''));
    fireEvent.click(within(drawerEl()).getByRole('button', { name: '静默1d' }));
    expect(
      await screen.findByText('静默 24 小时', { selector: '.ant-modal-confirm-title' }),
    ).toBeInTheDocument();
    confirmOk();
    expect(await screen.findByText('失败')).toBeInTheDocument();
    // 24h 非 Error → 「操作失败」
    mSilenceOps.mockRejectedValueOnce('plain' as never);
    fireEvent.click(within(drawerEl()).getByRole('button', { name: '静默1d' }));
    expect(
      await screen.findByText('静默 24 小时', { selector: '.ant-modal-confirm-title' }),
    ).toBeInTheDocument();
    confirmOk();
    expect(await screen.findByText('操作失败')).toBeInTheDocument();

    // 全程失败：不重拉、抽屉保持
    expect(mAlerts).toHaveBeenCalledTimes(1);
    expect(within(drawerEl()).getByRole('button', { name: '静默6h' })).toBeInTheDocument();
  });

  it('缺省字段行：matchers 空 + comment 空 + 抽屉 instance 兜底 + onClose 可重开', async () => {
    renderPage();
    await waitLoad();

    // 行内 1h：labels undefined → matchers {}；summary 缺省 → comment ''
    fireEvent.click(within(rowOf('9m')).getByRole('button', { name: '静默1h' }));
    expect(await screen.findByText('静默 1 小时？')).toBeInTheDocument();
    confirmOk();
    await waitFor(() =>
      expect(mSilenceOps).toHaveBeenCalledWith({ matchers: {}, duration: '1h', comment: '' }),
    );
    expect(await screen.findByText('已静默')).toBeInTheDocument();

    // 抽屉（行点击冒泡已开）：服务/实例双兜底 '-'
    expect(drawerLine('服务/实例: - / -')).toBeInTheDocument();
    expect(drawerLine('摘要: -')).toBeInTheDocument();
    expect(drawerLine('开始时间: - 时长: 9m')).toBeInTheDocument();

    // 抽屉 1h：comment '' 臂（1h 拷贝）
    fireEvent.click(within(drawerEl()).getByRole('button', { name: '静默1h' }));
    expect(
      await screen.findByText('静默 1 小时', { selector: '.ant-modal-confirm-title' }),
    ).toBeInTheDocument();
    confirmOk();
    await waitFor(() =>
      expect(mSilenceOps).toHaveBeenLastCalledWith({ matchers: {}, duration: '1h', comment: '' }),
    );

    // 重开 → 抽屉 6h：comment '' 臂（6h 拷贝）
    fireEvent.click(rowOf('9m'));
    expect(await screen.findByText('告警详情')).toBeInTheDocument();
    fireEvent.click(within(drawerEl()).getByRole('button', { name: '静默6h' }));
    expect(
      await screen.findByText('静默 6 小时', { selector: '.ant-modal-confirm-title' }),
    ).toBeInTheDocument();
    confirmOk();
    await waitFor(() =>
      expect(mSilenceOps).toHaveBeenLastCalledWith({ matchers: {}, duration: '6h', comment: '' }),
    );

    // 重开 → 抽屉 24h：comment '' 臂（24h 拷贝）
    fireEvent.click(rowOf('9m'));
    expect(await screen.findByText('告警详情')).toBeInTheDocument();
    fireEvent.click(within(drawerEl()).getByRole('button', { name: '静默1d' }));
    expect(
      await screen.findByText('静默 24 小时', { selector: '.ant-modal-confirm-title' }),
    ).toBeInTheDocument();
    confirmOk();
    await waitFor(() =>
      expect(mSilenceOps).toHaveBeenLastCalledWith({ matchers: {}, duration: '24h', comment: '' }),
    );

    // 关闭（onClose）后可重开 + 行内 1d：comment '' 臂（行 24h 拷贝）
    fireEvent.click(document.querySelector('.ant-drawer-close') as HTMLElement);
    fireEvent.click(within(rowOf('9m')).getByRole('button', { name: '静默1d' }));
    expect(await screen.findByText('静默 24 小时？')).toBeInTheDocument();
    confirmOk();
    await waitFor(() =>
      expect(mSilenceOps).toHaveBeenLastCalledWith({ matchers: {}, duration: '24h', comment: '' }),
    );
  });

  it('空值形态：severity 缺省/无起止/无标签注释/无 runbook；静默行无三档按钮', async () => {
    renderPage();
    await waitLoad();

    fireEvent.click(rowOf('odd alert'));
    expect(await screen.findByText('告警详情')).toBeInTheDocument();
    expect(drawerLine('服务/实例: worker / i-4')).toBeInTheDocument();
    expect(drawerLine('摘要: odd alert')).toBeInTheDocument();
    expect(drawerLine('开始时间: - 时长: -')).toBeInTheDocument();
    expect(
      within(drawerEl()).queryByRole('button', { name: '打开 Runbook' }),
    ).not.toBeInTheDocument();
    // severity 缺省：Tag 渲染空文案（critical/warning/info 都不在抽屉）
    expect(within(drawerEl()).queryByText('critical')).not.toBeInTheDocument();

    // 静默行：无三档按钮 + 状态 silenced
    fireEvent.click(rowOf('mem pressure'));
    await waitFor(() =>
      expect(within(drawerEl()).queryByRole('button', { name: '静默1h' })).not.toBeInTheDocument(),
    );
    expect(within(drawerEl()).queryByRole('button', { name: '静默6h' })).not.toBeInTheDocument();
    expect(within(drawerEl()).queryByRole('button', { name: '静默1d' })).not.toBeInTheDocument();
    expect(drawerLine('状态: silenced')).toBeInTheDocument();
  });

  it('cfg 无 grafana：抽屉内 Grafana 按钮不渲染', async () => {
    mConfig.mockResolvedValue({ alertmanagerUrl: 'https://am.example' });
    renderPage();
    await waitLoad();

    fireEvent.click(rowOf('CPU high'));
    expect(await screen.findByText('告警详情')).toBeInTheDocument();
    expect(
      within(drawerEl()).queryByRole('button', { name: '打开 Grafana' }),
    ).not.toBeInTheDocument();
  });
});

describe('告警中心 失败翼', () => {
  it('alerts 响应缺省：空表不白屏', async () => {
    mAlerts.mockResolvedValue({} as never);
    renderPage();
    await waitFor(() => expect(mAlerts).toHaveBeenCalledTimes(1));
    const cards = document.querySelectorAll('.ant-card');
    await waitFor(() =>
      expect(
        (cards[0] as HTMLElement).querySelector('.ant-table .ant-empty-description'),
      ).not.toBeNull(),
    );
  });

  it('load 失败三翼：Error.message / 非 Error「操作失败」/ 空 message「加载失败」', async () => {
    mAlerts.mockRejectedValueOnce(new Error('ops-down'));
    renderPage();
    expect(await screen.findByText('ops-down')).toBeInTheDocument();

    mAlerts.mockRejectedValueOnce('plain' as never);
    renderPage();
    expect(await screen.findByText('操作失败')).toBeInTheDocument();

    mAlerts.mockRejectedValueOnce(new Error(''));
    renderPage();
    expect(await screen.findByText('加载失败')).toBeInTheDocument();
  });

  it('silences/config 静默失败：不白屏、静默表空态', async () => {
    mSilences.mockRejectedValue(new Error('sil-down'));
    mConfig.mockRejectedValue(new Error('cfg-down'));
    renderPage();
    expect(await screen.findByText('CPU high')).toBeInTheDocument();

    const cards = document.querySelectorAll('.ant-card');
    await waitFor(() =>
      expect((cards[1] as HTMLElement).querySelector('.ant-empty-description')).not.toBeNull(),
    );

    // 双刷新按钮在 silences 持续 reject 下走静默 catch（不白屏不报错）
    const extras = document.querySelectorAll('.ant-card-extra');
    fireEvent.click(within(extras[0] as HTMLElement).getByRole('button', { name: '刷新' }));
    fireEvent.click(within(extras[1] as HTMLElement).getByRole('button', { name: '刷新' }));
    await waitFor(() => expect(mAlerts).toHaveBeenCalledTimes(2));
    expect(screen.getByText('CPU high')).toBeInTheDocument();
  });
});

describe('告警中心 Tabs', () => {
  it('切换告警规则页：AlertRulesTab 挂载', async () => {
    renderPage();
    await waitLoad();

    fireEvent.click(screen.getByText('告警规则'));
    expect(await screen.findByTestId('rules-tab-stub')).toBeInTheDocument();
  });
});
