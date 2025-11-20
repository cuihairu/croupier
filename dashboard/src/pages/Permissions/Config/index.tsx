import React, { useState } from 'react';
import { Card, Table, Tag, Space, Typography, Tabs, Collapse, Badge, Tooltip, Input, Button } from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import { getMessage } from '@/utils/antdApp';
import {
  SettingOutlined,
  SecurityScanOutlined,
  InfoCircleOutlined,
  SearchOutlined,
  SaveOutlined
} from '@ant-design/icons';

const { Title, Text, Paragraph } = Typography;
// Avoid deprecated Input.Search (uses addonAfter). Use Space.Compact instead.

interface PermissionDomain {
  domain: string;
  description: string;
  permissions: string[];
  color: string;
  icon: React.ReactNode;
}

// 权限域数据
const permissionDomains: PermissionDomain[] = [
  {
    domain: 'system',
    description: '系统级操作权限',
    permissions: ['system:config', 'system:restart', 'system:backup', 'system:monitor', 'system:security', 'system:audit'],
    color: '#ff4d4f',
    icon: <SettingOutlined />
  },
  {
    domain: 'user',
    description: '用户管理权限',
    permissions: ['user:create', 'user:update', 'user:delete', 'user:view', 'user:developer:manage', 'user:support:manage'],
    color: '#fa541c',
    icon: <SecurityScanOutlined />
  },
  {
    domain: 'game',
    description: '游戏配置管理权限',
    permissions: ['game:config', 'game:deploy', 'game:restart', 'game:config:read', 'game:config:update', 'game:config:numerical'],
    color: '#fa8c16',
    icon: <SettingOutlined />
  },
  {
    domain: 'player',
    description: '玩家管理权限',
    permissions: ['player:query', 'player:update', 'player:ban', 'player:create:test', 'player:update:basic', 'player:export', 'player:communicate', 'player:behavior', 'player:segment', 'player:bot', 'player:security', 'player:audit'],
    color: '#faad14',
    icon: <SecurityScanOutlined />
  },
  {
    domain: 'function',
    description: '函数管理权限',
    permissions: ['function:register', 'function:update', 'function:test', 'function:view', 'function:deploy'],
    color: '#1890ff',
    icon: <SettingOutlined />
  },
  {
    domain: 'job',
    description: '任务管理权限',
    permissions: ['job:create', 'job:view', 'job:cancel', 'job:retry', 'job:create:readonly'],
    color: '#13c2c2',
    icon: <SettingOutlined />
  },
  {
    domain: 'audit',
    description: '审计查看权限',
    permissions: ['audit:view', 'audit:export', 'audit:content', 'audit:support', 'audit:gm', 'audit:bot', 'audit:view:player', 'audit:view:user'],
    color: '#52c41a',
    icon: <SecurityScanOutlined />
  },
  {
    domain: 'monitor',
    description: '监控数据权限',
    permissions: ['monitor:view', 'monitor:alert', 'monitor:bot', 'monitor:security'],
    color: '#722ed1',
    icon: <SettingOutlined />
  },
  {
    domain: 'data',
    description: '数据分析权限',
    permissions: ['data:query', 'data:export', 'data:report', 'data:operation', 'data:marketing', 'data:economy', 'data:balance', 'data:level', 'data:user', 'data:bot', 'data:audit'],
    color: '#eb2f96',
    icon: <SecurityScanOutlined />
  },
  {
    domain: 'design',
    description: '游戏设计/策划权限',
    permissions: ['design:level', 'design:system', 'design:feature', 'design:numerical', 'design:ui', 'design:view', 'design:research'],
    color: '#f759ab',
    icon: <SettingOutlined />
  },
  {
    domain: 'numerical',
    description: '数值配置权限',
    permissions: ['numerical:balance', 'numerical:economy'],
    color: '#ff85c0',
    icon: <SettingOutlined />
  },
  {
    domain: 'level',
    description: '关卡管理权限',
    permissions: ['level:create', 'level:edit', 'level:publish'],
    color: '#b37feb',
    icon: <SettingOutlined />
  },
  {
    domain: 'content',
    description: '内容管理权限',
    permissions: ['content:create', 'content:edit', 'content:publish', 'content:manage', 'content:level', 'content:system', 'content:ui', 'content:marketing', 'content:community'],
    color: '#9254de',
    icon: <SettingOutlined />
  },
  {
    domain: 'marketing',
    description: '市场营销权限',
    permissions: ['marketing:campaign', 'marketing:analytics'],
    color: '#73d13d',
    icon: <SettingOutlined />
  },
  {
    domain: 'community',
    description: '社区管理权限',
    permissions: ['community:moderate', 'community:event'],
    color: '#95de64',
    icon: <SettingOutlined />
  },
  {
    domain: 'event',
    description: '活动管理权限',
    permissions: ['event:create', 'event:config', 'event:start', 'event:design', 'event:marketing', 'event:content', 'event:community', 'event:gm'],
    color: '#5cdbd3',
    icon: <SettingOutlined />
  },
  {
    domain: 'announcement',
    description: '公告系统权限',
    permissions: ['announcement:create', 'announcement:publish', 'announcement:marketing', 'announcement:community', 'announcement:gm'],
    color: '#69c0ff',
    icon: <SettingOutlined />
  },
  {
    domain: 'mail',
    description: '邮件系统权限',
    permissions: ['mail:send', 'mail:template', 'mail:system', 'mail:support', 'mail:support:template'],
    color: '#85a5ff',
    icon: <SettingOutlined />
  },
  {
    domain: 'ban',
    description: '封禁管理权限',
    permissions: ['ban:player', 'ban:temporary', 'ban:permanent'],
    color: '#ffc069',
    icon: <SecurityScanOutlined />
  },
  {
    domain: 'reward',
    description: '奖励发放权限',
    permissions: ['reward:send', 'reward:config', 'reward:compensation', 'reward:compensation:basic'],
    color: '#ffd666',
    icon: <SettingOutlined />
  },
  {
    domain: 'gm',
    description: 'GM工具权限',
    permissions: ['gm:teleport', 'gm:spawn', 'gm:modify'],
    color: '#fff566',
    icon: <SecurityScanOutlined />
  },
  {
    domain: 'bot',
    description: '机器人/托管理权限',
    permissions: ['bot:create', 'bot:config', 'bot:control'],
    color: '#d3adf7',
    icon: <SettingOutlined />
  },
  {
    domain: 'security',
    description: '安全管理权限',
    permissions: ['security:monitor', 'security:investigate'],
    color: '#efdbff',
    icon: <SecurityScanOutlined />
  },
  {
    domain: 'economy',
    description: '游戏经济系统权限',
    permissions: ['economy:balance', 'economy:report', 'economy:analyze'],
    color: '#f9f0ff',
    icon: <SettingOutlined />
  },
  {
    domain: 'support',
    description: '客服功能权限',
    permissions: ['support:ticket', 'support:chat'],
    color: '#ff7875',
    icon: <SecurityScanOutlined />
  }
];

