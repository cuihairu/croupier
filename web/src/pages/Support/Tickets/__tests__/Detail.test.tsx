/** Support/Tickets/Detail 回归与主路径覆盖。
 *
 * 回归点（本轮修复，必须持续守护）：
 * 1. 空评论（trim 后为空）提交 → getMessage().warning 且不发请求；
 * 2. submittingCmt state 控制「提交评论」按钮 loading，防双击重复提交；
 * 3. 按钮 disabled={!cmt.trim()}：纯空白输入时禁用。
 *
 * 其余主路径：详情渲染与枚举回退、评论附件（图片/文件/坏 JSON）、
 * 流转弹窗、编辑弹窗（ModalForm）、删除确认、指派给我、升级为缺陷、
 * 满意度评价、附件上传与移除。 */
import React from 'react';
import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { App } from 'antd';
import { useParams, history, useModel } from '@umijs/max';
import { uploadAsset } from '@/services/api/storage';
import {
  addTicketComment,
  convertTicketToBug,
  deleteTicket,
  getTicket,
  listTicketComments,
  rateTicket,
  transitionTicket,
  updateTicket,
} from '@/services/api/support';
import Detail from '../Detail';

jest.mock('@umijs/max', () => ({
  useParams: jest.fn(),
  history: { push: jest.fn(), back: jest.fn() },
  useModel: jest.fn(),
  // 与 tests/setupTests.jsx 同语义：返回 defaultMessage；
  // 额外做 {placeholder} 插值（真实 intl 行为），供「工单详情 #{id}」
  // 「已升级为缺陷 #{bugId}…」两条 ICU 模板串断言
  useIntl: () => ({
    formatMessage: ({ defaultMessage }, values) =>
      Object.entries(values || {}).reduce(
        (msg: string, [key, val]) => msg.split(`{${key}}`).join(String(val)),
        defaultMessage,
      ),
  }),
  FormattedMessage: ({ defaultMessage }) => defaultMessage,
}));

const mockMessageApi = {
  success: jest.fn(),
  error: jest.fn(),
  warning: jest.fn(),
  info: jest.fn(),
};
jest.mock('@/utils/antdApp', () => ({ getMessage: () => mockMessageApi }));

jest.mock('@/services/api/support');
jest.mock('@/services/api/storage', () => ({ uploadAsset: jest.fn() }));

const mockedUseParams = jest.mocked(useParams);
const mockedUseModel = jest.mocked(useModel);
const mockedHistoryPush = jest.mocked(history.push);
const mockedHistoryBack = jest.mocked(history.back);
const mockedGetTicket = jest.mocked(getTicket);
const mockedListTicketComments = jest.mocked(listTicketComments);
const mockedAddTicketComment = jest.mocked(addTicketComment);
const mockedUpdateTicket = jest.mocked(updateTicket);
const mockedDeleteTicket = jest.mocked(deleteTicket);
const mockedConvertTicketToBug = jest.mocked(convertTicketToBug);
const mockedRateTicket = jest.mocked(rateTicket);
const mockedTransitionTicket = jest.mocked(transitionTicket);
const mockedUploadAsset = jest.mocked(uploadAsset);

const baseTicket = {
  id: 1,
  title: '登录失败',
  status: 'inProgress',
  priority: 'high',
  assignee: 'bob',
  tags: ['lag'],
  playerId: 'p-1',
  gameId: 'demo',
  createdAt: '2024-01-01T00:00:00Z',
  updatedAt: '2024-01-02T00:00:00Z',
  content: '无法登录',
  contact: 'wechat:x',
  env: 'prod',
  source: 'web',
};

function renderDetail() {
  return render(
    <App>
      <Detail />
    </App>,
  );
}

function cmtTextarea(): HTMLTextAreaElement {
  return screen.getByPlaceholderText('输入评论内容') as HTMLTextAreaElement;
}

function submitCmtButton(): HTMLButtonElement {
  return screen.getByRole('button', { name: '提交评论' }) as HTMLButtonElement;
}

