/** OperationStageBar 共享阶段条（D1 状态反馈）：只读派生 + 本地化标题的展示组件。
 *
 * 覆盖：空输入不渲染、提交在途、提交失败、同步/任务成功、审批路径、
 * 执行阶段错误（任务失败/取消）各形态的阶段标题与 antd Steps item 状态类。
 */
import React from 'react';
import { render, screen } from '@testing-library/react';
import { App } from 'antd';
import OperationStageBar from '../OperationStageBar';
import type { OperationStageInput } from '../operationStages';

const base: OperationStageInput = {
  loading: false,
  hasError: false,
  resultKind: null,
  approvalStatus: null,
  continuationKind: null,
};

function renderBar(input: Partial<OperationStageInput> = {}) {
  return render(
    <App>
      <OperationStageBar input={{ ...base, ...input }} />
    </App>,
  );
}

const stage = (title: string): HTMLElement =>
  screen.getByText(title).closest('.ant-steps-item') as HTMLElement;

const expectStage = (title: string, status: string) => {
  expect(stage(title).className).toContain(`ant-steps-item-${status}`);
};

describe('OperationStageBar', () => {
  it('空输入：不渲染阶段条', () => {
    renderBar();
    expect(screen.queryByText('提交操作')).not.toBeInTheDocument();
    expect(screen.queryByText('审批')).not.toBeInTheDocument();
    expect(screen.queryByText('执行')).not.toBeInTheDocument();
  });

  it('提交在途：只有提交阶段 process', () => {
    renderBar({ loading: true });
    expectStage('提交操作', 'process');
    expect(screen.queryByText('审批')).not.toBeInTheDocument();
    expect(screen.queryByText('执行')).not.toBeInTheDocument();
  });

  it('提交失败：终态停在提交阶段 error', () => {
    renderBar({ hasError: true });
    expectStage('提交操作', 'error');
  });

  it('同步执行成功：提交与执行均 finish', () => {
    renderBar({ resultKind: 'sync' });
    expectStage('提交操作', 'finish');
    expectStage('执行', 'finish');
  });

  it('任务执行中：提交 finish、执行 process', () => {
    renderBar({ resultKind: 'task' });
    expectStage('提交操作', 'finish');
    expectStage('执行', 'process');
  });

  it('执行阶段错误（任务失败/取消）：提交 finish、执行 error', () => {
    renderBar({ resultKind: 'task', executeError: true });
    expectStage('提交操作', 'finish');
    expectStage('执行', 'error');
    expect(screen.queryByText('审批')).not.toBeInTheDocument();
  });

  it('审批待处理：提交 finish、审批 process、执行 wait', () => {
    renderBar({ resultKind: 'approval', approvalStatus: 'pending' });
    expectStage('提交操作', 'finish');
    expectStage('审批', 'process');
    expectStage('执行', 'wait');
  });
});
