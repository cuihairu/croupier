/**
 * 关卡分析页单测（覆盖率巡检：Analytics/Levels/index.tsx 683 行 0% → 收口，
 * 零测试页排行次席）。
 *
 * 锁定契约：
 * - 挂载两请求（levels/episodes；maps 分面无挂载 effect，仅「加载」按钮
 *   触发）与四卡渲染：漏斗表（step/users/
 *   rate `${v}%`）、分关卡表（winRate toFixed(2)%、avgDurationSec/
 *   avgRetries 原值、difficulty Tag 高→red/中→gold/其余 default、缺省 '-'）、
 *   分群胜率 SVG 图（标题、四条折线 path、图例四词、Top10 按参与数降序）、
 *   章节分面（Statistic 玩家/完成率%/平均进度%、episodeId 空 → 标题 '-'）、
 *   地图分面（热力点/死亡点计数、heatMap/deathSpots 非数组守卫 → []、
 *   mapId 空 → 标题 '-'）；
 * - 查询链：episode 输入即时触发重拉（useEffect 依赖）、查询按钮手动重拉、
 *   RangePicker 起止（focus+change+Enter 逐输入）→ start/end ISO 进
 *   三请求载荷（子分面组件随 range 重拉）；
 * - 分群下拉：切非 all 段 → perLevelSegments 空 → 表空态；
 * - 导出六入口：卡头导出 CSV（exportToCSV levels.csv + String 归一 +
 *   缺省字段空串）、漏斗底导出（XLSX levels_funnel.csv + rate 0 → ''）、
 *   统计底导出（XLSX levels_stats.csv：per_level 主表 + new/returning/payer
 *   三空段 sheet）、章节多 Sheet（ep_<id> + 空 id → ep_）、地图导出
 *   （map_<id> 计数行 + 空 id）；
 * - 响应缺省：levels/episodes {} + maps 点「加载」得 {} → 双表空态 +
 *   图不渲染 + 导出走 `|| []`/`|| {}` 右翼（表头行照常导出）；
 * - 加载前导出：慢接口（mLevels 悬置）下 data null 时三导出入口照常
 *   出表头行（`(data?.perLevel || [])`/`(data?.funnel || [])`/
 *   `data?.perLevelSegments || {}` 右翼）+ data null 下切分群段
 *   （`(data?.perLevelSegments || {})` 右翼）；
 * - 导出失败静默：exportToCSV 抛错 / exportToXLSX reject → catch 不白屏。
 *
 * mock 口径：services/api/analytics 三函数与 utils/export 两函数 jest.mock；
 * @umijs/max 本地 mock（defaultMessage 即文案）。
 *
 * 登记不可达（防御分支，不造假用例不删分支）：
 * - 漏斗 rate render `v != null ? … : '-'` 右翼：funnel 数据源由映射产出，
 *   rate 恒为 number（completionRate*100，缺省也是 NaN）；
 * - 分关卡 winRate render 同款右翼：winRate 恒为 number；
 * - 分群图 find 助手 `(arr || [])` 右翼：调用方传入 all 数组或 `|| []` 守卫值；
 * - 分群图 try/catch 的 catch：图体为纯计算（slice/sort/find/path 拼接），
 *   无可抛路径；
 * - EpisodeFacets/MapFacets 导出与渲染的 `(episodes || [])`/`(maps || [])`
 *   右翼：两组件 state 初始 []、setter 只赋 map 产出的数组；
 * - 统计底导出 mk 助手的 `(arr || [])` 右翼：arr 恒为 `segs[x] || []`
 *   产出的数组（空数组亦真值）。
 *
 * 坑实证（antd6 沿用）：RangePicker 单面板逐输入 focus+change+Enter 提交
 * （直接 change+OK 不提交，见 Functions/History 坑档）；同卡多个同名
 * 「导出 CSV」按钮按 DOM 序区分（卡头/漏斗底/统计底）；分群/双 limit
 * Select 按 .ant-card-extra 内 DOM 序 [seg, ep-limit, map-limit]。
 */
