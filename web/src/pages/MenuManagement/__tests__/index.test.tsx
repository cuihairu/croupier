import { configure, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { App, ConfigProvider } from 'antd';
import { useAccess } from '@umijs/max';
import MenuManagementPage from '../index';
import { createMenu, deleteMenu, listMenus, updateMenu } from '@/services/api/menu';

jest.mock('@/services/api/menu', () => ({
  createMenu: jest.fn(),
  deleteMenu: jest.fn(),
  listMenus: jest.fn(),
  updateMenu: jest.fn(),
  updateMenuSort: jest.fn(),
}));

jest.mock('@umijs/max', () => ({
  useAccess: jest.fn(),
  // 与 tests/setupTests.jsx 同语义：返回 defaultMessage + {placeholder} 插值；
  // 额外提供 locale（localizedText 渲染需要，真实 intl 均携带）
  useIntl: () => ({
    locale: 'zh-CN',
    formatMessage: ({ defaultMessage }, values) =>
      Object.entries(values || {}).reduce(
        (msg: string, [key, val]) => msg.split(`{${key}}`).join(String(val)),
        defaultMessage,
      ),
  }),
}));

const mockedAccess = useAccess as jest.MockedFunction<typeof useAccess>;
const mockedListMenus = listMenus as jest.MockedFunction<typeof listMenus>;
const mockedCreateMenu = createMenu as jest.MockedFunction<typeof createMenu>;
const mockedUpdateMenu = updateMenu as jest.MockedFunction<typeof updateMenu>;
const mockedDeleteMenu = deleteMenu as jest.MockedFunction<typeof deleteMenu>;

const treeItems = [
  {
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
        labels: { 'zh-CN': '玩家管理' },
        sortOrder: 1,
        isVisible: true,
        children: [],
      },
    ],
  },
  {
    id: 3,
    parentId: null,
    menuKey: 'secret',
    labels: { 'zh-CN': '机密' },
    sortOrder: 2,
    isVisible: false,
    children: [],
  },
];

const renderPage = () =>
  render(
    <App>
      <ConfigProvider button={{ autoInsertSpace: false }}>
        <MenuManagementPage />
      </ConfigProvider>
    </App>,
  );

jest.setTimeout(20000);

beforeAll(() => {
  configure({ asyncUtilTimeout: 8000 });
});

beforeEach(() => {
  jest.clearAllMocks();
  mockedAccess.mockReturnValue({ canMenuManage: true } as never);
  mockedListMenus.mockResolvedValue(treeItems as never);
});

describe('MenuManagement page', () => {
  it('加载并渲染菜单树（含层级、权限标记、隐藏标记）', async () => {
    renderPage();
    expect(mockedListMenus).toHaveBeenCalled();
    expect(await screen.findByText('资源管理')).toBeInTheDocument();
    expect(screen.getByText('玩家管理')).toBeInTheDocument();
    expect(screen.getByText('resource:read')).toBeInTheDocument();
    expect(screen.getByText('隐藏')).toBeInTheDocument();
    // menuKey Tag（exact 匹配避免命中 permission Tag "resource:read"）
    expect(screen.getByText('resource')).toBeInTheDocument();
  });

  it('无管理权限时不渲染操作按钮与新建入口', async () => {
    mockedAccess.mockReturnValue({} as never);
    renderPage();
    await screen.findByText('资源管理');
    expect(screen.queryByText('新建菜单')).not.toBeInTheDocument();
    expect(screen.queryByText('编辑')).not.toBeInTheDocument();
    expect(screen.queryByText('删除')).not.toBeInTheDocument();
  });

  it('空菜单渲染空态', async () => {
    mockedListMenus.mockResolvedValue([] as never);
    renderPage();
    expect(await screen.findByText('暂无菜单，点击「新建菜单」创建第一个菜单')).toBeInTheDocument();
  });

  it('新建菜单：打开弹窗、必填校验、提交调用 createMenu', async () => {
    mockedCreateMenu.mockResolvedValue(treeItems[0] as never);
    renderPage();
    await screen.findByText('资源管理');

    fireEvent.click(screen.getByRole('button', { name: /新建菜单/ }));
    const modal = await screen.findByRole('dialog');
    expect(within(modal).getByLabelText(/菜单标识/)).toBeInTheDocument();

    // 直接触发提交：menuKey/labels 校验失败，弹窗不关
    fireEvent.click(within(modal).getByRole('button', { name: /确\s*定/ }));
    await waitFor(() => {
      expect(mockedCreateMenu).not.toHaveBeenCalled();
    });

    // 填 menuKey + labels 后提交成功
    const keyInput = within(modal).getByLabelText(/菜单标识/);
    fireEvent.change(keyInput, { target: { value: 'analytics' } });
    const nameInput = within(modal).getByPlaceholderText('请输入菜单名称');
    fireEvent.change(nameInput, { target: { value: '数据分析' } });
    fireEvent.click(within(modal).getByRole('button', { name: /确\s*定/ }));

    await waitFor(() => {
      expect(mockedCreateMenu).toHaveBeenCalledWith(
        expect.objectContaining({
          menuKey: 'analytics',
          labels: expect.objectContaining({ 'zh-CN': '数据分析' }),
          parentId: 0,
          isVisible: true,
        }),
      );
    });
    await waitFor(() => {
      expect(mockedListMenus).toHaveBeenCalledTimes(2);
    });
  });

  it('编辑菜单：预填当前值并调用 updateMenu', async () => {
    mockedUpdateMenu.mockResolvedValue(treeItems[0] as never);
    renderPage();
    await screen.findByText('资源管理');

    // 树节点上的编辑按钮（第一个）
    const editButtons = await screen.findAllByRole('button', { name: /编\s*辑/ });
    fireEvent.click(editButtons[0]);

    const modal = await screen.findByRole('dialog');
    const keyInput = within(modal).getByLabelText(/菜单标识/) as HTMLInputElement;
    expect(keyInput.value).toBe('resource');

    fireEvent.change(keyInput, { target: { value: 'resource2' } });
    fireEvent.click(within(modal).getByRole('button', { name: /确\s*定/ }));

    await waitFor(() => {
      expect(mockedUpdateMenu).toHaveBeenCalledWith(
        1,
        expect.objectContaining({ menuKey: 'resource2' }),
      );
    });
  });

  it('删除菜单：Popconfirm 二次确认后调用 deleteMenu', async () => {
    mockedDeleteMenu.mockResolvedValue(undefined as never);
    renderPage();
    await screen.findByText('资源管理');

    const deleteButtons = await screen.findAllByRole('button', { name: /删\s*除/ });
    fireEvent.click(deleteButtons[0]);

    // Popconfirm 确认
    const popover = document.querySelector('.ant-popover');
    expect(popover).not.toBeNull();
    const okButton = within(popover as HTMLElement).getByRole('button', { name: /确\s*定|OK/ });
    fireEvent.click(okButton);

    await waitFor(() => {
      expect(mockedDeleteMenu).toHaveBeenCalledWith(1);
    });
    await waitFor(() => {
      expect(mockedListMenus).toHaveBeenCalledTimes(2);
    });
  });

  it('列表加载失败时渲染空态且不崩溃（错误提示由全局拦截器负责）', async () => {
    mockedListMenus.mockRejectedValue(new Error('boom') as never);
    renderPage();
    await waitFor(() => {
      expect(mockedListMenus).toHaveBeenCalled();
    });
    expect(await screen.findByText('暂无菜单，点击「新建菜单」创建第一个菜单')).toBeInTheDocument();
  });
});
