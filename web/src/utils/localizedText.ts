import type { LocalizedText } from '@/types/dashboard';

/**
 * 本地化文本渲染的唯一入口。
 *
 * 契约（见 types/dashboard.ts LocalizedText）：key 为 BCP47 locale，
 * 与后端 spec.LocalizedText 一致。历史遗留的 zh/en 短 key 仅在读取时
 * 兜底兼容，任何新数据出口（service normalize）不得产生短 key。
 *
 * 选择顺序：当前 locale → 系统默认（zh-CN → en-US）→ 遗留短 key →
 * 任一非空值 → fallback。
 */
export function localizedText(
  value: LocalizedText | Record<string, string> | string | undefined | null,
  locale: string,
  fallback = '',
): string {
  if (!value) return fallback;
  if (typeof value === 'string') return value || fallback;
  const zh = value['zh-CN'];
  const en = value['en-US'];
  const primary = locale.toLowerCase().startsWith('zh') ? zh || en : en || zh;
  if (primary) return primary;
  const legacyZh = (value as Record<string, string | undefined>).zh;
  const legacyEn = (value as Record<string, string | undefined>).en;
  const legacy = legacyZh || legacyEn;
  if (legacy) return legacy;
  for (const text of Object.values(value)) {
    if (typeof text === 'string' && text.trim()) return text;
  }
  return fallback;
}
