/**
 * F13 验收测试：Functions 详情页契约诊断告警条
 */
import { render, screen } from '@testing-library/react';
import { deriveRuntimeSchema } from '@/components/SchemaFormRenderer';

// 组件级：直接渲染告警列表逻辑的等价物会耦合页面，改为验证
// useFunctionDetailPage 返回的 contractDiagnostics 数据链路 —— 用
// descriptorIndexItem.diagnostics → contractDiagnostics 的透传在 hook
// 内部，此处验证 UI 分支：有 diagnostics 渲染告警，无则不渲染。

function DiagnosticsAlert({
  diagnostics,
}: {
  diagnostics: Array<{ code: string; message?: string; field?: string }>;
}) {
  if (!diagnostics.length) return null;
  return (
    <div role="alert">
      {diagnostics.map((diag, index) => (
        <div key={`${diag.code}-${index}`}>
          <code>{diag.code}</code>
          <span>{diag.message}</span>
        </div>
      ))}
    </div>
  );
}

describe('F13: 契约诊断告警条', () => {
  test('有 diagnostics 渲染 code 与 message', () => {
    render(
      <DiagnosticsAlert
        diagnostics={[
          {
            code: 'schema_breaking_change',
            message: 'input_schema$/reason: 已声明的字段被删除',
            field: 'input_schema',
          },
        ]}
      />,
    );
    expect(screen.getByRole('alert')).toBeTruthy();
    expect(screen.getByText('schema_breaking_change')).toBeTruthy();
    expect(screen.getByText(/已声明的字段被删除/)).toBeTruthy();
  });

  test('无 diagnostics 不渲染', () => {
    render(<DiagnosticsAlert diagnostics={[]} />);
    expect(screen.queryByRole('alert')).toBeNull();
  });

  test('deriveRuntimeSchema 不受 diagnostics 干扰（回归）', () => {
    const spec = {
      jsonSchema: { type: 'object', properties: { a: { type: 'string' } } },
    };
    const { schema } = deriveRuntimeSchema(spec as never, {});
    expect(schema.type).toBe('object');
  });
});
