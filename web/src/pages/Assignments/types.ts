export type CanaryConfig = {
  enabled?: boolean;
  percentage?: number;
  rules?: Record<string, unknown>;
  duration?: string;
};

export type AssignmentHistory = {
  id: string;
  gameId: string;
  env: string;
  functionId: string;
  action: 'assign' | 'remove' | string;
  count: number;
  operatedBy: string;
  operatedAt: string;
  details?: Record<string, unknown>;
};

export type HistoryAction = 'all' | 'assign' | 'remove' | 'clone';

export type AssignmentItem = {
  id: string;
  name: string;
  version: string;
  resource: string;
  operation?: string;
  status: 'active' | 'canary' | 'disabled';
  canary?: CanaryConfig;
  assignedAt?: string;
  updatedAt?: string;
  /** 函数目录总开关已禁用（enabled=false）：本页不可勾选，行内标注 */
  directoryDisabled?: boolean;
};

export type AssignmentGroup = {
  resource: string;
  items: AssignmentItem[];
  activeCount: number;
};
