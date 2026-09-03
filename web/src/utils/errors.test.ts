import {
  extractErrorCode,
  extractErrorDetails,
  extractErrorMessage,
  formatErrorDetails,
  isMfaRequiredError,
} from './errors';

const validationError = {
  response: {
    status: 422,
    data: {
      error: 'validation_failed',
      message: 'page validation failed',
      details: {
        'bindings[0].selectors.input./id': 'required field not assigned',
        'bindings[0].selectors.input./patch': 'target field not found in schema',
      },
    },
  },
};

describe('extractErrorMessage', () => {
  it('reads message from response.data', () => {
    expect(extractErrorMessage(validationError, 'fallback')).toBe('page validation failed');
  });

  it('falls back on unknown shapes', () => {
    expect(extractErrorMessage(null, 'fallback')).toBe('fallback');
    expect(extractErrorMessage(new Error('boom'), 'fallback')).toBe('boom');
  });
});

describe('extractErrorDetails', () => {
  it('flattens map-style details into field/message pairs', () => {
    expect(extractErrorDetails(validationError)).toEqual([
      { field: 'bindings[0].selectors.input./id', message: 'required field not assigned' },
      { field: 'bindings[0].selectors.input./patch', message: 'target field not found in schema' },
    ]);
  });

  it('supports array-style details', () => {
    const err = {
      response: {
        data: { error: 'validation_failed', details: [{ field: 'gameId', message: '不能为空' }] },
      },
    };
    expect(extractErrorDetails(err)).toEqual([{ field: 'gameId', message: '不能为空' }]);
  });

  it('returns empty when details missing or non-object', () => {
    expect(extractErrorDetails({})).toEqual([]);
    expect(extractErrorDetails({ response: { data: { details: 'oops' } } })).toEqual([]);
  });
});

describe('formatErrorDetails', () => {
  it('joins field: message lines', () => {
    expect(formatErrorDetails(validationError)).toBe(
      [
        'bindings[0].selectors.input./id: required field not assigned',
        'bindings[0].selectors.input./patch: target field not found in schema',
      ].join('\n'),
    );
  });
});

describe('extractErrorCode / isMfaRequiredError', () => {
  it('提取 response.data.error 稳定码', () => {
    const error = {
      response: { status: 401, data: { error: 'mfa_required', message: '需要动态验证码' } },
    };
    expect(extractErrorCode(error)).toBe('mfa_required');
    expect(isMfaRequiredError(error)).toBe(true);
  });

  it('data / info.data 回退路径', () => {
    expect(extractErrorCode({ data: { error: 'rate_limited' } })).toBe('rate_limited');
    expect(extractErrorCode({ info: { data: { error: 'forbidden' } } })).toBe('forbidden');
  });

  it('非对象 / 无 error 字段返回 undefined', () => {
    expect(extractErrorCode(undefined)).toBeUndefined();
    expect(extractErrorCode('boom')).toBeUndefined();
    expect(extractErrorCode({ message: 'no code here' })).toBeUndefined();
  });

  it('isMfaRequiredError 对其它错误码返回 false', () => {
    expect(isMfaRequiredError({ response: { data: { error: 'unauthorized' } } })).toBe(false);
    expect(isMfaRequiredError(new Error('network down'))).toBe(false);
  });
});
