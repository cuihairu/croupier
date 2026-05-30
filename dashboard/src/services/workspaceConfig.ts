/**
 * Workspace 配置服务
 *
 * 提供 Workspace 配置的加载、保存、缓存等功能。
 *
 * @module services/workspaceConfig
 */

import type { WorkspaceConfig, WorkspaceVersionRecord } from '@/types/workspace';
import { request } from '@umijs/max';
import {
  mockLoadWorkspaceConfig,
  mockSaveWorkspaceConfig,
  mockListWorkspaceConfigs,
  mockDeleteWorkspaceConfig,
  mockPublishWorkspaceConfig,
  mockUnpublishWorkspaceConfig,
  mockListPublishedWorkspaceConfigs,
} from './mock/workspaceMock';

// Workspace 配置接口后端已实现
const USE_MOCK = false;

/**
 * 配置缓存
 *
 * 使用 Map 缓存配置，减少 API 调用。
 */
const configCache = new Map<string, WorkspaceConfig>();

/**
 * 缓存过期时间（毫秒）
 */
const CACHE_TTL = 5 * 60 * 1000; // 5 分钟

export interface WorkspacePublishResult {
  published: boolean;
  objectKey: string;
}

export interface WorkspaceRollbackResult {
  objectKey: string;
  version: number;
}

export interface ListWorkspaceVersionsParams {
  from?: string;
  to?: string;
}

export interface WorkspaceExportMetadata {
  objectKey: string;
  title?: string;
  description?: string;
  status: string;
  version?: number;
  published: boolean;
  publishedAt?: string;
  publishedBy?: string;
  createdAt?: string;
  updatedAt?: string;
  currentDraftVersion?: number;
  currentPublishedVersion?: number;
  versionCount: number;
  exportedAt: string;
}

export interface ImportWorkspaceConfigOptions {
  targetObjectKey?: string;
  forceDraft?: boolean;
}

export interface WorkspaceBackupBundle {
  objectKey: string;
  exportedAt: string;
  metadata: WorkspaceExportMetadata;
  currentDraft: WorkspaceConfig | null;
  currentPublished: WorkspaceConfig | null;
  versions: WorkspaceVersionRecord[];
}

/**
 * 缓存时间戳
 */
const cacheTimestamps = new Map<string, number>();

/**
 * 检查缓存是否过期
 */
function isCacheExpired(objectKey: string): boolean {
  const timestamp = cacheTimestamps.get(objectKey);
  if (!timestamp) return true;
  return Date.now() - timestamp > CACHE_TTL;
}

/**
 * 设置缓存
 */
function setCache(objectKey: string, config: WorkspaceConfig): void {
  configCache.set(objectKey, config);
  cacheTimestamps.set(objectKey, Date.now());
}

/**
 * 获取缓存
 */
function getCache(objectKey: string): WorkspaceConfig | undefined {
  if (isCacheExpired(objectKey)) {
    configCache.delete(objectKey);
    cacheTimestamps.delete(objectKey);
    return undefined;
  }
  return configCache.get(objectKey);
}

/**
 * 清除缓存
 */
function clearCache(objectKey?: string): void {
  if (objectKey) {
    configCache.delete(objectKey);
    cacheTimestamps.delete(objectKey);
  } else {
    configCache.clear();
    cacheTimestamps.clear();
  }
}

/**
 * 加载 Workspace 配置
 *
 * @param objectKey - 对象标识
 * @param options - 加载选项
 * @returns Workspace 配置
 */
