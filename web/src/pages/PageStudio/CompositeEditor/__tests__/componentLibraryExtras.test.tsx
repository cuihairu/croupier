/** ComponentLibrary 补缺口（既有 componentLibrary.test.ts 纯函数 +
 * componentLibraryPanel.test.tsx 入口/缩略图之外）：
 * - 纯函数：内部引用全命中重映射（onClick target/chain 步骤/assignment/
 *   rowActions/嵌套 children preassign）、无 title 悬空兜底类型名、
 *   tree 缺省防御、reconnect default 分支与 prop 未命中/节点未命中/
 *   嵌套 children 树、chain 步骤命中替换与不匹配保留；
 * - UI：分组三态（显式 category/内置/自定义）与同组聚合、builtin/stale
 *   Tag、description 渲染、缺函数提示与点击拦截、带参模板 onInsert([],tpl)、
 *   普通模板点击实例化（含悬空上报 nodeTitle 兜底）、搜索过滤三源
 *   （name/category/key）与无命中、loading 态、fetch 失败与 items 缺省
 *   空态、数组响应形态、isDragging 半透明态。 */
import React from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App } from 'antd';
import { request } from '@umijs/max';
import ComponentLibrary, {
  instantiateTemplateDetailed,
  reconnectTemplateRefs,
  type ComponentTemplateDTO,
  type TemplateRefFix,
} from '../ComponentLibrary';
import type { PageNode } from '../model';

jest.setTimeout(20000);

// useDraggable 可控桩：isDragging 由用例切换（覆盖半透明分支），其余真渲染
jest.mock('@dnd-kit/core', () => {
  const actual = jest.requireActual('@dnd-kit/core');
  const state = { isDragging: false };
  const useDraggable = () => ({
    attributes: {},
    listeners: {},
    setNodeRef: () => undefined,
    isDragging: state.isDragging,
  });
  return { __esModule: true, ...actual, useDraggable, __draggableState: state };
});

const mockedRequest = request as unknown as jest.Mock;
const draggableState = (
  jest.requireMock('@dnd-kit/core') as {
    __draggableState: { isDragging: boolean };
  }
).__draggableState;

const baseTpl = (key: string, extra: Partial<ComponentTemplateDTO> = {}): ComponentTemplateDTO => ({
  key,
  name: { 'zh-CN': `名称-${key}` },
  tree: [],
  builtin: false,
  ...extra,
});

beforeEach(() => {
  draggableState.isDragging = false;
});

