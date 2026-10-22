/**
 * 留存分析页单测（覆盖率补缺轮：Analytics/Retention/index.tsx 179 行 0% →
 * 收口，零测试簇排行现席）。
 *
 * 锁定契约：
 * - 挂载：fetchAnalyticsRetention({cohort:'signup'})（range 空不带 start/end）
 *   → 卡片标题 + 工具栏（cohort Select + RangePicker + 查询 + 导出 CSV）+
 *   七列矩阵（Cohort/用户数/D1/D3/D7/D14/D30）；
 * - 列渲染：users `v?.toLocaleString() || 0`（1234 → '1,234'）、D 列
 *   `v != null ? (v*100).toFixed(2)+'%' : '-'` 双翼、retention 稀疏数组
 *   越界位落 null → '-'、`c.retention || []` 缺省翼全 '-'；
 * - cohort 切换：Select 选「按首次活跃」→ load 身份重建 → effect 自动重拉
 *   {cohort:'first_active'}；
 * - RangePicker：设区间 → effect 自动重拉带 start/end（dayjs ISO）；
 *   清空图标（mouseEnter 显形）→ onChange(null) → 重拉回到无 start/end；
 * - 查询按钮：直接调 load → +1 次同参拉取；
 * - 导出 CSV：exportToXLSX('retention.csv', [{sheet:'retention', rows:
 *   [header, ...rows]}])——users 原值 number、缺省 D 列 null 单元格；
 * - 响应缺省：resolve undefined → `r || {cohorts: []}` 右翼空表（服务
 *   返回无类型声明，204/空体为合法形态）；resolve {} → `data?.cohorts
 *   || []` 右翼空表；
 * - 排序：五列 sorter（D1/D3/D7/D14/D30）各点击列头一次全部触达，
 *   D1 升序首行翻转到 d1=0 行。
 *
 * mock 口径：services/api/analytics 只 mock fetchAnalyticsRetention（页面
 * 唯一消费；类型导入编译期擦除）；@/utils/export mock exportToXLSX（断言
 * 载荷不落盘）；@umijs/max 本地 defaultMessage mock；pro-components
 * spread requireActual + PageContainer 桩（本页 PageContainer 无 extra 槽）。
 * 页面不消费 App.useApp——无需 App 包裹。
 *
 * 坑实证（antd 6.6.0 实测，复用既有坑档）：
 * - RangePicker 逐输入 focus+change+Enter（单面板直接 change+OK 不提交，
 *   Behavior 套件同款配方）；清空图标须先 mouseEnter 显形（Levels 坑档）；
 * - 双字中文 Button 自动插空格（查 询），role name 宽松正则；
 * - Select option 点击配方 sleep≥60ms → mouseDown 落 .ant-select 根 →
 *   点可见 dropdown 内 .ant-select-item-option-content。
 *
 * 现状锁定 / 边界（诚实清单，不造假用例不删防御分支）：
 * 1. `if (range && range[0])` / `range[1]` 半开翼——RangePicker onChange
 *    只产「完整对或 null」，range 非空且单侧 null 构造性不可达，登记
 *    （Behavior 套件同结论）。
 * 2. users 渲染 `v?.toLocaleString() || 0` 的 falsy 右翼与 `?.` null 翼
 *    ——CohortData.users 类型必填 number，undefined 违反类型契约即造假，
 *    登记（v=0 走左翼渲染 '0'）。
 * 3. `data?.cohorts` 的 data-null 翼——state 初始即对象、setData 经
 *    `r || {cohorts: []}` 恒对象，结构不可达，登记；`|| []` 右翼经
 *    resolve {} 真实覆盖。
 * 4. load 的 try/finally 无 catch——fetchAnalyticsRetention reject 成
 *    unhandled rejection（同族页面既有口径），不造假 reject 场景。
 */
