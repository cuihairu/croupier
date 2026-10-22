/**
 * 采样控制（Server 下发）页单测（覆盖率巡检：Ops/AnalyticsFilters/
 * index.tsx 281 行 0% → 收口，零测试页排行第十）。
 *
 * 锁定契约：
 * - 挂载：listGamesMeta 一次；games 映射（gameId=String(name||'')、
 *   label 链 displayName→aliasName→name、envs 数组直取 / envMeta
 *   map(env?.env).filter(Boolean) 回退 / 双缺 → []、gameId 空 Filter 出列）；
 * - 游戏/环境联动：切游戏 → env 清空 + envs 选项重建；canQuery =
 *   gameId && env !== '' 双条件门（未齐时 加载/保存 双按钮 disabled）；
 * - 加载：fetchAnalyticsFilters({gameId, env}) → events||[]、
 *   paymentsEnabled !== false、sampleGlobal ?? 100 三归一；catch → 加载失败；
 * - 保存：saveAnalyticsFilters({gameId, env, events, paymentsEnabled,
 *   sampleGlobal}) → 已保存；catch → 保存失败（需要 analytics:manage 权限）；
 * - 交互面：全局采样 InputNumber（Number(v||0)，清空 → 0）、支付 Switch
 *   （允许上报/禁用（全部丢弃）+ 摘要 Tag 启用/禁用 双态）、事件白名单
 *   tags Select（增 tag → 事件数 N Tag；空 → 全部允许 Tag）；
 * - 摘要条：gameId/env 缺省 '-'、采样 Tag <100 gold / ≥100 green、
 *   支付 Tag 双态、事件数/全部允许 双臂。
 *
 * mock 口径：services/api/analytics 两函数 + services/api/games 的
 * listGamesMeta jest.mock（service 边界，返回已归一 Game 形态）；
 * @umijs/max 本地 defaultMessage mock；antd 真实实现。
 *
 * 登记不可达（防御分支，不造假用例不删分支）：
 * - save 的 `if (!canQuery)` warning 早退体（77-84 行，8 语句）：保存按钮
 *   disabled={!canQuery}，jsdom 对 disabled 按钮不派发 click，未选齐前
 *   不可触达（load 的同位守卫语句在 canQuery=true 调用中恒过、非早退）；
 * - label 链尾 'Unknown' 右臂：Boolean(gameId) 过滤已保证幸存行 name 非空
 *   （String(name||'') 真值 ⇔ name 非空），g.name 恒真、轮不到 'Unknown'；
 * - `(r?.games || [])` 双右臂：listGamesMeta 契约恒返 {games: Game[]}
 *   （real impl 硬保证 Array.isArray 归一），resolve undefined/无 games 形态
 *   违反服务返回类型，构造即造假；
 * - `selectedGame?.envs` 的 ?. 右臂：gameId 只能来自 games 选项（Select
 *   options 即同一数组），选中值恒命中 find；
 * - listGamesMeta 的 catch{} 静默翼：reject 后 games 保持 []，DOM 与
 *   resolve {games:[]} 形态不可区分（无文案无白屏差异），不造假断言。
 *
 * 坑实证（antd6 沿用）：页内三个 .ant-select 按 DOM 序 [游戏, 环境, 事件
 * tags]；rc-select 选项点击须 sleep ≥60ms 再 mouseDown、点
 * .ant-select-item-option-content；Select 选中值有 aria-live 镜像双 DOM
 * （getByText 双命中），按 Select 根 textContent 聚合断言；摘要条的
 * FormattedMessage 与 {gameId||'-'} 是同级文本节点（无独立元素），按
 * .ant-card-body toHaveTextContent 聚合断言；两字中文 Button 自动插空格
 * （加 载 / 保 存），getByRole name 用 /加\s*载/ 正则；InputNumber 直改
 * 内层 input。
 */
