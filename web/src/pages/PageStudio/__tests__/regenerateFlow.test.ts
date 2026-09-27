/**
 * 「重新生成草稿」冲突恢复流回归（OPEN-ISSUES #31）。
 *
 * 后端 regenerate 的 draftRevision 乐观锁是保护性的（regenerate 会覆盖
 * 草稿），但 revision 快照来自页面加载，accept-and-publish / 批量重发布 /
 * 其他运营保存都会在服务端 bump revision——409 成为常态死胡同。
 * 恢复契约：409 + details.current → 知情确认 → 用 current 重试一次。
 * （修复前红：无本模块，冲突被 catch 吞成通用「重新生成草稿失败」。）
 */
import { extractConflictCurrentRevision, runRegenerate } from '../regenerateFlow';
import type { RegenerateFlowDeps } from '../regenerateFlow';

function conflictError(current: number, provided: number): unknown {
  return {
    response: {
      status: 409,
      data: {
        error: 'conflict',
        message: '草稿版本冲突：页面已被其他修改更新，请刷新草稿后重试',
        details: { expected: current, current, provided },
      },
    },
  };
}

function makeDeps(overrides: Partial<RegenerateFlowDeps> = {}): {
  deps: RegenerateFlowDeps;
  calls: { revisions: number[]; confirmTitles: string[]; failures: number; successes: unknown[] };
} {
  const calls = {
    revisions: [] as number[],
    confirmTitles: [] as string[],
    failures: 0,
    successes: [] as unknown[],
  };
  // 记录器恒在：overrides.regenerate 只决定「返回什么」，revision 记录不被覆盖
  const rawRegenerate: RegenerateFlowDeps['regenerate'] =
    overrides.regenerate ??
    (async (_pageKey: string, revision: number) => ({ page: {}, draftRevision: revision + 1 }));
  const deps: RegenerateFlowDeps = {
    confirm: jest.fn((title: string, _content: string, onOk: () => Promise<void>) => {
      calls.confirmTitles.push(title);
      return onOk();
    }),
    onSuccess: jest.fn((result: unknown) => {
      calls.successes.push(result);
    }),
    onFailure: jest.fn(() => {
      calls.failures += 1;
    }),
    formatTitle: (current: number) => `草稿版本已过期（第 ${current} 版）`,
    formatContent: (current: number) => `当前草稿已是第 ${current} 版，重新生成会覆盖草稿内容`,
    ...overrides,
    regenerate: (pageKey: string, revision: number) => {
      calls.revisions.push(revision);
      return rawRegenerate(pageKey, revision);
    },
  };
  return { deps, calls };
}

describe('extractConflictCurrentRevision', () => {
  it('409 + details.current → 服务端权威 revision', () => {
    expect(extractConflictCurrentRevision(conflictError(7, 3))).toBe(7);
  });
  it('非 409 / 无 details / 非法值 → null', () => {
    expect(extractConflictCurrentRevision({ response: { status: 400 } })).toBeNull();
    expect(extractConflictCurrentRevision(new Error('x'))).toBeNull();
    expect(extractConflictCurrentRevision(null)).toBeNull();
    expect(
      extractConflictCurrentRevision({ response: { status: 409, data: { details: {} } } }),
    ).toBeNull();
  });
});

describe('runRegenerate', () => {
  it('首次成功：不弹确认、不重试', async () => {
    const { deps, calls } = makeDeps();
    await runRegenerate('p1', 3, deps);
    expect(calls.revisions).toEqual([3]);
    expect(calls.confirmTitles).toHaveLength(0);
    expect(calls.successes).toHaveLength(1);
    expect(calls.failures).toBe(0);
  });

  it('409（快照过期）→ 知情确认后用 details.current 重试一次并成功', async () => {
    const { deps, calls } = makeDeps({
      regenerate: jest
        .fn()
        .mockRejectedValueOnce(conflictError(7, 3))
        .mockResolvedValueOnce({ page: {}, draftRevision: 8 }),
    });
    await runRegenerate('p1', 3, deps);
    // 确认弹窗展示服务端当前版本
    expect(calls.confirmTitles).toEqual(['草稿版本已过期（第 7 版）']);
    // 重试用的是服务端 revision 而非本地快照
    expect(calls.revisions).toEqual([3, 7]);
    expect(calls.successes).toHaveLength(1);
    expect(calls.failures).toBe(0);
  });

  it('409 但 current === 本地快照（冲突另有原因）→ 不自动重试，归口失败', async () => {
    const { deps, calls } = makeDeps({
      regenerate: jest.fn().mockRejectedValue(conflictError(3, 3)),
    });
    await runRegenerate('p1', 3, deps);
    expect(calls.confirmTitles).toHaveLength(0);
    expect(calls.revisions).toEqual([3]);
    expect(calls.failures).toBe(1);
  });

  it('非 409 错误 → 直接失败，不弹确认', async () => {
    const { deps, calls } = makeDeps({
      regenerate: jest.fn().mockRejectedValue(new Error('network')),
    });
    await runRegenerate('p1', 3, deps);
    expect(calls.confirmTitles).toHaveLength(0);
    expect(calls.failures).toBe(1);
  });

  it('重试仍失败 → 归口 onFailure（不把异常抛给调用方）', async () => {
    const { deps, calls } = makeDeps({
      regenerate: jest
        .fn()
        .mockRejectedValueOnce(conflictError(7, 3))
        .mockRejectedValueOnce(conflictError(9, 7)),
    });
    await runRegenerate('p1', 3, deps);
    expect(calls.revisions).toEqual([3, 7]);
    expect(calls.failures).toBe(1);
  });
});
