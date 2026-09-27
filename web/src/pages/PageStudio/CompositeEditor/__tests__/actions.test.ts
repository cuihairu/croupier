/**
 * 事件动作模块（CompositeEditor/actions.ts）补缺单测：parseAction 此前仅经
 * 编辑器/预览组件传递命中 happy path——校验分支（未知 kind、needsTarget 丢
 * 目标、params/chain 类型守卫）与注册表谓词（targetFilter/needsTarget/
 * paramFields）、nodeSummary 回退链无直接断言。
 */
import type { PageNode } from '../model';
import { ACTIONS, EVENTS, nodeSummary, parseAction, type ActionKind } from '../actions';

function n(type: string, id: string, props: Record<string, unknown> = {}): PageNode {
  return { id, type: type as PageNode['type'], props, children: [] };
}

const KINDS: ActionKind[] = [
  'openModal',
  'closeModal',
  'runBinding',
  'refreshNode',
  'navigate',
  'showMessage',
];

describe('parseAction（props 动作字段解析守卫）', () => {
  it('非对象输入一律 null', () => {
    expect(parseAction(null)).toBeNull();
    expect(parseAction(undefined)).toBeNull();
    expect(parseAction('openModal')).toBeNull();
    expect(parseAction(42)).toBeNull();
  });

  it('未知 kind 或 kind 非字符串 → null', () => {
    expect(parseAction({ kind: 'explode', target: 'm1' })).toBeNull();
    expect(parseAction({ target: 'm1' })).toBeNull();
    expect(parseAction({ kind: 7, target: 'm1' })).toBeNull();
  });

  it('needsTarget 动作丢目标判非法；无需目标动作 target 缺省为空串', () => {
    expect(parseAction({ kind: 'openModal' })).toBeNull();
    expect(parseAction({ kind: 'openModal', target: '' })).toBeNull();
    expect(parseAction({ kind: 'runBinding' })).toBeNull();

    const close = parseAction({ kind: 'closeModal' });
    expect(close).toEqual({ kind: 'closeModal', target: '' });

    const navigate = parseAction({ kind: 'navigate', target: 123 });
    expect(navigate).toEqual({ kind: 'navigate', target: '' });
  });

  it('params 仅在对象形态透传；chain 仅在数组形态透传', () => {
    const full = parseAction({
      kind: 'showMessage',
      params: { message: '已保存' },
      chain: [{ kind: 'refreshNode', target: 't1' }],
    });
    expect(full).toEqual({
      kind: 'showMessage',
      target: '',
      params: { message: '已保存' },
      chain: [{ kind: 'refreshNode', target: 't1' }],
    });

    // 非法形态的 params/chain 被丢弃而非透传
    expect(parseAction({ kind: 'navigate', params: 'https://x' })).toEqual({
      kind: 'navigate',
      target: '',
    });
    expect(parseAction({ kind: 'navigate', chain: 'not-array' })).toEqual({
      kind: 'navigate',
      target: '',
    });
  });
});

describe('ACTIONS 注册表（label/needsTarget/targetFilter/paramFields）', () => {
  it('六种动作齐全且 label 非空', () => {
    for (const kind of KINDS) {
      expect(ACTIONS[kind].kind).toBe(kind);
      expect(ACTIONS[kind].label.length).toBeGreaterThan(0);
    }
  });

  it('needsTarget：openModal/runBinding/refreshNode 必选目标，其余无需', () => {
    expect(ACTIONS.openModal.needsTarget).toBe(true);
    expect(ACTIONS.runBinding.needsTarget).toBe(true);
    expect(ACTIONS.refreshNode.needsTarget).toBe(true);
    expect(ACTIONS.closeModal.needsTarget).toBe(false);
    expect(ACTIONS.navigate.needsTarget).toBe(false);
    expect(ACTIONS.showMessage.needsTarget).toBe(false);
  });

  it('targetFilter：弹窗动作只留 modal；执行/刷新只留 fn* 组件；其余无目标', () => {
    const nodes = [
      n('modal', 'm1'),
      n('fnTable', 't1'),
      n('fnForm', 'f1'),
      n('button', 'b1'),
      n('text', 'txt'),
    ];
    expect(ACTIONS.openModal.targetFilter(nodes).map((x) => x.id)).toEqual(['m1']);
    expect(ACTIONS.runBinding.targetFilter(nodes).map((x) => x.id)).toEqual(['t1', 'f1']);
    expect(ACTIONS.refreshNode.targetFilter(nodes).map((x) => x.id)).toEqual(['t1', 'f1']);
    expect(ACTIONS.closeModal.targetFilter(nodes)).toEqual([]);
    expect(ACTIONS.navigate.targetFilter(nodes)).toEqual([]);
    expect(ACTIONS.showMessage.targetFilter(nodes)).toEqual([]);
  });

  it('paramFields：navigate=url，showMessage=message，其余无参数字段', () => {
    expect(ACTIONS.navigate.paramFields?.map((f) => f.key)).toEqual(['url']);
    expect(ACTIONS.showMessage.paramFields?.map((f) => f.key)).toEqual(['message']);
    for (const kind of ['openModal', 'closeModal', 'runBinding', 'refreshNode'] as const) {
      expect(ACTIONS[kind].paramFields).toBeUndefined();
    }
  });
});

describe('EVENTS 与 nodeSummary', () => {
  it('内置事件注册：name 与键一致（props 键契约）', () => {
    expect(Object.keys(EVENTS)).toEqual([
      'onClick',
      'onRowClick',
      'onRowSelected',
      'onSuccess',
      'onError',
    ]);
    for (const [key, event] of Object.entries(EVENTS)) {
      expect(event.name).toBe(key);
      expect(event.label.length).toBeGreaterThan(0);
    }
  });

  it('nodeSummary 回退链：title → functionId → type', () => {
    expect(nodeSummary(n('fnTable', 't1', { title: '玩家表' }))).toBe('玩家表');
    expect(nodeSummary(n('fnTable', 't1', { functionId: 'player.ban' }))).toBe('player.ban');
    expect(nodeSummary(n('button', 'b1'))).toBe('button');
    // nullish 才回退：title 为空串按 `??` 语义原样返回（非缺失）
    expect(nodeSummary(n('modal', 'm1', { title: '', functionId: 'x.y' }))).toBe('');
    expect(nodeSummary(n('modal', 'm1', { title: null, functionId: 'x.y' }))).toBe('x.y');
  });
});
