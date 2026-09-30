/*
 * localizedText 纯函数专项测试
 *
 * 覆盖：空值/字符串直通、BCP47 优先匹配、遗留短 key 回退、任意非空值兜底、fallback。
 */
import { localizedText } from '../localizedText';

describe('localizedText', () => {
  it('null/undefined/空对象 → fallback', () => {
    expect(localizedText(null, 'zh-CN', 'FB')).toBe('FB');
    expect(localizedText(undefined, 'zh-CN', 'FB')).toBe('FB');
    expect(localizedText({}, 'zh-CN', 'FB')).toBe('FB');
  });

  it('字符串直通（非空返回自身、空串回退 fallback）', () => {
    expect(localizedText('plain', 'zh-CN', 'FB')).toBe('plain');
    expect(localizedText('', 'zh-CN', 'FB')).toBe('FB');
  });

  it('zh-CN locale：命中 zh-CN → en-US → 遗留 zh/en → 任意值 → fallback', () => {
    // BCP47 zh-CN 优先
    expect(localizedText({ 'zh-CN': '中文', 'en-US': 'English' }, 'zh-CN', 'FB')).toBe('中文');
    // zh-CN 缺失 → en-US
    expect(localizedText({ 'en-US': 'English' }, 'zh-CN', 'FB')).toBe('English');
    // 均无 → 遗留 zh
    expect(localizedText({ zh: '遗留中文' }, 'zh-CN', 'FB')).toBe('遗留中文');
    // 遗留 en
    expect(localizedText({ en: 'Legacy English' }, 'zh-CN', 'FB')).toBe('Legacy English');
    // 任意非空值（key 任意）
    expect(localizedText({ 'zh-TW': '繁體' }, 'zh-CN', 'FB')).toBe('繁體');
    // 全空 → fallback
    expect(localizedText({ 'zh-CN': '', 'en-US': '' }, 'zh-CN', 'FB')).toBe('FB');
  });

  it('en-US locale：命中 en-US → zh-CN → 遗留 en/zh → 任意值 → fallback', () => {
    // BCP47 en-US 优先
    expect(localizedText({ 'zh-CN': '中文', 'en-US': 'English' }, 'en-US', 'FB')).toBe('English');
    // en-US 缺失 → zh-CN
    expect(localizedText({ 'zh-CN': '中文' }, 'en-US', 'FB')).toBe('中文');
    // 均无 → 遗留 en
    expect(localizedText({ en: 'Legacy English' }, 'en-US', 'FB')).toBe('Legacy English');
    // 遗留 zh
    expect(localizedText({ zh: '遗留中文' }, 'en-US', 'FB')).toBe('遗留中文');
    // 任意非空值
    expect(localizedText({ fr: 'Français' }, 'en-US', 'FB')).toBe('Français');
    // 全空 → fallback
    expect(localizedText({ 'zh-CN': '', 'en-US': '' }, 'en-US', 'FB')).toBe('FB');
  });

  it('其它 locale（如 zh-TW / fr-FR）：按 zh 前缀分流 → 回退顺序同 zh-CN', () => {
    // zh-TW 视为 zh 前缀
    expect(localizedText({ 'zh-CN': '中文', 'en-US': 'English' }, 'zh-TW', 'FB')).toBe('中文');
    expect(localizedText({ 'en-US': 'English' }, 'zh-TW', 'FB')).toBe('English');
    // fr-FR 非 zh 前缀 → en-US 优先
    expect(localizedText({ 'zh-CN': '中文', 'en-US': 'English' }, 'fr-FR', 'FB')).toBe('English');
    expect(localizedText({ 'zh-CN': '中文' }, 'fr-FR', 'FB')).toBe('中文');
  });

  it('值中含空串与非空混合：仅非空字符串参与回退', () => {
    expect(localizedText({ 'zh-CN': '', 'en-US': 'English', foo: 'bar' }, 'zh-CN', 'FB')).toBe(
      'English',
    );
    expect(localizedText({ 'zh-CN': '', 'en-US': '', foo: 'bar' }, 'zh-CN', 'FB')).toBe('bar');
  });
});
