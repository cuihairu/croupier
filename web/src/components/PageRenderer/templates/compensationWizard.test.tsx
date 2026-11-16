/** CompensationWizard 固定分步表单（D1 StepsForm/Steps/Progress 接入模板面）覆盖。 */

import React from 'react';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import CompensationWizard from './compensationWizard';
import { createMailCompensationDemoExecute } from './mailCompensation';
import type { PageExecuteFn } from '@/types/dashboard';

// 该套件在模板目录并行跑时因 antd StepsForm 渲染较重，单用例在高负载下
// 可能逼近 jest 默认 5s 超时；放宽到套件级阈值避免 CI 抖动误报（同目录惯例）。
jest.setTimeout(30000);

function activeStep() {
  const root = document.querySelector('.ant-pro-steps-form-step-active');
  if (!root) throw new Error('active step not found');
  return within(root as HTMLElement);
}

function nextButton() {
  return screen.getByRole('button', { name: /下\s*一\s*步/ });
}

function submitButton() {
  return screen.getByRole('button', { name: /提\s*交/ });
}

async function fillMailSteps(title = 'S3 赛季维护补偿') {
  const first = activeStep();
  fireEvent.change(first.getByLabelText('标题/事由'), { target: { value: title } });
  fireEvent.click(nextButton());
  await waitFor(() => expect(activeStep().getAllByText('发放范围')[0]).toBeInTheDocument());
  fireEvent.click(nextButton());
  await waitFor(() => expect(activeStep().getByRole('checkbox')).toBeInTheDocument());
  fireEvent.click(activeStep().getByRole('checkbox'));
  fireEvent.click(submitButton());
}

