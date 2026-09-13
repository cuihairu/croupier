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

  it('添加常量：默认行 + 重名自增', async () => {
    const onChange = jest.fn();
    render(
      <ConstantFieldsEditor
        value={JSON.stringify({
          type: 'object',
          properties: { 新常量: { type: 'string', title: '新常量', enum: ['x'] } },
        })}
        onChange={onChange}
      />,
    );
    fireEvent.click(screen.getByRole('button', { name: /添加常量/ }));
    await waitFor(() => expect(onChange).toHaveBeenCalled());
    const schema = JSON.parse(onChange.mock.lastCall?.[0] as string);
    // 已有「新常量」→ 新增行为「新常量2」，带默认选项
    expect(schema.properties['新常量2']).toBeDefined();
  });

  it('空态提示（无常量）', () => {
    render(
      <ConstantFieldsEditor
        value={JSON.stringify({ type: 'object', properties: {} })}
        onChange={jest.fn()}
      />,
    );
    expect(screen.getByText(/暂无常量。导入常量请到组件库 Tab/)).toBeInTheDocument();
  });

  it('回车提交变量名改名', async () => {
    const onChange = jest.fn();
    render(<ConstantFieldsEditor value={initial} onChange={onChange} />);
    await screen.findByDisplayValue('环境');
    const input = screen.getByDisplayValue('env');
    fireEvent.change(input, { target: { value: 'envKey' } });
    fireEvent.keyDown(input, { key: 'Enter' });
    await waitFor(() => expect(onChange).toHaveBeenCalled());
    const schema = JSON.parse(onChange.mock.lastCall?.[0] as string);
    expect(schema.properties.envKey).toBeDefined();
  });

  it('选项编辑：label≠value 折叠格式与「值|标签」解析', async () => {
    const onChange = jest.fn();
    render(<ConstantFieldsEditor value={initial} onChange={onChange} />);
    await screen.findByDisplayValue('环境');
    const area = screen.getAllByPlaceholderText(
      '每行一个选项：值 或 值|标签',
    )[0] as HTMLTextAreaElement;
    // enum 形态（label===value）折叠为纯值
    expect(area.value).toBe('prod\nstage');
    fireEvent.change(area, { target: { value: 'p|生产\nstage' } });
    await waitFor(() => expect(onChange).toHaveBeenCalled());
    expect(onChange.mock.lastCall?.[0]).toContain('"p"');
    expect(onChange.mock.lastCall?.[0]).toContain('生产');
  });

  it('JSON 高级面板：展开/双向编辑/收起', async () => {
    const onChange = jest.fn();
    render(<ConstantFieldsEditor value={initial} onChange={onChange} />);
    await screen.findByDisplayValue('环境');
    fireEvent.click(screen.getByRole('button', { name: 'JSON' }));
    const jsonArea = screen.getByDisplayValue(initial) as HTMLTextAreaElement;
    expect(screen.getByText('JSON Schema（高级，双向同步）')).toBeInTheDocument();
    fireEvent.change(jsonArea, { target: { value: '{"type":"object"}' } });
    expect(onChange).toHaveBeenCalledWith('{"type":"object"}');
    fireEvent.click(screen.getByRole('button', { name: '收起 JSON' }));
    expect(screen.queryByText('JSON Schema（高级，双向同步）')).not.toBeInTheDocument();
  });

  it('未修改直接失焦：无草稿不提交（?? 空侧）', async () => {
    const onChange = jest.fn();
    render(<ConstantFieldsEditor value={initial} onChange={onChange} />);
    await screen.findByDisplayValue('环境');
    fireEvent.blur(screen.getByDisplayValue('env'));
    expect(onChange).not.toHaveBeenCalled();
  });

  it('变量名空串：引用提示回退「（未设置）」', async () => {
    render(
      <ConstantFieldsEditor
        value={JSON.stringify({
          type: 'object',
          properties: { '': { type: 'string', title: '空名', enum: ['x'] } },
        })}
        onChange={jest.fn()}
      />,
    );
    await screen.findByDisplayValue('空名');
    expect(screen.getByText(/下游引用变量名：（未设置）/)).toBeInTheDocument();
  });

  it('value undefined：空字段 + JSON 面板空串（?? 空侧）', async () => {
    render(<ConstantFieldsEditor value={undefined} onChange={jest.fn()} />);
    expect(screen.getByText(/暂无常量/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'JSON' }));
    // 空字段时唯一的 textarea 即高级 JSON 面板，值兜底空串
    const jsonArea = screen.getByRole('textbox') as HTMLTextAreaElement;
    expect(jsonArea.value).toBe('');
  });

  it('label 空串折叠纯值；label≠值 展开为「值|标签」形态', async () => {
    render(
      <ConstantFieldsEditor
        value={JSON.stringify({
          type: 'object',
          properties: {
            f: { type: 'string', title: '字段', enum: ['x'], enumNames: [''] },
            g: { type: 'string', title: '字段2', enum: ['y'], enumNames: ['演示'] },
          },
        })}
        onChange={jest.fn()}
      />,
    );
    await screen.findByDisplayValue('字段');
    const areas = screen.getAllByPlaceholderText('每行一个选项：值 或 值|标签');
    // label 空串：折叠为纯值
    expect((areas[0] as HTMLTextAreaElement).value).toBe('x');
    // label 存在且 ≠ 值：渲染为「值|标签」
    expect((areas[1] as HTMLTextAreaElement).value).toBe('y|演示');
  });
});
