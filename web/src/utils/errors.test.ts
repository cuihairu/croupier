import { extractErrorDetails, extractErrorMessage, formatErrorDetails } from './errors';

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