export default function ConfigPage() {
  const [searchText, setSearchText] = useState('');
  const [activeTab, setActiveTab] = useState('domains');

  const filteredDomains = permissionDomains.filter(domain =>
    domain.domain.toLowerCase().includes(searchText.toLowerCase()) ||
    domain.description.toLowerCase().includes(searchText.toLowerCase()) ||
    domain.permissions.some(perm => perm.toLowerCase().includes(searchText.toLowerCase()))
  );

  const domainColumns = [
    {
      title: '权限域',
      dataIndex: 'domain',
      key: 'domain',
      render: (text: string, record: PermissionDomain) => (
        <Space>
          <span style={{ color: record.color }}>{record.icon}</span>
          <Text strong>{text}:*</Text>
        </Space>
      ),
    },
    {
      title: '描述',
      dataIndex: 'description',
      key: 'description',
    },
    {
      title: '权限数量',
      dataIndex: 'permissions',
      key: 'permissions',
      render: (permissions: string[]) => (
        <Badge count={permissions.length} style={{ backgroundColor: '#1890ff' }} />
      ),
    },
    {
      title: '操作',
      key: 'action',
      render: (_: any, record: PermissionDomain) => (
        <Button type="link" icon={<InfoCircleOutlined />}>
          查看详情
        </Button>
      ),
    },
  ];

  const handleSaveConfig = () => {
    getMessage()?.success('权限配置保存成功');
  };

  return (
    <PageContainer>
    <div style={{ padding: '24px' }}>
      <Card>
        <div style={{ marginBottom: '16px' }}>
          <Title level={2}>
            <SettingOutlined style={{ marginRight: '8px', color: '#1890ff' }} />
            权限配置管理
          </Title>
          <Text type="secondary">
            管理系统权限域和具体权限配置，确保权限体系的完整性和安全性
          </Text>
        </div>

        <Tabs
          activeKey={activeTab}
          onChange={setActiveTab}
          items={[
            {
              key: 'domains',
              label: '权限域总览',
              children: (
                <>
                  <div style={{ marginBottom: '16px', display: 'flex', gap: '16px', alignItems: 'center' }}>
                    <Space.Compact style={{ width: 420 }}>
                      <Input
                        placeholder="搜索权限域或权限"
                        value={searchText}
                        onChange={(e) => setSearchText(e.target.value)}
                        prefix={<SearchOutlined />}
                      />
                      <Button type="primary" icon={<SearchOutlined />} onClick={() => { /* filter happens on change */ }} />
                    </Space.Compact>
                    <Button
                      type="primary"
                      icon={<SaveOutlined />}
                      onClick={handleSaveConfig}
                    >
                      保存配置
                    </Button>
                    <div style={{ marginLeft: 'auto' }}>
                      <Text type="secondary">
                        总计 {filteredDomains.length} 个权限域，
                        {permissionDomains.reduce((sum, domain) => sum + domain.permissions.length, 0)} 个具体权限
                      </Text>
                    </div>
                  </div>

                  <Table
                    columns={domainColumns}
                    dataSource={filteredDomains}
                    rowKey="domain"
                    pagination={{
                      pageSize: 10,
                      showSizeChanger: true,
                      showQuickJumper: true,
                      showTotal: (total, range) => `第 ${range[0]}-${range[1]} 项，共 ${total} 项`,
                    }}
                  />
                </>
              )
            },
            {
              key: 'details',
              label: '权限详情',
              children: (
                <>
                  <div style={{ marginBottom: '16px' }}>
                    <Space.Compact style={{ width: 420 }}>
                      <Input
                        placeholder="搜索权限域或权限"
                        value={searchText}
                        onChange={(e) => setSearchText(e.target.value)}
                        prefix={<SearchOutlined />}
                      />
                      <Button type="primary" icon={<SearchOutlined />} onClick={() => {}} />
                    </Space.Compact>
                  </div>

                  <Collapse
                    items={filteredDomains.map(domain => ({
                      key: domain.domain,
                      label: (
                        <Space>
                          <span style={{ color: domain.color }}>{domain.icon}</span>
                          <Text strong>{domain.domain}:*</Text>
                          <Badge count={domain.permissions.length} style={{ backgroundColor: domain.color }} />
                          <Text type="secondary">- {domain.description}</Text>
                        </Space>
                      ),
                      children: (
                        <div style={{ padding: '16px', background: '#fafafa', borderRadius: '6px' }}>
                          <Title level={5}>权限列表</Title>
                          <div style={{ display: 'flex', flexWrap: 'wrap', gap: '8px' }}>
                            {domain.permissions.map(permission => (
                              <Tooltip key={permission} title={`${domain.domain} 域下的 ${permission} 权限`}>
                                <Tag color={domain.color}>{permission}</Tag>
                              </Tooltip>
                            ))}
                          </div>

                          <Title level={5} style={{ marginTop: '16px' }}>权限说明</Title>
                          <Paragraph type="secondary">
                            {domain.description}。该权限域包含 {domain.permissions.length} 个具体权限，
                            用于控制 {domain.domain} 相关的操作权限。
                          </Paragraph>
                        </div>
                      )
                    }))}
                  />
                </>
              )
            },
            {
              key: 'matrix',
              label: '权限矩阵',
              children: (
                <>
                  <div style={{ marginBottom: '16px' }}>
                    <Title level={4}>权限域统计</Title>
                    <Text type="secondary">
                      系统共有 {permissionDomains.length} 个权限域，
                      {permissionDomains.reduce((sum, domain) => sum + domain.permissions.length, 0)} 个具体权限
                    </Text>
                  </div>

                  <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(300px, 1fr))', gap: '16px' }}>
                    {permissionDomains.map(domain => (
                      <Card
                        key={domain.domain}
                        size="small"
                        title={
                          <Space>
                            <span style={{ color: domain.color }}>{domain.icon}</span>
                            <Text strong>{domain.domain}</Text>
                            <Badge count={domain.permissions.length} style={{ backgroundColor: domain.color }} />
                          </Space>
                        }
                        hoverable
                      >
                        <Paragraph ellipsis={{ rows: 2 }} style={{ marginBottom: '8px' }}>
                          {domain.description}
                        </Paragraph>
                        <div style={{ display: 'flex', flexWrap: 'wrap', gap: '4px' }}>
                          {domain.permissions.slice(0, 5).map(permission => (
                            <Tag key={permission} color={domain.color} style={{ fontSize: 12, lineHeight: '20px', padding: '0 7px' }}>
                              {permission}
                            </Tag>
                          ))}
                          {domain.permissions.length > 5 && (
                            <Tag style={{ fontSize: 12, lineHeight: '20px', padding: '0 7px' }}>+{domain.permissions.length - 5}</Tag>
                          )}
                        </div>
                      </Card>
                    ))}
                  </div>
                </>
              )
            },
            {
              key: 'security',
              label: '安全配置',
              children: (
                <>
                  <div style={{ marginBottom: '16px' }}>
                    <Title level={4}>安全策略配置</Title>
                    <Text type="secondary">
                      配置权限安全策略，包括最小权限原则、权限分离等安全措施
                    </Text>
                  </div>

                  <Card title="权限安全原则" style={{ marginBottom: '16px' }}>
                    <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(250px, 1fr))', gap: '16px' }}>
                      <div>
                        <Title level={5}>最小权限原则</Title>
                        <Text type="secondary">
                          用户只获得完成其工作职能所需的最小权限集合，避免权限过度分配。
                        </Text>
                      </div>
                      <div>
                        <Title level={5}>权限分离</Title>
                        <Text type="secondary">
                          关键操作需要多人协作完成，避免单点权限风险。
                        </Text>
                      </div>
                      <div>
                        <Title level={5}>定期审查</Title>
                        <Text type="secondary">
                          定期检查用户权限分配，及时回收不需要的权限。
                        </Text>
                      </div>
                      <div>
                        <Title level={5}>审计记录</Title>
                        <Text type="secondary">
                          所有权限操作都有详细的审计日志记录。
                        </Text>
                      </div>
                    </div>
                  </Card>

                  <Card title="高风险权限" style={{ marginBottom: '16px' }}>
                    <div style={{ background: '#fff2e8', padding: '16px', borderRadius: '6px', border: '1px solid #ffbb96' }}>
                      <Space direction="vertical" style={{ width: '100%' }}>
                        <Text strong style={{ color: '#d4380d' }}>
                          <SecurityScanOutlined /> 以下权限需要特别注意
                        </Text>
                        <div>
                          <Tag color="red">*</Tag>
                          <Text>超级权限，拥有所有系统权限</Text>
                        </div>
                        <div>
                          <Tag color="orange">system:*</Tag>
                          <Text>系统级权限，可以重启、配置系统</Text>
                        </div>
                        <div>
                          <Tag color="orange">user:*</Tag>
                          <Text>用户管理权限，可以创建、删除用户</Text>
                        </div>
                        <div>
                          <Tag color="orange">ban:*</Tag>
                          <Text>封禁权限，可以封禁玩家账号</Text>
                        </div>
                        <div>
                          <Tag color="orange">security:*</Tag>
                          <Text>安全权限，可以查看和处理安全事件</Text>
                        </div>
                      </Space>
                    </div>
                  </Card>
                </>
              )
            }
          ]}
        />
      </Card>
    </div>
    </PageContainer>
  );
}
