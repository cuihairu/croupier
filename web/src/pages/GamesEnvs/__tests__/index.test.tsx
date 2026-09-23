/**
 * 游戏环境页入口提示冒烟：定位副标题、作用域边界 Alert 与
 * 「进入 Page Studio」跳转。环境是全局作用域入口，提示层改动
 * 按 headless 实渲染口径验收（jsdom 真挂载）。
 */
import React from 'react';
import { App as AntdApp } from 'antd';
import { configure, fireEvent, render, screen, waitFor } from '@testing-library/react';
import GamesEnvsPage from '../index';

configure({ asyncUtilTimeout: 5000 });
jest.setTimeout(20000);

jest.mock('@umijs/max', () => {
  const formatMessage = jest.fn(
    ({ defaultMessage }: { defaultMessage: string }, values?: Record<string, unknown>) =>
      Object.entries(values || {}).reduce(
        (msg: string, [key, val]) => msg.split(`{${key}}`).join(String(val)),
        defaultMessage,
      ),
  );
  const intl = { formatMessage };
  return {
    __esModule: true,
    useIntl: () => intl,
    getIntl: () => intl,
    // 工厂内联创建：jest.mock 提升早于模块体 const 声明，外部引用会 TDZ 报错
    history: { push: jest.fn() },
    FormattedMessage: ({ defaultMessage }: { defaultMessage: string }) => defaultMessage,
  };
});

const umiMock = jest.requireMock('@umijs/max') as {
  history: { push: jest.Mock };
};

// jsdom 下 PageContainer 依赖 ProLayout 的 RouteContext，mock 成透传渲染
jest.mock('@ant-design/pro-components', () => ({
  __esModule: true,
  PageContainer: ({
    subTitle,
    children,
  }: {
    title?: React.ReactNode;
    subTitle?: React.ReactNode;
    children: React.ReactNode;
  }) => (
    <div>
      <p data-testid="page-subtitle">{subTitle}</p>
      {children}
    </div>
  ),
  ModalForm: () => null,
}));

jest.mock('@/services/api', () => ({
  __esModule: true,
  listGamesMeta: jest.fn().mockResolvedValue([]),
  listMyGames: jest.fn().mockResolvedValue([]),
}));

jest.mock('@/services/api/envs', () => ({
  __esModule: true,
  listGameEnvs: jest.fn().mockResolvedValue([]),
  addGameEnv: jest.fn(),
  updateGameEnv: jest.fn(),
  deleteGameEnv: jest.fn(),
}));

describe('GamesEnvs 入口提示', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it('渲染定位副标题与作用域边界 Alert', async () => {
    render(
      <AntdApp>
        <GamesEnvsPage />
      </AntdApp>,
    );

    expect(screen.getByTestId('page-subtitle')).toHaveTextContent(
      '游戏与环境是所有能力的作用域；先选定作用域，再浏览函数、资源与页面',
    );
    expect(screen.getByText('环境只是作用域，不产生页面')).toBeInTheDocument();
    expect(
      screen.getByText(
        '选定游戏/环境后，函数目录、资源目录与 Page Studio 中的数据都会按当前作用域过滤；要编排运营页面，仍需进入 Page Studio。',
      ),
    ).toBeInTheDocument();
    // 回归：既有列表卡仍渲染
    await waitFor(() => expect(screen.getByText('游戏环境')).toBeInTheDocument());
  });

  it('Alert 按钮跳转 Page Studio', () => {
    render(
      <AntdApp>
        <GamesEnvsPage />
      </AntdApp>,
    );

    fireEvent.click(screen.getByRole('button', { name: '进入 Page Studio' }));
    expect(umiMock.history.push).toHaveBeenCalledWith('/functions/pages');
  });
});