// ---------------------------------------------------------------------------
// 纯函数补缺口
// ---------------------------------------------------------------------------
describe('instantiateTemplateDetailed：内部引用全命中重映射矩阵', () => {
  /** 模板：button 的 onClick 主动作/链步骤均指向模板内节点；表格的
   * refreshOnNode/inputAssignments/rowActions 全部内部引用；modal 嵌套
   * children（preassign 递归 + clone children 重映射）。 */
  const innerTpl = (): ComponentTemplateDTO => ({
    key: 'inner-refs',
    name: { 'zh-CN': '内部引用' },
    builtin: false,
    tree: [
      {
        id: 'btn-a',
        type: 'button',
        props: {
          title: 'A',
          onClick: {
            kind: 'openModal',
            target: 'm-in',
            // 第 1 步无 target（continue 分支）；第 2 步命中重映射
            chain: [{ kind: 'refreshNode' }, { kind: 'refreshNode', target: 't-in' }],
          },
        },
      },
      {
        id: 't-in',
        type: 'fnTable',
        props: {
          functionId: 'f.list',
          refreshOnNode: ['btn-a'],
          // [0] 无 sourceNodeId 原样展开；[1] 命中重映射
          inputAssignments: [
            { param: '/plain' },
            { param: '/self', kind: 'page_state', sourceNodeId: 't-in' },
          ],
          // [0] targetSection 命中重映射；[1] 无 targetSection 原样保留
          rowActions: [{ label: 'x', targetSection: 'm-in' }, { label: 'no-target' }],
        },
      },
      {
        id: 'm-in',
        type: 'modal',
        props: { title: 'M' },
        children: [{ id: 'f-in', type: 'fnForm', props: { functionId: 'f.send' } }],
      },
    ],
  });

  it('onClick 主动作与链步骤、refreshOnNode、assignment、rowActions、嵌套 children 全部重映射到新 id', () => {
    const { nodes, dangling } = instantiateTemplateDetailed(innerTpl());
    expect(dangling).toHaveLength(0); // 全内部引用 → 无悬空
    const btn = nodes.find((n) => n.id.startsWith('button'))!;
    const table = nodes.find((n) => n.id.startsWith('fnTable'))!;
    const modal = nodes.find((n) => n.id.startsWith('modal'))!;
    const onClick = btn.props.onClick as {
      target: string;
      chain: Array<{ kind?: string; target?: string }>;
    };
    expect(onClick.target).toBe(modal.id); // 主动作命中重映射
    expect(onClick.chain[0]).toEqual({ kind: 'refreshNode' }); // 无 target 原样
    expect(onClick.chain[1].target).toBe(table.id); // 链步骤命中重映射
    expect(table.props.refreshOnNode).toEqual([btn.id]);
    const assigns = table.props.inputAssignments as Array<{ param: string; sourceNodeId?: string }>;
    expect(assigns[0]).toEqual({ param: '/plain' });
    expect(assigns[1].sourceNodeId).toBe(table.id);
    const actions = table.props.rowActions as Array<{ label: string; targetSection?: string }>;
    expect(actions[0].targetSection).toBe(modal.id);
    expect(actions[1]).toEqual({ label: 'no-target' });
    // 嵌套 children：preassign 递归分配 + clone 重映射（modal 表单新 id）
    const form = modal.children![0];
    expect(form.id).not.toBe('f-in');
    expect(form.id.startsWith('fnForm')).toBe(true);
  });

  it('悬空上报的 nodeTitle 兜底组件类型名（节点无 title）', () => {
    const { dangling } = instantiateTemplateDetailed(
      baseTpl('no-title', {
        tree: [
          {
            id: 'plain',
            type: 'button',
            props: { onClick: { kind: 'openModal', target: 'nowhere' } },
          },
        ],
      }),
    );
    expect(dangling).toHaveLength(1);
    expect(dangling[0].nodeTitle).toBe('button');
    expect(dangling[0].prop).toBe('onClick');
    expect(dangling[0].kind).toBe('action');
  });

  it('tree 缺省防御：?? [] 兜底返回空 nodes', () => {
    const { nodes, dangling } = instantiateTemplateDetailed({
      ...baseTpl('no-tree'),
      tree: undefined as unknown as PageNode[],
    });
    expect(nodes).toEqual([]);
    expect(dangling).toEqual([]);
  });
});

