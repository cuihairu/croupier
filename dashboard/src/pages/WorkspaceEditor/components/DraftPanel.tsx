/**
 * 草稿管理面板
 *
 * 显示和管理配置草稿列表。
 *
 * @module pages/WorkspaceEditor/components/DraftPanel
 */

import React, { useEffect, useState, useCallback } from 'react';
import {
  Card,
  List,
  Tag,
  Space,
  Button,
  Popconfirm,
  Alert,
  Typography,
  Tooltip,
  Progress,
  Divider,
  Empty,
  Badge,
} from 'antd';
import {
  DeleteOutlined,
  RollbackOutlined,
  ClockCircleOutlined,
  UserOutlined,
  WarningOutlined,
  ClearOutlined,
} from '@ant-design/icons';
import type { DraftListItem } from '../utils/draftManager';
import {
  getDrafts,
  deleteDraft,
  restoreDraft,
  getAllDraftList,
  clearAllDrafts,
  cleanupExpiredDrafts,
  getDraftStats,
  formatDraftTime,
  getDraftRemainingTime,
} from '../utils/draftManager';
import { getCurrentUser } from '../utils/permissionManager';
import './DraftPanel.less';

const { Text } = Typography;

export interface DraftPanelProps {
  /** 对象标识 */
  objectKey: string;
  /** 恢复回调 */
  onRestore?: (config: any) => void;
  /** 是否显示 */
  visible?: boolean;
  /** 是否只显示当前对象的草稿 */
  filterByObject?: boolean;
}

/**
 * 单个草稿项
 */
function DraftItem({
  draft,
  onRestore,
  onDelete,
}: {
  draft: DraftListItem;
  onRestore: (draft: DraftListItem) => void;
  onDelete: (draftId: string) => void;
}) {
  const remainingTime = getDraftRemainingTime(draft.updatedAt);
  const totalDays = 7 * 24 * 60 * 60 * 1000;
  const progressPercent = (remainingTime / totalDays) * 100;

  return (
    <List.Item
      className={`draft-item ${draft.isExpired ? 'expired' : ''}`}
      actions={
        draft.recoverable
          ? [
              <Tooltip key="restore" title="恢复此草稿">
                <Button
                  type="link"
                  size="small"
                  icon={<RollbackOutlined />}
                  onClick={() => onRestore(draft)}
                  disabled={draft.isExpired}
                >
                  恢复
                </Button>
              </Tooltip>,
              <Popconfirm
                key="delete"
                title="确认删除此草稿？"
                onConfirm={() => onDelete(draft.id)}
                okText="确认"
                cancelText="取消"
              >
                <Button type="link" size="small" icon={<DeleteOutlined />} danger>
                  删除
                </Button>
              </Popconfirm>,
            ]
          : undefined
      }
    >
      <List.Item.Meta
        avatar={
          <Badge dot={!draft.recoverable}>
            <div className="draft-avatar">
              <UserOutlined />
            </div>
          </Badge>
        }
        title={
          <Space>
            <Text strong={!draft.isExpired}>{draft.title || draft.objectKey}</Text>
            {draft.isAutoSave && <Tag color="blue">自动保存</Tag>}
            {!draft.recoverable && <Tag color="default">其他用户</Tag>}
            {draft.isExpired && <Tag color="red">已过期</Tag>}
          </Space>
        }
        description={
          <div className="draft-description">
            <div className="draft-meta">
              <Space size="large">
                <span>
                  <UserOutlined /> {draft.creator.name}
                </span>
                <span>
                  <ClockCircleOutlined /> {formatDraftTime(draft.updatedAt)}
                </span>
              </Space>
            </div>
            {draft.description && <div className="draft-desc-text">{draft.description}</div>}
            <div className="draft-progress">
              <Progress
                percent={Math.max(0, Math.min(100, progressPercent))}
                showInfo={false}
                size="small"
                status={draft.isExpired ? 'exception' : 'active'}
                strokeColor={draft.recoverable ? '#52c41a' : '#d9d9d9'}
              />
              <Text type="secondary" style={{ fontSize: 11 }}>
                剩余 {Math.ceil(remainingTime / (24 * 60 * 60 * 1000))} 天
              </Text>
            </div>
          </div>
        }
      />
    </List.Item>
  );
}

/**
 * 草稿面板主组件
 */
