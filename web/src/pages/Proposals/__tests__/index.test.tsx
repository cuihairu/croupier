/**
 * 旧链接兼容跳转页单测（覆盖率补缺轮：Proposals/index.tsx 13 行 0% →
 * 收口，全站最后一个零测试页面目录——此后 web/src/pages 无零测试页）。
 *
 * 锁定契约：
 * - 挂载即 history.replace(`/functions/pages${location.search}`)——旧
 *   /proposals 链接带 query 原样转译到页面工作台入口；
 * - 组件渲染 null（无 DOM 输出）；
 * - location.search 变化 → effect 依赖触发二次 replace（新 query 转译）。
 *
 * mock 口径：@umijs/max mock history.replace + useLocation（search 可变）；
 * 页面无 UI 依赖（无 antd/pro-components/App 消费）。坑实证：jest.mock
 * 工厂随 import 链在 const 声明前执行（TDZ）——jest.fn 须定义在工厂内、
 * 测试侧经 mock 后模块自身导出取句柄（mock 前缀变量救不了工厂内引用）。
 *
 * 现状锁定 / 边界：无登记不可达分支——13 行直线逻辑，effect 依赖双态
 * （初始/变更）均真实构造。
 */
import React from 'react';
import { render, waitFor } from '@testing-library/react';
import ProposalsPage from '../index';
import { history, useLocation } from '@umijs/max';

jest.mock('@umijs/max', () => ({
  history: { replace: jest.fn() },
  useLocation: jest.fn(),
}));

const mReplace = history.replace as jest.MockedFunction<typeof history.replace>;
const mLocation = useLocation as jest.MockedFunction<typeof useLocation>;

describe('旧链接兼容跳转 /proposals → /functions/pages', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mLocation.mockReturnValue({ search: '?a=1' });
  });

  it('挂载即 replace 转译（query 原样保留）+ 渲染 null', async () => {
    const { container } = render(<ProposalsPage />);

    await waitFor(() => expect(mReplace).toHaveBeenCalledWith('/functions/pages?a=1'));
    expect(mReplace).toHaveBeenCalledTimes(1);
    expect(container.firstChild).toBeNull(); // 组件无 DOM 输出
  });

  it('search 变化 → effect 依赖触发二次 replace（新 query）', async () => {
    const { rerender } = render(<ProposalsPage />);
    await waitFor(() => expect(mReplace).toHaveBeenCalledWith('/functions/pages?a=1'));

    mLocation.mockReturnValue({ search: '?tab=v2&x=9' });
    rerender(<ProposalsPage />);
    await waitFor(() => expect(mReplace).toHaveBeenLastCalledWith('/functions/pages?tab=v2&x=9'));
    expect(mReplace).toHaveBeenCalledTimes(2);
  });
});
