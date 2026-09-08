import {
  isSingleExpression,
  matchVariable,
  parseExpression,
  pathToPointer,
  pointerToPath,
  refToText,
  resolveExpression,
  resolveRef,
} from './expression';

const VARS = new Set(['playerListTable', 'mailSendForm', 'player.list']);

describe('expression: 形态判定', () => {
  it('单表达式形态识别', () => {
    expect(isSingleExpression('{{playerListTable.data.total}}')).toBe(true);
    expect(isSingleExpression('  {{ row.uid }}  ')).toBe(true);
    expect(isSingleExpression('{{}}')).toBe(false); // 空表达式
    expect(isSingleExpression('固定文案')).toBe(false);
    expect(isSingleExpression('玩家 {{row.uid}} 已处理')).toBe(false); // 模板混排 = V5.1 边界
  });
});

describe('expression: 变量最长前缀匹配', () => {
  it('旧页面含点 key 命中最长前缀', () => {
    expect(matchVariable('player.list.data.total', VARS)).toBe('player.list');
    expect(matchVariable('playerListTable.selectedRow.uid', VARS)).toBe('playerListTable');
  });
  it('更短前缀不误匹配（playerListTable2 不在集合时不截断）', () => {
    expect(matchVariable('playerListTable2.data', VARS)).toBeUndefined();
  });
});

describe('expression: 解析', () => {
  it('属性路径 + 数组下标', () => {
    const r = parseExpression('{{playerListTable.data.items[0].uid}}', VARS);
    expect(r).toEqual({
      ok: true,
      ref: { variable: 'playerListTable', path: ['data', 'items', 0, 'uid'] },
    });
  });
  it('row 保留变量（无需登记）', () => {
    const r = parseExpression('{{row.playerId}}', new Set());
    expect(r).toEqual({ ok: true, ref: { variable: 'row', path: ['playerId'] } });
  });
  it('整变量引用（无路径）', () => {
    const r = parseExpression('{{mailSendForm}}', VARS);
    expect(r).toEqual({ ok: true, ref: { variable: 'mailSendForm', path: [] } });
  });
  it('非法输入给出可读错误', () => {
    expect(parseExpression('固定文案', VARS).ok).toBe(false);
    expect(parseExpression('{{}}', VARS).ok).toBe(false);
    expect(parseExpression('{{unknownVar.x}}', VARS)).toMatchObject({ ok: false });
    expect(parseExpression('{{playerListTable..x}}', VARS)).toMatchObject({ ok: false });
    expect(parseExpression('{{playerListTable.data[abc]}}', VARS)).toMatchObject({ ok: false });
    expect(parseExpression('{{playerListTable.data[1}}', VARS)).toMatchObject({ ok: false });
  });
});

describe('expression: JSON Pointer 互转', () => {
  it('path → pointer（含转义）', () => {
    expect(pathToPointer(['data', 'total'])).toBe('/data/total');
    expect(pathToPointer(['data', 'items', 0, 'uid'])).toBe('/data/items/0/uid');
    expect(pathToPointer([])).toBe('');
    expect(pathToPointer(['a/b', 'c~d'])).toBe('/a~1b/c~0d');
  });
  it('pointer → path 与 round-trip', () => {
    expect(pointerToPath('/data/items/0/uid')).toEqual(['data', 'items', 0, 'uid']);
    expect(pointerToPath('')).toEqual([]);
    for (const p of [['data', 'total'], ['data', 'items', 0], ['a']]) {
      expect(pointerToPath(pathToPointer(p))).toEqual(p);
    }
  });
});

describe('expression: 求值', () => {
  const state = {
    playerListTable: {
      data: { total: 3, items: [{ uid: 'u1' }, { uid: 'u2' }] },
      selectedRow: { uid: 'u1', nickname: '小明' },
    },
    filterForm: { values: { keyword: 'abc' } },
  };
  it('嵌套路径 / 下标 / 表单值 / 选中行', () => {
    expect(
      resolveRef({ variable: 'playerListTable', path: ['data', 'items', 1, 'uid'] }, state),
    ).toBe('u2');
    expect(
      resolveRef({ variable: 'playerListTable', path: ['selectedRow', 'nickname'] }, state),
    ).toBe('小明');
    expect(resolveRef({ variable: 'filterForm', path: ['values', 'keyword'] }, state)).toBe('abc');
  });
  it('中途缺失 → undefined（不抛错）', () => {
    expect(
      resolveRef({ variable: 'playerListTable', path: ['data', 'items', 9, 'uid'] }, state),
    ).toBeUndefined();
    expect(resolveRef({ variable: 'nope', path: ['x'] }, state)).toBeUndefined();
    expect(
      resolveRef({ variable: 'filterForm', path: ['values', 'a', 'b'] }, state),
    ).toBeUndefined();
  });
  it('row 从上下文取值', () => {
    expect(resolveRef({ variable: 'row', path: ['uid'] }, state, { uid: 'r1' })).toBe('r1');
    expect(resolveRef({ variable: 'row', path: ['uid'] }, state)).toBeUndefined();
  });
  it('resolveExpression：表达式求值、字面量原样、模板混排回退原样', () => {
    expect(resolveExpression('{{playerListTable.data.total}}', state)).toBe(3);
    expect(resolveExpression('固定文案', state)).toBe('固定文案');
    expect(resolveExpression('玩家 {{row.uid}} 已处理', state)).toBe('玩家 {{row.uid}} 已处理');
    // 解析失败的表达式求值期回退字面量（编辑期已被诊断拦截）
    expect(resolveExpression('{{unknownVar.x}}', state)).toBe('{{unknownVar.x}}');
  });
  it('refToText 反向展示', () => {
    expect(refToText({ variable: 'playerListTable', path: ['data', 'items', 0, 'uid'] })).toBe(
      '{{playerListTable.data.items[0].uid}}',
    );
  });
});
