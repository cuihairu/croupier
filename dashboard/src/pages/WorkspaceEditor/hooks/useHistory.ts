/**
 * 历史记录管理 Hook
 *
 * 提供撤销/重做功能，支持键盘快捷键。
 *
 * @module pages/WorkspaceEditor/hooks/useHistory
 */

import { useState, useCallback, useEffect, useRef } from 'react';

/**
 * 历史状态接口
 */
export interface HistoryState<T> {
  /** 过去的状态 */
  past: T[];
  /** 当前状态 */
  present: T;
  /** 未来的状态（用于重做） */
  future: T[];
}

/**
 * 历史记录选项
 */
export interface HistoryOptions {
  /** 最大历史记录数 */
  maxHistory?: number;
  /** 是否启用键盘快捷键 */
  enableShortcuts?: boolean;
  /** 防抖延迟（毫秒） */
  debounceMs?: number;
}

/**
 * 历史记录返回值
 */
export interface HistoryResult<T> {
  /** 当前状态 */
  state: T;
  /** 设置新状态 */
  setState: (newState: T, description?: string) => void;
  /** 撤销 */
  undo: () => void;
  /** 重做 */
  redo: () => void;
  /** 是否可以撤销 */
  canUndo: boolean;
  /** 是否可以重做 */
  canRedo: boolean;
  /** 清空历史 */
  clear: () => void;
  /** 重置到指定状态 */
  reset: (state: T) => void;
  /** 历史记录数量 */
  historyLength: number;
  /** 当前历史索引 */
  historyIndex: number;
  /** 历史记录描述列表 */
  historyDescriptions: string[];
}

/**
 * 历史记录项
 */
interface HistoryEntry<T> {
  state: T;
  description: string;
  timestamp: number;
}

/**
 * 历史记录管理 Hook
 *
 * @param initialState - 初始状态
 * @param options - 配置选项
 * @returns 历史记录管理对象
 */
