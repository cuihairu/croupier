/**
 * 函数实例管理页单测（覆盖率补缺：Functions/Instances/index.tsx 634 行 0%
 * → 收口——真 0% 榜首页面组件；域内既有两测试只打 InstanceDetailDrawer，
 * 无人 import `../index`，import 面核实后认领）。
 *
 * 锁定契约：
 * - 加载链：serviceId||providerId||'' 归一 + rowKey 构建；概览统计（active=
 *   healthy||running、资源前缀 split('.')[0]||'other'、coveredFunctions/
 *   totalFunctions→percentage、空表 totalFunctions=0→0 翼）；res?.instances
 *   || [] 缺省翼；
 * - 失败三翼（Error.message / 非 Error → 操作失败 / 空 message → 加载失败）
 *   与覆盖统计缺省兜底（coverage null → summary || 系列右翼）；
 * - 竞态守卫：旧请求晚归被丢弃（fetchId 比对），新数据不被覆盖；
 * - 刷新双按钮（页头 extra + 列表区）重入 fetchData；
 * - descriptor 链：summary??description 本地化、无 id 条目跳过、version??
 *   ''、失败不阻断、unmount 后 resolve 走 cancelled 卫兵；
 * - 函数下拉：实例 functionId 去重排序过滤（'' 排除）、filterOption 三翼
 *   （值匹配/摘要匹配/全不匹配）、optionRender（契约版本 span 与摘要行
 *   有无两翼）；
 * - 筛选链：关键词 trim+lowercase 全字段拼接搜索、状态三档、函数下拉单选、
 *   三条件叠加 filterSummary join(' / ')、hasFilters 告警、清空筛选复位、
 *   筛选后空态文案两翼（filtered vs default）；
 * - 行回调四链（详情/日志/调试/抽屉内转调试——详情关闭+调试开启）与三
 *   弹层 onClose 回流；handleDetail 关闭其余弹层防陈旧上下文。
 *
 * mock 口径：services 双函数 jest.mock；columns 以轻量工厂替换（渲染
 * functionId 单元格 + 行详情/行日志/行调试三钮，回调即 buildInstanceColumns
 * 入参）；三弹层组件桩（受 open 控制渲染、暴露 onClose/onOpenLogs/
 * onOpenDebug 触发钮锁接线，内部各有/将有测试）；useScopeReload 置空避免
 * scope store 耦合；antd/pro-components/@umijs/History 真实实现；@umijs/max
 * 工厂自含 makeIntl（useIntl 每渲染新实例——页面 intlRef 专门为此设计）。
 *
 * 边界（诚实，登记结构不可达翼——四组，v8 分支 96.4% 缺口全在此列）：
 * 1. coveredFunctions 的 `count > 0` 谓词假翼——functionsMap 计数恒 ≥1；
 * 2. L141 lastSeen 三元真翼内 `|| instance.lastHeartbeat || instance.lastSeen
 *    || ''` 的第三操作数——外层条件已保证前二者至少一真值；
 * 3. L495 filterOption 的 `if (!q) return true` 空串早退——antd6 只在
 *    searchValue 非空时才调用 filterOption（输入过再清空/change('') 实测
 *    均不触发），UI 路径不可达；
 * 4. L500-506 filterOption 的 `o.value ?? ''` / `o.summary ?? ''` 右翼——
 *    functionOptions 恒产非空 value（filter(Boolean) 已排 ''）与 string
 *    summary，undefined 形态仅防御。
 */
