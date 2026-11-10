/**
 * layoutTokenFor（ProLayout 壳层 token 按 主题套 × 明暗）回归：
 * - 暗色必须整体换深色壳（bgLayout/菜单底/菜单文字/选中态），杜绝
 *   「静态亮色 token 打在暗色底上」的对比度违例（axe color-contrast 家族）；
 * - 明暗回切不残留旧档字段（token 整体替换，调用方不做深合并）；
 * - colorPrimary 与 antd 侧（antdThemeConfig）同源。
 */
import { layoutTokenFor } from './defaultSettings';

type Token = Record<string, unknown>;

describe('layoutTokenFor', () => {
  it('亮色两套：中性亮壳，blue/inkpink 仅 colorPrimary 不同', () => {
    const blue = layoutTokenFor('blue', 'light');
    const ink = layoutTokenFor('inkpink', 'light');
    expect(blue.navTheme).toBe('light');
    expect(blue.token.bgLayout).toBe('#f3f5f7');
    const blueSider = blue.token.sider as Token;
    expect(blueSider.colorTextMenu).toBe('#4b5563');
    expect(blueSider.colorTextMenuSelected).toBe('#0f172a');
    expect(blue.colorPrimary).toBe('#1677ff');
    expect(ink.colorPrimary).toBe('#93394d');
    // 其余亮色壳层字段两套一致（强调色由 global.less CSS 变量接管）
    expect(ink.token.sider).toEqual(blueSider);
    expect(ink.token.bgLayout).toBe(blue.token.bgLayout);
  });

  it('暗色：整体换深色壳，文字 token 全部达标浅色', () => {
    for (const preset of ['blue', 'inkpink'] as const) {
      const t = layoutTokenFor(preset, 'dark');
      expect(t.navTheme).toBe('realDark');
      expect(t.token.bgLayout).toBe('#1a1a2e');
      const sider = t.token.sider as Token;
      expect(sider.colorMenuBackground).toBe('#1f1f33');
      expect(sider.colorTextMenu).toBe('rgba(255, 255, 255, 0.65)');
      expect(sider.colorTextMenuSelected).toBe('rgba(255, 255, 255, 0.92)');
      const header = t.token.header as Token;
      expect(header.colorBgHeader).toBe('rgba(26, 26, 46, 0.92)');
      expect(header.colorHeaderTitle).toBe('rgba(255, 255, 255, 0.88)');
    }
  });

  it('暗色 colorPrimary 与 antd 侧同源（blue #1668dc / inkpink #cf6a82）', () => {
    expect(layoutTokenFor('blue', 'dark').colorPrimary).toBe('#1668dc');
    expect(layoutTokenFor('inkpink', 'dark').colorPrimary).toBe('#cf6a82');
  });

  it('暗色 token 不残留亮色档（整体替换语义）', () => {
    const t = layoutTokenFor('blue', 'dark');
    const sider = t.token.sider as Token;
    expect(sider.colorTextMenu).not.toBe('#4b5563');
    expect(sider.colorMenuBackground).not.toBe('#fbfbfa');
    expect(t.token.bgLayout).not.toBe('#f3f5f7');
  });
});
