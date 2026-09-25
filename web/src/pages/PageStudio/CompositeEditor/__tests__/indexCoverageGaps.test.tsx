/** index.tsx 覆盖率缺口补齐（桩/工具与 index.test.tsx 同源，独立文件不改既有用例）：
 * 1) SortableList 桩改为真实调用 getKey → 覆盖 index.tsx L1136 getKey；
 * 2) 弹窗内联编辑中撤销删除该弹窗 → editingModalId 悬空 → editingModal 回退 null（L303）；
 * 3) 提案已创建弹窗 ESC 关闭（非「知道了」）→ onCancel → saving 复位（L485 onCancel）。
 * 注：L697-704 / L710-717 两段选中为空、集合悬空守卫在现有调用链下不可达
 *（三个入口均已保证集合非空；findNode 缺失返回 undefined 不被 filter 滤掉），
 * 见 index.test.tsx「集合悬空」用例注释，本文件不硬凑。 */
import React from 'react';
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App } from 'antd';
import { request } from '@umijs/max';
import CompositeEditorPage from '../index';
import { listDescriptors } from '@/services/api/functions';
import type { FunctionDescriptor } from '@/services/api/functions';
import type { PageNode } from '../model';
import type {
  ComponentTemplateDTO,
  DanglingTemplateRef,
  TemplateRefFix,
} from '../ComponentLibrary';

// 59 用例全量渲染主组件；coverage instrumentation 负载下个别交互用例
// 撞 20s 预算（隔离跑恒绿），放宽到 30s
jest.setTimeout(30000);
const FIND = { timeout: 5000 } as const;

// ---------------------------------------------------------------------------
// 桩组件 prop 形状（类型仅编译期；工厂体内 require('react')）
// ---------------------------------------------------------------------------
interface SelectEvt {
  shiftKey?: boolean;
}
type SelectHandler = (e?: SelectEvt) => void;

interface PageContainerStubProps {
  header?: {
    title?: React.ReactNode;
    extra?: React.ReactNode[];
    onBack?: () => void;
  };
  children?: React.ReactNode;
}

interface CanvasStubProps {
  tree: PageNode[];
  children?: React.ReactNode;
  onDelete: (id: string) => void;
  onShowTemplates?: () => void;
}

interface ModalPlaceholderStubProps {
  modal: PageNode;
  selected: boolean;
  onSelect: SelectHandler;
  onEnterModal: () => void;
}

interface CanvasNodeStubProps {
  node: PageNode;
  fn?: FunctionDescriptor;
  selected: boolean;
  onSelect: SelectHandler;
  onDelete: () => void;
  onDuplicate: () => void;
  onSpanChange: (span: number) => void;
  onSelectParent?: () => void;
  onMoveUp: () => void;
  onMoveDown: () => void;
  onSaveAsComponent: () => void;
  onChildSelect: (id: string) => void;
  onChildDelete: (id: string) => void;
  onChildMove: (id: string, dir: -1 | 1) => void;
}

interface ComponentPanelStubProps {
  onAddBasic: (type: 'button' | 'modal' | 'container' | 'tabs' | 'text') => void;
  onAddFunction: (e: {
    fn: FunctionDescriptor;
    componentType: 'fnTable' | 'fnFields' | 'fnForm';
  }) => void;
}

interface LibStubProps {
  availableFnIds: Set<string>;
  onInsert: (
    nodes: PageNode[],
    tpl: ComponentTemplateDTO,
    dangling?: DanglingTemplateRef[],
  ) => void;
  onCreateFromCanvas?: () => void;
}

interface QsStubProps {
  onPick: (nodes: PageNode[], tpl: ComponentTemplateDTO, dangling?: DanglingTemplateRef[]) => void;
  onStartBlank: () => void;
}

interface SaveComponentModalStubProps {
  state: { fnIds: string[]; selectedNodes: PageNode[] } | null;
  onClose: () => void;
}

interface InsertTemplateModalStubProps {
  tplState: { tpl: ComponentTemplateDTO; overId: string } | null;
  onClose: () => void;
  onConfirm: (tpl: ComponentTemplateDTO, values: Record<string, unknown>, overId: string) => void;
}

