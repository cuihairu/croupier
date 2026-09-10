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

  it('变量名修改（失焦提交）后序列化为新 key', async () => {
    const onChange = jest.fn();
    render(<ConstantFieldsEditor value={initial} onChange={onChange} />);
    await screen.findByDisplayValue('环境');
    const input = screen.getByDisplayValue('env');
    fireEvent.change(input, { target: { value: 'envKey' } });
    // 输入阶段不提交（草稿），失焦校验通过才落 schema
    expect(onChange).not.toHaveBeenCalled();
    fireEvent.blur(input);
    await waitFor(() => expect(onChange).toHaveBeenCalled());
    const schema = JSON.parse(onChange.mock.lastCall?.[0] as string);
    expect(schema.properties.envKey).toBeDefined();
    expect(schema.properties.env).toBeUndefined();
  });

  it('变量名重复：即时警示 + 失焦拒绝提交（不落 schema，回退原值）', async () => {
    const onChange = jest.fn();
    render(<ConstantFieldsEditor value={initial} onChange={onChange} />);
    await screen.findByDisplayValue('环境');
    const input = screen.getByDisplayValue('env');
    fireEvent.change(input, { target: { value: 'currency' } }); // 与另一行重复
    expect(await screen.findByText(/变量名不能为空、且不能与其他常量重复/)).toBeInTheDocument();
    fireEvent.blur(input);
    await waitFor(() => expect(screen.queryByText(/变量名不能为空/)).not.toBeInTheDocument());
    expect(onChange).not.toHaveBeenCalled(); // 非法值拒绝提交
    expect(screen.getByDisplayValue('env')).toBeInTheDocument(); // 回退原值
  });

  it('变量名清空：失焦拒绝提交（非空校验）', async () => {
    const onChange = jest.fn();
    render(<ConstantFieldsEditor value={initial} onChange={onChange} />);
    await screen.findByDisplayValue('环境');
    const input = screen.getByDisplayValue('env');
    fireEvent.change(input, { target: { value: '  ' } });
    fireEvent.blur(input);
    expect(onChange).not.toHaveBeenCalled();
    expect(screen.getByDisplayValue('env')).toBeInTheDocument();
  });

  it('连续输入不丢焦点（行 key 稳定，草稿不逐键提交）', async () => {
    render(<ConstantFieldsEditor value={initial} onChange={jest.fn()} />);
    await screen.findByDisplayValue('环境');
    const input = screen.getByDisplayValue('env');
    input.focus();
    fireEvent.change(input, { target: { value: 'e' } });
    fireEvent.change(input, { target: { value: 'en' } });
    fireEvent.change(input, { target: { value: 'env2' } });
    // 修复前：逐键提交改名 → React key 变化 → 行 remount 丢焦点
    expect(document.activeElement).toBe(input);
    expect((input as HTMLInputElement).value).toBe('env2');
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
