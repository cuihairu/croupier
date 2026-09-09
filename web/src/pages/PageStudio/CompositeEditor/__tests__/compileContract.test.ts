/** 契约一致性（「字段对上」硬保证）：compileTree 产物的字段面必须与后端
 * CompositeSectionRequest（internal/service/contract_service.go）的 json tag
 * 完全对齐——多字段会被 Go 静默丢弃，少字段/错形态会被校验拒绝。
 * 本测试用一棵「全特性树」逐区块断言字段白名单与值形态。
 */
import { compileTree, decompileToTree } from '../compiler';
import { nodeId, type PageNode } from '../model';

/** 后端允许的字段白名单（CompositeSectionRequest json tag 对照）。 */
const SECTION_ALLOWED_KEYS = [
  'key',
  'group',
  'events',
  'functionId',
  'view',
  'title',
  'span',
  'autoRun',
  'refreshOn',
  'static',
  'form',
  'inputAssignments',
  'display',
  'rowActions',
  'toolbarActions',
  'onSuccessRefresh',
] as const;
const ACTION_ALLOWED_KEYS = ['label', 'targetSection', 'params', 'danger', 'chain'] as const;
const ASSIGNMENT_ALLOWED_KEYS = ['target', 'kind', 'key', 'path', 'value'] as const;
const EVENT_ALLOWED_KEYS = ['event', 'action', 'chain'] as const;
const ACTION_STEP_ALLOWED_KEYS = ['kind', 'target', 'params'] as const;
const VIEWS = ['table', 'fields', 'form'];

function keysOf(value: unknown): string[] {
  return value && typeof value === 'object' ? Object.keys(value) : [];
}

function expectKeysWithin(label: string, value: unknown, allowed: readonly string[]) {
  const extra = keysOf(value).filter((k) => !allowed.includes(k));
  if (extra.length > 0) {
    throw new Error(`${label}: 多余字段 [${extra.join(',')}]`);
  }
  expect(extra).toEqual([]);
}

/** 全特性树：覆盖 static / 双函数多实例 / 行操作 / 工具栏按钮 / 事件链 /
 * 显式参数映射（page_state 分支路径 + literal）/ 弹窗分组 / refreshOn /
 * onSuccessRefresh。 */
function buildFullTree(): PageNode[] {
  const table: PageNode = {
    id: 'tbl1',
    type: 'fnTable',
    props: {
      sectionKey: 'playerListTable',
      functionId: 'player.list',
      title: '玩家列表',
      span: 24,
      autoRun: true,
      refreshOnNode: ['sf1'],
      columns: ['uid', 'nickname'],
      rowActions: [
        {
          label: '发邮件',
          targetSection: 'modal1',
          params: { playerId: 'row.uid' },
          danger: false,
        },
        {
          label: '封禁',
          targetSection: 'modal1',
          params: { playerId: 'row.uid' },
          danger: true,
        },
      ],
      onRowClick: {
        kind: 'showMessage',
        target: '',
        params: { message: 'clicked' },
        chain: [
          { kind: 'runBinding', target: 'form1', params: { playerId: '{{row.uid}}' } },
          { kind: 'closeModal', target: '' },
        ],
      },
      onRowSelected: {
        kind: 'openModal',
        target: 'modal1',
        params: { playerId: '{{playerListTable.selectedRow.uid}}' },
      },
    },
  };
  const button1: PageNode = {
    id: 'btn1',
    type: 'button',
    props: {
      title: '刷新列表',
      onClick: { kind: 'refreshNode', target: 'tbl1' },
    },
  };
  const button2: PageNode = {
    id: 'btn2',
    type: 'button',
    props: {
      title: '帮助',
      onClick: {
        kind: 'showMessage',
        target: '',
        params: { message: 'hello' },
        chain: [{ kind: 'navigate', target: '', params: { url: 'https://example.com' } }],
      },
    },
  };
  const button3: PageNode = {
    id: 'btn3',
    type: 'button',
    props: {
      title: '批量发邮件',
      onClick: {
        kind: 'openModal',
        target: 'modal1',
        params: { playerId: '{{playerListTable.selectedRow.uid}}' },
      },
    },
  };
  const staticForm: PageNode = {
    id: 'sf1',
    type: 'staticForm',
    props: {
      sectionKey: 'filterForm',
      title: '筛选',
      span: 8,
      staticSchema: '{"type":"object","properties":{"keyword":{"type":"string"}}}',
    },
  };
  const form: PageNode = {
    id: 'form1',
    type: 'fnForm',
    props: {
      sectionKey: 'mailSendForm',
      functionId: 'mail.send',
      title: '发邮件',
      display: 'dialog',
      inputAssignments: [
        { param: 'playerId', kind: 'page_state', sourceNodeId: 'tbl1', field: 'selectedRow/uid' },
        { param: 'keyword', kind: 'page_state', sourceNodeId: 'sf1', field: 'values/keyword' },
        { param: 'total', kind: 'literal', value: '{{playerListTable.data.total}}' },
        { param: 'title', kind: 'literal', value: '固定标题' },
      ],
      onSuccessRefresh: { kind: 'refreshNode', target: 'tbl1' },
    },
  };
  const modal: PageNode = {
    id: 'modal1',
    type: 'modal',
    props: { sectionKey: 'mailSendModal', title: '发邮件弹窗', width: 'medium' },
    children: [form],
  };
  const fields: PageNode = {
    id: 'fld1',
    type: 'fnFields',
    props: { sectionKey: 'playerGetFields', functionId: 'player.get', title: '玩家详情', span: 24 },
  };
  const table2: PageNode = {
    id: 'tbl2',
    type: 'fnTable',
    props: { functionId: 'player.list', title: '玩家列表副本', span: 24 },
  };
  return [table, staticForm, button1, button2, button3, modal, fields, table2];
}

