import { applyTransform } from '@/plugin/transform';

describe('applyTransform', () => {
  const sample = {
    a: { b: 1 },
    data: {
      result: [
        { metric: { instance: 'x' }, values: [[1, '2'], [2, '3']] },
        { metric: { instance: 'y' }, values: [[1, '5']] },
      ],
    },
  };

  it('extracts by expr path', () => {
    expect(applyTransform(sample, { expr: '$.a.b' })).toBe(1);
  });

  it('maps via template forEach', () => {
    const out = applyTransform(sample, { template: { forEach: { path: '$.data.result', template: { name: '$.metric.instance', data: '$.values' } } } });
    expect(Array.isArray(out)).toBe(true);
    expect(out[0]).toEqual({ name: 'x', data: [[1,'2'],[2,'3']] });
  });

  it('filters via where contains', () => {
    const out = applyTransform(sample, { template: { forEach: { path: '$.data.result', where: { contains: ['$.metric.instance', 'y'] }, template: { name: '$.metric.instance' } } } });
    expect(out).toEqual([{ name: 'y' }]);
  });

  it('map/pluck/sum/avg work', () => {
    const base = { arr: [ { v: '1' }, { v: '2' }, { v: '3' } ] };
    expect(applyTransform(base, { template: { map: { path: '$.arr', template: { v: '$.v' } } } })).toEqual([{ v: '1' }, { v: '2' }, { v: '3' }]);
    expect(applyTransform(base, { template: { pluck: { path: '$.arr', value: '$.v' } } })).toEqual(['1','2','3']);
    expect(applyTransform(base, { template: { sum: { path: '$.arr', value: '$.v' } } })).toEqual(6);
    expect(applyTransform(base, { template: { avg: { path: '$.arr', value: '$.v' } } })).toEqual(2);
  });

  it('bracket indexing works', () => {
    const o = { value: [ 1700000000, '3.14' ] };
    const num = applyTransform(o, { template: { number: '$.value[1]' } });
    const ms = applyTransform(o, { template: { msFromSec: '$.value[0]' } });
    expect(num).toBeCloseTo(3.14);
    expect(ms).toBe(1700000000 * 1000);
  });

  it('toFixed and isoFrom* work', () => {
    const o = { tsMs: 1700000000000, tsSec: 1700000000, v: 3.14159 };
    const fixed = applyTransform(o, { template: { toFixed: { value: '$.v', digits: 2 } } });
    const isoMs = applyTransform(o, { template: { isoFromMs: '$.tsMs' } }) as string;
    const isoSec = applyTransform(o, { template: { isoFromSec: '$.tsSec' } }) as string;
    expect(fixed).toBe(3.14);
    expect(typeof isoMs).toBe('string');
    expect(typeof isoSec).toBe('string');
    expect(isoMs).toContain('T');
    expect(isoSec).toContain('T');
  });
});