interface DanglingModalStubProps {
  open: boolean;
  refs: DanglingTemplateRef[];
  candidateNodes: Array<{ id: string; title: string; type: string }>;
  onClose: () => void;
  onApply: (fixes: TemplateRefFix[]) => void;
}

interface PropsPanelStubProps {
  node?: PageNode;
  onPatch: (patch: Record<string, unknown>) => void;
  onRenameVariable: (newName: string) => void;
  onCreateModal: (fn: FunctionDescriptor) => void;
  onDelete: () => void;
  onOpenBinding?: (fn: FunctionDescriptor) => void;
}

interface DataPanelStubProps {
  node?: PageNode;
  fn?: FunctionDescriptor;
}

interface OutlinePanelStubProps {
  tree: PageNode[];
  selectedId: string | null;
}

interface PreviewRuntimeStubProps {
  tree: PageNode[];
}

interface SortableStubProps {
  items: PageNode[];
  getKey: (node: PageNode) => string;
  onReorder: (next: PageNode[]) => void;
  children: (
    node: PageNode,
    idx: number,
    dragHandleProps: Record<string, unknown>,
  ) => React.ReactNode;
}

interface DndHandlers {
  onDragStart?: (e: unknown) => void;
  onDragOver?: (e: { over?: { id: string | number } | null }) => void;
  onDragEnd?: (e: unknown) => void;
}

interface DndHookState {
  setDragItem: ((v: unknown) => void) | null;
  handleDragStart: jest.Mock;
  handleDragEnd: jest.Mock;
  applyTemplateInsert: jest.Mock;
}

// ---------------------------------------------------------------------------
// mock 声明
// ---------------------------------------------------------------------------
jest.mock('@/services/api/functions', () => ({
  listDescriptors: jest.fn(async () => []),
}));

// T9 绑定抽屉（真实组件）依赖的 OpenAPI service
jest.mock('@/services/api/openapi', () => ({
  __esModule: true,
  listOpenAPISources: jest.fn(async () => ({ items: [] })),
  getOpenAPISource: jest.fn(async () => ({ source: { operations: [] } })),
  listRuntimeSources: jest.fn(async () => ({ items: [], total: 0 })),
  bindOpenAPISourceProvider: jest.fn(async () => ({})),
}));

jest.mock('@/stores/scope', () => {
  const state = { current: null as (() => void) | null };
  const subscribeScope = jest.fn((listener: () => void) => {
    state.current = listener;
    return () => undefined;
  });
  return { __esModule: true, subscribeScope, __scopeListener: state };
});

jest.mock('@umijs/max', () => {
  const R = require('react') as typeof React;
  const formatMessage = (
    descriptor: { defaultMessage?: string },
    values?: Record<string, unknown>,
  ): string =>
    Object.entries(values ?? {}).reduce(
      (msg, [key, val]) => msg.split(`{${key}}`).join(String(val)),
      descriptor.defaultMessage ?? '',
    );
  const state = { search: '' };
  const req = jest.fn(async () => ({}));
  const history = {
    push: jest.fn(),
    replace: jest.fn(),
    location: { pathname: '/functions/pages' },
  };
  const FormattedMessage = ({
    defaultMessage,
    values,
  }: {
    defaultMessage?: React.ReactNode;
    values?: Record<string, unknown>;
  }) =>
    R.createElement(
      R.Fragment,
      null,
      formatMessage(
        { defaultMessage: typeof defaultMessage === 'string' ? defaultMessage : '' },
        values,
      ),
    );
  return {
    __esModule: true,
    history,
    request: req,
    useIntl: () => ({ formatMessage }),
    getIntl: () => ({ formatMessage }),
    FormattedMessage,
    useSearchParams: () => [new URLSearchParams(state.search), jest.fn()],
    __umiState: state,
  };
});

jest.mock('@ant-design/pro-components', () => {
  const R = require('react') as typeof React;
  const PageContainer = ({ header, children }: PageContainerStubProps) =>
    R.createElement(
      'div',
      null,
      R.createElement('div', { 'data-testid': 'pc:title' }, header?.title),
      header?.onBack
        ? R.createElement('button', { 'data-testid': 'pc:back', onClick: header.onBack }, 'back')
        : null,
      R.createElement(
        'div',
        { 'data-testid': 'pc:extra' },
        ...(header?.extra ?? []).filter(Boolean),
      ),
      children,
    );
  return { __esModule: true, PageContainer };
});