describe('reconnectTemplateRefs：防御与嵌套分支', () => {
  const tree = (): PageNode[] => [
    {
      id: 'root-btn',
      type: 'button',
      props: {
        title: 'R',
        onClick: { kind: 'openModal', target: 'outside-x', chain: [{ target: 'outside-y' }] },
      },
    },
    {
      id: 'root-modal',
      type: 'modal',
      props: { title: 'M' },
      children: [{ id: 'inner-form', type: 'fnForm', props: { functionId: 'f' } }],
    },
  ];

  it('action 修复：主动作与链步骤均按旧值替换', () => {
    const fixed = reconnectTemplateRefs(tree(), [
      {
        nodeId: 'root-btn',
        kind: 'action',
        prop: 'onClick',
        ref: 'outside-x',
        target: 'root-modal',
      },
      {
        nodeId: 'root-btn',
        kind: 'action',
        prop: 'onClick',
        ref: 'outside-y',
        target: 'inner-form',
      },
    ]);
    const onClick = fixed[0].props.onClick as { target: string; chain: Array<{ target: string }> };
    expect(onClick.target).toBe('root-modal');
    expect(onClick.chain[0].target).toBe('inner-form');
  });

  it('action 修复旧值不匹配：引用保持（ref!==target 分支）', () => {
    const fixed = reconnectTemplateRefs(tree(), [
      {
        nodeId: 'root-btn',
        kind: 'action',
        prop: 'onClick',
        ref: 'not-match',
        target: 'root-modal',
      },
    ]);
    const onClick = fixed[0].props.onClick as { target: string; chain: Array<{ target: string }> };
    expect(onClick.target).toBe('outside-x');
    expect(onClick.chain[0].target).toBe('outside-y');
  });

  it('未知 kind 走 default 分支：值原样写回（不变）', () => {
    const fixed = reconnectTemplateRefs(tree(), [
      {
        nodeId: 'root-btn',
        kind: 'bogus' as unknown as TemplateRefFix['kind'],
        prop: 'onClick',
        ref: 'outside-x',
        target: 'root-modal',
      },
    ]);
    expect(fixed[0].props.onClick).toEqual({
      kind: 'openModal',
      target: 'outside-x',
      chain: [{ target: 'outside-y' }],
    });
  });

  it('fix.prop 不在节点 props：跳过不炸；fix.nodeId 不命中任何节点：原引用返回', () => {
    const src = tree();
    const fixed = reconnectTemplateRefs(src, [
      // prop 未命中（节点没有 refreshOnNode）
      {
        nodeId: 'root-btn',
        kind: 'refresh',
        prop: 'refreshOnNode',
        ref: 'a',
        target: 'root-modal',
      },
      // nodeId 未命中（画布无此节点）
      {
        nodeId: 'ghost-node',
        kind: 'action',
        prop: 'onClick',
        ref: 'outside-x',
        target: 'root-modal',
      },
    ]);
    expect(fixed[0].props.onClick).toEqual((src[0].props as { onClick: unknown }).onClick);
    expect(fixed[0].props).not.toHaveProperty('refreshOnNode');
  });

  it('嵌套 children 树：子节点命中修复（children walk 分支）', () => {
    const src: PageNode[] = [
      {
        id: 'box',
        type: 'container',
        props: { title: 'B' },
        children: [
          { id: 'leaf', type: 'fnTable', props: { functionId: 'f', refreshOnNode: ['old-ref'] } },
        ],
      },
    ];
    const fixed = reconnectTemplateRefs(src, [
      { nodeId: 'leaf', kind: 'refresh', prop: 'refreshOnNode', ref: 'old-ref', target: 'box' },
    ]);
    expect((fixed[0].children![0].props as { refreshOnNode: string[] }).refreshOnNode).toEqual([
      'box',
    ]);
    // 纯函数：入参未改
    expect((src[0].children![0].props as { refreshOnNode: string[] }).refreshOnNode).toEqual([
      'old-ref',
    ]);
  });

  it('无 children 节点命中修复：{...node, props} 分支', () => {
    const fixed = reconnectTemplateRefs(tree(), [
      {
        nodeId: 'root-btn',
        kind: 'action',
        prop: 'onClick',
        ref: 'outside-x',
        target: 'root-modal',
      },
    ]);
    expect(fixed[0].children).toBeUndefined(); // 无 children 字段保持
    expect((fixed[0].props.onClick as { target: string }).target).toBe('root-modal');
  });

  it('无 children 且无命中节点：返回原节点引用（children 节点 props 引用复用）', () => {
    const src = tree();
    const fixed = reconnectTemplateRefs(src, [
      { nodeId: 'ghost-node', kind: 'action', prop: 'onClick', ref: 'x', target: 'root-modal' },
    ]);
    expect(fixed[0]).toBe(src[0]); // 无 children + 无命中 → 原节点引用
    // 有 children 的节点恒重建外壳（children walk 展开），但 props 引用复用
    expect(fixed[1].props).toBe(src[1].props);
    expect(fixed[1].children![0]).toBe(src[1].children![0]);
  });

  it('action 无 chain：?? [] 兜底空循环，仅替换主动作', () => {
    const src: PageNode[] = [
      {
        id: 'no-chain-btn',
        type: 'button',
        props: { onClick: { kind: 'openModal', target: 'outside-z' } },
      },
    ];
    const fixed = reconnectTemplateRefs(src, [
      {
        nodeId: 'no-chain-btn',
        kind: 'action',
        prop: 'onClick',
        ref: 'outside-z',
        target: 'no-chain-btn',
      },
    ]);
    expect((fixed[0].props.onClick as { target: string }).target).toBe('no-chain-btn');
  });

  it('assignment/rowAction 数组含不匹配项：命中替换、未命中原对象保留', () => {
    const src: PageNode[] = [
      {
        id: 'mix-tbl',
        type: 'fnTable',
        props: {
          functionId: 'f',
          inputAssignments: [
            { param: '/hit', sourceNodeId: 'old-a' },
            { param: '/keep', sourceNodeId: 'other-b' },
          ],
          rowActions: [
            { label: 'hit', targetSection: 'old-a' },
            { label: 'keep', targetSection: 'other-b' },
          ],
        },
      },
    ];
    const fixed = reconnectTemplateRefs(src, [
      {
        nodeId: 'mix-tbl',
        kind: 'assignment',
        prop: 'inputAssignments',
        ref: 'old-a',
        target: 'mix-tbl',
      },
      { nodeId: 'mix-tbl', kind: 'rowAction', prop: 'rowActions', ref: 'old-a', target: 'mix-tbl' },
    ]);
    const assigns = fixed[0].props.inputAssignments as Array<{
      param: string;
      sourceNodeId?: string;
    }>;
    expect(assigns[0].sourceNodeId).toBe('mix-tbl'); // 命中替换
    expect(assigns[1].sourceNodeId).toBe('other-b'); // 未命中保留
    const actions = fixed[0].props.rowActions as Array<{ label: string; targetSection?: string }>;
    expect(actions[0].targetSection).toBe('mix-tbl');
    expect(actions[1].targetSection).toBe('other-b');
  });
});

