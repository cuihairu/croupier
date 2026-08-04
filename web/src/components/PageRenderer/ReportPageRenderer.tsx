/**
 * ReportPageRenderer - 报表页面渲染器
 *
 * 渲染报表页面，包括：
 * - 查询表单
 * - 图表展示（使用 @ant-design/charts）
 * - 数据表格
 * - 导出功能
 *
 * @module components/PageRenderer/ReportPageRenderer
 */

import React, { useState, useCallback } from 'react';
import {
  ProTable,
} from '@ant-design/pro-components';
import {
  Card,
  Button,
  Space,
  message,
  Typography,
  Tabs,
  Empty,
} from 'antd';
import {
  LineChartOutlined,
  TableOutlined,
  DownloadOutlined,
  ReloadOutlined,
} from '@ant-design/icons';
import { Line, Column, Pie, Area } from '@ant-design/charts';
import SchemaFormRenderer from '@/components/SchemaFormRenderer';
import type {
  ReportPageSpec,
  ChartSpec,
  PageFunctionBinding,
  PageExecuteFn,
  JSONValue,
  FormValues,
} from '@/types/dashboard';
import type { ProColumns } from '@ant-design/pro-components';

const { Text } = Typography;

// ---------------------------------------------------------------------------
// Props
// ---------------------------------------------------------------------------

export interface ReportPageRendererProps {
  /** 报表页面规格 */
  spec: ReportPageSpec;
  /** 页面绑定 */
  bindings: PageFunctionBinding[];
  /** 执行绑定函数 */
  onExecute: PageExecuteFn;
  /** 导出数据 */
  onExport?: (format: 'csv' | 'excel') => Promise<void>;
  /** 页面标题 */
  title?: string;
}

// ---------------------------------------------------------------------------
// 图表渲染器
// ---------------------------------------------------------------------------

const ChartRenderer: React.FC<{ chart: ChartSpec; data: FormValues[] }> = ({ chart, data }) => {
  const title = chart.title['zh-CN'] || chart.title['en'] || chart.type;

  // 准备图表数据
  const chartData = data.map((item) => ({
    x: String(item[chart.xField || ''] || ''),
    y: Number(item[chart.yField || ''] || 0),
    series: String(item[chart.seriesField || ''] || ''),
  }));

  const commonConfig = {
    data: chartData,
    xField: 'x',
    yField: 'y',
    seriesField: chart.seriesField ? 'series' : undefined,
    smooth: true,
    animation: {
      appear: {
        animation: 'path-in',
        duration: 1000,
      },
    },
  };

  const renderChart = () => {
    switch (chart.type) {
      case 'line':
        return <Line {...commonConfig} />;
      case 'bar':
        return <Column {...commonConfig} />;
      case 'area':
        return <Area {...commonConfig} />;
      case 'pie':
        return (
          <Pie
            data={chartData}
            angleField="y"
            colorField="x"
            radius={0.8}
            label={{
              type: 'outer',
              content: '{name}: {percentage}',
            }}
            interactions={[
              { type: 'element-active' },
            ]}
          />
        );
      default:
        return <Line {...commonConfig} />;
    }
  };

  return (
    <Card title={title} style={{ marginBottom: 16 }}>
      <div style={{ height: 400 }}>
        {data.length > 0 ? (
          renderChart()
        ) : (
          <div style={{ textAlign: 'center', padding: '100px 0' }}>
            <Text type="secondary">暂无数据</Text>
          </div>
        )}
      </div>
    </Card>
  );
};

// ---------------------------------------------------------------------------
// ReportPageRenderer 组件
// ---------------------------------------------------------------------------

