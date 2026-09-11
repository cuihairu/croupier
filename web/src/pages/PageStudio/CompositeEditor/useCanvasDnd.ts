import { useCallback, useState, type Dispatch, type RefObject, type SetStateAction } from 'react';
import { App } from 'antd';
import { useIntl } from '@umijs/max';
import type { DragEndEvent, DragStartEvent } from '@dnd-kit/core';
import type { FunctionDescriptor } from '@/services/api/functions';
import { acceptsChild, scaffoldProps } from './registry';
import { findNode, insertAfter, moveNode, nodeId, type PageNode } from './model';
import { assignVarNames, collectVarNames } from './varname';
import { instantiateTemplate, type ComponentTemplateDTO } from './ComponentLibrary';
import { planTemplateDrop } from './templateDrop';
import type { AddFnEvent } from './ComponentPanel';

/** 拖拽预览项（DragOverlay 渲染 + 落点指示）。 */
export type CanvasDragItem =
  | null
  | { kind: 'basic'; basicType: string }
  | { kind: 'fn'; fn: AddFnEvent['fn']; componentType: AddFnEvent['componentType'] }
  | { kind: 'template'; tpl: ComponentTemplateDTO; missing: string[] };

/** 画布拖拽 hook（面板→画布插入 / 画布内重排 / modal 收纳，T2.2/T2.3）：
 * - handleDragStart 记录面板拖拽项（DragOverlay/落点指示消费）
 * - handleDragEnd 分派：模板实例化按落点插入（planTemplateDrop 决策）、
 *   面板 basic/fn 节点构造与 modal-drop: 前缀落点、画布内 SortableList 重排
 * - applyTemplateInsert 是拖拽直接插入与带参模板弹窗确认后共用的插入入口 */
export function useCanvasDnd({
  treeRef,
  editingModalRef,
  allFns,
  addChild,
  registerFn,
  setTree,
  setSelectedId,
  setInsertTpl,
}: {
  treeRef: RefObject<PageNode[]>;
  editingModalRef: RefObject<string | null>;
  allFns: FunctionDescriptor[];
  addChild: (parentId: string, node: PageNode) => void;
  registerFn: (fn: FunctionDescriptor) => void;
  setTree: (action: SetStateAction<PageNode[]>) => void;
  setSelectedId: Dispatch<SetStateAction<string | null>>;
  setInsertTpl: Dispatch<SetStateAction<{ tpl: ComponentTemplateDTO; overId: string } | null>>;
}) {
  const { message } = App.useApp();
  const intl = useIntl();
  const [dragItem, setDragItem] = useState<CanvasDragItem>(null);
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
      | { source: 'panel'; kind: 'template'; tpl: ComponentTemplateDTO; missing: string[] }
      | { source: 'canvas' }
      | undefined;
    if (data?.source === 'panel') {
      setDragItem(
        data.kind === 'basic'
          ? { kind: 'basic', basicType: data.basicType }
          : data.kind === 'template'
            ? { kind: 'template', tpl: data.tpl, missing: data.missing }
            : { kind: 'fn', fn: data.fn, componentType: data.componentType },
      );
    } else {
      setDragItem(null);
    }
  }, []);

  /** 模板实例化并按落点插入（拖拽直接插入与带参弹窗确认后共用）：
   * instantiateTemplate（id/引用重映射）→ assignVarNames 语义命名 →
   * 函数契约登记 → planTemplateDrop 决定弹窗/容器/链式插入。 */
  const applyTemplateInsert = useCallback(
    (tpl: ComponentTemplateDTO, values: Record<string, unknown>, overId: string) => {
      const nodes = assignVarNames(
        instantiateTemplate(tpl, values),
        collectVarNames(treeRef.current),
      );
      if (nodes.length === 0) return;
      for (const fid of tpl.requiredFunctions ?? []) {
        const fn = allFns.find((f) => f.id === fid);
        if (fn) registerFn(fn);
      }
      const after = overId === 'canvas-root' ? undefined : findNode(treeRef.current, overId);
      const plan = planTemplateDrop(nodes, overId, editingModalRef.current, after);
      if (plan.kind === 'blocked') {
        message.warning(plan.reason);
        return;
      }
      if (plan.kind === 'modal' || plan.kind === 'container') {
        for (const node of nodes) addChild(plan.targetId, node);
      } else {
        // 链式插入：每个节点插到前一个之后，保持模板顺序
        let anchorId = plan.afterId;
        for (const node of nodes) {
          setTree((prev) => (anchorId ? insertAfter(prev, node, anchorId!) : [...prev, node]));
          anchorId = node.id;
        }
      }
      setSelectedId(nodes[0].id);
    },
    [addChild, message, registerFn, allFns, setTree],
  );

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
        | { source: 'panel'; kind: 'template'; tpl: ComponentTemplateDTO; missing: string[] }
        | { source: 'canvas' }
        | undefined;
      const overId = String(over.id);

      if (data?.source === 'panel' && data.kind === 'template') {
        // 模板拖入：实例化子树（id/引用重映射），按落点插入多节点
        if (data.missing.length > 0) {
          message.warning(
            intl.formatMessage(
              {
                id: 'pages.pageStudio.editor.canvas.missingDeps',
                defaultMessage: '缺少依赖函数：{fns}',
              },
              { fns: data.missing.join(', ') },
            ),
          );
          return;
        }
        // 带参数模板（U6）：先弹参数表单再实例化——保留拖拽落点，确认后按落点插入
        if (data.tpl.params?.length) {
          setInsertTpl({ tpl: data.tpl, overId });
          return;
        }
        applyTemplateInsert(data.tpl, {}, overId);
        return;
      }

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
          else
            message.warning(
              intl.formatMessage({
                id: 'pages.pageStudio.editor.canvas.modalFormOnly',
                defaultMessage: '弹窗内只能放函数表单（V1）',
              }),
            );
          return;
        }
        // 弹窗级编辑中：面板加入的节点落到当前弹窗 children（仅表单）
        if (editingModalRef.current) {
          if (node.type !== 'fnForm') {
            message.warning(
              intl.formatMessage({
                id: 'pages.pageStudio.editor.canvas.modalFormOnly',
                defaultMessage: '弹窗内只能放函数表单（V1）',
              }),
            );
            return;
          }
          addChild(editingModalRef.current, node);
          return;
        }
        // 落点=容器节点 → 契约校验后装入 children；其余=节点之后；根=末尾
        const after = overId === 'canvas-root' ? undefined : findNode(treeRef.current, overId);
        if (after?.type === 'container') {
          if (acceptsChild(after, node.type)) {
            addChild(after.id, node);
          } else {
            // 容器不接受该子类型（allowedChildren 契约）→ 回退为容器之后的兄弟插入
            message.warning(
              intl.formatMessage(
                {
                  id: 'pages.pageStudio.editor.canvas.containerFallback',
                  defaultMessage: '容器不接受「{type}」子组件，已放到容器之后',
                },
                { type: node.type },
              ),
            );
            setTree((prev) => {
              const [named] = assignVarNames([node], collectVarNames(prev));
              return insertAfter(prev, named, after.id);
            });
          }
        } else {
          setTree((prev) => {
            const [named] = assignVarNames([node], collectVarNames(prev));
            return after ? insertAfter(prev, named, after.id) : [...prev, named];
          });
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
    [addChild, intl, message, registerFn, allFns, applyTemplateInsert],
  );

  return {
    dragItem,
    overNodeId,
    setOverNodeId,
    handleDragStart,
    handleDragEnd,
    applyTemplateInsert,
  };
}
