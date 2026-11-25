/** resolveVariantPresentation（SUI-034）：SUI-031 协议 overlay 语义的合并
 * 实现——页面级属性整体替换/继承、fields 按 key 合并（变体赢、未声明属性
 * 保留基础）、变体独有 key 追加、jsonSchema 恒取基础、未知 key 兜底回基础。 */
import { resolveVariantPresentation } from '../formVariants';
import type { FormPresentationSpec } from '@/types/dashboard';

const base = (): FormPresentationSpec => ({
  jsonSchema: { type: 'object' },
  layout: 'vertical',
  fields: [
    { key: 'playerId', widget: 'Input', label: { 'zh-CN': '玩家' }, width: 12 },
    { key: 'reason', widget: 'TextArea' },
  ],
  groups: [{ key: 'basic', fields: ['playerId'] }],
  submitButton: { text: { 'zh-CN': '提交' } },
});

describe('resolveVariantPresentation', () => {
  it('variantKey 缺省或未声明：原样返回基础呈现', () => {
    const value = base();
    expect(resolveVariantPresentation(value)).toBe(value);
    expect(resolveVariantPresentation(value, '')).toBe(value);
    expect(resolveVariantPresentation(value, 'ghost')).toBe(value);
    const noVariants = { ...value, variants: [] };
    expect(resolveVariantPresentation(noVariants, 'compact')).toBe(noVariants);
  });

  it('fields 按 key 合并：变体声明属性覆盖，未声明属性保留基础', () => {
    const value: FormPresentationSpec = {
      ...base(),
      variants: [
        {
          key: 'compact',
          fields: [{ key: 'playerId', width: 6 }],
        },
      ],
    };
    const merged = resolveVariantPresentation(value, 'compact');
    expect(merged.fields?.[0]).toEqual({
      key: 'playerId',
      widget: 'Input',
      label: { 'zh-CN': '玩家' },
      width: 6,
    });
    // 未覆盖字段原样（引用保留）
    expect(merged.fields?.[1]).toBe(value.fields?.[1]);
  });

  it('变体独有 key 追加为呈现条目；基础 fields 顺序保持', () => {
    const value: FormPresentationSpec = {
      ...base(),
      variants: [
        {
          key: 'extended',
          fields: [
            { key: 'reason', widget: 'Input' },
            { key: 'couponCode', widget: 'Input', label: { 'zh-CN': '券码' } },
          ],
        },
      ],
    };
    const merged = resolveVariantPresentation(value, 'extended');
    expect(merged.fields?.map((field) => field.key)).toEqual(['playerId', 'reason', 'couponCode']);
    expect(merged.fields?.[1]?.widget).toBe('Input');
    expect(merged.fields?.[2]).toEqual({
      key: 'couponCode',
      widget: 'Input',
      label: { 'zh-CN': '券码' },
    });
  });

  it('页面级属性：变体声明即整体替换，未声明继承基础', () => {
    const value: FormPresentationSpec = {
      ...base(),
      variants: [
        { key: 'layoutOnly', layout: 'grid' },
        {
          key: 'groupsOnly',
          groups: [{ key: 'all', fields: ['playerId', 'reason'] }],
        },
      ],
    };
    expect(resolveVariantPresentation(value, 'layoutOnly').layout).toBe('grid');
    expect(resolveVariantPresentation(value, 'layoutOnly').groups).toBe(value.groups);
    expect(resolveVariantPresentation(value, 'groupsOnly').groups).toEqual([
      { key: 'all', fields: ['playerId', 'reason'] },
    ]);
    expect(resolveVariantPresentation(value, 'groupsOnly').layout).toBe('vertical');
    // 按钮：两组都未声明 → 继承
    expect(resolveVariantPresentation(value, 'layoutOnly').submitButton).toBe(value.submitButton);
  });

  it('jsonSchema 恒取基础；变体不改变调用相关项', () => {
    const value: FormPresentationSpec = {
      ...base(),
      variants: [{ key: 'compact', layout: 'inline' }],
    };
    const merged = resolveVariantPresentation(value, 'compact');
    expect(merged.jsonSchema).toBe(value.jsonSchema);
    // 合并结果里不出现变体专属调用键（required 等不在类型面，仅确认基础字段原样）
    expect(merged.fields?.every((field) => field.required === undefined)).toBe(true);
  });
});
