/**
 * antd ConfigProvider 主题：主题套（preset）× 生效明暗（resolved）四套。
 *
 * - blue（拂晓蓝，出厂默认）：只给 colorPrimary，其余 token 走 antd 6
 *   默认（#1677ff 种子派生 hover/active，暗色由 darkAlgorithm 提亮）——
 *   即「荷官墨粉」改造前的原默认观感；
 * - inkpink（荷官墨粉，可选保留）：双套 token 与 docs/.vitepress/theme/
 *   custom.css、global.less 的 CSS 变量同源：
 *   亮色 #93394d 系按钮 + 墨黑正文 #262626；暗色 #b8556b 系（原 #cf6a82
 *   实心按钮白字 4.44:1 不达 AA，2026-11-12 降档）。
 *
 * 对比度红线：正文/按钮/链接类组合一律 ≥4.5:1（axe color-contrast serious
 * 是 CI 硬门禁）。antd 6 默认色板多处不达（#1677ff 实心按钮与链接 4.1:1、
 * 占位符 1.8:1、暗色链接 3.55:1、暗色 danger 3.85:1），这里逐组显式覆写；
 * 品牌种子（菜单选中/图表/SVG accent）保持不动。
 *
 * 消费方两处：app.tsx 的 antd 运行时导出（首帧即正确）与 <ThemeSync>
 * （切换时实时推送）。algorithm 恒传满长数组（default/dark 各一）——
 * useAntdConfigSetter 内部按索引合并，回切亮色时才能干净覆盖。
 */
import { theme as antdTheme, type ThemeConfig } from 'antd';
import type { ThemePreset, ResolvedTheme } from './themeMode';

export const blueLightAntdTheme: ThemeConfig = {
  algorithm: [antdTheme.defaultAlgorithm],
  token: {
    colorPrimary: '#1677ff',
    // 链接文字打白底：antd 默认 colorLink 跟随种子 #1677ff 仅 4.1:1
    // （axe color-contrast serious；暗色扫查实证菜单树行内链接就是这组，
    // 亮色是树未渲染时的潜伏违例）；降到 blue-7 后 ≥6:1
    colorLink: '#0958d9',
    // antd 默认占位符 #bfbfbf 打白底 1.8:1，远低于 4.5:1（资源目录
    // 「选择分类」CI 实证 2026-11-12）；提到 gray-7 后 ≥6.4:1
    colorTextPlaceholder: '#595959',
    // antd 默认 #8c8c8c 打白底 3.3:1，不达 14px 正文 4.5:1（axe
    // color-contrast serious）；提到 gray-7 后 ≥7:1。antd 6 的
    // Typography.Text secondary / Empty description 吃的是
    // colorTextDescription（默认 rgba(0,0,0,0.45)≈#8c8c8c），同批提档
    colorTextSecondary: '#595959',
    colorTextDescription: '#595959',
  },
  components: {
    Button: {
      // 实心主按钮白字打 #1677ff 仅 4.1:1；按钮域内降档到 blue-7 ≥6:1，
      // 全局品牌种子（菜单选中/图表/SVG accent）保持 #1677ff 不动
      colorPrimary: '#0958d9',
      // danger 链接文字 antd 默认 #ff4d4f 打白底仅 3.26:1（菜单树删除
      // 本地实证 2026-11-12）；降到 red-7 后链接/实心（白字）均 ≥5.5:1
      colorError: '#cf1322',
    },
    Menu: {
      // 展开子菜单标题默认 colorPrimary（#1677ff 打 #e7edf4 仅 3.5:1），
      // 菜单项文字由 ProLayout sider token 接管，这里只补 submenu 标题缺口
      subMenuItemSelectedColor: '#0f172a',
    },
  },
};

export const blueDarkAntdTheme: ThemeConfig = {
  algorithm: [antdTheme.darkAlgorithm],
  token: {
    colorPrimary: '#1677ff',
    // 暗色链接 antd 默认 #1668dc 打 #141414 仅 3.55:1（菜单树编辑/加子
    // 菜单 CI 实证 2026-11-12）。注意 antd6 暗色算法会对十六进制种子再跑
    // 一遍调色板派生：#4096ff 实际渲染成 #3983dc（打墨粉暗色容器仅
    // 4.24:1，本地探针实证）；改用 blue-4 #69b1ff 作种子，渲染 #5c99dc
    // 后两套暗底均 ≥5.4:1
    colorLink: '#69b1ff',
    // 暗色占位符默认 rgba(255,255,255,0.25)，合成后 ~2.5:1；提到 0.62
    // 后 ≥6.6:1（与 secondary 0.72 拉开层级且同达 AA）
    colorTextPlaceholder: 'rgba(255, 255, 255, 0.62)',
    // antd 默认暗色 secondary rgba(235,235,245,0.58) 打暗色卡片 ~4:1；
    // 提到 0.72 后 ≥7.5:1（colorTextDescription 同因同批提档）
    colorTextSecondary: 'rgba(255, 255, 255, 0.72)',
    colorTextDescription: 'rgba(255, 255, 255, 0.72)',
  },
  components: {
    Button: {
      // 暗色 danger 链接文字 antd 默认 #dc4446 打暗底 3.85~4.35:1（菜单树
      // 删除 CI 实证 2026-11-12）；提到暗色红后 ≥5.4:1。副作用：danger
      // 实心按钮底色同步变浅，故 dangerColor 改深红字保组合 ≥5.8:1
      // （antd 默认白字打 #dc4446 仅 4.24:1，本就未达 AA）
      colorError: '#e8706d',
      dangerColor: '#2a1215',
    },
    Menu: {
      subMenuItemSelectedColor: 'rgba(255, 255, 255, 0.92)',
    },
  },
};