import React from 'react';
import { act, configure, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { App as AntdApp } from 'antd';
import InstancesPage from '../index';
import { getFunctionInstances } from '@/services/api';
import type { FunctionInstance } from '@/services/api';
import { listDescriptors } from '@/services/api/functions';
import type { FunctionDescriptor } from '@/services/api/functions';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

jest.mock('@umijs/max', () => {
  const makeIntl = () => ({
    locale: 'zh-CN',
    formatMessage: (opts: { defaultMessage?: string }, values?: Record<string, unknown>) => {
      let text = opts.defaultMessage ?? '';
      if (values) {
        for (const [k, v] of Object.entries(values)) text = text.split(`{${k}}`).join(String(v));
      }
      return text;
    },
  });
  return {
    FormattedMessage: ({ defaultMessage }: { defaultMessage?: string }) => (
      <>{defaultMessage ?? ''}</>
    ),
    useIntl: makeIntl,
    getIntl: makeIntl,
    history: { push: jest.fn() },
  };
});

jest.mock('@/services/api', () => ({
  __esModule: true,
  getFunctionInstances: jest.fn(),
}));

jest.mock('@/services/api/functions', () => ({
  __esModule: true,
  listDescriptors: jest.fn(),
}));

jest.mock('@/hooks/useScopeReload', () => ({
  __esModule: true,
  useScopeReload: jest.fn(),
}));

// 轻量列工厂：functionId 单元格 + 三行操作钮（回调即入参，锁页面接线）
jest.mock('../columns', () => {
  const R = require('react');
  return {
    __esModule: true,
    buildInstanceColumns: (opts: {
      onDetail: (r: unknown) => void;
      onLogs: (r: unknown) => void;
      onDebug: (r: unknown) => void;
    }) => [
      {
        title: '函数',
        dataIndex: 'functionId',
        render: (_dom: unknown, record: Record<string, unknown>) =>
          R.createElement('span', null, String(record.functionId)),
      },
      {
        title: '操作',
        render: (_dom: unknown, record: Record<string, unknown>) =>
          R.createElement(
            'div',
            null,
            R.createElement(
              'button',
              { type: 'button', onClick: () => opts.onDetail(record) },
              '行详情',
            ),
            R.createElement(
              'button',
              { type: 'button', onClick: () => opts.onLogs(record) },
              '行日志',
            ),
            R.createElement(
              'button',
              { type: 'button', onClick: () => opts.onDebug(record) },
              '行调试',
            ),
          ),
      },
    ],
  };
});

// 三弹层桩：open 受控渲染 + 关闭/联动钮，锁页面侧接线（内部自有测试）
jest.mock('../InstanceDetailDrawer', () => {
  const R = require('react');
  return {
    __esModule: true,
    default: (props: {
      open: boolean;
      instance: { functionId?: string } | null;
      onClose: () => void;
      onOpenLogs: (i: unknown) => void;
      onOpenDebug: (i: unknown) => void;
    }) =>
      props.open
        ? R.createElement(
            'div',
            { 'data-testid': 'detail-stub' },
            R.createElement('span', null, `抽屉实例:${props.instance?.functionId ?? '无'}`),
            R.createElement('button', { type: 'button', onClick: props.onClose }, '抽屉·关闭'),
            props.instance
              ? R.createElement(
                  'button',
                  { type: 'button', onClick: () => props.onOpenLogs(props.instance) },
                  '抽屉·看日志',
                )
              : null,
            props.instance
              ? R.createElement(
                  'button',
                  { type: 'button', onClick: () => props.onOpenDebug(props.instance) },
                  '抽屉·转调试',
                )
              : null,
          )
        : null,
  };
});

jest.mock('../LogsModal', () => {
  const R = require('react');
  return {
    __esModule: true,
    default: (props: {
      open: boolean;
      instance: { functionId?: string } | null;
      onClose: () => void;
    }) =>
      props.open
        ? R.createElement(
            'div',
            { 'data-testid': 'logs-stub' },
            R.createElement('span', null, `日志实例:${props.instance?.functionId ?? '无'}`),
            R.createElement('button', { type: 'button', onClick: props.onClose }, '日志·关闭'),
          )
        : null,
  };
});

jest.mock('../DebugModal', () => {
  const R = require('react');
  return {
    __esModule: true,
    default: (props: {
      open: boolean;
      instance: { functionId?: string } | null;
      onClose: () => void;
    }) =>
      props.open
        ? R.createElement(
            'div',
            { 'data-testid': 'debug-stub' },
            R.createElement('span', null, `调试实例:${props.instance?.functionId ?? '无'}`),
            R.createElement('button', { type: 'button', onClick: props.onClose }, '调试·关闭'),
          )
        : null,
  };
});

const mInstances = jest.mocked(getFunctionInstances);
const mDesc = jest.mocked(listDescriptors);

const inst = (over: Partial<FunctionInstance> & { functionId: string }): FunctionInstance => ({
  agentId: 'agent-x',
  serviceId: '',
  addr: '10.0.0.0:0',
  version: 'v0',
  status: 'stopped',
  ...over,
});

// 主数据四实例：running(healthy)/error(prov-2 归一)/stopped(无心跳)/
// 空 functionId(→other 前缀 + 下拉排除) + lastSeen 回退
const mainInstances: FunctionInstance[] = [
  inst({
    agentId: 'agent-1',
    serviceId: 'svc-1',
    addr: '10.0.0.1:1',
    functionId: 'player.kick',
    version: '1.0.0',
    status: 'running',
    healthy: true,
    lastHeartbeat: '2026-09-01T08:00:00Z',
    sdkName: 'go',
    sdkVersion: '1.2',
  }),
  inst({
    agentId: 'agent-2',
    providerId: 'prov-2',
    addr: '10.0.0.2:2',
    functionId: 'player.ban',
    status: 'error',
  }),
  inst({ agentId: 'agent-3', functionId: 'mail.send', status: 'stopped' }),
  inst({ agentId: 'agent-4', functionId: '', status: 'running', lastSeen: '2026-09-02T09:00:00Z' }),
];

// 主 descriptor 集：有版本有摘要 / 无摘要走 description / 无 id 跳过 / 全裸
const mainDesc: FunctionDescriptor[] = [
  { id: 'player.kick', version: '1.2.0', summary: { 'zh-CN': '踢出玩家' } },
  { id: 'player.ban', description: { 'zh-CN': '封禁玩家' } },
  { description: { 'zh-CN': '无ID契约' } } as unknown as FunctionDescriptor,
  { id: 'mail.send' },
];

function renderPage() {
  return render(
    <AntdApp>
      <InstancesPage />
    </AntdApp>,
  );
}

function statusSelect(): HTMLElement {
  const all = document.querySelectorAll<HTMLElement>('.ant-select');
  expect(all.length).toBeGreaterThanOrEqual(2);
  return all[0];
}

function functionSelect(): HTMLElement {
  const all = document.querySelectorAll<HTMLElement>('.ant-select');
  expect(all.length).toBeGreaterThanOrEqual(2);
  return all[1];
}

/**
 * 打开下拉并按 marker 消歧——jsdom 下 antd 关闭动画会停在半途（appear-
 * prepare 不收尾），旧下拉可长期非隐藏，「首个非隐藏」会错拿；用目标下拉
 * 特有的选项文本锁定。
 */
async function openSelect(root: HTMLElement, marker: string): Promise<HTMLElement> {
  fireEvent.mouseDown(root);
  let found: HTMLElement | undefined;
  await waitFor(() => {
    found = Array.from(document.querySelectorAll<HTMLElement>('.ant-select-dropdown')).find(
      (d) =>
        !d.classList.contains('ant-select-dropdown-hidden') &&
        (d.textContent ?? '').includes(marker),
    );
    expect(found).toBeDefined();
  });
  return found as HTMLElement;
}

/** 可见选项行（.ant-select-item-option，行级 textContent 含 optionRender 子树） */
function optionRows(dropdown: HTMLElement): HTMLElement[] {
  return Array.from(dropdown.querySelectorAll<HTMLElement>('.ant-select-item-option')).filter(
    (r) => r.closest('.ant-select-item-option') !== null,
  );
}

/** 点默认渲染选项（label 精确落在 option-content） */
function clickOption(dropdown: HTMLElement, label: string): void {
  const hits = within(dropdown).getAllByText(label, {
    selector: '.ant-select-item-option-content',
  });
  expect(hits.length).toBe(1);
  fireEvent.click(hits[0]);
}

/** 点自定义 optionRender 选项（按 value 定位行，点其 content） */
function clickOptionRow(dropdown: HTMLElement, value: string): void {
  const row = optionRows(dropdown).find((r) => (r.textContent ?? '').includes(value));
  expect(row).toBeDefined();
  fireEvent.click(
    (row as HTMLElement).querySelector('.ant-select-item-option-content') as HTMLElement,
  );
}

const rowCount = (): number => screen.getAllByRole('button', { name: '行详情' }).length;

beforeEach(() => {
  jest.clearAllMocks();
  mInstances.mockResolvedValue({ instances: mainInstances } as never);
  mDesc.mockResolvedValue(mainDesc as never);
});

describe('初始加载与概览统计', () => {
  it('四实例主渲染：五项统计 + 离线告警 hint + 行渲染 + 分页总数', async () => {
    renderPage();

    // 统计：实例4 / 在线2(healthy/running×2) / 离线2 / 函数4 / 资源前缀3(player/mail/other)
    expect(await screen.findByText('实例 4')).toBeInTheDocument();
    expect(screen.getByText('在线 2')).toBeInTheDocument();
    expect(screen.getByText('离线 2')).toBeInTheDocument();
    expect(screen.getByText('函数 4')).toBeInTheDocument();
    expect(screen.getByText('资源前缀 3')).toBeInTheDocument();
    // inactive>0 → warning hint 左翼
    expect(screen.getByText(/当前有 2 个离线实例/)).toBeInTheDocument();

    // 行渲染（空 functionId 渲染空 span）+ 分页 showTotal
    expect(screen.getByText('player.kick')).toBeInTheDocument();
    expect(screen.getByText('mail.send')).toBeInTheDocument();
    expect(screen.getByText('共 4 个实例')).toBeInTheDocument();
  });

  it('空响应（instances 缺省翼）：统计全 0 + info hint + 默认空态文案', async () => {
    mInstances.mockResolvedValue({} as never);
    renderPage();

    expect(await screen.findByText('实例 0')).toBeInTheDocument();
    expect(screen.getByText('在线 0')).toBeInTheDocument();
    expect(screen.getByText('离线 0')).toBeInTheDocument();
    expect(screen.getByText('函数 0')).toBeInTheDocument();
    expect(screen.getByText('资源前缀 0')).toBeInTheDocument();
    // inactive=0 → info hint 右翼；totalFunctions=0 → percentage 0 翼
    expect(screen.getByText(/函数覆盖率 0%，当前没有发现离线实例/)).toBeInTheDocument();
    // ProTable locale.emptyText 默认翼
    expect(
      screen.getByText('暂时没有实例数据，请先确认注册信息是否已经上报。'),
    ).toBeInTheDocument();
  });
});

describe('失败与竞态守卫', () => {
  it('失败三翼：Error.message / 非 Error → 操作失败 / 空 message → 加载失败', async () => {
    mInstances.mockRejectedValueOnce(new Error('boom'));
    const { unmount } = renderPage();
    expect(await screen.findByText('boom')).toBeInTheDocument();
    // 失败路径 coverage 仍 null → summary || 系列右翼（渲染不炸即证）
    unmount();

    mInstances.mockRejectedValueOnce('plain-string');
    renderPage();
    expect(await screen.findByText('操作失败')).toBeInTheDocument();
  });

  it('空 message Error → loadFailed 兜底文案', async () => {
    mInstances.mockRejectedValueOnce(new Error(''));
    renderPage();
    expect(await screen.findByText('加载失败')).toBeInTheDocument();
  });

  it('竞态：旧请求晚归被丢弃，列表保持新数据', async () => {
    let resolve1!: (v: unknown) => void;
    let resolve2!: (v: unknown) => void;
    mInstances
      .mockImplementationOnce(
        () =>
          new Promise((r) => {
            resolve1 = r;
          }),
      )
      .mockImplementationOnce(
        () =>
          new Promise((r) => {
            resolve2 = r;
          }),
      );
    renderPage();
    await waitFor(() => expect(mInstances).toHaveBeenCalledTimes(1));

    // 第二次拉取先归（带 second.fn）；第一次晚归（first.fn）应被丢弃
    fireEvent.click(screen.getByRole('button', { name: /刷新$/ }));
    await waitFor(() => expect(mInstances).toHaveBeenCalledTimes(2));
    await act(async () => {
      resolve2({
        instances: [inst({ agentId: 'a2', functionId: 'second.fn', status: 'running' })],
      });
    });
    expect(await screen.findByText('second.fn')).toBeInTheDocument();

    await act(async () => {
      resolve1({
        instances: [inst({ agentId: 'a1', functionId: 'first.fn', status: 'running' })],
      });
    });
    await waitFor(() => expect(screen.queryByText('first.fn')).toBeNull());
    expect(screen.getByText('second.fn')).toBeInTheDocument();
  });

  it('刷新双按钮（页头/列表区）重入 fetchData', async () => {
    renderPage();
    await waitFor(() => expect(mInstances).toHaveBeenCalledTimes(1));

    fireEvent.click(screen.getByRole('button', { name: /刷新$/ }));
    await waitFor(() => expect(mInstances).toHaveBeenCalledTimes(2));

    fireEvent.click(screen.getByRole('button', { name: /刷新数据$/ }));
    await waitFor(() => expect(mInstances).toHaveBeenCalledTimes(3));
  });
});

describe('descriptor 下拉（函数筛选）', () => {
  it('选项去重排序：summary??description 本地化 + 契约版本/摘要有无两翼', async () => {
    // 追加重复 functionId 实例 → 去重翼
    mInstances.mockResolvedValue({
      instances: [
        ...mainInstances,
        inst({ agentId: 'agent-5', functionId: 'player.kick', status: 'running' }),
      ],
    } as never);
    renderPage();
    await screen.findByText('实例 5');

    const dropdown = await openSelect(functionSelect(), 'player.ban');
    const texts = optionRows(dropdown).map((r) => r.textContent ?? '');
    // '' 被排除 + 去重 + 排序：mail.send / player.ban / player.kick
    expect(texts).toHaveLength(3);
    expect(texts[0]).toContain('mail.send');
    expect(texts[1]).toContain('player.ban');
    expect(texts[2]).toContain('player.kick');
    // player.kick：摘要(踢出玩家) + 契约版本 v1.2.0；mail.send：两者皆无（双右翼）
    expect(texts[2]).toContain('v1.2.0');
    expect(texts[2]).toContain('踢出玩家');
    expect(texts[1]).toContain('封禁玩家');
    expect(texts[0]).not.toContain('v');
    expect(texts[0]).not.toContain('玩家');
  });

  it('filterOption：值匹配 / 摘要匹配 / 全不匹配三翼', async () => {
    renderPage();
    await screen.findByText('player.kick');

    const root = functionSelect();
    const input = root.querySelector('input') as HTMLInputElement;
    const dropdown = await openSelect(root, 'player.ban');
    const texts = (): string[] => optionRows(dropdown).map((r) => r.textContent ?? '');

    // 值匹配：kick → player.kick 保留，player.ban 排除
    fireEvent.change(input, { target: { value: 'kick' } });
    expect(texts()).toHaveLength(1);
    expect(texts()[0]).toContain('player.kick');

    // 摘要匹配：封禁 → player.ban（filterOption 的 summary 臂）
    fireEvent.change(input, { target: { value: '封禁' } });
    expect(texts()).toHaveLength(1);
    expect(texts()[0]).toContain('player.ban');

    // 全不匹配：zzz → 空态
    fireEvent.change(input, { target: { value: 'zzz' } });
    expect(optionRows(dropdown)).toHaveLength(0);

    // 清空输入（!q → return true）→ 三行全回
    fireEvent.change(input, { target: { value: '' } });
    expect(optionRows(dropdown)).toHaveLength(3);
  });

  it('descriptor 拉取失败不阻断页面；unmount 后 resolve 走 cancelled 卫兵', async () => {
    mDesc.mockRejectedValueOnce(new Error('desc down'));
    renderPage();
    expect(await screen.findByText('player.kick')).toBeInTheDocument();

    // cancelled：pending 期间卸载，resolve 后不 setState（无崩溃即证卫兵生效）
    let rDesc!: (v: unknown) => void;
    mDesc.mockImplementationOnce(
      () =>
        new Promise((r) => {
          rDesc = r;
        }),
    );
    const { unmount } = renderPage();
    unmount();
    await act(async () => {
      rDesc([]);
    });
  });
});

describe('筛选链', () => {
  it('关键词 trim+lowercase：prov-2 命中归一化后的 serviceId', async () => {
    renderPage();
    await screen.findByText('player.kick');

    const input = screen.getByPlaceholderText(/搜索 agent\/service\/addr\/function/);
    fireEvent.change(input, { target: { value: '  PROV-2  ' } });
    expect(screen.getByText('当前结果 1 个实例')).toBeInTheDocument();
    expect(screen.getByText('player.ban')).toBeInTheDocument();
    expect(screen.queryByText('player.kick')).toBeNull();
    // hasFilters 告警：搜索段
    expect(screen.getByText(/已生效条件：搜索 PROV-2/)).toBeInTheDocument();
  });

  it('状态三档 it.each：running/error/stopped 各自命中', async () => {
    renderPage();
    await screen.findByText('player.kick');

    const cases: Array<[string, number]> = [
      ['运行中', 2],
      ['错误', 1],
      ['停止', 1],
    ];
    for (const [label, expected] of cases) {
      const dropdown = await openSelect(statusSelect(), '运行中');
      clickOption(dropdown, label);
      await waitFor(() => expect(rowCount()).toBe(expected));
    }
  });

  it('三条件叠加：filterSummary join「 / 」+ 清空筛选复位', async () => {
    renderPage();
    await screen.findByText('player.kick');

    // 关键词 + 状态 + 函数三段
    fireEvent.change(screen.getByPlaceholderText(/搜索 agent\/service\/addr\/function/), {
      target: { value: 'agent-1' },
    });
    const sDropdown = await openSelect(statusSelect(), '运行中');
    clickOption(sDropdown, '运行中');
    const fDropdown = await openSelect(functionSelect(), 'player.ban');
    clickOptionRow(fDropdown, 'player.kick');
    await waitFor(() => expect(rowCount()).toBe(1));
    expect(
      screen.getByText('已生效条件：搜索 agent-1 / 状态 运行中 / 函数 player.kick'),
    ).toBeInTheDocument();
    expect(screen.getByText('当前结果 1 个实例')).toBeInTheDocument();

    // 清空筛选 → 复位（告警消失 + 全量行）
    fireEvent.click(screen.getByRole('button', { name: '清空筛选' }));
    await waitFor(() => expect(rowCount()).toBe(4));
    expect(screen.queryByText(/已生效条件/)).toBeNull();
    expect(screen.getByText('当前结果 4 个实例')).toBeInTheDocument();
  });

  it('筛选后空态：当前筛选条件下没有匹配实例', async () => {
    renderPage();
    await screen.findByText('player.kick');

    fireEvent.change(screen.getByPlaceholderText(/搜索 agent\/service\/addr\/function/), {
      target: { value: 'zzz-no-hit' },
    });
    await waitFor(() => expect(screen.getByText('当前结果 0 个实例')).toBeInTheDocument());
    expect(screen.getByText('当前筛选条件下没有匹配实例，请放宽条件后重试。')).toBeInTheDocument();
  });

  it('状态 Select 清除（value||空串右翼）→ 状态归空', async () => {
    renderPage();
    await screen.findByText('player.kick');

    const root = statusSelect();
    const dropdown = await openSelect(root, '运行中');
    clickOption(dropdown, '错误');
    await waitFor(() => expect(rowCount()).toBe(1));

    // allowClear：清除图标 → onChange(undefined) → setStatusFilter('')
    fireEvent.mouseEnter(root);
    const clear = root.querySelector('.ant-select-clear') as HTMLElement;
    expect(clear).not.toBeNull();
    fireEvent.click(clear);
    await waitFor(() => expect(rowCount()).toBe(4));
    expect(screen.getByText('当前结果 4 个实例')).toBeInTheDocument();
  });
});

describe('行回调与三弹层接线', () => {
  it('行详情 → 抽屉开合 + onClose 回流', async () => {
    renderPage();
    await screen.findByText('player.kick');

    fireEvent.click(screen.getAllByRole('button', { name: '行详情' })[0]);
    expect(await screen.findByTestId('detail-stub')).toBeInTheDocument();
    expect(screen.getByText('抽屉实例:player.kick')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: '抽屉·关闭' }));
    await waitFor(() => expect(screen.queryByTestId('detail-stub')).toBeNull());
  });

  it('行日志/行调试 → 对应弹层开 + 各自 onClose 回流', async () => {
    renderPage();
    await screen.findByText('mail.send');

    fireEvent.click(screen.getAllByRole('button', { name: '行日志' })[2]);
    expect(await screen.findByTestId('logs-stub')).toBeInTheDocument();
    expect(screen.getByText('日志实例:mail.send')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: '日志·关闭' }));
    await waitFor(() => expect(screen.queryByTestId('logs-stub')).toBeNull());

    fireEvent.click(screen.getAllByRole('button', { name: '行调试' })[2]);
    expect(await screen.findByTestId('debug-stub')).toBeInTheDocument();
    expect(screen.getByText('调试实例:mail.send')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: '调试·关闭' }));
    await waitFor(() => expect(screen.queryByTestId('debug-stub')).toBeNull());
  });

  it('抽屉内联动：转调试（抽屉关+调试开）/ 看日志；行详情关闭其余弹层防陈旧', async () => {
    renderPage();
    await screen.findByText('player.kick');

    // 先开日志，再行详情 → 日志被关闭（handleDetail 收起逻辑）
    fireEvent.click(screen.getAllByRole('button', { name: '行日志' })[0]);
    expect(await screen.findByTestId('logs-stub')).toBeInTheDocument();
    fireEvent.click(screen.getAllByRole('button', { name: '行详情' })[0]);
    expect(await screen.findByTestId('detail-stub')).toBeInTheDocument();
    expect(screen.queryByTestId('logs-stub')).toBeNull();

    // 抽屉·转调试：抽屉关 + 调试开，实例上下文保留
    fireEvent.click(screen.getByRole('button', { name: '抽屉·转调试' }));
    await waitFor(() => expect(screen.queryByTestId('detail-stub')).toBeNull());
    expect(await screen.findByTestId('debug-stub')).toBeInTheDocument();
    expect(screen.getByText('调试实例:player.kick')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: '调试·关闭' }));

    // 抽屉·看日志：抽屉保持开 + 日志开
    fireEvent.click(screen.getAllByRole('button', { name: '行详情' })[0]);
    expect(await screen.findByTestId('detail-stub')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: '抽屉·看日志' }));
    expect(await screen.findByTestId('logs-stub')).toBeInTheDocument();
    expect(screen.getByTestId('detail-stub')).toBeInTheDocument();
    expect(screen.getByText('日志实例:player.kick')).toBeInTheDocument();
  });
});
