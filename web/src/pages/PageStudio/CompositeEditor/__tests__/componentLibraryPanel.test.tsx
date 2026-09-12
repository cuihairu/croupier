/** 组件库面板 UI（V1 发现性）：
 * 1. 头部常驻「从画布选中创建」入口——传入回调渲染并触发；
 *    未传回调不渲染（旧调用方零变化）；
 * 2. 模板卡片渲染结构缩略图（TemplateThumb 线框，tpl-thumb 类名）。
 * request 用 setupTests 全局虚拟 mock（jest.fn），此处按用例覆写返回模板列表。 */
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { App } from 'antd';
import { request } from '@umijs/max';
import ComponentLibrary, { type ComponentTemplateDTO } from '../ComponentLibrary';
import type { PageNode } from '../model';

const mockedRequest = request as unknown as jest.Mock;

// jsdom + antd 下模板拉取与渲染链路偏慢，放宽用例级预算（同 previewAlignment）
jest.setTimeout(20000);

const tplWithThumb: ComponentTemplateDTO = {
  key: 'crud--player',
  name: { 'zh-CN': '玩家 CRUD', 'en-US': 'Player CRUD' },
  requiredFunctions: ['player.list'],
  tree: [
    { id: 't1', type: 'fnTable', props: { functionId: 'player.list', title: '列表' } },
    { id: 'b1', type: 'button', props: { title: '发邮件' } },
  ] as PageNode[],
  builtin: false,
};

function renderLibrary(onCreateFromCanvas?: () => void) {
  return render(
    <App>
      <ComponentLibrary
        availableFnIds={new Set(['player.list'])}
        onInsert={() => undefined}
        onCreateFromCanvas={onCreateFromCanvas}
      />
    </App>,
  );
}

beforeEach(() => {
  mockedRequest.mockImplementation(async (url: string) => {
    if (typeof url === 'string' && url.includes('/api/v1/component-templates')) {
      return { items: [tplWithThumb] };
    }
    return {};
  });
});

describe('ComponentLibrary「从画布选中创建」入口（V1）', () => {
  it('传 onCreateFromCanvas 时渲染入口，点击回调一次', async () => {
    const onCreate = jest.fn();
    renderLibrary(onCreate);
    // jsdom + antd 下模板拉取链路偏慢，findBy* 放宽到 5s（快测默认 1s 不够）；
    // name 用正则：antd 图标 aria-label（plus）会拼进 accessible name
    const btn = await screen.findByRole('button', { name: /从画布选中创建/ }, { timeout: 5000 });
    expect(btn).toBeInTheDocument();
    fireEvent.click(btn);
    expect(onCreate).toHaveBeenCalledTimes(1);
  });

  it('未传 onCreateFromCanvas 时不渲染入口（模板列表不受影响）', async () => {
    renderLibrary();
    await screen.findByText('玩家 CRUD', undefined, { timeout: 5000 });
    expect(screen.queryByRole('button', { name: /从画布选中创建/ })).not.toBeInTheDocument();
  });
});

describe('ComponentLibrary 模板卡片缩略图（V1）', () => {
  it('卡片渲染 TemplateThumb 线框（表格 + 按钮形态）', async () => {
    const { container } = renderLibrary();
    await screen.findByText('玩家 CRUD', undefined, { timeout: 5000 });
    await waitFor(() => {
      expect(container.querySelector('.tpl-thumb')).toBeInTheDocument();
    });
    expect(container.querySelector('.tpl-thumb__table')).toBeInTheDocument();
    expect(container.querySelector('.tpl-thumb__btn')).toBeInTheDocument();
  });
});
