import React, { useState, useEffect } from 'react';
import { Card, Tabs, Badge, Space, Button, Dropdown, Modal, Form, Input, message, Select } from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import {
  FunctionOutlined,
  DatabaseOutlined,
  AppstoreOutlined,
  MonitorOutlined,
  SettingOutlined,
  PlusOutlined,
  ApartmentOutlined
} from '@ant-design/icons';
import { useModel, useIntl } from '@umijs/max';
import FunctionWorkspace from './components/FunctionWorkspace';
import RegistryDashboard from './components/RegistryDashboard';
import PackageCenter from './components/PackageCenter';
import ExecutionMonitor from './components/ExecutionMonitor';
import VirtualObjectManager from './components/VirtualObjectManager';
import GameSelector from '@/components/GameSelector';
import { reloadPacks, listPacks } from '@/services/croupier/packs';
import { createEntity, listEntities } from '@/services/croupier/entities';
import { fetchRegistry, listDescriptors } from '@/services/croupier';
import { apiUrl } from '@/utils/api';

const QUICK_ACTIONS = {
  importPack: {
    title: '导入组件包',
    description: '上传一个新的组件包或指定远程地址进行导入。',
  },
  createFunction: {
    title: '创建新组件',
    description: '快速定义一个新的函数组件并保存草稿。',
  },
  createVirtualObject: {
    title: '创建虚拟对象',
    description: '为游戏世界创建一个新的虚拟对象实体。',
  },
  bulkEnable: {
    title: '批量启用/禁用',
    description: '选择一个范围内的函数并统一切换状态。',
  },
  exportConfig: {
    title: '导出配置',
    description: '导出当前组件配置，便于备份或迁移。',
  },
} as const;

type QuickActionKey = keyof typeof QUICK_ACTIONS;

interface ComponentStats {
  totalFunctions: number;
  activeFunctions: number;
  runningJobs: number;
  availablePackages: number;
  connectedAgents: number;
  virtualObjects: number;
}