export const inkPinkLightAntdTheme: ThemeConfig = {
  algorithm: [antdTheme.defaultAlgorithm],
  token: {
    colorPrimary: '#93394d',
    colorPrimaryHover: '#b8556b',
    colorPrimaryActive: '#7a2b3e',
    colorPrimaryTextHover: '#b8556b',
    // 链接跟墨粉品牌色（docs 站 --vp-c-brand-1 同源）：不设时 antd 落到
    // 默认蓝 #1677ff 打白底仅 4.1:1（菜单树编辑/加子菜单本地实证
    // 2026-11-12）；#93394d 打白底 7.16:1
    colorLink: '#93394d',
    colorText: '#262626',
    // 同 blueLight：antd 默认占位符 #bfbfbf 打白底 1.8:1，提到 gray-7
    colorTextPlaceholder: '#595959',
    colorTextSecondary: '#595959',
    colorTextDescription: '#595959',
    colorBgLayout: '#f3f5f7',
    colorBgContainer: 'rgba(255, 255, 255, 0.9)',
    colorBorder: '#e6eaf0',
  },
  components: {
    Button: {
      // 同 blueLight：danger 链接文字 #ff4d4f 打白底 3.26:1，降到 red-7
      colorError: '#cf1322',
    },
    Menu: {
      // 与 global.less --menu-selected-text 同源：粉底上深档墨粉可达 7:1
      subMenuItemSelectedColor: '#7a2b3e',
    },
  },
};

export const inkPinkDarkAntdTheme: ThemeConfig = {
  algorithm: [antdTheme.darkAlgorithm],
  token: {
    // 原暗色主色 #cf6a82 打白字（合成底 #b35d72）仅 4.44:1 差半分达 AA
    // （CI 实证 2026-11-12 对比度扫查）；种子降档到 #b8556b，antd6 暗色
    // 调色板二次派生后实际渲染 #9f4b5e，白字实测 5.77:1；active 随之再
    // 降一档（hover/active 显式值不经派生，按所设值渲染）
    colorPrimary: '#b8556b',
    colorPrimaryHover: '#e58aa0',
    colorPrimaryActive: '#a53f5c',
    colorPrimaryTextHover: '#e58aa0',
    colorText: 'rgba(255, 255, 255, 0.87)',
    // 同 blueDark：链接不跟粉色主色（antd 派生蓝打墨蓝底 3.14:1），
    // 占位符默认合成 ~2.3:1，同批覆写；种子 #69b1ff 经暗色调色板渲染
    // #5c99dc（antd6 对十六进制种子有二次派生），打墨蓝底 ≥5.4:1
    colorLink: '#69b1ff',
    colorTextPlaceholder: 'rgba(255, 255, 255, 0.62)',
    colorTextSecondary: 'rgba(255, 255, 255, 0.72)',
    colorTextDescription: 'rgba(255, 255, 255, 0.72)',
    colorBgLayout: '#1a1a2e',
    colorBgContainer: 'rgba(30, 30, 50, 0.92)',
    colorBorder: 'rgba(255, 255, 255, 0.14)',
  },
  components: {
    Button: {
      // 同 blueDark：danger 链接/文字提亮达 AA，实心改深红字保持组合
      colorError: '#e8706d',
      dangerColor: '#2a1215',
    },
    Menu: {
      subMenuItemSelectedColor: 'rgba(255, 255, 255, 0.92)',
    },
  },
};

export function getAntdThemeConfig(preset: ThemePreset, resolved: ResolvedTheme): ThemeConfig {
  if (preset === 'inkpink')
    return resolved === 'dark' ? inkPinkDarkAntdTheme : inkPinkLightAntdTheme;
  return resolved === 'dark' ? blueDarkAntdTheme : blueLightAntdTheme;
}

/** 主题套主色 hex（SVG 属性/图表序列色/Tag color 等非 CSS 场景用） */
export function getPresetAccent(preset: ThemePreset): string {
  return preset === 'inkpink' ? '#93394d' : '#1677ff';
}