export function useHistory<T>(initialState: T, options: HistoryOptions = {}): HistoryResult<T> {
  const { maxHistory = 50, enableShortcuts = true, debounceMs = 100 } = options;

  // 使用 ref 存储防抖定时器
  const debounceTimerRef = useRef<NodeJS.Timeout | null>(null);
  const pendingStateRef = useRef<{ state: T; description?: string } | null>(null);

  // 历史记录状态
  const [history, setHistory] = useState<HistoryEntry<T>[]>([
    {
      state: initialState,
      description: '初始状态',
      timestamp: Date.now(),
    },
  ]);

  // 当前索引
  const [currentIndex, setCurrentIndex] = useState(0);

  // 获取当前状态
  const state = history[currentIndex]?.state ?? initialState;

  // 添加新状态到历史
  const pushState = useCallback(
    (newState: T, description: string = '更新') => {
      setHistory((prev) => {
        // 截断当前位置之后的历史
        const newHistory = prev.slice(0, currentIndex + 1);

        // 添加新状态
        newHistory.push({
          state: newState,
          description,
          timestamp: Date.now(),
        });

        // 限制历史记录数量
        if (newHistory.length > maxHistory) {
          return newHistory.slice(-maxHistory);
        }

        return newHistory;
      });

      setCurrentIndex((prev) => Math.min(prev + 1, maxHistory - 1));
    },
    [currentIndex, maxHistory],
  );

  // 防抖设置状态
  const debouncedPushState = useCallback(
    (newState: T, description?: string) => {
      pendingStateRef.current = { state: newState, description };

      if (debounceTimerRef.current) {
        clearTimeout(debounceTimerRef.current);
      }

      debounceTimerRef.current = setTimeout(() => {
        if (pendingStateRef.current) {
          pushState(pendingStateRef.current.state, pendingStateRef.current.description || '更新');
          pendingStateRef.current = null;
        }
      }, debounceMs);
    },
    [pushState, debounceMs],
  );

  // 设置状态
  const setState = useCallback(
    (newState: T, description?: string) => {
      // 使用防抖版本
      debouncedPushState(newState, description);
    },
    [debouncedPushState],
  );

  // 立即设置状态（不防抖）
  const setStateImmediate = useCallback(
    (newState: T, description: string = '更新') => {
      if (debounceTimerRef.current) {
        clearTimeout(debounceTimerRef.current);
      }
      pushState(newState, description);
    },
    [pushState],
  );

  // 撤销
  const undo = useCallback(() => {
    if (currentIndex > 0) {
      setCurrentIndex((prev) => prev - 1);
    }
  }, [currentIndex]);

  // 重做
  const redo = useCallback(() => {
    if (currentIndex < history.length - 1) {
      setCurrentIndex((prev) => prev + 1);
    }
  }, [currentIndex, history.length]);

  // 清空历史
  const clear = useCallback(() => {
    setHistory([
      {
        state,
        description: '初始状态',
        timestamp: Date.now(),
      },
    ]);
    setCurrentIndex(0);
  }, [state]);

  // 重置到指定状态
  const reset = useCallback((newState: T) => {
    setHistory([
      {
        state: newState,
        description: '重置',
        timestamp: Date.now(),
      },
    ]);
    setCurrentIndex(0);
  }, []);

  // 键盘快捷键
  useEffect(() => {
    if (!enableShortcuts) return;

    const handleKeyDown = (e: KeyboardEvent) => {
      // Ctrl/Cmd + Z: 撤销
      if ((e.ctrlKey || e.metaKey) && e.key === 'z' && !e.shiftKey) {
        e.preventDefault();
        undo();
      }
      // Ctrl/Cmd + Shift + Z 或 Ctrl/Cmd + Y: 重做
      else if ((e.ctrlKey || e.metaKey) && ((e.key === 'z' && e.shiftKey) || e.key === 'y')) {
        e.preventDefault();
        redo();
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [enableShortcuts, undo, redo]);

  // 清理定时器
  useEffect(() => {
    return () => {
      if (debounceTimerRef.current) {
        clearTimeout(debounceTimerRef.current);
      }
    };
  }, []);

  return {
    state,
    setState,
    undo,
    redo,
    canUndo: currentIndex > 0,
    canRedo: currentIndex < history.length - 1,
    clear,
    reset,
    historyLength: history.length,
    historyIndex: currentIndex,
    historyDescriptions: history.map((h) => h.description),
  };
}

/**
 * 简化版历史记录 Hook
 *
 * 不使用防抖，适用于需要精确控制的场景
 */
export function useSimpleHistory<T>(
  initialState: T,
  maxHistory: number = 50,
): Omit<HistoryResult<T>, 'historyDescriptions'> & { historyDescriptions: string[] } {
  const [past, setPast] = useState<T[]>([]);
  const [present, setPresent] = useState<T>(initialState);
  const [future, setFuture] = useState<T[]>([]);
  const [descriptions, setDescriptions] = useState<string[]>(['初始状态']);

  // 设置新状态
  const setState = useCallback(
    (newState: T, description: string = '更新') => {
      setPast((prev) => {
        const newPast = [...prev, present];
        if (newPast.length > maxHistory) {
          return newPast.slice(-maxHistory);
        }
        return newPast;
      });
      setPresent(newState);
      setFuture([]);
      setDescriptions((prev) => {
        const newDesc = [...prev, description];
        if (newDesc.length > maxHistory) {
          return newDesc.slice(-maxHistory);
        }
        return newDesc;
      });
    },
    [present, maxHistory],
  );

  // 撤销
  const undo = useCallback(() => {
    if (past.length === 0) return;

    const previous = past[past.length - 1];
    const newPast = past.slice(0, past.length - 1);

    setFuture((prev) => [present, ...prev]);
    setPast(newPast);
    setPresent(previous);
  }, [past, present]);

  // 重做
  const redo = useCallback(() => {
    if (future.length === 0) return;

    const next = future[0];
    const newFuture = future.slice(1);

    setPast((prev) => [...prev, present]);
    setFuture(newFuture);
    setPresent(next);
  }, [future, present]);

  // 清空历史
  const clear = useCallback(() => {
    setPast([]);
    setFuture([]);
    setDescriptions(['初始状态']);
  }, []);

  // 重置
  const reset = useCallback((newState: T) => {
    setPast([]);
    setPresent(newState);
    setFuture([]);
    setDescriptions(['重置']);
  }, []);

  return {
    state: present,
    setState,
    undo,
    redo,
    canUndo: past.length > 0,
    canRedo: future.length > 0,
    clear,
    reset,
    historyLength: past.length + 1 + future.length,
    historyIndex: past.length,
    historyDescriptions: descriptions,
  };
}

export default useHistory;
