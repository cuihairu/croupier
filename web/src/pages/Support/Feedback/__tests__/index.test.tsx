/**
 * Support/Feedback 页面回归：
 * 1. 「隐藏已转工单」Checkbox 默认勾选，ProTable request 传 excludeStatus='triaged'；
 * 2. 取消勾选后重查，excludeStatus 不再传（undefined）；
 * 3. 勾选但选择状态筛选时 excludeStatus 让位于 status；
 * 4. 关键词/分类/游戏筛选 + 查询按钮的参数透传；
 * 5. 新建/编辑 ModalForm（destroyOnHidden 预填）提交成功与失败路径；
 * 6. 删除二次确认（App.useApp modal.confirm）；
 * 7. 转工单：新转成功 / 已转过幂等提示 / 失败 toast；
 * 8. 列表加载失败降级（toast + 空表）；
 * 9. canSupportManage=false 隐藏管理按钮。
 */
import React from 'react';
import { render, screen, fireEvent, waitFor, configure } from '@testing-library/react';
import { App } from 'antd';
import SupportFeedbackPage from '../index';
import { useAccess } from '@umijs/max';
import {
  listFeedback,
  createFeedback,
  updateFeedback,
  deleteFeedback,
  convertFeedbackToTicket,
  type Feedback,
} from '@/services/api/support';
import { getMessage } from '@/utils/antdApp';

// ProTable 重查带内部 debounce，coverage instrumentation 下更慢：放宽异步查询与用例超时
configure({ asyncUtilTimeout: 5000 });
jest.setTimeout(20000);

jest.mock('@umijs/max', () => ({
  useAccess: jest.fn(),
  // 与 tests/setupTests.jsx 同语义：返回 defaultMessage；额外做 {placeholder}
  // 插值（真实 intl 行为），供「已转工单 #{ticketId}」等 ICU 模板串断言
  useIntl: () => ({
    formatMessage: ({ defaultMessage }, values) =>
      Object.entries(values || {}).reduce(
        (msg: string, [key, val]) => msg.split(`{${key}}`).join(String(val)),
        defaultMessage,
      ),
  }),
  FormattedMessage: ({ defaultMessage }) => defaultMessage,
}));

jest.mock('@/services/api/support', () => ({
  listFeedback: jest.fn(),
  createFeedback: jest.fn(),
  updateFeedback: jest.fn(),
  deleteFeedback: jest.fn(),
  convertFeedbackToTicket: jest.fn(),
}));

// 页面经 getMessage() 使用 App 上下文 message；工厂闭包保证断言与组件拿到同一 jest.fn
jest.mock('@/utils/antdApp', () => {
  const success = jest.fn();
  const error = jest.fn();
  return {
    __esModule: true,
    getMessage: () => ({ success, error }),
  };
});

const mockedUseAccess = useAccess as unknown as jest.Mock;
const messageApi = getMessage() as unknown as { success: jest.Mock; error: jest.Mock };
const mockList = listFeedback as unknown as jest.Mock;
const mockCreate = createFeedback as unknown as jest.Mock;
const mockUpdate = updateFeedback as unknown as jest.Mock;
const mockDelete = deleteFeedback as unknown as jest.Mock;
const mockConvert = convertFeedbackToTicket as unknown as jest.Mock;

function makeRow(overrides: Partial<Feedback>): Feedback {
  return {
    id: 0,
    playerId: '',
    contact: '',
    content: '',
    category: '',
    priority: '',
    status: '',
    rating: 0,
    attach: '',
    gameId: '',
    env: '',
    reply: '',
    createdAt: '2026-09-01T10:00:00Z',
    updatedAt: '2026-09-01T11:00:00Z',
    ...overrides,
  };
}

