/**
 * 菜单树数据映射（studio/menuTree.tsx）单测：此前 0% 覆盖——
 * toMenuTreeData 是 MenuMountModal 与 EditorModal 共用的纯函数，
 * 负责 MenuItem 树 → TreeSelect treeData（labels 本地化 + menuKey 标识）。
 */
import type { ReactElement } from 'react';
import { render } from '@testing-library/react';
import type { MenuItem } from '@/services/api/menu';
import { toMenuTreeData } from '../studio/menuTree';

const menu = (over: Partial<MenuItem> & { id: number; menuKey: string }): MenuItem => ({
  parentId: 0,
  labels: {},
  sortOrder: 0,
  isVisible: true,
  children: [],
  ...over,
});

const flat: MenuItem[] = [
  menu({ id: 1, menuKey: 'resource', labels: { 'zh-CN': '资源管理', 'en-US': 'Resources' } }),
  menu({ id: 2, menuKey: 'ops', labels: { 'en-US': 'Ops' } }),
];

describe('toMenuTreeData（PageStudio 菜单挂载/编辑器共用）', () => {
  it('扁平列表映射：title 为本地化 label + menuKey 标识，value=key=id', () => {
    const [node] = toMenuTreeData(flat, 'zh-CN');
    expect(node.value).toBe(1);
    expect(node.key).toBe('resource');

    const { container, getByText } = render(node.title as ReactElement);
    expect(getByText('资源管理')).toBeInTheDocument();
    expect(getByText('resource')).toBeInTheDocument();
    expect(container.textContent).toBe('资源管理resource');
  });

  it('locale 选择与回退：缺当前语言取其他语言，labels 全空回退 menuKey', () => {
    // 缺 zh-CN → 取 en-US「Ops」
    const ops = toMenuTreeData(flat, 'zh-CN')[1];
    const opsRendered = render(ops.title as ReactElement);
    expect(opsRendered.getByText('Ops')).toBeInTheDocument();
    opsRendered.unmount();

    // labels 全空 → fallback menuKey（localizedText 第三参）；label 与
    // menuKey 标识同文，两个节点都是「raw」
    const bare = toMenuTreeData([menu({ id: 3, menuKey: 'raw', labels: {} })], 'zh-CN')[0];
    const bareRendered = render(bare.title as ReactElement);
    expect(bareRendered.container.textContent).toBe('rawraw');
  });

  it('嵌套树递归映射；空 children 不产生 children 键', () => {
    const tree: MenuItem[] = [
      menu({
        id: 1,
        menuKey: 'system',
        labels: { 'zh-CN': '系统' },
        children: [
          menu({ id: 2, menuKey: 'users', labels: { 'zh-CN': '用户' }, parentId: 1 }),
          menu({ id: 3, menuKey: 'roles', labels: { 'zh-CN': '角色' }, parentId: 1, children: [] }),
        ],
      }),
    ];
    const [root] = toMenuTreeData(tree, 'zh-CN');
    expect(root.key).toBe('system');
    expect(root.children).toHaveLength(2);
    expect(root.children?.map((c) => c.key)).toEqual(['users', 'roles']);
    expect(root.children?.every((c) => c.children === undefined)).toBe(true);

    // children 未定义（显式缺省，命中 `?? []` 回退）同空处理
    const noChildren = toMenuTreeData(
      [menu({ id: 4, menuKey: 'leaf', labels: {}, children: undefined })],
      'zh-CN',
    )[0];
    expect(noChildren.children).toBeUndefined();
  });

  it('空列表映射为空数组', () => {
    expect(toMenuTreeData([], 'zh-CN')).toEqual([]);
  });
});
