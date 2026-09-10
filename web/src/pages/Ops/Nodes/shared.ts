import type { OpsNode } from '@/services/api/ops';
import type { RegistryAgent } from '@/services/api/registry';

export type NodeRow = RegistryAgent & {
  type?: string;
  ip?: string;
  version?: string;
  sdkName?: string;
  sdkLanguage?: string;
  sdkVersion?: string;
  lastSeen?: string;
  nodeStatus?: string;
  labels?: Record<string, string>;
  // System metrics (from detail)
  cpu?: {
    usagePercent: number;
    cores: number;
    perCore?: number[];
    load1m: number;
    load5m: number;
    load15m: number;
  };
  memory?: {
    totalBytes: number;
    usedBytes: number;
    availableBytes: number;
    usagePercent: number;
    swapTotal: number;
    swapUsed: number;
  };
  disks?: Array<{
    mountPoint: string;
    device: string;
    fsType: string;
    totalBytes: number;
    usedBytes: number;
    availableBytes: number;
    usagePercent: number;
    inodeTotal?: number;
    inodeUsed?: number;
  }>;
};

// 从 "host:port" 提取 host 部分作为 IP 展示。
export function addrHost(addr?: string): string {
  if (!addr) return '';
  const idx = addr.lastIndexOf(':');
  return idx > 0 ? addr.slice(0, idx) : addr;
}

export function normalizeOpsNode(node: OpsNode): NodeRow {
  return {
    agentId: node.id || node.addr || '',
    type: 'agent',
    gameId: node.gameId || '',
    env: node.env || '',
    addr: node.addr || '',
    ip: addrHost(node.addr),
    functions: node.functions || 0,
    healthy: ['active', 'healthy', 'online'].includes(node.status || ''),
    expiresInSec: node.expiresInSec || 0,
    sdkName: node.sdkName || '',
    sdkLanguage: node.sdkLanguage || '',
    sdkVersion: node.sdkVersion || '',
    version: node.sdkVersion || '',
    lastSeen: node.lastSeen || '',
    nodeStatus: node.status || 'active',
    labels: node.labels || {},
    // System metrics
    cpu: node.cpu,
    memory: node.memory,
    disks: node.disks,
  };
}
