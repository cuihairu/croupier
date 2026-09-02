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
  lastSeenUnix: number;
};

export type SdkStatsResponse = {
  totalInstances: number;
  languages: SdkLanguageStats[];
  instances: SdkInstanceItem[];
};

/** GET /api/v1/providers/sdk-stats：在线 provider 的 SDK 语言/版本分布 */
export async function fetchSdkStats(): Promise<SdkStatsResponse> {
  return request<SdkStatsResponse>('/api/v1/providers/sdk-stats');
}

export type { JSONValue };
