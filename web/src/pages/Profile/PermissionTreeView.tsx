import { useMemo, useState } from 'react';
import { Badge, Button, Card, Empty, Space, Tag, Tooltip, Tree, Typography } from 'antd';
import { CheckOutlined, CloseOutlined } from '@ant-design/icons';
import { useIntl } from '@umijs/max';
import {
  buildPermissionTree,
  type PermissionCatalogEntry,
  type RoleGrant,
} from './permissionTree';

const { Text } = Typography;

/** 绿=有权限、灰=无权限，附勾/叉，避免只靠颜色区分（色觉障碍用户）。 */
export const GRANTED_COLOR = '#52c41a';
export const DENIED_COLOR = '#bfbfbf';

/**
 * 权限树：角色 → 资源 → 操作。
 *
 * 修复前这里是一个单层 `SimpleList`，把「已授权」和「未授权」混在一起、
 * 也没有任何两态区分（BUG-020）。现在：
 *   - 三层可展开收起，角色层默认展开、资源层默认展开；
 *   - 每个操作叶子带绿/灰配色 + 勾/叉图标，灰项仍可展开看到「你没有它」；
 *   - 资源与操作的候选集来自**权限目录**（而不是已授权 id），否则未授权项
 *     根本没有数据来源，页面看上去就像「全部都有权限」。
 */
export default function PermissionTreeView({
  roles,
  catalog,
  fullAccess,
}: {
  /** 逐角色的授权明细（后端 /profile/permissions 的 rolePermissions） */
  roles: RoleGrant[];
  /** 全量权限目录（GET /api/v1/permissions） */
  catalog: PermissionCatalogEntry[];
  /** 账号持通配权限：树整体按「全绿」呈现并给出说明 */
  fullAccess: boolean;
}) {
  const intl = useIntl();
  const formatMessage = (id: string, fallback?: string) =>
    intl.formatMessage({ id, defaultMessage: fallback });

  const tree = useMemo(
    () =>
      buildPermissionTree(roles || [], catalog || [], (id) =>
        formatMessage('profile.permissions.tree.rawId', id),
      ),
    // formatMessage 每次渲染是新函数；用 catalog/roles 的内容做依赖即可
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [roles, catalog],
  );

  const treeData = useMemo(
    () =>
      tree.roles.map((role) => ({
        key: role.key,
        title: <RoleTitle role={role} />,
        children: role.resources.map((res) => ({
          key: res.key,
          title: <ResourceTitle resource={res} />,
          children: res.actions.map((act) => ({
            key: `${role.key}/${res.key}/${act.key}`,
            title: <ActionTitle action={act} />,
          })),
        })),
      })),
    [tree],
  );

  // 可展开的节点：角色层 + 资源层（操作叶子不展开）。
  // useState 必须放在 treeData 之后：惰性初始化要读 branchKeys，hook 顺序在
  // 各次渲染间保持一致，因此合法。
  const branchKeys = useMemo(
    () => treeData.flatMap((r) => [r.key, ...r.children.map((c) => c.key)]),
    [treeData],
  );
  const [expandedKeys, setExpandedKeys] = useState<React.Key[]>(() => branchKeys);
  // 权限或角色变化后，新出现的分支要自动展开
  const [seenBranchKeys, setSeenBranchKeys] = useState<string[]>(() => branchKeys.map(String));
  if (branchKeys.map(String).join('|') !== seenBranchKeys.join('|')) {
    setSeenBranchKeys(branchKeys.map(String));
    setExpandedKeys(branchKeys);
  }

  if (tree.roles.length === 0) {
    return (
      <Card title={formatMessage('profile.permissions.tree.title', '权限树')}>
        <Empty description={formatMessage('profile.permissions.tree.noRole', '当前账号没有任何角色')} />
      </Card>
    );
  }

  return (
    <Card
      title={formatMessage('profile.permissions.tree.title', '权限树')}
      extra={
        <Space wrap>
          {fullAccess && (
            <Tag color="success" data-testid="perm-tree-full-access">
              {formatMessage('profile.permissions.tree.fullAccess', '持有全部权限')}
            </Tag>
          )}
          <Tag data-testid="perm-tree-summary">
            {intl.formatMessage(
              { id: 'profile.permissions.tree.summary', defaultMessage: '已授权 {granted}/{total} 项操作' },
              { granted: tree.totalGranted, total: tree.totalActions },
            )}
          </Tag>
          <Button
            size="small"
            onClick={() => {
              // 全展开 ⇄ 只留角色层 ⇄ 全展开（三态循环）
              const roleOnly = treeData.map((r) => r.key);
              const allOpen = branchKeys.every((k) => expandedKeys.includes(k));
              const roleOnlyNow =
                expandedKeys.length === roleOnly.length &&
                roleOnly.every((k) => expandedKeys.includes(k));
              setExpandedKeys(allOpen ? roleOnly : roleOnlyNow ? branchKeys : roleOnly);
            }}
            data-testid="perm-tree-toggle-all"
          >
            {formatMessage('profile.permissions.tree.toggleAll', '展开/收起全部')}
          </Button>
        </Space>
      }
    >
      <Space orientation="vertical" size="small" style={{ width: '100%' }}>
        <Space wrap data-testid="perm-tree-legend">
          <LegendItem color={GRANTED_COLOR} icon={<CheckOutlined />} label={formatMessage('profile.permissions.tree.granted', '已授权')} />
          <LegendItem color={DENIED_COLOR} icon={<CloseOutlined />} label={formatMessage('profile.permissions.tree.denied', '未授权')} />
        </Space>
        {fullAccess && (
          <Badge
            status="success"
            text={formatMessage(
              'profile.permissions.tree.fullAccess.hint',
              '该账号持有通配权限（* / admin:all），下列所有操作均可用；灰色仅表示「未由具体角色显式授予」。',
            )}
            data-testid="perm-tree-full-access-hint"
          />
        )}
        <Tree
          showLine
          selectable={false}
          treeData={treeData}
          expandedKeys={expandedKeys}
          onExpand={(keys) => setExpandedKeys(keys)}
          data-testid="perm-tree"
        />
      </Space>
    </Card>
  );
}

