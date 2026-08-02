import React from 'react';
import { Modal } from 'antd';
import type { AssignmentItem } from './types';
import SchemaFormRenderer, {
  type SchemaFormRendererHandle,
} from '@/components/SchemaFormRenderer';
import type { FormValues } from '@/types/dashboard';
import { CANARY_FORM_SPEC } from './schemas';

type Props = {
  visible: boolean;
  assignment: AssignmentItem | null;
  onClose: () => void;
  onSave: (values: FormValues) => void;
};

export default function CanaryModal({ visible, assignment, onClose, onSave }: Props) {
  const formRef = React.useRef<SchemaFormRendererHandle | null>(null);
  const [formValues, setFormValues] = React.useState<FormValues>({});

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
      onOk={() => {
        if (!formRef.current?.validate()) return;
        onSave(formRef.current.getValues());
      }}
      width={600}
    >
      {assignment ? (
        <SchemaFormRenderer
          ref={formRef}
          spec={CANARY_FORM_SPEC}
          initialValues={formValues}
          onValuesChange={(_, allValues) => setFormValues(allValues)}
          hideSubmit
        />
      ) : null}
    </Modal>
  );
}
