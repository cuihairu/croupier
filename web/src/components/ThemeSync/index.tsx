/**
 * 主题生效器：把当前生效主题（亮/暗）同步进 antd 运行时配置与静态方法。
 *
 * - useAntdConfigSetter 推送 algorithm/token —— ConfigProvider 树内的组件
 *   （表格/表单/弹窗/按钮……）即时换肤；
 * - ConfigProvider.config 同步静态方法（message/notification/Modal.confirm
 *   不在 React 树内，吃的是全局静态配置）。
 *
 * 挂载点：app.tsx innerProvider —— 位于 plugin-antd rootContainer 的
 * ConfigProvider 之内、路由之外，登录页（layout:false）同样被覆盖。
 * 首帧的正确 algorithm 由 app.tsx 的 antd 运行时导出兜底，这里只管「切换」。
 */
import React, { useEffect, useRef } from 'react';
import { ConfigProvider, type ThemeConfig } from 'antd';
import { useAntdConfigSetter } from '@umijs/max';
import { useResolvedTheme } from '@/hooks/useThemePref';
import { getAntdThemeConfig } from '@/utils/antdThemeConfig';

export const ThemeSync: React.FC = () => {
  const resolved = useResolvedTheme();
  const setAntdConfig = useAntdConfigSetter();
  // umi 模板的 setter 每次渲染都是新引用（AntdProvider 内联创建，未 memo）：
  // 直接进 effect 依赖会「推配置 → AntdProvider 重渲染 → setter 换引用 →
  // effect 再推」死循环，React 抛 Maximum update depth 后整树卸载（线上
  // 全站白屏，jest 侧因 mock setter 恒稳而测不出）。latest-ref 持有最新
  // setter，effect 依赖只剩 resolved。
  const setterRef = useRef(setAntdConfig);
  setterRef.current = setAntdConfig;

  useEffect(() => {
    const theme: ThemeConfig = getAntdThemeConfig(resolved);
    // token 浅深两层合并：config.ts 里的既有 token（如 borderRadius）不被清掉
    setterRef.current((prev) => ({
      ...prev,
      theme: {
        ...prev.theme,
        ...theme,
        token: { ...prev.theme?.token, ...theme.token },
      },
    }));
    ConfigProvider.config({ theme });
  }, [resolved]);

  return null;
};
