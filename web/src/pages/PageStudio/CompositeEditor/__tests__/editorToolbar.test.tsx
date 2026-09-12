/** 编辑器顶栏「保存为组件」常驻按钮（V1 发现性，全页集成渲染）。
 * 回归点：此前按钮仅 multiIds.size > 0 才渲染——能力入口完全不可见。
 * 现在未多选时也常驻（disabled + Tooltip 教学），多选后启用并显示计数。 */
import React from 'react';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { App } from 'antd';
import CompositeEditorPage from '../index';
import { listDescriptors } from '@/services/api/functions';

jest.mock('@/services/api/functions', () => ({
  invokeFunction: jest.fn(async () => ({ result: {} })),
  listDescriptors: jest.fn(async () => []),
}));

// 全页渲染（DndContext + PageContainer + 模板引导/函数面板 fetch），放宽预算
jest.setTimeout(20000);
const FIND = { timeout: 5000 } as const;

describe('顶栏「保存为组件」常驻按钮（V1 发现性）', () => {
  it('未多选时渲染禁用态按钮（不再消失）', async () => {
    render(
      <App>
        <CompositeEditorPage />
      </App>,
    );
    // 空画布模板引导（request mock 返回 {} → 无组合模板）
    await screen.findByText(/从模板开始/, undefined, FIND);
    // name 用正则：antd 图标 aria-label（appstore）会拼进 accessible name
    const btn = screen.getByRole('button', { name: /保存为组件$/ });
    expect(btn).toBeDisabled();
  });

  it('悬停禁用按钮显示多选教学 Tooltip', async () => {
    render(
      <App>
        <CompositeEditorPage />
      </App>,
    );
    await screen.findByText(/从模板开始/, undefined, FIND);
    // hover 挂在外包 span 上（disabled button 不派发鼠标事件）
    const btn = screen.getByRole('button', { name: /保存为组件$/ });
    fireEvent.mouseEnter(btn.parentElement as HTMLElement);
    await waitFor(() => expect(screen.getByText(/Shift\+点击 多选画布节点/)).toBeInTheDocument(), {
      timeout: 5000,
    });
  });

  it('多选画布节点后按钮转为启用态并显示计数', async () => {
    (listDescriptors as unknown as jest.Mock).mockImplementation(async () => [
      {
        id: 'player.list',
        resource: 'player',
        summary: { 'zh-CN': '玩家列表' },
        outputSchema: {
          type: 'object',
          properties: { items: { type: 'array', items: { type: 'object' } } },
        },
      },
    ]);
    render(
      <App>
        <CompositeEditorPage />
      </App>,
    );
    // 空白起步：跳过模板引导，进入可编辑画布
    fireEvent.click(await screen.findByRole('button', { name: '从空白开始' }, FIND));
    // 左栏切到「函数」tab：函数树节点 label=operation||id 尾段（此处 'list'）
    fireEvent.click(await screen.findByRole('tab', { name: '函数' }, FIND));
    const fnNode = await screen.findByText('list', undefined, FIND);
    fireEvent.click(fnNode);
    // 画布出现 fnTable 卡片（scaffold title 取 summary），Shift+点击入多选集合
    // （卡片 title 与 Preview 内部标题同名，取第一个=卡片头，位于 onSelect 冒泡路径）
    const cards = await screen.findAllByText('玩家列表', undefined, FIND);
    fireEvent.click(cards[0], { shiftKey: true });
    // FormattedMessage 测试 mock 不做 ICU 插值 → 启用态文案为「保存为组件（{count}）」
    const active = await screen.findByRole('button', { name: /保存为组件（\{count\}）/ }, FIND);
    expect(active).toBeEnabled();
  });
});
