import React from 'react';
import { Modal } from 'antd';
import SchemaRenderer from '@/components/formily/SchemaRenderer';
import { CLONE_FORM_SCHEMA } from './schemas';

type Props = {
  visible: boolean;
  onClose: () => void;
  onSave: (targetEnv: string) => Promise<void> | void;
};

export default function CloneModal({ visible, onClose, onSave }: Props) {
  const [formValues, setFormValues] = React.useState<Record<string, any>>({});

  React.useEffect(() => {
    if (!visible) return;
    setFormValues({});
  }, [visible]);

  return (
    <Modal
      title="克隆分配配置"
      open={visible}
      onCancel={onClose}
      onOk={() => onSave(String(formValues.targetEnv || ''))}
      width={520}
    >
      <SchemaRenderer schema={CLONE_FORM_SCHEMA} value={formValues} onChange={setFormValues} />
    </Modal>
  );
}
