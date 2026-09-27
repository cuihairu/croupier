/**
 * 系统公告 / 广播管理页回归（docs/BUGS.md BUG-021）。
 *
 * 后端 `/api/v1/admin/announcements` 的 List/Create/Update/Delete 一直存在，
 * 但前端**零引用**——公告功能此前完全没有入口；唯一的「发送消息」按钮却挂在
 * 所有用户都有的个人中心里。本页是 admin-only 的落地面。
 */
import React from 'react';
import { configure, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App } from 'antd';
import AnnouncementsPage from '../index';
import {
  createAnnouncement,
  deleteAnnouncement,
  listAnnouncements,
  updateAnnouncement,
} from '@/services/api/announcements';
import { boundaryTexts, timeEdges } from '../../../../../tests/fixtures/boundaryDataset';

// 负载/串行组合跑时 worker 显著变慢（单独跑 ~21s/套件，跟在 ResourceCatalog/
// templateUsage 这类重套件后面 90s+，隔离跑稳定绿）——RTL waitFor 默认 1s、
// jest 每 test 默认 5s 都会 whiplash（同用例先后报过「waitFor 内 Unable to
// find」与「Exceeded timeout」两种症状）。放宽到与 MenuManagement 同款预算
// （086e1f4 先例），断言逻辑零改动。
configure({ asyncUtilTimeout: 5000 });
jest.setTimeout(30000);

jest.mock('@/services/api/announcements', () => ({
  listAnnouncements: jest.fn(),
  createAnnouncement: jest.fn(),
  updateAnnouncement: jest.fn(),
  deleteAnnouncement: jest.fn(),
}));

const mList = listAnnouncements as jest.MockedFunction<typeof listAnnouncements>;
const mCreate = createAnnouncement as jest.MockedFunction<typeof createAnnouncement>;
const mUpdate = updateAnnouncement as jest.MockedFunction<typeof updateAnnouncement>;
const mDelete = deleteAnnouncement as jest.MockedFunction<typeof deleteAnnouncement>;

const row = (over: Partial<Awaited<ReturnType<typeof listAnnouncements>>['items'][number]>) => ({
  id: 1,
  title: '停机维护通知',
  contentMd: '**今晚 02:00** 停机维护',
  audience: 'all',
  popup: true,
  active: true,
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
  ...over,
});

function renderPage() {
  return render(
    <App>
      <AnnouncementsPage />
    </App>,
  );
}

beforeEach(() => {
  jest.clearAllMocks();
  mList.mockResolvedValue({ items: [], total: 0 });
  mCreate.mockResolvedValue(row({}) as never);
  mUpdate.mockResolvedValue(row({}) as never);
  mDelete.mockResolvedValue(undefined);
});

describe('AnnouncementsPage 列表', () => {
  it('加载并展示已有公告', async () => {
    mList.mockResolvedValue({ items: [row({})], total: 1 });
    renderPage();
    expect(await screen.findByText('停机维护通知')).toBeInTheDocument();
    expect(mList).toHaveBeenCalled();
  });

  // 数据集驱动示范（web/tests/fixtures/boundaryDataset.ts）：整组边界字段
  // 灌入列表，断言渲染健壮性与 XSS 转义，替代「单条写死假数据」的旧形态。
  it('边界数据集：超长/空白/emoji/RTL 渲染不崩，XSS 原样转义', async () => {
    mList.mockResolvedValue({
      items: [
        row({ id: 1, title: boundaryTexts.long }),
        row({ id: 2, title: boundaryTexts.whitespace }),
        row({ id: 3, title: boundaryTexts.padded, active: false }),
        row({ id: 4, title: boundaryTexts.emoji }),
        row({ id: 5, title: boundaryTexts.cjk }),
        row({ id: 6, title: boundaryTexts.rtl }),
        row({ id: 7, title: boundaryTexts.xss }),
        row({ id: 8, title: '同秒排序-A', createdAt: timeEdges.newYear }),
        row({ id: 9, title: '同秒排序-B', createdAt: timeEdges.newYear }),
        row({ id: 10, title: '同秒排序-C', createdAt: timeEdges.newYear }),
      ],
      total: 10,
    });
    const { container } = renderPage();
    // 超长/emoji/中日韩/RTL 全部渲染（文本在文档中可见，页面不崩）
    expect(await screen.findByText(boundaryTexts.emoji)).toBeInTheDocument();
    expect(screen.getByText(boundaryTexts.cjk)).toBeInTheDocument();
    expect(screen.getByText(boundaryTexts.rtl)).toBeInTheDocument();
    expect(screen.getByText(boundaryTexts.long)).toBeInTheDocument();
    // XSS：文本原样转义出现，且不产生 script 元素
    expect(screen.getByText(boundaryTexts.xss)).toBeInTheDocument();
    expect(container.querySelectorAll('script')).toHaveLength(0);
    // 同 CreatedAt 三条全部渲染（同秒排序不丢行）
    expect(screen.getByText('同秒排序-A')).toBeInTheDocument();
    expect(screen.getByText('同秒排序-B')).toBeInTheDocument();
    expect(screen.getByText('同秒排序-C')).toBeInTheDocument();
  });

  it('空态给出明确文案', async () => {
    renderPage();
    expect(await screen.findByText('还没有公告')).toBeInTheDocument();
  });

  it('受众区分全体 / 指定角色', async () => {
    mList.mockResolvedValue({
      items: [row({ id: 1, audience: 'all' }), row({ id: 2, audience: 'role', role: 'ops' })],
      total: 2,
    });
    renderPage();
    expect(await screen.findByText('全体用户')).toBeInTheDocument();
    expect(await screen.findByText('角色：ops')).toBeInTheDocument();
  });

  it('停用与未弹窗的标记如实展示', async () => {
    mList.mockResolvedValue({ items: [row({ active: false, popup: false })], total: 1 });
    renderPage();
    expect(await screen.findByText('已停用')).toBeInTheDocument();
    expect(screen.queryByText('弹窗提示')).toBeNull();
    expect(screen.queryByText('生效中')).toBeNull();
  });

  it('列表加载失败时报错而不是静默', async () => {
    mList.mockRejectedValue(new Error('boom'));
    renderPage();
    await waitFor(() => expect(mList).toHaveBeenCalled());
    // 失败后仍渲染页面（空表），不白屏
    expect(await screen.findByText('还没有公告')).toBeInTheDocument();
  });
});

