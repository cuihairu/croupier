import { useCallback, useMemo, useState } from 'react';
import { App } from 'antd';
import { useIntl } from '@umijs/max';
import {
  getMyGames,
  getMyPermissions,
  getMyProfile,
  ProfileGame,
  ProfilePermission,
} from '@/services/api/me';
import { listAudit, AuditEvent } from '@/services/api/audit';
import { listMessages, markMessagesRead, MessageItem } from '@/services/api/messages';
import { listPermissions, type PermissionRecord } from '@/services/api/permissions';
import { fetchMyNotificationChannels } from '@/services/api/me';
import type { NotificationChannelState } from './shared';
import type { PermissionCatalogEntry, RoleGrant } from './permissionTree';
import type { JSONValue } from '@/types/dashboard';
import {
  FALLBACK_APPLY_PERMISSION_TEMPLATES,
  pickAuditMetaValue,
  type PermissionApplyItem,
  type ProfileData,
} from './shared';

/** Profile 页数据层：profile + 五类扩展数据（games/permissions/audit/login/messages）
 * 的一次性并行拉取与派生 memo。UI 编排状态（Tab/编辑态/弹窗）留在主页。
 * 表单回填不在本层——见下方 useProfileData 的说明。 */
export function useProfileData() {
  const { message } = App.useApp();
  const intl = useIntl();
  const formatMessage = useCallback((id: string) => intl.formatMessage({ id }), [intl]);

  const [profile, setProfile] = useState<ProfileData | null>(null);
  const [games, setGames] = useState<ProfileGame[]>([]);
  const [permissions, setPermissions] = useState<ProfilePermission[]>([]);
  const [permissionIds, setPermissionIds] = useState<string[]>([]);
  // 逐角色授权明细：权限树需要知道「哪个角色授予了哪些操作」
  const [roleGrants, setRoleGrants] = useState<RoleGrant[]>([]);
  // 账号级通配：树整体按全绿呈现并给出说明，避免把「未显式授予」误读为「不可用」
  const [fullAccess, setFullAccess] = useState(false);
  // 通知通道真实状态：后端判定「是否真的接入」，前端不再自行推断
  const [notificationChannels, setNotificationChannels] = useState<NotificationChannelState[]>([]);
  const [permissionCatalog, setPermissionCatalog] = useState<PermissionRecord[]>([]);
  const [permissionCatalogAvailable, setPermissionCatalogAvailable] = useState(true);
  const [activities, setActivities] = useState<AuditEvent[]>([]);
  const [loginRecords, setLoginRecords] = useState<AuditEvent[]>([]);
  const [notifications, setNotifications] = useState<MessageItem[]>([]);
  const [detailMessage, setDetailMessage] = useState<MessageItem | null>(null);
  const [extrasLoading, setExtrasLoading] = useState(false);

  const loadExtras = useCallback(
    async (username?: string) => {
      setExtrasLoading(true);
      try {
        const [
          gamesRes,
          permsRes,
          auditsRes,
          loginRes,
          notificationsRes,
          permissionCatalogRes,
          channelsRes,
        ] = await Promise.allSettled([
            getMyGames(),
            getMyPermissions({}),
            listAudit({ actor: username, size: 8 }),
            username
              ? listAudit({
                  actor: username,
                  kinds: 'login,auth_login,login_fail,login_rate_limited',
                  size: 20,
                })
              : Promise.resolve({ events: [] }),
            listMessages({ status: 'all', pageSize: 8 }),
            listPermissions({ page: 1, pageSize: 500 }),
            // 通知通道状态放最后：失败时留空数组，前端显示「正在读取」而不是假状态
            fetchMyNotificationChannels().then((r) => r?.channels || []),
          ]);

        setGames(gamesRes.status === 'fulfilled' ? gamesRes.value?.games || [] : []);
        if (permsRes.status === 'fulfilled') {
          const payload = permsRes.value || {};
          const ids = payload.permissionIDs || [];
          setPermissions(payload.permissions || []);
          setPermissionIds(Array.isArray(ids) ? ids : []);
          setRoleGrants(Array.isArray(payload.rolePermissions) ? payload.rolePermissions : []);
          setFullAccess(payload.fullAccess === true);
        } else {
          setPermissions([]);
          setPermissionIds([]);
          setRoleGrants([]);
          setFullAccess(false);
        }
        setActivities(auditsRes.status === 'fulfilled' ? auditsRes.value?.events || [] : []);
        setLoginRecords(loginRes.status === 'fulfilled' ? loginRes.value?.events || [] : []);
        setNotifications(
          notificationsRes.status === 'fulfilled' ? notificationsRes.value?.items || [] : [],
        );
        setNotificationChannels(
          channelsRes.status === 'fulfilled' ? channelsRes.value || [] : [],
        );
        if (permissionCatalogRes.status === 'fulfilled') {
          setPermissionCatalog(permissionCatalogRes.value?.items || []);
          setPermissionCatalogAvailable(true);
        } else {
          setPermissionCatalog([]);
          setPermissionCatalogAvailable(false);
        }
      } catch {
        message.error(formatMessage('profile.extras.error'));
      } finally {
        setExtrasLoading(false);
      }
    },
    [message, formatMessage],
  );

  const loadProfile = useCallback(async () => {
    try {
      const p = await getMyProfile();
      setProfile(p);
      // 注意：这里刻意**不**回填表单。表单实例由主页 useForm 创建，而承载
      // <Form> 的 InfoTab 位于 Tabs 的「资料」面板内——antd Tabs 惰性渲染，
      // 未激活的面板根本不挂载。若在此处 setFieldsValue，当用户直接停在
      // ?tab=security 等其它标签时，实例处于「未连接」状态，antd 会报
      // "Instance created by `useForm` is not connected to any Form element"
      // （docs/BUGS.md BUG-008）。回填改由 InfoTab 在自身挂载时完成——
      // 那正是表单存在、且回填才有意义的时候。
      loadExtras(p.username);
    } catch {
      message.error(formatMessage('profile.load.error'));
    }
  }, [message, formatMessage, loadExtras]);

  // 消息详情：打开未读消息即标记已读并刷新列表状态
  const openMessage = useCallback((item: MessageItem) => {
    setDetailMessage(item);
    if (item.status !== 'read') {
      markMessagesRead([item.id])
        .then(() => {
          setNotifications((prev) =>
            prev.map((m) => (m.id === item.id ? { ...m, status: 'read' } : m)),
          );
          setDetailMessage((prev) =>
            prev && prev.id === item.id ? { ...prev, status: 'read' } : prev,
          );
        })
        .catch(() => undefined);
    }
  }, []);

  // 单条标为已读：不必打开详情（点「标为已读」不该顺带弹 Modal）
  const markMessageRead = useCallback((item: MessageItem) => {
    if (item.status === 'read') return;
    markMessagesRead([item.id])
      .then(() => {
        setNotifications((prev) =>
          prev.map((m) => (m.id === item.id ? { ...m, status: 'read' } : m)),
        );
        setDetailMessage((prev) =>
          prev && prev.id === item.id ? { ...prev, status: 'read' } : prev,
        );
      })
      .catch(() => undefined);
  }, []);

  const markAllRead = useCallback(() => {
    const unread = notifications.filter((m) => m.status !== 'read');
    if (unread.length === 0) return;
    markMessagesRead(unread.map((m) => m.id))
      .then(() => {
        setNotifications((prev) => prev.map((m) => ({ ...m, status: 'read' })));
      })
      .catch(() => undefined);
  }, [notifications]);

  const fallbackApplyPermissions = useMemo<PermissionApplyItem[]>(
    () =>
      FALLBACK_APPLY_PERMISSION_TEMPLATES.map((item) => ({
        id: item.id,
        name: formatMessage(item.nameId),
        description: formatMessage(item.descriptionId),
        resource: item.resource,
        action: item.action,
        category: item.category,
      })),
    [formatMessage],
  );

  const permissionGroups = useMemo(() => {
    const map = new Map<string, { resource: string; actions: Set<string>; scope?: string }>();
    permissions.forEach((perm) => {
      const gameId = perm.gameId;
      const env = perm.env;
      const scope = gameId || env ? `${gameId || ''} ${env || ''}`.trim() : '';
      const key = `${perm.resource}-${scope}`;
      if (!map.has(key)) {
        map.set(key, {
          resource: perm.resource,
          actions: new Set(Array.isArray(perm.actions) ? perm.actions : []),
          scope,
        });
      } else {
        (Array.isArray(perm.actions) ? perm.actions : []).forEach((action) =>
          map.get(key)!.actions.add(action),
        );
      }
    });
    return Array.from(map.values()).map((item) => ({
      resource: item.resource,
      actions: Array.from(item.actions),
      scope: item.scope,
    }));
  }, [permissions]);

  const ownedPermissionKeySet = useMemo(() => {
    const keys = new Set<string>();
    permissions.forEach((perm) => {
      (perm.actions || []).forEach((action) => {
        keys.add(`${perm.resource}:${action}`.toLowerCase());
      });
    });
    permissionIds.forEach((id) => keys.add(String(id).toLowerCase()));
    return keys;
  }, [permissions, permissionIds]);

  /**
   * 权限树用的目录视图。
   *
   * 树的资源/操作候选集必须来自**全量权限目录**而不是已授权 id：只用已授权
   * 的话，未授权项没有数据来源，页面看上去就像「全部都有权限」
   * （docs/BUGS.md BUG-020）。
   *
   * 注意 catalog 的 `resource`/`action` 列存的是 module 粒度，渲染时按 **id
   * 前缀**分组（与后端 permissions.go 同一口径），因此这里只透传 id + 展示名。
   */
  const permissionCatalogForTree = useMemo<PermissionCatalogEntry[]>(
    () =>
      permissionCatalog.map((item) => ({
        id: item.id,
        name: item.name || item.id,
        description: item.description,
        category: item.category,
      })),
    [permissionCatalog],
  );

  const applyPermissionCandidates = useMemo(() => {
    const source: PermissionApplyItem[] =
      permissionCatalog.length > 0
        ? permissionCatalog.map((item) => ({
            id: item.id,
            name: item.name,
            description: item.description,
            resource: item.resource,
            action: item.action,
            category: item.category,
          }))
        : fallbackApplyPermissions;

    return source
      .filter((item) => {
        const key = `${item.resource}:${item.action}`.toLowerCase();
        return (
          !ownedPermissionKeySet.has(key) &&
          !ownedPermissionKeySet.has(String(item.id).toLowerCase())
        );
      })
      .slice(0, 20);
  }, [fallbackApplyPermissions, permissionCatalog, ownedPermissionKeySet]);

  const loginSessionRows = useMemo(() => {
    return (loginRecords || []).map((item, idx) => {
      const meta = (item.meta || {}) as Record<string, JSONValue>;
      const ip = pickAuditMetaValue(meta, ['ip', 'client_ip', 'remote_ip', 'x_forwarded_for']);
      const region = pickAuditMetaValue(meta, ['ipRegion', 'region', 'geo']);
      const userAgent = pickAuditMetaValue(meta, ['userAgent', 'ua', 'agent']);
      const success =
        !String(item.kind || '').includes('fail') &&
        !String(item.kind || '').includes('rate_limited');
      return {
        key: `${item.hash || item.time || idx}`,
        time: item.time,
        kind: item.kind,
        ip,
        region,
        userAgent,
        success,
        target: item.target,
      };
    });
  }, [loginRecords]);

  const latestLoginIP = useMemo(() => {
    const first = loginSessionRows.find((row) => row.ip);
    return first?.ip || formatMessage('profile.info.notSet');
  }, [formatMessage, loginSessionRows]);

  return {
    profile,
    notificationChannels,
    setProfile,
    games,
    permissions,
    permissionCatalogAvailable,
    activities,
    notifications,
    detailMessage,
    extrasLoading,
    permissionGroups,
    applyPermissionCandidates,
    roleGrants,
    fullAccess,
    permissionCatalogForTree,
    loginSessionRows,
    latestLoginIP,
    loadProfile,
    loadExtras,
    openMessage,
    markAllRead,
    markMessageRead,
    setDetailMessage,
    setNotifications,
  };
}
