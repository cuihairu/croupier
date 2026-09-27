import { configure, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { App, ConfigProvider } from 'antd';
import { useAccess } from '@umijs/max';
import MenuManagementPage from '../index';
import { listPageDrafts } from '@/services/api/pages';
import type { PageSpecDraftSummary } from '@/types/dashboard';
import { createMenu, deleteMenu, listMenus, updateMenu, updateMenuSort } from '@/services/api/menu';

jest.mock('@/services/api/menu', () => ({
  createMenu: jest.fn(),
  deleteMenu: jest.fn(),
  listMenus: jest.fn(),
  updateMenu: jest.fn(),
  updateMenuSort: jest.fn(),
}));

jest.mock('@/services/api/pages', () => ({
  listPageDrafts: jest.fn(),
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
  // 编辑页面跳转用（工厂内联创建，避免 TDZ）
  history: { push: jest.fn() },
}));

const umiMock = jest.requireMock('@umijs/max') as { history: { push: jest.Mock } };

const mockedAccess = useAccess as jest.MockedFunction<typeof useAccess>;
const mockedListMenus = listMenus as jest.MockedFunction<typeof listMenus>;
const mockedCreateMenu = createMenu as jest.MockedFunction<typeof createMenu>;
const mockedUpdateMenu = updateMenu as jest.MockedFunction<typeof updateMenu>;
const mockedDeleteMenu = deleteMenu as jest.MockedFunction<typeof deleteMenu>;
const mockedUpdateMenuSort = updateMenuSort as jest.MockedFunction<typeof updateMenuSort>;
const mockedListPageDrafts = listPageDrafts as jest.MockedFunction<typeof listPageDrafts>;

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

// 全量套件并行/串行组合跑时机器负载高，20s 曾连续超时（单独跑 2.5s/用例），
// 与 CI 慢机同型，放宽到与其他重页面套件一致的 30s
jest.setTimeout(30000);

// MenuForm.tsx L149 labels validator 分支登记（v8 coverage 两翼归零）：
// 逐条实证——(1) `value ?? {}` 的 undefined 侧由「空提交必填校验」用例执行
// （错误消息仅出自该 validator，探针验证过）；object 侧由填写/回填用例执行；
// (2) `(v ?? '')` 的 string 侧由全部提交路径执行，nullish 侧 UI 不可达
// （LocalizedTextEditor typing 恒产 string，见其 L80 `[locale]: text`）。
// 两翼均被真实用例触达却归零，属 v8 对 `??` 的块级归因工件（nullish 检查边
// 不产生独立 coverage 块），非真实缺口。
//
// index.tsx L104 `values.parentId ?? 0` 右侧登记为构造性不可达：create 链
// MenuForm initialValues `presetParentId ?? 0`（L103）恒有值，edit 链
// `editing.parentId ?? 0`（L94）已归一，TreeSelect allowClear={false} 不可
// 清空——到达提交载荷构造时 values.parentId 恒为 number，右侧仅是纵深防御。

/** jsdom 未实现 DragEvent：RTL 会退回 Event，clientY/clientX 会丢失，
 *  rc-tree 的「顶级上半区」判断需要真实坐标——补一个最小实现。 */
class TestDragEvent extends Event {
  dataTransfer: DataTransfer | null = null;
  clientX = 0;
  clientY = 0;
  constructor(type: string, init: DragEventInit = {}) {
    super(type, init);
    this.clientX = init.clientX ?? 0;
    this.clientY = init.clientY ?? 0;
    this.dataTransfer = init.dataTransfer ?? null;
  }
}

beforeAll(() => {
  if (typeof window.DragEvent !== 'function') {
    Object.defineProperty(window, 'DragEvent', {
      value: TestDragEvent,
      configurable: true,
      writable: true,
    });
  }
  configure({ asyncUtilTimeout: 8000 });
});

beforeEach(() => {
  jest.clearAllMocks();
  mockedAccess.mockReturnValue({ canMenuManage: true } as never);
  mockedListMenus.mockResolvedValue(treeItems as never);
  mockedListPageDrafts.mockResolvedValue({
    items: [],
    total: 0,
    page: 1,
    pageSize: 200,
  } as never);
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
        expect.objectContaining({
          menuKey: 'resource2',
          // 顶级菜单 parentId=null：MenuForm initialValues `editing.parentId ?? 0`
          // 已归一为 0，载荷必须落 0 而非 null（防归一链回归）
          parentId: 0,
          permission: 'resource:read',
        }),
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

  /** jsdom 未实现 DragEvent/DataTransfer：构造 rc-tree 拖拽链路所需的最小存根。 */
  const makeDataTransfer = (): DataTransfer =>
    ({
      dropEffect: 'move',
      effectAllowed: 'move',
      files: [],
      items: [],
      types: [],
      setData: jest.fn(),
      getData: jest.fn(() => ''),
      clearData: jest.fn(),
      setDragImage: jest.fn(),
      // jsdom 无完整 DataTransferItemList/FileList 实现，仅满足被测链路用到的成员
    }) as unknown as DataTransfer;

  /** 按可见标题定位其所属树节点行（.ant-tree-treenode）。 */
  const rowOf = (label: string): HTMLElement => {
    const row = screen.getByText(label).closest('.ant-tree-treenode');
    expect(row).not.toBeNull();
    return row as HTMLElement;
  };

  /** 模拟 antd Tree 拖拽三段事件：dragStart(拖动行) → dragEnter/drop(目标行)。
   *  jsdom 下 rect/坐标全 0，rc-tree 将 drop 归一为「放入目标内部」
   *  （dropToGap=false → position='inside'），恰好对应跨父级移动语义。 */
  const dragNodeOnto = (dragLabel: string, dropLabel: string) => {
    const dataTransfer = makeDataTransfer();
    fireEvent.dragStart(rowOf(dragLabel), { dataTransfer });
    fireEvent.dragEnter(rowOf(dropLabel), { dataTransfer });
    fireEvent.drop(rowOf(dropLabel), { dataTransfer });
  };

  it('拖拽跨父移动：先改父级再改排序并提示成功（handleMove 成功路径）', async () => {
    mockedUpdateMenu.mockResolvedValue(treeItems[0] as never);
    mockedUpdateMenuSort.mockResolvedValue(treeItems[0] as never);
    renderPage();
    await screen.findByText('资源管理');

    // 机密(3) 拖入 资源管理(1) 内部：跨父移动 → updateMenu(parentId=1) + updateMenuSort(1-based 序)
    dragNodeOnto('机密', '资源管理');

    await waitFor(() => {
      expect(mockedUpdateMenu).toHaveBeenCalledWith(3, { parentId: 1 });
    });
    await waitFor(() => {
      expect(mockedUpdateMenuSort).toHaveBeenCalledWith(3, 2);
    });
    expect(await screen.findByText('菜单排序已更新')).toBeInTheDocument();
    // finally 仍刷新列表
    await waitFor(() => {
      expect(mockedListMenus).toHaveBeenCalledTimes(2);
    });
  });

  it('拖拽移动失败：提示排序更新失败并仍刷新列表（handleMove catch 路径）', async () => {
    mockedUpdateMenu.mockResolvedValue(undefined as never);
    mockedUpdateMenuSort.mockRejectedValue(new Error('sort conflict') as never);
    renderPage();
    await screen.findByText('资源管理');

    dragNodeOnto('机密', '资源管理');

    expect(await screen.findByText('菜单排序更新失败')).toBeInTheDocument();
    await waitFor(() => {
      expect(mockedListMenus).toHaveBeenCalledTimes(2);
    });
  });

  it('新建子菜单：树节点入口打开弹窗、预填父级，提交 parentId 指向该节点', async () => {
    mockedCreateMenu.mockResolvedValue(treeItems[0] as never);
    renderPage();
    await screen.findByText('资源管理');

    // 第一个「加子菜单」按钮属于 资源管理(id=1) 节点
    const addChildButtons = await screen.findAllByRole('button', { name: /加子菜单/ });
    fireEvent.click(addChildButtons[0]);

    const modal = await screen.findByRole('dialog');
    // 弹窗为新建态（非编辑），父菜单 TreeSelect 预填「资源管理 (resource)」
    expect(within(modal).getByText('新建菜单')).toBeInTheDocument();
    expect(await within(modal).findByText('资源管理 (resource)')).toBeInTheDocument();

    fireEvent.change(within(modal).getByLabelText(/菜单标识/), { target: { value: 'analytics2' } });
    fireEvent.change(within(modal).getByPlaceholderText('请输入菜单名称'), {
      target: { value: '子菜单' },
    });
    // 清空排序输入：form 值变 undefined → 提交载荷以 ?? 0 兜底
    fireEvent.change(within(modal).getByRole('spinbutton'), { target: { value: '' } });
    fireEvent.click(within(modal).getByRole('button', { name: /确\s*定/ }));

    await waitFor(() => {
      expect(mockedCreateMenu).toHaveBeenCalledWith(
        expect.objectContaining({ menuKey: 'analytics2', parentId: 1, sortOrder: 0 }),
      );
    });
  });

  it('提交失败：服务端报错时保持弹窗开启供修正、不刷新列表（handleSubmit catch）', async () => {
    mockedCreateMenu.mockRejectedValue(new Error('duplicate menu key') as never);
    renderPage();
    await screen.findByText('资源管理');

    fireEvent.click(screen.getByRole('button', { name: /新建菜单/ }));
    const modal = await screen.findByRole('dialog');
    fireEvent.change(within(modal).getByLabelText(/菜单标识/), { target: { value: 'dup' } });
    fireEvent.change(within(modal).getByPlaceholderText('请输入菜单名称'), {
      target: { value: '重名菜单' },
    });
    fireEvent.click(within(modal).getByRole('button', { name: /确\s*定/ }));

    await waitFor(() => {
      expect(mockedCreateMenu).toHaveBeenCalledTimes(1);
    });
    // onSubmit 返回 false：弹窗保持开启（字段可修正），且不触发 load
    expect(within(screen.getByRole('dialog')).getByLabelText(/菜单标识/)).toBeInTheDocument();
    expect(mockedListMenus).toHaveBeenCalledTimes(1);
  });

  it('名称仅空白时 labels 校验拦截：提示必填且不调用 createMenu（validator trim 兜底）', async () => {
    renderPage();
    await screen.findByText('资源管理');

    fireEvent.click(screen.getByRole('button', { name: /新建菜单/ }));
    const modal = await screen.findByRole('dialog');
    fireEvent.change(within(modal).getByLabelText(/菜单标识/), { target: { value: 'blank' } });
    // 名称只填空白字符：LocalizedTextEditor 会存 { 'zh-CN': '   ' }，
    // validator 以 trim 判空拦截
    fireEvent.change(within(modal).getByPlaceholderText('请输入菜单名称'), {
      target: { value: '   ' },
    });
    fireEvent.click(within(modal).getByRole('button', { name: /确\s*定/ }));

    expect(await screen.findByText('至少填写一个语言的名称')).toBeInTheDocument();
    expect(mockedCreateMenu).not.toHaveBeenCalled();
    expect(mockedListMenus).toHaveBeenCalledTimes(1);
  });

  it('useAccess 返回 undefined 时按无权限处理（access || {} 兜底）', async () => {
    mockedAccess.mockReturnValue(undefined as never);
    renderPage();
    await screen.findByText('资源管理');
    expect(screen.queryByRole('button', { name: /新建菜单/ })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /编\s*辑/ })).not.toBeInTheDocument();
  });

  /** 页面草稿摘要（缺省挂到 资源管理(id=1)） */
  const draftOf = (overrides: Partial<PageSpecDraftSummary> = {}): PageSpecDraftSummary => ({
    pageKey: 'players',
    type: 'resource',
    title: { 'zh-CN': '玩家页面' },
    category: { key: 'resource' },
    menuId: 1,
    status: 'draft',
    draftRevision: 1,
    updatedAt: '2026-09-20T00:00:00Z',
    ...overrides,
  });

  it('页面草稿挂载：menuId>0 渲染菜单页叶子，点击「编辑页面」跳编辑器', async () => {
    const noTitle = draftOf({ pageKey: 'no-title', menuId: 3 });
    Reflect.deleteProperty(noTitle, 'title');
    mockedListPageDrafts.mockResolvedValue({
      items: [
        draftOf(),
        draftOf({ pageKey: 'zero-menu', menuId: 0 }),
        draftOf({ pageKey: 'null-menu', menuId: null }),
        noTitle,
      ],
      total: 4,
      page: 1,
      pageSize: 200,
    } as never);

    renderPage();
    await screen.findByText('玩家页面');

    const editBtns = await screen.findAllByRole('button', { name: /编辑页面/ });
    expect(editBtns).toHaveLength(2);
    fireEvent.click(editBtns[0]);
    expect(umiMock.history.push).toHaveBeenCalledWith('/functions/pages?focus=players');
  });

  it('拖拽到顶级空隙：parentId=null 以 0 兜底提交 updateMenu', async () => {
    mockedUpdateMenu.mockResolvedValue(treeItems[0] as never);
    mockedUpdateMenuSort.mockResolvedValue(treeItems[0] as never);
    renderPage();
    await screen.findByText('资源管理');

    // clientY=-1 使 rc-tree 命中「首个顶级节点上半区」→ dropPosition=-1（gap/顶级）
    const dataTransfer = makeDataTransfer();
    const evt = { dataTransfer, clientY: -1 };
    fireEvent.dragStart(rowOf('玩家管理'), evt);
    fireEvent.dragEnter(rowOf('资源管理'), evt);
    fireEvent.drop(rowOf('资源管理'), evt);

    await waitFor(() => {
      expect(mockedUpdateMenu).toHaveBeenCalledWith(2, { parentId: 0 });
    });
    expect(await screen.findByText('菜单排序已更新')).toBeInTheDocument();
  });
});
