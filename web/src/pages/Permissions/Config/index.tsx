import React, { useState } from 'react';
import {
  Card,
  Table,
  Tag,
  Space,
  Typography,
  Tabs,
  Collapse,
  Badge,
  Tooltip,
  Input,
  Button,
} from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import {
  SettingOutlined,
  SecurityScanOutlined,
  InfoCircleOutlined,
  SearchOutlined,
} from '@ant-design/icons';
import { permissionDomains, type PermissionDomain } from './domains';

const { Title, Text, Paragraph } = Typography;
// Avoid deprecated Input.Search (uses addonAfter). Use Space.Compact instead.

export default function ConfigPage() {
  const [searchText, setSearchText] = useState('');
  const [activeTab, setActiveTab] = useState('domains');

  const filteredDomains = permissionDomains.filter(
    (domain) =>
      domain.domain.toLowerCase().includes(searchText.toLowerCase()) ||
      domain.description.toLowerCase().includes(searchText.toLowerCase()) ||
      domain.permissions.some((perm) => perm.toLowerCase().includes(searchText.toLowerCase())),
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
      render: (_: unknown, _record: PermissionDomain) => (
        <Button type="link" icon={<InfoCircleOutlined />}>
          查看详情
        </Button>
      ),
    },
  ];

  // 本页为权限域目录只读展示（权限域为系统定义，无用户可编辑状态，
  // 后端亦无"保存权限配置"API），故不提供保存动作。

  return (
    <PageContainer>
      <div style={{ padding: '24px' }}>
        <Card>
          <div style={{ marginBottom: '16px' }}>
            <Title level={2}>
              <SettingOutlined style={{ marginRight: '8px', color: '#1890ff' }} />
              权限配置管理
            </Title>
            <Text type="secondary">管理系统权限域和具体权限配置，确保权限体系的完整性和安全性</Text>
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
                    <div
                      style={{
                        marginBottom: '16px',
                        display: 'flex',
                        gap: '16px',
                        alignItems: 'center',
                      }}
                    >
                      <Space.Compact style={{ width: 420 }}>
                        <Input
                          placeholder="搜索权限域或权限"
                          value={searchText}
                          onChange={(e) => setSearchText(e.target.value)}
                          prefix={<SearchOutlined />}
                        />
                        <Button
                          type="primary"
                          icon={<SearchOutlined />}
                          onClick={() => {
                            /* filter happens on change */
                          }}
                        />
                      </Space.Compact>
                      <div style={{ marginLeft: 'auto' }}>
                        <Text type="secondary">
                          总计 {filteredDomains.length} 个权限域，
                          {permissionDomains.reduce(
                            (sum, domain) => sum + domain.permissions.length,
                            0,
                          )}{' '}
                          个具体权限
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
                        showTotal: (total, range) =>
                          `第 ${range[0]}-${range[1]} 项，共 ${total} 项`,
                      }}
                    />
                  </>
                ),
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
                      items={filteredDomains.map((domain) => ({
                        key: domain.domain,
                        label: (
                          <Space>
                            <span style={{ color: domain.color }}>{domain.icon}</span>
                            <Text strong>{domain.domain}:*</Text>
                            <Badge
                              count={domain.permissions.length}
                              style={{ backgroundColor: domain.color }}
                            />
                            <Text type="secondary">- {domain.description}</Text>
                          </Space>
                        ),
                        children: (
                          <div style={{ padding: '16px', background: '#fafafa', borderRadius: 8 }}>
                            <Title level={5}>权限列表</Title>
                            <div style={{ display: 'flex', flexWrap: 'wrap', gap: '8px' }}>
                              {domain.permissions.map((permission) => (
                                <Tooltip
                                  key={permission}
                                  title={`${domain.domain} 域下的 ${permission} 权限`}
                                >
                                  <Tag color={domain.color}>{permission}</Tag>
                                </Tooltip>
                              ))}
                            </div>

                            <Title level={5} style={{ marginTop: '16px' }}>
                              权限说明
                            </Title>
                            <Paragraph type="secondary">
                              {domain.description}。该权限域包含 {domain.permissions.length}{' '}
                              个具体权限， 用于控制 {domain.domain} 相关的操作权限。
                            </Paragraph>
                          </div>
                        ),
                      }))}
                    />
                  </>
                ),
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
                        {permissionDomains.reduce(
                          (sum, domain) => sum + domain.permissions.length,
                          0,
                        )}{' '}
                        个具体权限
                      </Text>
                    </div>

                    <div
                      style={{
                        display: 'grid',
                        gridTemplateColumns: 'repeat(auto-fill, minmax(300px, 1fr))',
                        gap: '16px',
                      }}
                    >
                      {permissionDomains.map((domain) => (
                        <Card
                          key={domain.domain}
                          size="small"
                          title={
                            <Space>
                              <span style={{ color: domain.color }}>{domain.icon}</span>
                              <Text strong>{domain.domain}</Text>
                              <Badge
                                count={domain.permissions.length}
                                style={{ backgroundColor: domain.color }}
                              />
                            </Space>
                          }
                          hoverable
                        >
                          <Paragraph ellipsis={{ rows: 2 }} style={{ marginBottom: '8px' }}>
                            {domain.description}
                          </Paragraph>
                          <div style={{ display: 'flex', flexWrap: 'wrap', gap: '4px' }}>
                            {domain.permissions.slice(0, 5).map((permission) => (
                              <Tag
                                key={permission}
                                color={domain.color}
                                style={{ fontSize: 12, lineHeight: '20px', padding: '0 7px' }}
                              >
                                {permission}
                              </Tag>
                            ))}
                            {domain.permissions.length > 5 && (
                              <Tag style={{ fontSize: 12, lineHeight: '20px', padding: '0 7px' }}>
                                +{domain.permissions.length - 5}
                              </Tag>
                            )}
                          </div>
                        </Card>
                      ))}
                    </div>
                  </>
                ),
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
                      <div
                        style={{
                          display: 'grid',
                          gridTemplateColumns: 'repeat(auto-fit, minmax(250px, 1fr))',
                          gap: '16px',
                        }}
                      >
                        <div>
                          <Title level={5}>最小权限原则</Title>
                          <Text type="secondary">
                            用户只获得完成其工作职能所需的最小权限集合，避免权限过度分配。
                          </Text>
                        </div>
                        <div>
                          <Title level={5}>权限分离</Title>
                          <Text type="secondary">关键操作需要多人协作完成，避免单点权限风险。</Text>
                        </div>
                        <div>
                          <Title level={5}>定期审查</Title>
                          <Text type="secondary">定期检查用户权限分配，及时回收不需要的权限。</Text>
                        </div>
                        <div>
                          <Title level={5}>审计记录</Title>
                          <Text type="secondary">所有权限操作都有详细的审计日志记录。</Text>
                        </div>
                      </div>
                    </Card>

                    <Card title="高风险权限" style={{ marginBottom: '16px' }}>
                      <div
                        style={{
                          background: '#fff2e8',
                          padding: '16px',
                          borderRadius: 8,
                          border: '1px solid #ffbb96',
                        }}
                      >
                        <Space orientation="vertical" style={{ width: '100%' }}>
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
                ),
              },
            ]}
          />
        </Card>
      </div>
    </PageContainer>
  );
}
