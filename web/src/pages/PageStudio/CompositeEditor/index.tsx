import React, { useCallback, useMemo, useRef, useState, type SetStateAction } from 'react';
import { App, Button, Card, Col, Empty, Input, Row, Space, Tabs, Tag, Typography } from 'antd';
import { AppstoreOutlined, ArrowLeftOutlined, EyeOutlined, SaveOutlined } from '@ant-design/icons';
import { history, request, useSearchParams } from '@umijs/max';
import { subscribeScope } from '@/stores/scope';
import { PageContainer } from '@ant-design/pro-components';
import {
  DndContext,
  DragOverlay,
  PointerSensor,
  closestCenter,
  useSensor,
  useSensors,
  useDroppable,
  type DragEndEvent,
  type DragStartEvent,
} from '@dnd-kit/core';
import { sortableKeyboardCoordinates } from '@dnd-kit/sortable';
import { KeyboardSensor } from '@dnd-kit/core';
import { SortableList } from '@/components/SortableList';
import Canvas, { CanvasNode, ModalPlaceholder } from './Canvas';
import OutlinePanel from './OutlinePanel';
import DataPanel from './DataPanel';
import PreviewRuntime from './PreviewRuntime';
import { findParent } from './model';
import { compileTree, decompileToTree, type SpecSectionLike } from './compiler';
import { schemaProperties } from './types';
import { extractErrorMessage } from '@/utils/errors';
import { duplicateNode as duplicateTree, insertAfter, moveNode } from './model';
import ComponentPanel, { type AddFnEvent } from './ComponentPanel';
import ComponentLibrary from './ComponentLibrary';
import PropsPanel from './PropsPanel';
import { registerBuiltinComponents } from './components/builtin';
import { getComponent } from './registry';
import { listDescriptors, type FunctionDescriptor } from '@/services/api/functions';
import {
  countNodes,
  findNode,
  insertNode,
  nodeId,
  removeNode,
  updateProps,
  type PageNode,
} from './model';

const { Text } = Typography;

registerBuiltinComponents();

/** scaffold 按契约实例化节点 props。 */
function scaffoldProps(type: PageNode['type'], fn?: FunctionDescriptor): Record<string, unknown> {
  return getComponent(type)?.scaffold(fn) ?? {};
}

/**
 * 组合页编辑器 V3（组件化）：左=组件面板/大纲 Tabs，中=画布（组件树），
 * 右=属性面板（rjsf schema 驱动），顶栏=pageKey/预览切换/保存。
 * 页面状态是 PageNode 组件树；保存时编译为 CompositePageSpec（P4）。
 */
