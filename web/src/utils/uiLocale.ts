/**
 * SUI-024：表单文案 locale 取值入口。
 *
 * umi 运行时内返回当前平台 locale（getLocale）；umi 不可用（单测替身/异常）
 * 回退 zh-CN。放在 utils 层：SchemaFormRenderer 的 index.tsx 与 templates.tsx
 * 都要用，避免 renderer 内部互相引用成环。
 *
 * 统一回退链由 localizedText 保证（当前 locale → zh/en 互备 → 遗留短 key →
 * 任一非空 → fallback 实参）；这里只负责「当前 locale 是什么」。
 */
import { getLocale } from '@umijs/max';

export function currentUiLocale(): string {
  try {
    return getLocale() || 'zh-CN';
  } catch {
    return 'zh-CN';
  }
}