// ---------------------------------------------------------------------------
// UI 补缺口
// ---------------------------------------------------------------------------
/** 模板矩阵：覆盖分组三态/Tag/desc/缺函数/带参/普通点击。 */
const uiTemplates: ComponentTemplateDTO[] = [
  baseTpl('data--one', {
    name: { 'zh-CN': '玩家表' },
    category: '数据',
    description: { 'zh-CN': '数据类说明' },
    requiredFunctions: ['player.list'],
    builtin: true,
    tree: [{ id: 'n1', type: 'fnTable', props: { functionId: 'player.list' } }],
  }),
  baseTpl('custom--missing', {
    name: { 'zh-CN': '缺依赖' },
    stale: true,
    requiredFunctions: ['player.list', 'ghost.fn'],
    tree: [{ id: 'n2', type: 'text', props: { content: 'x' } }],
  }),
  baseTpl('custom--peer', {
    name: { 'zh-CN': '同组者' },
    tree: [{ id: 'n3', type: 'button', props: { title: 'B' } }],
  }),
  baseTpl('builtin--plain', {
    name: { 'zh-CN': '内置模板' },
    builtin: true,
    tree: [{ id: 'n4', type: 'text', props: { content: 'y' } }],
  }),
  baseTpl('param--one', {
    name: { 'zh-CN': '带参数' },
    params: [{ key: 'p1', nodeId: 'n5', prop: 'title', default: 'd' }],
    tree: [{ id: 'n5', type: 'button', props: { title: 'P' } }],
  }),
];

function renderLibrary(availableFnIds: Set<string> = new Set(['player.list'])): {
  onInsert: jest.Mock;
} {
  const onInsert = jest.fn();
  render(
    <App>
      <ComponentLibrary availableFnIds={availableFnIds} onInsert={onInsert} />
    </App>,
  );
  return { onInsert };
}

/** 模板卡片根容器（TemplateDraggable div：style 带 cursor）。 */
function cardOf(name: string): HTMLElement {
  const label = screen.getByText(name);
  return label.closest('div[style*="cursor"]') as HTMLElement;
}

async function renderWithList(): Promise<{ onInsert: jest.Mock; container: HTMLElement }> {
  mockedRequest.mockImplementation(async () => ({ items: uiTemplates }));
  const onInsert = jest.fn();
  const { container } = render(
    <App>
      <ComponentLibrary availableFnIds={new Set(['player.list'])} onInsert={onInsert} />
    </App>,
  );
  await screen.findByText('玩家表', undefined, { timeout: 5000 });
  return { onInsert, container };
}

