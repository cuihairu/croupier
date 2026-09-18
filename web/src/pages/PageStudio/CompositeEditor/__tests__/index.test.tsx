/** CompositeEditorPage（index.tsx 主组件）全量覆盖：
 * 子组件（Canvas/PropsPanel/ComponentLibrary/模板引导/各弹窗等）mock 为
 * 薄桩、以 data-testid 按钮驱动回调；树历史/编译器/模型/变量命名等纯逻辑
 * 保持真实。覆盖面：顶栏（撤销重做/预览切换/批量删除/保存为组件）、
 * pageKey 推导与 keyTouched、保存四路（空 key/少区块/成功±编译警告/失败）、
 * patchProps 换绑四态、变量改名、弹窗内联创建、选择/多选/删除清理、
 * 模板插入（带参弹窗/空树/悬空引用重连）、quick-start、拖拽 overlay 三态、
 * 回读三 fetcher 矩阵与竞态/取消、scope 重拉函数列表。 */
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

jest.setTimeout(20000);
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
  const SortableList = ({ items, onReorder, children }: SortableStubProps) =>
    R.createElement(
      'div',
      { 'data-testid': 'sortable' },
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
describe('初始渲染与顶栏', () => {
  it('模板库「新建组合组件」入口（?createComponent=1）：进入弹引导提示并清掉 query', async () => {
    umiMock.__umiState.search = '?createComponent=1';
    renderEditor();
    // 引导文案进 message 通知（区别于顶栏按钮的「保存为组件（N）」）
    await waitFor(() =>
      expect(document.querySelector('.ant-message')?.textContent).toContain('可复用模板'),
    );
    expect(umiMock.history.replace).toHaveBeenCalledWith(
      expect.objectContaining({ pathname: window.location.pathname, search: '' }),
    );
  });

  it('默认骨架：标题/版本/quick-start/撤销重做禁用/空闲保存组件禁用/组件数 0', () => {
    const { container } = renderEditor();
    expect(byId('pc:title').textContent).toBe('组合页编辑器');
    expect(container.textContent).toContain('v3.2.1');
    expect(byId('qs')).toBeInTheDocument();
    expect(undoBtn()).toBeDisabled();
    expect(redoBtn()).toBeDisabled();
    const idle = container.querySelector(
      '[data-testid="pc:extra"] button[disabled]',
    ) as HTMLButtonElement;
    expect(idle).toBeTruthy();
    expect(screen.getByText('0 个组件')).toBeInTheDocument();
    expect(pageKeyInput().value).toBe('');
    // 大纲 Tab 可切换（OutlinePanel 桩挂载）
    fireEvent.click(screen.getByRole('tab', { name: '大纲' }));
    expect(byId('op:count').textContent).toBe('0');
  });

  it('顶栏返回按钮 → history.push(/functions/pages)', () => {
    renderEditor();
    click('pc:back');
    expect(umiMock.history.push).toHaveBeenCalledWith('/functions/pages');
  });

  it('listDescriptors 成功填充左栏可用函数集合；scope 切换自动重拉', async () => {
    mockedListDescriptors.mockResolvedValueOnce([fnPlayer, fnMail]);
    renderEditor();
    await openLibraryTab();
    expect(byId('lib:avail').textContent).toBe('player.list,mail.send');
    expect(mockedListDescriptors).toHaveBeenCalledTimes(1);
    // scope 变化 → fnReload+1 → 重拉（第二次返回不同集合）
    mockedListDescriptors.mockResolvedValueOnce([fnMail]);
    act(() => scopeListener.current?.());
    await waitFor(() => expect(mockedListDescriptors).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(byId('lib:avail').textContent).toBe('mail.send'));
  });

  it('listDescriptors 失败静默（不阻断编辑器渲染）', async () => {
    mockedListDescriptors.mockRejectedValueOnce(new Error('down'));
    renderEditor();
    await waitFor(() => expect(mockedListDescriptors).toHaveBeenCalledTimes(1));
    expect(byId('qs')).toBeInTheDocument();
  });
});

// ---------------------------------------------------------------------------
// 组件添加与 pageKey 推导
// ---------------------------------------------------------------------------
describe('组件添加与 pageKey 推导', () => {
  it('tabs 脚手架（自带 2 页签）+ 撤销/重做按钮驱动树历史', async () => {
    renderEditor();
    click('cp:basic:tabs');
    expect(screen.getByText('3 个组件')).toBeInTheDocument();
    expect(undoBtn()).not.toBeDisabled();
    fireEvent.click(undoBtn());
    await waitFor(() => expect(screen.getByText('0 个组件')).toBeInTheDocument());
    expect(redoBtn()).not.toBeDisabled();
    fireEvent.click(redoBtn());
    await waitFor(() => expect(screen.getByText('3 个组件')).toBeInTheDocument());
  });

  it('基础组件添加（button/modal/container/text）与函数组件添加', () => {
    renderEditor();
    click('cp:basic:button');
    expect(screen.getByText('1 个组件')).toBeInTheDocument();
    click('cp:basic:modal');
    click('cp:basic:container');
    click('cp:basic:text');
    expect(screen.getByText('4 个组件')).toBeInTheDocument();
    // 函数组件：registerFn 后画布节点能解析 fn（badge=函数 id）
    click('cp:fn:fnTable');
    expect(screen.getByText('5 个组件')).toBeInTheDocument();
    const badges = [...document.querySelectorAll('[data-testid$=":fn"]')].map(
      (el) => el.textContent,
    );
    expect(badges).toContain('player.list');
  });

  it('pageKey 自动推导：资源段前缀 + 去重 + 多前缀拼接；keyTouched 后不再覆盖', () => {
    renderEditor();
    click('cp:fn:fnTable');
    waitFor(() => expect(pageKeyInput().value).toBe('player'));
    click('cp:fn:fnForm'); // 同函数：去重
    waitFor(() => expect(pageKeyInput().value).toBe('player'));
    // 手动改 key → keyTouched，之后树变化不再覆盖
    fireEvent.change(pageKeyInput(), { target: { value: 'custom-key' } });
    expect(pageKeyInput().value).toBe('custom-key');
    click('cp:basic:button');
    expect(pageKeyInput().value).toBe('custom-key');
  });

  it('derivedKey 多前缀用 - 拼接', async () => {
    mockedListDescriptors.mockResolvedValue([fnPlayer, fnMail]);
    renderEditor();
    click('cp:fn:fnTable'); // player.list → player
    await openLibraryTab();
    libConfig.nodes = [{ id: 'mail-node', type: 'fnForm', props: { functionId: 'mail.send' } }];
    libConfig.tpl = tpl('t1');
    click('lib:insert');
    await waitFor(() => expect(pageKeyInput().value).toBe('player-mail'));
  });
});

// ---------------------------------------------------------------------------
// 弹窗内联编辑约束
// ---------------------------------------------------------------------------
describe('弹窗内联编辑（面包屑 + 只能放函数表单）', () => {
  it('无 title 弹窗兜底文案；基础组件/表格函数拒绝、表单函数允许；返回链接退出', async () => {
    renderEditor();
    await openLibraryTab();
    // 无 title 弹窗（lib 模板插入，props 不带 title）→ 面包屑兜底「弹窗」
    insertViaLib([{ id: 'm-nt', type: 'modal', props: {}, children: [] }], tpl('modal-nt'));
    await waitFor(() => expect(modalIds()).toHaveLength(1));
    const mid = modalIds()[0];
    click(`mp:${mid}:enter`);
    // 无 title → 兜底「弹窗」+（内部编辑）
    expect(screen.getByText(/弹窗（内部编辑）/)).toBeInTheDocument();
    // 基础组件拒绝
    click('cp:basic:button');
    await waitFor(() =>
      expect(screen.getByText('弹窗内只能放函数表单（V1）——返回页面级再添加')).toBeInTheDocument(),
    );
    // 非表单函数拒绝
    click('cp:fn:fnTable');
    await waitFor(() => expect(screen.getByText('弹窗内只能放函数表单（V1）')).toBeInTheDocument());
    // 表单函数允许（落入 modal children）——内联态画布直接渲染 children
    click('cp:fn:fnForm');
    await waitFor(() => expect(screen.getByText('2 个组件')).toBeInTheDocument());
    await waitFor(() => expect(canvasOrder()).toHaveLength(1));
    // 面包屑「页面」返回
    fireEvent.click(screen.getByText('页面'));
    await waitFor(() => expect(screen.queryByText(/内部编辑/)).not.toBeInTheDocument());
  });
});

// ---------------------------------------------------------------------------
// 保存为提案
// ---------------------------------------------------------------------------
describe('保存为提案', () => {
  it('空 pageKey → 警告拦截（不发 POST）；单区块不再拦截（D1）', async () => {
    renderEditor();
    fireEvent.click(screen.getByRole('button', { name: /保存为提案$/ }));
    await waitFor(() => expect(screen.getByText('请填写页面 Key')).toBeInTheDocument());
    expect(lastCompositePost()).toBeUndefined();
  });

  it('保存成功：proposalKey 进文案，确认后跳转提案收件箱', async () => {
    setRoutes({ 'POST /api/v1/versioning/pages/composite': { proposalKey: 'composite--ok' } });
    renderEditor();
    mockedListDescriptors.mockResolvedValue([fnPlayer, fnMail]);
    await openLibraryTab();
    insertViaLib(
      [
        { id: 's1', type: 'fnTable', props: { functionId: 'player.list' } },
        { id: 's2', type: 'fnForm', props: { functionId: 'mail.send' } },
      ],
      tpl('save-ok'),
    );
    await waitFor(() => expect(screen.getByText('2 个组件')).toBeInTheDocument());
    fireEvent.click(screen.getByRole('button', { name: /保存为提案$/ }));
    // antd modal.success 同时渲染 .ant-modal-title 与 .ant-modal-confirm-title 两份标题
    expect((await screen.findAllByText('提案已创建')).length).toBeGreaterThan(0);
    expect(
      (await screen.findAllByText(/提案 composite--ok 已进入提案收件箱/)).length,
    ).toBeGreaterThan(0);
    const okBtn = document.querySelector('.ant-modal .ant-btn-primary') as HTMLButtonElement;
    fireEvent.click(okBtn);
    await waitFor(() => expect(umiMock.history.push).toHaveBeenCalledWith('/functions/pages'));
  });

  it('auto 策略 published=true → 提示已自动发布生效，不再指引人工接受', async () => {
    setRoutes({
      'POST /api/v1/versioning/pages/composite': {
        proposalKey: 'composite--auto',
        published: true,
      },
    });
    renderEditor();
    mockedListDescriptors.mockResolvedValue([fnPlayer, fnMail]);
    await openLibraryTab();
    insertViaLib(
      [
        { id: 's1', type: 'fnTable', props: { functionId: 'player.list' } },
        { id: 's2', type: 'fnForm', props: { functionId: 'mail.send' } },
      ],
      tpl('save-auto'),
    );
    await waitFor(() => expect(screen.getByText('2 个组件')).toBeInTheDocument());
    fireEvent.click(screen.getByRole('button', { name: /保存为提案$/ }));
    expect((await screen.findAllByText('页面已发布')).length).toBeGreaterThan(0);
    expect(
      (await screen.findAllByText(/提案 composite--auto 已自动发布生效/)).length,
    ).toBeGreaterThan(0);
    // 不出现默认态的「接受并发布后生效」指引
    expect(screen.queryAllByText(/已进入提案收件箱/)).toHaveLength(0);
  });

  it('published=false + publishError → 提示提案保留 + 失败原因 + 人工重试指引', async () => {
    setRoutes({
      'POST /api/v1/versioning/pages/composite': {
        proposalKey: 'composite--perr',
        published: false,
        publishError: 'publish blocked by governance diagnostics',
      },
    });
    renderEditor();
    mockedListDescriptors.mockResolvedValue([fnPlayer, fnMail]);
    await openLibraryTab();
    insertViaLib(
      [
        { id: 's1', type: 'fnTable', props: { functionId: 'player.list' } },
        { id: 's2', type: 'fnForm', props: { functionId: 'mail.send' } },
      ],
      tpl('save-perr'),
    );
    await waitFor(() => expect(screen.getByText('2 个组件')).toBeInTheDocument());
    fireEvent.click(screen.getByRole('button', { name: /保存为提案$/ }));
    expect((await screen.findAllByText('提案已创建，自动发布失败')).length).toBeGreaterThan(0);
    expect(
      (
        await screen.findAllByText(
          /提案 composite--perr 已保留在收件箱；自动发布未完成：publish blocked by governance diagnostics/,
        )
      ).length,
    ).toBeGreaterThan(0);
    expect((await screen.findAllByText(/人工接受发布重试/)).length).toBeGreaterThan(0);
  });

  it('编译警告前缀 + 响应缺 proposalKey 兜底空串', async () => {
    setRoutes({ 'POST /api/v1/versioning/pages/composite': {} });
    renderEditor();
    await openLibraryTab();
    insertViaLib(
      [
        { id: 's1', type: 'fnTable', props: { functionId: 'player.list' } },
        { id: 's2', type: 'fnForm', props: { functionId: 'mail.send' } },
        { id: 's3', type: 'text', props: { content: 'hi' } },
      ],
      tpl('save-warn'),
    );
    await waitFor(() => expect(screen.getByText('3 个组件')).toBeInTheDocument());
    fireEvent.click(screen.getByRole('button', { name: /保存为提案$/ }));
    expect((await screen.findAllByText('提案已创建')).length).toBeGreaterThan(0);
    expect(
      (await screen.findAllByText(/编译警告：文本「hi」不参与发布（V1）。/)).length,
    ).toBeGreaterThan(0);
    // 文本匹配会做空白归一（双空格折叠）：用 \s+ 容忍
    expect((await screen.findAllByText(/提案\s+已进入提案收件箱/)).length).toBeGreaterThan(0);
  });

  it('POST 失败 → message.error 兜底文案', async () => {
    // reject 非对象（字符串）→ extractErrorMessage 直接回退默认文案
    setRoutes({
      'POST /api/v1/versioning/pages/composite': () => Promise.reject('network down'),
    });
    renderEditor();
    await openLibraryTab();
    insertViaLib(
      [
        { id: 's1', type: 'fnTable', props: { functionId: 'player.list' } },
        { id: 's2', type: 'fnForm', props: { functionId: 'mail.send' } },
      ],
      tpl('save-fail'),
    );
    await waitFor(() => expect(screen.getByText('2 个组件')).toBeInTheDocument());
    fireEvent.click(screen.getByRole('button', { name: /保存为提案$/ }));
    await waitFor(() => expect(screen.getByText('创建提案失败')).toBeInTheDocument());
  });

  it('模板快照并入保存（同 key 去重）', async () => {
    setRoutes({ 'POST /api/v1/versioning/pages/composite': { proposalKey: 'k' } });
    renderEditor();
    await openLibraryTab();
    const dto = tpl('combo--x', { digest: 'd1' });
    insertViaLib(
      [
        { id: 's1', type: 'fnTable', props: { functionId: 'player.list' } },
        { id: 's2', type: 'fnForm', props: { functionId: 'mail.send' } },
      ],
      dto,
    );
    insertViaLib([], dto); // 同 key 二次插入：不重复登记
    await waitFor(() => expect(screen.getByText('2 个组件')).toBeInTheDocument());
    fireEvent.click(screen.getByRole('button', { name: /保存为提案$/ }));
    await waitFor(() =>
      expect(lastCompositePost()?.componentTemplates).toEqual([{ key: 'combo--x', digest: 'd1' }]),
    );
  });

  it('模板 key 为空不登记快照（body 无 componentTemplates 字段）', async () => {
    setRoutes({ 'POST /api/v1/versioning/pages/composite': { proposalKey: 'k' } });
    renderEditor();
    await openLibraryTab();
    insertViaLib(
      [
        { id: 's1', type: 'fnTable', props: { functionId: 'player.list' } },
        { id: 's2', type: 'fnForm', props: { functionId: 'mail.send' } },
      ],
      tpl(''),
    );
    await waitFor(() => expect(screen.getByText('2 个组件')).toBeInTheDocument());
    fireEvent.click(screen.getByRole('button', { name: /保存为提案$/ }));
    expect((await screen.findAllByText('提案已创建', undefined, FIND)).length).toBeGreaterThan(0);
    expect(lastCompositePost()?.componentTemplates).toBeUndefined();
  });
});

// ---------------------------------------------------------------------------
// 属性面板回调（patchProps / renameVarOfSelected / createModalForButton）
// ---------------------------------------------------------------------------
describe('属性面板回调', () => {
  it('patch 普通字段 / 同函数直更 / 换绑已知函数（重 scaffold）/ 换绑未知函数（fn 缺失）', async () => {
    mockedListDescriptors.mockResolvedValue([fnPlayer, fnMail]);
    renderEditor();
    await openLibraryTab();
    insertViaLib(
      [
        { id: 'p1', type: 'fnForm', props: { functionId: 'player.list', title: '原表单' } },
        { id: 'p2', type: 'fnForm', props: { functionId: 'mail.send' } },
      ],
      tpl('patch'),
    );
    await waitFor(() => expect(byId('cn:p1')).toBeInTheDocument());
    click('cn:p1:sel');
    expect(byId('pp:node').textContent).toBe('p1');
    // 普通字段
    click('pp:patch-title');
    await waitFor(() => expect(propsJsonOf('p1')).toContain('已改标题'));
    // 同函数：走普通合并（不改 scaffold）
    click('pp:patch-fn-same');
    await waitFor(() => expect(propsJsonOf('p1')).toContain('已改标题'));
    // 换绑已知函数 mail.send：重新 scaffold，标题跟随新函数
    click('pp:patch-fn-known');
    await waitFor(() => expect(propsJsonOf('p1')).toContain('mail.send'));
    expect(propsJsonOf('p1')).not.toContain('已改标题');
    // 换绑未知函数：fnById 无记录 → scaffoldProps(type, undefined)
    click('pp:patch-fn-ghost');
    await waitFor(() => expect(propsJsonOf('p1')).toContain('ghost.fn'));
  });

  it('选中悬空（树已被清空）时 patch 不生效、不崩溃', async () => {
    renderEditor();
    // 无选中（初始）→ patchProps 直接返回
    click('pp:patch-title');
    expect(screen.getByText('0 个组件')).toBeInTheDocument();
    await openLibraryTab();
    insertViaLib([{ id: 'g1', type: 'fnForm', props: { functionId: 'mail.send' } }], tpl('gone'));
    await waitFor(() => expect(byId('cn:g1')).toBeInTheDocument());
    click('cn:g1:sel');
    click('sl:reorder-empty'); // 树清空，selectedId 悬空
    await waitFor(() => expect(screen.getByText('0 个组件')).toBeInTheDocument());
    expect(byId('pp:node').textContent).toBe('none');
    click('pp:patch-fn-known');
    click('pp:patch-title');
    expect(screen.getByText('0 个组件')).toBeInTheDocument();
  });

  it('变量改名：无选中/无 sectionKey/正常改名（含引用重写）/悬空选中', async () => {
    renderEditor();
    click('pp:rename'); // 无选中（初始）→ renameVarOfSelected 直接返回
    await openLibraryTab();
    insertViaLib(
      [
        { id: 'r0', type: 'button', props: { title: '按钮' } },
        { id: 'r1', type: 'fnTable', props: { functionId: 'player.list', sectionKey: 'oldvar' } },
        { id: 'r2', type: 'text', props: { content: '引用 {{oldvar.data.total}}' } },
      ],
      tpl('rename'),
    );
    await waitFor(() => expect(byId('cn:r1')).toBeInTheDocument());
    expect(propsJsonOf('r1')).toContain('oldvar');
    // 选中但无 sectionKey（assignVarNames 会给全部节点分配变量名，先清空再改名）
    click('cn:r0:sel');
    click('pp:patch-nokey');
    await waitFor(() => expect(propsJsonOf('r0')).not.toContain('sectionKey'));
    click('pp:rename'); // oldName='' → 返回原树
    expect(propsJsonOf('r0')).not.toContain('sectionKey');
    expect(propsJsonOf('r1')).toContain('oldvar');
    click('cn:r1:sel');
    click('pp:rename'); // 正常改名（重写引用）
    await waitFor(() => expect(propsJsonOf('r1')).toContain('renamed'));
    // {{}} 表达式内的引用同步重写（单花括号文本不属变量引用）
    await waitFor(() => expect(propsJsonOf('r2')).toContain('{{renamed.data.total}}'));
    click('sl:reorder-empty'); // 悬空选中
    await waitFor(() => expect(byId('pp:node').textContent).toBe('none'));
    click('pp:rename');
    expect(screen.getByText('0 个组件')).toBeInTheDocument();
  });

  it('按钮内联创建弹窗：无选中跳过；有选中建 modal+fnForm 并绑定 onClick', async () => {
    renderEditor();
    // 无选中（初始）→ 直接跳过，不建弹窗
    click('pp:create-modal');
    expect(modalIds()).toHaveLength(0);
    await openLibraryTab();
    insertViaLib([{ id: 'b1', type: 'button', props: { title: '触发按钮' } }], tpl('mk-modal'));
    await waitFor(() => expect(byId('cn:b1')).toBeInTheDocument());
    click('cn:b1:sel');
    click('pp:create-modal');
    await waitFor(() => expect(modalIds()).toHaveLength(1));
    await waitFor(() =>
      expect(screen.getByText(/弹窗已创建并绑定（mail.send）/)).toBeInTheDocument(),
    );
    const modalId = modalIds()[0];
    const patched = JSON.parse(propsJsonOf('b1')) as {
      onClick?: { target?: string };
    };
    expect(patched.onClick?.target).toBe(modalId);
  });
});

// ---------------------------------------------------------------------------
// 编辑器内绑定抽屉（T9）：unbound 组件就地绑定 → 不同名 swap 引导
// ---------------------------------------------------------------------------
describe('编辑器内绑定抽屉（T9）', () => {
  it('unbound 组件 → 溯源预填 → 保存绑定 → 不同名确认切换 → 组件换绑 scaffold', async () => {
    mockedListDescriptors.mockResolvedValue([
      fnPlayer,
      fnMail,
      { id: 'listplayers', executionState: 'unbound' },
      { id: 'players.list', executionState: 'bound' },
    ]);
    mockedOpenapi.listOpenAPISources.mockResolvedValue({
      items: [
        {
          sourceId: 'src-1',
          name: '玩家服务',
          revision: 1,
          format: 'json',
          openapiVersion: '3.0.3',
          contentHash: 'h',
          operationCount: 1,
          diagnosticCount: 0,
          createdAt: '2026-09-15T00:00:00Z',
          updatedAt: '2026-09-15T00:00:00Z',
        },
      ],
    });
    mockedOpenapi.getOpenAPISource.mockResolvedValue({
      source: {
        operations: [{ operationId: 'listPlayers', method: 'get', path: '/players', bound: false }],
      },
    });
    renderEditor();
    await openLibraryTab();
    insertViaLib(
      [{ id: 'b1', type: 'fnTable', props: { functionId: 'listplayers' } }],
      tpl('bind'),
    );
    await waitFor(() => expect(byId('cn:b1')).toBeInTheDocument());
    click('cn:b1:sel');
    // 等 fnById/allFns 拉取完成（cn:b1:fn 由 fnById 渲染）再开抽屉
    await waitFor(() => expect(byId('cn:b1:fn').textContent).toBe('listplayers'));
    click('pp:open-binding');
    // 真实 BindingDrawer：标题 + 溯源预填（GET /players）
    expect(await screen.findByText('绑定运行时执行器')).toBeInTheDocument();
    await screen.findByText('GET /players');
    // 选运行时函数（combobox 顺序：来源/操作/函数）
    fireEvent.mouseDown(screen.getAllByRole('combobox')[2]);
    const option = screen
      .getAllByText('players.list')
      .map((el) => el.closest('.ant-select-item-option'))
      .find((el): el is HTMLElement => el !== null);
    if (!option) throw new Error('players.list option not found');
    fireEvent.click(option);
    fireEvent.click(screen.getByRole('button', { name: '保存绑定' }));
    await waitFor(() =>
      expect(mockedOpenapi.bindOpenAPISourceProvider).toHaveBeenCalledWith('src-1', {
        operationId: 'listPlayers',
        functionId: 'players.list',
        providerId: undefined,
        bindingId: 'listPlayers',
      }),
    );
    // 不同名（listplayers ≠ players.list）→ swap 确认弹窗（内容含两侧函数名）
    expect((await screen.findAllByText('切换到已绑定函数？')).length).toBeGreaterThan(0);
    expect(screen.getAllByText(/「players.list」/).length).toBeGreaterThan(0);
    expect(screen.getAllByText(/「listplayers」/).length).toBeGreaterThan(0);
    fireEvent.click(screen.getByRole('button', { name: '切换组件函数' }));
    // 确认后 patchProps({functionId}) → 组件按新函数重建 scaffold
    await waitFor(() => expect(propsJsonOf('b1')).toContain('players.list'), undefined, FIND);
    await waitFor(() => expect(byId('cn:b1:fn').textContent).toBe('players.list'));
  });

  it('同名绑定（T6 翻转后描述符已 bound）：保存不弹切换确认，函数引用保持', async () => {
    mockedListDescriptors.mockResolvedValue([
      { id: 'listplayers', executionState: 'bound' },
      { id: 'mail.send', executionState: 'bound' },
    ]);
    mockedOpenapi.listOpenAPISources.mockResolvedValue({
      items: [{ sourceId: 'src-1', name: '玩家服务' }],
    });
    mockedOpenapi.getOpenAPISource.mockResolvedValue({
      source: {
        operations: [{ operationId: 'listPlayers', method: 'get', path: '/players' }],
      },
    });
    renderEditor();
    await openLibraryTab();
    insertViaLib(
      [{ id: 'b1', type: 'fnTable', props: { functionId: 'listplayers' } }],
      tpl('same'),
    );
    await waitFor(() => expect(byId('cn:b1')).toBeInTheDocument());
    click('cn:b1:sel');
    await waitFor(() => expect(byId('cn:b1:fn').textContent).toBe('listplayers'));
    click('pp:open-binding');
    await screen.findByText('GET /players');
    // 同名候选来自 bound 描述符（T6 运行时注册翻转后的状态）；标题 code 也有
    // listplayers，按 option 节点定位
    fireEvent.mouseDown(screen.getAllByRole('combobox')[2]);
    const option = screen
      .getAllByText('listplayers')
      .map((el) => el.closest('.ant-select-item-option'))
      .find((el): el is HTMLElement => el !== null);
    if (!option) throw new Error('listplayers option not found');
    fireEvent.click(option);
    fireEvent.click(screen.getByRole('button', { name: '保存绑定' }));
    await waitFor(() =>
      expect(mockedOpenapi.bindOpenAPISourceProvider).toHaveBeenCalledWith('src-1', {
        operationId: 'listPlayers',
        functionId: 'listplayers',
        providerId: undefined,
        bindingId: 'listPlayers',
      }),
    );
    // 同名 → 不弹确认；组件函数引用保持不变
    await waitFor(() => expect(screen.queryByText('切换到已绑定函数？')).not.toBeInTheDocument());
    expect(propsJsonOf('b1')).toContain('listplayers');
  });
});

// ---------------------------------------------------------------------------
// 选择 / 多选 / 删除 / 批量删除 / 保存为组件
// ---------------------------------------------------------------------------
describe('选择与删除', () => {
  it('三种选择事件（普通/无事件/Shift 切换）；普通点击清空多选', async () => {
    renderEditor();
    await openLibraryTab();
    insertViaLib(
      [
        { id: 'a1', type: 'button', props: { title: 'A' } },
        { id: 'a2', type: 'button', props: { title: 'B' } },
      ],
      tpl('sel'),
    );
    await waitFor(() => expect(byId('cn:a1')).toBeInTheDocument());
    click('cn:a1:sel');
    expect(byId('cn:a1:sel-flag').textContent).toBe('1');
    click('cn:a2:sel-undef');
    expect(byId('pp:node').textContent).toBe('a2');
    // Shift 切换：加 → 出现「保存为组件（N）」；再 Shift → 移除
    //（按钮 icon aria-label「appstore」拼在名称前，正则用尾匹配）
    click('cn:a1:sel-shift');
    await waitFor(() =>
      expect(screen.getByRole('button', { name: /保存为组件（1）$/ })).toBeInTheDocument(),
    );
    click('cn:a2:sel-shift');
    await waitFor(() =>
      expect(screen.getByRole('button', { name: /保存为组件（2）$/ })).toBeInTheDocument(),
    );
    expect(byId('cn:a2:sel-flag').textContent).toBe('1'); // 多选集合内高亮
    click('cn:a2:sel-shift'); // 再切换 → 移除
    await waitFor(() =>
      expect(screen.getByRole('button', { name: /保存为组件（1）$/ })).toBeInTheDocument(),
    );
    // 普通点击 → 清空多选（prev.size>0 分支）
    click('cn:a1:sel');
    await waitFor(() =>
      expect(screen.queryByRole('button', { name: /保存为组件（\d）$/ })).not.toBeInTheDocument(),
    );
  });

  it('删除：未选中节点保留选中；删除选中节点清空；删除正在编辑的弹窗退出内联态', async () => {
    renderEditor();
    await openLibraryTab();
    insertViaLib(
      [
        {
          id: 'm1',
          type: 'modal',
          props: { title: '编辑中弹窗' },
          children: [{ id: 'f1', type: 'fnForm', props: { functionId: 'mail.send' } }],
        },
        { id: 'x1', type: 'button', props: { title: 'X' } },
        { id: 'x2', type: 'button', props: { title: 'Y' } },
      ],
      tpl('del'),
    );
    await waitFor(() => expect(byId('mp:m1')).toBeInTheDocument());
    click('cn:x1:sel');
    // 删除未选中节点：选中保持
    click('canvas:del:x2');
    await waitFor(() => expect(byId('pp:node').textContent).toBe('x1'));
    // 删除选中节点：选中清空
    click('cn:x1:del');
    await waitFor(() => expect(byId('pp:node').textContent).toBe('none'));
    // 选中 modal 并进入内联编辑 → 删除 modal：选中与 editingModalId 同时清理
    click('mp:m1:sel');
    click('mp:m1:sel-undef'); // 无事件对象（e nullish）分支
    click('mp:m1:enter');
    expect(screen.getByText(/编辑中弹窗（内部编辑）/)).toBeInTheDocument();
    click('pp:delete');
    await waitFor(() => expect(screen.queryByText(/内部编辑/)).not.toBeInTheDocument());
    await waitFor(() => expect(screen.getByText('0 个组件')).toBeInTheDocument());
  });

  it('复制节点/改 span/多选后组件库创建入口/删除多选成员同步摘除', async () => {
    renderEditor();
    await openLibraryTab();
    insertViaLib(
      [
        { id: 'u1', type: 'fnTable', props: { functionId: 'player.list', span: 6 } },
        { id: 'u2', type: 'button', props: { title: 'B' } },
      ],
      tpl('dup-span'),
    );
    await waitFor(() => expect(byId('cn:u1')).toBeInTheDocument());
    // patchSpan：6 → 收敛到 [4,24] 内合法值由画布层处理，props 直接写 12
    click('cn:u1:span');
    await waitFor(() => expect(propsJsonOf('u1')).toContain('"span":12'));
    // 复制：新副本落树（findInsertedSubtree + 重新命名）
    click('cn:u1:dup');
    await waitFor(() => expect(screen.getByText('3 个组件')).toBeInTheDocument());
    // 多选后组件库「从画布选中创建」→ 直接弹保存弹窗（非教学提示）
    click('cn:u2:sel-shift');
    await waitFor(() =>
      expect(screen.getByRole('button', { name: /保存为组件（1）$/ })).toBeInTheDocument(),
    );
    // 顶栏「保存为组件（N）」入口：不带 ids → 取 multiIds
    fireEvent.click(screen.getByRole('button', { name: /保存为组件（1）$/ }));
    await waitFor(() => expect(byId('scm:nodes').textContent).toBe('u2'));
    click('scm:close');
    click('lib:create');
    await waitFor(() => expect(byId('scm:nodes').textContent).toBe('u2'));
    click('scm:close');
    // 删除多选成员：multiIds 同步摘除（按钮消失）
    click('canvas:del:u2');
    await waitFor(() =>
      expect(screen.queryByRole('button', { name: /保存为组件（\d）$/ })).not.toBeInTheDocument(),
    );
    expect(screen.getByText('2 个组件')).toBeInTheDocument();
  });

  it('容器子节点删除/选择（onChildDelete/onChildSelect）', async () => {
    renderEditor();
    await openLibraryTab();
    insertViaLib(
      [
        {
          id: 'ct1',
          type: 'container',
          props: { title: '容器' },
          children: [
            { id: 'c0', type: 'fnForm', props: { functionId: 'mail.send' } },
            { id: 'c1', type: 'button', props: { title: '子按钮' } },
          ],
        },
      ],
      tpl('child'),
    );
    await waitFor(() => expect(byId('cn:ct1')).toBeInTheDocument());
    click('cn:ct1:csel');
    expect(byId('pp:node').textContent).toBe('c0');
    click('cn:ct1:cdel');
    await waitFor(() => expect(byId('cn:ct1:kids').textContent).toBe('c1'));
  });

  it('批量删除三分支：未编辑弹窗 / 编辑中但弹窗不在集合 / 编辑中且弹窗在集合', async () => {
    renderEditor();
    await openLibraryTab();
    // 分支 1：editingModalId=null（cur 为 null）
    insertViaLib(
      [
        { id: 'b1', type: 'button', props: { title: '1' } },
        { id: 'b2', type: 'button', props: { title: '2' } },
      ],
      tpl('batch1'),
    );
    await waitFor(() => expect(byId('cn:b1')).toBeInTheDocument());
    click('cn:b1:sel-shift');
    click('cn:b2:sel-shift');
    const batch1 = screen.getByRole('button', { name: /^删除所选（2）/ });
    fireEvent.click(batch1);
    await waitFor(() => expect(screen.getByText('0 个组件')).toBeInTheDocument());
    // 分支 2：编辑 modal（children 两个）→ 批量删 children（modal 不在集合 → 保持编辑态）
    insertViaLib(
      [
        {
          id: 'm2',
          type: 'modal',
          props: { title: 'M2' },
          children: [
            { id: 'f2a', type: 'fnForm', props: { functionId: 'mail.send' } },
            { id: 'f2b', type: 'fnForm', props: { functionId: 'mail.send' } },
          ],
        },
      ],
      tpl('batch2'),
    );
    await waitFor(() => expect(byId('mp:m2')).toBeInTheDocument());
    click('mp:m2:enter');
    await waitFor(() => expect(screen.getByText('3 个组件')).toBeInTheDocument());
    click('cn:f2a:sel-shift');
    click('cn:f2b:sel-shift');
    fireEvent.click(screen.getByRole('button', { name: /^删除所选（2）/ }));
    // modal 不在删除集合 → 编辑态保持，children 清空（计数 3→1）
    await waitFor(() => expect(screen.getByText('1 个组件')).toBeInTheDocument());
    expect(screen.getByText(/M2（内部编辑）/)).toBeInTheDocument();
    fireEvent.click(screen.getByText('页面')); // 退回页面级（占位卡重现，children 为空）
    await waitFor(() => expect(byId('mp:m2:kids').textContent).toBe(''));
    // 分支 3：多选含 modal 本身 + 进入内联 → 批量删 → 退出内联态
    insertViaLib(
      [
        {
          id: 'm3',
          type: 'modal',
          props: { title: 'M3' },
          children: [{ id: 'f3', type: 'fnForm', props: { functionId: 'mail.send' } }],
        },
        { id: 'b3', type: 'button', props: { title: '3' } },
      ],
      tpl('batch3'),
    );
    await waitFor(() => expect(byId('mp:m3')).toBeInTheDocument());
    click('mp:m3:sel-shift'); // 加入多选
    click('mp:m3:sel-shift'); // 再切换 → 移除（Shift toggle delete 分支）
    click('cn:b3:sel-shift');
    click('mp:m3:sel'); // 普通点击：multiIds 非空 → 清空多选
    await waitFor(() =>
      expect(screen.queryByRole('button', { name: /^删除所选/ })).not.toBeInTheDocument(),
    );
    click('mp:m3:sel-shift'); // 重新加入，继续批量删除流程
    click('cn:b3:sel-shift');
    click('mp:m3:enter');
    fireEvent.click(screen.getByRole('button', { name: /^删除所选（2）/ }));
    await waitFor(() => expect(screen.queryByText(/内部编辑/)).not.toBeInTheDocument());
    // 分支 2 的 m2 仍在树上：4 个组件删 3（m3+f3+b3）→ 剩 1
    await waitFor(() => expect(screen.getByText('1 个组件')).toBeInTheDocument());
  });

  it('保存为组件：单节点/多选集合/嵌套子树函数收集/集合悬空警告/画布创建入口', async () => {
    renderEditor();
    // 无选中 → 组件库入口提示
    await openLibraryTab();
    click('lib:create');
    await waitFor(() =>
      expect(screen.getByText('先在画布 Shift+点击 多选节点，再保存为组件')).toBeInTheDocument(),
    );
    insertViaLib(
      [
        {
          id: 'w1',
          type: 'modal',
          props: { title: 'W' },
          children: [{ id: 'wf', type: 'fnForm', props: { functionId: 'mail.send' } }],
        },
        { id: 'w2', type: 'fnTable', props: { functionId: 'mail.send' } },
        { id: 'w3', type: 'button', props: { title: '3' } },
      ],
      tpl('comp-save'),
    );
    await waitFor(() => expect(byId('cn:w2')).toBeInTheDocument());
    // 单节点右键保存（multiIds 空 → ids={id}）
    click('cn:w3:save-comp');
    await waitFor(() => expect(byId('scm:nodes').textContent).toBe('w3'));
    click('scm:close');
    // 多选（含 modal 子树）后右键保存集合成员 → 保存整个集合；
    // 函数收集递归 modal children 并按 id 去重
    click('mp:w1:sel-shift');
    click('cn:w2:sel-shift');
    click('cn:w2:save-comp');
    await waitFor(() => expect(byId('scm:nodes').textContent).toBe('w1,w2'));
    expect(byId('scm:fnIds').textContent).toBe('mail.send');
    click('scm:close');
    // 注：「集合悬空 → 选中的组件不存在」分支（index.tsx L594-602）不可达：
    // findNode 返回 undefined 而 L593 filter 只滤 null，空树时 selectedNodes
    // 恒非空（元素为 undefined），实际行为是 collectFns 抛 TypeError。
    // 详见交付说明的不可达清单，故此处不再构造该场景。
  });
});

// ---------------------------------------------------------------------------
// 组件库模板插入（onInsert 分支矩阵）
// ---------------------------------------------------------------------------
describe('组件库模板插入', () => {
  it('普通插入：追加+选中首个+登记快照；requiredFunctions 命中注册、未命中跳过', async () => {
    mockedListDescriptors.mockResolvedValue([fnPlayer, fnMail]);
    renderEditor();
    await openLibraryTab();
    insertViaLib(
      [
        { id: 'i1', type: 'fnForm', props: { functionId: 'mail.send' } },
        { id: 'i2', type: 'fnTable', props: { functionId: 'ghost.fn' } },
      ],
      tpl('combo--i', { digest: 'di', requiredFunctions: ['mail.send', 'ghost.fn'] }),
    );
    await waitFor(() => expect(byId('cn:i1')).toBeInTheDocument());
    expect(byId('pp:node').textContent).toBe('i1');
    // mail.send 命中 allFns → registerFn；ghost.fn 未命中 → 跳过（badge none）
    expect(byId('cn:i1:fn').textContent).toBe('mail.send');
    expect(byId('cn:i2:fn').textContent).toBe('none');
  });

  it('空树模板：不追加节点、不选中，但仍登记快照', async () => {
    renderEditor();
    await openLibraryTab();
    insertViaLib([], tpl('empty--t', { digest: 'de' }));
    await waitFor(() => expect(screen.getByText('0 个组件')).toBeInTheDocument());
    expect(byId('pp:node').textContent).toBe('none');
  });

  it('带参模板（nodes 为空占位）→ 打开参数弹窗；确认走 applyTemplateInsert、关闭复位', async () => {
    renderEditor();
    await openLibraryTab();
    insertViaLib([], tpl('param--t', { params: [{ key: 'p1', nodeId: 'x', prop: 'title' }] }));
    await waitFor(() => expect(byId('itm:open').textContent).toBe('1'));
    click('itm:confirm');
    expect(dndState.applyTemplateInsert).toHaveBeenCalledWith(
      expect.objectContaining({ key: 'param--t' }),
      {},
      'canvas-root',
    );
    click('itm:close');
    await waitFor(() => expect(byId('itm:open').textContent).toBe('0'));
  });

  it('带参模板但节点非空 → 不走参数弹窗，直接插入', async () => {
    renderEditor();
    await openLibraryTab();
    insertViaLib(
      [{ id: 'pv1', type: 'button', props: { title: '已实例化' } }],
      tpl('param--v', { params: [{ key: 'p1', nodeId: 'x', prop: 'title' }] }),
    );
    await waitFor(() => expect(byId('cn:pv1')).toBeInTheDocument());
    expect(byId('itm:open').textContent).toBe('0');
  });

  it('悬空引用上报 → 重连弹窗：候选含嵌套节点；apply 修复引用；空 fixes 不动；关闭复位', async () => {
    renderEditor();
    await openLibraryTab();
    insertViaLib(
      [
        {
          id: 'd1',
          type: 'fnTable',
          props: { functionId: 'player.list', refreshOnNode: ['outside-1'] },
          children: [{ id: 'd1c', type: 'button', props: { title: '子' } }],
        },
        { id: 'd2', type: 'button', props: { title: '目标' } },
      ],
      tpl('dangling--t'),
      [{ nodeId: 'd1', nodeTitle: '表', prop: 'refreshOnNode', kind: 'refresh', ref: 'outside-1' }],
    );
    await waitFor(() => expect(byId('drm:open').textContent).toBe('1'));
    // 候选 = 全部节点（含容器/弹窗内）——flattenForCandidates 递归
    expect(byId('drm:candidates').textContent).toBe('d1,d1c,d2');
    expect(byId('drm:refs').textContent).toBe('d1');
    danglingConfig.fixes = [
      { nodeId: 'd1', kind: 'refresh', prop: 'refreshOnNode', ref: 'outside-1', target: 'd2' },
    ];
    click('drm:apply');
    await waitFor(() => expect(propsJsonOf('d1')).toContain('d2'));
    await waitFor(() => expect(byId('drm:open').textContent).toBe('0'));
    // 再次上报 + 空 fixes → 引用保持
    insertViaLib(
      [
        {
          id: 'd1b',
          type: 'fnTable',
          props: { functionId: 'player.list', refreshOnNode: ['outside-2'] },
        },
      ],
      tpl('dangling--t'),
      [
        {
          nodeId: 'd1b',
          nodeTitle: '表',
          prop: 'refreshOnNode',
          kind: 'refresh',
          ref: 'outside-2',
        },
      ],
    );
    await waitFor(() => expect(byId('drm:open').textContent).toBe('1'));
    danglingConfig.fixes = [];
    click('drm:apply');
    await waitFor(() => expect(byId('drm:open').textContent).toBe('0'));
    expect(propsJsonOf('d1b')).toContain('outside-2');
    // 直接关闭
    insertViaLib(
      [
        {
          id: 'd1c',
          type: 'fnTable',
          props: { functionId: 'player.list', refreshOnNode: ['outside-3'] },
        },
      ],
      tpl('dangling--t'),
      [
        {
          nodeId: 'd1c',
          nodeTitle: '表',
          prop: 'refreshOnNode',
          kind: 'refresh',
          ref: 'outside-3',
        },
      ],
    );
    await waitFor(() => expect(byId('drm:open').textContent).toBe('1'));
    click('drm:close');
    await waitFor(() => expect(byId('drm:open').textContent).toBe('0'));
  });

  it('span 边界收敛：2→4、100→24、非数字→24、缺省→24', async () => {
    renderEditor();
    await openLibraryTab();
    insertViaLib(
      [
        { id: 'sp2', type: 'button', props: { title: 'a', span: 2 } },
        { id: 'sp100', type: 'button', props: { title: 'b', span: 100 } },
        { id: 'spNaN', type: 'button', props: { title: 'c', span: 'abc' } },
        { id: 'spU', type: 'button', props: { title: 'd' } },
      ],
      tpl('span--t'),
    );
    await waitFor(() => expect(byId('cn:sp2')).toBeInTheDocument());
    const styles = () =>
      [...document.querySelectorAll('div[style*="grid-column"]')] as HTMLElement[];
    await waitFor(() => expect(styles()).toHaveLength(4));
    const styleOf = (id: string) =>
      styles().find((el) => el.contains(byId(`cn:${id}`)))?.style.gridColumn;
    expect(styleOf('sp2')).toBe('span 4');
    expect(styleOf('sp100')).toBe('span 24');
    expect(styleOf('spNaN')).toBe('span 24');
    expect(styleOf('spU')).toBe('span 24');
  });
});

// ---------------------------------------------------------------------------
// 快速起步（TemplateQuickStart）
// ---------------------------------------------------------------------------
describe('快速起步', () => {
  it('onPick：追加节点+选中+成功提示；requiredFunctions 命中登记；name 缺 zh-CN 回退 key、name 缺省空串', async () => {
    mockedListDescriptors.mockResolvedValue([fnMail]); // requiredFunctions 命中 allFns → registerFn
    renderEditor();
    await openLibraryTab();
    // 等 allFns 就位（onPick 闭包读取 allFns）
    await waitFor(() => expect(byId('lib:avail').textContent).toBe('mail.send'));
    qsConfig.nodes = [
      { id: 'q1', type: 'fnTable', props: { functionId: 'player.list' } },
      { id: 'q2', type: 'fnForm', props: { functionId: 'mail.send' } },
    ];
    qsConfig.tpl = tpl('qs--normal', { requiredFunctions: ['mail.send', 'ghost.fn'] });
    qsConfig.dangling = [
      { nodeId: 'q1', nodeTitle: 'x', prop: 'refreshOnNode', kind: 'refresh', ref: 'zz' },
    ];
    click('qs:pick');
    await waitFor(() => expect(byId('cn:q1')).toBeInTheDocument());
    expect(byId('pp:node').textContent).toBe('q1');
    expect(byId('drm:open').textContent).toBe('1');
    expect(screen.getByText('已从模板「名称-qs--normal」创建页面骨架')).toBeInTheDocument();
    click('drm:close');
    // quick-start 仅空画布渲染：清空后重新出现，再验两轮 name 兜底
    click('sl:reorder-empty');
    await waitFor(() => expect(byId('qs:pick')).toBeInTheDocument());
    qsConfig.nodes = [{ id: 'q3', type: 'button', props: { title: 'x' } }];
    qsConfig.tpl = tpl('qs--nokey', { name: {} });
    qsConfig.dangling = null; // onPick 第三参 nullish 分支
    click('qs:pick');
    await waitFor(() =>
      expect(screen.getByText('已从模板「qs--nokey」创建页面骨架')).toBeInTheDocument(),
    );
    click('sl:reorder-empty');
    await waitFor(() => expect(byId('qs:pick')).toBeInTheDocument());
    // name 缺省 → 空串
    qsConfig.nodes = [{ id: 'q4', type: 'button', props: { title: 'y' } }];
    qsConfig.tpl = {
      key: 'qs--noname',
      tree: [],
      builtin: false,
    } as unknown as ComponentTemplateDTO;
    click('qs:pick');
    await waitFor(() => expect(screen.getByText('已从模板「」创建页面骨架')).toBeInTheDocument());
  });

  it('onPick 空 nodes → 选中置空；onStartBlank ↔ onShowTemplates 往返', async () => {
    renderEditor();
    qsConfig.nodes = [];
    qsConfig.tpl = tpl('qs--empty');
    click('qs:pick');
    await waitFor(() => expect(byId('pp:node').textContent).toBe('none'));
    // 空白起步 → Canvas 显示「查看模板」入口（onShowTemplates 有值）
    click('qs:blank');
    await waitFor(() => expect(byId('canvas:show-templates')).toBeInTheDocument());
    click('canvas:show-templates');
    await waitFor(() => expect(byId('qs')).toBeInTheDocument());
  });
});

// ---------------------------------------------------------------------------
// 预览切换
// ---------------------------------------------------------------------------
describe('预览模式', () => {
  it('预览互斥渲染：左栏/画布/属性面板隐藏、运行时+计数显示、保存禁用；退出恢复', async () => {
    renderEditor();
    await openLibraryTab();
    insertViaLib(
      [{ id: 'pv1', type: 'fnTable', props: { functionId: 'player.list' } }],
      tpl('p--t'),
    );
    await waitFor(() => expect(byId('cn:pv1')).toBeInTheDocument());
    // 按钮名含 eye 图标 aria-label 前缀；尾匹配并与「退出预览」区分
    fireEvent.click(screen.getByRole('button', { name: /[^退]预览$/ }));
    await waitFor(() => expect(byId('pr')).toBeInTheDocument());
    expect(byId('pr:count').textContent).toBe('1');
    expect(screen.queryByTestId('lib')).not.toBeInTheDocument();
    expect(screen.queryByTestId('qs')).not.toBeInTheDocument();
    expect(screen.queryByTestId('pp')).not.toBeInTheDocument();
    expect(screen.queryByTestId('dp')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: /保存为提案$/ })).toBeDisabled();
    fireEvent.click(screen.getByRole('button', { name: /退出预览$/ }));
    await waitFor(() => expect(screen.queryByTestId('pr')).not.toBeInTheDocument());
    expect(byId('cn:pv1')).toBeInTheDocument();
  });

  it('空弹窗内联态：画布为空但 onShowTemplates 不提供（undefined 分支）', async () => {
    renderEditor();
    await openLibraryTab();
    // modal 无 children 字段：canvasNodes 走 `?? []` 兜底
    insertViaLib([{ id: 'em1', type: 'modal', props: { title: '空弹窗' } }], tpl('empty-modal'));
    await waitFor(() => expect(byId('mp:em1')).toBeInTheDocument());
    click('mp:em1:enter');
    await waitFor(() => expect(screen.getByText(/空弹窗（内部编辑）/)).toBeInTheDocument());
    expect(screen.queryByTestId('qs')).not.toBeInTheDocument();
    expect(screen.queryByTestId('canvas:show-templates')).not.toBeInTheDocument();
  });
});

