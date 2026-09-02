import { render, screen } from '@testing-library/react';
import SchemaFormRenderer from '@/components/SchemaFormRenderer';
import { deriveRuntimeSchema } from '@/components/SchemaFormRenderer';
import type { FormPresentationSpec, JSONSchema } from '@/types/dashboard';

test('debug', () => {
  const spec: FormPresentationSpec = {
    jsonSchema: { type: 'object', properties: { extra: { type: 'object', title: '扩展' } } },
    fields: [{ key: 'extra', widget: 'KeyValue' }],
  } as unknown as FormPresentationSpec;
  const { uiSchema } = deriveRuntimeSchema(spec, {});
  console.log('UISHEMA', JSON.stringify(uiSchema));
  const { container } = render(<SchemaFormRenderer spec={spec} onFinish={jest.fn()} />);
  console.log('HTML', container.innerHTML.slice(0, 2500));
  console.log('HAS add btn?', !!screen.queryByTestId('root_extra-add'));
});
