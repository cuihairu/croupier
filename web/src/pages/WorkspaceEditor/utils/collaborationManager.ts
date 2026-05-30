/**
 * 协作编辑管理
 *
 * 管理多人编辑时的冲突检测和编辑锁定机制。
 *
 * @module pages/WorkspaceEditor/utils/collaborationManager
 */

import { getCurrentUser } from './permissionManager';

/** 编辑会话信息 */
export interface EditSession {
  /** 会话 ID */
  sessionId: string;
  /** 用户信息 */
  user: {
    id: string;
    name: string;
    roles: string[];
    avatar?: string;
  };
  /** 编辑的对象 key */
  objectKey: string;
  /** 会话开始时间 */
  startTime: number;
  /** 最后心跳时间 */
  lastHeartbeat: number;
  /** 是否持有编辑锁 */
  hasLock: boolean;
  /** 锁的过期时间 */
  lockExpireTime?: number;
}

/** 锁状态 */
export interface LockStatus {
  /** 是否已锁定 */
  isLocked: boolean;
  /** 锁持有者信息 */
  lockHolder?: {
    id: string;
    name: string;
    avatar?: string;
  };
  /** 锁的剩余时间（毫秒） */
  remainingTime?: number;
  /** 是否由当前会话持有 */
  isHeldByCurrentSession: boolean;
}

/** 协作事件类型 */
export type CollaborationEventType =
  | 'session:join' // 用户加入
  | 'session:leave' // 用户离开
  | 'session:heartbeat' // 心跳更新
  | 'lock:acquire' // 获取锁
  | 'lock:release' // 释放锁
  | 'lock:steal' // 强制解锁
  | 'conflict:detect'; // 冲突检测

/** 协作事件 */
export interface CollaborationEvent {
  type: CollaborationEventType;
  sessionId: string;
  objectKey: string;
  user: EditSession['user'];
  timestamp: number;
  data?: any;
}

const CHANNEL_NAME = 'workspace-collaboration';
const STORAGE_KEY = 'workspace:edit_sessions';
const HEARTBEAT_INTERVAL = 10000; // 10秒
const SESSION_TIMEOUT = 60000; // 60秒无心跳视为超时
const LOCK_DURATION = 1800000; // 30分钟锁定时长

/**
 * 协作管理器类
 */
class CollaborationManager {
  private channel: BroadcastChannel | null = null;
  private currentSession: EditSession | null = null;
  private heartbeatTimer: NodeJS.Timeout | null = null;
  private listeners: Map<string, Set<(event: CollaborationEvent) => void>> = new Map();

  constructor() {
    if (typeof window !== 'undefined' && 'BroadcastChannel' in window) {
      this.channel = new BroadcastChannel(CHANNEL_NAME);
      this.channel.addEventListener('message', this.handleChannelMessage.bind(this));
    }
  }

  /**
   * 生成会话 ID
   */
  private generateSessionId(): string {
    return `session_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`;
  }

  /**
   * 获取所有活跃会话
   */
  private getActiveSessions(objectKey: string): EditSession[] {
    if (typeof window === 'undefined') return [];

    try {
      const data = localStorage.getItem(STORAGE_KEY);
      if (!data) return [];

      const allSessions: EditSession[] = JSON.parse(data);
      const now = Date.now();

      // 过滤掉超时的会话
      const activeSessions = allSessions.filter(
        (s) => s.objectKey === objectKey && now - s.lastHeartbeat < SESSION_TIMEOUT,
      );

      // 更新存储，清理过期会话
      if (activeSessions.length !== allSessions.length) {
        localStorage.setItem(STORAGE_KEY, JSON.stringify(activeSessions));
      }

      return activeSessions;
    } catch {
      return [];
    }
  }

  /**
   * 保存会话到存储
   */
  private saveSession(session: EditSession): void {
    if (typeof window === 'undefined') return;

    try {
      const sessions = this.getActiveSessions(session.objectKey);
      const existingIndex = sessions.findIndex((s) => s.sessionId === session.sessionId);

      if (existingIndex >= 0) {
        sessions[existingIndex] = session;
      } else {
        sessions.push(session);
      }

      localStorage.setItem(STORAGE_KEY, JSON.stringify(sessions));
    } catch (error) {
      console.error('Failed to save session:', error);
    }
  }

