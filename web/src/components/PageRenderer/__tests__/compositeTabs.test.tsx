/** 发布渲染端 V2：display='tab' 区块按 group 聚合进 Tabs、组内按 tab
 * 标签聚合页、页内区块整行堆叠；tab 区块 autoRun 照常执行（不因分组
 * 被排除）。inline 与 tabbed 共存互不干扰。 */
import React from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App } from 'antd';
import { CompositeRenderer } from '../index';
import type { CompositeSection } from '@/types/dashboard';

jest.mock('@/services/api/functions', () => ({ invokeFunction: jest.fn() }));

const sections: CompositeSection[] = [
  {
    key: 'inlineTable',
    bindingId: 'b-inline',
    view: 'table',
    title: { 'zh-CN': '页首概览' },
    autoRun: false,
    table: { columns: [{ key: 'total', title: { 'zh-CN': '总数' } }] },
  },
  {
    key: 'tabTable',
    bindingId: 'b-tab-table',
    view: 'table',
    title: { 'zh-CN': '玩家列表' },
    autoRun: true,
    display: 'tab',
    group: 'mainTabs',
    tab: { 'zh-CN': '列表页' },
    table: {
      columns: [
        { key: 'uid', title: { 'zh-CN': 'UID' } },
        { key: 'nickname', title: { 'zh-CN': '昵称' } },
      ],
    },
  },
  {
    key: 'tabFields',
    bindingId: 'b-tab-fields',
    view: 'fields',
    title: { 'zh-CN': '玩家详情' },
    autoRun: false,
    display: 'tab',
    group: 'mainTabs',
    tab: { 'zh-CN': '列表页' },
  },
  {
    key: 'tabNoLabel',
    bindingId: 'b-tab-nolabel',
    view: 'form',
    title: { 'zh-CN': '批量操作' },
    display: 'tab',
    group: 'mainTabs',
    form: {
      jsonSchema: {
        type: 'object',
        // schema 内 title 是纯字符串（JSON Schema 语义，非 LocalizedText）
        properties: { reason: { type: 'string', title: '原因' } },
      },
    },
  },
];

function renderComposite(onExecute: jest.Mock) {
  return render(
    <App>
      <CompositeRenderer
        sections={sections}
        bindings={[]}
        onExecute={onExecute as never}
        preview={false}
      />
    </App>,
  );
}

describe('CompositeRenderer V2：页签分组聚合', () => {
  it('同 group 渲染进一个 Tabs；组内按 tab 标签聚合页；无标签兜底「页签 N」', async () => {
    const onExecute = jest.fn().mockResolvedValue({
      data: { items: [{ uid: 'u1', nickname: 'bob' }], total: 1 },
    });
    renderComposite(onExecute);
    // tabTable/tabFields 同标签「列表页」→ 聚进同一页；tabNoLabel 无 tab →
    // 组内第 2 个新建页，兜底「页签 2」
    expect(screen.getByRole('tab', { name: '列表页' })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: /页签\s*2/ })).toBeInTheDocument();
    // inline 区块仍在 Tabs 之外渲染
    expect(screen.getByText('页首概览')).toBeInTheDocument();
  });

  it('页内区块整行堆叠：同页表格+字段卡都渲染', async () => {
    const onExecute = jest.fn().mockResolvedValue({
      data: { items: [{ uid: 'u1', nickname: 'bob' }], total: 1 },
    });
    renderComposite(onExecute);
    await waitFor(() => expect(screen.getByText('bob')).toBeInTheDocument());
    expect(screen.getByText('玩家列表')).toBeInTheDocument();
    expect(screen.getByText('玩家详情')).toBeInTheDocument();
  });

  it('tab 区块 autoRun 照常执行（不因分组被排除）', async () => {
    const onExecute = jest.fn().mockResolvedValue({
      data: { items: [{ uid: 'u1', nickname: 'bob' }], total: 1 },
    });
    renderComposite(onExecute);
    await waitFor(() => expect(onExecute).toHaveBeenCalledWith('b-tab-table', expect.anything()));
    // inline 区块 autoRun=false 不执行
    expect(onExecute).not.toHaveBeenCalledWith('b-inline', expect.anything());
  });

  it('切换页签后另一页内容可见', async () => {
    const onExecute = jest.fn().mockResolvedValue({
      data: { items: [{ uid: 'u1', nickname: 'bob' }], total: 1 },
    });
    renderComposite(onExecute);
    fireEvent.click(screen.getByRole('tab', { name: /页签\s*2/ }));
    await waitFor(() => expect(screen.getByText('批量操作')).toBeInTheDocument());
    expect(screen.getByText('原因')).toBeInTheDocument();
  });
});
