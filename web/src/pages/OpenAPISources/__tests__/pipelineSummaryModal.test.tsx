/** PipelineSummaryModal（T7/D4 上传即成页摘要）覆盖：
 * 1. 四项管线计数渲染（解析操作/新建未绑定契约/更新组件模板/生成页面提案）；
 * 2. 解析诊断透传（warning → Alert description）；
 * 3. CTA：「打开编辑器」跳组合页编辑器、「查看提案」跳提案收件箱，
 *    均先 onClose 关闭；右上角 X 仅关闭不跳转；
 * 4. summary 为 null 时 Modal 不打开。 */
import React from 'react';
import { fireEvent, render, screen } from '@testing-library/react';
import { history } from '@umijs/max';
import PipelineSummaryModal from '../PipelineSummaryModal';
import type { OpenAPISourcePipelineSummary } from '@/services/api/openapi';

const pushMock = history.push as unknown as jest.Mock;

const summary: OpenAPISourcePipelineSummary = {
  operations: 12,
  contractsCreated: 3,
  templatesUpdated: 5,
  proposalsCreated: 2,
  diagnostics: [
    { code: 'operation_extension_unknown', severity: 'warning', message: '未知扩展将被忽略' },
    // severity 全三级 + field 定位后缀（Alert message 形如 code @ field）
    { code: 'pipeline_guard', severity: 'error', field: 'paths./mail', message: '超阈值' },
    { code: 'hint_only', severity: 'info', message: '仅提示' },
  ],
};

function renderModal(onClose = jest.fn(), s: OpenAPISourcePipelineSummary | null = summary) {
  render(<PipelineSummaryModal summary={s} onClose={onClose} />);
  return { onClose };
}

beforeEach(() => {
  pushMock.mockClear();
});

describe('PipelineSummaryModal 上传即成页摘要', () => {
  it('四项计数渲染 + 诊断透传', () => {
    renderModal();
    expect(screen.getByText('解析操作：12')).toBeInTheDocument();
    expect(screen.getByText('新建未绑定契约：3')).toBeInTheDocument();
    expect(screen.getByText('更新组件模板：5')).toBeInTheDocument();
    expect(screen.getByText('生成页面提案：2')).toBeInTheDocument();
    expect(screen.getByText('operation_extension_unknown')).toBeInTheDocument();
    expect(screen.getByText('未知扩展将被忽略')).toBeInTheDocument();
    // error 级带 field 定位、info 级无 field：三级 severity 各走对应 Alert type
    expect(screen.getByText('pipeline_guard @ paths./mail')).toBeInTheDocument();
    expect(screen.getByText('超阈值')).toBeInTheDocument();
    expect(screen.getByText('hint_only')).toBeInTheDocument();
  });

  it('无诊断时不渲染 Alert 区块', () => {
    renderModal(jest.fn(), {
      operations: 1,
      contractsCreated: 0,
      templatesUpdated: 0,
      proposalsCreated: 0,
    });
    expect(screen.queryByText('operation_extension_unknown')).not.toBeInTheDocument();
  });

  it('CTA：打开编辑器 → /functions/pages/composite-editor（先 onClose）', () => {
    const { onClose } = renderModal();
    fireEvent.click(screen.getByRole('button', { name: '打开编辑器' }));
    expect(onClose).toHaveBeenCalledTimes(1);
    expect(pushMock.mock.calls).toEqual([['/functions/pages/composite-editor']]);
  });

  it('CTA：查看提案 → /functions/pages（提案收件箱）', () => {
    const { onClose } = renderModal();
    fireEvent.click(screen.getByRole('button', { name: '查看提案' }));
    expect(onClose).toHaveBeenCalledTimes(1);
    expect(pushMock.mock.calls).toEqual([['/functions/pages']]);
  });

  it('右上角关闭：仅 onClose，不跳转', () => {
    const { onClose } = renderModal();
    fireEvent.click(screen.getByRole('button', { name: 'Close' }));
    expect(onClose).toHaveBeenCalledTimes(1);
    expect(pushMock).not.toHaveBeenCalled();
  });

  it('summary 为 null：Modal 不打开', () => {
    renderModal(jest.fn(), null);
    expect(screen.queryByText('解析操作：12')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: '打开编辑器' })).not.toBeInTheDocument();
  });
});
