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
