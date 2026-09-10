import React, { useCallback, useMemo, useRef, useState } from 'react';
import { App, Button, Card, Col, Input, Row, Space, Tabs, Typography } from 'antd';
import { AppstoreOutlined, ArrowLeftOutlined, EyeOutlined, SaveOutlined } from '@ant-design/icons';
import { FormattedMessage, history, request, useIntl, useSearchParams } from '@umijs/max';
import { subscribeScope } from '@/stores/scope';
import { PageContainer } from '@ant-design/pro-components';
import {
  DndContext,
  DragOverlay,
  PointerSensor,
  closestCenter,
  useSensor,
  useSensors,
} from '@dnd-kit/core';
import { sortableKeyboardCoordinates } from '@dnd-kit/sortable';
import { KeyboardSensor } from '@dnd-kit/core';
import { SortableList } from '@/components/SortableList';
import Canvas, { CanvasNode, ModalPlaceholder } from './Canvas';
import SaveComponentModal from './SaveComponentModal';
import InsertTemplateModal from './InsertTemplateModal';
import OutlinePanel from './OutlinePanel';
import DataPanel from './DataPanel';
import PreviewRuntime from './PreviewRuntime';
import { compileTree, decompileToTree, type SpecSectionLike } from './compiler';
import { scanParamCandidates, type ParamCandidate } from './types';
import { extractErrorMessage } from '@/utils/errors';
import {
  duplicateNode as duplicateTree,
  findInsertedSubtree,
  moveNode,
  replaceSubtree,
} from './model';
import ComponentPanel, { type AddFnEvent } from './ComponentPanel';
import TemplateQuickStart from './TemplateQuickStart';
import ComponentLibrary, { type ComponentTemplateDTO } from './ComponentLibrary';
import PropsPanel from './PropsPanel';
import { registerBuiltinComponents } from './components/builtin';
import { scaffoldProps } from './registry';
import { useCanvasDnd } from './useCanvasDnd';
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
import { assignVarNames, collectVarNames, renameVariable } from './varname';
import { localizedText } from '@/utils/localizedText';
import { useEditorHistory } from './useEditorHistory';

const { Text } = Typography;

registerBuiltinComponents();

/**
 * 组合页编辑器 V3（组件化）：左=组件面板/大纲 Tabs，中=画布（组件树），
 * 右=属性面板（rjsf schema 驱动），顶栏=pageKey/预览切换/保存。
 * 页面状态是 PageNode 组件树；保存时编译为 CompositePageSpec（P4）。
 */
