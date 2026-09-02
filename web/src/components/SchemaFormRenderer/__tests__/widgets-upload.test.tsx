/**
 * F4 验收测试：Upload 系与 KeyValue widget
 */
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import SchemaFormRenderer from '@/components/SchemaFormRenderer';
import { uploadValueFromFileList } from '@/components/SchemaFormRenderer/widgets-upload';
import type { FormPresentationSpec, JSONSchema } from '@/types/dashboard';

const schemaOf = (value: Record<string, unknown>): JSONSchema => value as unknown as JSONSchema;

describe('F4: uploadValueFromFileList 值归一', () => {
  const done = (url: string) => ({ uid: url, name: 'f', url, status: 'done' });
  const uploading = { uid: 'u', name: 'f', status: 'uploading' };
  const errored = { uid: 'e', name: 'f', status: 'error' };

  test('仅收集 done 文件的 URL；单值取首个，多值收集数组', () => {
    expect(uploadValueFromFileList([done('http://x/a.png')] as never, false)).toBe(
      'http://x/a.png',
    );
    expect(
      uploadValueFromFileList([done('http://x/a.png'), done('http://x/b.png')] as never, true),
    ).toEqual(['http://x/a.png', 'http://x/b.png']);
    expect(uploadValueFromFileList([uploading, errored] as never, true)).toEqual([]);
  });

  test('response.url / response 字符串 兜底；空 done 列表单值为空串', () => {
    expect(
      uploadValueFromFileList(
        [{ uid: 'r', name: 'f', status: 'done', response: { url: 'http://r/1' } }] as never,
        false,
      ),
    ).toBe('http://r/1');
    expect(
      uploadValueFromFileList(
        [{ uid: 's', name: 'f', status: 'done', response: 'http://s/2' }] as never,
        false,
      ),
    ).toBe('http://s/2');
    expect(uploadValueFromFileList([{ uid: 'd', name: 'f', status: 'done' }] as never, false)).toBe(
      '',
    );
  });
});

describe('F4: KeyValue widget', () => {
  test('渲染既有键值对、编辑值、新增行、删除行', async () => {
    const spec: FormPresentationSpec = {
      jsonSchema: {
        type: 'object',
        properties: { extra: { type: 'object', title: '扩展信息' } },
        required: ['extra'],
      },
      fields: [{ key: 'extra', widget: 'KeyValue' }],
    };
    const onFinish = jest.fn();
    const { container } = render(
      <SchemaFormRenderer
        spec={spec}
        initialValues={{ extra: { env: 'prod' } }}
        onFinish={onFinish}
      />,
    );
    // 既有行渲染
    const keyInput = screen.getByDisplayValue('env');
    const valueInput = screen.getByDisplayValue('prod');
    expect(keyInput).toBeTruthy();
    expect(valueInput).toBeTruthy();
    // 编辑值
    fireEvent.change(valueInput, { target: { value: 'prod2' } });
    // 新增行并填写
    fireEvent.click(screen.getByTestId('root_extra-add'));
    const rows = container.querySelectorAll('[data-testid^="root_extra-row-"]');
    expect(rows.length).toBe(2);
    const newRow = rows[1];
    const inputs = newRow.querySelectorAll('input');
    fireEvent.change(inputs[0], { target: { value: 'k1' } });
    fireEvent.change(inputs[1], { target: { value: 'v1' } });
    fireEvent.click(screen.getByRole('button', { name: /提\s*交/ }));
    await waitFor(() =>
      expect(onFinish).toHaveBeenCalledWith(
        expect.objectContaining({ extra: { env: 'prod2', k1: 'v1' } }),
      ),
    );

    // 删除首行
    onFinish.mockClear();
    fireEvent.click(
      within(container.querySelector('[data-testid="root_extra-row-0"]')!).getByText('删除'),
    );
    fireEvent.click(screen.getByRole('button', { name: /提\s*交/ }));
    await waitFor(() =>
      expect(onFinish).toHaveBeenCalledWith(expect.objectContaining({ extra: { k1: 'v1' } })),
    );
  });

  test('空 key 的行不计入提交对象', async () => {
    const spec: FormPresentationSpec = {
      jsonSchema: { type: 'object', properties: { extra: { type: 'object' } } },
      fields: [{ key: 'extra', widget: 'KeyValue' }],
    };
    const onFinish = jest.fn();
    render(<SchemaFormRenderer spec={spec} onFinish={onFinish} />);
    fireEvent.click(screen.getByTestId('root_extra-add'));
    const valueInput = screen.getByPlaceholderText('值');
    fireEvent.change(valueInput, { target: { value: 'orphan' } });
    fireEvent.click(screen.getByRole('button', { name: /提\s*交/ }));
    await waitFor(() =>
      expect(onFinish).toHaveBeenCalledWith(expect.objectContaining({ extra: {} })),
    );
  });
});

describe('F4: Upload widget', () => {
  test('渲染既有 URL 为 done 列表，移除后值清空（单值）', async () => {
    const spec: FormPresentationSpec = {
      jsonSchema: {
        type: 'object',
        properties: { logo: { type: 'string', title: '图标' } },
      },
      fields: [{ key: 'logo', widget: 'Upload', widgetProps: { action: '/upload' } }],
    };
    const onFinish = jest.fn();
    render(
      <SchemaFormRenderer
        spec={spec}
        initialValues={{ logo: 'http://x/a.png' }}
        onFinish={onFinish}
      />,
    );
    // 列表显示既有文件名
    expect(screen.getByText('a.png')).toBeTruthy();
    // 点击移除
    const remove = document.querySelector('.ant-upload-list-item-action .anticon-delete');
    expect(remove).toBeTruthy();
    fireEvent.click(remove as Element);
    fireEvent.click(screen.getByRole('button', { name: /提\s*交/ }));
    await waitFor(() =>
      expect(onFinish).toHaveBeenCalledWith(expect.objectContaining({ logo: '' })),
    );
  });
});
