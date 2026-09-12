/** U11 模板新鲜度比对门禁（纯函数）。
 *
 * 核心守护：提醒是增值信息，宁可漏报不可误报——
 * - 只有「双方都有 digest 且不一致」才算 stale；
 * - 旧快照（无 digest）/未迁移模板（无 digest）跳过，不误报；
 * - 模板已删除不算「有新版本」。 */
import { computeStaleTemplateNames } from '../templateFreshness';
import type { ComponentTemplateDTO } from '../ComponentLibrary';
import type { ComponentTemplateUsage } from '@/types/dashboard';

function tpl(partial: Partial<ComponentTemplateDTO> & { key: string }): ComponentTemplateDTO {
  return {
    name: { 'zh-CN': `名称-${partial.key}` },
    builtin: false,
    tree: [],
    ...partial,
  } as ComponentTemplateDTO;
}

const usage = (key: string, digest: string): ComponentTemplateUsage => ({ key, digest });

describe('computeStaleTemplateNames（U11 比对门禁）', () => {
  it('digest 不一致 → 列出模板显示名（zh-CN）', () => {
    const items = [tpl({ key: 'combo--a', digest: 'v2' })];
    expect(computeStaleTemplateNames([usage('combo--a', 'v1')], items)).toEqual(['名称-combo--a']);
  });

  it('digest 一致 → 不提示', () => {
    const items = [tpl({ key: 'combo--a', digest: 'v1' })];
    expect(computeStaleTemplateNames([usage('combo--a', 'v1')], items)).toEqual([]);
  });

  it('快照无 digest（旧页面快照）→ 无法判定，跳过不误报', () => {
    const items = [tpl({ key: 'combo--a', digest: 'v1' })];
    expect(computeStaleTemplateNames([usage('combo--a', '')], items)).toEqual([]);
  });

  it('当前模板无 digest（未迁移）→ 无法判定，跳过不误报', () => {
    const items = [tpl({ key: 'combo--a' })];
    expect(computeStaleTemplateNames([usage('combo--a', 'v1')], items)).toEqual([]);
  });

  it('模板已删除（不在当前列表）→ 不算有新版本', () => {
    expect(computeStaleTemplateNames([usage('combo--gone', 'v1')], [])).toEqual([]);
  });

  it('多条混合只列 stale 的；name 裸 string（遗留形态）直接兼容', () => {
    const items = [
      tpl({ key: 'a', digest: 'v2' }),
      { ...tpl({ key: 'b', digest: 'v2' }), name: '遗留名称' },
    ];
    expect(computeStaleTemplateNames([usage('a', 'v0'), usage('b', 'v0')], items)).toEqual([
      '名称-a',
      '遗留名称',
    ]);
  });
});