jest.mock('@dnd-kit/core', () => {
  const R = require('react') as typeof React;
  const actual = jest.requireActual('@dnd-kit/core');
  const state = { handlers: {} as DndHandlers };
  const DndContext = ({ children, ...handlers }: { children?: React.ReactNode } & DndHandlers) => {
    state.handlers = handlers;
    return R.createElement('div', { 'data-testid': 'dnd' }, children);
  };
  const DragOverlay = ({ children }: { children?: React.ReactNode }) =>
    R.createElement('div', { 'data-testid': 'overlay' }, children);
  return { ...actual, DndContext, DragOverlay, __dndKitState: state };
});

jest.mock('@/components/SortableList', () => {
  const R = require('react') as typeof React;
  const SortableList = ({ items, getKey, onReorder, children }: SortableStubProps) =>
    R.createElement(
      'div',
      { 'data-testid': 'sortable', 'data-keys': items.map((n) => getKey(n)).join(',') },
      R.createElement(
        'button',
        { 'data-testid': 'sl:reorder', onClick: () => onReorder([...items].reverse()) },
        'reorder',
      ),
      R.createElement(
        'button',
        { 'data-testid': 'sl:reorder-empty', onClick: () => onReorder([]) },
        'reorder-empty',
      ),
      ...items.map((n, i) => children(n, i, {})),
    );
  return { __esModule: true, SortableList };
});

jest.mock('../Canvas', () => {
  const R = require('react') as typeof React;
  const btn = (testid: string, onClick: () => void) =>
    R.createElement('button', { 'data-testid': testid, onClick }, testid);
  const Canvas = ({ tree, children, onDelete, onShowTemplates }: CanvasStubProps) =>
    R.createElement(
      'div',
      { 'data-testid': 'canvas' },
      ...(tree ?? []).map((n) => btn(`canvas:del:${n.id}`, () => onDelete(n.id))),
      onShowTemplates
        ? R.createElement(
            'button',
            { 'data-testid': 'canvas:show-templates', onClick: onShowTemplates },
            'show-templates',
          )
        : null,
      children,
    );
  const ModalPlaceholder = ({
    modal,
    selected,
    onSelect,
    onEnterModal,
  }: ModalPlaceholderStubProps) =>
    R.createElement(
      'div',
      { 'data-testid': `mp:${modal.id}` },
      R.createElement('span', { 'data-testid': `mp:${modal.id}:sel-flag` }, selected ? '1' : '0'),
      R.createElement(
        'span',
        { 'data-testid': `mp:${modal.id}:kids` },
        (modal.children ?? []).map((c) => c.id).join(','),
      ),
      btn(`mp:${modal.id}:sel`, () => onSelect({})),
      btn(`mp:${modal.id}:sel-shift`, () => onSelect({ shiftKey: true })),
      btn(`mp:${modal.id}:sel-undef`, () => onSelect(undefined)),
      btn(`mp:${modal.id}:enter`, onEnterModal),
    );
  const CanvasNode = (props: CanvasNodeStubProps) => {
    const { node, fn, selected } = props;
    return R.createElement(
      'div',
      { 'data-testid': `cn:${node.id}` },
      R.createElement('span', { 'data-testid': `cn:${node.id}:fn` }, fn ? fn.id : 'none'),
      R.createElement('span', { 'data-testid': `cn:${node.id}:sel-flag` }, selected ? '1' : '0'),
      R.createElement(
        'span',
        { 'data-testid': `cn:${node.id}:kids` },
        (node.children ?? []).map((c) => c.id).join(','),
      ),
      R.createElement(
        'span',
        { 'data-testid': `node-props:${node.id}` },
        JSON.stringify(node.props),
      ),
      btn(`cn:${node.id}:sel`, () => props.onSelect({})),
      btn(`cn:${node.id}:sel-shift`, () => props.onSelect({ shiftKey: true })),
      btn(`cn:${node.id}:sel-undef`, () => props.onSelect(undefined)),
      btn(`cn:${node.id}:del`, props.onDelete),
      btn(`cn:${node.id}:dup`, props.onDuplicate),
      btn(`cn:${node.id}:span`, () => props.onSpanChange(12)),
      props.onSelectParent ? btn(`cn:${node.id}:parent`, props.onSelectParent) : null,
      btn(`cn:${node.id}:up`, props.onMoveUp),
      btn(`cn:${node.id}:down`, props.onMoveDown),
      btn(`cn:${node.id}:save-comp`, props.onSaveAsComponent),
      btn(`cn:${node.id}:csel`, () => props.onChildSelect('c0')),
      btn(`cn:${node.id}:cdel`, () => props.onChildDelete('c0')),
      btn(`cn:${node.id}:cmv-down`, () => props.onChildMove('c0', 1)),
      btn(`cn:${node.id}:cmv-up`, () => props.onChildMove('c0', -1)),
      btn(`cn:${node.id}:cmv-ghost`, () => props.onChildMove('ghost-child', -1)),
    );
  };
  return { __esModule: true, default: Canvas, CanvasNode, ModalPlaceholder };
});

