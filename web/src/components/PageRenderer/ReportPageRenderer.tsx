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

import React, { useState, useCallback, useRef } from 'react';
import { FormattedMessage, useIntl } from '@umijs/max';
import { ProTable } from '@ant-design/pro-components';
import { App, Card, Button, Space, Typography, Tabs, Empty, Result } from 'antd';
import {
  LineChartOutlined,
  TableOutlined,
  DownloadOutlined,
  ReloadOutlined,
} from '@ant-design/icons';
import { Line, Column, Pie, Area } from '@ant-design/charts';
import SchemaFormRenderer from '@/components/SchemaFormRenderer';
import { getPageStateArray, mergePageState, outputPatchFromResult } from './runtime';
import type {
  ReportPageSpec,
  ChartSpec,
  PageFunctionBinding,
  PageExecuteFn,
  FormValues,
} from '@/types/dashboard';
import type { ProColumns } from '@ant-design/pro-components';
import { extractErrorMessage } from '@/utils/errors';
import { localizedText } from '@/utils/localizedText';
import { exportToCSV } from '@/utils/export';

const { Text } = Typography;

function downloadDatasetCsv(
  rows: FormValues[],
  columns: Array<{ key: string; title: string }>,
): void {
  const toCell = (value: FormValues[string] | undefined): string | number | boolean | null => {
    if (value === null || value === undefined) return '';
    if (typeof value === 'object') return JSON.stringify(value);
    return value;
  };
  exportToCSV('report.csv', [
    columns.map((column) => column.title),
    ...rows.map((row) => columns.map((column) => toCell(row[column.key]))),
  ]);
}

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
  /** 预览模式只展示页面结构，禁止触发真实函数执行 */
  preview?: boolean;
  /** 导出数据 */
  onExport?: (format: 'csv' | 'excel') => Promise<void>;
  /** 页面标题 */
  title?: string;
}

// ---------------------------------------------------------------------------
// 图表渲染器
// ---------------------------------------------------------------------------

