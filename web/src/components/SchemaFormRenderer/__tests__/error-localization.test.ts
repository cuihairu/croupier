/**
 * F6 验收测试：AJV 校验错误消息本地化
 */
import { localizeFormErrors } from '@/components/SchemaFormRenderer';
import type { RJSFValidationError } from '@rjsf/utils';
import type { RJSFSchema } from '@rjsf/utils';

const schema: RJSFSchema = {
  type: 'object',
  properties: {
    playerId: { type: 'string', title: '玩家' },
    reason: { type: 'string' },
  },
  required: ['playerId'],
};

const err = (
  name: string,
  property: string,
  params?: Record<string, unknown>,
): RJSFValidationError =>
  ({ name, property, params, message: 'raw ajv message' }) as RJSFValidationError;

describe('F6: localizeFormErrors', () => {
  test('zh-CN：required 带字段 title 插值', () => {
    const [out] = localizeFormErrors(
      [err('required', '', { missingProperty: 'playerId' })],
      schema,
      'zh-CN',
    );
    expect(out.message).toBe('「玩家」为必填项');
  });

  test('zh-CN：title 缺失的字段回退 key（humanize 已在派生时兜底，此处直接用 key）', () => {
    const [out] = localizeFormErrors([err('minLength', '.reason', { limit: 3 })], schema, 'zh-CN');
    expect(out.message).toBe('至少需要 3 个字符');
  });

  test('en-US：英文模板', () => {
    const [out] = localizeFormErrors(
      [err('required', '', { missingProperty: 'playerId' })],
      schema,
      'en-US',
    );
    expect(out.message).toBe('"玩家" is required');
  });

  test('format/enum/pattern 与未知关键字', () => {
    const [format, enumE, pattern, unknown] = localizeFormErrors(
      [
        err('format', '.playerId', { format: 'email' }),
        err('enum', '.playerId', { allowedValues: ['a', 'b'] }),
        err('pattern', '.reason'),
        err('someNewKeyword', '.reason'),
      ],
      schema,
      'zh-CN',
    );
    expect(format.message).toBe('格式不正确（email）');
    expect(enumE.message).toBe('可选值：a、b');
    expect(pattern.message).toBe('格式不正确');
    expect(unknown.message).toBe('raw ajv message');
  });

  test('nested property 取叶子字段 title', () => {
    const nestedSchema: RJSFSchema = {
      type: 'object',
      properties: {
        address: {
          type: 'object',
          properties: { city: { type: 'string', title: '城市' } },
        },
      },
    };
    const [out] = localizeFormErrors(
      [err('required', '.address', { missingProperty: 'city' })],
      nestedSchema,
      'zh-CN',
    );
    expect(out.message).toBe('「城市」为必填项');
  });
});