import React from 'react';
import { configure, fireEvent, render, screen, waitFor } from '@testing-library/react';
import AnalyticsLevelsPage from '../index';
import type {
  AnalyticsEpisodeMetric,
  AnalyticsLevelMetric,
  AnalyticsMapMetric,
} from '@/services/api/analytics';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

jest.mock('@/services/api/analytics', () => ({
  fetchAnalyticsLevels: jest.fn(),
  fetchAnalyticsLevelsEpisodes: jest.fn(),
  fetchAnalyticsLevelsMaps: jest.fn(),
}));

jest.mock('@/utils/export', () => ({
  exportToCSV: jest.fn(),
  exportToXLSX: jest.fn(),
}));

const mockIntl = {
  formatMessage: (opts: { defaultMessage?: string }) => opts.defaultMessage ?? '',
};

jest.mock('@umijs/max', () => ({
  FormattedMessage: ({ defaultMessage }: { defaultMessage?: string }) => (
    <>{defaultMessage ?? ''}</>
  ),
  useIntl: () => mockIntl,
}));

import {
  fetchAnalyticsLevels,
  fetchAnalyticsLevelsEpisodes,
  fetchAnalyticsLevelsMaps,
} from '@/services/api/analytics';
import { exportToCSV, exportToXLSX } from '@/utils/export';

const mLevels = fetchAnalyticsLevels as jest.MockedFunction<typeof fetchAnalyticsLevels>;
const mEpisodes = fetchAnalyticsLevelsEpisodes as jest.MockedFunction<
  typeof fetchAnalyticsLevelsEpisodes
>;
const mMaps = fetchAnalyticsLevelsMaps as jest.MockedFunction<typeof fetchAnalyticsLevelsMaps>;
const mCsv = exportToCSV as jest.MockedFunction<typeof exportToCSV>;
const mXlsx = exportToXLSX as jest.MockedFunction<typeof exportToXLSX>;

// 覆盖翼：difficulty 数字/高/中/缺省、winRate 0、无时长复试、levelId 空 +
// attempts 0（rowKey 空臂）、attempts 缺省（Top 排序 Number(players||0) 空臂）
const levels: AnalyticsLevelMetric[] = [
  {
    levelId: 'L1',
    attempts: 100,
    completions: 80,
    completionRate: 0.8,
    avgDuration: 90.5,
    avgRetries: 2.5,
    difficulty: 7,
  },
  {
    levelId: 'L2',
    attempts: 50,
    completions: 25,
    completionRate: 0.5,
    avgDuration: 60,
    avgRetries: 1,
    difficulty: '高' as never,
  },
  {
    levelId: 'L3',
    attempts: 40,
    completions: 20,
    completionRate: 0.5,
    avgDuration: 45,
    avgRetries: 3,
    difficulty: '中' as never,
  },
  { levelId: 'L4', attempts: 10, completions: 0, completionRate: 0 },
  { levelId: '', attempts: 0, completions: 0, completionRate: 0 },
  {
    levelId: 'L6',
    attempts: undefined as never,
    completions: 1,
    completionRate: 0.9,
    avgDuration: 10,
    avgRetries: 1,
  },
];

const episodes: AnalyticsEpisodeMetric[] = [
  { episodeId: 'ep-1', players: 42, completionRate: 0.55, avgProgress: 0.3 },
  { episodeId: '', players: 1, completionRate: 0, avgProgress: 0 },
];

// 覆盖翼：mapId 空 → 标题 '-'、heatMap/deathSpots 非数组 → [] 守卫
const maps: AnalyticsMapMetric[] = [
  { mapId: 'map-1', heatMap: [{ x: 1 }], deathSpots: [{ y: 1 }, { y: 2 }] },
  { mapId: '', heatMap: 'odd' as never, deathSpots: 'odd' as never },
];

