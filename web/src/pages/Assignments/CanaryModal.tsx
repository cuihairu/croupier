import React from 'react';
import { Modal } from 'antd';
import type { AssignmentItem } from './types';
import SchemaRenderer from '@/components/formily/SchemaRenderer';
import { CANARY_FORM_SCHEMA } from './schemas';

type Props = {
  visible: boolean;
  assignment: AssignmentItem | null;
  onClose: () => void;
  onSave: (values: Record<string, any>) => void;
};

export default function CanaryModal({ visible, assignment, onClose, onSave }: Props) {
  const [formValues, setFormValues] = React.useState<Record<string, any>>({});

  React.useEffect(() => {
    if (!visible) return;
    setFormValues({
      functionId: assignment?.id || '',
      enabled: assignment?.status === 'canary',
      percentage: assignment?.canary?.percentage ?? 10,
      rules: assignment?.canary?.rules ? JSON.stringify(assignment.canary.rules) : '',
      duration: assignment?.canary?.duration || '7d',
    });
  }, [visible, assignment]);

  return (
    <Modal
      title="灰度配置"
      open={visible}
      onCancel={onClose}
      onOk={() => onSave(formValues)}
      width={600}
    >
      {assignment ? (
        <SchemaRenderer schema={CANARY_FORM_SCHEMA} value={formValues} onChange={setFormValues} />
      ) : null}
    </Modal>
  );
}
