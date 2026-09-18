import { request } from '@umijs/max';
import type {
  BindingSelectorSyncReport,
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

export type PageSyncSelectorsPayload = {
  draftRevision: number;
  dryRun: boolean;
  bindingIds?: string[];
};

type PageSyncSelectorsResponse = {
  pageKey: string;
  dryRun: boolean;
  applied: boolean;
  draftRevision: number;
  syncedBindings: BindingSelectorSyncReport[];
  remainingDiagnostics?: Diagnostic[];
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

export async function syncPageSelectors(
  pageKey: string,
  payload: PageSyncSelectorsPayload,
): Promise<PageSyncSelectorsResponse> {
  return request<PageSyncSelectorsResponse>(
    `${BASE}/${encodeURIComponent(pageKey)}/sync-selectors`,
    {
      method: 'POST',
      data: payload,
    },
  );
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

/** 挂载响应：pageKey + 当前 menuId（null=已解除）。 */
export interface PageMenuResponse {
  pageKey: string;
  menuId: number | null;
}

/**
 * 挂载页面到菜单（menuId 非 null）或解除挂载（null）。
 * 挂载读 draft 表即时生效，无需重发页面；要求 pages:edit。
 */
export async function updatePageMenu(
  pageKey: string,
  menuId: number | null,
): Promise<PageMenuResponse> {
  return request<PageMenuResponse>(`${BASE}/${encodeURIComponent(pageKey)}/menu`, {
    method: 'PUT',
    data: { menuId },
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

/** 一键重新发布契约变更队列：pageKeys 为空时处理 scope 内全部 stale 已发布页面。 */
export async function bulkRepublishPages(pageKeys?: string[]): Promise<PageBulkResult> {
  return request<PageBulkResult>('/api/v1/pages/bulk-republish', {
    method: 'POST',
    data: pageKeys ? { pageKeys } : {},
  });
}

/** 批量 selector 同步中被跳过的页面（governance/版本等不可同步修复的漂移，诊断透传）。 */
export type PageBulkSyncSkipped = {
  pageKey: string;
  reason: string;
  manual?: Diagnostic[];
};

/** 批量 selector 同步结果：同步只写草稿，上线仍需 bulk-republish。 */
export type PageBulkSyncSelectorsResult = {
  total: number;
  synced?: string[];
  skipped?: PageBulkSyncSkipped[];
  failed?: { pageKey: string; error: string }[];
};

/** 批量同步契约变更队列的 stale selector：pageKeys 为空时处理 scope 内全部契约变更页面。 */
export async function bulkSyncPageSelectors(
  pageKeys?: string[],
): Promise<PageBulkSyncSelectorsResult> {
  return request<PageBulkSyncSelectorsResult>('/api/v1/pages/bulk-sync-selectors', {
    method: 'POST',
    data: pageKeys ? { pageKeys } : {},
  });
}