export default function CompositeEditorPage() {
  const { message, modal } = App.useApp();
  const [tree, setTreeState] = useState<PageNode[]>([]);
  // 撤销/重做历史（快照栈，最多 50 步）
  const [past, setPast] = useState<PageNode[][]>([]);
  const [future, setFuture] = useState<PageNode[][]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  /** Shift 多选集合（批量删除）。 */
  const [multiIds, setMultiIds] = useState<Set<string>>(new Set());
  const [pageKey, setPageKey] = useState('');
  const [keyTouched, setKeyTouched] = useState(false);
  const [mode, setMode] = useState<'edit' | 'preview'>('edit');
  const [leftTab, setLeftTab] = useState('components');
  /** 弹窗内嵌编辑（面包屑）：当前进入的 modal 节点 id。 */
  const [editingModalId, setEditingModalId] = useState<string | null>(null);
  const [searchParams] = useSearchParams();
  const loadKey = searchParams.get('pageKey');

  const fnById = useRef(new Map<string, FunctionDescriptor>());
  const [allFns, setAllFns] = useState<FunctionDescriptor[]>([]);
  const canvasRef = useRef<HTMLDivElement>(null);

  // 全量函数（属性面板换绑下拉）；scope 切换自动重拉
  const [fnReload, setFnReload] = useState(0);
  React.useEffect(() => {
    listDescriptors()
      .then((fns) => {
        setAllFns(fns);
        for (const f of fns) fnById.current.set(f.id, f);
      })
      .catch(() => undefined);
  }, [fnReload]);
  React.useEffect(() => subscribeScope(() => setFnReload((k) => k + 1)), []);

  // 回读编辑：?pageKey= 已有复合页 → 反编译为树（改完保存=同 proposalKey upsert）
  React.useEffect(() => {
    if (!loadKey || tree.length > 0) return;
    void (async () => {
      const fetchers: Array<() => Promise<{ sections?: SpecSectionLike[] } | undefined>> = [
        async () => {
          const resp = (await request(
            `/api/v1/proposals/${encodeURIComponent(`composite--${loadKey}`)}`,
            {
              skipErrorHandler: true,
            },
          )) as { pageSpec?: { composite?: { sections?: SpecSectionLike[] } } };
          return resp?.pageSpec?.composite;
        },
        async () => {
          const resp = (await request(`/api/v1/proposals/${encodeURIComponent(loadKey)}`, {
            skipErrorHandler: true,
          })) as { pageSpec?: { composite?: { sections?: SpecSectionLike[] } } };
          return resp?.pageSpec?.composite;
        },
        async () => {
          // draft/已发布页（无提案时）：GET /versioning/pages/:pageKey
          const resp = (await request(`/api/v1/versioning/pages/${encodeURIComponent(loadKey)}`, {
            skipErrorHandler: true,
          })) as Record<string, unknown>;
          const spec = (resp?.pageSpec ?? resp?.spec ?? resp) as
            { composite?: { sections?: SpecSectionLike[] } } | undefined;
          return spec?.composite;
        },
      ];
      for (const fetchSpec of fetchers) {
        try {
          const sections = (await fetchSpec())?.sections;
          if (!sections?.length) continue;
          const [nodes, warnings] = decompileToTree(sections);
          // 函数契约登记（fnById 供画布/属性面板）
          setTree(nodes);
          setPageKey(loadKey);
          setKeyTouched(true);
          if (warnings.length) message.warning(`回读警告：${warnings.join('；')}`);
          message.success(`已载入页面 ${loadKey}（${sections.length} 个区块）`);
          return;
        } catch {
          // 尝试下一个数据源
        }
      }
      message.warning(`未找到页面 ${loadKey} 的提案/草稿/发布 spec`);
    })();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [loadKey, fnReload]);
  const [, forceFn] = useState(0);
  const registerFn = useCallback((fn: FunctionDescriptor) => {
    fnById.current.set(fn.id, fn);
    forceFn((n) => n + 1);
  }, []);

  const selected = useMemo(() => findNode(tree, selectedId ?? ''), [tree, selectedId]);
  const editingModal = useMemo(
    () => (editingModalId ? (tree.find((n) => n.id === editingModalId) ?? null) : null),
    [tree, editingModalId],
  );
  /** 当前画布渲染的节点列表：弹窗级=其 children；页面级=全树。 */
  const canvasNodes = editingModal ? (editingModal.children ?? []) : tree;

  /** 函数 → 组件节点（scaffold 按契约实例化，amis 式拖入即骨架）。 */
  /** 子节点放入容器（modal children）。 */
  const addChild = useCallback((parentId: string, node: PageNode) => {
    setTree((prev) => insertNode(prev, node, parentId));
    setSelectedId(node.id);
  }, []);

  const addFunction = useCallback(
    (e: AddFnEvent) => {
      registerFn(e.fn);
      const node: PageNode = {
        id: nodeId(e.componentType),
        type: e.componentType,
        props: scaffoldProps(e.componentType, e.fn),
      };
      if (editingModalId) {
        if (node.type !== 'fnForm') {
          message.warning('弹窗内只能放函数表单（V1）');
          return;
        }
        addChild(editingModalId, node);
        return;
      }
      setTree((prev) => [...prev, node]);
      setSelectedId(node.id);
    },
    [registerFn, editingModalId, addChild, message],
  );

  /** 基础组件 → 节点。 */
  const addBasic = useCallback(
    (type: 'button' | 'modal' | 'container' | 'text') => {
      const node: PageNode = { id: nodeId(type), type, props: scaffoldProps(type) };
      if (editingModalId) {
        message.warning('弹窗内只能放函数表单（V1）——返回页面级再添加');
        return;
      }
      setTree((prev) => [...prev, node]);
      setSelectedId(node.id);
    },
    [editingModalId, message],
  );

  // pageKey 自动推导（函数 id 资源段）
  const derivedKey = useMemo(
    () =>
      tree
        .map((n) => String(n.props.functionId ?? ''))
        .filter(Boolean)
        .map((fid) => fid.split('.')[0])
        .filter(Boolean)
        .filter((v, i, a) => a.indexOf(v) === i)
        .join('-'),
    [tree],
  );
  React.useEffect(() => {
    if (!keyTouched) setPageKey((prev) => (prev === derivedKey ? prev : derivedKey));
  }, [derivedKey, keyTouched]);

  const [saving, setSaving] = useState(false);
  const save = useCallback(async () => {
    const key = pageKey.trim();
    if (!key) {
      message.warning('请填写页面 Key');
      return;
    }
    const { sections, warnings } = compileTree(tree);
    if (sections.length < 2) {
      message.warning('组合页至少需要 2 个函数区块（当前有效的 ' + sections.length + ' 个）');
      return;
    }
    setSaving(true);
    try {
      const resp = (await request('/api/v1/versioning/pages/composite', {
        method: 'POST',
        data: { pageKey: key, sections },
      })) as { proposalKey?: unknown };
      modal.success({
        title: '提案已创建',
        content:
          (warnings.length ? `编译警告：${warnings.join('；')}。提案 ` : '提案 ') +
          String(resp?.proposalKey ?? '') +
          ' 已进入提案收件箱，接受并发布后生效。',
        onOk: () => history.push('/functions/pages'),
      });
    } catch (err) {
      message.error(extractErrorMessage(err, '创建提案失败'));
    } finally {
      setSaving(false);
    }
  }, [pageKey, tree, message, modal]);

  const patchProps = useCallback(
    (patch: Record<string, unknown>) => {
      if (!selectedId) return;
      // 换绑函数：重新 scaffold（列/字段/联动跟随新函数）
      if (typeof patch.functionId === 'string') {
        const node = findNode(tree, selectedId);
        if (node && patch.functionId !== node.props.functionId) {
          const fn = fnById.current.get(patch.functionId);
          setTree((prev) =>
            updateProps(prev, selectedId, {
              ...scaffoldProps(node.type, fn),
              functionId: patch.functionId,
            }),
          );
          return;
        }
      }
      setTree((prev) => updateProps(prev, selectedId, patch));
    },
    [selectedId, tree],
  );

  // ---- 拖拽（T2.2/T2.3）：面板→画布插入 / 画布内重排 / modal 收纳 ----
  const [dragItem, setDragItem] = useState<
    | null
    | { kind: 'basic'; basicType: string }
    | { kind: 'fn'; fn: AddFnEvent['fn']; componentType: AddFnEvent['componentType'] }
  >(null);
  const treeRef = useRef(tree);
  treeRef.current = tree;
  const editingModalRef = useRef<string | null>(null);
  editingModalRef.current = editingModalId;

  /** history-aware setTree：所有树变更统一入口（撤销/重做安全网）。
   * 函数式 action 基于 treeRef 求值（避免 setState updater 内副作用
   * 在 StrictMode 双调用下重复入栈）。 */
  const setTree = useCallback((action: SetStateAction<PageNode[]>) => {
    const next =
      typeof action === 'function'
        ? (action as (prev: PageNode[]) => PageNode[])(treeRef.current)
        : action;
    if (next === treeRef.current) return;
    setPast((p) => [...p.slice(-49), treeRef.current]);
    setFuture(() => []);
    setTreeState(next);
    treeRef.current = next;
  }, []);

  const undo = useCallback(() => {
    if (past.length === 0) return;
    const prev = past[past.length - 1];
    setPast((p) => p.slice(0, -1));
    setFuture((f) => [treeRef.current, ...f]);
    setTreeState(prev);
    treeRef.current = prev;
  }, [past.length, past]);

  const redo = useCallback(() => {
    if (future.length === 0) return;
    const next = future[0];
    setFuture((f) => f.slice(1));
    setPast((p) => [...p, treeRef.current]);
    setTreeState(next);
    treeRef.current = next;
  }, [future.length, future]);

  // 快捷键：Ctrl/Cmd+Z 撤销、Ctrl/Cmd+Shift+Z / Ctrl+Y 重做
  React.useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (!(e.ctrlKey || e.metaKey) || e.key.toLowerCase() !== 'z') return;
      e.preventDefault();
      if (e.shiftKey) redo();
      else undo();
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [undo, redo]);

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 4 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  );

  const [overNodeId, setOverNodeId] = useState<string | null>(null);

  const handleDragStart = useCallback((e: DragStartEvent) => {
    const data = e.active.data.current as
      | { source: 'panel'; kind: 'basic'; basicType: string }
      | {
          source: 'panel';
          kind: 'fn';
          fn: AddFnEvent['fn'];
          componentType: AddFnEvent['componentType'];
        }
      | { source: 'canvas' }
      | undefined;
    if (data?.source === 'panel') {
      setDragItem(
        data.kind === 'basic'
          ? { kind: 'basic', basicType: data.basicType }
          : { kind: 'fn', fn: data.fn, componentType: data.componentType },
      );
    } else {
      setDragItem(null);
    }
  }, []);

  const handleDragEnd = useCallback(
    (e: DragEndEvent) => {
      setDragItem(null);
      const { active, over } = e;
      if (!over) return;
      const data = active.data.current as
        | { source: 'panel'; kind: 'basic'; basicType: string }
        | {
            source: 'panel';
            kind: 'fn';
            fn: AddFnEvent['fn'];
            componentType: AddFnEvent['componentType'];
          }
        | { source: 'canvas' }
        | undefined;
      const overId = String(over.id);

      if (data?.source === 'panel') {
        // 构造新节点
        let node: PageNode | null = null;
        if (data.kind === 'basic') {
          node = {
            id: nodeId(data.basicType as PageNode['type']),
            type: data.basicType as PageNode['type'],
            props: scaffoldProps(data.basicType as PageNode['type']),
          };
        } else {
          registerFn(data.fn);
          node = {
            id: nodeId(data.componentType),
            type: data.componentType,
            props: scaffoldProps(data.componentType, data.fn),
          };
        }
        // 弹窗占位卡 drop：fnForm 装入 modal
        if (overId.startsWith('modal-drop:')) {
          const modalId = overId.slice('modal-drop:'.length);
          if (node.type === 'fnForm') addChild(modalId, node);
          else message.warning('弹窗内只能放函数表单（V1）');
          return;
        }
        // 弹窗级编辑中：面板加入的节点落到当前弹窗 children（仅表单）
        if (editingModalRef.current) {
          if (node.type !== 'fnForm') {
            message.warning('弹窗内只能放函数表单（V1）');
            return;
          }
          addChild(editingModalRef.current, node);
          return;
        }
        // 落点=容器节点 → 装入 children；其余=节点之后；根=末尾
        const after = overId === 'canvas-root' ? undefined : findNode(treeRef.current, overId);
        if (after?.type === 'container') {
          addChild(after.id, node);
        } else {
          setTree((prev) => (after ? insertAfter(prev, node, after.id) : [...prev, node]));
        }
        setSelectedId(node.id);
        return;
      }

      // 画布内重排（active id = sortable 节点 id）
      if (overId === 'canvas-root') return;
      const activeId = String(active.id);
      const dragList = editingModalRef.current
        ? (treeRef.current.find((n) => n.id === editingModalRef.current)?.children ?? [])
        : treeRef.current;
      const overIdx = dragList.findIndex((n) => n.id === overId);
      if (overIdx === -1) return;
      setTree((prev) => {
        if (editingModalRef.current) {
          const m = prev.find((n) => n.id === editingModalRef.current);
          const kids = m?.children ?? [];
          const moved = moveNode(kids, activeId, overIdx);
          if (moved === kids) return prev;
          return prev.map((n) => (n.id === m?.id ? { ...n, children: moved } : n));
        }
        const moved = moveNode(prev, activeId, overIdx);
        return moved === prev ? prev : moved;
      });
    },
    [addChild, message, registerFn],
  );

  const duplicateNode = useCallback((id: string) => {
    setTree((prev) => duplicateTree(prev, id));
  }, []);

  const patchSpan = useCallback((id: string, span: number) => {
    setTree((prev) => updateProps(prev, id, { span }));
  }, []);

  /** 当前画布列表内上移/下移（右键菜单）。 */
  const moveWithin = useCallback(
    (list: PageNode[], id: string, dir: -1 | 1) => {
      const idx = list.findIndex((n) => n.id === id);
      if (idx === -1) return;
      setTree(() => moveNode(list, id, idx + dir));
    },
    [setTree],
  );

  /** 按钮动作内联创建弹窗：建 modal（装入 fn 表单）→ 绑定当前按钮 onClick。 */
  const createModalForButton = useCallback(
    (fn: FunctionDescriptor) => {
      if (!selectedId) return;
      registerFn(fn);
      const form: PageNode = {
        id: nodeId('fnForm'),
        type: 'fnForm',
        props: scaffoldProps('fnForm', fn),
      };
      const modal: PageNode = {
        id: nodeId('modal'),
        type: 'modal',
        props: { title: fn.summary?.['zh-CN'] || fn.id, width: 'medium' },
        children: [form],
      };
      setTree((prev) => [...prev, modal]);
      setTree((prev) =>
        updateProps(prev, selectedId, { onClick: { kind: 'openModal', target: modal.id } }),
      );
      message.success(`弹窗已创建并绑定（${fn.id}）——可双击弹窗卡片编辑内部`);
    },
    [selectedId, registerFn, message, setTree],
  );

  /** 多选节点保存为组件模板（V4：用户自定义组件）。 */
  const saveSelectionAsComponent = useCallback(async () => {
    if (multiIds.size < 1) {
      message.warning('请先选中至少一个组件');
      return;
    }
    const selectedNodes = tree.filter((n) => multiIds.has(n.id));
    const fnIds: string[] = [];
    const collectFns = (nodes: PageNode[]) => {
      for (const n of nodes) {
        const fid = String(n.props.functionId ?? '');
        if (fid && !fnIds.includes(fid)) fnIds.push(fid);
        if (n.children) collectFns(n.children);
      }
    };
    collectFns(selectedNodes);

    try {
      const key = `custom--${Date.now().toString(36)}`;
      await request('/api/v1/component-templates', {
        method: 'POST',
        data: {
          key,
          name: { 'zh-CN': `自定义组件 ${selectedNodes.length} 节点` },
          description: { 'zh-CN': `${selectedNodes.length} 个组件的组合` },
          category: '自定义',
          icon: 'AppstoreOutlined',
          requiredFunctions: fnIds,
          tree: selectedNodes,
        },
        skipErrorHandler: true,
      });
      message.success(`组件已保存——在组件库中可复用`);
    } catch {
      message.error('保存失败');
    }
  }, [multiIds, tree, message]);

  const deleteNode = useCallback((id: string) => {
    setTree((prev) => removeNode(prev, id)[0]);
    setSelectedId((cur) => (cur === id ? null : cur));
  }, []);

  const preview = mode === 'preview';

  return (
    <PageContainer
      header={{
        title: '组合页编辑器',
        onBack: () => history.push('/functions/pages'),
        backIcon: <ArrowLeftOutlined />,
        extra: [
          <Button
            key="undo"
            disabled={preview || past.length === 0}
            onClick={undo}
            title="撤销 (Ctrl+Z)"
          >
            ↩
          </Button>,
          <Button
            key="redo"
            disabled={preview || future.length === 0}
            onClick={redo}
            title="重做 (Ctrl+Shift+Z)"
          >
            ↪
          </Button>,
          <Button
            key="mode"
            type={preview ? 'primary' : 'default'}
            icon={<EyeOutlined />}
            onClick={() => setMode(preview ? 'edit' : 'preview')}
          >
            {preview ? '退出预览' : '预览'}
          </Button>,
          multiIds.size > 0 && (
            <Button
              key="save-component"
              icon={<AppstoreOutlined />}
              onClick={() => void saveSelectionAsComponent()}
            >
              保存为组件（{multiIds.size}）
            </Button>
          ),
          multiIds.size > 1 && (
            <Button
              key="batch-del"
              danger
              onClick={() => {
                setTree((prev) => {
                  let next = prev;
                  for (const id of multiIds) next = removeNode(next, id)[0];
                  return next;
                });
                setMultiIds(new Set());
                setSelectedId(null);
              }}
            >
              删除所选（{multiIds.size}）
            </Button>
          ),
          <Button
            key="save"
            type="primary"
            icon={<SaveOutlined />}
            loading={saving}
            disabled={preview}
            onClick={() => void save()}
          >
            保存为提案
          </Button>,
        ],
      }}
    >
      <Space wrap style={{ marginBottom: 12 }}>
        <Text type="secondary" style={{ fontSize: 10 }}>
          v3.2.1
        </Text>
        <Text strong>页面 Key</Text>
        <Input
          placeholder="按组件自动生成，可修改"
          value={pageKey}
          onChange={(e) => {
            setKeyTouched(true);
            setPageKey(e.target.value);
          }}
          style={{ width: 320 }}
        />
        <Text type="secondary">{countNodes(tree)} 个组件</Text>
      </Space>

      <DndContext
        sensors={sensors}
        collisionDetection={closestCenter}
        onDragStart={handleDragStart}
        onDragOver={(e) => setOverNodeId(e.over ? String(e.over.id) : null)}
        onDragEnd={(e) => {
          setOverNodeId(null);
          handleDragEnd(e);
        }}
      >
        <Row gutter={12}>
          {/* 左：组件面板 / 大纲 */}
          {!preview && (
            <Col flex="300px">
              <Card size="small" styles={{ body: { padding: 8 } }}>
                <Tabs
                  activeKey={leftTab}
                  onChange={setLeftTab}
                  items={[
                    {
                      key: 'library',
                      label: '组件库',
                      children: (
                        <ComponentLibrary
                          availableFnIds={new Set(allFns.map((f) => f.id))}
                          onInsert={(nodes, tpl) => {
                            setTree((prev) => [...prev, ...nodes]);
                            for (const fid of tpl.requiredFunctions ?? []) {
                              const fn = allFns.find((f) => f.id === fid);
                              if (fn) registerFn(fn);
                            }
                            if (nodes.length > 0) setSelectedId(nodes[0].id);
                          }}
                        />
                      ),
                    },
                    {
                      key: 'components',
                      label: '函数',
                      children: (
                        <ComponentPanel onAddBasic={addBasic} onAddFunction={addFunction} />
                      ),
                    },
                    {
                      key: 'outline',
                      label: '大纲',
                      children: (
                        <OutlinePanel
                          tree={tree}
                          selectedId={selectedId}
                          onSelect={setSelectedId}
                        />
                      ),
                    },
                  ]}
                />
              </Card>
            </Col>
          )}

          {/* 中：预览=发布形态（真实执行/弹窗/刷新动作）；编辑=画布 */}
          <Col flex="auto" style={{ minWidth: 420 }}>
            {preview ? (
              <PreviewRuntime tree={tree} fnById={fnById.current} />
            ) : (
              <div
                ref={canvasRef}
                style={{
                  border: '1px solid #f0f0f0',
                  borderRadius: 8,
                  minHeight: 'calc(100vh - 300px)',
                  padding: 12,
                  background: '#fafafa',
                }}
              >
                {editingModal && (
                  <div style={{ marginBottom: 8 }}>
                    <Space size={8}>
                      <a onClick={() => setEditingModalId(null)} style={{ fontSize: 12 }}>
                        页面
                      </a>
                      <span style={{ fontSize: 12 }}>/</span>
                      <Text strong style={{ fontSize: 12 }}>
                        {String(editingModal.props.title ?? '弹窗')}（内部编辑）
                      </Text>
                      <Text type="secondary" style={{ fontSize: 11 }}>
                        从左栏拖入/点击添加表单；拖拽排序
                      </Text>
                    </Space>
                  </div>
                )}
                <Canvas
                  tree={canvasNodes}
                  selectedId={selectedId}
                  fnById={fnById.current}
                  onSelect={setSelectedId}
                  onDelete={deleteNode}
                  onDuplicate={duplicateNode}
                  onSpanChange={patchSpan}
                  onEnterModal={setEditingModalId}
                  canvasWidthRef={canvasRef}
                >
                  <div
                    style={{
                      display: 'flex',
                      flexWrap: 'wrap',
                      gap: 12,
                      alignContent: 'flex-start',
                    }}
                  >
                    <SortableList
                      items={canvasNodes}
                      getKey={(n) => n.id}
                      onReorder={(next) => {
                        if (editingModal) {
                          setTree((prev) =>
                            prev.map((n) =>
                              n.id === editingModal.id ? { ...n, children: next } : n,
                            ),
                          );
                        } else {
                          setTree(next);
                        }
                      }}
                      externalDnd
                    >
                      {(n, _idx, dragHandleProps) => {
                        const spanNum = Math.min(24, Math.max(4, Number(n.props.span ?? 24) || 24));
                        return (
                          <div
                            key={n.id}
                            style={{
                              gridColumn: `span ${spanNum}`,
                              borderTop:
                                dragItem && overNodeId === n.id
                                  ? '3px solid #1677ff'
                                  : '3px solid transparent',
                              transition: 'border-color 0.1s',
                            }}
                          >
                            {n.type === 'modal' ? (
                              <div onClick={(e) => e.stopPropagation()}>
                                <ModalPlaceholder
                                  modal={n}
                                  selected={selectedId === n.id}
                                  fnById={fnById.current}
                                  onSelect={() => {
                                    setSelectedId(n.id);
                                    setMultiIds((prev) => {
                                      const next = new Set(prev);
                                      if (next.has(n.id)) next.delete(n.id);
                                      else next.add(n.id);
                                      return next;
                                    });
                                  }}
                                  onEnterModal={() => setEditingModalId(n.id)}
                                />
                              </div>
                            ) : (
                              <CanvasNode
                                node={n}
                                fn={
                                  n.props.functionId
                                    ? fnById.current.get(String(n.props.functionId))
                                    : undefined
                                }
                                selected={selectedId === n.id || multiIds.has(n.id)}
                                depth={0}
                                onSelect={() => {
                                  setSelectedId(n.id);
                                  setMultiIds((prev) => {
                                    const next = new Set(prev);
                                    if (next.has(n.id)) next.delete(n.id);
                                    else next.add(n.id);
                                    return next;
                                  });
                                }}
                                onDelete={() => deleteNode(n.id)}
                                onDuplicate={() => duplicateNode(n.id)}
                                onSpanChange={(span: number) => patchSpan(n.id, span)}
                                onSelectParent={
                                  editingModal
                                    ? () => {
                                        setEditingModalId(null);
                                        setSelectedId(editingModal.id);
                                      }
                                    : undefined
                                }
                                onMoveUp={() => moveWithin(canvasNodes, n.id, -1)}
                                onMoveDown={() => moveWithin(canvasNodes, n.id, 1)}
                                selectedChildId={selectedId}
                                onChildSelect={(id) => setSelectedId(id)}
                                onChildDelete={(id) => deleteNode(id)}
                                onChildMove={(id, dir) => {
                                  const container = findNode(tree, n.id);
                                  const kids = container?.children ?? [];
                                  const idx = kids.findIndex((k) => k.id === id);
                                  if (idx === -1) return;
                                  setTree((prev) =>
                                    prev.map((x) =>
                                      x.id === n.id
                                        ? { ...x, children: moveNode(kids, id, idx + dir) }
                                        : x,
                                    ),
                                  );
                                }}
                                dragHandleProps={dragHandleProps}
                                canvasWidthRef={canvasRef}
                              />
                            )}
                          </div>
                        );
                      }}
                    </SortableList>
                  </div>
                </Canvas>
              </div>
            )}
          </Col>

          {/* 右：属性面板（rjsf schema 驱动） */}
          {!preview && (
            <Col flex="360px">
              <PropsPanel
                node={selected}
                nodes={tree}
                allFns={allFns}
                fnById={fnById.current}
                onPatch={patchProps}
                onCreateModal={createModalForButton}
                onDelete={() => selected && deleteNode(selected.id)}
              />
            </Col>
          )}
        </Row>
        {!preview && (
          <div style={{ position: 'sticky', bottom: 0, zIndex: 5 }}>
            <DataPanel
              node={selected}
              fn={
                selected?.props.functionId
                  ? fnById.current.get(String(selected.props.functionId))
                  : undefined
              }
            />
          </div>
        )}
        <DragOverlay dropAnimation={null}>
          {dragItem ? (
            <div
              style={{
                background: '#fff',
                border: '1px solid #1677ff',
                borderRadius: 6,
                padding: '4px 10px',
                fontSize: 12,
                opacity: 0.9,
              }}
            >
              {dragItem.kind === 'basic'
                ? `组件：${dragItem.basicType}`
                : `函数：${dragItem.fn.id}`}
            </div>
          ) : null}
        </DragOverlay>
      </DndContext>
    </PageContainer>
  );
}
