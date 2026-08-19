import type { FunctionDescriptor } from '@/services/api';
import type { AssignmentItem } from './types';
import { localizedText } from '@/utils/localizedText';

export type AssignmentOption = {
  label: string;
  value: string;
  version?: string;
  resource: string;
  operation?: string;
  displayName: string;
};

export const buildAssignmentOptions = (descs: FunctionDescriptor[]): AssignmentOption[] =>
  (Array.isArray(descs) ? descs : []).map((d) => ({
    label: `${d.id} v${d.version || ''}`,
    value: d.id,
    version: d.version,
    resource: d.resource || 'unassigned',
    operation: d.operation,
    displayName:
      localizedText(d.displayName, 'zh-CN', '') || localizedText(d.summary, 'zh-CN', '') || d.id,
  }));

export const buildGroupedAssignments = (options: AssignmentOption[], selected: string[]) => {
  const groups: Record<string, AssignmentItem[]> = {};
  options.forEach((opt) => {
    const resource = opt.resource || 'unassigned';
    const status: AssignmentItem['status'] = selected.includes(opt.value) ? 'active' : 'disabled';
    if (!groups[resource]) groups[resource] = [];
    groups[resource].push({
      id: opt.value,
      name: opt.displayName,
      version: opt.version || '',
      resource: opt.resource,
      operation: opt.operation,
      status,
    });
  });

  return Object.entries(groups).map(([resource, items]) => ({
    resource,
    items,
    activeCount: items.filter((i) => i.status === 'active').length,
    canaryCount: items.filter((i) => i.status === 'canary').length,
  }));
};

export const buildAssignmentStats = (options: AssignmentOption[], selected: string[]) => {
  const total = options.length;
  const active = selected.length;
  const inactive = total - active;
  const resources = new Set(options.map((o) => o.resource)).size;
  return { total, active, inactive, resources };
};
