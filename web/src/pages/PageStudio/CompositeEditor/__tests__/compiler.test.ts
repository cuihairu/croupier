import { compileTree } from '../compiler';
import { nodeId, type PageNode } from '../model';

function fn(type: PageNode['type'], fid: string, extra: Record<string, unknown> = {}): PageNode {
  return { id: nodeId(type), type, props: { functionId: fid, title: fid, span: 24, ...extra } };
}

describe('compileTree（树→CompositeSection，防回归快照）', () => {
  it('基础：表格+字段卡+行内表单', () => {
    const { sections, warnings } = compileTree([
      fn('fnTable', 'player.list', { autoRun: true, columns: ['uid'] }),
      fn('fnFields', 'player.get', { autoRun: true }),
      fn('fnForm', 'mail.send', { display: 'inline' }),
    ]);
    expect(sections).toEqual([
      {
        key: 'player.list',
        functionId: 'player.list',
        view: 'table',
        title: 'player.list',
        span: 24,
        autoRun: true,
        display: 'inline',
      },
      {
        key: 'player.get',
        functionId: 'player.get',
        view: 'fields',
        title: 'player.get',
        span: 24,
        autoRun: true,
        display: 'inline',
      },
      {
        key: 'mail.send',
        functionId: 'mail.send',
        view: 'form',
        title: 'mail.send',
        span: 24,
        autoRun: false,
        display: 'inline',
      },
    ]);
    expect(warnings).toEqual([]);
  });

  it('玩家管理页典型：表格+行操作+按钮→弹窗+成功刷新', () => {
    const table = fn('fnTable', 'player.list', {
      autoRun: true,
      rowActions: [{ label: '发邮件', targetSection: 'MODAL_ID', params: { player_id: 'uid' } }],
    });
    const modal: PageNode = {
      id: 'MODAL_ID',
      type: 'modal',
      props: { title: '发邮件' },
      children: [
        fn('fnForm', 'mail.send', { onSuccessRefresh: { kind: 'refreshNode', target: 'TBL_ID' } }),
      ],
    };
    table.id = 'TBL_ID';
    const { sections, warnings } = compileTree([table, modal]);
    expect(warnings).toEqual([]);
    expect(sections).toHaveLength(2);
    expect(sections[0].rowActions).toEqual([
      { label: '发邮件', targetSection: 'mail.send', params: { player_id: 'uid' } },
    ]);
    expect(sections[1]).toMatchObject({
      functionId: 'mail.send',
      display: 'dialog',
      onSuccessRefresh: ['player.list'],
    });
  });

  it('按钮（openModal）编译为前一个表格的顶部按钮', () => {
    const table = fn('fnTable', 'order.list');
    const modal: PageNode = {
      id: 'M1',
      type: 'modal',
      props: { title: '补偿' },
      children: [fn('fnForm', 'order.compensate')],
    };
    const button = fn('button', '', {
      title: '批量补偿',
      onClick: { kind: 'openModal', target: 'M1' },
      btnStyle: 'danger',
    });
    const { sections, warnings } = compileTree([table, button, modal]);
    expect(warnings).toEqual([]);
    expect(sections[0].toolbarActions).toEqual([
      { label: '批量补偿', targetSection: 'order.compensate', danger: true },
    ]);
  });

  it('降级与警告：text 忽略 / 无表格按钮 / 空弹窗 / 动作缺失 / 重复函数', () => {
    const modal: PageNode = { id: 'M2', type: 'modal', props: {}, children: [] };
    const { sections, warnings } = compileTree([
      fn('text', '', { content: '说明' }),
      fn('button', '', { title: 'B1', onClick: { kind: 'openModal', target: 'M2' } }),
      modal,
      fn('fnForm', 'a.fn'),
      fn('fnForm', 'a.fn'),
      fn('button', '', { title: 'B2' }),
    ]);
    expect(sections).toHaveLength(2);
    expect(new Set(sections.map((x) => x.key))).toEqual(new Set(['a.fn', 'a.fn-2']));
    expect(warnings.some((w) => w.includes('文本'))).toBe(true);
    expect(warnings.some((w) => w.includes('弹窗') && w.includes('没有函数表单'))).toBe(true);
    expect(warnings.some((w) => w.includes('弹窗目标无效'))).toBe(true);
    expect(warnings.some((w) => w.includes('没有配置动作'))).toBe(true);
  });
});

describe('compileTree 补充：有弹窗但无表格的按钮', () => {
  it('按钮动作 openModal 但页面无表格——警告需放置在表格之后', () => {
    const modal: PageNode = {
      id: 'M3',
      type: 'modal',
      props: {},
      children: [fn('fnForm', 'x.do')],
    };
    const { warnings } = compileTree([
      fn('button', '', { title: 'B', onClick: { kind: 'openModal', target: 'M3' } }),
      modal,
    ]);
    expect(warnings.some((w) => w.includes('需放置在表格之后'))).toBe(true);
  });
});

describe('compileTree 多实例（同函数多组件）', () => {
  it('同一函数两个表格：key 唯一（fid/fid-2），行操作与刷新引用正确 key', () => {
    const t1 = fn('fnTable', 'player.list', { autoRun: true });
    const t2 = fn('fnTable', 'player.list', { autoRun: true, title: '玩家列表(精简)' });
    const modal: PageNode = {
      id: 'M9',
      type: 'modal',
      props: { title: '操作' },
      children: [
        fn('fnForm', 'mail.send', { onSuccessRefresh: { kind: 'refreshNode', target: t2.id } }),
      ],
    };
    t1.props.rowActions = [{ label: '操作', targetSection: 'M9', params: {} }];
    const { sections, warnings } = compileTree([t1, t2, modal]);
    expect(warnings).toEqual([]);
    expect(sections.map((x) => x.key)).toEqual(['player.list', 'player.list-2', 'mail.send']);
    expect(sections[0].rowActions).toEqual([
      { label: '操作', targetSection: 'mail.send', params: {} },
    ]);
    expect(sections[2].onSuccessRefresh).toEqual(['player.list-2']);
  });
});
