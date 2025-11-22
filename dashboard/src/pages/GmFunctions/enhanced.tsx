import React, { useEffect, useState, useMemo } from 'react';
import { PageContainer, Card, Row, Col, Divider, Space, Typography, Alert, Spin, Tabs, Button } from 'antd';
import {
  PlayCircleOutlined,
  FunctionOutlined,
  HistoryOutlined,
  InfoCircleOutlined,
  SyncOutlined,
  SettingOutlined
} from '@ant-design/icons';
import { useLocation, history } from '@umijs/max';
import { listDescriptors, invokeFunction, startJob, FunctionDescriptor } from '@/services/croupier';
import GameSelector from '@/components/GameSelector';
import {
  FunctionFormRenderer,
  FunctionDetailPanel,
  FunctionCallHistory,
  FunctionListTable
} from '@/components/FunctionComponents';
import { getFunctionSummary } from '@/services/croupier/functions-enhanced';

const { Title, Text } = Typography;

export default function EnhancedGmFunctions() {
  const location = useLocation();
  const searchParams = new URLSearchParams(location.search);
  const initialFunctionId = searchParams.get('fid');

  const [functions, setFunctions] = useState<FunctionDescriptor[]>([]);
  const [functionSummaries, setFunctionSummaries] = useState<any[]>([]);
  const [selectedFunction, setSelectedFunction] = useState<FunctionDescriptor | null>(null);
  const [loading, setLoading] = useState(false);
  const [executing, setExecuting] = useState(false);
  const [gameId, setGameId] = useState<string>('');
  const [env, setEnv] = useState<string>('');
  const [executionResult, setExecutionResult] = useState<any>(null);
  const [executionError, setExecutionError] = useState<string>('');
  const [activeTab, setActiveTab] = useState<'invoke' | 'catalog' | 'history' | 'settings'>('invoke');

  // Fetch available functions
  useEffect(() => {
    const fetchFunctions = async () => {
      setLoading(true);
      try {
        const [descriptors, summaries] = await Promise.all([
          listDescriptors(),
          getFunctionSummary({ game_id: gameId, env })
        ]);

        setFunctions(descriptors || []);
        setFunctionSummaries(summaries || []);

        // Auto-select function if provided in URL
        if (initialFunctionId) {
          const func = descriptors?.find(f => f.id === initialFunctionId);
          if (func) {
            setSelectedFunction(func);
          }
        }
      } catch (error) {
        console.error('Failed to fetch functions:', error);
      } finally {
        setLoading(false);
      }
    };

    fetchFunctions();
  }, [initialFunctionId, gameId, env]);

  const handleFunctionSelect = (func: FunctionDescriptor) => {
    setSelectedFunction(func);
    setExecutionResult(null);
    setExecutionError('');
    setActiveTab('invoke');
    // Update URL
    const newUrl = new URL(window.location.href);
    newUrl.searchParams.set('fid', func.id);
    window.history.replaceState({}, '', newUrl.toString());
  };

  const handleExecute = async (values: any) => {
    if (!selectedFunction) return;

    setExecuting(true);
    setExecutionResult(null);
    setExecutionError('');

    try {
      const result = await invokeFunction(
        selectedFunction.id,
        values,
        { game_id: gameId, env }
      );
      setExecutionResult(result);
    } catch (error: any) {
      setExecutionError(error?.message || '执行失败');
    } finally {
      setExecuting(false);
    }
  };

  const handleStartJob = async (values: any) => {
    if (!selectedFunction) return;

    setExecuting(true);
    setExecutionResult(null);
    setExecutionError('');

    try {
      const result = await startJob(
        selectedFunction.id,
        values,
        { game_id: gameId, env }
      );
      setExecutionResult(result);
    } catch (error: any) {
      setExecutionError(error?.message || '启动任务失败');
    } finally {
      setExecuting(false);
    }
  };

  const handleRefresh = async () => {
    setLoading(true);
    try {
      const [descriptors, summaries] = await Promise.all([
        listDescriptors(),
        getFunctionSummary({ game_id: gameId, env })
      ]);

      setFunctions(descriptors || []);
      setFunctionSummaries(summaries || []);
    } catch (error) {
      console.error('Failed to refresh functions:', error);
    } finally {
      setLoading(false);
    }
  };

  const processedFunction = useMemo(() => {
    if (!selectedFunction) return null;
    return {
      id: selectedFunction.id,
      version: selectedFunction.version,
      enabled: true,
      display_name: selectedFunction.display_name || { zh: selectedFunction.id },
      summary: selectedFunction.summary || { zh: selectedFunction.description },
      description: { zh: selectedFunction.description },
      tags: selectedFunction.tags || [],
      category: selectedFunction.category,
      params: selectedFunction.params,
      outputs: selectedFunction.outputs
    };
  }, [selectedFunction]);

  const renderInvokeTab = () => {
    if (!selectedFunction) {
      return (
        <Card>
          <div style={{ textAlign: 'center', padding: '60px 0' }}>
            <FunctionOutlined style={{ fontSize: 48, color: '#d9d9d9', marginBottom: 16 }} />
            <Title level={4} type="secondary">请选择要调用的函数</Title>
            <Text type="secondary">从左侧目录中选择一个函数开始调用</Text>
            <Button
              type="primary"
              style={{ marginTop: 16 }}
              onClick={() => setActiveTab('catalog')}
            >
              浏览函数目录
            </Button>
          </div>
        </Card>
      );
    }

    return (
      <Row gutter={[16, 16]}>
        <Col span={16}>
          <FunctionFormRenderer
            schema={selectedFunction.params || { type: 'object', properties: {} }}
            onSubmit={handleExecute}
            onChange={() => {
              setExecutionResult(null);
              setExecutionError('');
            }}
            loading={executing}
            submitText={executing ? '执行中...' : '执行函数'}
            resetText="重置参数"
            extra={
              <Space>
                <Button onClick={() => handleStartJob({})} loading={executing}>
                  作为任务执行
                </Button>
              </Space>
            }
          />
        </Col>
        <Col span={8}>
          <FunctionDetailPanel
            function={processedFunction!}
            showActions={false}
            compact={true}
          />
        </Col>
      </Row>
    );
  };

  const renderCatalogTab = () => {
    return (
      <FunctionListTable
        data={functionSummaries}
        loading={loading}
        onInvoke={handleFunctionSelect}
        onViewDetail={(func) => {
          const descriptor = functions.find(f => f.id === func.id);
          if (descriptor) {
            setSelectedFunction(descriptor);
            setActiveTab('invoke');
          }
        }}
        onRefresh={handleRefresh}
        showActions={{ view: true, invoke: true }}
        pagination={{ pageSize: 15 }}
        searchable={true}
        filters={true}
      />
    );
  };

  const renderHistoryTab = () => {
    return (
      <FunctionCallHistory
        gameId={gameId}
        functionId={selectedFunction?.id}
        onRefresh={() => console.log('Refreshing call history...')}
        onViewDetail={(call) => {
          console.log('View call detail:', call);
        }}
        onRerun={(call) => {
          console.log('Rerun call:', call);
        }}
        limit={30}
        autoRefresh={true}
        refreshInterval={30000}
      />
    );
  };

  const renderSettingsTab = () => {
    return (
      <Row gutter={[16, 16]}>
        <Col span={12}>
          <Card title="执行设置" size="small">
            <Space direction="vertical" style={{ width: '100%' }}>
              <div>
                <Text strong>默认超时时间</Text>
                <div style={{ marginTop: 8 }}>
                  <Text type="secondary">5分钟</Text>
                </div>
              </div>
              <div>
                <Text strong>重试次数</Text>
                <div style={{ marginTop: 8 }}>
                  <Text type="secondary">3次</Text>
                </div>
              </div>
              <div>
                <Text strong>自动保存参数</Text>
                <div style={{ marginTop: 8 }}>
                  <Text type="secondary">已启用</Text>
                </div>
              </div>
            </Space>
          </Card>
        </Col>
        <Col span={12}>
          <Card title="显示设置" size="small">
            <Space direction="vertical" style={{ width: '100%' }}>
              <div>
                <Text strong>主题</Text>
                <div style={{ marginTop: 8 }}>
                  <Text type="secondary">跟随系统</Text>
                </div>
              </div>
              <div>
                <Text strong>语言</Text>
                <div style={{ marginTop: 8 }}>
                  <Text type="secondary">简体中文</Text>
                </div>
              </div>
              <div>
                <Text strong>时区</Text>
                <div style={{ marginTop: 8 }}>
                  <Text type="secondary">Asia/Shanghai</Text>
                </div>
              </div>
            </Space>
          </Card>
        </Col>
      </Row>
    );
  };

  const renderResult = () => {
    if (executionResult) {
      return (
        <Card title="执行结果" style={{ marginTop: 16 }}>
          <pre style={{
            backgroundColor: '#f5f5f5',
            padding: 16,
            borderRadius: 6,
            maxHeight: 400,
            overflow: 'auto',
            fontSize: '12px'
          }}>
            {JSON.stringify(executionResult, null, 2)}
          </pre>
        </Card>
      );
    }

    if (executionError) {
      return (
        <Alert
          message="执行错误"
          description={executionError}
          type="error"
          showIcon
          closable
          onClose={() => setExecutionError('')}
          style={{ marginTop: 16 }}
        />
      );
    }

    return null;
  };

  if (loading && functions.length === 0) {
    return (
      <PageContainer title="函数调用">
        <div style={{ textAlign: 'center', padding: '100px 0' }}>
          <Spin size="large" />
        </div>
      </PageContainer>
    );
  }

  return (
    <PageContainer
      title="函数调用"
      subTitle="执行系统函数并查看结果"
      extra={[
        <GameSelector key="game-selector" value={gameId} onChange={setGameId} />,
        <Button
          key="refresh"
          icon={<SyncOutlined />}
          onClick={handleRefresh}
          loading={loading}
        >
          刷新
        </Button>,
        env && <Tag key="env-tag" color="blue">环境: {env}</Tag>
      ]}
      tabList={[
        {
          key: 'invoke',
          tab: (
            <Space>
              <PlayCircleOutlined />
              函数调用
            </Space>
          )
        },
        {
          key: 'catalog',
          tab: (
            <Space>
              <FunctionOutlined />
              函数目录
            </Space>
          )
        },
        {
          key: 'history',
          tab: (
            <Space>
              <HistoryOutlined />
              调用历史
            </Space>
          )
        },
        {
          key: 'settings',
          tab: (
            <Space>
              <SettingOutlined />
              设置
            </Space>
          )
        }
      ]}
      activeTabKey={activeTab}
      onTabChange={(key) => setActiveTab(key as any)}
    >
      {selectedFunction && activeTab === 'invoke' && (
        <Alert
          message={`当前函数: ${selectedFunction.display_name?.zh || selectedFunction.id}`}
          description={selectedFunction.summary?.zh || selectedFunction.description}
          type="info"
          showIcon
          closable
          onClose={() => setSelectedFunction(null)}
          style={{ marginBottom: 16 }}
        />
      )}

      {activeTab === 'invoke' && (
        <Space direction="vertical" style={{ width: '100%' }} size="middle">
          {renderInvokeTab()}
          {renderResult()}
        </Space>
      )}

      {activeTab === 'catalog' && renderCatalogTab()}
      {activeTab === 'history' && renderHistoryTab()}
      {activeTab === 'settings' && renderSettingsTab()}
    </PageContainer>
  );
}