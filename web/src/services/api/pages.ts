import { request } from '@umijs/max';
import type {
  Diagnostic,
  PageSpec,
  PageSpecDraft,
  PageSpecDraftSummary,
  PageVersionItem,
} from '@/types/dashboard';

const BASE = '/api/v1/pages';

export type PageDraftListParams = {
  resourceKey?: string;
  status?: PageSpecDraft['status'];
};

type PageDraftListResponse = {
  items?: PageSpecDraftSummary[];
};

type PageDraftResponse = PageSpecDraft;

export type PageSavePayload = PageSpec & {
  draftRevision: number;
};

type PageSaveResponse = {
  pageKey: string;
  draftRevision: number;
};

type PageValidateResponse = {
  valid: boolean;
  diagnostics: Diagnostic[];
};

type PagePreviewResponse = {
  page: PageSpec;
};

type PagePublishResponse = {
  pageKey: string;
  published: boolean;
  publishedVersion: number;
};

type PageUnpublishResponse = {
  pageKey: string;
  published: boolean;
};

type PageVersionsResponse = {
  currentDraftRevision: number;
  currentPublishedVersion?: number;
  total?: number;
  items: PageVersionItem[];
};

type PageVersionDetailResponse = {
  version: number;
  status: string;
  message?: string;
  createdAt: string;
  createdBy?: string;
  page: PageSpec;
};

type PageRollbackResponse = {
  pageKey: string;
  draftRevision: number;
};

type PageRegenerateResponse = {
  pageKey: string;
  draftRevision: number;
  page: PageSpecDraft;
  diagnostics?: Diagnostic[];
  quality: 'ready' | 'basic' | 'needs_review';
};

export async function listPageDrafts(
  params?: PageDraftListParams,
): Promise<PageSpecDraftSummary[]> {
  const response = await request<PageDraftListResponse>(BASE, {
    method: 'GET',
    params,
  });
  return Array.isArray(response?.items) ? response.items : [];
}

export async function getPageDraft(pageKey: string): Promise<PageSpecDraft> {
  return request<PageDraftResponse>(`${BASE}/${encodeURIComponent(pageKey)}`, {
    method: 'GET',
  });
}

export async function savePageDraft(payload: PageSavePayload): Promise<PageSaveResponse> {
  return request<PageSaveResponse>(`${BASE}/${encodeURIComponent(payload.pageKey)}`, {
    method: 'PUT',
    data: payload,
  });
}

export async function regeneratePageDraft(
  pageKey: string,
  draftRevision: number,
): Promise<PageRegenerateResponse> {
  return request<PageRegenerateResponse>(`${BASE}/${encodeURIComponent(pageKey)}/regenerate`, {
    method: 'POST',
    data: { draftRevision },
  });
}

export async function validatePageDraft(pageKey: string): Promise<PageValidateResponse> {
  return request<PageValidateResponse>(`${BASE}/${encodeURIComponent(pageKey)}/validate`, {
    method: 'POST',
  });
}

export async function previewPageDraft(pageKey: string): Promise<PageSpec> {
  const response = await request<PagePreviewResponse>(
    `${BASE}/${encodeURIComponent(pageKey)}/preview`,
    {
      method: 'POST',
    },
  );
  if (!response?.page) {
    throw new Error(`page preview returned empty page: ${pageKey}`);
  }
  return response.page;
}

export async function publishPageDraft(
  pageKey: string,
  draftRevision: number,
): Promise<PagePublishResponse> {
  return request<PagePublishResponse>(`${BASE}/${encodeURIComponent(pageKey)}/publish`, {
    method: 'POST',
    data: { draftRevision },
  });
}

export async function unpublishPage(pageKey: string): Promise<PageUnpublishResponse> {
  return request<PageUnpublishResponse>(`${BASE}/${encodeURIComponent(pageKey)}/unpublish`, {
    method: 'POST',
  });
}

export async function listPageVersions(
  pageKey: string,
  params?: { limit?: number; offset?: number },
): Promise<PageVersionsResponse> {
  return request<PageVersionsResponse>(`${BASE}/${encodeURIComponent(pageKey)}/versions`, {
    method: 'GET',
    params,
  });
}

export async function getPageVersion(
  pageKey: string,
  version: number,
): Promise<PageVersionDetailResponse> {
  return request<PageVersionDetailResponse>(
    `${BASE}/${encodeURIComponent(pageKey)}/versions/${encodeURIComponent(String(version))}`,
    { method: 'GET' },
  );
}

export async function rollbackPageDraft(
  pageKey: string,
  version: number,
  expectedDraftRevision: number,
): Promise<PageRollbackResponse> {
  return request<PageRollbackResponse>(`${BASE}/${encodeURIComponent(pageKey)}/rollback`, {
    method: 'POST',
    data: { versionId: String(version), expectedDraftRevision },
  });
}

// F：一键发布全部（ready/basic 提案走真实 accept-and-publish 链路）
export type PageBulkResult = {
  total: number;
  published?: string[];
  unpublished?: string[];
  skipped?: string[];
  failed?: { pageKey: string; error: string }[];
};

export async function bulkPublishPages(): Promise<PageBulkResult> {
  return request<PageBulkResult>('/api/v1/pages/bulk-publish', { method: 'POST' });
}

export async function bulkUnpublishPages(): Promise<PageBulkResult> {
  return request<PageBulkResult>('/api/v1/pages/bulk-unpublish', { method: 'POST' });
}