describe('AnnouncementsPage 发布', () => {
  it('填必填项后创建，提交 audience/popup/active', async () => {
    renderPage();
    fireEvent.click(await screen.findByTestId('announcement-create'));
    fireEvent.change(await screen.findByTestId('announcement-title'), {
      target: { value: '新公告' },
    });
    fireEvent.change(await screen.findByTestId('announcement-content'), {
      target: { value: '正文' },
    });
    fireEvent.click(screen.getByTestId('announcement-save'));
    await waitFor(() => expect(mCreate).toHaveBeenCalled());
    expect(mCreate).toHaveBeenCalledWith(
      expect.objectContaining({ title: '新公告', contentMd: '正文', audience: 'all' }),
    );
  });

  it('audience=role 时必须填角色名', async () => {
    renderPage();
    fireEvent.click(await screen.findByTestId('announcement-create'));
    fireEvent.change(await screen.findByTestId('announcement-title'), {
      target: { value: 'X' },
    });
    fireEvent.change(await screen.findByTestId('announcement-content'), {
      target: { value: 'Y' },
    });
    // 切到「指定角色」
    fireEvent.mouseDown(await screen.findByTestId('announcement-audience'));
    fireEvent.click(await screen.findByTitle('指定角色'));
    fireEvent.click(screen.getByTestId('announcement-save'));
    await waitFor(() => expect(screen.getAllByText('请填写角色名').length).toBeGreaterThan(0));
    expect(mCreate).not.toHaveBeenCalled();
    // 校验失败不得变成 unhandled rejection（submit 必须接住 validateFields 的 reject）
    expect(screen.getByTestId('announcement-save')).toBeInTheDocument();
  });

  it('创建成功后重新拉取列表', async () => {
    mCreate.mockResolvedValue({ id: 9 } as never);
    renderPage();
    fireEvent.click(await screen.findByTestId('announcement-create'));
    fireEvent.change(await screen.findByTestId('announcement-title'), { target: { value: 'T' } });
    fireEvent.change(await screen.findByTestId('announcement-content'), { target: { value: 'C' } });
    fireEvent.click(screen.getByTestId('announcement-save'));
    await waitFor(() => expect(mCreate).toHaveBeenCalledTimes(1));
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(2));
  });
});

describe('AnnouncementsPage 编辑 / 删除', () => {
  it('编辑走 update 且带 id', async () => {
    mList.mockResolvedValue({ items: [row({ id: 5, title: '旧标题' })], total: 1 });
    renderPage();
    fireEvent.click(await screen.findByTestId('announcement-edit-5'));
    await waitFor(() => expect(screen.getByTestId('announcement-title')).toBeInTheDocument());
    fireEvent.change(screen.getByTestId('announcement-title'), { target: { value: '新标题' } });
    fireEvent.click(screen.getByTestId('announcement-save'));
    await waitFor(() => expect(mUpdate).toHaveBeenCalled());
    expect(mUpdate).toHaveBeenCalledWith(5, expect.objectContaining({ title: '新标题' }));
    expect(mCreate).not.toHaveBeenCalled();
  });

  it('删除需二次确认，确认后调 delete', async () => {
    mList.mockResolvedValue({ items: [row({ id: 7 })], total: 1 });
    renderPage();
    fireEvent.click(await screen.findByTestId('announcement-delete-7'));
    fireEvent.click(await screen.findByTestId('announcement-delete-confirm-7'));
    await waitFor(() => expect(mDelete).toHaveBeenCalledWith(7));
  });
});
