/**
 * 行为分析页单测（覆盖率补缺轮：Analytics/Behavior 入口 index.tsx（383 行）+
 * PathControls.tsx（245 行）+ AdoptionControls.tsx（251 行）三文件 879 行
 * 0% → 收口；姊妹套件 FunnelPresetBar.test.tsx 已覆盖 FunnelPresetBar）。
 *
 * 锁定契约：
 * - 事件探索：挂载首拉 {event:'',propKey:'',propVal:''}（range null 不带
 *   start/end）；三输入 + RangePicker 后「查询」带全参数（ISO 载荷）；
 *   导出 events.csv（header + time/event/user_id 三列，user_id 缺失回退
 *   userId）；响应 {} → 空表右翼。
 * - 漏斗：默认态「计算」{steps:'',sequential:0}（无 sameSession/gapSec 键）；
 *   tags 双步 + 顺序 Switch + 同会话 Checkbox + 步间秒数 → 全参数载荷
 *   {steps:'a,b',sequential:1,sameSession:1,gapSec:300}；表渲染 `${v}%`；
 *   导出 funnel.csv（String 强转）。
 * - 复制链接：URLSearchParams 按插入序拼 steps/sequential/same_session/
 *   gap_sec/start/end → clipboard.writeText(origin+pathname+search)；
 *   空态全条件假翼 → 仅 '?'。
 * - 深链：?steps=（trim/filter Boolean 归一）&sequential=1&same_session=1&
 *   gap_sec>0&start/end（dayjs isValid 门）→ 预填四态 + setTimeout 自动
 *   loadFunnel（opts 显式透传）。**现状锁定**：自动计算经挂载期闭包捕获
 *   range=null，start/end 不进自动漏斗载荷——range 预填只对后续手动
 *   计算/事件查询生效（后续两段断言）；
 *   负翼：无 steps 不自动算、sequential 非 '1' 不置、gap_sec 非法不置、
 *   非法日期不置 range。
 * - 路径分析：默认 {per:'session',steps:5,limit:50}；全参数（per 切按用户、
 *   include/exclude tags、同会话、步间秒数、pathRe/pathNotRe trim 入参）；
 *   InputNumber 清空回默认（Number(v||5)/Number(v||0)）；空响应右翼；
 *   匹配漏斗指示器（currentSteps join '>' 与 pathRe 正则测试 → 是/否 Tag，
 *   非法正则 catch → 无指示器，无 steps/无 pathRe → null）；
 *   操作列：填充漏斗（onUsePath split('>') → setSteps + loadFunnel({steps}) +
 *   scrollIntoView('#funnel-anchor'））、复制步骤（clipboard）、导出
 *   paths.csv（path/groups 空值 String 兜底翼）。
 * - 功能采用率：默认 {features:'',per:'user'} + 基数行值内插
 *   （基数（用户）：0）；tags + 切按会话 → features join + per:'session' +
 *   基数行（基数（会话）：{baseline}）；表 `${v}%`；导出 adoption.csv；
 *   分层：by 切按平台 → breakdown 载荷 {features,per,by} + dim 表渲染 +
 *   导出 adoption_breakdown.csv；{} 响应右翼（空表 + baseline 归零）。
 *
 * mock 口径：services/api/analytics 五函数 jest.mock（返回归一化后形态——
 * 页面消费 service 层输出而非原始响应）；@umijs/max 本地 defaultMessage
 * mock（{per}/{baseline} 内插）；pro-components 仅换 PageContainer 桩；
 * utils/export exportToXLSX 桩（载荷断言）；jsdom 补 clipboard.writeText
 * 与 scrollIntoView。
 *
 * 现状锁定（页面 quirk，如实断言不代改）：
 * - 事件表「用户」列 dataIndex='user_id'，而归一化层产出的键是 userId——
 *   生产环境该列恒空；导出侧 (r.user_id || r.userId || '') 双翼都有回退。
 *   夹具两行分别带 user_id / 仅 userId：列只显 user_id 行，导出双回退可见。
 *
 * 边界（诚实清单，不造假用例不删防御分支）：
 * 1. load/loadFunnel/路径 load/采用率 load+loadDim 均 try/finally 无
 *    catch——接口 reject 产生 unhandled rejection（同族页面既有口径），
 *    不造假 reject 场景；nullish 右翼以 {} resolve 形态覆盖。
 * 2. 转化率/采用率列 render 的 `v != null ? ${v}% : '-'` 右翼——service
 *    归一层恒产 number（conversionRate||0 / adoptionRate*100），构造 null
 *    违反返回类型即造假，登记。
 * 3. 导出侧 `(rows || [])`/`(funnel || [])`/`(rowsDim || [])` 右翼
 *    （useState 恒数组）；PathControls `(currentSteps || [])` 右翼
 *    （页面恒传 steps 数组）与 `(rows || [])` 右翼。
 * 4. 复制链接/复制步骤的 try/catch catch 翼——writeText 为注入 mock 不抛，
 *    URLSearchParams/toString 无失败路径，登记。复制链接的
 *    `range && range[0]`/`range && range[1]` 半开翼（range 非空但单侧
 *    null）：RangePicker onChange 只产「完整对或 null」，单侧 null 是
 *    类型防御形态，经 UI 不可达，登记。
 * 5. 深链外层 catch（URLSearchParams 构造/取值不抛）与内层 catch
 *    （dayjs 解析不抛、isValid 门已兜）防御性不可达，登记。
 * 6. `onUsePath && onUsePath(...)` 右翼——页面恒传回调（组件独立使用的
 *    防御守卫），登记。
 * 7. rowKey/导出单元格的 `|| ''` 右翼族（无 id 行 `${event||''}-${time||''}`
 *    模板、events 导出 `r.time||''`/`r.event||''`）：EventRow 字段类型恒
 *    string 且夹具三行均非空串，空串右翼为类型防御；rowKey 兜底串整体
 *    经无 id 行的行渲染存在性锁定（path/dim/breakdown 的 falsy 翼已由
 *    第二行 dim ''/groups 0/baseline 0 真实覆盖）。
 */