const ChartRenderer: React.FC<{ chart: ChartSpec; data: FormValues[] }> = ({ chart, data }) => {
  const title = localizedText(chart.title, 'zh-CN', chart.type);

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
            interactions={[{ type: 'element-active' }]}
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
            <Text type="secondary">
              <FormattedMessage
                id="component.pageRenderer.reportPage.chart.empty"
                defaultMessage="暂无数据"
              />
            </Text>
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
  preview = false,
  onExport,
  title,
}) => {
  const { message } = App.useApp();
  const intl = useIntl();
  // useIntl 在测试 mock 下每次渲染返回新引用，直接进 useCallback 依赖会让
  // 查询/导出回调每渲染重建；经 ref 转发后回调依赖稳定，执行时仍读取最新实例
  const intlRef = useRef(intl);
  intlRef.current = intl;
  const [loading, setLoading] = useState(false);
  const [data, setData] = useState<FormValues[]>([]);
  const [activeTab, setActiveTab] = useState(
    spec.charts && spec.charts.length > 0 ? 'chart' : 'table',
  );

  // 查找主绑定
  const mainBinding = bindings.find((b) => b.usage === 'report');
  const dataset = spec.dataset;
  const hasDatasetSemantics = dataset.dimensions.length > 0 && dataset.metrics.length > 0;
  const hasDatasetOutputSelector = !!mainBinding?.selectors?.output?.some(
    (assignment) => assignment.stateKey === 'dataset' && assignment.shape === 'dataset',
  );

  // 处理查询
  const handleQuery = useCallback(
    async (values: FormValues) => {
      if (!mainBinding) {
        message.error(
          intlRef.current.formatMessage({
            id: 'component.pageRenderer.reportPage.error.missingBinding',
            defaultMessage: '未配置报表绑定',
          }),
        );
        return;
      }
      if (preview) {
        message.info(
          intlRef.current.formatMessage({
            id: 'component.pageRenderer.reportPage.preview.blocked',
            defaultMessage: '预览模式不执行报表查询',
          }),
        );
        return;
      }

      setLoading(true);

      try {
        const response = await onExecute(mainBinding.id, { form: values });
        const nextState = mergePageState({}, outputPatchFromResult(mainBinding, response));
        const dataset = getPageStateArray(nextState, 'dataset');
        if (!dataset.length) {
          message.error(
            intlRef.current.formatMessage({
              id: 'component.pageRenderer.reportPage.error.datasetMissed',
              defaultMessage: '报表查询结果未命中 dataset 映射',
            }),
          );
          setData([]);
          return;
        }
        setData(dataset);
        message.success(
          intlRef.current.formatMessage({
            id: 'component.pageRenderer.reportPage.message.querySuccess',
            defaultMessage: '查询成功',
          }),
        );
      } catch (error) {
        const detail = extractErrorMessage(
          error,
          intlRef.current.formatMessage({
            id: 'component.pageRenderer.reportPage.error.unknown',
            defaultMessage: '未知错误',
          }),
        );
        message.error(
          intlRef.current.formatMessage(
            {
              id: 'component.pageRenderer.reportPage.error.queryFailed',
              defaultMessage: `查询失败: ${detail}`,
            },
            { detail },
          ),
        );
      } finally {
        setLoading(false);
      }
    },
    [message, mainBinding, onExecute, preview],
  );

  // 处理导出
  const handleExport = useCallback(
    async (format: 'csv' | 'excel') => {
      if (preview) {
        message.info(
          intlRef.current.formatMessage({
            id: 'component.pageRenderer.reportPage.preview.exportBlocked',
            defaultMessage: '预览模式不导出数据',
          }),
        );
        return;
      }

      if (!onExport) {
        if (format !== 'csv') {
          message.warning(
            intlRef.current.formatMessage({
              id: 'component.pageRenderer.reportPage.export.csvOnly',
              defaultMessage: '当前页面仅支持 CSV 导出',
            }),
          );
          return;
        }
        if (!data.length) {
          message.warning(
            intlRef.current.formatMessage({
              id: 'component.pageRenderer.reportPage.export.noData',
              defaultMessage: '没有可导出的数据',
            }),
          );
          return;
        }
        const exportColumns = [
          ...dataset.dimensions.map((dimension) => ({
            key: dimension.key,
            title: localizedText(dimension.title, 'zh-CN', dimension.key),
          })),
          ...dataset.metrics.map((metric) => ({
            key: metric.key,
            title: localizedText(metric.title, 'zh-CN', metric.key),
          })),
        ];
        downloadDatasetCsv(data, exportColumns);
        message.success(
          intlRef.current.formatMessage({
            id: 'component.pageRenderer.reportPage.export.success',
            defaultMessage: '导出成功',
          }),
        );
        return;
      }

      try {
        await onExport(format);
        message.success(
          intlRef.current.formatMessage({
            id: 'component.pageRenderer.reportPage.export.success',
            defaultMessage: '导出成功',
          }),
        );
      } catch (error) {
        const detail = extractErrorMessage(
          error,
          intlRef.current.formatMessage({
            id: 'component.pageRenderer.reportPage.error.unknown',
            defaultMessage: '未知错误',
          }),
        );
        message.error(
          intlRef.current.formatMessage(
            {
              id: 'component.pageRenderer.reportPage.export.failed',
              defaultMessage: `导出失败: ${detail}`,
            },
            { detail },
          ),
        );
      }
    },
    [message, data, dataset, onExport, preview],
  );

  // 构建表格列
  const columns: ProColumns[] = dataset.dimensions.map((dim) => ({
    title: localizedText(dim.title, 'zh-CN', dim.key),
    dataIndex: dim.key,
    key: dim.key,
    valueType: dim.dataType === 'number' ? 'digit' : dim.dataType === 'date' ? 'date' : 'text',
  }));

  // 添加指标列
  dataset.metrics.forEach((metric) => {
    columns.push({
      title: localizedText(metric.title, 'zh-CN', metric.key),
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

  if (!hasDatasetSemantics) {
    return (
      <Result
        status="warning"
        title={intl.formatMessage({
          id: 'component.pageRenderer.reportPage.warning.semanticsTitle',
          defaultMessage: '报表语义未完成',
        })}
        subTitle={intl.formatMessage({
          id: 'component.pageRenderer.reportPage.warning.semanticsSubtitle',
          defaultMessage:
            'ReportPage 发布前必须配置 dataset.dimensions 和 dataset.metrics，否则无法生成可运行的图表和数据表。',
        })}
      />
    );
  }

  if (!mainBinding || !hasDatasetOutputSelector) {
    return (
      <Result
        status="warning"
        title={intl.formatMessage({
          id: 'component.pageRenderer.reportPage.warning.bindingTitle',
          defaultMessage: '报表绑定未完成',
        })}
        subTitle={intl.formatMessage({
          id: 'component.pageRenderer.reportPage.warning.bindingSubtitle',
          defaultMessage:
            'ReportPage 必须通过 output selector 将函数结果映射到 pageState.dataset。',
        })}
      />
    );
  }

  return (
    <div>
      {/* 查询表单 */}
      <Card
        title={
          title ||
          intl.formatMessage({
            id: 'component.pageRenderer.reportPage.form.title',
            defaultMessage: '报表查询',
          })
        }
      >
        <SchemaFormRenderer
          spec={spec.queryForm}
          onFinish={handleQuery}
          disabled={loading || preview}
        />
        <Button style={{ marginTop: 12 }} onClick={() => setData([])}>
          <FormattedMessage
            id="component.pageRenderer.reportPage.button.clearResults"
            defaultMessage="清空结果"
          />
        </Button>
      </Card>

      {/* 数据展示 */}
      {data.length > 0 && (
        <Card
          title={intl.formatMessage({
            id: 'component.pageRenderer.reportPage.data.title',
            defaultMessage: '数据展示',
          })}
          style={{ marginTop: 16 }}
          extra={
            <Space>
              {spec.exportable && (
                <>
                  <Button icon={<DownloadOutlined />} onClick={() => handleExport('csv')}>
                    <FormattedMessage
                      id="component.pageRenderer.reportPage.export.csv"
                      defaultMessage="导出 CSV"
                    />
                  </Button>
                  {onExport ? (
                    <Button icon={<DownloadOutlined />} onClick={() => handleExport('excel')}>
                      <FormattedMessage
                        id="component.pageRenderer.reportPage.export.excel"
                        defaultMessage="导出 Excel"
                      />
                    </Button>
                  ) : null}
                </>
              )}
              <Button icon={<ReloadOutlined />} onClick={() => setData([])}>
                <FormattedMessage
                  id="component.pageRenderer.reportPage.button.clear"
                  defaultMessage="清空"
                />
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
                          <FormattedMessage
                            id="component.pageRenderer.reportPage.tab.chart"
                            defaultMessage="图表"
                          />
                        </span>
                      ),
                      children: (
                        <Space orientation="vertical" style={{ width: '100%' }}>
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
                    <FormattedMessage
                      id="component.pageRenderer.reportPage.tab.table"
                      defaultMessage="表格"
                    />
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
                      showTotal: (total) =>
                        intl.formatMessage(
                          {
                            id: 'component.pageRenderer.reportPage.pagination.total',
                            defaultMessage: `共 ${total} 条`,
                          },
                          { total },
                        ),
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
          <Empty
            description={intl.formatMessage({
              id: 'component.pageRenderer.reportPage.empty.queryFirst',
              defaultMessage: '请先查询数据',
            })}
          />
        </Card>
      )}
    </div>
  );
};

export default ReportPageRenderer;