export default function DraftPanel({
  objectKey,
  onRestore,
  filterByObject = true,
}: DraftPanelProps) {
  const [drafts, setDrafts] = useState<DraftListItem[]>([]);
  const [stats, setStats] = useState({
    total: 0,
    myDrafts: 0,
    expired: 0,
    byObject: {} as Record<string, number>,
  });
  const [loading, setLoading] = useState(false);

  // 加载草稿列表
  const loadDrafts = useCallback(() => {
    setLoading(true);
    try {
      // 清理过期草稿
      const cleaned = cleanupExpiredDrafts();

      const draftList = filterByObject ? getDrafts(objectKey) : getAllDraftList();

      setDrafts(draftList);
      setStats(getDraftStats());

      if (cleaned > 0) {
        console.log(`已清理 ${cleaned} 个过期草稿`);
      }
    } finally {
      setLoading(false);
    }
  }, [objectKey, filterByObject]);

  useEffect(() => {
    loadDrafts();

    // 定时刷新（每分钟）
    const timer = setInterval(loadDrafts, 60000);
    return () => clearInterval(timer);
  }, [loadDrafts]);

  // 恢复草稿
  const handleRestore = useCallback(
    (draft: DraftListItem) => {
      if (!draft.recoverable) {
        return;
      }

      const config = restoreDraft(draft.id);
      if (config && onRestore) {
        onRestore(config);
      }
    },
    [onRestore],
  );

  // 删除草稿
  const handleDelete = useCallback(
    (draftId: string) => {
      const success = deleteDraft(draftId);
      if (success) {
        loadDrafts();
      }
    },
    [loadDrafts],
  );

  // 清空所有草稿
  const handleClearAll = useCallback(() => {
    const user = getCurrentUser();
    if (!user) return;

    const count = clearAllDrafts();
    loadDrafts();
    // message.success(`已清空 ${count} 个草稿`);
  }, [loadDrafts]);

  const currentUser = getCurrentUser();
  const hasOtherUsersDrafts = drafts.some((d) => d.creator.id !== currentUser?.id);

  return (
    <div className="draft-panel">
      <Card
        size="small"
        title={
          <Space>
            <span>草稿</span>
            <Badge count={drafts.length} showZero />
          </Space>
        }
        extra={
          <Space>
            <Text type="secondary" style={{ fontSize: 12 }}>
              我的草稿: {stats.myDrafts}
            </Text>
            {stats.myDrafts > 0 && (
              <Popconfirm
                title="确认清空所有草稿？"
                description="此操作将删除所有草稿且无法恢复"
                onConfirm={handleClearAll}
                okText="确认"
                cancelText="取消"
              >
                <Button type="text" size="small" icon={<ClearOutlined />} danger>
                  清空
                </Button>
              </Popconfirm>
            )}
          </Space>
        }
      >
        {/* 冲突警告 */}
        {hasOtherUsersDrafts && (
          <Alert
            message="检测到其他用户的草稿"
            description="当前配置正在被其他人编辑，请注意同步避免冲突。"
            type="warning"
            icon={<WarningOutlined />}
            showIcon
            closable
            style={{ marginBottom: 12 }}
          />
        )}

        {/* 草稿列表 */}
        <List
          size="small"
          loading={loading}
          dataSource={drafts}
          renderItem={(draft) => (
            <DraftItem
              key={draft.id}
              draft={draft}
              onRestore={handleRestore}
              onDelete={handleDelete}
            />
          )}
          locale={{
            emptyText: (
              <Empty
                image={Empty.PRESENTED_IMAGE_SIMPLE}
                description="暂无草稿"
                style={{ padding: '20px 0' }}
              />
            ),
          }}
        />

        {/* 提示信息 */}
        {drafts.length > 0 && (
          <>
            <Divider style={{ margin: '12px 0' }} />
            <div className="draft-tips">
              <Text type="secondary" style={{ fontSize: 12 }}>
                💡 草稿自动保存，保留 {7} 天后过期
              </Text>
            </div>
          </>
        )}
      </Card>
    </div>
  );
}

/**
 * 草稿指示器（紧凑模式）
 */
export function DraftIndicator({
  objectKey,
  onClick,
}: {
  objectKey: string;
  onClick?: () => void;
}) {
  const [count, setCount] = useState(0);

  useEffect(() => {
    const updateCount = () => {
      const drafts = getDrafts(objectKey);
      setCount(drafts.length);
    };

    updateCount();
    const timer = setInterval(updateCount, 30000);
    return () => clearInterval(timer);
  }, [objectKey]);

  if (count === 0) return null;

  return (
    <Badge count={count} offset={[-5, 5]}>
      <Button type="text" size="small" icon={<ClockCircleOutlined />} onClick={onClick}>
        草稿
      </Button>
    </Badge>
  );
}
