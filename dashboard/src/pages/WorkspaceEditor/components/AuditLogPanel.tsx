/**
 * 审计日志面板组件
 *
 * 显示和查询编辑操作审计日志。
 *
 * @module pages/WorkspaceEditor/components/AuditLogPanel
 */

import React, { useState, useEffect } from 'react';
import {
  Card,
  Table,
  Space,
  Tag,
  Button,
  Select,
  DatePicker,
  Input,
  Modal,
  Tooltip,
  Typography,
  message,
  Statistic,
  Row,
  Col,
} from 'antd';
import {
  DownloadOutlined,
  SearchOutlined,
  ClearOutlined,
  ReloadOutlined,
  FileTextOutlined,
  BarChartOutlined,
} from '@ant-design/icons';
import type { ColumnsType } from 'antd';
import moment from 'moment';
import type { Moment } from 'moment';
import {
  loadAllLogs,
  queryAuditLogs,
  getAuditLogStats,
  exportAuditLogsAsCSV,
  exportAuditLogsAsJSON,
  getActionText,
  getActionColor,
  getActionIcon,
  cleanupOldLogs,
  type AuditLogEntry,
  type AuditLogQuery,
  type AuditLogStats,
} from '../utils/auditLogger';

const { Text } = Typography;
const { RangePicker } = DatePicker;

export interface AuditLogPanelProps {
  /** 对象标识（过滤特定对象的日志） */
  objectKey?: string;
  /** 只读模式 */
  readOnly?: boolean;
  /** Modal 模式 */
  visible?: boolean;
  onClose?: () => void;
}

