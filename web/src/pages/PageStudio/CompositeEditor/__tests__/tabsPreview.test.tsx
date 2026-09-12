/** V2 页签容器组件：scaffoldTabsNode 构造（两入口共用）与编辑期 Preview
 * 的页签切换（activeTab 写 node.props——drop 落点定位依赖此 UI 态）。 */
import { render, screen, fireEvent } from '@testing-library/react';
import { App } from 'antd';
import { resetRegistryForTest } from '../registry';
import { registerBuiltinComponents } from '../components/builtin';
import { getComponent } from '../registry';
import { scaffoldTabsNode } from '../model';
import type { PageNode } from '../model';

describe('scaffoldTabsNode（tabs 节点构造入口）', () => {
  it('自带 2 个空页签：children=container、title=页签 N', () => {
    const node = scaffoldTabsNode();
    expect(node.type).toBe('tabs');
    expect(node.props.span).toBe(24);
    expect(node.children).toHaveLength(2);
    expect(node.children!.every((p) => p.type === 'container')).toBe(true);
    expect(node.children![0].props.title).toBe('页签 1');
    expect(node.children![1].props.title).toBe('页签 2');
    // 每页 children 就位（空数组，可 drop）
    expect(node.children!.every((p) => Array.isArray(p.children))).toBe(true);
  });
});

describe('Tabs Preview（编辑期页签渲染与切换）', () => {
  let TabsPreview: NonNullable<ReturnType<typeof getComponent>['Preview']>;

  beforeAll(() => {
    resetRegistryForTest();
    registerBuiltinComponents();
    TabsPreview = getComponent('tabs')!.Preview;
  });

  const withContent = (): PageNode => {
    const node = scaffoldTabsNode();
    node.children![0].children = [
      {
        id: 'tbl1',
        type: 'fnTable',
        props: { functionId: 'player.list', title: '玩家列表', columns: ['uid', 'gold'] },
      },
    ];
    return node;
  };

  it('渲染页签标签与页内子节点；切到空页显示提示（antd 面板懒渲染）', () => {
    const node = withContent();
    render(
      <App>
        <TabsPreview node={node} />
      </App>,
    );
    expect(screen.getByRole('tab', { name: '页签 1' })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: '页签 2' })).toBeInTheDocument();
    // FnTable Preview 渲染列名（不渲染 title）
    expect(screen.getByText('uid')).toBeInTheDocument();
    expect(screen.getByText('gold')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('tab', { name: '页签 2' }));
    expect(screen.getByText('空页签——拖入组件')).toBeInTheDocument();
  });

  it('无页容器 → 整体提示（不渲染 Tabs）', () => {
    const node: PageNode = { id: 't0', type: 'tabs', props: {}, children: [] };
    render(
      <App>
        <TabsPreview node={node} />
      </App>,
    );
    expect(screen.getByText(/空页签容器/)).toBeInTheDocument();
  });

  it('切换页签把激活页 id 写入 props.activeTab（drop 落点依据）', () => {
    const node = withContent();
    const first = render(
      <App>
        <TabsPreview node={node} />
      </App>,
    );
    expect(node.props.activeTab).toBeUndefined();
    fireEvent.click(screen.getByRole('tab', { name: '页签 2' }));
    expect(node.props.activeTab).toBe(node.children![1].id);
    first.unmount();
    // activeTab 失效（指向已删页）→ 回落首页展示
    node.props.activeTab = 'gone';
    render(
      <App>
        <TabsPreview node={node} />
      </App>,
    );
    expect(screen.getByText('uid')).toBeInTheDocument();
  });
});
