/**
 * Ops/Nodes shared 纯函数表驱动单测（覆盖率巡检：branch 13.3% → 全覆盖）。
 *
 * addrHost 的 ':' 切分边界与 normalizeOpsNode 的缺省归一分支逐一列表；
 * 既有 index.test.tsx 只经页面渲染间接触达 happy path，分支几乎全未走。
 */
import type { OpsNode } from '@/services/api/ops';
import { addrHost, normalizeOpsNode } from '../shared';

const node = (over: Partial<OpsNode>): OpsNode => ({ id: 'agent-1', ...over });

describe('addrHost', () => {
  it.each<[string | undefined, string]>([
    ['10.0.0.1:19091', '10.0.0.1'],
    ['[::1]:19091', '[::1]'],
    ['host:', 'host'],
    ['noport', 'noport'],
    // lastIndexOf=0 不满足 idx>0：原样返回（现状行为，防御 ':port' 畸形 addr）
    [':19091', ':19091'],
    ['', ''],
    [undefined, ''],
  ])('addrHost(%p) → %p', (input, expected) => {
    expect(addrHost(input)).toBe(expected);
  });
});

describe('normalizeOpsNode', () => {
  it('空壳 node（仅 id）全字段缺省归一', () => {
    expect(normalizeOpsNode(node({}))).toMatchObject({
      agentId: 'agent-1',
      type: 'agent',
      gameId: '',
      env: '',
      addr: '',
      ip: '',
      hostname: '',
      reportedIp: '',
      functions: 0,
      healthy: false,
      expiresInSec: 0,
      sdkName: '',
      sdkLanguage: '',
      sdkVersion: '',
      version: '',
      lastSeen: '',
      nodeStatus: 'active',
      labels: {},
    });
  });

  it('id 缺失时 agentId 回落 addr，addr/全空两级兜底；labels 自报 hostname/ip 透出', () => {
    expect(
      normalizeOpsNode(
        node({ id: '', addr: '10.0.0.9:19091', labels: { hostname: 'node-9', ip: '10.0.0.9' } }),
      ),
    ).toMatchObject({
      agentId: '10.0.0.9:19091',
      ip: '10.0.0.9',
      hostname: 'node-9',
      reportedIp: '10.0.0.9',
    });
    // id/addr 全空：最后一级 '' 兜底
    expect(normalizeOpsNode(node({ id: '', addr: '' })).agentId).toBe('');
  });

  it.each(['active', 'healthy', 'online'])('status=%s → healthy=true', (status) => {
    expect(normalizeOpsNode(node({ status })).healthy).toBe(true);
  });

  it.each(['expired', 'offline', ''])(
    'status=%s → healthy=false，nodeStatus 空值归 active',
    (status) => {
      const row = normalizeOpsNode(node({ status }));
      expect(row.healthy).toBe(false);
      expect(row.nodeStatus).toBe(status === '' ? 'active' : status);
    },
  );

  it('labels 非 string 值（wire 层松散数据）按缺失归一空串，不抛错', () => {
    expect(
      normalizeOpsNode(
        node({ labels: { hostname: 9, ip: 10 } as unknown as Record<string, string> }),
      ),
    ).toMatchObject({ hostname: '', reportedIp: '' });
  });

  it('数值与 sdk 字段映射：functions/expiresInSec 透传，version 同步 sdkVersion', () => {
    expect(
      normalizeOpsNode(node({ sdkVersion: '2.3.4', functions: 7, expiresInSec: 42 })),
    ).toMatchObject({ functions: 7, expiresInSec: 42, sdkVersion: '2.3.4', version: '2.3.4' });
  });

  it('metrics（cpu/memory/disks）原样透传不拷贝', () => {
    const cpu = { usagePercent: 12.5, cores: 8, load1m: 1, load5m: 2, load15m: 3 };
    expect(normalizeOpsNode(node({ cpu })).cpu).toBe(cpu);
  });
});
