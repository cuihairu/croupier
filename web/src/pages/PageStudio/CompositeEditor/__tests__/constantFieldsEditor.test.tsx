import React from 'react';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import ConstantFieldsEditor from '../ConstantFieldsEditor';

describe('ConstantFieldsEditor（实例字段编辑：显示名/变量名/选项/删除）', () => {
  const initial = JSON.stringify({
    type: 'object',
    properties: {
      env: { type: 'string', title: '环境', enum: ['prod', 'stage'] },
      currency: { type: 'string', title: '货币', enum: ['gold', 'diamond'] },
    },
  });

  it('渲染常量字段并支持显示名修改（序列化保留 enum）', async () => {
    const onChange = jest.fn();
    render(<ConstantFieldsEditor value={initial} onChange={onChange} />);
    expect(await screen.findByDisplayValue('环境')).toBeInTheDocument();
    expect(screen.getByDisplayValue('货币')).toBeInTheDocument();

    fireEvent.change(screen.getByDisplayValue('环境'), { target: { value: '部署环境' } });
    await waitFor(() => expect(onChange).toHaveBeenCalled());
    const schema = JSON.parse(onChange.mock.lastCall?.[0] as string);
    expect(schema.properties.env.title).toBe('部署环境');
    expect(schema.properties.env.enum).toEqual(['prod', 'stage']);
  });

  it('变量名修改后序列化为新 key', async () => {
    const onChange = jest.fn();
    render(<ConstantFieldsEditor value={initial} onChange={onChange} />);
    await screen.findByDisplayValue('环境');
    fireEvent.change(screen.getByDisplayValue('env'), { target: { value: 'envKey' } });
    await waitFor(() => expect(onChange).toHaveBeenCalled());
    const schema = JSON.parse(onChange.mock.lastCall?.[0] as string);
    expect(schema.properties.envKey).toBeDefined();
    expect(schema.properties.env).toBeUndefined();
  });

  it('删除常量字段', async () => {
    const onChange = jest.fn();
    render(<ConstantFieldsEditor value={initial} onChange={onChange} />);
    await screen.findByDisplayValue('环境');
    const delBtns = screen
      .getAllByRole('button')
      .filter((b) => b.querySelector('[aria-label="delete"]'));
    expect(delBtns.length).toBeGreaterThanOrEqual(2);
    delBtns[0].click();
    await waitFor(() => expect(onChange).toHaveBeenCalled());
    const schema = JSON.parse(onChange.mock.lastCall?.[0] as string);
    expect(schema.properties.currency).toBeDefined();
    expect(schema.properties.env).toBeUndefined();
  });
});
