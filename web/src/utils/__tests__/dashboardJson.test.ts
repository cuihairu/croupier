/**
 * utils/dashboardJson：JSONValue 归一矩阵 + 表单解析三件套。
 *
 * isRecord/isJSONValue/toJSONValue/toJSONRecord/isJSONRecord 判定与归一；
 * parseOptionalJSON（空串 → undefined）、parseJSONObject（非 object 抛错、
 * 错误文案带 label）、parseOptionalJSONObject（空串短路）。getIntl 走 mock，
 * 断言 defaultMessage 透出（label 由调用方传入）。
 */
jest.mock('@umijs/max', () => ({
  getIntl: () => ({
    formatMessage: (opts: { defaultMessage: string }) => opts.defaultMessage,
  }),
}));

import {
  isJSONRecord,
  isJSONValue,
  isRecord,
  parseJSONObject,
  parseOptionalJSON,
  parseOptionalJSONObject,
  toJSONRecord,
  toJSONValue,
} from '../dashboardJson';

describe('isRecord / isJSONValue / isJSONRecord', () => {
  it('isRecord：纯对象 true，数组/null/原始值 false', () => {
    expect(isRecord({ a: 1 })).toBe(true);
    expect(isRecord([1, 2])).toBe(false);
    expect(isRecord(null)).toBe(false);
    expect(isRecord('x')).toBe(false);
  });

  it('isJSONValue：null/标量/递归数组与对象 true，undefined/function false', () => {
    expect(isJSONValue(null)).toBe(true);
    expect(isJSONValue(1)).toBe(true);
    expect(isJSONValue('x')).toBe(true);
    expect(isJSONValue(true)).toBe(true);
    expect(isJSONValue({ a: [1, { b: 'c' }] })).toBe(true);
    expect(isJSONValue(undefined)).toBe(false);
    expect(isJSONValue(() => 1)).toBe(false);
    expect(isJSONValue({ a: undefined })).toBe(false);
  });

  it('isJSONRecord：全值为 JSONValue 的对象 true', () => {
    expect(isJSONRecord({ a: 1, b: 'x' })).toBe(true);
    expect(isJSONRecord({ a: undefined })).toBe(false);
    expect(isJSONRecord([1])).toBe(false);
  });
});

describe('toJSONValue / toJSONRecord', () => {
  it('undefined 归一为 null；JSONValue 原样透传（引用不变）', () => {
    const nested = { a: [1, 2] };
    expect(toJSONValue(undefined)).toBeNull();
    expect(toJSONValue(nested)).toBe(nested);
  });

  it('含不可序列化值（function）时走 JSON 往返序列化分支', () => {
    // 现状：isRecord 无原型检查，Date 被 isJSONValue 判 true 原样透传；
    // function 才会触发 JSON.parse(JSON.stringify(...)) 兜底（函数键被丢弃）。
    const fn = () => 1;
    expect(toJSONValue({ a: fn })).toEqual({});
  });

  it('toJSONRecord：逐键归一，非对象整体返回 {}', () => {
    expect(toJSONRecord({ a: 1, b: undefined, c: 'x' })).toEqual({ a: 1, b: null, c: 'x' });
    expect(toJSONRecord([1])).toEqual({});
    expect(toJSONRecord(null)).toEqual({});
  });
});

describe('parseOptionalJSON', () => {
  it('空串/纯空白 → undefined；合法 JSON 归一为 JSONValue', () => {
    expect(parseOptionalJSON('')).toBeUndefined();
    expect(parseOptionalJSON('   ')).toBeUndefined();
    expect(parseOptionalJSON('{"a":1}')).toEqual({ a: 1 });
  });
});

describe('parseJSONObject', () => {
  it('合法 object 原样返回', () => {
    expect(parseJSONObject('{"a":1}', '参数')).toEqual({ a: 1 });
  });

  it('数组/标量抛错，文案带 label', () => {
    expect(() => parseJSONObject('[1,2]', '参数')).toThrow('参数 必须是 JSON object');
    expect(() => parseJSONObject('42', '常量')).toThrow('常量 必须是 JSON object');
  });
});

describe('parseOptionalJSONObject', () => {
  it('空串 → undefined；object → 记录；数组 → 抛错', () => {
    expect(parseOptionalJSONObject('  ', '参数')).toBeUndefined();
    expect(parseOptionalJSONObject('{"a":1}', '参数')).toEqual({ a: 1 });
    expect(() => parseOptionalJSONObject('[]', '参数')).toThrow('参数 必须是 JSON object');
  });
});
