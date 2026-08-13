import React from 'react';
import { Modal } from 'antd';
import SchemaFormRenderer, { type SchemaFormRendererHandle } from '@/components/SchemaFormRenderer';
import type { FormValues } from '@/types/dashboard';
import { CLONE_FORM_SPEC } from './schemas';

type Props = {
  visible: boolean;
  onClose: () => void;
  onSave: (targetEnv: string) => Promise<void> | void;
};

export default function CloneModal({ visible, onClose, onSave }: Props) {
  const formRef = React.useRef<SchemaFormRendererHandle | null>(null);
  const [formValues, setFormValues] = React.useState<FormValues>({});

  React.useEffect(() => {
    if (!visible) return;
    setFormValues({});
  }, [visible]);

  return (
    <Modal
      title="克隆分配配置"
      open={visible}
      onCancel={onClose}
      onOk={() => {
        if (!formRef.current?.validate()) return;
        onSave(String(formRef.current.getValues().targetEnv || ''));
      }}
      width={520}
    >
      <SchemaFormRenderer
        ref={formRef}
        spec={CLONE_FORM_SPEC}
        initialValues={formValues}
        onValuesChange={(_, allValues) => setFormValues(allValues)}
        hideSubmit
      />
    </Modal>
  );
}
