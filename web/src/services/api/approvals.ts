import { request } from '@umijs/max';

// Source: croupier/internal/api/approval/dto.go ApprovalSummary / Approval
export type ApprovalRow = {
  id: string;
  created_at: string;
  updated_at?: string;
  actor: string;
  function_id: string;
  game_id?: string;
  env?: string;
  state: 'pending' | 'approved' | 'rejected';
  mode: 'invoke' | 'start_job';
  route?: string;
  reason?: string;
  idempotency_key?: string;
  target_service_id?: string;
  hash_key?: string;
  // Detail-only fields
  payload?: any;
  payload_preview?: string;
};

// Source: croupier/internal/api/approval/dto.go ApprovalsListRequest
export type ApprovalListParams = {
  page?: number;
  pageSize?: number;
  status?: string;
};

export async function listApprovals(params?: ApprovalListParams) {
  return request<{ approvals?: ApprovalRow[]; total?: number; page?: number; size?: number }>(
    '/api/v1/approvals/',
    { params },
  );
}

export async function getApproval(id: string) {
  return request<ApprovalRow>(`/api/v1/approvals/${encodeURIComponent(id)}`);
}

export async function approveApproval(data: { id: string; otp?: string }) {
  return request<{ id: string; state: string }>(
    `/api/v1/approvals/${encodeURIComponent(data.id)}/approve`,
    {
      method: 'POST',
      data: data.otp ? { otp: data.otp } : undefined,
    },
  );
}

export async function rejectApproval(data: { id: string; reason: string }) {
  return request<{ id: string; state: string; reason: string }>(
    `/api/v1/approvals/${encodeURIComponent(data.id)}/reject`,
    {
      method: 'POST',
      data: { reason: data.reason },
    },
  );
}
