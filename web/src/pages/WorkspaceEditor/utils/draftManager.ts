/**
 * 统一草稿管理
 *
 * 管理配置草稿的保存、加载、冲突检测和过期清理。
 *
 * @module pages/WorkspaceEditor/utils/draftManager
 */

import { getCurrentUser } from './permissionManager';
import type { WorkspaceConfig } from '@/types/workspace';

/** 草稿元数据 */
export interface DraftMetadata {
  /** 草稿 ID */
  id: string;
  /** 对象标识 */
  objectKey: string;
  /** 创建者 */
  creator: {
    id: string;
    name: string;
    roles: string[];
  };
  /** 创建时间 */
  createdAt: number;
  /** 最后更新时间 */
  updatedAt: number;
  /** 草稿标题 */
  title?: string;
  /** 草稿描述 */
  description?: string;
  /** 配置版本（用于冲突检测） */
  version?: string;
  /** 是否自动保存 */
  isAutoSave?: boolean;
  /** 关联的发布版本 ID */
  basedOnVersionId?: string;
}

/** 草稿内容 */
export interface DraftContent extends DraftMetadata {
  /** 配置数据 */
  config: WorkspaceConfig;
}

/** 草稿冲突信息 */
export interface DraftConflict {
  /** 是否有冲突 */
  hasConflict: boolean;
  /** 冲突的草稿列表 */
  conflicts: DraftMetadata[];
  /** 当前用户的草稿 */
  currentUserDraft?: DraftMetadata;
}

/** 草稿列表项 */
export interface DraftListItem extends DraftMetadata {
  /** 是否可以恢复 */
  recoverable: boolean;
  /** 是否过期 */
  isExpired: boolean;
}

const STORAGE_KEY = 'workspace:drafts';
const DRAFT_EXPIRY_DAYS = 7; // 7天过期
const MS_PER_DAY = 24 * 60 * 60 * 1000;

/**
 * 生成草稿 ID
 */
function generateDraftId(): string {
  return `draft_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`;
}

/**
 * 获取所有草稿
 */
function getAllDrafts(): DraftContent[] {
  if (typeof window === 'undefined') return [];

  try {
    const data = localStorage.getItem(STORAGE_KEY);
    return data ? JSON.parse(data) : [];
  } catch {
    return [];
  }
}

/**
 * 保存所有草稿
 */
function saveAllDrafts(drafts: DraftContent[]): void {
  if (typeof window === 'undefined') return;

  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(drafts));
  } catch (error) {
    console.error('Failed to save drafts:', error);
  }
}

/**
 * 清理过期草稿
 */
export function cleanupExpiredDrafts(): number {
  const drafts = getAllDrafts();
  const now = Date.now();
  const expiryTime = DRAFT_EXPIRY_DAYS * MS_PER_DAY;

  const validDrafts = drafts.filter((draft) => {
    const age = now - draft.updatedAt;
    return age < expiryTime;
  });

  if (validDrafts.length !== drafts.length) {
    saveAllDrafts(validDrafts);
    return drafts.length - validDrafts.length;
  }

  return 0;
}

/**
 * 保存草稿
 */
export function saveDraft(
  objectKey: string,
  config: WorkspaceConfig,
  options?: {
    title?: string;
    description?: string;
    isAutoSave?: boolean;
    basedOnVersionId?: string;
    version?: string;
  },
): DraftContent {
  // 先清理过期草稿
  cleanupExpiredDrafts();

  const user = getCurrentUser();
  if (!user) {
    throw new Error('无法获取当前用户信息');
  }

  const drafts = getAllDrafts();
  const now = Date.now();

  // 查找是否已有当前用户的草稿
  const existingIndex = drafts.findIndex(
    (d) => d.objectKey === objectKey && d.creator.id === user.id,
  );

  const draftContent: DraftContent = {
    id: existingIndex >= 0 ? drafts[existingIndex].id : generateDraftId(),
    objectKey,
    creator: {
      id: user.id,
      name: user.name,
      roles: user.roles,
    },
    createdAt: existingIndex >= 0 ? drafts[existingIndex].createdAt : now,
    updatedAt: now,
    title: options?.title || config.title,
    description: options?.description,
    config,
    isAutoSave: options?.isAutoSave ?? false,
    basedOnVersionId: options?.basedOnVersionId,
    version: options?.version,
  };

  if (existingIndex >= 0) {
    drafts[existingIndex] = draftContent;
  } else {
    drafts.push(draftContent);
  }

  saveAllDrafts(drafts);
  return draftContent;
}

/**
 * 加载草稿
 */
export function loadDraft(objectKey: string): DraftContent | null {
  const user = getCurrentUser();
  if (!user) return null;

  const drafts = getAllDrafts();

  // 查找当前用户的草稿
  const draft = drafts.find((d) => d.objectKey === objectKey && d.creator.id === user.id);

  if (!draft) return null;

  // 检查是否过期
  const age = Date.now() - draft.updatedAt;
  if (age > DRAFT_EXPIRY_DAYS * MS_PER_DAY) {
    // 删除过期草稿
    deleteDraft(draft.id);
    return null;
  }

  return draft;
}

/**
 * 获取对象的所有草稿
 */
export function getDrafts(objectKey: string): DraftListItem[] {
  const user = getCurrentUser();
  if (!user) return [];

  const drafts = getAllDrafts();
  const now = Date.now();
  const expiryTime = DRAFT_EXPIRY_DAYS * MS_PER_DAY;

  return drafts
    .filter((d) => d.objectKey === objectKey)
    .map((draft) => ({
      ...draft,
      recoverable: draft.creator.id === user.id,
      isExpired: now - draft.updatedAt > expiryTime,
    }))
    .sort((a, b) => b.updatedAt - a.updatedAt);
}

