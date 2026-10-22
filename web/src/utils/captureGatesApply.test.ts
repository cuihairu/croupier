import { summarizeGatesApply } from './captureGatesApply';
import type { OpsNode } from '@/services/api/ops';

const node = (id: string, revision?: string): OpsNode =>
  ({
    id,
    labels: revision ? { role: 'capture', captureGatesRevision: revision } : { role: 'capture' },
  }) as OpsNode;

describe('summarizeGatesApply（生效对账汇总，Capture/风控策略页共用）', () => {
  it('desired=0（未发布）返回 null', () => {
    expect(summarizeGatesApply([node('cap-1', '3')], 0)).toBeNull();
  });

  it('仅 role=capture 节点进对账，applied/stale 按自报版本分列', () => {
    const nodes = [
      node('cap-1', '3'),
      node('cap-2', '1'),
      node('cap-3'),
      { id: 'srv-1', labels: { role: 'server', captureGatesRevision: '3' } } as unknown as OpsNode,
      { id: 'bare' } as OpsNode,
    ];
    const summary = summarizeGatesApply(nodes, 3);
    expect(summary).not.toBeNull();
    expect(summary?.desired).toBe(3);
    expect(summary?.total).toBe(3);
    expect(summary?.applied.map((n) => n.id)).toEqual(['cap-1']);
    expect(summary?.stale.map((n) => n.id)).toEqual(['cap-2', 'cap-3']);
  });

  it('无 capture 节点：total=0、两列皆空（面板渲染「scope 内无在线」灰 tag）', () => {
    const summary = summarizeGatesApply(
      [{ id: 'srv-1', labels: { role: 'server' } } as OpsNode],
      5,
    );
    expect(summary).toEqual({ desired: 5, total: 0, applied: [], stale: [] });
  });
});
