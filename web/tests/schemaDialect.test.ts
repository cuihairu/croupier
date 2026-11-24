import { classifySchemaSupport, isNullableSchema, primarySchemaType } from '@/utils/schemaDialect';

describe('schemaDialect (SUI-009 前端侧，与 Go ClassifySchemaSupport/schemaTypeFromObject 同规则)', () => {
  describe('primarySchemaType', () => {
    it('returns string type as-is', () => {
      expect(primarySchemaType({ type: 'string' })).toBe('string');
    });

    it('takes first non-null element of a type array', () => {
      expect(primarySchemaType({ type: ['string', 'null'] })).toBe('string');
      expect(primarySchemaType({ type: ['integer', 'string', 'null'] })).toBe('integer');
    });

    it('moves past a leading null (normalized form has null last, but tolerate any order)', () => {
      expect(primarySchemaType({ type: ['null', 'integer'] })).toBe('integer');
    });

    it('returns "null" for an all-null type array', () => {
      expect(primarySchemaType({ type: ['null'] })).toBe('null');
    });

    it('returns undefined when type is absent', () => {
      expect(primarySchemaType({})).toBeUndefined();
      expect(primarySchemaType(undefined)).toBeUndefined();
    });
  });

  describe('isNullableSchema', () => {
    it('detects explicit null in a type array', () => {
      expect(isNullableSchema({ type: ['string', 'null'] })).toBe(true);
      expect(isNullableSchema({ type: ['string'] })).toBe(false);
    });

    it('detects legacy 3.0 nullable: true', () => {
      expect(isNullableSchema({ type: 'string', nullable: true })).toBe(true);
      expect(isNullableSchema({ type: 'string', nullable: false })).toBe(false);
    });

    it('treats absent type + nullable as false', () => {
      expect(isNullableSchema({})).toBe(false);
      expect(isNullableSchema(undefined)).toBe(false);
    });
  });

  describe('classifySchemaSupport', () => {
    it('classifies a plain object schema as supported', () => {
      expect(
        classifySchemaSupport({ type: 'object', properties: { name: { type: 'string' } } }),
      ).toBe('supported');
      expect(classifySchemaSupport({ type: ['string', 'null'] })).toBe('supported');
    });

    it('classifies multi-type unions (excluding null) as degraded', () => {
      expect(classifySchemaSupport({ type: ['string', 'integer'] })).toBe('degraded');
      expect(classifySchemaSupport({ type: ['string', 'integer', 'null'] })).toBe('degraded');
      // null 不算多类型联合（["string","null"] 仍 supported）
      expect(classifySchemaSupport({ type: ['string', 'null'] })).toBe('supported');
    });

    it('classifies conditional schemas as degraded', () => {
      expect(classifySchemaSupport({ if: {}, then: {} })).toBe('degraded');
      expect(classifySchemaSupport({ dependentSchemas: {} })).toBe('degraded');
    });

    it('classifies tuples as degraded (prefixItems or items-array)', () => {
      expect(classifySchemaSupport({ type: 'array', prefixItems: [{ type: 'string' }] })).toBe(
        'degraded',
      );
      expect(classifySchemaSupport({ type: 'array', items: [{ type: 'string' }] })).toBe(
        'degraded',
      );
    });

    it('recurses into subschema positions', () => {
      expect(
        classifySchemaSupport({
          type: 'object',
          properties: { x: { type: ['string', 'integer'] } },
        }),
      ).toBe('degraded');
      expect(
        classifySchemaSupport({
          allOf: [{ type: 'object', properties: { y: { if: {}, then: {} } } }],
        }),
      ).toBe('degraded');
    });

    it('returns supported for non-schema values', () => {
      expect(classifySchemaSupport(null)).toBe('supported');
      expect(classifySchemaSupport(undefined)).toBe('supported');
      expect(classifySchemaSupport('string')).toBe('supported');
    });
  });
});
