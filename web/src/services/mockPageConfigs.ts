/**
 * Mock 页面配置
 *
 * 用于开发测试的示例配置
 */

import type { PageConfig } from '@/components/PageGenerator/types';

export const mockPageConfigs: PageConfig[] = [
  // ==================== 用户管理（列表页） ====================
  {
    id: 'users-list-v2',
    type: 'list',
    title: '用户管理（V2）',
    path: '/v2/users',
    icon: 'UserOutlined',
    permissions: ['canUserManage'],
    dataSource: {
      type: 'static',
      staticData: [
        {
          id: '1',
          username: 'admin',
          email: 'admin@example.com',
          role: 'admin',
          status: true,
          created_at: '2024-01-01T00:00:00Z',
        },
        {
          id: '2',
          username: 'user1',
          email: 'user1@example.com',
          role: 'user',
          status: true,
          created_at: '2024-01-02T00:00:00Z',
        },
        {
          id: '3',
          username: 'user2',
          email: 'user2@example.com',
          role: 'user',
          status: false,
          created_at: '2024-01-03T00:00:00Z',
        },
      ],
    },
    ui: {
      list: {
        columns: [
          {
            key: 'id',
            title: 'ID',
            width: 80,
          },
          {
            key: 'username',
            title: '用户名',
            width: 150,
            copyable: true,
          },
          {
            key: 'email',
            title: '邮箱',
            width: 200,
            copyable: true,
          },
          {
            key: 'role',
            title: '角色',
            width: 100,
            render: 'tag',
            renderConfig: {
              tagColor: {
                admin: 'red',
                user: 'blue',
              },
            },
          },
          {
            key: 'status',
            title: '状态',
            width: 100,
            render: 'status',
            renderConfig: {
              statusMap: {
                true: { text: '启用', status: 'success' },
                false: { text: '禁用', status: 'default' },
              },
            },
          },
          {
            key: 'created_at',
            title: '创建时间',
            width: 180,
            render: 'datetime',
          },
        ],
        actions: [
          {
            key: 'create',
            label: '新建用户',
            type: 'primary',
            icon: 'PlusOutlined',
            onClick: {
              type: 'navigate',
              path: '/v2/users/create',
            },
          },
          {
            key: 'export',
            label: '导出',
            icon: 'ExportOutlined',
            onClick: {
              type: 'api',
              api: {
                endpoint: '/api/users/export',
                method: 'POST',
              },
              onSuccess: {
                message: '导出成功',
              },
            },
          },
        ],
        rowActions: [
          {
            key: 'edit',
            label: '编辑',
            icon: 'EditOutlined',
            onClick: {
              type: 'navigate',
              path: '/v2/users/{id}/edit',
            },
          },
          {
            key: 'delete',
            label: '删除',
            icon: 'DeleteOutlined',
            danger: true,
            confirm: {
              title: '确认删除',
              content: '删除后无法恢复，确认删除吗？',
            },
            onClick: {
              type: 'function',
              functionId: 'user.delete',
              onSuccess: {
                message: '删除成功',
                refresh: true,
              },
            },
          },
        ],
        pagination: true,
      },
    },
  },

  // ==================== 创建用户（表单页） ====================
  {
    id: 'users-create-v2',
    type: 'form',
    title: '创建用户（V2）',
    path: '/v2/users/create',
    icon: 'UserAddOutlined',
    permissions: ['canUserManage'],
    dataSource: {
      type: 'function',
      functionId: 'user.create',
    },
    ui: {
      form: {
        layout: 'horizontal',
        labelCol: { span: 6 },
        wrapperCol: { span: 14 },
        fields: [
          {
            key: 'username',
            label: '用户名',
            type: 'input',
            required: true,
            placeholder: '请输入用户名',
            rules: [
              {
                type: 'pattern',
                pattern: '^[a-zA-Z0-9_]{3,20}$',
                message: '用户名只能包含字母、数字和下划线，长度3-20',
              },
            ],
          },
          {
            key: 'email',
            label: '邮箱',
            type: 'input',
            required: true,
            placeholder: '请输入邮箱',
            rules: [
              {
                type: 'email',
                message: '请输入有效的邮箱地址',
              },
            ],
          },
          {
            key: 'password',
            label: '密码',
            type: 'input',
            required: true,
            placeholder: '请输入密码',
            rules: [
              {
                min: 6,
                message: '密码长度至少6位',
              },
            ],
          },
          {
            key: 'role',
            label: '角色',
            type: 'select',
            required: true,
            placeholder: '请选择角色',
            options: [
              { label: '管理员', value: 'admin' },
              { label: '普通用户', value: 'user' },
            ],
          },
          {
            key: 'status',
            label: '状态',
            type: 'switch',
            defaultValue: true,
            tooltip: '是否启用该用户',
          },
        ],
        submitText: '创建',
        showReset: true,
      },
    },
  },

  // ==================== 用户详情（详情页） ====================
  {
    id: 'users-detail-v2',
    type: 'detail',
    title: '用户详情（V2）',
    path: '/v2/users/:id',
    icon: 'UserOutlined',
    permissions: ['canUserManage'],
    dataSource: {
      type: 'function',
      functionId: 'user.get',
    },
    ui: {
      detail: {
        column: 2,
        sections: [
          {
            title: '基本信息',
            fields: [
              {
                key: 'id',
                label: 'ID',
                copyable: true,
              },
              {
                key: 'username',
                label: '用户名',
                copyable: true,
              },
              {
                key: 'email',
                label: '邮箱',
                copyable: true,
              },
              {
                key: 'role',
                label: '角色',
                render: 'tag',
                renderConfig: {
                  tagColor: {
                    admin: 'red',
                    user: 'blue',
                  },
                },
              },
              {
                key: 'status',
                label: '状态',
                render: 'status',
              },
              {
                key: 'created_at',
                label: '创建时间',
                render: 'datetime',
              },
            ],
          },
        ],
        actions: [
          {
            key: 'edit',
            label: '编辑',
            type: 'primary',
            icon: 'EditOutlined',
            onClick: {
              type: 'navigate',
              path: '/v2/users/{id}/edit',
            },
          },
          {
            key: 'back',
            label: '返回',
            onClick: {
              type: 'navigate',
              path: '/v2/users',
            },
          },
        ],
      },
    },
  },

  // ==================== 角色管理（列表页） ====================
  {
    id: 'roles-list-v2',
    type: 'list',
    title: '角色管理（V2）',
    path: '/v2/roles',
    icon: 'TeamOutlined',
    permissions: ['canRoleManage'],
    dataSource: {
      type: 'static',
      staticData: [
        {
          id: '1',
          name: '管理员',
          code: 'admin',
          description: '系统管理员，拥有所有权限',
          user_count: 5,
          created_at: '2024-01-01T00:00:00Z',
        },
        {
          id: '2',
          name: '普通用户',
          code: 'user',
          description: '普通用户，拥有基本权限',
          user_count: 20,
          created_at: '2024-01-02T00:00:00Z',
        },
      ],
    },
    ui: {
      list: {
        columns: [
          {
            key: 'id',
            title: 'ID',
            width: 80,
          },
          {
            key: 'name',
            title: '角色名称',
            width: 150,
          },
          {
            key: 'code',
            title: '角色代码',
            width: 150,
            copyable: true,
          },
          {
            key: 'description',
            title: '描述',
            width: 300,
            ellipsis: true,
          },
          {
            key: 'user_count',
            title: '用户数',
            width: 100,
          },
          {
            key: 'created_at',
            title: '创建时间',
            width: 180,
            render: 'datetime',
          },
        ],
        actions: [
          {
            key: 'create',
            label: '新建角色',
            type: 'primary',
            icon: 'PlusOutlined',
            onClick: {
              type: 'navigate',
              path: '/v2/roles/create',
            },
          },
        ],
        rowActions: [
          {
            key: 'edit',
            label: '编辑',
            onClick: {
              type: 'navigate',
              path: '/v2/roles/{id}/edit',
            },
          },
          {
            key: 'delete',
            label: '删除',
            danger: true,
            confirm: {
              title: '确认删除',
              content: '删除后无法恢复，确认删除吗？',
            },
            onClick: {
              type: 'function',
              functionId: 'role.delete',
              onSuccess: {
                message: '删除成功',
                refresh: true,
              },
            },
          },
        ],
        pagination: true,
      },
    },
  },
];
