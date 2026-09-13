import { localizedText } from './localizedText';

describe('localizedText', () => {
  describe('空值兜底', () => {
    it('undefined / null → fallback（默认空串）', () => {
      expect(localizedText(undefined, 'zh-CN')).toBe('');
      expect(localizedText(null, 'zh-CN', 'fb')).toBe('fb');
    });

    it('空字符串 value → fallback', () => {
      expect(localizedText('', 'zh-CN', 'fb')).toBe('fb');
    });
  });

  describe('裸字符串', () => {
    it('非空字符串原样返回', () => {
      expect(localizedText('plain', 'en-US', 'fb')).toBe('plain');
    });

    it('空串回落 fallback', () => {
      expect(localizedText('', 'zh-CN', 'fb')).toBe('fb');
    });
  });

  describe('主 locale 优先级', () => {
    const both = { 'zh-CN': '中文', 'en-US': 'English' };

    it('zh 系 locale → zh-CN 优先（大小写不敏感）', () => {
      expect(localizedText(both, 'zh-CN')).toBe('中文');
      expect(localizedText(both, 'zh')).toBe('中文');
      expect(localizedText(both, 'zh-TW')).toBe('中文');
      expect(localizedText(both, 'ZH')).toBe('中文');
    });

    it('非 zh locale → en-US 优先', () => {
      expect(localizedText(both, 'en-US')).toBe('English');
      expect(localizedText(both, 'fr-FR')).toBe('English');
    });

    it('单侧缺失：zh 缺 → zh 系回落 en；en 缺 → 非 zh 回落 zh', () => {
      expect(localizedText({ 'en-US': 'English' }, 'zh-CN')).toBe('English');
      expect(localizedText({ 'zh-CN': '中文' }, 'en-US')).toBe('中文');
    });
  });

  describe('遗留短 key 兼容（只读兜底）', () => {
    it('zh / en 短 key 命中（zh 优先，与 locale 无关）', () => {
      expect(localizedText({ zh: '旧中文' }, 'zh-CN')).toBe('旧中文');
      expect(localizedText({ en: 'old-en' }, 'en-US')).toBe('old-en');
      expect(localizedText({ zh: '旧中文', en: 'old-en' }, 'fr')).toBe('旧中文');
      expect(localizedText({ zh: '旧中文', en: 'old-en' }, 'zh-CN')).toBe('旧中文');
      expect(localizedText({ en: 'old-en' }, 'fr')).toBe('old-en');
    });
  });

  describe('任一非空值兜底', () => {
    it('其它 locale 的值按对象顺序取第一个非空字符串', () => {
      expect(localizedText({ 'ja-JP': '日本語', 'ko-KR': '한국어' }, 'en-US')).toBe('日本語');
    });

    it('空白字符串被跳过', () => {
      expect(localizedText({ fr: '   ', de: 'de' }, 'en-US')).toBe('de');
    });

    it('非字符串值被跳过，全空 → fallback', () => {
      expect(localizedText({ n: 1 } as unknown as Record<string, string>, 'en-US', 'fb')).toBe(
        'fb',
      );
      expect(localizedText({ fr: '  ' }, 'en-US', 'fb')).toBe('fb');
    });
  });
});
