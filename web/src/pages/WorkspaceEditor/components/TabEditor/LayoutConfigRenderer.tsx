import React, { useState } from 'react';
import {
  Card,
  Space,
  Form,
  Select,
  Button,
  Table,
  Popconfirm,
  message,
  Input,
  Upload,
  Tooltip,
  Collapse,
  Modal,
  Tag,
  Typography,
} from 'antd';
import {
  PlusOutlined,
  EditOutlined,
  DeleteOutlined,
  ThunderboltOutlined,
  HolderOutlined,
  ExportOutlined,
  ImportOutlined,
  AppstoreOutlined,
} from '@ant-design/icons';
import type { TabConfig, ColumnConfig, FieldConfig } from '@/types/workspace';
import type { FunctionDescriptor } from '@/services/api/functions';
import {
  schemaToColumns,
  schemaToDetailSections,
  schemaToFields,
} from '../../utils/schemaToLayout';
import { HelpTooltip } from '../HelpTooltip';
import FieldLibrary, { createFieldFromTemplate, type FieldTemplate } from '../FieldLibrary';
import {
  DndContext,
  closestCenter,
  PointerSensor,
  KeyboardSensor,
  useSensor,
  useSensors,
  DragEndEvent,
  DragOverlay,
} from '@dnd-kit/core';
import {
  SortableContext,
  sortableKeyboardCoordinates,
  useSortable,
  verticalListSortingStrategy,
  arrayMove,
} from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';

interface DraggableTableProps<T extends { key: string }> {
  dataSource: T[];
  columns: any[];
  onReorder: (items: T[]) => void;
  locale?: { emptyText: string };
}

const SortableRow = React.forwardRef<
  HTMLTableRowElement,
  React.HTMLAttributes<HTMLTableRowElement> & { 'data-row-key'?: string }
>(({ 'data-row-key': rowKey, ...props }, _ref) => {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging, over } =
    useSortable({
      id: rowKey || '',
    });
  const style: React.CSSProperties = {
    ...props.style,
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.5 : 1,
    backgroundColor: over ? '#e6f7ff' : undefined,
  };
  return <tr ref={setNodeRef} {...props} {...attributes} style={style} />;
});

