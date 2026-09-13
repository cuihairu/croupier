/** HeaderDropdown：Dropdown 透传包装——overlayClassName 与 createStyles 内置类合并、
 * placement/children/menu 透传、不传 overlayClassName 时仅内置样式。 */
import React from 'react';
import { fireEvent, render, screen } from '@testing-library/react';
import HeaderDropdown from './index';

const menuProps = { items: [{ key: 'edit', label: '下拉菜单项' }] };

/** hover 触发器打开弹层，返回弹层根元素（.ant-dropdown）。 */
async function openDropdown(triggerText: string): Promise<HTMLElement> {
  fireEvent.mouseEnter(screen.getByText(triggerText));
  const item = await screen.findByText('下拉菜单项', undefined, { timeout: 5000 });
  return item.closest('.ant-dropdown') as HTMLElement;
}

describe('HeaderDropdown', () => {
  it('渲染触发器，hover 打开菜单，overlayClassName 与内置样式类合并', async () => {
    render(
      <HeaderDropdown overlayClassName="my-overlay-cls" menu={menuProps}>
        <span>触发器甲</span>
      </HeaderDropdown>,
    );
    expect(screen.getByText('触发器甲')).toBeInTheDocument();
    const popup = await openDropdown('触发器甲');
    expect(popup.className).toContain('ant-dropdown');
    expect(popup.className).toContain('my-overlay-cls');
  });

  it('不传 overlayClassName：弹层仅含内置样式类，placement 透传生效', async () => {
    render(
      <HeaderDropdown placement="bottomCenter" menu={menuProps}>
        <span>触发器乙</span>
      </HeaderDropdown>,
    );
    const popup = await openDropdown('触发器乙');
    expect(popup.className).toContain('ant-dropdown');
    expect(popup.className).not.toContain('my-overlay-cls');
    // antd 将 bottomCenter 归一为 placement-bottom（区别于默认 bottomLeft）
    expect(popup.classList.contains('ant-dropdown-placement-bottom')).toBe(true);
    expect(popup.classList.contains('ant-dropdown-placement-bottomLeft')).toBe(false);
  });
});
