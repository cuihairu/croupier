/** LocalizedTextEditor 覆盖（T13：回归「默认名称 + 可选翻译」语义）：
 * 默认语言缺失给出不阻断发布的补录提示；仅默认语言有值时无任何提示；
 * 下拉标记回归单一 ✓（无非必填 ⚠）；契约提示不再表述双必填/发布被拒。
 * 注：tests/setupTests.jsx 的 FormattedMessage mock 只渲染 defaultMessage，
 * 断言避开 {placeholder} 插值片段。 */
import React from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import LocalizedTextEditor from '.';

/** 打开 🌐 气泡（唯一无文案按钮），等待契约提示渲染 */
const openPopover = async () => {
  fireEvent.click(screen.getByRole('button'));
  await waitFor(() => {
    if (!document.body.textContent?.includes('BCP47')) throw new Error('气泡未渲染');
  });
};

/** 打开 locale 下拉，返回全部选项元素 */
const openLocaleDropdown = async () => {
  fireEvent.mouseDown(screen.getByRole('combobox'));
  await waitFor(() => {
    if (document.querySelectorAll('.ant-select-item-option').length === 0)
      throw new Error('下拉选项未渲染');
  });
  return Array.from(document.querySelectorAll('.ant-select-item-option'));
};

describe('LocalizedTextEditor：默认语言提示语义（T13）', () => {
  it('默认语言缺失：气泡提示补录，不再表述必填/无法发布', async () => {
    render(<LocalizedTextEditor value={{ 'en-US': 'Players' }} onChange={jest.fn()} />);
    await openPopover();
    expect(document.body.textContent).toContain('尚未填写默认语言');
    expect(document.body.textContent).not.toContain('必填语言');
    expect(document.body.textContent).not.toContain('无法发布');
  });

  it('仅默认语言有值：无任何缺失提示', async () => {
    render(<LocalizedTextEditor value={{ 'zh-CN': '玩家' }} onChange={jest.fn()} />);
    await openPopover();
    expect(document.body.textContent).not.toContain('尚未填写默认语言');
  });

  it('契约提示：只要求任一语言非空，非默认语言为可选翻译', async () => {
    render(<LocalizedTextEditor value={{ 'zh-CN': '玩家' }} onChange={jest.fn()} />);
    await openPopover();
    expect(document.body.textContent).toContain('发布只要求任一语言非空');
    expect(document.body.textContent).not.toContain('缺失时发布会被拒');
  });

  it('下拉标记：已录 ✓，缺失语言（含默认语言之外全部）不再标 ⚠', async () => {
    render(<LocalizedTextEditor value={{ 'zh-CN': '玩家' }} onChange={jest.fn()} />);
    const options = await openLocaleDropdown();
    expect(options.length).toBeGreaterThan(0);
    const zhCN = options.find((o) => o.textContent?.includes('zh-CN'));
    expect(zhCN?.textContent).toContain('✓');
    expect(options.every((o) => !o.textContent?.includes('⚠'))).toBe(true);
  });
});

