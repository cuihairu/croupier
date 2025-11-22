import React, { useEffect, useState, useMemo } from 'react';
import { PageContainer, Card, Row, Col, Divider, Space, Typography, Alert, Spin, Empty } from 'antd';
import { PlayCircleOutlined, FunctionOutlined, HistoryOutlined } from '@ant-design/icons';
import { history, useLocation } from '@umijs/max';
import { listDescriptors, invokeFunction, startJob, FunctionDescriptor } from '@/services/croupier';
import FunctionFormRenderer from '@/components/FunctionFormRenderer';
import FunctionListTable from '@/components/FunctionListTable';
import FunctionDetailPanel from '@/components/FunctionDetailPanel';
import FunctionCallHistory from '@/components/FunctionCallHistory';
import GameSelector from '@/components/GameSelector';

const { Title, Text } = Typography;

export default () => {
  const location = useLocation();
  const searchParams = new URLSearchParams(location.search);
  const initialFunctionId = searchParams.get('fid');

  const [functions, setFunctions] = useState<FunctionDescriptor[]>([]);
  const [selectedFunction, setSelectedFunction] = useState<FunctionDescriptor | null>(null);
  const [loading, setLoading] = useState(false);
  const [executing, setExecuting] = useState(false);
  const [gameId, setGameId] = useState<string>('');
  const [env, setEnv] = useState<string>('');
  const [executionResult, setExecutionResult] = useState<any>(null);
  const [executionError, setExecutionError] = useState<string>('');
  const [activeTab, setActiveTab] = useState<'invoke' | 'catalog' | 'history'>('invoke');

  // Fetch available functions
  useEffect(() => {
    const fetchFunctions = async () => {
      setLoading(true);
      try {
        const descriptors = await listDescriptors();
        setFunctions(descriptors || []);

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
  }, [initialFunctionId]);

  const handleFunctionSelect = (func: FunctionDescriptor) => {
    setSelectedFunction(func);
    setExecutionResult(null);
    setExecutionError('');
    setActiveTab('invoke');
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

  const handleRefreshHistory = () => {
    // This would trigger a refresh in the FunctionCallHistory component
    console.log('Refreshing call history...');
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

  const renderInvokeForm = () => {
    if (!selectedFunction) {
      return (
        <Card>
          <Empty
            image={Empty.PRESENTED_IMAGE_SIMPLE}
            description="请先选择要调用的函数"
          />
        </Card>
      );
    }

    return (
      <Row gutter={[16, 16]}>
        <Col span={14}>
          <FunctionFormRenderer
            schema={selectedFunction.params || { type: 'object', properties: {} }}
            onSubmit={handleExecute}
            loading={executing}
            submitText={executing ? '执行中...' : '执行函数'}
            resetText="重置参数"
            extra={
              <Space>
                <Button onClick={() => handleStartJob({})}>
                  作为任务执行
                </Button>
              </Space>
            }
          />
        </Col>
        <Col span={10}>
          <FunctionDetailPanel
            function={processedFunction!}
            showActions={false}
            compact={true}
          />
        </Col>
      </Row>
    );
  };

  const renderResult = () => {
    if (executionResult) {
      return (
        <Card title="执行结果" size="default">
          <pre style={{
            backgroundColor: '#f5f5f5',
            padding: 16,
            borderRadius: 6,
            maxHeight: 400,
            overflow: 'auto'
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
        />
      );
    }

    return null;
  };

  if (loading) {
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
      extra={
        <Space>
          <GameSelector value={gameId} onChange={setGameId} />
          {env && <Tag color="blue">环境: {env}</Tag>}
        </Space>
      }
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
        }
      ]}
      activeTabKey={activeTab}
      onTabChange={(key) => setActiveTab(key as any)}
    >
      {activeTab === 'invoke' && (
        <Space direction="vertical" style={{ width: '100%' }} size="large">
          {selectedFunction && (
            <Alert
              message={`当前函数: ${selectedFunction.display_name?.zh || selectedFunction.id}`}
              description={selectedFunction.summary?.zh || selectedFunction.description}
              type="info"
              showIcon
              closable
              onClose={() => setSelectedFunction(null)}
            />
          )}

          {renderInvokeForm()}

          {renderResult()}
        </Space>
      )}

      {activeTab === 'catalog' && (
        <FunctionListTable
          data={functions.map(func => ({
            id: func.id,
            version: func.version,
            enabled: true,
            display_name: func.display_name || { zh: func.id },
            summary: func.summary || { zh: func.description },
            tags: func.tags || [],
            category: func.category
          }))}
          loading={loading}
          onInvoke={handleFunctionSelect}
          onViewDetail={(func) => {
            setSelectedFunction(functions.find(f => f.id === func.id) || null);
            setActiveTab('invoke');
          }}
          showActions={{ view: true, invoke: true }}
        />
      )}

      {activeTab === 'history' && (
        <FunctionCallHistory
          gameId={gameId}
          onRefresh={handleRefreshHistory}
          onViewDetail={(call) => {
            console.log('View call detail:', call);
          }}
        />
      )}
    </PageContainer>
  );
};