jest.mock('../SaveComponentModal', () => {
  const R = require('react') as typeof React;
  const SaveComponentModal = ({ state, onClose }: SaveComponentModalStubProps) =>
    state
      ? R.createElement(
          'div',
          { 'data-testid': 'scm' },
          R.createElement('span', { 'data-testid': 'scm:fnIds' }, state.fnIds.join(',')),
          R.createElement(
            'span',
            { 'data-testid': 'scm:nodes' },
            state.selectedNodes.map((n) => n.id).join(','),
          ),
          R.createElement('button', { 'data-testid': 'scm:close', onClick: onClose }, 'close'),
        )
      : null;
  return { __esModule: true, default: SaveComponentModal };
});

jest.mock('../InsertTemplateModal', () => {
  const R = require('react') as typeof React;
  const InsertTemplateModal = ({ tplState, onClose, onConfirm }: InsertTemplateModalStubProps) =>
    R.createElement(
      'div',
      { 'data-testid': 'itm' },
      R.createElement('span', { 'data-testid': 'itm:open' }, tplState ? '1' : '0'),
      R.createElement(
        'button',
        {
          'data-testid': 'itm:confirm',
          onClick: () => {
            if (tplState) onConfirm(tplState.tpl, {}, tplState.overId);
          },
        },
        'confirm',
      ),
      R.createElement('button', { 'data-testid': 'itm:close', onClick: onClose }, 'close'),
    );
  return { __esModule: true, default: InsertTemplateModal };
});

jest.mock('../DanglingRefsModal', () => {
  const R = require('react') as typeof React;
  const config = { fixes: [] as TemplateRefFix[] };
  const DanglingRefsModal = ({
    open,
    refs,
    candidateNodes,
    onClose,
    onApply,
  }: DanglingModalStubProps) =>
    R.createElement(
      'div',
      { 'data-testid': 'drm' },
      R.createElement('span', { 'data-testid': 'drm:open' }, open ? '1' : '0'),
      R.createElement('span', { 'data-testid': 'drm:refs' }, refs.map((r) => r.nodeId).join(',')),
      R.createElement(
        'span',
        { 'data-testid': 'drm:candidates' },
        candidateNodes.map((c) => c.id).join(','),
      ),
      R.createElement(
        'button',
        { 'data-testid': 'drm:apply', onClick: () => onApply(config.fixes) },
        'apply',
      ),
      R.createElement('button', { 'data-testid': 'drm:close', onClick: onClose }, 'close'),
    );
  return { __esModule: true, default: DanglingRefsModal, __danglingConfig: config };
});

