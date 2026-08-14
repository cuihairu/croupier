import { request } from '@umijs/max';

// Source: croupier/internal/api/assignment/dto.go AssignmentsListResponse
export type AssignmentsListPayload = {
  assignments: Record<string, string[]>;
  total?: number;
  page?: number;
  pageSize?: number;
};

// Source: croupier/internal/api/assignment/service.go assignmentHistoryEntry
export type AssignmentHistoryItem = {
  id: string;
  gameId: string;
  env: string;
  functionId: string;
  action: string;
  count: number;
  operatedBy: string;
  operatedAt: string;
  details?: Record<string, string | number | boolean | null | undefined>;
};

// Source: croupier/internal/api/assignment/dto.go AssignmentsHistoryResponse
export type AssignmentsHistoryPayload = {
  items: AssignmentHistoryItem[];
  total: number;
  page: number;
  pageSize: number;
};

// Source: croupier/internal/api/assignment/dto.go AssignmentsUpdateResponse
export type AssignmentsUpdatePayload = {
  ok: boolean;
  unknown?: string[];
  assignments?: Record<string, string[]>;
};

export async function fetchAssignments(params?: {
  gameId?: string;
  env?: string;
}): Promise<AssignmentsListPayload> {
  const resp = await request<AssignmentsListPayload>('/api/v1/assignments', {
    params,
  });
  return (
    resp || {
      assignments: {},
      total: 0,
      page: 1,
      pageSize: 20,
    }
  );
}

export async function setAssignments(params: {
  action?: 'assign' | 'clone' | 'remove' | string;
  targetEnv?: string;
  functions: string[];
}): Promise<AssignmentsUpdatePayload> {
  const resp = await request<AssignmentsUpdatePayload>('/api/v1/assignments', {
    method: 'PUT',
    data: params,
  });
  return resp || { ok: false, unknown: [], assignments: {} };
}

export async function fetchAssignmentsHistory(params?: {
  gameId?: string;
  env?: string;
  action?: string;
  page?: number;
  pageSize?: number;
}): Promise<AssignmentsHistoryPayload> {
  const resp = await request<AssignmentsHistoryPayload>('/api/v1/assignments/history', { params });
  return (
    resp || {
      items: [],
      total: 0,
      page: params?.page || 1,
      pageSize: params?.pageSize || 20,
    }
  );
}