function LegendItem({
  color,
  icon,
  label,
}: {
  color: string;
  icon: React.ReactNode;
  label: string;
}) {
  return (
    <Space size={4}>
      <span style={{ color }} data-testid={`legend-${label}`}>
        {icon}
      </span>
      <Text type="secondary" style={{ color }}>
        {label}
      </Text>
    </Space>
  );
}

function RoleTitle({ role }: { role: ReturnType<typeof buildPermissionTree>['roles'][number] }) {
  return (
    <Space wrap data-testid={`role-${role.role}`}>
      <Text strong>{role.role}</Text>
      <Tag color={role.empty ? 'default' : 'blue'}>
        {role.grantedCount}/{role.actionCount}
      </Tag>
      {role.empty && (
        <Tag color="warning" data-testid={`role-${role.role}-empty`}>
          无显式权限
        </Tag>
      )}
    </Space>
  );
}

function ResourceTitle({
  resource,
}: {
  resource: ReturnType<typeof buildPermissionTree>['roles'][number]['resources'][number];
}) {
  return (
    <Space wrap data-testid={`resource-${resource.resource}`}>
      <Text>{resource.name}</Text>
      <Text type="secondary" style={{ fontSize: 12 }}>
        {resource.resource}
      </Text>
      <Tag color={resource.grantedCount === resource.actions.length ? 'success' : 'default'}>
        {resource.grantedCount}/{resource.actions.length}
      </Tag>
    </Space>
  );
}

function ActionTitle({
  action,
}: {
  action: ReturnType<typeof buildPermissionTree>['roles'][number]['resources'][number]['actions'][number];
}) {
  const color = action.granted ? GRANTED_COLOR : DENIED_COLOR;
  return (
    <Tooltip
      title={
        action.granted
          ? action.description
          : action.grantedBy === 'wildcard'
            ? '当前角色未显式授予，但账号持有通配权限，实际可用'
            : action.description
      }
    >
      <span data-testid={`action-${action.key}`} data-granted={String(action.granted)}>
        <Space size={4}>
          <span style={{ color }} data-testid={`action-icon-${action.key}`}>
            {action.granted ? <CheckOutlined /> : <CloseOutlined />}
          </span>
          <Text style={{ color }}>{action.name}</Text>
          <Text type="secondary" style={{ fontSize: 12 }}>
            {action.id}
          </Text>
        </Space>
      </span>
    </Tooltip>
  );
}
