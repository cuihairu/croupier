/**
 * SUI-044：风格的主题外观范围决策——可读性校验落在主题 token 上。
 *
 * 变体（SUI-031）只表达布局/呈现，不含颜色与字体；「密度」通过既有的
 * `layout`/`width`/`groups` 纯展示键表达。同一风格因此与主题无关——它
 * 继承当前主题套的 token。本测试把 `getAntdThemeConfig` 经 antd 6
 * `theme.getDesignToken` 解析为**渲染值**（暗色算法会对十六进制种子再跑
 * 一遍调色板派生，如 #4096ff → #3983dc、#b8556b → #9f4b5e，用种子值算
 * 对比度是错的），对四套组合（blue/inkpink × 亮/暗）断言表单实际渲染的
 * 文字/控件色对均 ≥4.5:1（WCAG AA 正文）。守门 `web/src/utils/antdThemeConfig.ts`
 * 的对比度红线：任何主题套/明暗改动若跌破 AA，此套件红。
 */
import { theme } from 'antd';
import { getAntdThemeConfig } from '../antdThemeConfig';
import type { ResolvedTheme, ThemePreset } from '../themeMode';

/** 解析 CSS 颜色为 [r, g, b, alpha]（#rgb/#rrggbb 与 rgb()/rgba() 形态）。 */
function toRgb(color: string): [number, number, number, number] {
  const value = color.trim();
  if (value.startsWith('#')) {
    const hex = value.slice(1);
    const full =
      hex.length === 3
        ? hex
            .split('')
            .map((c) => c + c)
            .join('')
        : hex;
    return [
      parseInt(full.slice(0, 2), 16),
      parseInt(full.slice(2, 4), 16),
      parseInt(full.slice(4, 6), 16),
      1,
    ];
  }
  const parts = value.match(/[\d.]+/g);
  return [
    Number(parts?.[0] ?? 0),
    Number(parts?.[1] ?? 0),
    Number(parts?.[2] ?? 0),
    Number(parts?.[3] ?? 1),
  ];
}

/** 把半透明前景合成到不透明底上（表单容器浮在版面底色之上）。 */
function composite(fg: string, bg: string): string {
  const [r, g, b, a] = toRgb(fg);
  const [br, bgG, bb] = toRgb(bg);
  const mix = (v: number, base: number): number => Math.round(v * a + base * (1 - a));
  const hex = (v: number): string => v.toString(16).padStart(2, '0');
  return `#${hex(mix(r, br))}${hex(mix(g, bgG))}${hex(mix(b, bb))}`;
}

/** WCAG 2.x 相对亮度与对比度。 */
function luminance(color: string): number {
  const [r, g, b] = toRgb(color).map((v) => {
    const s = v / 255;
    return s <= 0.03928 ? s / 12.92 : Math.pow((s + 0.055) / 1.055, 2.4);
  });
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}

function contrast(fg: string, bg: string): number {
  const [x, y] = [luminance(fg), luminance(bg)].sort((a, b) => b - a);
  return (x + 0.05) / (y + 0.05);
}

const COMBINATIONS: Array<{ preset: ThemePreset; resolved: ResolvedTheme }> = [
  { preset: 'blue', resolved: 'light' },
  { preset: 'blue', resolved: 'dark' },
  { preset: 'inkpink', resolved: 'light' },
  { preset: 'inkpink', resolved: 'dark' },
];

interface ContrastCase {
  preset: ThemePreset;
  resolved: ResolvedTheme;
  pair: string;
  ratio: number;
}

/** 四套组合 × 表单实际渲染的色对，逐项算渲染值对比度。 */
const CASES: ContrastCase[] = COMBINATIONS.flatMap(({ preset, resolved }) => {
  // 与 <ThemeSync> 同一解析口径：主题配置 → getDesignToken 渲染值
  const config = getAntdThemeConfig(preset, resolved);
  const tokens = theme.getDesignToken(config);
  // 主按钮在主题配置里有 Button 域独立降档（blue 亮色 #1677ff → #0958d9），
  // getDesignToken 不展开组件 token，故取组件域声明值，未声明时用全局种子。
  const buttonSeed = config.components?.Button?.colorPrimary || tokens.colorPrimary;
  // 表单容器底色：colorBgContainer（半透明时合成到版面底色上）；
  // 版面底色取 colorBgLayout，缺省时按明暗取 antd 默认底
  const pageBg = tokens.colorBgLayout || (resolved === 'dark' ? '#141414' : '#ffffff');
  const surface = composite(tokens.colorBgContainer, pageBg);
  const pairs: Array<[string, number]> = [
    ['link', contrast(tokens.colorLink, surface)],
    ['secondary', contrast(composite(tokens.colorTextSecondary, surface), surface)],
    ['placeholder', contrast(composite(tokens.colorTextPlaceholder, surface), surface)],
    ['primary-button-white', contrast('#ffffff', buttonSeed)],
  ];
  return pairs.map(([pair, ratio]) => ({ preset, resolved, pair, ratio }));
});

describe('SUI-044: 同一风格在亮暗主题下的可读性（四套组合 AA）', () => {
  it.each(CASES)('$preset · $resolved · $pair ≥ 4.5:1', ({ ratio }) => {
    expect(ratio).toBeGreaterThanOrEqual(4.5);
  });
});
