import React, { useState, useEffect } from 'react';
import {
  Drawer,
  Form,
  Select,
  Button,
  Space,
  Typography,
  Tag,
  Divider,
  List,
  Empty,
} from 'antd';
import { PlusOutlined, DeleteOutlined, TeamOutlined, SafetyOutlined } from '@ant-design/icons';
import type { WorkspaceConfig } from '@/types/workspace';

const { Text, Title } = Typography;

/** 可选角色列表 */
const AVAILABLE_ROLES = [
  { label: '管理员', value: 'admin' },
  { label: '超级管理员', value: 'super_admin' },
  { label: '运营', value: 'operator' },
  { label: 'GM', value: 'gm' },
  { label: '开发者', value: 'developer' },
  { label: '测试', value: 'tester' },
  { label: '观察者', value: 'viewer' },
];

function getSelectedRoles(config: WorkspaceConfig | null): string[] {
  const permissions = config?.permissions;
  if (!permissions || Array.isArray(permissions)) return [];
  return Array.isArray(permissions.roles) ? permissions.roles : [];
}

export interface PermissionsDrawerProps {
  open: boolean;
  config: WorkspaceConfig | null;
  onClose: () => void;
  onSave: (permissions: WorkspaceConfig['permissions']) => void;
}

/**
 * 工作台权限设置抽屉
 */
export default function PermissionsDrawer({
  open,
  config,
  onClose,
  onSave,
}: PermissionsDrawerProps) {
  const [selectedRoles, setSelectedRoles] = useState<string[]>([]);

  useEffect(() => {
    setSelectedRoles(getSelectedRoles(config));
  }, [config]);

  const handleSave = () => {
    const permissions = selectedRoles.length > 0 ? { roles: selectedRoles } : undefined;
    onSave(permissions);
    onClose();
  };

  const handleAddRole = (role: string) => {
    if (!selectedRoles.includes(role)) {
      setSelectedRoles([...selectedRoles, role]);
    }
  };

  const handleRemoveRole = (role: string) => {
    setSelectedRoles(selectedRoles.filter((r) => r !== role));
  };

  return (
    <Drawer
      title="权限设置"
      open={open}
      onClose={onClose}
      width={400}
      extra={
        <Button type="primary" onClick={handleSave}>
          保存
        </Button>
      }
    >
      <Space direction="vertical" size={24} style={{ width: '100%' }}>
        {/* 说明 */}
        <div>
          <Title level={5} style={{ marginBottom: 8 }}>
            <SafetyOutlined style={{ marginRight: 8 }} />
            访问权限配置
          </Title>
          <Text type="secondary">
            配置哪些角色可以访问此工作台。如果不配置任何角色，则所有用户都可以访问。
          </Text>
        </div>

        <Divider />

        {/* 添加角色 */}
        <div>
          <Text strong style={{ marginBottom: 12, display: 'block' }}>
            <TeamOutlined style={{ marginRight: 8 }} />
            允许的角色
          </Text>
          <Select
            style={{ width: '100%' }}
            placeholder="选择要添加的角色"
            onChange={handleAddRole}
            value={undefined}
            options={AVAILABLE_ROLES.filter((role) => !selectedRoles.includes(role.value))}
          />
        </div>

        {/* 已选角色列表 */}
        <div>
          <Text type="secondary" style={{ marginBottom: 8, display: 'block' }}>
            已选择的角色：
          </Text>
          {selectedRoles.length > 0 ? (
            <List
              size="small"
              dataSource={selectedRoles}
              renderItem={(role) => {
                const roleConfig = AVAILABLE_ROLES.find((r) => r.value === role);
                return (
                  <List.Item
                    actions={[
                      <Button
                        key="delete"
                        type="text"
                        danger
                        icon={<DeleteOutlined />}
                        size="small"
                        onClick={() => handleRemoveRole(role)}
                      />,
                    ]}
                  >
                    <Tag color="blue">{roleConfig?.label || role}</Tag>
                  </List.Item>
                );
              }}
            />
          ) : (
            <Empty
              image={Empty.PRESENTED_IMAGE_SIMPLE}
              description="未配置角色限制，所有用户可访问"
              style={{ margin: '16px 0' }}
            />
          )}
        </div>

        <Divider />

        {/* 提示信息 */}
        <div
          style={{
            background: '#f6ffed',
            border: '1px solid #b7eb8f',
            borderRadius: 6,
            padding: 12,
          }}
        >
          <Text type="secondary" style={{ fontSize: 12 }}>
            <strong>提示：</strong>
            <ul style={{ margin: '4px 0 0 0', paddingLeft: 16 }}>
              <li>管理员（admin/super_admin）始终可以访问所有工作台</li>
              <li>如果不选择任何角色，则所有用户都可以访问</li>
              <li>权限设置在发布后生效</li>
            </ul>
          </Text>
        </div>
      </Space>
    </Drawer>
  );
}
