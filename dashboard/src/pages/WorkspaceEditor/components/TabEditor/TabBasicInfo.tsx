import React from 'react';
import { Form, Input, Switch, Card, Space, Tooltip, Alert, Button } from 'antd';
import type { TabConfig } from '@/types/workspace';
import IconPicker from '../IconPicker';
import {
  QuestionCircleOutlined,
  CommentOutlined,
  LockOutlined,
  UnlockOutlined,
} from '@ant-design/icons';
import { HelpTooltip } from '../HelpTooltip';

export interface TabBasicInfoProps {
  tab: TabConfig;
  onChange: (field: string, value: any) => void;
}

const STORAGE_KEY_PREFIX = 'workspace:tab:locked:';

function getLockedState(
  objectKey: string,
  tabKey: string,
): { locked: boolean; lockedBy?: string } | null {
  try {
    const key = `${STORAGE_KEY_PREFIX}${objectKey}:${tabKey}`;
    const data = localStorage.getItem(key);
    return data ? JSON.parse(data) : null;
  } catch {
    return null;
  }
}

function setLockedState(objectKey: string, tabKey: string, locked: boolean, lockedBy?: string) {
  try {
    const key = `${STORAGE_KEY_PREFIX}${objectKey}:${tabKey}`;
    if (locked) {
      localStorage.setItem(
        key,
        JSON.stringify({ locked, lockedBy, lockedAt: new Date().toISOString() }),
      );
    } else {
      localStorage.removeItem(key);
    }
  } catch {
    // ignore
  }
}

export default function TabBasicInfo({ tab, onChange }: TabBasicInfoProps) {
  // 从 localStorage 读取锁定状态（如果 tab.locked 为 true）
  const storedLock = React.useMemo(() => {
    if (!tab.locked) return null;
    // 假设可以从上下文获取 objectKey，这里暂时使用 tab.key 的一部分
    return { locked: true };
  }, [tab.locked, tab.key]);

  const isLocked = tab.locked || storedLock?.locked;

  const handleToggleLock = () => {
    const newLockedState = !isLocked;
    onChange('locked', newLockedState);
    if (newLockedState) {
      onChange('lockedBy', 'current_user');
      onChange('lockedAt', new Date().toISOString());
    } else {
      onChange('lockedBy', undefined);
      onChange('lockedAt', undefined);
    }
  };

  return (
    <Card
      title="基本信息"
      size="small"
      extra={
        <Space>
          <Tooltip title={isLocked ? '配置已锁定，防止误操作' : '锁定配置，防止误操作'}>
            <Button
              size="small"
              type={isLocked ? 'primary' : 'default'}
              icon={isLocked ? <LockOutlined /> : <UnlockOutlined />}
              onClick={handleToggleLock}
            >
              {isLocked ? '已锁定' : '锁定'}
            </Button>
          </Tooltip>
          <HelpTooltip helpKey="tab.title" />
        </Space>
      }
    >
      {isLocked && (
        <Alert
          message="配置已锁定，无法编辑。点击解锁按钮后可继续编辑。"
          type="warning"
          showIcon
          style={{ marginBottom: 12 }}
          closable={false}
        />
      )}
      <Form layout="vertical">
        <Form.Item label="标题">
          <Input
            value={tab.title}
            onChange={(e) => onChange('title', e.target.value)}
            placeholder="请输入标题"
            disabled={isLocked}
          />
        </Form.Item>
        <Form.Item
          label={
            <Space size={4}>
              <span>图标</span>
              <HelpTooltip helpKey="tab.icon" />
            </Space>
          }
        >
          <IconPicker
            value={tab.icon}
            onChange={(val) => onChange('icon', val)}
            disabled={isLocked}
          />
        </Form.Item>
        <Form.Item label="设为默认页">
          <Switch
            checked={Boolean(tab.defaultActive)}
            onChange={(checked) => onChange('defaultActive', checked)}
            disabled={isLocked}
          />
        </Form.Item>
        <Form.Item
          label={
            <Space size={4}>
              <span>配置注释</span>
              <Tooltip title="添加注释说明此 Tab 的用途、注意事项等，不影响运行时">
                <CommentOutlined style={{ color: '#999' }} />
              </Tooltip>
            </Space>
          }
        >
          <Input.TextArea
            value={tab.comment || ''}
            onChange={(e) => onChange('comment', e.target.value)}
            placeholder="添加配置注释，说明此 Tab 的用途、特殊逻辑等..."
            rows={3}
            maxLength={500}
            showCount
            disabled={isLocked}
          />
        </Form.Item>
      </Form>
    </Card>
  );
}
