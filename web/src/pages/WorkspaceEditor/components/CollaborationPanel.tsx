/**
 * 协作编辑面板组件
 *
 * 显示当前编辑者列表、锁定状态和冲突处理。
 *
 * @module pages/WorkspaceEditor/components/CollaborationPanel
 */

import React, { useEffect, useState, useCallback, useMemo } from 'react';
import {
  Badge,
  Button,
  Card,
  List,
  Avatar,
  Tag,
  Space,
  Tooltip,
  Popconfirm,
  Typography,
  Progress,
  Alert,
  Divider,
  Dropdown,
  Menu,
} from 'antd';
import {
  TeamOutlined,
  LockOutlined,
  UnlockOutlined,
  WarningOutlined,
  SyncOutlined,
  UserOutlined,
  ClockCircleOutlined,
  MoreOutlined,
  LogoutOutlined,
} from '@ant-design/icons';
import type { MenuProps } from 'antd';
import {
  getCollaborationManager,
  type EditSession,
  type LockStatus,
  formatRemainingTime,
} from '../utils/collaborationManager';
import { getCurrentUser } from '../utils/permissionManager';
import './CollaborationPanel.less';

const { Text } = Typography;

export interface CollaborationPanelProps {
  /** 编辑对象标识 */
  objectKey: string;
  /** 当前用户是否有管理权限 */
  isAdmin?: boolean;
  /** 是否显示在工具栏（紧凑模式） */
  compact?: boolean;
  /** 样式类名 */
  className?: string;
  /** 自定义样式 */
  style?: React.CSSProperties;
}

/**
 * 编辑者头像组件
 */
function EditorAvatar({ session, isCurrent }: { session: EditSession; isCurrent: boolean }) {
  const user = session.user;

  return (
    <Badge dot={isCurrent} offset={[-5, 5]}>
      <Avatar
        size="small"
        src={user.avatar}
        icon={!user.avatar ? <UserOutlined /> : undefined}
        style={{
          backgroundColor: isCurrent ? '#1890ff' : undefined,
        }}
      >
        {user.name?.charAt(0)?.toUpperCase()}
      </Avatar>
    </Badge>
  );
}

/**
 * 编辑状态指示器（紧凑模式）
 */
export function CollaborationIndicator({
  objectKey,
  onClick,
}: {
  objectKey: string;
  onClick?: () => void;
}) {
  const [stats, setStats] = useState({
    totalEditors: 0,
    lockHolder: undefined as string | undefined,
    isConflict: false,
  });

  const manager = useMemo(() => getCollaborationManager(), []);

  useEffect(() => {
    const updateStats = () => {
      setStats(manager.getCollaborationStats(objectKey));
    };

    updateStats();

    const unsubscribe = manager.on(objectKey, 'all', updateStats);

    // 定期更新
    const timer = setInterval(updateStats, 5000);

    return () => {
      unsubscribe();
      clearInterval(timer);
    };
  }, [manager, objectKey]);

  const isLocked = !!stats.lockHolder;
  const isConflict = stats.isConflict && stats.totalEditors > 1;

  return (
    <Tooltip
      title={
        <div>
          <div>当前编辑者: {stats.totalEditors} 人</div>
          {isLocked && <div>编辑锁: {stats.lockHolder}</div>}
          {isConflict && <div style={{ color: '#ff4d4f' }}>检测到多人编辑</div>}
        </div>
      }
    >
      <Badge count={stats.totalEditors} offset={[-5, 5]}>
        <Button
          type="text"
          icon={<TeamOutlined />}
          onClick={onClick}
          style={{
            color: isConflict ? '#ff4d4f' : isLocked ? '#faad14' : undefined,
          }}
        >
          {stats.totalEditors}
        </Button>
      </Badge>
    </Tooltip>
  );
}

/**
 * 协作面板主组件
 */
