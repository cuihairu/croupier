import { compileTree } from '../compiler';
import { nodeId, type PageNode } from '../model';

function fn(type: PageNode['type'], fid: string, extra: Record<string, unknown> = {}): PageNode {
  return { id: nodeId(type), type, props: { functionId: fid, title: fid, span: 24, ...extra } };
}

describe('compileTree（区块 key 命名空间，U5）', () => {
  it('声明 sectionKey 优先固化，不随树顺序漂移', () => {
    const a = fn('fnTable', 'player.list', { autoRun: true, sectionKey: 'gold-rank' });
    const b = fn('fnTable', 'player.list');
    // b 在前：声明 key 的 a 仍拿 gold-rank，b 自动分配 player.list
    const { sections, warnings } = compileTree([b, a]);
    expect(warnings).toEqual([]);
    expect(sections[0].key).toBe('player.list');
    expect(sections[1].key).toBe('gold-rank');
  });

  it('同函数多实例删除首实例后，已固化 key 不漂移', () => {
    // 模拟回读产物：两个实例 key 已固化
    const first = fn('fnTable', 'player.list', { sectionKey: 'player.list' });
    const second = fn('fnTable', 'player.list', { sectionKey: 'player.list-2' });
    // 删除首实例后重编译：second 的 key 不变（不会顶替 player.list）
    const { sections } = compileTree([second]);
    expect(sections[0].key).toBe('player.list-2');
    // 三个实例混合：声明与自动分配共存不冲突
    const third = fn('fnTable', 'player.list');
    const mixed = compileTree([first, second, third]);
    expect(mixed.sections.map((s) => s.key)).toEqual([
      'player.list',
      'player.list-2',
      'player.list-3',
    ]);
    expect(mixed.warnings).toEqual([]);
  });

  it('声明 key 非法或重复时回退自动分配并警告', () => {
    const a = fn('fnFields', 'player.get', { sectionKey: 'bad key!' });
    const b = fn('fnForm', 'mail.send', { sectionKey: 'dup' });
    const c = fn('fnFields', 'player.get', { sectionKey: 'dup' });
    const { sections, warnings } = compileTree([a, b, c]);
    // 非法字符忽略 → 自动分配 player.get；重复 dup → 先到先得，后者自动分配
    expect(sections.map((s) => s.key)).toEqual(['player.get', 'dup', 'player.get-2']);
    expect(warnings.some((w) => w.includes('bad key!'))).toBe(true);
    expect(warnings.some((w) => w.includes('dup'))).toBe(true);
  });

  it('staticForm 声明 key 优先生效且 refreshOn 透传', () => {
    const staticForm: PageNode = {
      id: nodeId('staticForm'),
      type: 'staticForm',
      props: {
        title: '筛选',
        staticSchema: '{"type":"object","properties":{"kw":{"type":"string"}}}',
        refreshOn: ['player.list'],
        sectionKey: 'filter-panel',
      },
    };
    const { sections, warnings } = compileTree([staticForm]);
    expect(warnings).toEqual([]);
    expect(sections[0].key).toBe('filter-panel');
    expect(sections[0].static).toBe(true);
    expect(sections[0].refreshOn).toEqual(['player.list']);
  });
});

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
    expect(sections[0].rowActions).toHaveLength(1);
    expect(sections[0].rowActions![0]).toMatchObject({
      label: '发邮件',
      params: { player_id: 'uid' },
    });
    expect(sections[0].rowActions![0].targetSection).toMatch(/^modal-/);
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
    expect(sections[0].toolbarActions).toHaveLength(1);
    expect(sections[0].toolbarActions![0]).toMatchObject({ label: '批量补偿', danger: true });
    expect(sections[0].toolbarActions![0].targetSection).toMatch(/^modal-/);
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
    expect(warnings.some((w) => w.includes('为空'))).toBe(true);
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
    expect(sections[0].rowActions).toHaveLength(1);
    expect(sections[0].rowActions![0].targetSection).toMatch(/^modal-/);
    expect(sections[2].onSuccessRefresh).toEqual(['player.list-2']);
  });
});

