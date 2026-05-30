import { request } from '@umijs/max';

const BASE = '/api/v1/platforms';

export interface PlatformInfo {
  name: string;
  version?: string;
  enabled: boolean;
  methods: string[];
  source?: 'extension' | string;
}

export interface CallPlatformRequest {
  platform: string;
  method: string;
  request: string;
}

export interface CallPlatformResponse {
  response?: any;
  source?: 'extension' | string;
}

// 获取所有平台列表
export async function listPlatforms() {
  return request<{ platforms: PlatformInfo[] }>(BASE);
}

// 获取指定平台支持的方法列表
export async function listPlatformMethods(platformName: string) {
  return request<{ methods: string[]; source?: string }>(`${BASE}/${platformName}/methods`);
}

// 调用第三方平台 API
export async function callPlatform(data: CallPlatformRequest) {
  return request<CallPlatformResponse>(`${BASE}/call`, {
    method: 'POST',
    data,
  });
}
