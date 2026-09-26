import { request } from '@umijs/max';

/**
 * 系统公告（广播）管理 API。
 *
 * Source: croupier/internal/api/announcement/dto.go
 *   - AdminAnnouncementItem / AdminListResponse
 *   - CreateRequest / UpdateRequest
 *   路由：GET/POST /api/v1/admin/announcements，PUT/DELETE /api/v1/admin/announcements/:id
 *
 * 为什么会从「个人中心」搬到这里：公告是对**全体或某角色**的广播，属于管理
 * 动作；此前个人中心的「消息通知」页顶部挂着「发送消息」按钮，普通用户与管理员
 * 看到的是同一个页面，语义混乱（docs/BUGS.md BUG-021）。后端本来就有这套
 * admin-only 接口，但前端**一个引用都没有**——即公告功能此前完全没有入口。
 */

export type AdminAnnouncement = {
  id: number;
  title: string;
  contentMd: string;
  /** all | role */
  audience: string;
  /** audience=role 时的目标角色名 */
  role?: string;
  /** 是否在用户登录后弹窗提示 */
  popup: boolean;
  active: boolean;
  startAt?: string;
  endAt?: string;
  createdBy?: string;
  createdAt: string;
  updatedAt: string;
};

export type AnnouncementDraft = {
  title: string;
  contentMd: string;
  audience: 'all' | 'role';
  role?: string;
  popup: boolean;
  /** 不传则由后端取默认（true） */
  active?: boolean;
  startAt?: string;
  endAt?: string;
};

export async function listAnnouncements(): Promise<{
  items: AdminAnnouncement[];
  total: number;
}> {
  const resp = await request<{ items?: AdminAnnouncement[]; total?: number }>(
    '/api/v1/admin/announcements',
    { method: 'GET' },
  );
  return {
    items: Array.isArray(resp?.items) ? resp.items : [],
    total: typeof resp?.total === 'number' ? resp.total : 0,
  };
}

export async function createAnnouncement(
  draft: AnnouncementDraft,
): Promise<AdminAnnouncement> {
  return request<AdminAnnouncement>('/api/v1/admin/announcements', {
    method: 'POST',
    data: draft,
  });
}

export async function updateAnnouncement(
  id: number,
  patch: Partial<AnnouncementDraft>,
): Promise<AdminAnnouncement> {
  return request<AdminAnnouncement>(`/api/v1/admin/announcements/${id}`, {
    method: 'PUT',
    data: patch,
  });
}

export async function deleteAnnouncement(id: number): Promise<void> {
  await request<void>(`/api/v1/admin/announcements/${id}`, { method: 'DELETE' });
}
