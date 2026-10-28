/** CompensationWizard 固定分步表单（D1 StepsForm/Steps/Progress 接入模板面）覆盖。 */

import React from 'react';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import CompensationWizard from './compensationWizard';
import { createMailCompensationDemoExecute } from './mailCompensation';
import type { PageExecuteFn } from '@/types/dashboard';

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
});
