import React from 'react';
import { Avatar, theme } from 'antd';
import { normalizeAvatarSrc } from '@/pages/Profile/shared';

/**
 * 用户头像：真实头像优先，无头像时用**姓名首字母**占位。
 *
 * 为什么不用一个通用图标占位（此前是 `<UserOutlined />`）：一屏上多个用户时
 * 图标占位完全无法区分是谁，而首字母至少能对上人；且不同用户首字母不同，
 * 天然避免「看起来像同一张假图」（docs/BUGS.md BUG-012）。
 *
 * 归一化只做一次：`src` 与「是否显示占位」必须用**同一个**判断值。此前
 * `src` 走 normalizeAvatarSrc、`icon` 却直接判原始值，遇到纯空白字符串就会
 * 既没有图也没有占位（渲染成空圆圈）。
 */
export type UserAvatarProps = {
  /** 后端返回的头像地址；空串/空白/未设置都走首字母占位 */
  src?: string | null;
  /** 参与首字母计算的展示名（优先） */
  name?: string | null;
  /** 参与首字母计算的登录名（name 为空时回退） */
  username?: string | null;
  size?: number;
  className?: string;
  style?: React.CSSProperties;
  alt?: string;
};

/**
 * 取占位文字：优先中文/日文等「首字」，英文取首字母大写，最多 2 个字符。
 *
 * 规则刻意保持简单可预期：
 * - 去掉首尾空白；
 * - 以空白分隔的多词名（"John Ronald Reuel Tolkien"）取首字母 → "JR"；
 * - 连续文本（"系统管理员"）取首字 → "系"；
 * - 全为空 → "?"。
 */
export function avatarInitials(name?: string | null, username?: string | null): string {
  const source = (name ?? '').trim() || (username ?? '').trim();
  if (!source) return '?';

  const words = source.split(/\s+/).filter(Boolean);
  if (words.length > 1) {
    return words
      .slice(0, 2)
      .map((w) => Array.from(w)[0] ?? '')
      .join('')
      .toUpperCase();
  }
  const chars = Array.from(source);
  // 连续文本：中文等取首字；纯 ASCII 单词取首字母。
  return chars.length > 1 && /^[A-Za-z]/.test(source)
    ? (chars[0] as string).toUpperCase()
    : (chars[0] as string);
}

/**
 * 首字母占位的视觉块。单独导出便于单测直接断言文字，不依赖 antd 渲染结构。
 */
export function AvatarFallback({
  initials,
  size,
}: {
  initials: string;
  size?: number;
}): React.ReactElement {
  const { token } = theme.useToken();
  const fontSize = size ? Math.max(12, Math.round(size * 0.4)) : undefined;
  return (
    <div
      data-testid="avatar-fallback"
      style={{
        width: size,
        height: size,
        borderRadius: '50%',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        background: token.colorFillTertiary,
        color: token.colorTextSecondary,
        fontSize,
        fontWeight: 600,
        lineHeight: 1,
        userSelect: 'none',
        overflow: 'hidden',
      }}
    >
      {initials}
    </div>
  );
}

const UserAvatar: React.FC<UserAvatarProps> = ({
  src,
  name,
  username,
  size,
  className,
  style,
  alt,
}) => {
  const normalized = normalizeAvatarSrc(src);
  const initials = avatarInitials(name, username);

  return (
    <Avatar
      size={size}
      className={className}
      style={style}
      // 有真实头像才传 src；空串会触发浏览器把 <img src=""> 当成当前页重新请求
      src={normalized}
      alt={alt}
      // 无头像时用首字母块。src / 占位 二者共用同一个归一结果，避免二者判定
      // 不一致导致渲染出空圆圈。
      icon={normalized ? undefined : <AvatarFallback initials={initials} size={size} />}
      data-testid="user-avatar"
      data-avatar-src={normalized ?? ''}
    />
  );
};

export default React.memo(UserAvatar);
