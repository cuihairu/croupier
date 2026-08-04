import vectors from '../../testdata/dashboard_selector_vectors.json';

import type {
  InputAssignment,
  OutputAssignment,
  SelectorAST,
  ValueSource,
} from '@/types/dashboard';

type SelectorVector = {
  selector?: SelectorAST;
  outputAssignments?: OutputAssignment[];
  wantValid?: boolean;
  wantOutputValid?: boolean;
};

const selectorVectors = vectors as SelectorVector[];

function assertValueSourceShape(source: ValueSource) {
  expect(source).toHaveProperty('kind');
  expect(source).not.toHaveProperty('type');
  if (source.kind === 'page_state' && source.path !== undefined) {
    expect(source.path.startsWith('/')).toBe(true);
  }
  if (source.kind !== 'literal' && source.kind !== 'page_state') {
    expect(source.path?.startsWith('/')).toBe(true);
  }
}

describe('dashboard selector contract', () => {
  it('共享测试向量使用 ValueSource.kind 和 JSON Pointer', () => {
    const assignments = selectorVectors
      .filter((vector) => vector.wantValid === true)
      .flatMap((vector) => vector.selector?.assignments ?? []);

    expect(assignments.length).toBeGreaterThan(0);
    for (const assignment of assignments as InputAssignment[]) {
      expect(assignment.target.startsWith('/')).toBe(true);
      assertValueSourceShape(assignment.source);
    }
  });

  it('共享测试向量使用独立 OutputAssignment', () => {
    const outputs = selectorVectors
      .filter((vector) => vector.wantOutputValid === true)
      .flatMap((vector) => vector.outputAssignments ?? []);

    expect(outputs.length).toBeGreaterThan(0);
    for (const output of outputs) {
      expect(output.stateKey).toBeTruthy();
      expect(output.source.startsWith('/')).toBe(true);
      expect(['scalar', 'object', 'collection', 'task', 'dataset']).toContain(output.shape);
      expect(output).not.toHaveProperty('assignments');
    }
  });
});
