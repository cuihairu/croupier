import React, { useEffect, useMemo, useRef, useState } from 'react';
import {
  Card,
  Select,
  Form,
  Input,
  InputNumber,
  Switch,
  Button,
  Space,
  Typography,
  Divider,
  Row,
  Col,
  Tabs,
  DatePicker,
  TimePicker,
} from 'antd';
import FormRender from 'form-render';
import { getMessage } from '@/utils/antdApp';
import { renderXUIField, XUISchemaField } from '@/components/XUISchema';
import {
  listDescriptors,
  listFunctionInstances,
  invokeFunction,
  startJob,
  cancelJob,
  FunctionDescriptor,
  fetchAssignments,
  fetchFunctionUiSchema,
  getFunctionOpenAPI,
  openJobEventSource,
} from '@/services/api';
import { getRenderer, registerBuiltins } from '@/plugin/registry';
import { applyTransform } from '@/plugin/transform';
import { useLocation } from '@umijs/max';
import { parseInputSchema, buildUISchemaFromJSONSchema } from '@/utils/json';

const { Text, Paragraph } = Typography;

// Render form items from JSON Schema subset
type UISchema = {
  fields?: Record<string, any>;
  'ui:layout'?: { type?: 'grid'; cols?: number };
  'ui:groups'?: Array<{ title?: string; fields: string[] }>;
};

function extractOpenAPIRequestSchema(operation: any): any | null {
  const schema = operation?.requestBody?.content?.['application/json']?.schema;
  return schema && typeof schema === 'object' ? schema : null;
}

function extractOpenAPIUISchema(operation: any): UISchema | undefined {
  const xui = operation?.['x-ui'] || operation?.extensions?.['x-ui'];
  if (xui && typeof xui === 'object') {
    return xui as UISchema;
  }
  return undefined;
}

