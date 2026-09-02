import { useCallback, useEffect, useMemo, useState } from 'react';
import { App, Form, Modal } from 'antd';
import { history } from '@umijs/max';
import {
  getFunctionDetail,
  getFunctionOpenAPI,
  updateFunction,
  deleteFunction,
  copyFunction,
  enableFunction,
  disableFunction,
  getFunctionPermissions,
  updateFunctionPermissions,
  listDescriptors,
  type FunctionPermission,
  type FunctionDescriptor,
} from '@/services/api/functions';
import type { JSONSchema, JSONValue } from '@/types/dashboard';
import { localizedText } from '@/utils/localizedText';

export interface FunctionDetail {
  id: string;
  name?: string;
  description?: string;
  resource?: string;
  operation?: string;
  version?: string;
  enabled: boolean;
  tags?: string[];
  createdAt: string;
  updatedAt: string;
  provider?: string;
  agentCount?: number;
  health?: 'healthy' | 'unhealthy' | 'unknown';
  descriptor?: FunctionDescriptor;
  permissions?: JSONValue;
  config?: JSONValue;
}

type OpenAPIOperationPreview = {
  extensions?: Record<string, string | number | boolean | null | undefined>;
  requestBody?: JSONValue;
};

// 契约：GET /api/v1/functions/descriptors -> { functions: [...] }；兼容裸数组。
type DescriptorListResponse =
  | FunctionDescriptor[]
  | { functions?: FunctionDescriptor[] }
  | { descriptors?: FunctionDescriptor[] };

type FunctionEditValues = {
  name?: string;
  description?: string;
  resource?: string;
  tags?: string;
};

function parseMaybeJSON(value: unknown): JSONSchema | undefined {
  if (!value) return undefined;
  if (typeof value === 'string') {
    try {
      const parsed = JSON.parse(value);
      return parsed && typeof parsed === 'object' && !Array.isArray(parsed)
        ? (parsed as JSONSchema)
        : undefined;
    } catch {
      return undefined;
    }
  }
  if (typeof value === 'object' && !Array.isArray(value)) return value as JSONSchema;
  return undefined;
}

function toDescriptorArray(input: DescriptorListResponse): FunctionDescriptor[] {
  if (Array.isArray(input)) return input;
  const envelope = input as {
    functions?: FunctionDescriptor[];
    descriptors?: FunctionDescriptor[];
  };
  if (Array.isArray(envelope.functions)) return envelope.functions;
  if (Array.isArray(envelope.descriptors)) return envelope.descriptors;
  return [];
}

