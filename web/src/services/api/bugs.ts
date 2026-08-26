import { request } from '@umijs/max';

// Source: internal/api/bug/dto.go
export type BugLink = {
  url: string;
  kind: 'github_issue' | 'github_pr' | 'gitlab' | 'jira' | 'wiki' | 'monitor' | 'other';
  title?: string;
};

export type BugItem = {
  id: number;
  title: string;
  content?: string;
  status: string;
  severity?: string;
  priority?: string;
  assignee?: string;
  gameId?: string;
  env?: string;
  serverId?: string;
  platform?: string;
  device?: string;
  os?: string;
  steps?: string;
  reproducibility?: string;
  affectsVersion?: string;
  fixVersion?: string;
  source?: string;
  sourceTicketId?: number;
  playerId?: string;
  links?: BugLink[];
  extra?: Record<string, unknown>;
  createdBy?: string;
  createdAt: string;
  updatedAt: string;
};

export type BugListParams = {
  q?: string;
  status?: string;
  severity?: string;
  priority?: string;
  assignee?: string;
  gameId?: string;
  env?: string;
  platform?: string;
  fixVersion?: string;
  playerId?: string;
  page?: number;
  pageSize?: number;
};

export type BugListResponse = {
  items: BugItem[];
  total: number;
  page: number;
  pageSize: number;
};

export type BugCreatePayload = {
  title: string;
  content?: string;
  status?: string;
  severity?: string;
  priority?: string;
  assignee?: string;
  gameId?: string;
  env?: string;
  serverId?: string;
  platform?: string;
  device?: string;
  os?: string;
  steps?: string;
  reproducibility?: string;
  affectsVersion?: string;
  fixVersion?: string;
  source?: string;
  sourceTicketId?: number;
  playerId?: string;
  links?: BugLink[];
  extra?: Record<string, unknown>;
};

export type BugUpdatePayload = {
  title?: string;
  content?: string;
  status?: string;
  severity?: string;
  priority?: string;
  assignee?: string;
  steps?: string;
  reproducibility?: string;
  affectsVersion?: string;
  fixVersion?: string;
  platform?: string;
  links?: BugLink[];
};

const BASE = '/api/v1/bugs';

export async function listBugs(params?: BugListParams): Promise<BugListResponse> {
  return request<BugListResponse>(BASE, { params });
}

export async function getBug(id: number | string): Promise<BugItem> {
  return request<BugItem>(`${BASE}/${encodeURIComponent(String(id))}`, { method: 'GET' });
}

export async function createBug(payload: BugCreatePayload): Promise<BugItem> {
  return request<BugItem>(BASE, { method: 'POST', data: payload });
}

export async function updateBug(id: number | string, payload: BugUpdatePayload): Promise<BugItem> {
  return request<BugItem>(`${BASE}/${encodeURIComponent(String(id))}`, {
    method: 'PUT',
    data: payload,
  });
}

export async function deleteBug(id: number | string): Promise<void> {
  return request<void>(`${BASE}/${encodeURIComponent(String(id))}`, { method: 'DELETE' });
}

// Status/severity/priority vocabularies mirror internal/model/bug.go.
export const BUG_STATUS_FLOW = ['triage', 'confirmed', 'fixing', 'verify', 'released'] as const;
export const BUG_STATUS_TERMINALS = ['wontfix', 'rejected'] as const;

export const bugStatusLabels: Record<string, string> = {
  triage: '待分诊',
  confirmed: '已确认',
  fixing: '修复中',
  verify: '待验证',
  released: '已发布',
  wontfix: '不修复',
  rejected: '驳回',
};

export const bugStatusColors: Record<string, string> = {
  triage: 'purple',
  confirmed: 'orange',
  fixing: 'blue',
  verify: 'cyan',
  released: 'green',
  wontfix: 'default',
  rejected: 'default',
};

export const bugSeverityLabels: Record<string, string> = {
  blocker: '阻断',
  critical: '严重',
  major: '一般',
  minor: '轻微',
};

export const bugSeverityColors: Record<string, string> = {
  blocker: 'red',
  critical: 'volcano',
  major: 'orange',
  minor: 'blue',
};

export const bugPriorityLabels: Record<string, string> = {
  urgent: '紧急',
  high: '高',
  normal: '中',
  low: '低',
};

export const bugReproducibilityLabels: Record<string, string> = {
  always: '必现',
  often: '经常',
  sometimes: '偶现',
  once: '仅一次',
};

export const bugPlatformOptions = [
  { label: 'iOS', value: 'ios' },
  { label: 'Android', value: 'android' },
  { label: 'PC', value: 'pc' },
  { label: 'WebGL', value: 'webgl' },
  { label: '编辑器', value: 'editor' },
];

export const bugLinkKindOptions = [
  { label: 'GitHub Issue', value: 'github_issue' },
  { label: 'GitHub PR', value: 'github_pr' },
  { label: 'GitLab', value: 'gitlab' },
  { label: 'Jira', value: 'jira' },
  { label: 'Wiki', value: 'wiki' },
  { label: '监控面板', value: 'monitor' },
  { label: '其他', value: 'other' },
];

// deriveBugLinkTitle guesses a display title from a URL, e.g.
// https://github.com/o/r/pull/123 -> "o/r#123".
export function deriveBugLinkTitle(url: string, kind: string): string {
  try {
    const u = new URL(url);
    if (kind === 'github_issue' || kind === 'github_pr') {
      const m = u.pathname.match(/^\/([^/]+)\/([^/]+)\/(?:issues|pull)\/(\d+)/);
      if (m) return `${m[1]}/${m[2]}#${m[3]}`;
    }
    return u.hostname + (u.pathname !== '/' ? u.pathname : '');
  } catch {
    return url;
  }
}
