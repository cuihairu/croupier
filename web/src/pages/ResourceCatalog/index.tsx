/**
 * ResourceCatalogPage - 资源能力目录。
 *
 * 这里只管理 FunctionContract 聚合后的 CapabilitySemantics：
 * identity、collection、lifecycle binding、语义来源和冲突决议。
 * 页面标题、菜单、列、按钮位置属于 Page Proposal/Page Studio。
 */

import React, { useCallback, useEffect, useRef, useState } from 'react';
import {
  Button,
  Card,
  Form,
  Input,
  message,
  Select,
  Space,
  Table,
  Tag,
  Tooltip,
  Typography,
} from 'antd';
import {
  BulbOutlined,
  EditOutlined,
  EyeOutlined,
  ReloadOutlined,
  SearchOutlined,
} from '@ant-design/icons';
import { FormattedMessage, history, useIntl } from '@umijs/max';
import type { ColumnsType } from 'antd/es/table';
import type {
  DiagnosticInfo,
  FunctionInfo,
  ResourceCatalogItem,
  ResourceSemanticConflicts,
  ResourceSemanticVersions,
  ResolveSemanticConflictRequest,
  SemanticConflictInfo,
  SemanticsInfo,
  UpdateResourceSemanticsRequest,
} from '@/types/dashboard';
import {
  getResourceDetail,
  getResourceSemanticConflicts,
  getResourceSemanticVersions,
  listResourceCatalog,
  resolveResourceSemanticConflict,
  updateResourceSemantics,
} from '@/services/dashboard';
import { extractErrorMessage } from '@/utils/errors';
import { localizedText } from '@/utils/localizedText';
import {
  compactSemanticsPayload,
  conflictSources,
  emptySemanticMeta,
  emptySemanticVersions,
  semanticsToFormValues,
  statusColors,
  statusLabels,
} from './shared';
import ResourceDetailModal from './ResourceDetailModal';
import EditSemanticsModal from './EditSemanticsModal';
import ResolveConflictModal from './ResolveConflictModal';

const { Text } = Typography;

