import React, { useEffect, useMemo, useState } from 'react';
import { Form, Input, Modal, Select, message } from 'antd';
import { useIntl } from '@umijs/max';
import dayjs from 'dayjs';
import {
  createIncident,
  fetchIncidentCategories,
  type IncidentCategory,
} from '@/services/api/incident';

/**
 * 一键转事故（docs/design/incident-reports.md §4.3）：告警行/构建失败行等
 * 自动源的一键登记入口。prefill 提供转换建议值（标题/严重度/时刻/Ref 关联/
 * 归因），类别与子类由操作者确认——v1 不做全自动归并。
 */
export type ConvertPrefill = {
  title: string;
  severity?: string;
  detectedAt?: string;
  refType?: string;
  refId?: string;
  categoryId?: number;
  subcategory?: string;
  responsibleType?: string;
  responsibleId?: string;
};

export function ConvertToIncidentModal({
  prefill,
  onClose,
  onConverted,
}: {
  prefill?: ConvertPrefill | null;
  onClose: () => void;
  onConverted?: () => void;
}) {
  const intl = useIntl();
  const [form] = Form.useForm();
  const [categories, setCategories] = useState<IncidentCategory[]>([]);
  const [categoryId, setCategoryId] = useState<number | undefined>(undefined);
  const [saving, setSaving] = useState(false);
  const text = (id: string, defaultMessage: string) => intl.formatMessage({ id, defaultMessage });

  useEffect(() => {
    if (prefill) {
      fetchIncidentCategories(true).then((resp) => setCategories(resp.items || []));
    }
  }, [prefill]);

  const selected = useMemo(
    () => categories.find((c) => c.id === categoryId),
    [categories, categoryId],
  );

  return (
    <Modal
      open={!!prefill}
      title={text('pages.incidents.convert.title', '转事故')}
      onCancel={onClose}
      confirmLoading={saving}
      destroyOnClose
      onOk={async () => {
        if (!prefill) return;
        const values = await form.validateFields();
        setSaving(true);
        try {
          await createIncident({
            title: values.title,
            categoryId: values.categoryId,
            subcategory: values.subcategory,
            severity: values.severity,
            responsibleType: values.responsibleType,
            responsibleId: values.responsibleId,
            detectedAt: prefill.detectedAt,
            refType: prefill.refType,
            refId: prefill.refId,
          });
          message.success(text('pages.incidents.convert.success', '已登记为事故'));
          onConverted?.();
          onClose();
        } finally {
          setSaving(false);
        }
      }}
    >
      <Form
        form={form}
        layout="vertical"
        preserve={false}
        initialValues={
          prefill
            ? {
                title: prefill.title,
                categoryId: prefill.categoryId,
                subcategory: prefill.subcategory,
                severity: prefill.severity || 'warning',
                responsibleType: prefill.responsibleType || 'unknown',
                responsibleId: prefill.responsibleId,
              }
            : undefined
        }
      >
        <Form.Item
          name="title"
          label={text('pages.incidents.form.title', '标题')}
          rules={[{ required: true }]}
        >
          <Input />
        </Form.Item>
        <Form.Item
          name="categoryId"
          label={text('pages.incidents.form.category', '类别')}
          rules={[
            {
              required: true,
              message: text('pages.incidents.form.categoryRequired', '请选择类别'),
            },
          ]}
        >
          <Select
            onChange={(v) => setCategoryId(v)}
            options={categories.map((c) => ({ value: c.id, label: c.name }))}
          />
        </Form.Item>
        <Form.Item name="subcategory" label={text('pages.incidents.form.subcategory', '子类')}>
          <Select
            allowClear
            disabled={!selected || selected.subcategories.length === 0}
            options={(selected?.subcategories || []).map((s) => ({ value: s, label: s }))}
          />
        </Form.Item>
        <Form.Item name="severity" label={text('pages.incidents.form.severity', '严重度')}>
          <Select options={['info', 'warning', 'critical'].map((s) => ({ value: s, label: s }))} />
        </Form.Item>
        <Form.Item
          name="responsibleType"
          label={text('pages.incidents.form.responsibleType', '归因类型')}
        >
          <Select
            options={[
              { value: 'unknown', label: text('pages.incidents.form.respUnknown', '未归因') },
              { value: 'agent', label: 'agent' },
              { value: 'operator', label: 'operator' },
              { value: 'change', label: 'change' },
            ]}
          />
        </Form.Item>
        <Form.Item
          name="responsibleId"
          label={text('pages.incidents.form.responsibleId', '归因对象')}
        >
          <Input />
        </Form.Item>
        {prefill?.refType && (
          <Form.Item label={text('pages.incidents.field.ref', '关联')}>
            <Input value={`${prefill.refType}:${prefill.refId || ''}`} disabled />
          </Form.Item>
        )}
        {prefill?.detectedAt && (
          <Form.Item label={text('pages.incidents.form.detectedAt', '检测时间')}>
            <Input value={dayjs(prefill.detectedAt).format('YYYY-MM-DD HH:mm:ss')} disabled />
          </Form.Item>
        )}
      </Form>
    </Modal>
  );
}

export default ConvertToIncidentModal;