  /**
   * 从存储移除会话
   */
  private removeSession(sessionId: string, objectKey: string): void {
    if (typeof window === 'undefined') return;

    try {
      const sessions = this.getActiveSessions(objectKey);
      const filtered = sessions.filter((s) => s.sessionId !== sessionId);
      localStorage.setItem(STORAGE_KEY, JSON.stringify(filtered));
    } catch (error) {
      console.error('Failed to remove session:', error);
    }
  }

  /**
   * 广播事件
   */
  private broadcast(event: CollaborationEvent): void {
    if (this.channel) {
      this.channel.postMessage(event);
    }
  }

  /**
   * 处理收到的频道消息
   */
  private handleChannelMessage(event: MessageEvent): void {
    const collabEvent = event.data as CollaborationEvent;
    this.notifyListeners(collabEvent);

    // 如果是锁相关事件，更新当前会话的锁状态
    if (this.currentSession && collabEvent.objectKey === this.currentSession.objectKey) {
      if (
        collabEvent.type === 'lock:acquire' &&
        collabEvent.sessionId !== this.currentSession.sessionId
      ) {
        // 其他人获取了锁，更新当前会话状态
        this.currentSession.hasLock = false;
        this.currentSession.lockExpireTime = undefined;
        this.saveSession(this.currentSession);
      } else if (
        collabEvent.type === 'lock:release' &&
        collabEvent.sessionId !== this.currentSession.sessionId
      ) {
        // 其他人释放了锁，可以尝试获取
        this.updateLockStatus();
      } else if (collabEvent.type === 'lock:steal') {
        // 锁被强制解除
        this.currentSession.hasLock = false;
        this.currentSession.lockExpireTime = undefined;
        this.saveSession(this.currentSession);
      }
    }
  }

  /**
   * 通知事件监听器
   */
  private notifyListeners(event: CollaborationEvent): void {
    const key = `${event.objectKey}:${event.type}`;
    const listeners = this.listeners.get(key);
    if (listeners) {
      listeners.forEach((fn) => fn(event));
    }

    // 通知通用的 objectKey 监听器
    const genericListeners = this.listeners.get(event.objectKey);
    if (genericListeners) {
      genericListeners.forEach((fn) => fn(event));
    }
  }

  /**
   * 启动心跳
   */
  private startHeartbeat(): void {
    this.stopHeartbeat();

    this.heartbeatTimer = setInterval(() => {
      if (this.currentSession) {
        this.currentSession.lastHeartbeat = Date.now();
        this.saveSession(this.currentSession);

        this.broadcast({
          type: 'session:heartbeat',
          sessionId: this.currentSession.sessionId,
          objectKey: this.currentSession.objectKey,
          user: this.currentSession.user,
          timestamp: Date.now(),
        });
      }
    }, HEARTBEAT_INTERVAL);
  }

  /**
   * 停止心跳
   */
  private stopHeartbeat(): void {
    if (this.heartbeatTimer) {
      clearInterval(this.heartbeatTimer);
      this.heartbeatTimer = null;
    }
  }

  /**
   * 更新锁状态
   */
  private updateLockStatus(): void {
    if (!this.currentSession) return;

    const sessions = this.getActiveSessions(this.currentSession.objectKey);
    const lockHolder = sessions.find(
      (s) => s.hasLock && s.lockExpireTime && s.lockExpireTime > Date.now(),
    );

    if (!lockHolder || lockHolder.sessionId === this.currentSession.sessionId) {
      // 没有锁持有者或锁已过期，当前会话获取锁
      this.currentSession.hasLock = true;
      this.currentSession.lockExpireTime = Date.now() + LOCK_DURATION;
      this.saveSession(this.currentSession);

      this.broadcast({
        type: 'lock:acquire',
        sessionId: this.currentSession.sessionId,
        objectKey: this.currentSession.objectKey,
        user: this.currentSession.user,
        timestamp: Date.now(),
      });
    }
  }

