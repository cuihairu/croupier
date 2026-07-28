import React, { useCallback, useEffect, useMemo, useState } from 'react';
import {
  Alert,
  App,
  Button,
  Card,
  Col,
  Empty,
  Popconfirm,
  Row,
  Select,
  Space,
  Spin,
  Switch,
  Tag,
  Typography,
} from 'antd';
import { ReloadOutlined } from '@ant-design/icons';
import { history, useModel } from '@umijs/max';
import {
  fetchFunctionFormHistory,
  listDescriptors,
  rollbackFunctionFormSchema,
  saveFunctionFormSchema,
  type FunctionDescriptor,
  type FunctionFormHistoryItem,
} from '@/services/api/functions';
import SchemaRenderer from '@/components/formily/SchemaRenderer';
import UISchemaEditor from '@/components/UISchemaEditor';
import type { FormilySchema } from '@/components/formily/schema/types';
import { fetchOptions } from '@/services/schema/async';
import { fetchFunctionFormSchemaDocument } from '@/services/schema';
import { validateFormilySchema } from '@/services/schema/validateSchema';
import { extractErrorMessage } from '@/utils/errors';
import type { FormilyValues } from '@/components/formily/schema/types';

interface FunctionFormManagerProps {
  functionId: string;
  jsonSchema?: unknown;
  descriptor?: Partial<FunctionDescriptor>;
  onSave?: (formSchema: { schema?: FormilySchema; clearCustom?: boolean }) => Promise<void>;
}

type FormConfig = {
  schema?: FormilySchema;
};

const sourceMeta: Record<string, { label: string; color: string }> = {
  custom_metadata: { label: '自定义元数据', color: 'blue' },
  config_file_override: { label: '配置文件覆盖', color: 'purple' },
  generated_default: { label: '生成默认值', color: 'gold' },
  none: { label: '未配置', color: 'default' },
};

const isObject = (value: unknown): value is Record<string, unknown> =>
  !!value && typeof value === 'object' && !Array.isArray(value);

type InitialStateWithAccess = {
  currentUser?: {
    access?: string;
  };
};