beforeEach(() => {
  jest.clearAllMocks();
  mLevels.mockResolvedValue({ levels });
  mEpisodes.mockResolvedValue({ episodes });
  mMaps.mockResolvedValue({ maps });
});

function renderPage() {
  return render(<AnalyticsLevelsPage />);
}

/** 等首拉落定：levels/episodes 有挂载 effect；maps 分面无挂载 effect
 * （load 仅接「加载」按钮），须显式点击才拉取 */
async function waitLoad() {
  await waitFor(() => expect(mLevels).toHaveBeenCalledTimes(1));
  await waitFor(() => expect(mEpisodes).toHaveBeenCalledTimes(1));
  expect(await screen.findByText('关卡漏斗')).toBeInTheDocument();
  expect(screen.getByText('按地图分面')).toBeInTheDocument();
}

/** 点地图分面「加载」触发首拉（同名按钮 DOM 序 [章节, 地图]） */
async function loadMaps() {
  fireEvent.click(screen.getAllByRole('button', { name: '加载' })[1]);
  await waitFor(() => expect(mMaps).toHaveBeenCalledTimes(1));
}

/** 按统计卡标题取 .ant-statistic-content 拼接文本（值/后缀可能分元素） */
function statContent(title: string) {
  const titleEl = Array.from(document.querySelectorAll('.ant-statistic-title')).find(
    (n) => n.textContent === title,
  ) as HTMLElement;
  expect(titleEl).not.toBeUndefined();
  const stat = titleEl.closest('.ant-statistic') as HTMLElement;
  return stat.querySelector('.ant-statistic-content')?.textContent ?? '';
}

/** 第 idx 个卡 extra 内 Select 选 option（antd6：mouseDown 根 + 点可见 option content） */
async function pickExtraSelect(idx: number, label: string) {
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
  const option = Array.from(dropdown.querySelectorAll('.ant-select-item-option')).find(
    (o) => o.textContent === label,
  ) as HTMLElement;
  expect(option).not.toBeUndefined();
  fireEvent.click(option.querySelector('.ant-select-item-option-content') as HTMLElement);
}

function cards() {
  return Array.from(document.querySelectorAll('.ant-card')) as HTMLElement[];
}

/** 名为「导出 CSV」的按钮（卡头/漏斗底/统计底，DOM 序） */
function csvButtons() {
  return screen.getAllByRole('button', { name: '导出 CSV' });
}

