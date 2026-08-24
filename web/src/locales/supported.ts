/**
 * 全局支持的语言列表 — 单一事实来源
 *
 * 与 src/locales/ 根级 *.ts 文件一一对应（文件名即 BCP47 locale），
 * 供 LocalizedTextEditor 等需要枚举全局语言的组件使用。
 * 后端契约（spec.LocalizedText）支持任意 BCP47 key；本列表是
 * "平台界面已提供翻译"的语言，编辑器下拉默认展示全集，
 * 其他 locale 仍可通过自定义 BCP47 入口添加。
 *
 * 新增语言文件（如 ko-KR.ts）时同步在此追加一行。
 */
export type SupportedLocale = {
  /** BCP47 locale，与 src/locales 文件名一致 */
  value: string;
  /** 语言显示名（用各自语言书写） */
  label: string;
};

export const SUPPORTED_LOCALES: SupportedLocale[] = [
  { value: 'zh-CN', label: '简体中文' },
  { value: 'zh-TW', label: '繁體中文' },
  { value: 'en-US', label: 'English (US)' },
  { value: 'ja-JP', label: '日本語' },
  { value: 'pt-BR', label: 'Português (Brasil)' },
  { value: 'bn-BD', label: 'বাংলা' },
  { value: 'fa-IR', label: 'فارسی' },
  { value: 'id-ID', label: 'Bahasa Indonesia' },
];

/** 必选基线语言（不可移除；后端LocalizedText以zh-CN为主回退） */
export const REQUIRED_LOCALE = 'zh-CN';

export const SUPPORTED_LOCALE_LABELS: Record<string, string> = Object.fromEntries(
  SUPPORTED_LOCALES.map((item) => [item.value, item.label]),
);
