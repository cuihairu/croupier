/**
 * Ops/Notifications 事件通知页回归测试
 *
 * 重点回归：
 * 1. 渠道/规则「删除」按钮必须经 Popconfirm 二次确认后才从 state 移除（本轮修复点）；
 * 2. ModalForm 新增/编辑（destroyOnHidden 重开重挂载，预填编辑值）、保存成功/失败、
 *    加载成功/失败等主交互路径。
 */
import React from 'react';
import { configure, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { App, ConfigProvider } from 'antd';
import OpsNotificationsPage from '../index';

// ModalForm/Table 在 jsdom 下渲染较重，放宽异步等待与单测超时
jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

jest.mock('@/services/api/ops', () => ({
  fetchOpsNotifications: jest.fn(),
  saveOpsNotifications: jest.fn(),
}));

const { fetchOpsNotifications, saveOpsNotifications } = jest.requireMock('@/services/api/ops') as {
  fetchOpsNotifications: jest.Mock;
  saveOpsNotifications: jest.Mock;
};

const INITIAL_CHANNELS = [
  { id: 'ding_main', type: 'dingtalk', url: 'https://oapi.example.com/ding/hook' },
  { id: 'feishu_main', type: 'feishu', url: 'https://open.example.com/feishu/hook' },
];
const INITIAL_RULES = [
  { event: 'certificate_expired', channels: ['ding_main'], thresholdDays: 7 },
  { event: 'audit_backlog', channels: ['feishu_main'], thresholdDays: 14 },
];

const renderPage = () =>
  render(
    <App>
      <ConfigProvider button={{ autoInsertSpace: false }}>
        <OpsNotificationsPage />
      </ConfigProvider>
    </App>,
  );

/** 渠道 id 会同时出现在渠道表格和规则 channels 标签里，统一用 AllBy 断言存在/消失 */
const awaitTextPresent = (text: string) =>
  waitFor(() => expect(screen.getAllByText(text).length).toBeGreaterThan(0));
const expectTextGone = (text: string) =>
  waitFor(() => expect(screen.queryAllByText(text)).toHaveLength(0));

/** Popconfirm 弹层容器（antd 默认英文 locale：OK / Cancel） */
const popconfirmRoot = async (title: string) => {
  await screen.findByText(title);
  const root = Array.from(document.querySelectorAll('.ant-popover')).find((node) =>
    node.textContent?.includes(title),
  );
  if (!root) throw new Error(`popconfirm ${title} not mounted`);
  return root as HTMLElement;
};

const clickPopconfirmOk = async (title: string) => {
  const root = await popconfirmRoot(title);
  const ok = within(root).getByRole('button', { name: 'OK' });
  fireEvent.click(ok);
  await expectTextGone(title);
};

/** 找到包含指定文本的表格行 */
const findRow = (text: string) => {
  const row = Array.from(document.querySelectorAll('.ant-table-row')).find((r) =>
    r.textContent?.includes(text),
  );
  if (!row) throw new Error(`row containing ${text} not found`);
  return row as HTMLElement;
};

/** 规则表格是页面里最后一个 ant Table */
const ruleTable = () => {
  const tables = document.querySelectorAll('.ant-table');
  return tables[tables.length - 1] as HTMLElement;
};

/** 渠道表格是页面里第一个 ant Table */
const channelTable = () => document.querySelector('.ant-table') as HTMLElement;

beforeEach(() => {
  fetchOpsNotifications.mockReset();
  saveOpsNotifications.mockReset();
  fetchOpsNotifications.mockResolvedValue({
    channels: INITIAL_CHANNELS.map((c) => ({ ...c })),
    rules: INITIAL_RULES.map((r) => ({ ...r })),
  });
  saveOpsNotifications.mockResolvedValue(undefined);
});

describe('Ops/Notifications 页面', () => {
  it('初始加载：渲染渠道与规则两张表', async () => {
    renderPage();
    await awaitTextPresent('ding_main');
    expect(screen.getAllByText('feishu_main').length).toBeGreaterThan(0);
    expect(screen.getByText('dingtalk')).toBeInTheDocument();
    expect(screen.getByText('certificate_expired')).toBeInTheDocument();
    expect(screen.getByText('audit_backlog')).toBeInTheDocument();
    expect(within(ruleTable()).getByText('7')).toBeInTheDocument();
    expect(fetchOpsNotifications).toHaveBeenCalledTimes(1);
  });

  it('加载失败：提示「加载失败」', async () => {
    fetchOpsNotifications.mockRejectedValue(new Error('network down'));
    renderPage();
    expect(await screen.findByText('加载失败')).toBeInTheDocument();
  });

  it('删除渠道：需 Popconfirm 确认，确认后渠道行移除', async () => {
    renderPage();
    await awaitTextPresent('ding_main');
    fireEvent.click(within(findRow('feishu_main')).getByRole('button', { name: '删除' }));

    // 弹出确认且未确认前行仍在
    await popconfirmRoot('确认删除该渠道？');
    expect(within(channelTable()).getByText('feishu_main')).toBeInTheDocument();
    await clickPopconfirmOk('确认删除该渠道？');

    // 渠道表行减少；规则表里的 channels 标签不受影响（页面只删渠道 state）
    await waitFor(() => expect(within(channelTable()).queryByText('feishu_main')).toBeNull());
    expect(channelTable().querySelectorAll('.ant-table-row')).toHaveLength(1);
    expect(within(ruleTable()).getAllByText('feishu_main').length).toBeGreaterThan(0);
  });

  it('删除渠道：点 Cancel 不删除', async () => {
    renderPage();
    await awaitTextPresent('ding_main');
    fireEvent.click(within(findRow('feishu_main')).getByRole('button', { name: '删除' }));
    const root = await popconfirmRoot('确认删除该渠道？');
    fireEvent.click(within(root).getByRole('button', { name: 'Cancel' }));
    // 未确认：渠道行原样保留
    expect(within(channelTable()).getByText('feishu_main')).toBeInTheDocument();
    expect(channelTable().querySelectorAll('.ant-table-row')).toHaveLength(2);
  });

  it('删除规则：需 Popconfirm 确认，确认后规则行移除', async () => {
    renderPage();
    await awaitTextPresent('ding_main');
    fireEvent.click(within(findRow('audit_backlog')).getByRole('button', { name: '删除' }));

    await popconfirmRoot('确认删除该规则？');
    expect(screen.getByText('audit_backlog')).toBeInTheDocument();
    await clickPopconfirmOk('确认删除该规则？');

    await expectTextGone('audit_backlog');
    expect(screen.getByText('certificate_expired')).toBeInTheDocument();
  });

  it('新增渠道：ID 必填校验 + 提交后新增一行', async () => {
    renderPage();
    await awaitTextPresent('ding_main');
    fireEvent.click(screen.getByRole('button', { name: '新增渠道' }));

    await screen.findByText('通知渠道');
    fireEvent.click(screen.getByRole('button', { name: '确定' }));
    // ID 为空被表单 required 拦截
    expect(await screen.findByText('请输入渠道ID（用于规则引用）')).toBeInTheDocument();
    expect(screen.queryAllByText('ding_new')).toHaveLength(0);

    fireEvent.change(screen.getByPlaceholderText('如 ding_main'), {
      target: { value: 'ding_new' },
    });
    fireEvent.change(screen.getByPlaceholderText('https://...'), {
      target: { value: 'https://oapi.example.com/new' },
    });
    fireEvent.click(screen.getByRole('button', { name: '确定' }));

    await awaitTextPresent('ding_new');
    await expectTextGone('通知渠道');
  });

  it('编辑渠道：弹窗预填原值，提交后更新对应行', async () => {
    renderPage();
    await awaitTextPresent('ding_main');
    fireEvent.click(within(findRow('ding_main')).getByRole('button', { name: '编辑' }));

    await screen.findByText('通知渠道');
    const idInput = screen.getByPlaceholderText('如 ding_main') as HTMLInputElement;
    expect(idInput.value).toBe('ding_main');
    fireEvent.change(screen.getByPlaceholderText('https://...'), {
      target: { value: 'https://oapi.example.com/updated' },
    });
    fireEvent.click(screen.getByRole('button', { name: '确定' }));

    await awaitTextPresent('https://oapi.example.com/updated');
    await expectTextGone('通知渠道');
    // 编辑而非新增：行数不增
    expect(document.querySelectorAll('.ant-table-row')).toHaveLength(4);
  });

  it('渠道弹窗：点取消关闭且不改动数据', async () => {
    renderPage();
    await awaitTextPresent('ding_main');
    fireEvent.click(screen.getByRole('button', { name: '新增渠道' }));
    await screen.findByText('通知渠道');
    // ModalForm footer 的取消按钮（非「确定」的那个）
    const footer = document.querySelector('.ant-modal-footer') as HTMLElement;
    const cancelBtn = Array.from(within(footer).getAllByRole('button')).find(
      (b) => b.textContent !== '确定',
    ) as HTMLElement;
    fireEvent.click(cancelBtn);
    await expectTextGone('通知渠道');
    expect(document.querySelectorAll('.ant-table-row')).toHaveLength(4);
  });

  it('新增规则：渠道必填 + 选择渠道后新增一行', async () => {
    renderPage();
    await awaitTextPresent('ding_main');
    fireEvent.click(screen.getByRole('button', { name: '新增规则' }));

    await screen.findByText('通知规则');
    // channels 必填：空提交被拦截
    fireEvent.click(screen.getByRole('button', { name: '确定' }));
    await waitFor(() => {
      const errors = Array.from(document.querySelectorAll('.ant-form-item-explain-error')).map(
        (n) => n.textContent,
      );
      expect(errors.some((t) => (t || '').includes('渠道'))).toBe(true);
    });
    // 初始 rules 不含 certificate_expiring，提交后应为新增行
    expect(screen.queryAllByText('certificate_expiring')).toHaveLength(0);

    // 多选渠道下拉（规则弹窗内唯一的 multiple Select；antd6 无 .ant-select-selector）
    const selector = document.querySelector('.ant-select-multiple');
    if (!selector) throw new Error('channels multiple select not found');
    fireEvent.mouseDown(selector as HTMLElement);
    const option = await waitFor(() => {
      const dropdown = document.querySelector(
        '.ant-select-dropdown:not(.ant-select-dropdown-hidden)',
      );
      expect(dropdown).toBeTruthy();
      const hit = Array.from(dropdown?.querySelectorAll('.ant-select-item-option') || []).find(
        (o) => o.textContent === 'ding_main (dingtalk)',
      );
      expect(hit).toBeTruthy();
      return hit as HTMLElement;
    });
    fireEvent.click(option);
    fireEvent.click(screen.getByRole('button', { name: '确定' }));

    await waitFor(() => {
      expect(within(ruleTable()).getAllByText('certificate_expiring').length).toBeGreaterThan(0);
    });
    await expectTextGone('通知规则');
  });

  it('编辑规则：预填后提交，替换同事件行', async () => {
    renderPage();
    await awaitTextPresent('ding_main');
    fireEvent.click(within(findRow('certificate_expired')).getByRole('button', { name: '编辑' }));

    await screen.findByText('通知规则');
    fireEvent.click(screen.getByRole('button', { name: '确定' }));

    // 同 event 替换而非新增
    await expectTextGone('通知规则');
    expect(within(ruleTable()).getAllByText('certificate_expired').length).toBe(1);
    expect(document.querySelectorAll('.ant-table-row')).toHaveLength(4);
  });

  it('保存成功：saveOpsNotifications 携带当前 channels/rules 并提示已保存', async () => {
    renderPage();
    await awaitTextPresent('ding_main');
    fireEvent.click(screen.getByRole('button', { name: '保存' }));
    await screen.findByText('已保存');
    await waitFor(() =>
      expect(saveOpsNotifications).toHaveBeenCalledWith({
        channels: INITIAL_CHANNELS.map((c) => expect.objectContaining({ id: c.id })),
        rules: INITIAL_RULES.map((r) => expect.objectContaining({ event: r.event })),
      }),
    );
  });

  it('保存失败：提示「保存失败」', async () => {
    renderPage();
    await awaitTextPresent('ding_main');
    saveOpsNotifications.mockRejectedValue(new Error('server error'));
    fireEvent.click(screen.getByRole('button', { name: '保存' }));
    expect(await screen.findByText('保存失败')).toBeInTheDocument();
  });
});
