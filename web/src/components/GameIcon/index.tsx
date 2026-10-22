/**
 * GameIcon 游戏图标：优先渲染游戏资料 icon 字段（games.icon，URL）；
 * 无 icon 或加载失败时一律兜底为内置彩色骰子占位图（设计资产
 * /dice-fallback.svg，不许改色/改形/代餐）。渲染小尺寸（24/32）圆角，
 * 规范上限 64x64，超界自动钳制。
 */
import React, { useEffect, useState } from 'react';
import classNames from 'classnames';
import styles from './index.less';

export const DICE_FALLBACK_SRC = '/dice-fallback.svg';

/** 尺寸规范上限（px） */
export const GAME_ICON_MAX_SIZE = 64;
/** 尺寸规范下限（px），防误传 0/负数 */
export const GAME_ICON_MIN_SIZE = 16;

/** 常用尺寸：列表/下拉 24，当前选中/摘要 32 */
export const GAME_ICON_SIZE = { sm: 24, md: 32 } as const;

export function clampGameIconSize(px: number): number {
  return Math.min(Math.max(Math.round(px), GAME_ICON_MIN_SIZE), GAME_ICON_MAX_SIZE);
}

type GameIconProps = {
  /** 游戏资料 icon 字段（URL/路径）；空值兜底骰子 */
  icon?: string;
  /** 游戏名，用于 alt 无障碍文案 */
  name?: string;
  /** 渲染尺寸（正方形边长，px），默认 24，钳制到 [16, 64] */
  size?: number;
  className?: string;
};

const GameIcon: React.FC<GameIconProps> = ({ icon, name, size = GAME_ICON_SIZE.sm, className }) => {
  const [failed, setFailed] = useState(false);

  // icon 值变化（组件复用切换游戏）时重置失败态，给新地址重新加载机会
  useEffect(() => {
    setFailed(false);
  }, [icon]);

  const clamped = clampGameIconSize(size);
  const src = !icon || failed ? DICE_FALLBACK_SRC : icon;

  return (
    <img
      src={src}
      alt={name ? `${name} icon` : 'game icon'}
      width={clamped}
      height={clamped}
      className={classNames(styles.gameIcon, className)}
      onError={() => setFailed(true)}
      loading="lazy"
      draggable={false}
    />
  );
};

export default GameIcon;
