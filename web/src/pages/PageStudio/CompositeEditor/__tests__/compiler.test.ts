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
        functionId: 'player.list',
        view: 'table',
        title: 'player.list',
        span: 24,
        autoRun: true,
        display: 'inline',
      },
      {
        functionId: 'player.get',
        view: 'fields',
        title: 'player.get',
        span: 24,
        autoRun: true,
        display: 'inline',
      },
      {
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
    expect(sections).toHaveLength(1);
    expect(warnings.some((w) => w.includes('文本'))).toBe(true);
    expect(warnings.some((w) => w.includes('弹窗') && w.includes('没有函数表单'))).toBe(true);
    expect(warnings.some((w) => w.includes('弹窗目标无效'))).toBe(true);
    expect(warnings.some((w) => w.includes('仅保留第一个'))).toBe(true);
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
