/**
 * SUI-034：命名展示变体（SUI-031 协议）的呈现合并。
 *
 * 协议 overlay 语义（docs/architecture/pagespec-protocol.md「命名展示变体」）：
 * - jsonSchema 与调用相关项恒取基础——变体是纯展示操作，任何变体不改变
 *   提交结构、校验或治理；
 * - 页面级属性（layout/groups/submitButton/cancelButton）变体声明即整体
 *   替换，未声明继承基础；
 * - fields 按 key 合并：同 key 条目按声明属性逐项覆盖（变体赢），变体独有
 *   key（须能在 jsonSchema 解析，SUI-033 校验）追加为呈现条目；
 * - 无删除语义（v1 无 null-to-delete）：变体不能收回基础覆盖。
 *
 * 纯函数、幂等：仅读取输入。SUI-034 编辑器逐风格预览使用；SUI-035（生效
 * 默认解析）与 SUI-036（已发布表单切换）复用同一合并规则。
 */
import type { FormFieldSpec, FormPresentationSpec, FormVariantFieldSpec } from '@/types/dashboard';

/** 解析命名风格的生效呈现：variantKey 为空或解析不到已声明变体时返回基础
 *  呈现本体（与协议解析链的兜底一致）。 */
export function resolveVariantPresentation(
  base: FormPresentationSpec,
  variantKey?: string,
): FormPresentationSpec {
  if (!variantKey) return base;
  const variant = (base.variants || []).find((item) => item && item.key === variantKey);
  if (!variant) return base;

  const baseFields: FormFieldSpec[] = base.fields || [];
  const variantFields: FormVariantFieldSpec[] = variant.fields || [];
  const overrides = new Map<string, FormVariantFieldSpec>();
  for (const item of variantFields) {
    if (item && typeof item.key === 'string') overrides.set(item.key, item);
  }
  const mergedFields: FormFieldSpec[] = baseFields.map((field) => {
    const override = overrides.get(field.key);
    if (!override) return field;
    return { ...field, ...override } as FormFieldSpec;
  });
  const baseKeys = new Set(baseFields.map((field) => field.key));
  for (const item of variantFields) {
    if (item && typeof item.key === 'string' && !baseKeys.has(item.key)) {
      mergedFields.push({ ...item } as FormFieldSpec);
    }
  }

  return {
    ...base,
    layout: variant.layout || base.layout,
    groups: variant.groups ?? base.groups,
    fields: mergedFields,
    submitButton: variant.submitButton ?? base.submitButton,
    cancelButton: variant.cancelButton ?? base.cancelButton,
  };
}
