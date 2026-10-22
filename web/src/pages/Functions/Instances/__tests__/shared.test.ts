/**
 * resolveDescriptorSchema 覆盖收口（web 覆盖率巡检：函数此前零测试触达）。
 * 候选优先级与形态兼容见 shared.ts 注释。
 */
import { resolveDescriptorSchema } from '../shared';
import type { FunctionDescriptor } from '@/services/api/functions';

const OBJ = { type: 'object', properties: { a: { type: 'string' } } };

function desc(fields: Record<string, unknown>): FunctionDescriptor {
  return fields as unknown as FunctionDescriptor;
}

describe('resolveDescriptorSchema', () => {
  it('descriptor 为空 → null', () => {
    expect(resolveDescriptorSchema(null)).toBeNull();
    expect(resolveDescriptorSchema(undefined)).toBeNull();
  });

  it('inputSchema 合法 JSON 字符串 → 解析为对象（缺 type 补 object）', () => {
    const out = resolveDescriptorSchema(desc({ inputSchema: '{"properties":{}}' }));
    expect(out).toEqual({ type: 'object', properties: {} });
  });

  it('inputSchema 非法 JSON 字符串 → 落到下一候选 schema', () => {
    const out = resolveDescriptorSchema(desc({ inputSchema: 'not-json', schema: OBJ }));
    expect(out).toEqual(OBJ);
  });

  it('inputSchema 对象形态 → 原样返回', () => {
    expect(resolveDescriptorSchema(desc({ inputSchema: OBJ }))).toEqual(OBJ);
  });

  it('无 inputSchema 时依次取 schema → params → input', () => {
    expect(resolveDescriptorSchema(desc({ schema: OBJ }))).toEqual(OBJ);
    expect(resolveDescriptorSchema(desc({ params: OBJ }))).toEqual(OBJ);
    expect(resolveDescriptorSchema(desc({ input: OBJ }))).toEqual(OBJ);
  });

  it('数组候选跳过；嵌套 input 对象参与候选', () => {
    // inputSchema 是数组（非对象）→ 跳过继续找
    expect(resolveDescriptorSchema(desc({ inputSchema: [], schema: OBJ }))).toEqual(OBJ);
    // input 非对象（字符串原语不 push）
    expect(resolveDescriptorSchema(desc({ input: 'raw', params: OBJ }))).toEqual(OBJ);
  });

  it('全部候选皆空 → null', () => {
    expect(resolveDescriptorSchema(desc({}))).toBeNull();
    expect(resolveDescriptorSchema(desc({ inputSchema: '', schema: null, params: 42 }))).toBeNull();
  });

  it('优先级：inputSchema 字符串合法时压过 schema 对象', () => {
    const parsed = resolveDescriptorSchema(
      desc({ inputSchema: '{"type":"object"}', schema: { type: 'string' } }),
    );
    expect(parsed).toEqual({ type: 'object' });
  });
});