export default function FunctionFormManager({
  functionId,
  jsonSchema,
  descriptor,
  onSave,
}: FunctionFormManagerProps) {
  const { Text } = Typography;
  const { message } = App.useApp();
  const { initialState } = useModel('@@initialState');
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [formConfig, setFormConfig] = useState<FormConfig>({});
  const [useCustomForm, setUseCustomForm] = useState(false);
  const [hasDefaultForm, setHasDefaultForm] = useState(false);
  const [formSource, setFormSource] = useState<string>('none');
  const [formSourceDetail, setFormSourceDetail] = useState('');
  const [updatedAt, setUpdatedAt] = useState('');
  const [formError, setFormError] = useState('');
  const [isDirty, setIsDirty] = useState(false);
  const [historyItems, setHistoryItems] = useState<FunctionFormHistoryItem[]>([]);
  const [historyLoading, setHistoryLoading] = useState(false);
  const [rollbackVersion, setRollbackVersion] = useState<number | undefined>();
  const [relatedFunctions, setRelatedFunctions] = useState<{ id: string; name: string }[]>([]);
  const [batchTargets, setBatchTargets] = useState<string[]>([]);
  const [batchSaving, setBatchSaving] = useState(false);
  const [previewValue, setPreviewValue] = useState<FormilyValues>({});

  const sourceDisplay = sourceMeta[formSource] || { label: formSource || '未知', color: 'default' };
  const access = String(
    (initialState as InitialStateWithAccess | undefined)?.currentUser?.access || '',
  );
  const permissions = useMemo(
    () =>
      access
        .split(',')
        .map((item) => item.trim())
        .filter(Boolean),
    [access],
  );

  const rendererContext = useMemo(
    () => ({
      gameId:
        typeof window !== 'undefined' ? localStorage.getItem('game_id') || undefined : undefined,
      env: typeof window !== 'undefined' ? localStorage.getItem('env') || undefined : undefined,
      functionId,
      permissions,
    }),
    [functionId, permissions],
  );

  const rendererScope = useMemo(
    () => ({
      hasPerm: (perm: string) => permissions.includes(perm),
      fetchOptions,
    }),
    [permissions],
  );

  const loadFormHistory = useCallback(async () => {
    if (!functionId) return;
    setHistoryLoading(true);
    try {
      const response = await fetchFunctionFormHistory(functionId);
      const items = Array.isArray(response?.items) ? response.items : [];
      setHistoryItems(items);
      setRollbackVersion(items[0]?.version);
    } catch {
      setHistoryItems([]);
      setRollbackVersion(undefined);
    } finally {
      setHistoryLoading(false);
    }
  }, [functionId]);

  const loadFormConfig = useCallback(async () => {
    if (!functionId) return;
    setLoading(true);
    setFormError('');
    try {
      const response = await fetchFunctionFormSchemaDocument(functionId);
      const schema = response?.schema;
      if (schema) {
        const validation = validateFormilySchema(schema);
        if (!validation.ok) {
          throw new Error(validation.error || '函数表单 Schema 不是有效 Formily Schema');
        }
      }
      setFormConfig({
        schema,
      });
      setUseCustomForm(!!response?.custom);
      setHasDefaultForm(
        typeof response?.hasDefault === 'boolean' ? response.hasDefault : Boolean(schema),
      );
      setFormSource(response?.formSource || 'none');
      setFormSourceDetail(response?.formSourceDetail || '');
      setUpdatedAt(response?.updatedAt || '');
      setIsDirty(false);
      setPreviewValue({});
    } catch (error: unknown) {
      const msg = extractErrorMessage(error, '加载函数表单配置失败');
      setFormError(msg);
      setFormConfig({});
      message.error(msg);
    } finally {
      setLoading(false);
    }
  }, [functionId, message]);

  useEffect(() => {
    loadFormConfig();
    loadFormHistory();
  }, [loadFormConfig, loadFormHistory]);

  useEffect(() => {
    const resourceKey = String(descriptor?.resource || '').trim().toLowerCase();
    if (!resourceKey) {
      setRelatedFunctions([]);
      setBatchTargets([]);
      return;
    }
    listDescriptors()
      .then((items) => {
        const sameResource = items
          .filter((item) => {
            const currentResource = String(item?.resource || '').trim().toLowerCase();
            return item.id && currentResource === resourceKey && item.id !== functionId;
          })
          .map((item) => ({
            id: item.id,
            name: item.displayName?.zh || item.displayName?.en || item.summary?.zh || item.id,
          }));
        setRelatedFunctions(sameResource);
        setBatchTargets((prev) => prev.filter((id) => sameResource.some((item) => item.id === id)));
      })
      .catch(() => setRelatedFunctions([]));
  }, [descriptor?.resource, functionId]);

  const handleSave = async (schema?: FormilySchema) => {
    if (!onSave || !schema) {
      message.warning('没有可保存的 Formily Schema');
      return;
    }
    const validation = validateFormilySchema(schema);
    if (!validation.ok) {
      message.error(validation.error || '函数表单 Schema 校验失败');
      return;
    }

    setSaving(true);
    try {
      await onSave({ schema });
      message.success('函数表单配置保存成功');
      await loadFormConfig();
      await loadFormHistory();
    } catch (error: unknown) {
      message.error(extractErrorMessage(error, '保存失败'));
    } finally {
      setSaving(false);
    }
  };

  const handleToggleCustomForm = async (checked: boolean) => {
    if (!onSave) {
      message.warning('未配置保存回调');
      return;
    }
    if (checked && !formConfig.schema) {
      message.error('当前没有可启用的 Formily Schema');
      return;
    }

    setSaving(true);
    try {
      if (checked) {
        await onSave({ schema: formConfig.schema });
      } else {
        await onSave({ clearCustom: true });
      }
      await loadFormConfig();
      await loadFormHistory();
      message.success(checked ? '已启用自定义表单' : '已清除自定义表单');
    } catch (error: unknown) {
      message.error(extractErrorMessage(error, '操作失败'));
    } finally {
      setSaving(false);
    }
  };

  const handleBatchApply = async () => {
    if (!formConfig.schema) {
      message.error('当前没有可同步的 Formily Schema');
      return;
    }
    const validation = validateFormilySchema(formConfig.schema);
    if (!validation.ok) {
      message.error(validation.error || '函数表单 Schema 校验失败');
      return;
    }
    if (batchTargets.length === 0) {
      message.warning('请先选择目标函数');
      return;
    }

    setBatchSaving(true);
    try {
      const results = await Promise.allSettled(
        batchTargets.map((targetId) =>
          saveFunctionFormSchema(targetId, {
            schema: formConfig.schema,
          }),
        ),
      );
      const ok = results.filter((result) => result.status === 'fulfilled').length;
      const fail = results.length - ok;
      if (fail === 0) {
        message.success(`已同步 Formily 表单到 ${ok} 个函数`);
      } else {
        message.warning(`同步完成：成功 ${ok}，失败 ${fail}`);
      }
    } finally {
      setBatchSaving(false);
    }
  };

  const handleEditorChange = (schema: FormilySchema) => {
    setFormConfig((prev) => ({ ...prev, schema }));
    setPreviewValue({});
    setIsDirty(true);
  };

  if (loading) {
    return (
      <Card>
        <div style={{ textAlign: 'center', padding: 40 }}>
          <Spin />
          <div style={{ marginTop: 12, color: 'rgba(0,0,0,0.45)' }}>加载函数表单...</div>
        </div>
      </Card>
    );
  }

  return (
    <Card
      title="函数 Formily 表单管理"
      extra={
        <Space wrap>
          <Tag color={useCustomForm ? 'blue' : hasDefaultForm ? 'green' : 'default'}>
            {useCustomForm ? '当前: 自定义' : hasDefaultForm ? '当前: 默认' : '当前: 未配置'}
          </Tag>
          <Tag color={sourceDisplay.color}>来源: {sourceDisplay.label}</Tag>
          <Button icon={<ReloadOutlined />} onClick={loadFormConfig} loading={loading}>
            刷新
          </Button>
          <Select
            size="small"
            style={{ minWidth: 220 }}
            placeholder="选择历史版本"
            loading={historyLoading}
            value={rollbackVersion}
            onChange={setRollbackVersion}
            options={historyItems.map((item) => ({
              label: `v${item.version} ${
                item.createdAt ? new Date(item.createdAt).toLocaleString('zh-CN') : ''
              }`,
              value: item.version,
            }))}
          />
          <Popconfirm
            title="确认回滚函数表单配置？"
            description={rollbackVersion ? `将回滚到版本 v${rollbackVersion}` : '请选择版本'}
            onConfirm={async () => {
              if (!rollbackVersion) return;
              try {
                setSaving(true);
                await rollbackFunctionFormSchema(functionId, rollbackVersion);
                message.success(`已回滚到版本 v${rollbackVersion}`);
                await loadFormConfig();
                await loadFormHistory();
              } catch (error: unknown) {
                message.error(extractErrorMessage(error, '回滚失败'));
              } finally {
                setSaving(false);
              }
            }}
            okButtonProps={{ disabled: !rollbackVersion }}
          >
            <Button size="small" disabled={!rollbackVersion} loading={saving}>
              回滚
            </Button>
          </Popconfirm>
          <Button
            type="primary"
            onClick={() =>
              history.push(`/system/functions/${encodeURIComponent(functionId)}/form-designer`)
            }
          >
            打开函数表单设计器
          </Button>
          {(hasDefaultForm || useCustomForm) && (
            <Space>
              <span>自定义表单:</span>
              <Switch
                checked={useCustomForm}
                onChange={handleToggleCustomForm}
                loading={saving}
                checkedChildren="启用"
                unCheckedChildren="清除"
              />
            </Space>
          )}
        </Space>
      }
    >
      <Alert
        message="这里只管理单个函数的 Formily 输入表单"
        description="分页、表格、详情、行操作和多函数组合属于 Dashboard Page，不在函数表单中表达。"
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
      />

      {formError && (
        <Alert
          type="error"
          showIcon
          message="函数表单 Schema 无效"
          description={formError}
          style={{ marginBottom: 16 }}
        />
      )}

      {formSourceDetail && (
        <Alert
          message="表单来源详情"
          description={formSourceDetail}
          type="info"
          showIcon
          style={{ marginBottom: 16 }}
        />
      )}

      {updatedAt && (
        <Alert
          message="最近更新时间"
          description={new Date(updatedAt).toLocaleString('zh-CN')}
          type="success"
          showIcon
          style={{ marginBottom: 16 }}
        />
      )}

      {isObject(descriptor) && (descriptor.resource || descriptor.operation) && (
        <Card size="small" style={{ marginBottom: 16 }} title="函数归属">
          <Space wrap>
            <Tag color="blue">Resource: {String(descriptor.resource || '-')}</Tag>
            <Tag color="purple">Operation: {String(descriptor.operation || '未声明')}</Tag>
            <Text type="secondary">Page 模型组合这些动作；函数表单不生成菜单、分页或 CRUD 页面。</Text>
          </Space>
        </Card>
      )}

      {formConfig.schema ? (
        <Row gutter={16}>
          <Col xs={24} xl={13}>
            <UISchemaEditor
              value={formConfig.schema}
              onChange={handleEditorChange}
              jsonSchema={jsonSchema}
            />
          </Col>
          <Col xs={24} xl={11}>
            <Card
              title="函数表单预览"
              extra={
                <Space>
                  <Button
                    size="small"
                    disabled={!isDirty}
                    loading={saving}
                    onClick={() => handleSave(formConfig.schema)}
                  >
                    保存当前函数
                  </Button>
                  <Button size="small" onClick={() => setPreviewValue({})}>
                    清空预览数据
                  </Button>
                </Space>
              }
            >
              <SchemaRenderer
                schema={formConfig.schema}
                value={previewValue}
                onChange={setPreviewValue}
                context={rendererContext}
                scope={rendererScope}
              />
            </Card>

            {relatedFunctions.length > 0 && (
              <Card size="small" title="同资源函数同步" style={{ marginTop: 16 }}>
                <Space direction="vertical" style={{ width: '100%' }}>
                  <Select
                    mode="multiple"
                    maxTagCount={2}
                    style={{ width: '100%' }}
                    placeholder="选择同资源函数"
                    value={batchTargets}
                    onChange={setBatchTargets}
                    options={relatedFunctions.map((item) => ({
                      label: item.name,
                      value: item.id,
                    }))}
                  />
                  <Button
                    type="primary"
                    loading={batchSaving}
                    disabled={batchTargets.length === 0}
                    onClick={handleBatchApply}
                  >
                    同步当前 Formily 表单
                  </Button>
                </Space>
              </Card>
            )}
          </Col>
        </Row>
      ) : (
        <Empty description="该函数没有可用 Formily 表单 Schema；请先补齐 input_schema，或在函数表单设计器中创建 override。" />
      )}
    </Card>
  );
}
