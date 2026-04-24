/**
 * 预览区高亮 Hook
 *
 * 用于在预览区元素上添加点击事件，触发配置区高亮。
 *
 * @module pages/WorkspaceEditor/hooks/usePreviewHighlight
 */

import { useCallback } from 'react';
import {
  highlightConfig,
  highlightPreview,
  type HighlightTarget,
} from '../utils/previewConfigLink';

export interface UsePreviewHighlightOptions {
  /** Tab 标识 */
  tabKey?: string;
  /** 布局类型 */
  layoutType?: string;
}

/**
 * 预览区高亮 Hook
 *
 * 为预览元素添加点击事件，点击时在配置区高亮对应配置项
 */
export function usePreviewHighlight(options: UsePreviewHighlightOptions = {}) {
  const { tabKey, layoutType } = options;

  /**
   * 创建点击处理器
   */
  const createClickHandler = useCallback(
    (type: 'field' | 'column' | 'section', key: string, path?: string[]) => {
      return (event: React.MouseEvent) => {
        event.stopPropagation();

        highlightConfig({
          type,
          key,
          tabKey,
          layoutType,
          path,
        });
      };
    },
    [tabKey, layoutType],
  );

  /**
   * 创建悬停处理器（用于预览区元素悬停时显示高亮）
   */
  const createMouseEnterHandler = useCallback(
    (type: 'field' | 'column' | 'section', key: string) => {
      return (event: React.MouseEvent) => {
        event.stopPropagation();

        highlightConfig({
          type,
          key,
          tabKey,
          layoutType,
        });
      };
    },
    [tabKey, layoutType],
  );

  /**
   * 创建悬停离开处理器
   */
  const createMouseLeaveHandler = useCallback(() => {
    return () => {
      // 高亮会自动清除，这里可以什么都不做
    };
  }, []);

  /**
   * 为预览元素生成 data 属性
   */
  const getDataAttributes = useCallback(
    (type: 'field' | 'column' | 'section', key: string) => {
      const attrs: Record<string, string> = {};

      switch (type) {
        case 'field':
          attrs['data-field-key'] = key;
          break;
        case 'column':
          attrs['data-column-key'] = key;
          break;
        case 'section':
          attrs['data-section-key'] = key;
          break;
      }

      if (tabKey) {
        attrs['data-tab'] = tabKey;
      }

      return attrs;
    },
    [tabKey],
  );

  return {
    createClickHandler,
    createMouseEnterHandler,
    createMouseLeaveHandler,
    getDataAttributes,
  };
}

/**
 * 配置区高亮 Hook
 *
 * 用于配置项悬停时在预览区高亮对应元素
 */
export function useConfigHighlight() {
  const [highlightTarget, setHighlightTarget] = React.useState<HighlightTarget | null>(null);

  React.useEffect(() => {
    const handlePreviewHighlight = (event: CustomEvent) => {
      const target = event.detail as HighlightTarget;
      setHighlightTarget(target);

      // 2秒后自动清除
      setTimeout(() => {
        setHighlightTarget(null);
      }, 2000);
    };

    window.addEventListener('preview-highlight', handlePreviewHighlight as EventListener);

    return () => {
      window.removeEventListener('preview-highlight', handlePreviewHighlight as EventListener);
    };
  }, []);

  /**
   * 获取预览区高亮样式
   */
  const getPreviewHighlightStyle = (
    targetKey: string,
    targetType: string,
  ): React.CSSProperties | undefined => {
    if (!highlightTarget) return undefined;

    const match =
      (targetType === 'field' &&
        highlightTarget.type === 'field' &&
        highlightTarget.key === targetKey) ||
      (targetType === 'column' &&
        highlightTarget.type === 'column' &&
        highlightTarget.key === targetKey) ||
      (targetType === 'section' &&
        highlightTarget.type === 'section' &&
        highlightTarget.key === targetKey);

    if (match) {
      return {
        outline: '2px solid #1890ff',
        outlineOffset: 2,
        boxShadow: '0 0 8px rgba(24, 144, 255, 0.5)',
        transition: 'all 0.3s',
      };
    }

    return undefined;
  };

  /**
   * 手动触发预览区高亮
   */
  const triggerHighlight = useCallback((target: HighlightTarget) => {
    highlightPreview(target);
  }, []);

  return {
    highlightTarget,
    getPreviewHighlightStyle,
    triggerHighlight,
  };
}