import React from 'react';
import { configure, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { App } from 'antd';
import dayjs from 'dayjs';
import AnalyticsBehaviorPage from '../index';
import {
  fetchAnalyticsAdoption,
  fetchAnalyticsAdoptionBreakdown,
  fetchAnalyticsEvents,
  fetchAnalyticsFunnel,
  fetchAnalyticsPaths,
} from '@/services/api/analytics';
import { exportToXLSX } from '@/utils/export';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

jest.mock('@/services/api/analytics', () => ({
  fetchAnalyticsEvents: jest.fn(),
  fetchAnalyticsFunnel: jest.fn(),
  fetchAnalyticsPaths: jest.fn(),
  fetchAnalyticsAdoption: jest.fn(),
  fetchAnalyticsAdoptionBreakdown: jest.fn(),
}));

jest.mock('@/utils/export', () => ({ exportToXLSX: jest.fn() }));

// 工厂自包含：defaultMessage 透传 + {key} 内插（采用率基数行依赖 values）
jest.mock('@umijs/max', () => {
  const fmt = (opts: { defaultMessage?: string }, values?: Record<string, unknown>): string => {
    let msg = opts.defaultMessage ?? '';
    if (values) {
      for (const [k, v] of Object.entries(values)) msg = msg.split(`{${k}}`).join(String(v));
    }
    return msg;
  };
  const intl = { formatMessage: fmt, locale: 'zh-CN' };
  return {
    FormattedMessage: ({ defaultMessage }: { defaultMessage?: string }) => (
      <>{defaultMessage ?? ''}</>
    ),
    useIntl: () => intl,
    getIntl: () => intl,
  };
});

jest.mock('@ant-design/pro-components', () => ({
  PageContainer: ({ children }: { children?: React.ReactNode }) => <div>{children}</div>,
}));

const mEvents = fetchAnalyticsEvents as jest.MockedFunction<typeof fetchAnalyticsEvents>;
const mFunnel = fetchAnalyticsFunnel as jest.MockedFunction<typeof fetchAnalyticsFunnel>;
const mPaths = fetchAnalyticsPaths as jest.MockedFunction<typeof fetchAnalyticsPaths>;
const mAdoption = fetchAnalyticsAdoption as jest.MockedFunction<typeof fetchAnalyticsAdoption>;
const mBreakdown = fetchAnalyticsAdoptionBreakdown as jest.MockedFunction<
  typeof fetchAnalyticsAdoptionBreakdown
>;
const mExport = exportToXLSX as jest.MockedFunction<typeof exportToXLSX>;

// jsdom 缺 clipboard / scrollIntoView（填充漏斗滚动锚点需要）
const clipboardWrite = jest.fn();
Object.defineProperty(navigator, 'clipboard', {
  value: { writeText: clipboardWrite },
  configurable: true,
});
if (!Element.prototype.scrollIntoView) {
  Element.prototype.scrollIntoView = jest.fn();
}
const scrollIntoViewMock = Element.prototype.scrollIntoView as jest.Mock;

// 归一化输出形态（service 层产物）：变量形态绕开 excess property 检查，
// user_id 行用于覆盖导出双回退左翼
const EVENT_ROWS: Array<{
  id?: string;
  event: string;
  userId: string;
  time: string;
  data?: null;
  user_id?: string;
}> = [
  { id: 'e1', event: 'login', userId: 'u-norm', time: '2026-09-01T10:00:00Z', data: null },
  { id: 'e2', event: 'pay', userId: 'u-hidden', time: '2026-09-02T11:00:00Z', user_id: 'u-legacy' },
  { event: 'x', userId: '', time: '2026-09-03T12:00:00Z', data: null },
];

const FUNNEL = {
  steps: [
    { step: 'login', users: 100, rate: 100, conversionRate: 100, dropOffRate: 0 },
    { step: 'pay', users: 30, rate: 30, conversionRate: 30, dropOffRate: 70 },
  ],
};

const PATH_ROWS = [
  { path: 'login>pay', groups: 10 },
  { path: 'login>shop>pay', groups: 5 },
  { path: '', groups: 0 },
];

const ADOPTION = {
  features: [{ feature: 'first_pay', groups: 50, rate: 25, frequency: 3 }],
  baseline: 200,
};

const BREAKDOWN = {
  by: 'channel',
  // 第二行 dim ''/baseline 0/groups 0：rowKey 三段 `||` 右翼 + rate 0 显式值
  rows: [
    { dim: 'ios', groups: 10, baseline: 200, rate: 5 },
    { dim: '', groups: 0, baseline: 0, rate: 0 },
  ],
};

function sleep(ms: number) {
  return new Promise((r) => setTimeout(r, ms));
}

/** 按卡片头文本找 Card（页面五张卡，同文按钮/列名靠它收窄） */
function card(title: string): HTMLElement {
  const el = Array.from(document.querySelectorAll('.ant-card')).find((c) =>
    c.querySelector('.ant-card-head')?.textContent?.includes(title),
  );
  expect(el).toBeTruthy();
  return el as HTMLElement;
}

const EVENTS_CARD = () => card('事件探索');
const FUNNEL_CARD = () => card('漏斗');
const PATH_CARD = () => card('路径分析');
const ADOPTION_CARD = () => card('功能采用率');

/** 打开单选下拉并点可见 option（rc-select 关闭动画，重开须留时间隙） */
async function pickOption(selectRoot: HTMLElement, label: string) {
  await sleep(60);
  fireEvent.mouseDown(selectRoot);
  const item = await waitFor(() => {
    const els = Array.from(
      document.querySelectorAll(
        '.ant-select-dropdown:not(.ant-dropdown-hidden) .ant-select-item-option-content',
      ),
    ).filter((el) => el.textContent === label);
    expect(els.length).toBeGreaterThan(0);
    return els[els.length - 1] as HTMLElement;
  });
  fireEvent.click(item);
}

/** tags 模式 Select 追加 tag：输入文本后点下拉里「新建项」option（antd6
 * tags 无 options 时输入文本即成为可选 option；Enter 提交在连续追 tag 时
 * 只落首个 token——实证 rc-select 宏任务竞态，改点选更稳） */
async function typeTag(selectRoot: HTMLElement, value: string) {
  await sleep(60);
  fireEvent.mouseDown(selectRoot);
  const input = selectRoot.querySelector('input') as HTMLInputElement;
  fireEvent.change(input, { target: { value } });
  const item = await waitFor(() => {
    const els = Array.from(
      document.querySelectorAll(
        '.ant-select-dropdown:not(.ant-dropdown-hidden) .ant-select-item-option-content',
      ),
    ).filter((el) => el.textContent === value);
    expect(els.length).toBeGreaterThan(0);
    return els[els.length - 1] as HTMLElement;
  });
  fireEvent.click(item);
}

/** RangePicker 逐输入 focus+change+Enter（单面板直接 change+OK 不提交） */
async function setRange(start: string, end: string) {
  const inputs = () => Array.from(document.querySelectorAll('.ant-picker input'));
  fireEvent.mouseDown(document.querySelector('.ant-picker') as HTMLElement);
  const first = inputs()[0] as HTMLInputElement;
  fireEvent.focus(first);
  fireEvent.change(first, { target: { value: start } });
  fireEvent.keyDown(first, { key: 'Enter', code: 'Enter' });
  await sleep(60);
  const second = inputs()[1] as HTMLInputElement;
  fireEvent.focus(second);
  fireEvent.change(second, { target: { value: end } });
  fireEvent.keyDown(second, { key: 'Enter', code: 'Enter' });
  await sleep(60);
}

function renderPage() {
  return render(
    <App>
      <AnalyticsBehaviorPage />
    </App>,
  );
}

async function waitEvents() {
  expect(await screen.findByText('login')).toBeInTheDocument();
  await waitFor(() => expect(mEvents).toHaveBeenCalledTimes(1));
}

beforeEach(() => {
  jest.clearAllMocks();
  window.history.pushState({}, '', '/');
  mEvents.mockResolvedValue({ events: EVENT_ROWS, total: EVENT_ROWS.length });
  mFunnel.mockResolvedValue(FUNNEL);
  mPaths.mockResolvedValue({ paths: PATH_ROWS });
  mAdoption.mockResolvedValue(ADOPTION);
  mBreakdown.mockResolvedValue(BREAKDOWN);
});

describe('行为分析 事件探索', () => {
  it('挂载首拉空筛选载荷 + 表渲染 + 用户列 dataIndex 现状（user_id 键才上列）', async () => {
    renderPage();
    await waitEvents();

    expect(mEvents).toHaveBeenCalledWith({ event: '', propKey: '', propVal: '' });

    const eventsCard = EVENTS_CARD();
    expect(within(eventsCard).getByText('时间')).toBeInTheDocument();
    expect(within(eventsCard).getByText('事件')).toBeInTheDocument();
    expect(within(eventsCard).getByText('用户')).toBeInTheDocument();
    expect(within(eventsCard).getByText('login')).toBeInTheDocument();
    expect(within(eventsCard).getByText('pay')).toBeInTheDocument();
    expect(within(eventsCard).getByText('x')).toBeInTheDocument();
    // dataIndex='user_id'：仅带 user_id 键的行上列；归一化键 userId 不上列
    expect(within(eventsCard).getByText('u-legacy')).toBeInTheDocument();
    expect(within(eventsCard).queryByText('u-norm')).not.toBeInTheDocument();
  });

  it('三输入 + RangePicker 后查询：全参数 ISO 载荷', async () => {
    renderPage();
    await waitEvents();

    fireEvent.change(screen.getByPlaceholderText('事件名'), { target: { value: 'login' } });
    fireEvent.change(screen.getByPlaceholderText('属性Key'), { target: { value: 'channel' } });
    fireEvent.change(screen.getByPlaceholderText('属性值'), { target: { value: 'ios' } });
    await setRange('2026-09-01', '2026-09-02');
    fireEvent.click(screen.getByRole('button', { name: /查\s*询/ }));

    await waitFor(() =>
      expect(mEvents).toHaveBeenLastCalledWith({
        event: 'login',
        propKey: 'channel',
        propVal: 'ios',
        start: dayjs('2026-09-01').toISOString(),
        end: dayjs('2026-09-02').toISOString(),
      }),
    );
  });

  it('导出 events.csv：header + user_id/userId 双回退 + 空 id 行', async () => {
    renderPage();
    await waitEvents();

    fireEvent.click(within(EVENTS_CARD()).getByRole('button', { name: /导出 CSV/ }));
    await waitFor(() =>
      expect(mExport).toHaveBeenCalledWith('events.csv', [
        {
          sheet: 'events',
          rows: [
            ['time', 'event', 'user_id'],
            ['2026-09-01T10:00:00Z', 'login', 'u-norm'],
            ['2026-09-02T11:00:00Z', 'pay', 'u-legacy'],
            ['2026-09-03T12:00:00Z', 'x', ''],
          ],
        },
      ]),
    );
  });

  it('响应 {} 右翼：空表；空态导出仅 header 行', async () => {
    mEvents.mockResolvedValueOnce({} as never);
    renderPage();
    await waitFor(() => expect(mEvents).toHaveBeenCalledTimes(1));
    expect(screen.queryByText('login')).not.toBeInTheDocument();

    fireEvent.click(within(EVENTS_CARD()).getByRole('button', { name: /导出 CSV/ }));
    await waitFor(() =>
      expect(mExport).toHaveBeenCalledWith('events.csv', [
        { sheet: 'events', rows: [['time', 'event', 'user_id']] },
      ]),
    );
  });
});

describe('行为分析 漏斗', () => {
  it('默认态计算 {steps:"",sequential:0}；全参数态四键齐 + 表 `${v}%` 渲染 + funnel.csv 导出', async () => {
    renderPage();
    await waitEvents();

    // 默认态：无 tag/开关，无 sameSession/gapSec 键
    fireEvent.click(within(FUNNEL_CARD()).getByRole('button', { name: /计\s*算/ }));
    await waitFor(() => expect(mFunnel).toHaveBeenLastCalledWith({ steps: '', sequential: 0 }));

    // 全参数：tags 双步 + 顺序 + 同会话 + 步间秒数
    await typeTag(FUNNEL_CARD().querySelector('.ant-select') as HTMLElement, 'a');
    await typeTag(FUNNEL_CARD().querySelector('.ant-select') as HTMLElement, 'b');
    fireEvent.click(within(FUNNEL_CARD()).getByRole('switch'));
    fireEvent.click(within(FUNNEL_CARD()).getByRole('checkbox'));
    fireEvent.change(within(FUNNEL_CARD()).getByRole('spinbutton'), { target: { value: '300' } });
    fireEvent.click(within(FUNNEL_CARD()).getByRole('button', { name: /计\s*算/ }));

    await waitFor(() =>
      expect(mFunnel).toHaveBeenLastCalledWith({
        steps: 'a,b',
        sequential: 1,
        sameSession: 1,
        gapSec: 300,
      }),
    );

    // 表渲染（mock 归一化产物）+ `${v}%`
    const funnelCard = FUNNEL_CARD();
    expect(await within(funnelCard).findByText('login')).toBeInTheDocument();
    expect(within(funnelCard).getByText('100%')).toBeInTheDocument();
    expect(within(funnelCard).getByText('30%')).toBeInTheDocument();
    expect(within(funnelCard).getByText('30')).toBeInTheDocument();

    // 导出：String 强转
    fireEvent.click(within(funnelCard).getByRole('button', { name: /导出 CSV/ }));
    await waitFor(() =>
      expect(mExport).toHaveBeenCalledWith('funnel.csv', [
        {
          sheet: 'funnel',
          rows: [
            ['step', 'users', 'rate'],
            ['login', '100', '100'],
            ['pay', '30', '30'],
          ],
        },
      ]),
    );

    // 步间秒数清空：Number(v||0) → gapSec 键消失（其余预置态保持）
    const funnelSpin = within(funnelCard).getByRole('spinbutton');
    fireEvent.change(funnelSpin, { target: { value: '' } });
    fireEvent.blur(funnelSpin);
    fireEvent.click(within(funnelCard).getByRole('button', { name: /计\s*算/ }));
    await waitFor(() =>
      expect(mFunnel).toHaveBeenLastCalledWith({
        steps: 'a,b',
        sequential: 1,
        sameSession: 1,
      }),
    );

    // 响应 {} 右翼：steps || [] → 漏斗表清空
    mFunnel.mockResolvedValueOnce({} as never);
    fireEvent.click(within(funnelCard).getByRole('button', { name: /计\s*算/ }));
    await waitFor(() => expect(mFunnel).toHaveBeenCalledTimes(4));
    await waitFor(() => expect(within(funnelCard).queryByText('100%')).not.toBeInTheDocument());
  });

  it('复制链接：空态仅 ?；全参态六键按插入序拼 URL', async () => {
    renderPage();
    await waitEvents();

    // 空态：全条件假 → URLSearchParams 空 → pathname + '?'
    fireEvent.click(within(FUNNEL_CARD()).getByRole('button', { name: /复制链接/ }));
    expect(clipboardWrite).toHaveBeenLastCalledWith(
      `${window.location.origin}${window.location.pathname}?`,
    );

    await typeTag(FUNNEL_CARD().querySelector('.ant-select') as HTMLElement, 'a');
    await typeTag(FUNNEL_CARD().querySelector('.ant-select') as HTMLElement, 'b');
    fireEvent.click(within(FUNNEL_CARD()).getByRole('switch'));
    fireEvent.click(within(FUNNEL_CARD()).getByRole('checkbox'));
    fireEvent.change(within(FUNNEL_CARD()).getByRole('spinbutton'), { target: { value: '300' } });
    await setRange('2026-09-01', '2026-09-02');
    fireEvent.click(within(FUNNEL_CARD()).getByRole('button', { name: /复制链接/ }));

    const expected = new URLSearchParams();
    expected.set('steps', 'a,b');
    expected.set('sequential', '1');
    expected.set('same_session', '1');
    expected.set('gap_sec', '300');
    expected.set('start', dayjs('2026-09-01').toISOString());
    expected.set('end', dayjs('2026-09-02').toISOString());
    await waitFor(() =>
      expect(clipboardWrite).toHaveBeenLastCalledWith(
        `${window.location.origin}${window.location.pathname}?${expected.toString()}`,
      ),
    );
  });
});

describe('行为分析 深链预填与自动计算', () => {
  it('全参深链：steps trim/filter 归一 + 四态预填 + setTimeout 自动漏斗（挂载闭包不含 range，现状锁定）+ range 进后续查询', async () => {
    window.history.pushState(
      {},
      '',
      '/analytics/behavior?steps=+a+,,+b+&sequential=1&same_session=1&gap_sec=120&start=2026-09-01&end=2026-09-02',
    );
    renderPage();
    await waitFor(() =>
      expect(mFunnel).toHaveBeenCalledWith({
        steps: 'a,b',
        sequential: 1,
        sameSession: 1,
        gapSec: 120,
      }),
    );
    // 现状锁定：自动计算经挂载期闭包捕获 range=null，start/end 不进自动载荷；
    // range 预填只对后续手动计算/查询生效（下方两段断言）
    const autoCall = mFunnel.mock.calls[0]?.[0] as Record<string, unknown>;
    expect(autoCall.start).toBeUndefined();
    expect(autoCall.end).toBeUndefined();

    // 预填的 range 传播到事件查询（挂载首拉无 start，重查带上）
    fireEvent.click(screen.getByRole('button', { name: /查\s*询/ }));
    await waitFor(() =>
      expect(mEvents).toHaveBeenLastCalledWith(
        expect.objectContaining({
          start: dayjs('2026-09-01').toISOString(),
          end: dayjs('2026-09-02').toISOString(),
        }),
      ),
    );

    // 手动「计算」用新渲染闭包：range 进漏斗载荷
    fireEvent.click(within(FUNNEL_CARD()).getByRole('button', { name: /计\s*算/ }));
    await waitFor(() =>
      expect(mFunnel).toHaveBeenLastCalledWith(
        expect.objectContaining({
          steps: 'a,b',
          sequential: 1,
          sameSession: 1,
          gapSec: 120,
          start: dayjs('2026-09-01').toISOString(),
          end: dayjs('2026-09-02').toISOString(),
        }),
      ),
    );
  });

  it('负翼深链：无 steps 不自动算、sequential 非 1/ gap_sec 非法/日期非法均不置态', async () => {
    window.history.pushState(
      {},
      '',
      '/analytics/behavior?sequential=0&gap_sec=abc&start=notadate&end=alsobad',
    );
    renderPage();
    await waitEvents();

    expect(mFunnel).not.toHaveBeenCalled();
    // Switch 未置 / range 未置：重查不带 start
    fireEvent.click(screen.getByRole('button', { name: /查\s*询/ }));
    await waitFor(() => expect(mEvents).toHaveBeenCalledTimes(2));
    expect(mEvents).toHaveBeenLastCalledWith({ event: '', propKey: '', propVal: '' });
    expect(
      (within(FUNNEL_CARD()).getByRole('switch') as HTMLElement).getAttribute('aria-checked'),
    ).toBe('false');
  });
});

describe('行为分析 路径分析', () => {
  it('默认载荷 {per:"session",steps:5,limit:50}；全参数（tags/同会话/秒数/正则 trim）+ InputNumber 清空回默认', async () => {
    renderPage();
    await waitEvents();
    const pathCard = PATH_CARD();

    fireEvent.click(within(pathCard).getByRole('button', { name: /计算路径/ }));
    await waitFor(() =>
      expect(mPaths).toHaveBeenLastCalledWith({ per: 'session', steps: 5, limit: 50 }),
    );

    // 全参数装配
    const selects = () => Array.from(pathCard.querySelectorAll('.ant-select'));
    await pickOption(selects()[0] as HTMLElement, '按用户');
    await typeTag(selects()[1] as HTMLElement, 'pay');
    await typeTag(selects()[2] as HTMLElement, 'ad');
    fireEvent.click(within(pathCard).getByRole('checkbox'));
    const spins = () => within(pathCard).getAllByRole('spinbutton');
    fireEvent.change(spins()[2], { target: { value: '300' } }); // 步间秒数
    fireEvent.change(screen.getByPlaceholderText('路径包含正则'), {
      target: { value: '  a.*b  ' },
    });
    fireEvent.change(screen.getByPlaceholderText('路径排除正则'), { target: { value: 'skip' } });
    await setRange('2026-09-01', '2026-09-02');
    fireEvent.click(within(pathCard).getByRole('button', { name: /计算路径/ }));

    await waitFor(() =>
      expect(mPaths).toHaveBeenLastCalledWith({
        per: 'user',
        steps: 5,
        limit: 50,
        include: 'pay',
        exclude: 'ad',
        sameSession: 1,
        gapSec: 300,
        pathRe: 'a.*b',
        pathNotRe: 'skip',
        start: dayjs('2026-09-01').toISOString(),
        end: dayjs('2026-09-02').toISOString(),
      }),
    );

    // InputNumber 清空：步数回 5、TopN 回 50、步间秒数回 0（Number(v||default)）
    fireEvent.change(spins()[0], { target: { value: '' } });
    fireEvent.blur(spins()[0]);
    fireEvent.change(spins()[1], { target: { value: '' } });
    fireEvent.blur(spins()[1]);
    fireEvent.change(spins()[2], { target: { value: '' } });
    fireEvent.blur(spins()[2]);
    fireEvent.click(within(pathCard).getByRole('button', { name: /计算路径/ }));
    await waitFor(() =>
      expect(mPaths).toHaveBeenLastCalledWith(
        expect.objectContaining({ per: 'user', steps: 5, limit: 50 }),
      ),
    );
    const last = mPaths.mock.calls[mPaths.mock.calls.length - 1]?.[0] as Record<string, unknown>;
    expect(last.sameSession).toBe(1);
    expect(last.gapSec).toBeUndefined();
  });

  it('表渲染 + 操作列：填充漏斗（载荷/scrollIntoView/步骤回填）、复制步骤、导出兜底翼', async () => {
    renderPage();
    await waitEvents();
    const pathCard = PATH_CARD();

    fireEvent.click(within(pathCard).getByRole('button', { name: /计算路径/ }));
    expect(await within(pathCard).findByText('login>pay')).toBeInTheDocument();
    expect(within(pathCard).getByText('login>shop>pay')).toBeInTheDocument();
    expect(within(pathCard).getByText('10')).toBeInTheDocument();

    // 填充漏斗：path split('>') → loadFunnel({steps}) + scrollIntoView + setSteps
    const row = within(pathCard)
      .getAllByRole('button', { name: /填充漏斗/ })[0]
      .closest('tr') as HTMLElement;
    fireEvent.click(within(row).getByRole('button', { name: /填充漏斗/ }));
    await waitFor(() =>
      expect(mFunnel).toHaveBeenLastCalledWith(expect.objectContaining({ steps: 'login,pay' })),
    );
    await waitFor(() => expect(scrollIntoViewMock).toHaveBeenCalled());

    // 复制步骤
    fireEvent.click(within(row).getByRole('button', { name: /复制步骤/ }));
    expect(clipboardWrite).toHaveBeenLastCalledWith('login>pay');

    // 导出：path ''/groups 0 的 String 兜底翼
    fireEvent.click(within(pathCard).getByRole('button', { name: /导出 CSV/ }));
    await waitFor(() =>
      expect(mExport).toHaveBeenCalledWith('paths.csv', [
        {
          sheet: 'paths',
          rows: [
            ['path', 'groups'],
            ['login>pay', '10'],
            ['login>shop>pay', '5'],
            ['', ''],
          ],
        },
      ]),
    );
  });

  it('匹配漏斗指示器：steps+pathRe 是/否两态、非法正则无指示器、无 steps 无指示器', async () => {
    window.history.pushState({}, '', '/analytics/behavior?steps=login,pay');
    renderPage();
    await waitEvents();
    const pathCard = PATH_CARD();

    fireEvent.change(screen.getByPlaceholderText('路径包含正则'), { target: { value: 'pay' } });
    expect(await within(pathCard).findByText('与当前漏斗步骤匹配：')).toBeInTheDocument();
    expect(within(pathCard).getByText('是')).toBeInTheDocument();

    fireEvent.change(screen.getByPlaceholderText('路径包含正则'), { target: { value: 'zzz' } });
    await waitFor(() => expect(within(pathCard).getByText('否')).toBeInTheDocument());

    // 非法正则：IIFE catch → null（无指示器）
    fireEvent.change(screen.getByPlaceholderText('路径包含正则'), { target: { value: '[' } });
    await waitFor(() =>
      expect(within(pathCard).queryByText('与当前漏斗步骤匹配：')).not.toBeInTheDocument(),
    );
  });

  it('无 steps 时指示器不渲染（!p 翼）+ 响应 {} 右翼空表', async () => {
    renderPage();
    await waitEvents();
    const pathCard = PATH_CARD();

    fireEvent.change(screen.getByPlaceholderText('路径包含正则'), { target: { value: 'pay' } });
    expect(within(pathCard).queryByText('与当前漏斗步骤匹配：')).not.toBeInTheDocument();

    mPaths.mockResolvedValueOnce({} as never);
    fireEvent.click(within(pathCard).getByRole('button', { name: /计算路径/ }));
    await waitFor(() => expect(mPaths).toHaveBeenCalledTimes(1));
    expect(within(pathCard).queryByText('login>pay')).not.toBeInTheDocument();
  });
});

describe('行为分析 功能采用率', () => {
  it('主链：默认载荷 + 基数行值内插；tags/per 切换后载荷 + 表渲染 + adoption.csv 导出', async () => {
    renderPage();
    await waitEvents();
    const adoptionCard = ADOPTION_CARD();

    // 默认态基数行：per=user、baseline=0
    expect(within(adoptionCard).getByText('基数（用户）：0')).toBeInTheDocument();

    fireEvent.click(within(adoptionCard).getByRole('button', { name: /计算采用率/ }));
    await waitFor(() => expect(mAdoption).toHaveBeenLastCalledWith({ features: '', per: 'user' }));

    // features 双 tag + 切按会话
    const selects = () => Array.from(adoptionCard.querySelectorAll('.ant-select'));
    await typeTag(selects()[0] as HTMLElement, 'first_pay');
    await typeTag(selects()[0] as HTMLElement, 'open_store');
    await pickOption(selects()[1] as HTMLElement, '按会话');
    fireEvent.click(within(adoptionCard).getByRole('button', { name: /计算采用率/ }));

    await waitFor(() =>
      expect(mAdoption).toHaveBeenLastCalledWith({
        features: 'first_pay,open_store',
        per: 'session',
      }),
    );
    // baseline 内插 + 表渲染（first_pay 与 Select tag 同文本，收窄到表格）
    expect(await within(adoptionCard).findByText('基数（会话）：200')).toBeInTheDocument();
    const adoptionTable = within(adoptionCard).getAllByRole('table')[0] as HTMLElement;
    expect(within(adoptionTable).getByText('first_pay')).toBeInTheDocument();
    expect(within(adoptionCard).getByText('25%')).toBeInTheDocument();

    // 导出主表
    fireEvent.click(within(adoptionCard).getAllByRole('button', { name: /导出 CSV/ })[0]);
    await waitFor(() =>
      expect(mExport).toHaveBeenCalledWith('adoption.csv', [
        {
          sheet: 'adoption',
          rows: [
            ['feature', 'groups', 'rate(%)', 'baseline'],
            ['first_pay', '50', '25', '200'],
          ],
        },
      ]),
    );
  });

  it('分层：range 传播 + by 切按平台 → breakdown 载荷 + dim 表渲染 + adoption_breakdown.csv 导出；{} 右翼归零', async () => {
    renderPage();
    await waitEvents();
    const adoptionCard = ADOPTION_CARD();

    // range 经 props 传播进 load/loadDim 载荷（start/end 翼）
    await setRange('2026-09-01', '2026-09-02');
    const selects = () => Array.from(adoptionCard.querySelectorAll('.ant-select'));
    await typeTag(selects()[0] as HTMLElement, 'first_pay');
    await pickOption(selects()[2] as HTMLElement, '按平台');
    fireEvent.click(within(adoptionCard).getByRole('button', { name: /分层采用率/ }));

    await waitFor(() =>
      expect(mBreakdown).toHaveBeenLastCalledWith({
        features: 'first_pay',
        per: 'user',
        by: 'platform',
        start: dayjs('2026-09-01').toISOString(),
        end: dayjs('2026-09-02').toISOString(),
      }),
    );
    expect(await within(adoptionCard).findByText('ios')).toBeInTheDocument();
    expect(within(adoptionCard).getByText('5%')).toBeInTheDocument();

    fireEvent.click(within(adoptionCard).getAllByRole('button', { name: /导出 CSV/ })[1]);
    await waitFor(() =>
      expect(mExport).toHaveBeenCalledWith('adoption_breakdown.csv', [
        {
          sheet: 'adoption_breakdown',
          rows: [
            ['dim', 'baseline', 'groups', 'rate(%)'],
            ['ios', '200', '10', '5'],
            ['', '0', '0', '0'],
          ],
        },
      ]),
    );

    // {} 双右翼：主表空 + baseline 归零 + 分层空（breakdown 第二次调用）
    mAdoption.mockResolvedValueOnce({} as never);
    mBreakdown.mockResolvedValueOnce({} as never);
    fireEvent.click(within(adoptionCard).getByRole('button', { name: /计算采用率/ }));
    await waitFor(() =>
      expect(within(adoptionCard).getByText('基数（用户）：0')).toBeInTheDocument(),
    );
    fireEvent.click(within(adoptionCard).getByRole('button', { name: /分层采用率/ }));
    await waitFor(() => expect(mBreakdown).toHaveBeenCalledTimes(2));
    expect(within(adoptionCard).queryByText('ios')).not.toBeInTheDocument();
  });
});
