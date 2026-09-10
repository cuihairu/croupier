import { useCallback, useMemo, useState } from 'react';
import { message } from 'antd';
import type { FormInstance } from 'antd';
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
import type { JSONValue } from '@/types/dashboard';
import {
  FALLBACK_APPLY_PERMISSION_TEMPLATES,
  pickAuditMetaValue,
  type PermissionApplyItem,
  type ProfileData,
} from './shared';

/** Profile 页数据层：profile + 五类扩展数据（games/permissions/audit/login/messages）
 * 的一次性并行拉取与派生 memo。UI 编排状态（Tab/编辑态/弹窗）留在主页。 */
export function useProfileData(form: FormInstance) {
  const intl = useIntl();
  const formatMessage = useCallback((id: string) => intl.formatMessage({ id }), [intl]);

  const [profile, setProfile] = useState<ProfileData | null>(null);
  const [games, setGames] = useState<ProfileGame[]>([]);
  const [permissions, setPermissions] = useState<ProfilePermission[]>([]);
  const [permissionIds, setPermissionIds] = useState<string[]>([]);
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
        const [gamesRes, permsRes, auditsRes, loginRes, notificationsRes, permissionCatalogRes] =
          await Promise.allSettled([
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
          ]);

        setGames(gamesRes.status === 'fulfilled' ? gamesRes.value?.games || [] : []);
        if (permsRes.status === 'fulfilled') {
          const payload = permsRes.value || {};
          const ids = payload.permissionIDs || [];
          setPermissions(payload.permissions || []);
          setPermissionIds(Array.isArray(ids) ? ids : []);
        } else {
          setPermissions([]);
          setPermissionIds([]);
        }
        setActivities(auditsRes.status === 'fulfilled' ? auditsRes.value?.events || [] : []);
        setLoginRecords(loginRes.status === 'fulfilled' ? loginRes.value?.events || [] : []);
        setNotifications(
          notificationsRes.status === 'fulfilled' ? notificationsRes.value?.items || [] : [],
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
    [formatMessage],
  );

  const loadProfile = useCallback(async () => {
    try {
      const p = await getMyProfile();
      setProfile(p);
      form.setFieldsValue({
        displayName: p.displayName || p.nickname,
        email: p.email,
        phone: p.phone,
      });
      loadExtras(p.username);
    } catch {
      message.error(formatMessage('profile.load.error'));
    }
  }, [form, formatMessage, loadExtras]);

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
    loginSessionRows,
    latestLoginIP,
    loadProfile,
    loadExtras,
    openMessage,
    markAllRead,
    setDetailMessage,
    setNotifications,
  };
}
