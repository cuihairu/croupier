/**
 * utils/apiError：extractApiErrorCode 从 ResponseError.data 提取稳定错误码。
 *
 * 响应契约（CLAUDE.md）：HTTP 状态表达错误类别，body.error 为 snake_case
 * 稳定码供前端分支；无结构化 body 时返回 ''（渲染层不得静默吞错）。
 */
import { extractApiErrorCode } from '../apiError';

describe('extractApiErrorCode', () => {
  it('data.error 为非空字符串时原样返回', () => {
    expect(extractApiErrorCode({ data: { error: 'executor_unbound', message: '未绑定' } })).toBe(
      'executor_unbound',
    );
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
