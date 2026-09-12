import React, { useEffect, useMemo, useRef } from 'react';
import { Button, Card, Descriptions, Space, Table, Tabs, Typography } from 'antd';
import { FormattedMessage, useIntl } from '@umijs/max';
import type { FunctionDescriptor } from '@/services/api/functions';
import type { FormPresentationSpec, FormValues, JSONSchema } from '@/types/dashboard';
import SchemaFormRenderer, { type SchemaFormRendererProps } from '@/components/SchemaFormRenderer';
import { derivePresentationSpec } from '@/utils/schemaHints';
import type { PageNode } from './model';
import { schemaProperties } from './types';
import { itemsOf, payloadOf, type JSONRecord } from './previewShared';

/** 预览渲染子组件：节点级渲染（表格/字段卡/表单/弹窗内容）与常量表单实时态。
 * 引擎（状态/执行/动作分派）在 ./PreviewRuntime，此处仅消费回调渲染。 */

const { Text } = Typography;

/** 行内节点渲染（预览态，无编辑装饰）。 */
export default function PreviewNode({
  node,
  fn,
  data,
  running,
  cascadeInputs,
  onAction,
  onRowAction,
  onSubmit,
  onFormValues,
  onStaticChange,
  onSelectionChange,
  renderChild,
}: {
  node: PageNode;
  fn: FunctionDescriptor | undefined;
  data: unknown;
  running: boolean;
  /** refreshOn 级联合并输入（行内表单初值，对齐发布端 sectionInputs）。 */
  cascadeInputs?: JSONRecord;
  onAction: (raw: unknown, ctx?: JSONRecord) => void;
  /** V5：行操作点击（行字段映射 → 弹窗预填），由父级处理目标与求值。 */
  onRowAction?: (raw: unknown, row: JSONRecord) => void;
  onSubmit: (params: JSONRecord) => void;
  /** 表单当前值 → 页面状态（{{var.values.x}} 求值来源）。 */
  onFormValues?: (nodeId: string, values: JSONRecord) => void;
  /** staticForm 值变化（防抖后）→ 预览页面状态。 */
  onStaticChange?: (nodeId: string, values: JSONRecord) => void;
  /** V5：表格选中行变化 → 预览页面状态（selectedRow/selectedRows）+ 选中事件。 */
  onSelectionChange?: (node: PageNode, rows: JSONRecord[]) => void;
  /** 容器子节点渲染回调（由主组件注入执行上下文）。 */
  renderChild?: (child: PageNode) => React.ReactNode;
}) {
  const payload = useMemo(() => payloadOf(data), [data]);
  const items = useMemo(() => itemsOf(payload), [payload]);
  const intl = useIntl();
  const title = String(node.props.title ?? node.type);
  // V5：列 = 声明列/schema 字段 + 行操作列（发布行为的预览等价物）
  const rowActionDrafts = Array.isArray(node.props.rowActions)
    ? (node.props.rowActions as Array<Record<string, unknown>>)
    : [];
  const previewColumns = useMemo(() => {
    const base = (
      Array.isArray(node.props.columns) && node.props.columns.length
        ? (node.props.columns as string[])
        : schemaProperties(fn?.outputSchema)
    )
      .slice(0, 8)
      .map((c) => ({ title: c, dataIndex: c, ellipsis: true }));
    if (!rowActionDrafts.length) return base;
    return [
      ...base,
      {
        title: intl.formatMessage({
          id: 'pages.pageStudio.editor.previewNode.rowActionColumn',
          defaultMessage: '操作',
        }),
        key: '__preview_row_actions',
        render: (_: unknown, row: JSONRecord) => (
          <Space size={4}>
            {rowActionDrafts.map((ra, i) => (
              <Button
                key={i}
                size="small"
                type="link"
                danger={ra.danger === true}
                onClick={() => onRowAction?.(ra, row)}
              >
                {/* ra.label 为 spec 数据回显；缺失兜底「操作」与 PreviewRuntime 同键 */}
                {String(
                  ra.label ??
                    intl.formatMessage({
                      id: 'pages.pageStudio.editor.preview.rowActionFallback',
                      defaultMessage: '操作',
                    }),
                )}
              </Button>
            ))}
          </Space>
        ),
      },
    ];
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [node.props.columns, fn?.outputSchema, rowActionDrafts, onRowAction, intl]);

  if (node.type === 'text') {
    const level = String(node.props.level ?? 'p');
    const content = String(node.props.content ?? '');
    const inner =
      level === 'h2' ? (
        <Typography.Title level={4}>{content}</Typography.Title>
      ) : level === 'h3' ? (
        <Typography.Title level={5}>{content}</Typography.Title>
      ) : (
        <Text>{content}</Text>
      );
    return node.props.onClick ? (
      <span style={{ cursor: 'pointer' }} onClick={() => onAction(node.props.onClick)}>
        {inner}
      </span>
    ) : (
      inner
    );
  }

  if (node.type === 'button') {
    return (
      <Button
        type={node.props.btnStyle === 'primary' ? 'primary' : 'default'}
        danger={node.props.btnStyle === 'danger'}
        onClick={() => onAction(node.props.onClick)}
      >
        {title}
      </Button>
    );
  }

  // 页签容器（V2）：非受控 Tabs（预览态切换不回写树）；页内 container 子节点
  // 经 renderChild 递归渲染（与发布端「tab 组 → Tabs → 页内区块堆叠」同构）。
  if (node.type === 'tabs') {
    const pages = (node.children ?? []).filter((c) => c.type === 'container');
    if (pages.length === 0) return null;
    return (
      <Tabs
        size="small"
        items={pages.map((page, i) => ({
          key: page.id,
          label: String(
            typeof page.props.title === 'string' && page.props.title.trim()
              ? page.props.title
              : intl.formatMessage(
                  {
                    id: 'pages.pageStudio.editor.component.tabs.tabFallback',
                    defaultMessage: '页签 {n}',
                  },
                  { n: i + 1 },
                ),
          ),
          children:
            page.children && page.children.length > 0 ? (
              <Space orientation="vertical" size={8} style={{ width: '100%' }}>
                {page.children.map((c) => (
                  <React.Fragment key={c.id}>
                    {renderChild?.(c) ?? <Text type="secondary">{c.type}</Text>}
                  </React.Fragment>
                ))}
              </Space>
            ) : (
              <Text type="secondary">
                <FormattedMessage
                  id="pages.pageStudio.editor.component.tabs.emptyTab"
                  defaultMessage="空页签——拖入组件"
                />
              </Text>
            ),
        }))}
      />
    );
  }

  // fnFields 点击事件（对齐发布端 fireEvent(sec,'click')）：整卡可点击
  const fieldsEl = (
    <Descriptions size="small" column={2} bordered>
      {Object.entries(payload)
        .filter(([k]) => k !== 'items' && k !== 'total')
        .slice(0, 10)
        .map(([k, v]) => (
          <Descriptions.Item key={k} label={k}>
            {typeof v === 'object' ? JSON.stringify(v) : String(v ?? '-')}
          </Descriptions.Item>
        ))}
    </Descriptions>
  );

  return (
    <Card
      size="small"
      title={title}
      loading={running}
      extra={
        (node.type === 'fnTable' || node.type === 'fnFields') && node.props.autoRun !== true ? (
          <Button size="small" onClick={() => onSubmit({})}>
            <FormattedMessage
              id="pages.pageStudio.editor.previewNode.executeButton"
              defaultMessage="执行"
            />
          </Button>
        ) : null
      }
    >
      {node.type === 'fnTable' ? (
        <Table
          size="small"
          rowKey={(_, i) => String(i)}
          pagination={{ pageSize: 10, showSizeChanger: false }}
          // 行点击事件（对齐发布端 onRow → rowClick）：row 作为求值上下文
          onRow={
            node.props.onRowClick
              ? (record) => ({
                  onClick: () => onAction(node.props.onRowClick, record as JSONRecord),
                })
              : undefined
          }
          rowSelection={{
            type: 'radio',
            onChange: (_keys, rows) => {
              // V5：选中行写状态（同步）+ 触发 onRowSelected 事件动作
              onSelectionChange?.(node, rows as JSONRecord[]);
            },
          }}
          columns={previewColumns}
          dataSource={items}
        />
      ) : node.type === 'fnFields' ? (
        node.props.onClick ? (
          <div style={{ cursor: 'pointer' }} onClick={() => onAction(node.props.onClick)}>
            {fieldsEl}
          </div>
        ) : (
          fieldsEl
        )
      ) : node.type === 'fnForm' ? (
        // 行内表单：级联输入作初值（对齐发布端 initialValues=sectionInputs），
        // 提交走 runNode（内部合并 cascade < assignments < params）
        <ModalForm
          fn={fn}
          running={running}
          initialValues={cascadeInputs}
          onValuesChange={(values) => onFormValues?.(node.id, values)}
          onSubmit={onSubmit}
          inline
        />
      ) : node.type === 'staticForm' ? (
        <StaticFormLive
          node={node}
          initialValues={undefined}
          onChange={(values) => onStaticChange?.(node.id, values)}
        />
      ) : node.type === 'container' ? (
        <Space orientation="vertical" size={8} style={{ width: '100%' }}>
          {(node.children ?? []).map((c) => (
            <React.Fragment key={c.id}>
              {renderChild?.(c) ?? <Text type="secondary">{c.type}</Text>}
            </React.Fragment>
          ))}
          {(node.children ?? []).length === 0 && (
            <Text type="secondary">
              <FormattedMessage
                id="pages.pageStudio.editor.previewNode.emptyContainer"
                defaultMessage="空容器"
              />
            </Text>
          )}
        </Space>
      ) : null}
    </Card>
  );
}

/** 表单（弹窗/行内共用）：复用 SchemaFormRenderer（与发布渲染器同一 RJSF
 * 运行时），控件与校验行为和 Invoke/操作页一致。 */
export function ModalForm({
  fn,
  running,
  initialValues,
  onValuesChange,
  onSubmit,
  inline,
}: {
  fn: FunctionDescriptor | undefined;
  running: boolean;
  /** V5：弹窗预填初值（行操作/带参 openModal 的求值结果）/ 行内级联输入。 */
  initialValues?: JSONRecord;
  /** 当前值变化（{{var.values.x}} 求值来源），由父级写入页面状态。 */
  onValuesChange?: (values: JSONRecord) => void;
  onSubmit: (params: JSONRecord) => void | Promise<void>;
  inline?: boolean;
}) {
  const raw = fn?.inputSchema;
  const schema = raw && typeof raw === 'object' && !Array.isArray(raw) ? (raw as JSONSchema) : null;
  const spec = useMemo(() => derivePresentationSpec(schema), [schema]);
  const properties = schema?.properties;
  if (!fn || !properties || typeof properties !== 'object') {
    return (
      <Button type="primary" block loading={running} onClick={() => void onSubmit({})}>
        <FormattedMessage
          id="pages.pageStudio.editor.previewNode.confirmButton"
          defaultMessage="确认执行"
        />
      </Button>
    );
  }
  return (
    <SchemaFormRenderer
      spec={spec}
      initialValues={(initialValues ?? {}) as FormValues}
      disabled={running}
      onValuesChange={(_, all) => onValuesChange?.(all as JSONRecord)}
      onFinish={async (values) => {
        await onSubmit(values as JSONRecord);
      }}
    />
  );
}

/** 常量表单预览态：与发布渲染同一 rjsf 运行时（真实控件可交互），
 * 值防抖并入预览页面状态（驱动预览内 refreshOn/动作链消费）。 */
export function StaticFormLive({
  node,
  initialValues,
  onChange,
  debounceMs = 300,
}: {
  node: PageNode;
  initialValues?: JSONRecord;
  onChange?: (values: JSONRecord) => void;
  debounceMs?: number;
}) {
  const raw = node.props.staticSchema;
  const spec = useMemo<FormPresentationSpec | null>(() => {
    try {
      const schema =
        typeof raw === 'string' ? (JSON.parse(raw) as JSONSchema) : (raw as unknown as JSONSchema);
      if (!schema || typeof schema !== 'object') return null;
      return derivePresentationSpec(schema);
    } catch {
      return null;
    }
  }, [raw]);

  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  useEffect(
    () => () => {
      if (timerRef.current) clearTimeout(timerRef.current);
    },
    [],
  );

  if (!spec) {
    return (
      <Text type="warning">
        {/* 与画布 staticForm 预览同义（字段定义 JSON 无效），复用同一键 */}
        <FormattedMessage
          id="pages.pageStudio.editor.component.staticForm.preview.invalid"
          defaultMessage="字段定义 JSON 无效"
        />
      </Text>
    );
  }
  const handleValuesChange: SchemaFormRendererProps['onValuesChange'] = (_changed, all) => {
    if (timerRef.current) clearTimeout(timerRef.current);
    timerRef.current = setTimeout(() => {
      onChange?.(all as JSONRecord);
    }, debounceMs);
  };
  return (
    <SchemaFormRenderer
      spec={spec}
      initialValues={initialValues as SchemaFormRendererProps['initialValues']}
      hideSubmit
      disabled={false}
      onValuesChange={handleValuesChange}
    />
  );
}