export async function loadWorkspaceConfig(
  objectKey: string,
  options?: {
    /** 是否强制刷新（跳过缓存） */
    forceRefresh?: boolean;
    /** 是否使用缓存 */
    useCache?: boolean;
  },
): Promise<WorkspaceConfig | null> {
  const { forceRefresh = false, useCache = true } = options || {};

  // 检查缓存
  if (useCache && !forceRefresh) {
    const cached = getCache(objectKey);
    if (cached) {
      return cached;
    }
  }

  try {
    // 如果使用 Mock 数据
    if (USE_MOCK) {
      const config = await mockLoadWorkspaceConfig(objectKey);
      if (useCache && config) {
        setCache(objectKey, config);
      }
      return config;
    }

    // 调用 API 加载配置，后端返回 {workspaceConfig: {...}} 结构
    // skipErrorHandler: true 避免配置不存在时显示 404 错误提示
    const response = await request<{ workspaceConfig?: WorkspaceConfig | null }>(
      `/api/v1/workspaces/${objectKey}/config`,
      {
        method: 'GET',
        skipErrorHandler: true,
      },
    );

    // 从嵌套结构中提取配置
    const config = response?.workspaceConfig ?? null;

    // 设置缓存
    if (useCache && config) {
      setCache(objectKey, config);
    }

    return config;
  } catch (error: any) {
    // 如果是 404，返回 null（配置不存在）
    if (error.response?.status === 404) {
      return null;
    }
    // 其他错误抛出
    throw error;
  }
}

/**
 * 保存 Workspace 配置
 *
 * @param config - Workspace 配置
 * @returns 保存后的配置
 */
export async function saveWorkspaceConfig(config: WorkspaceConfig): Promise<WorkspaceConfig> {
  try {
    // 如果使用 Mock 数据
    if (USE_MOCK) {
      const savedConfig = await mockSaveWorkspaceConfig(config);
      setCache(config.objectKey, savedConfig);
      return savedConfig;
    }

    // 调用 API 保存配置，后端返回 {workspaceConfig: {...}} 结构
    const response = await request<{ workspaceConfig?: WorkspaceConfig }>(
      `/api/v1/workspaces/${config.objectKey}/config`,
      {
        method: 'PUT',
        data: config,
      },
    );

    // 从嵌套结构中提取配置
    const savedConfig = response?.workspaceConfig;

    if (!savedConfig) {
      throw new Error('Failed to save workspace config');
    }

    // 更新缓存
    setCache(config.objectKey, savedConfig);

    return savedConfig;
  } catch (error) {
    throw error;
  }
}

/**
 * 获取 Workspace 配置列表
 *
 * @returns 配置列表
 */
export async function listWorkspaceConfigs(): Promise<WorkspaceConfig[]> {
  try {
    // 如果使用 Mock 数据
    if (USE_MOCK) {
      const configs = await mockListWorkspaceConfigs();
      configs.forEach((config) => {
        setCache(config.objectKey, config);
      });
      return configs;
    }

    const response = await request<{ items: WorkspaceConfig[] }>('/api/v1/workspaces/configs', {
      method: 'GET',
    });

    // 后端返回 { items: [...] } 结构，提取数组
    const configs = Array.isArray(response?.items) ? response.items : [];

    // 更新缓存
    configs.forEach((config) => {
      setCache(config.objectKey, config);
    });

    return configs;
  } catch (error) {
    throw error;
  }
}

/**
 * 删除 Workspace 配置
 *
 * @param objectKey - 对象标识
 */
export async function deleteWorkspaceConfig(objectKey: string): Promise<void> {
  try {
    // 如果使用 Mock 数据
    if (USE_MOCK) {
      await mockDeleteWorkspaceConfig(objectKey);
      clearCache(objectKey);
      return;
    }

    await request(`/api/v1/workspaces/${objectKey}/config`, {
      method: 'DELETE',
    });

    // 清除缓存
    clearCache(objectKey);
  } catch (error) {
    throw error;
  }
}

/**
 * 发布 Workspace 配置
 */
export async function publishWorkspaceConfig(objectKey: string): Promise<WorkspacePublishResult> {
  if (USE_MOCK) {
    const config = await mockPublishWorkspaceConfig(objectKey);
    setCache(objectKey, config);
    return { published: true, objectKey };
  }
  const response = await request<WorkspacePublishResult>(
    `/api/v1/workspaces/${objectKey}/publish`,
    {
      method: 'POST',
    },
  );
  // 清除缓存以便下次加载时获取最新状态
  clearCache(objectKey);
  return response;
}

/**
 * 取消发布 Workspace 配置
 */