import React from 'react';
import { configure, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import dayjs from 'dayjs';
import AnalyticsRetentionPage from '../index';
import { fetchAnalyticsRetention } from '@/services/api/analytics';
import { exportToXLSX } from '@/utils/export';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

jest.mock('@/services/api/analytics', () => ({
  fetchAnalyticsRetention: jest.fn(),
}));

jest.mock('@/utils/export', () => ({
  exportToXLSX: jest.fn().mockResolvedValue(undefined),
}));

// 本页 intl/FormattedMessage 全带 defaultMessage——回 defaultMessage 使文案可见
jest.mock('@umijs/max', () => ({
  FormattedMessage: ({ defaultMessage }: { defaultMessage?: string }) => (
    <>{defaultMessage ?? ''}</>
  ),
  useIntl: () => ({
    formatMessage: (opts: { defaultMessage?: string }) => opts.defaultMessage ?? opts.id ?? '',
  }),
}));

jest.mock('@ant-design/pro-components', () => ({
  ...jest.requireActual('@ant-design/pro-components'),
  PageContainer: ({ children }: { children?: React.ReactNode }) => <div>{children}</div>,
}));

const mFetch = fetchAnalyticsRetention as jest.MockedFunction<typeof fetchAnalyticsRetention>;
const mExport = exportToXLSX as jest.MockedFunction<typeof exportToXLSX>;

function sleep(ms: number) {
  return new Promise((r) => setTimeout(r, ms));
}

// c1 满数组 / c2 稀疏（越界位 null）/ c3、c4 缺 retention 键（|| [] 右翼；
// 两条 null-d1 行使 D1 sorter 出现 null 对 null 比较，`a.d1||0`/`b.d1||0`
// 双翼均可触达——3 行时 V8 插入排序不产生 null 作 b 操作数的比较）
const RESP = {
  cohorts: [
    { cohort: '2026-09-01', users: 1234, retention: [0.5, 0.25, 0.1, 0.05, 0.02] },
    { cohort: '2026-09-02', users: 7, retention: [0.333] },
    { cohort: '2026-09-03', users: 0 },
    { cohort: '2026-09-04', users: 42 },
  ],
};

function renderPage() {
  return render(<AnalyticsRetentionPage />);
}

async function waitLoaded() {
  await waitFor(() => expect(mFetch).toHaveBeenCalledTimes(1));
  await waitFor(() => expect(screen.getByText('2026-09-01')).toBeInTheDocument());
}

/** RangePicker 逐输入 focus+change+Enter（Behavior 套件同款配方） */
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

/** Select option 点击（sleep≥60ms → mouseDown → 点可见 option content） */
async function pickOption(label: string) {
  await sleep(60);
  fireEvent.mouseDown(document.querySelector('.ant-select') as HTMLElement);
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

beforeEach(() => {
  jest.clearAllMocks();
  mFetch.mockResolvedValue(RESP as never);
});

describe('留存分析 挂载与列表', () => {
  it('首拉 {cohort:signup} + 七列矩阵（千分位/百分率/缺省双翼）+ 工具栏', async () => {
    renderPage();
    await waitLoaded();

    expect(mFetch).toHaveBeenCalledWith({ cohort: 'signup' });
    expect(screen.getByText('留存分析')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /查\s*询/ })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /导出 CSV/ })).toBeInTheDocument();

    const thead = document.querySelector('.ant-table-thead') as HTMLElement;
    for (const h of ['Cohort', '用户数', 'D1', 'D3', 'D7', 'D14', 'D30']) {
      expect(within(thead).getByText(h)).toBeInTheDocument();
    }

    // users 千分位左翼 + 0 原样（0 走 toLocaleString 左翼渲染 '0'）
    expect(screen.getByText('1,234')).toBeInTheDocument();
    expect(screen.getByText('7')).toBeInTheDocument();
    expect(screen.getByText('0')).toBeInTheDocument();

    // c1 五列百分率
    const row1 = screen.getByText('2026-09-01').closest('tr') as HTMLElement;
    for (const pct of ['50.00%', '25.00%', '10.00%', '5.00%', '2.00%']) {
      expect(within(row1).getByText(pct)).toBeInTheDocument();
    }

    // c2 稀疏：d1 有值、d3-d30 '-'（retention 越界 → null → '-' 右翼）
    const row2 = screen.getByText('2026-09-02').closest('tr') as HTMLElement;
    expect(within(row2).getByText('33.30%')).toBeInTheDocument();
    expect(within(row2).getAllByText('-')).toHaveLength(4);

    // c3 缺 retention 键：五列全 '-'（`c.retention || []` 右翼）
    const row3 = screen.getByText('2026-09-03').closest('tr') as HTMLElement;
    expect(within(row3).getAllByText('-')).toHaveLength(5);
  });

  it('resolve undefined / {} → 空表（`r ||` 右翼 / `cohorts || []` 右翼）', async () => {
    mFetch.mockResolvedValueOnce(undefined as never);
    const { unmount } = renderPage();
    await waitFor(() => expect(mFetch).toHaveBeenCalledTimes(1));
    expect(
      await screen.findByText('No data', { selector: '.ant-empty-description' }),
    ).toBeInTheDocument();
    unmount();

    mFetch.mockResolvedValueOnce({} as never);
    renderPage();
    // 同用例内第二次渲染：累计第 2 次拉取
    await waitFor(() => expect(mFetch).toHaveBeenCalledTimes(2));
    expect(
      await screen.findByText('No data', { selector: '.ant-empty-description' }),
    ).toBeInTheDocument();
  });
});