describe('ComponentLibrary 渲染矩阵：分组/Tag/说明/缺函数', () => {
  it('分组三态（显式 category/内置兜底/自定义兜底）+ 同组聚合 + 排序', async () => {
    const { container } = await renderWithList();
    const heads = [...container.querySelectorAll('h5')].map((h) => h.textContent ?? '');
    const expected = ['内置', '数据', '自定义'].sort((a, b) => a.localeCompare(b));
    expect(heads).toEqual(expected);
    // 自定义组聚合三个模板（byCat.has else 分支）
    const customGroup = heads.indexOf('自定义');
    const groups = [...container.querySelectorAll('h5')].map((h) => h.parentElement!);
    const customCards = groups[customGroup].querySelectorAll('div[style*="cursor"]');
    expect(customCards).toHaveLength(3);
  });

  it('builtin/stale Tag 与 description 渲染；无 desc 模板不渲染说明行', async () => {
    await renderWithList();
    // builtin Tag（两个内置模板各一枚）
    expect((await screen.findAllByText('内置')).length).toBeGreaterThanOrEqual(2);
    expect(screen.getByText('已过期')).toBeInTheDocument();
    expect(screen.getByText('数据类说明')).toBeInTheDocument();
    // 缺依赖模板无 description → 无说明文本（desc && false 分支）
    expect(screen.queryByText(/^缺依赖说明/)).not.toBeInTheDocument();
  });

  it('缺依赖模板：!ok 提示缺函数、cursor not-allowed、点击拦截（onInsert 不调用）', async () => {
    const { onInsert } = await renderWithList();
    await waitFor(() => expect(screen.getByText('缺少函数：ghost.fn')).toBeInTheDocument());
    const card = cardOf('缺依赖');
    expect(card.style.cursor).toBe('not-allowed');
    expect(card.style.opacity).toBe('0.5');
    fireEvent.click(card);
    expect(onInsert).not.toHaveBeenCalled();
  });

  it('isDragging 态：卡片半透明（opacity 0.4）', async () => {
    mockedRequest.mockImplementation(async () => ({ items: [uiTemplates[2]] }));
    renderLibrary();
    await screen.findByText('同组者', undefined, { timeout: 5000 });
    draggableState.isDragging = true;
    // 用搜索输入触发重渲染（isDragging 读取自 mock state，重渲染后生效）
    const input = screen.getByPlaceholderText('搜索组件') as HTMLInputElement;
    fireEvent.change(input, { target: { value: '同' } });
    await waitFor(() => expect(cardOf('同组者').style.opacity).toBe('0.4'));
  });
});

describe('ComponentLibrary 点击插入', () => {
  it('普通模板点击：实例化（新 id）+ 悬空上报（nodeTitle 兜底）', async () => {
    mockedRequest.mockImplementation(async () => ({
      items: [
        baseTpl('click--plain', {
          name: { 'zh-CN': '点击我' },
          tree: [
            {
              id: 'c-btn',
              type: 'button',
              // 无 title → 悬空 nodeTitle 兜底类型名
              props: { onClick: { kind: 'openModal', target: 'nowhere' } },
            },
            { id: 'c-tbl', type: 'fnTable', props: { functionId: 'player.list' } },
          ],
        }),
      ],
    }));
    const { onInsert } = renderLibrary();
    await screen.findByText('点击我', undefined, { timeout: 5000 });
    fireEvent.click(cardOf('点击我'));
    expect(onInsert).toHaveBeenCalledTimes(1);
    const [nodes, tpl, dangling] = onInsert.mock.calls[0] as [
      PageNode[],
      ComponentTemplateDTO,
      Array<{ nodeId: string; nodeTitle: string }>,
    ];
    expect(nodes).toHaveLength(2);
    expect(nodes[0].id).not.toBe('c-btn'); // 实例化重分配 id
    expect(tpl.key).toBe('click--plain');
    expect(dangling).toHaveLength(1);
    expect(dangling[0].nodeTitle).toBe('button'); // 无 title 兜底
  });

  it('带参数模板点击：onInsert([], tpl) 抛给父级弹参数表单', async () => {
    mockedRequest.mockImplementation(async () => ({ items: [uiTemplates[4]] }));
    const { onInsert } = renderLibrary();
    await screen.findByText('带参数', undefined, { timeout: 5000 });
    fireEvent.click(cardOf('带参数'));
    expect(onInsert).toHaveBeenCalledTimes(1);
    const [nodes, tpl, dangling] = onInsert.mock.calls[0] as [
      PageNode[],
      ComponentTemplateDTO,
      Array<{ nodeId: string; nodeTitle: string }> | undefined,
    ];
    expect(nodes).toEqual([]);
    expect(tpl.key).toBe('param--one');
    expect(dangling).toBeUndefined(); // 未实例化 → 无悬空
  });
});