export async function unpublishWorkspaceConfig(objectKey: string): Promise<WorkspacePublishResult> {
  if (USE_MOCK) {
    const config = await mockUnpublishWorkspaceConfig(objectKey);
    setCache(objectKey, config);
    return { published: false, objectKey };
  }
  const response = await request<WorkspacePublishResult>(
    `/api/v1/workspaces/${objectKey}/unpublish`,
    {
      method: 'POST',
    },
  );
  // 清除缓存以便下次加载时获取最新状态
  clearCache(objectKey);
  return response;
}

/**
 * 获取已发布的 Workspace 列表（控制台用）
 */
export async function listPublishedWorkspaceConfigs(): Promise<WorkspaceConfig[]> {
  if (USE_MOCK) {
    return mockListPublishedWorkspaceConfigs();
  }
  const response = await request<{ items: WorkspaceConfig[] }>('/api/v1/workspaces/published', {
    method: 'GET',
  });
  return Array.isArray(response?.items) ? response.items : [];
}

/**
 * 验证 Workspace 配置
 *
 * @param config - Workspace 配置
 * @returns 验证结果
 */
export function validateWorkspaceConfig(config: WorkspaceConfig): {
  valid: boolean;
  errors: string[];
} {
  const errors: string[] = [];
  const supportedLayoutTypes = new Set([
    'list',
    'form',
    'detail',
    'form-detail',
    'kanban',
    'timeline',
    'split',
    'wizard',
    'dashboard',
    'grid',
    'custom',
    'single',
  ]);

  // 验证必填字段
  if (!config.objectKey) {
    errors.push('objectKey 不能为空');
  }

  if (!config.title) {
    errors.push('title 不能为空');
  }

  if (!config.layout) {
    errors.push('layout 不能为空');
  }

  // 验证布局
  if (config.layout) {
    if (!config.layout.type) {
      errors.push('layout.type 不能为空');
    }

    if (config.layout.type !== 'tabs') {
      errors.push(`当前正式工作台仍要求 tabs 顶层布局，当前为: ${config.layout.type}`);
    }

    if (config.layout.type === 'tabs') {
      if (!config.layout.tabs || config.layout.tabs.length === 0) {
        errors.push('tabs 布局至少需要一个 tab');
      }

      // 检测 tab key 重复
      const tabKeys = new Set<string>();
      config.layout.tabs?.forEach((tab, index) => {
        if (tab.key) {
          if (tabKeys.has(tab.key)) {
            errors.push(`tabs[${index}].key 重复: "${tab.key}"，每个 Tab 的 key 必须唯一`);
          }
          tabKeys.add(tab.key);
        }
      });

      // 验证每个 tab
      config.layout.tabs?.forEach((tab, index) => {
        if (!tab.key) {
          errors.push(`tabs[${index}].key 不能为空`);
        }
        if (!tab.title) {
          errors.push(`tabs[${index}].title 不能为空`);
        }
        if (!tab.layout) {
          errors.push(`tabs[${index}].layout 不能为空`);
        }
        if (!tab.layout?.type) {
          errors.push(`tabs[${index}].layout.type 不能为空`);
        }
        if (tab.layout?.type && !supportedLayoutTypes.has(tab.layout.type)) {
          errors.push(
            `tabs[${index}].layout.type 不受支持: ${tab.layout.type}，仅支持 list/form/detail/form-detail/kanban/timeline/split/wizard/dashboard/grid/custom/single`,
          );
        }

        if (tab.layout?.type === 'list') {
          if (!tab.layout.listFunction) {
            errors.push(`tabs[${index}].listFunction 不能为空`);
          }
          if (!Array.isArray(tab.layout.columns) || tab.layout.columns.length === 0) {
            errors.push(`tabs[${index}].columns 至少需要一列`);
          }
          // 检测列 key 重复
          if (Array.isArray(tab.layout.columns)) {
            const colKeys = new Set<string>();
            tab.layout.columns.forEach((col: any, ci: number) => {
              if (col.key) {
                if (colKeys.has(col.key)) {
                  errors.push(`tabs[${index}].columns[${ci}].key 重复: "${col.key}"`);
                }
                colKeys.add(col.key);
              }
            });
          }
        }

        if (tab.layout?.type === 'form') {
          if (!tab.layout.submitFunction) {
            errors.push(`tabs[${index}].submitFunction 不能为空`);
          }
          if (!Array.isArray(tab.layout.fields) || tab.layout.fields.length === 0) {
            errors.push(`tabs[${index}].fields 至少需要一个字段`);
          }
          // 检测字段 key 重复
          if (Array.isArray(tab.layout.fields)) {
            const fieldKeys = new Set<string>();
            tab.layout.fields.forEach((f: any, fi: number) => {
              if (f.key) {
                if (fieldKeys.has(f.key)) {
                  errors.push(`tabs[${index}].fields[${fi}].key 重复: "${f.key}"`);
                }
                fieldKeys.add(f.key);
              }
            });
          }
        }

        if (tab.layout?.type === 'detail') {
          if (!tab.layout.detailFunction) {
            errors.push(`tabs[${index}].detailFunction 不能为空`);
          }
        }

        if (tab.layout?.type === 'form-detail') {
          if (!tab.layout.queryFunction) {
            errors.push(`tabs[${index}].queryFunction 不能为空`);
          }
          if (tab.layout.queryFields !== undefined && !Array.isArray(tab.layout.queryFields)) {
            errors.push(`tabs[${index}].queryFields 必须为数组`);
          }
        }

        if (tab.layout?.type === 'split') {
          if (
            !Array.isArray((tab.layout as any).panels) ||
            (tab.layout as any).panels.length === 0
          ) {
            errors.push(`tabs[${index}].panels 至少需要一个面板`);
          }
        }
        if (tab.layout?.type === 'wizard') {
          if (!Array.isArray((tab.layout as any).steps) || (tab.layout as any).steps.length === 0) {
            errors.push(`tabs[${index}].steps 至少需要一个步骤`);
          }
        }
        if (tab.layout?.type === 'dashboard') {
          const hasStats =
            Array.isArray((tab.layout as any).stats) && (tab.layout as any).stats.length > 0;
          const hasPanels =
            Array.isArray((tab.layout as any).panels) && (tab.layout as any).panels.length > 0;
          if (!hasStats && !hasPanels) {
            errors.push(`tabs[${index}] dashboard 至少需要 stats 或 panels`);
          }
        }
        if (tab.layout?.type === 'grid') {
          if (!Array.isArray((tab.layout as any).items) || (tab.layout as any).items.length === 0) {
            errors.push(`tabs[${index}].items 至少需要一个网格项`);
          }
        }
        if (tab.layout?.type === 'custom') {
          if (!(tab.layout as any).component) {
            errors.push(`tabs[${index}].component 不能为空`);
          }
        }

        // 历史兼容：single 是旧版自动生成结构，允许保存以便逐步迁移。
        // 该结构一般形如 { type: 'single', component: { type: 'function', functionId } }。
        const legacyLayoutType = (tab.layout as any)?.type;
        if (legacyLayoutType === 'single') {
          const functionId = (tab.layout as any)?.component?.functionId;
          if (functionId && !Array.isArray(tab.functions)) {
            errors.push(`tabs[${index}].functions 必须为数组`);
          }
        }
      });
    }
  }

  return {
    valid: errors.length === 0,
    errors,
  };
}

