import { instantiateTemplate, type ComponentTemplateDTO } from '../ComponentLibrary';

function tpl(tree: ComponentTemplateDTO['tree']): ComponentTemplateDTO {
  return {
    key: 'player-management',
    name: { 'zh-CN': '玩家管理' },
    requiredFunctions: ['player.list'],
    tree,
    builtin: false,
  };
}

describe('instantiateTemplate（实例化，U5）', () => {
  it('复制子树 + 重分配 id + 重映射内部引用', () => {
    const table = {
      id: 'fnTable-orig',
      type: 'fnTable' as const,
      props: {
        functionId: 'player.list',
        refreshOnNode: ['fnForm-orig'],
        rowActions: [{ label: '发邮件', targetSection: 'fnForm-orig' }],
      },
    };
    const nodes = instantiateTemplate(
      tpl([
        table,
        { id: 'fnForm-orig', type: 'fnForm' as const, props: { functionId: 'mail.send' } },
      ]),
    );
    expect(nodes).toHaveLength(2);
    expect(nodes[0].id).not.toBe('fnTable-orig');
    expect(nodes[1].id).not.toBe('fnForm-orig');
    // 内部引用重映射到新 id
    expect(nodes[0].props.refreshOnNode).toEqual([nodes[1].id]);
    expect(
      (nodes[0].props.rowActions as Array<{ targetSection: string }>)[0].targetSection,
    ).toBe(nodes[1].id);
  });

  it('模板内 sectionKey 不随实例复制（多实例各自分配，避免冲突）', () => {
    const nodes = instantiateTemplate(
      tpl([
        {
          id: 'fnTable-orig',
          type: 'fnTable' as const,
          props: { functionId: 'player.list', sectionKey: 'gold-rank' },
        },
      ]),
    );
    expect(nodes[0].props.sectionKey).toBeUndefined();
  });

  it('参数应用（U6）：值覆盖白名单 prop，缺省回退 default，未参数化不动', () => {
    const template = tpl([
      { id: 'fnTable-orig', type: 'fnTable' as const, props: { functionId: 'player.list', title: '玩家列表', autoRun: false } },
      {
        id: 'modal-orig',
        type: 'modal' as const,
        props: { title: '弹窗' },
        children: [
          { id: 'fnForm-orig', type: 'fnForm' as const, props: { functionId: 'mail.send', title: '发邮件' } },
        ],
      },
    ]);
    template.params = [
      { key: 'table.title', nodeId: 'fnTable-orig', prop: 'title', default: '默认列表' },
      { key: 'table.autoRun', nodeId: 'fnTable-orig', prop: 'autoRun', default: true },
      { key: 'form.title', nodeId: 'fnForm-orig', prop: 'title', default: '默认表单' },
    ];
    // 显式值覆盖；未提供 key（form.title）回退 default；引用重映射不受影响
    const nodes = instantiateTemplate(template, { 'table.title': 'VIP 列表' });
    expect(nodes[0].props.title).toBe('VIP 列表');
    expect(nodes[0].props.autoRun).toBe(true);
    const form = nodes[1].children![0];
    expect(form.props.title).toBe('默认表单');
    expect(form.props.functionId).toBe('mail.send');
  });

  it('无参数模板实例化行为不变', () => {
    const nodes = instantiateTemplate(
      tpl([{ id: 'fnTable-orig', type: 'fnTable' as const, props: { functionId: 'player.list', title: '列表' } }]),
    );
    expect(nodes[0].props.title).toBe('列表');
  });
});

describe('scanParamCandidates（参数化候选扫描，U6）', () => {
  it('title 恒列出，span/autoRun 存在才列，容器/文本跳过，子树递归', async () => {
    const { scanParamCandidates } = await import('../types');
    const nodes = [
      {
        id: 't1',
        type: 'fnTable' as const,
        props: { functionId: 'player.list', title: '列表', span: 24, autoRun: true },
      },
      {
        id: 'm1',
        type: 'modal' as const,
        props: { title: '弹窗' },
        children: [
          { id: 'f1', type: 'fnForm' as const, props: { functionId: 'mail.send', title: '表单' } },
        ],
      },
      { id: 'x1', type: 'text' as const, props: { content: 'hi' } },
    ];
    const candidates = scanParamCandidates(nodes);
    // t1: title+span+autoRun；modal 容器跳过但子节点 f1: title
    expect(candidates.map((c) => c.key)).toEqual(['t1.title', 't1.span', 't1.autoRun', 'f1.title']);
    expect(candidates[0].current).toBe('列表');
    expect(candidates[2].current).toBe(true);
  });
});
