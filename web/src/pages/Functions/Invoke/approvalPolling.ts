/**
 * 审批轮询状态机（A4）：pending 期间按间隔轮询，到达终态
 * （approved/rejected/expired）回调一次并停止；停止后不再发起请求，
 * 也不会再回调。单次查询失败静默重试。
 *
 * 终态回调透传服务端续跑事实（continuation/resultKind/taskId/result）：
 * 服务端在 approve 时已按原 payload 自动重放（approval service
 * continueApprovedFunction），调用方必须消费该结果而非提示重新发起，
 * 否则用户二次点击即第二次真实副作用。
 */
import type { JSONValue } from '@/types/dashboard';

export type ApprovalPollStatus = 'pending' | 'approved' | 'rejected' | 'expired';

export type ApprovalPollUpdate = {
  status: Exclude<ApprovalPollStatus, 'pending'>;
  reason?: string;
  continuation?: boolean;
  resultKind?: 'sync' | 'task';
  taskId?: string;
  result?: JSONValue;
};

export type ApprovalPollFetchResult = {
  status: ApprovalPollStatus;
  reason?: string;
  continuation?: boolean;
  resultKind?: 'sync' | 'task';
  taskId?: string;
  result?: JSONValue;
};

export type ApprovalPollFetcher = (approvalId: string) => Promise<ApprovalPollFetchResult>;

export function startApprovalPolling(
  approvalId: string,
  onUpdate: (update: ApprovalPollUpdate) => void,
  fetcher: ApprovalPollFetcher,
  intervalMs: number,
): () => void {
  let stopped = false;
  let timer: ReturnType<typeof setInterval> | null = null;

  const tick = async () => {
    try {
      const result = await fetcher(approvalId);
      if (stopped) return;
      if (result.status !== 'pending') {
        stopped = true;
        if (timer) clearInterval(timer);
        onUpdate({
          status: result.status,
          reason: result.reason,
          continuation: result.continuation,
          resultKind: result.resultKind,
          taskId: result.taskId,
          result: result.result,
        });
      }
    } catch {
      /* 单次查询失败静默重试 */
    }
  };

  void tick();
  timer = setInterval(tick, intervalMs);

  return () => {
    stopped = true;
    if (timer) clearInterval(timer);
  };
}
