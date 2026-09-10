/** Permissions/Config 回归与主路径覆盖。
 *
 * 回归点（本轮修复，必须持续守护）：
 * 1. 装饰性空操作搜索按钮已删除，Input onChange 实时过滤（受控 state）；
 * 2. 搜索在「权限域总览 / 权限详情」两个 Tab 共享生效。
 *
 * 其余主路径：四个 Tab（domains 表格 / details 折叠面板 / matrix 统计卡片 /
 * security 静态策略）渲染与过滤降级（无匹配）。 */
import React from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import ConfigPage from '../index';

const ALL_DOMAINS = 25;
const PAGE_SIZE = 10;

function tableRows(): number {
  return document.querySelectorAll('.ant-table-row').length;
}

function searchInput(): HTMLInputElement {
  return screen.getAllByPlaceholderText('搜索权限域或权限')[0] as HTMLInputElement;
}

describe('Permissions/Config', () => {
  it('默认渲染权限域总览：标题、统计文本与表格（首页 10 行）', async () => {
    render(<ConfigPage />);
    expect(screen.getByText('权限配置管理')).toBeInTheDocument();
    expect(
      screen.getAllByText(
        (_content, element) => element?.textContent?.includes('个权限域') ?? false,
      ).length,
    ).toBeGreaterThan(0);
    await waitFor(() => expect(tableRows()).toBe(PAGE_SIZE));
    expect(screen.getByText('system:*')).toBeInTheDocument();
    expect(screen.getByText('user:*')).toBeInTheDocument();
  });

  it('搜索按权限域关键字实时过滤（受控 state）', async () => {
    render(<ConfigPage />);
    await waitFor(() => expect(tableRows()).toBe(PAGE_SIZE));
    fireEvent.change(searchInput(), { target: { value: 'system' } });
    // 匹配 domain/description/permissions 含 system 的权限域（跨域匹配）
    await waitFor(() => expect(tableRows()).toBeLessThan(PAGE_SIZE));
    expect(screen.getByText('system:*')).toBeInTheDocument();
    expect(screen.queryByText('user:*')).not.toBeInTheDocument();
    // 清空恢复全量
    fireEvent.change(searchInput(), { target: { value: '' } });
    await waitFor(() => expect(tableRows()).toBe(PAGE_SIZE));
  });

  it('搜索按权限名过滤且大小写不敏感', async () => {
    render(<ConfigPage />);
    await waitFor(() => expect(tableRows()).toBe(PAGE_SIZE));
    fireEvent.change(searchInput(), { target: { value: 'SYSTEM:RESTART' } });
    await waitFor(() => expect(tableRows()).toBe(1));
    expect(screen.getByText('system:*')).toBeInTheDocument();
  });

  it('搜索按描述过滤', async () => {
    render(<ConfigPage />);
    await waitFor(() => expect(tableRows()).toBe(PAGE_SIZE));
    fireEvent.change(searchInput(), { target: { value: '用户管理权限' } });
    await waitFor(() => expect(tableRows()).toBe(1));
    expect(screen.getByText('user:*')).toBeInTheDocument();
  });

  it('无匹配时列表为空', async () => {
    render(<ConfigPage />);
    await waitFor(() => expect(tableRows()).toBe(PAGE_SIZE));
    fireEvent.change(searchInput(), { target: { value: '不存在的权限域zzz' } });
    await waitFor(() => expect(tableRows()).toBe(0));
  });

  it('权限详情 Tab：折叠面板渲染，展开显示权限 Tag 与说明；搜索同样生效', async () => {
    render(<ConfigPage />);
    await waitFor(() => expect(tableRows()).toBe(PAGE_SIZE));
    fireEvent.click(screen.getByRole('tab', { name: '权限详情' }));
    const headers = document.querySelectorAll('.ant-collapse-header');
    expect(headers.length).toBe(ALL_DOMAINS);
    // 展开第一个面板：权限 Tag 与权限说明
    fireEvent.click(headers[0]);
    await screen.findByText('system:config');
    expect(
      screen.getAllByText(
        (_content, element) => element?.textContent?.includes('个具体权限') ?? false,
      ).length,
    ).toBeGreaterThan(0);
    // 搜索在详情 Tab 共享：过滤后折叠面板只剩匹配域
    fireEvent.change(searchInput(), { target: { value: 'system:restart' } });
    await waitFor(() => expect(document.querySelectorAll('.ant-collapse-header').length).toBe(1));
    fireEvent.change(searchInput(), { target: { value: '' } });
    await waitFor(() =>
      expect(document.querySelectorAll('.ant-collapse-header').length).toBe(ALL_DOMAINS),
    );
  });

  it('权限矩阵 Tab：域卡片渲染，超过 5 个权限显示 +N 溢出标记', async () => {
    render(<ConfigPage />);
    await waitFor(() => expect(tableRows()).toBe(PAGE_SIZE));
    fireEvent.click(screen.getByRole('tab', { name: '权限矩阵' }));
    expect(screen.getByText('权限域统计')).toBeInTheDocument();
    // system 域 6 个权限：前 5 Tag + '+1' 溢出
    expect(screen.getAllByText('system:config').length).toBeGreaterThan(0);
    expect(screen.getAllByText('+1').length).toBeGreaterThan(0);
  });

  it('安全配置 Tab：安全原则与高风险权限说明渲染', async () => {
    render(<ConfigPage />);
    await waitFor(() => expect(tableRows()).toBe(PAGE_SIZE));
    fireEvent.click(screen.getByRole('tab', { name: '安全配置' }));
    expect(screen.getByText('最小权限原则')).toBeInTheDocument();
    expect(screen.getByText('权限分离')).toBeInTheDocument();
    expect(screen.getByText('定期审查')).toBeInTheDocument();
    expect(screen.getByText('审计记录')).toBeInTheDocument();
    expect(screen.getByText('以下权限需要特别注意')).toBeInTheDocument();
    expect(screen.getByText('超级权限，拥有所有系统权限')).toBeInTheDocument();
  });
});
