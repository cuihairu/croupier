/**
 * ReportPageRenderer - 报表页面渲染器
 *
 * 渲染报表页面，包括：
 * - 查询表单
 * - 图表展示
 * - 数据表格
 * - 导出功能
 *
 * @module components/PageRenderer/ReportPageRenderer
 */

import React, { useState, useCallback } from 'react';
import {
  ProForm,
  ProFormText,
  ProFormTextArea,
  ProFormSelect,
  ProFormDigit,
  ProFormDatePicker,
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
  BarChartOutlined,
  PieChartOutlined,
  TableOutlined,
  DownloadOutlined,
  ReloadOutlined,
} from '@ant-design/icons';
import type {
  ReportPageSpec,
  FormFieldSpec,
  ChartSpec,
  PageFunctionBindingV2,
} from '@/types/dashboard-vnext';
import type { ProColumns } from '@ant-design/pro-components';

const { Text } = Typography;

// ---------------------------------------------------------------------------
// Props
// ---------------------------------------------------------------------------

export interface ReportPageRendererProps {
  /** 报表页面规格 */
  spec: ReportPageSpec;
  /** 页面绑定 */
  bindings: PageFunctionBindingV2[];
  /** 执行绑定函数 */
  onExecute: (bindingId: string, payload: unknown) => Promise<unknown>;
  /** 导出数据 */
  onExport?: (format: 'csv' | 'excel') => Promise<void>;
  /** 页面标题 */
  title?: string;
}

// ---------------------------------------------------------------------------
// 表单字段渲染
// ---------------------------------------------------------------------------

function renderFormField(field: FormFieldSpec): React.ReactNode {
  const label = field.label?.['zh-CN'] || field.label?.['en'] || field.key;
  const placeholder = field.placeholder?.['zh-CN'] || field.placeholder?.['en'];
  const required = field.required;
  const rules = required ? [{ required: true, message: `请输入${label}` }] : [];

  switch (field.widget) {
    case 'TextArea':
      return (
        <ProFormTextArea
          key={field.key}
          name={field.key}
          label={label}
          placeholder={placeholder}
          rules={rules}
        />
      );
    case 'InputNumber':
      return (
        <ProFormDigit
          key={field.key}
          name={field.key}
          label={label}
          placeholder={placeholder}
          rules={rules}
        />
      );
    case 'Select':
      return (
        <ProFormSelect
          key={field.key}
          name={field.key}
          label={label}
          placeholder={placeholder}
          rules={rules}
          options={field.enumOptions?.map((opt) => ({
            label: opt.label['zh-CN'] || opt.label['en'] || opt.value,
            value: opt.value,
          }))}
        />
      );
    case 'DateRange':
      return (
        <ProFormDatePicker.RangePicker
          key={field.key}
          name={field.key}
          label={label}
          placeholder={['开始日期', '结束日期']}
          rules={rules}
        />
      );
    default:
      return (
        <ProFormText
          key={field.key}
          name={field.key}
          label={label}
          placeholder={placeholder}
          rules={rules}
        />
      );
  }
}

// ---------------------------------------------------------------------------
// 图表占位符
// ---------------------------------------------------------------------------

const ChartPlaceholder: React.FC<{ chart: ChartSpec; data: unknown[] }> = ({ chart, data }) => {
  const getIcon = () => {
    switch (chart.type) {
      case 'line':
        return <LineChartOutlined style={{ fontSize: 48, color: '#1890ff' }} />;
      case 'bar':
        return <BarChartOutlined style={{ fontSize: 48, color: '#52c41a' }} />;
      case 'pie':
        return <PieChartOutlined style={{ fontSize: 48, color: '#faad14' }} />;
      default:
        return <LineChartOutlined style={{ fontSize: 48, color: '#1890ff' }} />;
    }
  };

  return (
    <Card title={chart.title['zh-CN'] || chart.title['en'] || chart.type}>
      <div style={{ textAlign: 'center', padding: '40px 0' }}>
        {getIcon()}
        <div style={{ marginTop: 16 }}>
          <Text type="secondary">
            {chart.type.toUpperCase()} 图表 - {data.length} 条数据
          </Text>
        </div>
        <div style={{ marginTop: 8 }}>
          <Text type="secondary" style={{ fontSize: 12 }}>
            X: {chart.xField || '-'} | Y: {chart.yField || '-'}
          </Text>
        </div>
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
  const [data, setData] = useState<unknown[]>([]);
  const [activeTab, setActiveTab] = useState('chart');

  // 查找主绑定
  const mainBinding = bindings.find((b) => b.usage === 'report') || bindings[0];

  // 处理查询
  const handleQuery = useCallback(
    async (values: unknown) => {
      if (!mainBinding) {
        message.error('未配置报表绑定');
        return;
      }

      setLoading(true);

      try {
        const response = await onExecute(mainBinding.id, values);
        const items = (response as any)?.items || (response as any)?.data || [];
        setData(Array.isArray(items) ? items : []);
        message.success('查询成功');
      } catch (error: any) {
        message.error('查询失败: ' + (error.message || '未知错误'));
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
      } catch (error: any) {
        message.error('导出失败: ' + (error.message || '未知错误'));
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
        <ProForm
          onFinish={handleQuery}
          submitter={{
            submitButtonProps: { loading },
            resetButtonProps: {
              onClick: () => setData([]),
            },
          }}
          layout="horizontal"
        >
          <Space wrap>
            {spec.queryForm.fields?.map(renderFormField)}
          </Space>
        </ProForm>
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
                            <ChartPlaceholder key={index} chart={chart} data={data} />
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
