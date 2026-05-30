import { request } from '@umijs/max';

// Source: croupier/internal/api/registry/dto.go RegistryAgent
export type RegistryAgent = {
  agentId: string;
  gameId: string;
  env: string;
  rpcAddr: string;
  functions: number;
  healthy: boolean;
  expiresInSec: number;
};

// Source: croupier/internal/api/registry/dto.go RegistryFunction
export type RegistryFunction = {
  gameId: string;
  id: string;
  agents: string[];
};

// Source: croupier/internal/api/registry/dto.go RegistryCoverageStat
export type RegistryCoverageStat = {
  total: number;
  healthy: number;
};

// Source: croupier/internal/api/registry/dto.go RegistryCoverage
export type RegistryCoverage = {
  gameEnv: string;
  functions: Record<string, RegistryCoverageStat>;
  uncovered: string[];
};

// Source: croupier/internal/api/registry/dto.go RegistryResponse
export type RegistryResponse = {
  agents: RegistryAgent[];
  functions: RegistryFunction[];
  assignments?: Record<string, string[]>;
  coverage?: RegistryCoverage[];
};

export async function fetchRegistry(): Promise<RegistryResponse> {
  return request<RegistryResponse>('/api/v1/registry');
}