const ResourceCatalogPage: React.FC = () => {
  const intl = useIntl();
  // useIntl 在测试 mock 下每次渲染返回新引用，直接进 useCallback 依赖会让请求
  // effect 无限重建；经 ref 转发后回调依赖稳定，执行时仍读取最新实例
  const intlRef = useRef(intl);
  intlRef.current = intl;
  const [loading, setLoading] = useState(false);
  const [detailLoading, setDetailLoading] = useState(false);
  const [data, setData] = useState<ResourceCatalogItem[]>([]);
  const [total, setTotal] = useState(0);
  const [category, setCategory] = useState<string>('');
  const [query, setQuery] = useState<string>('');
  const [selectedResource, setSelectedResource] = useState<ResourceCatalogItem | null>(null);
  const [semanticMeta, setSemanticMeta] = useState<ResourceSemanticConflicts>(emptySemanticMeta);
  const [semanticVersions, setSemanticVersions] =
    useState<ResourceSemanticVersions>(emptySemanticVersions);
  // 语义版本服务端分页：版本历史可达上万条，必须按页拉取
  const [versionPage, setVersionPage] = useState(1);
  const [versionPageSize, setVersionPageSize] = useState(5);
  const [selectedConflict, setSelectedConflict] = useState<SemanticConflictInfo | null>(null);
  const [detailVisible, setDetailVisible] = useState(false);
  const [editVisible, setEditVisible] = useState(false);
  const [resolveVisible, setResolveVisible] = useState(false);
  const [editForm] = Form.useForm<UpdateResourceSemanticsRequest>();
  const [resolveForm] = Form.useForm<ResolveSemanticConflictRequest>();

  const fetchData = useCallback(async () => {
    setLoading(true);
    try {
      const result = await listResourceCatalog({
        category: category || undefined,
        query: query || undefined,
      });
      setData(result.items);
      setTotal(result.total);
    } catch (error) {
      message.error(
        extractErrorMessage(
          error,
          intlRef.current.formatMessage({
            id: 'pages.resourceCatalog.list.error.operationFailed',
            defaultMessage: '操作失败',
          }),
        ),
      );
    } finally {
      setLoading(false);
    }
  }, [category, query]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  const fetchSemanticVersions = useCallback(
    async (resourceKey: string, page: number, pageSize: number) => {
      try {
        const versions = await getResourceSemanticVersions(resourceKey, {
          limit: pageSize,
          offset: (page - 1) * pageSize,
        });
        setSemanticVersions(versions);
      } catch (error) {
        message.error(
          intlRef.current.formatMessage(
            {
              id: 'pages.resourceCatalog.list.error.fetchVersionsFailed',
              defaultMessage: '获取语义版本失败: {message}',
            },
            {
              message: extractErrorMessage(
                error,
                intlRef.current.formatMessage({
                  id: 'pages.resourceCatalog.list.error.unknown',
                  defaultMessage: '未知错误',
                }),
              ),
            },
          ),
        );
      }
    },
    [],
  );

  const loadResourceDetail = useCallback(
    async (resourceKey: string) => {
      setDetailLoading(true);
      try {
        const [detail, meta] = await Promise.all([
          getResourceDetail(resourceKey),
          getResourceSemanticConflicts(resourceKey),
        ]);
        setSelectedResource(detail);
        setSemanticMeta(meta);
        await fetchSemanticVersions(resourceKey, versionPage, versionPageSize);
        return detail;
      } catch (error) {
        message.error(
          intlRef.current.formatMessage(
            {
              id: 'pages.resourceCatalog.list.error.fetchDetailFailed',
              defaultMessage: '获取详情失败: {message}',
            },
            {
              message: extractErrorMessage(
                error,
                intlRef.current.formatMessage({
                  id: 'pages.resourceCatalog.list.error.unknown',
                  defaultMessage: '未知错误',
                }),
              ),
            },
          ),
        );
        return null;
      } finally {
        setDetailLoading(false);
      }
    },
    [fetchSemanticVersions, versionPage, versionPageSize],
  );

  const handleViewDetail = useCallback(
    async (resourceKey: string) => {
      const detail = await loadResourceDetail(resourceKey);
      if (detail) {
        setDetailVisible(true);
      }
    },
    [loadResourceDetail],
  );

  const handleEditSemantics = useCallback(
    async (resourceKey: string) => {
      const detail = await loadResourceDetail(resourceKey);
      if (!detail) {
        return;
      }
      editForm.setFieldsValue(semanticsToFormValues(detail.semantics));
      setEditVisible(true);
    },
    [editForm, loadResourceDetail],
  );

  const handleSaveSemantics = useCallback(async () => {
    if (!selectedResource) {
      return;
    }

    try {
      const values = await editForm.validateFields();
      await updateResourceSemantics(selectedResource.resourceKey, compactSemanticsPayload(values));
      message.success(
        intlRef.current.formatMessage({
          id: 'pages.resourceCatalog.list.message.semanticsSaved',
          defaultMessage: '语义更新成功',
        }),
      );
      setEditVisible(false);
      await loadResourceDetail(selectedResource.resourceKey);
      fetchData();
    } catch (error) {
      message.error(
        intlRef.current.formatMessage(
          {
            id: 'pages.resourceCatalog.list.error.updateFailed',
            defaultMessage: '更新失败: {message}',
          },
          {
            message: extractErrorMessage(
              error,
              intlRef.current.formatMessage({
                id: 'pages.resourceCatalog.list.error.unknown',
                defaultMessage: '未知错误',
              }),
            ),
          },
        ),
      );
    }
  }, [editForm, fetchData, loadResourceDetail, selectedResource]);

  const handleOpenResolve = useCallback(
    (conflict: SemanticConflictInfo) => {
      const sources = conflictSources(conflict);
      setSelectedConflict(conflict);
      resolveForm.setFieldsValue({
        chosenSource: sources[0],
        reason: '',
      });
      setResolveVisible(true);
    },
    [resolveForm],
  );

  const handleResolveConflict = useCallback(async () => {
    if (!selectedResource || !selectedConflict) {
      return;
    }

    try {
      const values = await resolveForm.validateFields();
      await resolveResourceSemanticConflict(
        selectedResource.resourceKey,
        selectedConflict.field,
        values,
      );
      message.success(
        intlRef.current.formatMessage({
          id: 'pages.resourceCatalog.list.message.conflictResolved',
          defaultMessage: '冲突已解决，相关 Proposal 已触发重算',
        }),
      );
      setResolveVisible(false);
      await loadResourceDetail(selectedResource.resourceKey);
      fetchData();
    } catch (error) {
      message.error(
        intlRef.current.formatMessage(
          {
            id: 'pages.resourceCatalog.list.error.resolveConflictFailed',
            defaultMessage: '解决冲突失败: {message}',
          },
          {
            message: extractErrorMessage(
              error,
              intlRef.current.formatMessage({
                id: 'pages.resourceCatalog.list.error.unknown',
                defaultMessage: '未知错误',
              }),
            ),
          },
        ),
      );
    }
  }, [fetchData, loadResourceDetail, resolveForm, selectedConflict, selectedResource]);

  const handleOpenProposals = useCallback((resourceKey: string) => {
    history.push(`/functions/pages?resourceKey=${encodeURIComponent(resourceKey)}`);
  }, []);

  const categoryOptions = data
    .map((item) => item.categoryKey)
    .filter((item): item is string => Boolean(item))
    .filter((item, index, all) => all.indexOf(item) === index)
    .sort();

  const columns: ColumnsType<ResourceCatalogItem> = [
    {
      title: intl.formatMessage({
        id: 'pages.resourceCatalog.list.column.resourceKey',
        defaultMessage: '资源标识',
      }),
      dataIndex: 'resourceKey',
      key: 'resourceKey',
      render: (text: string) => <Text strong>{text}</Text>,
    },
    {
      title: intl.formatMessage({
        id: 'pages.resourceCatalog.list.column.labels',
        defaultMessage: '名称',
      }),
      dataIndex: 'labels',
      key: 'labels',
      render: (labels: ResourceCatalogItem['labels']) => localizedText(labels, 'zh-CN', '-'),
    },
    {
      title: intl.formatMessage({
        id: 'pages.resourceCatalog.list.column.category',
        defaultMessage: '分类',
      }),
      dataIndex: 'categoryKey',
      key: 'categoryKey',
      render: (text?: string) => text || '-',
    },
    {
      title: intl.formatMessage({
        id: 'pages.resourceCatalog.list.column.status',
        defaultMessage: '状态',
      }),
      dataIndex: 'status',
      key: 'status',
      render: (status: ResourceCatalogItem['status']) => (
        <Tag color={statusColors[status]}>{statusLabels[status]}</Tag>
      ),
    },
    {
      title: intl.formatMessage({
        id: 'pages.resourceCatalog.list.column.functionCount',
        defaultMessage: '函数数量',
      }),
      dataIndex: 'functions',
      key: 'functions',
      render: (functions: FunctionInfo[]) => functions?.length || 0,
    },
    {
      title: intl.formatMessage({
        id: 'pages.resourceCatalog.list.column.semanticsVersion',
        defaultMessage: '语义版本',
      }),
      dataIndex: 'semantics',
      key: 'semantics',
      render: (semantics?: SemanticsInfo) => semantics?.version || '-',
    },
    {
      title: intl.formatMessage({
        id: 'pages.resourceCatalog.list.column.diagnostics',
        defaultMessage: '诊断',
      }),
      dataIndex: 'diagnostics',
      key: 'diagnostics',
      render: (diagnostics?: DiagnosticInfo[]) => {
        if (!diagnostics || diagnostics.length === 0) {
          return (
            <Tag color="success">
              <FormattedMessage
                id="pages.resourceCatalog.list.diagnostics.none"
                defaultMessage="无"
              />
            </Tag>
          );
        }
        const errors = diagnostics.filter((diagnostic) => diagnostic.severity === 'error').length;
        const warnings = diagnostics.filter(
          (diagnostic) => diagnostic.severity === 'warning',
        ).length;
        return (
          <Space>
            {errors > 0 && (
              <Tag color="error">
                {intl.formatMessage(
                  {
                    id: 'pages.resourceCatalog.list.diagnostics.errorCount',
                    defaultMessage: '{count} 错误',
                  },
                  { count: errors },
                )}
              </Tag>
            )}
            {warnings > 0 && (
              <Tag color="warning">
                {intl.formatMessage(
                  {
                    id: 'pages.resourceCatalog.list.diagnostics.warningCount',
                    defaultMessage: '{count} 警告',
                  },
                  { count: warnings },
                )}
              </Tag>
            )}
          </Space>
        );
      },
    },
    {
      title: intl.formatMessage({
        id: 'pages.resourceCatalog.list.column.actions',
        defaultMessage: '操作',
      }),
      key: 'action',
      fixed: 'right',
      width: 120,
      render: (_, record) => (
        <Space size={0}>
          <Tooltip
            title={intl.formatMessage({
              id: 'pages.resourceCatalog.list.tooltip.viewDetail',
              defaultMessage: '查看详情',
            })}
          >
            <Button
              type="link"
              size="small"
              icon={<EyeOutlined />}
              onClick={() => handleViewDetail(record.resourceKey)}
            />
          </Tooltip>
          <Tooltip
            title={intl.formatMessage({
              id: 'pages.resourceCatalog.list.tooltip.editSemantics',
              defaultMessage: '编辑语义',
            })}
          >
            <Button
              type="link"
              size="small"
              icon={<EditOutlined />}
              onClick={() => handleEditSemantics(record.resourceKey)}
            />
          </Tooltip>
          <Tooltip
            title={intl.formatMessage({
              id: 'pages.resourceCatalog.list.tooltip.proposals',
              defaultMessage: '提案',
            })}
          >
            <Button
              type="link"
              size="small"
              icon={<BulbOutlined />}
              onClick={() => handleOpenProposals(record.resourceKey)}
            />
          </Tooltip>
        </Space>
      ),
    },
  ];

  return (
    <div>
      <Card style={{ marginBottom: 16 }}>
        <Space wrap>
          <Input
            placeholder={intl.formatMessage({
              id: 'pages.resourceCatalog.list.search.placeholder',
              defaultMessage: '搜索资源',
            })}
            prefix={<SearchOutlined />}
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            onPressEnter={fetchData}
            style={{ width: 220 }}
          />
          <Select
            placeholder={intl.formatMessage({
              id: 'pages.resourceCatalog.list.search.categoryPlaceholder',
              defaultMessage: '选择分类',
            })}
            value={category || undefined}
            onChange={(value) => setCategory(value || '')}
            allowClear
            style={{ width: 180 }}
            options={categoryOptions.map((item) => ({
              value: item,
              label: item,
            }))}
          />
          <Button type="primary" icon={<SearchOutlined />} onClick={fetchData}>
            <FormattedMessage id="pages.resourceCatalog.list.button.search" defaultMessage="搜索" />
          </Button>
          <Button icon={<ReloadOutlined />} onClick={fetchData}>
            <FormattedMessage
              id="pages.resourceCatalog.list.button.refresh"
              defaultMessage="刷新"
            />
          </Button>
        </Space>
      </Card>

      <Card
        title={intl.formatMessage({
          id: 'pages.resourceCatalog.list.card.title',
          defaultMessage: '资源能力目录',
        })}
      >
        <Table
          columns={columns}
          dataSource={data}
          rowKey="resourceKey"
          loading={loading}
          scroll={{ x: 1100 }}
          pagination={{
            total,
            pageSize: 20,
            showSizeChanger: true,
            showTotal: (value) =>
              intl.formatMessage(
                {
                  id: 'pages.resourceCatalog.list.pagination.total',
                  defaultMessage: '共 {total} 条',
                },
                { total: value },
              ),
          }}
        />
      </Card>

      <ResourceDetailModal
        open={detailVisible}
        resource={selectedResource}
        semanticMeta={semanticMeta}
        semanticVersions={semanticVersions}
        loading={detailLoading}
        versionPage={versionPage}
        versionPageSize={versionPageSize}
        onClose={() => setDetailVisible(false)}
        onOpenProposals={handleOpenProposals}
        onResolveConflict={handleOpenResolve}
        onVersionPageChange={(page, pageSize) => {
          if (!selectedResource) return;
          setVersionPage(page);
          setVersionPageSize(pageSize);
          fetchSemanticVersions(selectedResource.resourceKey, page, pageSize);
        }}
      />

      <EditSemanticsModal
        open={editVisible}
        form={editForm}
        functions={selectedResource?.functions || []}
        onOk={handleSaveSemantics}
        onCancel={() => setEditVisible(false)}
      />

      <ResolveConflictModal
        open={resolveVisible}
        form={resolveForm}
        conflict={selectedConflict}
        onOk={handleResolveConflict}
        onCancel={() => setResolveVisible(false)}
      />
    </div>
  );
};

export default ResourceCatalogPage;
