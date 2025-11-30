import React, { useEffect, useMemo, useRef, useState } from 'react';
import { Card, Select, Form, Input, InputNumber, Switch, Button, Space, Typography, Divider, Row, Col, Tabs, DatePicker, TimePicker } from 'antd';
import FormRender from 'form-render';
import { getMessage } from '@/utils/antdApp';
import GameSelector from '@/components/GameSelector';
import { renderXUIField, XUISchemaField } from '@/components/XUISchema';
import { listDescriptors, listFunctionInstances, invokeFunction, startJob, cancelJob, FunctionDescriptor, fetchAssignments, fetchFunctionUiSchema, openJobEventSource } from '@/services/croupier';
import { getRenderer, registerBuiltins, loadPackPlugins } from '@/plugin/registry';
import { applyTransform } from '@/plugin/transform';

const { Text, Paragraph } = Typography;

// Render form items from JSON Schema subset
type UISchema = {
  fields?: Record<string, any>;
  'ui:layout'?: { type?: 'grid'; cols?: number };
  'ui:groups'?: Array<{ title?: string; fields: string[] }>;
};

// Enhanced render function using XUISchema
function renderXFormItems(desc: FunctionDescriptor | undefined, ui: UISchema | undefined, form: any) {
  const items: any[] = [];
  const props = (desc?.params && desc.params.properties) || {};
  const required: string[] = (desc?.params && desc.params.required) || [];
  const values = Form.useWatch([], form);

  const uiFields = ui?.fields || {};

  // Enhanced group rendering with XUISchema
  const groups: Array<{ title?: string; fields: string[] }> = (ui && (ui as any)['ui:groups']) || [];
  const layoutType: string = (ui && (ui as any)['ui:layout'] && (ui as any)['ui:layout'].type) || 'grid';
  const cols = Math.max(1, Math.min(4, (ui && (ui as any)['ui:layout'] && (ui as any)['ui:layout'].cols) || 1));
  const span = Math.floor(24 / cols);

  if (groups.length > 0) {
    if (layoutType === 'tabs') {
      items.push(
        <Tabs key="tabs" items={groups.map((g, gi) => ({
          key: String(gi),
          label: g.title || `Group ${gi+1}`,
          children: (
            <Row gutter={12}>
              {g.fields.map((key) => {
                const schema = props[key] || {};
                const uiField = uiFields[key] as XUISchemaField || {};
                return (
                  <Col key={key} span={span}>
                    {renderXUIField(key, schema, uiField, values, form, [key], required.includes(key))}
                  </Col>
                );
              })}
            </Row>
          )
        }))} />
      );
    } else {
      groups.forEach((g, gi) => {
        items.push(<Divider key={`g-div-${gi}`}>{g.title || ''}</Divider>);
        items.push(
          <Row key={`g-${gi}`} gutter={12}>
            {g.fields.map((key) => {
              const schema = props[key] || {};
              const uiField = uiFields[key] as XUISchemaField || {};
              return (
                <Col key={key} span={span}>
                  {renderXUIField(key, schema, uiField, values, form, [key], required.includes(key))}
                </Col>
              );
            })}
          </Row>
        );
      });
    }
  } else {
    for (const key of Object.keys(props)) {
      const schema = props[key] || {};
      const uiField = uiFields[key] as XUISchemaField || {};
      items.push(renderXUIField(key, schema, uiField, values, form, [key], required.includes(key)));
    }
  }
  return items;
}
function renderFormItems(desc: FunctionDescriptor | undefined, ui: UISchema | undefined, form: any) {
  const items: any[] = [];
  const props = (desc?.params && desc.params.properties) || {};
  const required: string[] = (desc?.params && desc.params.required) || [];
  const values = Form.useWatch([], form);

  const getByPath = (obj: any, path: string) => {
    if (!path) return undefined;
    const p = path.replace(/^\$\.?/, '').split('.').filter(Boolean);
    let cur = obj;
    for (const k of p) { if (cur == null) return undefined; cur = cur[k]; }
    return cur;
  };
  const parseLiteral = (s: string): any => {
    const t = s.trim();
    if (t === 'true') return true; if (t === 'false') return false;
    if ((t.startsWith('"') && t.endsWith('"')) || (t.startsWith("'") && t.endsWith("'"))) return t.slice(1, -1);
    const n = Number(t); return isNaN(n) ? t : n;
  };
  const evalTerm = (expr: string): boolean => {
    const e = expr.trim();
    const ops = ['==', '!='];
    for (const op of ops) {
      const i = e.indexOf(op);
      if (i > 0) {
        const left = e.slice(0, i).trim();
        const right = e.slice(i + op.length).trim();
        const lv = getByPath(values, left.startsWith('$.') ? left : '$.' + left);
        const rv = parseLiteral(right);
        return op === '==' ? (lv === rv) : (lv !== rv);
      }
    }
    // bare path truthiness
    const v = getByPath(values, e.startsWith('$.') ? e : '$.' + e);
    return !!v;
  };
  const evalExpr = (expr?: string): boolean => {
    if (!expr) return true;
    // OR split
    const orParts = expr.split('||').map((s)=>s.trim()).filter(Boolean);
    for (const p of orParts) {
      const andParts = p.split('&&').map((s)=>s.trim()).filter(Boolean);
      let ok = true;
      for (const a of andParts) { if (!evalTerm(a)) { ok = false; break } }
      if (ok) return true;
    }
    return false;
  };

  const renderField = (key: string, schema: any, uiField: any, requiredNow: boolean, namePath: (string|number)[]) => {
    const labelText = uiField?.label || key;
    const label = requiredNow ? (<span>{labelText}<Text type="danger">*</Text></span>) : labelText;
    const hidden = uiField?.show_if ? !evalExpr(uiField.show_if) : false;
    const rules: any[] = requiredNow ? [{ required: true, message: `${key} is required` }] : [];
    // required_if
    if (!requiredNow && uiField?.required_if && evalExpr(uiField.required_if)) {
      rules.push({ required: true, message: `${key} is required` });
    }
    let node: any = <Input placeholder={uiField?.placeholder} />;
    // arrays
    if (schema.type === 'array') {
      const itemSchema = schema.items || {};
      return (
        <Form.Item key={namePath.join('.')} label={label} hidden={hidden}>
          <Form.List name={namePath}>
            {(fields, { add, remove }) => (
              <div>
                {fields.map((f) => (
                  <Space key={f.key} align="baseline" style={{ display: 'flex', marginBottom: 8 }}>
                    {renderField(String(f.name), itemSchema, {}, false, [...namePath, f.name])}
                    <Button onClick={() => remove(f.name)} danger>Remove</Button>
                  </Space>
                ))}
                <Button type="dashed" onClick={() => add()} block>Add</Button>
              </div>
            )}
          </Form.List>
        </Form.Item>
      );
    }
    // nested object with properties
    if (schema.type === 'object' && schema.properties) {
      const children: any[] = [];
      const props2 = schema.properties || {};
      const req2: string[] = schema.required || [];
      Object.keys(props2).forEach((k) => {
        const ui2 = ui?.fields?.[k] || {};
        children.push(renderField(k, props2[k], ui2, req2.includes(k), [...namePath, k]))
      });
      return (
        <div key={namePath.join('.')}
             style={{ border: '1px solid #eee', padding: 8, borderRadius: 4, marginBottom: 8 }}>
          <Text strong>{labelText}</Text>
          <div style={{ marginTop: 8 }}>{children}</div>
        </div>
      );
    }
    // map type via additionalProperties (render as entries array of {key,value})
    if (schema.type === 'object' && schema.additionalProperties) {
      const entryKey = [...namePath, '__entries'];
      return (
        <Form.Item key={namePath.join('.')} label={label} hidden={hidden}>
          <Form.List name={entryKey}>
            {(fields, { add, remove }) => (
              <div>
                {fields.map((f) => (
                  <Space key={f.key} align="baseline" style={{ display: 'flex', marginBottom: 8 }}>
                    <Form.Item name={[f.name, 'key']} rules={[{ required: true }]}>
                      <Input placeholder="key" />
                    </Form.Item>
                    <Form.Item name={[f.name, 'value']}>
                      <Input placeholder="value" />
                    </Form.Item>
                    <Button onClick={() => remove(f.name)} danger>Remove</Button>
                  </Space>
                ))}
                <Button type="dashed" onClick={() => add()} block>Add</Button>
              </div>
            )}
          </Form.List>
        </Form.Item>
      );
    }

    // primitives
    // JSON Schema validations
    if (schema && typeof schema === 'object') {
      if (typeof schema.minLength === 'number') { rules.push({ min: schema.minLength }); }
      if (typeof schema.maxLength === 'number') { rules.push({ max: schema.maxLength }); }
      if (typeof schema.pattern === 'string') {
        try { rules.push({ pattern: new RegExp(schema.pattern) }); } catch {}
      }
    }
    switch (schema.type) {
      case 'integer':
      case 'number':
        const numProps: any = { style: { width: '100%' } };
        if (typeof schema.minimum === 'number') numProps.min = schema.minimum;
        if (typeof schema.maximum === 'number') numProps.max = schema.maximum;
        node = <InputNumber {...numProps} />; break;
      case 'boolean':
        node = <Switch />; break;
      case 'string':
        const enums: string[] = (schema.enum as any) || (uiField?.enum as any);
        if (Array.isArray(enums) && enums.length) {
          const labels = (uiField?.['x-enum-labels'] || {}) as Record<string,string>;
          node = <Select options={enums.map((e: string)=>({ label: labels[e] || e, value: e }))} />;
          break;
        }
        // date/time widgets
        {
          const fmt = ((schema && (schema.format as string)) || uiField?.widget || '').toLowerCase();
          if (fmt === 'date-time') { node = <DatePicker showTime />; break; }
          if (fmt === 'date') { node = <DatePicker />; break; }
          if (fmt === 'time') { node = <TimePicker />; break; }
        }
      default:
        if (uiField?.widget === 'textarea') {
          const { TextArea } = Input as any;
          node = <TextArea rows={3} placeholder={uiField?.placeholder} />;
        } else {
          node = <Input placeholder={uiField?.placeholder} />;
        }
    }
    return (
      <Form.Item key={namePath.join('.')} name={namePath} label={label} hidden={hidden} rules={rules}>
        {node}
      </Form.Item>
    );
  };

  const groups: Array<{ title?: string; fields: string[] }> = (ui && (ui as any)['ui:groups']) || [];
  const layoutType: string = (ui && (ui as any)['ui:layout'] && (ui as any)['ui:layout'].type) || 'grid';
  const cols = Math.max(1, Math.min(4, (ui && (ui as any)['ui:layout'] && (ui as any)['ui:layout'].cols) || 1));
  const span = Math.floor(24 / cols);
  if (groups.length > 0) {
    if (layoutType === 'tabs') {
      items.push(
        <Tabs key="tabs" items={groups.map((g, gi) => ({
          key: String(gi),
          label: g.title || `Group ${gi+1}`,
          children: (
            <Row gutter={12}>
              {g.fields.map((key) => {
                const schema = props[key] || {};
                const uiField = ui?.fields?.[key] || {};
                return (
                  <Col key={key} span={span}>
                    {renderField(key, schema, uiField, required.includes(key), [key])}
                  </Col>
                );
              })}
            </Row>
          )
        }))} />
      );
    } else {
      groups.forEach((g, gi) => {
        items.push(<Divider key={`g-div-${gi}`}>{g.title || ''}</Divider>);
        items.push(
          <Row key={`g-${gi}`} gutter={12}>
            {g.fields.map((key) => {
              const schema = props[key] || {};
              const uiField = ui?.fields?.[key] || {};
              return (
                <Col key={key} span={span}>
                  {renderField(key, schema, uiField, required.includes(key), [key])}
                </Col>
              );
            })}
          </Row>
        );
      });
    }
  } else {
    for (const key of Object.keys(props)) {
      const schema = props[key] || {};
      const uiField = ui?.fields?.[key] || {};
      items.push(renderField(key, schema, uiField, required.includes(key), [key]));
    }
  }
  return items;
}