describe('关卡分析 初始渲染', () => {
  it('四卡矩阵：漏斗/分关卡/分群图/章节与地图分面', async () => {
    renderPage();
    await waitLoad();

    expect(screen.getByText('关卡漏斗')).toBeInTheDocument();
    expect(screen.getByText('分关卡统计（胜率/难度/时长/复试）')).toBeInTheDocument();
    expect(screen.getByText('按章节分面')).toBeInTheDocument();
    expect(screen.getByText('按地图分面')).toBeInTheDocument();

    // 漏斗表：rate `${v}%`（80% / 0%）+ rowKey 空臂行不炸
    expect(screen.getByText('80%')).toBeInTheDocument();
    expect(screen.getAllByText('0%').length).toBeGreaterThanOrEqual(2);

    // 分关卡表：winRate toFixed(2)%、时长/复试原值、难度 Tag 四态
    expect(screen.getByText('80.00%')).toBeInTheDocument();
    expect(screen.getAllByText('50.00%')).toHaveLength(2); // L2/L3 同率
    expect(screen.getByText('90.5')).toBeInTheDocument();
    expect(screen.getByText('2.5')).toBeInTheDocument();
    expect(screen.getByText('高').closest('.ant-tag')).toHaveClass('ant-tag-red');
    expect(screen.getByText('中').closest('.ant-tag')).toHaveClass('ant-tag-gold');
    expect(screen.getByText('7').closest('.ant-tag')).not.toHaveClass('ant-tag-red');
    // '-' 多处（分面标题兜底 + 难度兜底），取 Tag 实例断言
    const dashTag = screen
      .getAllByText('-')
      .find((el) => el.closest('.ant-tag') !== null) as HTMLElement;
    expect(dashTag).not.toBeUndefined();

    // 分群图：标题 + 四条折线 path + 图例
    expect(screen.getByText('分群胜率对比（Top 10 关卡）')).toBeInTheDocument();
    expect(screen.getByText('胜率(%)')).toBeInTheDocument();
    // 页面首个 svg 是 RangePicker 图标：按含纵轴 label 的 svg 锚定图表本体
    const svgs = Array.from(document.querySelectorAll('svg')) as SVGSVGElement[];
    const svg = svgs.find((s) =>
      Array.from(s.querySelectorAll('text')).some((t) => t.textContent === '胜率(%)'),
    );
    expect(svg).not.toBeUndefined();
    expect(svg?.querySelectorAll('path')).toHaveLength(4); // all/new/ret/pay 四折线
    expect(Array.from(svg?.querySelectorAll('text') ?? []).map((t) => t.textContent)).toContain(
      'L1',
    );

    // 章节分面：ep-1 统计 + 空 episodeId → 标题 '-'
    expect(cards().some((c) => c.textContent?.includes('ep-1'))).toBe(true);
    expect(statContent('玩家')).toBe('42');
    expect(statContent('完成率')).toBe('55.00%');
    expect(statContent('平均进度')).toBe('30.00%');

    // 地图分面（无挂载 effect，点「加载」首拉）：计数 + 非数组守卫 → 0 + 空 mapId → 标题 '-'
    await loadMaps();
    expect(statContent('热力点')).toBe('1');
    expect(statContent('死亡点')).toBe('2');
    const dashCards = cards().filter(
      (c) => c.querySelector('.ant-card-head-title')?.textContent === '-',
    );
    expect(dashCards.length).toBeGreaterThanOrEqual(2);
    const mapDash = dashCards.find((c) => c.textContent?.includes('热力点'));
    expect(mapDash?.querySelectorAll('.ant-statistic-content')[0]?.textContent).toBe('0');

    // 首拉载荷：episode 空串、无 start/end
    expect(mLevels).toHaveBeenLastCalledWith({ episode: '' });
    expect(mEpisodes).toHaveBeenLastCalledWith({});
    expect(mMaps).toHaveBeenLastCalledWith({});
  });
});

describe('关卡分析 查询与筛选', () => {
  it('episode 输入即时重拉 + 查询按钮手动重拉', async () => {
    renderPage();
    await waitLoad();

    fireEvent.change(screen.getByPlaceholderText('章节/地图（可选）'), {
      target: { value: 'ep-x' },
    });
    await waitFor(() => expect(mLevels).toHaveBeenLastCalledWith({ episode: 'ep-x' }));

    fireEvent.click(screen.getByRole('button', { name: '查询' }));
    await waitFor(() => expect(mLevels).toHaveBeenCalledTimes(3));
    expect(mLevels).toHaveBeenLastCalledWith({ episode: 'ep-x' });
  });

  it('RangePicker 起止 → start/end ISO 进三请求（分面组件随 range 重拉）', async () => {
    renderPage();
    await waitLoad();

    const picker = document.querySelector('.ant-picker-range') as HTMLElement;
    const inputs = Array.from(picker.querySelectorAll('input')) as HTMLInputElement[];
    // 本页 picker 无 showTime：须输日期串（datetime 串解析失败不提交）
    fireEvent.focus(inputs[0]);
    fireEvent.change(inputs[0], { target: { value: '2026-09-01' } });
    fireEvent.keyDown(inputs[0], { key: 'Enter' });
    fireEvent.focus(inputs[1]);
    fireEvent.change(inputs[1], { target: { value: '2026-09-02' } });
    fireEvent.keyDown(inputs[1], { key: 'Enter' });

    const start = new Date('2026-09-01T00:00:00').toISOString();
    const end = new Date('2026-09-02T00:00:00').toISOString();
    await waitFor(() => expect(mLevels).toHaveBeenLastCalledWith({ episode: '', start, end }));
    await waitFor(() => expect(mEpisodes).toHaveBeenLastCalledWith({ start, end }));
    // maps 无挂载 effect：range 变更后须点「加载」，载荷带 start/end
    fireEvent.click(screen.getAllByRole('button', { name: '加载' })[1]);
    await waitFor(() => expect(mMaps).toHaveBeenLastCalledWith({ start, end }));
  });

  it('分群下拉切非 all 段 → perLevelSegments 空 → 分关卡表空态', async () => {
    renderPage();
    await waitLoad();

    await pickExtraSelect(0, '新玩家');
    await waitFor(() => expect(screen.queryByText('80.00%')).not.toBeInTheDocument());
    expect(screen.getByText('80%')).toBeInTheDocument(); // 漏斗表不受影响

    await pickExtraSelect(0, '付费玩家');
    expect(screen.queryByText('80.00%')).not.toBeInTheDocument();
  });
});

