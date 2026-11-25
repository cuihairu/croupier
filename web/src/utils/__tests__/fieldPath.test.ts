/**
 * fieldPath（SUI-019 呈现路径寻址）纯函数矩阵：
 * 裸顶层 key 兼容、JSON Pointer 对象子字段、数组项字段（`-`）、
 * `~0`/`~1` 转义（键名含点或斜杠）、未知路径诊断，以及按路径的
 * 取值/写入/剔除（含穿过数组时不裁剪）。
 */
import {
  crossesArray,
  deleteAtPath,
  fieldPathTokens,
  joinFieldPath,
  isPointerFieldKey,
  resolveFieldSchema,
  setAtPath,
  toggleRequiredAt,
  uiSchemaNodeAt,
  unknownFieldPaths,
  valueAtFieldPath,
} from '../fieldPath';
import type { SchemaNode } from '../fieldPath';

const ROOT = {
  type: 'object',
  properties: {
    city: { type: 'string' },
    address: {
      type: 'object',
      properties: { street: { type: 'string' }, zip: { type: 'string' } },
    },
    tags: {
      type: 'array',
      items: {
        type: 'object',
        properties: { label: { type: 'string' }, on: { type: 'boolean' } },
      },
    },
    'a.b': { type: 'string' },
    'x/y': { type: 'string' },
  },
};

describe('字段标识形态', () => {
  it('裸 key 不是指针，token 为自身', () => {
    expect(isPointerFieldKey('address.city')).toBe(false);
    expect(isPointerFieldKey('/address/city')).toBe(true);
    expect(fieldPathTokens('address.city')).toEqual(['address.city']);
    expect(fieldPathTokens('/address/city')).toEqual(['address', 'city']);
  });

  it('~1/~0 转义往返无损（键名含点或斜杠）', () => {
    expect(fieldPathTokens('/a~1b/c~0d')).toEqual(['a/b', 'c~d']);
    expect(joinFieldPath('/x', ['a/b', 'c~d'])).toBe('/a~1b/c~0d');
    expect(joinFieldPath('bare', ['bare'])).toBe('bare');
  });

  it('空路径与空首段为未知路径', () => {
    expect(resolveFieldSchema(ROOT, '/').container).toBeNull();
    expect(resolveFieldSchema(ROOT, '/').reason).toMatch(/为空/);
  });
});

describe('schema 定位', () => {
  it('对象子字段：定位到父容器的 properties', () => {
    const target = resolveFieldSchema(ROOT, '/address/street');
    expect(target.container).toBe(ROOT.properties.address.properties);
    expect(target.property).toBe('street');
    expect(target.schema).toEqual({ type: 'string' });
  });

  it('兼容旧顶层 key：裸 key 与指针形态指向同一 schema', () => {
    const bare = resolveFieldSchema(ROOT, 'address');
    expect(bare.property).toBe('address');
    expect(bare.schema).toEqual(ROOT.properties.address);
    expect(resolveFieldSchema(ROOT, '/address').schema).toBe(bare.schema);
  });

  it('数组项字段：`-` 指向 items 的子属性', () => {
    const target = resolveFieldSchema(ROOT, '/tags/-/label');
    expect(target.property).toBe('label');
    expect(target.container).toBe(ROOT.properties.tags.items.properties);
    expect(crossesArray(target.tokens)).toBe(true);
  });

  it('键名含点或斜杠：按字面属性名定位', () => {
    // 点号不需转义；斜杠用 ~1 转义后与指针段分隔符区分
    expect(resolveFieldSchema(ROOT, '/a.b').property).toBe('a.b');
    expect(resolveFieldSchema(ROOT, '/x~1y').property).toBe('x/y');
    // 点号嵌套路径不是指针：按字面属性名查找
    expect(resolveFieldSchema(ROOT, 'address.street').container).toBeNull();
    expect(resolveFieldSchema(ROOT, 'address.street').reason).toMatch(/不在/);
  });

  it('未知路径给出可诊断原因（不静默降级）', () => {
    expect(resolveFieldSchema(ROOT, '/ghost/city').reason).toMatch(/不存在/);
    expect(resolveFieldSchema(ROOT, '/address/ghost').reason).toMatch(/properties/);
    expect(resolveFieldSchema(ROOT, '/city/ghost').reason).toMatch(/properties/);
  });
});