/** 默认导出面板组件 */
export default function AuditLogPanel({
  objectKey,
  readOnly,
  visible,
  onClose,
}: AuditLogPanelProps) {
  const [logs, setLogs] = useState<AuditLogEntry[]>([]);
  const [stats, setStats] = useState<AuditLogStats | null>(null);
  const [loading, setLoading] = useState(false);
  const [searchText, setSearchText] = useState('');
  const [actionFilter, setActionFilter] = useState<string>('');
  const [dateRange, setDateRange] = useState<[Moment, Moment] | null>(null);
  const [detailVisible, setDetailVisible] = useState(false);
  const [selectedLog, setSelectedLog] = useState<AuditLogEntry | null>(null);

  const loadLogs = () => {
    setLoading(true);
    try {
      const query: AuditLogQuery = {
        objectKey,
        limit: 500,
      };

      if (actionFilter) {
        query.action = actionFilter as any;
      }

      if (dateRange) {
        query.startTime = dateRange[0].valueOf();
        query.endTime = dateRange[1].valueOf();
      }

      const filteredLogs = queryAuditLogs(query);

      // 客户端搜索过滤
      if (searchText) {
        const text = searchText.toLowerCase();
        const searchedLogs = filteredLogs.filter(
          (log) =>
            log.description.toLowerCase().includes(text) ||
            log.objectKey.toLowerCase().includes(text) ||
            (log.actor && log.actor.toLowerCase().includes(text)),
        );
        setLogs(searchedLogs);
      } else {
        setLogs(filteredLogs);
      }

      // 加载统计
      setStats(getAuditLogStats(query));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (visible !== false) {
      loadLogs();
    }
  }, [objectKey, actionFilter, dateRange, searchText, visible]);

  const handleExportCSV = () => {
    try {
      const csv = exportAuditLogsAsCSV({ objectKey, limit: 1000 });
      const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' });
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `audit_log_${objectKey || 'all'}_${moment().format('YYYYMMDD_HHmmss')}.csv`;
      a.click();
      URL.revokeObjectURL(url);
      message.success('日志导出成功');
    } catch (error) {
      message.error('日志导出失败');
    }
  };

  const handleExportJSON = () => {
    try {
      const json = exportAuditLogsAsJSON({ objectKey, limit: 1000 });
      const blob = new Blob([json], { type: 'application/json;charset=utf-8;' });
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `audit_log_${objectKey || 'all'}_${moment().format('YYYYMMDD_HHmmss')}.json`;
      a.click();
      URL.revokeObjectURL(url);
      message.success('日志导出成功');
    } catch (error) {
      message.error('日志导出失败');
    }
  };

  const handleCleanup = () => {
    Modal.confirm({
      title: '清理日志',
      content: '将清理 90 天前的日志记录，此操作不可恢复。是否继续？',
      onOk: () => {
        const deleted = cleanupOldLogs(90);
        message.success(`已清理 ${deleted} 条旧日志`);
        loadLogs();
      },
    });
  };

  const columns: ColumnsType<AuditLogEntry> = [
    {
      title: '时间',
      dataIndex: 'timestampISO',
      key: 'timestamp',
      width: 160,
      render: (timestamp: string) => (
        <Text type="secondary" style={{ fontSize: 12 }}>
          {moment(timestamp).format('YYYY-MM-DD HH:mm:ss')}
        </Text>
      ),
    },
    {
      title: '操作',
      dataIndex: 'action',
      key: 'action',
      width: 100,
      render: (action: string) => (
        <Tag color={getActionColor(action as any)} style={{ fontSize: 11 }}>
          {getActionIcon(action as any)} {getActionText(action as any)}
        </Tag>
      ),
    },
    {
      title: '描述',
      dataIndex: 'description',
      key: 'description',
      ellipsis: true,
    },
    {
      title: '对象',
      dataIndex: 'objectKey',
      key: 'objectKey',
      width: 120,
      render: (objectKey: string) => (
        <Text code style={{ fontSize: 11 }}>
          {objectKey}
        </Text>
      ),
    },
    {
      title: '操作者',
      dataIndex: 'actor',
      key: 'actor',
      width: 100,
      render: (actor?: string) => actor || '-',
    },
    {
      title: '详情',
      key: 'details',
      width: 80,
      render: (_, record: AuditLogEntry) => (
        <Button
          type="link"
          size="small"
          icon={<FileTextOutlined />}
          onClick={() => {
            setSelectedLog(record);
            setDetailVisible(true);
          }}
        >
          查看
        </Button>
      ),
    },
  ];

  const content = (
    <Space direction="vertical" style={{ width: '100%' }} size={16}>
      {/* 统计卡片 */}
      {stats && (
        <Row gutter={16}>
          <Col span={6}>
            <Card size="small">
              <Statistic
                title="总记录数"
                value={stats.totalEntries}
                prefix={<BarChartOutlined />}
                valueStyle={{ fontSize: 20 }}
              />
            </Card>
          </Col>
          <Col span={6}>
            <Card size="small">
              <Statistic
                title="操作类型"
                value={Object.keys(stats.actionsByType).length}
                valueStyle={{ fontSize: 20 }}
              />
            </Card>
          </Col>
          <Col span={6}>
            <Card size="small">
              <Statistic
                title="涉及对象"
                value={Object.keys(stats.entriesByObject).length}
                valueStyle={{ fontSize: 20 }}
              />
            </Card>
          </Col>
          <Col span={6}>
            <Card size="small">
              <Statistic
                title="操作者"
                value={Object.keys(stats.entriesByActor).length}
                valueStyle={{ fontSize: 20 }}
              />
            </Card>
          </Col>
        </Row>
      )}

      {/* 过滤器 */}
      <Card size="small" title="筛选条件">
        <Space wrap>
          <Input
            placeholder="搜索描述、对象、操作者"
            prefix={<SearchOutlined />}
            value={searchText}
            onChange={(e) => setSearchText(e.target.value)}
            allowClear
            style={{ width: 200 }}
          />
          <Select
            placeholder="操作类型"
            value={actionFilter || undefined}
            onChange={setActionFilter}
            allowClear
            style={{ width: 120 }}
          >
            <Select.Option value="create">创建</Select.Option>
            <Select.Option value="update">更新</Select.Option>
            <Select.Option value="delete">删除</Select.Option>
            <Select.Option value="publish">发布</Select.Option>
            <Select.Option value="rollback">回滚</Select.Option>
            <Select.Option value="import">导入</Select.Option>
            <Select.Option value="export">导出</Select.Option>
            <Select.Option value="template_apply">应用模板</Select.Option>
          </Select>
          <RangePicker
            value={dateRange}
            onChange={setDateRange}
            format="YYYY-MM-DD"
            placeholder={['开始日期', '结束日期']}
            style={{ width: 240 }}
          />
          <Button icon={<ReloadOutlined />} onClick={loadLogs}>
            刷新
          </Button>
          <Button
            icon={<ClearOutlined />}
            onClick={() => {
              setSearchText('');
              setActionFilter('');
              setDateRange(null);
            }}
          >
            清空
          </Button>
        </Space>
      </Card>

      {/* 导出操作 */}
      <Card size="small">
        <Space>
          <Tooltip title="导出为 CSV">
            <Button icon={<DownloadOutlined />} onClick={handleExportCSV}>
              导出CSV
            </Button>
          </Tooltip>
          <Tooltip title="导出为 JSON">
            <Button icon={<DownloadOutlined />} onClick={handleExportJSON}>
              导出JSON
            </Button>
          </Tooltip>
          <Button onClick={handleCleanup} type="text" danger>
            清理90天前日志
          </Button>
        </Space>
      </Card>

      {/* 日志列表 */}
      <Card size="small">
        <Table
          columns={columns}
          dataSource={logs}
          rowKey="id"
          loading={loading}
          size="small"
          pagination={{
            pageSize: 20,
            showSizeChanger: true,
            showTotal: (total) => `共 ${total} 条`,
          }}
          scroll={{ y: 400 }}
          locale={{ emptyText: '暂无审计日志' }}
        />
      </Card>
    </Space>
  );

  // 如果是 Modal 模式
  if (visible !== undefined) {
    return (
      <Modal
        title="审计日志"
        open={visible}
        onCancel={onClose}
        width={1000}
        footer={null}
        styles={{ body: { padding: 16 } }}
      >
        {content}
      </Modal>
    );
  }

  return content;
}

