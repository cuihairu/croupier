/**
 * 字段 key 人性化：camelCase/snake_case/kebab-case/点路径 → 空格分词 + 首字母大写。
 * 用于 schema title 缺失时的表单 label 兜底（todo.md F5）。
 */
export function humanizeFieldKey(key: string): string {
  const words = key
    .replace(/[_\-.]+/g, ' ')
    .replace(/([a-z0-9])([A-Z])/g, '$1 $2')
    .split(/\s+/)
    .filter(Boolean)
    .map((word) => word.charAt(0).toUpperCase() + word.slice(1));
  return words.join(' ') || key;
}