import React from 'react';
import { configure, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { App } from 'antd';
import AnalyticsFiltersPage from '../index';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

jest.mock('@/services/api/analytics', () => ({
  fetchAnalyticsFilters: jest.fn(),
  saveAnalyticsFilters: jest.fn(),
}));

jest.mock('@/services/api/games', () => ({
  listGamesMeta: jest.fn(),
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

import { fetchAnalyticsFilters, saveAnalyticsFilters } from '@/services/api/analytics';
import { listGamesMeta } from '@/services/api/games';

const mFetch = fetchAnalyticsFilters as jest.MockedFunction<typeof fetchAnalyticsFilters>;
const mSave = saveAnalyticsFilters as jest.MockedFunction<typeof saveAnalyticsFilters>;
const mGames = listGamesMeta as jest.MockedFunction<typeof listGamesMeta>;

// 覆盖翼：displayName 首选 / aliasName 次选 / name 兜底；envs 直取 /
// envMeta 回退（含空 env 项与 null 条目过滤——env?.env 右臂）/ 双缺 [];
// name 空 → 整行过滤
const GAMES = {
  games: [
    { id: 1, name: 'demo', displayName: 'Demo Game', envs: ['dev', 'prod'] },
    {
      id: 2,
      name: 'alias-only',
      aliasName: '别名优先',
      envMeta: [{ env: 'beta' }, { env: '' }, null],
    },
    { id: 3, name: 'plain' },
    { id: 4, aliasName: '无名牌' },
  ],
};

beforeEach(() => {
  jest.clearAllMocks();
  mGames.mockResolvedValue(GAMES as never);
  mFetch.mockResolvedValue({
    events: ['e1', 'e2'],
    paymentsEnabled: false,
    sampleGlobal: 50,
  });
  mSave.mockResolvedValue(undefined);
});

function renderPage() {
  return render(
    <App>
      <AnalyticsFiltersPage />
    </App>,
  );
}

/** 页内三个 Select 按 DOM 序：[游戏, 环境, 事件 tags] */
function selectAt(idx: number) {
  return document.querySelectorAll('.ant-select')[idx] as HTMLElement;
}

/** 打开下拉并点选可见选项文本 */
async function pickOption(idx: number, label: string) {
  await new Promise((r) => setTimeout(r, 60));
  fireEvent.mouseDown(selectAt(idx));
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

/** 选齐 game+env（canQuery 置真） */
async function selectGameEnv(gameLabel: string, envLabel: string) {
  await pickOption(0, gameLabel);
  await new Promise((r) => setTimeout(r, 120));
  await pickOption(1, envLabel);
  await new Promise((r) => setTimeout(r, 120));
}

describe('采样控制 游戏映射与联动', () => {
  it('挂载映射矩阵：label 三臂 + envs 直取/envMeta 回退/双缺 + 空名过滤', async () => {
    renderPage();
    await waitFor(() => expect(mGames).toHaveBeenCalledTimes(1));

    // 打开游戏下拉：三个幸存项（无名牌被 Boolean(gameId) 过滤）
    await pickOption(0, 'Demo Game');
    // Demo Game 选中；重开看全量选项
    await new Promise((r) => setTimeout(r, 300));
    fireEvent.mouseDown(selectAt(0));
    const dropdown = await waitFor(() => {
      const visible = Array.from(document.querySelectorAll('.ant-select-dropdown')).find(
        (d) => !d.className.includes('ant-select-dropdown-hidden'),
      ) as HTMLElement;
      expect(visible).not.toBeUndefined();
      return visible;
    });
    const labels = Array.from(dropdown.querySelectorAll('.ant-select-item-option-content')).map(
      (o) => o.textContent,
    );
    expect(labels).toEqual(['Demo Game', '别名优先', 'plain']);
  });

  it('未选齐 canQuery 门：加载/保存 disabled；摘要缺省 + 全部允许 + 采样 100 绿 + 支付绿', async () => {
    renderPage();
    await waitFor(() => expect(mGames).toHaveBeenCalledTimes(1));

    expect(screen.getByRole('button', { name: /加\s*载/ })).toBeDisabled();
    expect(screen.getByRole('button', { name: /保\s*存/ })).toBeDisabled();

    // 摘要缺省：当前 - | -（FormattedMessage 与插值是同级文本节点，按
    // 卡片 body 聚合断言）；events 空 → 全部允许；sample 100 绿；支付启用绿
    const body = document.querySelector('.ant-card-body') as HTMLElement;
    expect(body).toHaveTextContent('当前：- | -');
    expect(screen.getByText('全部允许').closest('.ant-tag')).not.toHaveClass('ant-tag-blue');
    expect(screen.getByText('100%').closest('.ant-tag')).toHaveClass('ant-tag-green');
    expect(screen.getByText('启用').closest('.ant-tag')).toHaveClass('ant-tag-green');
    expect(screen.getByText('允许上报')).toBeInTheDocument();
  });

  it('切游戏清空 env + envs 选项重建（envMeta 回退）', async () => {
    renderPage();
    await waitFor(() => expect(mGames).toHaveBeenCalledTimes(1));

    // 选 demo（envs 直取 dev/prod）→ 选 prod
    await selectGameEnv('Demo Game', 'prod');
    expect(screen.getByRole('button', { name: /加\s*载/ })).toBeEnabled();

    // 切到 别名优先（envMeta 回退 → beta；空 env 项与 null 条目被过滤）
    // ——antd6 Select 选中值有 aria-live 镜像双 DOM，按环境 Select 根聚合断言
    await new Promise((r) => setTimeout(r, 300));
    await pickOption(0, '别名优先');
    await new Promise((r) => setTimeout(r, 120));
    await pickOption(1, 'beta');
    await waitFor(() => expect(selectAt(1).textContent).toContain('beta'));

    // 切到 plain（envs 双缺 → selectedGame?.envs || [] 右臂）：env 已被
    // 联动清空 + 环境下拉无选项 + canQuery 回 disabled
    await new Promise((r) => setTimeout(r, 300));
    await pickOption(0, 'plain');
    await new Promise((r) => setTimeout(r, 120));
    await waitFor(() => expect(screen.getByRole('button', { name: /加\s*载/ })).toBeDisabled());
    fireEvent.mouseDown(selectAt(1));
    const envDropdown = await waitFor(() => {
      const visible = Array.from(document.querySelectorAll('.ant-select-dropdown')).find(
        (d) => !d.className.includes('ant-select-dropdown-hidden'),
      ) as HTMLElement;
      expect(visible).not.toBeUndefined();
      return visible;
    });
    expect(envDropdown.querySelectorAll('.ant-select-item-option')).toHaveLength(0);
  });
});

describe('采样控制 加载与保存', () => {
  it('加载主链：载荷 + 三归一（events/支付关/采样 50 gold）+ 摘要事件数', async () => {
    renderPage();
    await waitFor(() => expect(mGames).toHaveBeenCalledTimes(1));
    await selectGameEnv('Demo Game', 'prod');

    fireEvent.click(screen.getByRole('button', { name: /加\s*载/ }));
    await waitFor(() => expect(mFetch).toHaveBeenCalledWith({ gameId: 'demo', env: 'prod' }));

    // events 2 → 事件数 Tag；支付 false → 红 禁用 + 禁用（全部丢弃）文案；
    // sample 50 → gold
    expect(await screen.findByText('· 事件数')).toBeInTheDocument();
    expect(screen.getByText('2').closest('.ant-tag')).toHaveClass('ant-tag-blue');
    expect(screen.getByText('禁用').closest('.ant-tag')).toHaveClass('ant-tag-red');
    expect(screen.getByText('禁用（全部丢弃）')).toBeInTheDocument();
    expect(screen.getByText('50%').closest('.ant-tag')).toHaveClass('ant-tag-gold');
  });

  it('加载缺省归一：{} → events [] / paymentsEnabled true / sampleGlobal 100；失败 → 加载失败', async () => {
    renderPage();
    await waitFor(() => expect(mGames).toHaveBeenCalledTimes(1));
    await selectGameEnv('Demo Game', 'prod');

    mFetch.mockResolvedValueOnce({} as never);
    fireEvent.click(screen.getByRole('button', { name: /加\s*载/ }));
    await waitFor(() => expect(mFetch).toHaveBeenCalledTimes(1));
    await waitFor(() => expect(screen.getByText('允许上报')).toBeInTheDocument());
    expect(screen.getByText('100%').closest('.ant-tag')).toHaveClass('ant-tag-green');
    expect(screen.getByText('全部允许')).toBeInTheDocument();

    mFetch.mockRejectedValueOnce(new Error('down'));
    fireEvent.click(screen.getByRole('button', { name: /加\s*载/ }));
    expect(await screen.findByText('加载失败')).toBeInTheDocument();
  });

  it('保存主链：全字段落库 + 已保存；失败 → 权限文案', async () => {
    renderPage();
    await waitFor(() => expect(mGames).toHaveBeenCalledTimes(1));
    await selectGameEnv('Demo Game', 'prod');

    // 先加载落态，再改采样与支付，保存透传改动后形态
    fireEvent.click(screen.getByRole('button', { name: /加\s*载/ }));
    await waitFor(() => expect(screen.getByText('50%')).toBeInTheDocument());

    fireEvent.click(screen.getByRole('switch'));
    fireEvent.click(screen.getByRole('button', { name: /保\s*存/ }));
    await waitFor(() =>
      expect(mSave).toHaveBeenCalledWith({
        gameId: 'demo',
        env: 'prod',
        events: ['e1', 'e2'],
        paymentsEnabled: true,
        sampleGlobal: 50,
      }),
    );
    expect(await screen.findByText('已保存')).toBeInTheDocument();

    mSave.mockRejectedValueOnce(new Error('denied'));
    fireEvent.click(screen.getByRole('button', { name: /保\s*存/ }));
    expect(await screen.findByText('保存失败（需要 analytics:manage 权限）')).toBeInTheDocument();
  });
});

describe('采样控制 交互面', () => {
  it('采样 InputNumber：改 30 → gold 30%；清空 → 0', async () => {
    renderPage();
    await waitFor(() => expect(mGames).toHaveBeenCalledTimes(1));

    const input = document.querySelector('.ant-input-number input') as HTMLInputElement;
    fireEvent.change(input, { target: { value: '30' } });
    await waitFor(() =>
      expect(screen.getByText('30%').closest('.ant-tag')).toHaveClass('ant-tag-gold'),
    );

    // 清空 → onChange(null) → Number(null || 0) → 0
    fireEvent.change(input, { target: { value: '' } });
    await waitFor(() => expect(screen.getByText('0%')).toBeInTheDocument());
  });

  it('事件 tags Select：增 tag → 事件数 1；支付 Switch 切换双态文案', async () => {
    renderPage();
    await waitFor(() => expect(mGames).toHaveBeenCalledTimes(1));

    const tagsSelect = selectAt(2);
    const input = tagsSelect.querySelector('input') as HTMLInputElement;
    fireEvent.change(input, { target: { value: 'evt.click' } });
    fireEvent.keyDown(input, { key: 'Enter', code: 'Enter', keyCode: 13 });
    expect(await screen.findByText('· 事件数')).toBeInTheDocument();
    expect(screen.getByText('1').closest('.ant-tag')).toHaveClass('ant-tag-blue');

    fireEvent.click(screen.getByRole('switch'));
    expect(await screen.findByText('禁用（全部丢弃）')).toBeInTheDocument();
    expect(screen.getByText('禁用').closest('.ant-tag')).toHaveClass('ant-tag-red');
  });
});