describe('ComponentLibrary 搜索过滤', () => {
  async function renderSearchable(): Promise<HTMLElement> {
    mockedRequest.mockImplementation(async () => ({ items: uiTemplates }));
    renderLibrary();
    await screen.findByText('玩家表', undefined, { timeout: 5000 });
    return screen.getByPlaceholderText('搜索组件') as HTMLInputElement;
  }

  it('name 命中：只留匹配卡片', async () => {
    const input = await renderSearchable();
    fireEvent.change(input, { target: { value: '玩家表' } });
    await waitFor(() => expect(screen.queryByText('缺依赖')).not.toBeInTheDocument());
    expect(screen.getByText('玩家表')).toBeInTheDocument();
  });

  it('category 命中与 key 命中；无命中清空列表', async () => {
    const input = await renderSearchable();
    // category 命中
    fireEvent.change(input, { target: { value: '数据' } });
    await waitFor(() => expect(screen.getByText('玩家表')).toBeInTheDocument());
    expect(screen.queryByText('同组者')).not.toBeInTheDocument();
    // key 命中（custom--peer）
    fireEvent.change(input, { target: { value: 'custom--peer' } });
    await waitFor(() => expect(screen.getByText('同组者')).toBeInTheDocument());
    expect(screen.queryByText('玩家表')).not.toBeInTheDocument();
    // 无命中：filtered 空（templates 非空 → 不显示空态 Empty）
    fireEvent.change(input, { target: { value: '不存在的东西' } });
    await waitFor(() => expect(screen.queryByText('玩家表')).not.toBeInTheDocument());
    expect(screen.queryByText('同组者')).not.toBeInTheDocument();
    expect(screen.queryByText('暂无组件模板')).not.toBeInTheDocument();
  });

  it('清空关键词回到全量', async () => {
    const input = await renderSearchable();
    fireEvent.change(input, { target: { value: '玩家表' } });
    await waitFor(() => expect(screen.queryByText('同组者')).not.toBeInTheDocument());
    fireEvent.change(input, { target: { value: '' } });
    await waitFor(() => expect(screen.getByText('同组者')).toBeInTheDocument());
  });
});

describe('ComponentLibrary 拉取形态', () => {
  it('数组响应形态同样解析出列表', async () => {
    mockedRequest.mockImplementation(async () => [uiTemplates[0]]);
    renderLibrary();
    expect(await screen.findByText('玩家表', undefined, { timeout: 5000 })).toBeInTheDocument();
    expect(screen.queryByText('暂无组件模板')).not.toBeInTheDocument();
  });

  it('items 缺省（空对象响应）：?? [] 兜底 → 空态', async () => {
    mockedRequest.mockImplementation(async () => ({}));
    renderLibrary();
    await screen.findByText('暂无组件模板', undefined, { timeout: 5000 });
    expect(screen.getByText(/选中画布多个节点/)).toBeInTheDocument();
  });

  it('fetch 失败：catch 兜底空列表 → 空态（不炸）', async () => {
    mockedRequest.mockImplementation(async () => Promise.reject(new Error('down')));
    renderLibrary();
    await screen.findByText('暂无组件模板', undefined, { timeout: 5000 });
  });

  it('模板 tree 缺省（防御形态）：卡片渲染空缩略图、点击插入空 nodes', async () => {
    mockedRequest.mockImplementation(async () => ({
      items: [
        {
          ...baseTpl('no-tree-card', { name: { 'zh-CN': '无树模板' } }),
          tree: undefined as unknown as PageNode[],
        },
      ],
    }));
    const { onInsert } = renderLibrary();
    await screen.findByText('无树模板', undefined, { timeout: 5000 });
    // tree ?? [] 兜底：缩略图空态渲染不炸
    fireEvent.click(cardOf('无树模板'));
    expect(onInsert).toHaveBeenCalledTimes(1);
    const [nodes] = onInsert.mock.calls[0] as [PageNode[], ComponentTemplateDTO];
    expect(nodes).toEqual([]);
  });

  it('loading 态：请求 pending 期间显示「加载组件库…」', async () => {
    let resolveList: (v: { items: ComponentTemplateDTO[] }) => void = () => undefined;
    mockedRequest.mockImplementation(
      () =>
        new Promise((res) => {
          resolveList = res as (v: { items: ComponentTemplateDTO[] }) => void;
        }),
    );
    renderLibrary();
    await screen.findByText('加载组件库…', undefined, { timeout: 5000 });
    resolveList({ items: [uiTemplates[0]] });
    await screen.findByText('玩家表', undefined, { timeout: 5000 });
    expect(screen.queryByText('加载组件库…')).not.toBeInTheDocument();
  });
});