describe('关卡分析 导出', () => {
  it('卡头导出 CSV：exportToCSV levels.csv + String 归一 + 缺省字段空串', async () => {
    renderPage();
    await waitLoad();

    fireEvent.click(csvButtons()[0]);
    await waitFor(() => expect(mCsv).toHaveBeenCalledTimes(1));
    const [name, rows] = mCsv.mock.calls[0];
    expect(name).toBe('levels.csv');
    expect(rows[0]).toEqual(['level', 'players', 'win_rate', 'avg_duration_sec', 'avg_retries']);
    expect(rows).toContainEqual(['L1', '100', '80', '90.5', '2.5']);
    expect(rows).toContainEqual(['L4', '10', '0', '', '']);
    expect(rows).toContainEqual(['', '0', '0', '', '']);
  });

  it('漏斗底导出：XLSX levels_funnel.csv（rate 0 → 空串）', async () => {
    renderPage();
    await waitLoad();

    fireEvent.click(csvButtons()[1]);
    await waitFor(() => expect(mXlsx).toHaveBeenCalledTimes(1));
    const [name, sheets] = mXlsx.mock.calls[0];
    expect(name).toBe('levels_funnel.csv');
    expect(sheets[0].sheet).toBe('funnel');
    expect(sheets[0].rows).toContainEqual(['L1', '100', '80']);
    expect(sheets[0].rows).toContainEqual(['L4', '10', '']);
  });

  it('统计底导出：XLSX levels_stats.csv（主表 + 三空段 sheet）', async () => {
    renderPage();
    await waitLoad();

    fireEvent.click(csvButtons()[2]);
    await waitFor(() => expect(mXlsx).toHaveBeenCalledTimes(1));
    const [name, sheets] = mXlsx.mock.calls[0];
    expect(name).toBe('levels_stats.csv');
    expect(sheets).toHaveLength(4);
    expect(sheets[0].sheet).toBe('per_level');
    expect(sheets[0].rows[0]).toEqual([
      'level',
      'players',
      'win_rate',
      'avg_duration_sec',
      'avg_retries',
      'difficulty',
    ]);
    expect(sheets[0].rows).toContainEqual(['L1', '100', '80', '90.5', '2.5', '7']);
    expect(sheets[0].rows).toContainEqual(['L4', '10', '0', '', '', '']);
    expect(sheets.map((s: { sheet: string }) => s.sheet)).toEqual([
      'per_level',
      'per_level_new',
      'per_level_returning',
      'per_level_payer',
    ]);
    expect(sheets[1].rows).toEqual([['level', 'players', 'win_rate']]);
  });

  it('章节分面：加载重拉 + limit 12 → 4 列栅格 + 多 Sheet 导出', async () => {
    renderPage();
    await waitLoad();

    fireEvent.click(screen.getAllByRole('button', { name: '加载' })[0]);
    await waitFor(() => expect(mEpisodes).toHaveBeenCalledTimes(2));

    // extra Select DOM 序 [seg, ep-limit, map-limit]
    await pickExtraSelect(1, '12');
    const grid = cards()[2].querySelector('div[style*="grid"]') as HTMLElement;
    expect(grid.style.gridTemplateColumns).toBe('repeat(4, 1fr)');

    fireEvent.click(screen.getByRole('button', { name: '导出 Excel（多 Sheet）' }));
    await waitFor(() => expect(mXlsx).toHaveBeenCalledTimes(1));
    const [name, sheets] = mXlsx.mock.calls[0];
    expect(name).toBe('levels_episodes.csv');
    expect(sheets.map((s: { sheet: string }) => s.sheet)).toEqual(['ep_ep-1', 'ep_']);
    expect(sheets[0].rows).toEqual([
      ['episode', 'players', 'completion_rate', 'avg_progress'],
      ['ep-1', '42', String(0.55 * 100), String(0.3 * 100)],
    ]);
  });

  it('地图分面：加载重拉 + limit 12 → 4 列栅格 + 导出计数行', async () => {
    renderPage();
    await waitLoad();

    fireEvent.click(screen.getAllByRole('button', { name: '加载' })[1]);
    await waitFor(() => expect(mMaps).toHaveBeenCalledTimes(1));

    await pickExtraSelect(2, '12');
    // 大卡 DOM 序：funnel/perLevel/episodes + 2 章节小卡 → maps 是第 6 张卡
    const grid = cards()[5].querySelector('div[style*="grid"]') as HTMLElement;
    expect(grid.style.gridTemplateColumns).toBe('repeat(4, 1fr)');

    fireEvent.click(screen.getByRole('button', { name: '导出 Excel' }));
    await waitFor(() => expect(mXlsx).toHaveBeenCalledTimes(1));
    const [name, sheets] = mXlsx.mock.calls[0];
    expect(name).toBe('levels_maps.csv');
    expect(sheets.map((s: { sheet: string }) => s.sheet)).toEqual(['map_map-1', 'map_']);
    expect(sheets[0].rows).toEqual([
      ['map', 'heat_points', 'death_points'],
      ['map-1', '1', '2'],
    ]);
    expect(sheets[1].rows).toEqual([
      ['map', 'heat_points', 'death_points'],
      ['', '0', '0'],
    ]);
  });

  it('导出失败静默：exportToCSV 抛错 / exportToXLSX reject → catch 不白屏', async () => {
    renderPage();
    await waitLoad();

    mCsv.mockImplementationOnce(() => {
      throw new Error('csv-boom');
    });
    fireEvent.click(csvButtons()[0]);
    await waitFor(() => expect(mCsv).toHaveBeenCalledTimes(1));
    expect(screen.getByText('关卡漏斗')).toBeInTheDocument();

    mXlsx.mockRejectedValueOnce(new Error('xlsx-boom') as never);
    fireEvent.click(screen.getAllByRole('button', { name: '加载' })[0]);
    await waitFor(() => expect(mEpisodes).toHaveBeenCalledTimes(2));
    fireEvent.click(screen.getByRole('button', { name: '导出 Excel（多 Sheet）' }));
    await waitFor(() => expect(mXlsx).toHaveBeenCalledTimes(1));
    expect(screen.getByText('按章节分面')).toBeInTheDocument();

    // 地图导出 catch：同款 reject 静默
    mXlsx.mockRejectedValueOnce(new Error('map-xlsx-boom') as never);
    fireEvent.click(screen.getAllByRole('button', { name: '加载' })[1]);
    await waitFor(() => expect(mMaps).toHaveBeenCalledTimes(1));
    fireEvent.click(screen.getByRole('button', { name: '导出 Excel' }));
    await waitFor(() => expect(mXlsx).toHaveBeenCalledTimes(2));
    expect(screen.getByText('按地图分面')).toBeInTheDocument();
  });

  it('加载前导出：data null → 三入口 || 右翼仅表头行 + 分群段 {} 兜底', async () => {
    mLevels.mockReturnValue(new Promise(() => {}) as never);
    renderPage();

    // data 尚为 null（慢接口真实形态）时点三个导出：仅表头行
    fireEvent.click(csvButtons()[0]);
    fireEvent.click(csvButtons()[1]);
    fireEvent.click(csvButtons()[2]);
    await waitFor(() => expect(mCsv).toHaveBeenCalledTimes(1));
    await waitFor(() => expect(mXlsx).toHaveBeenCalledTimes(2));
    expect(mCsv.mock.calls[0][1]).toEqual([
      ['level', 'players', 'win_rate', 'avg_duration_sec', 'avg_retries'],
    ]);
    expect(mXlsx.mock.calls[0][0]).toBe('levels_funnel.csv');
    expect(mXlsx.mock.calls[0][1][0].rows).toEqual([['step', 'users', 'rate']]);
    expect(mXlsx.mock.calls[1][0]).toBe('levels_stats.csv');
    expect(mXlsx.mock.calls[1][1]).toHaveLength(4);

    // data null 下切分群段：(data?.perLevelSegments || {}) 右翼 → 空表
    await pickExtraSelect(0, '新玩家');
    const segSelect = document.querySelector('.ant-card-extra .ant-select') as HTMLElement;
    // antd6 选中态类名是 .ant-select-content（非 antd5 的 selection-item）
    expect(segSelect.querySelector('.ant-select-content')?.textContent).toContain('新玩家');
    expect(mLevels).toHaveBeenCalledTimes(1); // 不触发重拉（seg 非 load 依赖）
  });
});

