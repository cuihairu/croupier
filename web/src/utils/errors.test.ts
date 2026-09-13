import {
  extractErrorCode,
  extractErrorDetails,
  extractErrorMessage,
  formatErrorDetails,
  hasErrorDetails,
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

describe('extractErrorMessage 优先级矩阵（追加）', () => {
  it('data.message 最优先', () => {
    expect(extractErrorMessage({ data: { message: 'from-data' } }, 'fb')).toBe('from-data');
  });

  it('info.data.message 次之', () => {
    expect(extractErrorMessage({ info: { data: { message: 'from-info' } } }, 'fb')).toBe(
      'from-info',
    );
  });

  it('message 缺失时回落 response.data.error', () => {
    expect(extractErrorMessage({ response: { data: { error: 'stable_code' } } }, 'fb')).toBe(
      'stable_code',
    );
  });

  it('空白 / 非字符串候选全部跳过 → fallback', () => {
    expect(extractErrorMessage({ data: { message: '   ' }, message: 42 }, 'fb')).toBe('fb');
    expect(extractErrorMessage('', 'fb')).toBe('fb');
  });
});

describe('extractErrorCode 分支补充（追加）', () => {
  it('error 字段为空串 / 非字符串 → undefined', () => {
    expect(extractErrorCode({ response: { data: { error: '' } } })).toBeUndefined();
    expect(extractErrorCode({ response: { data: { error: 500 } } })).toBeUndefined();
  });

  it('response.data 非对象时依次回退 data / info.data', () => {
    expect(extractErrorCode({ response: { data: 'oops' }, data: { error: 'from_data' } })).toBe(
      'from_data',
    );
    expect(extractErrorCode({ data: 'oops', info: { data: { error: 'from_info' } } })).toBe(
      'from_info',
    );
    expect(
      extractErrorCode({ response: { data: 'oops' }, data: 'oops2', info: { data: 'x' } }),
    ).toBeUndefined();
    expect(extractErrorCode({})).toBeUndefined();
  });
});

describe('extractErrorDetails 分支补充（追加）', () => {
  it('数组项非对象跳过；field/message 非字符串归空（双空丢弃、单边保留）', () => {
    const err = {
      response: {
        data: {
          details: [
            null,
            'plain',
            { field: 1, message: 2 },
            { field: 'onlyField' },
            { message: 'onlyMessage' },
          ],
        },
      },
    };
    expect(extractErrorDetails(err)).toEqual([
      { field: 'onlyField', message: '' },
      { field: '', message: 'onlyMessage' },
    ]);
  });

  it('map 值非字符串 → JSON.stringify', () => {
    const err = { response: { data: { details: { count: 3, nested: { a: 1 } } } } };
    expect(extractErrorDetails(err)).toEqual([
      { field: 'count', message: '3' },
      { field: 'nested', message: '{"a":1}' },
    ]);
  });

  it('details 为 null / 空数组 / 空对象 → []', () => {
    expect(extractErrorDetails({ response: { data: { details: null } } })).toEqual([]);
    expect(extractErrorDetails({ response: { data: { details: [] } } })).toEqual([]);
    expect(extractErrorDetails({ response: { data: { details: {} } } })).toEqual([]);
  });
});

describe('formatErrorDetails 分支补充（追加）', () => {
  it('无 field 仅输出 message；自定义分隔符', () => {
    const err = {
      response: { data: { details: [{ message: 'm1' }, { field: 'f', message: 'm2' }] } },
    };
    expect(formatErrorDetails(err, '; ')).toBe('m1; f: m2');
  });

  it('无明细 → 空串', () => {
    expect(formatErrorDetails({})).toBe('');
  });
});

describe('hasErrorDetails（追加）', () => {
  it('有明细 true / 无明细 false', () => {
    expect(hasErrorDetails({ response: { data: { details: { a: 'b' } } } })).toBe(true);
    expect(hasErrorDetails({ response: { data: { message: 'x' } } })).toBe(false);
    expect(hasErrorDetails(undefined)).toBe(false);
  });
});
