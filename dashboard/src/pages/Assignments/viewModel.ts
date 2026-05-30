import type { FunctionDescriptor } from '@/services/api';
import type { AssignmentItem } from './types';

export type AssignmentOption = {
  label: string;
  value: string;
  version?: string;
  category: string;
  displayName: string;
  menuNodes: string[];
  menuPath: string;
  menuSource: string;
};

export const buildAssignmentOptions = (descs: FunctionDescriptor[]): AssignmentOption[] =>
  (Array.isArray(descs) ? descs : []).map((d) => ({
    label: `${d.id} v${d.version || ''}`,
    value: d.id,
    version: d.version,
    category: d.category || 'general',
    displayName:
      (typeof d.display_name === 'object'
        ? d.display_name?.zh || d.display_name?.en
        : d.display_name) || d.id,
    menuNodes: Array.isArray(d.menu?.nodes) ? d.menu.nodes : [],
    menuPath: d.menu?.path || '',
    menuSource: (d as any).menuSource || 'default',
  }));

export const buildGroupedAssignments = (options: AssignmentOption[], selected: string[]) => {
  const groups: Record<string, AssignmentItem[]> = {};
  options.forEach((opt) => {
    const category = opt.category || 'general';
    const status: AssignmentItem['status'] = selected.includes(opt.value) ? 'active' : 'disabled';
    if (!groups[category]) groups[category] = [];
    groups[category].push({
      id: opt.value,
      name: opt.displayName,
      version: opt.version || '',
      category: opt.category,
      menuNodes: opt.menuNodes,
      menuPath: opt.menuPath,
      menuSource: opt.menuSource,
      status,
    });
  });

  return Object.entries(groups).map(([category, items]) => ({
    category,
    items,
    activeCount: items.filter((i) => i.status === 'active').length,
    canaryCount: items.filter((i) => i.status === 'canary').length,
  }));
};

export const buildAssignmentStats = (options: AssignmentOption[], selected: string[]) => {
  const total = options.length;
  const active = selected.length;
  const inactive = total - active;
  const categories = new Set(options.map((o) => o.category)).size;
  return { total, active, inactive, categories };
};
