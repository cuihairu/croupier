/** TemplateQuickStart（空白画布模板引导）覆盖：外部模板直用、自行拉取
 * （数组/信封 items/失败空表/loading Spin）、单节点模板同样展示（D1）、
 * 卡片内容（name 本地化/内置 Tag/描述回退区块数/函数标签截 3 + 溢出 +N）、
 * 点击实例化（id 重映射 + onPick 三参）、从空白开始、空态文案。 */
import React from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import TemplateQuickStart from '../TemplateQuickStart';
import type { ComponentTemplateDTO } from '../ComponentLibrary';
import type { PageNode } from '../model';

jest.mock('@umijs/max', () => {
  const ReactLib = require('react');
  const formatMessage = (
    descriptor: { defaultMessage: string },
    values?: Record<string, unknown>,
  ) =>
    Object.entries(values || {}).reduce(
      (msg: string, [key, val]) => msg.split(`{${key}}`).join(String(val)),
      descriptor.defaultMessage,
    );
  return {
    __esModule: true,
    request: jest.fn(async () => ({})),
    useIntl: () => ({ formatMessage }),
    FormattedMessage: ({ defaultMessage }: { defaultMessage: React.ReactNode }) =>
      ReactLib.createElement(ReactLib.Fragment, null, defaultMessage),
  };
});

// 模板树：两区块 + 一个引用第二区块的动作（实例化后 id 重映射）
const tree: PageNode[] = [
  { id: 'src-1', type: 'staticForm', props: {} },
  {
    id: 'src-2',
    type: 'fnTable',
    props: { onClick: { kind: 'runBinding', target: 'src-1', params: {} } },
  },
];

const tpl = (over: Partial<ComponentTemplateDTO> = {}): ComponentTemplateDTO => ({
  key: 'combo-1',
  name: { 'zh-CN': '玩家查询组合', 'en-US': 'Player Combo' },
  tree,
  builtin: true,
  requiredFunctions: ['player.query', 'player.ban', 'player.kick', 'player.mail'],
  ...over,
});

import { request } from '@umijs/max';
const req = request as jest.Mock;

describe('TemplateQuickStart', () => {
  beforeEach(() => req.mockReset());

  it('外部模板：卡片渲染（内置 Tag/描述/函数标签截 3 + 溢出）', () => {
    render(
      <TemplateQuickStart
        templates={[tpl({ description: { 'zh-CN': '按 ID 查玩家并展示' } })]}
        onPick={jest.fn()}
      />,
    );
    expect(screen.getByText('从模板开始')).toBeInTheDocument();
    expect(screen.getByText('玩家查询组合')).toBeInTheDocument();
    expect(screen.getByText('内置')).toBeInTheDocument();
    expect(screen.getByText('按 ID 查玩家并展示')).toBeInTheDocument();
    expect(screen.getByText('player.query')).toBeInTheDocument();
    expect(screen.getByText('+1')).toBeInTheDocument();
  });

  it('无描述：回退「N 个区块」；无 requiredFunctions 不渲染标签行', () => {
    render(
      <TemplateQuickStart
        templates={[tpl({ requiredFunctions: undefined, description: undefined })]}
        onPick={jest.fn()}
      />,
    );
    expect(screen.getByText('2 个区块')).toBeInTheDocument();
    expect(screen.queryByText('player.query')).not.toBeInTheDocument();
  });

  it('单节点与无 tree 模板同样展示（D1 不再过滤）；空表才空态', () => {
    const { container, unmount } = render(
      <TemplateQuickStart
        templates={[
          tpl({ key: 'single', tree: tree.slice(0, 1) }),
          tpl({ key: 'no-tree', tree: undefined }),
        ]}
        onPick={jest.fn()}
      />,
    );
    // D1：单函数区块页面合法——单节点/无 tree 模板不作过滤
    expect(container.querySelectorAll('.ant-card-hoverable').length).toBeGreaterThan(0);
    expect(screen.getByText('1 个区块')).toBeInTheDocument();
    expect(screen.getByText('0 个区块')).toBeInTheDocument();

    // 空表才显示空态文案
    unmount();
    render(<TemplateQuickStart templates={[]} onPick={jest.fn()} />);
    expect(
      screen.getByText(/暂无组合模板——可先到「组件模板」页从契约重新生成/),
    ).toBeInTheDocument();
  });

  it('点击卡片：实例化 id 重映射 + onPick(nodes, tpl, dangling)', () => {
    const onPick = jest.fn();
    render(<TemplateQuickStart templates={[tpl()]} onPick={onPick} />);
    fireEvent.click(screen.getByText('玩家查询组合'));
    expect(onPick).toHaveBeenCalledTimes(1);
    const [nodes, picked, dangling] = onPick.mock.calls[0] as [
      PageNode[],
      ComponentTemplateDTO,
      unknown,
    ];
    expect(picked.key).toBe('combo-1');
    // id 全部重映射（不再含模板原 id）
    expect(nodes.map((n) => n.id)).not.toContain('src-1');
    // 动作 target 跟随重映射
    const target = (nodes[1].props.onClick as { target: string }).target;
    expect(target).toBe(nodes[0].id);
    expect(dangling).toEqual([]);
  });

  it('从空白开始按钮（可选）', () => {
    const onStartBlank = jest.fn();
    render(<TemplateQuickStart templates={[]} onPick={jest.fn()} onStartBlank={onStartBlank} />);
    fireEvent.click(screen.getByText('从空白开始'));
    expect(onStartBlank).toHaveBeenCalledTimes(1);
  });

  it('自行拉取：数组响应渲染', async () => {
    req.mockResolvedValueOnce([tpl({ key: 'fetched' })]);
    render(<TemplateQuickStart onPick={jest.fn()} />);
    await waitFor(() => expect(screen.getByText('玩家查询组合')).toBeInTheDocument());
    expect(req).toHaveBeenCalledWith('/api/v1/component-templates', { skipErrorHandler: true });
  });

  it('自行拉取：信封 items 形态；失败 → 空态', async () => {
    req.mockResolvedValueOnce({ items: [tpl({ key: 'envelope' })] });
    const { unmount } = render(<TemplateQuickStart onPick={jest.fn()} />);
    await waitFor(() => expect(screen.getByText('玩家查询组合')).toBeInTheDocument());
    unmount();

    req.mockRejectedValueOnce(new Error('boom'));
    render(<TemplateQuickStart onPick={jest.fn()} />);
    await waitFor(() => expect(screen.getByText(/暂无组合模板/)).toBeInTheDocument());
  });

  it('自行拉取：无 items 键的信封 → 空表（?? 空侧）', async () => {
    req.mockResolvedValueOnce({ foo: 1 });
    render(<TemplateQuickStart onPick={jest.fn()} />);
    await waitFor(() => expect(screen.getByText(/暂无组合模板/)).toBeInTheDocument());
  });
});