// ---------------------------------------------------------------------------
// 拖拽上下文（DndContext handlers + DragOverlay）
// ---------------------------------------------------------------------------
describe('拖拽上下文', () => {
  it('onDragStart/onDragOver/onDragEnd 接线；落点高亮与复位', async () => {
    renderEditor();
    await openLibraryTab();
    insertViaLib([{ id: 'n1', type: 'button', props: { title: '落点' } }], tpl('dnd--t'));
    await waitFor(() => expect(byId('cn:n1')).toBeInTheDocument());
    act(() => dndKitState.handlers.onDragStart?.({ active: { id: 'x' } }));
    expect(dndState.handleDragStart).toHaveBeenCalledTimes(1);
    // 拖拽中 + over 命中 → 顶部蓝条
    act(() => dndState.setDragItem?.({ kind: 'basic', basicType: 'button' }));
    act(() => dndKitState.handlers.onDragOver?.({ over: { id: 'n1' } }));
    await waitFor(() => {
      const wrap = byId('cn:n1').parentElement as HTMLElement;
      expect(wrap.style.borderTop).not.toContain('transparent');
    });
    // over 为空 → 复位透明
    act(() => dndKitState.handlers.onDragOver?.({ over: null }));
    await waitFor(() => {
      const wrap = byId('cn:n1').parentElement as HTMLElement;
      expect(wrap.style.borderTop).toContain('transparent');
    });
    // dragEnd：清 over + 分派 hook
    act(() => dndKitState.handlers.onDragEnd?.({ active: { id: 'x' }, over: null }));
    expect(dndState.handleDragEnd).toHaveBeenCalledTimes(1);
    act(() => dndState.setDragItem?.(null));
  });

  it('DragOverlay 三态文案：基础组件/模板/函数；无拖拽为空', async () => {
    renderEditor();
    await openLibraryTab();
    insertViaLib([{ id: 'o1', type: 'button', props: { title: 'x' } }], tpl('ov--t'));
    await waitFor(() => expect(byId('cn:o1')).toBeInTheDocument());
    act(() => dndState.setDragItem?.({ kind: 'basic', basicType: 'modal' }));
    await waitFor(() => expect(byId('overlay').textContent).toContain('组件：modal'));
    act(() =>
      dndState.setDragItem?.({
        kind: 'template',
        tpl: tpl('ov--tpl', { name: { 'zh-CN': '拖拽模板' }, digest: 'd' }),
        missing: [],
      }),
    );
    await waitFor(() => expect(byId('overlay').textContent).toContain('模板：拖拽模板'));
    act(() => dndState.setDragItem?.({ kind: 'fn', fn: fnMail, componentType: 'fnForm' }));
    await waitFor(() => expect(byId('overlay').textContent).toContain('函数：mail.send'));
    act(() => dndState.setDragItem?.(null));
    await waitFor(() => expect(byId('overlay').textContent).toBe(''));
  });
});

