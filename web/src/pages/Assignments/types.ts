export type CanaryConfig = {
  enabled?: boolean;
  percentage?: number;
  rules?: Record<string, any>;
  duration?: string;
};

export type AssignmentHistory = {
  id: string;
  game_id: string;
  env: string;
  function_id: string;
  action: 'assign' | 'remove' | string;
  count: number;
  operated_by: string;
  operated_at: string;
  details?: Record<string, any>;
};

export type HistoryAction = 'all' | 'assign' | 'remove' | 'clone';

export type AssignmentItem = {
  id: string;
  name: string;
  version: string;
  category: string;
  menuNodes?: string[];
  menuPath?: string;
  menuSource?: string;
  status: 'active' | 'canary' | 'disabled';
  canary?: CanaryConfig;
  assignedAt?: string;
  updatedAt?: string;
};

export type AssignmentGroup = {
  category: string;
  items: AssignmentItem[];
  activeCount: number;
  canaryCount: number;
};
