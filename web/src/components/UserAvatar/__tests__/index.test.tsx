/**
 * 头像组件回归（docs/BUGS.md BUG-012）。
 *
 * 修复前的问题：
 * 1. 顶栏头像恒为占位——`initialState.currentUser.avatar` 声明了字段却从不被
 *    写入（toCurrentUser 丢字段、getInitialState 不回填），用户上传的头像在顶栏
 *    永远不显示；
 * 2. 占位统一是 `<UserOutlined />`，一屏多用户无法区分谁是谁，且看起来像同一张
 *    假图；改为姓名首字母；
 * 3. `src` 走 normalizeAvatarSrc 而 `icon` 判原始值，遇到纯空白头像会「既没有
 *    图也没有占位」，渲染成空圆圈。
 */
import React from 'react';
import { render, screen } from '@testing-library/react';
import UserAvatar, { avatarInitials, AvatarFallback } from '../index';

describe('avatarInitials 首字母规则', () => {
  it('中文等连续文本取首字', () => {
    expect(avatarInitials('系统管理员', 'admin')).toBe('系');
    expect(avatarInitials('张', 'zhang')).toBe('张');
  });

  it('英文多词取首字母（大写，最多 2 个）', () => {
    expect(avatarInitials('John Ronald Reuel Tolkien', 't')).toBe('JR');
    expect(avatarInitials('Ada Lovelace', 'a')).toBe('AL');
  });

  it('单个英文单词取首字母', () => {
    expect(avatarInitials('admin', 'admin')).toBe('A');
  });

  it('name 缺失时回退 username', () => {
    expect(avatarInitials('', 'super_admin')).toBe('S');
    expect(avatarInitials(undefined, undefined)).toBe('?');
    expect(avatarInitials('   ', '  ')).toBe('?');
  });

  it('首尾空白被裁掉', () => {
    expect(avatarInitials('  admin  ', 'x')).toBe('A');
  });
});

describe('UserAvatar 渲染', () => {
  it('有真实头像时渲染 img，且不渲染占位块', () => {
    const url = 'https://cdn.example.com/avatars/a.png';
    const { container } = render(<UserAvatar src={url} name="系统管理员" username="admin" />);
    expect(container.querySelector(`img[src="${url}"]`)).not.toBeNull();
    expect(screen.queryByTestId('avatar-fallback')).toBeNull();
  });

  it('无头像时用首字母占位，且不产生空 src 的 img', () => {
    const { container } = render(<UserAvatar src="" name="系统管理员" username="admin" />);
    // BUG-012：不能出现 <img src="">（浏览器会当成当前页 URL 重新请求）
    expect(container.querySelector('img[src=""]')).toBeNull();
    expect(screen.getByTestId('avatar-fallback')).toHaveTextContent('系');
  });

  it('纯空白头像同样走占位（src 与占位用同一归一结果，不会渲染空圆圈）', () => {
    const { container } = render(<UserAvatar src="   " name="系统管理员" username="admin" />);
    expect(container.querySelector('img')).toBeNull();
    expect(screen.getByTestId('avatar-fallback')).toBeInTheDocument();
  });

  it('头像与姓名都缺失时占位为 "?"', () => {
    render(<UserAvatar />);
    expect(screen.getByTestId('avatar-fallback')).toHaveTextContent('?');
  });

  it('data-avatar-src 反映归一后的真实值，便于断言', () => {
    const { container } = render(<UserAvatar src="  " name="A" username="a" />);
    expect(container.querySelector('[data-testid="user-avatar"]')?.getAttribute('data-avatar-src'))
      .toBe('');
  });
});

describe('AvatarFallback 独立渲染', () => {
  it('渲染给定首字母与尺寸', () => {
    const { container } = render(<AvatarFallback initials="XY" size={64} />);
    const el = screen.getByTestId('avatar-fallback');
    expect(el).toHaveTextContent('XY');
    expect(el.style.width).toBe('64px');
    expect(el.style.borderRadius).toBe('50%');
    expect(container).toBeTruthy();
  });
});
