import { request } from '@umijs/max';
import {
  createMenu,
  deleteMenu,
  listAccessibleMenus,
  listMenus,
  updateMenu,
  updateMenuSort,
} from './menu';

jest.mock('@umijs/max', () => ({ request: jest.fn() }));

const mockedRequest = request as jest.MockedFunction<typeof request>;

const rawMenu = {
  id: 1,
  parentId: null,
  menuKey: 'resource',
  labels: { 'zh-CN': '资源管理' },
  icon: 'DatabaseOutlined',
  sortOrder: 1,
  permission: 'resource:read',
  isVisible: true,
  children: [
    {
      id: 2,
      parentId: 1,
      menuKey: 'player',
      labels: { zh: '玩家管理' },
      sortOrder: 1,
      isVisible: true,
      children: [],
    },
  ],
};

describe('menu api', () => {
  afterEach(() => {
    jest.clearAllMocks();
  });

  it('listMenus 解包 items 并递归归一 labels/children', async () => {
    mockedRequest.mockResolvedValueOnce({ items: [rawMenu] });
    const items = await listMenus();
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/menus');
    expect(items).toHaveLength(1);
    expect(items[0].labels).toEqual({ 'zh-CN': '资源管理' });
    // 遗留短 key zh 归一为 zh-CN
    expect(items[0].children[0].labels).toEqual({ 'zh-CN': '玩家管理' });
    expect(items[0].children[0].parentId).toBe(1);
  });

  it('listMenus 兼容裸数组响应', async () => {
    mockedRequest.mockResolvedValueOnce([rawMenu]);
    const items = await listMenus();
    expect(items).toHaveLength(1);
  });

  it('listAccessibleMenus 请求 /api/v1/menus/accessible 并归一', async () => {
    mockedRequest.mockResolvedValueOnce({ items: [rawMenu] });
    const items = await listAccessibleMenus();
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/menus/accessible');
    expect(items).toHaveLength(1);
    expect(items[0].children[0].labels).toEqual({ 'zh-CN': '玩家管理' });
  });

  it('listMenus 空响应返回空数组，isVisible 缺省为 true', async () => {
    mockedRequest.mockResolvedValueOnce({ items: [] });
    expect(await listMenus()).toEqual([]);
    mockedRequest.mockResolvedValueOnce({});
    expect(await listMenus()).toEqual([]);
  });

  it('createMenu POST /api/v1/menus 并透传载荷', async () => {
    mockedRequest.mockResolvedValueOnce(rawMenu);
    const created = await createMenu({
      menuKey: 'resource',
      labels: { 'zh-CN': '资源管理' },
      parentId: 0,
    });
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/menus', {
      method: 'POST',
      data: { menuKey: 'resource', labels: { 'zh-CN': '资源管理' }, parentId: 0 },
    });
    expect(created.menuKey).toBe('resource');
    expect(created.children).toHaveLength(1);
    expect(created.children[0].menuKey).toBe('player');
  });

  it('updateMenu PUT /api/v1/menus/:id 仅提交出现字段', async () => {
    mockedRequest.mockResolvedValueOnce(rawMenu);
    await updateMenu(7, { icon: 'TeamOutlined', parentId: 0 });
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/menus/7', {
      method: 'PUT',
      data: { icon: 'TeamOutlined', parentId: 0 },
    });
  });

  it('deleteMenu DELETE /api/v1/menus/:id', async () => {
    mockedRequest.mockResolvedValueOnce(undefined);
    await deleteMenu(9);
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/menus/9', { method: 'DELETE' });
  });

  it('updateMenuSort PUT /api/v1/menus/:id/sort', async () => {
    mockedRequest.mockResolvedValueOnce(rawMenu);
    await updateMenuSort(3, 5);
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/menus/3/sort', {
      method: 'PUT',
      data: { sortOrder: 5 },
    });
  });
});

describe('menu api 归一兜底分支', () => {
  afterEach(() => {
    jest.clearAllMocks();
  });

  it('节点缺 labels/sortOrder/children：labels 兜底空对象、sortOrder 0、children 空数组', async () => {
    mockedRequest.mockResolvedValueOnce({
      items: [{ id: 7, parentId: null, menuKey: 'bare' }],
    });
    const items = await listMenus();
    expect(items).toEqual([
      {
        id: 7,
        parentId: null,
        menuKey: 'bare',
        labels: {},
        icon: undefined,
        sortOrder: 0,
        permission: undefined,
        isVisible: true,
        children: [],
      },
    ]);
  });

  it('labels 为空字符串：normalize 后兜底空对象', async () => {
    mockedRequest.mockResolvedValueOnce({
      items: [{ id: 8, parentId: null, menuKey: 'k', labels: '' }],
    });
    const items = await listMenus();
    expect(items[0].labels).toEqual({});
  });

  it('listAccessibleMenus 兼容裸数组响应与空体', async () => {
    mockedRequest.mockResolvedValueOnce([rawMenu]);
    expect(await listAccessibleMenus()).toHaveLength(1);
    // 空响应体 / items 缺省 → 空数组（不抛错）
    mockedRequest.mockResolvedValueOnce(undefined);
    expect(await listAccessibleMenus()).toEqual([]);
    mockedRequest.mockResolvedValueOnce({ items: [] });
    expect(await listAccessibleMenus()).toEqual([]);
  });
});
