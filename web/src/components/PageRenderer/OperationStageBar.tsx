/** D1 多阶段状态反馈的阶段条：只读派生 + 本地化标题的共享展示组件。
 *
 * 从 OperationPageRenderer 的内联 Steps 抽出，供操作页 / 复合编排 / 任务页
 * 共用同一阶段序列与标题（describeOperationStages + 本地化文案）。
 * 输入为空序列时渲染 null（不占位、不显示空条）。
 *
 * 阶段条为追加展示，不改变任何提交流程与结果判定，故无 PageSpec 字段变更。
 */

import React from 'react';
import { Steps } from 'antd';
import { useIntl } from '@umijs/max';
import { describeOperationStages } from './operationStages';
import type { OperationStageInput, OperationStageKey } from './operationStages';

/** 多阶段反馈的阶段标题文案（D1 状态反馈 Steps）：阶段标识 → 本地化消息描述符。 */
const STAGE_TITLE_MESSAGE: Record<OperationStageKey, { id: string; defaultMessage: string }> = {
  submit: {
    id: 'component.pageRenderer.operationPage.stage.submit',
    defaultMessage: '提交操作',
  },
  approval: {
    id: 'component.pageRenderer.operationPage.stage.approval',
    defaultMessage: '审批',
  },
  execute: {
    id: 'component.pageRenderer.operationPage.stage.execute',
    defaultMessage: '执行',
  },
};

export interface OperationStageBarProps {
  /** 与 describeOperationStages 同源的只读输入 */
  input: OperationStageInput;
}

const OperationStageBar: React.FC<OperationStageBarProps> = ({ input }) => {
  const intl = useIntl();
  const stages = describeOperationStages(input);
  if (stages.length === 0) return null;
  return (
    <Steps
      size="small"
      style={{ marginBottom: 16 }}
      items={stages.map((stage) => ({
        title: intl.formatMessage(STAGE_TITLE_MESSAGE[stage.key]),
        status: stage.status,
      }))}
    />
  );
};

export default OperationStageBar;