jest.mock('../PropsPanel', () => {
  const R = require('react') as typeof React;
  const btn = (testid: string, onClick: () => void) =>
    R.createElement('button', { 'data-testid': testid, onClick }, testid);
  const PropsPanel = ({
    node,
    onPatch,
    onRenameVariable,
    onCreateModal,
    onDelete,
    onOpenBinding,
  }: PropsPanelStubProps) =>
    R.createElement(
      'div',
      { 'data-testid': 'pp' },
      R.createElement('span', { 'data-testid': 'pp:node' }, node ? node.id : 'none'),
      btn('pp:patch-title', () => onPatch({ title: '已改标题' })),
      btn('pp:patch-fn-same', () =>
        onPatch({ functionId: String(node?.props.functionId ?? 'same.fn') }),
      ),
      btn('pp:patch-fn-known', () => onPatch({ functionId: 'mail.send' })),
      btn('pp:patch-fn-ghost', () => onPatch({ functionId: 'ghost.fn' })),
      btn('pp:rename', () => onRenameVariable('renamed')),
      btn('pp:patch-nokey', () => onPatch({ sectionKey: undefined })),
      btn('pp:create-modal', () =>
        onCreateModal({ id: 'mail.send', summary: { 'zh-CN': '发邮件' } }),
      ),
      onOpenBinding
        ? btn('pp:open-binding', () => onOpenBinding({ id: String(node?.props.functionId ?? '') }))
        : null,
      btn('pp:delete', onDelete),
    );
  return { __esModule: true, default: PropsPanel };
});

jest.mock('../DataPanel', () => {
  const R = require('react') as typeof React;
  const DataPanel = ({ node, fn }: DataPanelStubProps) =>
    R.createElement(
      'div',
      { 'data-testid': 'dp' },
      R.createElement('span', { 'data-testid': 'dp:node' }, node ? node.id : 'none'),
      R.createElement('span', { 'data-testid': 'dp:fn' }, fn ? fn.id : 'none'),
    );
  return { __esModule: true, default: DataPanel };
});

jest.mock('../OutlinePanel', () => {
  const R = require('react') as typeof React;
  const OutlinePanel = ({ tree, selectedId }: OutlinePanelStubProps) =>
    R.createElement(
      'div',
      { 'data-testid': 'op' },
      R.createElement('span', { 'data-testid': 'op:count' }, String(tree.length)),
      R.createElement('span', { 'data-testid': 'op:selected' }, selectedId ?? 'none'),
    );
  return { __esModule: true, default: OutlinePanel };
});

jest.mock('../PreviewRuntime', () => {
  const R = require('react') as typeof React;
  const PreviewRuntime = ({ tree }: PreviewRuntimeStubProps) =>
    R.createElement(
      'div',
      { 'data-testid': 'pr' },
      R.createElement('span', { 'data-testid': 'pr:count' }, String(tree.length)),
    );
  return { __esModule: true, default: PreviewRuntime };
});

jest.mock('../ComponentPanel', () => {
  const R = require('react') as typeof React;
  const btn = (testid: string, onClick: () => void) =>
    R.createElement('button', { 'data-testid': testid, onClick }, testid);
  const fnDesc: FunctionDescriptor = { id: 'player.list', summary: { 'zh-CN': '玩家列表' } };
  const basicTypes: Array<'button' | 'modal' | 'container' | 'tabs' | 'text'> = [
    'button',
    'modal',
    'container',
    'text',
  ];
  const ComponentPanel = ({ onAddBasic, onAddFunction }: ComponentPanelStubProps) =>
    R.createElement(
      'div',
      { 'data-testid': 'cp' },
      ...basicTypes.map((t) => btn(`cp:basic:${t}`, () => onAddBasic(t))),
      btn('cp:basic:tabs', () => onAddBasic('tabs')),
      btn('cp:fn:fnForm', () => onAddFunction({ fn: fnDesc, componentType: 'fnForm' })),
      btn('cp:fn:fnTable', () => onAddFunction({ fn: fnDesc, componentType: 'fnTable' })),
    );
  return { __esModule: true, default: ComponentPanel };
});

jest.mock('../TemplateQuickStart', () => {
  const R = require('react') as typeof React;
  const config = {
    nodes: [] as PageNode[],
    tpl: {
      key: 'qs-tpl',
      name: { 'zh-CN': '引导模板' },
      tree: [],
      builtin: true,
    } as ComponentTemplateDTO,
    dangling: null as DanglingTemplateRef[] | null,
  };
  const TemplateQuickStart = ({ onPick, onStartBlank }: QsStubProps) =>
    R.createElement(
      'div',
      { 'data-testid': 'qs' },
      R.createElement(
        'button',
        {
          'data-testid': 'qs:pick',
          onClick: () => onPick(config.nodes, config.tpl, config.dangling),
        },
        'pick',
      ),
      R.createElement('button', { 'data-testid': 'qs:blank', onClick: onStartBlank }, 'blank'),
    );
  return { __esModule: true, default: TemplateQuickStart, __qsConfig: config };
});

