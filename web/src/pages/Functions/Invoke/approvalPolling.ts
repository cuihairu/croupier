/**
 * 审批轮询状态机（A4）：pending 期间按间隔轮询，到达终态
 * （approved/rejected/expired）回调一次并停止；停止后不再发起请求，
 * 也不会再回调。单次查询失败静默重试。
 */

export type ApprovalPollStatus = 'pending' | 'approved' | 'rejected' | 'expired';

export type ApprovalPollUpdate = {
  status: Exclude<ApprovalPollStatus, 'pending'>;
  reason?: string;
};

export type ApprovalPollFetcher = (
  approvalId: string,
) => Promise<{ status: ApprovalPollStatus; reason?: string }>;

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
          status: result.status as Exclude<ApprovalPollStatus, 'pending'>,
          reason: result.reason,
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
