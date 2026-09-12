/** 模板结构缩略图（V1 发现性）：PageNode[] → 轻量线框。
 * 回归点：
 * 1. 按节点类型出对应形态（表格=表头+行线 / 表单=2 行 / 按钮=圆角块 /
 *    弹窗=紫框 / 容器=嵌套递归），类名 tpl-thumb-* 供测试定位；
 * 2. 根级超过 4 个节点显示 +N 溢出标记（不无限铺开）；
 * 3. 宽度按 span/24 占比（半宽节点 50%）。 */
import { render } from '@testing-library/react';
import TemplateThumb from '../TemplateThumb';
import type { PageNode } from '../model';

function node(partial: Partial<PageNode> & { id: string; type: PageNode['type'] }): PageNode {
  return { props: {}, ...partial } as PageNode;
}

describe('TemplateThumb（模板结构缩略图）', () => {
  it('空树不渲染任何缩略元素', () => {
    const { container } = render(<TemplateThumb tree={[]} />);
    expect(container.querySelector('.tpl-thumb')).not.toBeInTheDocument();
  });

  it('表格节点：表头 + 3 行行线', () => {
    const { container } = render(
      <TemplateThumb
        tree={[node({ id: 't1', type: 'fnTable', props: { functionId: 'x.list' } })]}
      />,
    );
    expect(container.querySelector('.tpl-thumb__table')).toBeInTheDocument();
    expect(container.querySelectorAll('.tpl-thumb__trow')).toHaveLength(3);
  });

  it('表单/字段卡：2 行标签线（fnForm/fnFields 分别出形态类名）', () => {
    const { container } = render(
      <TemplateThumb
        tree={[
          node({ id: 'f1', type: 'fnForm', props: { functionId: 'mail.send' } }),
          node({ id: 'f2', type: 'fnFields', props: { functionId: 'player.get' } }),
        ]}
      />,
    );
    expect(container.querySelector('.tpl-thumb__form')).toBeInTheDocument();
    expect(container.querySelector('.tpl-thumb__fields')).toBeInTheDocument();
    expect(container.querySelectorAll('.tpl-thumb__frow')).toHaveLength(4);
  });

  it('按钮=圆角小块、文本=灰条、弹窗=紫色框', () => {
    const { container } = render(
      <TemplateThumb
        tree={[
          node({ id: 'b1', type: 'button', props: { title: '发邮件' } }),
          node({ id: 'x1', type: 'text', props: { content: '说明' } }),
          node({
            id: 'm1',
            type: 'modal',
            props: { title: '弹窗' },
            children: [node({ id: 'mf', type: 'fnForm', props: {} })],
          }),
        ]}
      />,
    );
    expect(container.querySelector('.tpl-thumb__btn')).toBeInTheDocument();
    expect(container.querySelector('.tpl-thumb__text')).toBeInTheDocument();
    expect(container.querySelector('.tpl-thumb__modal')).toBeInTheDocument();
  });

  it('容器嵌套：内部递归渲染子节点缩略形态', () => {
    const { container } = render(
      <TemplateThumb
        tree={[
          node({
            id: 'c1',
            type: 'container',
            props: { title: '分组' },
            children: [
              node({ id: 'b1', type: 'button', props: {} }),
              node({ id: 't1', type: 'fnTable', props: {} }),
            ],
          }),
        ]}
      />,
    );
    const box = container.querySelector('.tpl-thumb__container');
    expect(box).toBeInTheDocument();
    // 子节点形态在容器内递归出现
    expect(box!.querySelector('.tpl-thumb__btn')).toBeInTheDocument();
    expect(box!.querySelector('.tpl-thumb__table')).toBeInTheDocument();
  });

  it('根级超过 4 个节点显示 +N 溢出标记', () => {
    const tree = Array.from({ length: 6 }, (_, i) =>
      node({ id: `x${i}`, type: 'text', props: { content: String(i) } }),
    );
    const { container } = render(<TemplateThumb tree={tree} />);
    expect(container.querySelectorAll('.tpl-thumb__text')).toHaveLength(4);
    expect(container.querySelector('.tpl-thumb__more')).toHaveTextContent('+2');
  });

  it('宽度按 span 占比：半宽节点 50%', () => {
    const { container } = render(
      <TemplateThumb tree={[node({ id: 't1', type: 'fnTable', props: { span: 12 } })]} />,
    );
    expect(container.querySelector<HTMLElement>('.tpl-thumb__table')?.style.width).toBe('50%');
  });
});
