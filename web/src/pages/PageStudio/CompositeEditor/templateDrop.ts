import { acceptsChild } from './registry';
import type { PageNode } from './model';

/**
 * 模板拖入落点决策（纯函数）。
 *
 * 落点优先级与函数组件一致：弹窗占位卡 / 编辑中弹窗（V1 仅 fnForm）→
 * 容器（装入 children，子类型须满足 allowedChildren 契约）→
 * 节点之后（链式保持模板顺序）→ 根级末尾。
 */
export type TemplateDropPlan =
  | { kind: 'modal'; targetId: string }
  | { kind: 'container'; targetId: string }
  | { kind: 'after'; afterId?: string }
  | { kind: 'blocked'; reason: string };

export function planTemplateDrop(
  nodes: PageNode[],
  overId: string,
  editingModalId: string | null,
  afterNode?: PageNode,
): TemplateDropPlan {
  if (nodes.length === 0) {
    return { kind: 'blocked', reason: '模板为空' };
  }

  const modalTarget = overId.startsWith('modal-drop:')
    ? overId.slice('modal-drop:'.length)
    : editingModalId;
  if (modalTarget) {
    if (!nodes.every((n) => n.type === 'fnForm')) {
      return { kind: 'blocked', reason: '弹窗内只能放函数表单（V1）' };
    }
    return { kind: 'modal', targetId: modalTarget };
  }

  if (afterNode?.type === 'container') {
    // 契约校验：容器 allowedChildren（同拖拽单节点路径），不满足即拦截
    const bad = nodes.find((n) => !acceptsChild(afterNode, n.type));
    if (bad) {
      return {
        kind: 'blocked',
        reason: `容器不接受「${bad.type}」子组件（容器仅允许表格/字段卡/按钮/文本）`,
      };
    }
    return { kind: 'container', targetId: afterNode.id };
  }
  return { kind: 'after', afterId: overId === 'canvas-root' ? undefined : overId };
}
