/** V5 T5.6：表达式编译/回读 round-trip 测试（§6 编译规则）。 */
import { compileTree, decompileToTree } from '../compiler';
import { nodeId, type PageNode } from '../model';

function fn(type: PageNode['type'], fid: string, extra: Record<string, unknown> = {}): PageNode {
  return { id: nodeId(type), type, props: { functionId: fid, title: fid, span: 24, ...extra } };
}

const table = fn('fnTable', 'player.list', {
  sectionKey: 'playerListTable',
  autoRun: true,
});
const staticForm: PageNode = {
  id: nodeId('staticForm'),
  type: 'staticForm',
  props: {
    sectionKey: 'filterForm',
    title: '筛选',
    staticSchema: '{"type":"object","properties":{"kw":{"type":"string"}}}',
  },
};

describe('compileTree：表达式 → wire（§6）', () => {
  it('单表达式字面值编译为 page_state（分支路径进 path）', () => {
    const plainForm = fn('fnForm', 'mail.send', {
      sectionKey: 'mailSendForm',
      inputAssignments: [
        { param: 'title', kind: 'literal', value: '{{playerListTable.selectedRow.nickname}}' },
        { param: 'keyword', kind: 'literal', value: '{{filterForm.values.kw}}' },
        { param: 'total', kind: 'literal', value: '{{playerListTable.data.total}}' },
        { param: 'note', kind: 'literal', value: '纯文本字面量' },
      ],
    });
    const { sections, warnings } = compileTree([table, staticForm, plainForm]);
    expect(warnings).toEqual([]);
    const target = sections.find((s) => s.key === 'mailSendForm');
    const byParam = Object.fromEntries((target?.inputAssignments ?? []).map((a) => [a.target, a]));
    expect(byParam['/title']).toMatchObject({
      kind: 'page_state',
      key: 'playerListTable',
      path: '/selectedRow/nickname',
    });
    expect(byParam['/keyword']).toMatchObject({
      kind: 'page_state',
      key: 'filterForm',
      path: '/values/kw',
    });
    expect(byParam['/total']).toMatchObject({
      kind: 'page_state',
      key: 'playerListTable',
      path: '/data/total',
    });
    expect(byParam['/note']).toMatchObject({ kind: 'literal', value: '纯文本字面量' });
  });

  it('未知变量表达式：警告 + 按字面量保留（不静默丢弃）', () => {
    const brokenForm = fn('fnForm', 'mail.send', {
      sectionKey: 'mailSendForm',
      inputAssignments: [{ param: 'broken', kind: 'literal', value: '{{noSuchVar.x}}' }],
    });
    const { sections, warnings } = compileTree([table, brokenForm]);
    const target = sections.find((s) => s.key === 'mailSendForm');
    const broken = (target?.inputAssignments ?? []).find((a) => a.target === '/broken');
    expect(broken).toMatchObject({ kind: 'literal', value: '{{noSuchVar.x}}' });
    expect(warnings.some((w) => w.includes('noSuchVar'))).toBe(true);
  });

  it('行操作 {{row.字段}} → row.字段；跨变量表达式警告并按字面量保留', () => {
    const modalForm = fn('fnForm', 'mail.send', { sectionKey: 'mailSendForm', display: 'dialog' });
    const t = fn('fnTable', 'player.list', {
      sectionKey: 'playerListTable',
      rowActions: [
        {
          label: '发邮件',
          targetSection: modalForm.id,
          params: { nickname: '{{row.nickname}}', x: '{{playerListTable.data.total}}' },
          danger: false,
        },
      ],
    });
    const { sections, warnings } = compileTree([t, modalForm]);
    const tableSec = sections.find((s) => s.key === 'playerListTable');
    const ra = tableSec?.rowActions?.[0];
    expect(ra?.params).toMatchObject({ nickname: 'row.nickname' });
    expect(ra?.params?.x).toBe('{{playerListTable.data.total}}');
    expect(warnings.some((w) => w.includes('仅支持'))).toBe(true);
  });
});

describe('decompileToTree：wire → 表达式（round-trip 可逆）', () => {
  const plainForm = fn('fnForm', 'mail.send', {
    sectionKey: 'mailSendForm',
    inputAssignments: [
      { param: 'title', kind: 'literal', value: '{{playerListTable.selectedRow.nickname}}' },
      { param: 'keyword', kind: 'literal', value: '{{filterForm.values.kw}}' },
      { param: 'total', kind: 'literal', value: '{{playerListTable.data.total}}' },
      { param: 'note', kind: 'literal', value: '纯文本字面量' },
    ],
  });
  const { sections } = compileTree([table, staticForm, plainForm]);

  it('多段 page_state 路径还原为表达式字面值', () => {
    const [tree] = decompileToTree(sections as never);
    const formNode = tree.find((n) => n.type === 'fnForm');
    const assignments = (formNode?.props.inputAssignments ?? []) as Array<Record<string, unknown>>;
    const title = assignments.find((a) => a.param === 'title');
    expect(title).toMatchObject({
      kind: 'literal',
      value: '{{playerListTable.selectedRow.nickname}}',
    });
    const kw = assignments.find((a) => a.param === 'keyword');
    expect(kw).toMatchObject({ kind: 'literal', value: '{{filterForm.values.kw}}' });
  });

  it('再次编译与原始 spec 等价（表达式 ⇄ wire 往返无损）', () => {
    const [tree] = decompileToTree(sections as never);
    const { sections: again } = compileTree(tree);
    const before = sections.find((s) => s.key === 'mailSendForm')?.inputAssignments;
    const after = again.find((s) => s.key === 'mailSendForm')?.inputAssignments;
    expect(after).toEqual(before);
  });

  it('行操作 row.字段 还原为 {{row.字段}}', () => {
    const modalForm = fn('fnForm', 'mail.send', { sectionKey: 'mailSendForm', display: 'dialog' });
    const t = fn('fnTable', 'player.list', {
      sectionKey: 'playerListTable',
      rowActions: [
        { label: '发邮件', targetSection: modalForm.id, params: { nickname: 'row.nickname' } },
      ],
    });
    const { sections: spec } = compileTree([t, modalForm]);
    // 服务端落库形态：section.rowActions（请求顶层）→ section.table.rowActions（存储）
    const stored = spec.map((s) => {
      if (!s.rowActions?.length) return s;
      const { rowActions, ...rest } = s;
      return { ...rest, table: { ...(rest.table ?? {}), rowActions } };
    });
    const [tree] = decompileToTree(stored as never);
    const tableNode = tree.find((n) => n.type === 'fnTable');
    const ra = ((tableNode?.props.rowActions ?? []) as Array<Record<string, unknown>>)[0];
    expect(ra?.params).toMatchObject({ nickname: '{{row.nickname}}' });
    // 再编译往返稳定
    const { sections: again } = compileTree(tree);
    expect(again.find((s) => s.key === 'playerListTable')?.rowActions?.[0]?.params).toEqual({
      nickname: 'row.nickname',
    });
  });
});