/**
 * 克隆 Workspace 配置
 *
 * @param sourceKey - 源对象标识
 * @param targetKey - 目标对象标识
 * @param targetTitle - 目标标题
 * @returns 克隆后的配置
 */
export async function cloneWorkspaceConfig(
  sourceKey: string,
  targetKey: string,
  targetTitle: string,
): Promise<WorkspaceConfig> {
  // 加载源配置
  const sourceConfig = await loadWorkspaceConfig(sourceKey);
  if (!sourceConfig) {
    throw new Error(`配置不存在: ${sourceKey}`);
  }

  // 创建新配置
  const newConfig: WorkspaceConfig = {
    ...sourceConfig,
    objectKey: targetKey,
    title: targetTitle,
    meta: {
      ...sourceConfig.meta,
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    },
  };

  // 保存新配置
  return saveWorkspaceConfig(newConfig);
}

/**
 * 导出 Workspace 配置
 *
 * @param objectKey - 对象标识
 * @returns 配置 JSON 字符串
 */
export async function exportWorkspaceConfig(objectKey: string): Promise<string> {
  const config = await loadWorkspaceConfig(objectKey);
  if (!config) {
    throw new Error(`配置不存在: ${objectKey}`);
  }

  return JSON.stringify(config, null, 2);
}

