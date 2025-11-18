/**
 * 用户管理页面示例
 * 演示如何添加新的权限管理功能
 * 
 * 需要在以下文件中进行配置:
 * 1. config/routes.ts - 添加路由
 * 2. src/access.ts - 添加权限检查
 * 3. src/locales/en-US/menu.ts - 添加英文菜单
 * 4. src/locales/zh-CN/menu.ts - 添加中文菜单
 * 5. src/services/croupier/index.ts - 添加 API 调用
 */

import React, { useEffect, useMemo, useState } from 'react';
import {
  Card,
  Table,
  Space,
  Button,
  Modal,
  Form,
  Input,
  Select,
  Tag,
  message,
  Popconfirm,
  Drawer,
  Switch,
} from 'antd';
import { useModel } from '@umijs/max';
import { request } from '@umijs/max';
import type { ColumnType } from 'antd/es/table';

// 类型定义
interface User {
  id: string;
  username: string;
  email?: string;
  roles: string[];
  enabled: boolean;
  created_at: string;
  updated_at: string;
}

export default function UsersPage() {
  const [data, setData] = useState<User[]>([]);
  const [loading, setLoading] = useState(false);
  const [form] = Form.useForm();
  
  // 权限检查
  const { initialState } = useModel('@@initialState');
  const roles = useMemo(() => {
    const acc = (initialState as any)?.currentUser?.access as string | undefined;
    return (acc ? acc.split(',') : []).filter(Boolean);
  }, [initialState]);

  const canRead = roles.includes('*') || roles.includes('users:read');
  const canWrite = roles.includes('*') || roles.includes('users:write');

  // API 调用
  const fetchUsers = async () => {
    setLoading(true);
    try {
      const response = await request<{ users: User[] }>('/api/users');
      setData(response.users || []);
    } catch (e: any) {
      message.error(e?.message || 'Failed');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (canRead) fetchUsers();
  }, [canRead]);

  // 权限检查
  if (!canRead) {
    return <Card title="Users"><span>No permission</span></Card>;
  }

  return (
    <Card title="Users" extra={<Button type="primary" disabled={!canWrite}>Add</Button>}>
      <Table
        rowKey="id"
        loading={loading}
        columns={[
          { title: 'Username', dataIndex: 'username' },
          { title: 'Email', dataIndex: 'email' },
          {
            title: 'Roles',
            dataIndex: 'roles',
            render: (roles: string[]) => (
              <Space size={4}>
                {(roles || []).map((r) => (
                  <Tag key={r} color="blue">{r}</Tag>
                ))}
              </Space>
            ),
          },
        ]}
        dataSource={data}
      />
    </Card>
  );
}
