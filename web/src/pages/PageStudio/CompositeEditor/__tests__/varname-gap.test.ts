/** varname 边界分支补齐：camelize 数字开头尾段、连续占用跳号（base3）、
 * 无操作函数 id 的空操作侧、基础组件三种无标题回退、未知类型 default、
 * renameVariable 叶子节点与非匹配兄弟节点、assignVarNames 以 content
 * 作标题来源与 children 未变路径。 */
import {
  assignVarNames,
  baseVarName,
  camelize,
  generateVarName,
  renameVariable,
  type VarNameSource,
} from '../varname';
import type { PageNode } from '../model';

const node = (over: Partial<PageNode>): PageNode => ({
  id: 'x',
  type: 'text',
  props: {},
  ...over,
});

describe('varname 边界分支补齐', () => {
  it('camelize：数字开头的后续段保持原样拼接（不大写化）', () => {
    expect(camelize('a 1abc')).toBe('a1abc');
  });

  it('generateVarName：base 与 base2 均被占用时跳到 base3（最小可用后缀）', () => {
    expect(
      generateVarName(
        { type: 'fnTable', functionId: 'player' },
        new Set(['playerTable', 'playerTable2']),
      ),
    ).toBe('playerTable3');
  });

  it('函数组件 functionId 无操作段：op 为空仍产出合法基名', () => {
    expect(baseVarName({ type: 'fnTable', functionId: 'player' })).toBe('playerTable');
  });

  it('基础组件无标题/不可命名标题：三种类型各自回退默认名', () => {
    expect(baseVarName({ type: 'staticForm', title: '筛选' })).toBe('filterForm');
    expect(baseVarName({ type: 'modal', title: '' })).toBe('modal');
    expect(baseVarName({ type: 'button', title: undefined })).toBe('button');
  });

  it('未知组件类型：default 分支回退 node', () => {
    expect(baseVarName({ type: 'tabs' as VarNameSource['type'] })).toBe('node');
  });

  it('renameVariable：叶子节点（无 children）同样完成改名', () => {
    const tree = [node({ props: { sectionKey: 'playerList', title: '{{playerList}}' } })];
    const out = renameVariable(tree, 'playerList', 'newList');
    expect(out[0]?.props.sectionKey).toBe('newList');
    expect(out[0]?.props.title).toBe('{{newList}}');
  });

  it('renameVariable：sectionKey 不匹配的兄弟节点保持原值', () => {
    const tree = [
      node({ id: 'a', props: { sectionKey: 'playerList' } }),
      node({ id: 'b', props: { sectionKey: 'otherVar', title: '{{otherVar.name}}' } }),
    ];
    const out = renameVariable(tree, 'playerList', 'newList');
    expect(out[0]?.props.sectionKey).toBe('newList');
    expect(out[1]?.props.sectionKey).toBe('otherVar');
    expect(out[1]?.props.title).toBe('{{otherVar.name}}');
  });

  it('assignVarNames：text 节点以 content 作命名来源；已声明无冲突子树原样保留', () => {
    // existing 只装「已占用」名：kept 未占用 → 保留声明 key（占用态下会判冲突改名）
    const existing = new Set(['unrelated']);
    const out = assignVarNames(
      [
        node({ props: { content: '说明文案' } }),
        node({ props: { sectionKey: 'kept' }, children: [node({ props: {} })] }),
      ],
      existing,
    );
    // content 来源命名（text 无后缀路径 → camelize('说明文案') 为空 → 'text'）
    expect(out[0]?.props.sectionKey).toBe('text');
    // 已声明且不冲突：保留原 key；递归对未命名的 children 继续分配（text2）
    expect(out[1]?.props.sectionKey).toBe('kept');
    expect(out[1]?.children?.[0]?.props.sectionKey).toBe('text2');
  });
});
