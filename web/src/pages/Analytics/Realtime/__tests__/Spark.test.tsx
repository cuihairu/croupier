import { render } from '@testing-library/react';
import Spark from '../Spark';

/** Spark 分支面：空数据占位、退化序列（x/y 全等走防除零分支）、常规折线路径。 */
describe('Spark 趋势折线', () => {
  it('data 为 null/undefined 或少于 2 点时渲染空占位（不画线）', () => {
    for (const data of [null, undefined, [], [[1, 2]] as [number, number][]]) {
      const { container, unmount } = render(
        <Spark data={data as unknown as [number, number][]} />,
      );
      expect(container.querySelector('svg')).toBeNull();
      expect(container.firstChild).toHaveStyle({ height: '40px' });
      unmount();
    }
  });

  it('x 全等（同一时间戳多点）走防除零分支仍成线', () => {
    const { container } = render(<Spark data={[[5, 1], [5, 3]]} />);
    const path = container.querySelector('path');
    expect(path).not.toBeNull();
    // x1===x0 → sx 恒 p=3；两点都在 x=3 处（垂直线段）
    expect(path!.getAttribute('d')).toContain('M3,');
    expect(path!.getAttribute('d')).toMatch(/L3,/);
  });

  it('y 全等（常数值序列）走防除零分支落在底边', () => {
    const { container } = render(<Spark data={[[1, 7], [2, 7], [3, 7]]} />);
    const d = container.querySelector('path')!.getAttribute('d')!;
    // y1===y0 → sy 恒 h-p=37：所有点的 y 坐标都是 37（水平线段）
    const ys = d.split(/[ML]/).filter(Boolean).map((seg) => seg.trim().split(',')[1]);
    expect(ys.every((y) => y === '37')).toBe(true);
  });

  it('常规序列生成 M 首点 + L 续点的折线路径', () => {
    const { container } = render(<Spark data={[[1, 1], [2, 2], [3, 3]]} />);
    const d = container.querySelector('path')!.getAttribute('d')!;
    expect(d.startsWith('M')).toBe(true);
    expect((d.match(/L/g) || []).length).toBe(2);
    expect(container.querySelector('svg')).toHaveAttribute('width', '100%');
  });
});