jest.mock('../ComponentLibrary', () => {
  const R = require('react') as typeof React;
  const actual = jest.requireActual('../ComponentLibrary');
  const config = {
    nodes: [] as PageNode[],
    tpl: null as ComponentTemplateDTO | null,
    dangling: [] as DanglingTemplateRef[],
  };
  const ComponentLibrary = ({ availableFnIds, onInsert, onCreateFromCanvas }: LibStubProps) =>
    R.createElement(
      'div',
      { 'data-testid': 'lib' },
      R.createElement('span', { 'data-testid': 'lib:avail' }, [...availableFnIds].join(',')),
      R.createElement(
        'button',
        {
          'data-testid': 'lib:insert',
          onClick: () => {
            if (config.tpl) onInsert(config.nodes, config.tpl, config.dangling);
          },
        },
        'insert',
      ),
      R.createElement(
        'button',
        {
          'data-testid': 'lib:create',
          onClick: () => {
            if (onCreateFromCanvas) onCreateFromCanvas();
          },
        },
        'create',
      ),
    );
  return {
    __esModule: true,
    default: ComponentLibrary,
    reconnectTemplateRefs: actual.reconnectTemplateRefs,
    __libConfig: config,
  };
});

jest.mock('../useCanvasDnd', () => {
  const R = require('react') as typeof React;
  const state: DndHookState = {
    setDragItem: null,
    handleDragStart: jest.fn(),
    handleDragEnd: jest.fn(),
    applyTemplateInsert: jest.fn(),
  };
  const useCanvasDnd = () => {
    const [dragItem, setDragItem] = R.useState<unknown>(null);
    const [overNodeId, setOverNodeId] = R.useState<string | null>(null);
    state.setDragItem = setDragItem;
    return {
      dragItem,
      overNodeId,
      setOverNodeId,
      handleDragStart: state.handleDragStart,
      handleDragEnd: state.handleDragEnd,
      applyTemplateInsert: state.applyTemplateInsert,
    };
  };
  return { __esModule: true, useCanvasDnd, __dndState: state };
});

// ---------------------------------------------------------------------------
// mock 状态访问与工具
// ---------------------------------------------------------------------------
const mockedRequest = request as unknown as jest.Mock;
const mockedListDescriptors = listDescriptors as unknown as jest.Mock;
const mockedOpenapi = jest.requireMock('@/services/api/openapi') as {
  listOpenAPISources: jest.Mock;
  getOpenAPISource: jest.Mock;
  listRuntimeSources: jest.Mock;
  bindOpenAPISourceProvider: jest.Mock;
};

const umiMock = jest.requireMock('@umijs/max') as {
  __umiState: { search: string };
  history: { push: jest.Mock; replace: jest.Mock };
};
const scopeListener = (
  jest.requireMock('@/stores/scope') as { __scopeListener: { current: (() => void) | null } }
).__scopeListener;
const qsConfig = (jest.requireMock('../TemplateQuickStart') as { __qsConfig: QsStubConfig })
  .__qsConfig;
const libConfig = (jest.requireMock('../ComponentLibrary') as { __libConfig: LibStubConfig })
  .__libConfig;
const danglingConfig = (
  jest.requireMock('../DanglingRefsModal') as { __danglingConfig: { fixes: TemplateRefFix[] } }
).__danglingConfig;
const dndState = (jest.requireMock('../useCanvasDnd') as { __dndState: DndHookState }).__dndState;
const dndKitState = (
  jest.requireMock('@dnd-kit/core') as { __dndKitState: { handlers: DndHandlers } }
).__dndKitState;

interface QsStubConfig {
  nodes: PageNode[];
  tpl: ComponentTemplateDTO;
  dangling: DanglingTemplateRef[] | null;
}
interface LibStubConfig {
  nodes: PageNode[];
  tpl: ComponentTemplateDTO | null;
  dangling: DanglingTemplateRef[];
}

const fnPlayer: FunctionDescriptor = { id: 'player.list', summary: { 'zh-CN': '玩家列表' } };
const fnMail: FunctionDescriptor = { id: 'mail.send', summary: { 'zh-CN': '发邮件' } };