describe('值寻址与按路径投影', () => {
  const values = {
    city: 'hz',
    address: { street: 'ninth', zip: '310000' },
    tags: [{ label: 'a', on: true }],
  };

  it('裸 key 与指针取值等价', () => {
    expect(valueAtFieldPath(values, 'address')).toEqual(values.address);
    expect(valueAtFieldPath(values, '/address/zip')).toBe('310000');
    expect(valueAtFieldPath(values, '/tags/0/label')).toBe('a');
    expect(valueAtFieldPath(values, '/address/ghost')).toBeUndefined();
  });

  it('按路径写入不改动兄弟分支', () => {
    const next = setAtPath(values, '/address/zip', '310010');
    expect(next.address?.zip).toBe('310010');
    expect(next.address?.street).toBe('ninth');
    expect(next.city).toBe('hz');
    // 顶层 key 与嵌套写入同路径语义
    expect(setAtPath(values, 'city', 'hz2').city).toBe('hz2');
  });

  it('按路径剔除：嵌套子字段移除，顶层键行为不变', () => {
    const next = deleteAtPath(values, '/address/zip');
    expect('zip' in (next.address ?? {})).toBe(false);
    expect(next.address?.street).toBe('ninth');
    expect(deleteAtPath(values, 'city')).toEqual({
      address: values.address,
      tags: values.tags,
    });
    expect(deleteAtPath(values, '/address/ghost')).toBe(values);
  });

  it('路径穿过数组时不裁剪、不写入（数组按整体提交）', () => {
    expect(deleteAtPath(values, '/tags/-/on')).toBe(values);
    expect(setAtPath(values, '/tags/-/on', false)).toBe(values);
    expect(valueAtFieldPath(values, '/tags/-/on')).toBeUndefined();
  });
});

describe('uiSchema 注入与 required 摘除', () => {
  it('按 uiTokens 逐层取（不存在即创建）uiSchema 分支', () => {
    const target = resolveFieldSchema(ROOT, '/tags/-/label');
    expect(target.uiTokens).toEqual(['tags', '-', 'label']);
    const uiSchema: SchemaNode = {};
    uiSchemaNodeAt(uiSchema, target.uiTokens)['ui:options'] = { readOnly: true };
    expect(uiSchema).toEqual({
      tags: { '-': { label: { 'ui:options': { readOnly: true } } } },
    });
    // 数组项之外按属性名注入
    const nested = resolveFieldSchema(ROOT, '/address/street');
    expect(nested.uiTokens).toEqual(['address', 'street']);
    uiSchemaNodeAt(uiSchema, nested.uiTokens)['ui:widget'] = 'hidden';
    expect((uiSchema.address as Record<string, unknown>).street).toEqual({
      'ui:widget': 'hidden',
    });
  });

  it('required 摘除/追加作用在字段所属宿主 schema 上', () => {
    const root = {
      type: 'object',
      properties: {
        address: {
          type: 'object',
          required: ['street'],
          properties: { street: { type: 'string' } },
        },
        flag: { type: 'boolean' },
      },
      required: ['flag'],
    };
    const nested = resolveFieldSchema(root, '/address/street');
    expect(toggleRequiredAt(nested, 'street', false)).toBe(true);
    expect(root.properties.address.required).toEqual([]);
    expect(toggleRequiredAt(nested, 'street', false)).toBe(false);

    const top = resolveFieldSchema(root, 'flag');
    expect(toggleRequiredAt(top, 'flag', false)).toBe(true);
    expect(root.required).toEqual([]);
    expect(toggleRequiredAt(top, 'flag', true)).toBe(true);
    expect(root.required).toEqual(['flag']);

    // 未知路径不改动 schema
    const ghost = resolveFieldSchema(root, '/address/ghost');
    expect(toggleRequiredAt(ghost, 'ghost', false)).toBe(false);
  });
});

describe('呈现配置诊断', () => {
  it('未知路径（fields 与 groups）一并列出', () => {
    const reasons = unknownFieldPaths(
      [{ key: 'city' }, { key: '/address/zip' }, { key: '/nope/city' }, { key: '/tags/-/on' }],
      ['/tags/-/label', '/address/ghost'],
      ROOT,
    );
    expect(reasons).toHaveLength(2);
    expect(reasons[0]).toMatch(/nope/);
    expect(reasons[1]).toMatch(/ghost/);
  });

  it('全部可定位时无诊断', () => {
    expect(
      unknownFieldPaths([{ key: 'address' }, { key: '/address/street' }], ['address'], ROOT),
    ).toEqual([]);
  });
});
