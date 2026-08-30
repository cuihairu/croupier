import React, { useCallback, useMemo, useRef, useState } from 'react';
import { App, Button, Card, Col, Empty, Input, Row, Space, Tabs, Tag, Typography } from 'antd';
import { ArrowLeftOutlined, EyeOutlined, SaveOutlined } from '@ant-design/icons';
import { history, request } from '@umijs/max';
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
import Canvas, { CanvasNode } from './Canvas';
import OutlinePanel from './OutlinePanel';
import PreviewRuntime from './PreviewRuntime';
import QuickStart from './QuickStart';
import { buildWizardTree } from './wizard';
import { compileTree } from './compiler';
import { schemaProperties } from './types';
import { extractErrorMessage } from '@/utils/errors';
import { duplicateNode as duplicateTree, insertAfter, moveNode } from './model';
import ComponentPanel, { type AddFnEvent } from './ComponentPanel';
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
  const [tree, setTree] = useState<PageNode[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [pageKey, setPageKey] = useState('');
  const [keyTouched, setKeyTouched] = useState(false);
  const [mode, setMode] = useState<'edit' | 'preview'>('edit');
  const [leftTab, setLeftTab] = useState('components');
  const [skipWizard, setSkipWizard] = useState(false);

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
  const [, forceFn] = useState(0);
  const registerFn = useCallback((fn: FunctionDescriptor) => {
    fnById.current.set(fn.id, fn);
    forceFn((n) => n + 1);
  }, []);

  const selected = useMemo(() => findNode(tree, selectedId ?? ''), [tree, selectedId]);

  /** 函数 → 组件节点（scaffold 按契约实例化，amis 式拖入即骨架）。 */
  const addFunction = useCallback(
    (e: AddFnEvent) => {
      registerFn(e.fn);
      const node: PageNode = {
        id: nodeId(e.componentType),
        type: e.componentType,
        props: scaffoldProps(e.componentType, e.fn),
      };
      setTree((prev) => [...prev, node]);
      setSelectedId(node.id);
    },
    [registerFn],
  );

  /** 基础组件 → 节点。 */
  const addBasic = useCallback((type: 'button' | 'modal' | 'container' | 'text') => {
    const node: PageNode = { id: nodeId(type), type, props: scaffoldProps(type) };
    setTree((prev) => [...prev, node]);
    setSelectedId(node.id);
  }, []);

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

  /** 向导生成：表格（行操作→弹窗）+ 弹窗（表单，成功刷新表格）。 */
  const generateFromWizard = useCallback(
    (tableFn: FunctionDescriptor, actionFn: FunctionDescriptor) => {
      registerFn(tableFn);
      registerFn(actionFn);
      const { tree: wizardTree, tableId } = buildWizardTree(tableFn, actionFn, scaffoldProps);
      setTree(wizardTree);
      setSelectedId(tableId);
    },
    [registerFn],
  );

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

  /** 子节点放入容器（V1：modal 单 fnForm）。 */
  const addChild = useCallback((parentId: string, node: PageNode) => {
    setTree((prev) => insertNode(prev, node, parentId));
    setSelectedId(node.id);
  }, []);

  // ---- 拖拽（T2.2/T2.3）：面板→画布插入 / 画布内重排 / modal 收纳 ----
  const [dragItem, setDragItem] = useState<
    | null
    | { kind: 'basic'; basicType: string }
    | { kind: 'fn'; fn: AddFnEvent['fn']; componentType: AddFnEvent['componentType'] }
  >(null);
  const treeRef = useRef(tree);
  treeRef.current = tree;

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 4 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  );

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
        // modal 收纳区：fnForm 装入 modal
        if (overId.startsWith('modal-drop:')) {
          const modalId = overId.slice('modal-drop:'.length);
          if (node.type === 'fnForm') addChild(modalId, node);
          else message.warning('弹窗内只能放函数表单（V1）');
          return;
        }
        // 落点=某节点之后（over.id 即节点 key，无前缀）；根=末尾
        const after = overId === 'canvas-root' ? undefined : findNode(treeRef.current, overId);
        setTree((prev) => (after ? insertAfter(prev, node, after.id) : [...prev, node]));
        setSelectedId(node.id);
        return;
      }

      // 画布内重排（active id = sortable 节点 id）
      if (overId === 'canvas-root') return;
      const activeId = String(active.id);
      const overIdx = treeRef.current
        .filter((n) => n.type !== 'modal')
        .findIndex((n) => n.id === overId);
      if (overIdx === -1) return;
      setTree((prev) => {
        const inline = prev.filter((n) => n.type !== 'modal');
        const fromIdx = inline.findIndex((n) => n.id === activeId);
        if (fromIdx === -1) return prev;
        const target = overIdx;
        const moved = moveNode(inline, activeId, target);
        if (moved === inline) return prev;
        return [...moved, ...prev.filter((n) => n.type === 'modal')];
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
            key="mode"
            type={preview ? 'primary' : 'default'}
            icon={<EyeOutlined />}
            onClick={() => setMode(preview ? 'edit' : 'preview')}
          >
            {preview ? '退出预览' : '预览'}
          </Button>,
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
        onDragEnd={handleDragEnd}
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
                      key: 'components',
                      label: '组件',
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
            ) : tree.length === 0 && !skipWizard ? (
              <QuickStart onGenerate={generateFromWizard} onSkip={() => setSkipWizard(true)} />
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
                <Canvas
                  tree={tree}
                  selectedId={selectedId}
                  fnById={fnById.current}
                  onSelect={setSelectedId}
                  onDelete={deleteNode}
                  onDuplicate={duplicateNode}
                  onSpanChange={patchSpan}
                  canvasWidthRef={canvasRef}
                >
                  <SortableList
                    items={tree.filter((n) => n.type !== 'modal')}
                    getKey={(n) => n.id}
                    onReorder={(next) =>
                      setTree((prev) => [...next, ...prev.filter((n) => n.type === 'modal')])
                    }
                    externalDnd
                  >
                    {(n, _idx, dragHandleProps) => (
                      <Col key={n.id} span={Number(n.props.span ?? 24) || 24}>
                        <CanvasNode
                          node={n}
                          fn={
                            n.props.functionId
                              ? fnById.current.get(String(n.props.functionId))
                              : undefined
                          }
                          selected={selectedId === n.id}
                          depth={0}
                          onSelect={() => setSelectedId(n.id)}
                          onDelete={() => deleteNode(n.id)}
                          onDuplicate={() => duplicateNode(n.id)}
                          onSpanChange={(span: number) => patchSpan(n.id, span)}
                          dragHandleProps={dragHandleProps}
                          canvasWidthRef={canvasRef}
                        />
                      </Col>
                    )}
                  </SortableList>
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
                onDelete={() => selected && deleteNode(selected.id)}
              />
            </Col>
          )}
        </Row>
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