describe('LocalizedTextEditor：编辑与自定义 locale', () => {
  it('输入当前 locale 文案：onChange 合并并保留其他语言', () => {
    const onChange = jest.fn();
    render(
      <LocalizedTextEditor value={{ 'zh-CN': '玩家', 'en-US': 'Players' }} onChange={onChange} />,
    );
    fireEvent.change(screen.getByDisplayValue('玩家'), { target: { value: '玩家们' } });
    expect(onChange).toHaveBeenCalledWith({ 'zh-CN': '玩家们', 'en-US': 'Players' });
  });

  it('切换 locale 后输入定位到对应语言文案', async () => {
    const onChange = jest.fn();
    render(
      <LocalizedTextEditor value={{ 'zh-CN': '玩家', 'en-US': 'Players' }} onChange={onChange} />,
    );
    const options = await openLocaleDropdown();
    fireEvent.click(options.find((o) => o.textContent?.includes('en-US')) as HTMLElement);
    fireEvent.change(await screen.findByDisplayValue('Players'), {
      target: { value: 'Gamers' },
    });
    expect(onChange).toHaveBeenCalledWith({ 'zh-CN': '玩家', 'en-US': 'Gamers' });
  });

  it('value 为空：回退默认语言，输入从 zh-CN 开始', () => {
    const onChange = jest.fn();
    render(<LocalizedTextEditor onChange={onChange} />);
    fireEvent.change(screen.getByRole('textbox'), { target: { value: '新页面' } });
    expect(onChange).toHaveBeenCalledWith({ 'zh-CN': '新页面' });
  });

  it('defaultLocale 命中已录入语言时优先选中', () => {
    render(
      <LocalizedTextEditor
        value={{ 'zh-CN': '玩家', 'en-US': 'Players' }}
        defaultLocale="en-US"
        onChange={jest.fn()}
      />,
    );
    expect(screen.getByDisplayValue('Players')).toBeInTheDocument();
  });

  it('placeholder 缺省回退主语言文案做翻译对照', () => {
    render(
      <LocalizedTextEditor value={{ 'zh-CN': '玩家', 'en-US': 'Players' }} onChange={jest.fn()} />,
    );
    expect((screen.getByRole('textbox') as HTMLInputElement).placeholder).toBe('玩家');
  });

  it('自定义 locale：合法 BCP47 点添加，合并空文案并关闭气泡', async () => {
    const onChange = jest.fn();
    render(<LocalizedTextEditor value={{ 'zh-CN': '玩家' }} onChange={onChange} />);
    await openPopover();
    fireEvent.change(screen.getByPlaceholderText(/自定义 BCP47/), {
      target: { value: 'ko-KR' },
    });
    fireEvent.click(screen.getByRole('button', { name: /添\s*加/ }));
    expect(onChange).toHaveBeenCalledWith({ 'zh-CN': '玩家', 'ko-KR': '' });
    // 关闭动画在 jsdom 挂起致气泡 DOM 残留且值定格；以 setActiveLocale('ko-KR')
    // 生效为主体输入框切到新 locale（无文案 → 清空）作为链路收口信号
    await waitFor(() => expect((screen.getByRole('textbox') as HTMLInputElement).value).toBe(''));
  });

  it('自定义 locale：回车与添加按钮等价', async () => {
    const onChange = jest.fn();
    render(<LocalizedTextEditor value={{ 'zh-CN': '玩家' }} onChange={onChange} />);
    await openPopover();
    const custom = screen.getByPlaceholderText(/自定义 BCP47/);
    fireEvent.change(custom, { target: { value: 'ru-RU' } });
    fireEvent.keyDown(custom, { key: 'Enter' });
    expect(onChange).toHaveBeenCalledWith({ 'zh-CN': '玩家', 'ru-RU': '' });
  });

  it('非法 BCP47：添加按钮 disabled，回车不触发', async () => {
    const onChange = jest.fn();
    render(<LocalizedTextEditor value={{ 'zh-CN': '玩家' }} onChange={onChange} />);
    await openPopover();
    const custom = screen.getByPlaceholderText(/自定义 BCP47/);
    fireEvent.change(custom, { target: { value: 'not-a-locale!' } });
    expect(screen.getByRole('button', { name: /添\s*加/ })).toBeDisabled();
    fireEvent.keyDown(custom, { key: 'Enter' });
    expect(onChange).not.toHaveBeenCalled();
  });

  it('已录入的 locale 不重复添加', async () => {
    const onChange = jest.fn();
    render(<LocalizedTextEditor value={{ 'zh-CN': '玩家' }} onChange={onChange} />);
    await openPopover();
    fireEvent.change(screen.getByPlaceholderText(/自定义 BCP47/), {
      target: { value: 'zh-CN' },
    });
    fireEvent.click(screen.getByRole('button', { name: /添\s*加/ }));
    expect(onChange).not.toHaveBeenCalled();
  });

  it('disabled：locale 下拉 / 输入框 / 🌐 全部禁用', () => {
    render(<LocalizedTextEditor value={{ 'zh-CN': '玩家' }} disabled onChange={jest.fn()} />);
    expect(screen.getByRole('combobox')).toBeDisabled();
    expect(screen.getByRole('textbox')).toBeDisabled();
    expect(screen.getByRole('button')).toBeDisabled();
  });
});
