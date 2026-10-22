/**
 * 事故类别设置页单测（docs/design/incident-reports.md §6）：
 * - 内建类别删除 → Modal.warning 拦截（不删、不调 usage）；
 * - 自定义类别删除 → 先查 usage 再二次确认，确认后 deleteIncidentCategory；
 * - 编辑器：slug 正则校验（非法 slug 拦截提交）。
 *
 * mock 口径：incident 服务全 mock；@umijs/max 走 defaultMessage；
 * Modal.confirm 是静态 portal——按 document 查询。
 */
import React from 'react';
import { configure, fireEvent, render, screen, waitFor } from '@testing-library/react';
import IncidentCategoriesPage from '../index';
import {
  createIncidentCategory,
  deleteIncidentCategory,
  fetchIncidentCategories,
  fetchIncidentCategoryUsage,
  type IncidentCategory,
} from '@/services/api/incident';

configure({ asyncUtilTimeout: 5000 });
jest.setTimeout(30000);

jest.mock('@umijs/max', () => ({
  __esModule: true,
  useIntl: () => ({
    formatMessage: ({ defaultMessage }: { defaultMessage?: string }) => defaultMessage || '',
  }),
  FormattedMessage: ({ defaultMessage }: { defaultMessage?: string }) => defaultMessage || '',
}));

jest.mock('@/services/api/incident', () => ({
  createIncidentCategory: jest.fn(),
  deleteIncidentCategory: jest.fn(),
  fetchIncidentCategories: jest.fn(),
  fetchIncidentCategoryUsage: jest.fn(),
  updateIncidentCategory: jest.fn(),
}));

const mList = fetchIncidentCategories as jest.MockedFunction<typeof fetchIncidentCategories>;
const mUsage = fetchIncidentCategoryUsage as jest.MockedFunction<typeof fetchIncidentCategoryUsage>;
const mDelete = deleteIncidentCategory as jest.MockedFunction<typeof deleteIncidentCategory>;
const mCreate = createIncidentCategory as jest.MockedFunction<typeof createIncidentCategory>;

const BUILTIN: IncidentCategory = {
  id: 1,
  name: '可用性',
  slug: 'availability',
  sort: 1,
  leader: 'alice',
  subcategories: [],
  enabled: true,
  builtin: true,
  createdAt: '',
  updatedAt: '',
};

const CUSTOM: IncidentCategory = {
  id: 2,
  name: '性能',
  slug: 'perf',
  sort: 2,
  leader: '',
  subcategories: [],
  enabled: true,
  builtin: false,
  createdAt: '',
  updatedAt: '',
};

beforeEach(() => {
  jest.clearAllMocks();
  mList.mockResolvedValue({ items: [BUILTIN, CUSTOM], total: 2 });
  mUsage.mockResolvedValue({ incidents: 3, bugs: 1 });
  mDelete.mockResolvedValue(undefined);
  mCreate.mockResolvedValue({ ...CUSTOM, id: 3 });
});

const clickDelete = async (name: string) => {
  const row = await screen.findByText(name);
  const rowEl = row.closest('tr');
  const anchors = rowEl!.querySelectorAll('a');
  fireEvent.click(anchors[anchors.length - 1]);
};

/** 静态 Modal 跨用例残留会叠多个确认框——按钮查询限定最后一个 */
const lastModalButton = (re: RegExp): HTMLButtonElement => {
  const confirms = document.querySelectorAll('.ant-modal-confirm');
  const btns = Array.from(
    confirms[confirms.length - 1].querySelectorAll<HTMLButtonElement>('button'),
  );
  const hit = btns.find((b) => re.test(b.textContent || ''));
  if (!hit) throw new Error(`button ${re} not found`);
  return hit;
};

describe('IncidentCategories 页', () => {
  it('内建类别：删除被 Modal.warning 拦截，不调 usage/删除', async () => {
    render(<IncidentCategoriesPage />);
    await clickDelete('可用性');
    // antd6 静态 Modal 标题在 aria 快照里渲染两份
    expect((await screen.findAllByText('内建类别不可删除')).length).toBeGreaterThan(0);
    expect(mUsage).not.toHaveBeenCalled();
    expect(mDelete).not.toHaveBeenCalled();
    // 关掉警告弹窗（其唯一按钮即确认），避免残留污染后续用例
    fireEvent.click(lastModalButton(/./));
  });

  it('自定义类别：usage 统计 → 二次确认 → 删除', async () => {
    render(<IncidentCategoriesPage />);
    await clickDelete('性能');
    // 第一层确认的正文即统计文案；usage 统计发生在其 onOk（点 OK）之后
    await screen.findByText('正在统计该类别下的存量数据…');
    fireEvent.click(lastModalButton(/删除并归并/));
    await waitFor(() => expect(mUsage).toHaveBeenCalledWith(2));
    expect((await screen.findAllByText('确认删除并归并？')).length).toBeGreaterThan(0);
    fireEvent.click(lastModalButton(/OK|确\s*定/));
    await waitFor(() => expect(mDelete).toHaveBeenCalledWith(2));
  });

  it('编辑器：非法 slug 拦截提交', async () => {
    render(<IncidentCategoriesPage />);
    fireEvent.click(await screen.findByRole('button', { name: /新建类别/ }));
    const nameInput = document.querySelector<HTMLInputElement>('.ant-modal input#name');
    fireEvent.change(nameInput!, { target: { value: '新类别' } });
    const slugInput = document.querySelector<HTMLInputElement>('.ant-modal input#slug');
    fireEvent.change(slugInput!, { target: { value: 'Bad_Slug' } });
    fireEvent.click(screen.getByRole('button', { name: /保存/ }));
    await screen.findByText('小写字母/数字/中划线');
    expect(mCreate).not.toHaveBeenCalled();
  });
});
