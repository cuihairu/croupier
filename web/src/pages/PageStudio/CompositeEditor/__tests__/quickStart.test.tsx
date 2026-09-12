/** 空画布起步模式（从模板开始 / 从空白开始）双向切换。
 *
 * 回归点（本轮修复，必须持续守护）：
 * 1. 「从模板开始」引导面板提供「从空白开始」入口——空白起步不再被模板
 *    引导静默替代（11913ff0d 曾把空态整体换成模板选择，无空白入口）；
 * 2. 空白模式渲染根落区（RootDropZone，droppable('canvas-root')）——
 *    此前 Canvas 只在 tree.length > 0 时渲染、RootDropZone 与外层条件互斥
 *    成死代码，空画布上拖拽无落区，模板引导文案声称「可直接拖入」实际无效；
 * 3. 根落区「查看组合模板」可返回模板引导（双向切换，非单向丢弃）。 */
import { render, screen, fireEvent } from '@testing-library/react';
import { App } from 'antd';
import Canvas from '../Canvas';
import TemplateQuickStart from '../TemplateQuickStart';
import type { ComponentTemplateDTO } from '../ComponentLibrary';
import type { PageNode } from '../model';

function tpl(partial: Partial<ComponentTemplateDTO> & { key: string }): ComponentTemplateDTO {
  return {
    name: { 'zh-CN': partial.key, 'en-US': partial.key },
    builtin: false,
    tree: [],
    ...partial,
  } as ComponentTemplateDTO;
}

const combo = tpl({
  key: 'crud--player',
  description: { 'zh-CN': '列表+详情+弹窗', 'en-US': 'crud combo' },
  requiredFunctions: ['player.list', 'player.get'],
  tree: [
    { id: 't1', type: 'fnTable', props: { functionId: 'player.list', title: '列表' } },
    { id: 't2', type: 'fnFields', props: { functionId: 'player.get', title: '详情' } },
  ],
});
const single = tpl({
  key: 'fn--mail.send',
  tree: [{ id: 's1', type: 'fnForm', props: { functionId: 'mail.send', title: '发邮件' } }],
});

describe('TemplateQuickStart（空画布模板引导）', () => {
  it('只列出多区块组合模板，点击卡片回调 onPick（节点 + 模板）', () => {
    const onPick = jest.fn();
    render(
      <App>
        <TemplateQuickStart templates={[combo, single]} onPick={onPick} />
      </App>,
    );
    expect(screen.getByText('crud--player')).toBeInTheDocument();
    // 单节点模板不出现（组合页需 ≥2 区块）
    expect(screen.queryByText('fn--mail.send')).not.toBeInTheDocument();
    // 依赖函数 Tag 展示
    expect(screen.getByText('player.list')).toBeInTheDocument();

    fireEvent.click(screen.getByText('crud--player'));
    expect(onPick).toHaveBeenCalledTimes(1);
    const [nodes, picked] = onPick.mock.calls[0] as unknown as [PageNode[], ComponentTemplateDTO];
    expect(nodes).toHaveLength(2);
    expect(picked.key).toBe('crud--player');
  });

  it('无组合模板时显示降级文案（指向左栏拖入与模板页）', () => {
    render(
      <App>
        <TemplateQuickStart templates={[single]} onPick={() => undefined} />
      </App>,
    );
    expect(screen.getByText(/暂无组合模板/)).toBeInTheDocument();
  });

  it('「从空白开始」入口触发 onStartBlank', () => {
    const onStartBlank = jest.fn();
    render(
      <App>
        <TemplateQuickStart
          templates={[combo]}
          onPick={() => undefined}
          onStartBlank={onStartBlank}
        />
      </App>,
    );
    fireEvent.click(screen.getByRole('button', { name: '从空白开始' }));
    expect(onStartBlank).toHaveBeenCalledTimes(1);
  });

  it('未提供 onStartBlank 时不渲染「从空白开始」入口', () => {
    render(
      <App>
        <TemplateQuickStart templates={[combo]} onPick={() => undefined} />
      </App>,
    );
    expect(screen.queryByRole('button', { name: '从空白开始' })).not.toBeInTheDocument();
  });
});

describe('Canvas 空树根落区（RootDropZone）', () => {
  function renderEmptyCanvas(onShowTemplates?: () => void) {
    return render(
      <App>
        <Canvas
          tree={[]}
          selectedId={null}
          fnById={new Map()}
          onSelect={() => undefined}
          onDelete={() => undefined}
          onDuplicate={() => undefined}
          onSpanChange={() => undefined}
          onEnterModal={() => undefined}
          canvasWidthRef={{ current: null }}
          onShowTemplates={onShowTemplates}
        >
          {null}
        </Canvas>
      </App>,
    );
  }

  it('空树渲染根落区提示（拖入落区激活，不再是死代码路径）', () => {
    renderEmptyCanvas();
    expect(screen.getByText('从左侧点击或拖入组件，开始搭建页面')).toBeInTheDocument();
  });

  it('提供 onShowTemplates 时渲染「查看组合模板」并回调', () => {
    const onShowTemplates = jest.fn();
    renderEmptyCanvas(onShowTemplates);
    fireEvent.click(screen.getByRole('button', { name: '查看组合模板' }));
    expect(onShowTemplates).toHaveBeenCalledTimes(1);
  });

  it('未提供 onShowTemplates 时不渲染返回模板入口（弹窗级空态）', () => {
    renderEmptyCanvas();
    expect(screen.queryByRole('button', { name: '查看组合模板' })).not.toBeInTheDocument();
  });
});
