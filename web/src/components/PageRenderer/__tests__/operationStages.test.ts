/** 操作页多阶段状态反馈派生（D1 状态反馈 Steps）：提交 → 审批 → 执行结果。
 *
 * 覆盖：在途与失败为短序列终态、审批路径随审批状态推进、审批通过后的续跑类别、
 * 任务路径保持执行中、以及无结果时的空序列。
 */
import { describeOperationStages, type OperationStageInput } from '../operationStages';

const base: OperationStageInput = {
  loading: false,
  hasError: false,
  resultKind: null,
  approvalStatus: null,
  continuationKind: null,
};

const of = (input: Partial<OperationStageInput>) => describeOperationStages({ ...base, ...input });

describe('describeOperationStages', () => {
  it('尚未有结果：阶段序列为空', () => {
    expect(of({})).toEqual([]);
  });

  it('提交在途：只有一阶段 process，后续阶段不可知', () => {
    expect(of({ loading: true })).toEqual([{ key: 'submit', status: 'process' }]);
  });

  it('在途优先：错误与既有结果都不展开后续阶段', () => {
    expect(
      of({
        loading: true,
        hasError: true,
        resultKind: 'approval',
        approvalStatus: 'approved',
        continuationKind: 'sync',
      }),
    ).toEqual([{ key: 'submit', status: 'process' }]);
  });

  it('提交失败：终态停在提交阶段 error，不渲染未到达的阶段', () => {
    expect(
      of({
        hasError: true,
        resultKind: 'approval',
        approvalStatus: 'approved',
        continuationKind: 'sync',
      }),
    ).toEqual([{ key: 'submit', status: 'error' }]);
  });

  it('同步执行成功：提交与执行结果均完成', () => {
    expect(of({ resultKind: 'sync' })).toEqual([
      { key: 'submit', status: 'finish' },
      { key: 'execute', status: 'finish' },
    ]);
  });

  it('任务已提交但仍在执行：执行阶段保持 process', () => {
    expect(of({ resultKind: 'task' })).toEqual([
      { key: 'submit', status: 'finish' },
      { key: 'execute', status: 'process' },
    ]);
  });

  it('任务查询为 running/pending：执行阶段保持 process（与未查询同形）', () => {
    expect(of({ resultKind: 'task', taskStatus: 'running' })).toEqual([
      { key: 'submit', status: 'finish' },
      { key: 'execute', status: 'process' },
    ]);
    expect(of({ resultKind: 'task', taskStatus: 'pending' })).toEqual([
      { key: 'submit', status: 'finish' },
      { key: 'execute', status: 'process' },
    ]);
  });

  it('任务查询为 completed：执行阶段落 finish', () => {
    expect(of({ resultKind: 'task', taskStatus: 'completed' })).toEqual([
      { key: 'submit', status: 'finish' },
      { key: 'execute', status: 'finish' },
    ]);
  });

  it('任务查询为 failed/cancelled：终态停在执行阶段 error', () => {
    expect(of({ resultKind: 'task', taskStatus: 'failed' })).toEqual([
      { key: 'submit', status: 'finish' },
      { key: 'execute', status: 'error' },
    ]);
    expect(of({ resultKind: 'task', taskStatus: 'cancelled' })).toEqual([
      { key: 'submit', status: 'finish' },
      { key: 'execute', status: 'error' },
    ]);
  });

  it('任务查询为 unknown（D3 软终态，结果未知）：非完成非失败也非进行中，按未决 wait 呈现', () => {
    expect(of({ resultKind: 'task', taskStatus: 'unknown' })).toEqual([
      { key: 'submit', status: 'finish' },
      { key: 'execute', status: 'wait' },
    ]);
  });

  it('taskStatus 不影响同步路径：sync 结果执行阶段仍为 finish', () => {
    expect(of({ resultKind: 'sync', taskStatus: 'failed' })).toEqual([
      { key: 'submit', status: 'finish' },
      { key: 'execute', status: 'finish' },
    ]);
  });

  it('已提交审批但尚未查询状态：审批进行中、执行结果待审批', () => {
    expect(of({ resultKind: 'approval' })).toEqual([
      { key: 'submit', status: 'finish' },
      { key: 'approval', status: 'process' },
      { key: 'execute', status: 'wait' },
    ]);
  });

  it('审批待处理：审批进行中、执行结果待审批', () => {
    expect(of({ resultKind: 'approval', approvalStatus: 'pending' })).toEqual([
      { key: 'submit', status: 'finish' },
      { key: 'approval', status: 'process' },
      { key: 'execute', status: 'wait' },
    ]);
  });

  it('审批通过且已同步续跑：审批与执行结果均完成', () => {
    expect(
      of({ resultKind: 'approval', approvalStatus: 'approved', continuationKind: 'sync' }),
    ).toEqual([
      { key: 'submit', status: 'finish' },
      { key: 'approval', status: 'finish' },
      { key: 'execute', status: 'finish' },
    ]);
  });

  it('审批通过但转任务执行：执行阶段保持 process', () => {
    expect(
      of({ resultKind: 'approval', approvalStatus: 'approved', continuationKind: 'task' }),
    ).toEqual([
      { key: 'submit', status: 'finish' },
      { key: 'approval', status: 'finish' },
      { key: 'execute', status: 'process' },
    ]);
  });

  it('审批通过但未标记续跑：执行阶段按已完成处理', () => {
    expect(of({ resultKind: 'approval', approvalStatus: 'approved' })).toEqual([
      { key: 'submit', status: 'finish' },
      { key: 'approval', status: 'finish' },
      { key: 'execute', status: 'finish' },
    ]);
  });

  it('审批被拒绝：审批阶段为 error，不推进执行阶段', () => {
    expect(of({ resultKind: 'approval', approvalStatus: 'rejected' })).toEqual([
      { key: 'submit', status: 'finish' },
      { key: 'approval', status: 'error' },
      { key: 'execute', status: 'wait' },
    ]);
  });

  it('审批已过期：同样视为审批阶段 error', () => {
    expect(of({ resultKind: 'approval', approvalStatus: 'expired' })).toEqual([
      { key: 'submit', status: 'finish' },
      { key: 'approval', status: 'error' },
      { key: 'execute', status: 'wait' },
    ]);
  });

  it('执行阶段错误（任务失败）：提交已成功，终态停在执行阶段 error', () => {
    expect(of({ resultKind: 'task', executeError: true })).toEqual([
      { key: 'submit', status: 'finish' },
      { key: 'execute', status: 'error' },
    ]);
  });

  it('执行阶段错误与提交失败互斥：hasError 优先，不渲染未到达的阶段', () => {
    expect(of({ hasError: true, resultKind: 'task', executeError: true })).toEqual([
      { key: 'submit', status: 'error' },
    ]);
  });

  it('executeError 缺省（undefined）：按任务执行中处理，不受影响', () => {
    expect(of({ resultKind: 'task' })).toEqual([
      { key: 'submit', status: 'finish' },
      { key: 'execute', status: 'process' },
    ]);
  });
});
