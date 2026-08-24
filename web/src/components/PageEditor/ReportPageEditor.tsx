/**
 * ReportPageEditor - 报表页面语义化编辑器。
 *
 * 只编辑 ReportPageSpec 的查询表单展示、数据集字段和图表配置；
 * datasetPath、dimension/metric selector 仍由 CapabilitySemantics/PageSpec 校验链负责。
 */

import React, { useCallback, useState } from 'react';
import { Button, Card, Collapse, Form, Input, Select, Space, Switch, Tag, Typography } from 'antd';
import {
  BarChartOutlined,
  DeleteOutlined,
  LineChartOutlined,
  PlusOutlined,
  ProfileOutlined,
  TableOutlined,
} from '@ant-design/icons';
import type { ChartSpec, DimensionSpec, MetricSpec, ReportPageSpec } from '@/types/dashboard';
import FormPresentationEditor from './FormPresentationEditor';
import LocalizedTextEditor from '@/components/LocalizedTextEditor';
import { localizedText } from '@/utils/localizedText';

const { Text } = Typography;
const { Panel } = Collapse;

export interface ReportPageEditorProps {
  value: ReportPageSpec;
  onChange: (value: ReportPageSpec) => void;
  readonly?: boolean;
}

const updateDimension = (
  dimensions: DimensionSpec[],
  index: number,
  updates: Partial<DimensionSpec>,
): DimensionSpec[] =>
  dimensions.map((dimension, currentIndex) =>
    currentIndex === index ? { ...dimension, ...updates } : dimension,
  );

const updateMetric = (
  metrics: MetricSpec[],
  index: number,
  updates: Partial<MetricSpec>,
): MetricSpec[] =>
  metrics.map((metric, currentIndex) =>
    currentIndex === index ? { ...metric, ...updates } : metric,
  );

const updateChart = (
  charts: ChartSpec[],
  index: number,
  updates: Partial<ChartSpec>,
): ChartSpec[] =>
  charts.map((chart, currentIndex) => (currentIndex === index ? { ...chart, ...updates } : chart));