describe('CompensationWizard 固定分步配置', () => {
  it('三步流程：必填拦截、合并上一步值提交、反馈 Steps/Progress 成功态', async () => {
    const onCreate = jest.fn();
    const execute: PageExecuteFn = jest.fn(createMailCompensationDemoExecute());
    render(<CompensationWizard execute={execute} onCreate={onCreate} />);

    expect(screen.getByText('任务信息')).toBeInTheDocument();
    expect(screen.getByText('目标与数量')).toBeInTheDocument();
    expect(screen.getByText('检查并提交')).toBeInTheDocument();

    // 第一步必填标题为空时下一步不前进
    fireEvent.click(nextButton());
    await waitFor(() => expect(screen.getByText('请输入标题/事由')).toBeInTheDocument());

    await fillMailSteps('S3 赛季维护补偿');
    await waitFor(() => expect(onCreate).toHaveBeenCalledTimes(1));

    expect(execute).toHaveBeenCalledWith(
      'create',
      expect.objectContaining({
        form: expect.objectContaining({
          type: 'mail',
          title: 'S3 赛季维护补偿',
          receiverRange: 'all',
          confirm: true,
        }),
      }),
    );
    expect(screen.getByText('任务已提交')).toBeInTheDocument();
    expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '100');
    expect(document.querySelector('.ant-steps-item-finish')).toBeInTheDocument();
  });

  it('补偿路径：待审批状态反馈与进行中的 Steps/Progress', async () => {
    const execute: PageExecuteFn = createMailCompensationDemoExecute();
    render(<CompensationWizard execute={execute} />);

    const first = activeStep();
    fireEvent.change(first.getByLabelText('标题/事由'), { target: { value: '事故补偿' } });
    fireEvent.click(first.getByRole('radio', { name: '补偿' }));
    fireEvent.click(nextButton());

    await waitFor(() => expect(activeStep().getAllByText('发放范围')[0]).toBeInTheDocument());
    fireEvent.change(activeStep().getByLabelText('每人数量（金）'), { target: { value: '100' } });
    fireEvent.click(nextButton());

    await waitFor(() => expect(activeStep().getByRole('checkbox')).toBeInTheDocument());
    fireEvent.click(activeStep().getByRole('checkbox'));
    fireEvent.click(submitButton());

    await waitFor(() =>
      expect(screen.getByText('已提交审批，等待审批通过后执行')).toBeInTheDocument(),
    );
    expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '70');
    expect(document.querySelector('.ant-steps-item-process')).toBeInTheDocument();
  });

  it('执行失败：提交错误反馈、进度异常、不触发 onCreate', async () => {
    const onCreate = jest.fn();
    const execute: PageExecuteFn = async () => {
      throw new Error('service_down');
    };
    render(<CompensationWizard execute={execute} onCreate={onCreate} />);

    await fillMailSteps('失败任务');
    await waitFor(() => expect(screen.getByText('service_down')).toBeInTheDocument());
    expect(onCreate).not.toHaveBeenCalled();
    expect(screen.getByText('service_down').closest('.ant-alert')).toHaveTextContent(
      'service_down',
    );
    expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '100');
    expect(document.querySelector('.ant-progress-status-exception')).toBeInTheDocument();
  });

  // GUX-011：executor 未绑定是「不可用」而非普通失败——ExecutorUnboundAlert
  // 取代错误 Alert，不出现提交失败标题与重试按钮
  it('executor 未绑定：以 ExecutorUnboundAlert 呈现不可用', async () => {
    const onCreate = jest.fn();
    const execute: PageExecuteFn = async () => {
      throw Object.assign(new Error('Request failed with status 409'), {
        data: { error: 'executor_unbound' },
      });
    };
    render(
      <CompensationWizard
        execute={execute}
        createFunctionId="mail_task.create"
        onCreate={onCreate}
      />,
    );

    await fillMailSteps('未绑定任务');
    await waitFor(() => expect(screen.getByText('未绑定执行器')).toBeInTheDocument());
    expect(screen.getByText('mail_task.create')).toBeInTheDocument();
    expect(screen.queryByText('提交失败')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /重\s*新\s*提\s*交/ })).not.toBeInTheDocument();
    expect(onCreate).not.toHaveBeenCalled();
  });

  // GUX-011：目标步骤即时汇总——指定玩家选中人数即时可见，不杜撰服务端人数
  it('指定玩家目标即时汇总已选人数', async () => {
    render(<CompensationWizard execute={createMailCompensationDemoExecute()} />);

    const first = activeStep();
    fireEvent.change(first.getByLabelText('标题/事由'), { target: { value: '定向补偿' } });
    fireEvent.click(first.getByRole('radio', { name: '补偿' }));
    fireEvent.click(nextButton());

    await waitFor(() => expect(activeStep().getAllByText('发放范围')[0]).toBeInTheDocument());
    expect(activeStep().getByText('全服发放：实际目标人数以服务端为准')).toBeInTheDocument();
    fireEvent.click(activeStep().getByRole('radio', { name: '指定玩家' }));
    fireEvent.mouseDown(activeStep().getByRole('combobox'));
    await waitFor(() =>
      expect(
        document.querySelectorAll('.ant-select-dropdown .ant-select-item-option').length,
      ).toBeGreaterThan(0),
    );
    const options = document.querySelectorAll('.ant-select-dropdown .ant-select-item-option');
    fireEvent.click(options[0]);
    fireEvent.click(options[1]);
    await waitFor(() => expect(screen.getByText('已选 2 名玩家')).toBeInTheDocument());
    expect(screen.queryByText('全服发放：实际目标人数以服务端为准')).not.toBeInTheDocument();
  });

  // GUX-012/013：补偿路径提交前核对摘要（类型/标题/范围/数量/审批提示）+
  // 审批回执（任务 ID + 审批单号）
  it('补偿路径：提交前核对摘要与审批回执', async () => {
    const onCreate = jest.fn();
    const execute: PageExecuteFn = jest.fn(createMailCompensationDemoExecute());
    render(
      <CompensationWizard
        execute={execute}
        createFunctionId="mail_task.create"
        onCreate={onCreate}
      />,
    );

    const first = activeStep();
    fireEvent.change(first.getByLabelText('标题/事由'), { target: { value: '事故补偿' } });
    fireEvent.click(first.getByRole('radio', { name: '补偿' }));
    fireEvent.click(nextButton());

    await waitFor(() => expect(activeStep().getAllByText('发放范围')[0]).toBeInTheDocument());
    fireEvent.change(activeStep().getByLabelText('每人数量（金）'), { target: { value: '100' } });
    fireEvent.click(nextButton());

    await waitFor(() => expect(screen.getByText('提交前核对')).toBeInTheDocument());
    expect(screen.getByText('事故补偿')).toBeInTheDocument();
    expect(screen.getAllByText('全服').length).toBeGreaterThan(0);
    expect(screen.getByText('每人数量')).toBeInTheDocument();
    expect(screen.getByText('100')).toBeInTheDocument();
    expect(screen.getByText('补偿任务提交后进入审批，审批通过后才会执行')).toBeInTheDocument();
    fireEvent.click(activeStep().getByRole('checkbox'));
    fireEvent.click(submitButton());

    await waitFor(() =>
      expect(screen.getByText('已提交审批，等待审批通过后执行')).toBeInTheDocument(),
    );
    expect(screen.getByText('任务 ID')).toBeInTheDocument();
    expect(screen.getByText('审批单号')).toBeInTheDocument();
    expect(screen.getAllByText(/task-demo-\d+/).length).toBeGreaterThan(0);
    expect(screen.getAllByText(/approval-/).length).toBeGreaterThan(0);
    expect(onCreate).toHaveBeenCalledTimes(1);
  });

  // GUX-013：邮件路径成功回执只带任务 ID（无审批单号，接口未返回不展示）
  it('邮件路径成功回执展示任务 ID', async () => {
    const execute: PageExecuteFn = jest.fn(createMailCompensationDemoExecute());
    render(<CompensationWizard execute={execute} />);

    await fillMailSteps('回执核对');
    await waitFor(() => expect(screen.getByText('任务已提交')).toBeInTheDocument());
    expect(screen.getByText('任务 ID')).toBeInTheDocument();
    expect(screen.getAllByText(/task-demo-\d+/).length).toBeGreaterThan(0);
    expect(screen.queryByText('审批单号')).not.toBeInTheDocument();
  });
});
