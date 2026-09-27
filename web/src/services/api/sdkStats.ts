import { request } from '@umijs/max';
import type { JSONValue } from '@/types/dashboard';

/** F：SDK 版本分布——单语言聚合 */
export type SdkVersionCount = {
  version: string;
  count: number;
};

export type SdkLanguageStats = {
  language: string;
  count: number;
  versions: SdkVersionCount[];
};

export type SdkInstanceItem = {
  providerId: string;
  agentId: string;
  gameId: string;
  env: string;
  serviceAddr?: string;
  sdkName?: string;
  sdkLanguage: string;
  sdkVersion: string;
  /** provider 自报的用户实例元数据（serverId 等多 KV；保留键已在 agent 侧剥离） */
  metadata?: Record<string, string>;
  lastSeenUnix: number;
};

export type SdkStatsResponse = {
  totalInstances: number;
  languages: SdkLanguageStats[];
  instances: SdkInstanceItem[];
};

/**
 * GET /api/v1/providers/sdk-stats：在线 provider 的 SDK 语言/版本分布。
 * metaKey/metaValue 对实例元数据做服务端子串过滤（大小写不敏感；value
 * 条件同时匹配键名，方便直接粘值搜索）。
 */
export async function fetchSdkStats(params?: {
  metaKey?: string;
  metaValue?: string;
}): Promise<SdkStatsResponse> {
  return request<SdkStatsResponse>('/api/v1/providers/sdk-stats', { params });
}

/** 元数据键的单值聚合（实例出现次数），#11 EAV 表聚合产物 */
export type ProviderMetaValueOption = {
  value: string;
  count: number;
};

/** 元数据键聚合项：键名 + 该键下出现过的值集合（count 降序由服务端保证） */
export type ProviderMetaKeyOption = {
  key: string;
  values: ProviderMetaValueOption[];
};

/**
 * GET /api/v1/providers/meta-options：实例元数据键→值聚合（#2 过滤下拉
 * 选项源；30s TTL 缓存，scoped）。
 */
export async function fetchProviderMetaOptions(): Promise<ProviderMetaKeyOption[]> {
  const res = await request<{ items?: ProviderMetaKeyOption[] }>('/api/v1/providers/meta-options');
  return Array.isArray(res?.items) ? res.items : [];
}

export type { JSONValue };