describe('留存分析 筛选与查询', () => {
  it('cohort Select 切换 → effect 自动重拉 {cohort:first_active}', async () => {
    renderPage();
    await waitLoaded();

    await pickOption('按首次活跃');
    await waitFor(() => expect(mFetch).toHaveBeenLastCalledWith({ cohort: 'first_active' }));
  });

  it('RangePicker 设区间 → 自动重拉带 start/end；清空 → 回到无 start/end', async () => {
    renderPage();
    await waitLoaded();

    await setRange('2026-09-01', '2026-09-02');
    await waitFor(() =>
      expect(mFetch).toHaveBeenLastCalledWith({
        cohort: 'signup',
        start: dayjs('2026-09-01').toISOString(),
        end: dayjs('2026-09-02').toISOString(),
      }),
    );

    // 清空图标须先 mouseEnter 显形（Levels 坑档）
    const picker = document.querySelector('.ant-picker') as HTMLElement;
    fireEvent.mouseEnter(picker);
    const clear = await waitFor(() => {
      const el = picker.querySelector('.ant-picker-clear') as HTMLElement;
      expect(el).not.toBeNull();
      return el;
    });
    fireEvent.click(clear);
    await waitFor(() => expect(mFetch).toHaveBeenLastCalledWith({ cohort: 'signup' }));
  });

  it('查询按钮 → +1 次同参拉取', async () => {
    renderPage();
    await waitLoaded();

    fireEvent.click(screen.getByRole('button', { name: /查\s*询/ }));
    await waitFor(() => expect(mFetch).toHaveBeenCalledTimes(2));
    expect(mFetch).toHaveBeenLastCalledWith({ cohort: 'signup' });
  });
});

describe('留存分析 导出与排序', () => {
  it('导出 CSV：header + 原值 users + null 单元格', async () => {
    renderPage();
    await waitLoaded();

    fireEvent.click(screen.getByRole('button', { name: /导出 CSV/ }));
    await waitFor(() => expect(mExport).toHaveBeenCalledTimes(1));
    expect(mExport).toHaveBeenCalledWith('retention.csv', [
      {
        sheet: 'retention',
        rows: [
          ['cohort', 'users', 'd1', 'd3', 'd7', 'd14', 'd30'],
          ['2026-09-01', 1234, 0.5, 0.25, 0.1, 0.05, 0.02],
          ['2026-09-02', 7, 0.333, null, null, null, null],
          ['2026-09-03', 0, null, null, null, null, null],
          ['2026-09-04', 42, null, null, null, null, null],
        ],
      },
    ]);
  });

  it('五列 sorter 触达：D1 升序首行翻转到 d1 最小行', async () => {
    renderPage();
    await waitLoaded();

    const ths = Array.from(document.querySelectorAll('.ant-table-thead th')) as HTMLElement[];
    // 列序：Cohort/用户数/D1/D3/D7/D14/D30——五列各点一次触达全部 sorter；
    // antd 单列排序每次点击整体切换 sortColumn，故 D1 留最后点，末态即 D1 升序
    for (const idx of [6, 5, 4, 3, 2]) {
      fireEvent.click(ths[idx]);
      await waitFor(() => {
        expect(ths[idx].className).toContain('ant-table-column-sort');
      });
    }

    // D1 首次点击 = 升序：d1 null(→0) 行 2026-09-03 < 0.333 < 0.5 居首
    const firstRow = document.querySelector('.ant-table-tbody tr') as HTMLElement;
    expect(within(firstRow).getByText('2026-09-03')).toBeInTheDocument();
  });
});
