/**
 * PageWorkflowGuide 单测（GUX-015：说明与文案收敛）。
 *
 * 锁定契约：引导默认折叠（Steps 与提醒不可见，标题行常驻），点击 header
 * 后展开——四步流程文案与「只有 Proposal 需要处理才补语义」的 warning
 * 按需出现。组件无其他分支；交互只有展开。
 *
 * mock 口径：@umijs/max useIntl 直出 defaultMessage（与 Directory 同款）；
 * antd Collapse/Steps/Alert 真实实现。
 */
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import PageWorkflowGuide from '../index';

jest.mock('@umijs/max', () => ({
  useIntl: () => ({
    locale: 'zh-CN',
    formatMessage: (opts: { defaultMessage?: string }) => opts.defaultMessage ?? '',
  }),
}));

describe('PageWorkflowGuide（GUX-015 按需帮助）', () => {
  afterEach(cleanup);

  it('默认折叠：标题行常驻，四步流程与提醒不可见', () => {
    render(<PageWorkflowGuide />);

    expect(screen.getByText('创建完整页面：只走这一条默认路径')).toBeInTheDocument();
    expect(screen.queryByText('注册函数')).not.toBeInTheDocument();
    expect(
      screen.queryByText('SDK / Agent 注册 FunctionContract，或导入 OpenAPI。'),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByText(/只有 Proposal 显示“需要处理”时，才进入资源目录补全语义/),
    ).not.toBeInTheDocument();
  });

  it('点击 header 展开：四步流程与 warning 提醒出现', () => {
    render(<PageWorkflowGuide />);

    fireEvent.click(screen.getByText('创建完整页面：只走这一条默认路径'));

    expect(screen.getByText('注册函数')).toBeInTheDocument();
    expect(
      screen.getByText('SDK / Agent 注册 FunctionContract，或导入 OpenAPI。'),
    ).toBeInTheDocument();
    expect(screen.getByText('在控制台运行')).toBeInTheDocument();
    expect(
      screen.getByText(/只有 Proposal 显示“需要处理”时，才进入资源目录补全语义/),
    ).toBeInTheDocument();
  });
});
