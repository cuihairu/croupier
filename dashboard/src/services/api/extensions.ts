import { request } from '@umijs/max';

const BASE = '/api/v1/extensions';

// Source: croupier/internal/api/extension/dto.go ExtensionCatalogItem.
export type ExtensionCatalogItem = {
  id: string;
  name: string;
  displayName: string;
  vendor: string;
  kind: string;
  summary: string;
  iconUrl: string;
  status: string;
  latestVersion: string;
  installed: boolean;
  defaultInstall: boolean;
  tags: string[];
};

// Source: croupier/internal/api/extension/dto.go ExtensionReleaseItem.
export type ExtensionReleaseItem = {
  version: string;
  releaseChannel: string;
  minCoreVersion: string;
  publishedAt: number;
  changelog: string;
};

// Source: croupier/internal/api/extension/dto.go ExtensionInstallationItem.
export type ExtensionInstallationItem = {
  id: number;
  installationKey: string;
  extensionId: string;
  displayName: string;
  releaseVersion: string;
  scopeType: string;
  scopeId: string;
  targetType: string;
  targetId: string;
  status: string;
  desiredState: string;
  enabled: boolean;
  healthStatus: string;
  lastError: string;
  updatedAt: number;
};

// Source: croupier/internal/api/extension/dto.go ExtensionBindingItem.
export type ExtensionBindingItem = {
  bindingType: string;
  bindingKey: string;
  targetRef: string;
  status: string;
  lastError: string;
};

// Source: croupier/internal/api/extension/dto.go ExtensionEventItem.
export type ExtensionEventItem = {
  eventType: string;
  level: string;
  message: string;
  payload: string;
  createdBy: string;
  createdAt: number;
};

// Source: croupier/internal/api/extension/dto.go request DTOs.
export type ExtensionCatalogListParams = {
  keyword?: string;
  kind?: string;
  status?: string;
  page?: number;
  pageSize?: number;
};

export type ExtensionInstallationListParams = {
  extensionId?: string;
  scopeType?: string;
  scopeId?: string;
  targetType?: string;
  targetId?: string;
  status?: string;
  enabled?: boolean;
  page?: number;
  pageSize?: number;
};

export type ExtensionInstallRequest = {
  extensionId: string;
  releaseVersion: string;
  scopeType: string;
  scopeId: string;
  targetType: string;
  targetId?: string;
  config?: Record<string, any>;
  secretRefs?: Record<string, string>;
};

type RawExtensionCatalogItem = {
  id: string;
  name: string;
  display_name: string;
  vendor: string;
  kind: string;
  summary: string;
  icon_url: string;
  status: string;
  latest_version: string;
  installed: boolean;
  default_install: boolean;
  tags: string[];
};

type RawExtensionReleaseItem = {
  version: string;
  release_channel: string;
  min_core_version: string;
  published_at: number;
  changelog: string;
};

type RawExtensionInstallationItem = {
  id: number;
  installation_key: string;
  extension_id: string;
  display_name: string;
  release_version: string;
  scope_type: string;
  scope_id: string;
  target_type: string;
  target_id: string;
  status: string;
  desired_state: string;
  enabled: boolean;
  health_status: string;
  last_error: string;
  updated_at: number;
};

type RawExtensionBindingItem = {
  binding_type: string;
  binding_key: string;
  target_ref: string;
  status: string;
  last_error: string;
};

type RawExtensionEventItem = {
  event_type: string;
  level: string;
  message: string;
  payload: string;
  created_by: string;
  created_at: number;
};

function normalizeCatalogItem(raw: RawExtensionCatalogItem): ExtensionCatalogItem {
  return {
    id: raw.id,
    name: raw.name,
    displayName: raw.display_name,
    vendor: raw.vendor,
    kind: raw.kind,
    summary: raw.summary,
    iconUrl: raw.icon_url,
    status: raw.status,
    latestVersion: raw.latest_version,
    installed: !!raw.installed,
    defaultInstall: !!raw.default_install,
    tags: Array.isArray(raw.tags) ? raw.tags : [],
  };
}

