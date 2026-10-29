/**
 * utils/apiError：extractApiErrorCode 从 ResponseError.data 提取稳定错误码。
 *
 * 响应契约（CLAUDE.md）：HTTP 状态表达错误类别，body.error 为 snake_case
 * 稳定码供前端分支；无结构化 body 时返回 ''（渲染层不得静默吞错）。
 */
import { extractApiErrorCode, isResultUnknownError } from '../apiError';

describe('extractApiErrorCode', () => {
  it('data.error 为非空字符串时原样返回', () => {
    expect(extractApiErrorCode({ data: { error: 'executor_unbound', message: '未绑定' } })).toBe(
      'executor_unbound',
    );
  });

  it('response.data.error（封装路径）同样提取', () => {
    expect(extractApiErrorCode({ response: { data: { error: 'scope_not_authorized' } } })).toBe(
      'scope_not_authorized',
    );
  });

  it('response.data 优先于 data（与 utils/errors 同口径）', () => {
    expect(
      extractApiErrorCode({
        data: { error: 'from_data' },
        response: { data: { error: 'from_response' } },
      }),
    ).toBe('from_response');
  });

  it('data.error 为空串/非字符串时返回空串', () => {
    expect(extractApiErrorCode({ data: { error: '' } })).toBe('');
    expect(extractApiErrorCode({ data: { error: 409 } })).toBe('');
    expect(extractApiErrorCode({ data: { error: null } })).toBe('');
  });

  it('data 缺失/非对象时返回空串', () => {
    expect(extractApiErrorCode({})).toBe('');
    expect(extractApiErrorCode({ data: 'raw text' })).toBe('');
    expect(extractApiErrorCode({ data: null })).toBe('');
  });

  it('非对象输入（null/undefined/原始值/网络 Error）返回空串', () => {
    expect(extractApiErrorCode(null)).toBe('');
    expect(extractApiErrorCode(undefined)).toBe('');
    expect(extractApiErrorCode('boom')).toBe('');
    expect(extractApiErrorCode(new Error('boom'))).toBe('');
  });
});

describe('isResultUnknownError', () => {
  it('传输层超时/网络中断判为结果未知', () => {
    expect(isResultUnknownError(new Error('timeout of 5000ms exceeded'))).toBe(true);
    expect(isResultUnknownError(new TypeError('Failed to fetch'))).toBe(true);
    expect(isResultUnknownError(new Error('Network Error'))).toBe(true);
  });

  it('AbortError 判为结果未知', () => {
    const abort = new Error('aborted');
    abort.name = 'AbortError';
    expect(isResultUnknownError(abort)).toBe(true);
  });

  it('服务端已应答（ResponseError 带 .data）即使文案含 timeout 也不属未知', () => {
    const resp = new Error('Request timeout') as Error & { data: Record<string, unknown> };
    resp.data = { error: 'gateway_timeout', message: 'Request timeout' };
    expect(isResultUnknownError(resp)).toBe(false);
  });

  it('本地业务错误/普通 Error/非对象输入不属结果未知', () => {
    expect(isResultUnknownError(new Error('executor not bound'))).toBe(false);
    expect(isResultUnknownError({ data: { error: 'validation_failed' } })).toBe(false);
    expect(isResultUnknownError(null)).toBe(false);
    expect(isResultUnknownError('boom')).toBe(false);
  });
});