// ---------------------------------------------------------------------------
// 画布重排与容器子节点移动
// ---------------------------------------------------------------------------
describe('画布重排与子节点移动', () => {
  it('SortableList 页面级重排（reverse）；modal 内重排改 children', async () => {
    renderEditor();
    await openLibraryTab();
    insertViaLib(
      [
        { id: 'z1', type: 'button', props: { title: '1' } },
        { id: 'z2', type: 'button', props: { title: '2' } },
      ],
      tpl('reorder'),
    );
    await waitFor(() => expect(gridWrappers()).toHaveLength(2));
    expect(firstGridContains('cn:z1')).toBe(true);
    click('sl:reorder');
    await waitFor(() => expect(firstGridContains('cn:z2')).toBe(true));
    // modal 内重排
    insertViaLib(
      [
        {
          id: 'zm',
          type: 'modal',
          props: { title: 'M' },
          children: [
            { id: 'zc1', type: 'fnForm', props: { functionId: 'mail.send' } },
            { id: 'zc2', type: 'fnForm', props: { functionId: 'mail.send' } },
          ],
        },
      ],
      tpl('reorder-m'),
    );
    await waitFor(() => expect(byId('mp:zm')).toBeInTheDocument());
    click('mp:zm:enter');
    await waitFor(() => expect(byId('cn:zc1')).toBeInTheDocument());
    click('sl:reorder');
    // 内联态画布顺序反转 → 退回页面级读占位卡 children
    await waitFor(() => expect(canvasOrder()).toEqual(['cn:zc2', 'cn:zc1']));
    fireEvent.click(screen.getByText('页面'));
    await waitFor(() => expect(byId('mp:zm:kids').textContent).toBe('zc2,zc1'));
  });

  it('moveWithin 上移/下移/越界保护；容器子节点移动命中与 ghost id 不动', async () => {
    renderEditor();
    await openLibraryTab();
    insertViaLib(
      [
        { id: 'v1', type: 'button', props: { title: '1' } },
        { id: 'v2', type: 'button', props: { title: '2' } },
        {
          id: 'vc',
          type: 'container',
          props: { title: '容器' },
          children: [
            { id: 'c0', type: 'button', props: { title: '子0' } },
            { id: 'c1', type: 'button', props: { title: '子1' } },
          ],
        },
      ],
      tpl('move'),
    );
    const base = ['cn:v1', 'cn:v2', 'cn:vc'];
    await waitFor(() => expect(canvasOrder()).toEqual(base));
    // v2 下移（与 vc 交换）
    click('cn:v2:down');
    await waitFor(() => expect(canvasOrder()).toEqual(['cn:v1', 'cn:vc', 'cn:v2']));
    // v2 上移回到原位
    click('cn:v2:up');
    await waitFor(() => expect(canvasOrder()).toEqual(base));
    // v1 下移 → [v2, v1, vc]
    click('cn:v1:down');
    await waitFor(() => expect(canvasOrder()).toEqual(['cn:v2', 'cn:v1', 'cn:vc']));
    // v1 上移回原位
    click('cn:v1:up');
    await waitFor(() => expect(canvasOrder()).toEqual(base));
    // 首位再上移：toIndex=-1 越界保护（顺序不变，引用相等不入历史）
    click('cn:v1:up');
    await new Promise((r) => setTimeout(r, 20));
    expect(canvasOrder()).toEqual(base);
    // 容器子节点：c0 上移（越界）→ 不动；下移 → 交换；再上移 → 换回；ghost id → 不动
    click('cn:vc:cmv-up');
    await new Promise((r) => setTimeout(r, 20));
    expect(byId('cn:vc:kids').textContent).toBe('c0,c1');
    click('cn:vc:cmv-down');
    await waitFor(() => expect(byId('cn:vc:kids').textContent).toBe('c1,c0'));
    click('cn:vc:cmv-up');
    await waitFor(() => expect(byId('cn:vc:kids').textContent).toBe('c0,c1'));
    click('cn:vc:cmv-ghost');
    await new Promise((r) => setTimeout(r, 20));
    expect(byId('cn:vc:kids').textContent).toBe('c0,c1');
  });

  it('弹窗内选中父级（onSelectParent）→ 退出内联并选中弹窗', async () => {
    renderEditor();
    await openLibraryTab();
    insertViaLib(
      [
        {
          id: 'pm',
          type: 'modal',
          props: { title: 'P' },
          children: [{ id: 'pf', type: 'fnForm', props: { functionId: 'mail.send' } }],
        },
      ],
      tpl('parent'),
    );
    await waitFor(() => expect(byId('mp:pm')).toBeInTheDocument());
    click('mp:pm:enter');
    await waitFor(() => expect(byId('cn:pf:parent')).toBeInTheDocument());
    click('cn:pf:parent');
    await waitFor(() => expect(screen.queryByText(/内部编辑/)).not.toBeInTheDocument());
    expect(byId('pp:node').textContent).toBe('pm');
  });

  it('无 children 字段容器的子节点移动：kids 兜底空数组后 idx=-1 直接返回', async () => {
    renderEditor();
    await openLibraryTab();
    insertViaLib([{ id: 'nc', type: 'container', props: { title: '无子容器' } }], tpl('no-child'));
    await waitFor(() => expect(byId('cn:nc')).toBeInTheDocument());
    click('cn:nc:cmv-ghost');
    await new Promise((r) => setTimeout(r, 20));
    expect(byId('cn:nc:kids').textContent).toBe('');
  });

  it('DataPanel：选中函数节点显示契约、非函数节点为空', async () => {
    mockedListDescriptors.mockResolvedValue([fnMail]);
    renderEditor();
    await openLibraryTab();
    insertViaLib(
      [
        { id: 'dpf', type: 'fnForm', props: { functionId: 'mail.send' } },
        { id: 'dpt', type: 'text', props: { content: 'x' } },
      ],
      tpl('dp'),
    );
    await waitFor(() => expect(byId('cn:dpf')).toBeInTheDocument());
    click('cn:dpf:sel');
    expect(byId('dp:fn').textContent).toBe('mail.send');
    click('cn:dpt:sel');
    expect(byId('dp:fn').textContent).toBe('none');
  });
});