/**
 * 导出当前已发布版本配置
 *
 * @param objectKey - 对象标识
 * @returns 已发布版本配置 JSON 字符串
 */
export async function exportPublishedWorkspaceConfig(objectKey: string): Promise<string> {
  const versions = await listWorkspaceVersions(objectKey);
  const currentPublished =
    versions.find((item) => item.isCurrentPublished) ||
    versions.find((item) => item.config?.published || item.config?.status === 'published');

  if (!currentPublished?.config) {
    throw new Error(`对象 ${objectKey} 不存在可导出的已发布版本`);
  }

  return JSON.stringify(currentPublished.config, null, 2);
}

/**
 * 导出 Workspace 元信息
 *
 * @param objectKey - 对象标识
 * @returns 元信息 JSON 字符串
 */
export async function exportWorkspaceMetadata(objectKey: string): Promise<string> {
  const config = await loadWorkspaceConfig(objectKey, { forceRefresh: true, useCache: false });
  if (!config) {
    throw new Error(`配置不存在: ${objectKey}`);
  }

  let versions: WorkspaceVersionRecord[] = [];
  try {
    versions = await listWorkspaceVersions(objectKey);
  } catch (error) {
    // 版本接口不可用时降级导出基础元信息
    versions = [];
  }

  const currentDraftVersion = versions.find((item) => item.isCurrentDraft)?.version;
  const currentPublishedVersion = versions.find((item) => item.isCurrentPublished)?.version;
  const status = config.status || (config.published ? 'published' : 'draft');
  const metadata: WorkspaceExportMetadata = {
    objectKey: config.objectKey,
    title: config.title,
    description: config.description,
    status,
    version: config.version,
    published: Boolean(config.published),
    publishedAt: config.publishedAt,
    publishedBy: config.publishedBy,
    createdAt: config.meta?.createdAt,
    updatedAt: config.meta?.updatedAt,
    currentDraftVersion,
    currentPublishedVersion,
    versionCount: versions.length,
    exportedAt: new Date().toISOString(),
  };

  return JSON.stringify(metadata, null, 2);
}

/**
 * 导出 Workspace 备份包（草稿/发布版/历史版本/元信息）
 *
 * @param objectKey - 对象标识
 * @returns 备份包 JSON 字符串
 */
export async function exportWorkspaceBackupBundle(objectKey: string): Promise<string> {
  const config = await loadWorkspaceConfig(objectKey, { forceRefresh: true, useCache: false });
  const versions = await listWorkspaceVersions(objectKey).catch(() => []);
  const currentPublished =
    versions.find((item) => item.isCurrentPublished)?.config ||
    versions.find((item) => item.config?.published || item.config?.status === 'published')
      ?.config ||
    null;
  const metadata: WorkspaceExportMetadata = {
    objectKey,
    title: config?.title,
    description: config?.description,
    status: config?.status || (config?.published ? 'published' : 'draft'),
    version: config?.version,
    published: Boolean(config?.published),
    publishedAt: config?.publishedAt,
    publishedBy: config?.publishedBy,
    createdAt: config?.meta?.createdAt,
    updatedAt: config?.meta?.updatedAt,
    currentDraftVersion: versions.find((item) => item.isCurrentDraft)?.version,
    currentPublishedVersion: versions.find((item) => item.isCurrentPublished)?.version,
    versionCount: versions.length,
    exportedAt: new Date().toISOString(),
  };
  const bundle: WorkspaceBackupBundle = {
    objectKey,
    exportedAt: new Date().toISOString(),
    metadata,
    currentDraft: config,
    currentPublished,
    versions,
  };
  return JSON.stringify(bundle, null, 2);
}