/**
 * 删除草稿
 */
export function deleteDraft(draftId: string): boolean {
  const drafts = getAllDrafts();
  const filtered = drafts.filter((d) => d.id !== draftId);

  if (filtered.length === drafts.length) {
    return false; // 没有找到要删除的草稿
  }

  saveAllDrafts(filtered);
  return true;
}

/**
 * 检测草稿冲突
 */
export function detectDraftConflict(objectKey: string): DraftConflict {
  const user = getCurrentUser();
  if (!user) {
    return { hasConflict: false, conflicts: [] };
  }

  const drafts = getAllDrafts();
  const objectDrafts = drafts.filter((d) => d.objectKey === objectKey);

  // 排除当前用户自己的草稿
  const otherDrafts = objectDrafts.filter((d) => d.creator.id !== user.id);
  const myDraft = objectDrafts.find((d) => d.creator.id === user.id);

  return {
    hasConflict: otherDrafts.length > 0,
    conflicts: otherDrafts,
    currentUserDraft: myDraft,
  };
}

/**
 * 恢复草稿
 */
export function restoreDraft(draftId: string): WorkspaceConfig | null {
  const drafts = getAllDrafts();
  const draft = drafts.find((d) => d.id === draftId);

  if (!draft) return null;

  // 检查权限
  const user = getCurrentUser();
  if (!user || draft.creator.id !== user.id) {
    console.warn('无权恢复此草稿');
    return null;
  }

  // 检查是否过期
  const age = Date.now() - draft.updatedAt;
  if (age > DRAFT_EXPIRY_DAYS * MS_PER_DAY) {
    console.warn('草稿已过期');
    deleteDraft(draftId);
    return null;
  }

  return draft.config;
}

/**
 * 获取所有草稿列表
 */
export function getAllDraftList(): DraftListItem[] {
  const user = getCurrentUser();
  if (!user) return [];

  const drafts = getAllDrafts();
  const now = Date.now();
  const expiryTime = DRAFT_EXPIRY_DAYS * MS_PER_DAY;

  return drafts
    .map((draft) => ({
      ...draft,
      recoverable: draft.creator.id === user.id,
      isExpired: now - draft.updatedAt > expiryTime,
    }))
    .sort((a, b) => b.updatedAt - a.updatedAt);
}

/**
 * 清空所有草稿
 */
export function clearAllDrafts(): number {
  const drafts = getAllDrafts();
  const count = drafts.length;

  localStorage.removeItem(STORAGE_KEY);
  return count;
}

/**
 * 获取草稿统计
 */
export function getDraftStats(): {
  total: number;
  myDrafts: number;
  expired: number;
  byObject: Record<string, number>;
} {
  const user = getCurrentUser();
  const drafts = getAllDrafts();
  const now = Date.now();
  const expiryTime = DRAFT_EXPIRY_DAYS * MS_PER_DAY;

  const byObject: Record<string, number> = {};

  drafts.forEach((draft) => {
    byObject[draft.objectKey] = (byObject[draft.objectKey] || 0) + 1;
  });

  return {
    total: drafts.length,
    myDrafts: user ? drafts.filter((d) => d.creator.id === user.id).length : 0,
    expired: drafts.filter((d) => now - d.updatedAt > expiryTime).length,
    byObject,
  };
}

/**
 * 自动保存草稿（带防抖）
 */
export class AutoSaveDraft {
  private timer: NodeJS.Timeout | null = null;
  private delay: number;
  private objectKey: string;
  private getConfig: () => WorkspaceConfig;
  private onSave?: (draft: DraftContent) => void;

  constructor(
    objectKey: string,
    getConfig: () => WorkspaceConfig,
    options?: {
      delay?: number;
      onSave?: (draft: DraftContent) => void;
    },
  ) {
    this.objectKey = objectKey;
    this.getConfig = getConfig;
    this.delay = options?.delay || 30000; // 默认30秒
    this.onSave = options?.onSave;
  }

  /** 触发自动保存（防抖） */
  trigger(): void {
    if (this.timer) {
      clearTimeout(this.timer);
    }

    this.timer = setTimeout(() => {
      this.save();
    }, this.delay);
  }

  /** 立即保存 */
  save(): DraftContent | null {
    try {
      const config = this.getConfig();
      const draft = saveDraft(this.objectKey, config, { isAutoSave: true });
      if (this.onSave) {
        this.onSave(draft);
      }
      return draft;
    } catch (error) {
      console.error('自动保存草稿失败:', error);
      return null;
    }
  }

  /** 取消待执行的保存 */
  cancel(): void {
    if (this.timer) {
      clearTimeout(this.timer);
      this.timer = null;
    }
  }

  /** 销毁 */
  dispose(): void {
    this.cancel();
  }
}

/**
 * 格式化草稿时间
 */
export function formatDraftTime(timestamp: number): string {
  const now = Date.now();
  const diff = now - timestamp;

  const minutes = Math.floor(diff / 60000);
  const hours = Math.floor(diff / 3600000);
  const days = Math.floor(diff / 86400000);

  if (minutes < 1) return '刚刚';
  if (minutes < 60) return `${minutes}分钟前`;
  if (hours < 24) return `${hours}小时前`;
  if (days < 7) return `${days}天前`;

  return new Date(timestamp).toLocaleDateString('zh-CN');
}

/**
 * 获取草稿剩余时间
 */
export function getDraftRemainingTime(updatedAt: number): number {
  const now = Date.now();
  const expiryTime = DRAFT_EXPIRY_DAYS * MS_PER_DAY;
  const age = now - updatedAt;
  return Math.max(0, expiryTime - age);
}
