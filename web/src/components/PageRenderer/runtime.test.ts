import { projectBindingContext } from './runtime';
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
});
