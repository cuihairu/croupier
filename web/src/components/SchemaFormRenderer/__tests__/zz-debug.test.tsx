import { fireEvent, render, screen } from '@testing-library/react';
import SchemaFormRenderer from '@/components/SchemaFormRenderer';
import type { FormPresentationSpec } from '@/types/dashboard';

// [skip] KeyValue 定制实现未就绪（rjsf object 走 ui:field），调试草稿保留待 F4 完成后处理。
test.skip('debug2', async () => {
  const spec: FormPresentationSpec = {
    jsonSchema: { type: 'object', properties: { extra: { type: 'object' } } },
    fields: [{ key: 'extra', widget: 'KeyValue' }],
  } as unknown as FormPresentationSpec;
  const onFinish = jest.fn();
  const { container } = render(<SchemaFormRenderer spec={spec} onFinish={onFinish} />);
  console.log('FULL_HTML', container.innerHTML);
  fireEvent.click(screen.getByRole('button', { name: /添\s*加/ }));
  console.log('AFTER_ADD_HTML', container.innerHTML.slice(0, 1200));
  const inputs = container.querySelectorAll('input');
  console.log('INPUT_COUNT', inputs.length);
  fireEvent.click(screen.getByRole('button', { name: /提\s*交/ }));
  await Promise.resolve();
  console.log('ONFINISH', JSON.stringify(onFinish.mock.calls));
});