function normalizeReleaseItem(raw: RawExtensionReleaseItem): ExtensionReleaseItem {
  return {
    version: raw.version,
    releaseChannel: raw.release_channel,
    minCoreVersion: raw.min_core_version,
    publishedAt: raw.published_at,
    changelog: raw.changelog,
  };
}

function normalizeInstallationItem(raw: RawExtensionInstallationItem): ExtensionInstallationItem {
  return {
    id: raw.id,
    installationKey: raw.installation_key,
    extensionId: raw.extension_id,
    displayName: raw.display_name,
    releaseVersion: raw.release_version,
    scopeType: raw.scope_type,
    scopeId: raw.scope_id,
    targetType: raw.target_type,
    targetId: raw.target_id,
    status: raw.status,
    desiredState: raw.desired_state,
    enabled: !!raw.enabled,
    healthStatus: raw.health_status,
    lastError: raw.last_error,
    updatedAt: raw.updated_at,
  };
}

function normalizeBindingItem(raw: RawExtensionBindingItem): ExtensionBindingItem {
  return {
    bindingType: raw.binding_type,
    bindingKey: raw.binding_key,
    targetRef: raw.target_ref,
    status: raw.status,
    lastError: raw.last_error,
  };
}

function normalizeEventItem(raw: RawExtensionEventItem): ExtensionEventItem {
  return {
    eventType: raw.event_type,
    level: raw.level,
    message: raw.message,
    payload: raw.payload,
    createdBy: raw.created_by,
    createdAt: raw.created_at,
  };
}

export async function listExtensionCatalog(params?: ExtensionCatalogListParams) {
  const response = await request<{ total: number; items: RawExtensionCatalogItem[] }>(
    `${BASE}/catalog`,
    {
      params: {
        keyword: params?.keyword,
        kind: params?.kind,
        status: params?.status,
        page: params?.page,
        page_size: params?.pageSize,
      },
    },
  );
  return {
    total: Number(response?.total || 0),
    items: (response?.items || []).map(normalizeCatalogItem),
  };
}

export async function getExtensionCatalogDetail(id: string) {
  const response = await request<{
    item?: RawExtensionCatalogItem;
    releases?: RawExtensionReleaseItem[];
    manifest?: Record<string, any>;
    capabilities?: string[];
  }>(`${BASE}/catalog/${encodeURIComponent(id)}`);
  return {
    item: response?.item ? normalizeCatalogItem(response.item) : undefined,
    releases: (response?.releases || []).map(normalizeReleaseItem),
    manifest: response?.manifest,
    capabilities: Array.isArray(response?.capabilities) ? response.capabilities : [],
  };
}

export async function listExtensionCatalogReleases(id: string) {
  const response = await request<{ total: number; releases: RawExtensionReleaseItem[] }>(
    `${BASE}/catalog/${encodeURIComponent(id)}/releases`,
  );
  return {
    total: Number(response?.total || 0),
    releases: (response?.releases || []).map(normalizeReleaseItem),
  };
}

export async function listExtensionInstallations(params?: ExtensionInstallationListParams) {
  const response = await request<{ total: number; items: RawExtensionInstallationItem[] }>(
    `${BASE}/installations`,
    {
      params: {
        extension_id: params?.extensionId,
        scope_type: params?.scopeType,
        scope_id: params?.scopeId,
        target_type: params?.targetType,
        target_id: params?.targetId,
        status: params?.status,
        enabled: params?.enabled,
        page: params?.page,
        page_size: params?.pageSize,
      },
    },
  );
  return {
    total: Number(response?.total || 0),
    items: (response?.items || []).map(normalizeInstallationItem),
  };
}

export async function installExtension(data: ExtensionInstallRequest) {
  return request<{ installationId: number; status: string }>(`${BASE}/install`, {
    method: 'POST',
    data: {
      extension_id: data.extensionId,
      release_version: data.releaseVersion,
      scope_type: data.scopeType,
      scope_id: data.scopeId,
      target_type: data.targetType,
      target_id: data.targetId,
      config: data.config,
      secret_refs: data.secretRefs,
    },
  }).then((response: any) => ({
    installationId: response?.installation_id ?? response?.installationId ?? 0,
    status: response?.status ?? '',
  }));
}

