/** PreviewRuntime 模拟模式「合成不出数据」警告契约（round-7 覆盖率巡检，补
 *  L168-180 warn-once 分支）：
 *  - generateMockResponse 返回 undefined 时按节点警告一次，结果仍写 {data:{}}
 *    空态，不触发真实调用；
 *  - 同节点重复执行不再重复警告（mockWarnedRef 去重）。
 *
 *  可达性边界（诚实登记）：当前 mockData.generateMockResponse 对非空
 *  functionId 恒有 fallback 兜底（L155-164），runNode 又保证 fid 非空——该
 *  分支在线上不可达，属防御性保留（函数契约缺失/顶层结构不支持的旧
 *  descriptor 形态）。本套件以模块边界 mock 锁定 PreviewRuntime 自身的
 *  warn-once 与空态回写逻辑，非伪造业务路径。 */
import React from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App } from 'antd';
import PreviewRuntime from '../PreviewRuntime';
import { generateMockResponse } from '../mockData';
import { invokeFunction } from '@/services/api/functions';
import type { FunctionDescriptor } from '@/services/api/functions';
import type { PageNode } from '../model';

jest.mock('@/services/api/functions', () => ({
  invokeFunction: jest.fn(async () => {
    throw new Error('real invocation should not happen in mock mode');
  }),
  listDescriptors: jest.fn(async () => []),
}));

jest.mock('../mockData', () => ({
  generateMockResponse: jest.fn((): { data: unknown } | undefined => undefined),
}));

// 重 DOM 套件（PreviewRuntime 全量渲染）在 coverage instrumentation 负载下
// 撞默认 5s 用例预算（隔离跑恒绿），与 previewMockData 同法放宽
jest.setTimeout(20000);
const FIND = { timeout: 5000 } as const;

const mockedGenerate = generateMockResponse as unknown as jest.Mock;
const mockedInvoke = invokeFunction as unknown as jest.Mock;

// 契约缺失形态：descriptor 只有 id（无 outputSchema）——历史上触发本分支的
// 真实形态；generateMockResponse 已被 mock，这里仅作为 fnById 键存在
const noSchemaDescriptor = { id: 'player.list' } as FunctionDescriptor;

function tree(autoRun: boolean): PageNode[] {
  return [
    {
      id: 'tbl1',
      type: 'fnTable',
      props: {
        functionId: 'player.list',
        title: '玩家列表',
        span: 24,
        autoRun,
        columns: ['uid'],
      },
    },
  ];
}

function renderPreview(nodes: PageNode[]) {
  return render(
    <App>
      <PreviewRuntime tree={nodes} fnById={new Map([['player.list', noSchemaDescriptor]])} />
    </App>,
  );
}

beforeEach(() => {
  mockedGenerate.mockClear();
  mockedInvoke.mockClear();
});

describe('PreviewRuntime 模拟数据合成失败告警', () => {
  it('autoRun 触发合成失败 → 警告一次且带节点标题，结果写 {data:{}} 空态，不真实调用', async () => {
    renderPreview(tree(true));
    expect(
      await screen.findByText(
        /无可用 outputSchema（或顶层结构不支持），模拟数据为空/,
        undefined,
        FIND,
      ),
    ).toBeInTheDocument();
    expect(screen.getAllByText(/「玩家列表」无可用 outputSchema/)).toHaveLength(1);
    expect(mockedGenerate).toHaveBeenCalledWith(noSchemaDescriptor, 'player.list');
    expect(mockedInvoke).not.toHaveBeenCalled();
  });

  it('同节点重复执行仅警告一次（mockWarnedRef 去重），仍不触发真实调用', async () => {
    renderPreview(tree(false));
    fireEvent.click(await screen.findByRole('button', { name: /执\s*行/ }, FIND));
    await screen.findByText(/无可用 outputSchema/, undefined, FIND);
    fireEvent.click(screen.getByRole('button', { name: /执\s*行/ }));
    await waitFor(() => expect(mockedGenerate).toHaveBeenCalledTimes(2), FIND);
    // 第二次执行不再追加警告——仍是首次那一条
    expect(screen.getAllByText(/无可用 outputSchema/)).toHaveLength(1);
    expect(mockedInvoke).not.toHaveBeenCalled();
  });
});
