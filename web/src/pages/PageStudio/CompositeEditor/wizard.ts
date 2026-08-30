import type { FunctionDescriptor } from '@/services/api/functions';
import { nodeId, type PageNode } from './model';
import { schemaProperties } from './types';

/** 向导生成产物：表格 + 弹窗（表单），同名字段自动映射行操作参数。 */
export interface WizardTree {
  tree: PageNode[];
  tableId: string;
  modalId: string;
}

/**
 * 快速开始向导 → 页面骨架：
 * - fnTable：契约 scaffold，autoRun，行操作指向弹窗（同名字段自动映射
 *   player_id ← 行.player_id）
 * - modal + fnForm：表单 onSuccessRefresh 指回表格（提交成功自动刷新）
 */
export function buildWizardTree(
  tableFn: FunctionDescriptor,
  actionFn: FunctionDescriptor,
  scaffold: (type: PageNode['type'], fn?: FunctionDescriptor) => Record<string, unknown>,
): WizardTree {
  const tableId = nodeId('fnTable');
  const modalId = nodeId('modal');

  const table: PageNode = {
    id: tableId,
    type: 'fnTable',
    props: {
      ...scaffold('fnTable', tableFn),
      rowActions: [
        {
          label: actionFn.summary?.['zh-CN'] || actionFn.id,
          targetSection: modalId,
          params: autoParams(tableFn, actionFn),
          danger: false,
        },
      ],
    },
  };

  const form: PageNode = {
    id: nodeId('fnForm'),
    type: 'fnForm',
    props: {
      ...scaffold('fnForm', actionFn),
      display: 'inline',
      onSuccessRefresh: { kind: 'refreshNode', target: tableId },
    },
  };

  const modal: PageNode = {
    id: modalId,
    type: 'modal',
    props: { title: actionFn.summary?.['zh-CN'] || actionFn.id, width: 'medium' },
    children: [form],
  };

  return { tree: [table, modal], tableId, modalId };
}

/** 同名字段映射：操作函数输入 ∩ 表格输出顶层字段。 */
export function autoParams(
  tableFn: FunctionDescriptor,
  actionFn: FunctionDescriptor,
): Record<string, string> {
  const rowFields = schemaProperties(tableFn.outputSchema);
  return Object.fromEntries(
    schemaProperties(actionFn.inputSchema)
      .filter((p) => rowFields.includes(p))
      .map((p) => [p, p]),
  );
}