export async function getExtensionInstallationDetail(id: number) {
  const response = await request<{
    installation?: RawExtensionInstallationItem;
    config_schema?: Record<string, any>;
    config?: Record<string, any>;
    secret_refs?: Record<string, string>;
    bindings?: RawExtensionBindingItem[];
    events?: RawExtensionEventItem[];
  }>(`${BASE}/installations/${id}`);
  return {
    installation: response?.installation
      ? normalizeInstallationItem(response.installation)
      : undefined,
    configSchema: response?.config_schema,
    config: response?.config || {},
    secretRefs: response?.secret_refs || {},
    bindings: (response?.bindings || []).map(normalizeBindingItem),
    events: (response?.events || []).map(normalizeEventItem),
  };
}

export async function updateExtensionConfig(
  id: number,
  data: { config?: Record<string, any>; secretRefs?: Record<string, string> },
) {
  return request<{ status: string }>(`${BASE}/installations/${id}/config`, {
    method: 'PUT',
    data: {
      config: data.config,
      secret_refs: data.secretRefs,
    },
  });
}

export async function getExtensionConfigSchema(id: number) {
  return request<{ schema: Record<string, any> }>(`${BASE}/installations/${id}/config-schema`);
}

export async function getExtensionConfig(id: number) {
  const response = await request<{
    config: Record<string, any>;
    secret_refs: Record<string, string>;
  }>(`${BASE}/installations/${id}/config`);
  return {
    config: response?.config || {},
    secretRefs: response?.secret_refs || {},
  };
}

export async function testExtensionConnection(id: number) {
  return request<{ status: string }>(`${BASE}/installations/${id}/test-connection`, {
    method: 'POST',
  });
}

export async function getExtensionCapabilities(id: number) {
  return request<{ capabilities: string[] }>(`${BASE}/installations/${id}/capabilities`);
}

export async function runExtensionHealthCheck(id: number) {
  return request<{ status: string; checkedAt: number }>(
    `${BASE}/installations/${id}/health-check`,
    { method: 'POST' },
  ).then((response: any) => ({
    status: response?.status ?? '',
    checkedAt: response?.checked_at ?? response?.checkedAt ?? 0,
  }));
}

export async function enableExtension(id: number) {
  return request<{ status: string }>(`${BASE}/installations/${id}/enable`, { method: 'POST' });
}

export async function disableExtension(id: number) {
  return request<{ status: string }>(`${BASE}/installations/${id}/disable`, { method: 'POST' });
}

export async function upgradeExtension(id: number, releaseVersion: string) {
  return request<{ status: string }>(`${BASE}/installations/${id}/upgrade`, {
    method: 'POST',
    data: { release_version: releaseVersion },
  });
}

export async function reconcileExtension(id: number) {
  return request<{ status: string; applied: number; failed: number }>(
    `${BASE}/installations/${id}/reconcile`,
    { method: 'POST' },
  );
}

export async function uninstallExtension(id: number) {
  return request<{ status: string }>(`${BASE}/installations/${id}`, {
    method: 'DELETE',
  });
}

export async function listExtensionEvents(
  id: number,
  params?: { level?: string; keyword?: string; page?: number; pageSize?: number },
) {
  const response = await request<{ total: number; items: RawExtensionEventItem[] }>(
    `${BASE}/installations/${id}/events`,
    {
      params: {
        level: params?.level,
        keyword: params?.keyword,
        page: params?.page,
        page_size: params?.pageSize,
      },
    },
  );
  return {
    total: Number(response?.total || 0),
    items: (response?.items || []).map(normalizeEventItem),
  };
}

export async function getAgentSyncPayload(agentId: string) {
  return request<{ payload: any }>(`${BASE}/agents/${encodeURIComponent(agentId)}/sync-payload`);
}

export async function listExtensionPages(id: string | number) {
  const response = await request<{
    items?: Array<{
      id?: string;
      title?: string;
      path?: string;
      icon?: string;
      order?: number;
      category?: string;
      extension_id?: string;
    }>;
  }>(`${BASE}/${encodeURIComponent(String(id))}/pages`);
  return {
    items: (response?.items || []).map((item) => ({
      id: item.id,
      title: item.title,
      path: item.path,
      icon: item.icon,
      order: item.order,
      category: item.category,
      extensionId: item.extension_id,
    })),
  };
}
