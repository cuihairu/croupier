import React, { useState } from 'react';
import { Alert, Button, Modal, Radio, Space, Typography, Upload } from 'antd';
import { UploadOutlined } from '@ant-design/icons';
import { request } from '@umijs/max';
import * as XLSX from 'xlsx';
import {
  fieldsToSchemaJson,
  jsonToFields,
  rowsToFields,
  schemaToFields,
  staticFormNodeFromFields,
  type ConstantField,
  type ImportMode,
} from './constants';
import ConstantFieldsEditor from './ConstantFieldsEditor';

const { Text } = Typography;

/**
 * 导入常量向导（组件库入口）：上传 Excel/JSON → 预览/微调 → 保存为
 * 常量模板（staticForm 子树）。保存后组件库出现该模板，组合页拖选即用；
 * 常量配置更新时删除旧模板重新导入（或后续做在线更新）。
 */
export default function ConstantImportModal({
  open,
  onCancel,
  onSaved,
}: {
  open: boolean;
  onCancel: () => void;
  /** 保存成功回调（携带模板名，用于提示/刷新列表）。 */
  onSaved: (name: string) => void;
}) {
  const [fields, setFields] = useState<ConstantField[]>([]);
  const [importMode, setImportMode] = useState<ImportMode>('long');
  const [error, setError] = useState('');
  const [saving, setSaving] = useState(false);

  const reset = () => {
    setFields([]);
    setError('');
  };

  const beforeUpload = (file: File) => {
    const reader = new FileReader();
    if (/\.(xlsx|xls|csv)$/i.test(file.name)) {
      reader.onload = () => {
        try {
          const wb = XLSX.read(reader.result, { type: 'array' });
          const sheet = wb.Sheets[wb.SheetNames[0]];
          const rows = XLSX.utils.sheet_to_json<unknown[]>(sheet, { header: 1, defval: '' });
          setFields(rowsToFields(rows, importMode));
          setError('');
        } catch (e) {
          setError(e instanceof Error ? e.message : 'Excel 解析失败');
        }
      };
      reader.readAsArrayBuffer(file);
    } else {
      reader.onload = () => {
        try {
          const parsed: unknown = JSON.parse(String(reader.result));
          const imported = jsonToFields(parsed);
          if (imported.length === 0) {
            setError('JSON 需为 {"常量名":[选项…]} 或 [{"name":"…","options":[…]}]');
            return;
          }
          setFields(imported);
          setError('');
        } catch (e) {
          setError(e instanceof Error ? e.message : 'JSON 解析失败');
        }
      };
      reader.readAsText(file);
    }
    return Upload.LIST_IGNORE;
  };

  const save = async () => {
    if (fields.length === 0) {
      setError('请先导入常量（Excel/JSON）或添加字段');
      return;
    }
    setSaving(true);
    try {
      // 一种常量一个组件：每个常量保存为独立的单下拉 staticForm 模板，
      // 组件库按常量名展示，组合页里自由拖选数量与位置
      const batch = Date.now().toString(36);
      let created = 0;
      for (const [i, f] of fields.entries()) {
        await request('/api/v1/component-templates', {
          method: 'POST',
          data: {
            key: `consts--${batch}-${i}`,
            name: { 'zh-CN': f.title || f.key, 'en-US': f.title || f.key },
            description: {
              'zh-CN': `常量下拉（${f.options.length} 个选项）`,
              'en-US': `Constant dropdown (${f.options.length} options)`,
            },
            category: '常量',
            icon: 'ControlOutlined',
            requiredFunctions: [],
            tree: [staticFormNodeFromFields([f], f.title || f.key, 12)],
          },
          skipErrorHandler: true,
        });
        created += 1;
      }
      onSaved(`${created} 个常量组件`);
      reset();
    } catch (e) {
      setError(e instanceof Error ? e.message : '保存失败');
    } finally {
      setSaving(false);
    }
  };

  return (
    <Modal
      title="导入常量"
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
            取消
          </Button>
          <Button type="primary" loading={saving} onClick={() => void save()}>
            全部保存（{fields.length} 个组件）
          </Button>
        </Space>
      }
    >
      <Space direction="vertical" size={10} style={{ width: '100%' }}>
        <Space size={8} wrap>
          <Radio.Group
            size="small"
            value={importMode}
            onChange={(e) => setImportMode(e.target.value as ImportMode)}
          >
            <Radio.Button value="long">长表：名称|值|标签</Radio.Button>
            <Radio.Button value="wide">宽表：名称|选项…</Radio.Button>
          </Radio.Group>
          <Upload accept=".xlsx,.xls,.csv,.json" showUploadList={false} beforeUpload={beforeUpload}>
            <Button icon={<UploadOutlined />}>上传 Excel / JSON</Button>
          </Upload>
        </Space>
        <Text type="secondary" style={{ fontSize: 12 }}>
          Excel 长表：名称|值|标签（同名称多行聚合）；宽表：名称|选项…；JSON：
          {'{"常量名":[选项…]}'}
        </Text>

        {error && <Alert type="error" showIcon message={error} />}

        {fields.length > 0 && (
          <>
            <Text strong style={{ fontSize: 12 }}>
              常量预览（{fields.length} 个，每个常量将保存为一个独立下拉组件）
            </Text>
            <ConstantFieldsEditor
              value={fieldsToSchemaJson(fields)}
              onChange={(v) => setFields(schemaToFields(v))}
            />
          </>
        )}
      </Space>
    </Modal>
  );
}
