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

export type { JSONValue };
