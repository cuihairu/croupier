/**
 * 配置变更影响分析面板
 *
 * 显示配置变更的影响分析结果，包括影响级别、详细列表和警告信息。
 *
 * @module pages/WorkspaceEditor/components/ImpactAnalysisPanel
 */

import React from 'react';
import { Alert, List, Tag, Space, Typography, Card, Progress, Tooltip } from 'antd';
import {
  WarningOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  ExclamationCircleOutlined,
  InfoCircleOutlined,
} from '@ant-design/icons';
import {
  getImpactLevelColor,
  getImpactLevelText,
  getImpactTypeIcon,
  type ImpactAnalysisResult,
  type ImpactLevel,
  type ImpactItem,
} from '../utils/impactAnalyzer';

const { Text, Paragraph } = Typography;

export interface ImpactAnalysisPanelProps {
  result: ImpactAnalysisResult;
  title?: React.ReactNode;
  showDetails?: boolean;
}

/** 影响级别图标 */
function LevelIcon({ level }: { level: ImpactLevel }) {
  const icons: Record<ImpactLevel, React.ReactNode> = {
    low: <CheckCircleOutlined style={{ color: '#52c41a' }} />,
    medium: <ExclamationCircleOutlined style={{ color: '#faad14' }} />,
    high: <CloseCircleOutlined style={{ color: '#ff4d4f' }} />,
    critical: <WarningOutlined style={{ color: '#722ed1' }} />,
  };
  return icons[level];
}

/** 总体影响卡片 */
function OverallImpactCard({ result, title }: ImpactAnalysisPanelProps) {
  const percent =
    result.level === 'critical'
      ? 100
      : result.level === 'high'
      ? 75
      : result.level === 'medium'
      ? 50
      : 25;

  const strokeColor =
    result.level === 'critical'
      ? '#722ed1'
      : result.level === 'high'
      ? '#ff4d4f'
      : result.level === 'medium'
      ? '#faad14'
      : '#52c41a';

  return (
    <Card size="small">
      <Space direction="vertical" style={{ width: '100%' }} size={12}>
        <Space>
          <LevelIcon level={result.level} />
          <Text strong style={{ fontSize: 16 }}>
            {title || '影响分析'}
          </Text>
          <Tag color={getImpactLevelColor(result.level)}>{getImpactLevelText(result.level)}</Tag>
        </Space>
        <div>
          <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 8 }}>
            <Text type="secondary">影响分数</Text>
            <Text strong>{result.score}/100</Text>
          </div>
          <Progress
            percent={Math.min(result.score, 100)}
            strokeColor={strokeColor}
            size="small"
            showInfo={false}
          />
        </div>
        <Paragraph style={{ marginBottom: 0 }}>{result.summary}</Paragraph>
      </Space>
    </Card>
  );
}

/** 单个影响项 */
function ImpactItemCard({ item }: { item: ImpactItem }) {
  return (
    <List.Item
      style={{
        padding: '8px 12px',
        borderRadius: 4,
        background:
          item.level === 'critical'
            ? '#fff0f6'
            : item.level === 'high'
            ? '#fff1f0'
            : item.level === 'medium'
            ? '#fffbe6'
            : '#f6ffed',
        marginBottom: 8,
      }}
    >
      <Space direction="vertical" style={{ width: '100%' }} size={4}>
        <Space>
          <span style={{ fontSize: 16 }}>{getImpactTypeIcon(item.type)}</span>
          <Text strong>{item.description}</Text>
          <Tag color={getImpactLevelColor(item.level)} style={{ marginLeft: 8 }}>
            {getImpactLevelText(item.level)}
          </Tag>
        </Space>
        {item.recommendation && (
          <Text type="secondary" style={{ fontSize: 12 }}>
            💡 {item.recommendation}
          </Text>
        )}
        {item.affectedUsers !== undefined && (
          <Text type="secondary" style={{ fontSize: 12 }}>
            👥 预计影响 {item.affectedUsers.toLocaleString()} 用户
          </Text>
        )}
      </Space>
    </List.Item>
  );
}

/** 默认导出面板组件 */
export default function ImpactAnalysisPanel({
  result,
  title,
  showDetails = true,
}: ImpactAnalysisPanelProps) {
  // 按影响级别分组
  const groupedImpacts = React.useMemo(() => {
    const groups: Record<ImpactLevel, ImpactItem[]> = {
      low: [],
      medium: [],
      high: [],
      critical: [],
    };
    result.impacts.forEach((impact) => {
      groups[impact.level].push(impact);
    });
    return groups;
  }, [result.impacts]);

  return (
    <Space direction="vertical" style={{ width: '100%' }} size={16}>
      {/* 总体影响 */}
      <OverallImpactCard result={result} title={title} />

      {/* 警告信息 */}
      {result.warnings.length > 0 && (
        <Alert
          message="重要警告"
          description={
            <ul style={{ margin: 0, paddingLeft: 16 }}>
              {result.warnings.map((warning, index) => (
                <li key={index}>{warning}</li>
              ))}
            </ul>
          }
          type="warning"
          showIcon
        />
      )}

      {/* 详细影响列表 */}
      {showDetails && result.impacts.length > 0 && (
        <Card title="影响详情" size="small">
          <List
            dataSource={result.impacts}
            renderItem={(item) => <ImpactItemCard item={item} />}
            style={{ maxHeight: 400, overflow: 'auto' }}
          />
        </Card>
      )}

      {/* 无变更提示 */}
      {result.impacts.length === 0 && (
        <Alert
          message="无变更影响"
          description="当前配置与原配置相比无变更，或仅有非关键性调整"
          type="success"
          showIcon
        />
      )}
    </Space>
  );
}

/** 紧凑版本的影响指示器 */
export function ImpactIndicator({ result }: { result: ImpactAnalysisResult }) {
  if (result.impacts.length === 0) {
    return (
      <Tag icon={<CheckCircleOutlined />} color="success">
        无变更
      </Tag>
    );
  }

  return (
    <Tooltip title={result.summary}>
      <Tag icon={<LevelIcon level={result.level} />} color={getImpactLevelColor(result.level)}>
        {result.impacts.length} 项变更
      </Tag>
    </Tooltip>
  );
}