export default function GmFunctionsPage() {
  const [descs, setDescs] = useState<FunctionDescriptor[]>([]);
  const [filteredDescs, setFilteredDescs] = useState<FunctionDescriptor[]>([]);
  const [currentId, setCurrentId] = useState<string>();
  const [invoking, setInvoking] = useState(false);
  const [route, setRoute] = useState<'lb'|'broadcast'|'targeted'|'hash'>('lb');
  const [instances, setInstances] = useState<{agent_id:string;service_id:string;addr:string;version:string}[]>([]);
  const [targetService, setTargetService] = useState<string | undefined>();
  const [hashKey, setHashKey] = useState<string | undefined>();
  const [jobId, setJobId] = useState<string | undefined>();
  const [events, setEvents] = useState<string[]>([]);
  const esRef = useRef<EventSource | null>(null);
  const [form] = Form.useForm();
  const [uiSchema, setUiSchema] = useState<UISchema | undefined>();
  const [lastOutput, setLastOutput] = useState<any>(undefined);

  // Form-render state
  const [formData, setFormData] = useState<any>({});
  const [renderMode, setRenderMode] = useState<'form-render' | 'enhanced' | 'legacy'>('enhanced');

  const currentDesc = useMemo(() => (Array.isArray(descs) ? descs : []).find((d) => d.id === currentId), [descs, currentId]);

  useEffect(() => {
    registerBuiltins();
    loadPackPlugins().catch(()=>{});
    listDescriptors().then((d) => {
      const raw = Array.isArray(d) ? d : Array.isArray((d as any)?.descriptors) ? (d as any).descriptors : [];
      const arr = Array.isArray(raw) ? raw : [];
      setDescs(arr);
      // initial filter by assignments (if any)
      const gid = localStorage.getItem('game_id') || undefined;
      const env = localStorage.getItem('env') || undefined;
      if (gid) {
        fetchAssignments({ game_id: gid, env }).then((res)=>{
          const m = res?.assignments || {};
          const fns = Object.values(m).flat();
          const dd = Array.isArray(arr) ? arr : [];
          const filt = (fns && fns.length>0) ? dd.filter(x => fns.includes(x.id)) : dd;
          setFilteredDescs(filt);
          if (filt?.length) setCurrentId(filt[0].id);
        }).catch(()=>{ setFilteredDescs(arr); if (arr?.length) setCurrentId(arr[0].id); });
      } else {
        setFilteredDescs(arr);
        if (arr?.length) setCurrentId(arr[0].id);
      }
    });
    return () => {
      if (esRef.current) esRef.current.close();
    };
  }, []);

  useEffect(() => {
    // reset form when function changes; only touch antd Form when it is mounted
    const props = (currentDesc?.params && currentDesc.params.properties) || {};
    const init: any = {};
    Object.keys(props).forEach((k) => (init[k] = undefined));
    if (renderMode !== 'form-render') {
      form.setFieldsValue(init);
    }
    setFormData({}); // Reset form-render data
    setUiSchema(undefined);
    setLastOutput(undefined);
    if (currentId) {
      const gid = localStorage.getItem('game_id') || undefined;
      const env = localStorage.getItem('env') || undefined;
      listFunctionInstances({ function_id: currentId, game_id: gid }).then((res)=>{
        setInstances(res.instances||[]);
      });
      // fetch UI schema (optional)
      fetchFunctionUiSchema(currentId)
        .then((json) => {
          setUiSchema(json?.uischema || json?.uiSchema || { fields: {} });
        })
        .catch(() => {});
      // refresh assignments filter when scope changes
      const safeDescs = Array.isArray(descs) ? descs : [];
      if (gid) {
        fetchAssignments({ game_id: gid, env }).then((res)=>{
          const m = res?.assignments || {};
          const fns = Object.values(m).flat();
          const filt = (fns && fns.length>0) ? safeDescs.filter(x => fns.includes(x.id)) : safeDescs;
          setFilteredDescs(filt);
        }).catch(()=>{ setFilteredDescs(safeDescs); });
      } else {
        setFilteredDescs(safeDescs);
      }
    } else {
      setInstances([]);
    }
  }, [currentDesc?.id, renderMode]);

  const onInvoke = async () => {
    try {
      let values: any;
      if (renderMode === 'form-render' && currentDesc?.params) {
        // Use form-render data directly - it's already in the correct format
        values = formData;
      } else {
        // Use traditional form validation and normalization for enhanced/legacy modes
        values = await form.validateFields();
        if (renderMode === 'legacy') {
          values = normalizeBySchema(values, currentDesc?.params || {});
        }
      }

      setInvoking(true);
      const payload: any = { ...values };
      const res = await invokeFunction(currentId!, payload, {
        route,
        target_service_id: route === 'targeted' ? targetService : undefined,
        hash_key: route === 'hash' ? hashKey : undefined,
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
      if (renderMode === 'form-render' && currentDesc?.params) {
        // Use form-render data directly
        values = formData;
      } else {
        // Use traditional form validation and normalization for enhanced/legacy modes
        values = await form.validateFields();
        if (renderMode === 'legacy') {
          values = normalizeBySchema(values, currentDesc?.params || {});
        }
      }

      const res = await startJob(currentId!, values, {
        route,
        target_service_id: route === 'targeted' ? targetService : undefined,
        hash_key: route === 'hash' ? hashKey : undefined,
      });
      setJobId(res.job_id);
      setEvents([]);
      setLastOutput(undefined);
      // open SSE
      if (esRef.current) esRef.current.close();
      const es = openJobEventSource(res.job_id);
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

  // Normalize form values to match JSON Schema (convert __entries arrays into map objects recursively)
  function normalizeBySchema(values: any, schema: any): any {
    const walk = (val: any, sch: any): any => {
      if (!sch) return val;
      const t = sch.type;
      if (t === 'object') {
        // map
        if (sch.additionalProperties) {
          const entries = (val && val.__entries) || [];
          const obj: any = {};
          if (Array.isArray(entries)) {
            entries.forEach((e: any) => { if (e && e.key) obj[e.key] = e.value; });
          }
          return obj;
        }
        // nested object
        const out: any = {};
        const props = sch.properties || {};
        Object.keys(val || {}).forEach((k) => {
          if (props[k]) out[k] = walk(val[k], props[k]); else out[k] = val[k];
        });
        return out;
      }
      if (t === 'array') {
        const itemSch = sch.items || {};
        if (Array.isArray(val)) return val.map((it) => walk(it, itemSch));
        return val;
      }
      return val;
    };
    return walk(values, schema);
  }

  return (
    <Card title="GM Functions" extra={<Text type="secondary">dev</Text>}>
      <Space direction="vertical" style={{ width: '100%' }} size="large">
        <GameSelector />
        <Space>
          <span>Select Function:</span>
          <Select style={{ minWidth: 320 }} value={currentId} onChange={setCurrentId} options={filteredDescs.map((d) => ({ label: `${d.id} v${d.version || ''}`, value: d.id }))} />
          <span>Form Renderer:</span>
          <Select
            style={{ width: 140 }}
            value={renderMode}
            onChange={(v) => setRenderMode(v)}
            options={[
              { label: 'Enhanced UI', value: 'enhanced' },
              { label: 'Form-Render', value: 'form-render' },
              { label: 'Legacy', value: 'legacy' },
            ]}
          />
          <span>Route:</span>
          <Select
            style={{ width: 180 }}
            value={route}
            onChange={(v)=>setRoute(v)}
            options={[
              {label:'lb', value:'lb'},
              {label:'broadcast', value:'broadcast'},
              {label:'targeted', value:'targeted'},
              {label:'hash', value:'hash'},
            ]}
          />
          {route === 'targeted' && (
            <>
              <span>Target:</span>
              <Select style={{ minWidth: 260 }} value={targetService} onChange={setTargetService}
                placeholder="Select service instance"
                options={instances.map(i=>({ label: `${i.service_id} @ ${i.agent_id} (${i.version})`, value: i.service_id }))} />
            </>
          )}
          {route === 'hash' && (
            <>
              <span>Hash Key:</span>
              <Input style={{ width: 260 }} value={hashKey} placeholder="e.g. player_id"
                onChange={(e)=>setHashKey(e.target.value)} />
            </>
          )}
      </Space>

      {/* 配置可视化，便于后台查看参数与 UI 定制 */}
      <Row gutter={16}>
        <Col span={12}>
          <Card size="small" title="参数 Schema">
            <pre style={{ whiteSpace: 'pre-wrap', margin: 0, maxHeight: 240, overflow: 'auto' }}>
              {currentDesc?.params ? JSON.stringify(currentDesc.params, null, 2) : '无参数定义'}
            </pre>
          </Card>
        </Col>
        <Col span={12}>
          <Card size="small" title="UI 参数 (uiSchema)">
            <pre style={{ whiteSpace: 'pre-wrap', margin: 0, maxHeight: 240, overflow: 'auto' }}>
              {uiSchema ? JSON.stringify(uiSchema, null, 2) : '无 UI Schema'}
            </pre>
          </Card>
        </Col>
      </Row>

      {/* Form Rendering Section */}
      {(() => {
        if (renderMode === 'form-render' && currentDesc?.params) {
            return (
              <FormRender
                schema={currentDesc.params}
                uiSchema={uiSchema?.fields || {}}
                formData={formData}
                onChange={setFormData}
                displayType="row"
                labelWidth={120}
              />
            );
          } else if (renderMode === 'enhanced') {
            return (
              <Form form={form} labelCol={{ span: 6 }} wrapperCol={{ span: 12 }}>
                {renderXFormItems(currentDesc, uiSchema, form)}
              </Form>
            );
          } else {
            // Legacy mode
            return (
              <Form form={form} labelCol={{ span: 6 }} wrapperCol={{ span: 12 }}>
                {renderFormItems(currentDesc, uiSchema, form)}
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
        {currentDesc?.outputs && (currentDesc as any).outputs.views && ((currentDesc as any).outputs.views as any[]).length > 0 && (
          <div>
            <Divider />
            <Text strong>Views</Text>
            {(() => {
              const outputs: any = (currentDesc as any).outputs || {};
              const views: any[] = (outputs.views as any[]) || [];
              const layout: any = outputs.layout || {};
              const gridCols = layout?.type === 'grid' ? (layout?.cols || 2) : 0;
              const items = views.map((v: any) => {
                const Renderer = getRenderer(v.renderer || v.type || 'json.view');
                if (!Renderer) return <div key={v.id || v.renderer}>No renderer: {v.renderer}</div>;
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
                  <div style={{ marginTop: 12, display: 'grid', gridTemplateColumns: `repeat(${gridCols}, 1fr)`, gap: 12 }}>
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