const ReportPageRenderer: React.FC<ReportPageRendererProps> = ({
  spec,
  bindings,
  onExecute,
  onExport,
  title,
}) => {
  const [loading, setLoading] = useState(false);
  const [data, setData] = useState<FormValues[]>([]);
  const [activeTab, setActiveTab] = useState('chart');

  // 查找主绑定
  const mainBinding = bindings.find((b) => b.usage === 'report') || bindings[0];

  // 处理查询
  const handleQuery = useCallback(
    async (values: FormValues) => {
      if (!mainBinding) {
        message.error('未配置报表绑定');
        return;
      }

      setLoading(true);

      try {
        const response = await onExecute(mainBinding.id, { form: values });
        const responseData = response.data as Record<string, JSONValue> | undefined;
        const items = (responseData?.items as FormValues[]) || (Array.isArray(responseData) ? responseData as FormValues[] : []);
        setData(Array.isArray(items) ? items : []);
        message.success('查询成功');
      } catch (error) {
        const msg = error instanceof Error ? error.message : '未知错误';
        message.error('查询失败: ' + msg);
      } finally {
        setLoading(false);
      }
    },
    [mainBinding, onExecute]
  );

  // 处理导出
  const handleExport = useCallback(
    async (format: 'csv' | 'excel') => {
      if (!onExport) {
        message.warning('导出功能未配置');
        return;
      }

      try {
        await onExport(format);
        message.success('导出成功');
      } catch (error) {
        const msg = error instanceof Error ? error.message : '未知错误';
        message.error('导出失败: ' + msg);
      }
    },
    [onExport]
  );

  // 构建表格列
  const columns: ProColumns[] = spec.dataset.dimensions.map((dim) => ({
    title: dim.title['zh-CN'] || dim.title['en'] || dim.key,
    dataIndex: dim.key,
    key: dim.key,
    valueType: dim.dataType === 'number' ? 'digit' : dim.dataType === 'date' ? 'date' : 'text',
  }));

  // 添加指标列
  spec.dataset.metrics.forEach((metric) => {
    columns.push({
      title: metric.title['zh-CN'] || metric.title['en'] || metric.key,
      dataIndex: metric.key,
      key: metric.key,
      valueType: 'digit',
      render: (_, record) => {
        const value = record[metric.key];
        if (value === undefined || value === null) {
          return '-';
        }
        if (metric.format === 'percent') {
          return `${(value * 100).toFixed(2)}%`;
        }
        if (metric.format === 'currency') {
          return `¥${value.toLocaleString()}`;
        }
        return value.toLocaleString();
      },
    });
  });

  return (
    <div>
      {/* 查询表单 */}
      <Card title={title || '报表查询'}>
        <SchemaFormRenderer
          spec={spec.queryForm}
          onFinish={handleQuery}
          disabled={loading}
        />
        <Button style={{ marginTop: 12 }} onClick={() => setData([])}>
          清空结果
        </Button>
      </Card>

      {/* 数据展示 */}
      {data.length > 0 && (
        <Card
          title="数据展示"
          style={{ marginTop: 16 }}
          extra={
            <Space>
              {spec.exportable && (
                <>
                  <Button
                    icon={<DownloadOutlined />}
                    onClick={() => handleExport('csv')}
                  >
                    导出 CSV
                  </Button>
                  <Button
                    icon={<DownloadOutlined />}
                    onClick={() => handleExport('excel')}
                  >
                    导出 Excel
                  </Button>
                </>
              )}
              <Button
                icon={<ReloadOutlined />}
                onClick={() => setData([])}
              >
                清空
              </Button>
            </Space>
          }
        >
          <Tabs
            activeKey={activeTab}
            onChange={setActiveTab}
            items={[
              ...(spec.charts && spec.charts.length > 0
                ? [
                    {
                      key: 'chart',
                      label: (
                        <span>
                          <LineChartOutlined />
                          图表
                        </span>
                      ),
                      children: (
                        <Space direction="vertical" style={{ width: '100%' }}>
                          {spec.charts.map((chart, index) => (
                            <ChartRenderer key={index} chart={chart} data={data} />
                          ))}
                        </Space>
                      ),
                    },
                  ]
                : []),
              {
                key: 'table',
                label: (
                  <span>
                    <TableOutlined />
                    表格
                  </span>
                ),
                children: (
                  <ProTable
                    columns={columns}
                    dataSource={data}
                    rowKey={(record, index) => index?.toString() || '0'}
                    search={false}
                    options={false}
                    pagination={{
                      pageSize: 20,
                      showSizeChanger: true,
                      showTotal: (total) => `共 ${total} 条`,
                    }}
                    scroll={{ x: 'max-content' }}
                  />
                ),
              },
            ]}
          />
        </Card>
      )}

      {/* 空状态 */}
      {!loading && data.length === 0 && (
        <Card style={{ marginTop: 16 }}>
          <Empty description="请先查询数据" />
        </Card>
      )}
    </div>
  );
};

export default ReportPageRenderer;
