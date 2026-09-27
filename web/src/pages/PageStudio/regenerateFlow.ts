/**
 * 「重新生成草稿」冲突恢复流（OPEN-ISSUES #31）。
 *
 * regenerate 会按最新 Proposal 覆盖草稿内容（后端 regenerateDraft 走
 * applyPageSpecToModel 重写），draftRevision 乐观锁是**保护性**的：防
 * 「覆盖未见过的他人编辑」。但 revision 快照来自页面加载，服务端链路
 * （accept-and-publish / 批量重发布 / 其他运营保存）随时会 bump revision
 * ——客户端列表必然变陈旧，409 成为常态死胡同。
 *
 * 恢复策略：409 details.current 是服务端权威 revision。regenerate 的内容
 * 来源是 Proposal 而非客户端，本地没有任何会被丢弃的未提交输入，所以
 * 「知情确认 + 用 current 重试一次」既保留乐观锁的保护语义（用户明确
 * 同意覆盖），又不把用户堵死在 409。
 */

/** Umi request ResponseError 中本功能消费的形态（收窄用，勿放宽）。 */
interface RegenerateRequestError {
  response?: {
    status?: number;
    data?: {
      error?: unknown;
      details?: { current?: unknown };
    };
  };
}

/**
 * 从 regenerate 409 冲突错误提取服务端当前草稿 revision。
 * 后端契约（internal/api/page/service.go regenerateDraft）：
 * 409 + `{ error: "conflict", details: { expected, current, provided } }`。
 */
export function extractConflictCurrentRevision(error: unknown): number | null {
  if (!error || typeof error !== 'object') return null;
  const e = error as RegenerateRequestError;
  if (e.response?.status !== 409) return null;
  const raw = e.response.data?.details?.current;
  const n = typeof raw === 'number' ? raw : Number(raw);
  return Number.isFinite(n) && n > 0 ? n : null;
}

export interface RegenerateFlowDeps {
  regenerate: (pageKey: string, draftRevision: number) => Promise<unknown>;
  /** 知情确认弹窗：onOk 内的重试失败已归口 onFailure。 */
  confirm: (title: string, content: string, onOk: () => Promise<void>) => void;
  onSuccess: (result: unknown) => void | Promise<void>;
  onFailure: () => void;
  formatTitle: (current: number) => string;
  formatContent: (current: number) => string;
}

export async function runRegenerate(
  pageKey: string,
  draftRevision: number,
  deps: RegenerateFlowDeps,
): Promise<void> {
  try {
    const result = await deps.regenerate(pageKey, draftRevision);
    await deps.onSuccess(result);
  } catch (error) {
    const current = extractConflictCurrentRevision(error);
    // revision 相同还 409 说明冲突另有原因，不自动重试
    if (current === null || current === draftRevision) {
      deps.onFailure();
      return;
    }
    deps.confirm(deps.formatTitle(current), deps.formatContent(current), async () => {
      try {
        const result = await deps.regenerate(pageKey, current);
        await deps.onSuccess(result);
      } catch {
        deps.onFailure();
      }
    });
  }
}
