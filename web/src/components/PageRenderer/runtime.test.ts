import { outputPatchFromResult, projectBindingContext } from './runtime';
import type { PageFunctionBinding } from '@/types/dashboard';

function binding(
  assignments: PageFunctionBinding['selectors']['input']['assignments'],
): PageFunctionBinding {
  return {
    id: 'action',
    functionId: 'player.ban',
    usage: 'action',
    execution: { mode: 'sync' },
    selectors: { input: { assignments } },
  };
}

describe('projectBindingContext', () => {
  test('仅发送 row selector 引用的字段', () => {
    const context = projectBindingContext(
      binding([
        { target: '/playerId', source: { kind: 'row', path: '/id' } },
        { target: '/reason', source: { kind: 'row', path: '/moderation/reason' } },
      ]),
      { row: { id: 'p-1', secret: 'must-not-leak', moderation: { reason: 'fraud', score: 99 } } },
    );

    expect(context).toEqual({ row: { id: 'p-1', moderation: { reason: 'fraud' } } });
  });

  test('批量 pick 仅发送每行引用字段，并保留 selector 路径结构', () => {
    const context = projectBindingContext(
      binding([
        {
          target: '/playerIds',
          source: { kind: 'selection', path: '/id', transform: { type: 'pick' } },
        },
      ]),
      {
        selection: [
          { id: 'p-1', email: 'one@example.test' },
          { id: 'p-2', email: 'two@example.test' },
        ],
      },
    );

    expect(context).toEqual({ selection: [{ id: 'p-1' }, { id: 'p-2' }] });
  });

  test('page state 按 state key 和 selector path 投影', () => {
    const context = projectBindingContext(
      binding([{ target: '/taskId', source: { kind: 'page_state', key: 'task', path: '/id' } }]),
      {
        pageState: { task: { id: 'task-1', traceId: 'private' }, unrelated: { token: 'private' } },
      },
    );

    expect(context).toEqual({ pageState: { task: { id: 'task-1' } } });
  });

  test('保留 selector 引用的 falsy JSON 值', () => {
    const context = projectBindingContext(
      binding([
        { target: '/enabled', source: { kind: 'form', path: '/enabled' } },
        { target: '/retryCount', source: { kind: 'form', path: '/retryCount' } },
        { target: '/note', source: { kind: 'form', path: '/note' } },
      ]),
      { form: { enabled: false, retryCount: 0, note: '', secret: 'must-not-leak' } },
    );

    expect(context).toEqual({ form: { enabled: false, retryCount: 0, note: '' } });
  });

  test('detail 上下文按引用字段投影', () => {
    const context = projectBindingContext(
      binding([{ target: '/uid', source: { kind: 'detail', path: '/uid' } }]),
      { detail: { uid: 'u-1', secret: 'must-not-leak' } },
    );
    expect(context).toEqual({ detail: { uid: 'u-1' } });
  });

  test('row 与 detail 同批投影各自归位', () => {
    const context = projectBindingContext(
      binding([
        { target: '/id', source: { kind: 'row', path: '/id' } },
        { target: '/uid', source: { kind: 'detail', path: '/uid' } },
      ]),
      { row: { id: 'p-1' }, detail: { uid: 'u-1' } },
    );
    expect(context).toEqual({ row: { id: 'p-1' }, detail: { uid: 'u-1' } });
  });

  test('同批多个 selection 引用逐行合并投影', () => {
    const context = projectBindingContext(
      binding([
        { target: '/ids', source: { kind: 'selection', path: '/id' } },
        { target: '/names', source: { kind: 'selection', path: '/nickname' } },
      ]),
      {
        selection: [
          { id: 'p-1', nickname: '小明', email: 'one@example.test' },
          { id: 'p-2', nickname: '小红', email: 'two@example.test' },
        ],
      },
    );
    expect(context).toEqual({
      selection: [
        { id: 'p-1', nickname: '小明' },
        { id: 'p-2', nickname: '小红' },
      ],
    });
  });

  test('selection 上下文为非数组脏值时按赋值整体覆盖（不经逐行合并）', () => {
    // 防御：selection 语义上是行数组，但契约是 JSONValue——服务端/历史状态
    // 可能给出对象形态，此时走通用投影并直接覆盖而非 mergeProjectedSelection
    const context = projectBindingContext(
      binding([{ target: '/id', source: { kind: 'selection', path: '/id' } }]),
      { selection: { id: 's-1' } },
    );
    expect(context).toEqual({ selection: { id: 's-1' } });
  });
});

describe('outputPatchFromResult 指针边界', () => {
  const run = (source: string | undefined, data: unknown) =>
    outputPatchFromResult(
      {
        id: 'q',
        functionId: 'fn',
        usage: 'query',
        execution: { mode: 'sync' },
        selectors: {
          input: { assignments: [] },
          output: [{ stateKey: 'picked', source: source as string }],
        },
      },
      { kind: 'invoke', requestId: 'r', data: data as never },
    );

  test('空指针取整个 data', () => {
    expect(run('', { items: [1], total: 1 })).toEqual({ picked: { items: [1], total: 1 } });
  });

  test('非 / 开头的指针视为未命中（patch 不含该键）', () => {
    expect(run('items', { items: [1] })).toEqual({});
  });
});