export default function CompositeEditorPage() {
  const { message, modal } = App.useApp();
  const intl = useIntl();
  // useIntl 在测试 mock 下每次渲染返回新引用，进 useCallback/effect 依赖会让回读
  // effect 反复触发；回调内文案统一走 intlRef（渲染期同步写回）。
  const intlRef = useRef(intl);
  intlRef.current = intl;
  const [selectedId, setSelectedId] = useState<string | null>(null);
  /** Shift 多选集合（批量删除）。 */
  const [multiIds, setMultiIds] = useState<Set<string>>(new Set());
  const [saveModalState, setSaveModalState] = useState<null | {
    fnIds: string[];
    selectedNodes: PageNode[];
    /** 参数化候选（U6）：白名单 prop 扫描结果。 */
    paramCandidates: ParamCandidate[];
  }>(null);
  /** 拖入带参数模板的快速配置（U6）。 */
  // 带参模板待插入（模板 + 拖拽落点：参数确认后按落点插入，不丢失 drop 位置）
  const [insertTpl, setInsertTpl] = useState<{ tpl: ComponentTemplateDTO; overId: string } | null>(
    null,
  );
  const [pageKey, setPageKey] = useState('');
  const [keyTouched, setKeyTouched] = useState(false);
  const [mode, setMode] = useState<'edit' | 'preview'>('edit');
  const [leftTab, setLeftTab] = useState('components');
  /** 弹窗内嵌编辑（面包屑）：当前进入的 modal 节点 id。 */
  const [editingModalId, setEditingModalId] = useState<string | null>(null);
  // 树历史（撤销/重做 50 步 + 统一树写入入口 setTree），选择清理由 setter 注入
  const { tree, setTree, treeRef, undo, redo, past, future } = useEditorHistory({
    setSelectedId,
    setMultiIds,
    setEditingModalId,
  });
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
    let cancelled = false;
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
          // 竞态防护：请求期间用户已开始编辑（树上已有节点/已取消）→ 放弃回读覆盖
          if (cancelled || treeRef.current.length > 0) return;
          const [nodes, warnings] = decompileToTree(sections);
          // 函数契约登记（fnById 供画布/属性面板）
          setTree(nodes);
          setPageKey(loadKey);
          setKeyTouched(true);
          if (warnings.length)
            message.warning(
              intlRef.current.formatMessage(
                {
                  id: 'pages.pageStudio.editor.loadback.warnings',
                  defaultMessage: '回读警告：{warnings}',
                },
                { warnings: warnings.join('；') },
              ),
            );
          message.success(
            intlRef.current.formatMessage(
              {
                id: 'pages.pageStudio.editor.loadback.success',
                defaultMessage: '已载入页面 {pageKey}（{sections} 个区块）',
              },
              { pageKey: loadKey, sections: sections.length },
            ),
          );
          return;
        } catch {
          // 尝试下一个数据源
        }
      }
      if (!cancelled)
        message.warning(
          intlRef.current.formatMessage(
            {
              id: 'pages.pageStudio.editor.loadback.notFound',
              defaultMessage: '未找到页面 {pageKey} 的提案/草稿/发布 spec',
            },
            { pageKey: loadKey },
          ),
        );
    })();
    return () => {
      cancelled = true;
    };
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
  /** 子节点放入容器（modal children）；V5：落树即分配语义变量名。 */
  const addChild = useCallback((parentId: string, node: PageNode) => {
    setTree((prev) => {
      const [named] = assignVarNames([node], collectVarNames(prev));
      return insertNode(prev, named, parentId);
    });
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
          message.warning(
            intlRef.current.formatMessage({
              id: 'pages.pageStudio.editor.modal.fnFormOnly',
              defaultMessage: '弹窗内只能放函数表单（V1）',
            }),
          );
          return;
        }
        addChild(editingModalId, node);
        return;
      }
      setTree((prev) => {
        const [named] = assignVarNames([node], collectVarNames(prev));
        return [...prev, named];
      });
      setSelectedId(node.id);
    },
    [registerFn, editingModalId, addChild, message],
  );

  /** 基础组件 → 节点（V5：同样分配变量名——可作动作目标、进补全列表）。 */
  const addBasic = useCallback(
    (type: 'button' | 'modal' | 'container' | 'text') => {
      const node: PageNode = { id: nodeId(type), type, props: scaffoldProps(type) };
      if (editingModalId) {
        message.warning(
          intlRef.current.formatMessage({
            id: 'pages.pageStudio.editor.modal.basicNotSupported',
            defaultMessage: '弹窗内只能放函数表单（V1）——返回页面级再添加',
          }),
        );
        return;
      }
      setTree((prev) => {
        const [named] = assignVarNames([node], collectVarNames(prev));
        return [...prev, named];
      });
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
      message.warning(
        intlRef.current.formatMessage({
          id: 'pages.pageStudio.editor.save.keyRequired',
          defaultMessage: '请填写页面 Key',
        }),
      );
      return;
    }
    const { sections, warnings } = compileTree(tree);
    if (sections.length < 2) {
      message.warning(
        intlRef.current.formatMessage(
          {
            id: 'pages.pageStudio.editor.save.minSections',
            defaultMessage: '组合页至少需要 2 个函数区块（当前有效的 {count} 个）',
          },
          { count: sections.length },
        ),
      );
      return;
    }
    setSaving(true);
    try {
      const resp = (await request('/api/v1/versioning/pages/composite', {
        method: 'POST',
        data: { pageKey: key, sections },
      })) as { proposalKey?: unknown };
      modal.success({
        title: intlRef.current.formatMessage({
          id: 'pages.pageStudio.editor.save.proposalCreated',
          defaultMessage: '提案已创建',
        }),
        content:
          (warnings.length
            ? intlRef.current.formatMessage(
                {
                  id: 'pages.pageStudio.editor.save.compileWarnings',
                  defaultMessage: '编译警告：{warnings}。',
                },
                { warnings: warnings.join('；') },
              )
            : '') +
          intlRef.current.formatMessage(
            {
              id: 'pages.pageStudio.editor.save.proposalCreatedContent',
              defaultMessage: '提案 {proposalKey} 已进入提案收件箱，接受并发布后生效。',
            },
            { proposalKey: String(resp?.proposalKey ?? '') },
          ),
        onOk: () => history.push('/functions/pages'),
      });
    } catch (err) {
      message.error(
        extractErrorMessage(
          err,
          intlRef.current.formatMessage({
            id: 'pages.pageStudio.editor.save.failed',
            defaultMessage: '创建提案失败',
          }),
        ),
      );
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
  const editingModalRef = useRef<string | null>(null);
  editingModalRef.current = editingModalId;
  const {
    dragItem,
    overNodeId,
    setOverNodeId,
    handleDragStart,
    handleDragEnd,
    applyTemplateInsert,
  } = useCanvasDnd({
    treeRef,
    editingModalRef,
    allFns,
    addChild,
    registerFn,
    setTree,
    setSelectedId,
    setInsertTpl,
  });

  /** V5 变量改名：同步重写树内全部表达式/裸引用（§3.2）。
   * 冲突/非法由 PropsPanel 先校验；此处 renameVariable 二次防御。 */
  const renameVarOfSelected = useCallback(
    (newName: string) => {
      if (!selectedId) return;
      setTree((prev) => {
        const node = findNode(prev, selectedId);
        const oldName =
          typeof node?.props.sectionKey === 'string' ? node.props.sectionKey.trim() : '';
        if (!oldName) return prev;
        return renameVariable(prev, oldName, newName);
      });
    },
    [selectedId, setTree],
  );

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 4 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  );

  /** 复制节点：副本不继承变量名（clone 已剥离），落树时重新语义命名，
   * 避免画布出现同名徽标与后续改名冲突。 */
  /** 复制节点：副本不继承变量名（clone 已剥离），仅对副本子树重新语义命名——
   * 不触碰原节点及其引用。 */
  const duplicateNode = useCallback((id: string) => {
    setTree((prev) => {
      const next = duplicateTree(prev, id);
      if (next === prev) return prev;
      const copy = findInsertedSubtree(prev, next);
      if (!copy) return next;
      const [named] = assignVarNames([copy], collectVarNames(prev));
      return replaceSubtree(next, named);
    });
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
        props: { title: localizedText(fn.summary, 'zh-CN', fn.id), width: 'medium' },
        children: [form],
      };
      setTree((prev) => {
        const [named] = assignVarNames([modal], collectVarNames(prev));
        return [...prev, named];
      });
      setTree((prev) =>
        updateProps(prev, selectedId, { onClick: { kind: 'openModal', target: modal.id } }),
      );
      message.success(
        intlRef.current.formatMessage(
          {
            id: 'pages.pageStudio.editor.action.modalCreated',
            defaultMessage: '弹窗已创建并绑定（{fnId}）——可双击弹窗卡片编辑内部',
          },
          { fnId: fn.id },
        ),
      );
    },
    [selectedId, registerFn, message, setTree],
  );

  /** 多选节点保存为组件模板（V4：用户自定义组件）。
   * 支持命名/描述/分类；任意层级节点（含嵌套在弹窗/容器内的子树）均可保存。 */
  const saveSelectionAsComponent = useCallback(async () => {
    if (multiIds.size < 1) {
      message.warning(
        intlRef.current.formatMessage({
          id: 'pages.pageStudio.editor.component.selectionRequired',
          defaultMessage: '请先选中至少一个组件',
        }),
      );
      return;
    }
    // 任意层级：优先按子树查找（嵌套节点），找不到再回落根级
    const selectedNodes = Array.from(multiIds)
      .map((id) => findNode(tree, id))
      .filter((n): n is PageNode => n !== null);
    if (selectedNodes.length === 0) {
      message.warning(
        intlRef.current.formatMessage({
          id: 'pages.pageStudio.editor.component.selectionMissing',
          defaultMessage: '选中的组件不存在',
        }),
      );
      return;
    }
    const fnIds: string[] = [];
    const collectFns = (nodes: PageNode[]) => {
      for (const n of nodes) {
        const fid = String(n.props.functionId ?? '');
        if (fid && !fnIds.includes(fid)) fnIds.push(fid);
        if (n.children) collectFns(n.children);
      }
    };
    collectFns(selectedNodes);

    setSaveModalState({
      fnIds,
      selectedNodes,
      paramCandidates: scanParamCandidates(selectedNodes),
    });
  }, [multiIds, tree, message]);

  const deleteNode = useCallback((id: string) => {
    setTree((prev) => removeNode(prev, id)[0]);
    setSelectedId((cur) => (cur === id ? null : cur));
    // 多选集合同步摘除（防悬空 id 残留）
    setMultiIds((prev) => {
      if (!prev.has(id)) return prev;
      const next = new Set(prev);
      next.delete(id);
      return next;
    });
    // 正在弹窗内部编辑时删掉该弹窗 → 退出弹窗编辑态
    setEditingModalId((cur) => (cur === id ? null : cur));
  }, []);

  const preview = mode === 'preview';

  return (
    <PageContainer
      header={{
        title: intl.formatMessage({
          id: 'pages.pageStudio.editor.title',
          defaultMessage: '组合页编辑器',
        }),
        onBack: () => history.push('/functions/pages'),
        backIcon: <ArrowLeftOutlined />,
        extra: [
          <Button
            key="undo"
            disabled={preview || past.length === 0}
            onClick={undo}
            title={intl.formatMessage({
              id: 'pages.pageStudio.editor.undo.tooltip',
              defaultMessage: '撤销 (Ctrl+Z)',
            })}
          >
            ↩
          </Button>,
          <Button
            key="redo"
            disabled={preview || future.length === 0}
            onClick={redo}
            title={intl.formatMessage({
              id: 'pages.pageStudio.editor.redo.tooltip',
              defaultMessage: '重做 (Ctrl+Shift+Z)',
            })}
          >
            ↪
          </Button>,
          <Button
            key="mode"
            type={preview ? 'primary' : 'default'}
            icon={<EyeOutlined />}
            onClick={() => setMode(preview ? 'edit' : 'preview')}
          >
            {preview
              ? intl.formatMessage({
                  id: 'pages.pageStudio.editor.preview.exit',
                  defaultMessage: '退出预览',
                })
              : intl.formatMessage({
                  id: 'pages.pageStudio.editor.preview.enter',
                  defaultMessage: '预览',
                })}
          </Button>,
          multiIds.size > 0 && (
            <Button
              key="save-component"
              icon={<AppstoreOutlined />}
              onClick={() => void saveSelectionAsComponent()}
            >
              <FormattedMessage
                id="pages.pageStudio.editor.saveComponent.button"
                defaultMessage="保存为组件（{count}）"
                values={{ count: multiIds.size }}
              />
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
                // 批量删除包含正在编辑的弹窗 → 退出弹窗编辑态（防悬空 editingModalId）
                setEditingModalId((cur) => (cur && multiIds.has(cur) ? null : cur));
              }}
            >
              <FormattedMessage
                id="pages.pageStudio.editor.deleteSelected.button"
                defaultMessage="删除所选（{count}）"
                values={{ count: multiIds.size }}
              />
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
            <FormattedMessage
              id="pages.pageStudio.editor.save.button"
              defaultMessage="保存为提案"
            />
          </Button>,
        ],
      }}
    >
      <Space wrap style={{ marginBottom: 12 }}>
        <Text type="secondary" style={{ fontSize: 10 }}>
          v3.2.1
        </Text>
        <Text strong>
          <FormattedMessage id="pages.pageStudio.editor.pageKey.label" defaultMessage="页面 Key" />
        </Text>
        <Input
          placeholder={intl.formatMessage({
            id: 'pages.pageStudio.editor.pageKey.placeholder',
            defaultMessage: '按组件自动生成，可修改',
          })}
          value={pageKey}
          onChange={(e) => {
            setKeyTouched(true);
            setPageKey(e.target.value);
          }}
          style={{ width: 320 }}
        />
        <Text type="secondary">
          {intl.formatMessage(
            { id: 'pages.pageStudio.editor.nodeCount', defaultMessage: '{count} 个组件' },
            { count: countNodes(tree) },
          )}
        </Text>
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
        <Row gutter={12} wrap={false}>
          {/* 左：组件面板 / 大纲 */}
          {!preview && (
            <Col flex="300px" style={{ minWidth: 260 }}>
              <Card size="small" styles={{ body: { padding: 8 } }}>
                <Tabs
                  activeKey={leftTab}
                  onChange={setLeftTab}
                  items={[
                    {
                      key: 'library',
                      label: intl.formatMessage({
                        id: 'pages.pageStudio.editor.tab.library',
                        defaultMessage: '组件库',
                      }),
                      children: (
                        <ComponentLibrary
                          availableFnIds={new Set(allFns.map((f) => f.id))}
                          onInsert={(nodes, tpl) => {
                            // 带参数模板（U6）：空 nodes = 待参数配置，弹窗确认后插入
                            // （点击插入无拖拽落点 → 按根级追加处理）
                            if (tpl.params?.length && nodes.length === 0) {
                              setInsertTpl({ tpl, overId: 'canvas-root' });
                              return;
                            }
                            // 语义命名（instantiateTemplate 剥离 sectionKey，须重新分配变量名）
                            setTree((prev) => [
                              ...prev,
                              ...assignVarNames(nodes, collectVarNames(prev)),
                            ]);
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
                      label: intl.formatMessage({
                        id: 'pages.pageStudio.editor.tab.functions',
                        defaultMessage: '函数',
                      }),
                      children: (
                        <ComponentPanel onAddBasic={addBasic} onAddFunction={addFunction} />
                      ),
                    },
                    {
                      key: 'outline',
                      label: intl.formatMessage({
                        id: 'pages.pageStudio.editor.tab.outline',
                        defaultMessage: '大纲',
                      }),
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
                        <FormattedMessage
                          id="pages.pageStudio.editor.breadcrumb.page"
                          defaultMessage="页面"
                        />
                      </a>
                      <span style={{ fontSize: 12 }}>/</span>
                      <Text strong style={{ fontSize: 12 }}>
                        {String(
                          editingModal.props.title ??
                            intl.formatMessage({
                              id: 'pages.pageStudio.editor.breadcrumb.modalFallback',
                              defaultMessage: '弹窗',
                            }),
                        )}
                        {intl.formatMessage({
                          id: 'pages.pageStudio.editor.breadcrumb.editingInside',
                          defaultMessage: '（内部编辑）',
                        })}
                      </Text>
                      <Text type="secondary" style={{ fontSize: 11 }}>
                        <FormattedMessage
                          id="pages.pageStudio.editor.modal.editHint"
                          defaultMessage="从左栏拖入/点击添加表单；拖拽排序"
                        />
                      </Text>
                    </Space>
                  </div>
                )}
                {canvasNodes.length === 0 && !editingModalRef.current ? (
                  <TemplateQuickStart
                    onPick={(nodes, tpl) => {
                      // 语义命名（instantiateTemplate 剥离 sectionKey，须重新分配变量名）
                      setTree((prev) => [...prev, ...assignVarNames(nodes, collectVarNames(prev))]);
                      for (const fid of tpl.requiredFunctions ?? []) {
                        const fnDesc = allFns.find((f) => f.id === fid);
                        if (fnDesc) registerFn(fnDesc);
                      }
                      setSelectedId(nodes[0]?.id ?? null);
                      message.success(
                        intlRef.current.formatMessage(
                          {
                            id: 'pages.pageStudio.editor.template.applied',
                            defaultMessage: '已从模板「{name}」创建页面骨架',
                          },
                          {
                            name: tpl.name
                              ? ((tpl.name as Record<string, string>)['zh-CN'] ?? tpl.key)
                              : '',
                          },
                        ),
                      );
                    }}
                  />
                ) : null}
                {canvasNodes.length > 0 ? (
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
                          const spanNum = Math.min(
                            24,
                            Math.max(4, Number(n.props.span ?? 24) || 24),
                          );
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
                                    onSelect={(e) => {
                                      setSelectedId(n.id);
                                      // Shift+点击=多选切换；普通点击=单选（清空多选）
                                      if (e?.shiftKey) {
                                        setMultiIds((prev) => {
                                          const next = new Set(prev);
                                          if (next.has(n.id)) next.delete(n.id);
                                          else next.add(n.id);
                                          return next;
                                        });
                                      } else {
                                        setMultiIds((prev) => (prev.size === 0 ? prev : new Set()));
                                      }
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
                                  onSelect={(e) => {
                                    setSelectedId(n.id);
                                    // Shift+点击=多选切换；普通点击=单选（清空多选）
                                    if (e?.shiftKey) {
                                      setMultiIds((prev) => {
                                        const next = new Set(prev);
                                        if (next.has(n.id)) next.delete(n.id);
                                        else next.add(n.id);
                                        return next;
                                      });
                                    } else {
                                      setMultiIds((prev) => (prev.size === 0 ? prev : new Set()));
                                    }
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
                ) : null}
              </div>
            )}
          </Col>

          {/* 右：属性面板（rjsf schema 驱动） */}
          {!preview && (
            <Col flex="360px" style={{ minWidth: 320 }}>
              <PropsPanel
                node={selected}
                nodes={tree}
                allFns={allFns}
                fnById={fnById.current}
                onPatch={patchProps}
                onRenameVariable={renameVarOfSelected}
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
                ? intl.formatMessage(
                    { id: 'pages.pageStudio.editor.drag.basic', defaultMessage: '组件：{type}' },
                    { type: dragItem.basicType },
                  )
                : dragItem.kind === 'template'
                  ? intl.formatMessage(
                      {
                        id: 'pages.pageStudio.editor.drag.template',
                        defaultMessage: '模板：{name}',
                      },
                      { name: localizedText(dragItem.tpl.name, 'zh-CN', dragItem.tpl.key) },
                    )
                  : intl.formatMessage(
                      {
                        id: 'pages.pageStudio.editor.drag.function',
                        defaultMessage: '函数：{fnId}',
                      },
                      { fnId: dragItem.fn.id },
                    )}
            </div>
          ) : null}
        </DragOverlay>
      </DndContext>
      <SaveComponentModal state={saveModalState} onClose={() => setSaveModalState(null)} />
      <InsertTemplateModal
        tplState={insertTpl}
        onClose={() => setInsertTpl(null)}
        onConfirm={applyTemplateInsert}
      />
    </PageContainer>
  );
}