export default function useFunctionDetailPage(functionId?: string) {
  const { message } = App.useApp();
  const [loading, setLoading] = useState(false);
  const [functionDetail, setFunctionDetail] = useState<FunctionDetail | null>(null);
  const [editing, setEditing] = useState(false);
  const [form] = Form.useForm();
  const [permLoading, setPermLoading] = useState(false);
  const [permSaving, setPermSaving] = useState(false);
  const [permError, setPermError] = useState<string>('');
  const [permForm] = Form.useForm();
  const [descriptorIndexItem, setDescriptorIndexItem] = useState<FunctionDescriptor | null>(null);
  const [openapiOperation, setOpenapiOperation] = useState<OpenAPIOperationPreview | null>(null);

  const parsedInputSchema = useMemo(() => {
    const detailDesc = functionDetail?.descriptor;
    const indexDesc = descriptorIndexItem;
    const requestBody = openapiOperation?.requestBody;
    const bodySchema =
      requestBody && typeof requestBody === 'object' && 'content' in requestBody
        ? (requestBody as { content?: Record<string, { schema?: JSONSchema }> }).content?.[
            'application/json'
          ]?.schema
        : undefined;
    return (
      parseMaybeJSON(detailDesc?.inputSchema) ||
      parseMaybeJSON(indexDesc?.inputSchema) ||
      parseMaybeJSON(detailDesc?.schema) ||
      parseMaybeJSON(indexDesc?.schema) ||
      parseMaybeJSON(detailDesc?.params) ||
      parseMaybeJSON(indexDesc?.params) ||
      bodySchema
    );
  }, [functionDetail?.descriptor, descriptorIndexItem, openapiOperation]);

  const effectiveResource = useMemo(() => {
    const direct = String(functionDetail?.resource || '').trim();
    if (direct) return direct;
    const fromIndex = String(descriptorIndexItem?.resource || '').trim();
    if (fromIndex) return fromIndex;
    const fromDetailDesc = String(functionDetail?.descriptor?.resource || '').trim();
    if (fromDetailDesc) return fromDetailDesc;
    const fromOpenapi = String(openapiOperation?.extensions?.['x-resource'] || '').trim();
    if (fromOpenapi) return fromOpenapi;
    return '';
  }, [functionDetail?.resource, functionDetail?.descriptor, descriptorIndexItem, openapiOperation]);

  const jsonViewData = useMemo(
    () => ({
      function: functionDetail
        ? {
            id: functionDetail.id,
            name: functionDetail.name,
            description: functionDetail.description,
            resource: effectiveResource,
            operation: functionDetail.operation,
            version: functionDetail.version,
            enabled: functionDetail.enabled,
            tags: functionDetail.tags || [],
            provider: functionDetail.provider,
          }
        : null,
      descriptorFromDetailApi: functionDetail?.descriptor || null,
      descriptorFromIndexApi: descriptorIndexItem || null,
      openapiOperation: openapiOperation || null,
    }),
    [functionDetail, descriptorIndexItem, openapiOperation, effectiveResource],
  );

  const formDescriptor = useMemo(() => {
    const detailDesc = functionDetail?.descriptor;
    const indexDesc = descriptorIndexItem;
    return {
      ...(detailDesc || {}),
      ...(indexDesc || {}),
      resource: indexDesc?.resource || detailDesc?.resource || functionDetail?.resource,
      operation: indexDesc?.operation || detailDesc?.operation,
    };
  }, [functionDetail?.descriptor, functionDetail?.resource, descriptorIndexItem]);

  const loadSourceOfTruth = useCallback(async (id: string) => {
    let indexItem: FunctionDescriptor | null = null;
    try {
      const [descsRes, openapiRes] = await Promise.allSettled([
        listDescriptors(),
        getFunctionOpenAPI(id),
      ]);
      if (descsRes.status === 'fulfilled') {
        const descArray = toDescriptorArray(descsRes.value as DescriptorListResponse);
        indexItem = descArray.find((descriptor) => descriptor.id === id) || null;
        setDescriptorIndexItem(indexItem);
      } else {
        setDescriptorIndexItem(null);
      }
      if (openapiRes.status === 'fulfilled') {
        setOpenapiOperation((openapiRes.value || null) as OpenAPIOperationPreview | null);
      } else {
        setOpenapiOperation(null);
      }
    } catch {
      setDescriptorIndexItem(null);
      setOpenapiOperation(null);
    }
    return indexItem;
  }, []);

  const loadDetail = useCallback(async () => {
    if (!functionId) return;
    setLoading(true);
    try {
      const detail = await getFunctionDetail(functionId);
      const indexItem = await loadSourceOfTruth(functionId);
      const normalizedDetail: FunctionDetail = {
        id: detail.id,
        name: localizedText(detail.displayName, 'zh-CN', detail.id),
        description: localizedText(
          detail.summary,
          'zh-CN',
          localizedText(detail.description, 'zh-CN', ''),
        ),
        resource: detail.resource || indexItem?.resource,
        operation: detail.operation || indexItem?.operation,
        version: detail.version,
        enabled: true,
        tags: detail.tags || [],
        createdAt: '',
        updatedAt: '',
        provider: 'runtime',
        health: 'healthy',
        descriptor: detail,
      };
      setFunctionDetail(normalizedDetail);
      form.setFieldsValue({
        name: normalizedDetail.name,
        description: normalizedDetail.description,
        resource: normalizedDetail.resource || '',
        tags: normalizedDetail.tags?.join(', '),
      });

      setPermError('');
      setPermLoading(true);
      try {
        const res = await getFunctionPermissions(functionId);
        const items = Array.isArray(res?.items) ? res.items : [];
        permForm.setFieldsValue({
          items: items.length
            ? items
            : [{ resource: 'function', actions: ['invoke'], roles: [] } as FunctionPermission],
        });
      } catch (e) {
        const errMsg = e instanceof Error ? e.message : '加载函数权限失败';
        permForm.setFieldsValue({ items: [] });
        setPermError(errMsg);
      } finally {
        setPermLoading(false);
      }
    } catch (error) {
      const errObj = error as { response?: { status?: number } };
      if (errObj?.response?.status === 400 || errObj?.response?.status === 404) {
        try {
          const descs = await listDescriptors();
          const descArray = toDescriptorArray(descs as DescriptorListResponse);
          const desc = descArray.find((descriptor) => descriptor.id === functionId);
          if (desc) {
            const detailFromDesc: FunctionDetail = {
              id: desc.id,
              name: localizedText(desc.displayName, 'zh-CN', desc.id),
              description: localizedText(
                desc.summary,
                'zh-CN',
                localizedText(desc.description, 'zh-CN', ''),
              ),
              resource: desc.resource,
              operation: desc.operation,
              version: desc.version || '1.0.0',
              enabled: true,
              tags: desc.tags || [],
              createdAt: '',
              updatedAt: '',
              provider: 'runtime',
              health: 'healthy',
              descriptor: desc,
            };
            await loadSourceOfTruth(functionId);
            setFunctionDetail(detailFromDesc);
            form.setFieldsValue({
              name: detailFromDesc.name,
              description: detailFromDesc.description,
              resource: detailFromDesc.resource || '',
              tags: detailFromDesc.tags?.join(', '),
            });
            permForm.setFieldsValue({ items: [] });
            setPermError('运行时注册的函数不支持权限管理');
          } else {
            message.error('函数不存在');
          }
        } catch {
          message.error('加载函数详情失败');
        }
      } else {
        message.error('加载函数详情失败');
      }
    } finally {
      setLoading(false);
    }
  }, [functionId, message, form, permForm, loadSourceOfTruth]);

  useEffect(() => {
    loadDetail();
  }, [loadDetail]);

  const handleSave = async (values: FunctionEditValues) => {
    if (!functionId) return;
    try {
      await updateFunction(functionId, {
        name: values.name,
        description: values.description,
        resource: values.resource,
        tags: values.tags
          ? values.tags
              .split(',')
              .map((t: string) => t.trim())
              .filter(Boolean)
          : [],
      });
      message.success('保存成功');
      setEditing(false);
      loadDetail();
    } catch {
      message.error('保存失败');
    }
  };

  const handleStatusToggle = async (enabled: boolean) => {
    if (!functionId) return;
    try {
      if (enabled) await enableFunction(functionId);
      else await disableFunction(functionId);
      message.success(enabled ? '函数已启用' : '函数已禁用');
      loadDetail();
    } catch {
      message.error('状态更新失败');
    }
  };

  const handleCopy = async () => {
    if (!functionId) return;
    try {
      const next = await copyFunction(functionId);
      message.success(`复制成功，新函数ID: ${next.functionId}`);
      history.push(`/functions/${next.functionId}`);
    } catch {
      message.error('复制失败');
    }
  };

  const handleDelete = () => {
    if (!functionId) return;
    Modal.confirm({
      title: '确认删除',
      content: '确定要删除这个函数吗？此操作不可恢复！',
      okType: 'danger',
      onOk: async () => {
        try {
          await deleteFunction(functionId);
          message.success('删除成功');
          history.push('/functions/catalog');
        } catch {
          message.error('删除失败');
        }
      },
    });
  };

  const handleSavePermissions = async () => {
    if (!functionId) return;
    try {
      setPermSaving(true);
      const values = await permForm.validateFields();
      const items = (values?.items || []) as FunctionPermission[];
      await updateFunctionPermissions(functionId, items);
      message.success('权限已更新');
    } catch (e) {
      const errMsg = e instanceof Error ? e.message : '更新失败';
      message.error(errMsg);
    } finally {
      setPermSaving(false);
    }
  };

  // F13：契约诊断（schema_breaking_change 等），来自 FunctionContract
  const contractDiagnostics = useMemo(
    () => descriptorIndexItem?.diagnostics ?? [],
    [descriptorIndexItem],
  );

  return {
    loading,
    functionDetail,
    editing,
    setEditing,
    form,
    permLoading,
    permSaving,
    permError,
    permForm,
    parsedInputSchema,
    effectiveResource,
    jsonViewData,
    formDescriptor,
    loadDetail,
    handleSave,
    handleStatusToggle,
    handleCopy,
    handleDelete,
    handleSavePermissions,
    contractDiagnostics,
  };
}
