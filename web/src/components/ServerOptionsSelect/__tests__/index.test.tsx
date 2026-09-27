/**
 * ServerOptionsSelect：服务端聚合选项下拉的统一封装回归（#14/#21/#22/#23/
 * #26/#33/#34 族组件化）。核心契约——
 * 1. 选项全集来自 fetchOptions，string/object 归一，count 渲染「 (n)」；
 * 2. 全局 scope 切换自动重拉（不塌缩、不残留旧游戏选项）；
 * 3. 取数失败保留上次选项且不抛出；
 * 4. epoch 变化强制重拉（写后刷新）。
 */
import React from 'react';
import { act, configure, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App } from 'antd';
import ServerOptionsSelect from '../index';
import { setScope } from '@/stores/scope';

configure({ asyncUtilTimeout: 5000 });

const fetchOptions = jest.fn();

const renderSelect = (extra?: Record<string, unknown>) =>
  render(
    <App>
      <ServerOptionsSelect
        fetchOptions={fetchOptions}
        placeholder="选择分类"
        style={{ width: 200 }}
        {...extra}
      />
    </App>,
  );

const openDropdownLabels = async (): Promise<(string | null)[]> => {
  fireEvent.mouseDown(screen.getByRole('combobox'));
  await waitFor(() =>
    expect(document.querySelectorAll('.ant-select-dropdown .ant-select-item-option').length),
  );
  return Array.from(document.querySelectorAll('.ant-select-dropdown .ant-select-item-option')).map(
    (el) => el.textContent,
  );
};

beforeEach(() => {
  jest.clearAllMocks();
  act(() => setScope({ gameId: 'default', env: 'dev' }));
});

describe('ServerOptionsSelect', () => {
  it('挂载拉取一次；string 与 {value,label,count} 归一，count 渲染后缀', async () => {
    fetchOptions.mockResolvedValue(['a', { value: 'b', label: '玩家运营', count: 2 }]);
    renderSelect();
    await waitFor(() => expect(fetchOptions).toHaveBeenCalledTimes(1));

    expect(await openDropdownLabels()).toEqual(['a', '玩家运营 (2)']);
  });

  it('顶栏切换游戏 scope 后自动重拉选项', async () => {
    fetchOptions.mockResolvedValue([]);
    renderSelect();
    await waitFor(() => expect(fetchOptions).toHaveBeenCalledTimes(1));

    act(() => setScope({ gameId: 'game-a', env: 'prod' }));
    await waitFor(() => expect(fetchOptions).toHaveBeenCalledTimes(2));
  });

  it('取数失败保留上次选项（第二次失败不清空）', async () => {
    fetchOptions.mockResolvedValue(['first']);
    renderSelect();
    await waitFor(() => expect(fetchOptions).toHaveBeenCalledTimes(1));

    fetchOptions.mockRejectedValueOnce(new Error('boom'));
    act(() => setScope({ gameId: 'game-b', env: 'dev' }));
    await waitFor(() => expect(fetchOptions).toHaveBeenCalledTimes(2));

    expect(await openDropdownLabels()).toEqual(['first']);
  });

  it('epoch 变化强制重拉；epoch 不变不重复请求', async () => {
    fetchOptions.mockResolvedValue([]);
    const { rerender } = render(
      <App>
        <ServerOptionsSelect fetchOptions={fetchOptions} epoch={1} />
      </App>,
    );
    await waitFor(() => expect(fetchOptions).toHaveBeenCalledTimes(1));

    rerender(
      <App>
        <ServerOptionsSelect fetchOptions={fetchOptions} epoch={1} />
      </App>,
    );
    expect(fetchOptions).toHaveBeenCalledTimes(1);

    rerender(
      <App>
        <ServerOptionsSelect fetchOptions={fetchOptions} epoch={2} />
      </App>,
    );
    await waitFor(() => expect(fetchOptions).toHaveBeenCalledTimes(2));
  });

  it('onOptionsLoaded 回传归一后的选项（供页面统计）', async () => {
    const onOptionsLoaded = jest.fn();
    fetchOptions.mockResolvedValue([{ value: 'x', count: 3 }]);
    renderSelect({ onOptionsLoaded });
    await waitFor(() =>
      expect(onOptionsLoaded).toHaveBeenCalledWith([{ value: 'x', label: 'x', count: 3 }]),
    );
  });
});
