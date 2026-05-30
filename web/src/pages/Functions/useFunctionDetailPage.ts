import { useEffect, useMemo, useState } from 'react';
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
  fetchFunctionRoute,
  saveFunctionUiSchema,
  saveFunctionRoute,
  listDescriptors,
  type FunctionPermission,
} from '@/services/api/functions';

export interface FunctionDetail {
  id: string;
  name?: string;
  description?: string;
  category?: string;
  version?: string;
  enabled: boolean;
  tags?: string[];
  createdAt: string;
  updatedAt: string;
  provider?: string;
  agentCount?: number;
  health?: 'healthy' | 'unhealthy' | 'unknown';
  descriptor?: any;
  permissions?: any;
  config?: any;
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
  const [routeConfigSaving, setRouteConfigSaving] = useState(false);
  const [routeConfigForm] = Form.useForm();
  const routePreview = Form.useWatch([], routeConfigForm);
  const [descriptorIndexItem, setDescriptorIndexItem] = useState<any>(null);
  const [openapiOperation, setOpenapiOperation] = useState<any>(null);

  const parsedInputSchema = useMemo(() => {
    const detailDesc = functionDetail?.descriptor || {};
    const indexDesc = descriptorIndexItem || {};
    const parseMaybeJSON = (v: any) => {
      if (!v) return undefined;
      if (typeof v === 'string') {
        try {
          return JSON.parse(v);
        } catch {
          return undefined;
        }
      }
      if (typeof v === 'object') return v;
      return undefined;
    };
    return (
      parseMaybeJSON(detailDesc.inputSchema) ||
      parseMaybeJSON(indexDesc.inputSchema) ||
      parseMaybeJSON(detailDesc.schema) ||
      parseMaybeJSON(indexDesc.schema) ||
      parseMaybeJSON(detailDesc.params) ||
      parseMaybeJSON(indexDesc.params) ||
      openapiOperation?.requestBody?.content?.['application/json']?.schema
    );
  }, [functionDetail?.descriptor, descriptorIndexItem, openapiOperation]);

  const effectiveCategory = useMemo(() => {
    const direct = String(functionDetail?.category || '').trim();
    if (direct) return direct;
    const fromIndex = String((descriptorIndexItem as any)?.category || '').trim();
    if (fromIndex) return fromIndex;
    const fromDetailDesc = String((functionDetail?.descriptor as any)?.category || '').trim();
    if (fromDetailDesc) return fromDetailDesc;
    const fromOpenapi = String((openapiOperation as any)?.extensions?.['x-category'] || '').trim();
    if (fromOpenapi) return fromOpenapi;
    return '';
  }, [functionDetail?.category, functionDetail?.descriptor, descriptorIndexItem, openapiOperation]);

  const jsonViewData = useMemo(
    () => ({
      function: functionDetail
        ? {
            id: functionDetail.id,
            name: functionDetail.name,
            description: functionDetail.description,
            category: effectiveCategory,
            version: functionDetail.version,
            enabled: functionDetail.enabled,
            tags: functionDetail.tags || [],
            provider: functionDetail.provider,
          }
        : null,
      descriptor_from_detail_api: functionDetail?.descriptor || null,
      descriptor_from_index_api: descriptorIndexItem || null,
      openapi_operation: openapiOperation || null,
      route: routePreview || null,
    }),
    [functionDetail, descriptorIndexItem, openapiOperation, routePreview, effectiveCategory],
  );

  const uiDescriptor = useMemo(() => {
    const detailDesc = functionDetail?.descriptor || {};
    const indexDesc = descriptorIndexItem || {};
    return {
      ...detailDesc,
      ...indexDesc,
      entity: indexDesc?.entity || detailDesc?.entity,
      operation: indexDesc?.operation || detailDesc?.operation,
      entityDisplay: indexDesc?.entityDisplay || detailDesc?.entityDisplay,
      operationDisplay: indexDesc?.operationDisplay || detailDesc?.operationDisplay,
    };
  }, [functionDetail?.descriptor, descriptorIndexItem]);

  const loadSourceOfTruth = async (id: string) => {
    let indexItem: any = null;
    try {
      const [descsRes, openapiRes] = await Promise.allSettled([
        listDescriptors(),
        getFunctionOpenAPI(id),
      ]);
      if (descsRes.status === 'fulfilled') {
        const descs = descsRes.value;
        const descArray = Array.isArray(descs) ? descs : (descs as any)?.descriptors || [];
        indexItem = descArray.find((d: any) => d.id === id) || null;
        setDescriptorIndexItem(indexItem);
      } else {
        setDescriptorIndexItem(null);
      }
      if (openapiRes.status === 'fulfilled') {
        setOpenapiOperation(openapiRes.value || null);
      } else {
        setOpenapiOperation(null);
      }
    } catch {
      setDescriptorIndexItem(null);
      setOpenapiOperation(null);
    }
    return indexItem;
  };

