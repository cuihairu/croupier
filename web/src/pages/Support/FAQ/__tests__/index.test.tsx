/**
 * Support/FAQ 列表页回归（#22 分类过滤选项服务端化）：
 * 1. 分类过滤不再是自由文本输入，选项来自服务端聚合接口
 *    （GET /api/v1/faqs/categories），渲染计数后缀，选择后作为 category
 *    查询参数下发（不从当前已过滤列表客户端推导）；
 * 2. 新建/编辑保存成功后分类选项重拉（epoch 写后刷新，计数随写更新）；
 * 3. 弹窗表单 Form.Item 子节点为单一元素——回归 `{' '}` 空白子元素导致
 *    rc-field-form 跳过注入（value/onChange 脱钩）的历史缺陷。
 */
import React from 'react';
import { configure, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App } from 'antd';
import SupportFAQPage from '../index';
import { useAccess } from '@umijs/max';
import {
  listFAQ,
  listFAQCategories,
  createFAQ,
  updateFAQ,
  deleteFAQ,
} from '@/services/api/support';

// ProTable 重查带内部 debounce，coverage instrumentation 下更慢：放宽超时
// （与 Support/Feedback 同法）
configure({ asyncUtilTimeout: 5000 });
jest.setTimeout(40000);

jest.mock('@umijs/max', () => ({
  useAccess: jest.fn(),
  history: { push: jest.fn(), back: jest.fn() },
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
  listFAQ: jest.fn(),
  listFAQCategories: jest.fn(),
  createFAQ: jest.fn(),
  updateFAQ: jest.fn(),
  deleteFAQ: jest.fn(),
}));

const mockedUseAccess = useAccess as unknown as jest.Mock;
const mockListFAQ = listFAQ as unknown as jest.Mock;
const mockCategories = listFAQCategories as unknown as jest.Mock;
const mockCreateFAQ = createFAQ as unknown as jest.Mock;
const mockUpdateFAQ = updateFAQ as unknown as jest.Mock;
const mockDeleteFAQ = deleteFAQ as unknown as jest.Mock;

const renderPage = () =>
  render(
    <App>
      <SupportFAQPage />
    </App>,
  );

const openDropdownOptions = async (): Promise<(string | null)[]> => {
  await waitFor(() =>
    expect(document.querySelectorAll('.ant-select-dropdown .ant-select-item-option').length),
  );
  return Array.from(document.querySelectorAll('.ant-select-dropdown .ant-select-item-option')).map(
    (el) => el.textContent,
  );
};

// 测试环境 antd Select 不渲染 placeholder 文本：过滤区下拉按 DOM 顺序定位
// （渲染序 = 分类过滤 → 分页 pageSize 切换器，取第一个）
const categoryCombobox = (): HTMLElement => {
  const inputs = screen.getAllByRole('combobox');
  const input = inputs[0];
  if (!input) throw new Error('category combobox not found');
  return input as HTMLElement;
};

beforeEach(() => {
  jest.clearAllMocks();
  mockedUseAccess.mockReturnValue({ canSupportManage: true });
  mockListFAQ.mockResolvedValue({ faq: [], items: [], total: 0 });
  mockCategories.mockResolvedValue([
    { name: 'general', count: 5 },
    { name: 'technical', count: 2 },
  ]);
});

describe('Support/FAQ 列表页（#22 分类过滤选项服务端化）', () => {
  it('分类过滤不再是自由文本输入', async () => {
    renderPage();
    await waitFor(() => expect(mockListFAQ).toHaveBeenCalled());

    // 旧的 <Input placeholder="分类"> 已移除（占位文本不再以 input 形式存在）
    expect(screen.queryByPlaceholderText('分类')).toBeNull();
  });

  it('分类选项来自服务端聚合接口并渲染计数；选择后作为查询参数下发', async () => {
    renderPage();
    await waitFor(() => expect(mockCategories).toHaveBeenCalledTimes(1));

    fireEvent.mouseDown(categoryCombobox());
    expect(await openDropdownOptions()).toEqual(['general (5)', 'technical (2)']);

    fireEvent.click(document.querySelectorAll('.ant-select-dropdown .ant-select-item-option')[0]);
    await waitFor(() => {
      const lastCall = mockListFAQ.mock.calls[mockListFAQ.mock.calls.length - 1][0];
      expect(lastCall.category).toBe('general');
    });
  });

  it('新建 FAQ 成功后重拉分类选项（写后计数刷新），且表单控件已绑定 store', async () => {
    renderPage();
    await waitFor(() => expect(mockCategories).toHaveBeenCalledTimes(1));

    fireEvent.click(screen.getByText('新建 FAQ'));
    // 回归点：Form.Item 子节点必须是单一元素——`{' '}<Input/>{' '}` 三段式会让
    // rc-field-form 跳过注入（无 id/value/onChange），表单输入与 store 脱钩
    const questionInput = await screen.findByLabelText('问题');
    fireEvent.change(questionInput, { target: { value: '如何退款？' } });
    const answerInput = await screen.findByLabelText('答案');
    fireEvent.change(answerInput, { target: { value: '联系客服。' } });
    mockCreateFAQ.mockResolvedValue({ id: 3 });
    fireEvent.click(await screen.findByRole('button', { name: /确\s*定/ }));

    await waitFor(() =>
      expect(mockCreateFAQ).toHaveBeenCalledWith(
        expect.objectContaining({ question: '如何退款？', answer: '联系客服。' }),
      ),
    );
    await waitFor(() => expect(mockCategories).toHaveBeenCalledTimes(2));
    expect(mockListFAQ.mock.calls.length).toBeGreaterThanOrEqual(2);
  });

  it('编辑既有 FAQ 走 updateFAQ 而非 createFAQ', async () => {
    mockListFAQ.mockResolvedValue({
      faq: [{ id: 8, question: '旧问题', answer: '旧答案', category: 'general' }],
      total: 1,
    });
    renderPage();
    await screen.findByText('旧问题');

    fireEvent.click(screen.getByText('编辑'));
    const questionInput = await screen.findByLabelText('问题');
    expect(questionInput).toHaveValue('旧问题');
    fireEvent.change(questionInput, { target: { value: '新问题' } });
    mockUpdateFAQ.mockResolvedValue({ id: 8 });
    fireEvent.click(await screen.findByRole('button', { name: /确\s*定/ }));

    await waitFor(() =>
      expect(mockUpdateFAQ).toHaveBeenCalledWith(
        8,
        expect.objectContaining({ question: '新问题' }),
      ),
    );
    expect(mockCreateFAQ).not.toHaveBeenCalled();
  });
});
