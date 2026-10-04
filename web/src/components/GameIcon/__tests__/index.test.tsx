/**
 * GameIcon 回归：icon 优先、空值/加载失败兜底骰子、尺寸钳制上限 64、
 * icon 值变化重置失败态（组件复用切游戏不残留兜底）。
 */
import React from 'react';
import { fireEvent, render, screen } from '@testing-library/react';
import GameIcon, { clampGameIconSize, DICE_FALLBACK_SRC, GAME_ICON_MAX_SIZE } from '../index';

describe('clampGameIconSize', () => {
  it('规范上限 64、下限 16，四舍五入取整', () => {
    expect(clampGameIconSize(24)).toBe(24);
    expect(clampGameIconSize(32)).toBe(32);
    expect(clampGameIconSize(200)).toBe(GAME_ICON_MAX_SIZE);
    expect(clampGameIconSize(0)).toBe(16);
    expect(clampGameIconSize(-8)).toBe(16);
    expect(clampGameIconSize(23.6)).toBe(24);
  });
});

describe('GameIcon', () => {
  it('无 icon：渲染骰子兜底图（src=/dice-fallback.svg）', () => {
    render(<GameIcon name="demo" />);
    const img = screen.getByAltText('demo icon') as HTMLImageElement;
    expect(img.getAttribute('src')).toBe(DICE_FALLBACK_SRC);
    expect(img.getAttribute('width')).toBe('24');
    expect(img.getAttribute('height')).toBe('24');
  });

  it('有 icon：渲染游戏资料地址，不走兜底', () => {
    render(<GameIcon icon="https://cdn.example.com/g1.png" name="game1" />);
    const img = screen.getByAltText('game1 icon') as HTMLImageElement;
    expect(img.getAttribute('src')).toBe('https://cdn.example.com/g1.png');
  });

  it('icon 加载失败：兜底骰子', () => {
    render(<GameIcon icon="https://cdn.example.com/broken.png" name="broken" />);
    const img = screen.getByAltText('broken icon') as HTMLImageElement;
    expect(img.getAttribute('src')).toBe('https://cdn.example.com/broken.png');
    fireEvent.error(img);
    expect(img.getAttribute('src')).toBe(DICE_FALLBACK_SRC);
  });

  it('尺寸超 64 钳制到 64（规范上限）', () => {
    render(<GameIcon size={128} />);
    const img = screen.getByAltText('game icon');
    expect(img.getAttribute('width')).toBe('64');
    expect(img.getAttribute('height')).toBe('64');
  });

  it('icon 值变化重置失败态：新地址恢复渲染、不再兜底', () => {
    const { rerender } = render(<GameIcon icon="https://cdn.example.com/a.png" name="g" />);
    const img = screen.getByAltText('g icon') as HTMLImageElement;
    fireEvent.error(img);
    expect(img.getAttribute('src')).toBe(DICE_FALLBACK_SRC);

    rerender(<GameIcon icon="https://cdn.example.com/b.png" name="g" />);
    expect(img.getAttribute('src')).toBe('https://cdn.example.com/b.png');
  });
});