export default function ReportPageEditor({
  value,
  onChange,
  readonly = false,
}: ReportPageEditorProps) {
  const [activeKey, setActiveKey] = useState<string[]>(['dataset']);

  const handleDatasetChange = useCallback(
    (updates: Partial<ReportPageSpec['dataset']>) => {
      onChange({
        ...value,
        dataset: {
          ...value.dataset,
          ...updates,
        },
      });
    },
    [onChange, value],
  );

  const handleChartsChange = useCallback(
    (charts: ChartSpec[]) => {
      onChange({
        ...value,
        charts,
      });
    },
    [onChange, value],
  );

  const handleAddChart = useCallback(() => {
    const charts = value.charts || [];
    handleChartsChange([
      ...charts,
      {
        type: 'line',
        title: { 'zh-CN': '新图表' },
      },
    ]);
  }, [handleChartsChange, value.charts]);

  return (
    <Collapse activeKey={activeKey} onChange={setActiveKey} bordered={false}>
      <Panel
        header={
          <Space>
            <ProfileOutlined />
            <Text strong>查询表单</Text>
            <Tag>{value.queryForm?.fields?.length || 0} 字段</Tag>
          </Space>
        }
        key="queryForm"
      >
        <FormPresentationEditor
          value={value.queryForm}
          onChange={(queryForm) => onChange({ ...value, queryForm })}
          readonly={readonly}
        />
      </Panel>

      <Panel
        header={
          <Space>
            <TableOutlined />
            <Text strong>数据集</Text>
            <Tag>{value.dataset.dimensions.length} 维度</Tag>
            <Tag>{value.dataset.metrics.length} 指标</Tag>
          </Space>
        }
        key="dataset"
      >
        <Space direction="vertical" style={{ width: '100%' }}>
          <div>
            <Space style={{ marginBottom: 8 }}>
              <Text strong>维度</Text>
            </Space>
            <Text type="secondary">维度 key 来自已审核的报表语义，页面只调整展示文本和类型。</Text>
            {value.dataset.dimensions.map((dimension, index) => (
              <Card
                key={dimension.key}
                size="small"
                style={{ marginBottom: 8 }}
                title={
                  <Space size={12} wrap>
                    <Text code>{dimension.key}</Text>
                    <Select
                      size="small"
                      value={dimension.dataType}
                      onChange={(dataType) =>
                        handleDatasetChange({
                          dimensions: updateDimension(value.dataset.dimensions, index, {
                            dataType,
                          }),
                        })
                      }
                      style={{ width: 100 }}
                      options={[
                        { value: 'string', label: '字符串' },
                        { value: 'number', label: '数字' },
                        { value: 'date', label: '日期' },
                      ]}
                    />
                  </Space>
                }
              >
                <Form layout="vertical" disabled={readonly} style={{ marginBottom: 0 }}>
                  <Form.Item label="标题" style={{ marginBottom: 0 }}>
                    <LocalizedTextEditor
                      value={dimension.title}
                      onChange={(title) =>
                        handleDatasetChange({
                          dimensions: updateDimension(value.dataset.dimensions, index, { title }),
                        })
                      }
                    />
                  </Form.Item>
                </Form>
              </Card>
            ))}
          </div>

          <div>
            <Space style={{ marginBottom: 8 }}>
              <Text strong>指标</Text>
            </Space>
            <Text type="secondary">指标 key 来自已审核的报表语义，页面只调整展示格式。</Text>
            {value.dataset.metrics.map((metric, index) => (
              <Card
                key={metric.key}
                size="small"
                style={{ marginBottom: 8 }}
                title={
                  <Space size={12} wrap>
                    <Text code>{metric.key}</Text>
                    <Select
                      size="small"
                      value={metric.aggType}
                      onChange={(aggType) =>
                        handleDatasetChange({
                          metrics: updateMetric(value.dataset.metrics, index, { aggType }),
                        })
                      }
                      style={{ width: 90 }}
                      options={[
                        { value: 'sum', label: 'sum' },
                        { value: 'avg', label: 'avg' },
                        { value: 'count', label: 'count' },
                        { value: 'min', label: 'min' },
                        { value: 'max', label: 'max' },
                      ]}
                    />
                    <Select
                      size="small"
                      value={metric.format}
                      allowClear
                      placeholder="格式"
                      onChange={(format) =>
                        handleDatasetChange({
                          metrics: updateMetric(value.dataset.metrics, index, { format }),
                        })
                      }
                      style={{ width: 100 }}
                      options={[
                        { value: 'number', label: 'number' },
                        { value: 'percent', label: 'percent' },
                        { value: 'currency', label: 'currency' },
                      ]}
                    />
                  </Space>
                }
              >
                <Form layout="vertical" disabled={readonly} style={{ marginBottom: 0 }}>
                  <Form.Item label="标题" style={{ marginBottom: 0 }}>
                    <LocalizedTextEditor
                      value={metric.title}
                      onChange={(title) =>
                        handleDatasetChange({
                          metrics: updateMetric(value.dataset.metrics, index, { title }),
                        })
                      }
                    />
                  </Form.Item>
                </Form>
              </Card>
            ))}
          </div>
        </Space>
      </Panel>

      <Panel
        header={
          <Space>
            <LineChartOutlined />
            <Text strong>图表</Text>
            <Tag>{value.charts?.length || 0} 个</Tag>
          </Space>
        }
        key="charts"
      >
        <Button
          type="dashed"
          icon={<PlusOutlined />}
          onClick={handleAddChart}
          disabled={readonly}
          style={{ marginBottom: 16 }}
        >
          添加图表
        </Button>

        {value.charts?.map((chart, index) => (
          <Card
            key={`${chart.type}-${index}`}
            size="small"
            style={{ marginBottom: 8 }}
            title={
              <Space>
                <BarChartOutlined />
                <Text>{localizedText(chart.title, 'zh-CN', chart.type)}</Text>
              </Space>
            }
            extra={
              !readonly && (
                <Button
                  type="text"
                  danger
                  icon={<DeleteOutlined />}
                  onClick={() => {
                    const charts = [...(value.charts || [])];
                    charts.splice(index, 1);
                    handleChartsChange(charts);
                  }}
                />
              )
            }
          >
            <Form layout="vertical" disabled={readonly} style={{ marginBottom: 0 }}>
              <Form.Item label="标题" style={{ marginBottom: 8 }}>
                <LocalizedTextEditor
                  value={chart.title}
                  onChange={(title) =>
                    handleChartsChange(updateChart(value.charts || [], index, { title }))
                  }
                />
              </Form.Item>
              <Form.Item label="图表配置" style={{ marginBottom: 0 }}>
                <Space size={12} wrap>
                  <Select
                    value={chart.type}
                    onChange={(type) =>
                      handleChartsChange(updateChart(value.charts || [], index, { type }))
                    }
                    style={{ width: 110 }}
                    options={[
                      { value: 'line', label: 'line' },
                      { value: 'bar', label: 'bar' },
                      { value: 'pie', label: 'pie' },
                      { value: 'area', label: 'area' },
                      { value: 'scatter', label: 'scatter' },
                    ]}
                  />
                  <Space size={4}>
                    <Text type="secondary">X 字段</Text>
                    <Input
                      value={chart.xField}
                      onChange={(event) =>
                        handleChartsChange(
                          updateChart(value.charts || [], index, {
                            xField: event.target.value,
                          }),
                        )
                      }
                      style={{ width: 140 }}
                    />
                  </Space>
                  <Space size={4}>
                    <Text type="secondary">Y 字段</Text>
                    <Input
                      value={chart.yField}
                      onChange={(event) =>
                        handleChartsChange(
                          updateChart(value.charts || [], index, {
                            yField: event.target.value,
                          }),
                        )
                      }
                      style={{ width: 140 }}
                    />
                  </Space>
                </Space>
              </Form.Item>
            </Form>
          </Card>
        ))}
      </Panel>

      <Panel
        header={
          <Space>
            <TableOutlined />
            <Text strong>表格与导出</Text>
          </Space>
        }
        key="table"
      >
        <Form layout="vertical" disabled={readonly}>
          <Form.Item label="允许导出">
            <Switch
              checked={Boolean(value.exportable)}
              onChange={(exportable) => onChange({ ...value, exportable })}
            />
          </Form.Item>
          <Form.Item label="表格">
            <Tag color={value.table ? 'success' : 'default'}>
              {value.table ? `${value.table.columns.length} 列` : '未配置'}
            </Tag>
          </Form.Item>
        </Form>
      </Panel>
    </Collapse>
  );
}