  /**
   * 加入编辑会话
   */
  join(objectKey: string): EditSession {
    const currentUser = getCurrentUser();
    if (!currentUser) {
      throw new Error('无法获取当前用户信息');
    }

    // 检查是否已有会话
    if (this.currentSession && this.currentSession.objectKey === objectKey) {
      return this.currentSession;
    }

    // 如果已有其他对象的会话，先离开
    if (this.currentSession) {
      this.leave();
    }

    const sessionId = this.generateSessionId();
    const now = Date.now();

    const session: EditSession = {
      sessionId,
      user: {
        id: currentUser.id,
        name: currentUser.name,
        roles: currentUser.roles,
        avatar: currentUser.avatar,
      },
      objectKey,
      startTime: now,
      lastHeartbeat: now,
      hasLock: false,
    };

    this.currentSession = session;
    this.saveSession(session);

    // 广播加入事件
    this.broadcast({
      type: 'session:join',
      sessionId,
      objectKey,
      user: session.user,
      timestamp: now,
    });

    // 尝试获取编辑锁
    this.updateLockStatus();

    // 启动心跳
    this.startHeartbeat();

    return session;
  }

  /**
   * 离开编辑会话
   */
  leave(): void {
    if (!this.currentSession) return;

    const { sessionId, objectKey, hasLock } = this.currentSession;

    // 如果持有锁，释放锁
    if (hasLock) {
      this.releaseLock();
    }

    // 广播离开事件
    this.broadcast({
      type: 'session:leave',
      sessionId,
      objectKey,
      user: this.currentSession.user,
      timestamp: Date.now(),
    });

    // 从存储移除
    this.removeSession(sessionId, objectKey);

    // 停止心跳
    this.stopHeartbeat();

    this.currentSession = null;
  }

  /**
   * 获取当前会话
   */
  getCurrentSession(): EditSession | null {
    return this.currentSession;
  }

  /**
   * 获取所有活跃的编辑者
   */
  getActiveEditors(objectKey: string): EditSession[] {
    return this.getActiveSessions(objectKey);
  }

  /**
   * 获取锁状态
   */
  getLockStatus(objectKey: string): LockStatus {
    const sessions = this.getActiveSessions(objectKey);
    const now = Date.now();

    // 找到持有锁的会话
    const lockHolder = sessions.find(
      (s) => s.hasLock && s.lockExpireTime && s.lockExpireTime > now,
    );

    if (!lockHolder) {
      return {
        isLocked: false,
        isHeldByCurrentSession: false,
      };
    }

    return {
      isLocked: true,
      lockHolder: lockHolder.user,
      remainingTime: (lockHolder.lockExpireTime || 0) - now,
      isHeldByCurrentSession: this.currentSession?.sessionId === lockHolder.sessionId,
    };
  }

  /**
   * 检测是否有冲突（多人同时编辑）
   */
  detectConflict(objectKey: string): { hasConflict: boolean; conflictWith?: EditSession[] } {
    const sessions = this.getActiveSessions(objectKey);
    const otherSessions = sessions.filter((s) => s.sessionId !== this.currentSession?.sessionId);

    return {
      hasConflict: otherSessions.length > 0,
      conflictWith: otherSessions.length > 0 ? otherSessions : undefined,
    };
  }

  /**
   * 尝试获取编辑锁
   */
  acquireLock(): { success: boolean; reason?: string } {
    if (!this.currentSession) {
      return { success: false, reason: '没有活跃的编辑会话' };
    }

    const lockStatus = this.getLockStatus(this.currentSession.objectKey);

    if (lockStatus.isLocked) {
      if (lockStatus.isHeldByCurrentSession) {
        // 续期锁
        this.currentSession.lockExpireTime = Date.now() + LOCK_DURATION;
        this.saveSession(this.currentSession);
        return { success: true };
      }

      return {
        success: false,
        reason: `当前由 ${lockStatus.lockHolder?.name} 编辑中`,
      };
    }

    // 获取锁
    this.currentSession.hasLock = true;
    this.currentSession.lockExpireTime = Date.now() + LOCK_DURATION;
    this.saveSession(this.currentSession);

    this.broadcast({
      type: 'lock:acquire',
      sessionId: this.currentSession.sessionId,
      objectKey: this.currentSession.objectKey,
      user: this.currentSession.user,
      timestamp: Date.now(),
    });

    return { success: true };
  }