describe('Support/Tickets/Detail', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    window.open = jest.fn();
    mockedUseParams.mockReturnValue({ id: '1' });
    mockedUseModel.mockReturnValue({ initialState: { currentUser: { name: 'alice' } } });
    mockedGetTicket.mockResolvedValue(baseTicket);
    mockedListTicketComments.mockResolvedValue({ comments: [] });
    mockedAddTicketComment.mockResolvedValue({ comments: [], items: [] });
    mockedUpdateTicket.mockResolvedValue(baseTicket);
    mockedDeleteTicket.mockResolvedValue(undefined);
    mockedTransitionTicket.mockResolvedValue({});
  });

  it('渲染工单详情：标题/状态/优先级标签与字段', async () => {
    renderDetail();
    expect(await screen.findByText('工单详情 #1')).toBeInTheDocument();
    expect(screen.getByText('登录失败')).toBeInTheDocument();
    expect(screen.getByText('处理中')).toBeInTheDocument();
    expect(screen.getByText('高')).toBeInTheDocument();
    expect(screen.getByText('bob')).toBeInTheDocument();
    expect(screen.getByText('demo/prod')).toBeInTheDocument();
    expect(screen.getByText('无法登录')).toBeInTheDocument();
    expect(mockedGetTicket).toHaveBeenCalledWith('1');
    expect(mockedListTicketComments).toHaveBeenCalledWith('1');
  });

  it('可选字段缺失时以 - 兜底；未知枚举显示原值', async () => {
    mockedGetTicket.mockResolvedValue({
      ...baseTicket,
      priority: undefined,
      assignee: undefined,
      tags: undefined,
      playerId: undefined,
      contact: undefined,
      content: undefined,
      createdAt: '',
      updatedAt: '',
      status: 'weird',
      source: undefined,
    });
    renderDetail();
    await screen.findByText('工单详情 #1');
    const dashCells = screen.getAllByText('-');
    expect(dashCells.length).toBeGreaterThanOrEqual(5);
    expect(screen.getByText('weird')).toBeInTheDocument();
  });

  it('路由无 id 时渲染空且不发请求', () => {
    mockedUseParams.mockReturnValue({});
    const { container } = renderDetail();
    expect(container.querySelector('.ant-card')).not.toBeInTheDocument();
    expect(mockedGetTicket).not.toHaveBeenCalled();
  });

  it('返回按钮调用 history.back', async () => {
    renderDetail();
    await screen.findByText('工单详情 #1');
    fireEvent.click(screen.getByRole('button', { name: '< 返回' }));
    expect(mockedHistoryBack).toHaveBeenCalled();
  });

  describe('评论列表', () => {
    it('渲染图片/文件附件；坏 JSON 与无附件评论安全降级', async () => {
      mockedListTicketComments.mockResolvedValue({
        comments: [
          {
            id: 1,
            content: '看截图',
            author: 'alice',
            createdAt: '2024-01-03T00:00:00Z',
            attach: JSON.stringify([{ name: 'shot', url: 'http://x/a.png' }]),
          },
          {
            id: 2,
            content: '日志文件',
            createdAt: '2024-01-04T00:00:00Z',
            attach: JSON.stringify([{ name: 'log', url: 'http://x/log.txt' }]),
          },
          { id: 3, content: '坏数据', attach: '{bad json' },
          { id: 4, content: '无附件内容' },
        ],
      });
      const { container } = renderDetail();
      await screen.findByText('看截图');
      const img = container.querySelector('img[src="http://x/a.png"]');
      expect(img).toBeInTheDocument();
      expect(screen.getByText('log')).toHaveAttribute('href', 'http://x/log.txt');
      expect(screen.getByText('坏数据')).toBeInTheDocument();
      expect(screen.getByText('无附件内容')).toBeInTheDocument();

      fireEvent.click(img as HTMLElement);
      expect(window.open).toHaveBeenCalledWith('http://x/a.png', '_blank');
    });
  });

  describe('提交评论（回归点）', () => {
    it('空内容（纯空白）点提交：warning 且不发请求；按钮同时处于 disabled', async () => {
      renderDetail();
      await screen.findByText('工单详情 #1');
      fireEvent.change(cmtTextarea(), { target: { value: '   ' } });
      const btn = submitCmtButton();
      expect(btn).toBeDisabled();
      expect(mockedAddTicketComment).not.toHaveBeenCalled();
      // disabled 是第一道防线（React 对 disabled 元素不派发合成 click，
      // DOM 层无法模拟点击）；直接调用挂载的 onClick 验证 submitComment
      // 的空校验兜底：warning 且不发请求
      // DOM 元素 fiber 上的 onClick 是 antd 内部 handleClick（disabled 分支
      // 拦截），向上遍历取业务组件挂载的用户 onClick
      const fiberKey = Object.keys(btn).find((k) => k.startsWith('__reactFiber$')) as string;
      expect(fiberKey).toBeTruthy();
      type FiberNode = {
        memoizedProps?: { onClick?: (e: unknown) => void };
        return?: FiberNode | null;
      };
      let cursor = (btn as unknown as Record<string, unknown>)[fiberKey] as FiberNode | undefined;
      let userClick: ((e: unknown) => void) | undefined;
      while (cursor) {
        const candidate = cursor.memoizedProps?.onClick;
        if (typeof candidate === 'function') userClick = candidate;
        cursor = cursor.return ?? undefined;
      }
      expect(userClick).toBeInstanceOf(Function);
      await act(async () => {
        userClick?.({
          preventDefault: () => undefined,
          stopPropagation: () => undefined,
        });
      });
      expect(mockMessageApi.warning).toHaveBeenCalledWith('评论内容不能为空');
      expect(mockedAddTicketComment).not.toHaveBeenCalled();
    });

    it('填内容提交：addTicketComment 被调、输入框清空、刷新详情', async () => {
      renderDetail();
      await screen.findByText('工单详情 #1');
      fireEvent.change(cmtTextarea(), { target: { value: 'hello' } });
      fireEvent.click(submitCmtButton());
      await waitFor(() =>
        expect(mockedAddTicketComment).toHaveBeenCalledWith('1', {
          content: 'hello',
          attach: '[]',
        }),
      );
      await waitFor(() => expect(cmtTextarea().value).toBe(''));
      // 提交成功后 load() 重新拉取详情与评论
      await waitFor(() => expect(mockedGetTicket).toHaveBeenCalledTimes(2));
    });

    it('提交中按钮 loading 且 disabled（防双击重复提交）', async () => {
      let release: () => void = () => {};
      mockedAddTicketComment.mockImplementationOnce(
        () =>
          new Promise<{ comments: []; items: [] }>((resolve) => {
            release = () => resolve({ comments: [], items: [] });
          }),
      );
      renderDetail();
      await screen.findByText('工单详情 #1');
      fireEvent.change(cmtTextarea(), { target: { value: 'slow' } });
      const btn = submitCmtButton();
      fireEvent.click(btn);
      // loading 期间按钮 accessible name 随 icon 变化，改用 DOM 引用断言
      await waitFor(() => expect(btn.className).toContain('ant-btn-loading'));
      // loading 态 antd 拦截 click：提交中再点不产生第二次请求
      fireEvent.click(btn);
      expect(mockedAddTicketComment).toHaveBeenCalledTimes(1);
      await act(async () => {
        release();
      });
      // 全程仅一次请求（loading 拦截期间 + 释放后均无重复提交）
      expect(mockedAddTicketComment).toHaveBeenCalledTimes(1);
    });

    it('提交失败：error 提示且输入内容保留', async () => {
      mockedAddTicketComment.mockRejectedValueOnce(new Error('网络错误'));
      renderDetail();
      await screen.findByText('工单详情 #1');
      fireEvent.change(cmtTextarea(), { target: { value: 'again' } });
      fireEvent.click(submitCmtButton());
      await waitFor(() => expect(mockMessageApi.error).toHaveBeenCalledWith('网络错误'));
      expect(cmtTextarea().value).toBe('again');
    });

    it('清空按钮重置输入与附件', async () => {
      renderDetail();
      await screen.findByText('工单详情 #1');
      fireEvent.change(cmtTextarea(), { target: { value: 'draft' } });
      fireEvent.click(screen.getByRole('button', { name: /清\s*空/ }));
      expect(cmtTextarea().value).toBe('');
    });
  });

  describe('附件上传', () => {
    function fileInput(container: HTMLElement): HTMLInputElement {
      return container.querySelector('input[type="file"]') as HTMLInputElement;
    }

    it('上传成功进入 fileList，提交评论携带附件 url', async () => {
      mockedUploadAsset.mockResolvedValueOnce({ Key: 'k1', URL: 'http://cdn/a.png' });
      const { container } = renderDetail();
      await screen.findByText('工单详情 #1');
      const file = new File(['x'], 'a.png', { type: 'image/png' });
      fireEvent.change(fileInput(container), { target: { files: [file] } });
      await waitFor(() => expect(mockedUploadAsset).toHaveBeenCalledWith(file));
      await screen.findByText('a.png');

      fireEvent.change(cmtTextarea(), { target: { value: '看图' } });
      fireEvent.click(submitCmtButton());
      await waitFor(() => expect(mockedAddTicketComment).toHaveBeenCalled());
      const payload = mockedAddTicketComment.mock.calls[0][1];
      const attach = JSON.parse(String(payload.attach)) as Array<{
        name?: string;
        url?: string;
      }>;
      expect(attach).toEqual([{ name: 'a.png', url: 'http://cdn/a.png', key: undefined }]);
    });

    it('移除附件后提交不带附件', async () => {
      mockedUploadAsset.mockResolvedValueOnce({ Key: 'k1', URL: 'http://cdn/b.png' });
      const { container } = renderDetail();
      await screen.findByText('工单详情 #1');
      fireEvent.change(fileInput(container), {
        target: { files: [new File(['x'], 'b.png', { type: 'image/png' })] },
      });
      await screen.findByText('b.png');
      fireEvent.click(container.querySelector('.anticon-delete') as HTMLElement);
      await waitFor(() => expect(screen.queryByText('b.png')).toBeNull());

      fireEvent.change(cmtTextarea(), { target: { value: 'no attach' } });
      fireEvent.click(submitCmtButton());
      await waitFor(() =>
        expect(mockedAddTicketComment).toHaveBeenCalledWith('1', {
          content: 'no attach',
          attach: '[]',
        }),
      );
    });

    it('上传失败：error 提示', async () => {
      mockedUploadAsset.mockRejectedValueOnce(new Error('存储不可用'));
      const { container } = renderDetail();
      await screen.findByText('工单详情 #1');
      fireEvent.change(fileInput(container), {
        target: { files: [new File(['x'], 'c.png', { type: 'image/png' })] },
      });
      await waitFor(() => expect(mockMessageApi.error).toHaveBeenCalledWith('存储不可用'));
    });
  });

  describe('工单流转', () => {
    it('提交流转：transitionTicket 被调、弹窗关闭、刷新详情', async () => {
      renderDetail();
      await screen.findByText('工单详情 #1');
      fireEvent.click(screen.getByRole('button', { name: /流\s*转/ }));
      const note = await screen.findByPlaceholderText('流转备注（可选）');
      fireEvent.change(note, { target: { value: '等待修复发布' } });
      fireEvent.click(screen.getByRole('button', { name: 'OK' }));
      await waitFor(() =>
        expect(mockedTransitionTicket).toHaveBeenCalledWith('1', {
          status: '',
          comment: '等待修复发布',
        }),
      );
      await waitFor(() => expect(screen.queryByPlaceholderText('选择状态')).toBeNull());
      await waitFor(() => expect(mockedGetTicket).toHaveBeenCalledTimes(2));
    });

    it('流转失败：error 提示且弹窗保持开启', async () => {
      mockedTransitionTicket.mockRejectedValueOnce(new Error('流转失败'));
      renderDetail();
      await screen.findByText('工单详情 #1');
      fireEvent.click(screen.getByRole('button', { name: /流\s*转/ }));
      await screen.findByPlaceholderText('流转备注（可选）');
      fireEvent.click(screen.getByRole('button', { name: 'OK' }));
      await waitFor(() => expect(mockMessageApi.error).toHaveBeenCalledWith('流转失败'));
      expect(screen.getByPlaceholderText('流转备注（可选）')).toBeInTheDocument();
    });
  });

  describe('编辑工单', () => {
    it('提交编辑：updateTicket 被调（已注册字段原值透传）并刷新详情', async () => {
      renderDetail();
      await screen.findByText('工单详情 #1');
      fireEvent.click(screen.getByRole('button', { name: '编辑工单' }));
      await screen.findByText('编辑工单', { selector: '.ant-modal-title' });
      // jsdom 下 pro-components ModalForm 的 input value 不回显（环境限制），
      // 输入交互不可用；此处断言提交链路：initialValues 已注册字段透传 + 刷新
      fireEvent.click(screen.getByRole('button', { name: /确\s*定/ }));
      await waitFor(() =>
        expect(mockedUpdateTicket).toHaveBeenCalledWith(
          1,
          expect.objectContaining({ title: '登录失败', status: 'inProgress' }),
        ),
      );
      await waitFor(() => expect(mockedGetTicket).toHaveBeenCalledTimes(2));
    });

    it('编辑失败：error 提示', async () => {
      mockedUpdateTicket.mockRejectedValueOnce(new Error('更新失败'));
      renderDetail();
      await screen.findByText('工单详情 #1');
      fireEvent.click(screen.getByRole('button', { name: '编辑工单' }));
      await screen.findByText('编辑工单', { selector: '.ant-modal-title' });
      fireEvent.click(screen.getByRole('button', { name: /确\s*定/ }));
      await waitFor(() => expect(mockMessageApi.error).toHaveBeenCalledWith('更新失败'));
    });
  });

  describe('删除工单', () => {
    it('确认删除：deleteTicket 被调并跳转列表', async () => {
      renderDetail();
      await screen.findByText('工单详情 #1');
      fireEvent.click(screen.getByRole('button', { name: '删除工单' }));
      await screen.findByText('确定删除该工单？');
      fireEvent.click(screen.getByRole('button', { name: 'OK' }));
      await waitFor(() => expect(mockedDeleteTicket).toHaveBeenCalledWith(1));
      await waitFor(() => expect(mockedHistoryPush).toHaveBeenCalledWith('/support/tickets'));
    });
  });

  describe('指派给我', () => {
    it('指派成功：以当前用户为处理人并刷新', async () => {
      renderDetail();
      await screen.findByText('工单详情 #1');
      fireEvent.click(screen.getByRole('button', { name: '指派给我' }));
      await waitFor(() =>
        expect(mockedUpdateTicket).toHaveBeenCalledWith(1, { assignee: 'alice' }),
      );
      await waitFor(() => expect(mockMessageApi.success).toHaveBeenCalledWith('已指派给我'));
      await waitFor(() => expect(mockedGetTicket).toHaveBeenCalledTimes(2));
    });

    it('未获取到当前用户：warning 且不请求', async () => {
      mockedUseModel.mockReturnValue({ initialState: {} });
      renderDetail();
      await screen.findByText('工单详情 #1');
      fireEvent.click(screen.getByRole('button', { name: '指派给我' }));
      await waitFor(() => expect(mockMessageApi.warning).toHaveBeenCalledWith('未获取到当前用户'));
      expect(mockedUpdateTicket).not.toHaveBeenCalled();
    });

    it('指派失败：error 提示', async () => {
      mockedUpdateTicket.mockRejectedValueOnce(new Error('指派失败'));
      renderDetail();
      await screen.findByText('工单详情 #1');
      fireEvent.click(screen.getByRole('button', { name: '指派给我' }));
      await waitFor(() => expect(mockMessageApi.error).toHaveBeenCalledWith('指派失败'));
    });
  });

  describe('升级为缺陷', () => {
    it('升级成功：success 提示带缺陷号并刷新', async () => {
      mockedConvertTicketToBug.mockResolvedValueOnce({ bugId: 77 });
      renderDetail();
      await screen.findByText('工单详情 #1');
      fireEvent.click(screen.getByRole('button', { name: '升级为缺陷' }));
      await waitFor(() =>
        expect(mockedConvertTicketToBug).toHaveBeenCalledWith(1, { steps: '无法登录' }),
      );
      await waitFor(() =>
        expect(mockMessageApi.success).toHaveBeenCalledWith('已升级为缺陷 #77（研发 → 缺陷追踪）'),
      );
      await waitFor(() => expect(mockedGetTicket).toHaveBeenCalledTimes(2));
    });

    it('升级失败：error 提示', async () => {
      mockedConvertTicketToBug.mockRejectedValueOnce(new Error('升级失败'));
      renderDetail();
      await screen.findByText('工单详情 #1');
      fireEvent.click(screen.getByRole('button', { name: '升级为缺陷' }));
      await waitFor(() => expect(mockMessageApi.error).toHaveBeenCalledWith('升级失败'));
    });
  });

  describe('满意度评价', () => {
    it('非 resolved/closed 状态显示提示文案不可评价', async () => {
      renderDetail();
      await screen.findByText('工单解决/关闭后可评价');
      expect(document.querySelectorAll('.ant-rate-star').length).toBe(0);
    });

    it('resolved 工单可评价：rateTicket 被调并刷新', async () => {
      mockedGetTicket.mockResolvedValue({ ...baseTicket, status: 'resolved' });
      renderDetail();
      await screen.findByText('已解决');
      const stars = document.querySelectorAll('.ant-rate-star');
      expect(stars.length).toBe(5);
      fireEvent.click((stars[2] as HTMLElement).querySelector('span') as HTMLElement);
      await waitFor(() => expect(mockedRateTicket).toHaveBeenCalledWith(1, 3));
      await waitFor(() => expect(mockMessageApi.success).toHaveBeenCalledWith('已记录评价'));
      await waitFor(() => expect(mockedGetTicket).toHaveBeenCalledTimes(2));
    });

    it('评价失败：error 提示', async () => {
      mockedGetTicket.mockResolvedValue({ ...baseTicket, status: 'closed' });
      mockedRateTicket.mockRejectedValueOnce(new Error('评价失败'));
      renderDetail();
      await screen.findByText('已关闭');
      fireEvent.click(
        document.querySelectorAll('.ant-rate-star')[1].querySelector('span') as HTMLElement,
      );
      await waitFor(() => expect(mockMessageApi.error).toHaveBeenCalledWith('评价失败'));
    });
  });
});
