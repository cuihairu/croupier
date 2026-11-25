/**
 * SUI-025 共享预期结果：与 internal/dashboard/generator/form_derivation_parity_test.go
 * 读取同一 fixture（testdata/form_presentation_parity.json），断言 Web 调试页与
 * Go 生成器默认表单派生一致（顶层字段顺序、控件、分组、layout hints）。
 */
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { derivePresentationSpec } from '@/utils/schemaHints';
import type { JSONSchema } from '@/types/dashboard';

interface ParityCase {
  name: string;
  schema: Record<string, unknown>;
  expected: {
    layout?: string;
    fields?: Array<Record<string, unknown>>;
    groups?: Array<Record<string, unknown>>;
  };
}

interface ParityFixture {
  cases: ParityCase[];
}

const fixturePath = resolve(__dirname, '../../../../testdata/form_presentation_parity.json');
const fixture = JSON.parse(readFileSync(fixturePath, 'utf8')) as ParityFixture;

describe('form presentation parity fixture (SUI-025)', () => {
  test('fixture contains parity cases', () => {
    expect(fixture.cases.length).toBeGreaterThan(0);
  });

  for (const parityCase of fixture.cases) {
    test(`case ${parityCase.name}`, () => {
      const spec = derivePresentationSpec(parityCase.schema as JSONSchema);
      expect({
        layout: spec.layout,
        fields: spec.fields,
        groups: spec.groups,
      }).toEqual(parityCase.expected);
    });
  }
});
