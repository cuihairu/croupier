import React, { useEffect, useState } from 'react';
import { Alert, Button, Modal, Space, Typography } from 'antd';
import { FormattedMessage, request, useIntl } from '@umijs/max';
import ConstantFieldsEditor from '../CompositeEditor/ConstantFieldsEditor';
import {
  fieldsToSchemaJson,
  schemaToFields,
  staticFormNodeFromFields,
  type ConstantField,
} from '../CompositeEditor/constants';
import type { PageNode } from '../CompositeEditor/model';
import { localizedText } from '@/utils/localizedText';
import type { LocalizedText } from '@/types/dashboard';

const { Text } = Typography;

/** 编辑目标：常量模板的最小结构面（与页面 TemplateDTO 解耦，避免循环 import）。 */
export interface ConstantTemplateTarget {
  key: string;
  name: LocalizedText | string;
  builtin: boolean;
  tree: PageNode[];
}

/** 取模板树里 staticForm 节点的 staticSchema（无则返回空串 → 编辑器空态）。 */
function constantSchemaOf(tpl: ConstantTemplateTarget): string {
  for (const node of tpl.tree ?? []) {
    if (!node || typeof node !== 'object') continue;
    if ((node as { type?: unknown }).type !== 'staticForm') continue;
    const props: unknown = (node as { props?: unknown }).props;
    if (!props || typeof props !== 'object') continue;
    const schema = (props as { staticSchema?: unknown }).staticSchema;
    if (typeof schema === 'string') return schema;
  }
  return '';
}

/**
 * 单条常量编辑弹窗（OPEN-ISSUES #6）：常量随功能开发单个新增/修改，不必走
 * 批量导入。复用 ConstantFieldsEditor（显示名/变量名/逐行选项增删）。
 * - template=null → 新增：与导入通道同语义，每个常量字段独立 POST 成一个
 *   `consts--*` 组件（一种常量一个组件规范）。
 * - template 给定 → 编辑：读取树内 staticSchema 回填，保存走既有
 *   `PUT /component-templates/:key`（后端会重算 digest 供 U11 提醒）。
 * 批量导入通道（ConstantImportModal）原样保留。
 */
export default function ConstantEditModal({
  open,
  template,
  onCancel,
  onSaved,
}: {
  open: boolean;
  /** null = 新增单常量；否则编辑该常量模板。 */
  template: ConstantTemplateTarget | null;
  onCancel: () => void;
  /** 保存成功回调（编辑传常量名；新增传创建数量文案）。 */
  onSaved: (summary: string) => void;
}) {
  const [fields, setFields] = useState<ConstantField[]>([]);
  const [error, setError] = useState('');
  const [saving, setSaving] = useState(false);
  const intl = useIntl();

  // 打开时按目标初始化：编辑回填 staticSchema；新增给一行空白常量即改即存
  useEffect(() => {
    if (!open) return;
    setError('');
    if (template) {
      setFields(schemaToFields(constantSchemaOf(template)));
    } else {
      setFields([{ key: '新常量', title: '新常量', options: [{ value: '选项1' }] }]);
    }
  }, [open, template]);

  const reset = () => {
    setFields([]);
    setError('');
  };

  const payloadTree = (fs: ConstantField[], displayName: string) => [
    staticFormNodeFromFields(fs, displayName, 12),
  ];

  const payloadBase = (displayName: string, count: number) => ({
    name: { 'zh-CN': displayName, 'en-US': displayName },
    description:
      count === 1
        ? {
            'zh-CN': `常量下拉（${fields[0].options.length} 个选项）`,
            'en-US': `Constant dropdown (${fields[0].options.length} options)`,
          }
        : {
            'zh-CN': `常量组件（${count} 个常量）`,
            'en-US': `Constant component (${count} constants)`,
          },
    category: '常量',
    icon: 'ControlOutlined',
    requiredFunctions: [] as string[],
  });

  const save = async () => {
    if (fields.length === 0) {
      setError(
        intl.formatMessage({
          id: 'pages.pageStudio.templates.constantEdit.emptyFields',
          defaultMessage: '请至少保留一个常量（空模板无法渲染下拉）',
        }),
      );
      return;
    }
    setSaving(true);
    try {
      if (template) {
        // 编辑：整模板保存（单常量场景即改这一个常量；遗留合并模板整组回写，
        // 「一种常量一个组件」规范由列表页 legacy 告警引导清理）
        const single = fields.length === 1;
        const displayName = single
          ? fields[0].title || fields[0].key
          : localizedText(template.name, 'zh-CN', template.key);
        await request(`/api/v1/component-templates/${encodeURIComponent(template.key)}`, {
          method: 'PUT',
          data: {
            key: template.key,
            ...payloadBase(displayName, fields.length),
            tree: payloadTree(fields, displayName),
          },
          skipErrorHandler: true,
        });
        onSaved(displayName);
      } else {
        // 新增：与导入同语义——一种常量一个独立组件，逐字段 POST
        const batch = Date.now().toString(36);
        let created = 0;
        for (const [i, f] of fields.entries()) {
          const displayName = f.title || f.key;
          await request('/api/v1/component-templates', {
            method: 'POST',
            data: {
              key: `consts--${batch}-${i}`,
              ...payloadBase(displayName, 1),
              tree: payloadTree([f], displayName),
            },
            skipErrorHandler: true,
          });
          created += 1;
        }
        onSaved(
          intl.formatMessage(
            {
              id: 'pages.pageStudio.templates.constantEdit.created',
              defaultMessage: '已创建 {count} 个常量组件',
            },
            { count: created },
          ),
        );
      }
      reset();
    } catch (e) {
      setError(
        e instanceof Error
          ? e.message
          : intl.formatMessage({
              id: 'pages.pageStudio.templates.constantEdit.saveFailed',
              defaultMessage: '保存失败',
            }),
      );
    } finally {
      setSaving(false);
    }
  };

  return (
    <Modal
      title={
        template ? (
          <FormattedMessage
            id="pages.pageStudio.templates.constantEdit.titleEdit"
            defaultMessage="编辑常量"
          />
        ) : (
          <FormattedMessage
            id="pages.pageStudio.templates.constantEdit.titleCreate"
            defaultMessage="新增常量"
          />
        )
      }
      width={640}
      open={open}
      onCancel={() => {
        reset();
        onCancel();
      }}
      footer={
        <Space>
          <Button
            onClick={() => {
              reset();
              onCancel();
            }}
          >
            <FormattedMessage id="app.cancel" defaultMessage="取 消" />
          </Button>
          <Button type="primary" loading={saving} onClick={() => void save()}>
            <FormattedMessage
              id="pages.pageStudio.templates.constantEdit.save"
              defaultMessage="保 存"
            />
          </Button>
        </Space>
      }
    >
      <Space orientation="vertical" size={10} style={{ width: '100%' }}>
        <Text type="secondary" style={{ fontSize: 12 }}>
          <FormattedMessage
            id="pages.pageStudio.templates.constantEdit.conventionHint"
            defaultMessage="一种常量一个独立组件：改显示名/变量名/选项后保存即可；批量改多条请走「导入常量」。"
          />
        </Text>
        {error && <Alert type="error" showIcon title={error} />}
        <ConstantFieldsEditor
          value={fieldsToSchemaJson(fields)}
          onChange={(v) => setFields(schemaToFields(v))}
        />
      </Space>
    </Modal>
  );
}
