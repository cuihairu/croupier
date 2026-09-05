import React from 'react';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import StaticSchemaEditor from '../StaticSchemaEditor';

describe('StaticSchemaEditor（常量导入与字段管理）', () => {
  it('添加常量 → 默认字段出现', async () => {
    render(<StaticSchemaEditor value={undefined} onChange={jest.fn()} />);
    fireEvent.click(screen.getByText('添加常量'));
    await waitFor(() =>
      expect(screen.getAllByDisplayValue('新常量').length).toBeGreaterThanOrEqual(2),
    );
  });

  it('显示名/变量名可编辑并序列化回 schema', async () => {
    const initial = JSON.stringify({
      type: 'object',
      properties: { env: { type: 'string', title: '环境', enum: ['prod', 'stage'] } },
    });
    const onChange = jest.fn();
    render(<StaticSchemaEditor value={initial} onChange={onChange} />);
    expect(await screen.findByDisplayValue('环境')).toBeInTheDocument();

    const titleInput = screen.getByDisplayValue('环境') as HTMLInputElement;
    fireEvent.change(titleInput, { target: { value: '部署环境' } });
    await waitFor(() => expect(onChange).toHaveBeenCalled());
    const schema = JSON.parse(onChange.mock.lastCall?.[0] as string);
    expect(schema.properties.env.title).toBe('部署环境');
    expect(schema.properties.env.enum).toEqual(['prod', 'stage']);
  });

  it('删除常量字段', async () => {
    const onChange = jest.fn();
    const initial = JSON.stringify({
      type: 'object',
      properties: {
        env: { type: 'string', title: '环境', enum: ['prod'] },
        currency: { type: 'string', title: '货币', enum: ['gold'] },
      },
    });
    render(<StaticSchemaEditor value={initial} onChange={onChange} />);
    await screen.findByDisplayValue('环境');
    const delBtns = screen
      .getAllByRole('button')
      .filter((b) => b.querySelector('[aria-label="delete"]'));
    delBtns[0].click();
    await waitFor(() => expect(onChange).toHaveBeenCalled());
    const schema = JSON.parse(onChange.mock.lastCall?.[0] as string);
    expect(Object.keys(schema.properties)).toEqual(['currency']);
  });
});
