import { addrHost, normalizeOpsNode } from './shared';
import type { OpsNode } from '@/services/api/ops';

describe('addrHost', () => {
  const cases: Array<[string | undefined, string]> = [
    ['192.168.1.10:19091', '192.168.1.10'],
    ['host.example.com:8080', 'host.example.com'],
    ['localhost', 'localhost'],
    ['', ''],
    [undefined, ''],
    ['10.0.0.1:9999:extra', '10.0.0.1:9999'],
  ];
  it.each(cases)('addrHost(%p) = %p', (input, expected) => {
    expect(addrHost(input)).toBe(expected);
  });
});

describe('normalizeOpsNode', () => {
  const baseNode = (overrides: Partial<OpsNode> = {}): OpsNode => ({
    id: 'agent-1',
    addr: '192.168.1.10:19091',
    gameId: 'demo',
    env: 'prod',
    status: 'active',
    functions: 5,
    expiresInSec: 3600,
    sdkName: 'go-sdk',
    sdkLanguage: 'go',
    sdkVersion: '1.2.3',
    lastSeen: '2026-09-28T00:00:00Z',
    labels: { hostname: 'agent-host', ip: '10.0.0.5' },
    cpu: { usagePercent: 10, cores: 4, load1m: 0.5, load5m: 0.4, load15m: 0.3 },
    memory: {
      totalBytes: 8_000_000_000,
      usedBytes: 4_000_000_000,
      availableBytes: 4_000_000_000,
      usagePercent: 50,
      swapTotal: 0,
      swapUsed: 0,
    },
    disks: [
      {
        mountPoint: '/',
        device: '/dev/sda1',
        fsType: 'ext4',
        totalBytes: 100_000_000_000,
        usedBytes: 30_000_000_000,
        availableBytes: 70_000_000_000,
        usagePercent: 30,
      },
    ],
    ...overrides,
  });

  it('full node: all fields mapped, healthy=true', () => {
    const row = normalizeOpsNode(baseNode());
    expect(row.agentId).toBe('agent-1');
    expect(row.type).toBe('agent');
    expect(row.gameId).toBe('demo');
    expect(row.env).toBe('prod');
    expect(row.addr).toBe('192.168.1.10:19091');
    expect(row.ip).toBe('192.168.1.10');
    expect(row.hostname).toBe('agent-host');
    expect(row.reportedIp).toBe('10.0.0.5');
    expect(row.functions).toBe(5);
    expect(row.healthy).toBe(true);
    expect(row.expiresInSec).toBe(3600);
    expect(row.sdkName).toBe('go-sdk');
    expect(row.sdkLanguage).toBe('go');
    expect(row.sdkVersion).toBe('1.2.3');
    expect(row.version).toBe('1.2.3');
    expect(row.lastSeen).toBe('2026-09-28T00:00:00Z');
    expect(row.nodeStatus).toBe('active');
    expect(row.labels).toEqual({ hostname: 'agent-host', ip: '10.0.0.5' });
    expect(row.cpu).toEqual(baseNode().cpu);
    expect(row.memory).toEqual(baseNode().memory);
    expect(row.disks).toEqual(baseNode().disks);
  });

  it('status variations: healthy / offline / unknown', () => {
    expect(normalizeOpsNode(baseNode({ status: 'healthy' })).healthy).toBe(true);
    expect(normalizeOpsNode(baseNode({ status: 'online' })).healthy).toBe(true);
    expect(normalizeOpsNode(baseNode({ status: 'offline' })).healthy).toBe(false);
    expect(normalizeOpsNode(baseNode({ status: 'unknown' })).healthy).toBe(false);
    expect(normalizeOpsNode(baseNode({ status: undefined })).healthy).toBe(false);
  });

  it('missing optional fields fall back to empty/zero', () => {
    const row = normalizeOpsNode({} as OpsNode);
    expect(row.agentId).toBe('');
    expect(row.gameId).toBe('');
    expect(row.env).toBe('');
    expect(row.addr).toBe('');
    expect(row.ip).toBe('');
    expect(row.hostname).toBe('');
    expect(row.reportedIp).toBe('');
    expect(row.functions).toBe(0);
    expect(row.healthy).toBe(false);
    expect(row.expiresInSec).toBe(0);
    expect(row.sdkName).toBe('');
    expect(row.sdkLanguage).toBe('');
    expect(row.sdkVersion).toBe('');
    expect(row.version).toBe('');
    expect(row.lastSeen).toBe('');
    expect(row.nodeStatus).toBe('active');
    expect(row.labels).toEqual({});
    expect(row.cpu).toBeUndefined();
    expect(row.memory).toBeUndefined();
    expect(row.disks).toBeUndefined();
  });

  it('labels missing hostname/ip -> empty string', () => {
    const row = normalizeOpsNode(baseNode({ labels: {} }));
    expect(row.hostname).toBe('');
    expect(row.reportedIp).toBe('');
  });

  it('labels hostname/ip non-string -> empty string', () => {
    // 故意喂非字符串运行时值（触发 typeof 守卫），经 unknown 双断言满足 labels 类型
    const row = normalizeOpsNode(
      baseNode({ labels: { hostname: 123 as unknown as string, ip: null as unknown as string } }),
    );
    expect(row.hostname).toBe('');
    expect(row.reportedIp).toBe('');
  });

  it('id missing falls back to addr', () => {
    const row = normalizeOpsNode(baseNode({ id: undefined, addr: '10.0.0.1:9999' }));
    expect(row.agentId).toBe('10.0.0.1:9999');
  });

  it('id and addr both missing -> empty agentId', () => {
    const row = normalizeOpsNode(baseNode({ id: undefined, addr: undefined }));
    expect(row.agentId).toBe('');
  });

  it('version falls back to sdkVersion', () => {
    const row = normalizeOpsNode(baseNode({ sdkVersion: '2.0.0' }));
    expect(row.version).toBe('2.0.0');
  });

  it('system metrics optional: undefined passes through', () => {
    const row = normalizeOpsNode(baseNode({ cpu: undefined, memory: undefined, disks: undefined }));
    expect(row.cpu).toBeUndefined();
    expect(row.memory).toBeUndefined();
    expect(row.disks).toBeUndefined();
  });
});