function byId(id: string): HTMLElement {
  return document.querySelector(`[data-testid="${id}"]`) as HTMLElement;
}
function click(id: string): void {
  fireEvent.click(byId(id));
}
function renderEditor() {
  return render(
    <App>
      <CompositeEditorPage />
    </App>,
  );
}
function pageKeyInput(): HTMLInputElement {
  return document.querySelector('input[placeholder="按组件自动生成，可修改"]') as HTMLInputElement;
}
function toolbarBtn(title: string): HTMLButtonElement {
  return document.querySelector(`button[title="${title}"]`) as HTMLButtonElement;
}
function undoBtn(): HTMLButtonElement {
  return toolbarBtn('撤销 (Ctrl+Z)');
}
function redoBtn(): HTMLButtonElement {
  return toolbarBtn('重做 (Ctrl+Shift+Z)');
}
function gridWrappers(): HTMLElement[] {
  return [...document.querySelectorAll('div[style*="grid-column"]')] as HTMLElement[];
}
function firstGridContains(id: string): boolean {
  const first = gridWrappers()[0];
  return Boolean(first && first.contains(byId(id)));
}
/** 画布节点顺序（每个 grid 包裹层的节点根 testid：cn:x / mp:x）。 */
function canvasOrder(): string[] {
  return gridWrappers().map((w) => {
    const root = [...w.querySelectorAll('[data-testid]')].find((el) => {
      const t = el.getAttribute('data-testid') ?? '';
      return /^(cn|mp):[^:]+$/.test(t);
    });
    return root?.getAttribute('data-testid') ?? '';
  });
}
function propsJsonOf(id: string): string {
  return byId(`node-props:${id}`).textContent ?? '';
}
/** 画布上 modal 占位卡 id 列表（testid 形如 mp:<id>，无二级冒号）。 */
function modalIds(): string[] {
  return [...document.querySelectorAll('[data-testid]')]
    .map((el) => el.getAttribute('data-testid') ?? '')
    .filter((t) => /^mp:[^:]+$/.test(t))
    .map((t) => t.slice(3));
}
async function openLibraryTab(): Promise<void> {
  fireEvent.click(screen.getByRole('tab', { name: '组件库' }), undefined);
  await waitFor(() => expect(byId('lib:insert')).toBeInTheDocument());
}
/** 按「METHOD url / url」分流（未命中返回 undefined → 编辑器各 fetcher 跳过）。 */
function setRoutes(routes: Record<string, unknown>): void {
  mockedRequest.mockImplementation(
    async (url: string, opts?: { method?: string; data?: Record<string, unknown> }) => {
      const method = (opts?.method ?? 'GET').toUpperCase();
      const hit = routes[`${method} ${url}`] ?? routes[url];
      return typeof hit === 'function' ? (hit as () => unknown)() : hit;
    },
  );
}
function lastCompositePost(): Record<string, unknown> | undefined {
  const call = mockedRequest.mock.calls.find(
    ([url, opts]) =>
      url === '/api/v1/versioning/pages/composite' &&
      (opts as { method?: string } | undefined)?.method === 'POST',
  );
  return (call?.[1] as { data?: Record<string, unknown> } | undefined)?.data;
}
function tpl(key: string, extra: Partial<ComponentTemplateDTO> = {}): ComponentTemplateDTO {
  return { name: { 'zh-CN': `名称-${key}` }, tree: [], builtin: false, key, ...extra };
}
function insertViaLib(
  nodes: PageNode[],
  dto: ComponentTemplateDTO,
  dangling: DanglingTemplateRef[] = [],
): void {
  libConfig.nodes = nodes;
  libConfig.tpl = dto;
  libConfig.dangling = dangling;
  click('lib:insert');
}