function DraggableTable<T extends { key: string }>({
  dataSource,
  columns,
  onReorder,
  locale,
}: DraggableTableProps<T>) {
  const [activeId, setActiveId] = React.useState<string | null>(null);

  const sensors = useSensors(
    useSensor(PointerSensor, {
      activationConstraint: {
        distance: 8,
      },
    }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  );

  const handleDragStart = (event: DragEndEvent) => {
    setActiveId(event.active.id as string);
  };

  const handleDragEnd = (event: DragEndEvent) => {
    const { active, over } = event;
    setActiveId(null);
    if (!over || active.id === over.id) return;
    const oldIndex = dataSource.findIndex((r) => r.key === active.id);
    const newIndex = dataSource.findIndex((r) => r.key === over.id);
    if (oldIndex !== -1 && newIndex !== -1) {
      onReorder(arrayMove(dataSource, oldIndex, newIndex));
    }
  };

  const dragHandleCol = {
    title: '',
    dataIndex: '_drag',
    width: 32,
    render: (_: any, record: T) => <DragHandle rowKey={record.key} />,
  };

  const activeItem = activeId ? dataSource.find((item) => item.key === activeId) : null;

  return (
    <DndContext
      sensors={sensors}
      collisionDetection={closestCenter}
      onDragStart={handleDragStart}
      onDragEnd={handleDragEnd}
    >
      <SortableContext items={dataSource.map((r) => r.key)} strategy={verticalListSortingStrategy}>
        <Table
          size="small"
          dataSource={dataSource}
          rowKey="key"
          pagination={false}
          columns={[dragHandleCol, ...columns]}
          locale={locale}
          components={{
            body: {
              row: SortableRow,
            },
          }}
        />
      </SortableContext>
      <DragOverlay>
        {activeItem && (
          <div
            style={{
              background: '#fff',
              padding: '8px 16px',
              borderRadius: 4,
              boxShadow: '0 4px 12px rgba(0,0,0,0.15)',
              border: '2px solid #1677ff',
              display: 'flex',
              alignItems: 'center',
              gap: 8,
              minWidth: 200,
            }}
          >
            <HolderOutlined style={{ color: '#999' }} />
            <span style={{ fontSize: 12 }}>
              {columns[0]?.render?.(undefined, activeItem) || activeItem.key}
            </span>
          </div>
        )}
      </DragOverlay>
    </DndContext>
  );
}

function DragHandle({ rowKey }: { rowKey: string }) {
  const { listeners, attributes } = useSortable({ id: rowKey });
  return (
    <HolderOutlined
      {...listeners}
      {...attributes}
      style={{ cursor: 'grab', color: '#999', touchAction: 'none' }}
    />
  );
}

export interface LayoutConfigRendererProps {
  layout: any;
  tab: TabConfig;
  descriptors: FunctionDescriptor[];
  onTabChange: (tab: TabConfig) => void;
  onOpenColumnEditor: (column: ColumnConfig | null) => void;
  onOpenFieldEditor: (field: FieldConfig | null) => void;
}

function SectionGuideCard({
  title,
  status,
  description,
  detail,
}: {
  title: string;
  status: string;
  description: string;
  detail: string;
}) {
  return (
    <Card size="small" style={{ background: 'linear-gradient(180deg, #fcfcfc 0%, #f7f9fc 100%)' }}>
      <Space direction="vertical" size={10} style={{ width: '100%' }}>
        <Space wrap size={[8, 8]}>
          <Typography.Text strong>{title}</Typography.Text>
          <Tag color={status.includes('已') ? 'success' : 'orange'}>{status}</Tag>
        </Space>
        <Typography.Text type="secondary" style={{ fontSize: 12 }}>
          {description}
        </Typography.Text>
        <Typography.Text type="secondary" style={{ fontSize: 12 }}>
          {detail}
        </Typography.Text>
      </Space>
    </Card>
  );
}

function StructureSectionCard({
  title,
  count,
  hint,
  extra,
  empty,
  children,
}: {
  title: string;
  count: number;
  hint: string;
  extra?: React.ReactNode;
  empty?: React.ReactNode;
  children: React.ReactNode;
}) {
  return (
    <Card
      size="small"
      title={
        <Space size={8}>
          <Typography.Text strong>{title}</Typography.Text>
          <Tag style={{ margin: 0 }}>{count}</Tag>
        </Space>
      }
      extra={extra}
    >
      <Space direction="vertical" size={12} style={{ width: '100%' }}>
        <Typography.Text type="secondary" style={{ fontSize: 12 }}>
          {hint}
        </Typography.Text>
        {empty}
        {children}
      </Space>
    </Card>
  );
}

function StarterEmptyCard({
  title,
  description,
  actions,
}: {
  title: string;
  description: string;
  actions: React.ReactNode;
}) {
  return (
    <Card size="small" style={{ background: 'rgba(22,119,255,0.03)' }}>
      <Space direction="vertical" size={10} style={{ width: '100%' }}>
        <Typography.Text strong>{title}</Typography.Text>
        <Typography.Text type="secondary" style={{ fontSize: 12 }}>
          {description}
        </Typography.Text>
        <Space wrap size={[8, 8]}>
          {actions}
        </Space>
      </Space>
    </Card>
  );
}

export default function LayoutConfigRenderer({
  layout,
  tab,
  descriptors,
  onTabChange,
  onOpenColumnEditor,
  onOpenFieldEditor,
}: LayoutConfigRendererProps) {
  const layoutTypeLabelMap: Record<string, string> = {
    list: '列表',
    form: '表单',
    detail: '详情',
    'form-detail': '查询详情',
    split: '分栏',
    dashboard: '仪表盘',
    wizard: '向导',
    grid: '网格',
    custom: '自定义',
    kanban: '看板',
    timeline: '时间线',
  };

  const configSummary = (() => {
    const layoutType = layout?.type || 'unknown';
    const labels = [
      { color: 'blue', text: `布局 ${layoutTypeLabelMap[layoutType] || layoutType}` },
    ];

    if (Array.isArray(layout?.columns) && layout.columns.length > 0) {
      labels.push({ color: 'default', text: `列 ${layout.columns.length}` });
    }
    if (Array.isArray(layout?.fields) && layout.fields.length > 0) {
      labels.push({ color: 'default', text: `字段 ${layout.fields.length}` });
    }
    if (Array.isArray(layout?.queryFields) && layout.queryFields.length > 0) {
      labels.push({ color: 'default', text: `查询字段 ${layout.queryFields.length}` });
    }
    if (Array.isArray(layout?.sections) && layout.sections.length > 0) {
      labels.push({ color: 'default', text: `分区 ${layout.sections.length}` });
    }
    return labels;
  })();

  const primaryFunctionId = tab.functions?.[0] || '';
  const primaryDescriptor = descriptors.find((d) => d.id === primaryFunctionId);
  const currentBindingId =
    layout?.listFunction ||
    layout?.submitFunction ||
    layout?.detailFunction ||
    layout?.queryFunction ||
    layout?.dataFunction ||
    '';
  const currentBindingDescriptor = descriptors.find((d) => d.id === currentBindingId);

  const nextStepHint =
    layout?.type === 'list'
      ? '先确认列表函数，再补列配置；列已经够用时，不必立即进入 JSON。'
      : layout?.type === 'form'
      ? '先确认提交函数，再补字段；字段库适合补常见输入项，JSON 只留给复杂场景。'
      : layout?.type === 'detail'
      ? '先确认详情函数，再按分区组织信息；结构清楚后再考虑 JSON 微调。'
      : layout?.type === 'form-detail'
      ? '先绑定查询函数，再补查询字段；等基础流转通了，再处理复杂联动。'
      : '当前布局建议优先确认核心结构，只有标准编辑器不够时再使用 JSON。';

  const handleExportLayout = () => {
    const json = JSON.stringify(tab.layout, null, 2);
    const blob = new Blob([json], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `${tab.key || 'tab'}-layout.json`;
    a.click();
    URL.revokeObjectURL(url);
    message.success('布局配置已导出');
  };

  const handleImportLayout = (file: File) => {
    const reader = new FileReader();
    reader.onload = (e) => {
      try {
        const parsed = JSON.parse(e.target?.result as string);
        if (!parsed || typeof parsed !== 'object' || !parsed.type) {
          message.error('无效的布局配置文件：缺少 type 字段');
          return;
        }
        onTabChange({ ...tab, layout: parsed });
        message.success('布局配置已导入');
      } catch {
        message.error('JSON 解析失败，请检查文件格式');
      }
    };
    reader.readAsText(file);
    return false; // prevent upload
  };

  const layoutContent = (() => {
    switch (layout.type) {
      case 'list':
        return (
          <ListLayoutConfig
            layout={layout}
            tab={tab}
            descriptors={descriptors}
            onTabChange={onTabChange}
            onOpenColumnEditor={onOpenColumnEditor}
          />
        );
      case 'form-detail':
        return (
          <FormDetailLayoutConfig
            layout={layout}
            tab={tab}
            descriptors={descriptors}
            onTabChange={onTabChange}
            onOpenFieldEditor={onOpenFieldEditor}
          />
        );
      case 'form':
        return (
          <FormLayoutConfig
            layout={layout}
            tab={tab}
            descriptors={descriptors}
            onTabChange={onTabChange}
            onOpenFieldEditor={onOpenFieldEditor}
          />
        );
      case 'detail':
        return (
          <DetailLayoutConfig
            layout={layout}
            tab={tab}
            descriptors={descriptors}
            onTabChange={onTabChange}
          />
        );
      case 'kanban':
      case 'timeline':
      case 'split':
      case 'wizard':
      case 'dashboard':
      case 'grid':
      case 'custom':
        return <SimpleJsonConfig layout={layout} tab={tab} onTabChange={onTabChange} />;
      default:
        return <div style={{ color: '#999' }}>请选择布局类型</div>;
    }
  })();

  return (
    <div>
      <Space direction="vertical" size={12} style={{ width: '100%' }}>
        <Space wrap size={[8, 8]}>
          {configSummary.map((item) => (
            <Tag key={`${item.color}-${item.text}`} color={item.color} style={{ margin: 0 }}>
              {item.text}
            </Tag>
          ))}
        </Space>
        <Card
          size="small"
          style={{ background: 'linear-gradient(180deg, #fffdf7 0%, #ffffff 100%)' }}
        >
          <Space direction="vertical" size={10} style={{ width: '100%' }}>
            <Space wrap size={[8, 8]}>
              <Typography.Text strong>详细配置建议分步完成</Typography.Text>
              <Tag color={currentBindingId ? 'success' : 'orange'}>
                {currentBindingId ? '核心函数已确定' : '当前待定核心函数'}
              </Tag>
            </Space>
            <Typography.Text type="secondary" style={{ fontSize: 12 }}>
              {nextStepHint}
            </Typography.Text>
            <Typography.Text type="secondary" style={{ fontSize: 12 }}>
              {currentBindingId
                ? `当前页面核心函数是 ${
                    currentBindingDescriptor?.displayName?.zh || currentBindingId
                  }。下面的自动填充、字段生成和结构补全都会优先围绕它展开。`
                : `建议先选一个核心函数。默认可从主函数 ${
                    primaryDescriptor?.displayName?.zh || primaryFunctionId || '未设置'
                  } 开始，先把页面主流程跑通。`}
            </Typography.Text>
          </Space>
        </Card>
        <Collapse
          ghost
          size="small"
          items={[
            {
              key: 'tools',
              label: '导入导出工具',
              children: (
                <Space wrap>
                  <Tooltip title="导出当前 Tab 布局配置为 JSON 文件">
                    <Button size="small" icon={<ExportOutlined />} onClick={handleExportLayout}>
                      导出布局
                    </Button>
                  </Tooltip>
                  <Upload accept=".json" showUploadList={false} beforeUpload={handleImportLayout}>
                    <Tooltip title="从 JSON 文件导入布局配置（将覆盖当前配置）">
                      <Button size="small" icon={<ImportOutlined />}>
                        导入布局
                      </Button>
                    </Tooltip>
                  </Upload>
                </Space>
              ),
            },
          ]}
        />
        {layoutContent}
      </Space>
    </div>
  );
}

// List Layout Config
function ListLayoutConfig({
  layout,
  tab,
  descriptors,
  onTabChange,
  onOpenColumnEditor,
}: {
  layout: any;
  tab: TabConfig;
  descriptors: FunctionDescriptor[];
  onTabChange: (tab: TabConfig) => void;
  onOpenColumnEditor: (column: ColumnConfig | null) => void;
}) {
  const columns: ColumnConfig[] = layout.columns || [];

  const autoFill = () => {
    const funcId = layout.listFunction || tab.functions[0];
    const desc = descriptors.find((d) => d.id === funcId);
    if (!desc) {
      message.warning('未找到函数描述符');
      return;
    }
    const cols = schemaToColumns(desc);
    onTabChange({ ...tab, layout: { ...layout, columns: cols } });
    message.success(`已自动生成 ${cols.length} 列`);
  };

  const removeCol = (key: string) => {
    onTabChange({ ...tab, layout: { ...layout, columns: columns.filter((c) => c.key !== key) } });
  };

  const reorderCols = (next: ColumnConfig[]) => {
    onTabChange({ ...tab, layout: { ...layout, columns: next } });
  };

  return (
    <Space direction="vertical" style={{ width: '100%' }}>
      <SectionGuideCard
        title={layout.listFunction ? '列表数据来源已设置' : '先选择列表数据来源'}
        status={layout.listFunction ? '数据源已确定' : '当前待定数据源'}
        description="先确定这个列表到底由哪个函数返回数据，再决定列配置。没有主数据来源时，下面的列配置很容易越配越乱。"
        detail="列表页通常先确定数据来源，再补列；如果只想先跑通页面，优先保留核心列。"
      />
      <Card size="small" title="核心配置">
        <Form layout="vertical">
          <Form.Item
            label={
              <Space size={4}>
                <span>列表函数</span>
                <HelpTooltip helpKey="layout.listFunction" />
              </Space>
            }
          >
            <Select
              value={layout.listFunction}
              onChange={(v) => {
                const nextLayout: any = { ...layout, listFunction: v };
                const missingColumns =
                  !Array.isArray(nextLayout.columns) || nextLayout.columns.length === 0;
                if (v && missingColumns) {
                  const desc = descriptors.find((d) => d.id === v);
                  if (desc) {
                    nextLayout.columns = schemaToColumns(desc);
                  }
                }
                onTabChange({ ...tab, layout: nextLayout });
              }}
              placeholder="选择列表函数"
              allowClear
              showSearch
            >
              {tab.functions.map((fid) => {
                const d = descriptors.find((x) => x.id === fid);
                return (
                  <Select.Option key={fid} value={fid}>
                    {d?.displayName?.zh || fid}
                  </Select.Option>
                );
              })}
            </Select>
          </Form.Item>
        </Form>
      </Card>

      <StructureSectionCard
        title="列表列"
        count={columns.length}
        hint="先把基础列跑通，再考虑细节渲染。表格里优先保留高频字段，低频信息后面再补。"
        extra={
          <Space>
            <Button size="small" icon={<ThunderboltOutlined />} onClick={autoFill}>
              自动填充
            </Button>
            <Button size="small" icon={<PlusOutlined />} onClick={() => onOpenColumnEditor(null)}>
              添加列
            </Button>
          </Space>
        }
        empty={
          columns.length === 0 ? (
            <StarterEmptyCard
              title="先补一版基础列表列"
              description="推荐先用自动填充生成一版基础列，再删掉不需要的列，比从空白开始逐个添加更快。"
              actions={
                <Space wrap size={[8, 8]}>
                  <Button
                    size="small"
                    type="primary"
                    icon={<ThunderboltOutlined />}
                    onClick={autoFill}
                  >
                    自动生成基础列
                  </Button>
                  <Button
                    size="small"
                    icon={<PlusOutlined />}
                    onClick={() => onOpenColumnEditor(null)}
                  >
                    手动添加首列
                  </Button>
                </Space>
              }
            />
          ) : null
        }
      >
        <DraggableTable
          dataSource={columns}
          onReorder={reorderCols}
          columns={[
            { title: '字段名', dataIndex: 'key', width: 100 },
            { title: '标题', dataIndex: 'title', width: 100 },
            { title: '渲染', dataIndex: 'render', width: 80, render: (v: any) => v || 'text' },
            {
              title: '操作',
              width: 80,
              render: (_: any, col: ColumnConfig) => (
                <Space>
                  <Button
                    type="link"
                    size="small"
                    icon={<EditOutlined />}
                    onClick={() => onOpenColumnEditor(col)}
                  />
                  <Popconfirm title="确认删除？" onConfirm={() => removeCol(col.key)}>
                    <Button type="link" danger size="small" icon={<DeleteOutlined />} />
                  </Popconfirm>
                </Space>
              ),
            },
          ]}
          locale={{ emptyText: '暂无列，点击添加或自动填充' }}
        />
      </StructureSectionCard>
    </Space>
  );
}

// Form-Detail Layout Config
function FormDetailLayoutConfig({
  layout,
  tab,
  descriptors,
  onTabChange,
  onOpenFieldEditor,
}: {
  layout: any;
  tab: TabConfig;
  descriptors: FunctionDescriptor[];
  onTabChange: (tab: TabConfig) => void;
  onOpenFieldEditor: (field: FieldConfig | null) => void;
}) {
  const queryFields: FieldConfig[] = layout.queryFields || [];

  const autoFill = () => {
    const funcId = layout.queryFunction || tab.functions[0];
    const desc = descriptors.find((d) => d.id === funcId);
    if (!desc) {
      message.warning('未找到函数描述符');
      return;
    }
    const fields = schemaToFields(desc);
    onTabChange({ ...tab, layout: { ...layout, queryFields: fields } });
    message.success(`已自动生成 ${fields.length} 个查询字段`);
  };

  const removeField = (key: string) => {
    onTabChange({
      ...tab,
      layout: { ...layout, queryFields: queryFields.filter((f) => f.key !== key) },
    });
  };

  const reorderFields = (next: FieldConfig[]) => {
    onTabChange({ ...tab, layout: { ...layout, queryFields: next } });
  };

  return (
    <Space direction="vertical" style={{ width: '100%' }}>
      <SectionGuideCard
        title={layout.queryFunction ? '查询入口已设置' : '先选择查询函数'}
        status={layout.queryFunction ? '查询入口已确定' : '当前待定查询入口'}
        description="查询详情页先把查询入口定住，再去补查询字段。这样页面主流程会更清楚。"
        detail="查询详情适合先搭建查询入口，再补详情区内容，不必一次配完全部字段。"
      />
      <Card size="small" title="核心配置">
        <Form layout="vertical">
          <Form.Item
            label={
              <Space size={4}>
                <span>查询函数</span>
                <HelpTooltip helpKey="layout.queryFunction" />
              </Space>
            }
          >
            <Select
              value={layout.queryFunction}
              onChange={(v) => onTabChange({ ...tab, layout: { ...layout, queryFunction: v } })}
              placeholder="选择查询函数"
              allowClear
              showSearch
            >
              {tab.functions.map((fid) => {
                const d = descriptors.find((x) => x.id === fid);
                return (
                  <Select.Option key={fid} value={fid}>
                    {d?.displayName?.zh || fid}
                  </Select.Option>
                );
              })}
            </Select>
          </Form.Item>
        </Form>
      </Card>

      <StructureSectionCard
        title="查询字段"
        count={queryFields.length}
        hint="查询区优先保留最关键的检索条件，避免首版就堆满条件项。"
        extra={
          <Space>
            <Button size="small" icon={<ThunderboltOutlined />} onClick={autoFill}>
              自动填充
            </Button>
            <Button size="small" icon={<PlusOutlined />} onClick={() => onOpenFieldEditor(null)}>
              添加字段
            </Button>
          </Space>
        }
        empty={
          queryFields.length === 0 ? (
            <StarterEmptyCard
              title="先补查询入口字段"
              description="推荐先自动生成查询字段，让页面先能检索，再回头删减或补充条件。"
              actions={
                <Space wrap size={[8, 8]}>
                  <Button
                    size="small"
                    type="primary"
                    icon={<ThunderboltOutlined />}
                    onClick={autoFill}
                  >
                    自动生成查询字段
                  </Button>
                  <Button
                    size="small"
                    icon={<PlusOutlined />}
                    onClick={() => onOpenFieldEditor(null)}
                  >
                    手动添加首个字段
                  </Button>
                </Space>
              }
            />
          ) : null
        }
      >
        <DraggableTable
          dataSource={queryFields}
          onReorder={reorderFields}
          columns={[
            { title: '字段名', dataIndex: 'key', width: 100 },
            { title: '标签', dataIndex: 'label', width: 100 },
            { title: '类型', dataIndex: 'type', width: 80 },
            {
              title: '操作',
              width: 80,
              render: (_: any, field: FieldConfig) => (
                <Space>
                  <Button
                    type="link"
                    size="small"
                    icon={<EditOutlined />}
                    onClick={() => onOpenFieldEditor(field)}
                  />
                  <Popconfirm title="确认删除？" onConfirm={() => removeField(field.key)}>
                    <Button type="link" danger size="small" icon={<DeleteOutlined />} />
                  </Popconfirm>
                </Space>
              ),
            },
          ]}
          locale={{ emptyText: '暂无字段，点击添加或自动填充' }}
        />
      </StructureSectionCard>
    </Space>
  );
}

// Form Layout Config
function FormLayoutConfig({
  layout,
  tab,
  descriptors,
  onTabChange,
  onOpenFieldEditor,
}: {
  layout: any;
  tab: TabConfig;
  descriptors: FunctionDescriptor[];
  onTabChange: (tab: TabConfig) => void;
  onOpenFieldEditor: (field: FieldConfig | null) => void;
}) {
  const fields: FieldConfig[] = layout.fields || [];
  const [fieldLibraryVisible, setFieldLibraryVisible] = useState(false);

  const autoFill = () => {
    const funcId = layout.submitFunction || tab.functions[0];
    const desc = descriptors.find((d) => d.id === funcId);
    if (!desc) {
      message.warning('未找到函数描述符');
      return;
    }
    const generatedFields = schemaToFields(desc);
    onTabChange({ ...tab, layout: { ...layout, fields: generatedFields } });
    message.success(`已自动生成 ${generatedFields.length} 个字段`);
  };

  const removeField = (key: string) => {
    onTabChange({
      ...tab,
      layout: { ...layout, fields: fields.filter((f) => f.key !== key) },
    });
  };

  const reorderFields = (next: FieldConfig[]) => {
    onTabChange({ ...tab, layout: { ...layout, fields: next } });
  };

  // 从字段库添加字段
  const handleAddFieldFromTemplate = (template: FieldTemplate) => {
    const existingKeys = fields.map((f) => f.key);
    const newField = createFieldFromTemplate(template, existingKeys);
    onTabChange({
      ...tab,
      layout: { ...layout, fields: [...fields, newField] },
    });
    message.success(`已添加字段: ${newField.label}`);
  };

  return (
    <Space direction="vertical" style={{ width: '100%' }}>
      <SectionGuideCard
        title={layout.submitFunction ? '表单提交目标已设置' : '先选择提交函数'}
        status={layout.submitFunction ? '提交目标已确定' : '当前待定提交目标'}
        description="先确定表单最终提交给谁，再去补字段。否则字段越多，返工成本越高。"
        detail="表单页建议先保留必填字段，再用字段库补充常见输入项，避免一次塞入过多低频字段。"
      />
      <Card size="small" title="核心配置">
        <Form layout="vertical">
          <Form.Item
            label={
              <Space size={4}>
                <span>提交函数</span>
                <HelpTooltip helpKey="layout.submitFunction" />
              </Space>
            }
          >
            <Select
              value={layout.submitFunction}
              onChange={(v) => onTabChange({ ...tab, layout: { ...layout, submitFunction: v } })}
              placeholder="选择提交函数"
              allowClear
              showSearch
            >
              {tab.functions.map((fid) => {
                const d = descriptors.find((x) => x.id === fid);
                return (
                  <Select.Option key={fid} value={fid}>
                    {d?.displayName?.zh || fid}
                  </Select.Option>
                );
              })}
            </Select>
          </Form.Item>
        </Form>
      </Card>

      <StructureSectionCard
        title="表单字段"
        count={fields.length}
        hint="字段编辑优先保证提交流程完整，再补说明性字段和次要输入项。"
        extra={
          <Space>
            <Button size="small" icon={<ThunderboltOutlined />} onClick={autoFill}>
              自动填充
            </Button>
            <Button
              size="small"
              icon={<AppstoreOutlined />}
              onClick={() => setFieldLibraryVisible(true)}
            >
              字段库
            </Button>
            <Button size="small" icon={<PlusOutlined />} onClick={() => onOpenFieldEditor(null)}>
              添加字段
            </Button>
          </Space>
        }
        empty={
          fields.length === 0 ? (
            <StarterEmptyCard
              title="先补一版表单字段"
              description="推荐先自动生成必填字段，或从字段库挑常见输入项，不要一开始就手写完整表单。"
              actions={
                <Space wrap size={[8, 8]}>
                  <Button
                    size="small"
                    type="primary"
                    icon={<ThunderboltOutlined />}
                    onClick={autoFill}
                  >
                    自动生成字段
                  </Button>
                  <Button
                    size="small"
                    icon={<AppstoreOutlined />}
                    onClick={() => setFieldLibraryVisible(true)}
                  >
                    从字段库选择
                  </Button>
                  <Button
                    size="small"
                    icon={<PlusOutlined />}
                    onClick={() => onOpenFieldEditor(null)}
                  >
                    手动添加首个字段
                  </Button>
                </Space>
              }
            />
          ) : null
        }
      >
        <DraggableTable
          dataSource={fields}
          onReorder={reorderFields}
          columns={[
            { title: '字段名', dataIndex: 'key', width: 100 },
            { title: '标签', dataIndex: 'label', width: 100 },
            { title: '类型', dataIndex: 'type', width: 80 },
            {
              title: '操作',
              width: 80,
              render: (_: any, field: FieldConfig) => (
                <Space>
                  <Button
                    type="link"
                    size="small"
                    icon={<EditOutlined />}
                    onClick={() => onOpenFieldEditor(field)}
                  />
                  <Popconfirm title="确认删除？" onConfirm={() => removeField(field.key)}>
                    <Button type="link" danger size="small" icon={<DeleteOutlined />} />
                  </Popconfirm>
                </Space>
              ),
            },
          ]}
          locale={{ emptyText: '暂无字段，点击添加或自动填充' }}
        />
      </StructureSectionCard>
      <Modal
        title="从字段库添加"
        open={fieldLibraryVisible}
        onCancel={() => setFieldLibraryVisible(false)}
        footer={null}
        width={400}
      >
        <FieldLibrary
          onFieldClick={(template) => {
            handleAddFieldFromTemplate(template);
            setFieldLibraryVisible(false);
          }}
        />
      </Modal>
    </Space>
  );
}

// Detail Layout Config
function DetailLayoutConfig({
  layout,
  tab,
  descriptors,
  onTabChange,
}: {
  layout: any;
  tab: TabConfig;
  descriptors: FunctionDescriptor[];
  onTabChange: (tab: TabConfig) => void;
}) {
  const sections = layout.sections || [];

  const removeSection = (index: number) => {
    const nextSections = sections.filter((_: any, i: number) => i !== index);
    onTabChange({ ...tab, layout: { ...layout, sections: nextSections } });
  };

  const addSection = () => {
    const newSection = {
      title: `分区 ${sections.length + 1}`,
      fields: [],
    };
    onTabChange({ ...tab, layout: { ...layout, sections: [...sections, newSection] } });
  };

  const updateSection = (index: number, key: string, value: any) => {
    const nextSections = [...sections];
    nextSections[index] = { ...nextSections[index], [key]: value };
    onTabChange({ ...tab, layout: { ...layout, sections: nextSections } });
  };

  const reorderSections = (nextSections: any[]) => {
    onTabChange({ ...tab, layout: { ...layout, sections: nextSections } });
  };

  return (
    <Space direction="vertical" style={{ width: '100%' }}>
      <SectionGuideCard
        title={layout.detailFunction ? '详情数据来源已设置' : '先选择详情函数'}
        status={layout.detailFunction ? '详情来源已确定' : '当前待定详情来源'}
        description="详情页先确定读取哪一个详情函数，再按阅读顺序组织分区，避免后面反复重排。"
        detail="详情页优先按阅读顺序组织分区，先让信息结构清楚，再决定是否进入 JSON 调整。"
      />
      <Card size="small" title="核心配置">
        <Form layout="vertical">
          <Form.Item
            label={
              <Space size={4}>
                <span>详情函数</span>
                <HelpTooltip helpKey="layout.detailFunction" />
              </Space>
            }
          >
            <Select
              value={layout.detailFunction}
              onChange={(v) => onTabChange({ ...tab, layout: { ...layout, detailFunction: v } })}
              placeholder="选择详情函数"
              allowClear
              showSearch
            >
              {tab.functions.map((fid) => {
                const d = descriptors.find((x) => x.id === fid);
                return (
                  <Select.Option key={fid} value={fid}>
                    {d?.displayName?.zh || fid}
                  </Select.Option>
                );
              })}
            </Select>
          </Form.Item>
        </Form>
      </Card>

      <StructureSectionCard
        title="分区配置"
        count={sections.length}
        hint="详情区建议先搭出阅读顺序，再逐步补字段；首版先做 1 到 2 个分区就够。"
        extra={
          <Button size="small" icon={<PlusOutlined />} onClick={addSection}>
            添加分区
          </Button>
        }
        empty={
          sections.length === 0 ? (
            <StarterEmptyCard
              title="先建 1 到 2 个信息分区"
              description="详情页先把阅读顺序搭出来，常见做法是“基本信息 + 扩展信息”两段。"
              actions={
                <Button size="small" type="primary" icon={<PlusOutlined />} onClick={addSection}>
                  添加首个分区
                </Button>
              }
            />
          ) : null
        }
      >
        <DraggableTable
          dataSource={sections.map((s: any, i: number) => ({ ...s, key: String(i) }))}
          onReorder={reorderSections}
          columns={[
            {
              title: '标题',
              dataIndex: 'title',
              width: 120,
              render: (v: any, rec: any, index: number) => (
                <Input
                  size="small"
                  value={v}
                  onChange={(e) => updateSection(Number(rec.key), 'title', e.target.value)}
                  placeholder="分区标题"
                />
              ),
            },
            {
              title: '字段数',
              dataIndex: 'fields',
              width: 80,
              render: (fields: any[]) => fields?.length || 0,
            },
            {
              title: '操作',
              width: 60,
              render: (_: any, rec: any) => (
                <Popconfirm title="确认删除？" onConfirm={() => removeSection(Number(rec.key))}>
                  <Button type="link" danger size="small" icon={<DeleteOutlined />} />
                </Popconfirm>
              ),
            },
          ]}
          locale={{ emptyText: '暂无分区，点击添加或切换到 JSON 编辑' }}
        />
      </StructureSectionCard>

      <Collapse
        size="small"
        items={[
          {
            key: 'advanced-detail-json',
            label: '高级 JSON 编辑',
            children: (
              <Space direction="vertical" size={10} style={{ width: '100%' }}>
                <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                  当标准分区编辑不够用时，再使用 JSON
                  直接调整分区结构。这里适合处理批量粘贴或复杂结构修正。
                </Typography.Text>
                <Input.TextArea
                  rows={6}
                  value={JSON.stringify(sections, null, 2)}
                  onChange={(e) => {
                    try {
                      const parsed = JSON.parse(e.target.value);
                      if (Array.isArray(parsed)) {
                        onTabChange({ ...tab, layout: { ...layout, sections: parsed } });
                      }
                    } catch {
                      // ignore invalid json while editing
                    }
                  }}
                />
              </Space>
            ),
          },
        ]}
      />
    </Space>
  );
}

// Simple JSON Config for complex layouts
function SimpleJsonConfig({
  layout,
  tab,
  onTabChange,
}: {
  layout: any;
  tab: TabConfig;
  onTabChange: (tab: TabConfig) => void;
}) {
  return (
    <Card size="small" title="高级布局配置">
      <Space direction="vertical" size={10} style={{ width: '100%' }}>
        <Typography.Text type="secondary" style={{ fontSize: 12 }}>
          当前布局暂时以 JSON
          方式编辑。建议先确认主结构，再在这里做精细调整；复杂布局不应作为默认主路径能力使用。
        </Typography.Text>
        <Form layout="vertical">
          <Form.Item label="布局配置(JSON)">
            <Input.TextArea
              rows={15}
              value={JSON.stringify(layout, null, 2)}
              onChange={(e) => {
                try {
                  const parsed = JSON.parse(e.target.value);
                  onTabChange({ ...tab, layout: parsed });
                } catch {
                  // ignore invalid json while editing
                }
              }}
            />
          </Form.Item>
        </Form>
      </Space>
    </Card>
  );
}
