import React, { useEffect, useState } from 'react';
import { Card, Table, Space, Tag, Button, App, Tabs, Badge } from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import type { ColumnsType } from 'antd/es/table';
import { ApiOutlined, ExperimentOutlined } from '@ant-design/icons';
import { listPlatforms, listPlatformMethods, type PlatformInfo } from '@/services/api/platforms';
import APITester from './APITester';
import PlatformConfig from './PlatformConfig';

export default function PlatformsPage() {
  const { message } = App.useApp();
  const [loading, setLoading] = useState(false);
  const [platforms, setPlatforms] = useState<PlatformInfo[]>([]);
  const [selectedPlatform, setSelectedPlatform] = useState<string | null>(null);
  const [platformMethods, setPlatformMethods] = useState<string[]>([]);
  const [activeTab, setActiveTab] = useState('list');

  const loadPlatforms = async () => {
    setLoading(true);
    try {
      const r = await listPlatforms();
      setPlatforms(r?.platforms || []);
    } catch (err: any) {
      if (err?.response?.status === 503) {
        message.warning('第三方平台扩展未就绪，请先安装并启用 official.external-platform');
      } else {
        message.error('加载平台列表失败');
      }
      setPlatforms([]);
    } finally {
      setLoading(false);
    }
  };

  const loadPlatformMethods = async (platformName: string) => {
    try {
      const r = await listPlatformMethods(platformName);
      setPlatformMethods(r?.methods || []);
    } catch {
      message.error('加载平台方法失败');
      setPlatformMethods([]);
    }
  };

  useEffect(() => {
    loadPlatforms();
  }, []);

  const columns: ColumnsType<PlatformInfo> = [
    {
      title: '平台名称',
      dataIndex: 'name',
      key: 'name',
      render: (name: string) => {
        const displayName = name === 'quicksdk' ? 'QuickSDK' : name;
        return <strong>{displayName}</strong>;
      },
    },
    {
      title: '状态',
      dataIndex: 'enabled',
      key: 'enabled',
      render: (enabled: boolean) => (
        <Badge status={enabled ? 'success' : 'default'} text={enabled ? '已启用' : '未启用'} />
      ),
    },
    {
      title: '支持方法数',
      dataIndex: 'methods',
      key: 'methods',
      render: (methods: string[]) => <Tag color="blue">{methods?.length || 0} 个 API</Tag>,
    },
    {
      title: '来源',
      dataIndex: 'source',
      key: 'source',
      render: (source?: string) => {
        if (source === 'extension') return <Tag color="green">扩展安装</Tag>;
        return <Tag color="default">{source || '未知'}</Tag>;
      },
    },
    {
      title: '操作',
      key: 'action',
      render: (_: any, record: PlatformInfo) => (
        <Space>
          <Button
            size="small"
            icon={<ApiOutlined />}
            onClick={() => {
              setSelectedPlatform(record.name);
              loadPlatformMethods(record.name);
              setActiveTab('test');
            }}
          >
            测试 API
          </Button>
        </Space>
      ),
    },
  ];

  const quickSDKMethods = [
    {
      category: '基础数据',
      methods: ['channel_list', 'server_list', 'product_list', 'role_info', 'order_list'],
    },
    {
      category: '运营报表',
      methods: [
        'day_report',
        'day_hour_report',
        'user_live',
        'channel_days_report',
        'channel_report',
      ],
    },
    {
      category: '广告管理',
      methods: [
        'ad_report',
        'media_app_list',
        'ad_plan_group_list',
        'package_version_list',
        'ad_pages_list',
        'create_ad_plan',
        'update_ad_plan',
        'ad_plan_list',
      ],
    },
    { category: '其他', methods: ['user_lost_list', 'push_message'] },
  ];

  return (
    <PageContainer
      title="第三方运营平台"
      subTitle="管理和测试基于扩展安装的第三方平台能力"
      extra={[
        <Button key="refresh" onClick={loadPlatforms} loading={loading}>
          刷新
        </Button>,
      ]}
    >
      <Tabs
        activeKey={activeTab}
        onChange={setActiveTab}
        items={[
          {
            key: 'list',
            label: (
              <span>
                平台列表 <Badge count={platforms?.length || 0} showZero />
              </span>
            ),
            children: (
              <Card>
                <Table
                  rowKey="name"
                  dataSource={platforms}
                  columns={columns}
                  loading={loading}
                  pagination={false}
                />
              </Card>
            ),
          },
          {
            key: 'test',
            label: (
              <span>
                API 测试 <ExperimentOutlined />
              </span>
            ),
            children: (
              <Card>
                <APITester
                  platforms={platforms}
                  selectedPlatform={selectedPlatform}
                  onPlatformChange={setSelectedPlatform}
                />
              </Card>
            ),
          },
          {
            key: 'config',
            label: '配置说明',
            children: (
              <Card>
                <PlatformConfig />
              </Card>
            ),
          },
        ]}
      />

      {/* 方法说明 */}
      {selectedPlatform && (
        <Card style={{ marginTop: 16 }} title={`QuickSDK 支持的 ${platformMethods.length} 个方法`}>
          {quickSDKMethods.map((cat) => (
            <div key={cat.category} style={{ marginBottom: 16 }}>
              <strong>{cat.category}:</strong>
              <Space wrap style={{ marginLeft: 8 }}>
                {cat.methods.map((m) => (
                  <Tag key={m} color={platformMethods.includes(m) ? 'green' : 'default'}>
                    {m}
                  </Tag>
                ))}
              </Space>
            </div>
          ))}
        </Card>
      )}
    </PageContainer>
  );
}
