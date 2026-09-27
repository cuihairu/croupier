/**
 * Assignments 执行闸门语义说明回归（OPEN-ISSUES #36 / BUG-032）
 *
 * 后端 EnsureFunctionAssigned 落地「默认开放 / 有分配记录即白名单」语义后，
 * 分配页必须把语义讲清（用户反馈 #36 ③），否则「未分配也能调用」的旧印象
 * 与「分配后调用 403」的新行为都会显得像 bug。
 */
import React from 'react';
import { render, screen } from '@testing-library/react';
import { App, ConfigProvider } from 'antd';
import AssignmentsPage from '../index';
import zhAssignments from '@/locales/zh-CN/assignments';
import enAssignments from '@/locales/en-US/assignments';
import { setScope } from '@/stores/scope';

jest.mock('@/services/api', () => ({
  listDescriptors: jest.fn().mockResolvedValue({ functions: [] }),
  fetchAssignments: jest.fn().mockResolvedValue({ assignments: {} }),
  fetchAssignmentsHistory: jest.fn().mockResolvedValue({ items: [], total: 0 }),
  setAssignments: jest.fn().mockResolvedValue({ ok: true }),
}));

jest.mock('@umijs/max', () => ({
  FormattedMessage: ({ defaultMessage }: { defaultMessage?: string }) => (
    <>{defaultMessage ?? ''}</>
  ),
  useIntl: () => ({
    formatMessage: (opts: { defaultMessage?: string }, values?: Record<string, string>) => {
      let text = opts.defaultMessage ?? '';
      if (values) {
        for (const [k, v] of Object.entries(values)) text = text.replace(`{${k}}`, v);
      }
      return text;
    },
  }),
  getIntl: () => ({
    formatMessage: (opts: { defaultMessage?: string }) => opts.defaultMessage ?? '',
  }),
  useModel: () => ({ initialState: { currentUser: { access: '*' } } }),
  history: { push: jest.fn(), replace: jest.fn() },
}));

describe('Assignments 执行闸门语义说明（#36 ③）', () => {
  it('页面渲染执行闸门 Alert：默认开放 / 白名单 / 403 / 清空恢复', () => {
    render(
      <App>
        <ConfigProvider>
          <AssignmentsPage />
        </ConfigProvider>
      </App>,
    );

    expect(screen.getByText('执行闸门')).toBeInTheDocument();
    const description = screen.getByText(/默认开放所有函数/);
    expect(description).toHaveTextContent('白名单');
    expect(description).toHaveTextContent('函数未开放执行权限');
    expect(description).toHaveTextContent('清空列表保存即恢复默认开放');
  });

  it('zh/en locale 均定义闸门语义键（en 不得回落中文 defaultMessage）', () => {
    for (const dict of [zhAssignments, enAssignments]) {
      const record = dict as Record<string, string>;
      expect(record['pages.assignments.gate.title']).toBeTruthy();
      expect(record['pages.assignments.gate.description']).toBeTruthy();
    }
    expect(enAssignments['pages.assignments.gate.title']).toBe('Execution gate');
    expect(zhAssignments['pages.assignments.gate.title']).toBe('执行闸门');
  });
});

// scope store 需要初始化（页面 useScope 读取）；置顶避免空 scope 噪声
beforeAll(() => {
  setScope({ gameId: 'default', env: 'dev' });
});
