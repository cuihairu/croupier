/** V5 T5.3：resolveStepParams（表达式求值器薄封装）+ 运行时状态形态测试。 */
import { resolveStepParams } from '../index';

const state = {
  playerListTable: {
    data: { items: [{ uid: 'u1', nickname: 'bob' }, { uid: 'u2' }], total: 2 },
    selectedRow: { uid: 'u2', nickname: 'alice' },
    selectedRows: [{ uid: 'u2', nickname: 'alice' }],
  },
  filterForm: { data: { keyword: 'kw' }, values: { keyword: 'kw-live' } },
  mailForm: { data: { ok: true } },
};

describe('resolveStepParams（V5 表达式 + 遗留兼容）', () => {
  it('{{var.selectedRow.字段}} 从运行时选中行取值', () => {
    expect(
      resolveStepParams({ nickname: '{{playerListTable.selectedRow.nickname}}' }, state),
    ).toEqual({ nickname: 'alice' });
  });

  it('{{var.values.字段}} 从表单当前值取值', () => {
    expect(resolveStepParams({ keyword: '{{filterForm.values.keyword}}' }, state)).toEqual({
      keyword: 'kw-live',
    });
  });

  it('{{var.data.路径[下标]}} 支持数组下标与多级路径', () => {
    expect(resolveStepParams({ uid: '{{playerListTable.data.items[0].uid}}' }, state)).toEqual({
      uid: 'u1',
    });
    expect(resolveStepParams({ total: '{{playerListTable.data.total}}' }, state)).toEqual({
      total: 2,
    });
  });

  it('{{row.字段}} 从事件上下文取值', () => {
    expect(resolveStepParams({ playerId: '{{row.uid}}' }, state, { uid: 'r-9' })).toEqual({
      playerId: 'r-9',
    });
  });

  it('遗留裸形态 区块key.字段 落在 data 上（V3 行为兼容）', () => {
    expect(resolveStepParams({ total: 'playerListTable.total' }, state)).toEqual({ total: 2 });
    expect(resolveStepParams({ keyword: 'filterForm.keyword' }, state)).toEqual({
      keyword: 'kw',
    });
  });

  it('遗留裸形态 row.字段 / ctx.字段 从上下文取值', () => {
    expect(resolveStepParams({ x: 'row.uid' }, state, { uid: 'ctx-1' })).toEqual({ x: 'ctx-1' });
  });

  it('未命中路径与字面量原样返回', () => {
    expect(resolveStepParams({ a: 'playerListTable.nope', b: 'plain-text' }, state)).toEqual({
      a: 'playerListTable.nope',
      b: 'plain-text',
    });
  });

  it('selectedRow 状态键不被误映射到 data（{{var.selectedRow}} 整体引用）', () => {
    expect(resolveStepParams({ row: '{{playerListTable.selectedRow}}' }, state)).toEqual({
      row: { uid: 'u2', nickname: 'alice' },
    });
  });
});