const rowNew = makeRow({
  id: 1,
  playerId: 'p1',
  contact: 'wx',
  content: '内容一',
  category: 'bug',
  priority: 'high',
  status: 'new',
  gameId: 'demo',
  env: 'prod',
});
const rowTriaged = makeRow({
  id: 2,
  playerId: 'p2',
  contact: 'qq',
  content: '内容二',
  category: 'idea',
  priority: 'low',
  status: 'triaged',
  gameId: 'demo',
  env: 'dev',
});
// 全空字段行：覆盖 gameId/env/playerId 等列渲染与转工单 note 的回退分支，
// updatedAt 显式缺失覆盖 formatDateTime(row.updatedAt ?? '') 的空值回退
const rowEmpty = makeRow({
  id: 3,
  status: 'new',
  updatedAt: undefined as unknown as string,
});

function listResponse(): Awaited<ReturnType<typeof listFeedback>> {
  return {
    feedback: [rowNew, rowTriaged, rowEmpty],
    items: [rowNew, rowTriaged, rowEmpty],
    total: 3,
    page: 1,
    size: 20,
  };
}

function renderPage() {
  return render(
    <App>
      <SupportFeedbackPage />
    </App>,
  );
}

describe('SupportFeedbackPage', () => {
  beforeEach(() => {
    mockList.mockReset().mockImplementation(() => Promise.resolve(listResponse()));
    mockCreate.mockReset();
    mockUpdate.mockReset();
    mockDelete.mockReset().mockResolvedValue(undefined);
    mockConvert.mockReset();
    messageApi.success.mockClear();
    messageApi.error.mockClear();
    mockedUseAccess.mockReset().mockReturnValue({ canSupportManage: true });
  });

  it('默认渲染：隐藏已转工单默认勾选，request 带 excludeStatus=triaged', async () => {
    renderPage();
    await waitFor(() => expect(screen.getByText('p1')).toBeInTheDocument());

    expect((screen.getByRole('checkbox') as HTMLInputElement).checked).toBe(true);
    expect(mockList).toHaveBeenCalledWith({
      q: '',
      category: '',
      status: '',
      gameId: '',
      page: 1,
      size: 20,
      excludeStatus: 'triaged',
    });
    // triaged 行按钮禁用并显示「已转工单」，new 行可转
    expect(screen.getByRole('button', { name: '已转工单' })).toBeDisabled();
    expect(screen.getAllByRole('button', { name: '转工单' }).length).toBe(2);
    expect(screen.getAllByRole('button', { name: '转工单' })[0]).toBeEnabled();
    // 管理员按钮（antd Button 对双汉字自动插空格，name 用正则）
    expect(screen.getByRole('button', { name: '新建反馈' })).toBeInTheDocument();
    expect(screen.getAllByRole('button', { name: /编\s*辑/ })).toHaveLength(3);
    expect(screen.getAllByRole('button', { name: /删\s*除/ })).toHaveLength(3);
    // 游戏/环境列
    expect(screen.getByText('demo/prod')).toBeInTheDocument();
    expect(screen.getByText('demo/dev')).toBeInTheDocument();
  });

  it('取消勾选「隐藏已转工单」：重查且 excludeStatus 不再传', async () => {
    renderPage();
    await waitFor(() => expect(screen.getByText('p1')).toBeInTheDocument());

    fireEvent.click(screen.getByRole('checkbox'));

    await waitFor(() =>
      expect(mockList).toHaveBeenLastCalledWith({
        q: '',
        category: '',
        status: '',
        gameId: '',
        page: 1,
        size: 20,
        excludeStatus: undefined,
      }),
    );
    expect((screen.getByRole('checkbox') as HTMLInputElement).checked).toBe(false);
  });

  it('勾选时选择状态筛选：excludeStatus 让位于 status', async () => {
    renderPage();
    await waitFor(() => expect(screen.getByText('p1')).toBeInTheDocument());

    // 页面存在两个 Select（筛选状态 + 分页 sizeChanger）：排除分页的那个
    const statusSelect = screen
      .getAllByRole('combobox')
      .map((el) => el.closest('.ant-select') as HTMLElement)
      .find((sel) => !sel.classList.contains('ant-pagination-options-size-changer')) as HTMLElement;
    fireEvent.mouseDown(statusSelect);
    // 下拉列表渲染在 body portal；虚拟列表另有同 role 占位，须按真实 option 类名定位
    const option = await waitFor(() => {
      const el = document.querySelector('.ant-select-item-option[title="已分流"]');
      if (!el) throw new Error('状态选项未渲染');
      return el as HTMLElement;
    });
    fireEvent.click(option);

    await waitFor(() =>
      expect(mockList).toHaveBeenLastCalledWith({
        q: '',
        category: '',
        status: 'triaged',
        gameId: '',
        page: 1,
        size: 20,
        excludeStatus: undefined,
      }),
    );
  });

  it('关键词/分类/游戏筛选 + 查询按钮：参数透传', async () => {
    renderPage();
    await waitFor(() => expect(screen.getByText('p1')).toBeInTheDocument());

    fireEvent.change(screen.getByPlaceholderText('关键词'), { target: { value: 'p1' } });
    fireEvent.change(screen.getByPlaceholderText('分类'), { target: { value: 'bug' } });
    fireEvent.change(screen.getByPlaceholderText('游戏'), { target: { value: 'demo' } });
    fireEvent.click(screen.getByRole('button', { name: /查\s*询/ }));

    await waitFor(() =>
      expect(mockList).toHaveBeenLastCalledWith({
        q: 'p1',
        category: 'bug',
        status: '',
        gameId: 'demo',
        page: 1,
        size: 20,
        excludeStatus: 'triaged',
      }),
    );
  });

  it('新建反馈：提交 createFeedback（含默认优先级/状态）并关闭弹窗', async () => {
    renderPage();
    await waitFor(() => expect(screen.getByText('p1')).toBeInTheDocument());

    fireEvent.click(screen.getByRole('button', { name: '新建反馈' }));
    const content = await screen.findByLabelText('内容');
    fireEvent.change(content, { target: { value: '新反馈内容' } });

    mockCreate.mockResolvedValueOnce(rowNew);
    fireEvent.click(screen.getByRole('button', { name: '确 定' }));

    await waitFor(() =>
      expect(mockCreate).toHaveBeenCalledWith(
        expect.objectContaining({ content: '新反馈内容', priority: 'normal', status: 'new' }),
      ),
    );
    // onFinish 返回 true：ModalForm 自动关闭
    await waitFor(() =>
      expect(screen.queryByRole('button', { name: '确 定' })).not.toBeInTheDocument(),
    );
  });

  it('编辑反馈：按选中行预填（destroyOnHidden 重开不残留）并提交 updateFeedback', async () => {
    renderPage();
    await waitFor(() => expect(screen.getByText('p1')).toBeInTheDocument());

    fireEvent.click(screen.getAllByRole('button', { name: /编\s*辑/ })[0]);
    expect(await screen.findByText('编辑反馈')).toBeInTheDocument();

    const content = screen.getByLabelText('内容');
    expect((content as HTMLTextAreaElement).value).toBe('内容一');
    fireEvent.change(content, { target: { value: '内容一改' } });

    mockUpdate.mockResolvedValueOnce(rowNew);
    fireEvent.click(screen.getByRole('button', { name: '确 定' }));

    await waitFor(() =>
      expect(mockUpdate).toHaveBeenCalledWith(
        1,
        expect.objectContaining({ content: '内容一改', playerId: 'p1' }),
      ),
    );
    await waitFor(() =>
      expect(screen.queryByRole('button', { name: '确 定' })).not.toBeInTheDocument(),
    );
  });

  it('提交失败：onFinish 返回 false，弹窗保持开启', async () => {
    renderPage();
    await waitFor(() => expect(screen.getByText('p1')).toBeInTheDocument());

    fireEvent.click(screen.getByRole('button', { name: '新建反馈' }));
    fireEvent.change(await screen.findByLabelText('内容'), { target: { value: '会失败' } });
    mockCreate.mockRejectedValueOnce(new Error('create failed'));
    fireEvent.click(screen.getByRole('button', { name: '确 定' }));

    await waitFor(() => expect(mockCreate).toHaveBeenCalledTimes(1));
    expect(await screen.findByRole('button', { name: '确 定' })).toBeInTheDocument();
  });

  it('删除：二次确认（modal.confirm）后调用 deleteFeedback', async () => {
    renderPage();
    await waitFor(() => expect(screen.getByText('p1')).toBeInTheDocument());

    fireEvent.click(screen.getAllByRole('button', { name: /删\s*除/ })[0]);
    // antd confirm 将 title 同时渲染于 modal-title 与 confirm-title，出现两次
    expect((await screen.findAllByText('删除反馈')).length).toBeGreaterThan(0);

    const okBtn = document.querySelector('.ant-modal-confirm-btns .ant-btn-primary') as HTMLElement;
    fireEvent.click(okBtn);

    await waitFor(() => expect(mockDelete).toHaveBeenCalledWith(1));
  });

  it('转工单：新转成功 / 已转过幂等提示 / 失败 toast / 空玩家ID 注「未知」', async () => {
    renderPage();
    await waitFor(() => expect(screen.getByText('p1')).toBeInTheDocument());

    mockConvert.mockResolvedValueOnce({ ticketId: 'T-9', alreadyConverted: false });
    fireEvent.click(screen.getAllByRole('button', { name: '转工单' })[0]);
    await waitFor(() => expect(messageApi.success).toHaveBeenCalledWith('已转工单 #T-9'));

    mockConvert.mockResolvedValueOnce({ ticketId: 'T-8', alreadyConverted: true });
    fireEvent.click(screen.getAllByRole('button', { name: '转工单' })[0]);
    await waitFor(() => expect(messageApi.success).toHaveBeenCalledWith('该反馈已转过工单 #T-8'));

    mockConvert.mockRejectedValueOnce(new Error('boom'));
    fireEvent.click(screen.getAllByRole('button', { name: '转工单' })[0]);
    await waitFor(() => expect(messageApi.error).toHaveBeenCalledWith('boom'));

    // 全空字段行（playerId 为空）：转工单 note 回退「未知」
    mockConvert.mockResolvedValueOnce({ ticketId: 'T-3', alreadyConverted: false });
    fireEvent.click(screen.getAllByRole('button', { name: '转工单' })[1]);
    await waitFor(() =>
      expect(mockConvert).toHaveBeenCalledWith(3, { note: '来源反馈#3 玩家:未知' }),
    );
  });

  it('列表加载失败：toast 错误并降级为空表', async () => {
    mockList.mockReset();
    mockList
      .mockRejectedValueOnce(new Error('load fail'))
      .mockImplementation(() => Promise.resolve(listResponse()));

    renderPage();
    await waitFor(() => expect(messageApi.error).toHaveBeenCalledWith('load fail'));
    expect(screen.queryByText('p1')).not.toBeInTheDocument();
  });

  it('列表响应缺失 feedback/total 字段：回退空数组与 0', async () => {
    mockList.mockImplementation(() =>
      Promise.resolve({} as unknown as Awaited<ReturnType<typeof listFeedback>>),
    );

    renderPage();
    await waitFor(() => expect(mockList).toHaveBeenCalledTimes(1));
    // request 正常返回 success:true，但行数据回退为空（antd Empty 占位）
    await waitFor(() => expect(document.querySelector('.ant-empty')).toBeInTheDocument());
    expect(screen.queryByText('p1')).not.toBeInTheDocument();
  });

  it('useAccess 未返回权限对象：按无权限渲染（|| {} 回退）', async () => {
    mockedUseAccess.mockReturnValue(undefined);
    renderPage();
    await waitFor(() => expect(screen.getByText('p1')).toBeInTheDocument());

    expect(screen.queryByRole('button', { name: /编\s*辑/ })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /删\s*除/ })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: '新建反馈' })).not.toBeInTheDocument();
  });

  it('canSupportManage=false：隐藏编辑/删除/新建按钮，转工单仍可用', async () => {
    mockedUseAccess.mockReturnValue({ canSupportManage: false });
    renderPage();
    await waitFor(() => expect(screen.getByText('p1')).toBeInTheDocument());

    expect(screen.queryByRole('button', { name: /编\s*辑/ })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /删\s*除/ })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: '新建反馈' })).not.toBeInTheDocument();
    expect(screen.getAllByRole('button', { name: '转工单' }).length).toBe(2);
  });
});