  const loadDetail = async () => {
    if (!functionId) return;
    setLoading(true);
    try {
      const detail = await getFunctionDetail(functionId);
      const indexItem = await loadSourceOfTruth(functionId);
      const normalizedDetail: FunctionDetail = {
        id: detail.id,
        name: detail.displayName?.zh || detail.displayName?.en || detail.id,
        description: detail.summary?.zh || detail.summary?.en || detail.description || '',
        category: detail.category,
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
      const categoryFromDetail = detail.category || (indexItem as any)?.category || '';
      form.setFieldsValue({
        name: normalizedDetail.name,
        description: normalizedDetail.description,
        category: categoryFromDetail,
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
      } catch (e: any) {
        permForm.setFieldsValue({ items: [] });
        setPermError(e?.message || '加载函数权限失败');
      } finally {
        setPermLoading(false);
      }

      const descriptor = detail || {};
      const menuConfig = descriptor?.menu || {};
      const mergedRoute = {
        nodes: Array.isArray(menuConfig.nodes) ? menuConfig.nodes : [],
        path: menuConfig.path ?? '',
        order: menuConfig.order ?? 10,
        hidden: menuConfig.hidden ?? false,
      };
      try {
        const routeRes = await fetchFunctionRoute(functionId);
        const rm = routeRes?.menu || {};
        mergedRoute.nodes = Array.isArray(rm.nodes) ? rm.nodes : mergedRoute.nodes;
        mergedRoute.path = rm.path ?? mergedRoute.path;
        mergedRoute.order = rm.order ?? mergedRoute.order;
        mergedRoute.hidden = rm.hidden ?? mergedRoute.hidden;
      } catch {
        // keep defaults from descriptor
      }
      routeConfigForm.setFieldsValue(mergedRoute);
    } catch (error: any) {
      if (error?.response?.status === 400 || error?.response?.status === 404) {
        try {
          const descs = await listDescriptors();
          const descArray = Array.isArray(descs) ? descs : (descs as any)?.descriptors || [];
          const desc = descArray.find((d: any) => d.id === functionId);
          if (desc) {
            const detailFromDesc: FunctionDetail = {
              id: desc.id,
              name: desc.displayName?.zh || desc.displayName?.en || desc.id,
              description: desc.summary?.zh || desc.summary?.en || desc.description || '',
              category: desc.category || 'general',
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
              category: detailFromDesc.category,
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
  };

  useEffect(() => {
    loadDetail();
  }, [functionId]);

  const handleSave = async (values: any) => {
    if (!functionId) return;
    try {
      await updateFunction(functionId, {
        name: values.name,
        description: values.description,
        category: values.category,
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
      history.push(`/system/functions/${next.functionId}`);
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
          history.push('/system/functions/catalog');
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
    } catch (e: any) {
      message.error(e?.message || '更新失败');
    } finally {
      setPermSaving(false);
    }
  };

  const handleSaveRoute = async () => {
    if (!functionId) return;
    try {
      setRouteConfigSaving(true);
      const v = await routeConfigForm.validateFields();
      await saveFunctionRoute(functionId, {
        nodes: Array.isArray(v.nodes) ? v.nodes : [],
        path: v.path || '',
        order: v.order ?? 10,
        hidden: !!v.hidden,
      });
      window.dispatchEvent(new CustomEvent('function-route:changed'));
      message.success('路由配置已保存');
    } catch {
      // validation error
    } finally {
      setRouteConfigSaving(false);
    }
  };

  const handleResetRoute = async () => {
    const descriptor = functionDetail?.descriptor || {};
    const menuConfig = descriptor?.menu || {};
    const resetRoute = {
      nodes: Array.isArray(menuConfig.nodes) ? menuConfig.nodes : [],
      path: menuConfig.path || '',
      order: menuConfig.order || 10,
      hidden: menuConfig.hidden || false,
    };
    routeConfigForm.setFieldsValue(resetRoute);
    if (functionId) {
      await saveFunctionRoute(functionId, resetRoute);
    }
    window.dispatchEvent(new CustomEvent('function-route:changed'));
    message.success('已恢复为默认路由');
  };

  const onSaveUi = async (uiConfig: any) => {
    if (!functionId) return;
    await saveFunctionUiSchema(functionId, uiConfig);
  };

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
    routeConfigSaving,
    routeConfigForm,
    routePreview,
    parsedInputSchema,
    effectiveCategory,
    jsonViewData,
    uiDescriptor,
    loadDetail,
    handleSave,
    handleStatusToggle,
    handleCopy,
    handleDelete,
    handleSavePermissions,
    handleSaveRoute,
    handleResetRoute,
    onSaveUi,
  };
}
