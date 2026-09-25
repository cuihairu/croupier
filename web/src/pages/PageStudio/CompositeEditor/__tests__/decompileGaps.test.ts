/** decompileToTree 缺口补齐：decompile.test.ts 覆盖主干与不可还原警告，
 * 本文件专补以下回读分支：
 * 1) visibleWhen 缺 key → 条件丢弃警告（L101 `c.key ?? ''` 兜底侧）；
 * 2) cascadePolicy 白名单命中 → 还原到节点 props（L177 三元真侧）；
 * 3) events.action 带 params / chain 步骤不带 params（L350 真侧、L356 假侧）；
 * 4) toolbar 动作无弹窗目标但带单步参数链 → 按钮 onClick（L506 真侧、L507 假侧）。 */
import { decompileToTree, type SpecSectionLike } from '../compiler';

describe('decompileToTree 缺口分支', () => {
  it('visibleWhen 缺 key：无法映射 → 丢弃并告警，不产出 visibleWhen', () => {
    const spec: SpecSectionLike[] = [
      {
        key: 'player.list',
        functionId: 'player.list',
        view: 'table',
        // kind/path 合法但缺 key → 判定不可还原
        visibleWhen: { kind: 'equals', path: '/data/total', value: '10' },
      },
    ];
    const [tree, warnings] = decompileToTree(spec);
    expect(tree).toHaveLength(1);
    expect(warnings.some((w) => w.includes('显示条件无法还原'))).toBe(true);
    expect(tree[0].props.visibleWhen).toBeUndefined();
  });

  it('cascadePolicy 白名单值 → 还原为节点 props（非白名单值丢弃）', () => {
    const spec: SpecSectionLike[] = [
      { key: 'a', functionId: 'a.list', view: 'table', cascadePolicy: 'keep' },
      { key: 'b', functionId: 'b.list', view: 'fields', cascadePolicy: 'nope' },
    ];
    const [tree] = decompileToTree(spec);
    expect(tree.find((n) => n.props.sectionKey === 'a')?.props.cascadePolicy).toBe('keep');
    expect(tree.find((n) => n.props.sectionKey === 'b')?.props.cascadePolicy).toBeUndefined();
  });

  it('events：action 带 params、chain 步骤不带 params → 原样还原', () => {
    const spec: SpecSectionLike[] = [
      {
        key: 'player.list',
        functionId: 'player.list',
        view: 'table',
        events: [
          {
            event: 'rowSelected',
            action: {
              kind: 'openModal',
              target: 'modal-g1',
              params: { playerId: 'uid' },
            },
            chain: [{ kind: 'showMessage', target: '' }],
          },
        ],
      },
      {
        key: 'mail.send',
        group: 'modal-g1',
        functionId: 'mail.send',
        view: 'form',
        display: 'dialog',
      },
    ];
    const [tree, warnings] = decompileToTree(spec);
    expect(warnings).toEqual([]);
    const table = tree.find((n) => n.type === 'fnTable')!;
    const modal = tree.find((n) => n.type === 'modal')!;
    expect(table.props.onRowSelected).toEqual({
      kind: 'openModal',
      target: modal.id,
      params: { playerId: 'uid' },
      chain: [{ kind: 'showMessage', target: '' }],
    });
  });

  it('toolbar 动作无弹窗目标：单步带参链 → 按钮 onClick 收敛为主动作（无 chain 字段）', () => {
    const spec: SpecSectionLike[] = [
      {
        key: 'player.list',
        functionId: 'player.list',
        view: 'table',
        toolbar: {
          actions: [
            {
              label: { 'zh-CN': '刷新列表' },
              chain: [{ kind: 'refreshNode', target: 'player.list', params: { scope: 'all' } }],
            },
          ],
        },
      },
    ];
    const [tree, warnings] = decompileToTree(spec);
    expect(warnings).toEqual([]);
    const table = tree.find((n) => n.type === 'fnTable')!;
    const button = tree.find((n) => n.type === 'button')!;
    expect(button).toBeDefined();
    expect(button.props.title).toBe('刷新列表');
    expect(button.props.onClick).toEqual({
      kind: 'refreshNode',
      target: table.id,
      params: { scope: 'all' },
    });
    expect(table.props.toolbar).toBeUndefined();
  });
});
