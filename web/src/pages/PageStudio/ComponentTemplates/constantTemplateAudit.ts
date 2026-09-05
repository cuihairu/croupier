import type { PageNode } from '../CompositeEditor/model';
import { staticFormNodeFromFields, type ConstantField } from '../CompositeEditor/constants';

/** 组件模板审计/示例数据工具（组件模板页专用）。 */

/**
 * 统计单个 staticForm 节点 staticSchema 声明的常量字段数。
 * 非 staticForm 节点或 schema 解析失败返回 0。
 */
export function countStaticFormFields(node: PageNode): number {
  if (node?.type !== 'staticForm') return 0;
  const schema = node.props?.staticSchema;
  if (typeof schema !== 'string' || !schema.trim()) return 0;
  try {
    const obj = JSON.parse(schema) as { properties?: Record<string, unknown> };
    return Object.keys(obj.properties ?? {}).length;
  } catch {
    return 0;
  }
}

/** 树中任一 staticForm 节点的最大常量字段数（无 staticForm 返回 0）。 */
export function maxConstantsInTree(tree: unknown[]): number {
  if (!Array.isArray(tree)) return 0;
  let max = 0;
  const walk = (nodes: unknown[]): void => {
    for (const item of nodes) {
      const node = item as PageNode;
      if (!node || typeof node !== 'object') continue;
      max = Math.max(max, countStaticFormFields(node));
      const children = (node.props as { children?: unknown[] } | undefined)?.children;
      if (Array.isArray(children)) walk(children);
    }
  };
  walk(tree);
  return max;
}

export interface LegacyMergedTemplateLike {
  key: string;
  tree: PageNode[];
}

/**
 * 旧版合并常量模板：一个 staticForm 节点塞了多个常量字段
 * （新规范是「一种常量一个独立模板」，每个模板恰好 1 个字段）。
 */
export function findLegacyMergedTemplates<T extends LegacyMergedTemplateLike>(templates: T[]): T[] {
  return templates.filter((t) => maxConstantsInTree(t.tree) > 1);
}

/** 示例常量模板创建 payload（与 ConstantImportModal 的 POST 形状一致）。 */
export interface ConstantTemplatePayload {
  key: string;
  name: { 'zh-CN': string; 'en-US': string };
  description: { 'zh-CN': string; 'en-US': string };
  category: string;
  icon: string;
  requiredFunctions: string[];
  tree: PageNode[];
}

const DEMO_CONSTANTS: Array<{ suffix: string; title: string; titleEn: string; options: string[] }> =
  [
    {
      suffix: 'ban-reason',
      title: '封禁原因',
      titleEn: 'Ban Reason',
      options: ['恶意刷单', '使用外挂', '辱骂他人', '账号风险'],
    },
    {
      suffix: 'vip-level',
      title: '会员等级',
      titleEn: 'VIP Level',
      options: ['VIP1', 'VIP2', 'VIP3', 'VIP4', 'VIP5', 'VIP6'],
    },
    {
      suffix: 'server-status',
      title: '服务器状态',
      titleEn: 'Server Status',
      options: ['正常', '繁忙', '维护中'],
    },
    {
      suffix: 'pay-channel',
      title: '支付渠道',
      titleEn: 'Pay Channel',
      options: ['微信支付', '支付宝', '苹果支付', '谷歌支付'],
    },
  ];

/**
 * 示例常量模板（演示假数据）：一种常量一个独立单下拉 staticForm 模板。
 * key 固定为 consts--demo-*（重复生成前可按 key 去重，保证幂等）。
 */
export function demoConstantTemplatePayloads(): ConstantTemplatePayload[] {
  return DEMO_CONSTANTS.map(({ suffix, title, titleEn, options }) => {
    const field: ConstantField = {
      key: suffix.replace(/-([a-z])/g, (_, c: string) => c.toUpperCase()),
      title,
      options: options.map((o) => ({ value: o, label: o })),
    };
    return {
      key: `consts--demo-${suffix}`,
      name: { 'zh-CN': title, 'en-US': titleEn },
      description: {
        'zh-CN': `示例常量下拉（${options.length} 个选项）`,
        'en-US': `Demo constant dropdown (${options.length} options)`,
      },
      category: '常量',
      icon: 'ControlOutlined',
      requiredFunctions: [],
      tree: [staticFormNodeFromFields([field], title, 12)],
    };
  });
}