describe('关卡分析 响应缺省', () => {
  it('三请求 {} → 双表空态 + 图不渲染 + 导出走 || 兜底右翼', async () => {
    mLevels.mockResolvedValue({} as never);
    mEpisodes.mockResolvedValue({} as never);
    mMaps.mockResolvedValue({} as never);
    renderPage();
    await waitLoad();

    expect(screen.queryByText('分群胜率对比（Top 10 关卡）')).not.toBeInTheDocument();
    const bigCards = cards().slice(0, 4);
    for (const c of bigCards) {
      if (c.querySelector('.ant-table')) {
        expect(c.querySelector('.ant-table .ant-empty-description')).not.toBeNull();
      }
    }

    // 三个导出按钮照常可点（表头行导出）
    fireEvent.click(csvButtons()[0]);
    fireEvent.click(csvButtons()[1]);
    fireEvent.click(csvButtons()[2]);
    await waitFor(() => expect(mCsv).toHaveBeenCalledTimes(1));
    await waitFor(() => expect(mXlsx).toHaveBeenCalledTimes(2));
    expect(mCsv.mock.calls[0][1]).toEqual([
      ['level', 'players', 'win_rate', 'avg_duration_sec', 'avg_retries'],
    ]);
    expect(mXlsx.mock.calls[0][0]).toBe('levels_funnel.csv');
    expect(mXlsx.mock.calls[0][1][0].rows).toEqual([['step', 'users', 'rate']]);

    // 地图分面点「加载」：{} → (r?.maps || []) 右翼 → 空网格（无小卡）
    fireEvent.click(screen.getAllByRole('button', { name: '加载' })[1]);
    await waitFor(() => expect(mMaps).toHaveBeenCalledTimes(1));
    expect(document.querySelectorAll('.ant-card')).toHaveLength(4);
  });
});
