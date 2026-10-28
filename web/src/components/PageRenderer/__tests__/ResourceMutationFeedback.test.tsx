/** ResourceMutationFeedback 写操作反馈（D1 状态反馈切片）。
 *
 * 覆盖流程进度（Steps 阶段 + Progress 状态）与结果面（Alert/Result）的
 * running / success / error 三形态，验证作为 ResourcePageRenderer 模板面
 * 写操作反馈的复用契约。
 */
import React from 'react';
import { fireEvent, render, screen } from '@testing-library/react';
import { App } from 'antd';
import ResourceMutationFeedback from '../ResourceMutationFeedback';

function renderFeedback(state: 'running' | 'success' | 'error') {
  return render(
    <App>
      <ResourceMutationFeedback state={state} onClose={() => undefined} />
    </App>,
  );
}

describe('ResourceMutationFeedback', () => {
  it('success：关闭提示触发 onClose 回调', () => {
    const onClose = jest.fn();
    const { container } = render(
      <App>
        <ResourceMutationFeedback state="success" onClose={onClose} />
      </App>,
    );
    fireEvent.click(container.querySelector('.ant-alert-close-icon') as HTMLButtonElement);
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('running：Steps 提交阶段进行中 + Progress active + 防重复提示', () => {
    renderFeedback('running');
    expect(screen.getByText('提交变更').closest('.ant-steps-item')?.className).toContain(
      'ant-steps-item-process',
    );
    expect(screen.getByText('同步列表').closest('.ant-steps-item')?.className).toContain(
      'ant-steps-item-wait',
    );
    expect(screen.getByTestId('resource-mutation-progress').className).toContain(
      'ant-progress-status-active',
    );
    expect(screen.getByText('资源操作正在执行，请勿重复提交')).toBeInTheDocument();
  });

  it('success：Steps 全部完成 + Progress success + Result 成功面', () => {
    renderFeedback('success');
    expect(screen.getByText('提交变更').closest('.ant-steps-item')?.className).toContain(
      'ant-steps-item-finish',
    );
    expect(screen.getByText('同步列表').closest('.ant-steps-item')?.className).toContain(
      'ant-steps-item-finish',
    );
    expect(screen.getByTestId('resource-mutation-progress').className).toContain(
      'ant-progress-status-success',
    );
    expect(screen.getByText('资源操作已完成')).toBeInTheDocument();
    expect(screen.getByText('反馈会保留到下一次操作，可关闭')).toBeInTheDocument();
  });

  it('error：Steps 提交阶段 error + Progress exception + Result 失败面 + Alert 可关', () => {
    renderFeedback('error');
    expect(screen.getByText('提交变更').closest('.ant-steps-item')?.className).toContain(
      'ant-steps-item-error',
    );
    expect(screen.getByTestId('resource-mutation-progress').className).toContain(
      'ant-progress-status-exception',
    );
    expect(screen.getByText('资源操作未完成')).toBeInTheDocument();
    expect(screen.getByText(/本次变更未写入/)).toBeInTheDocument();
  });
});
