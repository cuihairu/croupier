import React, { useState, useEffect } from 'react';
import { Card, Tabs, Badge, Space, Button, Dropdown, Menu } from 'antd';
import {
  FunctionOutlined,
  DatabaseOutlined,
  AppstoreOutlined,
  MonitorOutlined,
  SettingOutlined,
  PlusOutlined,
  ApartmentOutlined
} from '@ant-design/icons';
import { useModel } from '@umijs/max';
import FunctionWorkspace from './components/FunctionWorkspace';
import RegistryDashboard from './components/RegistryDashboard';
import PackageCenter from './components/PackageCenter';
import ExecutionMonitor from './components/ExecutionMonitor';
import VirtualObjectManager from './components/VirtualObjectManager';
import GameSelector from '@/components/GameSelector';

const { TabPane } = Tabs;

interface ComponentStats {
  totalFunctions: number;
  activeFunctions: number;
  runningJobs: number;
  availablePackages: number;
  connectedAgents: number;
  virtualObjects: number;
}

export default function ComponentManagement() {
  const [activeTab, setActiveTab] = useState('workspace');
  const [stats, setStats] = useState<ComponentStats>({
    totalFunctions: 0,
    activeFunctions: 0,
    runningJobs: 0,
    availablePackages: 0,
    connectedAgents: 0,
    virtualObjects: 0
  });

  const { initialState } = useModel('@@initialState');
  const currentUser = (initialState as any)?.currentUser;
  const roles = currentUser?.access?.split(',') || [];

  // 权限检查
  const canManagePackages = roles.includes('*') || roles.includes('functions:manage');
  const canViewRegistry = roles.includes('*') || roles.includes('registry:read');

  useEffect(() => {
    loadStats();
    const interval = setInterval(loadStats, 30000); // 30秒刷新
    return () => clearInterval(interval);
  }, []);

  const loadStats = async () => {
    try {
      const [descriptors, registry, packs, entities] = await Promise.all([
        fetch('/api/descriptors').then(r => r.json()).catch(() => ({})),
        fetch('/api/registry').then(r => r.json()).catch(() => ({})),
        fetch('/api/packs/list').then(r => r.json()).catch(() => ({})),
        fetch('/api/entities/list').then(r => r.json()).catch(() => ({}))
      ]);

      setStats({
        totalFunctions: descriptors && typeof descriptors === 'object' ? Object.keys(descriptors).length : 0,
        activeFunctions: descriptors && typeof descriptors === 'object' ? Object.values(descriptors).filter((d: any) => d?.enabled).length : 0,
        runningJobs: 0, // TODO: 从job API获取
        availablePackages: packs?.packages && Array.isArray(packs.packages) ? packs.packages.length : 0,
        connectedAgents: registry?.agents && Array.isArray(registry.agents) ? registry.agents.filter((a: any) => a?.connected).length : 0,
        virtualObjects: entities?.entities && Array.isArray(entities.entities) ? entities.entities.length : 0
      });
    } catch (error) {
      console.error('Failed to load function stats:', error);
      // 如果网络完全失败，设置默认值
      setStats({
        totalFunctions: 0,
        activeFunctions: 0,
        runningJobs: 0,
        availablePackages: 0,
        connectedAgents: 0,
        virtualObjects: 0
      });
    }
  };

  const quickActionsMenu = (
    <Menu>
      <Menu.Item key="import-pack" icon={<PlusOutlined />}>
        导入组件包
      </Menu.Item>
      <Menu.Item key="create-function">
        创建新组件
      </Menu.Item>
      <Menu.Item key="create-virtual-object" icon={<ApartmentOutlined />}>
        创建虚拟对象
      </Menu.Item>
      <Menu.Item key="bulk-enable">
        批量启用/禁用
      </Menu.Item>
      <Menu.Item key="export-config">
        导出配置
      </Menu.Item>
    </Menu>
  );

  const renderTabTitle = (title: string, count?: number, color?: string) => (
    <Space>
      {title}
      {count !== undefined && (
        <Badge
          count={count}
          style={{ backgroundColor: color || '#1890ff' }}
          overflowCount={999}
        />
      )}
    </Space>
  );

  return (
    <div>
      <Card
        title={
          <Space>
            <ApartmentOutlined />
            组件管理
          </Space>
        }
        extra={
          <Space>
            <GameSelector />
            <Dropdown overlay={quickActionsMenu} placement="bottomRight">
              <Button icon={<SettingOutlined />}>
                快速操作
              </Button>
            </Dropdown>
          </Space>
        }
        bordered={false}
      >
        {/* 统计概览 */}
        <Card.Grid hoverable={false} style={{ width: '16.66%', textAlign: 'center' }}>
          <div>
            <div style={{ fontSize: 24, fontWeight: 'bold', color: '#1890ff' }}>
              {stats.totalFunctions}
            </div>
            <div style={{ color: '#666' }}>总函数数</div>
          </div>
        </Card.Grid>

        <Card.Grid hoverable={false} style={{ width: '16.66%', textAlign: 'center' }}>
          <div>
            <div style={{ fontSize: 24, fontWeight: 'bold', color: '#52c41a' }}>
              {stats.activeFunctions}
            </div>
            <div style={{ color: '#666' }}>活跃函数</div>
          </div>
        </Card.Grid>

        <Card.Grid hoverable={false} style={{ width: '16.66%', textAlign: 'center' }}>
          <div>
            <div style={{ fontSize: 24, fontWeight: 'bold', color: '#faad14' }}>
              {stats.runningJobs}
            </div>
            <div style={{ color: '#666' }}>运行中任务</div>
          </div>
        </Card.Grid>

        <Card.Grid hoverable={false} style={{ width: '16.66%', textAlign: 'center' }}>
          <div>
            <div style={{ fontSize: 24, fontWeight: 'bold', color: '#722ed1' }}>
              {stats.availablePackages}
            </div>
            <div style={{ color: '#666' }}>可用包数</div>
          </div>
        </Card.Grid>

        <Card.Grid hoverable={false} style={{ width: '16.66%', textAlign: 'center' }}>
          <div>
            <div style={{ fontSize: 24, fontWeight: 'bold', color: '#13c2c2' }}>
              {stats.connectedAgents}
            </div>
            <div style={{ color: '#666' }}>在线代理</div>
          </div>
        </Card.Grid>

        <Card.Grid hoverable={false} style={{ width: '16.66%', textAlign: 'center' }}>
          <div>
            <div style={{ fontSize: 24, fontWeight: 'bold', color: '#eb2f96' }}>
              {stats.virtualObjects}
            </div>
            <div style={{ color: '#666' }}>虚拟对象</div>
          </div>
        </Card.Grid>
      </Card>

      {/* 功能标签页 */}
      <Card style={{ marginTop: 16 }} bodyStyle={{ padding: 0 }}>
        <Tabs
          activeKey={activeTab}
          onChange={setActiveTab}
          size="large"
          tabBarStyle={{ margin: 0, paddingLeft: 24, paddingRight: 24 }}
        >
          <TabPane
            tab={renderTabTitle("组件工作台", stats.activeFunctions, '#52c41a')}
            key="workspace"
            icon={<FunctionOutlined />}
          >
            <div style={{ padding: 24 }}>
              <FunctionWorkspace />
            </div>
          </TabPane>

          {canViewRegistry && (
            <TabPane
              tab={renderTabTitle("注册表", stats.connectedAgents, '#13c2c2')}
              key="registry"
              icon={<DatabaseOutlined />}
            >
              <div style={{ padding: 24 }}>
                <RegistryDashboard />
              </div>
            </TabPane>
          )}

          {canManagePackages && (
            <TabPane
              tab={renderTabTitle("组件包", stats.availablePackages, '#722ed1')}
              key="packages"
              icon={<AppstoreOutlined />}
            >
              <div style={{ padding: 24 }}>
                <PackageCenter />
              </div>
            </TabPane>
          )}

          <TabPane
            tab={renderTabTitle("执行监控", stats.runningJobs, '#faad14')}
            key="monitor"
            icon={<MonitorOutlined />}
          >
            <div style={{ padding: 24 }}>
              <ExecutionMonitor />
            </div>
          </TabPane>

          <TabPane
            tab={renderTabTitle("虚拟对象", stats.virtualObjects, '#eb2f96')}
            key="virtual-objects"
            icon={<ApartmentOutlined />}
          >
            <div style={{ padding: 24 }}>
              <VirtualObjectManager />
            </div>
          </TabPane>
        </Tabs>
      </Card>
    </div>
  );
}