/** T8/D2 执行边界前端覆盖：
 * 1. extractApiErrorCode：umi ResponseError 形态（.data.error）取稳定码，
 *    无结构化 body 返回 ''；
 * 2. executeErrorToastText：unbound 阻断给绑定指引文案，其余回退 fallback；
 * 3. ExecutorUnboundAlert：标题/描述/functionId + 「去绑定」跳 OpenAPI Sources；
 * 4. OperationPageRenderer：提交返回 409 executor_unbound → 结果区渲染
 *    「未绑定执行器」空态（不渲染通用失败 Result）；
 * 5. CompositeRenderer：区块执行 unbound 阻断 → 卡片内空态 + 空数据。 */
import React from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App } from 'antd';
import { history } from '@umijs/max';
import ExecutorUnboundAlert, {
  EXECUTOR_UNBOUND_CODE,
  executeErrorToastText,
} from '../ExecutorUnboundAlert';
import OperationPageRenderer from '../OperationPageRenderer';
import { CompositeRenderer } from '../CompositeRenderer';
import { extractApiErrorCode } from '@/utils/apiError';
import type {
  CompositeSection,
  OperationPageSpec,
  PageExecutionResult,
  PageFunctionBinding,
} from '@/types/dashboard';

// umi request 对非 2xx 抛 ResponseError：.data 为响应体（error/message 契约）
function responseError(status: number, body: Record<string, unknown>): Error {
  return Object.assign(new Error(`Request failed with status ${status}`), { data: body });
}
const unboundError = () =>
  responseError(409, { error: 'executor_unbound', message: '函数尚未绑定运行时执行器' });

const pushMock = history.push as unknown as jest.Mock;

// SchemaFormRenderer 替身：渲染提交按钮直接触发 onFinish（同 OperationPageRenderer.test）
jest.mock('@/components/SchemaFormRenderer', () => ({
  __esModule: true,
  default: jest.fn(({ onFinish }: { onFinish: (v: Record<string, unknown>) => void }) => (
    <button type="button" data-testid="form-submit" onClick={() => onFinish({ amount: 10 })}>
      form-submit
    </button>
  )),
}));

beforeEach(() => {
  pushMock.mockClear();
});

describe('extractApiErrorCode（稳定码提取）', () => {
  it('ResponseError 形态取 data.error', () => {
    expect(extractApiErrorCode(unboundError())).toBe('executor_unbound');
    expect(extractApiErrorCode(responseError(409, { error: 'binding_stale' }))).toBe(
      'binding_stale',
    );
  });

  it('无结构化 body：普通 Error / 非 Error / 空 code 均返回空串', () => {
    expect(extractApiErrorCode(new Error('boom'))).toBe('');
    expect(extractApiErrorCode('string')).toBe('');
    expect(extractApiErrorCode(null)).toBe('');
    expect(extractApiErrorCode(responseError(500, { message: 'no code' }))).toBe('');
    expect(extractApiErrorCode(responseError(400, { error: '' }))).toBe('');
  });
});

describe('executeErrorToastText（unbound toast 文案）', () => {
  it('unbound 阻断 → 绑定指引文案', () => {
    expect(executeErrorToastText(unboundError(), '操作失败')).toBe(
      '函数尚未绑定运行时执行器，请先完成绑定',
    );
  });

  it('其余错误回退 fallback', () => {
    expect(executeErrorToastText(new Error('boom'), '操作失败')).toBe('操作失败');
  });
});

describe('ExecutorUnboundAlert（结构化空态）', () => {
  it('渲染标题/描述/functionId；「去绑定」跳 OpenAPI Sources', () => {
    render(<ExecutorUnboundAlert functionId="player.get" />);
    expect(screen.getByText('未绑定执行器')).toBeInTheDocument();
    expect(screen.getByText(/尚未绑定运行时执行器，执行已被阻断/)).toBeInTheDocument();
    expect(screen.getByText('player.get')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: '去绑定' }));
    expect(pushMock).toHaveBeenCalledWith('/functions/openapi-sources');
  });

  it('functionId 缺省：不渲染 code 片段', () => {
    render(<ExecutorUnboundAlert />);
    expect(screen.queryByText('player.get')).not.toBeInTheDocument();
  });
});

describe('OperationPageRenderer unbound 错误态（T8）', () => {
  const spec: OperationPageSpec = { form: { jsonSchema: { type: 'object' } } };
  const bindings: PageFunctionBinding[] = [
    { id: 'b-act', functionId: 'fn-upload', usage: 'action', execution: { mode: 'sync' } },
  ];

  it('提交返回 409 executor_unbound → 「未绑定执行器」空态替代通用失败 Result', async () => {
    const onExecute = jest.fn().mockRejectedValue(unboundError());
    render(
      <App>
        <OperationPageRenderer
          spec={spec}
          bindings={bindings}
          onExecute={onExecute as never as () => Promise<PageExecutionResult>}
        />
      </App>,
    );
    fireEvent.click(screen.getByTestId('form-submit'));
    await waitFor(() => expect(screen.getByText('未绑定执行器')).toBeInTheDocument());
    // 引导文案与函数定位；「去绑定」入口在位
    expect(screen.getByText('fn-upload')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '去绑定' })).toBeInTheDocument();
    // 不渲染通用失败 Result 的 subTitle（服务端 message 不作为空态主体）
    expect(screen.queryByText('Request failed with status 409')).not.toBeInTheDocument();
  });

  it('普通错误不受影响：仍渲染通用失败 Result（无空态组件）', async () => {
    const onExecute = jest.fn().mockRejectedValue(new Error('backend boom'));
    render(
      <App>
        <OperationPageRenderer
          spec={spec}
          bindings={bindings}
          onExecute={onExecute as never as () => Promise<PageExecutionResult>}
        />
      </App>,
    );
    fireEvent.click(screen.getByTestId('form-submit'));
    await waitFor(() => expect(screen.getByText('backend boom')).toBeInTheDocument());
    expect(screen.queryByText('未绑定执行器')).not.toBeInTheDocument();
  });
});

describe('CompositeRenderer unbound 区块空态（T8）', () => {
  it('区块执行被 409 阻断 → 卡片内「未绑定执行器」+ 空数据（无伪数据兜底）', async () => {
    const sections: CompositeSection[] = [
      { key: 'secA', bindingId: 'b-a', view: 'fields', title: { 'zh-CN': '查询区块' } },
    ];
    const bindings: PageFunctionBinding[] = [
      { id: 'b-a', functionId: 'player.get', usage: 'query', execution: { mode: 'sync' } },
    ];
    const onExecute = jest.fn().mockRejectedValue(unboundError());
    render(
      <App>
        <CompositeRenderer
          sections={sections}
          bindings={bindings}
          onExecute={onExecute as never}
          preview={false}
        />
      </App>,
    );
    // 非 autoRun 非 actions/toolbar：卡片 extra 渲染「执行」按钮（antd zh 双字间空格容忍）
    fireEvent.click(await screen.findByRole('button', { name: /执\s*行/ }));
    await waitFor(() => expect(screen.getByText('未绑定执行器')).toBeInTheDocument());
    expect(screen.getByText('player.get')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '去绑定' })).toBeInTheDocument();
  });
});