/**
 * 导入 Workspace 配置
 *
 * @param configJson - 配置 JSON 字符串
 * @returns 导入后的配置
 */
export async function importWorkspaceConfig(
  configJson: string,
  options?: ImportWorkspaceConfigOptions,
): Promise<WorkspaceConfig> {
  try {
    const config = JSON.parse(configJson) as WorkspaceConfig;
    const normalizedConfig: WorkspaceConfig = {
      ...config,
      objectKey: options?.targetObjectKey || config.objectKey,
    };

    if (options?.forceDraft) {
      normalizedConfig.published = false;
      normalizedConfig.status = 'draft';
      normalizedConfig.publishedAt = undefined;
      normalizedConfig.publishedBy = undefined;
    }

    normalizedConfig.meta = {
      ...normalizedConfig.meta,
      updatedAt: new Date().toISOString(),
    };

    // 验证配置
    const validation = validateWorkspaceConfig(normalizedConfig);
    if (!validation.valid) {
      throw new Error(`配置验证失败: ${validation.errors.join(', ')}`);
    }

    // 保存配置
    return saveWorkspaceConfig(normalizedConfig);
  } catch (error: any) {
    if (error instanceof SyntaxError) {
      throw new Error('配置 JSON 格式错误');
    }
    throw error;
  }
}

/**
 * 清除所有缓存
 */
export function clearAllCache(): void {
  clearCache();
}

/**
 * 获取缓存统计信息
 */
export function getCacheStats(): {
  size: number;
  keys: string[];
} {
  return {
    size: configCache.size,
    keys: Array.from(configCache.keys()),
  };
}

/**
 * 预加载配置
 *
 * 在后台预加载配置，提升用户体验。
 *
 * @param objectKeys - 对象标识列表
 */
export async function preloadConfigs(objectKeys: string[]): Promise<void> {
  const promises = objectKeys.map((key) => loadWorkspaceConfig(key).catch(() => null));
  await Promise.all(promises);
}

/**
 * 批量加载配置
 *
 * @param objectKeys - 对象标识列表
 * @returns 配置映射
 */
export async function batchLoadConfigs(
  objectKeys: string[],
): Promise<Map<string, WorkspaceConfig | null>> {
  const results = await Promise.all(
    objectKeys.map(async (key) => {
      const config = await loadWorkspaceConfig(key).catch(() => null);
      return [key, config] as [string, WorkspaceConfig | null];
    }),
  );

  return new Map(results);
}

/**
 * 获取配置版本列表（为版本面板预留）
 */
export async function listWorkspaceVersions(
  objectKey: string,
  params?: ListWorkspaceVersionsParams,
): Promise<WorkspaceVersionRecord[]> {
  const response = await request<{ items: WorkspaceVersionRecord[] }>(
    `/api/v1/workspaces/${objectKey}/versions`,
    {
      method: 'GET',
      params,
    },
  );
  return Array.isArray(response?.items) ? response.items : [];
}

/**
 * 获取指定版本详情（后端若未启用该接口，调用方可自行降级）
 */
export async function getWorkspaceVersionDetail(
  objectKey: string,
  versionId: string,
): Promise<WorkspaceVersionRecord> {
  return request<WorkspaceVersionRecord>(`/api/v1/workspaces/${objectKey}/versions/${versionId}`, {
    method: 'GET',
  });
}

/**
 * 回滚到指定版本（为版本面板预留）
 */
export async function rollbackWorkspaceVersion(
  objectKey: string,
  versionId: string,
): Promise<WorkspaceRollbackResult> {
  const response = await request<WorkspaceRollbackResult>(
    `/api/v1/workspaces/${objectKey}/rollback`,
    {
      method: 'POST',
      data: {
        versionId,
      },
    },
  );
  clearCache(objectKey);
  return response;
}
