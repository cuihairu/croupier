/**
 * 性能分析显示组件
 *
 * 展示配置的性能分析结果，包括得分、指标和建议。
 *
 * @module pages/WorkspaceEditor/components/PerformancePanel
 */

import React from 'react';
import { Card, Progress, Tag, Collapse, List, Typography, Space } from 'antd';
import {
  CheckCircleOutlined,
  WarningOutlined,
  CloseCircleOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons';
import type { PerformanceAnalysisResult } from '../utils/configAnalyzer';
import { getPerformanceLevelColor, getPerformanceLevelText } from '../utils/configAnalyzer';

const { Text } = Typography;

export interface PerformancePanelProps {
  result: PerformanceAnalysisResult;
  title?: string;
}

export default function PerformancePanel({ result, title = '性能分析' }: PerformancePanelProps) {
  const { score, level, metrics, suggestions } = result;

  const getIcon = () => {
    switch (level) {
      case 'good':
        return <CheckCircleOutlined style={{ color: getPerformanceLevelColor(level) }} />;
      case 'warning':
        return <WarningOutlined style={{ color: getPerformanceLevelColor(level) }} />;
      case 'poor':
        return <CloseCircleOutlined style={{ color: getPerformanceLevelColor(level) }} />;
      default:
        return null;
    }
  };

  const getScoreColor = () => {
    switch (level) {
      case 'good':
        return '#52c41a';
      case 'warning':
        return '#faad14';
      case 'poor':
        return '#ff4d4f';
      default:
        return '#d9d9d9';
    }
  };

  return (
    <Card
      size="small"
      title={
        <Space size={8}>
          <ThunderboltOutlined />
          <span>{title}</span>
        </Space>
      }
    >
      <Space direction="vertical" style={{ width: '100%' }} size={12}>
        {/* 得分 */}
        <div style={{ display: 'flex', alignItems: 'center', gap: 16 }}>
          <div style={{ flex: 1 }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 4 }}>
              <Text type="secondary">性能得分</Text>
              <Text strong style={{ fontSize: 16 }}>
                {score}/100
              </Text>
            </div>
            <Progress percent={score} strokeColor={getScoreColor()} showInfo={false} size="small" />
          </div>
          <div
            style={{
              padding: '4px 12px',
              borderRadius: 4,
              background: `${getPerformanceLevelColor(level)}15`,
              color: getPerformanceLevelColor(level),
              fontWeight: 500,
            }}
          >
            {getIcon()} {getPerformanceLevelText(level)}
          </div>
        </div>

        {/* 指标详情 */}
        {metrics.length > 0 && (
          <Collapse
            ghost
            size="small"
            items={[
              {
                key: 'metrics',
                label: `性能指标 (${metrics.length})`,
                children: (
                  <div
                    style={{
                      display: 'grid',
                      gridTemplateColumns: 'repeat(auto-fill, minmax(200px, 1fr))',
                      gap: 8,
                    }}
                  >
                    {metrics.map((metric, index) => (
                      <div
                        key={index}
                        style={{
                          padding: '8px 12px',
                          borderRadius: 4,
                          border: '1px solid',
                          borderColor:
                            metric.level === 'good'
                              ? '#b7eb8f'
                              : metric.level === 'warning'
                              ? '#ffe58f'
                              : '#ffccc7',
                        }}
                      >
                        <div
                          style={{
                            display: 'flex',
                            justifyContent: 'space-between',
                            marginBottom: 4,
                          }}
                        >
                          <Text type="secondary" style={{ fontSize: 12 }}>
                            {metric.name}
                          </Text>
                          <Tag
                            color={
                              metric.level === 'good'
                                ? 'success'
                                : metric.level === 'warning'
                                ? 'warning'
                                : 'error'
                            }
                            style={{ margin: 0, fontSize: 11 }}
                          >
                            {metric.level === 'good'
                              ? '良好'
                              : metric.level === 'warning'
                              ? '一般'
                              : '较差'}
                          </Tag>
                        </div>
                        <div style={{ fontSize: 16, fontWeight: 500 }}>
                          {metric.value}
                          <Text type="secondary" style={{ fontSize: 12, marginLeft: 4 }}>
                            {metric.unit}
                          </Text>
                        </div>
                      </div>
                    ))}
                  </div>
                ),
              },
            ]}
          />
        )}

        {/* 优化建议 */}
        {suggestions.length > 0 && (
          <Collapse
            ghost
            size="small"
            items={[
              {
                key: 'suggestions',
                label: `优化建议 (${suggestions.length})`,
                children: (
                  <List
                    size="small"
                    dataSource={suggestions}
                    renderItem={(item) => (
                      <List.Item style={{ padding: '4px 0' }}>
                        <Text style={{ fontSize: 12 }}>• {item}</Text>
                      </List.Item>
                    )}
                  />
                ),
              },
            ]}
          />
        )}
      </Space>
    </Card>
  );
}