describe('compileTree ↔ CompositeSectionRequest 字段面对齐', () => {
  const { sections } = compileTree(buildFullTree());
  const byKey = Object.fromEntries(sections.map((s) => [s.key, s]));

  it('每个区块字段 ⊆ 后端白名单（Go 不会静默丢字段）', () => {
    for (const section of sections) {
      expectKeysWithin(`section[${section.key}]`, section, SECTION_ALLOWED_KEYS);
      for (const ra of section.rowActions ?? []) {
        expectKeysWithin(`rowAction[${section.key}]`, ra, ACTION_ALLOWED_KEYS);
      }
      for (const ta of section.toolbarActions ?? []) {
        expectKeysWithin(`toolbarAction[${section.key}]`, ta, ACTION_ALLOWED_KEYS);
      }
      for (const ia of section.inputAssignments ?? []) {
        expectKeysWithin(`inputAssignment[${section.key}]`, ia, ASSIGNMENT_ALLOWED_KEYS);
      }
      for (const ev of section.events ?? []) {
        expectKeysWithin(`event[${section.key}]`, ev, EVENT_ALLOWED_KEYS);
        expectKeysWithin(`event.action[${section.key}]`, ev.action, ACTION_STEP_ALLOWED_KEYS);
        for (const st of ev.chain ?? []) {
          expectKeysWithin(`event.chain[${section.key}]`, st, ACTION_STEP_ALLOWED_KEYS);
        }
      }
    }
  });

  it('值形态对齐：view 枚举 / title 字符串 / span 数值 / static 带 jsonSchema', () => {
    for (const section of sections) {
      expect(VIEWS).toContain(section.view);
      expect(typeof section.title).toBe('string');
      expect(typeof section.span).toBe('number');
      expect(typeof section.autoRun).toBe('boolean');
    }
    const sf = byKey['filterForm'];
    expect(sf.static).toBe(true);
    expect(sf.functionId).toBe('');
    expect((sf.form?.jsonSchema as Record<string, unknown>).properties).toBeDefined();
  });

  it('各组合特征如实编译', () => {
    // 表格：行操作 + 事件 + refreshOn（section key 形态）+ 工具栏按钮
    const table = byKey['playerListTable'];
    expect(table.view).toBe('table');
    expect(table.rowActions).toHaveLength(2);
    // 行操作目标 = 弹窗 group 名（modal sectionKey，稳定）
    expect(table.rowActions?.[0]).toMatchObject({
      label: '发邮件',
      targetSection: 'mailSendModal',
      params: { playerId: 'row.uid' },
    });
    expect(table.rowActions?.[1].danger).toBe(true);
    expect(table.refreshOn).toContain('filterForm');
    // 行点击事件 + 链
    const rowClick = table.events?.find((e) => e.event === 'rowClick');
    expect(rowClick?.action).toMatchObject({ kind: 'showMessage', params: { message: 'clicked' } });
    expect(rowClick?.chain?.[0]).toMatchObject({
      kind: 'runBinding',
      target: 'mailSendForm',
      params: { playerId: '{{row.uid}}' },
    });
    // 选中事件：表达式参数原样透传
    const rowSelected = table.events?.find((e) => e.event === 'rowSelected');
    expect(rowSelected?.action.params?.playerId).toBe('{{playerListTable.selectedRow.uid}}');
    // 工具栏按钮（按钮节点编译到最近表格）
    const toolbar = table.toolbarActions ?? [];
    expect(toolbar.map((t) => t.label)).toEqual(['刷新列表', '帮助', '批量发邮件']);
    expect(toolbar[0].chain?.[0].kind).toBe('refreshNode');
    // 无目标动作按钮：主动作本身编译为 chain[0]，extraChain 随后
    expect(toolbar[1].chain?.[0]).toMatchObject({
      kind: 'showMessage',
      params: { message: 'hello' },
    });
    expect(toolbar[1].chain?.[1]).toMatchObject({
      kind: 'navigate',
      params: { url: 'https://example.com' },
    });
    // openModal 按钮：预填 params 保留（此前编译静默丢弃——契约回归点）
    expect(toolbar[2]).toMatchObject({
      label: '批量发邮件',
      targetSection: 'mailSendModal',
      params: { playerId: '{{playerListTable.selectedRow.uid}}' },
    });

    // 弹窗表单：display/group + 四种参数映射
    const form = byKey['mailSendForm'];
    expect(form.display).toBe('dialog');
    expect(form.group).toBe('mailSendModal');
    const ia = Object.fromEntries((form.inputAssignments ?? []).map((a) => [a.target, a]));
    expect(ia['/playerId']).toMatchObject({
      kind: 'page_state',
      key: 'playerListTable',
      path: '/selectedRow/uid',
    });
    expect(ia['/keyword']).toMatchObject({
      kind: 'page_state',
      key: 'filterForm',
      path: '/values/keyword',
    });
    expect(ia['/total']).toMatchObject({
      kind: 'page_state',
      key: 'playerListTable',
      path: '/data/total',
    });
    expect(ia['/title']).toMatchObject({ kind: 'literal', value: '固定标题' });
    expect(form.onSuccessRefresh).toEqual(['playerListTable']);

    // 同函数多实例 key 区分
    expect(sections.filter((s) => s.functionId === 'player.list').map((s) => s.key)).toEqual([
      'playerListTable',
      'player.list',
    ]);
  });

  it('回读→再编译：字段面与值稳定（round-trip 无损）', () => {
    // 镜像服务端变换：请求顶层 rowActions/toolbarActions → spec 的
    // table.rowActions / toolbar.actions（generator 落库形态）
    const stored = sections.map((s) => {
      const { rowActions, toolbarActions, ...rest } = s;
      const next: Record<string, unknown> = { ...rest };
      if (rowActions?.length) {
        next.table = { ...(rest.table ?? {}), rowActions };
      }
      if (toolbarActions?.length) {
        next.toolbar = { actions: toolbarActions };
      }
      return next;
    });
    const [tree] = decompileToTree(stored as never);
    const { sections: again } = compileTree(tree);
    const before = sections.find((s) => s.key === 'mailSendForm');
    const after = again.find((s) => s.key === 'mailSendForm');
    expect(after?.display).toBe(before?.display);
    // group 名经反编译回写 modal sectionKey 后完全稳定（不再随节点 id 漂移）
    expect(after?.group).toBe(before?.group);
    expect(after?.inputAssignments).toEqual(before?.inputAssignments);
    const tBefore = sections.find((s) => s.key === 'playerListTable');
    const tAfter = again.find((s) => s.key === 'playerListTable');
    expect(tAfter?.rowActions).toEqual(tBefore?.rowActions);
    expect(tAfter?.events).toEqual(tBefore?.events);
    expect(tAfter?.toolbarActions).toEqual(tBefore?.toolbarActions);
    expect(tAfter?.refreshOn).toEqual(tBefore?.refreshOn);
  });
});
