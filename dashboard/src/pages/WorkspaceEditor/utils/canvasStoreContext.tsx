/**
 * 画布编辑器 React Context
 *
 * 提供 React Provider 和 hooks 用于画布编辑器状态管理。
 *
 * @module pages/WorkspaceEditor/utils/canvasStoreContext
 */

import React, { createContext, useContext, useReducer, useCallback } from 'react';
import {
  canvasReducer,
  INITIAL_STATE,
  type CanvasState,
  type CanvasAction,
  type CanvasComponent,
  type CanvasComponentTemplate,
  fromTabConfig,
  toTabConfig,
} from './canvasStore';

/** Context 值类型 */
interface CanvasContextValue {
  state: CanvasState;
  dispatch: React.Dispatch<CanvasAction>;
}

/** Context */
const CanvasContext = createContext<CanvasContextValue | null>(null);

/**
 * Provider：包裹整个画布编辑器
 */
export function CanvasProvider({ children }: { children: React.ReactNode }) {
  const [state, dispatch] = useReducer(canvasReducer, INITIAL_STATE);
  return <CanvasContext.Provider value={{ state, dispatch }}>{children}</CanvasContext.Provider>;
}

/**
 * 获取画布 context（内部使用）
 */
function useCanvas() {
  const ctx = useContext(CanvasContext);
  if (!ctx) throw new Error('useCanvas must be used within CanvasProvider');
  return ctx;
}

/**
 * 画布 Hook（替代原 useCanvasStore）
 * 接口与原 zustand store 保持一致，消费方无需改动。
 */
export function useCanvasStore() {
  const { state, dispatch } = useCanvas();

  const setRootComponent = useCallback(
    (component: CanvasComponent) => dispatch({ type: 'SET_ROOT', payload: component }),
    [dispatch],
  );

  const selectComponent = useCallback(
    (id: string | null) => dispatch({ type: 'SELECT', payload: id }),
    [dispatch],
  );

  const hoverComponent = useCallback(
    (id: string | null) => dispatch({ type: 'HOVER', payload: id }),
    [dispatch],
  );

  const addComponent = useCallback(
    (parentId: string | null, component: CanvasComponent, index?: number) =>
      dispatch({ type: 'ADD_COMPONENT', payload: { parentId, component, index } }),
    [dispatch],
  );

  const removeComponent = useCallback(
    (id: string) => dispatch({ type: 'REMOVE_COMPONENT', payload: id }),
    [dispatch],
  );

  const updateComponent = useCallback(
    (id: string, updates: Partial<CanvasComponent>) =>
      dispatch({ type: 'UPDATE_COMPONENT', payload: { id, updates } }),
    [dispatch],
  );

  const setDraggingComponent = useCallback(
    (component: CanvasComponent | null) => dispatch({ type: 'SET_DRAGGING', payload: component }),
    [dispatch],
  );

  const setResizing = useCallback(
    (resizing: boolean) => dispatch({ type: 'SET_RESIZING', payload: resizing }),
    [dispatch],
  );

  const undo = useCallback(() => dispatch({ type: 'UNDO' }), [dispatch]);
  const redo = useCallback(() => dispatch({ type: 'REDO' }), [dispatch]);
  const clearCanvas = useCallback(() => dispatch({ type: 'CLEAR' }), [dispatch]);

  return {
    ...state,
    setRootComponent,
    selectComponent,
    hoverComponent,
    addComponent,
    removeComponent,
    updateComponent,
    moveComponent: (_id: string, _targetParentId: string, _targetIndex: number) => {
      // TODO: 实现移动组件
    },
    setDraggingComponent,
    setResizing,
    undo,
    redo,
    fromTabConfig,
    toTabConfig,
    clearCanvas,
  };
}

// 重新导出类型
export type { CanvasComponent, CanvasState, CanvasComponentTemplate, CanvasAction };
