/** PreviewDrawer onExecute 契约（round-7 覆盖率巡检，补 L32-38 抛错回调）：
 * 预览态不执行函数——PageRenderer 的执行入口必须收到「引导去运行控制台」的
 * 拒绝错误（与 EditorModal 预览同语义，共用 editor.previewExecuteError 文案键）。
 * PageRenderer 以模块桩替换：桩渲染一个触发 onExecute 的按钮并捕获 rejection，
 * 被测契约（throw + intl 文案）完全在 PreviewDrawer 内，桩仅替代子树渲染。 */
import React from 'react';
import { fireEvent, render, screen } from '@testing-library/react';
import { App } from 'antd';
import PreviewDrawer from '../PreviewDrawer';
import type { PageExecuteFn } from '@/types/dashboard';
import type { PageSpecDraft } from '@/types/dashboard';

jest.mock('@/components/PageRenderer', () => {
  // 工厂体内 require('react')（jest.mock 工厂禁止引用外层作用域变量）
  // eslint-disable-next-line @typescript-eslint/no-var-requires
  const { useState } = require('react') as typeof import('react');
  return {
    __esModule: true,
    default: function MockPageRenderer(props: { onExecute?: PageExecuteFn }) {
      const [err, setErr] = useState('');
      return (
        <div>
          <button
            onClick={() => {
              props
                .onExecute?.('b1', {} as Parameters<PageExecuteFn>[1])
                .catch((e: unknown) => setErr(e instanceof Error ? e.message : String(e)));
            }}
          >
            mock-run
          </button>
          <span data-testid="exec-err">{err}</span>
        </div>
      );
    },
  };
});

const draft = (pageKey = 'op.a'): PageSpecDraft => ({
  pageKey,
  type: 'operation',
  title: { 'zh-CN': pageKey, 'en-US': pageKey },
  category: { key: 'misc', order: 0 },
  status: 'draft',
  draftRevision: 1,
  updatedAt: '2026-09-27T00:00:00Z',
});

describe('PreviewDrawer onExecute 契约', () => {
  it('draft 预览态：执行入口被拒绝并透出引导文案（预览不执行函数）', async () => {
    render(
      <App>
        <PreviewDrawer open draft={draft()} onClose={jest.fn()} />
      </App>,
    );
    expect(screen.getByText('页面预览')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'mock-run' }));
    // 抛错信息来自 intl（editor.previewExecuteError），桩捕获 rejection 后展示
    expect(
      await screen.findByText(/Page Studio 预览不执行函数；发布后请在运行控制台执行/),
    ).toBeInTheDocument();
  });

  it('draft=null：空态占位（请选择页面）', () => {
    render(
      <App>
        <PreviewDrawer open draft={null} onClose={jest.fn()} />
      </App>,
    );
    expect(screen.getByText('请选择页面')).toBeInTheDocument();
  });
});
