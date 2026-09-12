import { getIntl } from '@umijs/max';
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
    return {
      kind: 'blocked',
      reason: getIntl().formatMessage({
        id: 'pages.pageStudio.editor.canvas.templateEmpty',
        defaultMessage: '模板为空',
      }),
    };
  }

  const modalTarget = overId.startsWith('modal-drop:')
    ? overId.slice('modal-drop:'.length)
    : editingModalId;
  if (modalTarget) {
    if (!nodes.every((n) => n.type === 'fnForm')) {
      return {
        kind: 'blocked',
        reason: getIntl().formatMessage({
          id: 'pages.pageStudio.editor.canvas.modalFormOnly',
          defaultMessage: '弹窗内只能放函数表单（V1）',
        }),
      };
    }
    return { kind: 'modal', targetId: modalTarget };
  }

  if (afterNode?.type === 'container') {
    // 契约校验：容器 allowedChildren（同拖拽单节点路径），不满足即拦截
    const bad = nodes.find((n) => !acceptsChild(afterNode, n.type));
    if (bad) {
      return {
        kind: 'blocked',
        reason: getIntl().formatMessage(
          {
            id: 'pages.pageStudio.editor.canvas.containerNotAllowed',
            defaultMessage: '容器不接受「{type}」子组件（容器仅允许表格/字段卡/按钮/文本）',
          },
          { type: bad.type },
        ),
      };
    }
    return { kind: 'container', targetId: afterNode.id };
  }
  if (afterNode?.type === 'tabs') {
    // 页签容器：模板整体装入当前激活页（页即 container，同契约校验）；
    // 无页 → 拦截（页签容器构造时自带 2 空页签，此分支兜底异常形态）。
    const pages = (afterNode.children ?? []).filter((p) => p.type === 'container');
    const activeProp =
      typeof afterNode.props.activeTab === 'string' ? afterNode.props.activeTab : '';
    const page = pages.find((p) => p.id === activeProp) ?? pages[0];
    if (!page) {
      return {
        kind: 'blocked',
        reason: getIntl().formatMessage({
          id: 'pages.pageStudio.editor.canvas.tabsNoPage',
          defaultMessage: '页签容器没有可用的页，无法放入模板',
        }),
      };
    }
    const bad = nodes.find((n) => !acceptsChild(page, n.type));
    if (bad) {
      return {
        kind: 'blocked',
        reason: getIntl().formatMessage(
          {
            id: 'pages.pageStudio.editor.canvas.tabsNotAllowed',
            defaultMessage: '页签内不接受「{type}」子组件（页签页仅允许表格/字段卡/按钮/文本）',
          },
          { type: bad.type },
        ),
      };
    }
    return { kind: 'container', targetId: page.id };
  }
  return { kind: 'after', afterId: overId === 'canvas-root' ? undefined : overId };
}