  /**
   * 释放编辑锁
   */
  releaseLock(): { success: boolean; reason?: string } {
    if (!this.currentSession) {
      return { success: false, reason: '没有活跃的编辑会话' };
    }

    if (!this.currentSession.hasLock) {
      return { success: false, reason: '当前未持有编辑锁' };
    }

    this.currentSession.hasLock = false;
    this.currentSession.lockExpireTime = undefined;
    this.saveSession(this.currentSession);

    this.broadcast({
      type: 'lock:release',
      sessionId: this.currentSession.sessionId,
      objectKey: this.currentSession.objectKey,
      user: this.currentSession.user,
      timestamp: Date.now(),
    });

    return { success: true };
  }

  /**
   * 强制解锁（管理员功能）
   */
  forceUnlock(objectKey: string): { success: boolean; reason?: string } {
    const sessions = this.getActiveSessions(objectKey);
    const lockHolder = sessions.find((s) => s.hasLock);

    if (!lockHolder) {
      return { success: false, reason: '当前没有锁' };
    }

    // 清除所有会话的锁状态
    sessions.forEach((s) => {
      s.hasLock = false;
      s.lockExpireTime = undefined;
    });

    localStorage.setItem(STORAGE_KEY, JSON.stringify(sessions));

    // 更新当前会话
    if (this.currentSession && this.currentSession.objectKey === objectKey) {
      this.currentSession.hasLock = false;
      this.currentSession.lockExpireTime = undefined;
    }

    this.broadcast({
      type: 'lock:steal',
      sessionId: this.currentSession?.sessionId || 'system',
      objectKey,
      user: this.currentSession?.user || { id: 'system', name: '系统', roles: ['admin'] },
      timestamp: Date.now(),
      data: { originalHolder: lockHolder.sessionId },
    });

    return { success: true };
  }

  /**
   * 添加事件监听器
   */
  on(
    objectKey: string,
    eventType: CollaborationEventType | 'all',
    callback: (event: CollaborationEvent) => void,
  ): () => void {
    const key = eventType === 'all' ? objectKey : `${objectKey}:${eventType}`;

    if (!this.listeners.has(key)) {
      this.listeners.set(key, new Set());
    }

    this.listeners.get(key)!.add(callback);

    // 返回取消监听的函数
    return () => {
      const listeners = this.listeners.get(key);
      if (listeners) {
        listeners.delete(callback);
        if (listeners.size === 0) {
          this.listeners.delete(key);
        }
      }
    };
  }

  /**
   * 获取协作统计
   */
  getCollaborationStats(objectKey: string): {
    totalEditors: number;
    lockHolder?: string;
    lockRemainingTime?: number;
    isConflict: boolean;
  } {
    const sessions = this.getActiveSessions(objectKey);
    const lockStatus = this.getLockStatus(objectKey);
    const conflict = this.detectConflict(objectKey);

    return {
      totalEditors: sessions.length,
      lockHolder: lockStatus.lockHolder?.name,
      lockRemainingTime: lockStatus.remainingTime,
      isConflict: conflict.hasConflict,
    };
  }

  /**
   * 清理资源
   */
  dispose(): void {
    this.leave();
    if (this.channel) {
      this.channel.close();
      this.channel = null;
    }
    this.listeners.clear();
  }
}

// 单例
let managerInstance: CollaborationManager | null = null;

/**
 * 获取协作管理器实例
 */
export function getCollaborationManager(): CollaborationManager {
  if (!managerInstance) {
    managerInstance = new CollaborationManager();
  }
  return managerInstance;
}

/**
 * 格式化剩余时间
 */
export function formatRemainingTime(ms: number): string {
  if (ms <= 0) return '已过期';

  const minutes = Math.floor(ms / 60000);
  const seconds = Math.floor((ms % 60000) / 1000);

  if (minutes > 0) {
    return `${minutes}分${seconds}秒`;
  }
  return `${seconds}秒`;
}

/**
 * 组件卸载时自动清理
 */
export function useCollaborationCleanup() {
  if (typeof window !== 'undefined') {
    window.addEventListener('beforeunload', () => {
      getCollaborationManager().dispose();
    });
  }
}