export default function CollaborationPanel({
  objectKey,
  isAdmin = false,
  compact = false,
  className,
  style,
}: CollaborationPanelProps) {
  const manager = useMemo(() => getCollaborationManager(), []);
  const currentUser = useMemo(() => getCurrentUser(), []);

  const [sessions, setSessions] = useState<EditSession[]>([]);
  const [lockStatus, setLockStatus] = useState<LockStatus>({
    isLocked: false,
    isHeldByCurrentSession: false,
  });
  const [currentSession, setCurrentSession] = useState<EditSession | null>(null);
  const [remainingTime, setRemainingTime] = useState<number>(0);

  // 更新会话列表和锁状态
  const updateState = useCallback(() => {
    setSessions(manager.getActiveEditors(objectKey));
    setLockStatus(manager.getLockStatus(objectKey));
    setCurrentSession(manager.getCurrentSession());
  }, [manager, objectKey]);

  // 加入编辑会话
  useEffect(() => {
    let session: EditSession;

    try {
      session = manager.join(objectKey);
    } catch (error) {
      console.error('Failed to join edit session:', error);
      return;
    }

    updateState();

    const unsubscribe = manager.on(objectKey, 'all', () => {
      updateState();
    });

    return () => {
      unsubscribe();
    };
  }, [manager, objectKey, updateState]);

  // 更新剩余时间
  useEffect(() => {
    if (!lockStatus.isLocked || !lockStatus.remainingTime) {
      setRemainingTime(0);
      return;
    }

    setRemainingTime(lockStatus.remainingTime);

    const timer = setInterval(() => {
      setRemainingTime((prev) => Math.max(0, prev - 1000));
    }, 1000);

    return () => clearInterval(timer);
  }, [lockStatus]);

  // 尝试获取锁
  const handleAcquireLock = useCallback(() => {
    const result = manager.acquireLock();
    if (result.success) {
      updateState();
    } else {
      // eslint-disable-next-line no-alert
      alert(result.reason || '获取锁失败');
    }
  }, [manager, updateState]);

  // 释放锁
  const handleReleaseLock = useCallback(() => {
    const result = manager.releaseLock();
    if (result.success) {
      updateState();
    }
  }, [manager, updateState]);

  // 强制解锁
  const handleForceUnlock = useCallback(() => {
    const result = manager.forceUnlock(objectKey);
    if (result.success) {
      updateState();
    } else {
      // eslint-disable-next-line no-alert
      alert(result.reason || '强制解锁失败');
    }
  }, [manager, objectKey, updateState]);

  // 渲染编辑者列表项
  const renderEditorItem = (session: EditSession) => {
    const isCurrent = session.sessionId === currentSession?.sessionId;
    const isLockHolder = lockStatus.isLocked && lockStatus.lockHolder?.id === session.user.id;

    return (
      <List.Item
        key={session.sessionId}
        className={isCurrent ? 'current-editor' : ''}
        actions={
          isCurrent && lockStatus.isHeldByCurrentSession
            ? [
                <Tooltip title="释放编辑锁">
                  <Button
                    type="text"
                    size="small"
                    icon={<UnlockOutlined />}
                    onClick={handleReleaseLock}
                  />
                </Tooltip>,
              ]
            : undefined
        }
      >
        <List.Item.Meta
          avatar={<EditorAvatar session={session} isCurrent={isCurrent} />}
          title={
            <Space>
              <Text strong={isCurrent}>{session.user.name}</Text>
              {isCurrent && <Tag color="blue">我</Tag>}
              {isLockHolder && (
                <Tag color="orange" icon={<LockOutlined />}>
                  编辑中
                </Tag>
              )}
              {session.user.roles.includes('admin') && <Tag color="red">管理员</Tag>}
            </Space>
          }
          description={
            <Space size="large">
              <Text type="secondary">
                <ClockCircleOutlined /> 加入时间: {new Date(session.startTime).toLocaleTimeString()}
              </Text>
            </Space>
          }
        />
      </List.Item>
    );
  };

  // 紧凑模式：只显示状态指示器
  if (compact) {
    return (
      <CollaborationIndicator
        objectKey={objectKey}
        onClick={() => {
          // 可以触发打开完整面板
        }}
      />
    );
  }

  // 计算锁进度百分比
  const lockProgress = lockStatus.remainingTime
    ? Math.max(0, Math.min(100, (lockStatus.remainingTime / LOCK_DURATION) * 100))
    : 0;

  // 是否有冲突
  const hasConflict = sessions.length > 1;

  // 锁持有者操作菜单
  const lockHolderMenuItems: MenuProps['items'] = [
    {
      key: 'release',
      icon: <UnlockOutlined />,
      label: '释放编辑锁',
      onClick: handleReleaseLock,
      disabled: !lockStatus.isHeldByCurrentSession,
    },
  ];

  // 管理员菜单
  const adminMenuItems: MenuProps['items'] = [
    {
      key: 'force-unlock',
      icon: <LockOutlined />,
      label: '强制解锁',
      danger: true,
      onClick: handleForceUnlock,
      disabled: !lockStatus.isLocked || lockStatus.isHeldByCurrentSession,
    },
  ];

  return (
    <Card
      className={`collaboration-panel ${className || ''}`}
      style={style}
      size="small"
      title={
        <Space>
          <TeamOutlined />
          <span>协作状态</span>
          <Badge count={sessions.length} showZero />
        </Space>
      }
      extra={
        <Space>
          {lockStatus.isHeldByCurrentSession && (
            <Dropdown menu={{ items: lockHolderMenuItems }} trigger={['click']}>
              <Button type="link" size="small">
                <MoreOutlined />
              </Button>
            </Dropdown>
          )}
          {isAdmin && (
            <Dropdown menu={{ items: adminMenuItems }} trigger={['click']}>
              <Button type="link" size="small">
                管理
              </Button>
            </Dropdown>
          )}
        </Space>
      }
    >
      {/* 冲突警告 */}
      {hasConflict && (
        <Alert
          message="多人同时编辑"
          description={`当前有 ${sessions.length} 人正在编辑此配置，请注意及时同步避免冲突。`}
          type="warning"
          icon={<WarningOutlined />}
          showIcon
          closable
          style={{ marginBottom: 12 }}
        />
      )}

      {/* 锁状态 */}
      <Card size="small" style={{ marginBottom: 12 }}>
        <Space direction="vertical" style={{ width: '100%' }}>
          <Space style={{ justifyContent: 'space-between', width: '100%' }}>
            <Text strong>
              {lockStatus.isLocked ? <LockOutlined /> : <UnlockOutlined />} 编辑锁状态
            </Text>
            {lockStatus.isLocked ? (
              <Tag color={lockStatus.isHeldByCurrentSession ? 'green' : 'orange'}>
                {lockStatus.isHeldByCurrentSession
                  ? '我正在编辑'
                  : `${lockStatus.lockHolder?.name} 编辑中`}
              </Tag>
            ) : (
              <Tag color="default">未锁定</Tag>
            )}
          </Space>

          {lockStatus.isLocked && (
            <>
              <Space direction="vertical" style={{ width: '100%' }}>
                <Text type="secondary" style={{ fontSize: 12 }}>
                  剩余时间: {formatRemainingTime(remainingTime)}
                </Text>
                <Progress
                  percent={lockProgress}
                  status={lockProgress < 20 ? 'exception' : 'active'}
                  showInfo={false}
                  size="small"
                  strokeColor={lockStatus.isHeldByCurrentSession ? '#52c41a' : '#faad14'}
                />
              </Space>

              {!lockStatus.isHeldByCurrentSession && isAdmin && (
                <Popconfirm
                  title="确认强制解锁？"
                  description="强制解锁将中断对方的编辑权限，请谨慎操作。"
                  onConfirm={handleForceUnlock}
                  okText="确认"
                  cancelText="取消"
                >
                  <Button type="link" size="small" danger>
                    强制解锁
                  </Button>
                </Popconfirm>
              )}

              {!lockStatus.isHeldByCurrentSession && !isAdmin && (
                <Alert
                  message="当前配置正在被编辑"
                  description={`请联系 ${lockStatus.lockHolder?.name} 释放编辑锁，或等待锁自动过期。`}
                  type="info"
                  showIcon
                  style={{ marginTop: 8 }}
                />
              )}
            </>
          )}

          {!lockStatus.isLocked && !lockStatus.isHeldByCurrentSession && (
            <Button
              type="primary"
              size="small"
              icon={<LockOutlined />}
              onClick={handleAcquireLock}
              block
            >
              获取编辑锁
            </Button>
          )}
        </Space>
      </Card>

      <Divider orientation="left" style={{ margin: '12px 0' }}>
        当前编辑者 ({sessions.length})
      </Divider>

      {/* 编辑者列表 */}
      <List
        size="small"
        dataSource={sessions}
        renderItem={renderEditorItem}
        locale={{ emptyText: '暂无其他编辑者' }}
      />

      {/* 提示信息 */}
      <div style={{ marginTop: 12, textAlign: 'center' }}>
        <Text type="secondary" style={{ fontSize: 12 }}>
          <SyncOutlined spin /> 自动同步中 · 最后更新: {new Date().toLocaleTimeString()}
        </Text>
      </div>
    </Card>
  );
}

/** 锁定时长常量 */
const LOCK_DURATION = 1800000; // 30分钟
