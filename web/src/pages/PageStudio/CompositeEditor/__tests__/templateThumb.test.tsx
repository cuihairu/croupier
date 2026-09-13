/** TemplateThumb（模板结构缩略图）覆盖：空树 null、各节点形态线框
 * （button/text/modal/container 有无 children/fnTable 表头+行线/
 * fnFields/fnForm·staticForm 兜底 form）、span 宽度换算（合法区间/
 * 缺省·非法·越界回退 24）、子节点截前 3、根级截前 4 + 溢出 +N。 */
import React from 'react';
import { render, screen } from '@testing-library/react';
import TemplateThumb from '../TemplateThumb';
import type { PageNode } from '../model';

const n = (
  id: string,
  type: PageNode['type'],
  props: Record<string, unknown> = {},
  children?: PageNode[],
): PageNode => ({
  id,
  type,
  props,
  children,
});

describe('TemplateThumb', () => {
  it('空树：null', () => {
    const { container } = render(<TemplateThumb tree={[]} />);
    expect(container).toBeEmptyDOMElement();
  });

  it('button/text：特征类名', () => {
    const { container } = render(<TemplateThumb tree={[n('b', 'button'), n('t', 'text')]} />);
    expect(container.querySelector('.tpl-thumb__btn')).not.toBeNull();
    expect(container.querySelector('.tpl-thumb__text')).not.toBeNull();
  });

  it('modal：紫色框线 + span 宽度换算', () => {
    const { container } = render(<TemplateThumb tree={[n('m', 'modal', { span: 12 })]} />);
    const modal = container.querySelector('.tpl-thumb__modal') as HTMLElement;
    expect(modal.style.width).toBe('50%');
    expect(modal.style.border).toContain('rgb(179, 127, 235)');
  });

  it('span 缺省/非法/越界：回退满宽', () => {
    const { container } = render(
      <TemplateThumb
        tree={[
          n('a', 'fnForm'),
          n('b', 'fnForm', { span: 'abc' }),
          n('c', 'fnForm', { span: 3 }),
          n('d', 'fnForm', { span: 25 }),
        ]}
      />,
    );
    const widths = [...container.querySelectorAll('.tpl-thumb__form')].map(
      (el) => (el as HTMLElement).style.width,
    );
    expect(widths).toEqual(['100%', '100%', '100%', '100%']);
  });

  it('fnTable：表头线 + 3 行；fnFields：特征类名', () => {
    const { container } = render(
      <TemplateThumb tree={[n('tb', 'fnTable'), n('ff', 'fnFields')]} />,
    );
    expect(container.querySelectorAll('.tpl-thumb__trow')).toHaveLength(3);
    expect(container.querySelector('.tpl-thumb__table')).not.toBeNull();
    expect(container.querySelector('.tpl-thumb__fields')).not.toBeNull();
    // 非表格无表头线（rows=2）
    expect(container.querySelectorAll('.tpl-thumb__frow')).toHaveLength(2);
  });

  it('staticForm/fnForm：兜底 form 形态（2 行）', () => {
    const { container } = render(
      <TemplateThumb tree={[n('sf', 'staticForm'), n('ff', 'fnForm')]} />,
    );
    expect(container.querySelectorAll('.tpl-thumb__form')).toHaveLength(2);
  });

  it('container：children 截前 3；无 children 走 form 兜底', () => {
    const { container } = render(
      <TemplateThumb
        tree={[
          n('c1', 'container', {}, [
            n('k1', 'button'),
            n('k2', 'text'),
            n('k3', 'fnTable'),
            n('k4', 'modal'),
          ]),
          n('c2', 'container'),
        ]}
      />,
    );
    // 有 children 的渲染 container 线框；无 children 的走 form 兜底
    const boxes = container.querySelectorAll('.tpl-thumb__container');
    expect(boxes).toHaveLength(1);
    // 只渲染前 3 个子节点（第 4 个 modal 不出现）
    expect(boxes[0].querySelector('.tpl-thumb__btn')).not.toBeNull();
    expect(boxes[0].querySelector('.tpl-thumb__modal')).toBeNull();
    const forms = container.querySelectorAll('.tpl-thumb__form');
    expect(forms).toHaveLength(1);
    expect(forms[0].querySelectorAll('.tpl-thumb__frow')).toHaveLength(2);
  });

  it('根级截前 4 + 溢出 +N', () => {
    const tree = ['a', 'b', 'c', 'd', 'e', 'f'].map((id) => n(id, 'text'));
    const { container } = render(<TemplateThumb tree={tree} />);
    expect(container.querySelectorAll('.tpl-thumb__text')).toHaveLength(4);
    expect(screen.getByText('+2')).toBeInTheDocument();
  });
});
