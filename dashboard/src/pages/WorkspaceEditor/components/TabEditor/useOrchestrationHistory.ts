import { useState, useEffect, useCallback } from 'react';
import { message } from 'antd';

function cloneLayout(layout: any): any {
  try {
    return JSON.parse(JSON.stringify(layout ?? {}));
  } catch {
    return layout;
  }
}

export interface UseOrchestrationHistoryOptions {
  maxStackSize?: number;
  onLayoutChange: (layout: any) => void;
  getCurrentLayout?: () => any;
  enableHotkeys?: boolean;
}

export function useOrchestrationHistory({
  maxStackSize = 10,
  onLayoutChange,
  getCurrentLayout,
  enableHotkeys = false,
}: UseOrchestrationHistoryOptions) {
  const [undoStack, setUndoStack] = useState<any[]>([]);
  const [redoStack, setRedoStack] = useState<any[]>([]);

  const pushToHistory = useCallback(
    (currentLayout: any) => {
      setUndoStack((prev) => [...prev.slice(-(maxStackSize - 1)), cloneLayout(currentLayout)]);
      setRedoStack([]);
    },
    [maxStackSize],
  );

  const undo = useCallback(
    (currentLayout: any) => {
      if (undoStack.length === 0) return;
      const previous = undoStack[undoStack.length - 1];
      setUndoStack((prev) => prev.slice(0, -1));
      setRedoStack((prev) => [...prev, cloneLayout(currentLayout)]);
      onLayoutChange(previous);
      message.success('已撤销上一次编排变更');
    },
    [undoStack, onLayoutChange],
  );

  const redo = useCallback(
    (currentLayout: any) => {
      if (redoStack.length === 0) return;
      const next = redoStack[redoStack.length - 1];
      setRedoStack((prev) => prev.slice(0, -1));
      setUndoStack((prev) => [...prev.slice(-(maxStackSize - 1)), cloneLayout(currentLayout)]);
      onLayoutChange(next);
      message.success('已恢复上一次撤销的编排变更');
    },
    [redoStack, maxStackSize, onLayoutChange],
  );

  const clearRedoStack = useCallback(() => {
    setRedoStack([]);
  }, []);

  // 键盘快捷键：Ctrl/Cmd + Alt + Z / Y
  useEffect(() => {
    if (!enableHotkeys || !getCurrentLayout) return;
    const onKeyDown = (e: KeyboardEvent) => {
      if (!(e.ctrlKey || e.metaKey)) return;
      if (e.altKey && e.key.toLowerCase() === 'z') {
        e.preventDefault();
        undo(getCurrentLayout());
        return;
      }
      if (e.altKey && e.key.toLowerCase() === 'y') {
        e.preventDefault();
        redo(getCurrentLayout());
      }
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [enableHotkeys, getCurrentLayout, undo, redo]);

  return {
    undoStack,
    redoStack,
    pushToHistory,
    undo,
    redo,
    clearRedoStack,
    canUndo: undoStack.length > 0,
    canRedo: redoStack.length > 0,
  };
}
