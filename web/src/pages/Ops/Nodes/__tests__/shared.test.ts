/**
 * Ops/Nodes shared 纯函数补测（覆盖率巡检：语句 100% 但分支 13.33%）：
 * addrHost 三处分支（缺省/无分隔符/前导冒号）+ normalizeOpsNode 全部兜底
 * 分支（缺省字段、labels 非字符串、健康态集合边界）。均为可达分支。
 */
import { addrHost, normalizeOpsNode } from '../shared';
import type { OpsNode } from '@/services/api/ops';

describe('addrHost', () => {
  it('extracts the host part from host:port', () => {
    expect(addrHost('10.0.0.1:7001')).toBe('10.0.0.1');
    expect(addrHost('[::1]:9090')).toBe('[::1]');
  });

  it('returns empty for missing addr', () => {
    expect(addrHost(undefined)).toBe('');
    expect(addrHost('')).toBe('');
  });

  it('returns the addr whole when there is no port separator', () => {
    expect(addrHost('plain-host')).toBe('plain-host');
    // 前导冒号 lastIndexOf=0 不视为 host:port 分隔
    expect(addrHost(':8080')).toBe(':8080');
  });
});

describe('normalizeOpsNode', () => {
  it('maps a fully-populated node', () => {
    const node: OpsNode = {
      id: 'a1',
      addr: '10.0.0.1:7001',
      gameId: 'demo',
      env: 'prod',
      status: 'healthy',
      functions: 3,
      expiresInSec: 42,
      sdkName: 'croupier-go',
      sdkLanguage: 'go',
      sdkVersion: '1.2.3',
      lastSeen: '2026-09-28T00:00:00Z',
      labels: { hostname: 'h1', ip: '10.0.0.9' },
    };

    expect(normalizeOpsNode(node)).toMatchObject({
      agentId: 'a1',
      type: 'agent',
      ip: '10.0.0.1',
      hostname: 'h1',
      reportedIp: '10.0.0.9',
      healthy: true,
      functions: 3,
      expiresInSec: 42,
      sdkName: 'croupier-go',
      version: '1.2.3',
      nodeStatus: 'healthy',
      labels: { hostname: 'h1', ip: '10.0.0.9' },
    });
  });

  it('falls back for absent fields (id 从 addr 兜底、健康态集合边界)', () => {
    const row = normalizeOpsNode({ id: '', addr: '10.0.0.2:7002', status: 'draining' });
    expect(row.agentId).toBe('10.0.0.2:7002');
    expect(row.gameId).toBe('');
    expect(row.env).toBe('');
    expect(row.healthy).toBe(false);
    expect(row.functions).toBe(0);
    expect(row.expiresInSec).toBe(0);
    expect(row.sdkName).toBe('');
    expect(row.version).toBe('');
    expect(row.lastSeen).toBe('');
    expect(row.labels).toEqual({});
  });

  it('labels 非字符串值不透传、status 缺省兜底 active、id 缺省取 addr', () => {
    const row = normalizeOpsNode({
      id: 'a2',
      labels: { hostname: 42, ip: null } as unknown as Record<string, string>,
    });
    expect(row.hostname).toBe('');
    expect(row.reportedIp).toBe('');
    expect(row.healthy).toBe(false);
    expect(row.nodeStatus).toBe('active');
    expect(row.addr).toBe('');
  });

  it('healthy 覆盖 active/healthy/online 三态', () => {
    for (const status of ['active', 'healthy', 'online']) {
      expect(normalizeOpsNode({ id: 'n', status }).healthy).toBe(true);
    }
  });
});