/** 日志详情 Modal */
export function AuditLogDetailModal({
  visible,
  onClose,
  log,
}: {
  visible: boolean;
  onClose: () => void;
  log: AuditLogEntry | null;
}) {
  if (!log) return null;

  return (
    <Modal
      title="日志详情"
      open={visible}
      onCancel={onClose}
      footer={[
        <Button key="close" onClick={onClose}>
          关闭
        </Button>,
      ]}
      width={600}
    >
      <Space direction="vertical" style={{ width: '100%' }} size={12}>
        <div>
          <Text type="secondary">日志ID</Text>
          <br />
          <Text code copyable>
            {log.id}
          </Text>
        </div>
        <div>
          <Text type="secondary">操作时间</Text>
          <br />
          <Text>{dayjs(log.timestampISO).format('YYYY-MM-DD HH:mm:ss')}</Text>
        </div>
        <div>
          <Text type="secondary">操作类型</Text>
          <br />
          <Tag color={getActionColor(log.action)}>
            {getActionIcon(log.action)} {getActionText(log.action)}
          </Tag>
        </div>
        <div>
          <Text type="secondary">操作者</Text>
          <br />
          <Text>{log.actor || '-'}</Text>
        </div>
        <div>
          <Text type="secondary">对象</Text>
          <br />
          <Text code>{log.objectKey}</Text>
        </div>
        <div>
          <Text type="secondary">描述</Text>
          <br />
          <Text>{log.description}</Text>
        </div>
        {log.details && (
          <div>
            <Text type="secondary">变更详情</Text>
            <pre
              style={{
                background: '#f5f5f5',
                padding: 12,
                borderRadius: 4,
                marginTop: 8,
                fontSize: 12,
              }}
            >
              {JSON.stringify(log.details, null, 2)}
            </pre>
          </div>
        )}
      </Space>
    </Modal>
  );
}