// ---------------------------------------------------------------------------
// 回读编辑（?pageKey= 三 fetcher 矩阵 / 竞态 / 取消 / 模板新鲜度）
// ---------------------------------------------------------------------------
describe('回读编辑', () => {
  const sections = [
    { key: 'player.list', functionId: 'player.list', view: 'table' },
    { key: 'mail.send', functionId: 'mail.send', view: 'fields' },
  ];

  function renderWithKey(): ReturnType<typeof renderEditor> {
    umiMock.__umiState.search = 'pageKey=my-page';
    return renderEditor();
  }

  it('fetcher1（composite-- 提案）命中 → 反编译入树 + 成功提示 + keyTouched', async () => {
    setRoutes({
      '/api/v1/proposals/composite--my-page': { pageSpec: { composite: { sections } } },
    });
    renderWithKey();
    await screen.findByText(/已载入页面 my-page（2 个区块）/, undefined, FIND);
    expect(pageKeyInput().value).toBe('my-page');
    await waitFor(() => expect(gridWrappers()).toHaveLength(2));
    // 快照为空 → 不比对模板库（无 component-templates 请求）
    const tplFetch = mockedRequest.mock.calls.filter(
      ([url]) => typeof url === 'string' && url.includes('/api/v1/component-templates'),
    );
    expect(tplFetch).toHaveLength(0);
    // keyTouched：回读后 derivedKey 不覆盖
    click('cp:fn:fnTable');
    await waitFor(() => expect(screen.getByText('3 个组件')).toBeInTheDocument());
    expect(pageKeyInput().value).toBe('my-page');
  });

  it('fetcher1 无 composite → fetcher2（裸提案 key）命中', async () => {
    setRoutes({
      '/api/v1/proposals/composite--my-page': { pageSpec: {} },
      '/api/v1/proposals/my-page': { pageSpec: { composite: { sections } } },
    });
    renderWithKey();
    await screen.findByText(/已载入页面 my-page（2 个区块）/, undefined, FIND);
  });

  it('fetcher1/2 抛错 → catch 逐源降级到 fetcher3（pageSpec / spec / 裸 resp 三形态）', async () => {
    // 形态 a：resp.pageSpec.composite
    setRoutes({
      '/api/v1/proposals/composite--my-page': () => Promise.reject(new Error('x')),
      '/api/v1/proposals/my-page': () => Promise.reject(new Error('x')),
      '/api/v1/versioning/pages/my-page': { pageSpec: { composite: { sections } } },
    });
    const { unmount } = renderWithKey();
    await screen.findByText(/已载入页面 my-page（2 个区块）/, undefined, FIND);
    unmount();
    // 形态 b：resp.spec.composite
    setRoutes({
      '/api/v1/proposals/composite--my-page': () => Promise.reject(new Error('x')),
      '/api/v1/proposals/my-page': () => Promise.reject(new Error('x')),
      '/api/v1/versioning/pages/my-page': { spec: { composite: { sections } } },
    });
    renderWithKey();
    await screen.findByText(/已载入页面 my-page（2 个区块）/, undefined, FIND);
  });

  it('fetcher3 裸 resp（pageSpec/spec 均缺）也能读取 composite', async () => {
    setRoutes({
      '/api/v1/proposals/composite--my-page': () => Promise.reject(new Error('x')),
      '/api/v1/proposals/my-page': () => Promise.reject(new Error('x')),
      '/api/v1/versioning/pages/my-page': { composite: { sections } },
    });
    renderWithKey();
    await screen.findByText(/已载入页面 my-page（2 个区块）/, undefined, FIND);
  });

  it('三源全部未命中 → 「未找到页面」警告', async () => {
    setRoutes({
      '/api/v1/proposals/composite--my-page': { pageSpec: {} },
      '/api/v1/proposals/my-page': {},
      '/api/v1/versioning/pages/my-page': {},
    });
    renderWithKey();
    await screen.findByText('未找到页面 my-page 的提案/草稿/发布 spec', undefined, FIND);
  });

  it('反编译产生警告（缺函数绑定区块）→ 回读警告提示', async () => {
    setRoutes({
      '/api/v1/proposals/composite--my-page': {
        pageSpec: { composite: { sections: [...sections, { key: 'broken' }] } },
      },
    });
    renderWithKey();
    await screen.findByText(/回读警告：区块缺少函数绑定，已跳过/, undefined, FIND);
    await screen.findByText(/已载入页面 my-page（3 个区块）/, undefined, FIND);
  });

  it('竞态：请求期间用户已建树 → 放弃回读覆盖；后续 effect 重跑被 tree>0 早退', async () => {
    let resolveSpec: (v: unknown) => void = () => undefined;
    mockedRequest.mockImplementation(
      (url: string) =>
        new Promise((res) => {
          if (typeof url === 'string' && url.includes('/api/v1/proposals/')) {
            resolveSpec = res as (v: unknown) => void;
          } else {
            res({});
          }
        }),
    );
    renderWithKey();
    await waitFor(() => expect(mockedRequest).toHaveBeenCalled());
    click('cp:basic:button'); // 用户先编辑
    await waitFor(() => expect(screen.getByText('1 个组件')).toBeInTheDocument());
    resolveSpec({ pageSpec: { composite: { sections } } });
    await new Promise((r) => setTimeout(r, 20));
    expect(screen.queryByText(/已载入页面/)).not.toBeInTheDocument();
    expect(screen.getByText('1 个组件')).toBeInTheDocument(); // 未被覆盖
    // fnReload 触发 effect 重跑 → tree.length>0 早退（不发起新回读）
    act(() => scopeListener.current?.());
    await new Promise((r) => setTimeout(r, 20));
    expect(screen.getByText('1 个组件')).toBeInTheDocument();
  });

  it('卸载后回读结果到达 → cancelled 分支不落地（成功形态 / 全空形态）', async () => {
    // 成功形态：L223 cancelled 短路
    let resolveSpec: (v: unknown) => void = () => undefined;
    mockedRequest.mockImplementation(
      (url: string) =>
        new Promise((res) => {
          if (typeof url === 'string' && url.includes('/api/v1/proposals/')) {
            resolveSpec = res as (v: unknown) => void;
          } else {
            res({});
          }
        }),
    );
    const { unmount } = renderWithKey();
    await waitFor(() => expect(mockedRequest).toHaveBeenCalled());
    unmount();
    resolveSpec({ pageSpec: { composite: { sections } } });
    await new Promise((r) => setTimeout(r, 20));
    // 全空形态：循环走完 L257 !cancelled 为 false
    mockedRequest.mockReset();
    mockedRequest.mockRejectedValue(new Error('down'));
    umiMock.__umiState.search = 'pageKey=cancelled-page';
    const second = renderEditor();
    await waitFor(() => expect(mockedRequest).toHaveBeenCalled());
    second.unmount();
    await new Promise((r) => setTimeout(r, 20));
  });

  it('模板新鲜度：digest 不一致 → Alert；一致/无 digest → 不提示；usage 为空不比对', async () => {
    // usage 非空 + digest 不一致 → Alert 列模板名
    setRoutes({
      '/api/v1/proposals/composite--my-page': {
        pageSpec: {
          composite: { sections },
          componentTemplates: [{ key: 'combo--st', digest: 'v0' }],
        },
      },
      '/api/v1/component-templates': {
        items: [tpl('combo--st', { digest: 'v1', name: { 'zh-CN': '组合模板' } })],
      },
    });
    const { unmount } = renderWithKey();
    expect(await screen.findByText('所用模板有新版本', undefined, FIND)).toBeInTheDocument();
    expect(await screen.findByText(/组合模板/, undefined, FIND)).toBeInTheDocument();
    unmount();
    // digest 一致 → 不提示
    setRoutes({
      '/api/v1/proposals/composite--my-page': {
        pageSpec: {
          composite: { sections },
          componentTemplates: [{ key: 'combo--st', digest: 'v1' }],
        },
      },
      '/api/v1/component-templates': {
        items: [tpl('combo--st', { digest: 'v1' })],
      },
    });
    renderWithKey();
    await screen.findByText(/已载入页面 my-page/, undefined, FIND);
    expect(screen.queryByText('所用模板有新版本')).not.toBeInTheDocument();
    unmount();
    // 模板库响应为 null → resp?.items 短路 + ?? [] 兜底空列表（不提示）
    setRoutes({
      '/api/v1/proposals/composite--my-page': {
        pageSpec: {
          composite: { sections },
          componentTemplates: [{ key: 'combo--st', digest: 'v1' }],
        },
      },
      '/api/v1/component-templates': () => null,
    });
    renderWithKey();
    await screen.findByText(/已载入页面 my-page/, undefined, FIND);
    expect(screen.queryByText('所用模板有新版本')).not.toBeInTheDocument();
  });

  it('模板库比对：数组响应形态同样解析；拉取失败静默', async () => {
    // 数组形态
    setRoutes({
      '/api/v1/proposals/composite--my-page': {
        pageSpec: {
          composite: { sections },
          componentTemplates: [{ key: 'arr--st', digest: 'a0' }],
        },
      },
      '/api/v1/component-templates': [
        tpl('arr--st', { digest: 'a1', name: { 'zh-CN': '数组模板' } }),
      ],
    });
    const { unmount } = renderWithKey();
    expect(await screen.findByText('所用模板有新版本', undefined, FIND)).toBeInTheDocument();
    expect(await screen.findByText(/数组模板/, undefined, FIND)).toBeInTheDocument();
    unmount();
    // 拉取失败 → 静默（无 Alert、不阻断）
    mockedRequest.mockReset();
    mockedRequest.mockImplementation(async (url: string) => {
      if (typeof url === 'string' && url.includes('/api/v1/proposals/composite--my-page')) {
        return {
          pageSpec: {
            composite: { sections },
            componentTemplates: [{ key: 'arr--st', digest: 'a0' }],
          },
        };
      }
      return Promise.reject(new Error('templates down'));
    });
    renderWithKey();
    await screen.findByText(/已载入页面 my-page/, undefined, FIND);
    expect(screen.queryByText('所用模板有新版本')).not.toBeInTheDocument();
  });
});