export default function ComponentManagement() {
  const intl = useIntl();
  const [activeTab, setActiveTab] = useState('workspace');
  const [unauthorized, setUnauthorized] = useState(false);
  const [initError, setInitError] = useState<string | null>(null);
  const [stats, setStats] = useState<ComponentStats>({
    totalFunctions: 0,
    activeFunctions: 0,
    runningJobs: 0,
    availablePackages: 0,
    connectedAgents: 0,
    virtualObjects: 0
  });
  const refreshTimer = React.useRef<NodeJS.Timeout>();
  const mounted = React.useRef(true);

  const { initialState } = useModel('@@initialState');
  const currentUser = (initialState as any)?.currentUser;
  const roles = currentUser?.access?.split(',') || [];

  // 权限检查
  const canManagePackages = roles.includes('*') || roles.includes('functions:manage');
  const canViewRegistry = roles.includes('*') || roles.includes('registry:read');

  useEffect(() => {
    loadStats();
    refreshTimer.current = setInterval(loadStats, 30000); // 30秒刷新
    return () => {
      mounted.current = false;
      if (refreshTimer.current) clearInterval(refreshTimer.current);
    };
  }, []);

  const loadStats = async () => {
    if (!currentUser) {
      setUnauthorized(true);
      setStats({
        totalFunctions: 0,
        activeFunctions: 0,
        runningJobs: 0,
        availablePackages: 0,
        connectedAgents: 0,
        virtualObjects: 0
      });
      return;
    }
    if (unauthorized) return;

    try {
      const [descriptors, registry, packs, entities] = await Promise.all([
        listDescriptors().catch(() => []),
        fetchRegistry().catch(() => ({} as any)),
        listPacks().catch(() => ({} as any)),
        listEntities().catch(() => []),
      ]);

      const entityCount = Array.isArray(entities) ? entities.length : 0;
      const descriptorList = Array.isArray(descriptors) ? descriptors : [];
      const packCount =
        packs?.packages && Array.isArray(packs.packages) ? packs.packages.length : packs?.counts?.descriptors || 0;
      const connectedAgents =
        registry?.agents && Array.isArray(registry.agents)
          ? registry.agents.filter((a: any) => a?.connected).length
          : 0;

      setStats({
        totalFunctions: descriptorList.length,
        activeFunctions: descriptorList.filter((d: any) => d?.enabled).length,
        runningJobs: 0, // TODO: 从job API获取
        availablePackages: packCount,
        connectedAgents,
        virtualObjects: entityCount,
      });
      if (!mounted.current) return;
      setInitError(null);
    } catch (error: any) {
      if (error?.response?.status === 401 || error?.status === 401) {
        if (!mounted.current) return;
        setUnauthorized(true);
        if (refreshTimer.current) clearInterval(refreshTimer.current);
        message.warning('未授权，已停止自动刷新组件统计');
        return;
      }
      console.error('Failed to load function stats:', error);
      if (!initError) setInitError('组件统计加载失败');
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

  const [actionModal, setActionModal] = useState<{ key: QuickActionKey } | null>(null);
  const [actionSubmitting, setActionSubmitting] = useState(false);
  const [actionForm] = Form.useForm();

  const handleQuickAction = (key: QuickActionKey) => {
    actionForm.resetFields();
    if (key === 'createFunction') {
      setActiveTab('workspace');
    }
    setActionModal({ key });
  };

  const handleActionSubmit = async () => {
    if (!actionModal) return;
    try {
      const values = await actionForm.validateFields();
      setActionSubmitting(true);
      if (actionModal.key === 'importPack') {
        await reloadPacks();
        message.success('组件包已重新加载');
        await loadStats();
      } else if (actionModal.key === 'createVirtualObject') {
        const game_id = localStorage.getItem('game_id') || undefined;
        const env = localStorage.getItem('env') || undefined;
        await createEntity({ name: values.objectName, description: values.description }, { game_id, env });
        message.success('虚拟对象已创建');
        await loadStats();
      } else if (actionModal.key === 'exportConfig') {
        const link = document.createElement('a');
        link.href = apiUrl('/api/packs/export');
        link.target = '_blank';
        link.rel = 'noopener';
        link.click();
        message.success('开始导出配置');
      } else if (actionModal.key === 'createFunction') {
        message.info('已跳转到组件工作台，可在下方创建新组件');
      } else if (actionModal.key === 'bulkEnable') {
        message.info('批量启用/禁用暂未开放，请在组件包或函数列表中操作');
      }
      setActionModal(null);
    } catch (error) {
      if ((error as any)?.errorFields) {
        return; // 表单校验错误
      }
      message.error('操作失败，请稍后重试');
    } finally {
      setActionSubmitting(false);
    }
  };

  const closeActionModal = () => {
    setActionModal(null);
  };

  const renderActionFields = () => {
    if (!actionModal) return null;
    switch (actionModal.key) {
      case 'importPack':
        return (
          <>
            <Form.Item
              label="包名称"
              name="packName"
              rules={[{ required: true, message: '请输入包名称' }]}
            >
              <Input placeholder="如：core-pack" />
            </Form.Item>
            <Form.Item
              label="来源"
              name="source"
              rules={[{ required: true, message: '请输入或选择来源' }]}
            >
              <Input placeholder="https://example.com/packs/core-pack.tar.gz" />
            </Form.Item>
          </>
        );
      case 'createFunction':
        return (
          <>
            <Form.Item
              label="函数 ID"
              name="functionId"
              rules={[{ required: true, message: '请输入函数 ID' }]}
            >
              <Input placeholder="inventory.adjust_stock" />
            </Form.Item>
            <Form.Item
              label="运行时"
              name="runtime"
              rules={[{ required: true, message: '请选择运行时' }]}
            >
              <Select
                options={[
                  { label: 'Go', value: 'go' },
                  { label: 'Node.js', value: 'node' },
                  { label: 'Python', value: 'python' },
                ]}
                placeholder="请选择运行时"
              />
            </Form.Item>
          </>
        );
      case 'createVirtualObject':
        return (
          <>
            <Form.Item
              label="对象名称"
              name="objectName"
              rules={[{ required: true, message: '请输入对象名称' }]}
            >
              <Input placeholder="赛季奖励宝箱" />
            </Form.Item>
            <Form.Item label="描述" name="description">
              <Input.TextArea rows={3} placeholder="用于赛季奖励的虚拟对象定义" />
            </Form.Item>
          </>
        );
      case 'bulkEnable':
        return (
          <>
            <Form.Item
              label="操作类型"
              name="action"
              rules={[{ required: true, message: '请选择操作类型' }]}
            >
              <Select
                options={[
                  { label: '批量启用', value: 'enable' },
                  { label: '批量禁用', value: 'disable' },
                ]}
                placeholder="选择执行的动作"
              />
            </Form.Item>
            <Form.Item
              label="函数范围"
              name="scope"
              rules={[{ required: true, message: '请填写函数范围' }]}
            >
              <Input.TextArea rows={3} placeholder="输入函数 ID 列表，使用换行分隔" />
            </Form.Item>
          </>
        );
      case 'exportConfig':
        return (
          <>
            <Form.Item
              label="导出格式"
              name="format"
              initialValue="json"
              rules={[{ required: true, message: '请选择导出格式' }]}
            >
              <Select
                options={[
                  { label: 'JSON', value: 'json' },
                  { label: 'YAML', value: 'yaml' },
                ]}
              />
            </Form.Item>
            <Form.Item label="包含内容" name="includes">
              <Input placeholder="如：functions,packages,entities" />
            </Form.Item>
          </>
        );
      default:
        return null;
    }
  };

  const quickActionsMenu = {
    items: [
      { key: 'importPack', icon: <PlusOutlined />, label: '导入组件包' },
      { key: 'createFunction', label: '创建新组件' },
      { key: 'createVirtualObject', icon: <ApartmentOutlined />, label: '创建虚拟对象' },
      { key: 'bulkEnable', label: '批量启用/禁用' },
      { key: 'exportConfig', label: '导出配置' },
    ],
    onClick: ({ key }: { key: string }) => handleQuickAction(key as QuickActionKey),
  };

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
    <PageContainer>
      <Card
        title={
          <Space>
            <ApartmentOutlined />
            {intl.formatMessage({ id: 'pages.component.management.title' } || '组件管理')}
          </Space>
        }
        extra={
          <Space>
            <GameSelector />
            <Dropdown menu={quickActionsMenu} placement="bottomRight" trigger={['click']}>
              <Button icon={<SettingOutlined />}>
                {intl.formatMessage({ id: 'pages.component.management.quick.actions' } || '快速操作')}
              </Button>
            </Dropdown>
          </Space>
        }
        variant="bordered"
      >
        {/* 统计概览 */}
        <Card.Grid hoverable={false} style={{ width: '16.66%', textAlign: 'center' }}>
          <div>
            <div style={{ fontSize: 24, fontWeight: 'bold', color: '#1890ff' }}>
              {stats.totalFunctions}
            </div>
            <div style={{ color: '#666' }}>{intl.formatMessage({ id: 'pages.component.management.total.functions' })}</div>
          </div>
        </Card.Grid>

        <Card.Grid hoverable={false} style={{ width: '16.66%', textAlign: 'center' }}>
          <div>
            <div style={{ fontSize: 24, fontWeight: 'bold', color: '#52c41a' }}>
              {stats.activeFunctions}
            </div>
            <div style={{ color: '#666' }}>{intl.formatMessage({ id: 'pages.component.management.active.functions' })}</div>
          </div>
        </Card.Grid>

        <Card.Grid hoverable={false} style={{ width: '16.66%', textAlign: 'center' }}>
          <div>
            <div style={{ fontSize: 24, fontWeight: 'bold', color: '#faad14' }}>
              {stats.runningJobs}
            </div>
            <div style={{ color: '#666' }}>{intl.formatMessage({ id: 'pages.component.management.running.jobs' })}</div>
          </div>
        </Card.Grid>

        <Card.Grid hoverable={false} style={{ width: '16.66%', textAlign: 'center' }}>
          <div>
            <div style={{ fontSize: 24, fontWeight: 'bold', color: '#722ed1' }}>
              {stats.availablePackages}
            </div>
            <div style={{ color: '#666' }}>{intl.formatMessage({ id: 'pages.component.management.available.packages' })}</div>
          </div>
        </Card.Grid>

        <Card.Grid hoverable={false} style={{ width: '16.66%', textAlign: 'center' }}>
          <div>
            <div style={{ fontSize: 24, fontWeight: 'bold', color: '#13c2c2' }}>
              {stats.connectedAgents}
            </div>
            <div style={{ color: '#666' }}>{intl.formatMessage({ id: 'pages.component.management.connected.agents' })}</div>
          </div>
        </Card.Grid>

        <Card.Grid hoverable={false} style={{ width: '16.66%', textAlign: 'center' }}>
          <div>
            <div style={{ fontSize: 24, fontWeight: 'bold', color: '#eb2f96' }}>
              {stats.virtualObjects}
            </div>
            <div style={{ color: '#666' }}>{intl.formatMessage({ id: 'pages.component.management.virtual.objects' })}</div>
          </div>
        </Card.Grid>
      </Card>

      {unauthorized && (
        <Card style={{ marginTop: 16 }} type="inner" styles={{ header: { fontWeight: 600 } }} title="未授权" variant="bordered">
          <p>无法访问部分接口（如注册表、组件包列表），已停止自动刷新。请登录或联系管理员获取权限。</p>
        </Card>
      )}

      {/* 功能标签页 */}
      <Card style={{ marginTop: 16 }} styles={{ body: { padding: 0 } }} variant="bordered">
        <Tabs
          activeKey={activeTab}
          onChange={setActiveTab}
          size="large"
          tabBarStyle={{ margin: 0, paddingLeft: 24, paddingRight: 24 }}
          items={[
            {
              key: 'workspace',
              icon: <FunctionOutlined />,
              label: renderTabTitle("组件工作台", stats.activeFunctions, '#52c41a'),
              children: (
                <div style={{ padding: 24 }}>
                  <FunctionWorkspace />
                </div>
              ),
            },
            ...(canViewRegistry ? [{
              key: 'registry',
              icon: <DatabaseOutlined />,
              label: renderTabTitle("注册表", stats.connectedAgents, '#13c2c2'),
              children: (
                <div style={{ padding: 24 }}>
                  <RegistryDashboard />
                </div>
              ),
            }] : []),
            ...(canManagePackages ? [{
              key: 'packages',
              icon: <AppstoreOutlined />,
              label: renderTabTitle("组件包", stats.availablePackages, '#722ed1'),
              children: (
                <div style={{ padding: 24 }}>
                  <PackageCenter />
                </div>
              ),
            }] : []),
            {
              key: 'monitor',
              icon: <MonitorOutlined />,
              label: renderTabTitle("执行监控", stats.runningJobs, '#faad14'),
              children: (
                <div style={{ padding: 24 }}>
                  <ExecutionMonitor />
                </div>
              ),
            },
            {
              key: 'virtual-objects',
              icon: <ApartmentOutlined />,
              label: renderTabTitle("虚拟对象", stats.virtualObjects, '#eb2f96'),
              children: (
                <div style={{ padding: 24 }}>
                  <VirtualObjectManager />
                </div>
              ),
            },
          ]}
        />
      </Card>
      <Modal
        title={actionModal ? QUICK_ACTIONS[actionModal.key].title : ''}
        open={!!actionModal}
        onOk={handleActionSubmit}
        confirmLoading={actionSubmitting}
        onCancel={closeActionModal}
        destroyOnHidden
      >
        {actionModal && (
          <>
            <p style={{ color: '#666' }}>{QUICK_ACTIONS[actionModal.key].description}</p>
            <Form form={actionForm} layout="vertical" preserve={false}>
              {renderActionFields()}
            </Form>
          </>
        )}
      </Modal>
    </PageContainer>
  );
}