beforeEach(() => {
  mockedRequest.mockReset();
  mockedRequest.mockImplementation(async () => ({}));
  mockedListDescriptors.mockReset();
  mockedListDescriptors.mockResolvedValue([]);
  mockedOpenapi.listOpenAPISources.mockReset();
  mockedOpenapi.listOpenAPISources.mockResolvedValue({ items: [] });
  mockedOpenapi.getOpenAPISource.mockReset();
  mockedOpenapi.getOpenAPISource.mockResolvedValue({ source: { operations: [] } });
  mockedOpenapi.listRuntimeSources.mockReset();
  mockedOpenapi.listRuntimeSources.mockResolvedValue({ items: [], total: 0 });
  mockedOpenapi.bindOpenAPISourceProvider.mockReset();
  mockedOpenapi.bindOpenAPISourceProvider.mockResolvedValue({});
  umiMock.__umiState.search = '';
  umiMock.history.push.mockClear();
  umiMock.history.replace.mockClear();
  scopeListener.current = null;
  qsConfig.nodes = [];
  qsConfig.tpl = tpl('qs-tpl', { name: { 'zh-CN': '引导模板' }, builtin: true });
  qsConfig.dangling = [];
  libConfig.nodes = [];
  libConfig.tpl = null;
  libConfig.dangling = [];
  danglingConfig.fixes = [];
  dndState.handleDragStart.mockClear();
  dndState.handleDragEnd.mockClear();
  dndState.applyTemplateInsert.mockClear();
  dndState.setDragItem = null;
});

// ---------------------------------------------------------------------------
// 初始渲染 / 顶栏 / 函数列表拉取
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// index.tsx 覆盖率缺口（getKey / editingModal 悬空 / 保存弹窗 onCancel）
// ---------------------------------------------------------------------------
describe('index.tsx 缺口分支', () => {
  it('SortableList 桩逐项调用 getKey（data-keys 回显节点 id）', async () => {
    renderEditor();
    await openLibraryTab();
    insertViaLib([{ id: 'gk1', type: 'button', props: { title: 'A', span: 6 } }], tpl('key-gk'));
    await waitFor(() => expect(byId('sortable').getAttribute('data-keys')).toBe('gk1'));
  });

  it('弹窗内联编辑中撤销移除该弹窗 → editingModalId 悬空 → editingModal 回退 null', async () => {
    renderEditor();
    await openLibraryTab();
    insertViaLib(
      [{ id: 'm-cov', type: 'modal', props: { title: 'M' }, children: [] }],
      tpl('modal-cov'),
    );
    await waitFor(() => expect(modalIds()).toHaveLength(1));
    click(`mp:${modalIds()[0]}:enter`);
    expect(screen.getByText(/内部编辑/)).toBeInTheDocument();

    // 撤销只回滚树、不清 editingModalId → tree.find 落空走 `?? null` 兜底
    fireEvent.click(undoBtn());
    await waitFor(() => expect(screen.queryByText(/内部编辑/)).not.toBeInTheDocument());
    expect(modalIds()).toHaveLength(0);
    // 画布回落到页面级全树渲染（不残留弹窗内联态）
    expect(screen.getByText('页面 Key')).toBeInTheDocument();
  });

  it('提案已创建弹窗 ESC 关闭（非「知道了」）→ onCancel → saving 复位、按钮退出 loading', async () => {
    setRoutes({ 'POST /api/v1/versioning/pages/composite': { proposalKey: 'composite--x' } });
    renderEditor();
    await openLibraryTab();
    insertViaLib(
      [{ id: 'sv1', type: 'fnTable', props: { functionId: 'player.list' } }],
      tpl('save-oncancel'),
    );
    await waitFor(() => expect(screen.getByText('1 个组件')).toBeInTheDocument());

    fireEvent.click(screen.getByRole('button', { name: /保存为提案$/ }));
    expect((await screen.findAllByText('提案已创建')).length).toBeGreaterThan(0);

    const saveBtn = screen.getByRole('button', { name: /保存为提案$/ });
    expect(saveBtn.className).toContain('ant-btn-loading');

    const modalRoot = document.querySelector('.ant-modal') as HTMLElement | null;
    expect(modalRoot).not.toBeNull();
    // confirm 型弹窗无关闭图标：keyboard 默认开 → ESC 走 onCancel
    fireEvent.keyDown(modalRoot!, { key: 'Escape' });
    await waitFor(() => expect(saveBtn.className).not.toContain('ant-btn-loading'));
    // onCancel 只复位 saving，不跳转（跳转属 onOk 路径）
    expect(umiMock.history.push).not.toHaveBeenCalled();
  });
});
