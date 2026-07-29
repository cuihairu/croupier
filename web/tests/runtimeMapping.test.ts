import {
  applyInputMapping,
  buildBindingPayload,
  normalizePath,
  readPath,
} from '@/components/FormilyPageRenderer/runtimeMapping';
import {
  parseOptionalJSONObject,
  toJSONValue,
} from '@/utils/dashboardJson';
import type { PageFunctionBinding } from '@/types/dashboard';

const queryBinding: PageFunctionBinding = {
  id: 'player.query',
  functionId: 'player.list',
  usage: 'query',
  inputMapping: {
    page: 'values.pagination.current',
    pageSize: 'values.pagination.pageSize',
  },
  outputMapping: { stateKey: 'players' },
  execution: { mode: 'sync' },
};

describe('runtimeMapping', () => {
  it('normalizes dotted and jsonpath-like paths', () => {
    expect(normalizePath('$.data.items')).toEqual(['data', 'items']);
    expect(normalizePath(' row.id ')).toEqual(['row', 'id']);
  });

  it('reads nested values by explicit path only', () => {
    const source = { data: { items: [{ id: 'p1' }] } };
    expect(readPath(source, '$.data.items')).toEqual([{ id: 'p1' }]);
    expect(readPath(source, '$.missing.items')).toBeUndefined();
  });

  it('maps DataTable pagination through binding inputMapping', () => {
    const payload = buildBindingPayload(queryBinding, undefined, {
      values: { pagination: { current: 2, pageSize: 50 } },
    });

    expect(payload).toEqual({ page: 2, pageSize: 50 });
  });

  it('normalizes missing mapped values to JSON null', () => {
    const payload = buildBindingPayload(queryBinding, undefined, {
      values: { pagination: { current: 2 } },
    });

    expect(payload).toEqual({ page: 2, pageSize: null });
  });

  it('allows component-level action mapping to use row context', () => {
    const actionBinding: PageFunctionBinding = {
      id: 'player.ban',
      functionId: 'player.ban',
      usage: 'action',
      inputMapping: { fallback: 'values.playerId' },
      outputMapping: { stateKey: 'banResult' },
      execution: { mode: 'sync' },
    };

    const payload = buildBindingPayload(actionBinding, { playerId: 'row.id' }, {
      row: { id: 'p1' },
    });

    expect(payload).toEqual({ playerId: 'p1' });
  });

  it('rejects non-object mappings', () => {
    expect(() => applyInputMapping('values' as never, { values: {} })).toThrow('inputMapping 必须是对象');
    expect(() => parseOptionalJSONObject('["values.id"]', 'inputMapping')).toThrow('inputMapping 必须是 JSON object');
  });

  it('normalizes undefined and object values through the shared JSON boundary', () => {
    expect(toJSONValue(undefined)).toBeNull();
    expect(parseOptionalJSONObject('{"playerId":"row.id"}', 'inputMapping')).toEqual({
      playerId: 'row.id',
    });
  });
});