describe('compileTree V3.2：分组弹窗 / 动作链 / 通用事件', () => {
  it('弹窗多组件分组：modal 含表单+字段卡 → 两个 dialog section 共享同一 group', () => {
    const modal: PageNode = {
      id: 'M-G',
      type: 'modal',
      props: { title: '详情弹窗' },
      children: [fn('fnForm', 'mail.send'), fn('fnFields', 'player.get')],
    };
    const { sections, warnings } = compileTree([fn('fnTable', 'player.list'), modal]);
    expect(warnings).toEqual([]);
    const dialogs = sections.filter((x) => x.display === 'dialog');
    expect(dialogs).toHaveLength(2);
    const groups = new Set(dialogs.map((d) => d.group));
    expect(groups.size).toBe(1); // 同一弹窗
  });

  it('动作链编译：openModal + 后续（刷新 + 跳转带 url）→ toolbarActions chain 含 params', () => {
    const modal: PageNode = {
      id: 'M-C',
      type: 'modal',
      props: {},
      children: [fn('fnForm', 'mail.send')],
    };
    const table = fn('fnTable', 'player.list');
    const tableId = table.id;
    const btn = fn('button', '', {
      title: '发邮件',
      onClick: {
        kind: 'openModal',
        target: 'M-C',
        chain: [
          { kind: 'refreshNode', target: tableId },
          { kind: 'navigate', target: '', params: { url: 'https://example.com' } },
        ],
      },
    });
    const { sections, warnings } = compileTree([table, btn, modal]);
    expect(warnings).toEqual([]);
    const ta = sections[0].toolbarActions![0];
    expect(ta.chain).toEqual([
      { kind: 'refreshNode', target: 'player.list' },
      { kind: 'navigate', target: '', params: { url: 'https://example.com' } },
    ]);
  });

  it('无目标主动作（navigate）→ 发布为链首步带 params', () => {
    const btn = fn('button', '', {
      title: '帮助',
      onClick: { kind: 'navigate', target: '', params: { url: '/docs' } },
    });
    const { sections, warnings } = compileTree([fn('fnTable', 'player.list'), btn]);
    expect(warnings).toEqual([]);
    expect(sections[0].toolbarActions![0]).toMatchObject({
      label: '帮助',
      targetSection: '',
    });
    expect(sections[0].toolbarActions![0].chain![0]).toEqual({
      kind: 'navigate',
      target: '',
      params: { url: '/docs' },
    });
  });

  it('fnForm.onSuccess 事件：refresh 步骤→onSuccessRefresh；非刷新步骤警告', () => {
    const table = fn('fnTable', 'player.list');
    const form = fn('fnForm', 'mail.send', {
      onSuccess: {
        kind: 'refreshNode',
        target: table.id,
        chain: [{ kind: 'showMessage', target: '', params: { message: 'ok' } }],
      },
    });
    const { sections, warnings } = compileTree([table, form]);
    expect(sections[1].onSuccessRefresh).toEqual(['player.list']);
    expect(warnings.some((w) => w.includes('非刷新动作'))).toBe(true);
  });

  it('通用事件编译：表格行点击/行选中 → section.events', () => {
    const table = fn('fnTable', 'player.list', {
      onRowClick: { kind: 'openModal', target: 'M-E' },
      onRowSelected: { kind: 'refreshNode', target: 'SELF' },
    });
    table.props.onRowSelected = { kind: 'refreshNode', target: table.id };
    const modal: PageNode = {
      id: 'M-E',
      type: 'modal',
      props: {},
      children: [fn('fnForm', 'mail.send')],
    };
    const { sections, warnings } = compileTree([table, modal]);
    expect(warnings).toEqual([]);
    const evs = sections[0].events!;
    expect(evs.find((e) => e.event === 'rowClick')?.action.kind).toBe('openModal');
    expect(evs.find((e) => e.event === 'rowClick')?.action.target).toMatch(/^modal-/);
    expect(evs.find((e) => e.event === 'rowSelected')?.action).toEqual({
      kind: 'refreshNode',
      target: 'player.list',
    });
  });
});

describe('staticForm（常量表单，无绑定）', () => {
  const staticSchema = JSON.stringify({
    type: 'object',
    properties: { env: { type: 'string', title: '环境', enum: ['prod', 'stage'] } },
  });

  it('编译为 static 区块：透传 jsonSchema、无 functionId', () => {
    const { sections, warnings } = compileTree([
      { id: 'sf1', type: 'staticForm', props: { title: '常量筛选', span: 12, staticSchema } },
      fn('fnTable', 'player.list', { autoRun: true }),
    ]);
    expect(warnings).toEqual([]);
    expect(sections).toHaveLength(2);
    const staticSec = sections[0] as unknown as {
      static?: boolean;
      form?: { jsonSchema: Record<string, unknown> };
      functionId: string;
    };
    expect(staticSec.static).toBe(true);
    expect(staticSec.view).toBe('form');
    expect(staticSec.functionId).toBe('');
    expect(staticSec.form?.jsonSchema).toEqual({
      type: 'object',
      properties: { env: { type: 'string', title: '环境', enum: ['prod', 'stage'] } },
    });
  });

  it('staticSchema JSON 无效 → 警告并跳过', () => {
    const { sections, warnings } = compileTree([
      { id: 'sf-bad', type: 'staticForm', props: { staticSchema: '{bad json' } },
      fn('fnTable', 'player.list', {}),
    ]);
    expect(sections).toHaveLength(1);
    expect(warnings.join('')).toContain('JSON 定义无效');
  });

  it('回读：static 区块还原为 staticForm 节点（round-trip）', async () => {
    const { decompileToTree } = await import('../compiler');
    const [nodes] = decompileToTree([
      {
        key: 'consts',
        functionId: '',
        view: 'form',
        title: '常量筛选',
        span: 12,
        autoRun: false,
        static: true,
        form: {
          jsonSchema: { type: 'object', properties: { env: { type: 'string', enum: ['prod'] } } },
        },
      } as unknown as Parameters<typeof decompileToTree>[0][number],
      { key: 't', functionId: 'player.list', view: 'table', title: 't', span: 24, autoRun: true },
    ]);
    expect(nodes[0].type).toBe('staticForm');
    const schema = JSON.parse(String(nodes[0].props.staticSchema));
    expect(schema.properties.env.enum).toEqual(['prod']);
    expect(nodes[1].type).toBe('fnTable');
  });
});

describe('refreshOn 编译（查询组合模板：refreshOnNode 节点引用 → section key）', () => {
  it('refreshOnNode 引用解析为表单区块 key（同函数双实例键位漂移安全）', () => {
    const { sections } = compileTree([
      { id: 'qform', type: 'fnForm', props: { functionId: 'player.list', display: 'inline' } },
      {
        id: 'qtable',
        type: 'fnTable',
        props: { functionId: 'player.list', autoRun: false, refreshOnNode: ['qform'] },
      },
    ]);
    // 表单是同函数第一个实例 → key=player.list；表格 → player.list-2
    const table = sections.find((sec) => sec.key === 'player.list-2');
    expect(table?.refreshOn).toEqual(['player.list']);
    expect(sections.find((sec) => sec.key === 'player.list')?.refreshOn).toBeUndefined();
  });
});
