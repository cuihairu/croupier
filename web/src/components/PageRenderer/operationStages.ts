/** 操作页多阶段状态反馈（D1 状态反馈 Steps）。
 *
 * 只读派生：按当前状态机把「提交 → 审批 → 执行结果」压成可扫读的阶段序列，
 * 与既有 Result/Alert 反馈同源同语义（审批待处理与任务已提交都不算业务执行成功），
 * 不重复渲染审批单号、任务号与失败原因——那些仍由结果卡内的 Alert/Result 承载。
 *
 * 阶段条为追加展示，不改变任何提交流程与结果判定，故无 PageSpec 字段变更。
 */

/** 阶段状态，取值与 antd Steps 的 status 一致。 */
export type OperationStageStatus = 'wait' | 'process' | 'finish' | 'error';

/** 阶段标识：稳定 id，调用方据此取本地化标题。 */
export type OperationStageKey = 'submit' | 'approval' | 'execute';

/** 单个阶段：标识 + 状态。 */
export interface OperationStage {
  key: OperationStageKey;
  status: OperationStageStatus;
}

export interface OperationStageInput {
  /** 提交或审批确认在途 */
  loading: boolean;
  /** 提交失败（执行器抛错或返回错误结果体） */
  hasError: boolean;
  /** 执行结果类别；空 = 尚未有结果 */
  resultKind: 'sync' | 'approval' | 'task' | null;
  /** 审批状态；空 = 尚未查询 */
  approvalStatus: 'pending' | 'approved' | 'rejected' | 'expired' | null;
  /** 审批通过后的续跑类别（仅 approvalStatus.continuation 为真时有意义） */
  continuationKind: 'sync' | 'task' | null;
  /** 执行阶段失败（任务失败/取消）——提交已成功、终态停在执行阶段 error */
  executeError?: boolean;
}

/** 派生阶段序列。
 *
 * - 在途：只有一阶段 process，后续阶段不可知；
 * - 失败：终态停在提交阶段 error，不渲染后续阶段（操作未走到那一步）；
 * - 执行阶段错误（executeError）：提交已成功、终态停在执行阶段 error（任务页取消/失败）；
 * - 审批路径：审批阶段随审批状态推进，执行阶段在审批通过前保持 wait，
 *   通过后按续跑类别落成 finish（同步执行）或 process（转任务）；
 * - 任务路径：任务已提交但仍在执行，执行阶段保持 process。
 */
export function describeOperationStages(input: OperationStageInput): OperationStage[] {
  const submit: OperationStage = {
    key: 'submit',
    status: input.loading ? 'process' : input.hasError ? 'error' : 'finish',
  };

  if (input.loading) return [submit];
  if (input.hasError) return [submit];

  const { resultKind, approvalStatus, continuationKind, executeError } = input;
  if (!resultKind) return [];
  // 执行阶段终态错误：提交已成功，停在执行阶段 error，不渲染审批阶段。
  if (executeError) {
    return [submit, { key: 'execute', status: 'error' }];
  }

  if (resultKind === 'approval') {
    return [
      submit,
      {
        key: 'approval',
        status:
          approvalStatus === 'rejected' || approvalStatus === 'expired'
            ? 'error'
            : approvalStatus === 'approved'
              ? 'finish'
              : 'process',
      },
      {
        key: 'execute',
        status:
          approvalStatus === 'approved'
            ? continuationKind === 'task'
              ? 'process'
              : 'finish'
            : 'wait',
      },
    ];
  }

  return [submit, { key: 'execute', status: resultKind === 'task' ? 'process' : 'finish' }];
}
