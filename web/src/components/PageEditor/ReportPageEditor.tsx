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
  HolderOutlined,
  LineChartOutlined,
  PlusOutlined,
  ProfileOutlined,
  TableOutlined,
} from '@ant-design/icons';
import { FormattedMessage, useIntl } from '@umijs/max';
import type { ChartSpec, DimensionSpec, MetricSpec, ReportPageSpec } from '@/types/dashboard';
import FormPresentationEditor from './FormPresentationEditor';
import LocalizedTextEditor from '@/components/LocalizedTextEditor';
import { SortableList } from '@/components/SortableList';
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
  const intl = useIntl();
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
            <Text strong>
              <FormattedMessage
                id="component.pageEditor.reportPage.queryForm.title"
                defaultMessage="查询表单"
              />
            </Text>
            <Tag>
              <FormattedMessage
                id="component.pageEditor.reportPage.queryForm.fieldCount"
                defaultMessage={`${value.queryForm?.fields?.length || 0} 字段`}
                values={{ count: value.queryForm?.fields?.length || 0 }}
              />
            </Tag>
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
            <Text strong>
              <FormattedMessage
                id="component.pageEditor.reportPage.dataset.title"
                defaultMessage="数据集"
              />
            </Text>
            <Tag>
              <FormattedMessage
                id="component.pageEditor.reportPage.dataset.dimensionCount"
                defaultMessage={`${value.dataset.dimensions.length} 维度`}
                values={{ count: value.dataset.dimensions.length }}
              />
            </Tag>
            <Tag>
              <FormattedMessage
                id="component.pageEditor.reportPage.dataset.metricCount"
                defaultMessage={`${value.dataset.metrics.length} 指标`}
                values={{ count: value.dataset.metrics.length }}
              />
            </Tag>
          </Space>
        }
        key="dataset"
      >
        <Space orientation="vertical" style={{ width: '100%' }}>
          <div>
            <Space style={{ marginBottom: 8 }}>
              <Text strong>
                <FormattedMessage
                  id="component.pageEditor.reportPage.dataset.dimensions"
                  defaultMessage="维度"
                />
              </Text>
            </Space>
            <Text type="secondary">
              <FormattedMessage
                id="component.pageEditor.reportPage.dataset.dimensionsHint"
                defaultMessage="维度 key 来自已审核的报表语义，页面只调整展示文本和类型。"
              />
            </Text>
            {value.dataset.dimensions.length > 0 ? (
              <SortableList
                items={value.dataset.dimensions}
                getKey={(dimension) => dimension.key}
                onReorder={(dimensions) => handleDatasetChange({ dimensions })}
              >
                {(dimension, index, dragHandleProps) => (
                  <Card
                    size="small"
                    style={{ marginBottom: 8 }}
                    title={
                      <Space size={12} wrap>
                        {!readonly && (
                          <span {...dragHandleProps}>
                            <HolderOutlined />
                          </span>
                        )}
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
                            {
                              value: 'string',
                              label: intl.formatMessage({
                                id: 'component.pageEditor.reportPage.dataType.string',
                                defaultMessage: '字符串',
                              }),
                            },
                            {
                              value: 'number',
                              label: intl.formatMessage({
                                id: 'component.pageEditor.reportPage.dataType.number',
                                defaultMessage: '数字',
                              }),
                            },
                            {
                              value: 'date',
                              label: intl.formatMessage({
                                id: 'component.pageEditor.reportPage.dataType.date',
                                defaultMessage: '日期',
                              }),
                            },
                          ]}
                        />
                      </Space>
                    }
                  >
                    <Form layout="vertical" disabled={readonly} style={{ marginBottom: 0 }}>
                      <Form.Item
                        label={intl.formatMessage({
                          id: 'component.pageEditor.reportPage.dataset.fieldTitle',
                          defaultMessage: '标题',
                        })}
                        style={{ marginBottom: 0 }}
                      >
                        <LocalizedTextEditor
                          value={dimension.title}
                          onChange={(title) =>
                            handleDatasetChange({
                              dimensions: updateDimension(value.dataset.dimensions, index, {
                                title,
                              }),
                            })
                          }
                        />
                      </Form.Item>
                    </Form>
                  </Card>
                )}
              </SortableList>
            ) : null}
          </div>

          <div>
            <Space style={{ marginBottom: 8 }}>
              <Text strong>
                <FormattedMessage
                  id="component.pageEditor.reportPage.dataset.metrics"
                  defaultMessage="指标"
                />
              </Text>
            </Space>
            <Text type="secondary">
              <FormattedMessage
                id="component.pageEditor.reportPage.dataset.metricsHint"
                defaultMessage="指标 key 来自已审核的报表语义，页面只调整展示格式。"
              />
            </Text>
            {value.dataset.metrics.length > 0 ? (
              <SortableList
                items={value.dataset.metrics}
                getKey={(metric) => metric.key}
                onReorder={(metrics) => handleDatasetChange({ metrics })}
              >
                {(metric, index, dragHandleProps) => (
                  <Card
                    size="small"
                    style={{ marginBottom: 8 }}
                    title={
                      <Space size={12} wrap>
                        {!readonly && (
                          <span {...dragHandleProps}>
                            <HolderOutlined />
                          </span>
                        )}
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
                          placeholder={intl.formatMessage({
                            id: 'component.pageEditor.reportPage.dataset.format',
                            defaultMessage: '格式',
                          })}
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
                      <Form.Item
                        label={intl.formatMessage({
                          id: 'component.pageEditor.reportPage.dataset.fieldTitle',
                          defaultMessage: '标题',
                        })}
                        style={{ marginBottom: 0 }}
                      >
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
                )}
              </SortableList>
            ) : null}
          </div>
        </Space>
      </Panel>

      <Panel
        header={
          <Space>
            <LineChartOutlined />
            <Text strong>
              <FormattedMessage
                id="component.pageEditor.reportPage.charts.title"
                defaultMessage="图表"
              />
            </Text>
            <Tag>
              <FormattedMessage
                id="component.pageEditor.reportPage.charts.count"
                defaultMessage={`${value.charts?.length || 0} 个`}
                values={{ count: value.charts?.length || 0 }}
              />
            </Tag>
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
          <FormattedMessage
            id="component.pageEditor.reportPage.charts.addChart"
            defaultMessage="添加图表"
          />
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
              <Form.Item
                label={intl.formatMessage({
                  id: 'component.pageEditor.reportPage.chart.title',
                  defaultMessage: '标题',
                })}
                style={{ marginBottom: 8 }}
              >
                <LocalizedTextEditor
                  value={chart.title}
                  onChange={(title) =>
                    handleChartsChange(updateChart(value.charts || [], index, { title }))
                  }
                />
              </Form.Item>
              <Form.Item
                label={intl.formatMessage({
                  id: 'component.pageEditor.reportPage.chart.config',
                  defaultMessage: '图表配置',
                })}
                style={{ marginBottom: 0 }}
              >
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
                    <Text type="secondary">
                      <FormattedMessage
                        id="component.pageEditor.reportPage.chart.xField"
                        defaultMessage="X 字段"
                      />
                    </Text>
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
                    <Text type="secondary">
                      <FormattedMessage
                        id="component.pageEditor.reportPage.chart.yField"
                        defaultMessage="Y 字段"
                      />
                    </Text>
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
            <Text strong>
              <FormattedMessage
                id="component.pageEditor.reportPage.table.title"
                defaultMessage="表格与导出"
              />
            </Text>
          </Space>
        }
        key="table"
      >
        <Form layout="vertical" disabled={readonly}>
          <Form.Item
            label={intl.formatMessage({
              id: 'component.pageEditor.reportPage.table.exportable',
              defaultMessage: '允许导出',
            })}
          >
            <Switch
              checked={Boolean(value.exportable)}
              onChange={(exportable) => onChange({ ...value, exportable })}
            />
          </Form.Item>
          <Form.Item
            label={intl.formatMessage({
              id: 'component.pageEditor.reportPage.table.label',
              defaultMessage: '表格',
            })}
          >
            <Tag color={value.table ? 'success' : 'default'}>
              {value.table ? (
                <FormattedMessage
                  id="component.pageEditor.reportPage.table.columnCount"
                  defaultMessage={`${value.table.columns.length} 列`}
                  values={{ count: value.table.columns.length }}
                />
              ) : (
                intl.formatMessage({
                  id: 'component.pageEditor.reportPage.table.notConfigured',
                  defaultMessage: '未配置',
                })
              )}
            </Tag>
          </Form.Item>
        </Form>
      </Panel>
    </Collapse>
  );
}