// Enhanced render function using XUISchema
function renderXFormItems(
  desc: FunctionDescriptor | undefined,
  ui: UISchema | undefined,
  form: any,
  formValues: any,
) {
  const items: any[] = [];
  const props = (desc?.params && desc.params.properties) || {};
  const required: string[] = (desc?.params && desc.params.required) || [];
  const values = formValues;

  const uiFields = ui?.fields || {};

  // Enhanced group rendering with XUISchema
  const groups: Array<{ title?: string; fields: string[] }> =
    (ui && (ui as any)['ui:groups']) || [];
  const layoutType: string =
    (ui && (ui as any)['ui:layout'] && (ui as any)['ui:layout'].type) || 'grid';
  const cols = Math.max(
    1,
    Math.min(4, (ui && (ui as any)['ui:layout'] && (ui as any)['ui:layout'].cols) || 1),
  );
  const span = Math.floor(24 / cols);

  if (groups.length > 0) {
    if (layoutType === 'tabs') {
      items.push(
        <Tabs
          key="tabs"
          items={groups.map((g, gi) => ({
            key: String(gi),
            label: g.title || `Group ${gi + 1}`,
            children: (
              <Row gutter={12}>
                {g.fields.map((key) => {
                  const schema = props[key] || {};
                  const uiField = (uiFields[key] as XUISchemaField) || {};
                  return (
                    <Col key={key} span={span}>
                      {renderXUIField(
                        key,
                        schema,
                        uiField,
                        values,
                        form,
                        [key],
                        required.includes(key),
                      )}
                    </Col>
                  );
                })}
              </Row>
            ),
          }))}
        />,
      );
    } else {
      groups.forEach((g, gi) => {
        items.push(<Divider key={`g-div-${gi}`}>{g.title || ''}</Divider>);
        items.push(
          <Row key={`g-${gi}`} gutter={12}>
            {g.fields.map((key) => {
              const schema = props[key] || {};
              const uiField = (uiFields[key] as XUISchemaField) || {};
              return (
                <Col key={key} span={span}>
                  {renderXUIField(
                    key,
                    schema,
                    uiField,
                    values,
                    form,
                    [key],
                    required.includes(key),
                  )}
                </Col>
              );
            })}
          </Row>,
        );
      });
    }
  } else {
    for (const key of Object.keys(props)) {
      const schema = props[key] || {};
      const uiField = (uiFields[key] as XUISchemaField) || {};
      items.push(renderXUIField(key, schema, uiField, values, form, [key], required.includes(key)));
    }
  }
  return items;
}
export default function GmFunctionsPage() {
  const location = useLocation();
  const [descs, setDescs] = useState<FunctionDescriptor[]>([]);
  const [filteredDescs, setFilteredDescs] = useState<FunctionDescriptor[]>([]);
  const [currentId, setCurrentId] = useState<string>();
  const [invoking, setInvoking] = useState(false);
  const [route, setRoute] = useState<'lb' | 'broadcast' | 'targeted' | 'hash'>('lb');
  const [instances, setInstances] = useState<
    { agentId: string; serviceId: string; addr: string; version: string }[]
  >([]);
  const [targetService, setTargetService] = useState<string | undefined>();
  const [hashKey, setHashKey] = useState<string | undefined>();
  const [jobId, setJobId] = useState<string | undefined>();
  const [events, setEvents] = useState<string[]>([]);
  const esRef = useRef<EventSource | null>(null);
  const [form] = Form.useForm();
  const formValues = Form.useWatch([], form);
  const [uiSchema, setUiSchema] = useState<UISchema | undefined>();
  const [openapiOperation, setOpenapiOperation] = useState<any>(undefined);
  const [lastOutput, setLastOutput] = useState<any>(undefined);

  // Form-render state
  const [formData, setFormData] = useState<any>({});
  const [renderMode, setRenderMode] = useState<'form-render' | 'enhanced'>('enhanced');
  const getFunctionIdFromQuery = () => {
    const query = new URLSearchParams(location.search);
    return query.get('fid') || query.get('id') || undefined;
  };

  const currentDesc = useMemo(
    () => (Array.isArray(descs) ? descs : []).find((d) => d.id === currentId),
    [descs, currentId],
  );

  // Parse input_schema first; fallback to OpenAPI requestBody schema.
  const effectiveSchema = useMemo(() => {
    if (!currentDesc) return null;
    const parsed = parseInputSchema(currentDesc.inputSchema);
    if (parsed) return parsed;
    return extractOpenAPIRequestSchema(openapiOperation);
  }, [currentDesc, openapiOperation]);

  // Auto-generate UI Schema when not provided by API
  const effectiveUISchema = useMemo(() => {
    // 如果有 API 返回的 uiSchema，直接使用
    if (uiSchema && Object.keys(uiSchema).length > 0) {
      return uiSchema;
    }
    const openapiUi = extractOpenAPIUISchema(openapiOperation);
    if (openapiUi && Object.keys(openapiUi).length > 0) {
      return openapiUi;
    }
    // 否则从 JSON Schema 自动生成
    if (effectiveSchema) {
      return buildUISchemaFromJSONSchema(effectiveSchema);
    }
    return { fields: {} };
  }, [uiSchema, openapiOperation, effectiveSchema]);

  useEffect(() => {
    registerBuiltins();
    listDescriptors().then((d) => {
      const raw = Array.isArray(d)
        ? d
        : Array.isArray((d as any)?.descriptors)
        ? (d as any).descriptors
        : [];
      const arr = Array.isArray(raw) ? raw : [];
      setDescs(arr);
      // initial filter by assignments (if any)
      const gid = localStorage.getItem('game_id') || undefined;
      const env = localStorage.getItem('env') || undefined;
      if (gid) {
        fetchAssignments({ game_id: gid, env })
          .then((res) => {
            const m = res?.assignments || {};
            const fns = Object.values(m).flat();
            const dd = Array.isArray(arr) ? arr : [];
            const filt = fns && fns.length > 0 ? dd.filter((x) => fns.includes(x.id)) : dd;
            const fid = getFunctionIdFromQuery();
            let final = filt;
            if (fid && dd.some((x) => x.id === fid) && !final.some((x) => x.id === fid)) {
              const picked = dd.find((x) => x.id === fid);
              if (picked) final = [picked, ...final];
            }
            setFilteredDescs(final);
            if (fid && dd.some((x) => x.id === fid)) setCurrentId(fid);
            else if (final?.length) setCurrentId(final[0].id);
          })
          .catch(() => {
            setFilteredDescs(arr);
            if (arr?.length) setCurrentId(arr[0].id);
          });
      } else {
        const fid = getFunctionIdFromQuery();
        setFilteredDescs(arr);
        if (fid && arr.some((x) => x.id === fid)) setCurrentId(fid);
        else if (arr?.length) setCurrentId(arr[0].id);
      }
    });
    return () => {
      if (esRef.current) esRef.current.close();
    };
  }, []);

  useEffect(() => {
    const fid = getFunctionIdFromQuery();
    if (fid && fid !== currentId && descs.some((d) => d.id === fid)) {
      setCurrentId(fid);
      if (!filteredDescs.some((d) => d.id === fid)) {
        const picked = descs.find((d) => d.id === fid);
        if (picked) setFilteredDescs([picked, ...filteredDescs]);
      }
    }
  }, [location.search, descs, filteredDescs, currentId]);

  useEffect(() => {
    // reset form when function changes; only touch antd Form when it is mounted
    const props = (effectiveSchema && effectiveSchema.properties) || {};
    const init: any = {};
    Object.keys(props).forEach((k) => (init[k] = undefined));
    if (renderMode !== 'form-render') {
      form.setFieldsValue(init);
    }
    setFormData({}); // Reset form-render data
    setUiSchema(undefined);
    setOpenapiOperation(undefined);
    setLastOutput(undefined);
    if (currentId) {
      const gid = localStorage.getItem('game_id') || undefined;
      const env = localStorage.getItem('env') || undefined;
      listFunctionInstances({ functionId: currentId, gameId: gid }).then((res) => {
        setInstances(res.instances || []);
      });
      fetchFunctionUiSchema(currentId)
        .then((resp) => {
          if (resp?.schema && typeof resp.schema === 'object') {
            setUiSchema(resp.schema as UISchema);
            return;
          }
          setUiSchema({ fields: {} });
        })
        .catch(() => {
          setUiSchema({ fields: {} });
        });
      getFunctionOpenAPI(currentId)
        .then((resp) => {
          setOpenapiOperation(resp || undefined);
        })
        .catch(() => {
          setOpenapiOperation(undefined);
        });
      // refresh assignments filter when scope changes
      const safeDescs = Array.isArray(descs) ? descs : [];
      if (gid) {
        fetchAssignments({ game_id: gid, env })
          .then((res) => {
            const m = res?.assignments || {};
            const fns = Object.values(m).flat();
            const filt =
              fns && fns.length > 0 ? safeDescs.filter((x) => fns.includes(x.id)) : safeDescs;
            setFilteredDescs(filt);
          })
          .catch(() => {
            setFilteredDescs(safeDescs);
          });
      } else {
        setFilteredDescs(safeDescs);
      }
    } else {
      setInstances([]);
    }
  }, [currentDesc?.id, renderMode, effectiveSchema]);

  const onInvoke = async () => {
    try {
      let values: any;
      if (renderMode === 'form-render' && effectiveSchema) {
        // Use form-render data directly - it's already in the correct format
        values = formData;
      } else {
        // Use antd form validation for enhanced mode.
        values = await form.validateFields();
      }

      setInvoking(true);
      const payload: any = { ...values };
      const res = await invokeFunction(currentId!, payload, {
        route,
        targetServiceId: route === 'targeted' ? targetService : undefined,
        hashKey: route === 'hash' ? hashKey : undefined,
      });
      getMessage()?.success('Invoke OK');
      setEvents([JSON.stringify(res)]);
      setLastOutput(res);
    } catch (e: any) {
      if (e?.errorFields) return; // form error
      getMessage()?.error(e?.message || 'Invoke failed');
    } finally {
      setInvoking(false);
    }
  };

  const onStartJob = async () => {
    try {
      let values: any;
      if (renderMode === 'form-render' && effectiveSchema) {
        // Use form-render data directly
        values = formData;
      } else {
        // Use antd form validation for enhanced mode.
        values = await form.validateFields();
      }

      const res = await startJob(currentId!, values, {
        route,
        targetServiceId: route === 'targeted' ? targetService : undefined,
        hashKey: route === 'hash' ? hashKey : undefined,
      });
      const createdJobId = res.jobId || res.jobID || '';
      setJobId(createdJobId);
      setEvents([]);
      setLastOutput(undefined);
      // open SSE
      if (esRef.current) esRef.current.close();
      const es = openJobEventSource(createdJobId);
      es.onmessage = (ev) => setEvents((prev) => [...prev, ev.data]);
      es.addEventListener('done', () => es.close());
      es.addEventListener('error', () => es.close());
      esRef.current = es;
    } catch (e: any) {
      if (e?.errorFields) return;
      getMessage()?.error(e?.message || 'Start job failed');
    }
  };

  const onCancel = async () => {
    if (!jobId) return;
    await cancelJob(jobId);
    getMessage()?.info('Cancel sent');
  };

  return (
    <Card title="GM Functions" extra={<Text type="secondary">dev</Text>}>
      <Space direction="vertical" style={{ width: '100%' }} size="large">
        <Space>
          <span>Select Function:</span>
          <Select
            style={{ minWidth: 320 }}
            value={currentId}
            onChange={setCurrentId}
            options={filteredDescs.map((d) => ({
              label: `${d.id} v${d.version || ''}`,
              value: d.id,
            }))}
          />
          <span>Form Renderer:</span>
          <Select
            style={{ width: 140 }}
            value={renderMode}
            onChange={(v) => setRenderMode(v)}
            options={[
              { label: 'Enhanced UI', value: 'enhanced' },
              { label: 'Form-Render', value: 'form-render' },
            ]}
          />
          <span>Route:</span>
          <Select
            style={{ width: 180 }}
            value={route}
            onChange={(v) => setRoute(v)}
            options={[
              { label: 'lb', value: 'lb' },
              { label: 'broadcast', value: 'broadcast' },
              { label: 'targeted', value: 'targeted' },
              { label: 'hash', value: 'hash' },
            ]}
          />
          {route === 'targeted' && (
            <>
              <span>Target:</span>
              <Select
                style={{ minWidth: 260 }}
                value={targetService}
                onChange={setTargetService}
                placeholder="Select service instance"
                options={instances.map((i) => ({
                  label: `${i.serviceId} @ ${i.agentId} (${i.version})`,
                  value: i.serviceId,
                }))}
              />
            </>
          )}
          {route === 'hash' && (
            <>
              <span>Hash Key:</span>
              <Input
                style={{ width: 260 }}
                value={hashKey}
                placeholder="e.g. player_id"
                onChange={(e) => setHashKey(e.target.value)}
              />
            </>
          )}
        </Space>

        {/* 配置可视化，便于后台查看参数与 UI 定制 */}
        <Row gutter={16}>
          <Col span={12}>
            <Card
              size="small"
              title={`参数 Schema${
                currentDesc?.inputSchema
                  ? ' (from proto)'
                  : openapiOperation
                  ? ' (from OpenAPI requestBody)'
                  : ''
              }`}
            >
              <pre style={{ whiteSpace: 'pre-wrap', margin: 0, maxHeight: 240, overflow: 'auto' }}>
                {effectiveSchema ? JSON.stringify(effectiveSchema, null, 2) : '无参数定义'}
              </pre>
            </Card>
          </Col>
          <Col span={12}>
            <Card
              size="small"
              title={`UI Schema${
                uiSchema && Object.keys(uiSchema).length > 0
                  ? ' (from API)'
                  : extractOpenAPIUISchema(openapiOperation)
                  ? ' (from OpenAPI x-ui)'
                  : ' (auto-generated)'
              }`}
            >
              <pre style={{ whiteSpace: 'pre-wrap', margin: 0, maxHeight: 240, overflow: 'auto' }}>
                {effectiveUISchema ? JSON.stringify(effectiveUISchema, null, 2) : '无 UI Schema'}
              </pre>
            </Card>
          </Col>
        </Row>

        {/* Form Rendering Section */}
        {(() => {
          if (renderMode === 'form-render' && effectiveSchema) {
            return (
              <FormRender
                schema={effectiveSchema as any}
                uiSchema={effectiveUISchema?.fields || {}}
                formData={formData}
                onChange={setFormData}
                displayType="row"
                labelWidth={120}
              />
            );
          } else {
            const descWithSchema = currentDesc
              ? { ...currentDesc, params: effectiveSchema }
              : undefined;
            return (
              <Form form={form} labelCol={{ span: 6 }} wrapperCol={{ span: 12 }}>
                {renderXFormItems(descWithSchema, effectiveUISchema, form, formValues)}
              </Form>
            );
          }
        })()}
        <Space>
          <Button type="primary" onClick={onInvoke} loading={invoking} disabled={!currentId}>
            Invoke
          </Button>
          <Button onClick={onStartJob} disabled={!currentId}>
            Start Job
          </Button>
          <Button onClick={onCancel} danger disabled={!jobId}>
            Cancel Job
          </Button>
        </Space>
        <Divider />
        <Typography>
          <Text strong>Output / Events</Text>
          <Paragraph>
            <pre style={{ whiteSpace: 'pre-wrap' }}>{events.join('\n')}</pre>
          </Paragraph>
        </Typography>
        {/* Views rendering (outputs.views) */}
        {currentDesc?.outputs &&
          (currentDesc as any).outputs.views &&
          ((currentDesc as any).outputs.views as any[]).length > 0 && (
            <div>
              <Divider />
              <Text strong>Views</Text>
              {(() => {
                const outputs: any = (currentDesc as any).outputs || {};
                const views: any[] = (outputs.views as any[]) || [];
                const layout: any = outputs.layout || {};
                const gridCols = layout?.type === 'grid' ? layout?.cols || 2 : 0;
                const items = views.map((v: any) => {
                  const Renderer = getRenderer(v.renderer || v.type || 'json.view');
                  if (!Renderer)
                    return <div key={v.id || v.renderer}>No renderer: {v.renderer}</div>;
                  // optional view-level show_if: treat as expr path; falsy or empty array -> hide
                  if (typeof v.show_if === 'string') {
                    try {
                      const cond = applyTransform(lastOutput, { expr: v.show_if });
                      if (!cond || (Array.isArray(cond) && cond.length === 0)) return null;
                    } catch {}
                  }
                  const data = applyTransform(lastOutput, v.transform);
                  return (
                    <div key={v.id || v.renderer} style={{ marginBottom: gridCols ? 0 : 16 }}>
                      <Renderer data={data} options={v.options} />
                    </div>
                  );
                });
                const filtered = items.filter(Boolean) as any[];
                if (gridCols > 0) {
                  return (
                    <div
                      style={{
                        marginTop: 12,
                        display: 'grid',
                        gridTemplateColumns: `repeat(${gridCols}, 1fr)`,
                        gap: 12,
                      }}
                    >
                      {filtered}
                    </div>
                  );
                }
                return <div style={{ marginTop: 12 }}>{filtered}</div>;
              })()}
            </div>
          )}
      </Space>
    </Card>
  );
}
