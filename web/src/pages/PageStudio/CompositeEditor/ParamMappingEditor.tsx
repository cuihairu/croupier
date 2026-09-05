import React, { useMemo } from 'react';
import { Input, Select, Space, Typography } from 'antd';
import type { PageNode } from './model';
import type { FunctionDescriptor } from '@/services/api/functions';

const { Text } = Typography;

export type InputAssignment = {
  param: string;
  kind: 'page_state' | 'literal';
  sourceNodeId?: string;
  field?: string;
  value?: unknown;
};

type ParamInfo = { name: string; required: boolean };

/** 来源区块（其他节点）可用字段：fn* 用 outputSchema，staticForm 用 staticSchema。 */
function fieldsOf(node: PageNode | undefined, fnById: Map<string, FunctionDescriptor>): string[] {
  if (!node) return [];
  if (node.type === 'staticForm') {
    const raw = node.props.staticSchema;
    try {
      const schema = typeof raw === 'string' ? JSON.parse(raw) : (raw as Record<string, unknown>);
      return Object.keys((schema?.properties ?? {}) as Record<string, unknown>);
    } catch {
      return [];
    }
  }
  const fid = String(node.props.functionId ?? '');
  const fn = fnById.get(fid);
  const raw = fn?.outputSchema;
  if (!raw) return [];
  try {
    const schema = typeof raw === 'string' ? JSON.parse(raw) : (raw as Record<string, unknown>);
    return Object.keys((schema?.properties ?? {}) as Record<string, unknown>);
  } catch {
    return [];
  }
}

/** 显式参数映射编辑器（P0）：每个参数 ← 来源区块.字段 / 固定值；未列出 = 自动。 */
export default function ParamMappingEditor({
  fn,
  nodes,
  selfId,
  fnById,
  value,
  onChange,
}: {
  fn: FunctionDescriptor | undefined;
  nodes: PageNode[];
  selfId: string;
  fnById: Map<string, FunctionDescriptor>;
  value?: InputAssignment[];
  onChange: (v: InputAssignment[]) => void;
}) {
  const params = useMemo<ParamInfo[]>(() => {
    const raw = fn?.inputSchema;
    let props: Record<string, unknown> = {};
    let required: string[] = [];
    try {
      const schema = typeof raw === 'string' ? JSON.parse(raw) : (raw as Record<string, unknown>);
      props = (schema?.properties ?? {}) as Record<string, unknown>;
      required = Array.isArray(schema?.required) ? (schema?.required as string[]) : [];
    } catch {
      props = {};
    }
    return Object.entries(props).map(([name, p]) => {
      const meta = p as Record<string, unknown>;
      return {
        name,
        required: required.includes(name),
      };
    });
  }, [fn?.inputSchema]);

  const sources = useMemo(
    () =>
      nodes
        .filter((n) => n.id !== selfId)
        .map((n) => ({ node: n, fields: fieldsOf(n, fnById) }))
        .filter((s) => s.fields.length > 0),
    [nodes, fnById],
  );

  const assignmentFor = (param: string) => (value ?? []).find((a) => a.param === param);

  const setAssignment = (param: string, next: InputAssignment | null) => {
    const rest = (value ?? []).filter((a) => a.param !== param);
    onChange(next ? [...rest, next] : rest);
  };

  if (!fn || params.length === 0) return null;

  return (
    <Space direction="vertical" size={8} style={{ width: '100%' }}>
      <Text type="secondary" style={{ fontSize: 11, display: 'block' }}>
        参数映射：默认取本区块表单值；跨区块取数在此显式声明（未列出的参数保持自动）。
      </Text>
      {params.map((p) => {
        const a = assignmentFor(p.name);
        const kind = a?.kind ?? 'auto';
        const sourceNode = sources.find((s) => s.node.id === a?.sourceNodeId)?.node;
        const fieldOptions = fieldsOf(sourceNode, fnById).map((f) => ({ label: f, value: f }));
        return (
          <div
            key={p.name}
            style={{ border: '1px solid #f0f0f0', borderRadius: 6, padding: '6px 8px' }}
          >
            <Space size={4} wrap style={{ width: '100%' }}>
              <Text code style={{ fontSize: 12 }}>
                {p.name}
                {p.required ? ' *' : ''}
              </Text>
              <Select
                size="small"
                style={{ width: 110 }}
                value={kind}
                onChange={(v) => {
                  if (v === 'auto') setAssignment(p.name, null);
                  else if (v === 'literal')
                    setAssignment(p.name, { param: p.name, kind: 'literal', value: '' });
                  else {
                    const first = sources[0];
                    setAssignment(p.name, {
                      param: p.name,
                      kind: 'page_state',
                      sourceNodeId: first?.node.id ?? '',
                      field: first?.fields[0] ?? '',
                    });
                  }
                }}
                options={[
                  { label: '自动', value: 'auto' },
                  { label: '上游区块', value: 'page_state' },
                  { label: '固定值', value: 'literal' },
                ]}
              />
            </Space>
            {kind === 'page_state' && (
              <Space size={4} style={{ marginTop: 4 }} wrap>
                <Select
                  size="small"
                  style={{ width: 150 }}
                  placeholder="来源区块"
                  value={a?.sourceNodeId || undefined}
                  onChange={(v) => {
                    const src = sources.find((s) => s.node.id === v);
                    setAssignment(p.name, {
                      param: p.name,
                      kind: 'page_state',
                      sourceNodeId: String(v),
                      field: fieldsOf(src?.node, fnById)[0] ?? '',
                    });
                  }}
                  options={sources.map((s) => ({
                    label: String(s.node.props.title ?? s.node.id),
                    value: s.node.id,
                  }))}
                />
                <Select
                  size="small"
                  style={{ width: 130 }}
                  placeholder="字段"
                  value={a?.field || undefined}
                  onChange={(v) =>
                    setAssignment(p.name, {
                      param: p.name,
                      kind: 'page_state',
                      sourceNodeId: a?.sourceNodeId ?? '',
                      field: String(v),
                    })
                  }
                  options={fieldOptions}
                />
              </Space>
            )}
            {kind === 'literal' && (
              <Input
                size="small"
                style={{ marginTop: 4 }}
                placeholder="固定值"
                value={a?.value === undefined ? '' : String(a.value)}
                onChange={(e) =>
                  setAssignment(p.name, {
                    param: p.name,
                    kind: 'literal',
                    value: e.target.value,
                  })
                }
              />
            )}
          </div>
        );
      })}
    </Space>
  );
}
