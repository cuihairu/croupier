/**
 * Agent 扩展同步调试页单测（覆盖率巡检：Extensions 簇余量收口，AgentSync
 * index.tsx 93 行 0% → 全覆盖）。
 *
 * 锁定契约：初态渲染（标题/副标题/Payload JSON 卡片空态「暂无数据」）、
 * 空 ID 查询 warning 拦截（服务不触达）、查询链（getAgentSyncPayload 载荷
 * + payload JSON 回显 + 按钮退出 loading）、resp.payload 缺省 `|| {}` 右翼、
 * ID trim 翼、onPressEnter 触发查询、清空复位双态。
 *
 * mock 口径：services/api/extensions 仅 getAgentSyncPayload；
 * @ant-design/pro-components 本地 mock（PageContainer 桩透传）；@umijs/max
 * 本地 mock。App 包裹（App.useApp message）。
 *
 * 边界（诚实）：runSyncQuery 的 try/finally 无 catch——接口 reject 产生
 * unhandled rejection（组件现状缺陷，同簇既定结论），不造假 reject 用例。
 */
import React from 'react';
import { configure, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App } from 'antd';
import ExtensionAgentSyncPage from '../index';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

jest.mock('@/services/api/extensions', () => ({
  getAgentSyncPayload: jest.fn(),
}));

jest.mock('@ant-design/pro-components', () => ({
  PageContainer: ({
    children,
    title,
    subTitle,
  }: {
    children?: React.ReactNode;
    title?: React.ReactNode;
    subTitle?: React.ReactNode;
  }) => (
    <div>
      <h1>
        {title}
        {subTitle ? <span>{subTitle}</span> : null}
      </h1>
      {children}
    </div>
  ),
}));

jest.mock('@umijs/max', () => ({
  FormattedMessage: ({ defaultMessage }: { defaultMessage?: string }) => <>{defaultMessage}</>,
  useIntl: () => ({ formatMessage: (o: { defaultMessage?: string }) => o.defaultMessage ?? '' }),
}));

import { getAgentSyncPayload } from '@/services/api/extensions';

const mPayload = getAgentSyncPayload as jest.MockedFunction<typeof getAgentSyncPayload>;

function renderPage() {
  return render(
    <App>
      <ExtensionAgentSyncPage />
    </App>,
  );
}

beforeEach(() => {
  jest.clearAllMocks();
  mPayload.mockResolvedValue({ payload: { functions: ['chat.echo'], version: '1.4.0' } } as never);
});

describe('AgentSync 初态与拦截', () => {
  it('初态渲染：标题/副标题/Payload JSON 卡片空态「暂无数据」', () => {
    renderPage();

    expect(screen.getByText('Agent 扩展同步调试')).toBeInTheDocument();
    expect(screen.getByText('查看指定 Agent 的扩展同步载荷')).toBeInTheDocument();
    expect(screen.getByText('Payload JSON')).toBeInTheDocument();
    expect(screen.getByText('暂无数据')).toBeInTheDocument();
    expect(mPayload).not.toHaveBeenCalled();
  });

  it('空 ID 查询：warning 拦截、不触达服务', async () => {
    renderPage();

    fireEvent.click(screen.getByRole('button', { name: '查询同步载荷' }));
    expect(await screen.findByText('请输入 Agent ID')).toBeInTheDocument();
    expect(mPayload).not.toHaveBeenCalled();
    expect(screen.getByText('暂无数据')).toBeInTheDocument();
  });
});

describe('AgentSync 查询链', () => {
  it('输入 ID 点查询：getAgentSyncPayload 载荷 + payload JSON 回显 + 按钮退出 loading', async () => {
    renderPage();

    fireEvent.change(screen.getByPlaceholderText('例如: agent-001'), {
      target: { value: 'agent-001' },
    });
    fireEvent.click(screen.getByRole('button', { name: '查询同步载荷' }));

    await waitFor(() => expect(mPayload).toHaveBeenCalledWith('agent-001'));
    // multiline JSON 回显（坑 10：期望串用正则；indent=2 时数组也折行，拆键断言）
    expect(await screen.findByDisplayValue(/"functions": \[/)).toBeInTheDocument();
    expect(screen.getByDisplayValue(/"chat\.echo"/)).toBeInTheDocument();
    expect(screen.getByDisplayValue(/"version": "1.4.0"/)).toBeInTheDocument();
    await waitFor(() =>
      expect(screen.getByRole('button', { name: '查询同步载荷' })).not.toHaveClass(
        'ant-btn-loading',
      ),
    );
  });

  it('ID trim 翼：输入带空白 → 调用 trim 后 ID', async () => {
    renderPage();

    fireEvent.change(screen.getByPlaceholderText('例如: agent-001'), {
      target: { value: '  agent-007  ' },
    });
    fireEvent.click(screen.getByRole('button', { name: '查询同步载荷' }));

    await waitFor(() => expect(mPayload).toHaveBeenCalledWith('agent-007'));
  });

  it('resp.payload 缺省：`|| {}` 右翼 → 空 JSON 对象回显', async () => {
    mPayload.mockResolvedValue({} as never);
    renderPage();

    fireEvent.change(screen.getByPlaceholderText('例如: agent-001'), {
      target: { value: 'agent-001' },
    });
    fireEvent.click(screen.getByRole('button', { name: '查询同步载荷' }));

    expect(await screen.findByDisplayValue('{}')).toBeInTheDocument();
  });

  it('onPressEnter：输入框回车触发查询', async () => {
    renderPage();

    const input = screen.getByPlaceholderText('例如: agent-001');
    fireEvent.change(input, { target: { value: 'agent-009' } });
    fireEvent.keyDown(input, { key: 'Enter', keyCode: 13 });

    await waitFor(() => expect(mPayload).toHaveBeenCalledWith('agent-009'));
    expect(await screen.findByDisplayValue(/"functions": \[/)).toBeInTheDocument();
  });
});

describe('AgentSync 清空', () => {
  it('查询后点清空：输入与回显双态复位、回空态「暂无数据」', async () => {
    renderPage();

    fireEvent.change(screen.getByPlaceholderText('例如: agent-001'), {
      target: { value: 'agent-001' },
    });
    fireEvent.click(screen.getByRole('button', { name: '查询同步载荷' }));
    await waitFor(() => expect(mPayload).toHaveBeenCalledWith('agent-001'));
    expect(await screen.findByDisplayValue(/"functions"/)).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: '清空' }));
    expect((screen.getByPlaceholderText('例如: agent-001') as HTMLInputElement).value).toBe('');
    expect(screen.getByText('暂无数据')).toBeInTheDocument();
  });
});
